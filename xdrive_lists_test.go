//go:build !guionly

package main

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func listHashes(entries []CrossDriveEntry) []string {
	var out []string
	for _, e := range entries {
		var drives []string
		for _, p := range e.Places {
			drives = append(drives, p.Label+":"+p.Path)
		}
		sort.Strings(drives)
		out = append(out, strings.Join(drives, ","))
	}
	sort.Strings(out)
	return out
}

// The lists use each drive's latest complete scan: files only on one drive, and
// files on two or more drives (with every place they are). A drive with no
// complete scan is left out and said so.
func TestCrossDriveLists(t *testing.T) {
	app := dockApp(t)
	a, b, c, d := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	writeTree(t, a, map[string]string{"a.txt": "alpha\n", "b.txt": "bravo\n", "c.txt": "charlie\n", "c2.txt": "charlie\n"})
	writeTree(t, b, map[string]string{"x/a.txt": "alpha\n", "b.txt": "bravo\n"})
	writeTree(t, c, map[string]string{"a.txt": "alpha\n", "d.txt": "delta\n"})
	writeTree(t, d, map[string]string{"locked/e.txt": "echo\n"})
	scanDrive(t, app, a, "A")
	scanDrive(t, app, b, "B")
	scanDrive(t, app, c, "C")
	// C's newer scan is partial (it could not read a folder), so its complete
	// one is used.
	writeTree(t, c, map[string]string{"new/z.txt": "zulu\n"})
	makeUnlistable(t, mkdirFor(t, filepath.Join(c, "locked")))
	scanDrive(t, app, c, "C")
	makeUnlistable(t, filepath.Join(d, "locked"))
	scanDrive(t, app, d, "D")

	volA := app.Store.VolumeBySerial("A")
	l, err := app.CrossDriveLists(volA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := listHashes(l.OnlyOn); len(got) != 1 || got[0] != "A:c.txt,A:c2.txt" {
		t.Errorf("only on A: %v, want charlie at both its places", got)
	}
	want := []string{"A:a.txt,B:x/a.txt,C:a.txt", "A:b.txt,B:b.txt"}
	if got := listHashes(l.OnTwoOrMore); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("on 2+ drives: %v, want %v", got, want)
	}
	if len(l.Scans) != 3 || len(l.LeftOut) != 1 || l.LeftOut[0].Label != "D" {
		t.Errorf("scans used %+v, left out %+v; want A, B, C used and D left out", l.Scans, l.LeftOut)
	}
	for _, s := range l.Scans {
		if s.Label == "C" && s.ScanID == app.Store.ScanRecords(app.Store.VolumeBySerial("C").ID)[0].ID {
			t.Error("C's partial scan was used")
		}
	}
}
