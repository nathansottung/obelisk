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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
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
		if strings.HasPrefix(base, ".catalog.") && strings.Contains(base, ".check-") {
			return true
		}
		return strings.HasPrefix(base, "catalog.") && (strings.Contains(base, ".export-") || strings.Contains(base, ".stage-") || strings.HasSuffix(base, ".stage"))
	}}
	// kindDBSide is a side file SQLite keeps beside the live catalog database.
	kindDBSide = scratchKind{label: "catalog side file", match: func(n string) bool {
		return n == "catalog.db-wal" || n == "catalog.db-shm" || n == "catalog.db-journal"
	}}
	// kindCatalogWork is a work folder made beside the catalog for one restore.
	kindCatalogWork = scratchKind{label: "unfinished catalog restore", tree: true, leftover: true, match: func(n string) bool {
		return strings.HasPrefix(n, "restore-stage-")
	}}
	// kindLock is a lock file Obelisk holds while it moves its catalog.
	kindLock = scratchKind{label: "lock file", match: func(n string) bool { return n == "catalog.db.move-lock" }}
	// kindCopyID is the COPY-ID.txt a burn puts in a staged package for that burn.
	kindCopyID = scratchKind{label: "burn copy ID", match: func(n string) bool { return n == copyIDFile }}
	// kindIntermediateTar is a build's plain tar, removed once it is encrypted.
	kindIntermediateTar = scratchKind{label: "intermediate tar", match: func(n string) bool { return strings.HasSuffix(n, ".tar") }}
	// kindEccStage is the error-correction file dvdisaster is writing, before it
	// is given its name.
	kindEccStage = scratchKind{label: "unfinished error-correction file", leftover: true, match: func(n string) bool {
		return strings.HasPrefix(n, ".") && strings.HasSuffix(n, ".partial.ecc")
	}}
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

// kindRestoreWork is a restore's private work folder in its output folder.
var kindRestoreWork = scratchKind{label: "restore work folder", tree: true, leftover: true, match: func(n string) bool {
	return strings.HasPrefix(n, ".obelisk-restore-")
}}

// removeKindInRoot removes rel inside root when its name is of kind (a folder
// whose removal follows no links). For Obelisk's own work folder inside an
// output folder it holds a handle on.
func removeKindInRoot(root *os.Root, rel string, kind scratchKind) error {
	base := filepath.Base(filepath.FromSlash(rel))
	if !validEntryName(base) || !kind.match(base) {
		return fmt.Errorf("refusing to remove %s: not a %s Obelisk makes", rel, kind.label)
	}
	return removeEntryIn(root, filepath.FromSlash(rel), kind.tree, nil)
}

// removeMadeInRoot removes the file rel inside root when it is still the object
// with identity id (one this operation created).
func removeMadeInRoot(root *os.Root, rel string, id fileIdent) error {
	return removeEntryIn(root, filepath.FromSlash(rel), false, &id)
}

// removeMadeTree removes, inside root, only the entries recorded in made (files
// by identity; folders by identity and only once empty), deepest first. Anything
// else is left where it is and returned.
func removeMadeTree(root *os.Root, made map[string]fileIdent) (left []string, err error) {
	names := make([]string, 0, len(made))
	for n := range made {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j]) || len(names[i]) == len(names[j]) && names[i] > names[j]
	})
	for _, n := range names {
		rel := filepath.FromSlash(n)
		fi, lerr := root.Lstat(rel)
		if lerr != nil {
			continue
		}
		id := made[n]
		if !fi.IsDir() {
			err = errors.Join(err, removeEntryIn(root, rel, false, &id))
			continue
		}
		if isLinkMode(fi.Mode()) {
			continue
		}
		d, oerr := root.Open(rel)
		if oerr != nil {
			continue
		}
		got, ierr := handleIdentity(d)
		entries, _ := d.ReadDir(1)
		d.Close()
		if ierr != nil || got != id || len(entries) > 0 {
			left = append(left, n)
			continue
		}
		err = errors.Join(err, root.Remove(rel))
	}
	return left, err
}

// removeEmptyFolderIn removes the empty folder rel inside root; a file or link
// there is refused, and a folder with anything in it is kept.
func removeEmptyFolderIn(root *os.Root, rel string) error {
	fi, err := root.Lstat(rel)
	if err != nil {
		return err
	}
	if !fi.IsDir() || isLinkMode(fi.Mode()) {
		return fmt.Errorf("refusing to remove %s: not a folder", rel)
	}
	return root.Remove(rel) // a folder is removed only when empty
}

// removeEmptyFolder removes the folder at path only while it is empty and is
// still the folder with identity id. It never removes a file.
func removeEmptyFolder(path string, id fileIdent) error {
	got, err := ownFolderIdentity(path)
	if err != nil {
		return err
	}
	if got != id {
		return fmt.Errorf("refusing to remove %s: it is no longer the folder Obelisk made", path)
	}
	return rmdir(path)
}

// publishNewInRoot gives the staged file tmp (identity id) the name v inside
// root only when nothing is there: a hard link refuses an existing name in the
// same step; where links are not supported, the name is checked just before the
// rename. The staging name is then removed. An existing entry is never replaced
// (the error wraps fs.ErrExist).
func publishNewInRoot(root *os.Root, tmp, v string, id fileIdent) error {
	tmp, v = filepath.FromSlash(tmp), filepath.FromSlash(v)
	err := root.Link(tmp, v)
	if err == nil {
		_ = removeEntryIn(root, tmp, false, &id) // the staging name of the published file
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s is already there and was left as it is: %w", filepath.ToSlash(v), fs.ErrExist)
	}
	if _, lerr := root.Lstat(v); lerr == nil {
		return fmt.Errorf("%s is already there and was left as it is: %w", filepath.ToSlash(v), fs.ErrExist)
	} else if !errors.Is(lerr, fs.ErrNotExist) {
		return lerr
	}
	return root.Rename(tmp, v)
}

// publishNoReplace moves the finished file from to final, refusing when final
// already exists. publishAbsent is the atomic no-replace publish. Off Windows, on a
// filesystem without hard links (FAT, exFAT), the absence is checked just before
// the rename; on Windows the move needs no such fallback, so a failure is returned
// and nothing can replace a file that arrived. The folder entry is then flushed.
func publishNoReplace(from, final string) error {
	if err := publishAbsent(from, final); err == nil {
		// final is in place. Off Windows the staging name is a second name for the
		// same file, made by this process; one that cannot be removed now is
		// listed to the user as a leftover.
		_ = removeStage(from)
	} else {
		if _, serr := os.Lstat(final); !errors.Is(serr, fs.ErrNotExist) {
			return fmt.Errorf("%s already exists; refusing to replace it", final)
		}
		if !publishAbsentHasFallback {
			return err
		}
		if err := renameReplacing(from, final); err != nil {
			return err
		}
	}
	return syncDir(filepath.Dir(final))
}

// kindBurnHold is the folder a burn keeps staged-only files in, beside the
// burned folder.
var kindBurnHold = scratchKind{label: "burn hold folder", tree: true, leftover: true, match: func(n string) bool {
	return strings.HasPrefix(n, ".obelisk-burn-hold-")
}}

// removeEmptyOwnFolder removes parent/name, an empty folder of kind; a folder
// with anything in it, a file or a link is kept.
func removeEmptyOwnFolder(parent, name string, kind scratchKind) error {
	if !validEntryName(name) || !kind.match(name) {
		return fmt.Errorf("refusing to remove %s: not a %s Obelisk makes", filepath.Join(parent, name), kind.label)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	return removeEmptyFolderIn(root, name)
}

// kindWriteProbe is the small file a write check makes and removes again.
var kindWriteProbe = scratchKind{label: "write-check file", leftover: true, match: func(n string) bool {
	return strings.HasPrefix(n, ".obelisk-write-check-")
}}

// renameNewInRoot renames from to to inside root only when nothing is at to.
func renameNewInRoot(root *os.Root, from, to string) error {
	if _, err := root.Lstat(to); err == nil {
		return fmt.Errorf("%s is already there: %w", to, fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return root.Rename(from, to)
}

// leftoverKinds are the kinds listed to the user when found from an earlier run.
var leftoverKinds = []scratchKind{kindRemoving, kindBuild, kindPrevious, kindRefill, kindStage, kindCatalogTemp, kindCatalogWork, kindEccStage, kindBurnHold, kindWriteProbe}

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

// madeStages records, by path, the staging files this process created
// exclusively (createStage), so a failure path can remove one by name and the
// helper still checks it is the same file.
var madeStages sync.Map // path -> fileIdent

// rememberMade records f, just created exclusively, for removeStage.
func rememberMade(f *os.File) {
	if id, err := handleIdentity(f); err == nil {
		madeStages.Store(f.Name(), id)
	}
}

// stageIdentity is the identity recorded for a staging file this process made.
func stageIdentity(path string) (fileIdent, bool) {
	v, ok := madeStages.Load(path)
	if !ok {
		return fileIdent{}, false
	}
	return v.(fileIdent), true
}

// publishedStage removes final, a file this operation just published from the
// staging file whose identity is id, when nothing has replaced it since.
func removePublished(final string, id fileIdent, ok bool) {
	if ok {
		_ = removeMadePath(final, id)
	}
}

// forgetMade drops the record of a staging file that was given its final name.
func forgetMade(path string) { madeStages.Delete(path) }

// removeStage removes a staging file this process created, when what is at its
// name is still that file. A name it did not create is refused.
func removeStage(path string) error {
	v, ok := madeStages.LoadAndDelete(path)
	if !ok {
		return fmt.Errorf("refusing to remove %s: Obelisk did not make it in this run", path)
	}
	return removeMadePath(path, v.(fileIdent))
}

// removeStageIn removes a staging file Obelisk made in dir: by identity when
// this process recorded it, otherwise by its staging name in that folder only.
func removeStageIn(dir, path string) error {
	if _, ok := stageIdentity(path); ok {
		return removeStage(path)
	}
	if filepath.Dir(path) != filepath.Clean(dir) {
		return fmt.Errorf("refusing to remove %s: not in %s", path, dir)
	}
	return removeOwned(nil, []string{dir}, dir, filepath.Base(path), kindStage)
}

// writeNewFile writes data to a new file at path, which must not exist yet: it
// never truncates or replaces anything. For files written into a folder this
// same operation just made.
func writeNewFile(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		if id, ierr := ownFileIdentity(path); ierr == nil {
			_ = removeMadePath(path, id)
		}
	}
	return err
}

// ownFileIdentity is the identity of the file at path, from an open handle.
func ownFileIdentity(path string) (fileIdent, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileIdent{}, err
	}
	defer f.Close()
	return handleIdentity(f)
}

// isCatalogFolder reports whether dir holds an Obelisk catalog (catalog.db or
// catalog.json) or its owner lock, which makes it Obelisk's data folder.
func isCatalogFolder(dir string) bool {
	for _, n := range []string{"catalog.db", "catalog.json", dataOwnerFile} {
		if fi, err := os.Lstat(filepath.Join(dir, n)); err == nil && fi.Mode().IsRegular() {
			return true
		}
	}
	return false
}

// sqliteSideFiles are the files SQLite keeps beside a database.
var sqliteSideFiles = []string{"-wal", "-shm", "-journal"}

// removeDBScratch removes a database file this operation made at path (by
// identity, when given) and the side files SQLite made beside it. Only catalog
// scratch names are accepted (kindCatalogTemp), and only when path is directly
// in ownedDir, the folder that holds the catalog (compared by identity).
func removeDBScratch(ownedDir, path string, id *fileIdent) error {
	dir, name := filepath.Dir(path), filepath.Base(path)
	if !isCatalogFolder(ownedDir) {
		return fmt.Errorf("refusing to remove %s: %s does not hold an Obelisk catalog", path, ownedDir)
	}
	owned, err := ownFolderIdentity(ownedDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	if id != nil {
		if here, err := ownFolderIdentity(dir); err != nil || here != owned {
			return fmt.Errorf("refusing to remove %s: not in the catalog's folder", path)
		}
		errs = append(errs, removeMadePath(path, *id))
	} else {
		errs = append(errs, removeOwnedByID(nil, []fileIdent{owned}, dir, name, kindCatalogTemp))
	}
	for _, side := range sqliteSideFiles {
		errs = append(errs, removeOwnedByID(nil, []fileIdent{owned}, dir, name+side, kindCatalogTemp))
	}
	return errors.Join(errs...)
}

// rmdir removes the folder at path only if it is empty; it never removes a file.
func rmdir(path string) error { return syscall.Rmdir(path) }

// randomHex is n random bytes in hex, for unique names.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
