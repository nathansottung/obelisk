//go:build !guionly

package main

// Tests for the destination decisions that do not need a real mirror: offline
// sources, recorded volume identity, file-size limits and the result notes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// identOf reads an object's identity from an OPEN HANDLE, forcing it now. On
// Windows a FileInfo's identity is filled in lazily by reopening the name, so
// os.SameFile on a FileInfo taken before a mutation can vouch for a replacement.
func identOf(t *testing.T, p string) fileIdent {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatalf("open %s for identity: %v", p, err)
	}
	defer f.Close()
	id, err := handleIdentity(f)
	if err != nil {
		t.Fatalf("identity of %s: %v", p, err)
	}
	return id
}

// chooseAnyway records "Use it anyway" for a destination that needs a choice.
func chooseAnyway(t *testing.T, app *App, dest string) {
	t.Helper()
	c := app.Store.CheckDestination(dest)
	if c.Status != destStatusChoose {
		t.Fatalf("%s: status %q (%s), want a choice", dest, c.Status, c.Reason)
	}
	if _, err := app.Store.ChooseUseAnyway(dest, c.Fingerprint, destUseAnywayWord); err != nil {
		t.Fatal(err)
	}
}

func offlineSnap(refs ...sourceRef) *sourceSnapshot {
	for i := range refs {
		refs[i].offline = true
		if refs[i].registered == "" {
			refs[i].registered = refs[i].abs
		}
	}
	return &sourceSnapshot{srcs: refs}
}

func TestDestAuth_OfflineSourceWithRecordedVolume(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "Photos")
	snap := offlineSnap(sourceRef{abs: src, volumeID: "vol:A"})

	// The source's drive is not attached, so a destination on any other volume —
	// even one spelled exactly like the source's path — cannot be inside it.
	for _, dest := range []string{src, filepath.Join(src, "mirror"), base, filepath.Join(base, "elsewhere")} {
		if err := snap.decideOffline("vol:B", dest, dest, dest); err != nil {
			t.Errorf("destination %s on another volume refused: %v", dest, err)
		}
	}
	// The drive is attached somewhere else, and the destination is on it.
	err := snap.decideOffline("vol:A", filepath.Join(base, "x"), filepath.Join(base, "x"), "x")
	if err == nil || !strings.Contains(err.Error(), "attached at a different location") {
		t.Errorf("destination on the offline source's own volume: got %v, want a refusal", err)
	}
	// Destination volume unknown: fall back to the path rule.
	if err := snap.decideOffline("", src, src, src); err == nil {
		t.Error("unknown destination volume at the source's own path: want a refusal")
	}
	if err := snap.decideOffline("", filepath.Join(base, "other"), filepath.Join(base, "other"), "other"); err != nil {
		t.Errorf("unknown destination volume, unrelated path: %v", err)
	}
}

func TestDestAuth_OfflineSourceWithoutRecordedVolume(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "Photos")
	snap := offlineSnap(sourceRef{abs: src})
	for _, dest := range []string{src, filepath.Join(src, "inside"), base} {
		if err := snap.decideOffline("vol:B", dest, dest, dest); err == nil {
			t.Errorf("destination %s could be the offline source's path: want a refusal", dest)
		} else if !strings.Contains(err.Error(), "not attached") {
			t.Errorf("refusal for %s should say the source's drive is not attached: %v", dest, err)
		}
	}
	// The canonical spelling is checked too, not only the requested one.
	if err := snap.decideOffline("vol:B", filepath.Join(base, "alias"), src, "alias"); err == nil {
		t.Error("a destination that resolves to the offline source's path: want a refusal")
	}
	for _, dest := range []string{filepath.Join(base, "Photos2"), filepath.Join(base, "Other", "Photos")} {
		if err := snap.decideOffline("vol:B", dest, dest, dest); err != nil {
			t.Errorf("unrelated destination %s refused: %v", dest, err)
		}
	}
}

// An offline source is skipped by the online decision; it must not count as an
// unresolved location.
func TestDestAuth_OfflineSourceSkippedByOnlineDecision(t *testing.T) {
	snap := offlineSnap(sourceRef{abs: filepath.Join(t.TempDir(), "gone")})
	dest := t.TempDir()
	if err := snap.decide(dest, dest); err != nil {
		t.Fatalf("online decision with only an offline source: %v", err)
	}
}

// End to end: a registered source whose recorded volume is not where it was
// registered does not stop a mirror destination from being authorized.
func TestDestVolume_AuthorizeWithOfflineRecordedSource(t *testing.T) {
	app := dockApp(t)
	here := pathVolumeID(t.TempDir())
	if here == "" {
		t.Skip("this filesystem has no readable volume identity")
	}
	coll := app.Store.AddCollection("Offline")
	gone := filepath.Join(t.TempDir(), "unplugged", "Photos")
	f := app.Store.AddFolder(coll.ID, gone)
	if f.VolumeID != "" {
		t.Fatalf("an absent folder recorded volume %q", f.VolumeID)
	}
	f.VolumeID = "test:not-attached"

	dest := filepath.Join(t.TempDir(), "mirror")
	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatalf("an offline source on another volume blocked an unrelated destination: %v", err)
	}
	d.Close()

	// Even its own registered path is acceptable: that volume is not here.
	d, err = app.Store.AuthorizeDestination(gone)
	if err != nil {
		t.Fatalf("an offline source's own path on a different volume: %v", err)
	}
	d.Close()
}

func TestDestVolume_RecordsSourceVolume(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	want := pathVolumeID(src)
	if want == "" {
		t.Skip("this filesystem has no readable volume identity")
	}
	coll := app.Store.AddCollection("Rec")
	if f := app.Store.AddFolder(coll.ID, src); f.VolumeID != want {
		t.Fatalf("AddFolder recorded %q, want %q", f.VolumeID, want)
	}

	// A folder registered before identities were recorded is filled in the first
	// time a destination check sees it online.
	legacy := t.TempDir()
	lf := app.Store.AddFolder(coll.ID, legacy)
	lf.VolumeID = ""
	d, err := app.Store.AuthorizeDestination(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	if lf.VolumeID != pathVolumeID(legacy) {
		t.Fatalf("legacy folder volume = %q after a destination check, want %q", lf.VolumeID, pathVolumeID(legacy))
	}
}

func TestDestAuth_FileSizeLimit(t *testing.T) {
	fat := &DestRoot{fs: destFS{name: "FAT32", maxFile: 1<<32 - 1}}
	if err := fat.CheckFileSizes([]int64{1 << 20, 1<<32 - 1}); err != nil {
		t.Errorf("files that fit FAT32 refused: %v", err)
	}
	err := fat.CheckFileSizes([]int64{1 << 20, 5 << 30, 1 << 32})
	if err == nil || !strings.Contains(err.Error(), "2 file(s)") || !strings.Contains(err.Error(), "FAT32") {
		t.Errorf("files over 4 GB on FAT32: got %v", err)
	}
	exfat := &DestRoot{fs: destFS{name: "exFAT"}}
	if err := exfat.CheckFileSizes([]int64{100 << 30}); err != nil {
		t.Errorf("exFAT has no 4 GB limit: %v", err)
	}
}

func TestDestAuth_NotesListFindingsAndUncheckedItems(t *testing.T) {
	d := &DestRoot{fs: destFS{name: "exFAT", notChecked: []string{"folder permissions"},
		findings: []destFinding{{Kind: destFindingSharedGroup, Folder: "X", Group: "Authenticated Users"}}}}
	notes := d.Notes()
	if len(notes) != 2 || !strings.Contains(notes[0], "Authenticated Users can change X") || notes[1] != "not checked on this drive: folder permissions" {
		t.Fatalf("notes = %q", notes)
	}
	if n := (&DestRoot{fs: destFS{name: "NTFS"}}).Notes(); len(n) != 0 {
		t.Fatalf("a fully checked destination has notes: %q", n)
	}
}
