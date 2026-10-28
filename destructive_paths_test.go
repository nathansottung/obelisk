//go:build !guionly

package main

import (
	"encoding/json"
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

func TestDestructive_RecordDeletes(t *testing.T) {
	type rec struct {
		kind, route, method string
		make                func(app *App) any // returns the id param
		gone                func(app *App, id any) bool
		change              func(app *App, id any)
	}
	cases := []rec{
		{"event", "/api/events/%v/delete", "POST",
			func(app *App) any {
				c := app.Store.AddCollection("E")
				return float64(app.Store.AddEvent(&Event{CollectionID: c.ID, Name: "Wedding"}).ID)
			},
			func(app *App, id any) bool { return app.Store.Event(int(id.(float64))) == nil },
			func(app *App, id any) {
				ev := app.Store.Event(int(id.(float64)))
				ev.Name = "Renamed"
				app.Store.UpdateEvent(ev)
			}},
		{"template", "/api/templates/%v/delete", "POST",
			func(app *App) any {
				return float64(app.Store.AddTemplate(&Template{Name: "T", Routes: map[string]string{RoleDeliverables: "x/{orig_name}"}}).ID)
			},
			func(app *App, id any) bool { return app.Store.Template(int(id.(float64))) == nil },
			func(app *App, id any) {
				tm := app.Store.Template(int(id.(float64)))
				tm.Name = "Renamed"
				app.Store.UpdateTemplate(tm)
			}},
		{"plan", "/api/plans/%v/delete", "POST",
			func(app *App) any { return float64(app.Store.AddPlan(&Plan{Name: "P", DestinationRoot: "dest"}).ID) },
			func(app *App, id any) bool { return app.Store.Plan(int(id.(float64))) == nil },
			func(app *App, id any) {
				pl := app.Store.Plan(int(id.(float64)))
				pl.Name = "Renamed"
				app.Store.UpdatePlan(pl)
			}},
		{"profile", "/api/profiles/%v", "DELETE",
			func(app *App) any { return app.Store.AddProfile(Profile{Name: "Custom", RequiredCopies: 2}).ID },
			func(app *App, id any) bool { return app.Store.Profile(id.(string)) == nil },
			func(app *App, id any) {
				pr := *app.Store.Profile(id.(string))
				pr.Description = "changed"
				if err := app.Store.UpdateProfile(pr); err != nil {
					panic(err)
				}
			}},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			app := dockApp(t)
			id := c.make(app)
			oldRouteRefused(t, app, c.method, fmt.Sprintf(c.route, id), "{}")
			if c.gone(app, id) {
				t.Fatal("the old route deleted the record")
			}
			params := map[string]any{"kind": c.kind, "id": id}

			p := prepareForTest(t, app, "record-delete", params)
			c.change(app, id)
			_, err := app.ExecuteDestructive(testSession, p.Token, "Delete")
			refusedWith(t, err, 409, "changed record")
			if c.gone(app, id) {
				t.Fatal("deleted although the record changed after prepare")
			}

			p = prepareForTest(t, app, "record-delete", params)
			if _, err := app.ExecuteDestructive(testSession, p.Token, "Delete"); err != nil {
				t.Fatal(err)
			}
			if !c.gone(app, id) {
				t.Fatal("the record was not deleted")
			}
		})
	}
}

func TestDestructive_RewriteCopy(t *testing.T) {
	s := newIT(t)
	s.setConfig(nil)
	src := s.makeSource(map[string][]byte{"a.txt": []byte("one\n")})
	c := s.scanPlanBuild(src, 1, false, 10)
	pid, name := int(c["id"].(float64)), c["name"].(string)
	medium := t.TempDir()
	vid := s.addVolume("REWRITE-1", "lab")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": medium, "volume_id": vid}))
	params := map[string]any{"id": float64(pid), "volume_id": float64(vid)}

	if m := s.obj("POST", fmt.Sprintf("/api/chunks/%d/rewrite-copy", pid), map[string]any{"volume_id": vid}); s.status(m) != 428 {
		t.Fatalf("old route answered %v, want 428", m)
	}

	// A file in the copy's folder changes after prepare: refused, nothing re-written.
	p := prepareForTest(t, s.app, "rewrite-copy", params)
	if p.Word != wordOverwrite || len(p.Targets) < 3 {
		t.Fatalf("prepare: word %q, %d target(s)", p.Word, len(p.Targets))
	}
	readme := filepath.Join(medium, name, "RESTORE.txt")
	mustWrite(t, readme, "edited on the medium\n")
	_, err := s.app.ExecuteDestructive(testSession, p.Token, "Overwrite")
	refusedWith(t, err, 409, "changed copy")
	assertFileBytes(t, readme, "edited on the medium\n")
	for _, cp := range s.app.Store.Chunk(pid).Copies {
		if cp.Superseded {
			t.Fatal("the copy was marked superseded although the re-write was refused")
		}
	}

	// As prepared: the re-write runs.
	p = prepareForTest(t, s.app, "rewrite-copy", params)
	res, err := s.app.ExecuteDestructive(testSession, p.Token, "Overwrite")
	if err != nil {
		t.Fatal(err)
	}
	var asJSON map[string]any // as the UI receives it
	b, _ := json.Marshal(res)
	_ = json.Unmarshal(b, &asJSON)
	s.job(asJSON)
	superseded := 0
	for _, cp := range s.app.Store.Chunk(pid).Copies {
		if cp.Superseded {
			superseded++
		}
	}
	if superseded != 1 {
		t.Errorf("%d superseded copies after the re-write, want 1", superseded)
	}
}
