package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

// TestWriteCatalog_SaveFailurePropagates proves the P0-1 fix end-to-end: when a
// persistence write fails, an error-returning mutator (RemoveCollection here) SURFACES
// the failure instead of reporting a false success, and the on-disk catalog is left
// intact — the "removed" archive is still present after a reopen. Uses the failSave
// fault-injection seam on Store (nil in production, consulted in writeCatalog).
func TestWriteCatalog_SaveFailurePropagates(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	c := st.AddCollection("Keeper") // persisted to disk (failSave still nil)

	// Inject a disk-write failure, then attempt the permanent removal.
	st.failSave = func() error { return fmt.Errorf("disk full (injected)") }
	if _, err := st.RemoveCollection(c.ID); err == nil {
		t.Fatal("RemoveCollection must return the injected save error, not a false success")
	}

	// The failed write must not have persisted the removal: a fresh open still sees it.
	st.failSave = nil
	st2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if st2.Collection(c.ID) == nil {
		t.Fatal("archive must still be present after a failed removal (the write never landed)")
	}
}

// TestOpenStore_RecoverySaveFailureIsNonFatal proves option (a) for the recovery save inside
// OpenStore: when the one-time recovery/seeding write fails, OpenStore still returns a usable
// Store and logs a prominent warning instead of refusing to boot. A fresh open always
// "recovers" (it seeds the built-in profiles and routing template), so the injected fault fires
// on that in-OpenStore save via the openStoreFailSave seam (nil in production).
func TestOpenStore_RecoverySaveFailureIsNonFatal(t *testing.T) {
	var logbuf bytes.Buffer
	log.SetOutput(&logbuf)
	defer log.SetOutput(os.Stderr)

	openStoreFailSave = func() error { return fmt.Errorf("disk full (injected)") }
	defer func() { openStoreFailSave = nil }()

	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("recovery-save failure must be non-fatal: OpenStore returned error %v", err)
	}
	if st == nil {
		t.Fatal("OpenStore must return a usable Store even when the recovery save fails")
	}
	if !strings.Contains(logbuf.String(), "recovery could not be persisted") {
		t.Fatalf("expected a prominent recovery-save warning, got log: %q", logbuf.String())
	}

	// The returned Store is genuinely usable: clear the fault and a normal mutation persists
	// and survives a reopen.
	st.failSave = nil
	openStoreFailSave = nil
	c := st.AddCollection("Post-recovery")
	st2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if st2.Collection(c.ID) == nil {
		t.Fatal("post-recovery mutation must persist once the write fault is cleared")
	}
}
