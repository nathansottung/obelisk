//go:build !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A job whose catalog change could not be saved never ends COMPLETED, even when
// the work called a Store method that does not return the save's error.
func TestJob_UnsavedCatalogChangeFailsTheJob(t *testing.T) {
	app, cid := dcApp(t)
	c := app.Store.AddChunk(Chunk{CollectionID: cid, Name: "JOB-SAVE", Status: "STAGED"})
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }
	resp, jerr := runJob(app, "write", "probe", func(p func(float64, string)) (map[string]any, error) {
		c.Status = "VERIFIED"
		app.Store.UpdateChunk(c) // swallows the save error
		return map[string]any{"ok": true}, nil
	})
	if jerr != nil {
		t.Fatal(jerr)
	}
	j := waitTerminalJob(t, app.Store, resp["job_id"].(int))
	app.Store.failSave = nil
	if j.Status != "FAILED" || !strings.Contains(j.Label, "catalog could not be saved") {
		t.Errorf("job %q %q, want FAILED naming the unsaved catalog", j.Status, j.Label)
	}
}

// A job's save that was put off (another job held a batch) is written before the
// job reports COMPLETED.
func TestJob_DeferredSaveIsWrittenBeforeCompleted(t *testing.T) {
	app, cid := dcApp(t)
	c := app.Store.AddChunk(Chunk{CollectionID: cid, Name: "JOB-DEFER", Status: "STAGED"})
	app.Store.BeginBatch() // another job's batch, still open
	defer app.Store.EndBatch()
	app.Store.batchInterval = time.Hour
	app.Store.Flush() // a real write now, so the next save is put off
	resp, jerr := runJob(app, "write", "probe", func(p func(float64, string)) (map[string]any, error) {
		c.Status = "VERIFIED"
		app.Store.UpdateChunk(c)
		return map[string]any{"ok": true}, nil
	})
	if jerr != nil {
		t.Fatal(jerr)
	}
	j := waitTerminalJob(t, app.Store, resp["job_id"].(int))
	if j.Status != "COMPLETED" {
		t.Fatalf("job %q: %s", j.Status, j.Label)
	}
	b, err := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	if err != nil || !strings.Contains(string(b), `"VERIFIED"`) {
		t.Error("the job reported COMPLETED before its catalog change was written")
	}
}
