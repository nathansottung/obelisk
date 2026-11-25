//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ownTree makes dir/name holding a few files.
func ownTree(t *testing.T, dir, name string, files ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		fp := filepath.Join(p, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, fp, "scratch "+f)
	}
	return p
}

func scratchKept(t *testing.T, p, why string) {
	t.Helper()
	if _, err := os.Lstat(p); err != nil {
		t.Errorf("%s: %s is gone: %v", why, p, err)
	}
}

func scratchGone(t *testing.T, p, why string) {
	t.Helper()
	if _, err := os.Lstat(p); err == nil {
		t.Errorf("%s: %s is still there", why, p)
	}
}

// A folder link inside a scratch folder that points outside is removed as a
// link: what it points at is never entered or removed.
func TestScratch_LinkInsideIsNotFollowed(t *testing.T) {
	owned, outside := t.TempDir(), t.TempDir()
	keep := filepath.Join(outside, "keep.txt")
	mustWrite(t, keep, "the user's file")
	tree := ownTree(t, owned, ".PKG.build-1", "a.tar", "sub/b.par2")
	mustDirLink(t, filepath.Join(tree, "sub", "out"), outside)

	if err := removeOwned(nil, []string{owned}, owned, ".PKG.build-1", kindBuild); err != nil {
		t.Fatalf("removing an own build folder: %v", err)
	}
	scratchGone(t, tree, "the build folder")
	scratchKept(t, keep, "a file behind a link inside the scratch folder")
}

// A scratch name that is itself a link to somewhere else is refused whole.
func TestScratch_LinkAtTheNameIsRefused(t *testing.T) {
	owned, outside := t.TempDir(), t.TempDir()
	keep := filepath.Join(outside, "keep.txt")
	mustWrite(t, keep, "the user's file")
	mustDirLink(t, filepath.Join(owned, ".PKG.build-2"), outside)

	if err := removeOwned(nil, []string{owned}, owned, ".PKG.build-2", kindBuild); err == nil {
		t.Error("a link with a scratch name was removed")
	}
	scratchKept(t, keep, "a file behind a scratch-named link")
}

// A folder under a source root is never Obelisk's own, even when a caller names
// it as one.
func TestScratch_PathUnderASourceRootIsRefused(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	src := t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Photos").ID, src)
	tree := ownTree(t, src, ".PKG.build-3", "a.tar")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(src, ".x.123.tmp")
	mustWrite(t, stage, "x")

	if err := removeOwned(app.Store, []string{src}, src, ".PKG.build-3", kindBuild); err == nil {
		t.Error("a folder inside a source root was removed")
	}
	sub := filepath.Join(src, "inner")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sub, ".x.1.tmp"), "x")
	if err := removeOwned(app.Store, []string{sub}, sub, ".x.1.tmp", kindStage); err == nil {
		t.Error("a file in a folder under a source root was removed")
	}
	scratchKept(t, tree, "the folder under a source root")
	scratchKept(t, stage, "the file under a source root")
}

// Only Obelisk's own folders, and only the kinds of file it makes there.
func TestScratch_RefusesOtherFoldersAndOtherKinds(t *testing.T) {
	owned, other := t.TempDir(), t.TempDir()
	mustWrite(t, filepath.Join(other, ".x.1.tmp"), "x")
	mustWrite(t, filepath.Join(owned, "holiday.jpg"), "a photo")
	mustWrite(t, filepath.Join(owned, ".x.2.tmp"), "x")

	if err := removeOwned(nil, []string{owned}, other, ".x.1.tmp", kindStage); err == nil {
		t.Error("a file in a folder Obelisk does not own was removed")
	}
	if err := removeOwned(nil, []string{owned}, owned, "holiday.jpg", kindStage); err == nil {
		t.Error("a file Obelisk never makes was removed")
	}
	for _, bad := range []string{"../x", `..\x`, "a/b", ""} {
		if err := removeOwned(nil, []string{owned}, owned, bad, kindStage); err == nil {
			t.Errorf("name %q was accepted", bad)
		}
	}
	scratchKept(t, filepath.Join(other, ".x.1.tmp"), "a file in another folder")
	scratchKept(t, filepath.Join(owned, "holiday.jpg"), "a file of another kind")
	// The same folder spelled another way is still the same folder.
	if err := removeOwned(nil, []string{owned}, filepath.Join(owned, "sub", ".."), ".x.2.tmp", kindStage); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the owned folder spelled differently was refused: %v", err)
	}
}

// The owned folder is compared by what it is, not by its name: a folder put in
// its place is not owned.
func TestScratch_OwnedFolderComparedByIdentity(t *testing.T) {
	base := t.TempDir()
	owned := filepath.Join(base, "work")
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatal(err)
	}
	ownedID, err := ownFolderIdentity(owned)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(owned, owned+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(owned, ".x.1.tmp"), "someone else's")
	if err := removeOwnedByID(nil, []fileIdent{ownedID}, owned, ".x.1.tmp", kindStage); err == nil {
		t.Error("a folder put where the owned folder was is treated as owned")
	}
	scratchKept(t, filepath.Join(owned, ".x.1.tmp"), "a file in the replacement folder")
}

// Something this operation made is removed only while it is still the object it
// made; one put under its name since is left alone.
func TestScratch_MadeEntryReplacedIsRefused(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".x.*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	m, err := madeFile(f)
	if err != nil {
		t.Fatal(err)
	}
	// Off Windows a filesystem may give a freed inode number to the next new
	// file at once; the original stays open while it is replaced, so the
	// replacement is a different object, as it would be for anyone else's file.
	if runtime.GOOS == "windows" {
		f.Close()
	} else {
		defer f.Close()
	}
	if err := os.Remove(f.Name()); err != nil { // test setup: replace it
		t.Fatal(err)
	}
	mustWrite(t, f.Name(), "a different file")
	if err := m.Remove(); err == nil {
		t.Error("a file put under the made file's name was removed")
	}
	scratchKept(t, f.Name(), "the replacement")

	g, err := os.CreateTemp(dir, ".y.*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	m2, err := madeFile(g)
	g.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := m2.Remove(); err != nil {
		t.Fatalf("removing what this operation made: %v", err)
	}
	scratchGone(t, g.Name(), "the made file")
	if err := m2.Remove(); err != nil {
		t.Errorf("a second remove of a gone file: %v", err)
	}
}

// A cleanup that stops part-way (a crash) never leaves a half-removed folder
// under a name the app uses: the folder is renamed out of the way first, and
// what is left is a leftover listed for the user.
func TestScratch_CrashPartWayLeavesNothingHalfRemovedUnderItsName(t *testing.T) {
	owned := t.TempDir()
	tree := ownTree(t, owned, ".PKG.previous-1", "a.tar", "b.par2", "c.par2", "d/e.txt")
	n := 0
	scratchRemoveHook = func(string) error {
		if n++; n == 2 {
			return errors.New("crash")
		}
		return nil
	}
	t.Cleanup(func() { scratchRemoveHook = nil })
	if err := removeOwned(nil, []string{owned}, owned, ".PKG.previous-1", kindPrevious); err == nil {
		t.Fatal("setup: the injected crash did not stop the cleanup")
	}
	scratchGone(t, tree, "a half-removed folder under its own name")
	left, err := listLeftoversIn(owned)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || !strings.HasPrefix(left[0].Name, removingPrefix) {
		t.Fatalf("the interrupted cleanup is not listed as a leftover: %+v", left)
	}
	if left[0].Size == 0 {
		t.Error("the leftover's size is not shown")
	}
}
