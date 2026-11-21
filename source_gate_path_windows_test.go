//go:build windows

package main

import "testing"

// unattachedPathForTest is a location nothing answers at.
func unattachedPathForTest(t *testing.T) string {
	t.Helper()
	return unusedDriveLetter(t)
}
