//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A rebuild is made in a folder of its own, and one that fails leaves nothing a
// write could ship: the earlier build stays exactly as it was (removing it is
// not the build's to do), nothing of the failed build is in it, and writing the
// package is refused until a build succeeds.
func TestBuild_FailedRebuildLeavesNothingToWrite(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "REBUILD-1")
	work := filepath.Join(staging, c.Name)
	mustWrite(t, filepath.Join(work, "REBUILD-1.tar.vol99+1.par2"), "old par2 volume")
	before := stagedFiles(t, work)
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
	if c.Status != "FAILED" {
		t.Errorf("after a failed rebuild: status %s; want FAILED", c.Status)
	}
	if got := stagedFiles(t, work); !sameFiles(got, before) {
		t.Errorf("a failed rebuild changed the earlier build in staging:\nbefore %v\nafter  %v", before, got)
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

// stagedFiles is every file under dir with its contents' hash.
func stagedFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		h, err := hashFileHex(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[rel] = h
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// stagingEntries is the names directly in the staging folder.
func stagingEntries(t *testing.T, staging string) []string {
	t.Helper()
	es, err := os.ReadDir(staging)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name())
	}
	return names
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// A rebuild keeps the earlier staged build until the new one has succeeded and
// verified: a build that fails part-way leaves the earlier one exactly as it was,
// still recorded, and nothing of the failed build behind.
func TestRebuild_FailedBuildKeepsTheEarlierBuild(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "KEEP-1")
	work := filepath.Join(staging, c.Name)
	before := stagedFiles(t, work)
	sidecars, tarHash := c.Sidecars, c.TarHash
	c.Status = "FAILED" // a write to a medium failed; the package is built again
	app.Store.UpdateChunk(c)

	buildAfterTarHook = func(tarPath string) { _ = os.Remove(tarPath) }
	err := app.BuildChunk(c.ID, noProg)
	buildAfterTarHook = nil
	if err == nil {
		t.Fatal("setup: the rebuild without its tar succeeded")
	}
	if got := stagedFiles(t, work); !sameFiles(got, before) {
		t.Errorf("a failed rebuild changed the earlier build:\nbefore %v\nafter  %v", before, got)
	}
	c = app.Store.Chunk(c.ID)
	if c.StagedDir != work || c.TarHash != tarHash || len(c.Sidecars) != len(sidecars) {
		t.Errorf("a failed rebuild no longer records the earlier build: staged %q, tar hash kept %v, sidecars %v", c.StagedDir, c.TarHash == tarHash, c.Sidecars)
	}
	for name, h := range c.Sidecars {
		if before[name] != h {
			t.Errorf("recorded sidecar %s does not match the earlier build's file", name)
		}
	}
	if names := stagingEntries(t, staging); len(names) != 1 || names[0] != c.Name {
		t.Errorf("a failed rebuild left something in staging: %v", names)
	}
}

// A rebuild that succeeds replaces the earlier build whole: nothing of the
// earlier build is shipped beside the new one, and nothing is left beside it.
func TestRebuild_SuccessReplacesTheEarlierBuild(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "KEEP-2")
	work := filepath.Join(staging, c.Name)
	mustWrite(t, filepath.Join(work, "KEEP-2.tar.vol99+1.par2"), "old par2 volume")
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, err := os.Stat(filepath.Join(work, "KEEP-2.tar.vol99+1.par2")); err == nil {
		t.Error("a file of the earlier build is beside the new one")
	}
	c = app.Store.Chunk(c.ID)
	if c.Status != "STAGED" || c.StagedDir != work || !app.stagedPayloadPresent(c) {
		t.Errorf("after the rebuild: status %s, staged %q", c.Status, c.StagedDir)
	}
	for name, h := range c.Sidecars {
		if got, err := hashFileHex(filepath.Join(work, name)); err != nil || got != h {
			t.Errorf("sidecar %s in staging does not match the record", name)
		}
	}
	if names := stagingEntries(t, staging); len(names) != 1 || names[0] != c.Name {
		t.Errorf("a rebuild left something in staging: %v", names)
	}
}

// A crash while the new build is being put in place never loses either build,
// and the next build first puts the one the catalog records back under the
// package's name; the other stays beside it, listed as a leftover.
func TestRebuild_CrashDuringTheSwapKeepsTheEarlierBuild(t *testing.T) {
	for _, step := range []string{"aside", "in-place"} {
		t.Run(step, func(t *testing.T) {
			app, staging := newTestApp(t, nativeTools(t))
			c := builtTestChunk(t, app, "KEEP-3")
			work := filepath.Join(staging, c.Name)
			before := stagedFiles(t, work)
			c.Status = "FAILED"
			app.Store.UpdateChunk(c)

			buildSwapCrash = func(s string) bool { return s == step }
			err := app.BuildChunk(c.ID, noProg)
			buildSwapCrash = nil
			if err == nil {
				t.Fatal("setup: the crash was not injected")
			}
			// As after a restart: the catalog is opened again.
			if err := app.Store.Close(); err != nil {
				t.Fatal(err)
			}
			st, err := openTestStore(t, app.DataDir)
			if err != nil {
				t.Fatal(err)
			}
			app.Store = st
			recorded := app.Store.Chunk(c.ID).Sidecars
			// The next build first recovers staging; make it fail so what is left
			// is what recovery did.
			buildAfterTarHook = func(tarPath string) { _ = os.Remove(tarPath) }
			_ = app.BuildChunk(c.ID, noProg)
			buildAfterTarHook = nil
			for name, h := range recorded {
				if got, err := hashFileHex(filepath.Join(work, name)); err != nil || got != h {
					t.Errorf("after a crash at %s, staging does not hold the build the catalog records (%s)", step, name)
				}
			}
			// Nothing is lost: the earlier build is in place or beside it.
			found := sameFiles(stagedFiles(t, work), before)
			es, _ := os.ReadDir(staging)
			for _, e := range es {
				if e.IsDir() && e.Name() != c.Name && sameFiles(stagedFiles(t, filepath.Join(staging, e.Name())), before) {
					found = true
					if l, _ := listLeftoversIn(staging); len(l) == 0 {
						t.Error("the build not in place is not listed as a leftover")
					}
				}
			}
			if !found {
				t.Errorf("after a crash at %s the earlier build is gone", step)
			}
		})
	}
}
