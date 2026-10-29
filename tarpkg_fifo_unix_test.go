//go:build !windows && !guionly

package main

import (
	"archive/tar"
	"io"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A FIFO put where a catalogued file was is refused, not opened: opening it
// would block the build forever.
func TestAddTarFile_RefusesAFIFO(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(p, 0o644); err != nil {
		t.Skipf("no FIFO here: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- addTarFile(tar.NewWriter(io.Discard), p, "pipe") }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("adding a FIFO: %v, want refused", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("adding a FIFO blocked")
	}
}
