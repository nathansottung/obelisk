//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An app backup carries the drive scan history: each recorded scan's file list
// goes in as scans/<name>, and a restore puts them back so the scans load. The
// pre-restore copy holds the scans that were there before.
func TestAppBackup_CarriesScanHistory(t *testing.T) {
	src := dockApp(t)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	rec := scanDrive(t, src, drive, "BK1")
	res, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !memberNames(readManifest(t, res.TarPath))["scans/"+rec.FileList] {
		t.Fatalf("the backup has no scans/%s", rec.FileList)
	}

	dst := dockApp(t)
	other := t.TempDir()
	writeTree(t, other, map[string]string{"b.txt": "bravo\n"})
	before := scanDrive(t, dst, other, "OLD")
	out, err := dst.RestoreAppBackup(res.TarPath)
	if err != nil {
		t.Fatal(err)
	}
	vol := dst.Store.VolumeBySerial("BK1")
	recs := dst.Store.ScanRecords(vol.ID)
	if len(recs) != 1 {
		t.Fatalf("%d scan records after restore", len(recs))
	}
	list, err := dst.LoadScanFiles(recs[0])
	if err != nil || len(list.Files) != 1 || list.Files[0].RelPath != "a.txt" {
		t.Fatalf("restored scan: %v %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(out.PreRestoreDir, "scans", before.FileList)); err != nil {
		t.Errorf("the pre-restore copy lacks the scans that were there: %v", err)
	}
}

// A restore refuses, before changing anything, a backup whose scan file list
// does not match the scan record in the backup's own catalog.
func TestAppBackup_RestoreChecksScanFiles(t *testing.T) {
	src := dockApp(t)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	rec := scanDrive(t, src, drive, "BK2")
	// The file list changes after it was recorded.
	if err := os.WriteFile(filepath.Join(src.DataDir, "scans", rec.FileList), []byte("not the scan"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := dockApp(t)
	catBefore, _ := os.ReadFile(filepath.Join(dst.DataDir, "catalog.json"))
	_, err = dst.RestoreAppBackup(res.TarPath)
	if err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("a backup with a changed scan file list: %v, want a refusal naming it", err)
	}
	if catAfter, _ := os.ReadFile(filepath.Join(dst.DataDir, "catalog.json")); string(catAfter) != string(catBefore) {
		t.Error("a refused restore changed the catalog")
	}
}
