//go:build !guionly

package main

import (
	"os"
	"testing"
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
