package main

// store_chunks.go — package (chunk) file listings, reached only through these
// methods.
//
// A package's header, copies, segments and verify events are records the store
// keeps in memory on both engines. Its file listings (Files, and UnknownFiles for
// a mirror whose drive a partial scan could not fully read) can be a million
// entries: on the JSON engine they stay on the record, and on the database engine
// they live only in the chunk_file table and are read when asked for
// (catalogdb_chunks.go). Outside the store nothing reads a listing as a field,
// and nothing changes a package's copies, segments or verify events except
// through a method; TestPackages_OnlyTheStoreTouchesCatalogRecords checks both.

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// The two listings of a package.
const (
	listFiles   = 0 // the files the package holds
	listUnknown = 1 // files an earlier scan found on a mirror that a later one could not read
)

// chunkListLocked returns a package's listing. On the JSON engine it is the
// record's own slice: read it, do not change it.
func (s *Store) chunkListLocked(ch *Chunk, list int) ([]ChunkFileRef, error) {
	if s.be != nil {
		return s.be.chunkFiles(ch.ID, list)
	}
	if list == listUnknown {
		return ch.UnknownFiles, nil
	}
	return ch.Files, nil
}

// setChunkListLocked replaces a package's listing.
func (s *Store) setChunkListLocked(ch *Chunk, list int, refs []ChunkFileRef) error {
	return s.setChunkListsLocked(ch, map[int][]ChunkFileRef{list: refs})
}

// setChunkListsLocked replaces the given listings of a package together: on the
// database engine either all of them are written or none.
func (s *Store) setChunkListsLocked(ch *Chunk, lists map[int][]ChunkFileRef) error {
	if err := s.heldLocked(); err != nil {
		return err
	}
	copied := map[int][]ChunkFileRef{}
	for list, refs := range lists {
		if refs != nil {
			refs = append(make([]ChunkFileRef, 0, len(refs)), refs...) // an empty list stays empty, not nil
		}
		copied[list] = refs
	}
	s.changedLocked()
	if s.be != nil {
		return s.be.chunkFilesPut(ch, s.chunkOrdLocked(ch), copied)
	}
	for list, refs := range copied {
		if list == listUnknown {
			ch.UnknownFiles = refs
		} else {
			ch.Files = refs
		}
	}
	return nil
}

// chunksLocked is the catalog's packages, for Store methods kept outside the
// store's own files. Read the records; change them only through Store methods.
func (s *Store) chunksLocked() []*Chunk { return s.c.Chunks }

// chunkOrdLocked is a package's place in the catalog's list of packages.
func (s *Store) chunkOrdLocked(ch *Chunk) int {
	for i, c := range s.c.Chunks {
		if c == ch {
			return i
		}
	}
	return len(s.c.Chunks)
}

// listing is one entry of a package's file list, with its package.
type listing struct {
	ch  *Chunk
	ref ChunkFileRef
}

// eachListingLocked calls fn for every file listed by the packages keep selects
// (their Files, not UnknownFiles). With idsOnly, only ref.FileID is filled (the
// database engine then reads nothing else). fn must not call Store methods.
func (s *Store) eachListingLocked(keep func(*Chunk) bool, idsOnly bool, fn func(ch *Chunk, ref ChunkFileRef) error) error {
	if s.be == nil {
		for _, ch := range s.c.Chunks {
			if !keep(ch) {
				continue
			}
			for _, ref := range ch.Files {
				if err := fn(ch, ref); err != nil {
					return err
				}
			}
		}
		return nil
	}
	byID := map[int]*Chunk{}
	var ids []int
	for _, ch := range s.c.Chunks {
		if keep(ch) {
			byID[ch.ID] = ch
			ids = append(ids, ch.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return s.be.listingScan(ids, nil, idsOnly, func(chunkID int, ref ChunkFileRef) error {
		return fn(byID[chunkID], ref)
	})
}

// listingsOfFilesLocked calls fn for every package listing (Files) that names one
// of fileIDs, in any package.
func (s *Store) listingsOfFilesLocked(fileIDs []int, fn func(ch *Chunk, ref ChunkFileRef) error) error {
	if len(fileIDs) == 0 {
		return nil
	}
	if s.be == nil {
		want := idSet(fileIDs)
		for _, ch := range s.c.Chunks {
			for _, ref := range ch.Files {
				if want[ref.FileID] {
					if err := fn(ch, ref); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	byID := map[int]*Chunk{}
	for _, ch := range s.c.Chunks {
		byID[ch.ID] = ch
	}
	return s.be.listingScan(nil, fileIDs, false, func(chunkID int, ref ChunkFileRef) error {
		if ch := byID[chunkID]; ch != nil {
			return fn(ch, ref)
		}
		return nil
	})
}

// unlinkedListingsLocked calls fn for every Files listing that names no file
// (FileID 0, as a package adopted from a medium's manifest lists them) and holds
// one of hashes.
func (s *Store) unlinkedListingsLocked(hashes []string, fn func(ch *Chunk, ref ChunkFileRef) error) error {
	if len(hashes) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, h := range hashes {
		want[h] = true
	}
	if s.be == nil {
		for _, ch := range s.c.Chunks {
			for _, ref := range ch.Files {
				if ref.FileID == 0 && want[ref.Hash] {
					if err := fn(ch, ref); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	byID := map[int]*Chunk{}
	for _, ch := range s.c.Chunks {
		byID[ch.ID] = ch
	}
	return s.be.unlinkedListings(hashes, func(chunkID int, ref ChunkFileRef) error {
		if ch := byID[chunkID]; ch != nil && want[ref.Hash] {
			return fn(ch, ref)
		}
		return nil
	})
}

// ChunkFiles returns a copy of a package's file listing.
func (s *Store) ChunkFiles(chunkID int) ([]ChunkFileRef, error) {
	return s.chunkList(chunkID, listFiles)
}

// ChunkUnknownFiles returns a copy of the files an earlier scan found on a
// mirror's drive that a later, partial scan could not read.
func (s *Store) ChunkUnknownFiles(chunkID int) ([]ChunkFileRef, error) {
	return s.chunkList(chunkID, listUnknown)
}

func (s *Store) chunkList(chunkID, list int) ([]ChunkFileRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.chunkLocked(chunkID)
	if ch == nil {
		return nil, fmt.Errorf("package %d is not in the catalog", chunkID)
	}
	refs, err := s.chunkListLocked(ch, list)
	return append([]ChunkFileRef(nil), refs...), err
}

func (s *Store) chunkLocked(id int) *Chunk {
	for _, c := range s.c.Chunks {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// SetChunkFiles replaces a package's file listing and saves.
func (s *Store) SetChunkFiles(c *Chunk, files []ChunkFileRef) error {
	return s.setChunkList(c, listFiles, files)
}

// SetChunkUnknownFiles replaces the files a mirror's partial scan could not read.
func (s *Store) SetChunkUnknownFiles(c *Chunk, unknown []ChunkFileRef) error {
	return s.setChunkList(c, listUnknown, unknown)
}

// SetChunkLists replaces a package's file listing and, unless unknown is nil,
// its unknown files, together, with the header as it now is, in one save. Change
// the header's tallies before calling it, so they are saved with the listings.
func (s *Store) SetChunkLists(c *Chunk, files []ChunkFileRef, unknown *[]ChunkFileRef) error {
	lists := map[int][]ChunkFileRef{listFiles: files}
	if unknown != nil {
		lists[listUnknown] = *unknown
	}
	return s.setChunkLists(c, lists)
}

func (s *Store) setChunkList(c *Chunk, list int, refs []ChunkFileRef) error {
	return s.setChunkLists(c, map[int][]ChunkFileRef{list: refs})
}

func (s *Store) setChunkLists(c *Chunk, lists map[int][]ChunkFileRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chunkLocked(c.ID) != c {
		return fmt.Errorf("package %d is not in the catalog", c.ID)
	}
	if err := s.setChunkListsLocked(c, lists); err != nil {
		return err
	}
	return s.save()
}

// SetChunkSegments replaces a spanned package's segments and saves.
func (s *Store) SetChunkSegments(c *Chunk, segs []Segment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.Segments = append([]Segment(nil), segs...)
	return s.save()
}

// UpdateSegment changes one segment of a spanned package and saves.
func (s *Store) UpdateSegment(c *Chunk, idx int, change func(*Segment)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx < 0 || idx >= len(c.Segments) {
		return fmt.Errorf("package %s has no segment %d", c.Name, idx+1)
	}
	change(&c.Segments[idx])
	return s.save()
}

// CopyValues is what one copy on a medium was written with.
type CopyValues struct {
	KeyRef      string
	PayloadHash string
	TarHash     string
	Sidecars    map[string]string
}

// valuesOf is the values of cp: its own, or the package's for a copy that has none
// recorded.
func (c *Chunk) valuesOf(cp *Copy) CopyValues {
	if cp.PayloadHash == "" && cp.KeyRef == "" {
		return CopyValues{KeyRef: c.KeyRef, PayloadHash: c.EncHash, TarHash: c.TarHash, Sidecars: c.Sidecars}
	}
	return CopyValues{KeyRef: cp.KeyRef, PayloadHash: cp.PayloadHash, TarHash: cp.TarHash, Sidecars: cp.Sidecars}
}

// stampCopy records the package's current values on cp.
func (c *Chunk) stampCopy(cp *Copy) {
	cp.KeyRef, cp.PayloadHash, cp.TarHash = c.KeyRef, c.EncHash, c.TarHash
	cp.Sidecars = cloneSidecars(c.Sidecars)
}

// stampIfBare records the package's values on a copy that has none yet.
func (c *Chunk) stampIfBare(cp *Copy) {
	if cp.PayloadHash == "" && cp.KeyRef == "" {
		c.stampCopy(cp)
	}
}

func cloneSidecars(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// keyRefs is every key a copy of this package or its current build needs, the
// current build's first.
func (c *Chunk) keyRefs() []string {
	var out []string
	seen := map[string]bool{}
	add := func(ref string) {
		if ref != "" && !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	add(c.KeyRef)
	for i := range c.Copies {
		add(c.valuesOf(&c.Copies[i]).KeyRef)
	}
	return out
}

// spanSets is each tape set this package has on record as a whole verified copy,
// plus its current segments when no copy records them.
func (c *Chunk) spanSets() [][]Segment {
	var sets [][]Segment
	for i := range c.Copies {
		if cp := &c.Copies[i]; !cp.Superseded && len(cp.Segments) > 0 {
			sets = append(sets, cp.Segments)
		}
	}
	if len(sets) == 0 && len(c.Segments) > 0 {
		sets = append(sets, c.Segments)
	}
	return sets
}

// withSegments is a private view of the package with the given tape layout. The
// catalog's record is not changed.
func (c *Chunk) withSegments(segs []Segment) *Chunk {
	view := *c
	view.Segments = segs
	return &view
}

// CopyKeyRefs is every key any copy on record needs, sorted.
func (s *Store) CopyKeyRefs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	for _, c := range s.c.Chunks {
		for i := range c.Copies {
			if ref := c.valuesOf(&c.Copies[i]).KeyRef; ref != "" {
				seen[ref] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for ref := range seen {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// pinCopyValues writes the package's current values onto any copy without its own,
// before a build changes them.
func (s *Store) pinCopyValues(c *Chunk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := range c.Copies {
		if cp := &c.Copies[i]; cp.PayloadHash == "" && cp.KeyRef == "" && (c.EncHash != "" || c.KeyRef != "") {
			c.stampCopy(cp)
			changed = true
		}
	}
	if changed {
		_ = s.save()
	}
}

// copyValuesAt is the values of the copy a check at at reads, if one fits.
func (s *Store) copyValuesAt(c *Chunk, at CopyAt) (CopyValues, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fit, _ := s.copiesReadLocked(c, at)
	if len(fit) == 0 {
		return CopyValues{}, false
	}
	return c.valuesOf(&c.Copies[fit[0]]), true
}

// NoteCopyRepairable records whether the damage a full check just found on the copy
// at at can be repaired by its par2 set.
func (s *Store) NoteCopyRepairable(c *Chunk, at CopyAt, repairable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fit, _ := s.copiesReadLocked(c, at)
	for _, i := range fit {
		c.Copies[i].Repairable = repairable
	}
	if len(fit) > 0 {
		_ = s.save()
	}
}

// RetireOverwrittenCopy marks as history a current copy at path on the volume whose
// bytes a write has just replaced with different ones. A copy of the same bytes is
// still good and is left alone.
func (s *Store) RetireOverwrittenCopy(c *Chunk, volumeID int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.VolumeID == volumeID && !cp.Superseded && normPath(cp.Path) == normPath(path) &&
			c.valuesOf(cp).PayloadHash != c.EncHash {
			c.stampIfBare(cp)
			cp.Superseded = true
			_ = s.save()
			return
		}
	}
}

// SupersedeCopyAt keeps the current copy at path on the volume as history, for a
// re-write that has finished and is about to record its replacement.
func (s *Store) SupersedeCopyAt(c *Chunk, volumeID int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.VolumeID == volumeID && !cp.Superseded && normPath(cp.Path) == normPath(path) {
			c.stampIfBare(cp)
			cp.Superseded = true
			_ = s.save()
			return
		}
	}
}

// RetireOverwrittenSpan marks as history a current spanned copy whose segment index
// on this tape a write has just replaced with different bytes.
func (s *Store) RetireOverwrittenSpan(c *Chunk, volumeID int, dest string, index int, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded {
			continue
		}
		for _, sg := range cp.Segments {
			if sg.Index == index && sg.VolumeID == volumeID && normPath(sg.Dest) == normPath(dest) && sg.Hash != hash {
				cp.Superseded = true
				_ = s.save()
				return
			}
		}
	}
}

// RecordSpanCopy records a whole spanned set as one verified copy with its tapes. The
// same set written again refreshes its record; any other set is a new copy.
func (s *Store) RecordSpanCopy(c *Chunk, volumeID int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	ok := true
	segs := append([]Segment(nil), c.Segments...)
	for i := range c.Copies {
		cp := &c.Copies[i]
		if !cp.Superseded && c.valuesOf(cp).PayloadHash == c.EncHash && (len(cp.Segments) == 0 || reflect.DeepEqual(cp.Segments, segs)) {
			cp.VolumeID, cp.Path, cp.LastVerifiedAt, cp.VerifyOK, cp.Segments, cp.Repairable = volumeID, path, &now, &ok, segs, false
			c.stampCopy(cp)
			_ = s.save()
			return
		}
	}
	fresh := Copy{VolumeID: volumeID, Path: path, WrittenAt: &now, LastVerifiedAt: &now, VerifyOK: &ok, Segments: segs}
	c.stampCopy(&fresh)
	c.Copies = append(c.Copies, fresh)
	_ = s.save()
}

// verifiedCopyOn is a snapshot of the verified current copy on the volume.
func (s *Store) verifiedCopyOn(c *Chunk, volumeID int) (Copy, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		cp := c.Copies[i]
		if cp.VolumeID == volumeID && !cp.Superseded && cp.VerifyOK != nil && *cp.VerifyOK {
			cp.KeyRef, cp.PayloadHash, cp.TarHash = c.valuesOf(&cp).KeyRef, c.valuesOf(&cp).PayloadHash, c.valuesOf(&cp).TarHash
			cp.Sidecars = cloneSidecars(c.valuesOf(&c.Copies[i]).Sidecars)
			cp.Segments = append([]Segment(nil), cp.Segments...)
			return cp, true
		}
	}
	return Copy{}, false
}

// RefilledFrom makes the package describe staging refilled from a copy with values
// vals. A spanned package gets that set's layout, with every tape still to write.
func (s *Store) RefilledFrom(c *Chunk, vals CopyValues, encBytes int64, stagedDir string, segs []Segment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.KeyRef, c.EncHash, c.TarHash, c.EncBytes = vals.KeyRef, vals.PayloadHash, vals.TarHash, encBytes
	c.Sidecars = cloneSidecars(vals.Sidecars)
	c.StagedDir = stagedDir
	for name := range vals.Sidecars {
		if strings.HasSuffix(name, ".manifest.json.gpg") {
			c.PrivateManifest = true
		}
	}
	if len(segs) > 0 {
		next := make([]Segment, len(segs))
		for i, sg := range segs {
			next[i] = Segment{Index: sg.Index, Bytes: sg.Bytes, Status: "PENDING", Par2: sg.Par2}
		}
		c.Segments = next
	}
	return s.save()
}

// dropNotedTapes removes the records noteOldTapes added, once the segments that
// hold the same locations are back.
func (s *Store) dropNotedTapes(c *Chunk, from, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > 0 && from+n <= len(c.Copies) {
		c.Copies = append(c.Copies[:from:from], c.Copies[from+n:]...)
	}
}

// noteOldTapes records where each tape of a spanned set is before a build replaces
// the segments that say so. It runs first so a crash during the build loses none.
func (s *Store) noteOldTapes(c *Chunk, oldSegments []Segment) (added int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	known := map[string]bool{}
	for _, cp := range c.Copies {
		known[fmt.Sprintf("%d|%s", cp.VolumeID, normPath(cp.Path))] = true
		for _, sg := range cp.Segments { // a whole set is already on record as a copy
			known[fmt.Sprintf("%d|%s", sg.VolumeID, normPath(sg.Dest))] = true
		}
	}
	for _, sg := range oldSegments {
		key := fmt.Sprintf("%d|%s", sg.VolumeID, normPath(sg.Dest))
		if (sg.VolumeID == 0 && sg.Dest == "") || known[key] {
			continue
		}
		known[key] = true
		noted := Copy{VolumeID: sg.VolumeID, Path: sg.Dest, Superseded: true}
		c.stampCopy(&noted)
		c.Copies = append(c.Copies, noted)
		added++
	}
	return added
}

// fileCopiesLocked maps each file listed by the packages keep selects, among those
// not FAILED, to the verified physical copies its packages have (chunkPhysCopies).
func (s *Store) fileCopiesLocked(vols map[int]*Volume, locs map[int]*Location, keep func(*Chunk) bool) (map[int]map[string]physCopy, error) {
	pcsOf := map[int]map[string]physCopy{}
	for _, ch := range s.c.Chunks {
		if ch.Status == "FAILED" || !keep(ch) {
			continue
		}
		if pcs := chunkPhysCopies(ch, vols, locs); len(pcs) > 0 {
			pcsOf[ch.ID] = pcs
		}
	}
	fileCopies := map[int]map[string]physCopy{}
	err := s.eachListingLocked(func(ch *Chunk) bool { return pcsOf[ch.ID] != nil }, true, func(ch *Chunk, ref ChunkFileRef) error {
		m := fileCopies[ref.FileID]
		if m == nil {
			m = map[string]physCopy{}
			fileCopies[ref.FileID] = m
		}
		for sig, pc := range pcsOf[ch.ID] {
			m[sig] = pc
		}
		return nil
	})
	return fileCopies, err
}

// PackageRecord returns a copy of a package with both of its listings filled in,
// as the package's own page shows it (nil when there is no such package).
func (s *Store) PackageRecord(id int) (*Chunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.chunkLocked(id)
	if ch == nil {
		return nil, nil
	}
	out := *ch
	files, err := s.chunkListLocked(ch, listFiles)
	if err != nil {
		return nil, err
	}
	unknown, err := s.chunkListLocked(ch, listUnknown)
	if err != nil {
		return nil, err
	}
	out.Files = append([]ChunkFileRef{}, files...)
	out.UnknownFiles = append([]ChunkFileRef(nil), unknown...)
	return &out, nil
}

// PackageHeaders returns copies of an archive's packages (every package for 0)
// without their listings, which can hold a million entries; PackageRecord and
// ChunkFiles read one package's.
func (s *Store) PackageHeaders(collectionID int) []*Chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Chunk
	for _, ch := range s.c.Chunks {
		if collectionID != 0 && ch.CollectionID != collectionID {
			continue
		}
		c := *ch
		c.Files, c.UnknownFiles = nil, nil
		out = append(out, &c)
	}
	return out
}
