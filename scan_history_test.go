//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Each drive scan is its own record (ID, time, drive, complete or not) whose
// file list is stored compressed outside the catalog, and only the newest
// scan_history_retained scans of a drive are kept.
func TestScanHistory_RecordsEachScanWithRetention(t *testing.T) {
	app := dockApp(t)
	if _, err := app.SaveConfig(map[string]any{"scan_history_retained": 2}); err != nil {
		t.Fatal(err)
	}
	coll := app.Store.AddCollection("Photos")
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	ds, err := app.StartDockSession([]int{coll.ID})
	if err != nil {
		t.Fatal(err)
	}
	for i, body := range []string{"one", "two", "three"} {
		writeTree(t, drive, map[string]string{"n.txt": body})
		if i == 2 {
			makeUnlistable(t, mkdirFor(t, filepath.Join(drive, "locked")))
		}
		if _, err := app.IngestDrive(ds.ID, drive, "SER-H", "DH", "", "", false, noProg); err != nil {
			t.Fatal(err)
		}
	}
	vol := app.Store.VolumeBySerial("SER-H")
	recs := app.Store.ScanRecords(vol.ID)
	if len(recs) != 2 {
		t.Fatalf("%d scan records kept, want 2 (the cap)", len(recs))
	}
	newest, older := recs[0], recs[1]
	if newest.ID <= older.ID || newest.At.Before(older.At) || newest.VolumeID != vol.ID {
		t.Errorf("records not newest first for the drive: %+v %+v", newest, older)
	}
	if newest.Scan != ScanPartial || older.Scan != ScanComplete || newest.UnreadableFolders != 1 {
		t.Errorf("completeness: newest %q (%d unread), older %q", newest.Scan, newest.UnreadableFolders, older.Scan)
	}
	// The file lists live in the scans folder, compressed; the pruned one is gone.
	entries, err := os.ReadDir(filepath.Join(app.DataDir, "scans"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("scans folder: %v %v, want two file lists", entries, err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json.gz") {
			t.Errorf("file list %s is not compressed JSON", e.Name())
		}
	}
	list, err := app.LoadScanFiles(older)
	if err != nil {
		t.Fatal(err)
	}
	var n string
	for _, f := range list.Files {
		if f.RelPath == "n.txt" {
			n = f.Hash
		}
	}
	if n != sha256Hex([]byte("two")) {
		t.Errorf("older scan's n.txt hash %q, want the second scan's content", n)
	}
	// The catalog holds the records, not the file lists.
	b, err := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"volume_scans"`) || strings.Count(string(b), `"n.txt"`) > 1 {
		t.Errorf("catalog should hold scan records and only the latest snapshot's files")
	}
}

// A scan whose file list or record cannot be saved fails the ingest, and leaves
// the drive's scan records, snapshot and earlier file lists as they were. The
// record is written to disk, not just kept for a later save, before an older
// file list is removed.
func TestScanHistory_FailedSaveFailsTheIngest(t *testing.T) {
	app := dockApp(t)
	if _, err := app.SaveConfig(map[string]any{"scan_history_retained": 1}); err != nil {
		t.Fatal(err)
	}
	coll := app.Store.AddCollection("Photos")
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	ds, err := app.StartDockSession([]int{coll.ID})
	if err != nil {
		t.Fatal(err)
	}
	// The scans folder cannot be made: a file sits at its name.
	mustWrite(t, filepath.Join(app.DataDir, "scans"), "not a folder")
	if _, err := app.IngestDrive(ds.ID, drive, "SER-F", "DF", "", "", false, noProg); err == nil {
		t.Error("an ingest whose file list could not be saved succeeded")
	}
	vol := app.Store.VolumeBySerial("SER-F")
	if app.Store.VolumeSnapshot(vol.ID) != nil || len(app.Store.ScanRecords(vol.ID)) != 0 {
		t.Error("a failed ingest stored a snapshot or a scan record")
	}
	if err := os.Remove(filepath.Join(app.DataDir, "scans")); err != nil {
		t.Fatal(err)
	}

	// One good scan, then the catalog cannot be written.
	if _, err := app.IngestDrive(ds.ID, drive, "SER-F", "DF", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	first := app.Store.ScanRecords(vol.ID)
	snapAt := app.Store.VolumeSnapshot(vol.ID).CapturedAt
	app.Store.failSave = func() error { return errors.New("disk full") }
	_, err = app.IngestDrive(ds.ID, drive, "SER-F", "DF", "", "", false, noProg)
	app.Store.failSave = nil
	if err == nil || !strings.Contains(err.Error(), "saving the scan record") {
		t.Errorf("an ingest whose scan record could not be saved: %v, want that failure named", err)
	}
	recs := app.Store.ScanRecords(vol.ID)
	if len(recs) != 1 || recs[0].ID != first[0].ID {
		t.Errorf("scan records after a failed save: %+v, want only the first scan", recs)
	}
	if _, err := app.LoadScanFiles(recs[0]); err != nil {
		t.Errorf("the kept scan's file list was removed: %v", err)
	}
	if !app.Store.VolumeSnapshot(vol.ID).CapturedAt.Equal(snapAt) {
		t.Error("a failed ingest replaced the snapshot")
	}
}

func mkdirFor(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// File lists no record points at (left by a crash, or by an older catalog) are
// removed once they are an hour old; a fresh one is left for the scan writing it.
func TestScanHistory_SweepsOrphanedFileLists(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Photos")
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	dir := mkdirFor(t, filepath.Join(app.DataDir, "scans"))
	old := filepath.Join(dir, "scan-v9-20200101T000000Z-aaaaaaaaaaaa.json.gz")
	fresh := filepath.Join(dir, "scan-v9-20200101T000000Z-bbbbbbbbbbbb.json.gz")
	other := filepath.Join(dir, "notes.txt")
	for _, p := range []string{old, fresh, other} {
		mustWrite(t, p, "x")
	}
	long := time.Now().Add(-2 * time.Hour)
	for _, p := range []string{old, other} {
		if err := os.Chtimes(p, long, long); err != nil {
			t.Fatal(err)
		}
	}
	ds, err := app.StartDockSession([]int{coll.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, "SER-S", "DS", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("an hour-old orphaned file list was kept")
	}
	for _, p := range []string{fresh, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed: %v", filepath.Base(p), err)
		}
	}
}
