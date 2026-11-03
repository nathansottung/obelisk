//go:build !guionly

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Times far outside what nanoseconds hold — the zero FILETIME of 1601 that Windows
// reports for a file with no date, or a camera clock set to 2300 — are kept
// exactly on both engines, and one such record never stops other changes saving.
func TestCatalogEngine_OutOfRangeTimesAreKept(t *testing.T) {
	old := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2300, 7, 4, 1, 2, 3, 456789, time.UTC)
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			s := openEngineStore(t, engine, dir)
			coll := s.AddCollection("A")
			fo := s.AddFolder(coll.ID, "/src")
			v := s.AddVolume(Volume{Label: "OLD-DRIVE", Kind: "HDD"})
			if err := s.PutVolumeSnapshot(&VolumeSnapshot{VolumeID: v.ID, CapturedAt: time.Now().UTC(), Scan: ScanComplete,
				Files: []SnapFile{{RelPath: "undated.bin", SizeBytes: 1, ModTime: old, Hash: testHash("u")}}}); err != nil {
				t.Fatal(err)
			}
			f, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "undated.bin", SizeBytes: 1,
				HashAlg: "SHA256", Hash: testHash("u"), ModTime: old, ShotAt: future})
			if err != nil {
				t.Fatalf("a file dated 1601 was refused: %v", err)
			}
			after := s.AddCollection("added after")
			if err := s.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			s = openEngineStore(t, engine, dir)
			if s.Collection(after.ID) == nil {
				t.Fatal("a change saved after the old-dated records was lost")
			}
			snapFiles := testSnapFiles(t, s, v.ID)
			if s.VolumeSnapshot(v.ID) == nil || len(snapFiles) != 1 || !snapFiles[0].ModTime.Equal(old) {
				t.Fatalf("snapshot file time: %+v", snapFiles)
			}
			got := noErr[*File](t)(s.FileByID(f.ID))
			if got == nil || !got.ModTime.Equal(old) || !got.ShotAt.Equal(future) {
				t.Fatalf("file times after reopening: %+v", got)
			}
			if n, _, _ := s.FileStats(coll.ID); n != 1 {
				t.Fatalf("the archive holds %d files, want 1", n)
			}
			// A capture-date search treats them like any other dated file.
			if rows := noErr[[]map[string]any](t)(s.Search(SearchQuery{From: time.Date(2299, 1, 1, 0, 0, 0, 0, time.UTC)})); len(rows) != 1 {
				t.Errorf("a search from 2299 found %d files, want the one shot in 2300", len(rows))
			}
			if rows := noErr[[]map[string]any](t)(s.Search(SearchQuery{To: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)})); len(rows) != 0 {
				t.Errorf("a search up to 2000 found %d files, want none", len(rows))
			}
		})
	}
}

// A file write the database refuses is reported by the next save, the batch's end
// and FlushPending, even when the method that failed returns no error.
func TestCatalogEngine_FileWriteFailureIsReported(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	be := s.be.(*dbBackend)
	be.failFileWrite = func() error { return errors.New("disk full") }
	s.BeginBatch()
	s.UpsertFile(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "lost.nef", HashAlg: "SHA256", Hash: testHash("l")})
	be.failFileWrite = nil
	if err := s.EndBatch(); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("EndBatch after a failed file write: %v", err)
	}
	if err := s.FlushPending(); err == nil {
		t.Fatal("FlushPending forgot the failed file write")
	}
}

// A file update that fails partway leaves the file exactly as it was: no
// half-written row, no lost version history.
func TestCatalogEngine_FailedFileUpdateLeavesNothingHalfDone(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	day := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	up := func(content string) (*File, error) {
		return s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "a.jpg", SizeBytes: int64(len(content)),
			HashAlg: "SHA256", Hash: testHash(content), ModTime: day})
	}
	if _, err := up("v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := up("v2"); err != nil { // v1 is now a retained version
		t.Fatal(err)
	}
	s.Flush()
	before := noErr[*File](t)(s.FileByID(1))
	be := s.be.(*dbBackend)
	be.failAfterFileUpdate = func() error { return errors.New("I/O error") }
	if _, err := up("v3"); err == nil {
		t.Fatal("the failed update reported success")
	}
	be.failAfterFileUpdate = nil
	if got := noErr[*File](t)(s.FileByID(1)); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("a failed update changed the file:\nbefore %+v\nafter  %+v", before, got)
	}
}

// A commit that fails keeps the store's records in memory (a caller's record is
// still saved by the next save), and every later save reports that changes were
// lost, so no job reports success for work that is not there.
func TestCatalogEngine_FailedCommitIsReportedAndMemoryKept(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	ch := s.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-1", Status: "PLANNED"})
	be := s.be.(*dbBackend)
	s.batchInterval = time.Hour
	s.Flush()
	s.BeginBatch()
	if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "in-batch.nef", HashAlg: "SHA256",
		Hash: testHash("b")}); err != nil {
		t.Fatal(err)
	}
	be.failCommit = func() error { return errors.New("disk full") }
	err := s.EndBatch()
	be.failCommit = nil
	if err == nil {
		t.Fatal("EndBatch reported success for a failed commit")
	}
	if err := s.FlushPending(); err == nil || !strings.Contains(err.Error(), "lost") {
		t.Fatalf("a later save does not report the lost changes: %v", err)
	}
	// The caller's record is still the store's: changing it and saving lands.
	ch.Status = "STAGED"
	_ = s.UpdateChunkErr(ch)
	if got := persistedCatalog(t, dir); len(got.Chunks) != 1 || got.Chunks[0].Status != "STAGED" {
		t.Fatalf("a record changed after the failed commit was not saved: %+v", got.Chunks)
	}
}

// Lists of file IDs larger than SQLite's limit on statement parameters work.
func TestCatalogEngine_ManyFileIDs(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	const n = 40_000
	ids := make([]int, 0, n)
	s.BeginBatch()
	for i := 0; i < n; i++ {
		f, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: fmt.Sprintf("f%06d", i),
			HashAlg: "SHA256", Hash: testHash(fmt.Sprint(i))})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.ID)
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	ev := s.AddEvent(&Event{Name: "All", CreatedAt: time.Now().UTC()})
	if got := noErr[int](t)(s.AssignFilesToEvent(ids, ev.ID)); got != n {
		t.Fatalf("assigned %d of %d", got, n)
	}
	s.mu.Lock()
	count := 0
	err := s.eachFileLocked(fileQuery{IDs: ids}, func(*File) error { count++; return nil })
	s.mu.Unlock()
	if err != nil || count != n {
		t.Fatalf("a scan by %d IDs found %d (%v)", n, count, err)
	}
}

// Shutting the app down saves what a batch left pending and closes the catalog.
func TestApp_ShutdownSavesAndCloses(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	app := &App{DataDir: dir, Store: s}
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	s.batchInterval = time.Hour
	s.Flush()
	s.BeginBatch()
	if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "pending.nef", HashAlg: "SHA256",
		Hash: testHash("p")}); err != nil {
		t.Fatal(err)
	}
	if err := app.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(persistedCatalogTextFrom(t, dir), "pending.nef") {
		t.Fatal("shutdown did not save the pending file")
	}
	if err := s.FlushPending(); err == nil && !s.closed {
		t.Fatal("the store is still open after shutdown")
	}
}

// A time Go cannot write as RFC 3339 (a year past 9999 or before 0, which a
// damaged file system can report) is refused, as the JSON engine refuses it, and
// never leaves the catalog database unreadable.
func TestCatalogEngine_YearsBeyondRFC3339AreRefused(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	for _, year := range []int{10000, -5} {
		_, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: fmt.Sprint(year), HashAlg: "SHA256",
			Hash: testHash("y"), ModTime: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)})
		if err == nil {
			t.Errorf("a file dated %d was accepted", year)
		}
	}
	far := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	ch := s.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG", Status: "PLANNED"})
	ch.WrittenAt = &far
	if err := s.UpdateChunkErr(ch); err == nil {
		t.Error("a package written in 10000 was saved")
	}
	ch.WrittenAt = nil
	if err := s.UpdateChunkErr(ch); err != nil {
		t.Fatalf("after the bad time was removed: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	if n, _, err := s.FileStats(coll.ID); err != nil || n != 0 {
		t.Fatalf("after reopening: %d files, %v", n, err)
	}
}

// After a commit that lost file writes, saves that land still succeed (so no
// caller rolls back a change the database holds), while FlushPending, EndBatch
// and Close keep reporting the loss.
func TestCatalogEngine_LostChangesDoNotFailSavesThatLand(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	ch := s.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG", Status: "PLANNED"})
	be := s.be.(*dbBackend)
	s.batchInterval = time.Hour
	s.Flush()
	s.BeginBatch()
	if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "x", HashAlg: "SHA256",
		Hash: testHash("x")}); err != nil {
		t.Fatal(err)
	}
	be.failCommit = func() error { return errors.New("database is locked") }
	_ = s.EndBatch()
	be.failCommit = nil
	if err := s.AppendVerifyEventErr(ch, VerifyEvent{At: time.Now().UTC(), OK: true, Note: "after"}); err != nil {
		t.Fatalf("a save that landed reported failure: %v", err)
	}
	if got := persistedCatalog(t, dir); len(got.Chunks) != 1 || len(got.Chunks[0].VerifyEvents) != 1 {
		t.Fatalf("the verify event is not in the database: %+v", got.Chunks)
	}
	if err := s.FlushPending(); err == nil || !strings.Contains(err.Error(), "lost") {
		t.Errorf("FlushPending: %v", err)
	}
	if err := s.Close(); err == nil || !strings.Contains(err.Error(), "lost") {
		t.Errorf("Close: %v", err)
	}
}

// A commit that fails while holding only in-memory records loses nothing: they
// are written again, and nothing is reported as lost.
func TestCatalogEngine_FailedCommitWithoutFileWritesLosesNothing(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	be := s.be.(*dbBackend)
	be.failCommit = func() error { return errors.New("database is locked") }
	c := s.AddCollection("kept in memory")
	be.failCommit = nil
	if err := s.FlushPending(); err != nil {
		t.Fatalf("nothing was lost, yet: %v", err)
	}
	if got := persistedCatalog(t, dir); len(got.Collections) != 1 || got.Collections[0].ID != c.ID {
		t.Fatalf("the record was not written again: %+v", got.Collections)
	}
}
