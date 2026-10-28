//go:build !windows && !guionly

package main

import (
	"os"
	"testing"
)

// mustDirLink makes link a folder link to target: a symbolic link.
func mustDirLink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
