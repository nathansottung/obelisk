//go:build !guionly

package main

import (
	"os"
	"testing"
	"time"
)

// A re-write that cannot run changes nothing: the copy it would replace stays
// current, whether the folder is refused or the package cannot be written.
func TestRewriteCopy_RefusedLeavesTheCopyCurrent(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "REWRITE-ORDER")
	medium := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "HDD-02", Kind: "HDD"})
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	current := func(what string) {
		t.Helper()
		n := 0
		for _, cp := range app.Store.Chunk(c.ID).Copies {
			if cp.VolumeID == vol.ID && !cp.Superseded {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %d current copies on the volume, want 1 (the copy was superseded)", what, n)
		}
	}

	// The package's staged payload is gone, so it cannot be written.
	ch := app.Store.Chunk(c.ID)
	staged := ch.StagedDir
	ch.StagedDir = ""
	app.Store.UpdateChunk(ch)
	if _, err := app.RewriteCopy(c.ID, vol.ID, 0, 0, 0, noProg); err == nil {
		t.Fatal("a re-write of a package with no staged payload ran")
	}
	current("package cannot be written")
	ch = app.Store.Chunk(c.ID)
	ch.StagedDir = staged
	app.Store.UpdateChunk(ch)

	// The medium's folder is now a link to a source.
	if err := os.RemoveAll(medium); err != nil {
		t.Fatal(err)
	}
	_, link, untouched := sourceBehindLink(t, app)
	mustDirLink(t, medium, link)
	_, err := app.RewriteCopy(c.ID, vol.ID, 0, 0, 0, noProg)
	assertSourceRefusal(t, "re-write into a link to a source", err)
	untouched()
	current("folder refused")
}

// Once this period's automatic app backup exists, the hourly run does nothing:
// it does not open the folder, so it neither waits on another write to that
// drive nor fails because the folder now needs a choice.
func TestAutoExport_DoneForThePeriodTouchesNothing(t *testing.T) {
	app, _ := compatApp(t)
	dir := t.TempDir()
	cfg := mustConfig(t, app)
	cfg.AutoExportDir, cfg.AutoExportCadence = dir, "daily"
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := app.maybeAutoExport(now); err != nil {
		t.Fatal(err)
	}
	// Another write holds the drive.
	busy, err := app.openOutFolder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	done := make(chan error, 1)
	go func() { done <- app.maybeAutoExport(now) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("a run with this period's backup present: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("a run with this period's backup present waited on the drive")
	}
}
