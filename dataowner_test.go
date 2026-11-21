//go:build !guionly

package main

// dataowner_test.go — one process owns a data folder. lockDataDir stands in for a
// second process: it ignores this process's own holds, exactly as another process's
// lock call would see them.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assertSecondRefused fails unless a second owner of dir is refused with the
// in-use error, naming this process.
func assertSecondRefused(t *testing.T, dir, when string) {
	t.Helper()
	f, err := lockDataDir(dir)
	if err == nil {
		f.Close()
		t.Fatalf("%s: a second owner of %s was not refused", when, dir)
	}
	var inUse *DataDirInUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("%s: got %v, want the in-use refusal", when, err)
	}
	if inUse.PID != os.Getpid() {
		t.Errorf("%s: the refusal names process %d, want this one (%d)", when, inUse.PID, os.Getpid())
	}
	msg := err.Error()
	for _, want := range []string{dir, "Another Obelisk", "Only one Obelisk", "did not open it"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s: the refusal %q does not say %q", when, msg, want)
		}
	}
}

func assertOwnable(t *testing.T, dir, when string) {
	t.Helper()
	f, err := lockDataDir(dir)
	if err != nil {
		t.Fatalf("%s: %s should be free: %v", when, dir, err)
	}
	unlockFile(f)
	f.Close()
}

// An open catalog owns its folder, on both engines; closing it gives the folder up.
func TestOwner_OpenCatalogRefusesASecondOwner(t *testing.T) {
	for _, engine := range []string{"sqlite", "json"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			dir := t.TempDir()
			s, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			s.AddCollection("Kept")
			assertSecondRefused(t, dir, "while the catalog is open")
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			assertOwnable(t, dir, "after the catalog closed")
		})
	}
}

// Holds in one process are counted: the folder stays owned until the last is given up.
func TestOwner_HoldsAreCounted(t *testing.T) {
	dir := t.TempDir()
	r1, err := acquireDataDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := acquireDataDir(filepath.Join(dir, ".")) // another spelling of the same folder
	if err != nil {
		t.Fatalf("this process's second hold was refused: %v", err)
	}
	r1()
	r1() // giving up a hold twice gives it up once
	assertSecondRefused(t, dir, "with one hold left")
	r2()
	assertOwnable(t, dir, "with no hold left")
}

// The lock file is never removed, and a leftover one never blocks a start: the
// operating system's lock is what counts, and it ends with its process.
func TestOwner_LeftoverLockFileDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, dataOwnerFile), []byte("pid=999999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := acquireDataDir(dir)
	if err != nil {
		t.Fatalf("a leftover lock file blocked the start: %v", err)
	}
	release()
	if _, err := os.Stat(filepath.Join(dir, dataOwnerFile)); err != nil {
		t.Fatalf("the lock file was removed: %v", err)
	}
}

// The upgrade of catalog.json into the catalog database refuses a second owner
// while it runs.
func TestOwner_UpgradeRefusesASecondOwnerMidway(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(catalogEngineEnv, "json")
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.AddCollection("Before the upgrade")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	assertOwnable(t, dir, "before the upgrade")
	ran := false
	if _, err := migrateCatalogToDB(dir, catalogMigrateOptions{beforeStub: func() error {
		ran = true
		assertSecondRefused(t, dir, "during the upgrade")
		return nil
	}}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("the upgrade never reached the point under test")
	}
	assertOwnable(t, dir, "after the upgrade")
}

// An app-state restore refuses a second owner while it runs, including after the
// old catalog is closed and before the restored one opens.
func TestOwner_RestoreRefusesASecondOwnerMidway(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	app := abTestApp(t)
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	prev := restoreAfterClose
	checked := false
	restoreAfterClose = func(dataDir string) {
		checked = true
		assertSecondRefused(t, dataDir, "during the restore, with the catalog closed")
		if prev != nil {
			prev(dataDir)
		}
	}
	t.Cleanup(func() { restoreAfterClose = prev })
	if _, err := app.RestoreAppBackup(tarPath); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("the restore never reached the point under test")
	}
	assertSecondRefused(t, app.DataDir, "after the restore")
}

// A data folder move owns both folders until the switch is done, and a target
// another process owns is refused before anything is copied.
func TestOwner_MoveHoldsBothFolders(t *testing.T) {
	for _, engine := range []string{"sqlite", "json"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			legacy := t.TempDir()
			target := filepath.Join(t.TempDir(), "new-home")
			s, err := openTestStore(t, legacy)
			if err != nil {
				t.Fatal(err)
			}
			app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
			app.Store.AddCollection("Moved")

			checked := false
			dataDirSwapHook = func(stage string) {
				if stage == "copied" {
					checked = true
					assertSecondRefused(t, legacy, "during the move (old folder)")
					assertSecondRefused(t, target, "during the move (new folder)")
				}
			}
			t.Cleanup(func() { dataDirSwapHook = nil })
			if _, err := app.migrateDataDir(legacy, target); err != nil {
				t.Fatalf("move: %v", err)
			}
			t.Cleanup(func() { app.Store.Close() })
			if !checked {
				t.Fatal("the move never reached the point under test")
			}
			assertSecondRefused(t, target, "after the move")
			if _, err := os.Stat(filepath.Join(target, dataOwnerFile)); err != nil {
				t.Fatalf("the new folder has no lock file of its own: %v", err)
			}
		})
	}
}

func TestOwner_MoveToAFolderAnotherProcessOwnsIsRefused(t *testing.T) {
	legacy := t.TempDir()
	target := filepath.Join(t.TempDir(), "new-home")
	s, err := openTestStore(t, legacy)
	if err != nil {
		t.Fatal(err)
	}
	app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
	app.Store.AddCollection("Stays")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	other, err := lockDataDir(target) // another process owns the target
	if err != nil {
		t.Fatal(err)
	}
	defer func() { unlockFile(other); other.Close() }()
	if _, err := app.migrateDataDir(legacy, target); !isDataDirInUse(err) {
		t.Fatalf("got %v, want the move refused because another Obelisk owns %s", err, target)
	}
	if app.DataDir != legacy {
		t.Fatalf("the app switched to %s", app.DataDir)
	}
	if got := collectionNames(app.Store); strings.Join(got, ",") != "Stays" {
		t.Fatalf("the catalog holds %v after the refused move", got)
	}
}
