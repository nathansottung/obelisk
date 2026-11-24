package main

// scratch.go — the one place Obelisk removes files it made itself.
//
// The rule: nothing the user owns is removed or overwritten unless it passes the
// delete gate (destructive.go) and the user types "Delete" or "Overwrite". The one
// exception lives here. Obelisk may remove a file or folder it made itself:
//
//   - something this same operation just created exclusively (madeFile, madeDir),
//     matched by the identity of the object it created, wherever it is; or
//   - a kind of file Obelisk makes (scratchKind), inside one of its own folders,
//     the folder matched by its opened identity and never under a source root
//     (removeOwned). Retention (catalog backups, scan lists, comparison results)
//     goes through here too, under the rule in retention.go.
//
// It never follows a link: a link at the name is refused, and a link inside a
// folder being removed is removed as a link, never entered. A folder is renamed
// out of the way (removingPrefix) before its contents go, so a cleanup that stops
// part-way leaves no half-removed folder under a name the app uses; what is left
// is listed to the user as a leftover (listLeftoversIn).
//
// Leftovers from an earlier or crashed run are never removed here: they are
// listed with their size and age and removed only through the gate.
// store_access_test.go's removal rule fails any other file that calls a removal
// function directly.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// removingPrefix names a folder whose removal has begun.
const removingPrefix = ".obelisk-removing-"

// scratchRemoveHook, when set (tests only), runs before each entry of a folder
// is removed; an error stops the removal there, as a crash would.
var scratchRemoveHook func(rel string) error

// scratchKind is a kind of file or folder Obelisk makes, told by its name.
type scratchKind struct {
	label string
	match func(name string) bool
	tree  bool // a folder with contents; otherwise a single file
	// leftover marks the kinds that, found from an earlier run, are listed to
	// the user. Retained records (backups, scan lists) are not leftovers.
	leftover bool
}

func hasInfix(name, prefix, infix string) bool {
	return strings.HasPrefix(name, prefix) && strings.Contains(name[len(prefix):], infix)
}

var (
	// kindStage is a staging file from createStage (".<name>.<random>.tmp") or a
	// like-named one Obelisk makes beside a file it is about to write.
	kindStage = scratchKind{label: "unfinished save", leftover: true, match: func(n string) bool {
		return len(n) > len(".x.tmp") && strings.HasPrefix(n, ".") && strings.HasSuffix(n, ".tmp")
	}}
	// kindBuild is a package build in progress, in staging.
	kindBuild = scratchKind{label: "unfinished package build", tree: true, leftover: true, match: func(n string) bool {
		return hasInfix(n, ".", ".build-")
	}}
	// kindPrevious is a staged build a newer one has replaced.
	kindPrevious = scratchKind{label: "replaced package build", tree: true, leftover: true, match: func(n string) bool {
		return hasInfix(n, ".", ".previous-")
	}}
	// kindRefill is a staging refill in progress.
	kindRefill = scratchKind{label: "unfinished staging refill", tree: true, leftover: true, match: func(n string) bool {
		return hasInfix(n, ".", ".refill-")
	}}
	// kindRemoving is a folder whose removal began and did not finish.
	kindRemoving = scratchKind{label: "unfinished cleanup", tree: true, leftover: true, match: func(n string) bool {
		return strings.HasPrefix(n, removingPrefix)
	}}
	// kindCatalogTemp is a catalog snapshot, stage or move file beside the catalog
	// (".creating-", ".migrating-", ".export-", ".restore-", ".stage-", "restore-stage-").
	kindCatalogTemp = scratchKind{label: "unfinished catalog step", leftover: true, match: func(n string) bool {
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(n, "-wal"), "-shm"), "-journal")
		for _, p := range []string{"catalog.db.creating-", "catalog.db.migrating-", "catalog.db.restore-"} {
			if strings.HasPrefix(base, p) {
				return true
			}
		}
		return strings.HasPrefix(base, "catalog.") && (strings.Contains(base, ".export-") || strings.Contains(base, ".stage-") || strings.HasSuffix(base, ".stage"))
	}}
	// kindCatalogWork is a work folder made beside the catalog for one restore.
	kindCatalogWork = scratchKind{label: "unfinished catalog restore", tree: true, leftover: true, match: func(n string) bool {
		return strings.HasPrefix(n, "restore-stage-")
	}}
	// kindLock is a lock file Obelisk holds while it moves its catalog.
	kindLock = scratchKind{label: "lock file", match: func(n string) bool { return strings.HasSuffix(n, ".lock") }}
	// kindCopyID is the COPY-ID.txt a burn puts in a staged package for that burn.
	kindCopyID = scratchKind{label: "burn copy ID", match: func(n string) bool { return n == copyIDFile }}
	// kindIntermediateTar is a build's plain tar, removed once it is encrypted.
	kindIntermediateTar = scratchKind{label: "intermediate tar", match: func(n string) bool { return strings.HasSuffix(n, ".tar") }}
	// kindTempHome is a private folder made under the system temp folder.
	kindTempHome = scratchKind{label: "private temporary folder", tree: true, match: func(n string) bool {
		return strings.HasPrefix(n, "obelisk-")
	}}
	// kindSignIn is a sign-in forwarding page or address file in the private
	// sign-in folder.
	kindSignIn = scratchKind{label: "sign-in page", match: func(n string) bool {
		return (strings.HasPrefix(n, "open-") && strings.HasSuffix(n, ".html")) || (strings.HasPrefix(n, "addr-") && strings.HasSuffix(n, ".json"))
	}}
	// Retained records: removed only by retention.go's rule.
	kindCatalogBackup = scratchKind{label: "catalog backup", match: func(n string) bool {
		return strings.HasPrefix(n, "catalog.db.bak-") || strings.HasPrefix(n, "catalog.json.bak-")
	}}
	kindScanList    = scratchKind{label: "scan file list", match: func(n string) bool { return validScanFileName(n) }}
	kindCompareFile = scratchKind{label: "comparison result", match: func(n string) bool { return validCompareFileName(n) }}
	// kindHeldSettings is the held-settings record, removed when the user applies
	// or discards it.
	kindHeldSettings = scratchKind{label: "held settings", match: func(n string) bool { return n == "held-settings.json" }}
)

// leftoverKinds are the kinds listed to the user when found from an earlier run.
var leftoverKinds = []scratchKind{kindRemoving, kindBuild, kindPrevious, kindRefill, kindStage, kindCatalogTemp, kindCatalogWork}

// validEntryName is a single name in a folder: no separators, not "." or "..".
func validEntryName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`) && filepath.Base(name) == name
}

// ownFolderIdentity is the identity of the folder at path, from an open handle.
func ownFolderIdentity(path string) (fileIdent, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileIdent{}, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return fileIdent{}, err
	}
	if !fi.IsDir() {
		return fileIdent{}, fmt.Errorf("%s is not a folder", path)
	}
	return handleIdentity(f)
}

// removeOwned removes dir/name, a kind of file Obelisk makes, when dir is one of
// the owned folders (compared by identity). s, when set, refuses anything under
// a source root.
func removeOwned(s *Store, owned []string, dir, name string, kind scratchKind) error {
	var ids []fileIdent
	for _, o := range owned {
		if strings.TrimSpace(o) == "" {
			continue
		}
		if id, err := ownFolderIdentity(o); err == nil {
			ids = append(ids, id)
		}
	}
	return removeOwnedByID(s, ids, dir, name, kind)
}

// removeOwnedByID is removeOwned for folders already known by identity.
func removeOwnedByID(s *Store, owned []fileIdent, dir, name string, kind scratchKind) error {
	if !validEntryName(name) {
		return fmt.Errorf("refusing to remove %q: not a single name in a folder", name)
	}
	if !kind.match(name) {
		return fmt.Errorf("refusing to remove %s: not a %s Obelisk makes", filepath.Join(dir, name), kind.label)
	}
	if s != nil {
		for _, p := range []string{dir, filepath.Join(dir, name)} {
			if err := s.CheckOutsideSources(p); err != nil {
				return fmt.Errorf("refusing to remove %s: %w", filepath.Join(dir, name), err)
			}
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer root.Close()
	id, err := rootIdentity(root)
	if err != nil {
		return err
	}
	ok := false
	for _, o := range owned {
		ok = ok || o == id
	}
	if !ok {
		return fmt.Errorf("refusing to remove %s: %s is not one of Obelisk's own folders", filepath.Join(dir, name), dir)
	}
	return removeEntryIn(root, name, kind.tree, nil)
}

// madeEntry is a file or folder this operation created exclusively, recorded by
// the identity of what it created.
type madeEntry struct {
	dir, name string
	id        fileIdent
}

// madeFile records f, just created exclusively (os.CreateTemp, O_EXCL).
func madeFile(f *os.File) (*madeEntry, error) {
	id, err := handleIdentity(f)
	if err != nil {
		return nil, err
	}
	return &madeEntry{dir: filepath.Dir(f.Name()), name: filepath.Base(f.Name()), id: id}, nil
}

// madeDir records the folder at path, just created exclusively (os.MkdirTemp,
// os.Mkdir).
func madeDir(path string) (*madeEntry, error) {
	id, err := ownFolderIdentity(path)
	if err != nil {
		return nil, err
	}
	return &madeEntry{dir: filepath.Dir(path), name: filepath.Base(path), id: id}, nil
}

// Path is where the entry was made.
func (m *madeEntry) Path() string { return filepath.Join(m.dir, m.name) }

// Moved records that this operation renamed the entry to path.
func (m *madeEntry) Moved(path string) { m.dir, m.name = filepath.Dir(path), filepath.Base(path) }

// Remove removes the entry if what is at its name is still the object this
// operation made. Already gone is not an error.
func (m *madeEntry) Remove() error {
	if m == nil {
		return nil
	}
	root, err := os.OpenRoot(m.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer root.Close()
	return removeEntryIn(root, m.name, true, &m.id)
}

// removeMade removes the file f this operation created, by identity. For the
// failure paths that hold only the open (or just-closed) file.
func removeMade(m *madeEntry) { _ = m.Remove() }

// removeMadePath removes path when it is still the object recorded at creation.
func removeMadePath(path string, id fileIdent) error {
	m := &madeEntry{dir: filepath.Dir(path), name: filepath.Base(path), id: id}
	return m.Remove()
}

// isLinkMode reports a link or other reparse point: never entered.
func isLinkMode(m fs.FileMode) bool { return m&(fs.ModeSymlink|fs.ModeIrregular) != 0 }

// removeEntryIn removes name inside root. A link at the name is refused. When
// want is set, the object at the name must have that identity. A folder (when
// tree allows one) is renamed to a removingPrefix name first, then emptied
// without following links.
func removeEntryIn(root *os.Root, name string, tree bool, want *fileIdent) error {
	fi, err := root.Lstat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if isLinkMode(fi.Mode()) {
		return fmt.Errorf("refusing to remove %s: it is a link", name)
	}
	if want != nil {
		f, err := root.Open(name)
		if err != nil {
			return err
		}
		id, err := handleIdentity(f)
		f.Close()
		if err != nil {
			return err
		}
		if id != *want {
			return fmt.Errorf("refusing to remove %s: it is no longer the one Obelisk made", name)
		}
	}
	if !fi.IsDir() {
		if scratchRemoveHook != nil {
			if err := scratchRemoveHook(name); err != nil {
				return err
			}
		}
		return root.Remove(name)
	}
	if !tree {
		return fmt.Errorf("refusing to remove %s: it is a folder", name)
	}
	aside := name
	if !strings.HasPrefix(name, removingPrefix) {
		aside = removingPrefix + randomHex(8)
		if err := root.Rename(name, aside); err != nil {
			return err
		}
	}
	return removeTreeIn(root, aside)
}

// removeTreeIn removes the folder rel inside root and everything in it. A link
// inside is removed as a link; nothing outside root is reached.
func removeTreeIn(root *os.Root, rel string) error {
	d, err := root.Open(rel)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		child := filepath.Join(rel, e.Name())
		fi, err := root.Lstat(child)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if fi.IsDir() && !isLinkMode(fi.Mode()) {
			if err := removeTreeIn(root, child); err != nil {
				return err
			}
			continue
		}
		if scratchRemoveHook != nil {
			if err := scratchRemoveHook(child); err != nil {
				return err
			}
		}
		if err := root.Remove(child); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := root.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Leftover is something an earlier or crashed run left in a folder, shown to the
// user with its size and age and removed only through the gate.
type Leftover struct {
	Folder   string    `json:"folder"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Kind     string    `json:"kind"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	AgeHours float64   `json:"age_hours"`
}

// leftoverKindOf is the leftover kind a name belongs to, if any.
func leftoverKindOf(name string) (scratchKind, bool) {
	for _, k := range leftoverKinds {
		if k.match(name) {
			return k, true
		}
	}
	return scratchKind{}, false
}

// listLeftoversIn lists the leftovers directly inside dir, without following
// links. A folder that does not exist has none.
func listLeftoversIn(dir string) ([]Leftover, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer root.Close()
	d, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var out []Leftover
	for _, e := range entries {
		k, ok := leftoverKindOf(e.Name())
		if !ok {
			continue
		}
		fi, err := root.Lstat(e.Name())
		if err != nil || isLinkMode(fi.Mode()) || fi.IsDir() != k.tree {
			continue
		}
		size := fi.Size()
		if fi.IsDir() {
			size = treeSizeIn(root, e.Name())
		}
		out = append(out, Leftover{Folder: dir, Name: e.Name(), Path: filepath.Join(dir, e.Name()), Kind: k.label,
			Size: size, Modified: fi.ModTime(), AgeHours: round2(now.Sub(fi.ModTime()).Hours())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// treeSizeIn is the total size of the regular files under rel, links not followed.
func treeSizeIn(root *os.Root, rel string) int64 {
	var n int64
	d, err := root.Open(rel)
	if err != nil {
		return 0
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return 0
	}
	for _, e := range entries {
		child := filepath.Join(rel, e.Name())
		fi, err := root.Lstat(child)
		if err != nil || isLinkMode(fi.Mode()) {
			continue
		}
		if fi.IsDir() {
			n += treeSizeIn(root, child)
		} else {
			n += fi.Size()
		}
	}
	return n
}
