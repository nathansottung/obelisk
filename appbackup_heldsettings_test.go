//go:build !guionly

package main

import (
	"path/filepath"
	"testing"
)

// A restored app backup does not apply the settings that choose what runs (the
// burn command, tool paths, par2's extra arguments). The current values stay;
// the backup's are held and applied only after the typed word "Confirm".
func TestRestoreAppBackup_HoldsProgramSettings(t *testing.T) {
	app, _ := compatApp(t)
	gpg := filepath.Join(t.TempDir(), "gpg.exe")
	mustWrite(t, gpg, "not really gpg")
	backupCmd := `xorriso -outdev /dev/sr1 -map "{SRC}" / -commit`
	cfg := mustConfig(t, app)
	cfg.BurnCommand, cfg.Par2ExtraArgs, cfg.Tools = backupCmd, "-t4", map[string]string{"gpg": gpg}
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	// This machine's own settings.
	cfg = mustConfig(t, app)
	cfg.BurnCommand, cfg.Par2ExtraArgs, cfg.Tools = "", "-t0", map[string]string{}
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}

	res, err := app.RestoreAppBackup(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	now := mustConfig(t, app)
	if now.BurnCommand != "" || now.Par2ExtraArgs != "-t0" || now.Tools["gpg"] != "" {
		t.Fatalf("the backup's program settings were applied by the restore: burn %q, par2 %q, tools %v", now.BurnCommand, now.Par2ExtraArgs, now.Tools)
	}
	if res.HeldSettings == nil || res.HeldSettings.BurnCommand != backupCmd {
		t.Fatalf("the restore did not report the held settings: %+v", res.HeldSettings)
	}
	held, err := app.HeldSettings()
	if err != nil || held == nil || held.Par2ExtraArgs != "-t4" || held.Tools["gpg"] != gpg {
		t.Fatalf("held settings %+v, %v", held, err)
	}
	if _, err := app.ApplyHeldSettings("confirm"); err == nil {
		t.Fatal("held settings were applied without the typed word")
	}
	if _, err := app.ApplyHeldSettings("Confirm"); err != nil {
		t.Fatal(err)
	}
	now = mustConfig(t, app)
	if now.BurnCommand != backupCmd || now.Par2ExtraArgs != "-t4" || now.Tools["gpg"] != gpg {
		t.Fatalf("confirmed settings not applied: burn %q, par2 %q, tools %v", now.BurnCommand, now.Par2ExtraArgs, now.Tools)
	}
	if h, _ := app.HeldSettings(); h != nil {
		t.Error("held settings are still pending after they were applied")
	}
}

// Held settings are checked like any other when they are applied: a shell
// burn command from a backup is refused even with the typed word, and can be
// discarded.
func TestRestoreAppBackup_HeldSettingsAreChecked(t *testing.T) {
	app, _ := compatApp(t)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `cmd /c del /q {SRC}`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	cfg.BurnCommand = ""
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RestoreAppBackup(tarPath); err != nil {
		t.Fatal(err)
	}
	if mustConfig(t, app).BurnCommand != "" {
		t.Fatal("a shell burn command from the backup was applied")
	}
	if _, err := app.ApplyHeldSettings("Confirm"); err == nil {
		t.Fatal("a shell burn command was applied from held settings")
	}
	if err := app.DiscardHeldSettings(); err != nil {
		t.Fatal(err)
	}
	if h, _ := app.HeldSettings(); h != nil {
		t.Error("held settings remain after discard")
	}
}
