//go:build !guionly

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// swapTestHandler is the API as main serves it, behind the swap gate.
func swapTestHandler(app *App) http.Handler {
	mux := http.NewServeMux()
	api(mux, app)
	return app.swapGate(mux)
}

// apiPost sends a JSON POST through h and returns the status and body.
func apiPost(h http.Handler, path, body string) (int, string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func locationNames(s *Store) []string {
	var out []string
	for _, l := range s.Locations() {
		out = append(out, l.Name)
	}
	return out
}

// During a restore's swap, on either engine, every API request is refused with
// 409 and every store edit is refused: none changes the catalog that was copied,
// none reaches the restored one, and none reports success.
func TestSwapGate_ARestoreRefusesEveryEditDuringTheSwap(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			src := abTestApp(t)
			abPopulate(t, src)
			exp, err := src.ExportAppBackup(t.TempDir(), false)
			if err != nil {
				t.Fatal(err)
			}
			dst := abTestApp(t)
			held := dst.Store.AddCollection("Held")
			h := swapTestHandler(dst)
			reached := false
			dst.duringSwap = func(what string) {
				reached = true
				if code, body := apiPost(h, "/api/locations", `{"name":"Edited during the swap"}`); code != http.StatusConflict {
					t.Errorf("an edit during %s answered %d %s, want 409", what, code, body)
				}
				if _, err := dst.Store.NewJob("scan", "during the swap"); err == nil {
					t.Errorf("a job started during %s", what)
				}
				if err := dst.Store.SetCollectionIntegrity(held.ID, &Integrity{}); err == nil {
					t.Errorf("a store edit during %s reported success", what)
				}
			}
			res, err := dst.RestoreAppBackup(exp.TarPath)
			if err != nil {
				t.Fatalf("restore: %v", err)
			}
			if !reached {
				t.Fatal("the restore never reached its swap")
			}
			if got := locationNames(dst.Store); len(got) != 0 {
				t.Fatalf("the restored catalog holds locations %v", got)
			}
			pre, err := OpenStore(res.PreRestoreDir)
			if err == nil {
				defer pre.Close()
				if got := locationNames(pre); len(got) != 0 {
					t.Fatalf("the pre-restore copy holds locations %v", got)
				}
			}
			// After the swap the API answers again, on the restored catalog.
			if code, body := apiPost(h, "/api/locations", `{"name":"After"}`); code != http.StatusOK {
				t.Fatalf("an edit after the restore answered %d %s", code, body)
			}
			if got := locationNames(dst.Store); strings.Join(got, ",") != "After" {
				t.Fatalf("after the restore the catalog holds locations %v", got)
			}
		})
	}
}

// A restore waits for an API request already in progress to finish before it
// holds the catalog, so that request's edit is in the pre-restore copy rather
// than lost in a store being closed.
func TestSwapGate_ARestoreWaitsForARequestInProgress(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			src := abTestApp(t)
			abPopulate(t, src)
			exp, err := src.ExportAppBackup(t.TempDir(), false)
			if err != nil {
				t.Fatal(err)
			}
			dst := abTestApp(t)
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/slow-edit", func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-release
				dst.Store.AddLocation("In flight", false, "")
				close(done)
			})
			h := dst.swapGate(mux)
			go apiPost(h, "/api/slow-edit", `{}`)
			<-entered
			swapped := make(chan struct{})
			dst.duringSwap = func(string) {
				select {
				case <-done:
				default:
					t.Error("the restore held the catalog while a request was still in progress")
				}
				close(swapped)
			}
			errc := make(chan error, 1)
			var res RestoreResult
			go func() {
				var err error
				res, err = dst.RestoreAppBackup(exp.TarPath)
				errc <- err
			}()
			select {
			case <-swapped:
				t.Fatal("the restore did not wait for the request in progress")
			case <-time.After(200 * time.Millisecond):
			}
			close(release)
			if err := <-errc; err != nil {
				t.Fatalf("restore: %v", err)
			}
			pre, err := OpenStore(res.PreRestoreDir)
			if err != nil {
				t.Fatalf("open the pre-restore copy: %v", err)
			}
			defer pre.Close()
			if got := locationNames(pre); strings.Join(got, ",") != "In flight" {
				t.Fatalf("the pre-restore copy holds locations %v, want the edit in flight", got)
			}
		})
	}
}

// A data folder move refuses every edit while it copies, on either engine: the
// edit reaches neither the original folder nor the copy the app switches to.
func TestSwapGate_AMoveRefusesEveryEditWhileItCopies(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			legacy := t.TempDir()
			target := filepath.Join(t.TempDir(), "new-home")
			s, err := openTestStore(t, legacy)
			if err != nil {
				t.Fatal(err)
			}
			app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
			app.Store.AddLocation("Before", false, "")
			held := app.Store.AddCollection("Held")
			h := swapTestHandler(app)
			reached := false
			app.duringSwap = func(what string) {
				reached = true
				if code, body := apiPost(h, "/api/locations", `{"name":"During"}`); code != http.StatusConflict {
					t.Errorf("an edit during %s answered %d %s, want 409", what, code, body)
				}
				if err := app.Store.SetCollectionIntegrity(held.ID, &Integrity{}); err == nil {
					t.Errorf("a store edit during %s reported success", what)
				}
			}
			if _, err := app.migrateDataDir(legacy, target); err != nil {
				t.Fatalf("move: %v", err)
			}
			t.Cleanup(func() { app.Store.Close() })
			if !reached {
				t.Fatal("the move never held the catalog")
			}
			if got := locationNames(app.Store); strings.Join(got, ",") != "Before" {
				t.Fatalf("the moved catalog holds locations %v", got)
			}
			if got := locationNames(openEngineStore(t, engine, legacy)); strings.Join(got, ",") != "Before" {
				t.Fatalf("the original folder holds locations %v", got)
			}
		})
	}
}

// A refused restore lets edits in again, on the same store, with nothing lost.
func TestSwapGate_ARefusedRestoreLetsEditsInAgain(t *testing.T) {
	dst := abTestApp(t)
	h := swapTestHandler(dst)
	bad := filepath.Join(t.TempDir(), "not-a-backup.tar")
	if err := os.WriteFile(bad, []byte("not a tar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.RestoreAppBackup(bad); err == nil {
		t.Fatal("a damaged backup was restored")
	}
	if code, body := apiPost(h, "/api/locations", `{"name":"After"}`); code != http.StatusOK {
		t.Fatalf("an edit after a refused restore answered %d %s", code, body)
	}
}

// The store's own hold: while held, no job starts and no edit is saved; a hold is
// refused while a job runs; released, edits save again.
func TestStoreHold_RefusesJobsAndEdits(t *testing.T) {
	s, err := openTestStore(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := s.AddCollection("A")
	j, err := s.NewJob("scan", "running")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.holdEdits("a restore"); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("a hold while a job runs: %v", err)
	}
	if err := s.FinishJob(j.ID, 1, "", "COMPLETED", nil, nil); err != nil {
		t.Fatal(err)
	}
	release, err := s.holdEdits("a restore")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.NewJob("scan", "held"); !errors.Is(err, errCatalogHeld) {
		t.Fatalf("a job during the hold: %v", err)
	}
	if err := s.SetCollectionIntegrity(c.ID, &Integrity{}); !errors.Is(err, errCatalogHeld) {
		t.Fatalf("an edit during the hold: %v", err)
	}
	release()
	if err := s.SetCollectionIntegrity(c.ID, &Integrity{}); err != nil {
		t.Fatalf("an edit after the hold: %v", err)
	}
}

// A destructive action is refused while a restore swaps the catalog, and a
// restore run as a destructive action still runs: it leaves the gate before it
// takes it for its swap.
func TestSwapGate_ADestructiveActionDuringARestoreIsRefused(t *testing.T) {
	src := abTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := abTestApp(t)
	ev := dst.Store.AddEvent(&Event{Name: "Keep me"})
	del := prepareForTest(t, dst, "record-delete", map[string]any{"kind": "event", "id": float64(ev.ID)})
	restore := prepareForTest(t, dst, "appbackup-restore", map[string]any{"tar_path": exp.TarPath})
	reached := false
	dst.duringSwap = func(string) {
		reached = true
		_, err := dst.ExecuteDestructive(testSession, del.Token, del.Word)
		var de *destructiveError
		if !errors.As(err, &de) || de.status != http.StatusConflict {
			t.Errorf("a destructive action during the restore: %v, want a 409 refusal", err)
		}
	}
	if _, err := dst.ExecuteDestructive(testSession, restore.Token, restore.Word); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !reached {
		t.Fatal("the restore never reached its swap")
	}
}

// A held store writes nothing, on either engine and by any path: not a flush, not
// the end of a batch, not the job board. Only its holder's copy and close do.
func TestStoreHold_WritesNothingByAnyPath(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			dir := t.TempDir()
			s, err := openTestStore(t, dir)
			if err != nil {
				t.Fatal(err)
			}
			s.AddCollection("Saved")
			j, err := s.NewJob("scan", "before the hold")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.FinishJob(j.ID, 1, "", "COMPLETED", nil, nil); err != nil {
				t.Fatal(err)
			}
			release, err := s.holdEdits("a restore")
			if err != nil {
				t.Fatal(err)
			}
			before := persistedCatalogNames(t, engine, dir)
			// An edit that reached memory past every gate, as a missed one would.
			s.mu.Lock()
			s.c.Collections = append(s.c.Collections, &Collection{ID: 99, Name: "Refused"})
			s.dirty = true
			s.mu.Unlock()
			s.BeginBatch()
			if err := s.EndBatch(); err == nil {
				t.Error("EndBatch wrote a held store")
			}
			if err := s.FlushPending(); err == nil {
				t.Error("FlushPending wrote a held store")
			}
			if err := s.SetJob(j.ID, 1, "relabelled", "COMPLETED"); !errors.Is(err, errCatalogHeld) {
				t.Errorf("a job board write on a held store: %v", err)
			}
			if after := persistedCatalogNames(t, engine, dir); after != before {
				t.Fatalf("a held store wrote: %s, then %s", before, after)
			}
			release()
		})
	}
}

// persistedCatalogNames reads the archive names dir's catalog holds on disk.
func persistedCatalogNames(t *testing.T, engine, dir string) string {
	t.Helper()
	if engine == "json" {
		var c catalog
		raw, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeCatalogJSON(raw, &c); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, x := range c.Collections {
			names = append(names, x.Name)
		}
		return strings.Join(names, ",")
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	c, err := exportCatalogDB(d)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, x := range c.Collections {
		names = append(names, x.Name)
	}
	return strings.Join(names, ",")
}

// A request that never finishes does not hold the app off: the swap gives up,
// refuses with nothing changed, and requests are let in again. Reads outside the
// catalog (the folder browser) never hold a swap at all.
func TestSwapGate_AStalledRequestDoesNotHoldTheAppOff(t *testing.T) {
	prev := swapDrainWait
	swapDrainWait = 200 * time.Millisecond
	t.Cleanup(func() { swapDrainWait = prev })
	app := abTestApp(t)
	release, entered, browsing := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(browsing) })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stalled", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
	})
	mux.HandleFunc("GET /api/browse", func(w http.ResponseWriter, r *http.Request) { <-browsing })
	h := app.swapGate(mux)
	go apiGet(h, "/api/browse")
	go apiGet(h, "/api/stalled")
	<-entered
	if _, err := app.beginSwap("a restore"); err == nil || !strings.Contains(err.Error(), "in progress") {
		t.Fatalf("a swap with a stalled request: %v", err)
	}
	if leave, err := app.enterCatalog(); err != nil {
		t.Fatalf("after the swap gave up, a request was refused: %v", err)
	} else {
		leave()
	}
	close(release)
	// With the stalled request gone, only the folder browser is left, still
	// waiting, and it does not hold the swap.
	deadline := time.Now().Add(5 * time.Second)
	for {
		end, err := app.beginSwap("a restore")
		if err == nil {
			end()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the swap never started: %v", err)
		}
	}
}
