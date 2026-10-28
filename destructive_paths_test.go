//go:build !guionly

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oldRouteRefused requires a direct request to a gated route to answer 428.
func oldRouteRefused(t *testing.T, app *App, method, path, body string) {
	t.Helper()
	mux := http.NewServeMux()
	api(mux, app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	if rec.Code != 428 {
		t.Fatalf("%s %s answered %d (%s), want 428", method, path, rec.Code, rec.Body.String())
	}
}

func prepareForTest(t *testing.T, app *App, op string, params map[string]any) *PreparedAction {
	t.Helper()
	p, err := app.PrepareDestructive(testSession, op, params)
	if err != nil {
		t.Fatalf("prepare %s: %v", op, err)
	}
	return p
}

func TestDestructive_Quarantine(t *testing.T) {
	t.Run("old route", func(t *testing.T) {
		app, destRoot, _ := quarantineScenario(t)
		oldRouteRefused(t, app, "POST", "/api/quarantine", `{"path":`+jsonQuote(filepath.Join(destRoot, "raw"))+`}`)
		assertFileBytes(t, filepath.Join(destRoot, "raw", "a.nef"), "AAA")
	})
	t.Run("lists every file and moves them", func(t *testing.T) {
		app, destRoot, _ := quarantineScenario(t)
		raw := filepath.Join(destRoot, "raw")
		p := prepareForTest(t, app, "quarantine", map[string]any{"path": raw, "reason": "dupe"})
		if p.Word != wordDelete || len(p.Targets) != 3 { // raw, a.nef, b.nef
			t.Fatalf("prepare: word %q, %d target(s), want Delete and 3", p.Word, len(p.Targets))
		}
		if _, err := app.ExecuteDestructive(testSession, p.Token, "Delete"); err != nil {
			t.Fatal(err)
		}
		assertFileBytes(t, filepath.Join(destRoot, QuarantineDir, "raw", "a.nef"), "AAA")
	})
	t.Run("a file added after prepare", func(t *testing.T) {
		app, destRoot, _ := quarantineScenario(t)
		raw := filepath.Join(destRoot, "raw")
		p := prepareForTest(t, app, "quarantine", map[string]any{"path": raw})
		mustWrite(t, filepath.Join(raw, "new.nef"), "NEW")
		_, err := app.ExecuteDestructive(testSession, p.Token, "Delete")
		refusedWith(t, err, 409, "added file")
		assertFileBytes(t, filepath.Join(raw, "new.nef"), "NEW")
		if _, err := os.Stat(filepath.Join(destRoot, QuarantineDir)); !os.IsNotExist(err) {
			t.Error("something was moved although the targets changed")
		}
	})
	t.Run("a file changed after prepare", func(t *testing.T) {
		app, destRoot, _ := quarantineScenario(t)
		raw := filepath.Join(destRoot, "raw")
		p := prepareForTest(t, app, "quarantine", map[string]any{"path": raw})
		mustWrite(t, filepath.Join(raw, "a.nef"), "AAAA") // a different size
		_, err := app.ExecuteDestructive(testSession, p.Token, "Delete")
		refusedWith(t, err, 409, "changed file")
		assertFileBytes(t, filepath.Join(raw, "a.nef"), "AAAA")
	})
}

func TestDestructive_ArchiveRemoval(t *testing.T) {
	setup := func(t *testing.T) (*App, int) {
		t.Helper()
		app := dockApp(t)
		src := t.TempDir()
		writeTree(t, src, map[string]string{"a.txt": "A", "b.txt": "B"})
		c := app.Store.AddCollection("Old Shoot")
		if _, _, err := app.ScanFolder(c.ID, src, noProgMirror); err != nil {
			t.Fatal(err)
		}
		return app, c.ID
	}
	params := func(id int) map[string]any {
		return map[string]any{"id": float64(id), "confirm_name": "Old Shoot"}
	}
	t.Run("old route", func(t *testing.T) {
		app, id := setup(t)
		oldRouteRefused(t, app, "DELETE", fmt.Sprintf("/api/collections/%d", id), `{"confirm_name":"Old Shoot"}`)
		if app.Store.Collection(id) == nil {
			t.Fatal("the old route removed the archive")
		}
	})
	t.Run("removes after the typed word", func(t *testing.T) {
		app, id := setup(t)
		p := prepareForTest(t, app, "archive-remove", params(id))
		if _, err := app.ExecuteDestructive(testSession, p.Token, "Delete"); err != nil {
			t.Fatal(err)
		}
		if app.Store.Collection(id) != nil || len(app.Store.FilesOf(id)) != 0 {
			t.Fatal("the archive was not removed")
		}
	})
	t.Run("the archive's name is still required", func(t *testing.T) {
		app, id := setup(t)
		_, err := app.PrepareDestructive(testSession, "archive-remove", map[string]any{"id": float64(id), "confirm_name": "old shoot"})
		refusedWith(t, err, 400, "wrong name")
	})
	t.Run("a record added after prepare", func(t *testing.T) {
		app, id := setup(t)
		p := prepareForTest(t, app, "archive-remove", params(id))
		folder := app.Store.FoldersOf(id)[0]
		app.Store.UpsertFile(File{CollectionID: id, FolderID: folder.ID, RelPath: "c.txt", Hash: "cc", SizeBytes: 1})
		_, err := app.ExecuteDestructive(testSession, p.Token, "Delete")
		refusedWith(t, err, 409, "added record")
		if app.Store.Collection(id) == nil {
			t.Fatal("removed although its records changed")
		}
	})
}
