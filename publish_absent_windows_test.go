//go:build windows && !guionly

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// fileNameCount is the number of names (hard links) the file at path has.
func fileNameCount(t *testing.T, path string) uint32 {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		t.Fatal(err)
	}
	return info.NumberOfLinks
}

// A hard-linked config.json can refuse to be replaced for several seconds after
// the link is made, longer than the rename retry. First-use publication must
// therefore never give the file a second name, even for a moment.
func TestConfigInit_PublishesWithoutASecondName(t *testing.T) {
	a := &App{DataDir: t.TempDir()}
	var names uint32
	a.configIO.removeTemp = func(string) error {
		names = fileNameCount(t, a.configPath())
		return nil
	}
	if _, err := a.InitializeConfig(); err != nil {
		t.Fatal(err)
	}
	if names != 1 {
		t.Fatalf("config.json had %d names when published, want 1", names)
	}
}

// The catalog database is published the same way and is later replaced by a
// catalog restore. An open handle follows a move to the new name; with a link and
// a delete it stays on the removed staging name.
func TestPublishNoReplace_MovesTheFile(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".stage.tmp")
	final := filepath.Join(dir, "catalog.db")
	mustWrite(t, stage, "database")
	p, err := windows.UTF16PtrFromString(stage)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)

	if err := publishNoReplace(stage, final); err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(windows.UTF16ToString(buf[:n])); got != "catalog.db" {
		t.Fatalf("the staged file now has the name %q, want it moved to catalog.db", got)
	}
	mustBytes(t, final, "database", "published catalog")
}

func TestPublishAbsent_RefusesAnExistingName(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, ".stage.tmp")
	final := filepath.Join(dir, "config.json")
	mustWrite(t, stage, "new")
	mustWrite(t, final, "old")
	if err := publishAbsent(stage, final); err == nil {
		t.Fatal("publishing over an existing file must be refused")
	}
	mustBytes(t, final, "old", "refused publication")
	mustBytes(t, stage, "new", "refused publication (staged file is the caller's)")
}

// A save that cannot replace config.json keeps the old settings, leaves nothing
// behind, tells the user, and works again once the file is free.
func TestConfigSave_HeldFileKeepsOldConfigAndTellsTheUser(t *testing.T) {
	a := newSetupApp(t)
	if _, err := a.SaveConfig(map[string]any{"ui_mode": "standard"}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.configPath())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api(mux, a)

	release := holdWithoutShareDelete(t, a.configPath())
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"ui_mode":"complete"}`)))
	release()

	if w.Code < 400 {
		t.Fatalf("a failed save answered %d, want an error", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body["error"], "configuration was not published") {
		t.Fatalf("the user was told %q", body["error"])
	}
	after, err := os.ReadFile(a.configPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a failed save changed config.json (read error %v)", err)
	}
	entries, err := os.ReadDir(a.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") {
			t.Fatalf("a failed save left %s behind", e.Name())
		}
	}
	cfg, err := a.SaveConfig(map[string]any{"ui_mode": "complete"})
	if err != nil || cfg.UIMode != "complete" {
		t.Fatalf("the save after the file was freed: mode %q, error %v", cfg.UIMode, err)
	}
}
