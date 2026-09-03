package main

// cardcheck_test.go — the "is this card already backed up?" check. It must match by
// CONTENT (not name/path) against the whole inventory, split the card into
// already-safe vs new, name where the safe ones live, give a correct safe-to-format
// verdict, and — critically — touch nothing (read-only: no source root, no ingest).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func hasStr(list []string, sub string) bool {
	for _, s := range list {
		if s == sub {
			return true
		}
	}
	return false
}

func TestCardCheck_BackedUpVsNew(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Wedding", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "photo1.jpg", "AAA-content")
	writeFile(t, src, "photo2.jpg", "BBB-content")
	writeFile(t, src, "photo3.jpg", "CCC-content")
	scanInto(t, app, coll.ID, src)

	beforeSources := len(app.Store.SourceRoots())
	beforeFiles := len(app.Store.FilesOf(coll.ID))

	// The card: two frames whose CONTENT is already archived (different names/paths),
	// plus one genuinely new frame.
	card := t.TempDir()
	writeFile(t, card, "DCIM/100CANON/IMG_0001.JPG", "AAA-content") // == photo1
	writeFile(t, card, "DCIM/100CANON/IMG_0002.JPG", "BBB-content") // == photo2
	writeFile(t, card, "DCIM/100CANON/IMG_0003.JPG", "NEW-unsaved") // new

	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalFiles != 3 {
		t.Fatalf("total = %d, want 3", res.TotalFiles)
	}
	if res.BackedUpFiles != 2 || res.NewFiles != 1 {
		t.Fatalf("backed=%d new=%d, want 2 backed / 1 new", res.BackedUpFiles, res.NewFiles)
	}
	if res.SafeToFormat {
		t.Error("safe_to_format must be false while a new file exists")
	}
	// The new file is named so the user knows what to copy off.
	if len(res.New) != 1 || res.New[0].Rel != "DCIM/100CANON/IMG_0003.JPG" {
		t.Errorf("new list = %+v, want the one unsaved frame", res.New)
	}
	// Backed-up frames say WHERE they already live.
	if len(res.Sample) == 0 || !hasStr(res.Sample[0].Locations, "Archive: Wedding") {
		t.Errorf("backed-up sample missing its location: %+v", res.Sample)
	}
	if !hasStr(res.Where, "Archive: Wedding") {
		t.Errorf("where-summary should include the archive: %v", res.Where)
	}

	// READ-ONLY: the card was neither registered as a source nor ingested.
	if got := len(app.Store.SourceRoots()); got != beforeSources {
		t.Errorf("card check registered a source root (%d → %d)", beforeSources, got)
	}
	if got := len(app.Store.FilesOf(coll.ID)); got != beforeFiles {
		t.Errorf("card check mutated the catalog (files %d → %d)", beforeFiles, got)
	}
}

func TestCardCheck_SafeToFormat(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Trip", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "one")
	writeFile(t, src, "b.jpg", "two")
	scanInto(t, app, coll.ID, src)

	// A card holding only content that's already archived → safe to reformat.
	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "one")
	writeFile(t, card, "IMG_2.JPG", "two")
	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if !res.SafeToFormat || res.NewFiles != 0 || res.BackedUpFiles != 2 {
		t.Fatalf("expected safe-to-format with 2 backed/0 new, got %+v", res)
	}
	// Green only when the card was read COMPLETELY: nothing skipped, nothing unlistable.
	if res.Skipped != 0 || res.WalkErrors != 0 || len(res.Problems) != 0 {
		t.Fatalf("a fully readable card must report no losses, got skipped=%d walk=%d problems=%+v",
			res.Skipped, res.WalkErrors, res.Problems)
	}
}

// FAIL-CLOSED, gate 1: a file we could not hash is a file we cannot vouch for. It drops
// out of TotalFiles, so NewFiles stays 0 — the verdict must still refuse to go green.
func TestCardCheck_UnhashableFileBlocksFormat(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "one")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "one")        // already archived
	writeFile(t, card, "IMG_2.JPG", "unreadable") // will fail to hash

	scanFaultHook = func(p string) string {
		if filepath.Base(p) == "IMG_2.JPG" {
			return "hash"
		}
		return ""
	}
	defer func() { scanFaultHook = nil }()

	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the unhashable frame)", res.Skipped)
	}
	if res.NewFiles != 0 {
		t.Errorf("new = %d, want 0 — the point is that a skip is invisible to NewFiles", res.NewFiles)
	}
	if res.SafeToFormat {
		t.Fatalf("safe_to_format must be false when a file could not be read: %+v", res)
	}
	if len(res.Problems) == 0 {
		t.Error("the result must name what could not be read, not just flip the boolean")
	}
}

// FAIL-CLOSED, gate 2: a folder we could not LIST hides files that never reach the hash
// pass at all — so they are invisible to Skipped by construction. WalkErrors is therefore
// its own independent gate, not a duplicate of Skipped.
func TestCardCheck_UnlistableDirBlocksFormat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "one")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "one")                 // already archived
	writeFile(t, card, "DCIM/LOCKED/IMG_9.JPG", "unsaved") // hidden behind an unreadable folder
	locked := filepath.Join(card, "DCIM", "LOCKED")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755) // let TempDir clean up

	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.WalkErrors < 1 {
		t.Errorf("walk_errors = %d, want >= 1 (the unlistable folder)", res.WalkErrors)
	}
	// The hidden frame is counted NOWHERE else — that is exactly why WalkErrors must gate.
	if res.Skipped != 0 || res.NewFiles != 0 {
		t.Errorf("skipped=%d new=%d — a walk loss is invisible to both, by construction", res.Skipped, res.NewFiles)
	}
	if res.SafeToFormat {
		t.Fatalf("safe_to_format must be false when a folder could not be listed: %+v", res)
	}
	if len(res.Problems) == 0 {
		t.Error("the result must name the folder it could not list")
	}
}

// Content on an inventoried (adopted) drive counts too — not just the NAS archive.
func TestCardCheck_MatchesDriveSnapshot(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Archive", ArchiveSourceless)
	vol := app.Store.AddVolume(Volume{Label: "OLD-DRIVE-7", Kind: "HDD"})
	// A snapshot of a drive we inventoried, holding a known frame by content hash.
	sha := "deadbeef00112233445566778899aabbccddeeff00112233445566778899aabb"
	app.Store.PutVolumeSnapshot(&VolumeSnapshot{
		VolumeID: vol.ID, Label: "OLD-DRIVE-7", TotalFiles: 1,
		Files: []SnapFile{{RelPath: "2019/x.nef", Hash: sha, SizeBytes: 100}},
	})
	_ = coll

	idx := app.knownContent()
	if locs, ok := idx[sha]; !ok || !hasStr(locs, "Drive: OLD-DRIVE-7") {
		t.Errorf("known-content index missing the snapshot frame: %v (ok=%v)", idx[sha], ok)
	}
}
