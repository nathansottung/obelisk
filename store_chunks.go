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

import "fmt"

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
