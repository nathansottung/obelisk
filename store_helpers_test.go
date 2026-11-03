package main

import (
	"encoding/json"
	"iter"
	"os"
	"path/filepath"
	"testing"
)

// testFiles collects what a file iterator yields, failing the test on an error.
func testFiles(t testing.TB, seq iter.Seq2[*File, error]) []*File {
	t.Helper()
	files, err := collectFiles(seq)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// noErr returns a function that unwraps a (value, error) result, failing the test
// on the error: noErr[Census](t)(app.FormatCensus(0)).
func noErr[T any](t testing.TB) func(T, error) T {
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

// openTestStore opens dataDir's store with the engine the run chose (see
// catalogEngineEnv) and closes it when the test ends, so a catalog database never
// holds a test's temporary folder open.
func openTestStore(t testing.TB, dataDir string) (*Store, error) {
	t.Helper()
	s, err := OpenStore(dataDir)
	if s != nil {
		t.Cleanup(func() { s.Close() })
	}
	return s, err
}

// sqliteEngineRun reports whether this test run uses the catalog database.
func sqliteEngineRun() bool {
	engine, _ := catalogEngine()
	return engine == "sqlite"
}

// jsonEngineOnly skips a test that is about catalog.json itself (its bytes, its
// daily backups, app backups that carry it) when the run uses the database
// engine. why names the slice that brings the database's own version of it.
func jsonEngineOnly(t testing.TB, why string) {
	t.Helper()
	if sqliteEngineRun() {
		t.Skip("catalog.json behaviour; on the database engine: " + why)
	}
}

// persistedCatalog reads what dataDir's catalog holds on disk (catalog.json, or
// the catalog database's last commit, whichever the run uses), not what a store
// holds in memory.
func persistedCatalog(t testing.TB, dataDir string) *catalog {
	t.Helper()
	var c catalog
	if !sqliteEngineRun() {
		raw, err := os.ReadFile(filepath.Join(dataDir, "catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeCatalogJSON(raw, &c); err != nil {
			t.Fatal(err)
		}
		return &c
	}
	return persistedCatalogFromDB(t, dataDir)
}

// persistedCatalogFromDB reads the catalog database (store_engine_test.go); the
// launcher-only build has no database.
var persistedCatalogFromDB func(t testing.TB, dataDir string) *catalog

// persistedCatalogText is persistedCatalog as JSON, for a test that looks for a
// value in what was saved.
func persistedCatalogText(t testing.TB, dataDir string) string {
	t.Helper()
	b, err := json.Marshal(persistedCatalog(t, dataDir))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// catalogFileName is the file the run keeps its catalog in.
func catalogFileName() string {
	if sqliteEngineRun() {
		return "catalog.db"
	}
	return "catalog.json"
}

// testHash is a stand-in SHA-256 for test records: the hash of s, so equal names
// give equal hashes and the database stores it like a real one.
func testHash(s string) string { return sha256Hex([]byte(s)) }

// pkgFiles is a package's file listing, read through the store as on either
// engine (the database engine keeps none on the record).
func pkgFiles(t testing.TB, s *Store, c *Chunk) []ChunkFileRef {
	t.Helper()
	refs, err := s.ChunkFiles(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	return refs
}

// AddChunk adds a package for a test, failing loudly when it cannot be recorded
// (the app itself uses AddChunkErr).
func (s *Store) AddChunk(c Chunk) *Chunk {
	nc, err := s.AddChunkErr(c)
	if err != nil {
		panic(err)
	}
	return nc
}

// testPlanMaps reads a plan's maps, failing the test on an error.
func testPlanMaps(t testing.TB, s *Store, planID int) *PlanMaps {
	t.Helper()
	m, err := s.PlanMaps(planID)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// testSnapFiles reads a drive snapshot's files, failing the test on an error.
func testSnapFiles(t testing.TB, s *Store, volumeID int) []SnapFile {
	t.Helper()
	files, err := s.SnapshotFileList(volumeID)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// AddPlan is AddPlanErr for tests, which panics on an error.
func (s *Store) AddPlan(p *Plan) *Plan {
	np, err := s.AddPlanErr(p)
	if err != nil {
		panic(err)
	}
	return np
}
