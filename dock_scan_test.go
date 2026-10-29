//go:build !guionly

package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

// mirrorRefsOn lists the archive paths recorded on vol's mirror package.
func mirrorRefsOn(app *App, archiveID, volID int) []string {
	return mirrorPathsOn(app, archiveID, volID, false)
}

// unknownRefsOn lists the archive paths kept as unknown on vol's mirror package.
func unknownRefsOn(app *App, archiveID, volID int) []string {
	return mirrorPathsOn(app, archiveID, volID, true)
}

func mirrorPathsOn(app *App, archiveID, volID int, unknown bool) []string {
	var out []string
	if c := app.mirrorChunkOn(archiveID, volID); c != nil {
		refs := c.Files
		if unknown {
			refs = c.UnknownFiles
		}
		for _, r := range refs {
			out = append(out, r.RelPath)
		}
	}
	return out
}

// A drive scan records every folder it could not read (and the system folders it
// skips), and says whether it is complete. A file under a folder that could not
// be read is unknown, never missing: a rescan that cannot read it keeps what an
// earlier scan found there instead of dropping it.
func TestDockScan_UnreadableFoldersMakeItPartial(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "alpha\n", "deep/b.txt": "bravo\n"})
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{
		"a.txt":                           "alpha\n",
		"locked/b.txt":                    "bravo\n",
		"System Volume Information/x.dat": "os\n",
	})
	ds, err := app.StartDockSession([]int{coll.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, "SER-1", "D1", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	vol := app.Store.VolumeBySerial("SER-1")
	snap := app.Store.VolumeSnapshot(vol.ID)
	if snap.Scan != ScanComplete {
		t.Errorf("first scan is %q, want complete", snap.Scan)
	}
	if len(snap.SkippedFolders) != 1 || snap.SkippedFolders[0].RelPath != "System Volume Information" {
		t.Errorf("skipped folders: %+v, want the system folder", snap.SkippedFolders)
	}
	if got := mirrorRefsOn(app, coll.ID, vol.ID); len(got) != 2 {
		t.Fatalf("first scan matched %v, want both files", got)
	}

	makeUnlistable(t, filepath.Join(drive, "locked"))
	res, err := app.IngestDrive(ds.ID, drive, "SER-1", "D1", "", "", false, noProg)
	if err != nil {
		t.Fatal(err)
	}
	snap = app.Store.VolumeSnapshot(vol.ID)
	if snap.Scan != ScanPartial {
		t.Errorf("scan with an unreadable folder is %q, want partial", snap.Scan)
	}
	if len(snap.UnreadableFolders) != 1 || snap.UnreadableFolders[0].RelPath != "locked" || snap.UnreadableFolders[0].Reason == "" {
		t.Errorf("unreadable folders: %+v, want locked with a reason", snap.UnreadableFolders)
	}
	if snap.Known("locked/b.txt") || !snap.Known("a.txt") || snap.Known("System Volume Information/x.dat") {
		t.Error("Known: want a.txt known, and files under unread or skipped folders unknown")
	}
	if c, _ := res["complete"].(bool); c {
		t.Error("ingest result says complete for a partial scan")
	}
	// The file under the unread folder is unknown: kept, but not counted as a copy.
	if got, unk := mirrorRefsOn(app, coll.ID, vol.ID), unknownRefsOn(app, coll.ID, vol.ID); len(got) != 1 || len(unk) != 1 || unk[0] != "deep/b.txt" {
		t.Errorf("after a partial rescan: mirror %v, unknown %v; want a.txt counted and deep/b.txt unknown", got, unk)
	}
	if cov := app.archiveCoverage([]int{coll.ID}); cov.CoveredFiles != 1 || cov.Unknown != 1 {
		t.Errorf("coverage %d covered, %d unknown; want 1 and 1", cov.CoveredFiles, cov.Unknown)
	}
	// Still unknown after another scan that cannot read it.
	if _, err := app.IngestDrive(ds.ID, drive, "SER-1", "D1", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	if unk := unknownRefsOn(app, coll.ID, vol.ID); len(unk) != 1 {
		t.Errorf("a second partial rescan dropped the unknown file: %v", unk)
	}
	// Once the archive holds a newer version, the old one on the drive is not
	// kept as the archive's file.
	writeTree(t, src, map[string]string{"deep/b.txt": "bravo v2\n"})
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, "SER-1", "D1", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	if unk := unknownRefsOn(app, coll.ID, vol.ID); len(unk) != 0 {
		t.Errorf("an outdated version was kept as unknown: %v", unk)
	}
}

// On Windows a folder whose letter case changed is still the folder the scan
// could not read.
func TestSnapshotKnown_IgnoresCaseOnWindows(t *testing.T) {
	snap := &VolumeSnapshot{UnreadableFolders: []ScanGap{{RelPath: "Locked"}}}
	if got := snap.Known("locked/b.txt"); got != (runtime.GOOS != "windows") {
		t.Errorf("Known(locked/b.txt) = %v on %s", got, runtime.GOOS)
	}
}
