//go:build linux && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A data folder on NFS or SMB is refused before anything is created there.
func TestCatalogDB_NetworkFilesystemRefusedOnLinux(t *testing.T) {
	for _, magic := range []int64{0x6969, 0xFF534D42, 0xFE534D42} {
		prev := catalogDBStatfs
		catalogDBStatfs = func(string) (int64, error) { return magic, nil }
		dir := t.TempDir()
		_, err := openCatalogDB(filepath.Join(dir, "catalog.db"), true)
		catalogDBStatfs = prev
		if err == nil || !strings.Contains(err.Error(), "network") {
			t.Errorf("magic %#x: %v", magic, err)
		}
		if names, _ := os.ReadDir(dir); len(names) != 0 {
			t.Errorf("magic %#x: a refused open wrote %v", magic, names)
		}
	}
}
