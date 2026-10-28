//go:build linux && !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Every write re-reads the destination: a folder moved away and replaced, or one
// whose mode changes, during a job stops the job.
func TestDestRoot_WriteStopsWhenFolderChangesLinux(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, dir string)
	}{
		{"replaced", func(t *testing.T, dir string) {
			if err := os.Rename(dir, dir+"-old"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode changed", func(t *testing.T, dir string) {
			if err := os.Chmod(dir, 0o750); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := dockApp(t)
			dir := filepath.Join(t.TempDir(), "dest")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			d, err := app.Store.AuthorizeDestination(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.WriteFileAtomic("first.txt", []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			tc.change(t, dir)
			var ch *DestinationChangedError
			if err := d.WriteFileAtomic("second.txt", []byte("2"), 0o644); !errors.As(err, &ch) {
				t.Fatalf("write after the change: got %v, want a DestinationChangedError", err)
			}
		})
	}
}
