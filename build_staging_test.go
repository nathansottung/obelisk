//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A rebuild starts from an empty staging folder, and one that fails leaves
// nothing a write could ship: the old payload, par2 set and sidecars are gone,
// the package records no staged folder, and writing it is refused until a build
// succeeds.
func TestBuild_FailedRebuildLeavesNothingToWrite(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "REBUILD-1")
	work := filepath.Join(staging, c.Name)
	mustWrite(t, filepath.Join(work, "REBUILD-1.tar.vol99+1.par2"), "old par2 volume")
	// The staged payload was found damaged (a write's stream check), so the
	// package is FAILED and is rebuilt over its old staging.
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)

	// The rebuild fails part-way: its tar disappears before it is hashed.
	buildAfterTarHook = func(tarPath string) { _ = os.Remove(tarPath) }
	err := app.BuildChunk(c.ID, noProg)
	buildAfterTarHook = nil
	if err == nil {
		t.Fatal("setup: the rebuild without its tar succeeded")
	}
	c = app.Store.Chunk(c.ID)
	if c.Status != "FAILED" || c.StagedDir != "" {
		t.Errorf("after a failed rebuild: status %s, staged dir %q; want FAILED and none", c.Status, c.StagedDir)
	}
	for _, old := range []string{"REBUILD-1.tar.vol99+1.par2", "REBUILD-1.manifest.json", "RESTORE.txt"} {
		if _, err := os.Stat(filepath.Join(work, old)); err == nil {
			t.Errorf("a failed rebuild left %s in staging", old)
		}
	}
	if _, err := app.WriteChunk(c.ID, t.TempDir(), 0, 0, 0, 0, noProg); err == nil {
		t.Error("a package whose last build failed was written")
	}
	// Even with a staged folder still recorded, a FAILED package is not written.
	c.StagedDir = work
	app.Store.UpdateChunk(c)
	if _, err := app.WriteChunk(c.ID, t.TempDir(), 0, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "build it first") {
		t.Errorf("a FAILED package was written: %v", err)
	}
}

// The manifest and RESTORE.txt are recorded when a package is built, must be in
// staging (unchanged) for a write, and are read back from the medium by a verify.
func TestSidecars_RequiredAndChecked(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "SIDECAR-1")
	if len(c.Sidecars) < 2 {
		t.Fatalf("the build recorded sidecars %v, want the manifest and RESTORE.txt", c.Sidecars)
	}
	restore := filepath.Join(staging, c.Name, "RESTORE.txt")
	orig, _ := os.ReadFile(restore)
	if err := os.Remove(restore); err != nil {
		t.Fatal(err)
	}
	if _, err := app.WriteChunk(c.ID, t.TempDir(), 0, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "RESTORE.txt") {
		t.Errorf("a write without RESTORE.txt in staging: %v, want a refusal naming it", err)
	}
	mustWrite(t, restore, "changed since the build")
	if _, err := app.WriteChunk(c.ID, t.TempDir(), 0, 0, 0, 0, noProg); err == nil || !strings.Contains(err.Error(), "RESTORE.txt") {
		t.Errorf("a write with a changed RESTORE.txt in staging: %v, want a refusal naming it", err)
	}
	if err := os.WriteFile(restore, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(medium, c.Name, "RESTORE.txt"), "damaged on the medium")
	res, err := app.VerifyChunk(c.ID, medium)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := res["verify_ok"].(bool); ok {
		t.Error("a verify passed with RESTORE.txt changed on the medium")
	}
}
