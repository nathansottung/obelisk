//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A short 8.3 name names the same folder as its long name: quarantine matches by
// identity, so neither a source's parent nor the source itself can be moved
// through one.
func TestQuarantine_ShortNamesAreTheSameFolder(t *testing.T) {
	app, destRoot, _ := quarantineScenario(t)
	parent := filepath.Join(destRoot, "rawfolderlongname")
	src := filepath.Join(parent, "livesourcefolder")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	shortParent := shortPathName(t, parent)
	shortSrc := shortPathName(t, src)
	if shortParent == "" || shortSrc == "" || strings.EqualFold(shortParent, parent) {
		t.Skip("SKIP (NOT TESTED): 8.3 short names are not generated on this volume")
	}

	for _, p := range []string{shortParent, shortSrc, filepath.Join(shortSrc, "orig.nef")} {
		if app.Store.QuarantineEligible(p) {
			t.Errorf("%s is offered for quarantine although it is or holds a source", p)
		}
		if _, err := app.Quarantine(p, "", ""); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("quarantine %s: got %v, want a refusal naming the source", p, err)
		}
	}
	assertFileBytes(t, filepath.Join(src, "orig.nef"), "ORIG")
	if _, err := os.Stat(filepath.Join(destRoot, QuarantineDir)); !os.IsNotExist(err) {
		t.Errorf("something was moved into %s: %v", QuarantineDir, err)
	}
}

// A file quarantined through a short name is recorded, staged and restored under
// its real name.
func TestQuarantine_RecordsTheRealName(t *testing.T) {
	app, destRoot, _ := quarantineScenario(t)
	dir := filepath.Join(destRoot, "rawfolderlongname")
	writeTree(t, dir, map[string]string{"keep.nef": "KEEP"})
	short := shortPathName(t, dir)
	if short == "" || strings.EqualFold(short, dir) {
		t.Skip("SKIP (NOT TESTED): 8.3 short names are not generated on this volume")
	}
	e, err := app.Quarantine(short, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if e.OriginalRel != "rawfolderlongname" {
		t.Errorf("recorded as %q, want the real name rawfolderlongname", e.OriginalRel)
	}
	assertFileBytes(t, filepath.Join(destRoot, QuarantineDir, "rawfolderlongname", "keep.nef"), "KEEP")
	if _, err := app.UnQuarantine(e.ID); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, filepath.Join(dir, "keep.nef"), "KEEP")
}
