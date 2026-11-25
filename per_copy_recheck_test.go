//go:build !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpanSet writes every pending segment of a package to its own tape folder.
func writeSpanSet(t *testing.T, app *App, id int, root, prefix string, volumeID int) []string {
	t.Helper()
	var tapes []string
	for i := 0; i < 50; i++ {
		tape := filepath.Join(root, fmt.Sprintf("%s%02d", prefix, i))
		if err := os.MkdirAll(tape, 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := app.SpanWriteNext(id, tape, 0, 0, 0, volumeID, noProg)
		if err != nil {
			t.Fatal(err)
		}
		tapes = append(tapes, tape)
		if done, _ := res["complete"].(bool); done {
			return tapes
		}
	}
	t.Fatal("the set never finished")
	return nil
}

// writeSpanSetOver is writeSpanSet onto tapes that hold another set: each tape
// already holding one goes through the delete gate's span-write-over.
func writeSpanSetOver(t *testing.T, app *App, id int, root, prefix string, volumeID int) []string {
	t.Helper()
	var tapes []string
	for i := 0; i < 50; i++ {
		tape := filepath.Join(root, fmt.Sprintf("%s%02d", prefix, i))
		if err := os.MkdirAll(tape, 0o755); err != nil {
			t.Fatal(err)
		}
		var res map[string]any
		var err error
		if c := app.Store.Chunk(id); app.spanTargetConflict(c, filepath.Join(tape, c.Name)) != nil {
			res, err = spanWriteOver(t, app, id, tape, volumeID)
		} else {
			res, err = app.SpanWriteNext(id, tape, 0, 0, 0, volumeID, noProg)
		}
		if err != nil {
			t.Fatal(err)
		}
		tapes = append(tapes, tape)
		if done, _ := res["complete"].(bool); done {
			return tapes
		}
	}
	t.Fatal("the set never finished")
	return nil
}

func resetSegments(t *testing.T, app *App, c *Chunk) {
	t.Helper()
	for i := range c.Segments {
		if err := app.Store.UpdateSegment(c, i, func(s *Segment) { s.Status = "PENDING" }); err != nil {
			t.Fatal(err)
		}
	}
}

// Overwriting a tape of one set leaves an intact replica of the same build alone.
func TestPerCopy_OverwritingOneSetKeepsItsReplica(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "TWO-SETS", true)
	x := app.Store.AddVolume(Volume{Label: "SET-X", Kind: "TAPE"})
	y := app.Store.AddVolume(Volume{Label: "SET-Y", Kind: "TAPE"})
	root := t.TempDir()
	xTapes := writeSpanSet(t, app, c.ID, root, "x", x.ID)
	c = app.Store.Chunk(c.ID)
	resetSegments(t, app, c)
	writeSpanSet(t, app, c.ID, root, "y", y.ID)
	c = app.Store.Chunk(c.ID)
	if c.CurrentCopyCount() != 2 {
		t.Fatalf("two sets on record expected: %+v", c.Copies)
	}
	c = rebuildPackage(t, app, c)
	if _, err := spanWriteOver(t, app, c.ID, xTapes[0], x.ID); err != nil {
		t.Fatal(err)
	}
	got := app.Store.Chunk(c.ID)
	if !got.Copies[0].Superseded {
		t.Fatal("the overwritten set still counts")
	}
	if got.Copies[1].Superseded || got.VerifiedCopyCount() != 1 {
		t.Fatalf("the intact replica was retired: %+v", got.Copies)
	}
}

// twoBuildsRemounted is an old and a new build on one drive, now mounted elsewhere.
func twoBuildsRemounted(t *testing.T, name string) (*App, *Chunk, string) {
	t.Helper()
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, name, false)
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "drive", "a"), filepath.Join(root, "drive", "b")
	for _, d := range []string{oldDir, newDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.WriteChunk(c.ID, oldDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
	if _, err := app.WriteChunk(c.ID, newDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "mounted-elsewhere")
	if err := os.Rename(filepath.Join(root, "drive"), moved); err != nil {
		t.Fatal(err)
	}
	return app, app.Store.Chunk(c.ID), moved
}

// A damaged copy whose RESTORE.txt names its build loses its credit, and only it.
func TestPerCopy_RemountedDamageFailsTheBuildItsNoteNames(t *testing.T) {
	app, c, moved := twoBuildsRemounted(t, "DAMAGE-NAMED")
	flipByte(t, filepath.Join(moved, "b", c.Name, payloadName(c)))
	r, err := app.VerifyChunk(c.ID, filepath.Join(moved, "b"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := r["verify_ok"].(bool); ok || r["credited"] != 1 {
		t.Fatalf("result: %v", r)
	}
	got := app.Store.Chunk(c.ID)
	if got.VerifiedCopyCount() != 1 || got.Copies[1].VerifyOK == nil || *got.Copies[1].VerifyOK {
		t.Fatalf("copies: %+v", got.Copies)
	}
	if got.Copies[0].VerifyOK == nil || !*got.Copies[0].VerifyOK {
		t.Fatal("the other build lost its credit")
	}
}

// With no note to say which build was read, the copy ID still does: that copy is
// failed, and only it.
func TestPerCopy_RemountedDamageWithNoNoteFailsTheCopyItsIDNames(t *testing.T) {
	app, c, moved := twoBuildsRemounted(t, "DAMAGE-UNNAMED")
	dir := filepath.Join(moved, "b", c.Name)
	flipByte(t, filepath.Join(dir, payloadName(c)))
	if err := os.Remove(filepath.Join(dir, "RESTORE.txt")); err != nil {
		t.Fatal(err)
	}
	r, err := app.VerifyChunk(c.ID, filepath.Join(moved, "b"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := r["verify_ok"].(bool); ok || r["credited"] != 1 {
		t.Fatalf("result: %v", r)
	}
	got := app.Store.Chunk(c.ID)
	if got.VerifiedCopyCount() != 1 || got.Copies[1].VerifyOK == nil || *got.Copies[1].VerifyOK {
		t.Fatalf("copies: %+v", got.Copies)
	}
	if got.Copies[0].VerifyOK == nil || !*got.Copies[0].VerifyOK {
		t.Fatal("the other build lost its credit")
	}
}

// The same set rewritten to the same tapes after a remount is still one copy.
func TestPerCopy_SameSetRewrittenAfterARemountIsOneCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := app.Store.AddVolume(Volume{Label: "ONE-SET", Kind: "TAPE"})
	c, _ := privateChunk(t, app, "ONE-SET", true)
	root := t.TempDir()
	first := filepath.Join(root, "first")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpanSet(t, app, c.ID, first, "t", vol.ID)
	c = app.Store.Chunk(c.ID)
	resetSegments(t, app, c)
	second := filepath.Join(root, "second")
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	writeSpanSetOver(t, app, c.ID, second, "t", vol.ID)
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 1 {
		t.Fatalf("one physical set, but current=%d verified=%d", got.CurrentCopyCount(), got.VerifiedCopyCount())
	}
	if n := len(got.allTapes()); n != len(got.Segments) {
		t.Fatalf("%d tape entries for %d tapes", n, len(got.Segments))
	}
}

// A par2 write that fails before touching the medium leaves the old set current.
func TestPerCopy_FailedPar2WriteKeepsTheOldSet(t *testing.T) {
	h, app, c := frozenSpanRebuilt(t)
	last := len(c.Segments) - 1
	if !c.Segments[last].Par2 {
		t.Fatal("the fixture's set has no par2 tape")
	}
	for i := 0; i < last; i++ {
		if err := app.Store.UpdateSegment(c, i, func(s *Segment) { s.Status = "VERIFIED" }); err != nil {
			t.Fatal(err)
		}
	}
	staged, _ := filepath.Glob(filepath.Join(c.StagedDir, "*.par2"))
	if len(staged) == 0 {
		t.Fatal("no staged par2 set")
	}
	for _, p := range staged {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	tape := filepath.Join(h.root, "tapes", fmt.Sprintf("tape%02d", last))
	if _, err := app.SpanWriteNext(c.ID, tape, 0, 0, 0, c.Copies[0].VolumeID, noProg); err == nil {
		t.Fatal("the write did not fail")
	}
	// The frozen set has no copy ID, so it is not identified since the upgrade; a
	// write that changed nothing leaves it exactly so.
	got := frozenChunk(t, app, c.Name)
	if got.Copies[0].Superseded || got.Copies[0].Unidentified != idNone {
		t.Fatalf("a write that changed nothing retired the set: %+v", got.Copies[0])
	}
}

// With its RESTORE.txt missing or damaged, an overwritten copy is still found.
func TestPerCopy_OverwriteWithABadNoteLeavesNoPhantom(t *testing.T) {
	for name, spoil := range map[string]func(string) error{
		"missing": os.Remove,
		"damaged": func(p string) error { return os.WriteFile(p, []byte("not the note"), 0o644) },
	} {
		t.Run(name, func(t *testing.T) {
			app, _ := newTestApp(t, nativeTools(t))
			vol := remountable(t, app)
			c, _ := privateChunk(t, app, "BAD-NOTE", false)
			root := t.TempDir()
			first := filepath.Join(root, "first-letter")
			if err := os.MkdirAll(first, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := app.WriteChunk(c.ID, first, 0, 0, 0, vol.ID, noProg); err != nil {
				t.Fatal(err)
			}
			c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
			second := filepath.Join(root, "second-letter")
			if err := os.Rename(first, second); err != nil {
				t.Fatal(err)
			}
			if err := spoil(filepath.Join(second, c.Name, "RESTORE.txt")); err != nil {
				t.Fatal(err)
			}
			if _, err := writeOver(t, app, c.ID, second, vol.ID); err != nil {
				t.Fatal(err)
			}
			if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 || got.CurrentCopyCount() != 1 {
				t.Fatalf("one physical copy, but verified=%d current=%d", got.VerifiedCopyCount(), got.CurrentCopyCount())
			}
		})
	}
}

func TestPerCopy_SpanOverwriteWithNoNoteRetiresTheSet(t *testing.T) {
	h, app, c := frozenSpanRebuilt(t)
	from := filepath.Join(h.root, "tapes", "tape00")
	to := filepath.Join(h.root, "tapes", "tape00-remounted")
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(to, c.Name, "RESTORE.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := spanWriteOver(t, app, c.ID, to, c.Copies[0].VolumeID); err != nil {
		t.Fatal(err)
	}
	if got := frozenChunk(t, app, c.Name); !got.Copies[0].Superseded {
		t.Fatal("the old set still counts after one of its tapes was overwritten")
	}
}

// adoptedAtRoot is a package with no recorded sidecar hashes at a shared medium root.
func adoptedAtRoot(t *testing.T, app *App, name string) (*Chunk, string, int) {
	t.Helper()
	c, medium, _ := copiedPackage(t, app, name)
	vol := c.Copies[0].VolumeID
	flat := t.TempDir()
	copyFolderFiles(t, filepath.Join(medium, c.Name), flat, all)
	c.Sidecars, c.Adopted = nil, true
	c.Copies[0].Sidecars, c.Copies[0].Path = nil, flat
	app.Store.UpdateChunk(c)
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	return c, flat, vol
}

func TestPerCopy_RefillOfAnAdoptedPackageAtARootCanBeWritten(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, flat, vol := adoptedAtRoot(t, app, "ADOPTED-FLAT")
	if _, err := app.RefillStaging(c.ID, vol, flat, noProg); err != nil {
		t.Fatal(err)
	}
	if err := chunkStagedForWrite(app.Store.Chunk(c.ID)); err != nil {
		t.Fatalf("the refill cannot be written: %v", err)
	}
}

// A RESTORE.txt at a shared root that is another package's is not taken as this one's.
func TestPerCopy_RefillRefusesAnotherPackagesNote(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, flat, vol := adoptedAtRoot(t, app, "ADOPTED-OTHER")
	if err := os.WriteFile(filepath.Join(flat, "RESTORE.txt"), []byte("HOW TO RESTORE NEIGHBOUR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := app.RefillStaging(c.ID, vol, flat, noProg)
	if err == nil || !strings.Contains(err.Error(), "RESTORE.txt") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.StagedDir, payloadName(c))); err == nil {
		t.Fatal("a refused refill changed staging")
	}
}
