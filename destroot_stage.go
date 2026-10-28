package main

// destroot_stage.go — staging and publication under an authorized DestRoot.
//
// Every new file is created as a uniquely named, exclusively owned staging object
// in its final folder, written and verified through the handle that created it,
// and only then renamed to its real name. A failed publication never deletes the
// existing destination, and a staged file is never reopened by name.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

// A stage retains the file used for write, metadata and verification until
// publication/cleanup. Platform code supplies the ownership/publication mechanism.
type destStage struct {
	file      *os.File
	parent    *os.Root
	private   *os.Root // Unix: mode-0700 namespace, retained independently of its name
	container string   // Unix: that namespace's name in parent
	perms     bool     // Unix: the filesystem enforces the private namespace's owner and mode
	rel       string
	leaf      string
	finalDir  string
	identity  fileIdent
	published bool
	expected  string // supplied by the source, never adopted from a replacement
}

// stagingFor creates without following an existing leaf and retains the object.
// The returned file is borrowed: Publish/Discard, not the caller, closes it.
// See deststage_windows.go and deststage_unix.go for the different ownership tiers.
func (d *DestRoot) stagingFor(rel string) (string, *os.File, error) {
	v, err := destRel(rel)
	if err != nil {
		return "", nil, err
	}
	dir := path.Dir(v)
	parent, err := d.root.OpenRoot(dir)
	if err != nil {
		return "", nil, err
	}
	if err := d.checkParent(parent); err != nil {
		parent.Close()
		return "", nil, err
	}
	name := fmt.Sprintf(".%s-%d%s", d.token, d.seq.Add(1), mirrorTmpSuffix)
	stage, err := createDestStage(parent, name)
	if err != nil {
		parent.Close()
		return "", nil, err
	}
	stage.parent = parent
	stage.finalDir = dir
	stage.rel = path.Join(dir, stage.leaf)
	stage.identity, err = handleIdentity(stage.file)
	if err != nil {
		_ = discardDestStage(stage)
		parent.Close()
		return "", nil, err
	}
	d.mu.Lock()
	if d.stages == nil {
		d.stages = make(map[string]*destStage)
	}
	d.stages[stage.rel] = stage
	d.mu.Unlock()
	return stage.rel, stage.file, nil
}

// Deterministic fault/substitution seam at the pre-publication boundary. It does
// not perform the publication itself.
var beforeDestPublish = func(r *os.Root, oldname, newname string) error { return nil }

// Deterministic test seam before sidecar metadata is applied to the owned file.
var beforeDestMetadata = func(r *os.Root, staging string) error { return nil }

func (d *DestRoot) stage(rel string) *destStage {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stages[rel]
}

// Publish renames staging into its final name, both relative to the authorized
// root, replacing whatever is already there in a single filesystem operation.
//
// It NEVER deletes the destination as a fallback. A failed rename preserves the
// prior destination; a close error AFTER a successful rename withholds catalog
// credit but leaves the published file in place. Windows renames the owned file
// handle; Unix uses a private source directory and renameat.
//
// It also does not become a truncating write. Staging plus rename is what keeps a
// destination leaf that is a hard link to a protected original intact — the rename
// replaces the NAME, leaving the shared object untouched.
func (d *DestRoot) Publish(tmpRel, finalRel string) error {
	return d.publish(tmpRel, finalRel, true)
}

// PublishNew is Publish for a name that must not exist yet: the rename itself
// refuses an existing entry (file, folder or link) in the same filesystem
// operation, so nothing already at finalRel is ever replaced, even one created
// a moment before. That refusal is reported as an error wrapping fs.ErrExist.
func (d *DestRoot) PublishNew(tmpRel, finalRel string) error {
	return d.publish(tmpRel, finalRel, false)
}

func (d *DestRoot) publish(tmpRel, finalRel string, replace bool) error {
	t, err := destRel(tmpRel)
	if err != nil {
		return err
	}
	f, err := destRel(finalRel)
	if err != nil {
		return err
	}
	stage := d.stage(t)
	if stage == nil {
		return fmt.Errorf("publish %s: staging object is not owned by this operation", finalRel)
	}
	if stage.finalDir != path.Dir(f) {
		return fmt.Errorf("publish %s: staging parent differs", finalRel)
	}
	if err := beforeDestPublish(d.root, t, f); err != nil {
		return fmt.Errorf("publish %s: %w", finalRel, err)
	}
	if stage.expected == "" {
		return fmt.Errorf("publish %s: staging has no verified source digest", finalRel)
	}
	if err := d.VerifyStage(t, stage.expected); err != nil {
		return err
	}
	if err := d.checkParent(stage.parent); err != nil {
		return err
	}
	if err := publishDestStage(stage, path.Base(f), replace); err != nil {
		if !replace && errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("publish %s: %w: Obelisk never replaces an existing file here", finalRel, fs.ErrExist)
		}
		return fmt.Errorf("publish %s: %w", finalRel, err)
	}
	stage.published = true
	// Closing cannot change which object the publication moved. A close error
	// still prevents catalog credit; never delete the just-published destination.
	return d.Discard(t)
}

// VerifyStage retains the caller's expected digest. Rechecking immediately before
// publication catches observed changes; it is not isolation from a writer with
// authority to change the file again after the check.
func (d *DestRoot) VerifyStage(rel, expected string) error {
	stage := d.stage(rel)
	if stage == nil {
		return fmt.Errorf("verify: staging object is not owned")
	}
	if stage.expected != "" && stage.expected != expected {
		return fmt.Errorf("verify: expected source digest changed")
	}
	if expected == "" {
		return fmt.Errorf("verify: expected source digest is empty")
	}
	stage.expected = expected // retain even when this read-back fails
	if _, err := stage.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	got, err := hashReader(stage.file)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("staging content mismatch: refusing unverified publication")
	}
	return nil
}

// Discard releases an owned stage. Unowned paths are never removed. Unix leaves
// an empty private container rather than removing a potentially substituted
// public directory entry. Windows cleanup marks the retained file for deletion.
func (d *DestRoot) Discard(rel string) error {
	v, err := destRel(rel)
	if err != nil {
		return err
	}
	d.mu.Lock()
	stage := d.stages[v]
	delete(d.stages, v)
	d.mu.Unlock()
	if stage == nil {
		return fmt.Errorf("discard %s: staging object is not owned by this operation", rel)
	}
	err = discardDestStage(stage)
	return errors.Join(err, stage.parent.Close())
}

// discardAll releases every stage still owned, for Close.
func (d *DestRoot) discardAll() error {
	d.mu.Lock()
	rels := make([]string, 0, len(d.stages))
	for rel := range d.stages {
		rels = append(rels, rel)
	}
	d.mu.Unlock()
	var err error
	for _, rel := range rels {
		err = errors.Join(err, d.Discard(rel))
	}
	return err
}

// WriteFileAtomic writes a whole small file (a sidecar) beneath the authorized
// root with the same discipline as a payload: staged into a uniquely owned file,
// then published by rename. Metadata and read-back use the same open file as the
// write.
func (d *DestRoot) WriteFileAtomic(rel string, data []byte, perm fs.FileMode) error {
	v, err := destRel(rel)
	if err != nil {
		return err
	}
	if dir := path.Dir(v); dir != "." {
		if err := d.MkdirAll(dir); err != nil {
			return err
		}
	}
	tmpRel, f, err := d.stagingFor(v)
	if err != nil {
		return err
	}
	defer func() {
		if d.stage(tmpRel) != nil {
			_ = d.Discard(tmpRel)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := beforeDestMetadata(d.root, tmpRel); err != nil {
		return err
	}
	// Metadata belongs to the open object, never whichever file has its name.
	if err := f.Chmod(perm); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := d.VerifyStage(tmpRel, sha256Hex(data)); err != nil {
		return fmt.Errorf("sidecar read-back failed for %s: %v", rel, err)
	}
	return d.Publish(tmpRel, v)
}
