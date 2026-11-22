//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreFolderDACL gives the disposable test folder back to this account, so the
// test's cleanup can remove it.
func restoreFolderDACL(t *testing.T, dir, user string) {
	t.Helper()
	t.Cleanup(func() { setDestFixtureDACL(t, dir, "D:P(A;OICI;FA;;;"+user+")") })
}

func TestWriteAccess_RealDenyEntry(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "denied")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	restoreFolderDACL(t, dir, user)
	setDestFixtureDACL(t, dir, "D:P(D;OICI;0x6;;;"+user+")(A;OICI;FA;;;"+user+")")
	err = checkWriteAccess(dir, "backup output")
	assertWriteRefusal(t, err, "deny entry for")
}

func TestWriteAccess_RealReadButNotChange(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	restoreFolderDACL(t, dir, user)
	setDestFixtureDACL(t, dir, "D:P(A;OICI;0x1200a9;;;"+user+")")
	err = checkWriteAccess(dir, "backup output")
	assertWriteRefusal(t, err, "can read this folder but cannot change it")
}

// Users or Authenticated Users granted directly: the same notice, and the user acts.
func TestDestination_DirectUsersGrantWarnsAndUserDecides(t *testing.T) {
	for sddl, group := range map[string]string{"AU": "Authenticated Users", "BU": "Users"} {
		t.Run(group, func(t *testing.T) {
			app := dockApp(t)
			dir := folderWithGrant(t, "(A;OICI;0x1301bf;;;"+sddl+")")
			c := app.Store.CheckDestination(dir)
			if c.Status != destStatusChoose || c.Notice == nil {
				t.Fatalf("status %q, reason %q; want a notice to act on", c.Status, c.Reason)
			}
			for _, want := range []string{"✗ What we found:", "✗ Why it matters:", "✓ Who it's fine for:", group,
				"Windows does not set this up by default"} {
				if !strings.Contains(c.Notice.Text, want) {
					t.Errorf("the notice does not say %q:\n%s", want, c.Notice.Text)
				}
			}
			if d, err := app.Store.AuthorizeDestination(dir); err == nil {
				d.Close()
				t.Fatal("the folder was usable without the user's choice")
			}
			if _, err := app.Store.ChooseUseAnyway(dir, c.Fingerprint, destUseAnywayWord); err != nil {
				t.Fatal(err)
			}
			o, err := app.openOutFolder(dir)
			if err != nil {
				t.Fatalf("after Use it anyway: %v", err)
			}
			o.Close()
		})
	}
}

func TestDestination_DirectUsersWhoCanChangePermissionsStillRefuse(t *testing.T) {
	app := dockApp(t)
	for name, ace := range map[string]string{
		"Users with full control":            "(A;OICI;FA;;;BU)",
		"Authenticated Users, change rights": "(A;OICI;0x1701bf;;;AU)",
		"Everyone":                           "(A;OICI;0x1301bf;;;WD)",
		"Guests":                             "(A;OICI;0x1301bf;;;BG)",
	} {
		dir := folderWithGrant(t, ace)
		if c := app.Store.CheckDestination(dir); c.Status != destStatusRefuse {
			t.Errorf("%s: status %q, want a refusal", name, c.Status)
		}
	}
}

// The manual check's Windows line always has a partition to compare.
func TestManualPartition_WindowsLine(t *testing.T) {
	gpt := windowsPartition{Style: "GPT", Guid: "{6F1D2E3C-AAAA-4BBB-8CCC-0123456789AB}"}
	mbr := windowsPartition{Style: "MBR", Signature: 0x0a1b2c3d, Number: 1}
	for _, c := range []struct {
		p       windowsPartition
		line    string
		obelisk string
		ok      bool
	}{
		{gpt, "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab", "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab", true},
		{gpt, "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab", "gpt:00000000-aaaa-4bbb-8ccc-0123456789ab", false},
		{mbr, "mbr:0a1b2c3d-01", "mbr:0a1b2c3d-01", true},
		{mbr, "mbr:0a1b2c3d-01", "mbr:0a1b2c3d-02", false},
		{mbr, "mbr:0a1b2c3d-01", "mbr:ffffffff-01", false},
		{mbr, "mbr:0a1b2c3d-01", "", false},
		{windowsPartition{Style: "MBR"}, "", "mbr:0a1b2c3d-01", false},
		{windowsPartition{Style: "RAW"}, "", "", false},
	} {
		if got := c.p.id(); got != c.line {
			t.Errorf("%+v: line %q, want %q", c.p, got, c.line)
		}
		if err := c.p.compare(c.obelisk); (err == nil) != c.ok {
			t.Errorf("%+v against %q: %v, want ok=%v", c.p, c.obelisk, err, c.ok)
		}
	}
}
