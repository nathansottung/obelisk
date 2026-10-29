//go:build !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A mirror verify at an overriding mount checks only the drive's own record; a
// record of one adopted folder is not read at that mount and is left as it was.
func TestMirrorVerify_OverrideLeavesFolderRecords(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"W2019/a.jpg": "A", "W2020/b.jpg": "B"})
	vol := app.Store.AddVolume(Volume{Label: "OLD-M1", Kind: "HDD"})
	for _, f := range []string{"W2019", "W2020"} {
		if _, err := app.AdoptFolder(filepath.Join(drive, f), arch.ID, vol.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	_, _ = app.VerifyMirrorVolume(vol.ID, filepath.Join(drive, "W2019"), VerifyB, nil)
	for _, c := range app.Store.Chunks(arch.ID) {
		if cp := copyOn(c, vol.ID); c.Mirror && cp != nil && cp.VerifyOK != nil && !*cp.VerifyOK {
			t.Errorf("%s was failed by a verify at another folder", c.Name)
		}
	}
}

// A copy whose drive is now mounted at a different path is credited when the
// drive read is known by its serial to be the copy's drive, and the copy's path
// is updated to where it was read.
func TestVerify_MovedDriveCreditedBySerial(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "MOVED-1")
	medium := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "USB-M", Kind: "HDD", Serial: "SER-MOVED"})
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.Rename(medium, moved); err != nil {
		t.Fatal(err)
	}
	orig := driveSerialAt
	driveSerialAt = func(string) string { return "SER-MOVED" }
	defer func() { driveSerialAt = orig }()
	res, err := app.VerifyChunk(c.ID, moved)
	if err != nil {
		t.Fatal(err)
	}
	cp := copyOn(app.Store.Chunk(c.ID), vol.ID)
	if cr, _ := res["credited"].(int); cr != 1 || cp.VerifyOK == nil || !*cp.VerifyOK {
		t.Errorf("credited %v, copy %+v; want the moved drive's copy credited", res["credited"], cp)
	}
	if normPath(cp.Path) != normPath(filepath.Join(moved, c.Name)) {
		t.Errorf("copy path %s, want it updated to where it was read", cp.Path)
	}
}

// A job is failed only for its own unsaved change. An earlier save failure that
// nothing in the job touched does not fail it, and on a read-only catalog (an
// older app reading a newer catalog) a job completes, noting that nothing was
// recorded.
func TestJob_FailsOnlyForItsOwnUnsavedChange(t *testing.T) {
	app, cid := dcApp(t)
	c := app.Store.AddChunk(Chunk{CollectionID: cid, Name: "JOB-OWN", Status: "STAGED"})
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }
	app.Store.UpdateChunk(c) // an earlier change whose save failed; the disk still fails
	resp, _ := runJob(app, "verify", "probe", func(p func(float64, string)) (map[string]any, error) {
		return map[string]any{"ok": true}, nil // touches nothing
	})
	j0 := waitTerminalJob(t, app.Store, resp["job_id"].(int))
	app.Store.failSave = nil
	if j0.Status != "COMPLETED" {
		t.Errorf("a job that changed nothing: %s %s", j0.Status, j0.Label)
	}

	app.Store.readOnly, app.Store.readOnlyReason = true, "a newer Obelisk wrote this catalog"
	defer func() { app.Store.readOnly = false }()
	resp, _ = runJob(app, "restore", "probe", func(p func(float64, string)) (map[string]any, error) {
		app.Store.Log("restore", "restored a package")
		return map[string]any{"ok": true}, nil
	})
	j := waitTerminalJob(t, app.Store, resp["job_id"].(int))
	if j.Status != "COMPLETED" {
		t.Errorf("a job on a read-only catalog: %s %s, want COMPLETED", j.Status, j.Label)
	}
	if note, _ := j.Result["catalog_not_recorded"].(string); !strings.Contains(note, "read-only") {
		t.Errorf("result %v, want a note that the catalog is read-only", j.Result)
	}
}
