//go:build !guionly

package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What an earlier run left in staging is listed with its size and age, is not
// removed by a build, and goes only through the delete gate with "Delete".
func TestLeftovers_ListedAndRemovedOnlyThroughTheGate(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	tree := filepath.Join(staging, ".PKG.build-crashed")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tree, "PKG.tar"), "half a tar")
	userFile := filepath.Join(staging, "notes.txt")
	mustWrite(t, userFile, "the user's own file")

	builtTestChunk(t, app, "OTHER") // a build in the same staging folder
	if _, err := os.Stat(tree); err != nil {
		t.Fatal("a build removed what an earlier run left in staging")
	}
	var found *Leftover
	for _, l := range app.Leftovers() {
		if l.Path == tree {
			l := l
			found = &l
		}
		if l.Path == userFile {
			t.Error("a file of the user's is listed as a leftover")
		}
	}
	if found == nil || found.Size != int64(len("half a tar")) || found.Kind == "" {
		t.Fatalf("the leftover is not listed with its size: %+v", found)
	}

	p := prepareForTest(t, app, "leftover-remove", map[string]any{"path": tree})
	if _, err := app.ExecuteDestructive(testSession, p.Token, "delete"); err == nil {
		t.Fatal("removed without the exact word")
	}
	scratchKept(t, tree, "a leftover after a wrong word")
	p = prepareForTest(t, app, "leftover-remove", map[string]any{"path": tree})
	if _, err := app.ExecuteDestructive(testSession, p.Token, "Delete"); err != nil {
		t.Fatalf("removing a listed leftover: %v", err)
	}
	scratchGone(t, tree, "a leftover after Delete")

	if _, err := app.PrepareDestructive(testSession, "leftover-remove", map[string]any{"path": userFile}); err == nil {
		t.Error("the gate offered to remove a file that is not a listed leftover")
	}
	scratchKept(t, userFile, "the user's file")
}

// An export never replaces a file already in the folder.
func TestOutFolder_NeverReplacesAnExistingFile(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	dir := t.TempDir()
	existing := filepath.Join(dir, "REPORT.md")
	mustWrite(t, existing, "the user's own report")
	out, err := app.openOutFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	err = out.WriteFile("REPORT.md", []byte("new"), 0o644)
	if err == nil || !errors.Is(err, fs.ErrExist) {
		t.Fatalf("an export over an existing file: %v; want a refusal", err)
	}
	if got := mustRead(t, existing); got != "the user's own report" {
		t.Errorf("the existing file was changed: %q", got)
	}
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("the refused export left %s", e.Name())
		}
	}
}

// copyFile never replaces a file already at the destination.
func TestCopyFile_NeverReplaces(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	mustWrite(t, src, "new")
	mustWrite(t, dst, "keep")
	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile replaced an existing file")
	}
	if got := mustRead(t, dst); got != "keep" {
		t.Errorf("the existing file was changed: %q", got)
	}
}

// fetchEscrowFromBytes is a download of data to dest, as fetchEscrowArtifact
// publishes it.
func fetchEscrowFromBytes(dest string, data []byte) error {
	tmp, err := writeStage(dest, data, 0o644)
	if err != nil {
		return err
	}
	return publishEscrowDownload(tmp, dest, sha256Hex(data))
}

// The tool cache replaces only a file it downloaded itself.
func TestEscrowCache_ReplacesOnlyItsOwnDownloads(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "par2.exe")
	mustWrite(t, dest, "a file the user put here")
	stage, err := createStage(dest, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	stage.Close()
	// Not recorded as a download: refused, and left as it is.
	downloads := readEscrowDownloads(dir)
	if _, ok := downloads["par2.exe"]; ok {
		t.Fatal("setup: recorded")
	}
	_ = removeStage(stage.Name())
	if err := fetchEscrowFromBytes(dest, []byte("downloaded")); err == nil {
		t.Error("the cache replaced a file it did not download")
	}
	if got := mustRead(t, dest); got != "a file the user put here" {
		t.Errorf("the user's file was changed: %q", got)
	}
	// Its own download is replaced by the next one.
	other := filepath.Join(dir, "gpg.exe")
	if err := fetchEscrowFromBytes(other, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := fetchEscrowFromBytes(other, []byte("v2")); err != nil {
		t.Fatalf("the cache could not replace its own download: %v", err)
	}
	if got := mustRead(t, other); got != "v2" {
		t.Errorf("got %q", got)
	}
}

// A restore into a new folder that fails removes only what it unpacked: a file
// someone put there while it ran stays, and so does the folder.
func TestRestoreCleanup_RemovesOnlyWhatItMade(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "restored")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	made := map[string]fileIdent{}
	if err := mkdirAllRecording(r, "a/b", made); err != nil {
		t.Fatal(err)
	}
	f, err := r.OpenFile("a/b/c.txt", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := handleIdentity(f)
	f.Close()
	made["a/b/c.txt"] = id
	mustWrite(t, filepath.Join(dir, "a", "dropped.txt"), "someone else's")

	left, err := removeMadeTree(r, made)
	if err != nil {
		t.Fatal(err)
	}
	scratchGone(t, filepath.Join(dir, "a", "b"), "what the restore made")
	scratchKept(t, filepath.Join(dir, "a", "dropped.txt"), "a file the restore did not make")
	if len(left) != 1 || left[0] != "a" {
		t.Errorf("left %v; want the folder holding the other file", left)
	}
}
