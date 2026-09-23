package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Portable to the pre-follow-up candidate: arrival is coordinated after its real
// absence read, not by replacing the production publication operation with a mock.
func TestConfigInit_LateArrivalPreserved(t *testing.T) {
	old := hashAccelOn.Load()
	defer setHashAccel(old)
	for _, kind := range []string{"valid", "damaged", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			a := &App{DataDir: dir}
			dest := a.configPath()
			prior := []byte(`{"auth_token":"init-fixture-secret","tools":{"gpg":"fixture-helper"},"keystore_paths":["fixture-key-path"],"hash_accel":false}`)
			if kind == "damaged" {
				prior = []byte(`{"auth_token":"init-fixture-secret",`)
			}
			target := filepath.Join(t.TempDir(), "held.json")
			if kind == "symlink" {
				if os.WriteFile(target, prior, 0600) != nil {
					t.Fatal("link target fixture failed")
				}
				probe := filepath.Join(dir, "link-probe")
				if e := os.Symlink(target, probe); e != nil {
					t.Skip("platform does not permit a symbolic-link fixture")
				}
				if os.Remove(probe) != nil {
					t.Fatal("link probe cleanup failed")
				}
			}
			entered := false
			a.configIO.createTemp = func(d, p string) (configTempFile, error) {
				entered = true
				switch kind {
				case "directory":
					if e := os.Mkdir(dest, 0700); e != nil {
						return nil, e
					}
				case "symlink":
					if e := os.Symlink(target, dest); e != nil {
						return nil, e
					}
				default:
					if e := os.WriteFile(dest, prior, 0600); e != nil {
						return nil, e
					}
				}
				return os.CreateTemp(d, p)
			}
			setHashAccel(false)
			cfg, e := a.InitializeConfig()
			var pe *ConfigPublicationError
			if !entered {
				t.Fatal("arrival hook did not follow real absence read")
			}
			if e == nil || !errors.As(e, &pe) || pe.Published || !reflect.DeepEqual(cfg, Config{}) {
				t.Error("late-arrival initialization was not refused before publication")
			}
			if hashAccelOn.Load() {
				t.Error("runtime switched to unpublished defaults")
			}
			fi, statErr := os.Lstat(dest)
			if statErr != nil {
				t.Fatal("arriving destination lost")
			}
			switch kind {
			case "directory":
				if !fi.IsDir() {
					t.Error("arriving directory replaced")
				}
			case "symlink":
				if fi.Mode()&os.ModeSymlink == 0 {
					t.Error("arriving link replaced")
				}
				link, e := os.Readlink(dest)
				if e != nil || link != target {
					t.Error("arriving link changed")
				}
				b, e := os.ReadFile(target)
				if e != nil || !bytes.Equal(b, prior) {
					t.Error("link target changed")
				}
			default:
				b, e := os.ReadFile(dest)
				if e != nil || !bytes.Equal(b, prior) {
					t.Error("arriving config bytes changed")
				}
			}
			entries, e := os.ReadDir(dir)
			if e != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
				t.Error("owned staging was not cleaned")
			}
		})
	}
}

func TestConfigInit_NoReplacePositiveControl(t *testing.T) {
	old := hashAccelOn.Load()
	defer setHashAccel(old)
	a := &App{DataDir: t.TempDir()}
	setHashAccel(false)
	cfg, e := a.InitializeConfig()
	if e != nil || !cfg.HashAccel || !hashAccelOn.Load() {
		t.Fatal("absent-destination initialization did not publish")
	}
	reopened, e := a.LoadConfig()
	if e != nil || !reflect.DeepEqual(cfg, reopened) {
		t.Fatal("initialized bytes did not reopen")
	}
	entries, e := os.ReadDir(a.DataDir)
	if e != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatal("successful initialization left staging")
	}
}
