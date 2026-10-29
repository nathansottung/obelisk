//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// driftSetup scans files into an archive and records them all as packaged.
func driftSetup(t *testing.T, files map[string]string) (*App, *Collection, string) {
	t.Helper()
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, files)
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	var refs []ChunkFileRef
	for _, f := range app.Store.FilesOf(coll.ID) {
		refs = append(refs, ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: f.SizeBytes, Hash: f.Hash})
	}
	app.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-1", Status: "VERIFIED", Files: refs, FileCount: len(refs)})
	return app, coll, src
}

func driftStates(rep *DriftReport) []string {
	var out []string
	for _, it := range rep.Items {
		s := it.State + ":" + it.Path
		if it.MovedFrom != "" {
			s += "<" + it.MovedFrom
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Drift reports a moved file as MOVED (from where), a deleted one as MISSING,
// and one copy of a duplicated file removed as MISSING while the other stays
// unchanged.
func TestDrift_MovedAndMissing(t *testing.T) {
	app, coll, src := driftSetup(t, map[string]string{
		"a.txt": "alpha\n", "b.txt": "bravo\n", "d1/x.jpg": "same\n", "d2/x.jpg": "same\n",
	})
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(src, "a.txt"), filepath.Join(src, "sub", "a.txt")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"b.txt", "d2/x.jpg"} {
		if err := os.Remove(filepath.Join(src, filepath.FromSlash(p))); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := app.ReconcileCollection(coll.ID, "", noProg)
	if err != nil {
		t.Fatal(err)
	}
	got := driftStates(rep)
	want := []string{"MISSING:b.txt", "MISSING:d2/x.jpg", "MOVED:sub/a.txt<a.txt"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("drift items %v, want %v", got, want)
	}
	if rep.Counts["unchanged"] != 1 || rep.Counts["moved"] != 1 || rep.Counts["missing"] != 2 {
		t.Errorf("counts %v", rep.Counts)
	}
}

// A packaged file in a folder the rescan cannot read is UNKNOWN, never MISSING.
func TestDrift_UnreadableFolderIsUnknown(t *testing.T) {
	app, coll, src := driftSetup(t, map[string]string{"a.txt": "alpha\n", "locked/b.txt": "bravo\n"})
	makeUnlistable(t, filepath.Join(src, "locked"))
	rep, err := app.ReconcileCollection(coll.ID, "", noProg)
	if err != nil {
		t.Fatal(err)
	}
	if got := driftStates(rep); len(got) != 1 || got[0] != "UNKNOWN:locked/b.txt" {
		t.Errorf("drift items %v, want locked/b.txt UNKNOWN", got)
	}
	if rep.Counts["missing"] != 0 || rep.Counts["unknown"] != 1 {
		t.Errorf("counts %v", rep.Counts)
	}
}

// Where the platform ignores letter case, a file renamed only in case is
// unchanged; elsewhere it moved.
func TestDrift_CaseOnlyRename(t *testing.T) {
	app, coll, src := driftSetup(t, map[string]string{"photo.jpg": "p\n"})
	if err := os.Rename(filepath.Join(src, "photo.jpg"), filepath.Join(src, "PHOTO.JPG")); err != nil {
		t.Fatal(err)
	}
	rep, err := app.ReconcileCollection(coll.ID, "", noProg)
	if err != nil {
		t.Fatal(err)
	}
	if hostFoldsCase() {
		if rep.Counts["unchanged"] != 1 || len(rep.Items) != 0 {
			t.Errorf("a case-only rename: %v %v, want unchanged", driftStates(rep), rep.Counts)
		}
	} else if got := driftStates(rep); len(got) != 1 || got[0] != "MOVED:PHOTO.JPG<photo.jpg" {
		t.Errorf("a case-only rename: %v, want MOVED", got)
	}
}
