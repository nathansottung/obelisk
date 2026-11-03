package main

// store_backup.go — the store's side of catalog database copies (see
// catalogdb_backup.go): the daily backup after a commit, the snapshot an app
// backup or a data-folder move takes, and the close before a restore swaps the
// database.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// catalogDBBackupNow is the clock the daily backup of the catalog database reads.
// A test seam: a zero time makes no backup.
var catalogDBBackupNow = time.Now

// dailyBackupDBLocked writes the day's backup of the catalog database after a
// commit that saved everything, once a day. Best effort: a backup never fails a
// save; why one was not made is logged. Caller holds s.mu.
func (s *Store) dailyBackupDBLocked() {
	now := catalogDBBackupNow()
	if now.IsZero() {
		return
	}
	day := now.Format("20060102")
	if s.lastBak == day {
		return
	}
	s.lastBak = day
	s.notePersist("backup-create", s.path+".bak-"+day)
	// The copy reads the last commit through a connection of its own, so it is made
	// without holding the store: a large catalog takes a while, and the store keeps
	// answering meanwhile.
	be := s.be
	s.backups.Add(1)
	go func() {
		defer s.backups.Done()
		if err := be.dailyBackup(now); err != nil {
			log.Printf("daily backup of the catalog database: %v", err)
		}
	}()
}

// waitBackups waits for a daily backup being made.
func (s *Store) waitBackups() { s.backups.Wait() }

// runningJobsLocked names the jobs that are running. Caller holds s.jobs.mu.
func (s *Store) runningJobsLocked() []string {
	var out []string
	for _, j := range s.jobs.rows {
		if j.Status == "RUNNING" {
			out = append(out, fmt.Sprintf("%d (%s)", j.ID, j.Label))
		}
	}
	return out
}

// RunningJobs names the jobs that are running.
func (s *Store) RunningJobs() []string {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	return s.runningJobsLocked()
}

// errJobsRunning refuses a restore or a data folder move while jobs run: their work
// would go to a store that is closed or replaced under them.
func errJobsRunning(what string, jobs []string) error {
	return fmt.Errorf("%s waits until no job is running, so nothing was changed. Running: %s", what, strings.Join(jobs, ", "))
}

// errNotDatabase refuses a copy of the catalog database on catalog.json.
var errNotDatabase = errors.New("the catalog is not in a catalog database")

// snapshotDB commits every pending change, then writes a checked copy of the
// catalog database to dest, which must not exist, and returns its schema version.
// A change that cannot be committed refuses the copy, so a copy never leaves out
// what the store holds.
func (s *Store) snapshotDB(dest string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.commitAllLocked(); err != nil {
		return 0, err
	}
	return s.be.snapshot(dest)
}

// commitAllLocked commits every pending change on the database engine, or says
// why it cannot. Caller holds s.mu.
func (s *Store) commitAllLocked() error {
	if s.be == nil {
		return errNotDatabase
	}
	if s.closed {
		return errors.New("the catalog is closed")
	}
	if s.dirty && !s.readOnly {
		if err := s.writeCatalog(); err != nil {
			return fmt.Errorf("the catalog has changes that could not be saved: %w", err)
		}
	}
	return nil
}

// closeWithCopy commits every pending change, writes a copy of the catalog
// database to dest (the pre-restore copy, or the database of a data folder being
// moved), and closes the store, all without letting another change in between: the
// copy holds everything the store held. It is refused while a job runs, and the
// board is held throughout, so no job starts: after the close every new job is
// refused. When the commit or the copy fails, the store stays open and nothing is
// changed. A failed close leaves the store closed; the caller opens it again.
func (s *Store) closeWithCopy(what, dest string) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if running := s.runningJobsLocked(); len(running) > 0 {
		return errJobsRunning(what, running)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.commitAllLocked(); err != nil {
		return fmt.Errorf("%s stopped before changing anything: %w", what, err)
	}
	if _, err := s.be.snapshot(dest); err != nil {
		return fmt.Errorf("%s stopped before changing anything: the copy of the catalog database (%s) failed: %w", what, dest, err)
	}
	// The copy's folder entry is on disk before anything relies on it.
	_ = syncDir(filepath.Dir(dest))
	s.closed = true
	s.backups.Wait()
	if err := s.be.close(); err != nil {
		return fmt.Errorf("%s stopped before changing the catalog: closing the catalog database: %w", what, err)
	}
	return nil
}

// savePending saves every pending change, on either engine, for a restore that
// is about to copy the records. A change lost earlier is not counted against it:
// it is already gone.
func (s *Store) savePending() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("the catalog is closed")
	}
	if s.dirty && !s.readOnly {
		return s.writeCatalog()
	}
	return nil
}

// isClosed reports whether the store has been closed.
func (s *Store) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// snapshotDBBytes is snapshotDB into a file beside the catalog database, read back
// and removed: the catalog database whole, for an app backup.
func (s *Store) snapshotDBBytes() ([]byte, int, error) {
	tok := make([]byte, 6)
	if _, err := rand.Read(tok); err != nil {
		return nil, 0, err
	}
	tmp := s.path + ".export-" + hex.EncodeToString(tok)
	defer func() {
		for _, side := range []string{"", "-wal", "-shm", "-journal"} {
			os.Remove(tmp + side)
		}
	}()
	schema, err := s.snapshotDB(tmp)
	if err != nil {
		return nil, 0, err
	}
	b, err := os.ReadFile(tmp)
	return b, schema, err
}
