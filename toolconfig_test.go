//go:build !guionly

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// par2's extra arguments are limited to tuning flags: a flag that deletes
// files (purge) or anything else unknown is refused when saved and when used.
func TestPar2ExtraArgs_RefusesDeleteFlags(t *testing.T) {
	app := dockApp(t)
	for _, bad := range []string{"-p", "-t0 -p", "--purge", "-t0 -R", "-t0;del", "-N", "x"} {
		if _, err := app.SaveConfig(map[string]any{"par2_extra_args": bad}); err == nil {
			t.Errorf("accepted par2_extra_args %q", bad)
		}
	}
	for _, ok := range []string{"", "-t0", "-t4 -q", "-T2 -m512 -qq", "-v"} {
		if _, err := app.SaveConfig(map[string]any{"par2_extra_args": ok}); err != nil {
			t.Errorf("refused par2_extra_args %q: %v", ok, err)
		}
	}
	// Arriving another way (an edited file), it is refused where it is used.
	if err := runPar2Create("par2-not-run", "-t0 -p", 10, "x.par2", "x"); err == nil || !strings.Contains(err.Error(), "par2_extra_args") {
		t.Errorf("par2 ran with a purge flag: %v", err)
	}
}

// A tool path must be an absolute path to that tool, by name: another program
// (a shell, a delete command), a relative path or an unknown tool is refused
// when saved, and a configured path naming another program is not run.
func TestToolPaths_AreValidated(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	par2 := filepath.Join(dir, "par2.exe")
	mustWrite(t, par2, "not really par2")
	shell := filepath.Join(dir, "cmd.exe")
	mustWrite(t, shell, "a shell")
	for name, bad := range map[string]map[string]any{
		"another program": {"par2": shell},
		"relative path":   {"par2": "par2.exe"},
		"unknown tool":    {"nuke": par2},
		"a folder":        {"par2": dir},
	} {
		if _, err := app.SaveConfig(map[string]any{"tools": bad}); err == nil {
			t.Errorf("%s: accepted tools %v", name, bad)
		}
	}
	if _, err := app.SaveConfig(map[string]any{"tools": map[string]any{"par2": par2}}); err != nil {
		t.Fatalf("a path to par2 was refused: %v", err)
	}
	cfg := mustConfig(t, app)
	cfg.Tools["gpg"] = shell // as if edited into the file
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	if p, err := app.tool("gpg"); err == nil {
		t.Errorf("a configured path to another program was used for gpg: %s", p)
	}
}
