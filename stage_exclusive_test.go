//go:build !guionly

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// plantAtStageName puts a hard link to someone else's file at name, where a
// writer used to stage by a fixed name. A writer that opened that name with
// O_TRUNC, or renamed it into place, would wreck the other file or publish it.
func plantAtStageName(t *testing.T, name string) (victim string) {
	t.Helper()
	victim = filepath.Join(t.TempDir(), "someone-elses.txt")
	mustWrite(t, victim, "SOMEONE ELSE'S")
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(victim, name); err != nil {
		t.Skipf("SKIP (NOT TESTED): hard links unavailable here: %v", err)
	}
	return victim
}

// requireUntouched checks the planted file and its link are exactly as planted.
func requireUntouched(t *testing.T, victim, name string) {
	t.Helper()
	assertFileBytes(t, victim, "SOMEONE ELSE'S")
	assertFileBytes(t, name, "SOMEONE ELSE'S")
}

// Every writer that stages a file beside its final name stages into a new,
// uniquely named file it creates exclusively: whatever already sits at the old
// fixed staging name is never opened, truncated or published.
func TestStageFiles_AreUniqueAndExclusive(t *testing.T) {
	t.Run("catalog", func(t *testing.T) {
		app, dir := compatApp(t)
		name := filepath.Join(dir, "catalog.json.tmp")
		victim := plantAtStageName(t, name)
		app.Store.AddCollection("After the plant")
		requireUntouched(t, victim, name)
		if b, _ := os.ReadFile(filepath.Join(dir, "catalog.json")); len(b) == 0 || string(b) == "SOMEONE ELSE'S" {
			t.Fatal("the catalog was not saved, or was replaced by the planted file")
		}
	})
	t.Run("jobs", func(t *testing.T) {
		app, dir := compatApp(t)
		name := filepath.Join(dir, "jobs.json.tmp")
		victim := plantAtStageName(t, name)
		if _, err := app.Store.NewJob("scan", "After the plant"); err != nil {
			t.Fatal(err)
		}
		requireUntouched(t, victim, name)
	})
	t.Run("keystore", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "keys", "obelisk.keystore.json")
		victim := plantAtStageName(t, path+".tmp")
		if err := writeStoreObserved(path, &keystoreFile{}, nil); err != nil {
			t.Fatal(err)
		}
		requireUntouched(t, victim, path+".tmp")
	})
	t.Run("app backup export", func(t *testing.T) {
		app, _ := compatApp(t)
		tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
		victim := plantAtStageName(t, tarPath+".tmp")
		if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
			t.Fatal(err)
		}
		requireUntouched(t, victim, tarPath+".tmp")
	})
	t.Run("app backup restore", func(t *testing.T) {
		app, dir := compatApp(t)
		tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
		if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
			t.Fatal(err)
		}
		name := filepath.Join(dir, "catalog.json.tmp")
		victim := plantAtStageName(t, name)
		if _, err := app.RestoreAppBackup(tarPath); err != nil {
			t.Fatal(err)
		}
		requireUntouched(t, victim, name)
	})
	t.Run("escrow download", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("artifact")) }))
		t.Cleanup(srv.Close)
		dest := filepath.Join(t.TempDir(), "par2.tar.gz")
		victim := plantAtStageName(t, dest+".part")
		if err := fetchEscrowArtifact(srv.URL, dest, ""); err != nil {
			t.Fatal(err)
		}
		requireUntouched(t, victim, dest+".part")
		assertFileBytes(t, dest, "artifact")
	})
}
