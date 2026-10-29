//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JSON field names match case-insensitively, so the save checks must look at
// the settings as decoded, not at the exact keys sent: a letter-case variant
// of a key is the same setting.
func TestSaveConfig_CaseVariantKeysGetTheSameChecks(t *testing.T) {
	app := dockApp(t)
	if _, err := app.SaveConfig(map[string]any{"Burn_Command": "xorriso -map {SRC} /"}); err == nil || !strings.Contains(err.Error(), "Confirm") {
		t.Errorf("Burn_Command changed without Confirm: got %v, want a refusal asking for it", err)
	}
	if _, err := app.SaveConfig(map[string]any{"BURN_COMMAND": "rm -rf {SRC}", "burn_command_confirm": "Confirm"}); err == nil {
		t.Error("BURN_COMMAND saved a program that is not a disc burner")
	}
	other := filepath.Join(t.TempDir(), "notatar.exe")
	if err := os.WriteFile(other, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveConfig(map[string]any{"Tools": map[string]any{"tar": other}}); err == nil {
		t.Error("Tools saved a tar path that names another program")
	}
	ks := filepath.Join(t.TempDir(), "keys.json")
	if _, err := app.SaveConfig(map[string]any{"Keystore_Paths": []any{ks, strings.ToUpper(ks)}}); err == nil {
		t.Error("Keystore_Paths saved the same keystore twice")
	}
	cfg := mustConfig(t, app)
	if strings.Contains(cfg.BurnCommand, "{SRC}") || cfg.Tools["tar"] == other || len(cfg.KeystorePaths) == 2 && strings.EqualFold(cfg.KeystorePaths[0], cfg.KeystorePaths[1]) {
		t.Fatalf("a refused setting was saved: %+v", cfg)
	}
}
