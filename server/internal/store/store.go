// Package store owns the SQLite database: opening with the required pragmas,
// migrations, users and sessions. One write connection, a small read pool.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure Go driver, registers "sqlite"
)

//go:embed schema.sql
var schemaV1 string

// migrations[i] upgrades user_version i to i+1.
var migrations = []string{schemaV1}

const (
	SessionTTL = 30 * 24 * time.Hour
	readConns  = 4
)

var ErrNotFound = errors.New("store: not found")

type Store struct {
	w *sql.DB // single write connection
	r *sql.DB // read pool
}

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Nama  string `json:"nama"`
	Plan  string `json:"plan"`
}

type Session struct {
	UserID    int64
	CreatedAt int64
	ExpiresAt int64
	LastSeen  int64
}

// Open creates the directory (0700) and file (0600) when missing, applies
// migrations and returns the store.
func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := prepareDir(dir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, err
	}

	s := &Store{}
	// _txlock=immediate makes every write transaction BEGIN IMMEDIATE.
	if s.w, err = openDB(path, "_txlock=immediate"); err != nil {
		return nil, err
	}
	s.w.SetMaxOpenConns(1)
	if err := s.migrate(); err != nil {
		s.w.Close()
		return nil, err
	}
	if s.r, err = openDB(path, "_pragma=query_only(1)"); err != nil {
		s.w.Close()
		return nil, err
	}
	s.r.SetMaxOpenConns(readConns)
	s.r.SetMaxIdleConns(readConns)
	return s, nil
}

// prepareDir creates dir (and missing parents) with mode 0700. Only the
// directories created here are chmod-ed; an existing directory keeps its mode
// (it may be shared, like /tmp) and only produces a warning when it is open to
// group or others.
func prepareDir(dir string) error {
	var created []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			break
		}
		created = append(created, d)
		if filepath.Dir(d) == d {
			break
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for _, d := range created {
		if err := os.Chmod(d, 0o700); err != nil {
			return err
		}
	}
	if len(created) == 0 {
		if fi, err := os.Stat(dir); err == nil && fi.Mode().Perm()&0o077 != 0 {
			log.Printf("store: warning: database directory is accessible by group or others (mode %04o)", fi.Mode().Perm())
		}
	}
	return nil
}

func openDB(path, extra string) (*sql.DB, error) {
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() +
		"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&" + extra
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *Store) migrate() error {
	var v int
	if err := s.w.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v > len(migrations) {
		return fmt.Errorf("store: database schema version %d is newer than this binary (%d)", v, len(migrations))
	}
	for ; v < len(migrations); v++ {
		err := s.Tx(context.Background(), func(tx *sql.Tx) error {
			if _, err := tx.Exec(migrations[v]); err != nil {
				return err
			}
			_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", v+1))
			return err
		})
		if err != nil {
			return fmt.Errorf("store: migration %d: %w", v+1, err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return errors.Join(s.r.Close(), s.w.Close())
}

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.r.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

// Reader returns the read pool (query_only). Never use it for writes.
func (s *Store) Reader() *sql.DB { return s.r }

// Tx runs fn in a BEGIN IMMEDIATE transaction on the write connection and
// commits when fn returns nil, otherwise rolls back.
func (s *Store) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.w.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpsertUser finds the user by sub or creates it. The email is refreshed on
// every login; nama (display cache) is only filled when still empty.
func (s *Store) UpsertUser(ctx context.Context, sub, email, nama string, now time.Time) (u User, created bool, err error) {
	return s.upsertUser(ctx, sub, email, nama, now, false)
}

// UpsertLoginUser is UpsertUser for logins whose provider is the source of
// truth (Google): email and a non-empty nama replace the cached values on every
// login. An empty nama never erases the cached one.
func (s *Store) UpsertLoginUser(ctx context.Context, sub, email, nama string, now time.Time) (u User, created bool, err error) {
	return s.upsertUser(ctx, sub, email, nama, now, true)
}

func (s *Store) upsertUser(ctx context.Context, sub, email, nama string, now time.Time, overwriteNama bool) (u User, created bool, err error) {
	t := now.Unix()
	over := 0
	if overwriteNama {
		over = 1
	}
	err = s.Tx(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `UPDATE users SET email=?1, nama=CASE WHEN nama='' OR (?5<>0 AND ?2<>'') THEN ?2 ELSE nama END, last_seen=?3
			WHERE sub=?4 RETURNING id,email,nama,plan`, email, nama, t, sub, over).Scan(&u.ID, &u.Email, &u.Nama, &u.Plan)
		if e == nil {
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		created = true
		return tx.QueryRowContext(ctx, `INSERT INTO users(sub,email,nama,created_at,last_seen) VALUES(?,?,?,?,?)
			RETURNING id,email,nama,plan`, sub, email, nama, t, t).Scan(&u.ID, &u.Email, &u.Nama, &u.Plan)
	})
	return u, created && err == nil, err
}

// UserRev returns users.rev, the sync cursor of the user.
func (s *Store) UserRev(ctx context.Context, userID int64) (int64, error) {
	var rev int64
	err := s.r.QueryRowContext(ctx, `SELECT rev FROM users WHERE id=?`, userID).Scan(&rev)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return rev, err
}

// NewToken returns a random session token (32 bytes, base64url, 43 chars) and
// the hash that is stored in the database.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CreateSession stores a session valid for SessionTTL.
func (s *Store) CreateSession(ctx context.Context, userID int64, hash []byte, now time.Time) error {
	t := now.Unix()
	return s.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,created_at,expires_at,last_seen) VALUES(?,?,?,?,?)`,
			hash, userID, t, now.Add(SessionTTL).Unix(), t)
		return err
	})
}

// LookupSession returns the user and session for a token hash, or ErrNotFound
// when the session is missing or expired.
func (s *Store) LookupSession(ctx context.Context, hash []byte, now time.Time) (User, Session, error) {
	var u User
	var ss Session
	err := s.r.QueryRowContext(ctx, `SELECT u.id,u.email,u.nama,u.plan,s.created_at,s.expires_at,s.last_seen
		FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, hash, now.Unix()).
		Scan(&u.ID, &u.Email, &u.Nama, &u.Plan, &ss.CreatedAt, &ss.ExpiresAt, &ss.LastSeen)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ss, ErrNotFound
	}
	ss.UserID = u.ID
	return u, ss, err
}

// DeleteSession removes a session. Deleting a missing one is not an error.
func (s *Store) DeleteSession(ctx context.Context, hash []byte) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, hash)
		return err
	})
}

// ExtendSession moves the expiry of a session to expiresAt and its last_seen to
// now, only when the stored expiry is earlier and last_seen is not after
// seenBefore. The conditions make concurrent callers write once. It reports
// whether a row changed.
func (s *Store) ExtendSession(ctx context.Context, hash []byte, expiresAt, now, seenBefore time.Time) (bool, error) {
	var n int64
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE sessions SET expires_at=?1, last_seen=?2
			WHERE token_hash=?3 AND expires_at<?1 AND last_seen<=?4 AND expires_at>?2`,
			expiresAt.Unix(), now.Unix(), hash, seenBefore.Unix())
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return n > 0, err
}

// DeleteExpiredSessions removes sessions that expired at or before now and
// returns how many.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	var n int64
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, now.Unix())
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}

// VacuumInto writes a consistent copy of the database to path (which must not
// exist). The path comes from the program, never from a client. It holds the
// write connection while it runs.
func (s *Store) VacuumInto(ctx context.Context, path string) error {
	_, err := s.w.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}

// PruneMut deletes sync mutation ids recorded before the given time and returns how many.
func (s *Store) PruneMut(ctx context.Context, before time.Time) (int64, error) {
	var n int64
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM mut WHERE at<?`, before.Unix())
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}
