//go:build !guionly

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testSession = "test-session-0123456789"

// restorableAppForTest is an app with a backup of its own records to restore.
func restorableAppForTest(t *testing.T) (*App, string, string) {
	t.Helper()
	app, dataDir := compatApp(t)
	app.Store.AddCollection("Before the backup")
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatal(err)
	}
	app.Store.AddCollection("After the backup")
	return app, dataDir, tarPath
}

func refusedWith(t *testing.T, err error, status int, what string) {
	t.Helper()
	var de *destructiveError
	if !errors.As(err, &de) || de.status != status {
		t.Fatalf("%s: got %v, want a refusal with status %d", what, err, status)
	}
}

// The whole path: prepare lists exactly the files the restore replaces, and
// execute with the typed word runs it.
func TestDestructive_PrepareThenExecute(t *testing.T) {
	app, dataDir, tarPath := restorableAppForTest(t)
	p, err := app.PrepareDestructive(testSession, "appbackup-restore", map[string]any{"tar_path": tarPath})
	if err != nil {
		t.Fatal(err)
	}
	if p.Word != wordOverwrite || len(p.Token) != 64 || p.Digest == "" {
		t.Fatalf("prepare returned %+v", p)
	}
	listed := map[string]bool{}
	for _, tg := range p.Targets {
		listed[tg.Path] = true
		if tg.Kind == "file" && (tg.FileID == 0 && tg.Volume == 0 || tg.Mtime == 0) {
			t.Errorf("target %s is not recorded exactly: %+v", tg.Path, tg)
		}
	}
	for _, want := range []string{tarPath, filepath.Join(dataDir, "catalog.json"), filepath.Join(dataDir, "config.json")} {
		if !listed[want] {
			t.Errorf("%s is not among the targets", want)
		}
	}
	if _, err := app.ExecuteDestructive(testSession, p.Token, "Overwrite"); err != nil {
		t.Fatal(err)
	}
	for _, c := range app.Store.Collections() {
		if c.Name == "After the backup" {
			t.Fatal("the restore did not run")
		}
	}
}

// Every refusal: another session, an expired token, the wrong word, a used
// token, and targets that changed after prepare. None of them changes anything.
func TestDestructive_Refusals(t *testing.T) {
	prepare := func(t *testing.T, app *App, tarPath string) *PreparedAction {
		t.Helper()
		p, err := app.PrepareDestructive(testSession, "appbackup-restore", map[string]any{"tar_path": tarPath})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	unchanged := func(t *testing.T, app *App) {
		t.Helper()
		for _, c := range app.Store.Collections() {
			if c.Name == "After the backup" {
				return
			}
		}
		t.Fatal("a refused restore changed the records")
	}

	t.Run("another session", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		p := prepare(t, app, tarPath)
		_, err := app.ExecuteDestructive("someone-else-0123456789", p.Token, "Overwrite")
		refusedWith(t, err, 403, "another session")
		_, err = app.ExecuteDestructive(testSession, p.Token, "Overwrite")
		refusedWith(t, err, 410, "the token after a refused attempt")
		unchanged(t, app)
	})
	t.Run("expired", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		p := prepare(t, app, tarPath)
		prev := destructiveNow
		destructiveNow = func() time.Time { return time.Now().Add(destructiveTTL + time.Second) }
		t.Cleanup(func() { destructiveNow = prev })
		_, err := app.ExecuteDestructive(testSession, p.Token, "Overwrite")
		refusedWith(t, err, 410, "expired")
		unchanged(t, app)
	})
	t.Run("wrong word", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		for _, word := range []string{"", "overwrite", "Delete", "Overwrite ", "yes"} {
			p := prepare(t, app, tarPath)
			_, err := app.ExecuteDestructive(testSession, p.Token, word)
			refusedWith(t, err, 400, "word "+word)
		}
		unchanged(t, app)
	})
	t.Run("used twice", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		p := prepare(t, app, tarPath)
		if _, err := app.ExecuteDestructive(testSession, p.Token, "Overwrite"); err != nil {
			t.Fatal(err)
		}
		_, err := app.ExecuteDestructive(testSession, p.Token, "Overwrite")
		refusedWith(t, err, 410, "second use")
	})
	t.Run("a target changed", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		p := prepare(t, app, tarPath)
		app.Store.AddCollection("Saved after prepare") // the catalog file changes
		_, err := app.ExecuteDestructive(testSession, p.Token, "Overwrite")
		refusedWith(t, err, 409, "changed target")
		unchanged(t, app)
	})
	t.Run("no session", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		_, err := app.PrepareDestructive("", "appbackup-restore", map[string]any{"tar_path": tarPath})
		refusedWith(t, err, 400, "no session")
	})
	t.Run("another app", func(t *testing.T) {
		app, _, tarPath := restorableAppForTest(t)
		other, _, _ := restorableAppForTest(t)
		p := prepare(t, app, tarPath)
		_, err := other.ExecuteDestructive(testSession, p.Token, "Overwrite")
		refusedWith(t, err, 410, "another app")
	})
}

// The old direct routes answer 428 and change nothing; the gate's routes work
// over HTTP with the session header.
func TestDestructive_OldRoutesNeedConfirmation(t *testing.T) {
	app, _, tarPath := restorableAppForTest(t)
	mux := http.NewServeMux()
	api(mux, app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/appbackup/restore", strings.NewReader(`{"tar_path":`+jsonQuote(tarPath)+`}`)))
	if rec.Code != 428 {
		t.Fatalf("old route answered %d, want 428", rec.Code)
	}
	for _, c := range app.Store.Collections() {
		if c.Name == "After the backup" {
			return
		}
	}
	t.Fatal("the old route restored")
}

func jsonQuote(s string) string {
	b := strings.NewReplacer(`\`, `\`, `"`, `\"`).Replace(s)
	return `"` + b + `"`
}
