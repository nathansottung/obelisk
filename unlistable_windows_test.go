//go:build windows && !guionly

package main

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// makeUnlistable denies the current user listing dir (an explicit deny entry),
// and removes the entry again when the test ends.
func makeUnlistable(t *testing.T, dir string) {
	t.Helper()
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := "*" + u.User.Sid.String()
	if out, err := exec.Command("icacls", dir, "/deny", sid+":(RD)").CombinedOutput(); err != nil {
		t.Skipf("SKIP (NOT TESTED): cannot deny listing %s: %v %s", dir, err, out)
	}
	t.Cleanup(func() { _ = exec.Command("icacls", dir, "/remove:d", sid).Run() })
}
