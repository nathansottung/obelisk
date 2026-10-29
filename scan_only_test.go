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

// Adopting a folder records a scan of it (with what it could not read) and its
// snapshot, but never replaces a snapshot of the whole drive with one folder.
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
	snap := app.Store.VolumeSnapshot(vol.ID)
	if snap == nil || snap.Root != dir || snap.TotalFiles != 1 {
		t.Fatalf("snapshot %+v, want the folder's", snap)
	}

	// A whole-drive snapshot is not replaced by a folder's.
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"x.txt": "x\n"})
	ds, err := app.StartDockSession(nil)
	if err != nil {
		t.Fatal(err)
	}
	vol.Serial = "SER-A"
	app.Store.UpdateVolume(vol)
	if _, err := app.IngestDrive(ds.ID, drive, "SER-A", "OLD-1", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	writeTree(t, other, map[string]string{"y.txt": "y\n"})
	if _, err := app.AdoptFolder(other, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	if snap := app.Store.VolumeSnapshot(vol.ID); snap.Root != "" {
		t.Errorf("a folder adopt replaced the drive's snapshot with %s", snap.Root)
	}
	if n := len(app.Store.ScanRecords(vol.ID)); n != 3 {
		t.Errorf("%d scan records, want 3 (each scan kept)", n)
	}
}
