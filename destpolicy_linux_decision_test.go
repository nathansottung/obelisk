//go:build linux && !guionly

package main

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// withDestStatfs presents every filesystem as the given magic number for one test.
func withDestStatfs(t *testing.T, magic int64) {
	t.Helper()
	prev := destStatfsType
	destStatfsType = func(*os.File) (int64, error) { return magic, nil }
	t.Cleanup(func() { destStatfsType = prev })
}

func TestDestPolicy_DockerOverlayRefused(t *testing.T) {
	app := dockApp(t)
	withDestStatfs(t, unix.OVERLAYFS_SUPER_MAGIC)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err == nil {
		d.Close()
		t.Fatal("overlay destination accepted")
	}
	for _, want := range []string{"container overlay", "mount a host folder"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("overlay refusal should say %q: %v", want, err)
		}
	}
}

// exFAT and FAT owner and mode bits come from mount options, so a folder that
// looks world-writable is accepted there, and the result says what was not checked.
func TestDestPolicy_LinuxFATDestinationAcceptedWithNotes(t *testing.T) {
	for name, magic := range map[string]int64{"exFAT": destMagicExFAT, "FAT": destMagicMSDOS} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			dest := t.TempDir()
			if err := os.Chmod(dest, 0o777); err != nil {
				t.Fatal(err)
			}
			if d, err := app.Store.AuthorizeDestination(dest); err == nil {
				d.Close()
				t.Fatal("precondition: a world-writable ext4/tmpfs folder must be refused")
			}
			withDestStatfs(t, magic)
			d, err := app.Store.AuthorizeDestination(dest)
			if err != nil {
				t.Fatalf("%s destination refused: %v", name, err)
			}
			defer d.Close()
			notes := strings.Join(d.Notes(), "\n")
			for _, want := range []string{"folder permissions", "hard links", "stable file IDs"} {
				if !strings.Contains(notes, want) {
					t.Errorf("notes do not mention %q:\n%s", want, notes)
				}
			}
			if name == "FAT" && d.MaxFileSize() != 1<<32-1 {
				t.Errorf("FAT max file %d", d.MaxFileSize())
			}
		})
	}
}

func TestDestPolicy_LinuxNetworkFilesystemRefused(t *testing.T) {
	app := dockApp(t)
	withDestStatfs(t, unix.NFS_SUPER_MAGIC)
	if d, err := app.Store.AuthorizeDestination(t.TempDir()); err == nil {
		d.Close()
		t.Fatal("NFS destination accepted")
	}
}
