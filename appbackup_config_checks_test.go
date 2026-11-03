//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Restoring an app backup applies the same checks to the settings it brings as
// saving them does: a backup whose staging or export folder is a registered
// source (in the catalog it restores, or the one in use) is refused before
// anything is restored.
func TestRestoreAppBackup_RerunsSaveTimeConfigChecks(t *testing.T) {
	for _, key := range []string{"staging_dir", "auto_export_dir", "escrow_cache_dir"} {
		t.Run(key, func(t *testing.T) {
			app, dataDir := compatApp(t)
			src := filepath.Join(t.TempDir(), "photos")
			writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
			// Saved while the folder was not a source yet, which is allowed...
			if _, err := app.SaveConfig(map[string]any{key: src}); err != nil {
				t.Fatalf("saving %s before it is a source: %v", key, err)
			}
			// ...then registered as a source, so the backup carries both.
			app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
			tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
			if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
				t.Fatal(err)
			}
			// Put the live settings and catalog somewhere safe, and note them.
			if _, err := app.SaveConfig(map[string]any{key: ""}); err != nil {
				t.Fatal(err)
			}
			beforeCfg, _ := os.ReadFile(filepath.Join(dataDir, "config.json"))
			beforeCat, _ := os.ReadFile(filepath.Join(dataDir, "catalog.json"))

			_, err := app.RestoreAppBackup(tarPath)
			if err == nil || !strings.Contains(err.Error(), "source") {
				t.Fatalf("restore: got %v, want a refusal naming the source", err)
			}
			if after, _ := os.ReadFile(filepath.Join(dataDir, "config.json")); string(after) != string(beforeCfg) {
				t.Error("a refused restore changed the settings")
			}
			if after, _ := os.ReadFile(filepath.Join(dataDir, "catalog.json")); string(after) != string(beforeCat) {
				t.Error("a refused restore changed the catalog")
			}
			if pre, _ := filepath.Glob(filepath.Join(dataDir, "pre-restore-*")); len(pre) > 0 {
				t.Errorf("a refused restore started restoring: %v", pre)
			}
		})
	}
}
