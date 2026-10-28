//go:build windows && !guionly

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// grantInheritOnly adds an inherit-only grant for sid on dir, keeping its other
// entries (inherited ones included), as another account with Full control could.
func grantInheritOnly(t *testing.T, dir, sid, rights string) {
	t.Helper()
	if out, err := exec.Command("icacls", dir, "/grant", "*"+sid+":(OI)(CI)(IO)("+rights+")").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v %s", err, out)
	}
}

// useAnyway checks dest, which must need a choice, and records "Use it anyway".
func useAnyway(t *testing.T, app *App, dest string) *DestinationCheck {
	t.Helper()
	c := checkCreatingForTest(t, app, dest)
	if c.Status != destStatusChoose {
		t.Fatalf("%s: status %q (%s), want a choice", dest, c.Status, c.Reason)
	}
	if _, err := app.Store.ChooseUseAnyway(dest, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
	return c
}

// A choice is for one folder, by identity. The same folder deleted and made again
// at the same path is asked about again, whether its new permissions are the same
// or looser, and nothing is written into it first.
func TestDestChoice_DeletedAndRecreatedAsksAgain(t *testing.T) {
	for _, looser := range []bool{false, true} {
		name := "same permissions"
		if looser {
			name = "looser permissions"
		}
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			folder, _ := sharedFolder(t)
			useAnyway(t, app, folder)
			d, err := app.Store.AuthorizeDestination(folder)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.WriteFileAtomic("first.txt", []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			d.Close()

			if err := os.RemoveAll(folder); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(folder, 0o755); err != nil {
				t.Fatal(err)
			}
			if looser {
				// Folders made inside now give Authenticated Users Full control,
				// including changing permissions: more than the user accepted.
				grantInheritOnly(t, folder, sidAuthenticatedUsers, "F")
			}
			c := app.Store.CheckDestination(folder)
			if c.Status != destStatusChoose {
				t.Fatalf("recreated folder: %q, want a choice", c.Status)
			}
			if !strings.Contains(c.Notice.Text, "You chose to use this folder on") {
				t.Fatalf("the notice does not say why it asks again:\n%s", c.Notice.Text)
			}
			var ce *DestinationChoiceError
			if _, err := app.Store.AuthorizeDestination(folder); !errors.As(err, &ce) {
				t.Fatalf("authorize the recreated folder: got %v, want a DestinationChoiceError", err)
			}
			if got := entriesOf(t, folder); len(got) != 0 {
				t.Fatalf("written into the recreated folder before a choice: %q", got)
			}
		})
	}
}

// A destination moved away and replaced at its path during a job stops the job:
// the next write finds the folder at the authorized path is no longer the one
// authorized, and nothing more is written in either folder.
//
// Windows normally refuses to move a folder the job holds open, which is itself a
// protection; then the writes go on into the authorized folder, and the identity
// check is exercised directly by pointing the held destination's recorded path at
// another folder, exactly what a successful swap would leave. Either way the
// detection is tested on every run. (destroot_recheck_linux_test.go swaps for real.)
func TestDestRoot_WriteStopsWhenFolderReplaced(t *testing.T) {
	app := dockApp(t)
	dest := filepath.Join(privateDestForTest(t), "A", "B")
	if c := checkCreatingForTest(t, app, dest); c.Status != destStatusPass {
		t.Fatalf("new private destination: %q (%s), want pass", c.Status, c.Reason)
	}
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.WriteFileAtomic("first.txt", []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ch *DestinationChangedError
	if err := os.Rename(dest, dest+"-old"); err != nil {
		// Refused: the destination is still the authorized folder and writes go on.
		if err := d.WriteFileAtomic("second.txt", []byte("2"), 0o644); err != nil {
			t.Fatalf("write after a refused move: %v", err)
		}
		// What a swap would leave: the authorized path names a different folder.
		other := filepath.Join(filepath.Dir(dest), "Other")
		if err := os.Mkdir(other, 0o755); err != nil {
			t.Fatal(err)
		}
		d.canonical = other
		if err := d.WriteFileAtomic("third.txt", []byte("3"), 0o644); !errors.As(err, &ch) {
			t.Fatalf("write when the authorized path names another folder: got %v, want a DestinationChangedError", err)
		}
		if got := entriesOf(t, other); len(got) != 0 {
			t.Fatalf("the other folder holds %q, want nothing", got)
		}
		if got := entriesOf(t, dest); len(got) != 2 {
			t.Fatalf("the destination holds %q, want the two files written before the change", got)
		}
		return
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	loosenDestForTest(t, dest)
	if err := d.WriteFileAtomic("second.txt", []byte("2"), 0o644); !errors.As(err, &ch) {
		t.Fatalf("write after the folder was replaced: got %v, want a DestinationChangedError", err)
	}
	if err := d.MkdirAll("sub"); !errors.As(err, &ch) {
		t.Fatalf("mkdir after the folder was replaced: got %v, want a DestinationChangedError", err)
	}
	if got := entriesOf(t, dest); len(got) != 0 {
		t.Fatalf("the replacement holds %q, want nothing", got)
	}
	if got := entriesOf(t, dest+"-old"); len(got) != 1 || got[0] != "first.txt" {
		t.Fatalf("the moved destination holds %q, want only the first file", got)
	}
}

// Every write re-reads the destination's real permissions: loosening them during
// a job stops the job, for a chosen folder and for a private one.
func TestDestRoot_WriteStopsWhenPermissionsLoosened(t *testing.T) {
	user, _ := destUserSID()
	cases := map[string]func(t *testing.T, app *App) string{
		"chosen folder": func(t *testing.T, app *App) string {
			folder, _ := sharedFolder(t)
			useAnyway(t, app, folder)
			return folder
		},
		"private folder": func(t *testing.T, app *App) string {
			p := t.TempDir()
			setDestFixtureDACL(t, p, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)")
			return p
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			folder := setup(t, app)
			d, err := app.Store.AuthorizeDestination(folder)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if err := d.WriteFileAtomic("first.txt", []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			grantInheritOnly(t, folder, sidAuthenticatedUsers, "F")
			var ch *DestinationChangedError
			if err := d.WriteFileAtomic("second.txt", []byte("2"), 0o644); !errors.As(err, &ch) {
				t.Fatalf("write after the permissions changed: got %v, want a DestinationChangedError", err)
			}
			if got := entriesOf(t, folder); len(got) != 1 {
				t.Fatalf("%s holds %q, want only the first file", folder, got)
			}
		})
	}
}

// A grant only the first new folder gets (no-propagate) is accepted for that folder
// only. If another account with Full control of it moves the destination away and
// adds a grant to it, the next run makes a new destination with that grant, and
// is asked about it rather than covered by the old choice.
func TestDestChoice_FirstFolderGrantDoesNotCoverRecreatedDestination(t *testing.T) {
	app := dockApp(t)
	user, _ := destUserSID()
	p := t.TempDir()
	setDestFixtureDACL(t, p, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)(A;OICIIONP;FA;;;AU)")
	a := filepath.Join(p, "A")
	dest := filepath.Join(a, "B")
	useAnyway(t, app, dest)
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	if err := os.Rename(dest, filepath.Join(a, "B-old")); err != nil {
		t.Fatal(err)
	}
	grantInheritOnly(t, a, sidAuthenticatedUsers, "F")
	c := checkCreatingForTest(t, app, dest)
	if c.Status != destStatusChoose {
		t.Fatalf("the next run: %q with findings %+v, want a choice", c.Status, c.Findings)
	}
	var ce *DestinationChoiceError
	if _, err := app.Store.AuthorizeDestination(dest); !errors.As(err, &ce) {
		t.Fatalf("authorize: got %v, want a DestinationChoiceError", err)
	}
	if got := entriesOf(t, dest); len(got) != 0 {
		t.Fatalf("written into the new destination before a choice: %q", got)
	}
}

// privateDestForTest is an existing destination only this user and the system can
// change, so it needs no choice.
func privateDestForTest(t *testing.T) string {
	t.Helper()
	user, _ := destUserSID()
	p := t.TempDir()
	setDestFixtureDACL(t, p, "D:P(A;OICI;FA;;;"+user+")(A;OICI;FA;;;SY)")
	return p
}

// loosenDestForTest lets Authenticated Users take Full control of what is made in dir.
func loosenDestForTest(t *testing.T, dir string) {
	t.Helper()
	grantInheritOnly(t, dir, sidAuthenticatedUsers, "F")
}
