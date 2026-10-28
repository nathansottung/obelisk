//go:build windows && !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// setDestFixtureDACL replaces a disposable fixture folder's DACL. It grants a
// distinct principal authority to check real ACL refusal; nothing impersonates or
// runs as another user.
func setDestFixtureDACL(t *testing.T, dir, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

// withDestVolume presents every volume as the named filesystem for one test.
func withDestVolume(t *testing.T, name string, flags uint32) {
	t.Helper()
	prev := destVolumeInformation
	destVolumeInformation = func(*os.File) (string, uint32, error) { return name, flags, nil }
	t.Cleanup(func() { destVolumeInformation = prev })
}

// The DACL Windows puts on a volume root, as `icacls C:\` shows it: Authenticated
// Users may create folders there, and nothing else.
const defaultRootDACL = "D:(A;OICI;FA;;;BA)(A;OICI;FA;;;SY)(A;OICI;0x1200a9;;;BU)(A;CI;LC;;;AU)(A;OICIIO;SDGXGWGR;;;AU)(A;OICIIO;GA;;;CO)"

func TestDestPolicy_DriveRootDefaultACLWarns(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("O:BA" + defaultRootDACL)
	if err != nil {
		t.Fatal(err)
	}
	found, err := checkDestDACL(sd, user, `D:\`, true, true)
	if err != nil {
		t.Fatalf("default drive-root ACL as a destination: %v", err)
	}
	if len(found) != 1 || found[0].Kind != destFindingDriveRoot || found[0].Group != "Authenticated Users" {
		t.Fatalf("default drive-root ACL: findings %+v, want one drive-root finding", found)
	}
	// As an ancestor of the destination, the same grant is not a change right.
	if f, err := checkDestDACL(sd, user, `D:\`, false, true); err != nil || len(f) != 0 {
		t.Fatalf("default drive-root ACL as an ancestor: findings %+v err %v, want neither", f, err)
	}
	// The same grant set on a folder that is not a volume root is not the default.
	if _, err := checkDestDACL(sd, user, `D:\x`, true, false); err == nil {
		t.Fatal("create-folders grant set explicitly on an ordinary destination folder: want a refusal")
	}
}

func TestDestPolicy_DriveRootNonDefaultACLRefuses(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	for name, extra := range map[string]string{
		"users can add files":    "(A;;0x2;;;AU)",
		"users have full access": "(A;OICI;FA;;;BU)",
		"everyone can delete":    "(A;;0x40;;;WD)",
		"users can write":        "(A;;GW;;;BU)",
	} {
		sd, err := windows.SecurityDescriptorFromString("O:BA" + defaultRootDACL + extra)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkDestDACL(sd, user, `D:\`, true, true); err == nil || !strings.Contains(err.Error(), "untrusted principal") {
			t.Errorf("%s at a drive root: got %v, want a refusal", name, err)
		}
	}
}

// Each kind of grant on an ordinary destination folder: inherited Authenticated
// Users or Users is a finding; Everyone, Guests, Anonymous Logon, another account,
// or any grant set explicitly on the folder refuses; read-only grants pass.
func TestDestPolicy_GrantsWarnOrRefuse(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	const modify = "0x1301bf"
	other := "S-1-5-21-1111111111-2222222222-3333333333-1001"
	cases := []struct {
		name   string
		aces   string
		warn   []string // groups expected as findings
		refuse string   // substring of the refusal, or "" to accept
	}{
		{"inherited Authenticated Users", "(A;OICIID;" + modify + ";;;AU)", []string{"Authenticated Users"}, ""},
		{"inherited Users", "(A;OICIID;" + modify + ";;;BU)", []string{"Users"}, ""},
		{"inherited Authenticated Users and Users", "(A;OICIID;" + modify + ";;;AU)(A;OICIID;" + modify + ";;;BU)", []string{"Authenticated Users", "Users"}, ""},
		{"read-only Authenticated Users", "(A;OICIID;0x1200a9;;;AU)", nil, ""},
		{"explicit Authenticated Users", "(A;OICI;" + modify + ";;;AU)", nil, "Authenticated Users"},
		{"explicit Users", "(A;OICI;" + modify + ";;;BU)", nil, "Users"},
		{"inherited Everyone", "(A;OICIID;" + modify + ";;;WD)", nil, "Everyone"},
		{"explicit Everyone", "(A;OICI;" + modify + ";;;WD)", nil, "Everyone"},
		{"inherited Guests", "(A;OICIID;" + modify + ";;;BG)", nil, "Guests"},
		{"inherited Anonymous", "(A;OICIID;" + modify + ";;;AN)", nil, "Anonymous Logon"},
		{"inherited other account", "(A;OICIID;" + modify + ";;;" + other + ")", nil, other},
		{"explicit other account", "(A;OICI;" + modify + ";;;" + other + ")", nil, other},
		{"Users plus Everyone", "(A;OICIID;" + modify + ";;;BU)(A;OICIID;" + modify + ";;;WD)", nil, "Everyone"},
	}
	for _, c := range cases {
		sd, err := windows.SecurityDescriptorFromString("O:" + user + "D:P(A;OICI;FA;;;" + user + ")(A;OICIID;FA;;;SY)" + c.aces)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		found, err := checkDestDACL(sd, user, `C:\Backups`, true, false)
		if c.refuse != "" {
			if err == nil || !strings.Contains(err.Error(), c.refuse) {
				t.Errorf("%s: got %v, want a refusal naming %q", c.name, err, c.refuse)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: refused: %v", c.name, err)
			continue
		}
		var groups []string
		for _, f := range found {
			if f.Kind != destFindingSharedGroup || !f.Inherited {
				t.Errorf("%s: unexpected finding %+v", c.name, f)
			}
			groups = append(groups, f.Group)
		}
		if strings.Join(groups, ",") != strings.Join(c.warn, ",") {
			t.Errorf("%s: findings %q, want %q", c.name, groups, c.warn)
		}
	}
}

func TestDestPolicy_FilesystemKinds(t *testing.T) {
	f, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cases := []struct {
		name    string
		flags   uint32
		ok      bool
		acls    bool
		maxFile int64
	}{
		{"NTFS", windows.FILE_PERSISTENT_ACLS, true, true, 0},
		{"NTFS", 0, false, false, 0},
		{"exFAT", 0, true, false, 0},
		{"FAT32", 0, true, false, fatMaxFile},
		{"FAT", 0, true, false, fatMaxFile},
		{"ReFS", windows.FILE_PERSISTENT_ACLS, false, false, 0},
		{"UDF", 0, false, false, 0},
	}
	for _, c := range cases {
		withDestVolume(t, c.name, c.flags)
		fs, acls, err := destFilesystem(f)
		if (err == nil) != c.ok {
			t.Errorf("%s flags %#x: err %v, want ok=%v", c.name, c.flags, err, c.ok)
			continue
		}
		if !c.ok {
			continue
		}
		if acls != c.acls || fs.maxFile != c.maxFile || fs.name != c.name {
			t.Errorf("%s: got name %q acls %v maxFile %d", c.name, fs.name, acls, fs.maxFile)
		}
		if !c.acls && len(fs.notChecked) != 3 {
			t.Errorf("%s: not-checked list %q, want permissions, hard links and file IDs", c.name, fs.notChecked)
		}
	}
}

// sharedFolder makes a folder that inherits "Authenticated Users can change" from
// its parent, the way Windows lays out a folder made directly on C:\. It returns
// the folder and its parent.
func sharedFolder(t *testing.T) (folder, parent string) {
	t.Helper()
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	parent = t.TempDir()
	setDestFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;0x1301bf;;;AU)")
	folder = filepath.Join(parent, "Backups")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	return folder, parent
}

func entriesOf(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func folderSDDLOf(t *testing.T, path string) string {
	t.Helper()
	h, _, err := openFolderForSecurity(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	s, err := folderSDDL(h)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fileSDDLOf(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd.String()
}

// A warned destination cannot be written to until the user has chosen. A missing
// one is created, empty, so its real permissions can be read; nothing else is.
func TestDestChoice_NoWriteBeforeChoice(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)

	c := app.Store.CheckDestination(folder)
	if c.Status != destStatusChoose || c.Notice == nil {
		t.Fatalf("check: status %q notice %v, want a choice", c.Status, c.Notice)
	}
	_, err := app.Store.AuthorizeDestination(folder)
	var ce *DestinationChoiceError
	if !errors.As(err, &ce) {
		t.Fatalf("authorize before a choice: got %v, want a DestinationChoiceError", err)
	}
	if got := entriesOf(t, folder); len(got) != 0 {
		t.Fatalf("a refused destination was written to: %q", got)
	}
	// A folder that does not exist yet is created empty, and nothing is written in it.
	sub := filepath.Join(folder, "new", "deeper")
	if _, err := app.Store.AuthorizeDestination(sub); !errors.As(err, &ce) {
		t.Fatalf("authorize a new folder inside before a choice: got %v, want a DestinationChoiceError", err)
	}
	if got := entriesOf(t, folder); len(got) != 1 || got[0] != "new" {
		t.Fatalf("before the choice, %s holds %q, want only the new folder", folder, got)
	}
	if got := entriesOf(t, sub); len(got) != 0 {
		t.Fatalf("a file was written into the new folder before the choice: %q", got)
	}
	if err := os.RemoveAll(filepath.Join(folder, "new")); err != nil {
		t.Fatal(err)
	}
	// And a mirror is refused before copying anything.
	app2, collID, _, vol := mirrorSetup(t, "Gate", map[string]string{"a.txt": "alpha"})
	if _, err := app2.MirrorToVolume(collID, nil, folder, vol.ID, 0, "", noProgMirror); !errors.As(err, &ce) {
		t.Fatalf("mirror before a choice: got %v, want a DestinationChoiceError", err)
	}
	if got := entriesOf(t, folder); len(got) != 0 {
		t.Fatalf("a mirror wrote before the choice: %q", got)
	}
}

// "Use it anyway" needs the exact word, and then the destination can be used.
func TestDestChoice_UseAnywayNeedsTypedWord(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	c := app.Store.CheckDestination(folder)
	for _, typed := range []string{"", "continue", "CONTINUE", "Continue please", "yes", "Delete"} {
		if _, err := app.Store.ChooseUseAnyway(folder, c.Fingerprint, typed); err == nil {
			t.Fatalf("typed %q was accepted", typed)
		}
	}
	if len(app.Store.c.DestinationChoices) != 0 {
		t.Fatal("a choice was recorded without the typed word")
	}
	if _, err := app.Store.ChooseUseAnyway(folder, "stale-fingerprint", destUseAnywayWord); err == nil {
		t.Fatal("a choice against a fingerprint the user was not shown was accepted")
	}
	got, err := app.Store.ChooseUseAnyway(folder, c.Fingerprint, destUseAnywayWord)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != destStatusChosen || got.ChosenAt == nil {
		t.Fatalf("after the choice: %+v", got)
	}
	rec := app.Store.c.DestinationChoices[0]
	if rec.Choice != destChoiceUseAnyway || rec.ChosenAt.IsZero() || rec.Volume == 0 || rec.Folder == 0 {
		t.Fatalf("choice record incomplete: %+v", rec)
	}
	d, err := app.Store.AuthorizeDestination(folder)
	if err != nil {
		t.Fatalf("authorize after the choice: %v", err)
	}
	if err := d.WriteFileAtomic("ok.txt", []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.Close() // one job per volume at a time: release the lease before the next
	// A new folder inside the chosen folder is a different folder: it is created,
	// its real permissions read, and the user asked about it.
	photos := filepath.Join(folder, "Photos")
	var ce *DestinationChoiceError
	if _, err := app.Store.AuthorizeDestination(photos); !errors.As(err, &ce) {
		t.Fatalf("authorize a new folder inside a chosen folder: got %v, want a DestinationChoiceError", err)
	}
	if _, err := app.Store.ChooseUseAnyway(photos, ce.Check.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	d2, err := app.Store.AuthorizeDestination(photos)
	if err != nil {
		t.Fatalf("authorize the new folder after its own choice: %v", err)
	}
	d2.Close()
	d3, err := app.Store.AuthorizeDestination(filepath.Join(folder, "Photos"))
	if err != nil {
		t.Fatalf("authorize the folder Obelisk created, on a later run: %v", err)
	}
	d3.Close()
}

// Any change to the permissions of the folder or a folder above it means asking
// again.
func TestDestChoice_ReaskWhenPermissionsChange(t *testing.T) {
	app := dockApp(t)
	folder, parent := sharedFolder(t)
	c := app.Store.CheckDestination(folder)
	if _, err := app.Store.ChooseUseAnyway(folder, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	if d, err := app.Store.AuthorizeDestination(folder); err != nil {
		t.Fatal(err)
	} else {
		d.Close()
	}
	user, _ := destUserSID()
	// Users also gain read access through the parent: a harmless change, but a change.
	setDestFixtureDACL(t, parent, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIO;0x1301bf;;;AU)(A;OICI;0x1200a9;;;BU)")
	_, err := app.Store.AuthorizeDestination(folder)
	var ce *DestinationChoiceError
	if !errors.As(err, &ce) {
		t.Fatalf("after a permission change: got %v, want the choice asked again", err)
	}
	if !strings.Contains(ce.Check.Notice.Text, "asking again") {
		t.Fatalf("the notice should say why it is asking again:\n%s", ce.Check.Notice.Text)
	}
}

// Make private shows the exact change, needs a second confirm, and changes only
// the folder's own permissions: not its files, their permissions or contents.
func TestDestChoice_MakePrivateChangesOnlyTheFolderACL(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	inside := filepath.Join(folder, "existing.txt")
	mustWrite(t, inside, "KEEP")
	when := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(inside, when, when); err != nil {
		t.Fatal(err)
	}
	fileBefore := fileSDDLOf(t, inside)
	folderBefore := folderSDDLOf(t, folder)
	idBefore := identOf(t, inside)

	c := app.Store.CheckDestination(folder)
	var offered []string
	for _, ch := range c.Notice.Choices {
		offered = append(offered, ch.ID)
	}
	if strings.Join(offered, ",") != "make_private,use_anyway,pick_another" {
		t.Fatalf("choices %q", offered)
	}

	plan, err := app.Store.PreviewMakePrivate(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Removed) != 1 || !strings.Contains(plan.Removed[0], "Authenticated Users") {
		t.Fatalf("removed %q, want the Authenticated Users entry", plan.Removed)
	}
	if !strings.Contains(plan.AfterSDDL, "D:P") || strings.Contains(plan.AfterSDDL, ";AU)") {
		t.Fatalf("after %q should be protected and without Authenticated Users", plan.AfterSDDL)
	}
	if folderSDDLOf(t, folder) != folderBefore {
		t.Fatal("the preview changed the folder's permissions")
	}
	if _, err := app.Store.ApplyMakePrivate(folder, plan.Token, false); err == nil {
		t.Fatal("applied without the second confirm")
	}
	plan, err = app.Store.PreviewMakePrivate(folder)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.Store.ApplyMakePrivate(folder, plan.Token, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != destStatusPass {
		t.Fatalf("after Make private: status %q, findings %+v", got.Status, got.Findings)
	}
	if _, err := app.Store.ApplyMakePrivate(folder, plan.Token, true); err == nil {
		t.Fatal("a used token was accepted again")
	}
	if after := folderSDDLOf(t, folder); after == folderBefore || strings.Contains(after, ";AU)") {
		t.Fatalf("folder permissions after: %s", after)
	}
	if fileSDDLOf(t, inside) != fileBefore {
		t.Fatalf("a file's permissions changed:\nbefore %s\nafter  %s", fileBefore, fileSDDLOf(t, inside))
	}
	mustBytes(t, inside, "KEEP", "make private")
	st, err := os.Stat(inside)
	if err != nil || !st.ModTime().Equal(when) || identOf(t, inside) != idBefore {
		t.Fatalf("the file inside changed: %v %v", st.ModTime(), err)
	}
	if got := entriesOf(t, folder); strings.Join(got, ",") != "existing.txt" {
		t.Fatalf("folder contents changed: %q", got)
	}
}

// Make private is only offered when it resolves the finding: not when the grant
// sits on a folder above the destination.
func TestDestChoice_MakePrivateNotOfferedForAncestorGrant(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	inner := filepath.Join(folder, "inner")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	c := app.Store.CheckDestination(inner)
	if c.Status != destStatusChoose {
		t.Fatalf("status %q", c.Status)
	}
	for _, ch := range c.Notice.Choices {
		if ch.ID == destChoiceMakePrivate {
			t.Fatal("Make private offered for a grant on a folder above the destination")
		}
	}
	if _, err := app.Store.PreviewMakePrivate(inner); err == nil {
		t.Fatal("Make private previewed for a grant on a folder above the destination")
	}
}

// exFAT and FAT32 destinations need the same choice, and their notice says which
// checks could not run.
func TestDestChoice_FATNeedsChoice(t *testing.T) {
	for _, name := range []string{"exFAT", "FAT32"} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			dest := t.TempDir()
			withDestVolume(t, name, 0)
			_, err := app.Store.AuthorizeDestination(dest)
			var ce *DestinationChoiceError
			if !errors.As(err, &ce) {
				t.Fatalf("%s before a choice: got %v, want a DestinationChoiceError", name, err)
			}
			text := ce.Check.Notice.Text
			for _, want := range []string{name, "Who can change the folder", "Whether a file is the one", "Hard links"} {
				if !strings.Contains(text, want) {
					t.Errorf("notice does not mention %q:\n%s", want, text)
				}
			}
			if (name == "FAT32") != strings.Contains(text, "4 GB") {
				t.Errorf("4 GB line wrong for %s:\n%s", name, text)
			}
			for _, ch := range ce.Check.Notice.Choices {
				if ch.ID == destChoiceMakePrivate {
					t.Errorf("Make private offered on %s", name)
				}
			}
			if _, err := app.Store.ChooseUseAnyway(dest, ce.Check.Fingerprint, destUseAnywayWord); err != nil {
				t.Fatal(err)
			}
			d, err := app.Store.AuthorizeDestination(dest)
			if err != nil {
				t.Fatalf("%s after a choice: %v", name, err)
			}
			defer d.Close()
			notes := strings.Join(d.Notes(), "\n")
			for _, want := range []string{"folder permissions", "hard links", "stable file IDs"} {
				if !strings.Contains(notes, want) {
					t.Errorf("notes do not mention %q:\n%s", want, notes)
				}
			}
		})
	}
}

// Notices use only ✓ and ✗ as symbols, every line starts with one, and no choice
// is marked as the default.
func TestDestChoice_NoticeWording(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	c := app.Store.CheckDestination(folder)
	n := c.Notice
	// The notice names the folder as resolved from its handle (long name, real case).
	if !strings.Contains(n.Text, "Another account on this PC could change or delete your backup copies") ||
		!strings.Contains(n.Text, "If you're the only person who signs in to this PC, this is usually fine") ||
		!strings.Contains(n.Text, c.Folder) || !strings.Contains(n.Text, "Authenticated Users") {
		t.Fatalf("notice is missing a required part:\n%s", n.Text)
	}
	for _, l := range n.Lines {
		if !strings.HasPrefix(l, "✗ ") && !strings.HasPrefix(l, "✓ ") {
			t.Errorf("line does not start with ✓ or ✗: %q", l)
		}
	}
	for _, r := range n.Text {
		if r > 0x7f && r != '✓' && r != '✗' && r != '’' {
			t.Errorf("notice uses symbol %q", r)
		}
	}
	if strings.Contains(strings.ToLower(n.Text), "recommended") || strings.Contains(n.Text, "(*)") {
		t.Errorf("a choice is marked as preselected:\n%s", n.Text)
	}
}

func TestDestPolicy_NetworkAndReFSRefused(t *testing.T) {
	app := dockApp(t)
	withDestVolume(t, "ReFS", windows.FILE_PERSISTENT_ACLS)
	if d, err := app.Store.AuthorizeDestination(t.TempDir()); err == nil {
		d.Close()
		t.Fatal("ReFS destination accepted")
	} else if !strings.Contains(err.Error(), "NTFS, exFAT or FAT32") {
		t.Fatalf("ReFS refusal should name the accepted filesystems: %v", err)
	}
}

// unusedDriveLetter returns a drive root that no volume or mapped share uses. The
// logical-drive mask is read rather than stat-ing each letter: a disconnected
// mapped drive makes a stat wait for the network.
func unusedDriveLetter(t *testing.T) string {
	t.Helper()
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		t.Fatal(err)
	}
	for c := 'X'; c >= 'G'; c-- {
		if mask&(1<<uint(c-'A')) == 0 {
			return string(c) + `:\`
		}
	}
	t.Skip("no unused drive letter")
	return ""
}

// A source on a drive that is not attached does not block a mirror elsewhere,
// whether or not its volume identity was recorded.
func TestDestAuth_OfflineDriveLetterSource(t *testing.T) {
	gone := filepath.Join(unusedDriveLetter(t), "Photos")
	for _, recorded := range []string{"", "win-vsn:0BADF00D"} {
		app := dockApp(t)
		coll := app.Store.AddCollection("Unplugged")
		f := app.Store.AddFolder(coll.ID, gone)
		f.VolumeID = recorded
		d, err := app.Store.AuthorizeDestination(filepath.Join(t.TempDir(), "mirror"))
		if err != nil {
			t.Fatalf("recorded=%q: an unplugged source blocked an unrelated destination: %v", recorded, err)
		}
		d.Close()
	}
}

// The unplugged source's drive is attached under another letter, and the
// destination is on it: refuse, because the destination may be inside the source.
func TestDestAuth_OfflineSourceVolumeAttachedElsewhere(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "mirror")
	here := pathVolumeID(filepath.Dir(dest))
	if here == "" {
		t.Skip("no volume identity")
	}
	app := dockApp(t)
	coll := app.Store.AddCollection("Moved")
	f := app.Store.AddFolder(coll.ID, filepath.Join(unusedDriveLetter(t), "Photos"))
	f.VolumeID = here
	d, err := app.Store.AuthorizeDestination(dest)
	if err == nil {
		d.Close()
		t.Fatal("destination on the volume of a source registered under another letter: want a refusal")
	}
	if !strings.Contains(err.Error(), "attached at a different location") {
		t.Fatalf("refusal should explain the drive moved: %v", err)
	}
}

// Make private matches the folder, not its spelling: a change previewed under the
// long name applies under the 8.3 short name of the same folder.
func TestDestChoice_MakePrivateAcceptsShortName(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	short := shortPathName(t, folder)
	if strings.EqualFold(short, folder) {
		t.Skip("8.3 short names are not generated on this volume")
	}
	plan, err := app.Store.PreviewMakePrivate(folder)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.Store.ApplyMakePrivate(short, plan.Token, true)
	if err != nil {
		t.Fatalf("apply under the short name: %v", err)
	}
	if got.Status != destStatusPass {
		t.Fatalf("status %q", got.Status)
	}
	// A different folder is still refused.
	other, _ := sharedFolder(t)
	plan, err = app.Store.PreviewMakePrivate(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Store.ApplyMakePrivate(folder, plan.Token, true); err == nil {
		t.Fatal("a change shown for one folder was applied to another")
	}
}

// A source registered before volume identities were recorded, whose drive is now
// attached under another letter: its old letter is empty, and the same folder
// exists at the same place on the destination's drive. That folder may be the
// source itself, so a destination inside it, around it or equal to it is refused.
func TestDestAuth_UnrecordedSourceUnderNewLetter(t *testing.T) {
	app := dockApp(t)
	real := t.TempDir() // the source, as it appears under its new letter
	mustWrite(t, filepath.Join(real, "photo.jpg"), "ORIGINAL")
	old := unusedDriveLetter(t)[:2] + real[len(filepath.VolumeName(real)):]
	coll := app.Store.AddCollection("Legacy")
	f := app.Store.AddFolder(coll.ID, old)
	f.VolumeID = "" // registered before identities were recorded

	for _, dest := range []string{real, filepath.Join(real, "mirror"), filepath.Dir(real)} {
		if d, err := app.Store.AuthorizeDestination(dest); err == nil {
			d.Close()
			t.Fatalf("destination %s may be inside the source under its new letter: want a refusal", dest)
		} else if !strings.Contains(err.Error(), "new letter") {
			t.Fatalf("refusal for %s should explain the drive may have a new letter: %v", dest, err)
		}
	}
	// Elsewhere on the same drive is fine.
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatalf("unrelated destination refused: %v", err)
	}
	d.Close()
	mustBytes(t, filepath.Join(real, "photo.jpg"), "ORIGINAL", "refused destinations")
}

// A legacy source registered as a whole drive (Q:\) whose drive is not attached
// cannot be located on another drive by its path; it must not make every
// destination on every other drive look like its root.
func TestDestAuth_UnrecordedRootSourceDoesNotBlockEverything(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Whole drive")
	f := app.Store.AddFolder(coll.ID, unusedDriveLetter(t))
	f.VolumeID = ""
	d, err := app.Store.AuthorizeDestination(filepath.Join(t.TempDir(), "mirror"))
	if err != nil {
		t.Fatalf("an unplugged whole-drive source refused an unrelated destination: %v", err)
	}
	d.Close()
}
