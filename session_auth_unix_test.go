//go:build unix && !guionly

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// runAsNobody runs a command as an unprivileged second account.
func runAsNobody(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	return cmd.Run()
}

func TestSigninParent_OpenFolderOrLinkIsRefused(t *testing.T) {
	for _, mode := range []string{"open", "link"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("XDG_RUNTIME_DIR", base)
			parent := filepath.Join(base, "Obelisk")
			switch mode {
			case "open":
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(parent, 0o755); err != nil {
					t.Fatal(err)
				}
			case "link":
				target := filepath.Join(base, "elsewhere")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, parent); err != nil {
					t.Skipf("cannot make a link here: %v", err)
				}
			}
			want := map[string]string{"open": "chmod 700", "link": "real folder"}[mode]
			_, err := signinDir()
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("signinDir: %v, want it to say %q", err, want)
			}
			if _, err := os.Lstat(filepath.Join(parent, "signin")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("the signin folder was created: %v", err)
			}
		})
	}
}

func TestSigninParent_PrivateFolderIsAccepted(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if _, err := signinDir(); err != nil {
		t.Fatal(err)
	}
}
