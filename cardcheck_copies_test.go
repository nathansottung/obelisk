package main

// cardcheck_copies_test.go — what counts as a backup copy in Card Check, and how the
// report states it. A copy counts only when it is a package or adopted copy, is not
// superseded, last verified OK, and sits on a different physical device from the card.
// Card Check never gives a format verdict; it reports copies and dates.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Tests never probe real hardware: the card's device is unknown and nothing is
// connected unless a test says otherwise.
func init() {
	cardDeviceIdentity = func(string) (DeviceIdentity, error) { return DeviceIdentity{}, nil }
	cardOnlineVolumes = func(*App) map[int]bool { return nil }
}

// stubCard names the card's device serial and the volumes connected now.
func stubCard(t *testing.T, serial string, online ...int) {
	t.Helper()
	oldID, oldOn := cardDeviceIdentity, cardOnlineVolumes
	on := map[int]bool{}
	for _, id := range online {
		on[id] = true
	}
	cardDeviceIdentity = func(string) (DeviceIdentity, error) { return DeviceIdentity{Serial: serial}, nil }
	cardOnlineVolumes = func(*App) map[int]bool { return on }
	t.Cleanup(func() { cardDeviceIdentity, cardOnlineVolumes = oldID, oldOn })
}

// cardPackage records a package holding the given contents, with one copy per volume,
// each verified OK at the given time.
func cardPackage(t *testing.T, app *App, collID int, name string, contents []string, copies map[*Volume]time.Time) *Chunk {
	t.Helper()
	var refs []ChunkFileRef
	for i, c := range contents {
		refs = append(refs, ChunkFileRef{RelPath: fmt.Sprintf("f%d", i), SizeBytes: int64(len(c)), Hash: sha256Hex([]byte(c))})
	}
	ch := app.Store.AddChunk(Chunk{CollectionID: collID, Name: name, Status: "VERIFIED", Files: refs})
	for v, at := range copies {
		app.Store.RecordCopy(ch, v.ID, "X:/"+v.Label+"/"+name, true)
		setCopyVerified(t, app, ch, v.ID, at)
	}
	return ch
}

// setCopyVerified sets the last-verified time of ch's current copy on volume vid.
func setCopyVerified(t *testing.T, app *App, ch *Chunk, vid int, at time.Time) {
	t.Helper()
	for i := range ch.Copies {
		if ch.Copies[i].VolumeID == vid && !ch.Copies[i].Superseded {
			at := at.UTC()
			ch.Copies[i].LastVerifiedAt = &at
		}
	}
	if err := app.Store.UpdateChunkErr(ch); err != nil {
		t.Fatal(err)
	}
}

func checkCard(t *testing.T, app *App, card string) *CardCheckResult {
	t.Helper()
	res, err := app.CardCheck(card, func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func onlyFile(t *testing.T, list []CardFileResult) CardFileResult {
	t.Helper()
	if len(list) != 1 {
		t.Fatalf("want exactly one file, got %+v", list)
	}
	return list[0]
}

// No field, summary or per-file text ever carries a format, erase or reuse verdict.
var formatVerdict = regexp.MustCompile(`(?i)safe[ _-]?to|ok[ _-]?to[ _-]?format|\b(re)?format|\berase|\breuse|\bwipe`)

func assertNoVerdict(t *testing.T, res *CardCheckResult) {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if m := formatVerdict.FindString(string(b)); m != "" {
		t.Fatalf("the card check result carries a verdict (%q): %s", m, b)
	}
}

func TestCardCheck_ReportsCopiesAndTheirOldestDate(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Wedding", ArchiveSourced)
	a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
	b := app.Store.AddVolume(Volume{Label: "DRIVE-B", Kind: "HDD", Serial: "SER-B"})
	older := time.Date(2026, 11, 20, 18, 42, 7, 0, time.Local)
	newer := older.Add(48 * time.Hour)
	cardPackage(t, app, coll.ID, "PKG-1", []string{"frame-1", "frame-2"}, map[*Volume]time.Time{a: older, b: newer})

	card := t.TempDir()
	writeFile(t, card, "DCIM/IMG_1.JPG", "frame-1")
	writeFile(t, card, "DCIM/IMG_2.JPG", "frame-2")
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)

	if res.BackedUpFiles != 2 || res.NoCopyFiles != 0 || res.FewestCopies != 2 {
		t.Fatalf("backed=%d no_copy=%d fewest=%d, want 2/0/2", res.BackedUpFiles, res.NoCopyFiles, res.FewestCopies)
	}
	if res.AsOf == nil || !res.AsOf.Equal(older) {
		t.Fatalf("as_of = %v, want the stalest proof %v", res.AsOf, older)
	}
	want := "2 of 2 files backed up as of 11/20/2026 18:42:07. Fewest copies: 2."
	if res.Summary != want {
		t.Fatalf("summary = %q, want %q", res.Summary, want)
	}
	f := res.BackedUp[0]
	if len(f.Copies) != 2 || f.Copies[0].Where != "DRIVE-A" || f.Copies[1].Where != "DRIVE-B" {
		t.Fatalf("each copy must say where it lives: %+v", f.Copies)
	}
	if !f.Copies[0].LastVerifiedAt.Equal(older) || !f.Copies[1].LastVerifiedAt.Equal(newer) {
		t.Fatalf("each copy must say when it was last verified: %+v", f.Copies)
	}
}

func TestCardCheck_NoCopyIsSaidFirst(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Trip", ArchiveSourced)
	a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)
	cardPackage(t, app, coll.ID, "PKG-1", []string{"kept"}, map[*Volume]time.Time{a: at})

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "kept")
	writeFile(t, card, "IMG_2.JPG", "never-copied")
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	want := "1 of 2 files have no backup copy. 1 of 2 files backed up as of 01/02/2026 03:04:05. Fewest copies: 1."
	if res.Summary != want {
		t.Fatalf("summary = %q, want %q", res.Summary, want)
	}
	if f := onlyFile(t, res.NoCopy); f.Rel != "IMG_2.JPG" || len(f.Copies) != 0 {
		t.Fatalf("no-copy list = %+v", res.NoCopy)
	}
}

// a. The card scanned as its own source is a record, not a backup.
func TestCardCheck_CardScannedAsSourceIsNotABackup(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Card", ArchiveSourced)
	card := t.TempDir()
	writeFile(t, card, "DCIM/IMG_0001.JPG", "only-on-the-card-1")
	writeFile(t, card, "DCIM/IMG_0002.JPG", "only-on-the-card-2")
	scanInto(t, app, coll.ID, card)

	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.BackedUpFiles != 0 || res.NoCopyFiles != 2 || res.RecordedOnly != 2 {
		t.Fatalf("a card scanned as its own source has no backup copy: %+v", res)
	}
	for _, f := range res.NoCopy {
		if f.RecordedIn != "Archive: Card" || f.RecordedAt == nil {
			t.Fatalf("a recorded file must say where and when it was recorded: %+v", f)
		}
	}
	if !strings.HasPrefix(res.Summary, "2 of 2 files have no backup copy.") {
		t.Fatalf("summary = %q", res.Summary)
	}
}

// b. A deleted source leaves the card as the only copy.
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
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.BackedUpFiles != 0 || res.NoCopyFiles != 1 {
		t.Fatalf("the card holds the only copy: %+v", res)
	}
}

// c. Hidden files and folders are counted, listed, and make the result incomplete.
func TestCardCheck_HiddenItemsAreNotDropped(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
	cardPackage(t, app, coll.ID, "PKG-1", []string{"known-photo"}, map[*Volume]time.Time{a: time.Now()})

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "known-photo")
	writeFile(t, card, ".hidden-frame.jpg", "never-seen-1")
	writeFile(t, card, ".private/IMG_9.JPG", "never-seen-2")
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.BackedUpFiles != 1 || res.TotalFiles != 1 {
		t.Fatalf("the known photo is still reported: %+v", res)
	}
	if !res.Incomplete || res.Excluded != 2 || len(res.ExcludedItems) != 2 {
		t.Fatalf("hidden items must be counted and make the result incomplete: incomplete=%v excluded=%d items=%+v",
			res.Incomplete, res.Excluded, res.ExcludedItems)
	}
	if !strings.Contains(res.Summary, "Incomplete: 2 hidden or system item(s) were not checked.") {
		t.Fatalf("the summary must say the result is incomplete: %q", res.Summary)
	}
}

// A copy on the card itself counts as zero, however the card is recognised.
func TestCardCheck_CopyOnTheCardCountsZero(t *testing.T) {
	for _, tc := range []struct {
		name   string
		serial string // the card's serial
		vol    Volume
		path   func(card string) string
	}{
		{"same serial", "SER-CARD", Volume{Label: "CARD", Serial: "SER-CARD"}, func(string) string { return "Q:/PKG-1" }},
		{"same disk, other partition", "SER-CARD", Volume{Label: "CARD-P2", Serial: "ser-card", PartitionID: "p2"}, func(string) string { return "Q:/PKG-1" }},
		{"recorded under the card's mount", "", Volume{Label: "UNKNOWN"}, func(card string) string { return filepath.Join(card, "PKG-1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newSetupApp(t)
			stubCard(t, tc.serial)
			coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
			v := app.Store.AddVolume(tc.vol)
			card := t.TempDir()
			ch := app.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-1", Status: "VERIFIED",
				Files: []ChunkFileRef{{RelPath: "f", Hash: sha256Hex([]byte("on-card"))}}})
			app.Store.RecordCopy(ch, v.ID, tc.path(card), true)
			writeFile(t, card, "IMG_1.JPG", "on-card")

			res := checkCard(t, app, card)
			if res.BackedUpFiles != 0 {
				t.Fatalf("a copy on the card itself must count as zero: %+v", res)
			}
			if f := onlyFile(t, res.NoCopy); len(f.NotCounted) == 0 {
				t.Fatalf("the uncounted copy must say why: %+v", f)
			}
		})
	}
}

// A copy whose drive cannot be told apart from the card's does not count.
func TestCardCheck_UnknownDeviceCountsZero(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "") // the card's serial could not be read
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
	cardPackage(t, app, coll.ID, "PKG-1", []string{"frame"}, map[*Volume]time.Time{a: time.Now()})

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "frame")
	res := checkCard(t, app, card)
	if res.BackedUpFiles != 0 {
		t.Fatalf("an unprovable device must not count: %+v", res)
	}
	f := onlyFile(t, res.NoCopy)
	if len(f.NotCounted) != 1 || !strings.Contains(f.NotCounted[0], "could not tell") {
		t.Fatalf("not_counted = %v", f.NotCounted)
	}
}

// A failed, missing, superseded or later-failed copy counts as zero.
func TestCardCheck_FailedOrMissingCopyCountsZero(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(ch *Chunk)
		why    string
	}{
		{"last verification failed", func(ch *Chunk) { f := false; ch.Copies[0].VerifyOK = &f }, "last verification failed"},
		{"never verified", func(ch *Chunk) { ch.Copies[0].VerifyOK = nil }, "never verified"},
		{"later check found it missing", func(ch *Chunk) {
			f, later := false, ch.Copies[0].LastVerifiedAt.Add(time.Hour)
			ch.Copies[0].LastCheckOK, ch.Copies[0].LastCheckAt, ch.Copies[0].LastCheckLevel = &f, &later, "A"
		}, "a later check failed"},
		{"drive record gone", func(ch *Chunk) { ch.Copies[0].VolumeID = 9999 }, "no longer registered"},
		{"superseded", func(ch *Chunk) { ch.Copies[0].Superseded = true }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newSetupApp(t)
			stubCard(t, "CARD-SERIAL")
			coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
			a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
			ch := cardPackage(t, app, coll.ID, "PKG-1", []string{"frame"}, map[*Volume]time.Time{a: time.Now()})
			tc.mutate(ch)
			if err := app.Store.UpdateChunkErr(ch); err != nil {
				t.Fatal(err)
			}
			card := t.TempDir()
			writeFile(t, card, "IMG_1.JPG", "frame")

			res := checkCard(t, app, card)
			if res.BackedUpFiles != 0 || res.NoCopyFiles != 1 {
				t.Fatalf("this copy must count as zero: %+v", res)
			}
			f := onlyFile(t, res.NoCopy)
			if tc.why != "" && (len(f.NotCounted) != 1 || !strings.Contains(f.NotCounted[0], tc.why)) {
				t.Fatalf("not_counted = %v, want %q", f.NotCounted, tc.why)
			}
		})
	}
}

// An offline copy keeps its last verified date and is marked as not rechecked; a
// connected one is not marked.
func TestCardCheck_OfflineCopyKeepsItsDate(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	a := app.Store.AddVolume(Volume{Label: "DRIVE-A", Kind: "HDD", Serial: "SER-A"})
	b := app.Store.AddVolume(Volume{Label: "DRIVE-B", Kind: "HDD", Serial: "SER-B"})
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.Local)
	cardPackage(t, app, coll.ID, "PKG-1", []string{"frame"}, map[*Volume]time.Time{a: at, b: at})
	stubCard(t, "CARD-SERIAL", b.ID) // only DRIVE-B is connected

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "frame")
	res := checkCard(t, app, card)
	f := onlyFile(t, res.BackedUp)
	if len(f.Copies) != 2 {
		t.Fatalf("an offline copy still counts: %+v", f.Copies)
	}
	if !f.Copies[0].Offline || !f.Copies[0].LastVerifiedAt.Equal(at) {
		t.Fatalf("DRIVE-A is offline and keeps its date: %+v", f.Copies[0])
	}
	if f.Copies[1].Offline {
		t.Fatalf("DRIVE-B is connected: %+v", f.Copies[1])
	}
}

// Two partitions of one disk are one device, and so one copy.
func TestCardCheck_TwoPartitionsOneDiskCountOnce(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Shoot", ArchiveSourced)
	p1 := app.Store.AddVolume(Volume{Label: "DISK-P1", Kind: "HDD", Serial: "SER-DISK", PartitionID: "p1"})
	p2 := app.Store.AddVolume(Volume{Label: "DISK-P2", Kind: "HDD", Serial: "SER-DISK", PartitionID: "p2"})
	now := time.Now()
	cardPackage(t, app, coll.ID, "PKG-1", []string{"frame"}, map[*Volume]time.Time{p1: now, p2: now})

	card := t.TempDir()
	writeFile(t, card, "IMG_1.JPG", "frame")
	res := checkCard(t, app, card)
	if res.FewestCopies != 1 || len(onlyFile(t, res.BackedUp).Copies) != 1 {
		t.Fatalf("two partitions of one disk must count once: %+v", res.BackedUp)
	}
}

// Content known only from a drive snapshot, or as an earlier version, is recorded,
// with no backup copy.
func TestCardCheck_SnapshotOrEarlierVersionIsNoBackup(t *testing.T) {
	app := newSetupApp(t)
	coll := app.Store.AddCollectionKind("Archive", ArchiveSourced)
	vol := app.Store.AddVolume(Volume{Label: "OLD-DRIVE-7", Kind: "HDD"})
	captured := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	app.Store.PutVolumeSnapshot(&VolumeSnapshot{VolumeID: vol.ID, Label: "OLD-DRIVE-7", CapturedAt: captured, TotalFiles: 1,
		Files: []SnapFile{{RelPath: "2019/x.nef", Hash: sha256Hex([]byte("on-old-drive")), SizeBytes: 12}}})

	src := t.TempDir()
	writeFile(t, src, "edit.jpg", "first-version")
	scanInto(t, app, coll.ID, src)
	writeFile(t, src, "edit.jpg", "second-version")
	scanInto(t, app, coll.ID, src)

	card := t.TempDir()
	writeFile(t, card, "x.nef", "on-old-drive")
	writeFile(t, card, "edit.jpg", "first-version")
	res := checkCard(t, app, card)
	assertNoVerdict(t, res)
	if res.BackedUpFiles != 0 || res.NoCopyFiles != 2 || res.RecordedOnly != 2 {
		t.Fatalf("records alone are not backups: %+v", res)
	}
	byRel := map[string]CardFileResult{}
	for _, f := range res.NoCopy {
		byRel[f.Rel] = f
	}
	if f := byRel["x.nef"]; f.RecordedIn != "Drive: OLD-DRIVE-7" || f.RecordedAt == nil || !f.RecordedAt.Equal(captured) {
		t.Fatalf("snapshot record = %+v", f)
	}
	if f := byRel["edit.jpg"]; f.RecordedIn != "Archive: Archive (earlier version)" {
		t.Fatalf("earlier-version record = %+v", f)
	}
}

// A spanned package counts once, only when every segment is verified off the card.
func TestCardCheck_SpannedPackage(t *testing.T) {
	app := newSetupApp(t)
	stubCard(t, "CARD-SERIAL")
	coll := app.Store.AddCollectionKind("Big", ArchiveSourced)
	t1 := app.Store.AddVolume(Volume{Label: "TAPE-1", Kind: "TAPE", Serial: "T1"})
	t2 := app.Store.AddVolume(Volume{Label: "TAPE-2", Kind: "TAPE", Serial: "T2"})
	at := time.Date(2026, 5, 6, 7, 8, 9, 0, time.Local)
	ch := app.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "SPAN-1", Status: "VERIFIED", Spanned: true, VerifiedAt: &at,
		Files: []ChunkFileRef{{RelPath: "f", Hash: sha256Hex([]byte("big-frame"))}}})
	if err := app.Store.SetChunkSegments(ch, []Segment{{Index: 1, Status: "VERIFIED", VolumeID: t1.ID}, {Index: 2, Status: "WRITTEN", VolumeID: t2.ID}}); err != nil {
		t.Fatal(err)
	}
	card := t.TempDir()
	writeFile(t, card, "IMG.JPG", "big-frame")
	if res := checkCard(t, app, card); res.BackedUpFiles != 0 {
		t.Fatalf("a span with an unverified segment must not count: %+v", res)
	}
	ch.Segments[1].Status = "VERIFIED"
	if err := app.Store.SetChunkSegments(ch, ch.Segments); err != nil {
		t.Fatal(err)
	}
	res := checkCard(t, app, card)
	f := onlyFile(t, res.BackedUp)
	if len(f.Copies) != 1 || f.Copies[0].Where != "TAPE-1 + TAPE-2" || !f.Copies[0].LastVerifiedAt.Equal(at) {
		t.Fatalf("a verified span is one copy: %+v", f.Copies)
	}
}

func TestCardCheck_EmptyCardSaysSo(t *testing.T) {
	app := newSetupApp(t)
	res := checkCard(t, app, t.TempDir())
	assertNoVerdict(t, res)
	if res.Summary != "No files found." || res.Incomplete {
		t.Fatalf("summary = %q incomplete=%v", res.Summary, res.Incomplete)
	}
}
