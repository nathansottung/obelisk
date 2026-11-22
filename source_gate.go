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
	// file: the output is one file (a keystore). A source may sit beside it, but may
	// not be it or hold it. where.canonical is the file, and where.chain its folder
	// and each folder above.
	file bool
}

// blocks reports whether a source at src would overlap this output.
func (o *liveOutput) blocks(src gateFolder) bool {
	if !o.file {
		return src.overlaps(o.where)
	}
	s, f := normPath(src.canonical), normPath(o.where.canonical)
	if s != "" && f != "" && (s == f || canonicalWithin(s, f)) {
		return true
	}
	return o.where.heldBy(src)
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
	recorded := s.folderRecorded(collectionID, path)
	if len(s.srcGate.live) > 0 || !recorded {
		where, err := locateForGate(path)
		if err != nil {
			return nil, fmt.Errorf("cannot add %s as a source: could not establish where it really is: %w", path, err)
		}
		// A source already in the catalog stays scannable, so an older catalog with
		// a source around the data folder is not locked out of its own records.
		if !recorded {
			if err := s.refuseDataFolder(path, where); err != nil {
				return nil, err
			}
		}
		for o := range s.srcGate.live {
			if o.blocks(where) {
				return nil, fmt.Errorf("cannot add %s as a source: Obelisk is writing to %s right now, and a folder cannot be a source and a live output at once. Nothing was added. Wait for that job to finish, then scan again", path, o.display)
			}
		}
	}
	return s.addFolderRecord(collectionID, path), nil
}

// folderRecorded reports whether this archive already has path as a source.
func (s *Store) folderRecorded(collectionID int, path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Folders {
		if f.CollectionID == collectionID && f.Path == path {
			return true
		}
	}
	return false
}

// refuseDataFolder refuses a source that is Obelisk's data folder, is inside it, or
// holds it: Obelisk writes there all the time, so it would be writing into a source.
func (s *Store) refuseDataFolder(path string, where gateFolder) error {
	if s.path == "" {
		return nil
	}
	data, err := locateForGate(filepath.Dir(s.path))
	if err != nil {
		return fmt.Errorf("cannot add %s as a source: could not establish where Obelisk's data folder is: %w", path, err)
	}
	if where.overlaps(data) {
		return fmt.Errorf("cannot add %s as a source: it overlaps Obelisk's own data folder %s, which Obelisk writes to all the time. Nothing was added. Choose a folder that does not include it, or start Obelisk with -data set to a folder outside this one", path, data.canonical)
	}
	return nil
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
	if len(where.chain) == 0 { // not there yet: it must not become an away source's folder
		if err := snap.decideCreate(where.canonical, display); err != nil {
			return nil, nil, err
		}
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

// afterPathReserved is a test seam: a by-path writer calls it once its folder is held.
var afterPathReserved func(path string)

// reserveOutputPath reserves a folder or file moved or written by path rather than
// through a held handle (staging, the escrow cache, a quarantine move). It may not
// exist yet.
func (s *Store) reserveOutputPath(path string) (func(), error) {
	_, release, err := s.reserveOutputPathSnap(path)
	return release, err
}

// reserveOutputPathSnap is reserveOutputPath, also returning the sources it decided on.
func (s *Store) reserveOutputPathSnap(path string) (*sourceSnapshot, func(), error) {
	where, err := locateForGate(path)
	if err != nil {
		return nil, nil, fmt.Errorf("refusing: could not establish the real location of %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	vol := pathVolumeID(abs)
	if vol == "" {
		if ancestor, _, derr := deepestExistingDir(abs); derr == nil {
			vol = pathVolumeID(ancestor)
		}
	}
	return s.reserveOutput(where, path, abs, vol)
}

// reserveOutputPaths reserves every path or none. Empty paths are skipped.
func (s *Store) reserveOutputPaths(paths ...string) (func(), error) {
	var releases []func()
	release := func() {
		for _, r := range releases {
			r()
		}
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		r, err := s.reserveOutputPath(p)
		if err != nil {
			release()
			return nil, err
		}
		releases = append(releases, r)
	}
	return release, nil
}

// reserveOutputFiles reserves files written by path (keystores), all or none: each
// must not be, or be inside, a registered source, and until released no source may
// be registered that is one of them or holds one.
func (s *Store) reserveOutputFiles(paths []string) (func(), error) {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	outputs := make([]*liveOutput, 0, len(paths))
	for _, p := range paths {
		if err := s.CheckFileOutsideSources(p); err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		dir, err := locateForGate(filepath.Dir(abs))
		if err != nil {
			return nil, fmt.Errorf("refusing: could not establish the real location of %s: %w", p, err)
		}
		outputs = append(outputs, &liveOutput{display: p, file: true,
			where: gateFolder{canonical: filepath.Join(dir.canonical, filepath.Base(abs)), chain: dir.chain}})
	}
	if s.srcGate.live == nil {
		s.srcGate.live = map[*liveOutput]struct{}{}
	}
	for _, o := range outputs {
		s.srcGate.live[o] = struct{}{}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			s.srcGate.mu.Lock()
			defer s.srcGate.mu.Unlock()
			for _, o := range outputs {
				delete(s.srcGate.live, o)
			}
		})
	}, nil
}

// liveOutputCount is how many outputs running jobs hold.
func (s *Store) liveOutputCount() int {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	return len(s.srcGate.live)
}
