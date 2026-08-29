package main

import (
	"os"
	"path/filepath"
	"testing"
)

func removalApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	return &App{DataDir: dir, Store: st, Perf: NewPerfMeter()}
}

// TestRetireRoundTrip exercises both retire tiers and unretire (all reversible).
func TestRetireRoundTrip(t *testing.T) {
	a := removalApp(t)
	c := a.Store.AddCollection("Project X")

	if err := a.Store.SetCollectionRetired(c.ID, true, false); err != nil {
		t.Fatalf("retire soft: %v", err)
	}
	got := a.Store.Collection(c.ID)
	if !got.Retired || got.RetireHidden {
		t.Fatalf("soft retire flags wrong: %+v", got)
	}
	if got.RetiredAt == nil {
		t.Fatal("RetiredAt should be stamped on retire")
	}

	if err := a.Store.SetCollectionRetired(c.ID, true, true); err != nil {
		t.Fatalf("retire hidden: %v", err)
	}
	if got = a.Store.Collection(c.ID); !got.Retired || !got.RetireHidden {
		t.Fatalf("hidden retire flags wrong: %+v", got)
	}

	if err := a.Store.SetCollectionRetired(c.ID, false, false); err != nil {
		t.Fatalf("unretire: %v", err)
	}
	if got = a.Store.Collection(c.ID); got.Retired || got.RetireHidden || got.RetiredAt != nil {
		t.Fatalf("unretire should clear every flag: %+v", got)
	}
	if got.IsRetired() {
		t.Fatal("IsRetired should be false after unretire")
	}
}

// TestRemoveEmptyArchive: a test/mistake archive (no packages) removes with just the
// typed name; its name is then reusable.
func TestRemoveEmptyArchive(t *testing.T) {
	a := removalApp(t)
	c := a.Store.AddCollection("Scratch")
	fo := a.Store.AddFolder(c.ID, "/src")
	a.Store.UpsertFile(File{CollectionID: c.ID, FolderID: fo.ID, RelPath: "a.txt", SizeBytes: 10, HashAlg: "SHA256", Hash: "h1"})

	if _, err := a.RemoveArchive(c.ID, "wrong name", ""); err == nil {
		t.Fatal("typed-name guard should refuse a mismatched name")
	}
	counts, err := a.RemoveArchive(c.ID, "Scratch", "")
	if err != nil {
		t.Fatalf("remove empty archive: %v", err)
	}
	if counts.Files != 1 || counts.Folders != 1 || counts.Packages != 0 {
		t.Fatalf("counts wrong: %+v", counts)
	}
	if a.Store.Collection(c.ID) != nil {
		t.Fatal("collection should be gone")
	}
	if len(a.Store.FilesOf(c.ID)) != 0 {
		t.Fatal("file records should be gone")
	}
	// Name is reusable (a fresh archive gets a new id, monotonic).
	c2 := a.Store.AddCollection("Scratch")
	if c2.ID == c.ID {
		t.Fatalf("removed archive's id must not be reused (got %d again)", c2.ID)
	}
}

// TestRemoveGuardedKeepsVolumes: an archive with a package on a tape is refused without
// export + typed name, succeeds with both, and leaves the volume + package/copy rows
// intact (so the volume view still lists it as "from a removed archive").
func TestRemoveGuardedKeepsVolumes(t *testing.T) {
	a := removalApp(t)
	c := a.Store.AddCollection("Client Work")
	fo := a.Store.AddFolder(c.ID, "/src")
	a.Store.UpsertFile(File{CollectionID: c.ID, FolderID: fo.ID, RelPath: "a.nef", SizeBytes: 100, HashAlg: "SHA256", Hash: "hh"})
	vol := a.Store.AddVolume(Volume{Label: "TAPE-01", Kind: "TAPE", Serial: "S1"})
	ch := a.Store.AddChunk(Chunk{Name: "PKG-1", Status: "VERIFIED", CollectionID: c.ID, MediaKind: "TAPE",
		FileCount: 1, Files: []ChunkFileRef{{FileID: 1, RelPath: "a.nef", SizeBytes: 100, Hash: "hh"}}})
	a.Store.RecordCopy(ch, vol.ID, "T:/PKG-1", true)

	if _, err := a.RemoveArchive(c.ID, "Client Work", ""); err == nil {
		t.Fatal("guarded remove must require a fresh Structure Export")
	}
	if _, err := a.RemoveArchive(c.ID, "nope", t.TempDir()); err == nil {
		t.Fatal("guarded remove must still require the typed name")
	}

	dir := t.TempDir()
	counts, err := a.RemoveArchive(c.ID, "Client Work", dir)
	if err != nil {
		t.Fatalf("guarded remove with both guards met: %v", err)
	}
	if counts.Packages != 1 || counts.Volumes != 1 {
		t.Fatalf("counts wrong: %+v", counts)
	}

	// The fresh export (json + csv + md) was written to the chosen folder.
	ents, _ := os.ReadDir(dir)
	var haveJSON, haveCSV, haveMD bool
	for _, e := range ents {
		switch filepath.Ext(e.Name()) {
		case ".json":
			haveJSON = true
		case ".csv":
			haveCSV = true
		case ".md":
			haveMD = true
		}
	}
	if !(haveJSON && haveCSV && haveMD) {
		t.Fatalf("expected json+csv+md structure export in %s, got %v", dir, ents)
	}

	// Archive gone; volume + package/copy rows survive.
	if a.Store.Collection(c.ID) != nil {
		t.Fatal("collection should be gone after remove")
	}
	if len(a.Store.Volumes()) != 1 {
		t.Fatal("volume records must survive removal")
	}
	kept := a.Store.Chunk(ch.ID)
	if kept == nil {
		t.Fatal("package row must survive (it becomes 'from a removed archive')")
	}
	if len(kept.Copies) != 1 || kept.Copies[0].VolumeID != vol.ID {
		t.Fatalf("the copy on the volume must survive: %+v", kept.Copies)
	}

	// Reload from disk: the removal persisted, and the kept records reload cleanly.
	st2, err := OpenStore(a.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if st2.Collection(c.ID) != nil {
		t.Fatal("removed collection must not resurrect on reload")
	}
	if len(st2.Volumes()) != 1 || st2.Chunk(ch.ID) == nil {
		t.Fatal("volume + kept package must persist across a store reload")
	}
}
