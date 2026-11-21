package main

// cardcheck_firstfail_test.go — three cases in which Card Check must not report a
// card as backed up. Each is written against the current result shape so it can
// run, and fail, before the fix.

import (
	"os"
	"path/filepath"
	"testing"
)

// a. The card itself was scanned as a source; no package or backup drive exists.
func TestCardCheck_CardScannedAsSourceIsNotABackup(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Card", ArchiveSourced)
	card := t.TempDir()
	writeFile(t, card, "DCIM/IMG_0001.JPG", "only-on-the-card-1")
	writeFile(t, card, "DCIM/IMG_0002.JPG", "only-on-the-card-2")
	scanInto(t, app, coll.ID, card)

	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.SafeToFormat || res.BackedUpFiles != 0 {
		t.Fatalf("a card scanned as its own source has no backup copy: safe_to_format=%v backed_up=%d",
			res.SafeToFormat, res.BackedUpFiles)
	}
}

// b. A source file was scanned and then deleted; its only surviving copy is the card.
func TestCardCheck_DeletedSourceLeavesOnlyTheCard(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Trip", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "the-last-copy")
	scanInto(t, app, coll.ID, src)
	if err := os.Remove(filepath.Join(src, "a.jpg")); err != nil {
		t.Fatal(err)
	}

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "the-last-copy")
	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.SafeToFormat || res.BackedUpFiles != 0 {
		t.Fatalf("the card holds the only copy: safe_to_format=%v backed_up=%d", res.SafeToFormat, res.BackedUpFiles)
	}
}

// c. One known photo plus unknown dot-prefixed files and folders. The hidden items
// were never read, so the result cannot be a complete one.
func TestCardCheck_HiddenItemsAreNotDropped(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	src := t.TempDir()
	writeFile(t, src, "a.jpg", "known-photo")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "known-photo")
	writeFile(t, card, ".hidden-frame.jpg", "never-seen-1")
	writeFile(t, card, ".private/IMG_9.JPG", "never-seen-2")
	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.SafeToFormat {
		t.Fatalf("hidden files and folders were dropped without marking the result incomplete: %+v", res)
	}
}
