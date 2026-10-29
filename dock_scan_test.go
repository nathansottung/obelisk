//go:build !guionly

package main

import (
	"path/filepath"
	"testing"
)

// mirrorRefsOn lists the archive paths recorded on vol's mirror package.
func mirrorRefsOn(app *App, archiveID, volID int) []string {
	var out []string
	for _, c := range app.Store.Chunks(archiveID) {
		if !c.Mirror {
			continue
		}
		for _, cp := range c.Copies {
			if cp.VolumeID == volID && !cp.Superseded {
				for _, r := range c.Files {
					out = append(out, r.RelPath)
				}
			}
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
	if got := mirrorRefsOn(app, coll.ID, vol.ID); len(got) != 2 {
		t.Errorf("after a partial rescan the drive's mirror lists %v; the file under the unread folder was dropped (unknown, not missing)", got)
	}

	// A drive whose root cannot be listed is not scanned at all: the ingest
	// fails and the earlier snapshot and record stay as they were.
	before := app.Store.VolumeSnapshot(vol.ID).CapturedAt
	makeUnlistable(t, drive)
	if _, err := app.IngestDrive(ds.ID, drive, "SER-1", "D1", "", "", false, noProg); err == nil {
		t.Error("a drive whose root cannot be listed was ingested")
	}
	if got := app.Store.VolumeSnapshot(vol.ID).CapturedAt; !got.Equal(before) {
		t.Error("a failed scan replaced the snapshot")
	}
	if got := mirrorRefsOn(app, coll.ID, vol.ID); len(got) != 2 {
		t.Errorf("a failed scan changed the drive's mirror: %v", got)
	}
}
