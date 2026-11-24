//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On a real exFAT volume (no hard links), a mirror still keeps the older
// version: copied aside and checked before the new copy takes the name. CI makes
// the volume (a small virtual disk) and sets OBELISK_EXFAT_TEST_DIR, and
// OBELISK_REQUIRE_EXFAT=1 makes a missing volume a failure, never a skip.
func TestMirrorKeep_OnExFAT(t *testing.T) {
	root := os.Getenv("OBELISK_EXFAT_TEST_DIR")
	if root == "" {
		if os.Getenv("OBELISK_REQUIRE_EXFAT") == "1" {
			t.Fatal("OBELISK_REQUIRE_EXFAT=1 but OBELISK_EXFAT_TEST_DIR is not set: the exFAT volume was not made")
		}
		t.Skip("SKIP (NOT TESTED): no exFAT volume here (set OBELISK_EXFAT_TEST_DIR); the no-link path is tested on NTFS by TestMirrorKeep_KeepsTheOlderVersion")
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatalf("the exFAT test volume %s cannot be opened: %v", root, err)
	}
	fsName, _, err := destVolumeInformation(f)
	f.Close()
	if err != nil || !strings.EqualFold(fsName, "exFAT") {
		t.Fatalf("%s is %q (%v), not exFAT", root, fsName, err)
	}
	dest, err := os.MkdirTemp(root, "mirror-keep-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dest) }) // test cleanup of the test's own folder
	app, collID, vol, _, _ := keptScenarioAt(t, dest)
	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(dest, "a.txt")); got != "alpha v2, changed\n" {
		t.Errorf("the new copy is not in place: %q", got)
	}
	run := keptRunDir(t, dest)
	if got := mustRead(t, filepath.Join(run, "a.txt")); got != "alpha v1\n" {
		t.Errorf("the earlier version was not kept on exFAT: %q", got)
	}
	if len(res.KeptOwn) != 1 || runStatus(t, run) != "complete" {
		t.Errorf("kept report %+v, run %s", res.KeptSummary, runStatus(t, run))
	}
}
