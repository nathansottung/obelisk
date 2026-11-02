//go:build !guionly

package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// catalogFiller sets every exported field reachable from a catalog to a distinct
// non-zero value, so a round trip that drops or bends any field is caught.
type catalogFiller struct{ n int }

func (f *catalogFiller) next() int { f.n++; return f.n }

var catalogHashFields = map[string]bool{"Hash": true, "Blake3": true, "SampleHash": true}

func (f *catalogFiller) fill(v reflect.Value, name string) {
	switch v.Kind() {
	case reflect.String:
		if catalogHashFields[name] {
			v.SetString(fakeHash(f.next()))
		} else {
			v.SetString(fmt.Sprintf("%s-%d ✓", name, f.next()))
		}
	case reflect.Int, reflect.Int64, reflect.Int32:
		v.SetInt(int64(f.next()))
	case reflect.Uint64, reflect.Uint32:
		v.SetUint(uint64(f.next()))
	case reflect.Float64:
		v.SetFloat(float64(f.next()) + 0.25)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		f.fill(v.Elem(), name)
	case reflect.Struct:
		if v.Type() == timeType {
			// Nanoseconds, and an offset other than UTC on every other time: the
			// database keeps the instant.
			t := time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC).Add(time.Duration(f.next()) * time.Hour)
			if f.n%2 == 0 {
				t = t.In(time.FixedZone("CEST", 2*3600))
			}
			v.Set(reflect.ValueOf(t))
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				f.fill(v.Field(i), v.Type().Field(i).Name)
			}
		}
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 2, 2)
		for i := 0; i < 2; i++ {
			f.fill(s.Index(i), name)
		}
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		for i := 0; i < 2; i++ {
			k := reflect.New(v.Type().Key()).Elem()
			if k.Kind() == reflect.String {
				k.SetString(fakeHash(f.next())) // plan maps are keyed by content hash
			} else {
				f.fill(k, name)
			}
			e := reflect.New(v.Type().Elem()).Elem()
			f.fill(e, name)
			m.SetMapIndex(k, e)
		}
		v.Set(m)
	}
}

// fullCatalog is a catalog with every field set, plus the shapes the importer
// stores specially: package listings that repeat their file's path, size and hash
// (stored once), one with an empty hash where the file has one, one with a
// different path, a pointer to a zero time, a false *bool, a false plan flag and
// an empty but present list.
func fullCatalog(t *testing.T) *catalog {
	t.Helper()
	c := &catalog{}
	(&catalogFiller{}).fill(reflect.ValueOf(c).Elem(), "catalog")
	c.SchemaVersion = currentSchemaVersion
	f0, f1 := c.Files[0], c.Files[1]
	ch := c.Chunks[0]
	ch.Files = []ChunkFileRef{
		{FileID: f0.ID, RelPath: f0.RelPath, SizeBytes: f0.SizeBytes, Hash: f0.Hash, SampleHash: fakeHash(9001)},
		{FileID: f1.ID, RelPath: f1.RelPath, SizeBytes: f1.SizeBytes},
		{FileID: f1.ID, RelPath: "renamed/" + f1.RelPath, SizeBytes: f1.SizeBytes + 1, Hash: fakeHash(9002)},
		{FileID: 999999, RelPath: "gone.nef", SizeBytes: 5, Hash: fakeHash(9003)},
		{FileID: 999998, RelPath: "gone-unhashed.nef", SizeBytes: 6},
	}
	zero, no := time.Time{}, false
	ch.WrittenAt = &zero
	ch.VerifyOK = &no
	c.Chunks[1].Files = []ChunkFileRef{}
	c.Chunks[1].Segments = []Segment{}
	for k := range c.Plans[0].Parked {
		c.Plans[0].Parked[k] = false
	}
	// One hash in two of a plan's maps is one row.
	for k := range c.Plans[1].Overrides {
		c.Plans[1].Mapping[k] = "also mapped"
	}
	return c
}

// writeCatalogJSON writes c as dataDir/catalog.json and returns its bytes.
func writeCatalogJSON(t *testing.T, dataDir string, c *catalog) []byte {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "catalog.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return raw
}

// assertMoved checks the end state of a finished move: catalog.db holds exactly
// the original catalog, catalog.json is the stub, the original bytes are kept once
// and no staging file is left.
func assertMoved(t *testing.T, dataDir string, original []byte) {
	t.Helper()
	stub := fileBytes(t, filepath.Join(dataDir, "catalog.json"))
	if !isCatalogStub(stub) {
		t.Fatalf("catalog.json is not the stub: %s", stub)
	}
	kept, _ := filepath.Glob(filepath.Join(dataDir, catalogPreSQLite+"*"))
	if len(kept) != 1 || !bytes.Equal(fileBytes(t, kept[0]), original) {
		t.Fatalf("want one kept copy of the original catalog.json, got %v", kept)
	}
	if !strings.Contains(string(stub), filepath.Base(kept[0])) {
		t.Errorf("the stub does not name the kept copy %s", filepath.Base(kept[0]))
	}
	if stages, _ := filepath.Glob(filepath.Join(dataDir, catalogStagePrefix+"*")); len(stages) != 0 {
		t.Errorf("staging files left: %v", stages)
	}
	d, err := openCatalogDB(filepath.Join(dataDir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	got, err := exportCatalogDB(d)
	if err != nil {
		t.Fatal(err)
	}
	want := &catalog{}
	if err := decodeCatalogJSON(original, want); err != nil {
		t.Fatal(err)
	}
	if want.SchemaVersion < currentSchemaVersion {
		s := &Store{c: *want}
		s.runMigrations(want.SchemaVersion)
		*want = s.c
		// The v1 to v2 migration stamps the Locations it creates with the time it
		// runs, so a second run here differs from the move's in that stamp alone.
		if len(want.Locations) == len(got.Locations) {
			for i := range want.Locations {
				want.Locations[i].CreatedAt = got.Locations[i].CreatedAt
			}
		}
	}
	canonCatalog(want)
	canonCatalog(got)
	ws, gs := catalogSectionDigests(want), catalogSectionDigests(got)
	for _, k := range catalogKeys(ws) {
		if ws[k] != gs[k] {
			a, _ := json.Marshal(reflect.ValueOf(want).Elem().FieldByIndex(fieldIndexByTag(k)).Interface())
			b, _ := json.Marshal(reflect.ValueOf(got).Elem().FieldByIndex(fieldIndexByTag(k)).Interface())
			t.Errorf("section %s differs:\nwant %.600s\ngot  %.600s", k, a, b)
		}
	}
}

func fieldIndexByTag(tag string) []int {
	rt := reflect.TypeOf(catalog{})
	for i := 0; i < rt.NumField(); i++ {
		if name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ","); name == tag {
			return []int{i}
		}
	}
	panic(tag)
}

// Every field of every record survives the move exactly.
func TestCatalogMove_RoundTripsEveryField(t *testing.T) {
	c := fullCatalog(t)
	// The filled catalog really fills every section: each differs from empty.
	empty := catalogSectionDigests(&catalog{})
	for k, v := range catalogSectionDigests(c) {
		if empty[k] == v {
			t.Fatalf("the test catalog leaves section %s empty", k)
		}
	}
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, c)
	res, err := migrateCatalogToDB(dir, catalogMigrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Package 1 lists 5 files and 2 unknown ones, package 2 none and 2 unknown ones.
	if !res.Moved || res.Counts["file"] != len(c.Files) || res.Counts["chunk_file"] != 9 {
		t.Errorf("result %+v", res)
	}
	assertMoved(t, dir, raw)

	// Listings that repeat their file's values store them once.
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var stored, deduped, emptyHash int
	if err := d.Read(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT count(*) FROM chunk_file WHERE rel_path IS NOT NULL`).Scan(&stored); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT count(*) FROM chunk_file WHERE rel_path IS NULL AND size_bytes IS NULL AND sha256 IS NULL`).Scan(&deduped); err != nil {
			return err
		}
		return tx.QueryRow(`SELECT count(*) FROM chunk_file WHERE rel_path IS NULL AND sha256 IS NOT NULL AND length(sha256) = 0`).Scan(&emptyHash)
	}); err != nil {
		t.Fatal(err)
	}
	// Stored whole: the renamed listing, the two whose file is gone, and the four
	// unknown ones (their file IDs match no file). Shared whole: the first listing.
	// The second shares its path and size and keeps its empty hash as an empty BLOB.
	if stored != 7 || deduped != 1 || emptyHash != 1 {
		t.Errorf("listings stored whole: %d (want 7), shared whole: %d (want 1), empty hash kept: %d (want 1)",
			stored, deduped, emptyHash)
	}
}

// The move continues each ID counter from the catalog's next_id, so an ID is never
// reused even when the newest record was removed.
func TestCatalogMove_IDSequencesContinue(t *testing.T) {
	c := fullCatalog(t)
	c.NextID["file"] = 500000
	dir := t.TempDir()
	writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var id int64
	if err := d.Write(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO file(ord, collection_id, folder_id, rel_path, size_bytes, hash_alg, role,
			camera_serial, event_id) VALUES(0, 1, 1, 'new.nef', 1, 'SHA256', '', '', 0)`)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if id != 500001 {
		t.Errorf("a new file got id %d, want 500001", id)
	}
}

// The shipped fixtures: their placeholder hashes are refused, and with real hashes
// both move, the schema-v1 one through the in-memory migrations.
func TestCatalogMove_Fixtures(t *testing.T) {
	for _, name := range []string{"catalog_current.json", "catalog_schema1.json"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "catalog.json"), raw, 0o644)
			before := dirSnapshot(t, dir)
			if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err == nil || !strings.Contains(err.Error(), "lowercase hexadecimal") {
				t.Fatalf("placeholder hashes: %v", err)
			}
			if after := dirSnapshot(t, dir); after != before {
				t.Fatalf("a refused move changed the folder:\n%s\n%s", before, after)
			}

			fixed := bytes.ReplaceAll(raw, []byte(`"h1"`), []byte(`"`+fakeHash(1)+`"`))
			fixed = bytes.ReplaceAll(fixed, []byte(`"h2"`), []byte(`"`+fakeHash(2)+`"`))
			os.WriteFile(filepath.Join(dir, "catalog.json"), fixed, 0o644)
			if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
				t.Fatal(err)
			}
			assertMoved(t, dir, fixed)
			kept, _ := filepath.Glob(filepath.Join(dir, catalogPreSQLite+"*"))
			var c catalog
			json.Unmarshal(fixed, &c)
			if want := fmt.Sprintf("%s%d-", catalogPreSQLite, c.SchemaVersion); !strings.HasPrefix(filepath.Base(kept[0]), want) {
				t.Errorf("kept copy %s, want the name to start %s", kept[0], want)
			}
		})
	}
}

// Every fault refuses the move and leaves the folder exactly as it was: no
// catalog.db, no staging files, no kept copy, catalog.json untouched.
func TestCatalogMove_RefusesFaults(t *testing.T) {
	flip := func(q string) func(*sql.DB) error {
		return func(db *sql.DB) error { _, err := db.Exec(q); return err }
	}
	type fault struct {
		name   string
		edit   func(raw []byte) []byte
		damage func(*sql.DB) error
		want   string
	}
	c := fullCatalog(t)
	firstFile := `"rel_path":"` + c.Files[0].RelPath + `"`
	faults := []fault{
		{name: "dropped file row", damage: flip(`DELETE FROM file WHERE id = (SELECT max(id) FROM file)`),
			want: "file has"},
		{name: "dropped snapshot file row", damage: flip(`DELETE FROM snap_file WHERE ord = 1`), want: "snap_file has"},
		{name: "dropped version row", damage: flip(`DELETE FROM file_version WHERE seq = 1`), want: "file_version has"},
		{name: "dropped copy row", damage: flip(`DELETE FROM copy WHERE ord = 1`), want: "copy has"},
		{name: "flipped file hash", want: "files",
			damage: flip(`UPDATE file SET sha256 = zeroblob(32) WHERE id = (SELECT min(id) FROM file)`)},
		{name: "flipped snapshot hash", want: "volume_snapshots",
			damage: flip(`UPDATE snap_file SET sha256 = zeroblob(32) WHERE ord = 0`)},
		{name: "bent time", want: "audit", damage: flip(`UPDATE audit SET at = at + 1 WHERE id = 1`)},
		{name: "unknown key", want: "does not know: catalog.files[0].colour",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(firstFile), []byte(firstFile+`,"colour":"red"`), 1)
			}},
		{name: "differently-cased key", want: "does not know: catalog.files[0].Rel_Path",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(`"rel_path":`), []byte(`"Rel_Path":`), 1)
			}},
		{name: "duplicate key", want: "twice",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(firstFile), []byte(firstFile+`,"rel_path":"other.nef"`), 1)
			}},
		{name: "uppercase hash", want: "lowercase hexadecimal",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(c.Files[0].Hash), []byte(strings.ToUpper(c.Files[0].Hash)), 1)
			}},
		{name: "short hash", want: "lowercase hexadecimal",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(c.Files[0].Hash), []byte(c.Files[0].Hash[:40]), 1)
			}},
		{name: "newer catalog", want: "newer version",
			edit: func(raw []byte) []byte {
				return bytes.Replace(raw, []byte(`"schema_version":8`), []byte(`"schema_version":9`), 1)
			}},
		{name: "data after the catalog", want: "more data", edit: func(raw []byte) []byte { return append(raw, `{}`...) }},
		{name: "empty", want: "empty", edit: func([]byte) []byte { return []byte{} }},
		{name: "already a stub", want: "already moved", edit: func([]byte) []byte {
			b, _ := json.Marshal(catalogStub{SchemaVersion: catalogStubSchemaVersion, MovedTo: catalogDBName})
			return b
		}},
	}
	for _, f := range faults {
		t.Run(f.name, func(t *testing.T) {
			dir := t.TempDir()
			raw := writeCatalogJSON(t, dir, fullCatalog(t))
			if f.edit != nil {
				raw = f.edit(raw)
				os.WriteFile(filepath.Join(dir, "catalog.json"), raw, 0o644)
			}
			before := dirSnapshot(t, dir)
			_, err := migrateCatalogToDB(dir, catalogMigrateOptions{afterImport: f.damage})
			if err == nil {
				t.Fatal("the move succeeded")
			}
			if f.name == "already a stub" {
				// A stub with no catalog.db is its own refusal.
				if !strings.Contains(err.Error(), "is missing") {
					t.Errorf("error %q", err)
				}
			} else if !strings.Contains(err.Error(), f.want) {
				t.Errorf("error %q does not mention %q", err, f.want)
			}
			if after := dirSnapshot(t, dir); after != before {
				t.Errorf("a refused move changed the folder:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}

// A move stopped at any step, as if the process died there, is finished by the
// next run, and a third run finds nothing to do.
func TestCatalogMove_ResumesAfterStop(t *testing.T) {
	for _, step := range []string{"imported", "backup", "publish", "stub"} {
		t.Run(step, func(t *testing.T) {
			dir := t.TempDir()
			raw := writeCatalogJSON(t, dir, fullCatalog(t))
			if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{stopAt: step}); !errors.Is(err, errCatalogMoveStopped) {
				t.Fatalf("stop at %s: %v", step, err)
			}
			if !bytes.Equal(fileBytes(t, filepath.Join(dir, "catalog.json")), raw) {
				t.Fatal("catalog.json changed before the last step")
			}
			res, err := migrateCatalogToDB(dir, catalogMigrateOptions{})
			if err != nil {
				t.Fatalf("resume: %v", err)
			}
			if step == "stub" && !res.Finished || step != "stub" && !res.Moved {
				t.Errorf("resume result %+v", res)
			}
			assertMoved(t, dir, raw)
			if res, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil || !res.Already {
				t.Errorf("third run: %+v, %v", res, err)
			}
			assertMoved(t, dir, raw)
		})
	}
}

// When catalog.db exists but catalog.json is not the one it was made from (an
// older Obelisk wrote it after the move), the move refuses to choose.
func TestCatalogMove_RefusesChangedJSONAfterPublish(t *testing.T) {
	dir := t.TempDir()
	writeCatalogJSON(t, dir, fullCatalog(t))
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{stopAt: "stub"}); !errors.Is(err, errCatalogMoveStopped) {
		t.Fatal(err)
	}
	other := fullCatalog(t)
	other.Collections[0].Name = "written after the move"
	writeCatalogJSON(t, dir, other)
	before := dirSnapshot(t, dir)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err == nil || !strings.Contains(err.Error(), "Refusing to choose") {
		t.Fatalf("got %v", err)
	}
	if after := dirSnapshot(t, dir); after != before {
		t.Errorf("the refusal changed the folder:\n%s\n%s", before, after)
	}
}

// A stub without its catalog.db is refused, and nothing is created.
func TestCatalogMove_StubWithoutDatabaseRefused(t *testing.T) {
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, fullCatalog(t))
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, catalogDBName), filepath.Join(t.TempDir(), "away.db")); err != nil {
		t.Fatal(err)
	}
	before := dirSnapshot(t, dir)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("got %v", err)
	}
	if after := dirSnapshot(t, dir); after != before {
		t.Errorf("the refusal changed the folder")
	}
	_ = raw
}

// A deleted catalog.json after the move gets its stub back, so an older Obelisk
// never starts an empty catalog there.
func TestCatalogMove_MissingJSONGetsItsStubBack(t *testing.T) {
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, fullCatalog(t))
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "catalog.json"))
	res, err := migrateCatalogToDB(dir, catalogMigrateOptions{})
	if err != nil || !res.Finished {
		t.Fatalf("%+v %v", res, err)
	}
	if !isCatalogStub(fileBytes(t, filepath.Join(dir, "catalog.json"))) {
		t.Fatal("no stub")
	}
	if kept, _ := filepath.Glob(filepath.Join(dir, catalogPreSQLite+"*")); len(kept) != 1 || !bytes.Equal(fileBytes(t, kept[0]), raw) {
		t.Fatalf("kept copies %v", kept)
	}
}

// The JSON store of this build, and so every build since schema versioning, opens
// the moved folder read-only and writes nothing to it.
func TestCatalogMove_StubOpensReadOnlyInTheJSONStore(t *testing.T) {
	t.Setenv(catalogEngineEnv, "json")
	dir := t.TempDir()
	writeCatalogJSON(t, dir, fullCatalog(t))
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	stub := fileBytes(t, filepath.Join(dir, "catalog.json"))
	s, err := openTestStore(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	ro, why := s.ReadOnly()
	if !ro || !strings.Contains(why, "newer version") {
		t.Fatalf("ReadOnly = %v, %q", ro, why)
	}
	s.AddCollection("must not be saved")
	if !bytes.Equal(stub, fileBytes(t, filepath.Join(dir, "catalog.json"))) {
		t.Error("the JSON store wrote over the stub")
	}
	if baks, _ := filepath.Glob(filepath.Join(dir, "catalog.json.bak-*")); len(baks) != 0 {
		t.Errorf("the JSON store made daily backups of the stub: %v", baks)
	}
}

// The portable export is a catalog.json the JSON store opens, holding the same
// catalog.
func TestCatalogDB_ExportJSONIsPortable(t *testing.T) {
	dir := t.TempDir()
	c := fullCatalog(t)
	writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = exportCatalogJSON(d, &buf)
	d.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkCatalogJSONKeys(buf.Bytes()); err != nil {
		t.Fatalf("the export has keys a catalog does not: %v", err)
	}
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "catalog.json"), buf.Bytes(), 0o644)
	s, err := openTestStore(t, other)
	if err != nil {
		t.Fatal(err)
	}
	if ro, why := s.ReadOnly(); ro {
		t.Fatalf("the export opened read-only: %s", why)
	}
	if got := len(testFiles(t, s.AllFiles())); got != len(c.Files) {
		t.Errorf("the export has %d files, want %d", got, len(c.Files))
	}
	var back catalog
	if err := decodeCatalogJSON(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	canonCatalog(c)
	canonCatalog(&back)
	ws, gs := catalogSectionDigests(c), catalogSectionDigests(&back)
	for k := range ws {
		if ws[k] != gs[k] {
			t.Errorf("section %s differs in the export", k)
		}
	}
}

// TestCatalogDBRoundTrip1M moves a catalog of a million files, each listed in a
// package, into a database and proves it. SKIPPED by default; CI runs it:
//
//	OBELISK_SCALE=1 go test -run TestCatalogDBRoundTrip1M -v -timeout 30m
func TestCatalogDBRoundTrip1M(t *testing.T) {
	if os.Getenv("OBELISK_SCALE") == "" {
		t.Skip("set OBELISK_SCALE=1 to run the million-file move")
	}
	n := envInt("OBELISK_FILES", 1_000_000)
	t0 := time.Now()
	c := &catalog{SchemaVersion: currentSchemaVersion, NextID: map[string]int{"file": n, "collection": 1, "folder": 1, "chunk": 10}}
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Collections = []*Collection{{ID: 1, Name: "Scale", CreatedAt: created}}
	c.Folders = []*Folder{{ID: 1, CollectionID: 1, Path: "/srv/scale"}}
	c.Files = make([]*File, n)
	for i := range c.Files {
		c.Files[i] = &File{ID: i + 1, CollectionID: 1, FolderID: 1,
			RelPath:   fmt.Sprintf("Clients/%04d/RAW/DSC_%08d.NEF", i%5000, i),
			SizeBytes: 38_000_000 + int64(i%1000), HashAlg: "SHA256", Hash: fakeHash(i), Blake3: fakeHash(i + n),
			ModTime: created.Add(time.Duration(i) * time.Second), FirstSeen: created, Role: RoleOriginals}
	}
	ok := true
	for k := 0; k < 10; k++ {
		ch := &Chunk{ID: k + 1, CollectionID: 1, Name: fmt.Sprintf("MIRROR-%d", k+1), Status: StatusAdoptedVerified,
			Mirror: true, Encrypted: false, CreatedAt: created,
			Copies: []Copy{{VolumeID: k + 1, Path: "M:/mirror", VerifyOK: &ok, LastVerifiedAt: &created}}}
		for i := k; i < n; i += 10 {
			f := c.Files[i]
			ch.Files = append(ch.Files, ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: f.SizeBytes, Hash: f.Hash,
				SampleHash: fakeHash(i + 2*n)})
		}
		ch.FileCount = len(ch.Files)
		c.Chunks = append(c.Chunks, ch)
	}
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, c)
	build := time.Since(t0)

	t0 = time.Now()
	res, err := migrateCatalogToDB(dir, catalogMigrateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	move := time.Since(t0)
	if res.Counts["file"] != n || res.Counts["chunk_file"] != n {
		t.Fatalf("counts %v", res.Counts)
	}
	fi, err := os.Stat(filepath.Join(dir, catalogDBName))
	if err != nil {
		t.Fatal(err)
	}
	t0 = time.Now()
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	open := time.Since(t0)
	var dedup int
	if err := d.Read(func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM chunk_file WHERE rel_path IS NULL AND size_bytes IS NULL AND sha256 IS NULL`).Scan(&dedup)
	}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if dedup != n {
		t.Errorf("%d of %d listings share their file's values", dedup, n)
	}
	t.Logf("\n=== Move of %s files and %s package listings ===\n"+
		"catalog.json             %10.1f MB\n"+
		"catalog.db               %10.1f MB\n"+
		"move (import + check)    %10s\n"+
		"open the database        %10s\n"+
		"(building the test catalog took %s)",
		comma(n), comma(n), float64(len(raw))/1e6, float64(fi.Size())/1e6, round(move), open, round(build))
}

// A sourceless archive's union of drives keeps two files at the same path when
// their bytes differ (a camera counter that wrapped): the move keeps both.
func TestCatalogMove_UnionArchiveSamePathTwice(t *testing.T) {
	c := fullCatalog(t)
	coll := c.Collections[0]
	for i := 0; i < 2; i++ {
		c.Files = append(c.Files, &File{ID: 900000 + i, CollectionID: coll.ID, FolderID: 0,
			RelPath: "DCIM/100CANON/IMG_0001.JPG", SizeBytes: int64(1000 + i), HashAlg: "SHA256",
			Hash: fakeHash(800000 + i)})
	}
	c.NextID["file"] = 900002
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatalf("a union archive with one path twice: %v", err)
	}
	assertMoved(t, dir, raw)
}

// A write to catalog.json while the move runs (another Obelisk still open) is
// never lost: before publishing, the move abandons and keeps the change; after
// publishing, it refuses to replace catalog.json.
func TestCatalogMove_RefusesWhenCatalogJSONChangesDuringMove(t *testing.T) {
	changed := fullCatalog(t)
	changed.Collections[0].Name = "written while the move ran"

	t.Run("during the import", func(t *testing.T) {
		dir := t.TempDir()
		writeCatalogJSON(t, dir, fullCatalog(t))
		var newer []byte
		_, err := migrateCatalogToDB(dir, catalogMigrateOptions{afterImport: func(*sql.DB) error {
			newer = writeCatalogJSON(t, dir, changed)
			return nil
		}})
		if err == nil || !strings.Contains(err.Error(), "changed while") {
			t.Fatalf("got %v", err)
		}
		if !bytes.Equal(fileBytes(t, filepath.Join(dir, "catalog.json")), newer) {
			t.Fatal("the change to catalog.json was lost")
		}
		for _, pattern := range []string{catalogDBName, catalogStagePrefix + "*", catalogPreSQLite + "*"} {
			if m, _ := filepath.Glob(filepath.Join(dir, pattern)); len(m) != 0 {
				t.Errorf("an abandoned move left %v", m)
			}
		}
	})
	t.Run("after publishing", func(t *testing.T) {
		dir := t.TempDir()
		writeCatalogJSON(t, dir, fullCatalog(t))
		var newer []byte
		_, err := migrateCatalogToDB(dir, catalogMigrateOptions{beforeStub: func() error {
			newer = writeCatalogJSON(t, dir, changed)
			return nil
		}})
		if err == nil || !strings.Contains(err.Error(), "Refusing to choose") {
			t.Fatalf("got %v", err)
		}
		if !bytes.Equal(fileBytes(t, filepath.Join(dir, "catalog.json")), newer) {
			t.Fatal("the change to catalog.json was replaced by the stub")
		}
	})
}

// Two moves of one folder at once: the second refuses while the first holds the
// folder, and nothing of the first is swept away.
func TestCatalogMove_OneMoveAtATime(t *testing.T) {
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, fullCatalog(t))
	var second error
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{afterImport: func(*sql.DB) error {
		_, second = migrateCatalogToDB(dir, catalogMigrateOptions{})
		return nil
	}}); err != nil {
		t.Fatalf("first move: %v", err)
	}
	if second == nil || !strings.Contains(second.Error(), "another") {
		t.Fatalf("the second move during the first: %v", second)
	}
	assertMoved(t, dir, raw)
	if m, _ := filepath.Glob(filepath.Join(dir, "*lock*")); len(m) != 0 {
		t.Errorf("the move left its lock behind: %v", m)
	}
}

// A package listing that shares its file's path, size and hash keeps them when
// that file later changes or is removed: the package still holds the old bytes.
func TestCatalogDB_ListingsKeepTheirValuesWhenTheFileChanges(t *testing.T) {
	c := fullCatalog(t)
	dir := t.TempDir()
	writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	before, err := exportCatalogDB(d)
	if err != nil {
		t.Fatal(err)
	}
	want := before.Chunks[0].Files
	f0, f1 := c.Files[0], c.Files[1]
	newHash, _ := hashBytes(fakeHash(777777))
	if err := d.Write(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE file SET sha256 = ?, size_bytes = size_bytes + 1, rel_path = 'moved/' || rel_path
			WHERE id = ?`, newHash, f0.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE file SET sha256 = ? WHERE id = ?`, newHash, f1.ID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM file WHERE id = ?`, f1.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	after, err := exportCatalogDB(d)
	if err != nil {
		t.Fatalf("export after the file changed: %v", err)
	}
	if !reflect.DeepEqual(after.Chunks[0].Files, want) {
		t.Errorf("the package's listing changed with its file:\nbefore %+v\nafter  %+v", want, after.Chunks[0].Files)
	}
}

// Replacing a file row (INSERT OR REPLACE, the usual upsert) also keeps the
// listings that share its values: SQLite runs delete triggers for the row a
// REPLACE removes only with recursive triggers on, which every writer has.
func TestCatalogDB_ListingsKeepTheirValuesWhenTheFileIsReplaced(t *testing.T) {
	c := fullCatalog(t)
	dir := t.TempDir()
	writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	before, err := exportCatalogDB(d)
	if err != nil {
		t.Fatal(err)
	}
	f0 := c.Files[0]
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT OR REPLACE INTO file(id, ord, collection_id, folder_id, rel_path, size_bytes,
			hash_alg, role, camera_serial, event_id) VALUES(?, 0, ?, ?, 'replaced.nef', 1, 'SHA256', '', '', 0)`,
			f0.ID, f0.CollectionID, f0.FolderID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	after, err := exportCatalogDB(d)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Chunks[0].Files, before.Chunks[0].Files) {
		t.Errorf("replacing the file changed the package's listing:\nbefore %+v\nafter  %+v",
			before.Chunks[0].Files, after.Chunks[0].Files)
	}
}

// Times outside what nanoseconds hold move exactly: a file dated 1601 (the zero
// FILETIME Windows reports) and an audit entry dated 2300.
func TestCatalogMove_KeepsTimesOutsideNanoseconds(t *testing.T) {
	c := fullCatalog(t)
	c.Files[0].ModTime = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Audit[0].At = time.Date(2300, 1, 2, 3, 4, 5, 6, time.UTC)
	dir := t.TempDir()
	raw := writeCatalogJSON(t, dir, c)
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertMoved(t, dir, raw)
}
