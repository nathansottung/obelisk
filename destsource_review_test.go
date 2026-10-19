//go:build !guionly

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// A source that opens at its registered path but on a different volume than the
// one recorded (another drive now holds that letter) leaves its real drive
// somewhere else. That drive still holds the source, so a destination on it must
// be refused.
func TestDestSource_RecordedVolumeStillProtectedWhenPathMoved(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	if pathVolumeID(src) == "" {
		t.Skip("this filesystem has no readable volume identity")
	}
	coll := app.Store.AddCollection("Moved drive")
	f := app.Store.AddFolder(coll.ID, src)
	f.VolumeID = "test:the-original-drive" // recorded when the source was on another drive

	snap, err := app.Store.sourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(`Z:\`, "Photos")
	err = snap.decideOffline("test:the-original-drive", dest, dest, dest)
	if err == nil || !strings.Contains(err.Error(), "attached at a different location") {
		t.Fatalf("destination on the source's recorded drive: got %v, want a refusal", err)
	}
	if err := snap.decideOffline("test:another-drive", dest, dest, dest); err != nil {
		t.Fatalf("destination on an unrelated drive: %v", err)
	}
	if f.VolumeID != "test:the-original-drive" {
		t.Fatalf("the recorded volume was overwritten: %q", f.VolumeID)
	}
}
