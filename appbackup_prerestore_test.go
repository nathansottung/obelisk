//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// keystoreAppForTest is an app whose configuration names one keystore in its
// data folder, holding the given key refs.
func keystoreAppForTest(t *testing.T, refs ...string) (*App, string, string) {
	t.Helper()
	app, dataDir := compatApp(t)
	ks := filepath.Join(dataDir, "keystores", "ks.json")
	writeKeysForTest(t, ks, refs...)
	cfg := mustConfig(t, app)
	cfg.KeystorePaths = []string{ks}
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	return app, dataDir, ks
}

func writeKeysForTest(t *testing.T, path string, refs ...string) {
	t.Helper()
	ks := &keystoreFile{}
	for _, r := range refs {
		ks.Keys = append(ks.Keys, map[string]any{"ref": r, "passphrase": "synthetic-" + r})
	}
	if err := writeStoreObserved(path, ks, nil); err != nil {
		t.Fatal(err)
	}
}

// Restoring an app backup replaces the keystores in the data folder. The ones in
// use are copied into the pre-restore folder first, so a key that only the
// current keystore holds is not lost.
func TestRestoreAppBackup_CopiesKeystoresFirst(t *testing.T) {
	app, dataDir, ks := keystoreAppForTest(t, "K1")
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, true); err != nil {
		t.Fatal(err)
	}
	writeKeysForTest(t, ks, "K1", "K2") // K2 exists only in the current keystore
	if _, err := app.RestoreAppBackup(tarPath); err != nil {
		t.Fatal(err)
	}
	pre, _ := filepath.Glob(filepath.Join(dataDir, "pre-restore-*", "keystores", "*"))
	found := false
	for _, p := range pre {
		if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), `"K2"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the keystore holding K2 was not copied into pre-restore (found %v)", pre)
	}
	if fi, err := os.Stat(ks); err != nil || (fi.Mode().Perm()&0o077 != 0 && runtime.GOOS != "windows") {
		t.Errorf("the restored keystore is readable by others: %v %v", fi.Mode(), err)
	}
}

// If any current record or keystore cannot be copied into the pre-restore
// folder, the restore stops before it changes anything.
func TestRestoreAppBackup_StopsWhenThePreCopyFails(t *testing.T) {
	app, dataDir, ks := keystoreAppForTest(t, "K1")
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, true); err != nil {
		t.Fatal(err)
	}
	writeKeysForTest(t, ks, "K1", "K2")
	beforeKS, _ := os.ReadFile(ks)
	beforeCat, _ := os.ReadFile(filepath.Join(dataDir, "catalog.json"))
	prev := preRestoreWriteFile
	preRestoreWriteFile = func(name string, b []byte, perm os.FileMode) error {
		if strings.Contains(filepath.ToSlash(name), "/keystores/") {
			return errors.New("disk full")
		}
		return prev(name, b, perm)
	}
	t.Cleanup(func() { preRestoreWriteFile = prev })

	if _, err := app.RestoreAppBackup(tarPath); err == nil || !strings.Contains(err.Error(), "pre-restore") {
		t.Fatalf("got %v, want the restore stopped because the pre-restore copy failed", err)
	}
	if after, _ := os.ReadFile(ks); string(after) != string(beforeKS) {
		t.Error("the keystore was replaced although its copy failed")
	}
	if after, _ := os.ReadFile(filepath.Join(dataDir, "catalog.json")); string(after) != string(beforeCat) {
		t.Error("the catalog was replaced although the pre-restore copy failed")
	}
}
