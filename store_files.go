package main

// store_files.go — file records, reached only through these methods.
//
// On the JSON engine files live in s.c.Files (with fileIdx for upserts); on the
// database engine they live in the file and file_version tables and never in
// memory (catalogdb_files.go). Either way, every *File a public method returns is
// the caller's own copy: changing it changes nothing in the catalog until it is
// passed back through a method (UpsertFileErr, BackfillFileMedia, …). In tests,
// fileCopyGuard records every copy handed out and fails the run if one was changed
// and never passed back.

import (
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// fileQuery selects file records. The zero value selects every file in catalog
// order without their versions.
type fileQuery struct {
	CollectionID int       // 0 = every archive
	IDs          []int     // nil = any; otherwise only these files
	EventID      int       // > 0: only that event's files
	InEvent      bool      // only files that belong to an event
	NoEvent      bool      // only files that belong to no event
	WithHash     bool      // only files with a content hash
	WithVersions bool      // only files with retained versions
	Text         string    // lower-cased substring of the path (strings.ToLower)
	HashPrefix   string    // lower-case hex prefix of the SHA-256
	Role         string    // role, compared without case
	ShotFrom     time.Time // capture time at or after (files without one are left out)
	ShotTo       time.Time // capture time at or before (files without one are left out)
	ByPath       bool      // order by path, then catalog order; otherwise catalog order
	Versions     bool      // load each file's retained versions
	// PathsOnly fills only ID, CollectionID, FolderID and RelPath on the database
	// engine, for a scan that reads nothing else. It applies only with no filter
	// but CollectionID, IDs and ByPath; otherwise the whole record is read.
	PathsOnly bool
}

// pathsOnly reports whether q can be answered from the path columns alone.
func (q *fileQuery) pathsOnly() bool {
	return q.PathsOnly && q.EventID == 0 && !q.InEvent && !q.NoEvent && !q.WithHash && !q.WithVersions &&
		q.Text == "" && q.HashPrefix == "" && q.Role == "" && q.ShotFrom.IsZero() && q.ShotTo.IsZero() && !q.Versions
}

// errStopFiles ends a file scan early without an error.
var errStopFiles = errors.New("stop")

// filePageSize is how many files an iterator reads per page from the database.
const filePageSize = 2000

// fileMatches is the JSON engine's reading of a fileQuery; catalogdb_files.go
// writes the same rules as SQL.
func fileMatches(f *File, q *fileQuery, ids map[int]bool) bool {
	switch {
	case q.CollectionID != 0 && f.CollectionID != q.CollectionID,
		ids != nil && !ids[f.ID],
		q.EventID > 0 && f.EventID != q.EventID,
		q.InEvent && f.EventID == 0,
		q.NoEvent && f.EventID != 0,
		q.WithHash && f.Hash == "",
		q.WithVersions && len(f.Versions) == 0,
		q.Text != "" && !strings.Contains(strings.ToLower(f.RelPath), q.Text),
		q.HashPrefix != "" && !strings.HasPrefix(strings.ToLower(f.Hash), q.HashPrefix),
		q.Role != "" && !strings.EqualFold(f.Role, q.Role),
		!q.ShotFrom.IsZero() && (f.ShotAt.IsZero() || f.ShotAt.Before(q.ShotFrom)),
		!q.ShotTo.IsZero() && (f.ShotAt.IsZero() || f.ShotAt.After(q.ShotTo)):
		return false
	}
	return true
}

func idSet(ids []int) map[int]bool {
	if ids == nil {
		return nil
	}
	m := make(map[int]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// cloneFile returns a copy of f that shares nothing with it.
func cloneFile(f *File) *File {
	c := *f
	c.Versions = append([]FileVersion(nil), f.Versions...)
	return &c
}

// eachFileLocked calls fn for every file q selects. The caller holds s.mu; fn must
// not call Store methods, and must treat f as read-only (on the JSON engine it is
// the stored record). fn returning errStopFiles ends the scan without an error.
func (s *Store) eachFileLocked(q fileQuery, fn func(f *File) error) error {
	if s.be != nil {
		err := s.be.fileScan(q, fn)
		if errors.Is(err, errStopFiles) {
			return nil
		}
		return err
	}
	ids := idSet(q.IDs)
	files := s.c.Files
	if q.ByPath {
		files = make([]*File, 0, len(s.c.Files))
		for _, f := range s.c.Files {
			if fileMatches(f, &q, ids) {
				files = append(files, f)
			}
		}
		sort.SliceStable(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	}
	for _, f := range files {
		if !q.ByPath && !fileMatches(f, &q, ids) {
			continue
		}
		if err := fn(f); err != nil {
			if errors.Is(err, errStopFiles) {
				return nil
			}
			return err
		}
	}
	return nil
}

// fileLocked returns a copy of file id, or nil when there is none.
func (s *Store) fileLocked(id int) (*File, error) {
	if s.be != nil {
		return s.be.fileGet(id)
	}
	for _, f := range s.c.Files {
		if f.ID == id {
			return cloneFile(f), nil
		}
	}
	return nil, nil
}

// putFileLocked stores f whole (its versions too): a new record when isNew,
// otherwise in place of the record with its ID.
func (s *Store) putFileLocked(f *File, isNew bool) error {
	if s.be != nil {
		s.changedLocked() // the open transaction holds a change the next save commits
		return s.be.filePut(f, isNew)
	}
	s.changedLocked()
	nf := cloneFile(f)
	if isNew {
		s.c.Files = append(s.c.Files, nf)
		if s.fileIdx != nil {
			s.fileIdx[fileKey(nf.CollectionID, nf.FolderID, nf.RelPath)] = nf
		}
		return nil
	}
	for i, x := range s.c.Files {
		if x.ID == f.ID {
			s.c.Files[i] = nf
			s.fileIdx = nil // the key may have changed; rebuilt on the next upsert
			return nil
		}
	}
	return fmt.Errorf("file %d is not in the catalog", f.ID)
}

// deleteFilesLocked removes the files q selects (by IDs or archive) and returns
// how many there were.
func (s *Store) deleteFilesLocked(q fileQuery) (int, error) {
	if s.be != nil {
		s.changedLocked()
		return s.be.fileDelete(q)
	}
	s.changedLocked()
	ids := idSet(q.IDs)
	kept, n := s.c.Files[:0], 0
	for _, f := range s.c.Files {
		if fileMatches(f, &q, ids) {
			n++
		} else {
			kept = append(kept, f)
		}
	}
	for i := len(kept); i < len(s.c.Files); i++ {
		s.c.Files[i] = nil
	}
	s.c.Files = kept
	s.fileIdx = nil
	return n, nil
}

// fileIter yields a copy of every file q selects. On the database engine it reads
// a page at a time and holds the store lock only while it reads one, so the loop
// body may call Store methods; a change made meanwhile may or may not be seen. On
// the JSON engine it copies every match first, as FilesOf always has.
func (s *Store) fileIter(q fileQuery) iter.Seq2[*File, error] {
	return func(yield func(*File, error) bool) {
		if s.be == nil {
			var out []*File
			s.mu.Lock()
			err := s.eachFileLocked(q, func(f *File) error {
				out = append(out, cloneFile(f))
				return nil
			})
			s.mu.Unlock()
			if err != nil {
				yield(nil, err)
				return
			}
			for _, f := range out {
				if !yield(s.handOut(f), nil) {
					return
				}
			}
			return
		}
		var cur fileCursor
		for {
			s.mu.Lock()
			page, next, more, err := s.be.filePage(q, cur, filePageSize)
			s.mu.Unlock()
			if err != nil {
				yield(nil, err)
				return
			}
			for _, f := range page {
				if !yield(s.handOut(f), nil) {
					return
				}
			}
			if !more {
				return
			}
			cur = next
		}
	}
}

// fileCursor is where a page of files ends, in the query's order.
type fileCursor struct {
	set     bool
	relPath string
	ord     int64
}

// FilesOf yields a copy of every file of an archive, by path (then catalog
// order), each with its retained versions.
func (s *Store) FilesOf(collectionID int) iter.Seq2[*File, error] {
	return s.fileIter(fileQuery{CollectionID: collectionID, ByPath: true, Versions: true})
}

// AllFiles yields a copy of every catalog file in catalog order, each with its
// retained versions.
func (s *Store) AllFiles() iter.Seq2[*File, error] {
	return s.fileIter(fileQuery{Versions: true})
}

// collectFiles gathers what a file iterator yields, for the few callers that need
// every file at once.
func collectFiles(seq iter.Seq2[*File, error]) ([]*File, error) {
	var out []*File
	for f, err := range seq {
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// FileByID returns a copy of file id, or nil when there is none.
func (s *Store) FileByID(id int) (*File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.fileLocked(id)
	if f == nil || err != nil {
		return nil, err
	}
	return s.handOut(f), nil
}

// FilesOfEvent returns a copy of each file whose membership points at an event.
func (s *Store) FilesOfEvent(eventID int) ([]*File, error) {
	return collectFiles(s.fileIter(fileQuery{EventID: eventID, Versions: true}))
}

// FilesWithVersions returns a copy of every file that has retained at least one
// superseded version, sorted by path — the input to the Recovery Kit's
// multi-version note.
func (s *Store) FilesWithVersions() ([]*File, error) {
	return collectFiles(s.fileIter(fileQuery{WithVersions: true, ByPath: true, Versions: true}))
}

// FileStats counts the files of an archive (0 = all) and their bytes.
func (s *Store) FileStats(collectionID int) (files int, bytes int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.be != nil {
		return s.be.fileStats(collectionID)
	}
	for _, f := range s.c.Files {
		if collectionID == 0 || f.CollectionID == collectionID {
			files++
			bytes += f.SizeBytes
		}
	}
	return files, bytes, nil
}

// ExtTally counts files and bytes per extension for a collection (0 = all).
func (s *Store) ExtTally(collectionID int) (*extTally, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := newExtTally()
	err := s.eachFileLocked(fileQuery{CollectionID: collectionID}, func(f *File) error {
		t.add(f.RelPath, f.SizeBytes)
		return nil
	})
	return t, err
}

// mergeFileUpsert applies an upsert of f onto the stored record e: content that
// changed moves the outgoing version into e.Versions (capped by keep, 0 =
// unlimited), and media metadata is refreshed only when re-supplied. EventID is
// never touched: membership is the operator's, not the scanner's, to change.
func mergeFileUpsert(e *File, f File, now time.Time, keep int) {
	if e.FirstSeen.IsZero() { // backfill legacy rows so history has a start date
		e.FirstSeen = now
	}
	// Content changed → RETAIN the outgoing version instead of discarding it. The
	// old bytes are still sealed in whatever package(s) hold e.Hash; keeping the
	// record means the catalog can still point a restore at them.
	if e.Hash != "" && f.Hash != "" && e.Hash != f.Hash {
		e.Versions = append(e.Versions, FileVersion{
			Hash: e.Hash, HashAlg: e.HashAlg, SizeBytes: e.SizeBytes,
			ModTime: e.ModTime, FirstSeen: e.FirstSeen, SupersededAt: now,
		})
		if keep > 0 && len(e.Versions) > keep {
			// Forget only the OLDEST catalog pointers; the bytes remain on media.
			e.Versions = append([]FileVersion(nil), e.Versions[len(e.Versions)-keep:]...)
		}
		e.FirstSeen = now // the new current version is first seen now
	}
	contentChanged := e.Hash != f.Hash
	e.SizeBytes, e.HashAlg, e.Hash = f.SizeBytes, f.HashAlg, f.Hash
	switch {
	case f.Blake3 != "":
		e.Blake3 = f.Blake3
	case contentChanged:
		e.Blake3 = "" // the old BLAKE3 is of the old content: never left to match it
	}
	if !f.ModTime.IsZero() {
		e.ModTime = f.ModTime
	}
	if !f.ShotAt.IsZero() {
		e.ShotAt = f.ShotAt
	}
	if f.Role != "" {
		e.Role = f.Role
	}
	if f.CameraSerial != "" {
		e.CameraSerial = f.CameraSerial
	}
}

// UpsertFile is UpsertFileErr for callers that cannot take an error: a failure is
// kept, and every later save reports it (see lostErr).
func (s *Store) UpsertFile(f File) *File {
	nf, err := s.UpsertFileErr(f)
	if err != nil {
		s.mu.Lock()
		s.changedLocked()
		s.noteLostLocked(fmt.Errorf("a file change could not be recorded: %w", err))
		s.mu.Unlock()
	}
	return nf
}

// UpsertFileErr replaces the prior entry for (collection, folder, rel_path), or
// adds one, and returns a copy of the stored record.
func (s *Store) UpsertFileErr(f File) (*File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseFile(f.ID)
	now := time.Now().UTC()
	if s.be != nil {
		e, err := s.be.fileByKey(f.CollectionID, f.FolderID, f.RelPath)
		if err != nil {
			return nil, err
		}
		if e != nil {
			before := cloneFile(e)
			mergeFileUpsert(e, f, now, s.versionsRetained)
			if reflect.DeepEqual(before, e) { // a rescan of an unchanged file writes nothing
				return s.handOut(e), nil
			}
			s.changedLocked()
			if err := s.be.filePut(e, false); err != nil {
				return nil, err
			}
			return s.handOut(e), nil
		}
		nf := f
		nf.ID, nf.EventID, nf.Versions = s.next("file"), f.EventID, append([]FileVersion(nil), f.Versions...)
		if nf.FirstSeen.IsZero() {
			nf.FirstSeen = now
		}
		s.changedLocked()
		if err := s.be.filePut(&nf, true); err != nil {
			s.c.NextID["file"]--
			return nil, err
		}
		return s.handOut(cloneFile(&nf)), nil
	}
	if s.fileIdx == nil {
		s.buildFileIndexLocked()
	}
	key := fileKey(f.CollectionID, f.FolderID, f.RelPath)
	s.changedLocked()                  // a file change is a catalog change: EndBatch and FlushPending write it
	if e := s.fileIdx[key]; e != nil { // O(1) replace of a prior entry
		mergeFileUpsert(e, f, now, s.versionsRetained)
		return s.handOut(cloneFile(e)), nil
	}
	nf := f
	nf.ID = s.next("file")
	nf.Versions = append([]FileVersion(nil), f.Versions...)
	if nf.FirstSeen.IsZero() {
		nf.FirstSeen = now
	}
	s.c.Files = append(s.c.Files, &nf)
	s.fileIdx[key] = &nf
	return s.handOut(cloneFile(&nf)), nil
}

// BackfillFileMedia fills a file's capture time, role and camera serial where they
// are still empty (a scan that predates media metadata). Values already set are
// kept. It reports whether anything changed.
func (s *Store) BackfillFileMedia(id int, shotAt time.Time, role, cameraSerial string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseFile(id)
	f, err := s.fileLocked(id)
	if f == nil || err != nil {
		return false, err
	}
	changed := false
	if f.ShotAt.IsZero() && !shotAt.IsZero() {
		f.ShotAt, changed = shotAt, true
	}
	if f.Role == "" && role != "" {
		f.Role, changed = role, true
	}
	if f.CameraSerial == "" && cameraSerial != "" {
		f.CameraSerial, changed = cameraSerial, true
	}
	if !changed {
		return false, nil
	}
	if err := s.putFileLocked(f, false); err != nil {
		return false, err
	}
	return true, s.save()
}

// ---- the copy guard (tests) ----------------------------------------------

// fileCopyGuard, when set (by TestMain), records every file copy a method hands
// out, so a test run can prove that no code changes a file record without
// passing it back through a method. nil in production.
var fileCopyGuard *fileGuard

type fileGuard struct {
	mu     sync.Mutex
	copies []guardedCopy
	limit  int
}

type guardedCopy struct {
	f     *File
	sum   string
	where string
}

func newFileGuard(limit int) *fileGuard { return &fileGuard{limit: limit} }

func fileFingerprint(f *File) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// handOut records f with the guard, if one is set, and returns it.
func (s *Store) handOut(f *File) *File {
	if g := fileCopyGuard; g != nil {
		g.track(f)
	}
	return f
}

// releaseFile tells the guard that a method took file id back: copies of it may
// now differ from what they were handed out as.
func (s *Store) releaseFile(id int) {
	if g := fileCopyGuard; g != nil && id != 0 {
		g.release(id)
	}
}

func (g *fileGuard) track(f *File) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.copies) >= g.limit {
		return
	}
	g.copies = append(g.copies, guardedCopy{f: f, sum: fileFingerprint(f), where: callerOutsideStore()})
}

func (g *fileGuard) release(id int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.copies {
		if g.copies[i].f != nil && g.copies[i].f.ID == id {
			g.copies[i].sum = "" // taken back
		}
	}
}

// violations lists each copy that was changed and never passed back through a
// method, with where it was handed out.
func (g *fileGuard) violations() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for _, c := range g.copies {
		if c.sum != "" && fileFingerprint(c.f) != c.sum {
			out = append(out, fmt.Sprintf("file %d (%s), handed out at %s, was changed without a Store method",
				c.f.ID, c.f.RelPath, c.where))
		}
	}
	return out
}

// callerOutsideStore names the first caller that is not the store's own code.
func callerOutsideStore() string {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		fr, more := frames.Next()
		base := fr.File[strings.LastIndexAny(fr.File, `/\`)+1:]
		if base != "store_files.go" && base != "catalogdb_files.go" && !strings.HasPrefix(fr.Function, "iter.") {
			return fmt.Sprintf("%s:%d", base, fr.Line)
		}
		if !more {
			return "unknown"
		}
	}
}
