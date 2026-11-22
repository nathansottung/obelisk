//go:build !guionly

package main

// source_gate_writers_test.go: the real writers that do not hold a DestRoot keep
// their folder or file reserved while they work.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The real build holds its staging folder.
func TestBuild_StagingCannotBecomeASourceMidBuild(t *testing.T) {
	s := newIT(t)
	coll := s.app.Store.AddCollection("Live")
	var regErr error
	held := ""
	afterPathReserved = func(p string) {
		if held == "" {
			held = p
			regErr = tryRegisterSource(s.app.Store, coll.ID, p)
		}
	}
	t.Cleanup(func() { afterPathReserved = nil })
	builtPlainPackageForTest(s, map[string][]byte{"a.txt": []byte("one\n")})
	if held == "" {
		t.Fatal("the build did not reach its staging folder")
	}
	if regErr == nil {
		t.Fatal("the staging folder became a source during a build")
	}
	if n := s.app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("%d outputs still live after the build", n)
	}
}

// The real escrow fetch holds its cache folder.
func TestEscrowFetch_CacheCannotBecomeASourceMidFetch(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	var regErr error
	held := ""
	afterPathReserved = func(p string) {
		held = p
		regErr = tryRegisterSource(app.Store, coll.ID, p)
	}
	t.Cleanup(func() { afterPathReserved = nil })
	_, _ = app.FetchEscrowCache(false, Census{}, nil) // a development build stops here
	if held == "" {
		t.Fatal("the fetch did not reach its cache folder")
	}
	if regErr == nil {
		t.Fatal("the escrow cache became a source during a fetch")
	}
}

// Two different names for one object overlap by identity alone, and a file ID of
// zero (some network shares) never matches.
func TestGateFolder_OverlapByIdentity(t *testing.T) {
	folder := gateFolder{canonical: filepath.Join("x", "one"), chain: []idEntry{{"one", fileIdent{vol: 1, idx: 5}}}}
	inside := gateFolder{canonical: filepath.Join("y", "two"), chain: []idEntry{{"two", fileIdent{vol: 1, idx: 6}}, {"one", fileIdent{vol: 1, idx: 5}}}}
	other := gateFolder{canonical: filepath.Join("z", "three"), chain: []idEntry{{"three", fileIdent{vol: 2, idx: 5}}}}
	if !folder.overlaps(inside) || !inside.overlaps(folder) {
		t.Error("a folder inside another by identity did not overlap")
	}
	if folder.overlaps(other) {
		t.Error("the same file ID on another volume overlapped")
	}
	noID := gateFolder{canonical: filepath.Join("p", "a"), chain: []idEntry{{"a", fileIdent{vol: 1}}}}
	noID2 := gateFolder{canonical: filepath.Join("q", "b"), chain: []idEntry{{"b", fileIdent{vol: 1}}}}
	if noID.overlaps(noID2) {
		t.Error("two folders with no file ID matched")
	}
}

func TestBurn_StagedFolderCannotBecomeASourceMidBurn(t *testing.T) {
	app, qid, staged := burnSetupForTest(t, "Disc")
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	coll := app.Store.AddCollection("Live")
	var regErr error
	ran := false
	prev := runBurnProgram
	runBurnProgram = func([]string) error {
		ran = true
		regErr = tryRegisterSource(app.Store, coll.ID, staged)
		return errStopBurnForTest
	}
	t.Cleanup(func() { runBurnProgram = prev })
	_, _ = app.BurnNext(qid, func(float64, string) {})
	if !ran {
		t.Fatal("the burn command did not run")
	}
	if regErr == nil {
		t.Fatal("the staged folder became a source during its burn")
	}
	if n := app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("%d outputs still live after the burn", n)
	}
}

func TestBurnCarry_RefusesANextDiscThatBecameASource(t *testing.T) {
	app := dockApp(t)
	next := t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, next)
	c := app.Store.AddChunk(Chunk{Name: "first", Status: "BUILT"})
	nc := app.Store.AddChunk(Chunk{Name: "second", Status: "BUILT", StagedDir: next})
	current := &BurnDisc{ChunkID: c.ID, Status: "BURNING"}
	q := &BurnQueue{Discs: []*BurnDisc{current, {ChunkID: nc.ID, Status: "PENDING"}}}
	ecc := filepath.Join(t.TempDir(), "first.ecc")
	mustWrite(t, ecc, "ECC")
	app.carryEccToNextDisc(q, current, c, ecc)
	if _, err := os.Stat(filepath.Join(next, "first.ecc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the ECC file was written into a registered source: %v", err)
	}
}

func TestKeystoreSync_RefusedInsideASourceRegisteredSinceConfig(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("K-1", "one")}, []map[string]any{kvKey("K-2", "two")})
	f.app.Store.AddFolder(f.app.Store.AddCollection("Live").ID, filepath.Dir(f.paths[1]))
	writes := 0
	f.app.keystoreMutationObserver = func(string) { writes++ }
	before := kvSnapshot(t, f.root)
	if _, err := f.app.SyncKeystores(); err == nil {
		t.Fatal("a keystore inside a registered source was written")
	}
	if writes != 0 {
		t.Fatalf("%d keystore(s) were written before the refusal", writes)
	}
	if !reflect.DeepEqual(before, kvSnapshot(t, f.root)) {
		t.Fatal("the keystores changed")
	}
}

func TestKeystoreGenerate_RefusedInsideASourceRegisteredSinceConfig(t *testing.T) {
	f := newKV(t, []map[string]any{}, []map[string]any{})
	f.app.Store.AddFolder(f.app.Store.AddCollection("Live").ID, filepath.Dir(f.paths[1]))
	before := kvSnapshot(t, f.root)
	if _, _, _, err := f.app.GenerateKey("x"); err == nil {
		t.Fatal("a key was written into a registered source")
	}
	if !reflect.DeepEqual(before, kvSnapshot(t, f.root)) {
		t.Fatal("the keystores changed")
	}
}

func TestKeystoreWrite_HoldsItsFileAgainstANewSource(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("K-1", "one")}, []map[string]any{kvKey("K-2", "two")})
	coll := f.app.Store.AddCollection("Live")
	beside := filepath.Join(filepath.Dir(f.paths[0]), "beside")
	if err := os.Mkdir(beside, 0o755); err != nil {
		t.Fatal(err)
	}
	var holding, besideErr error
	seen := false
	f.app.keystoreMutationObserver = func(string) {
		if !seen {
			seen = true
			holding = tryRegisterSource(f.app.Store, coll.ID, filepath.Dir(f.paths[0]))
			besideErr = tryRegisterSource(f.app.Store, coll.ID, beside)
		}
	}
	if _, err := f.app.SyncKeystores(); err != nil {
		t.Fatal(err)
	}
	if holding == nil {
		t.Error("the folder holding a keystore being written became a source")
	}
	if besideErr != nil {
		t.Errorf("a folder beside the keystore was refused: %v", besideErr)
	}
	if n := f.app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("%d outputs still live", n)
	}
}

func TestQuarantine_TargetCannotBecomeASourceMidMove(t *testing.T) {
	app, root, _ := quarantineScenario(t)
	coll := app.Store.AddCollection("Live")
	target := filepath.Join(root, "raw")
	var regErr error
	quarantineBeforeMove = func() { regErr = tryRegisterSource(app.Store, coll.ID, target) }
	t.Cleanup(func() { quarantineBeforeMove = nil })
	_, err := app.Quarantine(target, "user", "test")
	if regErr == nil && err == nil {
		t.Fatal("the folder became a source and was still moved to quarantine")
	}
}

func TestUnQuarantine_TargetCannotBecomeASourceMidMove(t *testing.T) {
	app, root, _ := quarantineScenario(t)
	coll := app.Store.AddCollection("Live")
	target := filepath.Join(root, "raw")
	e, err := app.Quarantine(target, "user", "test")
	if err != nil {
		t.Fatal(err)
	}
	var regErr error
	quarantineBeforeMove = func() { regErr = tryRegisterSource(app.Store, coll.ID, root) }
	t.Cleanup(func() { quarantineBeforeMove = nil })
	_, err = app.UnQuarantine(e.ID)
	if regErr == nil && err == nil {
		t.Fatal("a folder was moved back into a source registered during the move")
	}
}

func TestCreateDestination_SeesASourceRegisteredSinceItsCheck(t *testing.T) {
	app := dockApp(t)
	base := t.TempDir()
	dest := filepath.Join(base, "new", "deep")
	coll := app.Store.AddCollection("Live")
	c := app.Store.CheckDestination(dest)
	if c.Status != destStatusCreate {
		t.Fatalf("status %q: %s", c.Status, c.Reason)
	}
	destBeforeCreate = func() {
		if err := tryRegisterSource(app.Store, coll.ID, base); err != nil {
			t.Errorf("registering: %v", err)
		}
	}
	t.Cleanup(func() { destBeforeCreate = nil })
	_, err := app.Store.CreateDestination(dest, c.Folder, true)
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("creating a folder inside a new source: %v", err)
	}
	if _, serr := os.Stat(filepath.Join(base, "new")); !errors.Is(serr, os.ErrNotExist) {
		t.Fatalf("a folder was created inside the source: %v", serr)
	}
}
