package main

// source_gate.go — a folder cannot be a source and a live output at once.
//
// A write job decides its destination against the registered sources when it
// starts, and can then run for hours. Without more, a folder that was a fair output
// at the start could be registered as a source mid-job and still be written to.
//
// The gate makes the two decisions exclusive:
//
//   - RegisterSource checks the outputs being written right now and adds the source in
//     one locked step, refusing a source that is one of them, is inside one, or holds
//     one.
//   - reserveOutput looks at the sources afresh (including any registered since the
//     output was first checked) and records the output as live only if no source was
//     added while it looked. If one was, it looks again.
//
// The lock is never held for disk work, so a source drive that stops answering
// delays only the job looking at it, and a finished job can always let go of its
// folder. Whichever request comes second is refused. Folders are compared by the object
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
	gen  uint64              // counts sources added, so a decision can tell it went stale
	busy map[string]struct{} // packages and burn queues a job is working on
}

// gateRetries is how many times a decision is made again because a source was added
// while it was being made.
const gateRetries = 8

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
	// ancestors, for a folder not there yet, is its deepest existing folder and each
	// folder above. It is kept apart from chain: a source that is one of these holds
	// the new folder, but a source beside it does not.
	ancestors []idEntry
}

// locateForGate finds what path really is.
func locateForGate(path string) (gateFolder, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return gateFolder{}, err
	}
	f, err := os.Open(abs)
	if errors.Is(err, fs.ErrNotExist) {
		where := gateFolder{canonical: abs}
		if canon := canonicalOfMissing(abs); canon != "" {
			where.canonical = canon
		}
		where.ancestors = existingAncestors(abs)
		return where, nil
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

// existingAncestors is the identity of the deepest existing folder above abs and each
// folder above that, or nil when none can be read (the real location is still compared).
func existingAncestors(abs string) []idEntry {
	dir, _, err := deepestExistingDir(abs)
	if err != nil {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	canon, err := handleCanonicalPath(f)
	if err != nil {
		return nil
	}
	id, err := handleIdentity(f)
	if err != nil {
		return nil
	}
	return gateFolderAt(canon, id).chain
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
	if len(b.chain) == 0 || b.chain[0].id.idx == 0 {
		return false
	}
	chain := a.chain
	if len(chain) == 0 {
		chain = a.ancestors
	}
	return len(chain) > 0 && chainHas(chain, b.chain[0].id)
}

// RegisterSource records path as a source of the archive, unless it overlaps a folder
// a running job is writing to. Where things really are is read before the gate is
// taken; the check against live outputs and the new record are one locked step.
func (s *Store) RegisterSource(collectionID int, path string) (*Folder, error) {
	recorded := s.folderRecorded(collectionID, path)
	var where gateFolder
	located := false
	locate := func() error {
		w, err := locateForGate(path)
		if err != nil {
			return fmt.Errorf("cannot add %s as a source: could not establish where it really is: %w", path, err)
		}
		where, located = w, true
		return nil
	}
	if !recorded {
		if err := locate(); err != nil {
			return nil, err
		}
		// A source already in the catalog stays scannable, so an older catalog with
		// a source around the data folder is not locked out of its own records.
		if err := s.refuseDataFolder(path, where); err != nil {
			return nil, err
		}
	}
	vid := pathVolumeID(path) // disk access before taking any lock
	for range gateRetries {
		if !located && s.liveOutputCount() > 0 {
			if err := locate(); err != nil {
				return nil, err
			}
		}
		// After locating: an output that exists by then is compared by identity.
		s.refreshLiveOutputs()
		f, err := s.insertSource(collectionID, path, vid, func() error {
			if !located && len(s.srcGate.live) > 0 {
				return errGateStale
			}
			for o := range s.srcGate.live {
				if o.blocks(where) {
					return fmt.Errorf("cannot add %s as a source: Obelisk is writing to %s right now, and a folder cannot be a source and a live output at once. Nothing was added. Wait for that job to finish, then scan again", path, o.display)
				}
			}
			return nil
		})
		if errors.Is(err, errGateStale) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("cannot add %s as a source: jobs kept starting while it was being checked. Nothing was added. Try again", path)
}

// errGateStale means a decision must be made again on what is true now.
var errGateStale = errors.New("the decision is out of date")

// refreshLiveOutputs reads the identity of outputs that did not exist when they were
// reserved and may exist now. The disk work is done outside the gate.
func (s *Store) refreshLiveOutputs() {
	type pending struct {
		o      *liveOutput
		target string
	}
	var list []pending
	s.srcGate.mu.Lock()
	for o := range s.srcGate.live {
		if len(o.where.chain) == 0 {
			target := o.where.canonical
			if o.file {
				target = filepath.Dir(target)
			}
			list = append(list, pending{o, target})
		}
	}
	s.srcGate.mu.Unlock()
	for _, p := range list {
		loc, err := locateForGate(p.target)
		if err != nil {
			continue
		}
		s.srcGate.mu.Lock()
		if _, live := s.srcGate.live[p.o]; live && len(p.o.where.chain) == 0 {
			p.o.where.chain, p.o.where.ancestors = loc.chain, loc.ancestors
		}
		s.srcGate.mu.Unlock()
	}
}

// folderRecorded reports whether this archive already has path as a source.
func (s *Store) folderRecorded(collectionID int, path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Folders {
		if f.CollectionID == collectionID && (f.Path == path || normPath(f.Path) == normPath(path)) {
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
		return fmt.Errorf("cannot add %s as a source: it overlaps Obelisk's own data folder %s, which Obelisk writes to all the time. Nothing was added. Choose a folder that does not include it. To back up a folder around it, such as your user folder, add the folders inside it one at a time, like Documents and Pictures", path, data.canonical)
	}
	return nil
}

// insertSource adds the catalog record if allowed (called with the gate held) says it
// may. Only RegisterSource calls it. The record is visible to every decision as soon
// as the gate is released; saving it happens after.
func (s *Store) insertSource(collectionID int, path, vid string, allowed func() error) (*Folder, error) {
	s.srcGate.mu.Lock()
	if allowed != nil {
		if err := allowed(); err != nil {
			s.srcGate.mu.Unlock()
			return nil, err
		}
	}
	s.mu.Lock()
	for _, f := range s.c.Folders { // idempotent per (collection, path)
		if f.CollectionID == collectionID && f.Path == path {
			s.mu.Unlock()
			s.srcGate.mu.Unlock()
			return f, nil
		}
	}
	f := &Folder{ID: s.next("folder"), CollectionID: collectionID, Path: path, VolumeID: vid}
	s.c.Folders = append(s.c.Folders, f)
	s.srcGate.gen++
	s.mu.Unlock()
	s.srcGate.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.save()
	return f, nil
}

// sourceGeneration is the count of sources added so far.
func (s *Store) sourceGeneration() uint64 {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	return s.srcGate.gen
}

// sourceIdentityOf reads a source's identity. A variable so a test can present one
// network share under two names.
var sourceIdentityOf = pathIdentity

// decideOutput decides the output at where against the sources as they are now. It
// does disk work, so it runs outside the gate.
func (s *Store) decideOutput(where gateFolder, display, requested, volumeID string) (*sourceSnapshot, error) {
	snap, err := s.sourceSnapshot()
	if err != nil {
		return nil, err
	}
	if err := snap.decide(where.canonical, display); err != nil {
		return nil, err
	}
	if err := snap.decideOffline(volumeID, requested, where.canonical, display); err != nil {
		return nil, err
	}
	if len(where.chain) == 0 { // not there yet: it must not become an away source's folder
		if err := snap.decideCreate(where.canonical, display); err != nil {
			return nil, err
		}
	}
	for _, src := range snap.srcs {
		if src.offline || !src.fromHandle {
			continue
		}
		id, err := sourceIdentityOf(src.canonical)
		if err != nil {
			continue // its real location was compared above
		}
		if where.overlaps(gateFolderAt(src.canonical, id)) {
			return nil, fmt.Errorf("refusing: %s overlaps registered source %s; Obelisk never writes into source data", display, src.registered)
		}
	}
	return snap, nil
}

// reserveOutput decides the output at where against the sources as they are now and
// records it as live, deciding again if a source was added meanwhile. requested is
// the path as given and volumeID its volume, for sources whose drive is away. It
// returns that decision's snapshot and the release, which is safe to call more than
// once.
func (s *Store) reserveOutput(where gateFolder, display, requested, volumeID string) (*sourceSnapshot, func(), error) {
	for range gateRetries {
		gen := s.sourceGeneration()
		snap, err := s.decideOutput(where, display, requested, volumeID)
		if err != nil {
			return nil, nil, err
		}
		o := &liveOutput{display: display, where: where}
		if release, ok := s.recordLive(gen, o); ok {
			return snap, release, nil
		}
	}
	return nil, nil, fmt.Errorf("refusing: sources kept being added while %s was being checked. Try again", display)
}

// recordLive records outputs as live if no source was added since generation gen.
func (s *Store) recordLive(gen uint64, outputs ...*liveOutput) (func(), bool) {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	if s.srcGate.gen != gen {
		return nil, false
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
	}, true
}

// claimWork marks key (a package or burn queue) as being worked on, or says it already
// is. It is held in memory, so a crash leaves nothing to clear.
func (s *Store) claimWork(key, busy string) (func(), error) {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	if _, taken := s.srcGate.busy[key]; taken {
		return nil, errors.New(busy)
	}
	if s.srcGate.busy == nil {
		s.srcGate.busy = map[string]struct{}{}
	}
	s.srcGate.busy[key] = struct{}{}
	var once sync.Once
	return func() {
		once.Do(func() {
			s.srcGate.mu.Lock()
			defer s.srcGate.mu.Unlock()
			delete(s.srcGate.busy, key)
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
// be registered that is one of them or holds one. Every folder must also really take
// writes, so one that cannot is found before any keystore is changed.
func (s *Store) reserveOutputFiles(paths []string) (func(), error) {
	for range gateRetries {
		gen := s.sourceGeneration()
		outputs := make([]*liveOutput, 0, len(paths))
		for _, p := range paths {
			if err := s.CheckFileOutsideSources(p); err != nil {
				return nil, err
			}
			o, err := fileOutputAt(p)
			if err != nil {
				return nil, fmt.Errorf("refusing: could not establish the real location of %s: %w", p, err)
			}
			outputs = append(outputs, o)
		}
		release, ok := s.recordLive(gen, outputs...)
		if !ok {
			continue
		}
		for _, p := range paths {
			abs, err := filepath.Abs(p)
			if err == nil {
				err = proveWriteAccess(filepath.Dir(abs), "a keystore")
			}
			if err != nil {
				release()
				return nil, err
			}
		}
		return release, nil
	}
	return nil, fmt.Errorf("refusing: sources kept being added while the keystores were being checked. Try again")
}

// fileOutputAt is the live output for a file written by path.
func fileOutputAt(p string) (*liveOutput, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	dir, err := locateForGate(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	return &liveOutput{display: p, file: true, where: gateFolder{canonical: filepath.Join(dir.canonical, filepath.Base(abs)),
		chain: dir.chain, ancestors: dir.ancestors}}, nil
}

// liveOutputCount is how many outputs running jobs hold.
func (s *Store) liveOutputCount() int {
	s.srcGate.mu.Lock()
	defer s.srcGate.mu.Unlock()
	return len(s.srcGate.live)
}
