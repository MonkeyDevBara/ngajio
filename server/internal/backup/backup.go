// Package backup makes a daily SQLite copy (VACUUM INTO) with rotation, and
// prunes old sync mutation ids in the same daily job.
package backup

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"ngajio/server/internal/store"
)

const (
	Keep       = 7                   // backups kept
	MutMaxAge  = 90 * 24 * time.Hour // age after which mut rows are pruned
	checkEvery = time.Hour
	dayLayout  = "20060102"
)

var nameRe = regexp.MustCompile(`^ngajio-\d{8}\.db$`)

type Job struct {
	Store  *store.Store
	Dir    string           // empty: no backup files (mut is still pruned)
	Now    func() time.Time // nil means time.Now
	Logger *log.Logger

	done string // day of the last successful run
}

func (j *Job) now() time.Time {
	if j.Now != nil {
		return j.Now()
	}
	return time.Now()
}

// Run does the job at start-up, then once per calendar day, until ctx is done.
func (j *Job) Run(ctx context.Context) {
	t := time.NewTicker(checkEvery)
	defer t.Stop()
	for {
		if err := j.RunOnce(ctx); err != nil && ctx.Err() == nil && j.Logger != nil {
			j.Logger.Printf("backup: %v", err) // never contains record data
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// RunOnce does the daily job unless it already ran for today's date. The backup
// file is skipped when today's file exists.
func (j *Job) RunOnce(ctx context.Context) error {
	now := j.now()
	day := now.Format(dayLayout)
	if j.done == day {
		return nil
	}
	if j.Dir != "" {
		if err := j.backup(ctx, day); err != nil {
			return err
		}
	}
	if _, err := j.Store.PruneMut(ctx, now.Add(-MutMaxAge)); err != nil {
		return err
	}
	j.done = day
	return nil
}

func (j *Job) backup(ctx context.Context, day string) error {
	if err := os.MkdirAll(j.Dir, 0o700); err != nil {
		return err
	}
	final := filepath.Join(j.Dir, "ngajio-"+day+".db")
	if _, err := os.Stat(final); err == nil {
		return j.rotate()
	}
	tmp := final + ".tmp"
	os.Remove(tmp)
	if err := j.Store.VacuumInto(ctx, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return err
	}
	return j.rotate()
}

// rotate keeps the Keep newest backups (names sort by date).
func (j *Job) rotate() error {
	entries, err := os.ReadDir(j.Dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && nameRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > Keep {
		if err := os.Remove(filepath.Join(j.Dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
