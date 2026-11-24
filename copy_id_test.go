//go:build !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var copyIDFileRe = regexp.MustCompile(`^obelisk-copy-id 1\n([0-9a-f]{32})\n$`)

// copyIDIn reads the copy ID file in a package folder on a medium, failing the test
// if it is not exactly the format version and one ID.
func copyIDIn(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, copyIDFile))
	if err != nil {
		t.Fatalf("no copy ID file in %s: %v", dir, err)
	}
	m := copyIDFileRe.FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("the copy ID file holds more than a version and an ID: %q", b)
	}
	return m[1]
}

func currentCopyAt(t *testing.T, c *Chunk, dir string) Copy {
	t.Helper()
	for _, cp := range c.Copies {
		if !cp.Superseded && normPath(cp.Path) == normPath(dir) {
			return cp
		}
	}
	t.Fatalf("no current copy at %s: %+v", dir, c.Copies)
	return Copy{}
}

func verified(cp Copy) bool { return cp.VerifyOK != nil && *cp.VerifyOK }

// Each write puts a fresh random ID beside the package, and the copy records it.
// Writing the same build again gives that copy a new ID.
func TestCopyID_EachWriteHasItsOwnID(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "ONE-ID-EACH", false)
	vol := app.Store.EnsureUnregistered().ID
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		if _, err := app.WriteChunk(c.ID, d, 0, 0, 0, vol, noProg); err != nil {
			t.Fatal(err)
		}
	}
	got := app.Store.Chunk(c.ID)
	idA, idB := copyIDIn(t, filepath.Join(a, c.Name)), copyIDIn(t, filepath.Join(b, c.Name))
	if idA == idB {
		t.Fatal("two writes of one build share an ID")
	}
	if cp := currentCopyAt(t, got, filepath.Join(a, c.Name)); cp.CopyID != idA {
		t.Fatalf("copy at a records %q, medium holds %q", cp.CopyID, idA)
	}
	if cp := currentCopyAt(t, got, filepath.Join(b, c.Name)); cp.CopyID != idB {
		t.Fatalf("copy at b records %q, medium holds %q", cp.CopyID, idB)
	}
	if _, err := writeOver(t, app, c.ID, a, vol); err != nil {
		t.Fatal(err)
	}
	again := copyIDIn(t, filepath.Join(a, c.Name))
	if again == idA {
		t.Fatal("a second write of the same folder kept the old ID")
	}
	got = app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 2 || got.VerifiedCopyCount() != 2 {
		t.Fatalf("current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
	}
	if cp := currentCopyAt(t, got, filepath.Join(a, c.Name)); cp.CopyID != again {
		t.Fatalf("the rewritten copy records %q, medium holds %q", cp.CopyID, again)
	}
	for _, cp := range got.Copies {
		if cp.CopyID == idA && !cp.Superseded {
			t.Fatal("the copy the rewrite replaced is still current")
		}
	}
	if strings.Contains(readFileString(t, filepath.Join(a, c.Name, copyIDFile)), c.Name) {
		t.Fatal("the copy ID file names the package")
	}
	// Staging never holds an ID: it is not a copy.
	if _, err := os.Stat(filepath.Join(got.StagedDir, copyIDFile)); err == nil {
		t.Fatal("staging holds a copy ID file")
	}
}

func readFileString(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The note on every new build says what the file is in one line.
func TestCopyID_RestoreNoteExplainsTheFile(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "NOTE-LINE")
	note := readFileString(t, filepath.Join(medium, c.Name, "RESTORE.txt"))
	var lines []string
	for _, l := range strings.Split(note, "\n") {
		if strings.Contains(l, copyIDFile) {
			lines = append(lines, l)
		}
	}
	if len(lines) != 1 {
		t.Fatalf("want one line about %s, got %q", copyIDFile, lines)
	}
	l := strings.ToLower(lines[0])
	for _, want := range []string{"not needed", "restore", "no private"} {
		if !strings.Contains(l, want) {
			t.Fatalf("the line %q does not say %q", lines[0], want)
		}
	}
}

// remountedDrive writes copies to folders of one drive with a known serial, then
// mounts the drive somewhere else. It returns the new mount.
func remountedDrive(t *testing.T, app *App, vol *Volume, root string, write func(drive string)) string {
	t.Helper()
	drive := filepath.Join(root, "drive")
	write(drive)
	moved := filepath.Join(root, "mounted-elsewhere")
	if err := os.Rename(drive, moved); err != nil {
		t.Fatal(err)
	}
	return moved
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Overwriting one folder of a moved drive retires the copy that folder held, and
// no other copy of the same build.
func TestCopyID_OverwriteRetiresOnlyTheCopyInThatFolder(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, "TWO-FOLDERS", false)
	root := t.TempDir()
	moved := remountedDrive(t, app, vol, root, func(drive string) {
		for _, sub := range []string{"a", "b"} {
			mkdirs(t, filepath.Join(drive, sub))
			if _, err := app.WriteChunk(c.ID, filepath.Join(drive, sub), 0, 0, 0, vol.ID, noProg); err != nil {
				t.Fatal(err)
			}
		}
	})
	before := app.Store.Chunk(c.ID)
	idA, idB := copyIDIn(t, filepath.Join(moved, "a", c.Name)), copyIDIn(t, filepath.Join(moved, "b", c.Name))
	c = rebuildPackage(t, app, before)
	if _, err := writeOver(t, app, c.ID, filepath.Join(moved, "a"), vol.ID); err != nil {
		t.Fatal(err)
	}
	got := app.Store.Chunk(c.ID)
	for _, cp := range got.Copies {
		switch cp.CopyID {
		case idA:
			if !cp.Superseded {
				t.Fatalf("the overwritten copy is still current: %+v", cp)
			}
		case idB:
			if cp.Superseded || !verified(cp) {
				t.Fatalf("the intact copy in the other folder lost its standing: %+v", cp)
			}
		}
	}
	if got.VerifiedCopyCount() != 2 {
		t.Fatalf("want the intact old copy and the new one, got %d: %+v", got.VerifiedCopyCount(), got.Copies)
	}
}

// Rewriting the current build into one folder leaves an older build's intact copy in
// another folder its credit.
func TestCopyID_RewriteKeepsAnOlderBuildsCredit(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, "OLDER-CREDIT", false)
	root := t.TempDir()
	moved := remountedDrive(t, app, vol, root, func(drive string) {
		mkdirs(t, filepath.Join(drive, "old"), filepath.Join(drive, "new"))
		if _, err := app.WriteChunk(c.ID, filepath.Join(drive, "old"), 0, 0, 0, vol.ID, noProg); err != nil {
			t.Fatal(err)
		}
		c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
		if _, err := app.WriteChunk(c.ID, filepath.Join(drive, "new"), 0, 0, 0, vol.ID, noProg); err != nil {
			t.Fatal(err)
		}
	})
	oldID := copyIDIn(t, filepath.Join(moved, "old", c.Name))
	if _, err := writeOver(t, app, c.ID, filepath.Join(moved, "new"), vol.ID); err != nil {
		t.Fatal(err)
	}
	got := app.Store.Chunk(c.ID)
	for _, cp := range got.Copies {
		if cp.CopyID == oldID && (cp.Superseded || !verified(cp)) {
			t.Fatalf("the older build's intact copy lost its credit: %+v", cp)
		}
	}
	if got.CurrentCopyCount() != 2 || got.VerifiedCopyCount() != 2 {
		t.Fatalf("current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
	}
}

// The same set written again after its tapes moved is one copy, even when another
// medium now sits at an old tape's mount point.
func TestCopyID_SameSetRewrittenAfterARemountIsOneCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := app.Store.AddVolume(Volume{Label: "ONE-SET-ID", Kind: "TAPE"})
	c, _ := privateChunk(t, app, "ONE-SET-ID", true)
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	mkdirs(t, first)
	tapes := writeSpanSet(t, app, c.ID, first, "t", vol.ID)
	setID := copyIDIn(t, filepath.Join(tapes[0], c.Name))
	for _, tape := range tapes {
		if got := copyIDIn(t, filepath.Join(tape, c.Name)); got != setID {
			t.Fatalf("tapes of one set hold different IDs: %s and %s", setID, got)
		}
	}
	c = app.Store.Chunk(c.ID)
	resetSegments(t, app, c)
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(first, "t00", c.Name) // another medium, mounted where the first tape was
	mkdirs(t, other)
	if err := os.WriteFile(filepath.Join(other, "unrelated.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSpanSetOver(t, app, c.ID, second, "t", vol.ID)
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 1 {
		t.Fatalf("one physical set, but current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
	}
	if id := copyIDIn(t, filepath.Join(second, "t00", c.Name)); id == setID {
		t.Fatal("the second write of the set kept the first write's ID")
	}
}

// Checking the folder of a retired build, whole or damaged, credits nothing, blames
// no current copy and moves no copy's record.
func TestCopyID_CheckingARetiredBuildTouchesNoCurrentCopy(t *testing.T) {
	for _, damaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("damaged=%v", damaged), func(t *testing.T) {
			app, _ := newTestApp(t, nativeTools(t))
			vol := remountable(t, app)
			c, _ := privateChunk(t, app, "RETIRED-CHECK", false)
			root := t.TempDir()
			moved := remountedDrive(t, app, vol, root, func(drive string) {
				x := filepath.Join(drive, "x")
				mkdirs(t, x)
				if _, err := app.WriteChunk(c.ID, x, 0, 0, 0, vol.ID, noProg); err != nil {
					t.Fatal(err)
				}
				mkdirs(t, filepath.Join(drive, "kept", c.Name))
				if err := copyFolderInto(filepath.Join(x, c.Name), filepath.Join(drive, "kept", c.Name)); err != nil {
					t.Fatal(err)
				}
				c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
				if _, err := writeOver(t, app, c.ID, x, vol.ID); err != nil {
					t.Fatal(err)
				}
			})
			before := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(root, "drive", "x", c.Name))
			if damaged {
				flipByte(t, filepath.Join(moved, "kept", c.Name, payloadName(c)))
			}
			r, err := app.VerifyChunk(c.ID, filepath.Join(moved, "kept"))
			if err != nil {
				t.Fatal(err)
			}
			if r["credited"] != 0 {
				t.Fatalf("a retired build's folder earned credit: %v", r)
			}
			got := app.Store.Chunk(c.ID)
			cur := currentCopyAt(t, got, filepath.Join(root, "drive", "x", c.Name))
			if !verified(cur) || cur.Path != before.Path {
				t.Fatalf("the current copy was judged or moved by a check of another folder: %+v", cur)
			}
		})
	}
}

// A bracket in a mount path does not hide a tape's old contents.
func TestCopyID_PriorBuildReadsAFolderWithBrackets(t *testing.T) {
	c := &Chunk{Name: "BRACKETS"}
	dir := filepath.Join(t.TempDir(), "tape [1]", c.Name)
	mkdirs(t, dir)
	if err := os.WriteFile(filepath.Join(dir, c.Name+".seg001"), []byte("segment bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, c.Name+".tar.vol000+01.par2"), []byte("par2"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := priorBuildAt(c, dir)
	if !p.Present || len(p.SegHashes) != 1 {
		t.Fatalf("the tape's segment was not seen: %+v", p)
	}
}

// While a new set overwrites the only verified set, the package is not shown as
// verified.
func TestCopyID_SpanOverwriteShowsNoVerifiedPackage(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := app.Store.AddVolume(Volume{Label: "ONLY-SET", Kind: "TAPE"})
	c, _ := privateChunk(t, app, "ONLY-SET", true)
	root := t.TempDir()
	tapes := writeSpanSet(t, app, c.ID, root, "t", vol.ID)
	c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
	if _, err := spanWriteOver(t, app, c.ID, tapes[0], vol.ID); err != nil {
		t.Fatal(err)
	}
	got := app.Store.Chunk(c.ID)
	if got.VerifiedCopyCount() != 0 {
		t.Fatalf("the overwritten set still counts: %+v", got.Copies)
	}
	if got.Status == "VERIFIED" {
		t.Fatal("the package shows VERIFIED with no verified copy")
	}
}

// A copy whose ID file is gone, or cannot be read, earns no credit however well its
// bytes check, and says why.
func TestCopyID_MissingOrUnreadableIDEarnsNoCredit(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		spoil      func(t *testing.T, p string)
	}{
		{"missing", "missing", func(t *testing.T, p string) {
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
		}},
		{"unreadable", "unreadable", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("obelisk-copy-id 9\nnot an id\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"another ID", "mismatch", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("obelisk-copy-id 1\n"+strings.Repeat("ab", 16)+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := newTestApp(t, nativeTools(t))
			c, medium, _ := copiedPackage(t, app, "SPOILED-ID")
			tc.spoil(t, filepath.Join(medium, c.Name, copyIDFile))
			r, err := app.VerifyChunk(c.ID, medium)
			if err != nil {
				t.Fatal(err)
			}
			if r["credited"] != 0 {
				t.Fatalf("credit with a %s ID: %v", tc.name, r)
			}
			cp := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
			if verified(cp) || cp.Unidentified != tc.want {
				t.Fatalf("want no credit and %q, got %+v", tc.want, cp)
			}
			if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 0 {
				t.Fatal("the package still counts the copy")
			}
			if _, err := os.Stat(filepath.Join(medium, c.Name, copyIDFile)); tc.name == "missing" && err == nil {
				t.Fatal("a check wrote an ID onto the medium")
			}
		})
	}
}

// A drive mounted somewhere else is found by its ID and keeps its one record.
func TestCopyID_MovedCopyIsFoundByItsID(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "MOVED-ID", false)
	root := t.TempDir()
	first := filepath.Join(root, "first")
	mkdirs(t, first)
	if _, err := app.WriteChunk(c.ID, first, 0, 0, 0, app.Store.EnsureUnregistered().ID, noProg); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(root, "second")
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	wantVerifies(t, app, c.ID, second)
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 1 {
		t.Fatalf("current=%d verified=%d", got.CurrentCopyCount(), got.VerifiedCopyCount())
	}
	if cp := currentCopyAt(t, got, filepath.Join(second, c.Name)); cp.Unidentified != "" {
		t.Fatalf("the moved copy is marked %q", cp.Unidentified)
	}
}

// Two media holding one ID are a conflict: neither earns credit until the user says
// which one is the copy.
func TestCopyID_DuplicatedIDIsAConflictTheUserSettles(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "CLONED")
	clone := t.TempDir()
	mkdirs(t, filepath.Join(clone, c.Name))
	if err := copyFolderInto(filepath.Join(medium, c.Name), filepath.Join(clone, c.Name)); err != nil {
		t.Fatal(err)
	}
	r, err := app.VerifyChunk(c.ID, clone)
	if err != nil {
		t.Fatal(err)
	}
	if r["credited"] != 0 {
		t.Fatalf("a clone earned credit: %v", r)
	}
	got := app.Store.Chunk(c.ID)
	cp := currentCopyAt(t, got, filepath.Join(medium, c.Name))
	if cp.Unidentified != "conflict" || verified(cp) || cp.Conflict == nil || normPath(cp.Conflict.Path) != normPath(filepath.Join(clone, c.Name)) {
		t.Fatalf("want a conflict naming the clone, got %+v", cp)
	}
	// Checking the recorded medium does not settle it either.
	if r, err := app.VerifyChunk(c.ID, medium); err != nil || r["credited"] != 0 {
		t.Fatalf("credit while the conflict is open: %v %v", r, err)
	}
	if got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 0 {
		t.Fatalf("current=%d verified=%d", got.CurrentCopyCount(), got.VerifiedCopyCount())
	}

	// The user keeps the recorded medium. It must still be checked to earn credit.
	if err := app.SettleCopyConflict(c.ID, cp.CopyID, "recorded"); err != nil {
		t.Fatalf("settle: %v", err)
	}
	cp = currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
	if cp.Conflict != nil || cp.Unidentified != "unchecked" || verified(cp) {
		t.Fatalf("after settling: %+v", cp)
	}
	wantVerifies(t, app, c.ID, medium)
	// The set-aside clone earns nothing and raises no new conflict.
	if r, err := app.VerifyChunk(c.ID, clone); err != nil || r["credited"] != 0 {
		t.Fatalf("the set-aside clone: %v %v", r, err)
	}
	cp = currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
	if cp.Conflict != nil || !verified(cp) {
		t.Fatalf("the set-aside clone disturbed the copy: %+v", cp)
	}
}

// The user can instead keep the medium that was found, and the record moves there.
func TestCopyID_SettlingCanKeepTheFoundMedium(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "CLONED-KEEP")
	clone := t.TempDir()
	mkdirs(t, filepath.Join(clone, c.Name))
	if err := copyFolderInto(filepath.Join(medium, c.Name), filepath.Join(clone, c.Name)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.VerifyChunk(c.ID, clone); err != nil {
		t.Fatal(err)
	}
	cp := currentCopyAt(t, app.Store.Chunk(c.ID), filepath.Join(medium, c.Name))
	if err := app.SettleCopyConflict(c.ID, cp.CopyID, "found"); err != nil {
		t.Fatalf("settle: %v", err)
	}
	wantVerifies(t, app, c.ID, clone)
	if r, err := app.VerifyChunk(c.ID, medium); err != nil || r["credited"] != 0 {
		t.Fatalf("the set-aside original: %v %v", r, err)
	}
	got := app.Store.Chunk(c.ID)
	if got.CurrentCopyCount() != 1 || got.VerifiedCopyCount() != 1 {
		t.Fatalf("current=%d verified=%d: %+v", got.CurrentCopyCount(), got.VerifiedCopyCount(), got.Copies)
	}
}

// A refill from a copy takes none of its identity: the next write gets a new ID.
func TestCopyID_RefillCarriesNoID(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "REFILL-ID")
	oldID := copyIDIn(t, filepath.Join(medium, c.Name))
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	cp := currentCopyAt(t, c, filepath.Join(medium, c.Name))
	if _, err := app.RefillStaging(c.ID, cp.VolumeID, filepath.Join(medium, c.Name), noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	if _, err := os.Stat(filepath.Join(c.StagedDir, copyIDFile)); err == nil {
		t.Fatal("the refill copied the copy's ID into staging")
	}
	other := t.TempDir()
	if _, err := app.WriteChunk(c.ID, other, 0, 0, 0, cp.VolumeID, noProg); err != nil {
		t.Fatal(err)
	}
	if copyIDIn(t, filepath.Join(other, c.Name)) == oldID {
		t.Fatal("a write from refilled staging reused the source copy's ID")
	}
}

// Adopting a medium takes the ID it holds.
func TestCopyID_AdoptionKeepsTheMediumsID(t *testing.T) {
	src, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, src, "ADOPT-ID")
	id := copyIDIn(t, filepath.Join(medium, c.Name))
	app, _ := newTestApp(t, nativeTools(t))
	coll := app.Store.AddCollection("adopted")
	if _, err := app.AdoptMedia(medium, coll.ID, app.Store.EnsureUnregistered().ID, false, noProg); err != nil {
		t.Fatal(err)
	}
	for _, ch := range app.Store.Chunks(0) {
		if ch.Name == c.Name {
			if cp := currentCopyAt(t, ch, filepath.Join(medium, c.Name)); cp.CopyID != id || !verified(cp) {
				t.Fatalf("adopted copy: %+v", cp)
			}
			return
		}
	}
	t.Fatal("nothing was adopted")
}

// The settle route takes the copy's ID and which medium to keep.
func TestCopyID_SettleRoute(t *testing.T) {
	s := newIT(t)
	code, body := s.do("POST", "/api/chunks/999999/copies/settle", map[string]any{"copy_id": strings.Repeat("ab", 16), "keep": "recorded"})
	if strings.Contains(string(body), "page not found") {
		t.Fatal("no settle route")
	}
	if code < 400 {
		t.Fatalf("settling an unknown package: %d %s", code, body)
	}
}

// The recovery kit names an unidentified copy as such, not as unverified.
func TestCopyID_KitNamesAnUnidentifiedCopy(t *testing.T) {
	for why, want := range map[string]string{idNone: "not identified, verify to confirm", idConflict: "two media hold its copy ID"} {
		if got := copyResult(Copy{Unidentified: why}); !strings.Contains(got, want) {
			t.Fatalf("%s: %q", why, got)
		}
	}
}
