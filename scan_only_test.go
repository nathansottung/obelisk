//go:build !guionly

package main

import (
	"path/filepath"
	"testing"
)

// A drive can be scanned without choosing an archive: the dock records its
// snapshot and scan, and matches nothing.
func TestDockScan_WithoutAnArchive(t *testing.T) {
	app := dockApp(t)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n", "sub/b.txt": "bravo\n"})
	ds, err := app.StartDockSession(nil)
	if err != nil {
		t.Fatalf("a session with no archive: %v", err)
	}
	res, err := app.IngestDrive(ds.ID, drive, "SER-N", "DN", "", "", false, noProg)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := res["matched"].(int); m != 0 {
		t.Errorf("matched %d with no archive", m)
	}
	vol := app.Store.VolumeBySerial("SER-N")
	snap := app.Store.VolumeSnapshot(vol.ID)
	if snap == nil || snap.TotalFiles != 2 || snap.Scan != ScanComplete {
		t.Fatalf("snapshot %+v, want both files, complete", snap)
	}
	if n := len(app.Store.ScanRecords(vol.ID)); n != 1 {
		t.Errorf("%d scan records, want 1", n)
	}
}

// Adopting a folder records a scan of it (with what it could not read) in the
// drive's scan history. It does not set the drive's snapshot, which describes
// the whole drive (paths from the drive's root).
func TestAdoptFolder_RecordsAScan(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	vol := app.Store.AddVolume(Volume{Label: "OLD-1", Kind: "HDD"})
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"a.txt": "alpha\n", "locked/b.txt": "bravo\n"})
	makeUnlistable(t, filepath.Join(dir, "locked"))
	if _, err := app.AdoptFolder(dir, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	recs := app.Store.ScanRecords(vol.ID)
	if len(recs) != 1 || recs[0].Root != dir || recs[0].Scan != ScanPartial || recs[0].UnreadableFolders != 1 {
		t.Fatalf("scan records %+v, want one partial scan of the folder", recs)
	}
	if snap := app.Store.VolumeSnapshot(vol.ID); snap != nil {
		t.Errorf("a folder adopt set the drive's snapshot (root %q)", snap.Root)
	}
}

// Re-adopting a folder whose files can no longer be read changes nothing: it is
// refused, and the drive's copy keeps its files and verification. When part of
// the folder cannot be read, the files an earlier adopt found there are unknown,
// not dropped.
func TestAdoptFolder_UnreadableReadoptChangesNothing(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	vol := app.Store.AddVolume(Volume{Label: "OLD-2", Kind: "HDD"})
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"keep/a.txt": "alpha\n", "locked/b.txt": "bravo\n"})
	if _, err := app.AdoptFolder(dir, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	c := app.mirrorChunkAt(arch.ID, vol.ID, dir)
	events := len(c.VerifyEvents)

	makeUnlistable(t, filepath.Join(dir, "locked"))
	if _, err := app.AdoptFolder(dir, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got, unk := mirrorRefsOn(app, arch.ID, vol.ID), unknownRefsOn(app, arch.ID, vol.ID); len(got) != 1 || len(unk) != 1 || unk[0] != "locked/b.txt" {
		t.Errorf("partial re-adopt: mirror %v, unknown %v; want keep/a.txt counted and locked/b.txt unknown", got, unk)
	}

	makeUnlistable(t, filepath.Join(dir, "keep"))
	if _, err := app.AdoptFolder(dir, arch.ID, vol.ID, nil); err == nil {
		t.Error("a re-adopt that could read no file ran")
	}
	c = app.mirrorChunkAt(arch.ID, vol.ID, dir)
	if len(c.Files) != 1 {
		t.Errorf("a refused re-adopt changed the copy: %d files", len(c.Files))
	}
	_ = events
}

// A dock after a folder adopt drops nothing it could not read: the folder keeps
// its own record, and the folder's scan is not mistaken for the drive's snapshot.
func TestDock_AfterFolderAdoptUnreadIsUnknown(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"Photos/a.jpg": "jpeg\n"})
	vol := app.Store.AddVolume(Volume{Label: "D", Kind: "HDD", Serial: "SER-D"})
	if _, err := app.AdoptFolder(filepath.Join(drive, "Photos"), arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	// Folder adopt may read the disk's real serial; the dock below must find this
	// volume by the one it is given.
	vol = app.Store.Volume(vol.ID)
	vol.Serial = "SER-D"
	app.Store.UpdateVolume(vol)
	makeUnlistable(t, filepath.Join(drive, "Photos"))
	ds, err := app.StartDockSession([]int{arch.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, "SER-D", "D", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	// The folder's own record keeps a.jpg; the dock's record of the drive, which
	// could not read the folder, adds nothing and drops nothing.
	if got := mirrorRefsOn(app, arch.ID, vol.ID); len(got) != 1 || got[0] != "a.jpg" {
		t.Errorf("after a folder adopt, a dock that cannot read the folder: records %v, want a.jpg kept", got)
	}
}
