package main

import (
	"sort"
	"strings"
	"testing"
)

func cf(path, hash string) ClassFile { return ClassFile{Path: path, Hash: hash, Size: 1} }

// statesOf renders results as "state:refpath>path" lines, sorted, for comparing.
func statesOf(rs []ClassResult) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.State+":"+r.RefPath+">"+r.Path)
	}
	sort.Strings(out)
	return out
}

func wantStates(t *testing.T, got []ClassResult, want ...string) {
	t.Helper()
	sort.Strings(want)
	if g := statesOf(got); strings.Join(g, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n  %s\nwant\n  %s", strings.Join(g, "\n  "), strings.Join(want, "\n  "))
	}
}

var exact = PathRules{}

func TestClassify_FiveStates(t *testing.T) {
	ref := ClassSide{Files: []ClassFile{cf("a", "1"), cf("b", "2"), cf("c", "3"), cf("d", "4")}}
	other := ClassSide{Files: []ClassFile{cf("a", "1"), cf("b", "9"), cf("x/c", "3"), cf("e", "5")}}
	r := Classify(ref, other, exact)
	wantStates(t, r.Results, "same:a>a", "changed:b>b", "moved:c>x/c", "missing:d>", "new:>e")
	if r.Counts[ClassSame] != 1 || r.Counts[ClassMissing] != 1 || r.Counts[ClassNew] != 1 {
		t.Errorf("counts %v", r.Counts)
	}
}

// One hash at many paths: each place is paired once, by the same name first, so
// a copy kept in place stays "same", a second copy that went away is "missing"
// (or "moved" when it shows up elsewhere), and extra copies are "new".
func TestClassify_OneHashAtManyPaths(t *testing.T) {
	ref := ClassSide{Files: []ClassFile{cf("p/a.jpg", "h"), cf("q/a.jpg", "h"), cf("r/b.jpg", "h")}}
	other := ClassSide{Files: []ClassFile{cf("p/a.jpg", "h"), cf("z/a.jpg", "h"), cf("z/b.jpg", "h"), cf("z/c.jpg", "h")}}
	r := Classify(ref, other, exact)
	wantStates(t, r.Results, "same:p/a.jpg>p/a.jpg", "moved:q/a.jpg>z/a.jpg", "moved:r/b.jpg>z/b.jpg", "new:>z/c.jpg")

	// Duplicates removed: two copies become one.
	r = Classify(ClassSide{Files: []ClassFile{cf("a", "h"), cf("b", "h")}}, ClassSide{Files: []ClassFile{cf("a", "h")}}, exact)
	wantStates(t, r.Results, "same:a>a", "missing:b>")
}

// A place a scan could not read is unknown: nothing there is missing, nothing is
// paired as moved out of it, and a file the reference could not have seen is not
// new.
func TestClassify_UnknownPlaces(t *testing.T) {
	unreadable := func(dir string) func(string) bool {
		return func(p string) bool { return !strings.HasPrefix(p, dir+"/") }
	}
	ref := ClassSide{Files: []ClassFile{cf("locked/a", "1"), cf("b", "2")}}
	other := ClassSide{Files: []ClassFile{cf("x/a", "1"), cf("b", "2")}, Known: unreadable("locked")}
	r := Classify(ref, other, exact)
	wantStates(t, r.Results, "unknown:locked/a>", "new:>x/a", "same:b>b")

	ref = ClassSide{Files: []ClassFile{cf("b", "2")}, Known: unreadable("hidden")}
	other = ClassSide{Files: []ClassFile{cf("b", "2"), cf("hidden/c", "3")}}
	r = Classify(ref, other, exact)
	wantStates(t, r.Results, "same:b>b", "unknown:>hidden/c")
}

// Case and Unicode: with folding, a name that differs only in letter case, or in
// how the same accented letter is encoded (NFC vs NFD), is the same name. Two
// names that fold together on one side are matched exactly instead.
func TestClassify_PathRules(t *testing.T) {
	nfc, nfd := "café.txt", "café.txt"
	fold := PathRules{FoldCase: true, Normalize: true}
	r := Classify(ClassSide{Files: []ClassFile{cf("Photos/A.JPG", "1"), cf(nfc, "2")}},
		ClassSide{Files: []ClassFile{cf("photos/a.jpg", "1"), cf(nfd, "2")}}, fold)
	wantStates(t, r.Results, "same:Photos/A.JPG>photos/a.jpg", "same:"+nfc+">"+nfd)

	r = Classify(ClassSide{Files: []ClassFile{cf("Photos/A.JPG", "1")}},
		ClassSide{Files: []ClassFile{cf("photos/a.jpg", "1")}}, exact)
	wantStates(t, r.Results, "moved:Photos/A.JPG>photos/a.jpg")

	// Linux-made names that differ only in case on both sides stay distinct.
	r = Classify(ClassSide{Files: []ClassFile{cf("a.txt", "1"), cf("A.txt", "2")}},
		ClassSide{Files: []ClassFile{cf("a.txt", "1"), cf("A.txt", "3")}}, fold)
	wantStates(t, r.Results, "same:a.txt>a.txt", "changed:A.txt>A.txt")
}

func TestHostPathRules(t *testing.T) {
	r := HostPathRules()
	if !r.Normalize {
		t.Error("names are always compared in one Unicode form")
	}
	if want := hostFoldsCase(); r.FoldCase != want {
		t.Errorf("FoldCase = %v, want %v on this platform", r.FoldCase, want)
	}
}
