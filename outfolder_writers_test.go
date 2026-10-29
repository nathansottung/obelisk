//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// sourceBehindLink registers a source holding one file and returns the source,
// a folder link to it, and a check that the source still holds only that file.
func sourceBehindLink(t *testing.T, app *App) (src, link string, untouched func()) {
	t.Helper()
	src = filepath.Join(t.TempDir(), "photos")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	link = filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, src)
	return src, link, func() {
		t.Helper()
		if got := treeOf(t, src); !equalStrings(got, []string{"orig.nef"}) {
			t.Errorf("the source was written into: %v", got)
		}
	}
}

// builtTestChunk builds a small plain package and returns it.
func builtTestChunk(t *testing.T, app *App, name string) *Chunk {
	t.Helper()
	src, refs := makeSource(t)
	c := app.Store.AddChunk(Chunk{
		Name: name, Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk: %v", err)
	}
	return app.Store.Chunk(c.ID)
}

// A package write, and a re-write of a recorded copy, decide the destination by
// where it really is: a folder link to a source is refused and nothing is
// written into the source.
func TestWriteChunk_DestinationDecidedByRealLocation(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "WRITE-GUARD")
	_, link, untouched := sourceBehindLink(t, app)

	_, err := app.WriteChunk(c.ID, link, 0, 0, 0, 0, noProg)
	assertSourceRefusal(t, "write into a link to a source", err)
	untouched()

	t.Run("rewrite", func(t *testing.T) {
		medium := t.TempDir()
		vol := app.Store.AddVolume(Volume{Label: "HDD-01", Kind: "HDD"})
		if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, vol.ID, noProg); err != nil {
			t.Fatal(err)
		}
		// The medium's folder is now a link to the source.
		if err := os.RemoveAll(medium); err != nil {
			t.Fatal(err)
		}
		_, link2, untouched2 := sourceBehindLink(t, app)
		mustDirLink(t, medium, link2)
		_, err := app.RewriteCopy(c.ID, vol.ID, 0, 0, 0, noProg)
		assertSourceRefusal(t, "re-write into a link to a source", err)
		untouched2()
	})
}

// A package write gives each file its name only after it reads back as the
// staged bytes, so a file already at that name (here a hard link to another
// file) is replaced as a name and the other file keeps its bytes.
func TestWriteChunk_NeverWritesThroughExistingFiles(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "WRITE-LINKS")
	medium := t.TempDir()
	payload := plantAtStageName(t, filepath.Join(medium, c.Name, payloadName(c)))
	sidecar := filepath.Join(t.TempDir(), "other.txt")
	mustWrite(t, sidecar, "OTHER")
	if err := os.Link(sidecar, filepath.Join(medium, c.Name, "RESTORE.txt")); err != nil {
		t.Fatal(err)
	}
	res, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := res["verify_ok"].(bool); !ok {
		t.Fatalf("write did not verify: %v", res)
	}
	assertFileBytes(t, payload, "SOMEONE ELSE'S")
	assertFileBytes(t, sidecar, "OTHER")
}
