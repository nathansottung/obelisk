package main

// durability_gate_test.go — I-003 on the build path: a package the catalog cannot
// record as STAGED must not be reported as a successful build, and a verify result
// that failed to persist must never be left looking recorded. These drive the real
// error-returning store methods (UpdateChunkErr / AppendVerifyEventErr) through the
// failSave fault-injection seam.
//
// The build test shares the package-level failSave seam, so none call t.Parallel().

import (
	"fmt"
	"strings"
	"testing"
)

// (a)+(b) The terminal STAGED write is the durability gate: when the catalog cannot
// record the built package as staged, BuildChunk surfaces a wrapped error instead of
// reporting success, and a reopened catalog does not show the package as STAGED.
func TestBuildChunk_TerminalStageWriteIsDurabilityGate(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	src, refs := makeSource(t)
	refs = hashRefs(t, src, refs)
	c := newVerifyChunk(app, src, refs, false) // plaintext keeps the build simple

	// Fail every save for the duration of the build. Interim BUILDING/verify writes are
	// best-effort and must not abort the build; only the terminal STAGED write is gated.
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }

	err := app.BuildChunk(c.ID, noProg)
	if err == nil {
		t.Fatal("BuildChunk must return an error when the terminal STAGED write fails")
	}
	if !strings.Contains(err.Error(), "catalog could not record it as staged") {
		t.Errorf("expected the durability-gate wrapped error, got: %v", err)
	}

	// A fresh open must not show the package as STAGED — the staged write never landed.
	app.Store.failSave = nil
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := store2.Chunk(c.ID); got == nil {
		t.Fatal("package missing after reopen")
	} else if got.Status == "STAGED" {
		t.Errorf("reopened catalog must not show the package STAGED (the write never landed), got %s", got.Status)
	}
}

// (c) AppendVerifyEventErr is atomic against its save: a failed save leaves no phantom
// event, and a clean save persists exactly one.
func TestAppendVerifyEventErr_RollsBackOnSaveFailure(t *testing.T) {
	app, _ := newTestApp(t, map[string]string{}) // no native tools needed for a pure-store test
	c := app.Store.AddChunk(Chunk{Name: "VE-PKG", Status: "WRITTEN", MediaKind: "CUSTOM"})

	// Failed save: the append must roll back so the chunk shows no verify event.
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }
	if err := app.Store.AppendVerifyEventErr(c, VerifyEvent{OK: true, Path: "X:/pkg"}); err == nil {
		t.Fatal("AppendVerifyEventErr must return the injected save error")
	}
	if n := len(c.VerifyEvents); n != 0 {
		t.Fatalf("failed save must leave no phantom event, found %d", n)
	}

	// Clean save: exactly one event, and it survives a reopen.
	app.Store.failSave = nil
	if err := app.Store.AppendVerifyEventErr(c, VerifyEvent{OK: true, Path: "X:/pkg"}); err != nil {
		t.Fatalf("clean AppendVerifyEventErr must succeed: %v", err)
	}
	if n := len(c.VerifyEvents); n != 1 {
		t.Fatalf("clean save must persist exactly one event, found %d", n)
	}
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if n := len(store2.Chunk(c.ID).VerifyEvents); n != 1 {
		t.Fatalf("reopened catalog must show exactly one persisted event, found %d", n)
	}
}
