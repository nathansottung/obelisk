package main

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestConfigInit_CheckedFailureStages(t *testing.T) {
	old := hashAccelOn.Load()
	defer setHashAccel(old)
	for _, stage := range []string{"create", "write", "short-write", "sync", "close", "unsupported-link", "cleanup", "dir-sync"} {
		t.Run(stage, func(t *testing.T) {
			a := &App{DataDir: t.TempDir()}
			setHashAccel(false)
			a.configIO.createTemp = func(d, p string) (configTempFile, error) {
				if stage == "create" {
					return nil, errors.New("injected create failure")
				}
				f, e := os.CreateTemp(d, p)
				if e != nil {
					return nil, e
				}
				return configFaultFile{File: f, failure: stage}, nil
			}
			if stage == "unsupported-link" {
				a.configIO.link = func(string, string) error { return errors.ErrUnsupported }
			}
			if stage == "cleanup" {
				a.configIO.removeTemp = func(string) error { return errors.New("injected cleanup failure") }
			}
			if stage == "dir-sync" {
				a.configIO.syncDir = func(string) error { return errors.New("injected directory sync failure") }
			}
			// Replacement must never be used as a fallback, even when linking fails.
			a.configIO.rename = func(string, string) error {
				t.Error("initialization entered replacement publisher")
				return errors.New("unexpected replacement")
			}
			cfg, e := a.InitializeConfig()
			var pe *ConfigPublicationError
			published := stage == "cleanup" || stage == "dir-sync"
			if e == nil || !errors.As(e, &pe) || pe.Published != published || !reflect.DeepEqual(cfg, Config{}) {
				t.Fatal("initialization error misreported publication phase")
			}
			if stage == "unsupported-link" && !errors.Is(e, errors.ErrUnsupported) {
				t.Fatal("unsupported cause was lost")
			}
			if hashAccelOn.Load() != published {
				t.Fatal("runtime did not follow actual publication")
			}
			entries, e := os.ReadDir(a.DataDir)
			if e != nil {
				t.Fatal("fixture enumeration failed")
			}
			if published {
				if _, e := a.LoadConfig(); e != nil || len(entries) != 1 || entries[0].Name() != "config.json" {
					t.Fatal("published config lost or staging cleanup failed")
				}
			} else if len(entries) != 0 {
				t.Fatal("failed initialization left entries")
			}
		})
	}
}
