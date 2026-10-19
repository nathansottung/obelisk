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
	// The destination check may refuse first (the entry would be inherited by new
	// folders); either way the folder is left as it was.
	if !strings.Contains(err.Error(), "can't safely") && !strings.Contains(err.Error(), "cannot evaluate") {
		t.Fatalf("refusal should explain why: %v", err)
	}
	if folderSDDLOf(t, folder) != before {
		t.Fatal("the folder's permissions changed")
	}
	// And the Make private builder itself refuses such an entry.
	user, _ := destUserSID()
	if _, _, _, _, err := privateDACL(before, user); err == nil || !strings.Contains(err.Error(), "can't safely") {
		t.Fatalf("Make private builder with an entry it cannot rewrite: %v", err)
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

// A folder inside a private destination that other accounts can still change (here
// one moved in from a shared folder, keeping its inherited grant) is not written
// into: the user never chose to allow that.
func TestDestGuard_SharedSubfolderRefused(t *testing.T) {
	app := dockApp(t)
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	setDestFixtureDACL(t, dest, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)")
	shared, _ := sharedFolder(t)
	sub := filepath.Join(dest, "Sub")
	if err := os.Rename(shared, sub); err != nil { // same volume: keeps its inherited grant
		t.Fatal(err)
	}
	if c := app.Store.CheckDestination(dest); c.Status != destStatusPass {
		t.Fatalf("precondition: the destination itself is private, got %q", c.Status)
	}
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.WriteFileAtomic("Sub/new.txt", []byte("x"), 0o644); err == nil || !strings.Contains(err.Error(), "Authenticated Users") {
		t.Fatalf("write into a shared folder inside a private destination: got %v, want a refusal naming the group", err)
	}
	if err := d.MkdirAll("Sub/Deeper"); err == nil {
		t.Fatal("folder created inside a shared folder of a private destination")
	}
	if got := entriesOf(t, sub); len(got) != 0 {
		t.Fatalf("written into the shared folder: %q", got)
	}
	// Writing elsewhere in the private destination still works.
	if err := d.WriteFileAtomic("Other/ok.txt", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A destination whose only grant to other accounts is one new folders inside would
// inherit (inherit-only) needs the same choice as one they can change directly:
// otherwise it would pass the check and then refuse every folder it creates.
func TestDestGuard_InheritOnlyGrantNeedsChoice(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	dest := t.TempDir()
	setDestFixtureDACL(t, dest, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;0x1301bf;;;AU)")
	c := app.Store.CheckDestination(dest)
	if c.Status != destStatusChoose {
		t.Fatalf("status %q, want a choice", c.Status)
	}
	if _, err := app.Store.ChooseUseAnyway(dest, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	// Once chosen, folders Obelisk creates inside (which inherit the grant) are fine.
	if err := d.WriteFileAtomic("New/file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("write into a folder created inside a chosen destination: %v", err)
	}
}

// A choice made for a folder that did not exist yet covers that folder only, not
// every other new folder under the same existing parent.
func TestDestChoice_MissingFolderChoiceIsNarrow(t *testing.T) {
	app := dockApp(t)
	_, parent := sharedFolder(t)
	a := filepath.Join(parent, "NewA")
	c := app.Store.CheckDestination(a)
	if c.Status != destStatusChoose {
		t.Fatalf("precondition: status %q", c.Status)
	}
	if _, err := app.Store.ChooseUseAnyway(a, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.CheckDestination(a).Status; got != destStatusChosen {
		t.Fatalf("the chosen folder: %q", got)
	}
	if got := app.Store.CheckDestination(filepath.Join(parent, "NewB")).Status; got != destStatusChoose {
		t.Fatalf("another new folder under the same parent: %q, want a choice", got)
	}
}

// A folder Obelisk created under a choice, then moved somewhere that adds a group
// the user never saw, is no longer covered by that choice.
func TestDestChoice_MovedCreatedFolderAskedAgain(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	folder, _ := sharedFolder(t)
	c := app.Store.CheckDestination(folder)
	if _, err := app.Store.ChooseUseAnyway(folder, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	d, err := app.Store.AuthorizeDestination(filepath.Join(folder, "New"))
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	// Q is a folder where Users can change things through an inherited grant.
	qParent := t.TempDir()
	setDestFixtureDACL(t, qParent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;0x1301bf;;;BU)")
	q := filepath.Join(qParent, "Q")
	if err := os.Mkdir(q, 0o755); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(q, "New")
	if err := os.Rename(filepath.Join(folder, "New"), moved); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.CheckDestination(moved); got.Status != destStatusChoose {
		t.Fatalf("moved created folder: %q with findings %+v, want a choice", got.Status, got.Findings)
	}
}

// A CREATOR OWNER grant on the parent becomes an entry for the folder's creator —
// this user — on a new folder, so it is not a reason to refuse.
func TestDestPredict_CreatorOwnerIsTheUser(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	parent := t.TempDir()
	setDestFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;GA;;;CO)")
	if c := app.Store.CheckDestination(filepath.Join(parent, "New")); c.Status != destStatusPass {
		t.Fatalf("new folder under a CREATOR OWNER grant: %q %s", c.Status, c.Reason)
	}
}

// An inheritable entry Obelisk cannot evaluate (here a conditional one) refuses
// before the new folder is created, not after, leaving nothing behind.
func TestDestPredict_UnknownInheritableEntryRefusedBeforeCreating(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	parent := t.TempDir()
	setDestFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)"+`(XA;OICIIO;FA;;;WD;(@User.title == "x"))`)
	newDir := filepath.Join(parent, "New")
	if d, err := app.Store.AuthorizeDestination(newDir); err == nil {
		d.Close()
		t.Fatal("a folder that would inherit an entry Obelisk cannot evaluate was authorized")
	}
	if _, err := os.Stat(newDir); err == nil {
		t.Fatal("the folder was created before the refusal")
	}
}

// When the shared group can also change the folder's permissions or take
// ownership (Full control), the notice says so: it is still a choice, but the
// user should know those accounts could undo Make private or lock them out.
func TestDestNotice_SaysWhenGroupCanChangePermissions(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	parent := t.TempDir()
	setDestFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;FA;;;AU)")
	folder := filepath.Join(parent, "Full")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	c := app.Store.CheckDestination(folder)
	if c.Status != destStatusChoose {
		t.Fatalf("status %q %s", c.Status, c.Reason)
	}
	if !strings.Contains(c.Notice.Text, "change who can use it") {
		t.Fatalf("notice should say the group can change permissions:\n%s", c.Notice.Text)
	}
	// A plain Modify grant does not get that line.
	plain, _ := sharedFolder(t)
	if strings.Contains(app.Store.CheckDestination(plain).Notice.Text, "change who can use it") {
		t.Fatal("a Modify grant was described as able to change permissions")
	}
}

// "Use it anyway" is on disk when the call returns, even while a job is batching
// catalog writes.
func TestDestChoice_SavedImmediatelyDuringBatch(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	app.Store.BeginBatch()
	defer app.Store.EndBatch()
	app.Store.AddCollection("forces a recent write")
	c := app.Store.CheckDestination(folder)
	if _, err := app.Store.ChooseUseAnyway(folder, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"destination_choices"`) {
		t.Fatal("the choice was not written to disk")
	}
}

// Names Windows treats as devices, with or without an extension or trailing
// spaces, are not written as destination files.
func TestDestRel_RejectsDeviceNames(t *testing.T) {
	for _, bad := range []string{"NUL.tar.gz", "nul.txt", "LPT1.x", "COM0", "NUL .txt", "con", "sub/AUX.json", "CONIN$", "conout$.log", "COM¹.txt"} {
		if _, err := destRel(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, good := range []string{"nullify.txt", "CONSOLE.txt", "com10.txt", "lpt.txt", "Aux Files/a.txt"} {
		if _, err := destRel(good); err != nil {
			t.Errorf("%q refused: %v", good, err)
		}
	}
}

// movedSharedFolder makes a folder that inherits the given rights for
// Authenticated Users and moves it to dst (same volume: it keeps the grant).
func movedSharedFolder(t *testing.T, dst, rights string) {
	t.Helper()
	user, _ := destUserSID()
	p := t.TempDir()
	setDestFixtureDACL(t, p, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;"+rights+";;;AU)")
	src := filepath.Join(p, "Moved")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
}

// A grant the user accepted on a folder ABOVE a private destination does not
// cover a shared folder later moved inside the destination.
func TestDestGuard_AncestorAcceptanceDoesNotCoverSubfolders(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	shared, _ := sharedFolder(t)
	dest := filepath.Join(shared, "Private")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	setDestFixtureDACL(t, dest, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)")
	c := app.Store.CheckDestination(dest)
	if c.Status != destStatusChoose {
		t.Fatalf("precondition: the shared folder above needs a choice, got %q", c.Status)
	}
	if _, err := app.Store.ChooseUseAnyway(dest, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	movedSharedFolder(t, filepath.Join(dest, "Sub"), "0x1301bf")
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.WriteFileAtomic("Sub/x.txt", []byte("x"), 0o644); err == nil {
		t.Fatal("a shared folder inside a private destination was written into because of a grant above it")
	}
	if err := d.WriteFileAtomic("Fine/x.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("a new private folder inside: %v", err)
	}
}

// Accepting a group that can modify files does not cover a folder inside where the
// same group can also change permissions (Full control).
func TestDestGuard_ModifyAcceptanceDoesNotCoverFullControl(t *testing.T) {
	app := dockApp(t)
	dest, _ := sharedFolder(t) // Authenticated Users: Modify, inherited
	c := app.Store.CheckDestination(dest)
	if _, err := app.Store.ChooseUseAnyway(dest, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	movedSharedFolder(t, filepath.Join(dest, "Full"), "FA")
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.WriteFileAtomic("Full/x.txt", []byte("x"), 0o644); err == nil {
		t.Fatal("a folder where the group has Full control was covered by accepting Modify")
	}
	if err := d.WriteFileAtomic("New/x.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("a folder Obelisk creates inside: %v", err)
	}
}
