//go:build !guionly

package main

import (
	"os"
	"path/filepath"
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

// One damaged scan file never makes an app backup unrestorable: the export
// leaves it out (and says so), and a restore that meets one skips it and
// restores everything else.
func TestAppBackup_DamagedScanFileIsLeftOut(t *testing.T) {
	src := dockApp(t)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	rec := scanDrive(t, src, drive, "BK3")
	if err := os.WriteFile(filepath.Join(src.DataDir, "scans", rec.FileList), []byte("not the scan"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if memberNames(readManifest(t, res.TarPath))["scans/"+rec.FileList] || len(res.ScanFilesLeftOut) != 1 {
		t.Errorf("export: damaged scan file included, or not reported (%v)", res.ScanFilesLeftOut)
	}
	dst := dockApp(t)
	if _, err := dst.RestoreAppBackup(res.TarPath); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if v := dst.Store.VolumeBySerial("BK3"); v == nil || len(dst.Store.ScanRecords(v.ID)) != 1 {
		t.Error("the catalog was not restored")
	}

	// A backup that carries a scan file not matching its record (made by hand):
	// the restore skips that file and says so.
	man := appBackupManifest{Members: []appBackupMember{{Name: "scans/" + rec.FileList}}}
	members := map[string][]byte{"scans/" + rec.FileList: []byte("tampered")}
	catBytes, _ := os.ReadFile(filepath.Join(src.DataDir, "catalog.json"))
	members["catalog.json"] = catBytes
	skip, err := checkBackupScanFiles(man, members)
	if err != nil || !skip["scans/"+rec.FileList] {
		t.Errorf("a mismatched scan file: skip %v, err %v; want it skipped", skip, err)
	}
}
