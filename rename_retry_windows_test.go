//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// holdWithoutShareDelete opens path the way an antivirus or indexer scan does: shared
// for reading and writing but not for delete, which is what a rename of the file needs.
// It returns a function that closes the handle; calling it more than once is harmless.
func holdWithoutShareDelete(t *testing.T, path string) func() {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("open %s without share-delete: %v", path, err)
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

func stageReplacement(t *testing.T) (tmp, final string) {
	t.Helper()
	dir := t.TempDir()
	final = filepath.Join(dir, "config.json")
	tmp = filepath.Join(dir, ".config-staged.tmp")
	mustWrite(t, final, "old")
	mustWrite(t, tmp, "new")
	return tmp, final
}

func TestRenameReplacing_RetriesUntilLockReleased(t *testing.T) {
	tmp, final := stageReplacement(t)
	release := holdWithoutShareDelete(t, tmp)
	// The lock is real: a plain rename fails while it is held.
	if err := os.Rename(tmp, final); err == nil || !transientRenameError(err) {
		t.Fatalf("rename while held: got %v, want a transient lock error", err)
	}
	timer := time.AfterFunc(200*time.Millisecond, release)
	defer timer.Stop()

	start := time.Now()
	if err := renameReplacing(tmp, final); err != nil {
		t.Fatalf("rename after the lock was released: %v", err)
	}
	if took := time.Since(start); took < 150*time.Millisecond || took > renameRetryLimit {
		t.Fatalf("took %v; want it to wait for the ~200ms release and finish inside the limit", took)
	}
	mustBytes(t, final, "new", "rename after release")
	if fileExists(tmp) {
		t.Fatal("staged file still present after a successful rename")
	}
}

func TestRenameReplacing_GivesUpAtLimitWithOriginalError(t *testing.T) {
	tmp, final := stageReplacement(t)
	release := holdWithoutShareDelete(t, tmp)
	want := os.Rename(tmp, final)
	if want == nil || !transientRenameError(want) {
		t.Fatalf("rename while held: got %v, want a transient lock error", want)
	}

	start := time.Now()
	err := renameReplacing(tmp, final)
	took := time.Since(start)
	release()

	var got, wantErrno syscall.Errno
	if !errors.As(err, &got) || !errors.As(want, &wantErrno) || got != wantErrno {
		t.Fatalf("got %v, want the rename's own error %v", err, want)
	}
	var link *os.LinkError
	if !errors.As(err, &link) || link.Old != tmp || link.New != final {
		t.Fatalf("got %#v, want the unwrapped *os.LinkError for %s -> %s", err, tmp, final)
	}
	if took < renameRetryLimit || took > renameRetryLimit+time.Second {
		t.Fatalf("gave up after %v; want about %v", took, renameRetryLimit)
	}
	mustBytes(t, final, "old", "rename that hit the limit")
	mustBytes(t, tmp, "new", "rename that hit the limit (staged file is the caller's)")
}

func TestRenameReplacing_OtherErrorsAreNotRetried(t *testing.T) {
	calls := 0
	prev := renameFile
	renameFile = func(string, string) error {
		calls++
		return &os.LinkError{Op: "rename", Err: syscall.ERROR_PATH_NOT_FOUND}
	}
	t.Cleanup(func() { renameFile = prev })

	err := renameReplacing("a", "b")
	if !errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) || calls != 1 {
		t.Fatalf("got %v after %d attempts; want the error after exactly 1", err, calls)
	}
}

func TestRenameReplacing_RecognisesLockErrors(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{&os.LinkError{Op: "rename", Err: syscall.Errno(5)}, true},
		{&os.LinkError{Op: "rename", Err: syscall.Errno(32)}, true},
		{&os.LinkError{Op: "rename", Err: syscall.Errno(33)}, true},
		{&os.LinkError{Op: "rename", Err: syscall.Errno(2)}, false},
		{&os.LinkError{Op: "rename", Err: syscall.Errno(17)}, false}, // not same device
		{os.ErrPermission, false},
		{nil, false},
	} {
		if got := transientRenameError(c.err); got != c.want {
			t.Errorf("transientRenameError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}
