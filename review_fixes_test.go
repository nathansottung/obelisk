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

// A burn is gated like a write: a package whose last build failed is not burned.
func TestBurn_RefusesAFailedPackage(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "BURN-FAILED")
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -volid {LABEL} -map {SRC} /`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	q := app.Store.AddBurnQueue(BurnQueue{Name: "Q", MediaKind: "BD-R25", Discs: []*BurnDisc{{ChunkID: c.ID, Status: "PENDING"}}})
	if _, err := app.BurnNext(q.ID, func(float64, string) {}); err == nil || !strings.Contains(err.Error(), "build it first") {
		t.Errorf("burning a FAILED package: %v, want a refusal", err)
	}
}

// A disk serial is shared by every partition of the disk, so a check read at
// another folder of that disk is not credited to the copy by its serial alone.
func TestCreditCopyCheck_SerialNeedsThePackageFolder(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v := st.AddVolume(Volume{Label: "DISK", Serial: "SER-P"})
	c := st.AddChunk(Chunk{Name: "PKG"})
	st.RecordCopy(c, v.ID, "E:/PKG", false)
	if n := st.CreditCopyCheck(c, CopyAt{Path: "F:/OTHER", Serial: "SER-P"}, true, "B"); n != 0 {
		t.Errorf("credited %d from another folder of the same disk, want 0", n)
	}
	if n := st.CreditCopyCheck(c, CopyAt{Path: "F:/PKG", Serial: "SER-P"}, true, "B"); n != 1 {
		t.Errorf("credited %d at the package's own folder on the moved disk, want 1", n)
	}
}

// A campaign's mirror check at an advisory level verifies nothing, so it never
// counts toward the campaign's verified total.
func TestVerifyCampaign_AdvisoryMirrorIsNotCounted(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Photos")
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.jpg": "A"})
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "CAMP-1", Kind: "HDD"})
	if _, err := app.MirrorToVolume(coll.ID, nil, dest, vol.ID, 0, "", noProgMirror); err != nil {
		t.Fatal(err)
	}
	res, err := app.VerifyCampaign(dest, VerifyA, noProg)
	if err != nil {
		t.Fatal(err)
	}
	if mr, _ := res["mirror_results"].([]map[string]any); len(mr) != 1 {
		t.Fatalf("setup: mirror results %v, want the mirror checked", res["mirror_results"])
	}
	if ok, _ := res["ok"].(int); ok != 0 {
		t.Errorf("ok %d after an advisory check, want 0", ok)
	}
}
