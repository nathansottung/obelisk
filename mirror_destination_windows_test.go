//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A FAT32 destination refuses a mirror holding a file over 4 GB before anything is
// written, rather than failing part-way through.
func TestMirror_FAT32RefusesOversizeFileUpFront(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "Big", map[string]string{"small.txt": "s", "huge.bin": "h"})
	for _, f := range app.Store.FilesOf(collID) {
		if f.RelPath == "huge.bin" {
			f.SizeBytes = 5 << 30 // as cataloged; the fixture need not be 5 GB on disk
		}
	}
	withDestVolume(t, "FAT32", 0)
	dest := filepath.Join(t.TempDir(), "mirror")
	chooseAnyway(t, app, dest)
	_, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err == nil || !strings.Contains(err.Error(), "FAT32") || !strings.Contains(err.Error(), "4 GB") {
		t.Fatalf("got %v, want a FAT32 4 GB refusal", err)
	}
	if ents, _ := os.ReadDir(dest); len(ents) != 0 {
		t.Fatalf("a refused mirror wrote %d entries", len(ents))
	}
	backup := filepath.Join(t.TempDir(), "backup")
	chooseAnyway(t, app, backup)
	_, err = app.BackupChanges(collID, nil, vol.ID, BaseVolume, ModeMirror, backup, 0, "", noProgMirror)
	if err == nil || !strings.Contains(err.Error(), "4 GB") {
		t.Fatalf("back up changes: got %v, want a FAT32 4 GB refusal", err)
	}
}

// On exFAT the mirror runs and its result lists the checks that could not run.
func TestMirror_ExFATResultListsUncheckedItems(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "Card", map[string]string{"a.txt": "alpha"})
	withDestVolume(t, "exFAT", 0)
	dest := filepath.Join(t.TempDir(), "mirror")
	chooseAnyway(t, app, dest)
	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatal(err)
	}
	if res.Mirrored != 1 {
		t.Fatalf("mirrored %d", res.Mirrored)
	}
	notes := strings.Join(res.DestinationNotes, "\n")
	for _, want := range []string{"folder permissions", "hard links", "stable file IDs"} {
		if !strings.Contains(notes, want) {
			t.Errorf("result notes do not mention %q:\n%s", want, notes)
		}
	}
}

// A registered source on a drive that is not attached does not stop a mirror.
func TestMirror_UnpluggedSourceDoesNotBlock(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "Here", map[string]string{"a.txt": "alpha"})
	other := app.Store.AddCollection("Unplugged")
	app.Store.AddFolder(other.ID, filepath.Join(unusedDriveLetter(t), "Photos"))
	res, err := app.MirrorToVolume(collID, nil, createdForTest(t, app, filepath.Join(t.TempDir(), "mirror")), vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("an unplugged source blocked the mirror: %v", err)
	}
	if res.Mirrored != 1 || len(res.DestinationNotes) != 0 {
		t.Fatalf("mirrored %d, notes %q", res.Mirrored, res.DestinationNotes)
	}
}
