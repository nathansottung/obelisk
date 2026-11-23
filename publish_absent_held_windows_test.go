//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A file that arrives at the final name while the stage is held must survive: the
// publish is refused, and nothing falls through to a replacing rename.
func TestPublishNoReplace_HeldStageNeverReplacesAnArrival(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".stage.tmp")
	final := filepath.Join(dir, "catalog.db")
	mustWrite(t, stage, "OURS")
	release := holdWithoutShareDelete(t, stage)
	arrived := make(chan error, 1)
	time.AfterFunc(250*time.Millisecond, func() { arrived <- os.WriteFile(final, []byte("RIVAL"), 0o600) })
	time.AfterFunc(500*time.Millisecond, release)

	err := publishNoReplace(stage, final)
	if e := <-arrived; e != nil {
		t.Fatal(e)
	}
	if err == nil {
		t.Fatal("publishing over a file that arrived must be refused")
	}
	mustBytes(t, final, "RIVAL", "arrival during a held publish")
	mustBytes(t, stage, "OURS", "refused publish (staged file is the caller's)")
}

// A scanner briefly holding the new stage must not refuse first-use setup.
func TestConfigInit_BrieflyHeldStageIsRetried(t *testing.T) {
	a := &App{DataDir: t.TempDir()}
	a.configIO.link = func(from, to string) error {
		release := holdWithoutShareDelete(t, from)
		time.AfterFunc(200*time.Millisecond, release)
		return publishAbsent(from, to)
	}
	if _, err := a.InitializeConfig(); err != nil {
		t.Fatalf("first-use setup with a briefly held stage: %v", err)
	}
	if _, err := a.LoadConfig(); err != nil {
		t.Fatal(err)
	}
}

// Win32 drops a trailing dot from a short path, as the os package's calls see it.
func TestConfigInit_FolderNameEndingInADot(t *testing.T) {
	base := t.TempDir()
	a := &App{DataDir: filepath.Join(base, "data") + "."}
	if _, err := a.InitializeConfig(); err != nil {
		t.Fatalf("first-use setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "data", "config.json")); err != nil {
		t.Fatal(err)
	}
}

func TestExtendedPath(t *testing.T) {
	long := `C:\` + strings.Repeat(`folder\`, 40) + "config.json"
	for _, c := range []struct{ in, want string }{
		{`C:\data\config.json`, `C:\data\config.json`},
		{`C:\data.\config.json`, `C:\data.\config.json`},
		{`relative\config.json`, `relative\config.json`},
		{`\\.\C:\a`, `\\.\C:\a`},
		{`\\?\C:\a`, `\\?\C:\a`},
		{`//?/C:/a`, `//?/C:/a`},
		{long, `\\?\` + long},
		{`\\server\share\` + long[3:], `\\?\UNC\server\share\` + long[3:]},
		{`\\?\` + long, `\\?\` + long},
		{`\\.\` + long, `\\.\` + long},
	} {
		if got := extendedPath(c.in); got != c.want {
			t.Errorf("extendedPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
