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
	if err := s.be.dailyBackup(now); err != nil {
		log.Printf("daily backup of the catalog database: %v", err)
	}
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

// closeForRestore commits every pending change, writes the pre-restore copy of the
// catalog database to preCopy, and closes the store, all without letting another
// change in between: the copy holds everything the store held. When the commit or
// the copy fails, the store stays open and nothing is changed. A failed close
// leaves the store closed; the caller opens the data folder again.
func (s *Store) closeForRestore(preCopy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.commitAllLocked(); err != nil {
		return fmt.Errorf("the restore stopped before changing anything: %w", err)
	}
	if _, err := s.be.snapshot(preCopy); err != nil {
		return fmt.Errorf("the restore stopped before changing anything: the pre-restore copy of the catalog database (%s) failed: %w", preCopy, err)
	}
	s.closed = true
	if err := s.be.close(); err != nil {
		return fmt.Errorf("the restore stopped before changing the catalog: closing the catalog database: %w", err)
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
