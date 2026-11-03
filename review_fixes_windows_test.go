//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// unattachedDrive returns a drive letter with nothing attached.
func unattachedDrive(t *testing.T) string {
	t.Helper()
	for c := 'Z'; c >= 'G'; c-- {
		d := string(c) + `:\`
		if _, err := os.Stat(d); err != nil {
			return d
		}
	}
	t.Skip("SKIP (NOT TESTED): every drive letter is in use")
	return ""
}

// A folder on a drive that is not attached has nothing to resolve: it cannot be
// inside an attached source, so settings naming it are saved, and an app backup
// carrying them restores. Only its path is compared with sources whose drive is
// not attached.
func TestConfigPaths_UnattachedDriveIsNotRefused(t *testing.T) {
	app, _ := compatApp(t)
	q := unattachedDrive(t)
	for key, val := range map[string]any{
		"staging_dir":     q + `obelisk-staging`,
		"auto_export_dir": q + `backups`,
		"keystore_paths":  []string{q + `keys\ks.json`, filepath.Join(t.TempDir(), "k2.json")},
	} {
		if _, err := app.SaveConfig(map[string]any{key: val}); err != nil {
			t.Errorf("saving %s on an unattached drive: %v", key, err)
		}
	}
	if _, err := app.SaveConfig(map[string]any{"throttle_mbps": 10}); err != nil {
		t.Errorf("an unrelated save with folders on an unattached drive: %v", err)
	}
	tarPath := filepath.Join(t.TempDir(), "b.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RestoreAppBackup(tarPath); err != nil {
		t.Errorf("restoring settings that name an unattached drive: %v", err)
	}
	// A source registered on that drive still blocks that same path.
	app.Store.AddFolder(app.Store.AddCollection("Away").ID, q+`photos`)
	if err := app.Store.CheckOutsideSources(q + `photos\x`); err == nil {
		t.Error("a folder inside an unattached source was allowed")
	}
	if err := app.Store.CheckFileOutsideSources(q + `photos\ks.json`); err == nil {
		t.Error("a keystore inside an unattached source was allowed")
	}
}

// A keystore is held to the same rule as a folder for a source whose drive is
// attached somewhere other than where it was registered: anything on that drive
// is refused, since the source may be on it.
func TestKeystoreCheck_SourceDriveAttachedElsewhere(t *testing.T) {
	app, _ := compatApp(t)
	here := t.TempDir()
	vid := pathVolumeID(here)
	if vid == "" {
		t.Skip("SKIP (NOT TESTED): no volume identity for the temp folder")
	}
	// Registered on this drive under a letter that is not attached now.
	f := app.Store.AddFolder(app.Store.AddCollection("Moved").ID, unattachedDrive(t)+`Photos`)
	app.Store.recordFolderVolume(f.ID, vid)
	if err := app.Store.CheckOutsideSources(here); err == nil {
		t.Fatal("setup: a folder on the source's drive was allowed")
	}
	if err := app.Store.CheckFileOutsideSources(filepath.Join(here, "ks.json")); err == nil {
		t.Error("a keystore on the drive that holds a moved source was allowed")
	}
}
