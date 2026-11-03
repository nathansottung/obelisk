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
