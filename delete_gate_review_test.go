//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A removal that stops part-way (a file still held open) and is tried again
// carries on with the folder it renamed aside, so it finishes and leaves
// nothing behind; it never reports success while the folder is still there.
func TestScratch_RetryAfterAPartWayRemovalFinishes(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "obelisk-gpg-x")
	ownTree(t, base, "obelisk-gpg-x", "a", "b", "c/d")
	m, err := madeDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	scratchRemoveHook = func(string) error {
		if n++; n == 2 {
			return errors.New("held open")
		}
		return nil
	}
	err = m.Remove()
	scratchRemoveHook = nil
	if err == nil {
		t.Fatal("setup: the first removal did not stop")
	}
	if err := m.Remove(); err != nil {
		t.Fatalf("the retry: %v", err)
	}
	if es, _ := os.ReadDir(base); len(es) != 0 {
		t.Errorf("a retried removal left %v", es)
	}
}

// A job's result carries what a mirror kept, so the job screen can say how many
// files it kept, how much space they use, which were not Obelisk's own, and
// offer their removal.
func TestMirrorKeep_JobResultCarriesWhatWasKept(t *testing.T) {
	k := KeptSummary{KeptRun: `X:\_obelisk-previous\20261124T120000Z`, KeptRunName: "20261124T120000Z",
		KeptOwn: []string{"a.txt"}, KeptOther: []string{"b.txt"}, KeptBytes: 42}
	res := map[string]any{}
	k.addTo(res)
	if res["kept_run_name"] != "20261124T120000Z" || res["kept_bytes"] != int64(42) || len(res["kept_other"].([]string)) != 1 {
		t.Fatalf("job result: %v", res)
	}
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "KeptSummary.addTo(") < 2 {
		t.Error("the mirror and back-up-changes jobs do not both return what was kept")
	}
}

// Exports are ordered by when they were made, whatever their names' shape: a
// weekly automatic backup from November is newer than a manual one from March.
func TestExports_OrderedByWhenMade(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"obelisk-appbackup-20260301-101500.tar", "obelisk-appbackup-2026-W47.tar", "obelisk-appbackup-20261124.tar"} {
		mustWrite(t, filepath.Join(dir, n), "x")
	}
	list, _, err := ExportsIn(dir, "appbackup")
	if err != nil || len(list) != 3 {
		t.Fatalf("%v %v", list, err)
	}
	if list[0].Name != "obelisk-appbackup-20261124.tar" || list[1].Name != "obelisk-appbackup-2026-W47.tar" || !list[0].Newest || !list[2].Superseded {
		t.Errorf("order: %s, %s, %s", list[0].Name, list[1].Name, list[2].Name)
	}
	if got := exportTime("obelisk-appbackup-2026-W47.tar", time.Time{}); got.Weekday() != time.Monday || got.Month() != time.November {
		t.Errorf("week 47 of 2026 starts %v", got)
	}
}

// The Overwrite list says exactly what happens to each file: the ones the new
// copy writes are replaced, anything else stays where it is.
func TestWriteOver_ListSaysWhichFilesStay(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "WHICH-STAY")
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(medium, c.Name, c.Name+".tar.vol000+99.par2")
	mustWrite(t, stray, "an earlier build's par2 volume")
	p := prepareForTest(t, app, "write-over", map[string]any{"id": float64(c.ID), "dest_dir": medium})
	var stays, replaced bool
	for _, tg := range p.Targets {
		switch {
		case tg.Path == stray:
			stays = strings.HasPrefix(tg.Note, "left in place")
		case filepath.Base(tg.Path) == "RESTORE.txt":
			replaced = strings.HasPrefix(tg.Note, "replaced")
		}
	}
	if !stays || !replaced {
		t.Errorf("the Overwrite list does not say which files are replaced and which stay: %+v", p.Targets)
	}
}

// A bag export's name is <name>-bag, or <name>-bag-<time>; other folders that
// happen to contain "-bag" are not exports.
func TestExports_BagNames(t *testing.T) {
	k := exportKinds["bag"]
	for name, want := range map[string]bool{
		"my-bagels-bag": true, "my-bagels-bag-20261124-120000": true, "PKG-bag-20261124-120000-2": true,
		"travel-bags": false, "travel-bags-20260301": false, "bag": false, "notes-bag.txt": false,
	} {
		if got := matchesExport(k, name); got != want {
			t.Errorf("%q counted as a bag export: %v, want %v", name, got, want)
		}
	}
	if b, _ := bagBase("my-bagels-bag-20261124-120000"); b != "my-bagels" {
		t.Errorf("bag base %q", b)
	}
}

// A weekly backup made on Thursday ranks above a manual one made on Tuesday of
// the same week.
func TestExports_WeeklyBackupUsesItsOwnTime(t *testing.T) {
	dir := t.TempDir()
	weekly := filepath.Join(dir, "obelisk-appbackup-2026-W47.tar")
	mustWrite(t, weekly, "x")
	thu := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	if err := os.Chtimes(weekly, thu, thu); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "obelisk-appbackup-20261117-100000.tar"), "x")
	list, _, err := ExportsIn(dir, "appbackup")
	if err != nil || len(list) != 2 || list[0].Name != "obelisk-appbackup-2026-W47.tar" || !list[0].Made.Equal(thu) {
		t.Errorf("order or time: %+v %v", list, err)
	}
}
