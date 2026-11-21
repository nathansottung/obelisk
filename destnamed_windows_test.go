//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const otherAccountSID = "S-1-5-21-1111111111-2222222222-3333333333-1001"

func folderWithGrant(t *testing.T, ace string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "granted")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	addFolderACEs(t, dir, ace)
	return dir
}

// A tool's account with change rights on a folder (as on a user's temp folder) gets
// a notice the user must act on. It is not a dead-end refusal.
func TestDestination_NamedAccountWarnsAndUserDecides(t *testing.T) {
	app := dockApp(t)
	dir := folderWithGrant(t, "(A;OICI;0x1301bf;;;"+otherAccountSID+")")

	c := app.Store.CheckDestination(dir)
	if c.Status != destStatusChoose || c.Notice == nil {
		t.Fatalf("status %q, reason %q; want a notice to act on", c.Status, c.Reason)
	}
	for _, want := range []string{"Another account on this PC can change this folder", otherAccountSID,
		"add, change and delete files", "Windows does not set this up by default", "If you don't recognize it"} {
		if !strings.Contains(c.Notice.Text, want) {
			t.Errorf("the notice does not say %q:\n%s", want, c.Notice.Text)
		}
	}
	for _, o := range c.Notice.Choices {
		if o.ID == destChoiceMakePrivate {
			t.Error("Make private is offered, which would remove an account a tool may depend on")
		}
	}
	// The user must act: nothing is written until they choose.
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
	defer o.Close()
	if err := o.WriteFile("sub/f.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("writing after Use it anyway: %v", err)
	}
}

// Grants beyond the common case still refuse.
func TestDestination_StrongerGrantsStillRefuse(t *testing.T) {
	app := dockApp(t)
	for name, ace := range map[string]string{
		"an account with full control":        "(A;OICI;FA;;;" + otherAccountSID + ")",
		"an account that changes permissions": "(A;OICI;0x1701bf;;;" + otherAccountSID + ")",
		"Everyone":                            "(A;OICI;0x1301bf;;;WD)",
		"Guests":                              "(A;OICI;0x1301bf;;;BG)",
	} {
		dir := folderWithGrant(t, ace)
		if c := app.Store.CheckDestination(dir); c.Status != destStatusRefuse {
			t.Errorf("%s: status %q, want a refusal", name, c.Status)
		}
	}
}
