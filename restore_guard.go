package main

// restore_guard.go — restore output through the destination guard.
//
// A restore writes only into an output folder authorized by the destination
// guard. Packages are extracted into a private work folder inside it, and each
// file is then published under its real name with the guard's no-replace step:
// a name already taken in the output folder is never replaced, and if any is
// taken the whole restore is refused before anything is published.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// restoreWork is a restore's private work folder inside the authorized output.
type restoreWork struct {
	rel string // relative to the output folder
	abs string // its real location, for tools that take a path
}

// newRestoreWork creates a uniquely named, new work folder in dest.
func newRestoreWork(dest *DestRoot) (restoreWork, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return restoreWork{}, err
	}
	rel := ".obelisk-restore-" + hex.EncodeToString(b)
	if _, err := dest.Stat(rel); err == nil {
		return restoreWork{}, fmt.Errorf("restore work folder %s already exists", rel)
	}
	if err := dest.MkdirAll(rel + "/x"); err != nil {
		return restoreWork{}, err
	}
	return restoreWork{rel: rel, abs: filepath.Join(dest.Canonical(), rel)}, nil
}

// extracted is where the package's files are unpacked.
func (w restoreWork) extracted() string { return filepath.Join(w.abs, "x") }

// removeWork removes a restore work folder, through the output folder's handle.
func (d *DestRoot) removeWork(rel string) error {
	v, err := destRel(rel)
	if err != nil {
		return err
	}
	return d.root.RemoveAll(v)
}

// copyForRepair copies the payload enc and every par2 file of its set (enc.par2,
// enc.vol….par2) into dir, and returns the copy of the payload. Only enc's folder
// is read; nothing is written there.
func copyForRepair(enc, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	src, base := filepath.Dir(enc), filepath.Base(enc)
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", err
	}
	names := []string{base}
	for _, e := range entries {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasPrefix(n, base+".") && strings.HasSuffix(strings.ToLower(n), ".par2") {
			names = append(names, n)
		}
	}
	for _, n := range names {
		if err := copyFile(filepath.Join(src, n), filepath.Join(dir, n)); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, base), nil
}

// publishRestored moves what was extracted into w to its real names in dest.
// Every name is checked first: if any file would replace something already in
// the output folder, or a folder would replace a file, nothing is published and
// the restore is refused, naming what is in the way. Files are then published
// with copy-then-verify and the no-replace step, so a name taken in the
// meantime is refused too. Links and other special entries are not restored
// and are reported. Returns the number of files restored.
func publishRestored(dest *DestRoot, w restoreWork) (int, []string, error) {
	root := w.extracted()
	var files, dirs, warnings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			dirs = append(dirs, rel)
		case d.Type().IsRegular():
			files = append(files, rel)
		default:
			warnings = append(warnings, fmt.Sprintf("%s was not restored: it is a link or special file, not a regular file", rel))
		}
		return nil
	})
	if err != nil {
		return 0, nil, fmt.Errorf("reading the extracted files: %w", err)
	}
	var taken []string
	for _, rel := range dirs {
		if fi, err := dest.Stat(rel); err == nil && !fi.IsDir() {
			taken = append(taken, rel)
		}
	}
	for _, rel := range files {
		if _, err := dest.Stat(rel); err == nil {
			taken = append(taken, rel)
		}
	}
	if len(taken) > 0 {
		shown := taken
		if len(shown) > 5 {
			shown = shown[:5]
		}
		more := ""
		if len(taken) > len(shown) {
			more = fmt.Sprintf(" and %d more", len(taken)-len(shown))
		}
		return 0, nil, fmt.Errorf("refusing to restore into %s: %d name(s) it would write are already taken there (%s%s). Obelisk never replaces a file when restoring; restore into an empty folder, or move those aside",
			dest.Name(), len(taken), strings.Join(shown, ", "), more)
	}
	for _, rel := range dirs {
		if err := dest.MkdirAll(rel); err != nil {
			return 0, warnings, err
		}
	}
	n := 0
	for _, rel := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		var meta *fileMeta
		if fi, err := os.Lstat(p); err == nil {
			meta = &fileMeta{mode: fi.Mode().Perm(), mtime: fi.ModTime()}
		}
		if _, _, err := copyVerifyNewToRoot(dest, p, rel, "", meta, nil, nil); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return n, warnings, fmt.Errorf("restore stopped: %s appeared in the output folder while restoring and was not replaced (%d file(s) were restored before it)", rel, n)
			}
			return n, warnings, fmt.Errorf("restore %s: %w (%d file(s) were restored before it)", rel, err, n)
		}
		_ = os.Remove(p) // free the space as we go; the work folder is removed at the end
		n++
	}
	// Folders last, deepest first, so adding their contents does not change the
	// times they are given.
	for i := len(dirs) - 1; i >= 0; i-- {
		rel := dirs[i]
		if fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			if err := dest.setFolderMeta(rel, fi.Mode().Perm(), fi.ModTime()); err != nil {
				warnings = append(warnings, fmt.Sprintf("the time or mode of folder %s was not restored: %v", rel, err))
			}
		}
	}
	return n, warnings, nil
}

// setFolderMeta gives a folder in the output its mode and modification time,
// through the output folder's handle.
func (d *DestRoot) setFolderMeta(rel string, mode fs.FileMode, mtime time.Time) error {
	v, err := destRel(rel)
	if err != nil {
		return err
	}
	if err := d.root.Chmod(v, mode); err != nil {
		return err
	}
	return d.root.Chtimes(v, mtime, mtime)
}

// destGuardAvailable reports whether the destination guard can authorize a
// folder on this platform. A variable so tests can exercise the new-folder
// restore that platforms without the guard use.
var destGuardAvailable = destGuardPlatform

// restoreTarget is where a restore writes: an output folder authorized by the
// destination guard, or, where the guard is not available, a new folder this
// restore created (newRestoreFolder).
type restoreTarget struct {
	dest    *DestRoot // nil for a new folder
	out     string    // the new folder (dest == nil)
	work    restoreWork
	into    string // where the package is unpacked
	notes   []string
	created bool      // out was created by this restore
	root    *os.Root  // a handle on the new folder (dest == nil)
	id      fileIdent // the new folder's identity
}

// openRestoreTarget prepares where a restore into outputDir writes.
func (a *App) openRestoreTarget(outputDir string) (*restoreTarget, error) {
	if !destGuardAvailable {
		return a.newRestoreFolder(outputDir)
	}
	dest, err := a.Store.AuthorizeDestination(outputDir)
	if err != nil {
		return nil, err
	}
	work, err := newRestoreWork(dest)
	if err != nil {
		dest.Close()
		return nil, err
	}
	return &restoreTarget{dest: dest, work: work, into: work.extracted(), notes: dest.Notes()}, nil
}

// newRestoreFolder is a restore's output where the destination guard is not
// available. The folder must not exist yet and its parent must. Before anything
// is created, the parent is opened and matched against every registered source
// by identity (volume and file ID of it and each folder above it, as quarantine
// does), so no spelling of a source (another letter case, a firmlink, a link)
// can put the new folder inside one; the path check on real locations is applied
// too. The folder is then created by this restore (mode 0700; creation fails if
// anything appeared at that name), opened, and checked again by identity. From
// then on the restore reaches it through that handle where it can: the work
// folder is made in it, what was unpacked is checked through it, and a failed
// restore empties it through it. tar is given its path, which is checked to
// still be the same folder before and after unpacking.
func (a *App) newRestoreFolder(outputDir string) (*restoreTarget, error) {
	abs, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	if err := a.Store.checkNewRestoreFolder(abs, outputDir); err != nil {
		return nil, err
	}
	if err := os.Mkdir(abs, 0o700); err != nil {
		return nil, fmt.Errorf("cannot create the restore folder %s: %w", outputDir, err)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		_ = os.Remove(abs) // removes the folder only while it is still empty
		return nil, fmt.Errorf("cannot open the restore folder %s: %w", outputDir, err)
	}
	tgt := &restoreTarget{out: abs, into: abs, created: true, root: r}
	fail := func(err error) (*restoreTarget, error) {
		r.Close()
		_ = os.Remove(abs) // still empty: remove only an empty folder
		return nil, err
	}
	if tgt.id, err = rootIdentity(r); err != nil {
		return fail(err)
	}
	real, err := rootCanonicalPath(r)
	if err != nil {
		return fail(err)
	}
	if err := a.Store.outsideSourcesByIdentity(identityChain(real, tgt.id), outputDir); err != nil {
		return fail(err)
	}
	if err := newRestoreFolderPathCheck(a.Store, real); err != nil {
		return fail(err)
	}
	if entries, err := fs.ReadDir(r.FS(), "."); err != nil || len(entries) != 0 {
		return fail(fmt.Errorf("the restore folder %s is not empty: something else wrote into it", outputDir))
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fail(err)
	}
	work := ".obelisk-restore-" + hex.EncodeToString(b)
	if err := r.Mkdir(work, 0o700); err != nil {
		return fail(err)
	}
	tgt.work = restoreWork{rel: work, abs: filepath.Join(abs, work)}
	tgt.notes = []string{fmt.Sprintf("On %s a restore goes only into a new folder; Obelisk created %s for it.", runtime.GOOS, outputDir)}
	return tgt, nil
}

// newRestoreFolderPathCheck is the path check on real locations for a new
// restore folder. A variable so a test can switch it off to show the identity
// match refuses on its own.
var newRestoreFolderPathCheck = (*Store).CheckOutsideSources

// checkNewRestoreFolder is the check before a new restore folder is created:
// it must not exist, its parent must, and neither the parent nor any folder
// above it may be a registered source, by identity or by real location.
func (s *Store) checkNewRestoreFolder(abs, display string) error {
	if _, err := os.Lstat(abs); err == nil {
		return fmt.Errorf("on %s a restore goes only into a new folder, and %s already exists. Choose a folder that does not exist yet; Obelisk creates it", runtime.GOOS, display)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("cannot check %s: %w", display, err)
	}
	parent := filepath.Dir(abs)
	pr, err := os.OpenRoot(parent)
	if err != nil {
		return fmt.Errorf("the folder that would hold %s does not exist or cannot be opened; create it first: %w", display, err)
	}
	pid, idErr := rootIdentity(pr)
	preal, realErr := rootCanonicalPath(pr)
	pr.Close()
	if idErr != nil || realErr != nil {
		return fmt.Errorf("cannot establish what holds %s: %w", display, errors.Join(idErr, realErr))
	}
	if err := s.outsideSourcesByIdentity(identityChain(preal, pid), display); err != nil {
		return err
	}
	return newRestoreFolderPathCheck(s, filepath.Join(preal, filepath.Base(abs)))
}

// outsideSourcesByIdentity refuses when any entry of chain (a folder and the
// folders above it) is a registered source, matched by identity. An identity
// with no file ID (some network shares report none) matches nothing; the path
// checks cover those.
func (s *Store) outsideSourcesByIdentity(chain []idEntry, display string) error {
	snap, err := s.sourceSnapshot()
	if err != nil {
		return err
	}
	for _, src := range snap.srcs {
		if src.offline || !src.fromHandle {
			continue
		}
		sid, err := pathIdentity(src.canonical)
		if err != nil || sid.idx == 0 {
			continue
		}
		for _, e := range chain {
			if e.id == sid {
				return fmt.Errorf("refusing: %s would be inside registered source %s (%s is that folder); Obelisk never writes into source data", display, src.registered, e.path)
			}
		}
	}
	return nil
}

// stillTheFolder checks that the new folder's path still names the folder that
// was created and is held open.
func (t *restoreTarget) stillTheFolder() error {
	id, err := pathIdentity(t.out)
	if err != nil || id != t.id {
		return fmt.Errorf("the restore folder %s was replaced while restoring; stopped", t.out)
	}
	return nil
}

// publish gives the unpacked files their real names (see publishRestored). In
// a new folder they are already in place: the work folder is removed and what
// was unpacked is checked through the folder's handle. Links and other special
// entries are removed and reported, as a guarded restore does not restore them.
func (t *restoreTarget) publish() (int, []string, error) {
	if t.dest != nil {
		return publishRestored(t.dest, t.work)
	}
	if err := t.stillTheFolder(); err != nil {
		return 0, nil, err
	}
	if err := t.root.RemoveAll(t.work.rel); err != nil {
		return 0, nil, err
	}
	n := 0
	var warnings, special []string
	err := fs.WalkDir(t.root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case p == "." || d.IsDir():
		case d.Type().IsRegular():
			n++
		default:
			special = append(special, p)
		}
		return nil
	})
	for _, p := range special {
		if rerr := t.root.Remove(p); rerr != nil {
			err = errors.Join(err, rerr)
		}
		warnings = append(warnings, fmt.Sprintf("%s was not restored: it is a link or special file, not a regular file", p))
	}
	return n, warnings, err
}

// closeRestoreTarget removes the work folder, and empties and removes a new
// restore folder that a failed restore leaves behind (everything in it came
// from this restore). The new folder is emptied through its handle; the folder
// itself is removed only while its path still names it.
func (a *App) closeRestoreTarget(t *restoreTarget, succeeded bool) {
	if t.dest != nil {
		if err := t.dest.removeWork(t.work.rel); err != nil {
			a.Store.Log("restore", fmt.Sprintf("could not remove the work folder %s in %s: %v", t.work.rel, t.dest.Name(), err))
		}
		t.dest.Close()
		return
	}
	defer t.root.Close()
	_ = t.root.RemoveAll(t.work.rel)
	if succeeded || !t.created {
		return
	}
	var err error
	if entries, rerr := fs.ReadDir(t.root.FS(), "."); rerr != nil {
		err = rerr
	} else {
		for _, e := range entries {
			err = errors.Join(err, t.root.RemoveAll(e.Name()))
		}
	}
	if err == nil {
		if err = t.stillTheFolder(); err == nil {
			err = os.Remove(t.out) // removes only an empty folder
		}
	}
	if err != nil {
		a.Store.Log("restore", fmt.Sprintf("could not remove the restore folder %s after the restore failed: %v", t.out, err))
	}
}
