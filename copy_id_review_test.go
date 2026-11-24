//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serialFromMarker makes each test drive report the serial written in a marker
// file at its root, so moving a drive's folder moves its serial with it.
func serialFromMarker(t *testing.T) {
	t.Helper()
	prevSerial, prevPart := driveSerialAt, partitionIDAt
	driveSerialAt = func(p string) string {
		for d := p; ; d = filepath.Dir(d) {
			if b, err := os.ReadFile(filepath.Join(d, ".test-serial")); err == nil {
				return strings.TrimSpace(string(b))
			}
			if filepath.Dir(d) == d {
				return ""
			}
		}
	}
	partitionIDAt = func(string) string { return "" }
	t.Cleanup(func() { driveSerialAt, partitionIDAt = prevSerial, prevPart })
}

func markedDrive(t *testing.T, dir, serial string) string {
	t.Helper()
	mkdirs(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".test-serial"), []byte(serial), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A disc burned with the package's files at its root reads back with its own ID,
// and a later check of the disc credits it.
func TestCopyID_FlatBurnIsIdentified(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "FLAT-BURN", false)
	disc := filepath.Join(t.TempDir(), "disc")
	mkdirs(t, disc)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	cfg.BurnVerifyMount = disc
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	q := app.Store.AddBurnQueue(BurnQueue{Name: "Q", MediaKind: "CUSTOM", Discs: []*BurnDisc{{ChunkID: c.ID, Status: "PENDING"}}})
	prev := runBurnProgram
	runBurnProgram = func([]string) error { copyFolderFiles(t, c.StagedDir, disc, all); return nil }
	t.Cleanup(func() { runBurnProgram = prev })
	if _, err := app.BurnNext(q.ID, noProg); err != nil {
		t.Fatalf("burn: %v", err)
	}
	got := app.Store.Chunk(c.ID)
	cp := currentCopyAt(t, got, disc)
	if cp.CopyID == "" || !verified(cp) || cp.CopyID != copyIDIn(t, disc) {
		t.Fatalf("burned disc copy: %+v", cp)
	}
	wantVerifies(t, app, c.ID, disc)
}

// A disc with no ID at a mount every disc shares says nothing about the others.
func TestCopyID_MissingIDAtASharedMountMarksNoOtherDisc(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "SHARED-MOUNT", false)
	drive := t.TempDir()
	vol := app.Store.EnsureUnregistered().ID
	for i := 0; i < 2; i++ {
		id, _ := newCopyID()
		app.Store.RecordWrittenCopy(c, vol, drive, true, id, priorBuild{NotAPlace: true})
	}
	copyFolderFiles(t, c.StagedDir, drive, all) // a disc in the drive with no ID
	if r, err := app.VerifyChunk(c.ID, drive); err != nil || r["credited"] != 0 {
		t.Fatalf("%v %v", r, err)
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 2 {
		t.Fatalf("one unidentified disc cost the others their credit: %+v", got.Copies)
	}
}

// After the user keeps one medium, swapping which drive sits where never credits the
// one set aside, and still credits the one kept.
func TestCopyID_SetAsideMediumIsKnownByItsDrive(t *testing.T) {
	for _, keep := range []string{"recorded", "found"} {
		t.Run(keep, func(t *testing.T) {
			serialFromMarker(t)
			app, _ := newTestApp(t, nativeTools(t))
			c, _ := privateChunk(t, app, "SWAP-"+strings.ToUpper(keep), false)
			root := t.TempDir()
			x, y := markedDrive(t, filepath.Join(root, "x"), "SERIAL-X"), markedDrive(t, filepath.Join(root, "y"), "SERIAL-Y")
			if _, err := app.WriteChunk(c.ID, x, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
				t.Fatal(err)
			}
			mkdirs(t, filepath.Join(y, c.Name))
			if err := copyFolderInto(filepath.Join(x, c.Name), filepath.Join(y, c.Name)); err != nil {
				t.Fatal(err)
			}
			if r, _ := app.VerifyChunk(c.ID, y); r["credited"] != 0 {
				t.Fatalf("clone credited: %v", r)
			}
			cp := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(x, c.Name))
			if err := app.SettleCopyConflict(c.ID, cp.CopyID, keep); err != nil {
				t.Fatal(err)
			}
			kept, other := "SERIAL-X", "SERIAL-Y"
			if keep == "found" {
				kept, other = other, kept
			}
			// Swap the drives' mount points.
			tmp := filepath.Join(root, "tmp")
			for _, mv := range [][2]string{{x, tmp}, {y, x}, {tmp, y}} {
				if err := os.Rename(mv[0], mv[1]); err != nil {
					t.Fatal(err)
				}
			}
			at := map[string]string{driveSerialAt(x): x, driveSerialAt(y): y}
			if r, _ := app.VerifyChunk(c.ID, at[other]); r["credited"] != 0 {
				t.Fatalf("the set-aside drive earned credit: %v", r)
			}
			if r, _ := app.VerifyChunk(c.ID, at[kept]); r["credited"] != 1 {
				t.Fatalf("the kept drive earned none: %v", r)
			}
			if got := app.Store.Chunk(c.ID); got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 1 {
				t.Fatalf("current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
			}
		})
	}
}

// A tape set's folder found through a path with pattern characters still rejoins.
func TestCopyID_SpanViewReadsAFolderWithBrackets(t *testing.T) {
	// The package's current layout has three data tapes; the set in the folder is an
	// older one with two, which only the files found there can tell.
	older := []Segment{{Index: 1, Bytes: 4}, {Index: 2, Bytes: 4}, {Index: 3, Par2: true}}
	c := &Chunk{Name: "BRACKET-SPAN", Spanned: true, Segments: []Segment{{Index: 1, Bytes: 3}, {Index: 2, Bytes: 3}, {Index: 3, Bytes: 2}},
		Copies: []Copy{{Segments: older}}}
	dir := filepath.Join(t.TempDir(), "pool [1]")
	mkdirs(t, dir)
	for _, n := range []string{".seg001", ".seg002"} {
		if err := os.WriteFile(filepath.Join(dir, c.Name+n), []byte("abcd"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if v := spanViewFor(c, dir); dataSegmentCount(v.Segments) != 2 {
		t.Fatalf("segments found: %+v", v.Segments)
	}
}

// The par2 set of a package on a medium is found through a path with pattern
// characters.
func TestCopyID_Par2SetFoundUnderBrackets(t *testing.T) {
	c := &Chunk{Name: "BRACKET-PAR2"}
	dir := filepath.Join(t.TempDir(), "disc [2]")
	mkdirs(t, dir)
	for _, n := range []string{".tar.par2", ".tar.vol000+01.par2"} {
		if err := os.WriteFile(filepath.Join(dir, c.Name+n), []byte("p"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := par2SetFiles(dir, c); len(got) != 2 {
		t.Fatalf("par2 files found: %v", got)
	}
	if !hasPar2Set(adoptCand{name: c.Name, dir: dir}) {
		t.Fatal("adoption did not see the par2 set")
	}
}

// No check exists for a whole spanned set, so an unidentified set is not told to
// verify.
func TestCopyID_UnidentifiedSpannedSetSaysToWriteItAgain(t *testing.T) {
	got := copyResult(Copy{Unidentified: idNone, Segments: []Segment{{Index: 1}}})
	if !strings.Contains(got, "write the set again to confirm") || strings.Contains(got, "verify") {
		t.Fatalf("%q", got)
	}
}

// Writing a new copy to the medium set aside, as the app advises, gives it its own
// ID and leaves the kept copy current.
func TestCopyID_WritingToTheSetAsideMediumKeepsTheKeptCopy(t *testing.T) {
	for _, bySerial := range []bool{false, true} {
		t.Run(map[bool]string{false: "same serial", true: "own serials"}[bySerial], func(t *testing.T) {
			if bySerial {
				serialFromMarker(t)
			}
			app, _ := newTestApp(t, nativeTools(t))
			c, _ := privateChunk(t, app, "WRITE-SET-ASIDE", false)
			root := t.TempDir()
			medium, clone := markedDrive(t, filepath.Join(root, "m"), "SERIAL-M"), markedDrive(t, filepath.Join(root, "c"), "SERIAL-C")
			if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
				t.Fatal(err)
			}
			mkdirs(t, filepath.Join(clone, c.Name))
			if err := copyFolderInto(filepath.Join(medium, c.Name), filepath.Join(clone, c.Name)); err != nil {
				t.Fatal(err)
			}
			if _, err := app.VerifyChunk(c.ID, clone); err != nil {
				t.Fatal(err)
			}
			cp := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
			if err := app.SettleCopyConflict(c.ID, cp.CopyID, "recorded"); err != nil {
				t.Fatal(err)
			}
			wantVerifies(t, app, c.ID, medium)
			if _, err := app.WriteChunk(c.ID, clone, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
				t.Fatal(err)
			}
			got := app.Store.Chunk(c.ID)
			// Told apart by serial, the kept copy keeps its credit; by place alone, it
			// waits for a check, as the write may have been to it.
			kept := currentCopyAt(t, got, filepath.Join(medium, c.Name))
			if kept.CopyID != cp.CopyID || verified(kept) != bySerial {
				t.Fatalf("the kept copy: %+v", kept)
			}
			if want := map[bool]int{false: 1, true: 2}[bySerial]; got.CurrentCopyCount() != 2 || got.VerifiedCopyCount() != want {
				t.Fatalf("current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
			}
			wantVerifies(t, app, c.ID, medium)
			wantVerifies(t, app, c.ID, clone)
		})
	}
}

// When nothing tells the drives apart, a write at the place set aside may have been
// to the kept medium itself: the kept copy keeps its record but not its credit, so
// one medium never counts twice.
func TestCopyID_RewriteAtTheSetAsidePlaceNeverCountsOneMediumTwice(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "KEPT-MOVED", false)
	root := t.TempDir()
	medium, clone := filepath.Join(root, "m"), filepath.Join(root, "c")
	mkdirs(t, medium)
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
		t.Fatal(err)
	}
	mkdirs(t, filepath.Join(clone, c.Name))
	if err := copyFolderInto(filepath.Join(medium, c.Name), filepath.Join(clone, c.Name)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.VerifyChunk(c.ID, clone); err != nil {
		t.Fatal(err)
	}
	cp := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
	if err := app.SettleCopyConflict(c.ID, cp.CopyID, "recorded"); err != nil {
		t.Fatal(err)
	}
	wantVerifies(t, app, c.ID, medium)
	// The clone is unplugged and the kept drive is mounted where the clone was.
	if err := os.RemoveAll(clone); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(medium, clone); err != nil {
		t.Fatal(err)
	}
	if _, err := app.WriteChunk(c.ID, clone, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 {
		t.Fatalf("one medium counts as %d verified copies: %+v", got.VerifiedCopyCount(), got.Copies)
	}
}
