//go:build !guionly

package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// openEngineStore opens a store on the named engine, whatever the run chose.
func openEngineStore(t *testing.T, engine, dataDir string) *Store {
	t.Helper()
	t.Setenv(catalogEngineEnv, engine)
	s, err := openTestStore(t, dataDir)
	if err != nil {
		t.Fatalf("open on %s: %v", engine, err)
	}
	return s
}

// A store on the SQLite engine keeps its catalog in catalog.db, not catalog.json,
// and finds it there again.
func TestCatalogEngine_SQLiteStoreUsesTheDatabase(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	c := s.AddCollection("Weddings")
	s.Log("test", "kept")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the database engine wrote catalog.json (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, catalogDBName)); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	if got := s.Collection(c.ID); got == nil || got.Name != "Weddings" {
		t.Fatalf("after reopening: %+v", got)
	}
	if a := s.LastAudit(); a == nil || a.Detail != "kept" {
		t.Errorf("audit after reopening: %+v", a)
	}
}

// An engine name the build does not know is refused before anything is created.
func TestCatalogEngine_UnknownEngineRefused(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(catalogEngineEnv, "postgres")
	if _, err := openTestStore(t, dir); err == nil || !strings.Contains(err.Error(), "not a catalog engine") {
		t.Fatalf("got %v", err)
	}
	if names, _ := os.ReadDir(dir); len(names) != 0 {
		t.Errorf("a refused open wrote %v", names)
	}
}

// Opening a catalog.json folder on the database engine moves the catalog in.
func TestCatalogEngine_MovesAnExistingCatalog(t *testing.T) {
	dir := t.TempDir()
	js := openEngineStore(t, "json", dir)
	c := js.AddCollection("Family")
	js.AddFolder(c.ID, "/photos")
	js.Close()
	s := openEngineStore(t, "sqlite", dir)
	if got := s.Collection(c.ID); got == nil || got.Name != "Family" {
		t.Fatalf("after the move: %+v", got)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "catalog.json")); !isCatalogStub(raw) {
		t.Error("catalog.json is not the stub after the move")
	}
}

// A save writes only the records that changed since the last commit.
func TestCatalogEngine_SaveWritesOnlyChangedRows(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	var colls []*Collection
	for i := 0; i < 50; i++ {
		colls = append(colls, s.AddCollection("archive"))
	}
	be := s.be.(*dbBackend)
	if err := s.SetCollectionRetired(colls[20].ID, true, false); err != nil {
		t.Fatal(err)
	}
	// The archive and the audit entry that records the retirement.
	if be.lastWritten != 2 || be.lastRemoved != 0 {
		t.Errorf("retiring one archive wrote %d records and removed %d; want 2 and 0", be.lastWritten, be.lastRemoved)
	}
}

// During a batch, saves are coalesced: a change reaches the database within about
// one batch interval, and EndBatch commits the rest and reports its error.
func TestCatalogEngine_BatchCommitsOnInterval(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	s.batchInterval = 300 * time.Millisecond
	s.Flush()
	committed := func(name string) bool {
		db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(filepath.Join(dir, catalogDBName)))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM collection WHERE name = ?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n > 0
	}
	s.BeginBatch()
	s.AddCollection("first") // a save just happened, so this one waits
	if committed("first") {
		t.Fatal("a coalesced save committed at once")
	}
	time.Sleep(400 * time.Millisecond)
	s.AddCollection("second") // the interval has passed: this save commits both
	if !committed("first") || !committed("second") {
		t.Fatal("a save after the interval did not commit the batch so far")
	}
	s.AddCollection("third")
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	if !committed("third") {
		t.Fatal("EndBatch did not commit")
	}
}

// A record the database cannot hold makes the save fail with the reason, and the
// next save still tries again: nothing is dropped quietly.
func TestCatalogEngine_WriteErrorsPropagate(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	s.BeginBatch()
	s.mu.Lock()
	s.c.Snapshots = append(s.c.Snapshots, &VolumeSnapshot{VolumeID: 7,
		CapturedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)})
	err := s.save()
	s.mu.Unlock()
	err = errors.Join(err, s.EndBatch())
	if err == nil || !strings.Contains(err.Error(), "outside the years 0 to 9999") {
		t.Fatalf("save and EndBatch: %v", err)
	}
	if err := s.FlushPending(); err == nil {
		t.Fatal("the failed change was forgotten instead of retried")
	}
}

// catalogEngineScenario drives a store through records of every kind and the
// ways they change, the same way on any engine.
func catalogEngineScenario(t *testing.T, s *Store) {
	t.Helper()
	day := time.Date(2025, 5, 1, 9, 0, 0, 0, time.UTC)
	arch := s.AddCollection("Weddings")
	union := s.AddCollectionKind("Old drives", ArchiveSourceless)
	gone := s.AddCollection("Removed later")
	fo := s.AddFolder(arch.ID, "/photos")
	s.AddFolder(gone.ID, "/gone")
	var files []*File
	for i, rel := range []string{"a/one.nef", "a/two.nef", "b/three.jpg"} {
		f, err := s.UpsertFileErr(File{CollectionID: arch.ID, FolderID: fo.ID, RelPath: rel, SizeBytes: int64(100 + i),
			HashAlg: "SHA256", Hash: testHash(rel), ModTime: day, Role: RoleOriginals})
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	// New content for one file keeps the old as a version.
	if _, err := s.UpsertFileErr(File{CollectionID: arch.ID, FolderID: fo.ID, RelPath: "a/one.nef", SizeBytes: 111,
		HashAlg: "SHA256", Hash: testHash("one v2"), ModTime: day.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	s.UpsertUnionFiles(union.ID, []unionFile{{RelPath: "DCIM/IMG_1.JPG", Hash: testHash("img1"), Size: 5},
		{RelPath: "DCIM/IMG_1.JPG", Hash: testHash("img1-other"), Size: 6}, {RelPath: "x.jpg", Hash: testHash("img1"), Size: 5}})
	if _, err := s.UpsertFileErr(File{CollectionID: gone.ID, FolderID: fo.ID + 1, RelPath: "z.nef", SizeBytes: 1,
		HashAlg: "SHA256", Hash: testHash("z")}); err != nil {
		t.Fatal(err)
	}
	loc := s.AddLocation("Grandma's", true, "offsite")
	v := s.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD", Serial: "S1"})
	if err := s.SetVolumeLocation(v.ID, loc.ID); err != nil {
		t.Fatal(err)
	}
	ok := true
	s.AppendSmartSnapshot(v, SmartSnapshot{At: day, Passed: &ok, Note: "fine"})
	ch := s.AddChunk(Chunk{CollectionID: arch.ID, Name: "NSP-C0001", Status: "PLANNED", HashAlg: "SHA256",
		Encrypted: true, CreatedAt: day, Files: []ChunkFileRef{
			{FileID: files[1].ID, RelPath: files[1].RelPath, SizeBytes: files[1].SizeBytes, Hash: files[1].Hash},
			{FileID: files[2].ID, RelPath: files[2].RelPath, SizeBytes: files[2].SizeBytes, Hash: files[2].Hash},
		}})
	ch.Status, ch.TarHash = "VERIFIED", testHash("tar")
	s.UpdateChunk(ch)
	s.RecordCopy(ch, v.ID, "D:/NSP-C0001", true)
	s.AppendVerifyEvent(ch, VerifyEvent{At: day, OK: true, Path: "D:/NSP-C0001", Note: "media verify", Level: "B"})
	ev := s.AddEvent(&Event{Name: "Smith wedding", EventType: "wedding", CreatedAt: day})
	other := s.AddEvent(&Event{Name: "Dropped", CreatedAt: day})
	noErr[int](t)(s.AssignFilesToEvent([]int{files[1].ID, files[2].ID}, ev.ID))
	noErr[int](t)(s.AssignFilesToEvent([]int{files[0].ID}, other.ID))
	s.DeleteEvent(other.ID)
	if err := s.PutVolumeSnapshot(&VolumeSnapshot{VolumeID: v.ID, CapturedAt: day, Label: v.Label, Scan: ScanComplete,
		Files: []SnapFile{{RelPath: "photos/a/two.nef", SizeBytes: 101, Hash: testHash("a/two.nef")}}, TotalFiles: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddScanRecord(&ScanRecord{VolumeID: v.ID, At: day, Scan: ScanComplete, TotalFiles: 1,
		FileList: "scan-v1-x.json.gz", FileListSHA256: testHash("list")}, 10); err != nil {
		t.Fatal(err)
	}
	s.AddBackupSession(&BackupSession{CollectionID: arch.ID, VolumeID: v.ID, Mode: "mirror", Files: 2, At: day, Name: "Incremental"})
	s.AddKeyMeta(KeyMeta{Ref: "key-1", Fingerprint: "ABCD", CreatedAt: day})
	if err := s.SetCollectionRetired(union.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemoveCollection(gone.ID); err != nil {
		t.Fatal(err)
	}
	s.Log("scenario", "done")
	if err := s.FlushPending(); err != nil {
		t.Fatal(err)
	}
}

// exportedCatalog is the store's whole catalog as the database or catalog.json
// holds it, with every time cleared (two runs of the scenario stamp different
// times) and in canonical form.
func exportedCatalog(t *testing.T, engine, dir string) *catalog {
	t.Helper()
	var c catalog
	switch engine {
	case "json":
		raw, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeCatalogJSON(raw, &c); err != nil {
			t.Fatal(err)
		}
	default:
		d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
		if err != nil {
			t.Fatal(err)
		}
		got, err := exportCatalogDB(d)
		d.Close()
		if err != nil {
			t.Fatal(err)
		}
		c = *got
	}
	canonCatalog(&c)
	clearTimes(reflect.ValueOf(&c).Elem())
	c.SchemaVersion = 0
	return &c
}

// clearTimes sets every time reachable from v to the zero time, and every
// pointer to a time to a pointer to the zero time.
func clearTimes(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			clearTimes(v.Elem())
		}
	case reflect.Struct:
		if v.Type() == timeType {
			v.Set(reflect.ValueOf(time.Time{}))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				clearTimes(v.Field(i))
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			clearTimes(v.Index(i))
		}
	}
}

// The same work on both engines leaves the same catalog: exported JSON from
// catalog.json and from catalog.db agree section by section.
func TestCatalogEngine_DifferentialExport(t *testing.T) {
	results := map[string]*catalog{}
	for _, engine := range []string{"json", "sqlite"} {
		dir := t.TempDir()
		s := openEngineStore(t, engine, dir)
		catalogEngineScenario(t, s)
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		results[engine] = exportedCatalog(t, engine, dir)
	}
	want, got := catalogSectionDigests(results["json"]), catalogSectionDigests(results["sqlite"])
	for _, k := range catalogKeys(want) {
		if want[k] != got[k] {
			a, _ := json.Marshal(reflect.ValueOf(results["json"]).Elem().FieldByIndex(fieldIndexByTag(k)).Interface())
			b, _ := json.Marshal(reflect.ValueOf(results["sqlite"]).Elem().FieldByIndex(fieldIndexByTag(k)).Interface())
			t.Errorf("section %s differs between the engines:\njson   %.800s\nsqlite %.800s", k, a, b)
		}
	}
}

func init() { persistedCatalogFromDB = readPersistedCatalogDB }

// readPersistedCatalogDB reads the catalog database's last commit.
func readPersistedCatalogDB(t testing.TB, dataDir string) *catalog {
	t.Helper()
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(filepath.Join(dataDir, catalogDBName)))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	got, err := exportCatalogRows(tx)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// The guard behind TestMain catches a file record changed without a method, on
// either engine, names where the copy was handed out, and lets a copy that went
// back through a method pass.
func TestFileGuard_CatchesAChangeWithoutAMethod(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			prev := fileCopyGuard
			g := newFileGuard(1000)
			fileCopyGuard = g
			defer func() { fileCopyGuard = prev }()

			s := openEngineStore(t, engine, t.TempDir())
			coll := s.AddCollection("A")
			fo := s.AddFolder(coll.ID, "/src")
			f, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "a.nef", HashAlg: "SHA256",
				Hash: testHash("a")})
			if err != nil {
				t.Fatal(err)
			}
			got := noErr[*File](t)(s.FileByID(f.ID))
			got.Role = RoleOriginals // changed without a method: the catalog never sees it
			v := g.violations()
			if len(v) != 1 || !strings.Contains(v[0], "store_engine_test.go") || !strings.Contains(v[0], "a.nef") {
				t.Fatalf("violations: %q", v)
			}
			if stored := noErr[*File](t)(s.FileByID(f.ID)); stored.Role != "" {
				t.Fatal("changing a handed-out copy changed the catalog")
			}
			// Passing it back through a method is the sanctioned change.
			if _, err := s.UpsertFileErr(*got); err != nil {
				t.Fatal(err)
			}
			if v := g.violations(); len(v) != 0 {
				t.Fatalf("a copy passed back through a method is still reported: %q", v)
			}
			if stored := noErr[*File](t)(s.FileByID(f.ID)); stored.Role != RoleOriginals {
				t.Fatalf("the change made through the method is missing: %+v", stored)
			}
		})
	}
}

// Only the store's own file code reaches the file records: every other file goes
// through a Store method, so the database engine sees every change.
func TestFiles_OnlyTheStoreTouchesFileRecords(t *testing.T) {
	allowed := map[string]bool{"store.go": true, "store_files.go": true}
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	direct := regexp.MustCompile(`\.c\.Files\b`)
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") || allowed[name] {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if direct.MatchString(line) {
				t.Errorf("%s:%d reaches the file records directly: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// On the database engine files live only in their tables: the store holds none in
// memory, and its iterators read them a page at a time, in path order, with the
// loop free to call the store as it goes.
func TestCatalogEngine_FilesStayInTheDatabaseAndPage(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	coll := s.AddCollection("Big")
	fo := s.AddFolder(coll.ID, "/big")
	n := filePageSize*2 + 7
	s.BeginBatch()
	for i := n - 1; i >= 0; i-- { // inserted in reverse, so path order is not insertion order
		if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: fmt.Sprintf("d/%06d.nef", i),
			SizeBytes: int64(i), HashAlg: "SHA256", Hash: testHash(fmt.Sprint(i))}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	if len(s.c.Files) != 0 {
		t.Fatalf("the database engine holds %d files in memory", len(s.c.Files))
	}
	prev, count := "", 0
	for f, err := range s.FilesOf(coll.ID) {
		if err != nil {
			t.Fatal(err)
		}
		if f.RelPath <= prev {
			t.Fatalf("out of order: %q after %q", f.RelPath, prev)
		}
		prev = f.RelPath
		count++
		if count%1000 == 0 { // calling the store inside the loop must not deadlock
			if _, err := s.FileByID(f.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if count != n {
		t.Fatalf("FilesOf yielded %d of %d files", count, n)
	}
	if files, bytes, err := s.FileStats(coll.ID); err != nil || files != n || bytes != int64(n*(n-1)/2) {
		t.Fatalf("FileStats = %d, %d, %v", files, bytes, err)
	}
	// Stopping early stops the reads.
	seen := 0
	for _, err := range s.AllFiles() {
		if err != nil {
			t.Fatal(err)
		}
		if seen++; seen == 3 {
			break
		}
	}
}

// File writes join the batch like every other change: a coalesced save leaves
// them uncommitted, and EndBatch commits them.
func TestCatalogEngine_FileWritesCommitWithTheBatch(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	coll := s.AddCollection("A")
	fo := s.AddFolder(coll.ID, "/src")
	s.batchInterval = time.Hour
	s.Flush()
	s.BeginBatch()
	if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: "late.nef", HashAlg: "SHA256",
		Hash: testHash("late")}); err != nil {
		t.Fatal(err)
	}
	s.Log("batch", "coalesced") // a save inside the batch, put off
	if strings.Contains(persistedCatalogTextFrom(t, dir), "late.nef") {
		t.Fatal("a file write committed before the batch did")
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(persistedCatalogTextFrom(t, dir), "late.nef") {
		t.Fatal("EndBatch did not commit the file write")
	}
}

// persistedCatalogTextFrom reads the catalog database's last commit as JSON,
// whatever engine the run uses.
func persistedCatalogTextFrom(t *testing.T, dir string) string {
	t.Helper()
	b, err := json.Marshal(readPersistedCatalogDB(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Search returns the same files on both engines for every filter it has,
// including a case-insensitive path search on non-ASCII names.
func TestCatalogEngine_SearchMatchesAcrossEngines(t *testing.T) {
	day := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	results := map[string][][]int{}
	queries := []SearchQuery{
		{Text: "é"}, {Text: "ÉTÉ"}, {Text: "kelvin"}, {Text: "/b/"},
		{Hash: testHash("2")[:6]}, {Hash: strings.ToUpper(testHash("3")[:10])}, {Hash: "zz"},
		{Ext: "jpg"}, {Role: "originals"}, {EventID: -1}, {From: day, To: day.Add(48 * time.Hour)},
		{Text: "a", Limit: 2},
	}
	for _, engine := range []string{"json", "sqlite"} {
		s := openEngineStore(t, engine, t.TempDir())
		coll := s.AddCollection("A")
		fo := s.AddFolder(coll.ID, "/src")
		names := []string{"a/Été.NEF", "a/ete.jpg", "b/KELVIN.nef", "b/plain.jpg", "c/x.NEF", "a/more.jpg"}
		var ids []int
		for i, name := range names {
			f, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: name, SizeBytes: int64(i),
				HashAlg: "SHA256", Hash: testHash(fmt.Sprint(i)), Role: []string{RoleOriginals, ""}[i%2],
				ShotAt: day.Add(time.Duration(i) * 24 * time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, f.ID)
		}
		ev := s.AddEvent(&Event{Name: "E", CreatedAt: day})
		noErr[int](t)(s.AssignFilesToEvent(ids[:2], ev.ID))
		for _, q := range queries {
			var got []int
			for _, row := range noErr[[]map[string]any](t)(s.Search(q)) {
				got = append(got, row["file_id"].(int))
			}
			results[engine] = append(results[engine], got)
		}
	}
	for i, q := range queries {
		if !reflect.DeepEqual(results["json"][i], results["sqlite"][i]) {
			t.Errorf("Search(%+v): json %v, sqlite %v", q, results["json"][i], results["sqlite"][i])
		}
	}
	// "ÉTÉ" finds "Été" (non-ASCII case folding); neither finds the unaccented name.
	if len(results["json"][0]) != 1 || !reflect.DeepEqual(results["json"][0], results["json"][1]) {
		t.Errorf("the accented searches found %v and %v; want the one file named Été", results["json"][0], results["json"][1])
	}
}

// TestCatalogEngineScale1M drives a million-file archive through the store on the
// database engine: upsert in one batch, reopen, page through every file, count,
// search and protection. SKIPPED by default; CI runs it:
//
//	OBELISK_SCALE=1 go test -run TestCatalogEngineScale1M -v -timeout 30m
func TestCatalogEngineScale1M(t *testing.T) {
	if os.Getenv("OBELISK_SCALE") == "" {
		t.Skip("set OBELISK_SCALE=1 to run the million-file store benchmark")
	}
	n := envInt("OBELISK_FILES", 1_000_000)
	prev := fileCopyGuard
	fileCopyGuard = nil // a million handed-out copies would only measure the guard
	defer func() { fileCopyGuard = prev }()
	engine := os.Getenv("OBELISK_SCALE_ENGINE") // json, to compare; the database by default
	if engine == "" {
		engine = "sqlite"
	}
	dir := t.TempDir()
	s := openEngineStore(t, engine, dir)
	coll := s.AddCollection("Scale")
	fo := s.AddFolder(coll.ID, "/srv/scale")
	t0 := time.Now()
	s.BeginBatch()
	refs := make([]ChunkFileRef, 0, n)
	for i := 0; i < n; i++ {
		f, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID,
			RelPath:   fmt.Sprintf("Clients/%04d/RAW/DSC_%08d.NEF", i%5000, i),
			SizeBytes: 38_000_000 + int64(i%1000), HashAlg: "SHA256", Hash: fakeHash(i), Blake3: fakeHash(i + n),
			ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Role: RoleOriginals})
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: f.SizeBytes, Hash: f.Hash})
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	upsert := time.Since(t0)
	// Every file in a package of 10,000 with a verified copy, as after a backup.
	t0 = time.Now()
	vol := s.AddVolume(Volume{Label: "SCALE-01", Kind: "HDD", Serial: "SCALE1"})
	s.BeginBatch()
	const perPackage = 10_000
	for k := 0; k*perPackage < n; k++ {
		part := refs[k*perPackage : min((k+1)*perPackage, n)]
		ch, err := s.AddChunkErr(Chunk{CollectionID: coll.ID, Name: fmt.Sprintf("NSP-C%05d", k+1), Status: "VERIFIED",
			HashAlg: "SHA256", FileCount: len(part), Files: part})
		if err != nil {
			t.Fatal(err)
		}
		s.RecordCopy(ch, vol.ID, "D:/"+ch.Name, true)
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	packages := time.Since(t0)
	refs = nil
	t0 = time.Now()
	for i := 0; i < 1000; i++ { // a rescan's worth of unchanged files
		if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID,
			RelPath:   fmt.Sprintf("Clients/%04d/RAW/DSC_%08d.NEF", i%5000, i),
			SizeBytes: 38_000_000 + int64(i%1000), HashAlg: "SHA256", Hash: fakeHash(i)}); err != nil {
			t.Fatal(err)
		}
	}
	rescan := time.Since(t0) / 1000
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	t0 = time.Now()
	s = openEngineStore(t, engine, dir)
	open := time.Since(t0)
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t0 = time.Now()
	count := 0
	for _, err := range s.FilesOf(coll.ID) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	iterate := time.Since(t0)
	if count != n {
		t.Fatalf("FilesOf yielded %d of %d", count, n)
	}
	t0 = time.Now()
	files, _, err := s.FileStats(coll.ID)
	if err != nil || files != n {
		t.Fatalf("FileStats: %d, %v", files, err)
	}
	stats := time.Since(t0)
	search := func(q SearchQuery) time.Duration {
		t0 := time.Now()
		if _, err := s.Search(q); err != nil {
			t.Fatal(err)
		}
		return time.Since(t0)
	}
	sPath := search(SearchQuery{Text: "dsc_00000042", Limit: 50})
	sHash := search(SearchQuery{Hash: fakeHash(n / 2)[:12], Limit: 50})
	if out := os.Getenv("OBELISK_CPUPROFILE"); out != "" { // a profile of the protection read
		pf, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		if err := pprof.StartCPUProfile(pf); err != nil {
			t.Fatal(err)
		}
		defer pf.Close()
	}
	t0 = time.Now()
	prot, err := s.Protection(coll.ID)
	if err != nil {
		t.Fatal(err)
	}
	protection := time.Since(t0)
	if os.Getenv("OBELISK_CPUPROFILE") != "" {
		pprof.StopCPUProfile()
	}
	if prot.TotalFiles != n {
		t.Fatalf("protection counted %d of %d files", prot.TotalFiles, n)
	}
	fi, err := os.Stat(filepath.Join(dir, map[string]string{"sqlite": catalogDBName, "json": "catalog.json"}[engine]))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\n=== Store on the %s engine, %s files ===\n"+
		"upsert all (one batch)   %10s  (%s files/s)\n"+
		"upsert one, unchanged    %10s\n"+
		"package every file       %10s\n"+
		"reopen the store         %10s\n"+
		"heap in use after open   %10.1f MB\n"+
		"FilesOf, every file      %10s\n"+
		"FileStats                %10s\n"+
		"search by path           %10s\n"+
		"search by hash prefix    %10s\n"+
		"protection of the archive %9s\n"+
		"catalog file             %10.1f MB",
		engine, comma(n), round(upsert), comma(int(float64(n)/upsert.Seconds())), rescan, round(packages), round(open),
		float64(m.HeapInuse)/1e6, round(iterate), round(stats), round(sPath), round(sHash), round(protection),
		float64(fi.Size())/1e6)
}
