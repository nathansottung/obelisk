package main

// outfolder.go — the output folder of a writer other than mirrors, plans and
// restores: package and span writes, seal sidecars, BagIt exports, the recovery
// kit, escrow bundles and app-backup exports.
//
// Where the destination guard is available (Windows, Linux) the folder is an
// authorized DestRoot, so it is decided by where it really is and every write is
// confined beneath the handle that was decided about. Elsewhere it is opened after
// the guard's source check, its identity is matched against every source (as a new
// restore folder is), and writes go through that handle.
//
// Either way each file is written into a new staging file created exclusively,
// checked against what was meant to be written, and only then given its name, so
// an existing file (or a hard link to someone else's file) is never opened for
// writing. Giving it its name replaces whatever entry is there; the object that
// entry named is left as it was.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type outFolder struct {
	dest *DestRoot // guard platforms
	root *os.Root  // elsewhere: the folder, opened after its source checks
	path string    // the folder as given, for messages and records

	tok    string
	seq    atomic.Uint64
	mu     sync.Mutex
	stages map[string]*os.File // elsewhere: staging files this folder owns
}

// openOutFolder opens path, an existing folder, for writing. On guard platforms
// a folder that does not exist yet, or that needs the user's choice, is refused
// with a *DestinationChoiceError (the routes ask first; see destinationGate).
func (a *App) openOutFolder(path string) (*outFolder, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("no folder given")
	}
	if destGuardAvailable {
		d, err := a.Store.AuthorizeDestination(path)
		if err != nil {
			return nil, err
		}
		return &outFolder{dest: d, path: path}, nil
	}
	return a.openOutFolderUnguarded(path)
}

// outFolderReady reports, without writing, whether openOutFolder(path) would be
// allowed now: refused, waiting on the user's choice (a *DestinationChoiceError),
// or nil.
func (a *App) outFolderReady(path string) error {
	if !destGuardAvailable {
		return a.Store.CheckOutsideSources(path)
	}
	c := a.Store.CheckDestination(path)
	switch c.Status {
	case destStatusRefuse:
		return fmt.Errorf("%s", c.Reason)
	case destStatusChoose, destStatusCreate:
		return &DestinationChoiceError{Check: c}
	}
	return nil
}

// openOutFolderUnguarded is openOutFolder where the destination guard is not
// available. The folder is checked by real location before anything is created,
// created if missing, opened, and checked again on the handle: its real location
// by path and it and every folder above it by identity.
func (a *App) openOutFolderUnguarded(p string) (*outFolder, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	if err := a.Store.CheckOutsideSources(abs); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*outFolder, error) {
		r.Close()
		return nil, err
	}
	id, err := rootIdentity(r)
	if err != nil {
		return fail(err)
	}
	real, err := rootCanonicalPath(r)
	if err != nil {
		return fail(fmt.Errorf("refusing: could not establish the real location of %s: %w", p, err))
	}
	chain, err := identityChain(real, id)
	if err != nil {
		return fail(fmt.Errorf("refusing to write into %s: %w", p, err))
	}
	if err := a.Store.outsideSourcesByIdentity(chain, p); err != nil {
		return fail(err)
	}
	if err := a.Store.CheckOutsideSources(real); err != nil {
		return fail(err)
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fail(err)
	}
	return &outFolder{root: r, path: p, tok: hex.EncodeToString(b), stages: map[string]*os.File{}}, nil
}

// Name is the folder as given.
func (o *outFolder) Name() string { return o.path }

// Join is the path of rel inside the folder, for records and messages. It is
// never reopened to write.
func (o *outFolder) Join(rel string) string {
	if rel == "" || rel == "." {
		return o.path
	}
	return filepath.Join(o.path, filepath.FromSlash(rel))
}

// Notes are the destination's notes for the job summary (guard platforms).
func (o *outFolder) Notes() []string {
	if o.dest != nil {
		return o.dest.Notes()
	}
	return nil
}

// Close releases the folder and removes any staging file not published.
func (o *outFolder) Close() error {
	if o == nil {
		return nil
	}
	if o.dest != nil {
		return o.dest.Close()
	}
	o.mu.Lock()
	tmps := make([]string, 0, len(o.stages))
	for t := range o.stages {
		tmps = append(tmps, t)
	}
	o.mu.Unlock()
	for _, t := range tmps {
		o.Discard(t)
	}
	return o.root.Close()
}

// MkdirAll creates rel and its parents inside the folder.
func (o *outFolder) MkdirAll(rel string) error {
	r := filepath.ToSlash(rel)
	if r == "" || r == "." {
		return nil
	}
	if o.dest != nil {
		return o.dest.MkdirAll(r)
	}
	v, err := destRel(r)
	if err != nil {
		return err
	}
	return o.root.MkdirAll(v, 0o755)
}

// Create makes a new staging file for rel (creating rel's folder) and returns
// its name and the open file. The file belongs to the folder: Commit or Discard
// it, never Close it.
func (o *outFolder) Create(rel string) (string, *os.File, error) {
	v, err := destRel(filepath.ToSlash(rel))
	if err != nil {
		return "", nil, err
	}
	if err := o.MkdirAll(path.Dir(v)); err != nil {
		return "", nil, err
	}
	if o.dest != nil {
		return o.dest.stagingFor(v)
	}
	tmp := path.Join(path.Dir(v), fmt.Sprintf(".obelisk-%s-%d.tmp", o.tok, o.seq.Add(1)))
	f, err := o.root.OpenFile(tmp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", nil, err
	}
	o.mu.Lock()
	o.stages[tmp] = f
	o.mu.Unlock()
	return tmp, f, nil
}

// Commit gives the staging file tmp the mode perm, flushes it, reads it back,
// and only if it holds exactly the bytes with SHA-256 sum publishes it as rel,
// replacing whatever entry is at rel. On any failure nothing is published and
// the staging file is removed; a read-back that differs wraps errStageMismatch.
func (o *outFolder) Commit(tmp, rel string, perm fs.FileMode, sum string) error {
	v, err := destRel(filepath.ToSlash(rel))
	if err != nil {
		o.Discard(tmp)
		return err
	}
	if writeFlushFaultHook != nil {
		if err := writeFlushFaultHook(o.Join(rel)); err != nil {
			o.Discard(tmp)
			return fmt.Errorf("flushing %s: %w", o.Join(rel), err)
		}
	}
	if o.dest != nil {
		st := o.dest.stage(tmp)
		if st == nil {
			return fmt.Errorf("publish %s: staging file is not owned by this write", rel)
		}
		err := st.file.Chmod(perm)
		if err == nil {
			err = st.file.Sync()
		}
		if err == nil && sum == "" {
			err = fmt.Errorf("publish %s: no content digest to check against", rel)
		}
		if err == nil && st.expected != "" && st.expected != sum {
			err = fmt.Errorf("publish %s: expected content changed", rel)
		}
		if err == nil {
			// Publish reads the staged bytes back and checks them against this
			// digest before giving the file its name (VerifyStage).
			st.expected = sum
			err = o.dest.Publish(tmp, v)
		}
		if err != nil {
			o.Discard(tmp)
		}
		return err
	}
	o.mu.Lock()
	f := o.stages[tmp]
	o.mu.Unlock()
	if f == nil {
		return fmt.Errorf("publish %s: staging file is not owned by this write", rel)
	}
	err = f.Chmod(perm)
	if err == nil {
		err = f.Sync()
	}
	if err == nil {
		_, err = f.Seek(0, io.SeekStart)
	}
	var got string
	if err == nil {
		got, err = hashReader(f)
	}
	if err == nil && got != sum {
		err = fmt.Errorf("%s: %w", rel, errStageMismatch)
	}
	if err != nil {
		o.Discard(tmp)
		return err
	}
	o.mu.Lock()
	delete(o.stages, tmp)
	o.mu.Unlock()
	if err := f.Close(); err != nil {
		_ = o.root.Remove(tmp)
		return err
	}
	if err := o.root.Rename(tmp, v); err != nil {
		_ = o.root.Remove(tmp)
		return fmt.Errorf("publish %s: %w", rel, err)
	}
	return nil
}

// Discard removes a staging file that is not to be published.
func (o *outFolder) Discard(tmp string) {
	if o.dest != nil {
		if o.dest.stage(tmp) != nil {
			_ = o.dest.Discard(tmp)
		}
		return
	}
	o.mu.Lock()
	f := o.stages[tmp]
	delete(o.stages, tmp)
	o.mu.Unlock()
	if f != nil {
		f.Close()
		_ = o.root.Remove(tmp)
	}
}

// WriteFile writes data as rel with mode perm (staged, checked, published).
func (o *outFolder) WriteFile(rel string, data []byte, perm fs.FileMode) error {
	tmp, f, err := o.Create(rel)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		o.Discard(tmp)
		return err
	}
	return o.Commit(tmp, rel, perm, sha256Hex(data))
}

// CopyIn copies the file at src to rel with mode perm and returns the SHA-256 of
// what was read from src. The copy is published only once it reads back as
// exactly those bytes.
func (o *outFolder) CopyIn(src, rel string, perm fs.FileMode) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp, f, err := o.Create(rel)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), in); err != nil {
		o.Discard(tmp)
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if err := o.Commit(tmp, rel, perm, sum); err != nil {
		return "", err
	}
	return sum, nil
}

// Hash is the SHA-256 of rel, read back through the folder's handle.
func (o *outFolder) Hash(rel string) (string, error) {
	v, err := destRel(filepath.ToSlash(rel))
	if err != nil {
		return "", err
	}
	if o.dest != nil {
		return o.dest.HashRel(v)
	}
	f, err := o.root.Open(v)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(f)
}

// Stat stats rel through the folder's handle.
func (o *outFolder) Stat(rel string) (fs.FileInfo, error) {
	v, err := destRel(filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	if o.dest != nil {
		return o.dest.Stat(v)
	}
	return o.root.Stat(v)
}
