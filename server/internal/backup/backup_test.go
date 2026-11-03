package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ngajio/server/internal/store"
)

func TestBackupCreatesRotatesAndPrunes(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "db", "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, _, err := st.UpsertUser(ctx, "dev:a", "a@x.id", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 3, 6, 0, 0, 0, time.UTC)
	old := now.Add(-100 * 24 * time.Hour).Unix()
	st.Tx(ctx, func(tx *sql.Tx) error {
		tx.Exec(`INSERT INTO mut(user_id,mid,at) VALUES(1,'old',?),(1,'new',?)`, old, now.Unix())
		return nil
	})

	bdir := filepath.Join(dir, "bk")
	j := &Job{Store: st, Dir: bdir, Now: func() time.Time { return now }}
	if err := j.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(bdir, "ngajio-20261103.db")
	fi, err := os.Stat(f)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("backup file: %v %v", fi, err)
	}
	bk, err := sql.Open("sqlite", "file:"+f+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	var email string
	if err := bk.QueryRow(`SELECT email FROM users`).Scan(&email); err != nil || email != "a@x.id" {
		t.Fatalf("backup content: %q %v", email, err)
	}
	bk.Close()

	var n int
	st.Reader().QueryRow(`SELECT count(*) FROM mut`).Scan(&n)
	if n != 1 {
		t.Fatalf("mut rows after prune: %d", n)
	}

	// ten more days: only the last 7 files stay; same-day second run does nothing
	if err := j.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 10; i++ {
		now = now.Add(24 * time.Hour)
		if err := j.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	ents, _ := os.ReadDir(bdir)
	if len(ents) != 7 || ents[0].Name() != "ngajio-20261107.db" || ents[6].Name() != "ngajio-20261113.db" {
		t.Fatalf("rotation: %v", ents)
	}
}

func TestEmptyDirSkipsBackup(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "ngajio.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := (&Job{Store: st}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}
