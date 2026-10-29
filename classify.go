package main

// classify.go — the one comparison of two sets of files (a reference and another
// set) that drift and drive comparisons share. Each file ends up in one state:
//
//	same     — at the same path (by the path rules) with the same content
//	changed  — at the same path with different content
//	moved    — gone from its reference path, and the same content is at a path
//	           the reference did not have
//	new      — in the other set only
//	missing  — in the reference only, at a place the other set did read
//	unknown  — at a place one side could not read, so nothing can be said
//
// One content hash may sit at many paths on either side. Each place is paired at
// most once: by path first, then moves by the same file name, then in path order.
// A file under a place the other side could not read is never missing, new or
// moved: it is unknown.

import (
	"path"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Classification states.
const (
	ClassSame    = "same"
	ClassChanged = "changed"
	ClassMoved   = "moved"
	ClassNew     = "new"
	ClassMissing = "missing"
	ClassUnknown = "unknown"
)

// ClassStates lists the states in display order.
var ClassStates = []string{ClassSame, ClassChanged, ClassMoved, ClassNew, ClassMissing, ClassUnknown}

// ClassFile is one file of a set: its path (slash-separated), content hash and
// size.
type ClassFile struct {
	Path string
	Hash string
	Size int64
}

// ClassSide is one set of files. Known reports whether the set's scan read the
// place a path would be; nil means it read everything.
type ClassSide struct {
	Files []ClassFile
	Known func(path string) bool
}

func (s ClassSide) known(p string) bool { return s.Known == nil || s.Known(p) }

// ClassResult is one file's state. RefPath is its path in the reference (empty
// for new), Path its path in the other set (empty for missing).
type ClassResult struct {
	State   string `json:"state"`
	RefPath string `json:"ref_path,omitempty"`
	Path    string `json:"path,omitempty"`
	RefHash string `json:"ref_hash,omitempty"`
	Hash    string `json:"hash,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

// Classification is the result of Classify.
type Classification struct {
	Results []ClassResult
	Counts  map[string]int
}

// PathRules say when two paths name the same place. FoldCase ignores letter
// case; Normalize compares names in one Unicode form (NFC), so an accented letter
// written as one code point or as a letter plus accent is the same name.
type PathRules struct {
	FoldCase  bool
	Normalize bool
}

// hostFoldsCase reports whether this platform's usual filesystems ignore case
// (NTFS, exFAT and FAT on Windows; APFS and HFS+ on macOS).
func hostFoldsCase() bool { return runtime.GOOS == "windows" || runtime.GOOS == "darwin" }

// HostPathRules are the rules for this platform. Names are always compared in
// one Unicode form, since the same name can be stored either way on a drive
// (macOS writes NFD); where two names on one side differ only by the rules,
// they are matched exactly instead (see Classify).
func HostPathRules() PathRules { return PathRules{FoldCase: hostFoldsCase(), Normalize: true} }

func (r PathRules) key(p string) string {
	if r.Normalize {
		p = norm.NFC.String(p)
	}
	if r.FoldCase {
		p = strings.ToLower(p)
	}
	return p
}

// Classify compares other against ref.
func Classify(ref, other ClassSide, rules PathRules) Classification {
	var out []ClassResult
	add := func(r ClassResult) { out = append(out, r) }

	byKey := func(files []ClassFile) map[string][]int {
		m := map[string][]int{}
		for i, f := range files {
			k := rules.key(f.Path)
			m[k] = append(m[k], i)
		}
		return m
	}
	refKeys, otherKeys := byKey(ref.Files), byKey(other.Files)
	refPaired := make([]bool, len(ref.Files))
	otherPaired := make([]bool, len(other.Files))
	pair := func(i, j int) {
		refPaired[i], otherPaired[j] = true, true
		rf, of := ref.Files[i], other.Files[j]
		st := ClassSame
		if rf.Hash != of.Hash {
			st = ClassChanged
		}
		add(ClassResult{State: st, RefPath: rf.Path, Path: of.Path, RefHash: rf.Hash, Hash: of.Hash, Size: of.Size})
	}

	// 1. By path. Where a name is unique on both sides under the rules, it pairs;
	// where the rules fold several names together, only exact names pair.
	for k, ri := range refKeys {
		oi := otherKeys[k]
		if len(oi) == 0 {
			continue
		}
		if len(ri) == 1 && len(oi) == 1 {
			pair(ri[0], oi[0])
			continue
		}
		for _, i := range ri {
			for _, j := range oi {
				if !otherPaired[j] && ref.Files[i].Path == other.Files[j].Path {
					pair(i, j)
					break
				}
			}
		}
	}

	// 2. What is left: unknown where the other side could not see it, otherwise a
	// candidate to be moved, missing or new.
	refLeft, otherLeft := map[string][]int{}, map[string][]int{}
	for i, f := range ref.Files {
		switch {
		case refPaired[i]:
		case !other.known(f.Path):
			add(ClassResult{State: ClassUnknown, RefPath: f.Path, RefHash: f.Hash, Size: f.Size})
		default:
			refLeft[f.Hash] = append(refLeft[f.Hash], i)
		}
	}
	for j, f := range other.Files {
		switch {
		case otherPaired[j]:
		case !ref.known(f.Path):
			add(ClassResult{State: ClassUnknown, Path: f.Path, Hash: f.Hash, Size: f.Size})
		default:
			otherLeft[f.Hash] = append(otherLeft[f.Hash], j)
		}
	}

	// 3. Moves: the same content gone from one place and at another. Same file
	// name first, then in path order.
	byPath := func(files []ClassFile, idx []int) {
		sort.Slice(idx, func(a, b int) bool { return files[idx[a]].Path < files[idx[b]].Path })
	}
	for h, ri := range refLeft {
		oi := otherLeft[h]
		if h == "" || len(oi) == 0 {
			continue
		}
		byPath(ref.Files, ri)
		byPath(other.Files, oi)
		used := make([]bool, len(oi))
		move := func(i, j int) {
			refPaired[i], otherPaired[j] = true, true
			rf, of := ref.Files[i], other.Files[j]
			add(ClassResult{State: ClassMoved, RefPath: rf.Path, Path: of.Path, RefHash: rf.Hash, Hash: of.Hash, Size: of.Size})
		}
		var rest []int
		for _, i := range ri {
			base := rules.key(path.Base(ref.Files[i].Path))
			found := false
			for n, j := range oi {
				if !used[n] && rules.key(path.Base(other.Files[j].Path)) == base {
					used[n], found = true, true
					move(i, j)
					break
				}
			}
			if !found {
				rest = append(rest, i)
			}
		}
		n := 0
		for _, i := range rest {
			for n < len(oi) && used[n] {
				n++
			}
			if n == len(oi) {
				break
			}
			used[n] = true
			move(i, oi[n])
		}
	}

	// 4. Whatever is still unpaired is missing (reference) or new (other).
	for _, ri := range refLeft {
		for _, i := range ri {
			if !refPaired[i] {
				f := ref.Files[i]
				add(ClassResult{State: ClassMissing, RefPath: f.Path, RefHash: f.Hash, Size: f.Size})
			}
		}
	}
	for _, oi := range otherLeft {
		for _, j := range oi {
			if !otherPaired[j] {
				f := other.Files[j]
				add(ClassResult{State: ClassNew, Path: f.Path, Hash: f.Hash, Size: f.Size})
			}
		}
	}

	sort.Slice(out, func(a, b int) bool {
		ka, kb := out[a].RefPath+"\x00"+out[a].Path, out[b].RefPath+"\x00"+out[b].Path
		if out[a].RefPath == "" {
			ka = out[a].Path
		}
		if out[b].RefPath == "" {
			kb = out[b].Path
		}
		return ka < kb
	})
	counts := map[string]int{}
	for _, r := range out {
		counts[r.State]++
	}
	return Classification{Results: out, Counts: counts}
}
