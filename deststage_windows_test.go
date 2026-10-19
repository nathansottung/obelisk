//go:build windows && !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Replacing a destination file that another process briefly holds open without
// share-delete (antivirus scanning it, say) waits for the lock rather than failing.
func TestDestStage_PublishRetriesUntilLockReleased(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"good": "OLD"})
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	release := holdWithoutShareDelete(t, filepath.Join(dir, "good"))
	timer := time.AfterFunc(200*time.Millisecond, release)
	defer timer.Stop()
	start := time.Now()
	if err := d.WriteFileAtomic("good", []byte("NEW"), 0o600); err != nil {
		t.Fatalf("publish over a briefly locked file: %v", err)
	}
	if took := time.Since(start); took < 150*time.Millisecond || took > renameRetryLimit {
		t.Fatalf("took %v; want it to wait for the ~200ms release and finish inside the limit", took)
	}
	mustBytes(t, filepath.Join(dir, "good"), "NEW", "publish after release")
	assertNoStagingIn(t, dir)
}

// Held past the limit, the publication fails with the rename's own error, the
// existing file keeps its bytes, and the staged copy is cleaned up.
func TestDestStage_PublishGivesUpAtLimit(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"good": "OLD"})
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	release := holdWithoutShareDelete(t, filepath.Join(dir, "good"))
	start := time.Now()
	err = d.WriteFileAtomic("good", []byte("NEW"), 0o600)
	took := time.Since(start)
	release()
	if err == nil {
		t.Fatal("publication over a file held past the limit succeeded")
	}
	if !transientRenameError(err) {
		t.Fatalf("got %v, want the rename's own lock error", err)
	}
	if took < renameRetryLimit || took > renameRetryLimit+time.Second {
		t.Fatalf("gave up after %v; want about %v", took, renameRetryLimit)
	}
	mustBytes(t, filepath.Join(dir, "good"), "OLD", "publication that hit the limit")
	assertNoStagingIn(t, dir)
}

// The handle rename recognises the NTSTATUS forms of the three lock errors, and
// retries nothing else.
func TestDestStage_HandleRenameErrorsClassified(t *testing.T) {
	for status, want := range map[windows.NTStatus]bool{
		windows.STATUS_SHARING_VIOLATION:     true,
		windows.STATUS_ACCESS_DENIED:         true,
		windows.STATUS_FILE_LOCK_CONFLICT:    true,
		windows.STATUS_OBJECT_NAME_NOT_FOUND: false,
		windows.STATUS_NOT_SAME_DEVICE:       false,
	} {
		if got := transientRenameError(status); got != want {
			t.Errorf("%v: transient=%v, want %v", status, got, want)
		}
	}

	calls := 0
	prev := handleRename
	handleRename = func(windows.Handle, *windows.IO_STATUS_BLOCK, *byte, uint32) error {
		calls++
		return windows.STATUS_OBJECT_NAME_NOT_FOUND
	}
	t.Cleanup(func() { handleRename = prev })
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	err = d.WriteFileAtomic("x", []byte("x"), 0o600)
	if !errors.Is(err, windows.STATUS_OBJECT_NAME_NOT_FOUND) || calls != 1 {
		t.Fatalf("got %v after %d attempts; want that error after exactly 1", err, calls)
	}
}

func assertNoStagingIn(t *testing.T, dir string) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), mirrorTmpSuffix) {
			t.Errorf("staging left behind: %s", e.Name())
		}
	}
}
