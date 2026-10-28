//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A folder in managed territory that holds a registered source is that source's
// parent: quarantining it would move the source, so it is refused and nothing
// moves.
func TestQuarantine_NeverMovesASourceParent(t *testing.T) {
	app, destRoot, _ := quarantineScenario(t)
	src := filepath.Join(destRoot, "raw", "live")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)

	for _, p := range []string{filepath.Join(destRoot, "raw"), src, filepath.Join(src, "orig.nef")} {
		if app.Store.QuarantineEligible(p) {
			t.Errorf("%s is offered for quarantine although it is or holds a source", p)
		}
		if _, err := app.Quarantine(p, "", ""); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("quarantine %s: got %v, want a refusal naming the source", p, err)
		}
	}
	assertFileBytes(t, filepath.Join(src, "orig.nef"), "ORIG")
	assertFileBytes(t, filepath.Join(destRoot, "raw", "a.nef"), "AAA")
	if _, err := os.Stat(filepath.Join(destRoot, QuarantineDir)); !os.IsNotExist(err) {
		t.Errorf("something was moved into %s: %v", QuarantineDir, err)
	}
}

// A path that reaches a source through a folder link inside managed territory
// is decided by where it really is: inside the source, so it is refused, and a
// link itself is never quarantined.
func TestQuarantine_DecidesByRealLocation(t *testing.T) {
	app, destRoot, _ := quarantineScenario(t)
	src := t.TempDir()
	writeTree(t, src, map[string]string{"orig.nef": "ORIG", "sub/deep.nef": "DEEP"})
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	link := filepath.Join(destRoot, "raw", "shortcut")
	mustDirLink(t, link, src)

	for _, p := range []string{filepath.Join(link, "orig.nef"), filepath.Join(link, "sub")} {
		if app.Store.QuarantineEligible(p) {
			t.Errorf("%s is offered for quarantine although it is inside a source", p)
		}
		if _, err := app.Quarantine(p, "", ""); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("quarantine %s: got %v, want a refusal naming the source", p, err)
		}
	}
	if _, err := app.Quarantine(link, "", ""); err == nil {
		t.Error("a folder link was quarantined")
	}
	assertFileBytes(t, filepath.Join(src, "orig.nef"), "ORIG")
	assertFileBytes(t, filepath.Join(src, "sub", "deep.nef"), "DEEP")
}
