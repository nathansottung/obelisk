//go:build !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// A restore brings files back with the modification time and permission bits
// the package holds for them, as unpacking with tar does, both into a guarded
// folder and into a new one.
func TestRestore_KeepsTimesAndModes(t *testing.T) {
	for _, guard := range []bool{true, false} {
		t.Run(fmt.Sprintf("guard=%v", guard), func(t *testing.T) {
			s := newIT(t)
			s.setConfig(nil)
			src := s.makeSource(map[string][]byte{"old.txt": []byte("from 2001\n"), "bin/run.sh": []byte("#!/bin/sh\n")})
			when := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
			if err := os.Chtimes(filepath.Join(src, "old.txt"), when, when); err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" {
				if err := os.Chmod(filepath.Join(src, "bin", "run.sh"), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			c := s.scanPlanBuild(src, 1, false, 10)
			pid, name := int(c["id"].(float64)), c["name"].(string)
			medium := t.TempDir()
			vid := s.addVolume("META-1", "lab")
			s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": medium, "volume_id": vid}))

			out := t.TempDir()
			if !guard {
				withoutDestGuard(t)
				out = filepath.Join(out, "restored")
			}
			s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/restore", pid), map[string]any{"source_dir": filepath.Join(medium, name), "output_dir": out}))

			fi, err := os.Stat(filepath.Join(out, "old.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !fi.ModTime().Equal(when) {
				t.Errorf("old.txt restored with time %v, want %v", fi.ModTime().UTC(), when)
			}
			if runtime.GOOS != "windows" {
				if fi, err := os.Stat(filepath.Join(out, "bin", "run.sh")); err != nil || fi.Mode().Perm() != 0o750 {
					t.Errorf("run.sh restored with mode %v (err %v), want 0750", fi.Mode().Perm(), err)
				}
			}
		})
	}
}

// A file placed by a plan gets the source file's permission bits, not the
// owner-only bits of its staging file.
func TestPlan_PlacedFilesKeepTheSourceMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SKIP (NOT TESTED): permission bits are a Unix property")
	}
	app := dockApp(t)
	destRoot := createdForTest(t, app, filepath.Join(t.TempDir(), "dest"))
	mount := t.TempDir()
	plan := compiledPlanForTest(t, app, mount, "MODE", map[string]string{"a.jpg": "AAA"}, map[string]string{"a.jpg": "photos/a.jpg"}, destRoot)
	if err := os.Chmod(filepath.Join(mount, "a.jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ExecutePlanFromDrive(plan.ID, mount, "MODE", func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(destRoot, "photos", "a.jpg")); err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("placed with mode %v (err %v), want 0644", fi.Mode().Perm(), err)
	}
}
