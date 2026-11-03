//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// secretFilesForTest writes each kind of secret file Obelisk keeps and returns
// their paths: a keystore, the settings (which hold the access token), held
// settings, a pre-restore keystore copy, and an app backup that carries keys.
func secretFilesForTest(t *testing.T) map[string]string {
	t.Helper()
	app, dataDir, ks := keystoreAppForTest(t, "K1")
	if _, err := app.SaveConfig(map[string]any{"buffer_gb": 4}); err != nil {
		t.Fatal(err)
	}
	if err := app.saveHeldSettings(&HeldSettings{BurnCommand: "xorriso"}); err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(t.TempDir(), "pre-restore")
	if err := os.MkdirAll(filepath.Join(pre, "keystores"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := app.copyPreRestore(mustConfig(t, app), pre); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, true); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"keystore":             ks,
		"settings":             filepath.Join(dataDir, "config.json"),
		"held settings":        app.heldSettingsPath(),
		"pre-restore keystore": filepath.Join(pre, "keystores", "ks.json"),
		"key-bearing backup":   tarPath,
	}
}

// Secret files are readable and writable by their owner only: on Windows, an
// ACL that grants only the current user and inherits nothing; elsewhere, mode
// 0600.
func TestSecretFiles_AreOwnerOnly(t *testing.T) {
	for what, p := range secretFilesForTest(t) {
		if runtime.GOOS == "windows" {
			if err := ownerOnlyACLForTest(p); err != nil {
				t.Errorf("%s (%s): %v", what, p, err)
			}
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s (%s) has mode %v, want owner-only", what, p, fi.Mode().Perm())
		}
	}
}
