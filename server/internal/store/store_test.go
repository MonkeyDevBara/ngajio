package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var ctx = context.Background()

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "ngajio.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestOpenPermsAndPragmas(t *testing.T) {
	s, path := open(t)
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("db mode %v, want 0600", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(filepath.Dir(path)); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode %v, want 0700", fi.Mode().Perm())
	}
	for name, db := range map[string]*sql.DB{"write": s.w, "read": s.r} {
		want := map[string]string{"journal_mode": "wal", "synchronous": "1", "busy_timeout": "5000", "foreign_keys": "1"}
		for p, w := range want {
			var got string
			if err := db.QueryRow("PRAGMA " + p).Scan(&got); err != nil || got != w {
				t.Errorf("%s pool PRAGMA %s = %q (%v), want %q", name, p, got, err, w)
			}
		}
	}
}

func TestWALFilesMode(t *testing.T) {
	s, path := open(t)
	if _, _, err := s.UpsertUser(ctx, "s", "a@b.c", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, suf := range []string{"-wal", "-shm"} {
		fi, err := os.Stat(path + suf)
		if err != nil {
			t.Fatalf("%s: %v", suf, err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode %v is open to group/other", suf, fi.Mode().Perm())
		}
	}
}

// An existing directory (maybe shared, like /tmp) keeps its mode and only
// produces a warning; the DB file itself is still 0600.
func TestOpenKeepsExistingDirMode(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0o755)
	path := filepath.Join(dir, "x.db")
	os.WriteFile(path, nil, 0o644)

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("db mode %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o755 {
		t.Errorf("existing dir mode changed to %v", fi.Mode().Perm())
	}
	if !strings.Contains(logs.String(), "accessible by group or others") || strings.Contains(logs.String(), dir) {
		t.Errorf("want a warning without the path, got %q", logs.String())
	}
	s.UpsertUser(ctx, "s", "a@b.c", "", time.Now())
	for _, suf := range []string{"-wal", "-shm"} {
		if fi, err := os.Stat(path + suf); err != nil || fi.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v %v", suf, fi, err)
		}
	}
}

func TestOpenSilentForPrivateExistingDir(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0o700)
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	s, err := Open(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if logs.Len() != 0 {
		t.Errorf("unexpected log %q", logs.String())
	}
}

// Every directory Open creates is 0700, nested ones included.
func TestOpenCreatesNestedDirsPrivate(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0o755)
	path := filepath.Join(root, "a", "b", "ngajio.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, d := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b")} {
		if fi, _ := os.Stat(d); fi.Mode().Perm() != 0o700 {
			t.Errorf("%s mode %v", d, fi.Mode().Perm())
		}
	}
	if fi, _ := os.Stat(root); fi.Mode().Perm() != 0o755 {
		t.Errorf("existing parent changed to %v", fi.Mode().Perm())
	}
}

func TestSchemaAndMigrationIdempotent(t *testing.T) {
	s, path := open(t)
	var v int
	s.w.QueryRow("PRAGMA user_version").Scan(&v)
	if v != len(migrations) {
		t.Fatalf("user_version %d", v)
	}
	for _, tbl := range []string{"users", "sessions", "rec", "mut"} {
		var n int
		if err := s.r.QueryRow("SELECT count(*) FROM " + tbl).Scan(&n); err != nil {
			t.Errorf("table %s: %v", tbl, err)
		}
	}
	for _, idx := range []string{"sessions_user", "sessions_exp", "rec_rev"} {
		var n int
		s.r.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&n)
		if n != 1 {
			t.Errorf("index %s missing", idx)
		}
	}
	s.Close()
	s2, err := Open(path) // reopening must not re-run the DDL
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
}

func TestNewerSchemaRefused(t *testing.T) {
	s, path := open(t)
	s.w.Exec("PRAGMA user_version=99")
	s.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("want newer-version error, got %v", err)
	}
}

func TestReadPoolIsReadOnly(t *testing.T) {
	s, _ := open(t)
	if _, err := s.r.Exec("INSERT INTO mut VALUES(1,'x',1)"); err == nil {
		t.Fatal("read pool accepted a write")
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	s, _ := open(t)
	err := s.CreateSession(ctx, 999, HashToken("x"), time.Now())
	if err == nil {
		t.Fatal("session for missing user accepted")
	}
}

func TestUpsertUser(t *testing.T) {
	s, _ := open(t)
	now := time.Now()
	u, created, err := s.UpsertUser(ctx, "dev:a@b.c", "a@b.c", "", now)
	if err != nil || !created || u.ID == 0 || u.Plan != "gratis" || u.Email != "a@b.c" {
		t.Fatalf("first: %+v %v %v", u, created, err)
	}
	u2, created, err := s.UpsertUser(ctx, "dev:a@b.c", "new@b.c", "Ust", now)
	if err != nil || created || u2.ID != u.ID || u2.Email != "new@b.c" || u2.Nama != "Ust" {
		t.Fatalf("second: %+v %v %v", u2, created, err)
	}
	u3, _, _ := s.UpsertUser(ctx, "dev:a@b.c", "new@b.c", "Other", now)
	if u3.Nama != "Ust" {
		t.Errorf("nama overwritten: %q", u3.Nama)
	}
	u4, created, _ := s.UpsertUser(ctx, "dev:other@b.c", "other@b.c", "", now)
	if !created || u4.ID == u.ID {
		t.Errorf("other sub must be a different user: %+v", u4)
	}
}

func TestUpsertUserConcurrent(t *testing.T) {
	s, _ := open(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	made, ids := 0, map[int64]bool{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u, c, err := s.UpsertUser(ctx, "same", "a@b.c", "", time.Now())
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			ids[u.ID] = true
			if c {
				made++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if made != 1 || len(ids) != 1 {
		t.Fatalf("created %d times, %d ids", made, len(ids))
	}
}

func TestToken(t *testing.T) {
	a, ha, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewToken()
	if len(a) != 43 || a == b || len(ha) != 32 || !bytes.Equal(ha, HashToken(a)) {
		t.Fatalf("bad token %q", a)
	}
	if strings.ContainsAny(a, "+/=") {
		t.Errorf("token not base64url: %q", a)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s, _ := open(t)
	now := time.Now()
	u, _, _ := s.UpsertUser(ctx, "s", "a@b.c", "", now)
	tok, hash, _ := NewToken()
	if err := s.CreateSession(ctx, u.ID, hash, now); err != nil {
		t.Fatal(err)
	}
	got, ss, err := s.LookupSession(ctx, HashToken(tok), now)
	if err != nil || got != u || ss.UserID != u.ID || ss.ExpiresAt != now.Add(SessionTTL).Unix() {
		t.Fatalf("lookup: %+v %+v %v", got, ss, err)
	}
	// only the hash is stored, never the token
	var n int
	s.r.QueryRow("SELECT count(*) FROM sessions WHERE token_hash=?", []byte(tok)).Scan(&n)
	if n != 0 {
		t.Error("raw token stored")
	}
	if _, _, err := s.LookupSession(ctx, HashToken("wrong"), now); !errors.Is(err, ErrNotFound) {
		t.Errorf("wrong token: %v", err)
	}
	// expiry boundary
	if _, _, err := s.LookupSession(ctx, hash, now.Add(SessionTTL-time.Second)); err != nil {
		t.Errorf("still valid: %v", err)
	}
	if _, _, err := s.LookupSession(ctx, hash, now.Add(SessionTTL)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired: %v", err)
	}
	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.LookupSession(ctx, hash, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: %v", err)
	}
	if err := s.DeleteSession(ctx, hash); err != nil {
		t.Errorf("second delete: %v", err)
	}
}

func TestTxRollbackAndCommit(t *testing.T) {
	s, _ := open(t)
	boom := errors.New("boom")
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		tx.Exec("INSERT INTO users(sub,email,created_at,last_seen) VALUES('r','r@r.r',1,1)")
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var n int
	s.r.QueryRow("SELECT count(*) FROM users").Scan(&n)
	if n != 0 {
		t.Fatal("rollback failed")
	}
	if err := s.Tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO users(sub,email,created_at,last_seen) VALUES('r','r@r.r',1,1)")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s.r.QueryRow("SELECT count(*) FROM users").Scan(&n)
	if n != 1 {
		t.Fatal("commit failed")
	}
}

func TestSessionsCascadeOnUserDelete(t *testing.T) {
	s, _ := open(t)
	u, _, _ := s.UpsertUser(ctx, "s", "a@b.c", "", time.Now())
	_, h, _ := NewToken()
	s.CreateSession(ctx, u.ID, h, time.Now())
	s.Tx(ctx, func(tx *sql.Tx) error { _, err := tx.Exec("DELETE FROM users WHERE id=?", u.ID); return err })
	var n int
	s.r.QueryRow("SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Fatal("cascade failed")
	}
}

func TestPing(t *testing.T) {
	s, _ := open(t)
	if err := s.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err := s.Ping(ctx); err == nil {
		t.Fatal("ping on closed store must fail")
	}
}

// Tx must take the write lock at BEGIN (BEGIN IMMEDIATE): a second immediate
// transaction from another connection cannot start while the first is open.
func TestTxIsImmediate(t *testing.T) {
	s, path := open(t)
	other, err := sql.Open("sqlite", "file:"+path+"?_txlock=immediate&_pragma=busy_timeout(50)")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	err = s.Tx(ctx, func(*sql.Tx) error {
		tx, err := other.Begin()
		if err == nil {
			tx.Rollback()
			t.Error("second immediate tx started while first held the write lock")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
