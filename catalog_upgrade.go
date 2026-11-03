//go:build !guionly

package main

// catalog_upgrade.go — the first start on the catalog database engine moves an
// existing catalog.json into catalog.db as a job the UI shows.
//
// The move (migrateCatalogToDB) can take minutes on a large catalog, so the server
// starts first and the move runs behind the swap gate (swapgate.go): while it runs,
// GET /api/jobs answers with the upgrade job and its progress, GET /api/health says
// the catalog is being upgraded, and every other API request is answered 503 with
// nothing changed. When the move has proved the database holds every record, the
// store opens on it and the job is recorded on the job board, COMPLETED with what
// was moved and where the old catalog.json bytes were kept.
//
// A move that is refused changes nothing: catalog.json stays the catalog, the job
// says FAILED with the reason, and the API keeps answering 503 with it. Obelisk
// started with OBELISK_CATALOG_ENGINE=json keeps using catalog.json. Stopping the
// app during the move is safe: each step of the move is resumable, and the next
// start finishes it or starts it again.

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// catalogUpgradeNeeded reports whether opening dataDir would move catalog.json
// into a new catalog database: the database engine, no catalog.db, and a
// catalog.json. Anything else (a finished or half-finished move, an unreadable
// folder) is left to OpenStore, which finishes or refuses it at once.
func catalogUpgradeNeeded(dataDir string) (bool, error) {
	engine, err := catalogEngine()
	if err != nil || engine != "sqlite" || openCatalogBackend == nil {
		return false, err
	}
	if _, err := os.Lstat(filepath.Join(dataDir, catalogDBName)); !errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	_, err = os.Lstat(filepath.Join(dataDir, "catalog.json"))
	return err == nil, nil
}

// beginCatalogUpgrade holds the swap gate for the move of dataDir's catalog.json
// and starts it. The app has no store until it finishes; done is closed then,
// whatever the outcome.
func (a *App) beginCatalogUpgrade() (done <-chan struct{}) {
	u := &catalogUpgrade{job: Job{ID: 0, Kind: catalogUpgradeJobKind, Label: catalogUpgradeLabel,
		Status: "RUNNING", CreatedAt: time.Now().UTC()}, onStep: a.upgradeStep}
	g := &a.swap
	g.mu.Lock()
	g.what, g.upgrade = "the catalog upgrade", u
	g.mu.Unlock()
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		a.runCatalogUpgrade(u)
	}()
	return ch
}

// runCatalogUpgrade moves the catalog, opens the store on it and records the job,
// or leaves the refusal for every request.
func (a *App) runCatalogUpgrade(u *catalogUpgrade) {
	start := time.Now()
	log.Printf("upgrading the catalog in %s to the catalog database", a.DataDir)
	res, err := migrateCatalogToDB(a.DataDir, catalogMigrateOptions{progress: u.progress})
	var store *Store
	if err == nil {
		if store, err = OpenStore(a.DataDir); err != nil {
			err = fmt.Errorf("the catalog moved into %s, every record checked, but opening it failed: %w",
				filepath.Join(a.DataDir, catalogDBName), err)
		}
	}
	g := &a.swap
	if err != nil {
		log.Printf("the catalog upgrade was refused: %v", err)
		now := time.Now().UTC()
		u.mu.Lock()
		u.err = err
		u.job.Status, u.job.FinishedAt = "FAILED", &now
		u.job.Label = catalogUpgradeLabel + " — ERROR: " + err.Error()
		u.mu.Unlock()
		return
	}
	result := map[string]any{"catalog": filepath.Join(a.DataDir, catalogDBName), "kept_catalog_json": res.Backup,
		"rows": res.Counts, "seconds": time.Since(start).Seconds()}
	if rerr := store.recordFinishedJob(catalogUpgradeJobKind, catalogUpgradeLabel, u.snapshot().CreatedAt, result); rerr != nil {
		// The catalog moved and opened; only the job's own record is missing.
		log.Printf("the catalog upgrade finished, but its job could not be recorded: %v", rerr)
	}
	log.Printf("the catalog is now in %s (the old catalog.json is kept as %s)", filepath.Join(a.DataDir, catalogDBName), res.Backup)
	g.mu.Lock()
	a.Store = store
	g.what, g.upgrade = "", nil
	g.mu.Unlock()
}

// answerDuringUpgrade answers an API request while the move runs or after it was
// refused, and reports whether it did.
func (a *App) answerDuringUpgrade(w http.ResponseWriter, r *http.Request) bool {
	g := &a.swap
	g.mu.Lock()
	u := g.upgrade
	g.mu.Unlock()
	if u == nil {
		return false
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/jobs":
		jsonOut(w, []Job{u.snapshot()})
	case r.Method == http.MethodGet && r.URL.Path == "/api/health":
		j := u.snapshot()
		out := map[string]any{"ok": j.Status == "RUNNING", "version": appVersion, "schema_version": currentSchemaVersion,
			"upgrading": j.Status == "RUNNING", "upgrade": j}
		if j.Status != "RUNNING" {
			out["error"] = u.refusal().Error()
		}
		jsonOut(w, out)
	default:
		jsonErr(w, http.StatusServiceUnavailable, u.refusal())
	}
	return true
}
