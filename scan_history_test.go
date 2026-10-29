//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// A scan whose record cannot be saved fails the ingest, and changes nothing.
func TestScanHistory_FailedSaveFailsTheIngest(t *testing.T) {
	app := dockApp(t)
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
	if v := app.Store.VolumeBySerial("SER-F"); v != nil && app.Store.VolumeSnapshot(v.ID) != nil {
		t.Error("a failed ingest stored a snapshot")
	}
	if err := os.Remove(filepath.Join(app.DataDir, "scans")); err != nil {
		t.Fatal(err)
	}
	// The catalog itself cannot be written.
	app.Store.failSave = func() error { return errors.New("disk full") }
	defer func() { app.Store.failSave = nil }()
	if _, err := app.IngestDrive(ds.ID, drive, "SER-F", "DF", "", "", false, noProg); err == nil {
		t.Error("an ingest whose catalog save failed succeeded")
	}
}

func mkdirFor(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}
