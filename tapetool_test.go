//go:build !guionly

package main

import (
	"path/filepath"
	"testing"
)

// tape_tool chooses a program to run, so it gets the same checks as the other
// tool paths: a known tape tool by exact name, at an absolute path, saved only
// when it exists.
func TestTapeTool_IsChecked(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	bat := filepath.Join(dir, "itdt.bat")
	other := filepath.Join(dir, "itdtx.exe")
	mustWrite(t, bat, "@echo off")
	mustWrite(t, other, "x")
	for _, bad := range []string{bat, other, "itdt.exe", filepath.Join(dir, "missing", "itdt.exe")} {
		if _, err := app.SaveConfig(map[string]any{"tape_tool": bad}); err == nil {
			t.Errorf("tape_tool %q was saved", bad)
		}
	}
	// A real tape tool on PATH may still be found; the batch file never is.
	if _, bin, ok := app.resolveTapeTool(Config{TapeTool: bat}); ok && bin == bat {
		t.Errorf("a configured %s was accepted as the tape tool", bat)
	}
	good := filepath.Join(dir, "itdt.exe")
	mustWrite(t, good, "x")
	if _, err := app.SaveConfig(map[string]any{"tape_tool": good}); err != nil {
		t.Errorf("tape_tool %q: %v", good, err)
	}
}

// A restore holds the backup's tape tool with the other program settings.
func TestRestoreAppBackup_HoldsTapeTool(t *testing.T) {
	app, _ := compatApp(t)
	tool := filepath.Join(t.TempDir(), "itdt.exe")
	mustWrite(t, tool, "x")
	cfg := mustConfig(t, app)
	cfg.TapeTool = tool
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	cfg.TapeTool = ""
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	res, err := app.RestoreAppBackup(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustConfig(t, app).TapeTool; got != "" {
		t.Fatalf("the backup's tape tool %q was applied by the restore", got)
	}
	if res.HeldSettings == nil || res.HeldSettings.TapeTool != tool {
		t.Fatalf("the backup's tape tool was not held: %+v", res.HeldSettings)
	}
	if _, err := app.ApplyHeldSettings("Confirm"); err != nil {
		t.Fatal(err)
	}
	if got := mustConfig(t, app).TapeTool; got != tool {
		t.Fatalf("applied tape tool %q, want %q", got, tool)
	}
}
