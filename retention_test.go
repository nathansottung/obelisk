//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Scan lists: past the count, a drive keeps every scan made in the last day and
// its newest complete scan.
func TestRetention_ScansKeepTheLastDayAndTheNewestComplete(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := app.Store.AddVolume(Volume{Label: "HDD-R", Kind: "HDD"})
	old := time.Now().Add(-72 * time.Hour)
	complete, err := app.Store.AddScanRecord(&ScanRecord{VolumeID: vol.ID, At: old, Scan: ScanComplete, FileList: "a"}, 2)
	if err != nil || len(complete) != 0 {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ { // older partial scans, then three today
		if _, err := app.Store.AddScanRecord(&ScanRecord{VolumeID: vol.ID, At: old.Add(time.Duration(i+1) * time.Hour), Scan: ScanPartial, FileList: "p"}, 2); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := app.Store.AddScanRecord(&ScanRecord{VolumeID: vol.ID, At: time.Now().Add(-time.Duration(i) * time.Minute), Scan: ScanPartial, FileList: "t"}, 2); err != nil {
			t.Fatal(err)
		}
	}
	lists := app.Store.ScanFileListsInUse()
	if !lists["a"] {
		t.Error("the drive's only complete scan was dropped by the count")
	}
	if !lists["t"] {
		t.Error("scans made in the last day were dropped by the count")
	}
	if lists["p"] {
		t.Error("older partial scans past the count were kept")
	}
}

// Comparisons: the newest run and every run made in the last day are kept.
func TestRetention_ComparisonsKeepTheLastDay(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	for i := 0; i < 4; i++ {
		if _, err := app.Store.AddCompareRun(&CompareRun{At: time.Now().Add(-time.Duration(i) * time.Minute)}, 2); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(app.Store.CompareRuns()); n != 4 {
		t.Errorf("runs made in the last day were dropped: %d kept, want 4", n)
	}
	if _, err := app.Store.AddCompareRun(&CompareRun{At: time.Now().Add(-96 * time.Hour)}, 2); err != nil {
		t.Fatal(err)
	}
	if n := len(app.Store.CompareRuns()); n != 5 {
		t.Errorf("the newest run, just added, was dropped: %d kept", n)
	}
}

// File retention never removes the newest file of its kind, or one made in the
// last day, whatever the count asks.
func TestRetention_FilesKeepTheNewestAndTheLastDay(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "catalog.json"), "{}")
	names := []string{"catalog.json.bak-20260101", "catalog.json.bak-20260102", "catalog.json.bak-20260103", "catalog.json.bak-20260104"}
	for i, n := range names {
		p := filepath.Join(dir, n)
		mustWrite(t, p, n)
		at := time.Now().Add(-time.Duration(100-i) * time.Hour)
		if n == names[2] {
			at = time.Now().Add(-time.Hour) // made today, under an older name
		}
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	removeRetained(nil, dir, names, kindCatalogBackup, true)
	for i, n := range names {
		_, err := os.Stat(filepath.Join(dir, n))
		switch i {
		case 0, 1:
			if err == nil {
				t.Errorf("%s, old and not the newest, was kept", n)
			}
		case 2:
			if err != nil {
				t.Errorf("%s, made in the last day, was removed", n)
			}
		case 3:
			if err != nil {
				t.Errorf("%s, the newest, was removed", n)
			}
		}
	}
}

// The rule is stated where the user can read it: Settings and the handbook.
func TestRetention_RuleIsStatedInTheUIAndDocs(t *testing.T) {
	for _, p := range []string{"ui/index.html", "docs/handbook/10-where-your-data-lives.md"} {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, want := range []string{"the newest good copy is never removed", "nothing made in the last day is removed"} {
			if !strings.Contains(strings.ToLower(stripTags(s)), want) {
				t.Errorf("%s does not state %q", p, want)
			}
		}
	}
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// The helper's own spellings of catalog names match the constants they stand
// for (scratch.go uses literals so the GUI-only build has them too).
func TestScratch_CatalogNamesMatch(t *testing.T) {
	if catalogDBName != "catalog.db" || catalogMoveLock != "catalog.db.move-lock" {
		t.Fatalf("catalog names changed (%q, %q); update scratch.go's kinds", catalogDBName, catalogMoveLock)
	}
}
