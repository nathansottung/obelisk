//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A copy by path writes into a new staging file it creates exclusively and then
// gives it its name only if nothing is there. A file already at that name, here
// a hard link to someone else's file, is never replaced or written through.
func TestPathCopies_NeverWriteThroughAnExistingFile(t *testing.T) {
	newSrc := func(t *testing.T, body string) string {
		p := filepath.Join(t.TempDir(), "new.bin")
		mustWrite(t, p, body)
		return p
	}
	t.Run("copyFile", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "sidecar.par2")
		victim := plantAtStageName(t, dst)
		if err := copyFile(newSrc(t, "NEW SIDECAR"), dst); err == nil {
			t.Fatal("copyFile replaced a file already at its target")
		}
		requireUntouched(t, victim, dst)
	})
	t.Run("ringCopy", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "payload.tar")
		victim := plantAtStageName(t, dst)
		if _, _, err := ringCopy(newSrc(t, "NEW PAYLOAD"), dst, 0, 0, 1, 0.01, 0, func(int64, int64) {}, nil); err == nil {
			t.Fatal("ringCopy replaced a file already at its target")
		}
		requireUntouched(t, victim, dst)
	})
	t.Run("rejoin", func(t *testing.T) {
		segs := t.TempDir()
		mustWrite(t, filepath.Join(segs, "PKG.seg001"), "JOINED")
		out := t.TempDir()
		victim := plantAtStageName(t, filepath.Join(out, "PKG.tar"))
		c := &Chunk{Name: "PKG", Spanned: true, Segments: []Segment{{Index: 1}}}
		if _, _, err := rejoinSegments(segs, out, c, noProg); err == nil {
			t.Fatal("rejoin replaced a file already at its target")
		}
		requireUntouched(t, victim, filepath.Join(out, "PKG.tar"))
	})
	t.Run("no staging file left", func(t *testing.T) {
		dir := t.TempDir()
		if err := copyFile(newSrc(t, "X"), filepath.Join(dir, "a")); err != nil {
			t.Fatal(err)
		}
		if got := treeOf(t, dir); !equalStrings(got, []string{"a"}) {
			t.Errorf("folder holds %v, want only the copy", got)
		}
		if _, err := os.Stat(filepath.Join(dir, "a")); err != nil {
			t.Fatal(err)
		}
	})
}

// The mirror's path copy stages too, and never replaces a file already at the
// target: neither the file nor what it is linked to changes.
func TestMirrorCopyFile_NeverWritesThroughAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	mustWrite(t, src, "NEW")
	dst := filepath.Join(dir, "dst.bin")
	victim := plantAtStageName(t, dst)
	if _, _, err := mirrorCopyFile(src, dst, nil, nil); err == nil {
		t.Fatal("a copy replaced a file already at its target")
	}
	requireUntouched(t, victim, dst)
}
