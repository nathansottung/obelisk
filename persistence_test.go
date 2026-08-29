package main

import (
	"fmt"
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
