//go:build !windows && !guionly

package main

import (
	"os"
	"testing"
)

// makeUnlistable removes every permission from dir, and restores them when the
// test ends.
func makeUnlistable(t *testing.T, dir string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("SKIP (NOT TESTED): root ignores folder permissions")
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}
