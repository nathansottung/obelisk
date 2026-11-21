package main

// source_gate.go — a folder cannot be a source and a live output at once.
//
// A write job decides its destination against the registered sources when it
// starts, and can then run for hours. Without more, a folder that was a fair output
// at the start could be registered as a source mid-job and still be written to.
//
// The gate makes the two decisions exclusive. Both happen while holding it:
//
//   - RegisterSource checks the outputs being written right now, and refuses a source
//     that is one of them, is inside one, or holds one.
//   - reserveOutput looks at the sources afresh (including any registered since the
//     output was first checked) and only then records the output as live.
//
// So whichever request comes second is refused. Folders are compared by the object
// that was opened (volume and file ID, of it and each folder above it) as well as by
// its real location, so another spelling of a folder is the same folder.
//
// RegisterSource is the only way a source enters a running catalog. A restore or a
// data folder move replaces the whole catalog, and holdForSwap refuses that while
// any output is live.
//
// This coordinates Obelisk's own writers. Changes made outside Obelisk are not seen.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// sourceGate is the lock and the set of outputs being written now.
type sourceGate struct {
	mu   sync.Mutex
	live map[*liveOutput]struct{}
}

// liveOutput is one folder a running job holds for writing.
type liveOutput struct {
	display string
	where   gateFolder
}

// gateFolder is where a folder really is. chain is the folder and each folder above
// it by identity; it is empty for a folder that does not exist yet.
type gateFolder struct {
	canonical string
	chain     []idEntry
}

// locateForGate finds what path really is.
func locateForGate(path string) (gateFolder, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return gateFolder{}, err
	}
	f, err := os.Open(abs)
	if errors.Is(err, fs.ErrNotExist) {
		if canon := canonicalOfMissing(abs); canon != "" {
			return gateFolder{canonical: canon}, nil
		}
		return gateFolder{canonical: abs}, nil
	}
	if err != nil {
		return gateFolder{}, err
	}
	defer f.Close()
	canon, err := handleCanonicalPath(f)
	if err != nil {
		return gateFolder{}, err
	}
	id, err := handleIdentity(f)
	if err != nil {
		return gateFolder{}, err
	}
	return gateFolderAt(canon, id), nil
}

// gateFolderAt is the folder at canon with identity id. A folder above it that cannot
// be read shortens the chain; the real location is still compared.
func gateFolderAt(canon string, id fileIdent) gateFolder {
	chain, _ := identityChain(canon, id)
	if len(chain) == 0 {
		chain = []idEntry{{canon, id}}
	}
	return gateFolder{canonical: canon, chain: chain}
}

// overlaps reports whether two folders are the same, or one holds the other. An
// identity with no file ID (some network shares report none) matches nothing.
func (a gateFolder) overlaps(b gateFolder) bool {
	x, y := normPath(a.canonical), normPath(b.canonical)
	if x != "" && y != "" && (x == y || canonicalWithin(x, y) || canonicalWithin(y, x)) {
		return true
	}
	return a.heldBy(b) || b.heldBy(a)
}

// heldBy reports whether b, by identity, is a or one of the folders above a.
func (a gateFolder) heldBy(b gateFolder) bool {
	if len(a.chain) == 0 || len(b.chain) == 0 || b.chain[0].id.idx == 0 {
		return false
	}
	return chainHas(a.chain, b.chain[0].id)
}

// RegisterSource records path as a source of the archive, unless it overlaps a folder
// a running job is writing to.
func (s *Store) RegisterSource(collectionID int, path string) (*Folder, error) {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	if len(s.srcGate.live) > 0 {
		where, err := locateForGate(path)
		if err != nil {
			return nil, fmt.Errorf("cannot add %s as a source while Obelisk is writing: could not establish where it really is: %w", path, err)
		}
		for o := range s.srcGate.live {
			if where.overlaps(o.where) {
				return nil, fmt.Errorf("cannot add %s as a source: Obelisk is writing to %s right now, and a folder cannot be a source and a live output at once. Nothing was added. Wait for that job to finish, then scan again", path, o.display)
			}
		}
	}
	return s.addFolderRecord(collectionID, path), nil
}

// addFolderRecord adds the catalog record. Only RegisterSource calls it.
func (s *Store) addFolderRecord(collectionID int, path string) *Folder {
	vid := pathVolumeID(path) // disk access before taking the lock
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Folders { // idempotent per (collection, path)
		if f.CollectionID == collectionID && f.Path == path {
			return f
		}
	}
	f := &Folder{ID: s.next("folder"), CollectionID: collectionID, Path: path, VolumeID: vid}
	s.c.Folders = append(s.c.Folders, f)
	_ = s.save()
	return f
}

// reserveOutput decides the output at where against the sources as they are now and
// records it as live. requested is the path as given and volumeID its volume, for
// sources whose drive is away. It returns that decision's snapshot and the release,
// which is safe to call more than once.
func (s *Store) reserveOutput(where gateFolder, display, requested, volumeID string) (*sourceSnapshot, func(), error) {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	snap, err := s.sourceSnapshot()
	if err != nil {
		return nil, nil, err
	}
	if err := snap.decide(where.canonical, display); err != nil {
		return nil, nil, err
	}
	if err := snap.decideOffline(volumeID, requested, where.canonical, display); err != nil {
		return nil, nil, err
	}
	for _, src := range snap.srcs {
		if src.offline || !src.fromHandle {
			continue
		}
		id, err := pathIdentity(src.canonical)
		if err != nil {
			continue // its real location was compared above
		}
		if where.overlaps(gateFolderAt(src.canonical, id)) {
			return nil, nil, fmt.Errorf("refusing: %s overlaps registered source %s; Obelisk never writes into source data", display, src.registered)
		}
	}
	o := &liveOutput{display: display, where: where}
	if s.srcGate.live == nil {
		s.srcGate.live = map[*liveOutput]struct{}{}
	}
	s.srcGate.live[o] = struct{}{}
	var once sync.Once
	return snap, func() {
		once.Do(func() {
			s.srcGate.mu.Lock()
			defer s.srcGate.mu.Unlock()
			delete(s.srcGate.live, o)
		})
	}, nil
}

// reserveOutputPath reserves a folder written by path rather than through a held
// handle (staging, the escrow cache). It may not exist yet.
func (s *Store) reserveOutputPath(path string) (func(), error) {
	where, err := locateForGate(path)
	if err != nil {
		return nil, fmt.Errorf("refusing: could not establish the real location of %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	vol := ""
	if ancestor, _, derr := deepestExistingDir(abs); derr == nil {
		vol = pathVolumeID(ancestor)
	}
	_, release, err := s.reserveOutput(where, path, abs, vol)
	return release, err
}

// liveOutputCount is how many outputs running jobs hold.
func (s *Store) liveOutputCount() int {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	return len(s.srcGate.live)
}
