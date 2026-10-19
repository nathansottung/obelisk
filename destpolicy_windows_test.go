//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDestAuth_UntrustedWindowsNamespace(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
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
			private := "D:P(A;OICI;FA;;;" + user + ")"
			setDestFixtureDACL(t, child, private)
			target := child
			if which == "ancestor" {
				target = base
			}
			// Everyone can delete children, even though the selected child's DACL
			// itself remains private in the ancestor case.
			setDestFixtureDACL(t, target, private+"(A;;0x40;;;WD)")
			d, err := app.Store.AuthorizeDestination(child)
			if err == nil {
				d.Close()
				t.Fatal("untrusted mutation grant admitted")
			}
			if !strings.Contains(err.Error(), "untrusted principal") {
				t.Fatalf("wrong refusal: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(child, "good")); err != nil || string(got) != "KEEP" || identOf(t, filepath.Join(child, "good")) != id {
				t.Fatalf("prior data read: %q %v", got, err)
			}
			setDestFixtureDACL(t, target, private) // only our disposable fixture
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

func TestDestAuth_WindowsPrivateStage(t *testing.T) {
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
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil || owner.String() != user {
		t.Fatalf("owner: %v %v", owner, err)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil || acl.AceCount != 1 {
		t.Fatalf("private DACL: %v", err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(acl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() != user {
		t.Fatal("stage grants another principal access")
	}
	if _, err := checkDestDACL(sd, user, true, false); err != nil {
		t.Fatal(err)
	}
	if err := d.Discard(rel); err != nil {
		t.Fatal(err)
	}
}

func TestDestAuth_UnsupportedWindowsACL(t *testing.T) {
	user, err := destUserSID()
	if err != nil {
		t.Fatal(err)
	}
	for _, sddl := range []string{"O:" + user + "D:NO_ACCESS_CONTROL", "O:WDD:P(A;;FA;;;" + user + ")"} {
		sd, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkDestDACL(sd, user, true, false); err == nil {
			t.Fatalf("unsupported security descriptor admitted: %s", sddl)
		}
	}
}
