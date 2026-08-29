package main

// scan_problems_test.go — the scanner no longer silently drops files it cannot
// catalog. A directory with an unreadable (stat-failing) file and an unhashable
// (read-failing) file must yield two ScanProblems of the correct Kind, and the
// scanned count must reflect only the files actually recorded.
//
// Faults are injected at parallelHash's real stat/hash classification branches via
// the scanFaultHook seam (nil in production), so no OS-specific unreadable files are
// needed. This test sets that package-level seam, so it does not run in parallel.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFolder_CollectsProblemsAndExcludesFromCount(t *testing.T) {
	app, _ := newTestApp(t, map[string]string{}) // no external tools needed to scan
	coll := app.Store.AddCollection("ScanProblems")

	src := t.TempDir()
	for _, name := range []string{"good1.txt", "good2.txt", "unreadable.bin", "unhashable.bin"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Force a stat failure for the "unreadable" file and a hash failure for the
	// "unhashable" one — both at parallelHash's real report branches.
	scanFaultHook = func(p string) string {
		switch filepath.Base(p) {
		case "unreadable.bin":
			return "stat"
		case "unhashable.bin":
			return "hash"
		}
		return ""
	}
	defer func() { scanFaultHook = nil }()

	n, problems, err := app.ScanFolder(coll.ID, src, func(float64, string) {})
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}

	// Count reflects files actually recorded (the two good files), not the four enqueued.
	if n != 2 {
		t.Errorf("scanned count must exclude the two problem files, got %d (want 2)", n)
	}

	if len(problems) != 2 {
		t.Fatalf("want exactly 2 problems, got %d: %+v", len(problems), problems)
	}
	byKind := map[string]ScanProblem{}
	for _, pr := range problems {
		byKind[pr.Kind] = pr
	}
	if pr, ok := byKind["stat"]; !ok {
		t.Errorf("missing a 'stat' problem: %+v", problems)
	} else if filepath.Base(pr.Path) != "unreadable.bin" || pr.Err == "" {
		t.Errorf("stat problem should name unreadable.bin with a reason, got %+v", pr)
	}
	if pr, ok := byKind["hash"]; !ok {
		t.Errorf("missing a 'hash' problem: %+v", problems)
	} else if filepath.Base(pr.Path) != "unhashable.bin" || pr.Err == "" {
		t.Errorf("hash problem should name unhashable.bin with a reason, got %+v", pr)
	}
}
