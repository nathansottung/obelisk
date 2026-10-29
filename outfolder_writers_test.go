//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
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

// A span write decides the mounted medium by where it really is: a folder link
// to a source is refused, nothing is written into the source, and the segment
// stays pending.
func TestSpanWrite_DestinationDecidedByRealLocation(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	src := t.TempDir()
	big := strings.Repeat("spanning-payload-", 2500)
	mustWrite(t, filepath.Join(src, "big.dat"), big)
	c := app.Store.AddChunk(Chunk{
		Name: "SPAN-GUARD", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 12000, DataBytes: int64(len(big)), FileCount: 1,
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Spanned: true,
		Segments: planSegments(int64(len(big))+4096, 12000),
		Files:    []ChunkFileRef{{RelPath: "big.dat", SizeBytes: int64(len(big))}},
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk: %v", err)
	}
	_, link, untouched := sourceBehindLink(t, app)
	_, err := app.SpanWriteNext(c.ID, link, 0, 0, 0, 0, noProg)
	assertSourceRefusal(t, "span write into a link to a source", err)
	untouched()
	if st := app.Store.Chunk(c.ID).Segments[0].Status; st != "PENDING" {
		t.Errorf("refused segment is %s, want PENDING", st)
	}

	// A segment file already on the medium (a hard link to another file) is
	// replaced as a name; the other file keeps its bytes.
	medium := t.TempDir()
	victim := plantAtStageName(t, filepath.Join(medium, c.Name, c.Name+".seg001"))
	if _, err := app.SpanWriteNext(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, victim, "SOMEONE ELSE'S")
}

// An escrow bundle stages each of its files and names it once it reads back,
// so a file already in the bundle folder (a hard link to another file) is
// replaced as a name and the other file keeps its bytes.
func TestEscrowBundle_NeverWritesThroughExistingFiles(t *testing.T) {
	a := escrowApp(t)
	seedEscrowCache(t, a, struct{ binaries, toolchain, readers bool }{binaries: true})
	dir := t.TempDir()
	readme := plantAtStageName(t, filepath.Join(dir, escrowBundleDir, "ESCROW_README.md"))
	plan := a.planEscrow(EscrowBinariesOnly, false, Census{}, mustConfig(t, a))
	if _, err := a.WriteEscrowBundle(dir, plan, nil); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, readme, "SOMEONE ELSE'S")
	if b, err := os.ReadFile(filepath.Join(dir, escrowBundleDir, "ESCROW_README.md")); err != nil || len(b) == 0 || string(b) == "SOMEONE ELSE'S" {
		t.Errorf("bundle README not written: %q %v", b, err)
	}
}
