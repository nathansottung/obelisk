//go:build linux && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Real permission configurations, not a simulated second user's execution.
func TestDestAuth_UntrustedLinuxNamespace(t *testing.T) {
	for _, which := range []string{"publication", "ancestor"} {
		t.Run(which, func(t *testing.T) {
			app := dockApp(t)
			base := t.TempDir()
			child := filepath.Join(base, "private")
			if err := os.Mkdir(child, 0o700); err != nil {
				t.Fatal(err)
			}
			writeTree(t, child, map[string]string{"good": "KEEP"})
			id := identOf(t, filepath.Join(child, "good"))
			target := child
			if which == "ancestor" {
				target = base
			}
			if err := os.Chmod(target, 0o777); err != nil {
				t.Fatal(err)
			}
			d, err := app.Store.AuthorizeDestination(child)
			if err == nil {
				d.Close()
				t.Fatal("untrusted namespace admitted")
			}
			if !strings.Contains(err.Error(), "untrusted entry replacement") {
				t.Fatalf("wrong refusal: %v", err)
			}
			if got, _ := os.ReadFile(filepath.Join(child, "good")); string(got) != "KEEP" || identOf(t, filepath.Join(child, "good")) != id {
				t.Fatal("prior data changed")
			}
			if err := os.Chmod(target, 0o700); err != nil {
				t.Fatal(err)
			}
			d, err = app.Store.AuthorizeDestination(child)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.WriteFileAtomic("good", []byte("SAFE"), 0o600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDestAuth_LinuxPrivateStage(t *testing.T) {
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	rel, f, err := d.stagingFor("private")
	if err != nil {
		t.Fatal(err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		t.Fatal(err)
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0o777 != 0o600 || st.Nlink != 1 {
		t.Fatalf("file authority: %+v", st)
	}
	p, err := d.stage(rel).private.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := unix.Fstat(int(p.Fd()), &st); err != nil {
		t.Fatal(err)
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0o777 != 0o700 {
		t.Fatalf("directory authority: %+v", st)
	}
	if err := d.Discard(rel); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFileAtomic("unsafe", []byte("NO"), 0o666); err == nil {
		t.Fatal("world-writable publication admitted")
	}
	if _, err := d.Stat("unsafe"); !os.IsNotExist(err) {
		t.Fatalf("unsafe publication exists: %v", err)
	}
}

func TestDestAuth_UnsupportedLinuxFilesystem(t *testing.T) {
	// procfs is already mounted for handle canonicalization; no mount or privilege
	// changes. Check the actual kernel filesystem, never create files in it.
	f, err := os.Open("/proc/self")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, _, err := destFilesystem(f); err == nil || !strings.Contains(err.Error(), "unsupported destination filesystem") {
		t.Fatalf("procfs admitted: %v", err)
	}
}

// Excluded actor, kept visible as evidence. A passing assertion here means the
// deliberate attack was CHARACTERIZED, not that the implementation defended it.
func TestDestAuth_SamePrincipalLateSubstitutionCharacterization(t *testing.T) {
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	old := beforeUnixDestRename
	defer func() { beforeUnixDestRename = old }()
	beforeUnixDestRename = func(s *destStage) error {
		// Deliberately use the operation's private root, AFTER its final checks.
		if err := s.private.Rename("payload"+mirrorTmpSuffix, "saved-owned"); err != nil {
			return err
		}
		f, err := s.private.OpenFile("payload"+mirrorTmpSuffix, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if _, err := f.WriteString("HOSTILE SUBSTITUTE"); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	if err := d.WriteFileAtomic("final", []byte("EXPECTED"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := d.HashRel("final")
	if err != nil || got != sha256Hex([]byte("HOSTILE SUBSTITUTE")) {
		t.Fatalf("characterization changed: %s %v", got, err)
	}
	t.Log("CHARACTERIZATION ONLY: deliberate late same-principal private-entry replacement succeeded; NOT a security pass or evidence of isolation")
}
