package main

// writecheck.go — before Obelisk puts a catalog, data folder, keystore or backup
// output somewhere, it checks the current account can really create, write, rename
// and delete there.
//
// Two steps. The first only reads: the volume's and media's read-only state and the
// folder's access list. The second, for a folder that exists, really does it with a
// small file of its own and removes it, because some limits (a share's own
// permissions, say) cannot be read from here. A folder that cannot take writes is
// refused before anything is written, the cause is named, and Obelisk never moves on
// to another location by itself.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// destWriteUse is how a refusal names an output folder.
const destWriteUse = "this output"

// WriteAccessError is the refusal for a place the account cannot really write to.
type WriteAccessError struct {
	Folder string
	What   string // what Obelisk meant to put there
	Cause  string
	Fix    string
	Err    error
	Left   string // a test file that could not be removed
}

func (e *WriteAccessError) Error() string {
	if e.Left != "" {
		return fmt.Sprintf("Obelisk can't use %s for %s: %s. Its small test file %s could not be removed, so you can delete it. Obelisk wrote nothing else there and did not switch to another location. %s",
			e.Folder, e.What, e.Cause, e.Left, e.Fix)
	}
	return fmt.Sprintf("Obelisk can't use %s for %s: %s. Nothing was written there, and Obelisk did not switch to another location. %s",
		e.Folder, e.What, e.Cause, e.Fix)
}

func (e *WriteAccessError) Unwrap() error { return e.Err }

// writeFacts is what can be read about a folder without writing to it.
type writeFacts struct {
	ReadOnlyVolume bool     // the volume is mounted read-only
	WriteProtected bool     // the media reports write protection
	ReadOnlyShare  bool     // a network share that is read-only
	DeniedBy       []string // accounts or groups with a deny entry that applies here
	CannotChange   bool     // the access list gives this account no change rights
}

// writeAccessFacts reads the facts about dir, an existing folder. newFolder means
// only a subfolder would be made in it. A variable so tests can present media and
// shares that cannot be set up here.
var writeAccessFacts = readWriteAccessFacts

// writeProbeStep is a test seam: an error stops the real attempt at that step
// ("create", "rename" or "delete").
var writeProbeStep = func(string) error { return nil }

func writeRefusal(folder, what string, f writeFacts) *WriteAccessError {
	e := &WriteAccessError{Folder: folder, What: what}
	switch {
	case f.ReadOnlyShare:
		e.Cause = "this network share is read-only for your account"
		e.Fix = "Ask whoever manages the share for change access, or choose another folder."
	case f.WriteProtected:
		e.Cause = "the card or disk is write-protected"
		e.Fix = "Slide its lock switch off or remove the write protection, or choose another folder."
	case f.ReadOnlyVolume:
		e.Cause = "the drive is read-only"
		e.Fix = "Make the drive writable, or choose another folder."
	case len(f.DeniedBy) > 0:
		e.Cause = "a deny entry for " + joinAnd(f.DeniedBy) + " in the folder's permissions stops your account from changing it"
		e.Fix = "Remove the deny entry, or choose another folder."
	case f.CannotChange:
		e.Cause = "your account can read this folder but cannot change it"
		e.Fix = "Give your account change access, or choose another folder."
	default:
		return nil
	}
	return e
}

// checkWriteAccess reads, and writes nothing. For a folder not there yet it checks
// the deepest existing folder could hold a new one.
func checkWriteAccess(path, what string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return &WriteAccessError{Folder: path, What: what, Cause: "its location could not be read", Fix: "Choose another folder.", Err: err}
	}
	target, newFolder := abs, false
	st, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		target, _, err = deepestExistingDir(abs)
		if errors.Is(err, errNoExistingParent) {
			return &WriteAccessError{Folder: abs, What: what, Cause: "its drive is not connected", Fix: "Connect the drive, or choose another folder.", Err: err}
		}
		if err != nil {
			return &WriteAccessError{Folder: abs, What: what, Cause: "a folder on the way to it could not be read (" + err.Error() + ")", Fix: "Check that path, or choose another folder.", Err: err}
		}
		newFolder = true
	case err != nil:
		return &WriteAccessError{Folder: abs, What: what, Cause: "it could not be opened (" + err.Error() + ")", Fix: "Check the drive and its permissions, or choose another folder.", Err: err}
	case !st.IsDir():
		return &WriteAccessError{Folder: abs, What: what, Cause: "it is a file, not a folder", Fix: "Choose a folder."}
	}
	facts, err := writeAccessFacts(target, newFolder)
	if err != nil {
		return &WriteAccessError{Folder: target, What: what, Cause: "its permissions could not be read (" + err.Error() + ")", Fix: "Check the drive and its permissions, or choose another folder.", Err: err}
	}
	if r := writeRefusal(target, what, facts); r != nil {
		return r
	}
	return nil
}

// proveWriteAccess is checkWriteAccess and, for a folder that exists, the real
// attempt. A folder not there yet is proved once it has been made.
func proveWriteAccess(path, what string) error {
	if err := checkWriteAccess(path, what); err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil
	}
	return probeResult(abs, what, runWriteProbe(pathProbe{abs}))
}

// proveRootWrite is proveWriteAccess through a held folder handle.
func proveRootWrite(r *os.Root, canon, what string) error {
	if err := checkWriteAccess(canon, what); err != nil {
		return err
	}
	return probeResult(canon, what, runWriteProbe(rootProbe{r}))
}

// prepareDataFolder makes sure dir can hold a catalog, creating it if needed. If it
// cannot, the folders this call made are removed again.
func prepareDataFolder(dir string) error {
	_, err := makeProvedFolder(dir, "the data folder")
	return err
}

// makeProvedFolder makes sure dir exists and really takes writes. If it does not, the
// folders this call made are removed again. undo removes them later.
func makeProvedFolder(dir, what string) (undo func(), err error) {
	if err := checkWriteAccess(dir, what); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	undo, err = makeFolderTracked(abs)
	if err != nil {
		return nil, probeResult(abs, what, err)
	}
	if err := proveWriteAccess(abs, what); err != nil {
		undo()
		return nil, err
	}
	return undo, nil
}

// makeFolderTracked creates abs and any missing folders above it. undo removes the
// ones this call made, deepest first, and only while they are empty.
func makeFolderTracked(abs string) (undo func(), err error) {
	var made []string
	if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
		base, missing, derr := deepestExistingDir(abs)
		if derr != nil {
			return nil, derr
		}
		p := base
		for _, m := range missing {
			p = filepath.Join(p, m)
			made = append(made, p)
		}
	}
	mkErr := os.MkdirAll(abs, 0o755)
	// Each folder that was missing and is there now was made by this call: its
	// identity is recorded, so undo removes only those folders.
	ids := folderIdentities(made)
	undo = func() { removeMadeFolders(made, ids) }
	if mkErr != nil {
		undo()
		return nil, mkErr
	}
	return undo, nil
}

// folderIdentities is the identity of each folder in paths that is there now.
func folderIdentities(paths []string) []*fileIdent {
	ids := make([]*fileIdent, len(paths))
	for i, p := range paths {
		if id, err := ownFolderIdentity(p); err == nil {
			ids[i] = &id
		}
	}
	return ids
}

// removeMadeFolders removes empty folders Obelisk made, deepest first, each only
// while it is still the folder made (ids), and returns those still there.
func removeMadeFolders(made []string, ids []*fileIdent) []string {
	var left []string
	for i := len(made) - 1; i >= 0; i-- {
		if i >= len(ids) || ids[i] == nil {
			if _, err := os.Lstat(made[i]); err == nil {
				left = append([]string{made[i]}, left...)
			}
			continue
		}
		if err := removeEmptyFolder(made[i], *ids[i]); err != nil && !errors.Is(err, fs.ErrNotExist) {
			left = append([]string{made[i]}, left...)
		}
	}
	return left
}

// probeFS is what the real attempt needs: a path-based folder or an *os.Root.
type probeFS interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Rename(oldname, newname string) error
	Remove(name string) error
}

type pathProbe struct{ dir string }

func (p pathProbe) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(filepath.Join(p.dir, name), flag, perm)
}
func (p pathProbe) Rename(a, b string) error {
	return renameNew(filepath.Join(p.dir, a), filepath.Join(p.dir, b))
}
func (p pathProbe) Remove(name string) error {
	return removeOwned(nil, []string{p.dir}, p.dir, name, kindWriteProbe)
}

// rootProbe is a write check through a held folder handle.
type rootProbe struct{ r *os.Root }

func (p rootProbe) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return p.r.OpenFile(name, flag, perm)
}
func (p rootProbe) Rename(a, b string) error { return renameNewInRoot(p.r, a, b) }
func (p rootProbe) Remove(name string) error { return removeKindInRoot(p.r, name, kindWriteProbe) }

// runWriteProbe creates, writes, renames and deletes one small file of its own, and
// leaves nothing whichever step fails.
func runWriteProbe(fsys probeFS) (err error) {
	tok := make([]byte, 8)
	if _, err := rand.Read(tok); err != nil {
		return err
	}
	name := ".obelisk-write-check-" + hex.EncodeToString(tok)
	if err := writeProbeStep("create"); err != nil {
		return err
	}
	f, err := fsys.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	current := name
	made, idErr := handleIdentity(f)
	// stillMade reports whether the test file is still the one this check made.
	stillMade := func(n string) bool {
		g, err := fsys.OpenFile(n, os.O_RDONLY, 0)
		if err != nil {
			return false
		}
		id, err := handleIdentity(g)
		g.Close()
		return idErr == nil && err == nil && id == made
	}
	defer func() {
		if err != nil {
			var rerr error
			if _, serr := fsys.OpenFile(current, os.O_RDONLY, 0); serr != nil {
				rerr = serr
			} else if !stillMade(current) {
				rerr = fmt.Errorf("%s is no longer the file this check made", current)
			} else {
				rerr = retryTransientRename(func() error { return fsys.Remove(current) })
			}
			if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
				err = &probeLeftError{name: current, err: err}
			}
		}
	}()
	_, err = f.Write([]byte("obelisk write check\n"))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err = writeProbeStep("rename"); err != nil {
		return err
	}
	moved := name + ".moved"
	if err = retryTransientRename(func() error { return fsys.Rename(name, moved) }); err != nil {
		return err
	}
	current = moved
	if err = writeProbeStep("delete"); err != nil {
		return err
	}
	if !stillMade(moved) {
		return fmt.Errorf("%s is no longer the file this check made", moved)
	}
	return retryTransientRename(func() error { return fsys.Remove(moved) })
}

// probeLeftError is a failed attempt whose test file could not be removed.
type probeLeftError struct {
	name string
	err  error
}

func (e *probeLeftError) Error() string { return e.err.Error() }
func (e *probeLeftError) Unwrap() error { return e.err }

// probeResult turns a failed real attempt into the refusal that names its cause.
func probeResult(folder, what string, err error) error {
	if err == nil {
		return nil
	}
	var r *WriteAccessError
	switch {
	case errWriteProtected(err):
		r = writeRefusal(folder, what, writeFacts{WriteProtected: true})
	case errors.Is(err, fs.ErrPermission) && pathOnNetwork(folder):
		r = writeRefusal(folder, what, writeFacts{ReadOnlyShare: true})
	case errors.Is(err, fs.ErrPermission):
		r = writeRefusal(folder, what, writeFacts{CannotChange: true})
	default:
		r = &WriteAccessError{Folder: folder, What: what,
			Cause: "a test write failed (" + strings.TrimSpace(err.Error()) + ")",
			Fix:   "Check the drive, or choose another folder."}
	}
	r.Err = err
	var left *probeLeftError
	if errors.As(err, &left) {
		r.Left = filepath.Join(folder, left.name)
	}
	return r
}
