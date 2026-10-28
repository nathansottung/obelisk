//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// holdWithoutSharing opens path with no sharing at all, the way a scanner that
// locks a file while it reads it does: nobody else can open it until it is
// released. It returns a function that closes the handle; calling it more than
// once is harmless.
func holdWithoutSharing(t *testing.T, path string) func() {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("open %s without sharing: %v", path, err)
	}
	closed := false
	release := func() {
		if !closed {
			closed = true
			_ = syscall.CloseHandle(h)
		}
	}
	t.Cleanup(release)
	return release
}

// existingCatalog makes a data folder that holds a saved catalog and jobs file.
func existingCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddCollection("Kept")
	if _, err := s.NewJob("scan", "Kept"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalog.json", "jobs.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("setup: %s was not saved: %v", name, err)
		}
	}
	return dir
}

func TestOpenStore_RetriesWhileFileIsBrieflyLocked(t *testing.T) {
	for _, name := range []string{"catalog.json", "jobs.json"} {
		t.Run(name, func(t *testing.T) {
			dir := existingCatalog(t)
			release := holdWithoutSharing(t, filepath.Join(dir, name))
			// The lock is real: a plain read fails while it is held.
			if _, err := os.ReadFile(filepath.Join(dir, name)); err == nil || !transientRenameError(err) {
				t.Fatalf("read while held: got %v, want a transient lock error", err)
			}
			timer := time.AfterFunc(200*time.Millisecond, release)
			defer timer.Stop()

			start := time.Now()
			s, err := OpenStore(dir)
			if err != nil {
				t.Fatalf("open after the lock was released: %v", err)
			}
			if elapsed := time.Since(start); elapsed < 150*time.Millisecond {
				t.Fatalf("open returned after %v, before the lock was released", elapsed)
			}
			found := false
			for _, c := range s.Collections() {
				if c.Name == "Kept" {
					found = true
				}
			}
			if !found {
				t.Fatal("the reopened catalog is not the saved one")
			}
		})
	}
}

func TestOpenStore_FailsWhenLockOutlastsTheLimit(t *testing.T) {
	dir := existingCatalog(t)
	catalog := filepath.Join(dir, "catalog.json")
	before, err := os.ReadFile(catalog)
	if err != nil {
		t.Fatal(err)
	}
	release := holdWithoutSharing(t, catalog)
	start := time.Now()
	_, err = OpenStore(dir)
	elapsed := time.Since(start)
	release()
	if err == nil {
		t.Fatal("open succeeded while the catalog stayed locked")
	}
	if !transientRenameError(err) {
		t.Fatalf("got %v, want the lock error from the read", err)
	}
	if elapsed < renameRetryLimit {
		t.Fatalf("gave up after %v, before the %v limit", elapsed, renameRetryLimit)
	}
	if elapsed > renameRetryLimit+time.Second {
		t.Fatalf("kept trying for %v, well past the %v limit", elapsed, renameRetryLimit)
	}
	// Refusing to open never replaces the catalog it could not read.
	after, err := os.ReadFile(catalog)
	if err != nil || string(after) != string(before) {
		t.Fatalf("the catalog changed after a refused open (err=%v)", err)
	}
}
