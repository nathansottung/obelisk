package main

// cardcheck_test.go — the card check. It must match by CONTENT (not name/path), report
// how many backup copies each file has and where, never give a format verdict, mark a
// result incomplete when anything was not read, and — critically — touch nothing
// (read-only: no source root, no ingest).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func hasStr(list []string, sub string) bool {
	for _, s := range list {
		if s == sub {
			return true
		}
	}
	return false
}

func TestCardCheck_BackedUpVsNoCopy(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Wedding", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "photo1.jpg", "AAA-content")
	writeFile(t, src, "photo2.jpg", "BBB-content")
	writeFile(t, src, "photo3.jpg", "CCC-content")
	scanInto(t, app, coll.ID, src)
	vol := app.Store.AddVolume(Volume{Label: "BACKUP-1", Kind: "HDD", Serial: "SER-B1"})
	cardPackage(t, app, coll.ID, "PKG-1", []string{"AAA-content", "BBB-content"}, map[*Volume]time.Time{vol: time.Now()})

	beforeSources := len(app.Store.SourceRoots())
	beforeFiles := len(testFiles(t, app.Store.FilesOf(coll.ID)))

	// The card: two frames whose CONTENT has a backup copy (different names/paths),
	// plus one frame that was never copied anywhere.
	card := t.TempDir()
	writeFile(t, card, "DCIM/100CANON/IMG_0001.JPG", "AAA-content") // == photo1
	writeFile(t, card, "DCIM/100CANON/IMG_0002.JPG", "BBB-content") // == photo2
	writeFile(t, card, "DCIM/100CANON/IMG_0003.JPG", "NEW-unsaved") // no copy

	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.TotalFiles != 3 {
		t.Fatalf("total = %d, want 3", res.TotalFiles)
	}
	if res.BackedUpFiles != 2 || res.NoCopyFiles != 1 {
		t.Fatalf("backed=%d no_copy=%d, want 2 backed / 1 no copy", res.BackedUpFiles, res.NoCopyFiles)
	}
	// The uncopied file is named so the user knows what to copy off.
	if f := onlyFile(t, res.NoCopy); f.Rel != "DCIM/100CANON/IMG_0003.JPG" {
		t.Errorf("no-copy list = %+v, want the one unsaved frame", res.NoCopy)
	}
	// Backed-up frames say WHERE their copies live.
	if len(res.BackedUp) == 0 || len(res.BackedUp[0].Copies) != 1 || res.BackedUp[0].Copies[0].Where != "BACKUP-1" {
		t.Errorf("backed-up file missing its copy: %+v", res.BackedUp)
	}
	if !hasStr(res.Where, "BACKUP-1") {
		t.Errorf("where-summary should name the backup drive: %v", res.Where)
	}

	// READ-ONLY: the card was neither registered as a source nor ingested.
	if got := len(app.Store.SourceRoots()); got != beforeSources {
		t.Errorf("card check registered a source root (%d → %d)", beforeSources, got)
	}
	if got := len(testFiles(t, app.Store.FilesOf(coll.ID))); got != beforeFiles {
		t.Errorf("card check mutated the catalog (files %d → %d)", beforeFiles, got)
	}
}

// A source scan alone is a record, not a backup: content known only from the archive
// has no backup copy, and the report says when it was recorded.
func TestCardCheck_SourceScanAloneIsNoBackup(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Trip", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "one")
	writeFile(t, src, "b.jpg", "two")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "one")
	writeFile(t, card, "IMG_2.JPG", "two")
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.BackedUpFiles != 0 || res.NoCopyFiles != 2 || res.RecordedOnly != 2 {
		t.Fatalf("a source scan alone must show no backup copy: %+v", res)
	}
	for _, f := range res.NoCopy {
		if f.RecordedIn != "Archive: Trip" || f.RecordedAt == nil {
			t.Fatalf("a recorded file must say where and when: %+v", f)
		}
	}
	// A fully readable card reports no losses.
	if res.Incomplete || res.Skipped != 0 || res.WalkErrors != 0 || len(res.Problems) != 0 {
		t.Fatalf("a fully readable card must report no losses, got incomplete=%v skipped=%d walk=%d problems=%+v",
			res.Incomplete, res.Skipped, res.WalkErrors, res.Problems)
	}
}

// Gate 1: a file we could not hash drops out of TotalFiles, so NoCopyFiles stays 0 —
// the result must still say it is incomplete, and name what it could not read.
func TestCardCheck_UnhashableFileMakesItIncomplete(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "one")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "one")        // recorded
	writeFile(t, card, "IMG_2.JPG", "unreadable") // will fail to hash

	scanFaultHook = func(p string) string {
		if filepath.Base(p) == "IMG_2.JPG" {
			return "hash"
		}
		return ""
	}
	defer func() { scanFaultHook = nil }()

	res := checkCard(t, app, card)
	if res.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the unhashable frame)", res.Skipped)
	}
	if res.TotalFiles != 1 {
		t.Errorf("total = %d, want 1 — a skip is invisible to the file counts", res.TotalFiles)
	}
	if !res.Incomplete {
		t.Fatalf("a file that could not be read makes the result incomplete: %+v", res)
	}
	if len(res.Problems) == 0 {
		t.Error("the result must name what could not be read, not just flip a flag")
	}
}

// Gate 2: a folder we could not LIST hides files that never reach the hash pass at all
// — so they are invisible to Skipped by construction. WalkErrors is its own gate.
func TestCardCheck_UnlistableDirMakesItIncomplete(t *testing.T) {
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
	writeFile(t, card, "IMG_1.JPG", "one")                 // recorded
	writeFile(t, card, "DCIM/LOCKED/IMG_9.JPG", "unsaved") // hidden behind an unreadable folder
	locked := filepath.Join(card, "DCIM", "LOCKED")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755) // let TempDir clean up

	res := checkCard(t, app, card)
	if res.WalkErrors < 1 {
		t.Errorf("walk_errors = %d, want >= 1 (the unlistable folder)", res.WalkErrors)
	}
	if res.Skipped != 0 || res.TotalFiles != 1 {
		t.Errorf("skipped=%d total=%d — a walk loss is invisible to both, by construction", res.Skipped, res.TotalFiles)
	}
	if !res.Incomplete {
		t.Fatalf("a folder that could not be listed makes the result incomplete: %+v", res)
	}
	if len(res.Problems) == 0 {
		t.Error("the result must name the folder it could not list")
	}
}

// The tool's own sidecar folders hold its records, not anyone's files: they are passed
// over. Operating-system folders are listed as not read, without making the result
// incomplete: Windows makes them on nearly every drive it mounts.
func TestCardCheck_SidecarsSkippedSystemFoldersCounted(t *testing.T) {
	app := newSetupApp(t)
	card := t.TempDir()
	writeFile(t, card, dockSidecarDir+"/INVENTORY.md", "tool record")
	writeFile(t, card, "IMG_1.JPG", "frame")
	res := checkCard(t, app, card)
	if res.Incomplete || res.Excluded != 0 || res.TotalFiles != 1 {
		t.Fatalf("a sidecar folder must not count: %+v", res)
	}
	writeFile(t, card, "System Volume Information/x.dat", "system")
	writeFile(t, card, "$RECYCLE.BIN/S-1/y.dat", "recycled")
	res = checkCard(t, app, card)
	if res.Incomplete || res.Excluded != 0 || res.SystemFolders != 2 || len(res.SystemFolderItems) != 2 {
		t.Fatalf("system folders are listed, not counted as incomplete: %+v", res)
	}
	writeFile(t, card, ".Trashes/z.dat", "hidden")
	if res = checkCard(t, app, card); !res.Incomplete || res.Excluded != 1 {
		t.Fatalf("a dot-prefixed system folder is still a hidden item: %+v", res)
	}
}
