//go:build !guionly

package main

import (
	"io/fs"
	"strings"
	"testing"
)

// A drive whose root cannot be listed is not scanned at all: the ingest fails
// saying the drive cannot be read, and the earlier snapshot, scan history and
// mirror record stay as they were. That holds for a root the walk itself cannot
// read too (a fault injected into the walk).
func TestDockScan_UnreadableRootFailsTheIngest(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "alpha\n"})
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.txt": "alpha\n"})
	ds, err := app.StartDockSession([]int{coll.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, "SER-R", "DR", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	vol := app.Store.VolumeBySerial("SER-R")
	before := app.Store.VolumeSnapshot(vol.ID).CapturedAt
	unchanged := func(what string) {
		t.Helper()
		if got := app.Store.VolumeSnapshot(vol.ID).CapturedAt; !got.Equal(before) {
			t.Errorf("%s: the snapshot was replaced", what)
		}
		if n := len(app.Store.ScanRecords(vol.ID)); n != 1 {
			t.Errorf("%s: %d scan records, want the first one only", what, n)
		}
		if got := mirrorRefsOn(app, coll.ID, vol.ID); len(got) != 1 {
			t.Errorf("%s: the drive's mirror changed: %v", what, got)
		}
	}

	orig := dockWalkDir
	dockWalkDir = func(root string, fn fs.WalkDirFunc) error { return fn(root, nil, fs.ErrPermission) }
	_, err = app.IngestDrive(ds.ID, drive, "SER-R", "DR", "", "", false, noProg)
	dockWalkDir = orig
	if err == nil || !strings.Contains(err.Error(), "cannot read the docked drive") {
		t.Errorf("a root the walk cannot read: %v, want the drive reported unreadable", err)
	}
	unchanged("walk cannot read the root")

	makeUnlistable(t, drive)
	_, err = app.IngestDrive(ds.ID, drive, "SER-R", "DR", "", "", false, noProg)
	if err == nil || !strings.Contains(err.Error(), "cannot read the docked drive") {
		t.Errorf("a root that cannot be listed: %v, want the drive reported unreadable", err)
	}
	unchanged("root cannot be listed")
}
