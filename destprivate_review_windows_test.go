//go:build windows && !guionly

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

// Make private changes only the folder itself, so folders already inside keep the
// grant they inherited. The preview must say so and name them, rather than let the
// user think everything inside became private.
func TestMakePrivate_PreviewNamesStillSharedFolders(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	for _, d := range []string{"Old", `Old\Deeper`} {
		if err := os.Mkdir(filepath.Join(folder, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A junction inside, sorted first, must neither be followed nor end the walk.
	mustJunction(t, filepath.Join(folder, "Aaa-link"), t.TempDir())
	mux := http.NewServeMux()
	api(mux, app)
	code, plan := postJSON(t, mux, "/api/destinations/make-private/preview", map[string]any{"path": folder})
	if code != 200 {
		t.Fatalf("preview: %d %v", code, plan)
	}
	shared, _ := plan["still_shared"].([]any)
	var names []string
	for _, s := range shared {
		names = append(names, fmt.Sprint(s))
	}
	if strings.Join(names, ",") != `Old,Old\Deeper` {
		t.Fatalf("still_shared = %q, want the two folders inside that keep the grant", names)
	}
	if note := fmt.Sprint(plan["note"]); !strings.Contains(note, "can still change") {
		t.Fatalf("the preview should say other accounts can still change those folders: %q", note)
	}
}
