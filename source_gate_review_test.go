//go:build !guionly

package main

// source_gate_review_test.go: un-quarantine creates nothing before deciding, the data
// folder is never a source, and a new output folder is held before it is created.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A refused un-quarantine must leave nothing inside the source.
func TestUnQuarantine_CreatesNothingInsideASource(t *testing.T) {
	app, root, _ := quarantineScenario(t)
	sub := filepath.Join(root, "raw", "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "f.txt")
	mustWrite(t, file, "kept")
	e, err := app.Quarantine(file, "user", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "raw", "sub")); err != nil {
		t.Fatal(err)
	}
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, filepath.Join(root, "raw"))
	if _, err := app.UnQuarantine(e.ID); err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("un-quarantine into a source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "raw", "sub")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused un-quarantine created a folder inside the source: %v", err)
	}
}

// With no source in the way, the missing parents are still made.
func TestUnQuarantine_StillRecreatesMissingParents(t *testing.T) {
	app, root, _ := quarantineScenario(t)
	sub := filepath.Join(root, "raw", "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "f.txt")
	mustWrite(t, file, "kept")
	e, err := app.Quarantine(file, "user", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "raw", "sub")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UnQuarantine(e.ID); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, file, "kept")
}

// Obelisk writes to its data folder all the time, so it cannot be a source.
func TestRegisterSource_RefusesTheDataFolder(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	inside := filepath.Join(app.DataDir, "inside")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, app.DataDir)
	for name, p := range map[string]string{
		"the data folder":     app.DataDir,
		"a folder holding it": filepath.Dir(app.DataDir),
		"a folder inside it":  inside,
		"another name for it": link,
	} {
		if err := tryRegisterSource(app.Store, coll.ID, p); err == nil || !strings.Contains(err.Error(), "data folder") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := app.Store.SourceRoots(); len(got) != 0 {
		t.Fatalf("recorded: %v", got)
	}
	if err := tryRegisterSource(app.Store, coll.ID, t.TempDir()); err != nil {
		t.Fatalf("an unrelated folder: %v", err)
	}
}

// A catalog from before this rule keeps working: its source can still be scanned.
func TestRegisterSource_ExistingRecordIsNotLockedOut(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	holder := filepath.Dir(app.DataDir)
	app.Store.addFolderRecord(coll.ID, holder)
	if err := tryRegisterSource(app.Store, coll.ID, holder); err != nil {
		t.Fatalf("re-scanning a source already in the catalog: %v", err)
	}
}

// Where the guard is missing, a new folder is held before it is made.
func TestUnguardedOutput_HeldBeforeItIsCreated(t *testing.T) {
	withoutDestGuard(t)
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	base := t.TempDir()
	out := filepath.Join(base, "new")
	var regErr error
	ran := false
	afterPathReserved = func(string) {
		if !ran {
			ran = true
			regErr = tryRegisterSource(app.Store, coll.ID, base)
		}
	}
	t.Cleanup(func() { afterPathReserved = nil })
	o, err := app.openOutFolder(out)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if !ran || regErr == nil {
		t.Fatalf("ran %v, registering the parent: %v", ran, regErr)
	}
}
