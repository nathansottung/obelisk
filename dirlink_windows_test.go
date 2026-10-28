//go:build windows && !guionly

package main

import "testing"

// mustDirLink makes link a folder link to target: a junction on Windows.
func mustDirLink(t *testing.T, link, target string) {
	t.Helper()
	mustJunction(t, link, target)
}
