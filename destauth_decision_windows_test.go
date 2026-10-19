//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	warning, err := checkDestDACL(sd, user, true, true)
	if err != nil {
		t.Fatalf("default drive-root ACL as a destination: %v", err)
	}
	if !strings.Contains(warning, "drive root") {
		t.Fatalf("default drive-root ACL: warning %q, want one about the drive root", warning)
	}
	// As an ancestor of the destination, the same grant is not a mutation right.
	if w, err := checkDestDACL(sd, user, false, true); err != nil || w != "" {
		t.Fatalf("default drive-root ACL as an ancestor: warning %q err %v, want neither", w, err)
	}
	// The same grant on a folder that is not a volume root is not the default.
	if _, err := checkDestDACL(sd, user, true, false); err == nil {
		t.Fatal("create-folders grant to all users on an ordinary destination folder: want a refusal")
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
		if _, err := checkDestDACL(sd, user, true, true); err == nil || !strings.Contains(err.Error(), "untrusted principal") {
			t.Errorf("%s at a drive root: got %v, want a refusal", name, err)
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

// On exFAT or FAT32 there are no ACLs to check, so a folder whose permissions
// would be refused on NTFS is accepted, and the result says what was not checked.
func TestDestPolicy_FATDestinationAcceptedWithNotes(t *testing.T) {
	for _, name := range []string{"exFAT", "FAT32"} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			dest := t.TempDir()
			user, err := destUserSID()
			if err != nil {
				t.Fatal(err)
			}
			setDestFixtureDACL(t, dest, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;WD)")
			if d, err := app.Store.AuthorizeDestination(dest); err == nil {
				d.Close()
				t.Fatal("precondition: an Everyone-writable NTFS folder must be refused")
			}
			withDestVolume(t, name, 0)
			d, err := app.Store.AuthorizeDestination(dest)
			if err != nil {
				t.Fatalf("%s destination refused: %v", name, err)
			}
			defer d.Close()
			if d.Filesystem() != name {
				t.Errorf("filesystem %q, want %q", d.Filesystem(), name)
			}
			notes := strings.Join(d.Notes(), "\n")
			for _, want := range []string{"folder permissions", "hard links", "stable file IDs"} {
				if !strings.Contains(notes, want) {
					t.Errorf("notes do not mention %q:\n%s", want, notes)
				}
			}
			if name == "FAT32" && d.MaxFileSize() != fatMaxFile {
				t.Errorf("FAT32 max file %d", d.MaxFileSize())
			}
		})
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
