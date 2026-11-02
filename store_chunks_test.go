//go:build !guionly

package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

// listingFixture is an archive with three files and a volume, on the named engine.
type listingFixture struct {
	s     *Store
	dir   string
	arch  *Collection
	files []*File
	vol   *Volume
}

func newListingFixture(t *testing.T, engine string) *listingFixture {
	t.Helper()
	dir := t.TempDir()
	s := openEngineStore(t, engine, dir)
	fx := &listingFixture{s: s, dir: dir, arch: s.AddCollection("Weddings")}
	fo := s.AddFolder(fx.arch.ID, "/photos")
	for i, rel := range []string{"a/one.nef", "a/two.nef", "b/three.jpg"} {
		f, err := s.UpsertFileErr(File{CollectionID: fx.arch.ID, FolderID: fo.ID, RelPath: rel, SizeBytes: int64(100 + i),
			HashAlg: "SHA256", Hash: testHash(rel)})
		if err != nil {
			t.Fatal(err)
		}
		fx.files = append(fx.files, f)
	}
	fx.vol = s.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD", Serial: "S1"})
	return fx
}

func refOf(f *File) ChunkFileRef {
	return ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: f.SizeBytes, Hash: f.Hash}
}

func (fx *listingFixture) reopen(t *testing.T, engine string) {
	t.Helper()
	if err := fx.s.Close(); err != nil {
		t.Fatal(err)
	}
	fx.s = openEngineStore(t, engine, fx.dir)
}

// On the database engine a package's listings are not held in memory: they are
// read from chunk_file when asked for, and survive a reopen.
func TestPackages_ListingsAreReadWhenAskedFor(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			fx := newListingFixture(t, engine)
			files := []ChunkFileRef{refOf(fx.files[0]), refOf(fx.files[2])}
			unknown := []ChunkFileRef{refOf(fx.files[1])}
			ch, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "MIRROR-1", Mirror: true, Files: files,
				UnknownFiles: unknown})
			if err != nil {
				t.Fatal(err)
			}
			if engine == "sqlite" && (fx.s.c.Chunks[0].Files != nil || fx.s.c.Chunks[0].UnknownFiles != nil) {
				t.Fatalf("the database engine holds a package's listings in memory: %d files, %d unknown",
					len(fx.s.c.Chunks[0].Files), len(fx.s.c.Chunks[0].UnknownFiles))
			}
			for round := 0; round < 2; round++ {
				if got := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); !reflect.DeepEqual(got, files) {
					t.Fatalf("round %d: ChunkFiles = %+v, want %+v", round, got, files)
				}
				if got := noErr[[]ChunkFileRef](t)(fx.s.ChunkUnknownFiles(ch.ID)); !reflect.DeepEqual(got, unknown) {
					t.Fatalf("round %d: ChunkUnknownFiles = %+v, want %+v", round, got, unknown)
				}
				fx.reopen(t, engine)
			}
			// The copy handed out is the caller's.
			got := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID))
			got[0].RelPath = "changed"
			if again := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); again[0].RelPath != files[0].RelPath {
				t.Fatal("changing the returned listing changed the package's")
			}
			if _, err := fx.s.ChunkFiles(ch.ID + 99); err == nil {
				t.Fatal("ChunkFiles of an unknown package returned no error")
			}
		})
	}
}

// A listing stores its path, size and hash only where its file has other values
// (an older version, or no file at all); the rest are read from the file.
func TestPackages_ListingStoresOnlyWhatDiffersFromItsFile(t *testing.T) {
	fx := newListingFixture(t, "sqlite")
	older := refOf(fx.files[1])
	older.Hash, older.SizeBytes = testHash("two, as it was"), 7
	noFile := ChunkFileRef{FileID: 0, RelPath: "adopted/x.jpg", SizeBytes: 9, Hash: testHash("x")}
	noHash := refOf(fx.files[2])
	noHash.Hash = "" // listed before it was hashed
	want := []ChunkFileRef{refOf(fx.files[0]), older, noFile, noHash}
	ch, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "NSP-C0001", Files: want})
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.s.FlushPending(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(filepath.Join(fx.dir, catalogDBName)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT rel_path IS NULL, size_bytes IS NULL, sha256 IS NULL, length(sha256)
		FROM chunk_file WHERE chunk_id = ? AND list = 0 ORDER BY ord`, ch.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type stored struct {
		relNull, sizeNull, hashNull bool
		hashLen                     sql.NullInt64
	}
	var got []stored
	for rows.Next() {
		var st stored
		if err := rows.Scan(&st.relNull, &st.sizeNull, &st.hashNull, &st.hashLen); err != nil {
			t.Fatal(err)
		}
		got = append(got, st)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	expect := []stored{
		{true, true, true, sql.NullInt64{}},                          // all the file's
		{true, false, false, sql.NullInt64{Int64: 32, Valid: true}},  // older size and hash kept
		{false, false, false, sql.NullInt64{Int64: 32, Valid: true}}, // no file: everything kept
		{true, true, false, sql.NullInt64{Int64: 0, Valid: true}},    // no hash, unlike its file's
	}
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("stored columns = %+v, want %+v", got, expect)
	}
	if read := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); !reflect.DeepEqual(read, want) {
		t.Fatalf("read back %+v, want %+v", read, want)
	}
	// The file changing later does not change what the package holds.
	if _, err := fx.s.UpsertFileErr(File{CollectionID: fx.arch.ID, FolderID: fx.files[0].FolderID, RelPath: fx.files[0].RelPath,
		SizeBytes: 555, HashAlg: "SHA256", Hash: testHash("one, edited")}); err != nil {
		t.Fatal(err)
	}
	if read := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); !reflect.DeepEqual(read, want) {
		t.Fatalf("after the file changed, read %+v, want %+v", read, want)
	}
}

// Saving a package's header, copies or verify events leaves its listings as they
// were, and SetChunkFiles replaces them, on both engines.
func TestPackages_HeaderSavesKeepListingsAndSetReplaces(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			fx := newListingFixture(t, engine)
			files := []ChunkFileRef{refOf(fx.files[0]), refOf(fx.files[1])}
			ch, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "NSP-C0001", Status: "PLANNED", Files: files})
			if err != nil {
				t.Fatal(err)
			}
			if err := fx.s.FlushPending(); err != nil {
				t.Fatal(err)
			}
			ch.Status = "VERIFIED"
			fx.s.UpdateChunk(ch)
			fx.s.RecordCopy(ch, fx.vol.ID, "D:/NSP-C0001", true)
			fx.s.AppendVerifyEvent(ch, VerifyEvent{OK: true, Path: "D:/NSP-C0001", Note: "media verify"})
			fx.reopen(t, engine)
			if got := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); !reflect.DeepEqual(got, files) {
				t.Fatalf("after header saves the listing is %+v, want %+v", got, files)
			}
			ch = fx.s.Chunk(ch.ID)
			if ch.Status != "VERIFIED" || len(ch.Copies) != 1 || len(ch.VerifyEvents) != 1 {
				t.Fatalf("header after reopen: status %s, %d copies, %d events", ch.Status, len(ch.Copies), len(ch.VerifyEvents))
			}
			replaced := []ChunkFileRef{refOf(fx.files[2])}
			if err := fx.s.SetChunkFiles(ch, replaced); err != nil {
				t.Fatal(err)
			}
			if err := fx.s.SetChunkUnknownFiles(ch, []ChunkFileRef{refOf(fx.files[0])}); err != nil {
				t.Fatal(err)
			}
			if err := fx.s.SetChunkUnknownFiles(ch, nil); err != nil {
				t.Fatal(err)
			}
			fx.reopen(t, engine)
			if got := noErr[[]ChunkFileRef](t)(fx.s.ChunkFiles(ch.ID)); !reflect.DeepEqual(got, replaced) {
				t.Fatalf("after SetChunkFiles the listing is %+v, want %+v", got, replaced)
			}
			if got := noErr[[]ChunkFileRef](t)(fx.s.ChunkUnknownFiles(ch.ID)); len(got) != 0 {
				t.Fatalf("cleared unknown listing reads back %+v", got)
			}
		})
	}
}

// A package whose listings cannot all be recorded is not added, and the header
// row written ahead of its listings is removed with them.
func TestPackages_FailedAddLeavesNoRows(t *testing.T) {
	fx := newListingFixture(t, "sqlite")
	fx.s.BeginBatch()
	_, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "BAD", Files: []ChunkFileRef{refOf(fx.files[0])},
		UnknownFiles: []ChunkFileRef{{FileID: 0, RelPath: "x", Hash: "not a hash"}}})
	if err == nil {
		t.Fatal("a listing with a malformed hash was recorded")
	}
	if len(fx.s.Chunks(0)) != 0 {
		t.Fatalf("the package was kept in memory: %d packages", len(fx.s.Chunks(0)))
	}
	if err := fx.s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	c := persistedCatalog(t, fx.dir)
	if len(c.Chunks) != 0 {
		t.Fatalf("the database kept %d package rows", len(c.Chunks))
	}
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(filepath.Join(fx.dir, catalogDBName)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM chunk_file`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("chunk_file holds %d rows (%v)", n, err)
	}
}

// What protection and the coverage reads report is the same on both engines.
func TestPackages_ProtectionAgreesAcrossEngines(t *testing.T) {
	type result struct {
		prot   ProtectionResult
		hashes map[int]string
		ids    map[int]bool
	}
	got := map[string]result{}
	for _, engine := range []string{"json", "sqlite"} {
		fx := newListingFixture(t, engine)
		ch, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "NSP-C0001", Status: "VERIFIED",
			Files: []ChunkFileRef{refOf(fx.files[0]), refOf(fx.files[1])}})
		if err != nil {
			t.Fatal(err)
		}
		fx.s.RecordCopy(ch, fx.vol.ID, "D:/NSP-C0001", true)
		other := fx.s.AddVolume(Volume{Label: "DRIVE-02", Kind: "HDD", Serial: "S2"})
		mirror, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "MIRROR", Status: StatusAdoptedVerified, Mirror: true,
			Files: []ChunkFileRef{refOf(fx.files[1]), refOf(fx.files[2])}})
		if err != nil {
			t.Fatal(err)
		}
		fx.s.RecordCopy(mirror, other.ID, "E:/", true)
		if _, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "FAILED", Status: "FAILED",
			Files: []ChunkFileRef{refOf(fx.files[2])}}); err != nil {
			t.Fatal(err)
		}
		fx.reopen(t, engine)
		prot, err := fx.s.Protection(fx.arch.ID)
		if err != nil {
			t.Fatal(err)
		}
		got[engine] = result{prot: prot,
			hashes: noErr[map[int]string](t)(fx.s.ChunkedFileHashes(fx.arch.ID)),
			ids:    noErr[map[int]bool](t)(fx.s.ChunkedFileIDs(fx.arch.ID))}
		if err := fx.s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(got["json"], got["sqlite"]) {
		t.Fatalf("engines disagree:\njson   %+v\nsqlite %+v", got["json"], got["sqlite"])
	}
	if len(got["json"].ids) != 3 {
		t.Fatalf("expected the three listed files, got %v", got["json"].ids)
	}
}

// The package list the API sends leaves listings out; one package's page has them.
func TestPackages_RecordAndHeaders(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			fx := newListingFixture(t, engine)
			files := []ChunkFileRef{refOf(fx.files[0])}
			ch, err := fx.s.AddChunkErr(Chunk{CollectionID: fx.arch.ID, Name: "NSP-C0001", Files: files})
			if err != nil {
				t.Fatal(err)
			}
			rec, err := fx.s.PackageRecord(ch.ID)
			if err != nil || rec == nil || !reflect.DeepEqual(rec.Files, files) {
				t.Fatalf("PackageRecord = %+v, %v", rec, err)
			}
			if rec, err := fx.s.PackageRecord(ch.ID + 9); rec != nil || err != nil {
				t.Fatalf("PackageRecord of an unknown package = %+v, %v", rec, err)
			}
			hs := fx.s.PackageHeaders(fx.arch.ID)
			if len(hs) != 1 || hs[0].Name != ch.Name || hs[0].Files != nil {
				t.Fatalf("PackageHeaders = %+v", hs)
			}
		})
	}
}

// A path-only scan reads the same files, pages and paths as a full one, and is
// ignored when the query filters on anything the path columns do not hold.
func TestFiles_PathsOnlyScanMatchesTheFullScan(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	coll := s.AddCollection("Big")
	other := s.AddCollection("Other")
	fo := s.AddFolder(coll.ID, "/big")
	n := filePageSize + 9
	s.BeginBatch()
	for i := 0; i < n; i++ {
		cid := coll.ID
		if i%7 == 0 {
			cid = other.ID
		}
		if _, err := s.UpsertFileErr(File{CollectionID: cid, FolderID: fo.ID, RelPath: fmt.Sprintf("d/%06d.nef", n-i),
			SizeBytes: int64(i), HashAlg: "SHA256", Hash: testHash(fmt.Sprint(i)), Role: RoleOriginals}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	type row struct {
		id, coll, folder int
		rel              string
	}
	scan := func(q fileQuery) (rows []row, full bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := s.eachFileLocked(q, func(f *File) error {
			rows = append(rows, row{f.ID, f.CollectionID, f.FolderID, f.RelPath})
			full = full || f.Hash != ""
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return rows, full
	}
	for _, byPath := range []bool{false, true} {
		want, _ := scan(fileQuery{CollectionID: coll.ID, ByPath: byPath})
		got, full := scan(fileQuery{CollectionID: coll.ID, ByPath: byPath, PathsOnly: true})
		if !reflect.DeepEqual(got, want) || full {
			t.Fatalf("byPath %v: path-only scan read %d files (full records %v), want %d", byPath, len(got), full, len(want))
		}
	}
	if _, full := scan(fileQuery{CollectionID: coll.ID, Role: RoleOriginals, PathsOnly: true}); !full {
		t.Fatal("a query filtering on role read only the path columns")
	}
}
