//go:build windows && !guionly

package main

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

var sddlACE = regexp.MustCompile(`\(([^()]*)\)`)

// addFolderACEs adds explicit entries to a folder's own permissions while it keeps
// inheriting from its parent.
func addFolderACEs(t *testing.T, dir, aces string) {
	t.Helper()
	cur, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	var explicit []string
	for _, m := range sddlACE.FindAllStringSubmatch(destDACLPart(cur.String()), -1) {
		if !strings.Contains(strings.Split(m[1], ";")[1], "ID") {
			explicit = append(explicit, "("+m[1]+")")
		}
	}
	sd, err := windows.SecurityDescriptorFromString("D:" + aces + strings.Join(explicit, ""))
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

// Make private must never drop a permission entry it did not show: an entry it
// cannot rewrite safely (here an inherit-only conditional entry) makes the preview
// refuse, and the folder is left as it was.
func TestMakePrivate_UnshownEntryRefused(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	addFolderACEs(t, folder, `(XA;OICIIO;FA;;;WD;(@User.title == "S:x"))`)
	before := folderSDDLOf(t, folder)
	plan, err := app.Store.PreviewMakePrivate(folder)
	if err == nil {
		t.Fatalf("preview of a folder with an entry it cannot rewrite succeeded:\nbefore %q\nremoved %q\nafter %s", plan.Before, plan.Removed, plan.AfterSDDL)
	}
	if !strings.Contains(err.Error(), "can't safely") {
		t.Fatalf("refusal should explain why: %v", err)
	}
	if folderSDDLOf(t, folder) != before {
		t.Fatal("the folder's permissions changed")
	}
}

// Every entry the preview lists as kept is in the result, the result lists deny
// entries before allow entries, and SYSTEM and Administrators stay.
func TestMakePrivate_KeepsShownEntriesInOrder(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	addFolderACEs(t, folder, "(D;;DC;;;WD)(A;OICI;FA;;;BA)")
	plan, err := app.Store.PreviewMakePrivate(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removed) != 1 || !strings.Contains(plan.Removed[0], "Authenticated Users") {
		t.Fatalf("removed %q", plan.Removed)
	}
	// Every entry not removed is still listed after (the user's full control may be
	// added on top).
	if kept := len(plan.Before) - len(plan.Removed); len(plan.After) < kept || len(plan.After) > kept+1 {
		t.Fatalf("before %q\nremoved %q\nafter %q", plan.Before, plan.Removed, plan.After)
	}
	dacl := destDACLPart(plan.AfterSDDL)
	firstAllow := strings.Index(dacl, "(A;")
	lastDeny := strings.LastIndex(dacl, "(D;")
	if lastDeny < 0 || firstAllow < 0 || lastDeny > firstAllow {
		t.Fatalf("deny entries must come before allow entries: %s", dacl)
	}
	for _, want := range [][2]string{{";SY)", ";S-1-5-18)"}, {";BA)", ";S-1-5-32-544)"}} {
		if !strings.Contains(dacl, want[0]) && !strings.Contains(dacl, want[1]) {
			t.Fatalf("result lost %s: %s", want[0], dacl)
		}
	}
	if _, err := app.Store.ApplyMakePrivate(folder, plan.Token, true); err != nil {
		t.Fatal(err)
	}
	if got := destDACLPart(folderSDDLOf(t, folder)); !strings.Contains(got, ";SY)") || !strings.Contains(got, ";BA)") || !strings.Contains(got, "(D;") {
		t.Fatalf("applied permissions lost an entry it showed as kept: %s", got)
	}
}

// When the user has no full-control entry yet, the one Make private adds must not
// be placed ahead of a deny entry, which would let it override that deny.
func TestMakePrivate_AddedEntryAfterDenies(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	after, _, _, _, err := privateDACL("O:BAD:AI(A;;FA;;;BA)(D;ID;DC;;;WD)(A;ID;0x1301bf;;;AU)", user)
	if err != nil {
		t.Fatal(err)
	}
	dacl := destDACLPart(after)
	if strings.Index(dacl, "(D;") > strings.Index(dacl, "(A;") {
		t.Fatalf("an allow entry comes before a deny entry: %s", dacl)
	}
	if !strings.Contains(dacl, ";"+user+")") {
		t.Fatalf("the user's full control was not added: %s", dacl)
	}
}
