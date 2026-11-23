package main

// source_lookup.go — reading where the registered sources really are.
//
// Each source is read on its own, with a time limit, so one drive or share that stops
// answering cannot hold up a job writing somewhere else. A source that does not answer
// in time is decided on what was last read about it. With nothing read yet and no
// recorded volume there is nothing to decide on, and the job is refused.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// sourceLookupLimit is how long a decision waits for one source to answer.
var sourceLookupLimit = 15 * time.Second

// sourceKey names a source record. The path is part of it because a restored catalog
// can use the same ID for another folder.
type sourceKey struct {
	id   int
	path string
}

// sourceLookup is one read of a source. Decisions made while it runs share it, so a
// stalled drive holds one goroutine however many jobs ask.
type sourceLookup struct {
	started time.Time
	done    chan struct{}
	refs    []sourceRef
	err     error
}

// sourceSnapshot resolves every registered source root to a canonical name,
// preferring the identity of the OPEN OBJECT over the spelling of its name.
//
// A root that cannot be opened for a reason other than "does not exist" is a
// refusal: authorizing a destination while a source location is unresolved would
// be authorizing on a guess. A root that does not exist is one of three things:
//
//   - its folder was removed but its volume is still there: resolve the existing
//     ancestor and protect the path the source would occupy;
//   - its volume is not attached (no ancestor exists, or a different volume now
//     answers at that path): it cannot hold the destination, so it is offline and
//     only blocks a destination on the volume it was recorded on;
//   - its volume identity was never recorded and nothing answers at its path: it
//     is offline and blocks only a destination whose path could be its path.
func (s *Store) sourceSnapshot() (*sourceSnapshot, error) {
	folders := s.sourceFolders() // takes and RELEASES the Store mutex
	snap := &sourceSnapshot{at: time.Now().UTC()}
	limit := sourceLookupLimit
	lookups := make([]*sourceLookup, len(folders))
	for i, folder := range folders {
		lookups[i] = s.startSourceLookup(folder)
	}
	for i, l := range lookups {
		timer := time.NewTimer(time.Until(l.started.Add(limit)))
		select {
		case <-l.done:
			timer.Stop()
			if l.err != nil {
				return nil, l.err
			}
			snap.srcs = append(snap.srcs, l.refs...)
		case <-timer.C:
			refs, err := s.stalledSourceRefs(folders[i])
			if err != nil {
				return nil, err
			}
			snap.srcs = append(snap.srcs, refs...)
		}
	}
	return snap, nil
}

// startSourceLookup reads a source in the background, or joins the read under way.
func (s *Store) startSourceLookup(folder Folder) *sourceLookup {
	key := sourceKey{folder.ID, folder.Path}
	s.srcGate.mu.Lock()
	if l, ok := s.srcGate.lookups[key]; ok {
		s.srcGate.mu.Unlock()
		return l
	}
	if s.srcGate.lookups == nil {
		s.srcGate.lookups = map[sourceKey]*sourceLookup{}
		s.srcGate.lastRead = map[sourceKey][]sourceRef{}
	}
	l := &sourceLookup{started: time.Now(), done: make(chan struct{})}
	s.srcGate.lookups[key] = l
	s.srcGate.mu.Unlock()
	open, identity := openSourceRoot, sourceIdentityOf
	go func() {
		refs, err := s.resolveSource(folder, open, identity)
		s.srcGate.mu.Lock()
		delete(s.srcGate.lookups, key)
		if err == nil {
			s.srcGate.lastRead[key] = refs
		}
		s.srcGate.mu.Unlock()
		l.refs, l.err = refs, err
		close(l.done)
	}()
	return l
}

// stalledSourceRefs is what is known about a source that did not answer in time.
func (s *Store) stalledSourceRefs(folder Folder) ([]sourceRef, error) {
	s.srcGate.mu.Lock()
	refs, ok := s.srcGate.lastRead[sourceKey{folder.ID, folder.Path}]
	s.srcGate.mu.Unlock()
	if ok {
		return refs, nil
	}
	if folder.VolumeID != "" {
		abs, err := filepath.Abs(folder.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve registered source %s: %w", folder.Path, err)
		}
		return []sourceRef{{registered: folder.Path, offline: true, volumeID: folder.VolumeID, abs: abs, unavailable: true}}, nil
	}
	return nil, fmt.Errorf("refusing: registered source %s is not answering, and Obelisk has no record of where it is. Nothing was changed. Reconnect it or wait for it, then try again", folder.Path)
}

// resolveSource reads where one source really is.
func (s *Store) resolveSource(folder Folder, open func(string) (*os.File, error), identity func(string) (fileIdent, error)) ([]sourceRef, error) {
	r := folder.Path
	abs, err := filepath.Abs(r)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve registered source %s: %w", r, err)
	}
	f, oerr := open(abs)
	if oerr == nil {
		c, cerr := handleCanonicalPath(f)
		vid, verr := volumeIDOf(f)
		f.Close()
		if cerr != nil {
			return nil, fmt.Errorf("cannot establish the real location of registered source root %s: %w — refusing to authorize a destination while a protected source is unresolved", r, cerr)
		}
		ref := sourceRef{registered: r, canonical: c, fromHandle: true}
		if id, err := identity(c); err == nil { // if not, its real location is still compared
			g := gateFolderAt(c, id)
			ref.gate = &g
		}
		refs := []sourceRef{ref}
		switch {
		case verr != nil || vid == "":
		case folder.VolumeID == "":
			s.recordFolderVolume(folder.ID, vid)
		case vid != folder.VolumeID:
			// The path now answers on a different drive than the one the source
			// was recorded on. That drive may be attached elsewhere, still holding
			// the source: protect it too.
			refs = append(refs, sourceRef{registered: r, offline: true, volumeID: folder.VolumeID, abs: abs})
		}
		return refs, nil
	}
	// A card reader with no card or an unreachable share: the source is away, so
	// it blocks only a destination that could be it (decideOffline).
	// With no recorded volume, an unreachable share could be the same place under
	// another name, so that case still refuses below.
	if sourceUnavailable(oerr) && (folder.VolumeID != "" || !sourceNetworkError(oerr)) {
		ref := sourceRef{registered: r, offline: true, volumeID: folder.VolumeID, abs: abs, unavailable: true}
		if parent := canonicalOfMissing(filepath.Dir(abs)); parent != "" {
			ref.viaParent = filepath.Join(parent, filepath.Base(abs))
		}
		return []sourceRef{ref}, nil
	}
	if !errors.Is(oerr, fs.ErrNotExist) {
		return nil, fmt.Errorf("cannot open registered source root %s to establish its real location: %w — refusing to authorize a destination on a guess", r, oerr)
	}
	offline := sourceRef{registered: r, offline: true, volumeID: folder.VolumeID, abs: abs}
	ancestor, missing, err := deepestExistingDir(abs)
	if errors.Is(err, errNoExistingParent) {
		return []sourceRef{offline}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registered source %s is unresolved: %w; restore access to its parent or repair the alias before retrying", r, err)
	}
	if folder.VolumeID != "" {
		if vid := pathVolumeID(ancestor); vid != "" && vid != folder.VolumeID {
			return []sourceRef{offline}, nil
		}
	}
	parent, err := os.OpenRoot(ancestor)
	if err != nil {
		return nil, fmt.Errorf("cannot open ancestor of registered source %s: %w", r, err)
	}
	canon, err := rootCanonicalPath(parent)
	parent.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot locate ancestor of registered source %s: %w", r, err)
	}
	canon = filepath.Join(append([]string{canon}, missing...)...)
	return []sourceRef{{registered: r, canonical: canon}}, nil
}
