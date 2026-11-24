package main

// store_copies.go — which copy a write replaced and which copy a check read. A copy
// written with a copy ID (copy_id.go) is found by that ID and nothing else. A copy
// from before copy IDs is found only at the exact folder and volume it was recorded
// at; anything less certain costs it its credit until a check, and never retires it.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// priorBuild is what a package folder on a medium held before a write to it.
type priorBuild struct {
	Present     bool // the folder held some file of this package
	Note        string
	PayloadHash string          // read only when the note names no build on record
	SegHashes   map[string]bool // likewise, for a tape's segment files
	// CopyID is the ID the folder's copy ID file held, if it could be read.
	CopyID string
	// Gone is each folder of this package's copies (normPath) found missing before
	// the write. Only a copy whose own folder is gone can be the one a write to
	// another folder replaced.
	Gone map[string]bool
	// NotAPlace: the path the copy is recorded at does not name one medium (a disc
	// drive's mount, which every burned disc shares), so no copy is found by it.
	NotAPlace bool
}

func (p priorBuild) names(vals CopyValues) bool {
	return (p.Note != "" && vals.Sidecars["RESTORE.txt"] == p.Note) ||
		(p.PayloadHash != "" && vals.PayloadHash == p.PayloadHash)
}

// copyFolders is every folder and tape mount the package's current copies are
// recorded at, for a caller to look for outside the catalog lock.
func (s *Store) copyFolders(c *Chunk) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, cp := range c.Copies {
		if cp.Superseded {
			continue
		}
		out = append(out, cp.Path)
		for _, sg := range cp.Segments {
			out = append(out, sg.Dest)
		}
	}
	return out
}

// replacedByWriteLocked is which current copies (not spanned sets) a write to path
// on the volume replaces. sure: the copy whose ID the folder held, and an ID-less
// copy recorded at exactly that folder. unsure: copies that may have been there but
// cannot be told, which lose their credit and are not retired.
func (s *Store) replacedByWriteLocked(c *Chunk, volumeID int, path string, prior priorBuild) (sure, unsure []int) {
	exactLegacy := false
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded || len(cp.Segments) > 0 {
			continue
		}
		if prior.CopyID != "" && cp.CopyID == prior.CopyID {
			sure = append(sure, i)
			continue
		}
		if prior.NotAPlace || cp.VolumeID != volumeID || normPath(cp.Path) != normPath(path) {
			continue
		}
		if cp.CopyID == "" {
			sure, exactLegacy = append(sure, i), true
		} else {
			// Recorded here with an ID the folder did not hold: its medium has moved
			// or its ID file is gone.
			unsure = append(unsure, i)
		}
	}
	if prior.CopyID == "" && prior.Present && !exactLegacy && !s.isUnregisteredLocked(volumeID) {
		for i := range c.Copies {
			cp := &c.Copies[i]
			if cp.Superseded || len(cp.Segments) > 0 || cp.CopyID != "" || cp.VolumeID != volumeID {
				continue
			}
			if filepath.Base(cp.Path) == c.Name && prior.Gone[normPath(cp.Path)] {
				unsure = append(unsure, i)
			}
		}
	}
	return sure, unsure
}

// clearCopyCredit takes the verified credit from copies a write may have replaced
// when nothing says which it was. None is called failed: a full check settles it.
func clearCopyCredit(c *Chunk, idx []int) {
	for _, i := range idx {
		c.Copies[i].VerifyOK, c.Copies[i].Repairable = nil, false
	}
}

// RetireOverwrittenCopy runs once a write's payload is on the medium. The copy it
// replaced is history at once if its bytes differ; one of the same bytes is still
// good until the write is recorded (RecordWrittenCopy). A copy that may have been
// there loses its credit until checked.
func (s *Store) RetireOverwrittenCopy(c *Chunk, volumeID int, path string, prior priorBuild) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sure, unsure := s.replacedByWriteLocked(c, volumeID, path, prior)
	changed := len(unsure) > 0
	for _, i := range sure {
		if c.valuesOf(&c.Copies[i]).PayloadHash != c.EncHash {
			c.stampIfBare(&c.Copies[i])
			c.Copies[i].Superseded, changed = true, true
		}
	}
	clearCopyCredit(c, unsure)
	if changed {
		_ = s.save()
	}
}

// RecordWrittenCopy records a write of the package's current build to path on the
// volume, with the ID it wrote beside it.
//   - A failed read-back published nothing, so a copy already at that path is left
//     exactly as it was, and otherwise a failed copy is recorded.
//   - Otherwise the copy the write replaced is kept as history and the new copy is
//     added; copies that may have been there lose their credit until checked.
func (s *Store) RecordWrittenCopy(c *Chunk, volumeID int, path string, ok bool, id string, prior priorBuild) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	v := ok
	if !ok {
		for i := range c.Copies {
			cp := &c.Copies[i]
			if !cp.Superseded && cp.VolumeID == volumeID && normPath(cp.Path) == normPath(path) {
				_ = s.save()
				return
			}
		}
	} else {
		sure, unsure := s.replacedByWriteLocked(c, volumeID, path, prior)
		for _, i := range sure {
			c.stampIfBare(&c.Copies[i])
			c.Copies[i].Superseded = true
		}
		clearCopyCredit(c, unsure)
	}
	fresh := Copy{VolumeID: volumeID, Path: path, WrittenAt: &now, LastVerifiedAt: &now, VerifyOK: &v}
	if ok {
		fresh.CopyID = id
	}
	c.stampCopy(&fresh)
	c.Copies = append(c.Copies, fresh)
	_ = s.save()
}

// RecordAdoptedCopy records a medium adopted into the catalog, with the copy ID it
// holds ("" for a medium written before copy IDs).
func (s *Store) RecordAdoptedCopy(c *Chunk, volumeID int, path, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	ok := true
	fresh := Copy{VolumeID: volumeID, Path: path, WrittenAt: &now, LastVerifiedAt: &now, VerifyOK: &ok, CopyID: id}
	c.stampCopy(&fresh)
	c.Copies = append(c.Copies, fresh)
	_ = s.save()
}

// RetireOverwrittenSpan runs once a write has replaced files on a tape. A set whose
// ID the tape held is history at once if it is another build, and loses its credit if
// it is the same build: that tape will hold the new set's ID. For a tape with no ID,
// only a set from before copy IDs can have been there: the one recorded at exactly
// that mount, or the one set on that volume the tape's old contents name.
func (s *Store) RetireOverwrittenSpan(c *Chunk, volumeID int, dest string, prior priorBuild) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed, exact := false, false
	var named, others []int
	retire := func(i int) {
		if c.valuesOf(&c.Copies[i]).PayloadHash != c.EncHash {
			c.stampIfBare(&c.Copies[i])
			c.Copies[i].Superseded = true
		} else {
			clearCopyCredit(c, []int{i})
		}
		changed = true
	}
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded || len(cp.Segments) == 0 {
			continue
		}
		onVolume, here, holds, gone := false, false, false, false
		for _, sg := range cp.Segments {
			if sg.VolumeID != volumeID {
				continue
			}
			onVolume = true
			here = here || normPath(sg.Dest) == normPath(dest)
			holds = holds || (sg.Hash != "" && prior.SegHashes[sg.Hash])
			gone = gone || prior.Gone[normPath(sg.Dest)]
		}
		if prior.CopyID != "" {
			if cp.CopyID == prior.CopyID {
				retire(i)
			}
			continue
		}
		if cp.CopyID != "" {
			if here { // recorded here, but the tape held no ID: moved, or its ID file is gone
				clearCopyCredit(c, []int{i})
				changed = true
			}
			continue
		}
		if c.valuesOf(cp).PayloadHash == c.EncHash {
			continue // the same bytes, still good until the new set is recorded
		}
		switch {
		case here:
			retire(i)
			exact = true
		case !onVolume || !prior.Present:
		case holds || prior.names(c.valuesOf(cp)):
			named = append(named, i)
		case gone:
			others = append(others, i)
		}
	}
	switch {
	case exact:
	case len(named) == 1:
		retire(named[0])
	case len(named) > 1:
		clearCopyCredit(c, named)
		changed = true
	case len(others) > 0 && !s.isUnregisteredLocked(volumeID):
		clearCopyCredit(c, others)
		changed = true
	}
	if changed {
		_ = s.save()
	}
}

// RecordSpanCopy records a whole spanned set as one verified copy with its tapes and
// the set's ID. Each set it was written over is kept as history: one whose ID a tape
// held, and one from before copy IDs recorded on exactly these tapes.
func (s *Store) RecordSpanCopy(c *Chunk, volumeID int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	ok := true
	segs := append([]Segment(nil), c.Segments...)
	id := ""
	replaced := map[string]bool{}
	tapes := map[string]bool{}
	for i, sg := range segs {
		if i == 0 {
			id = sg.CopyID
		} else if sg.CopyID != id {
			id = ""
		}
		if sg.Replaced != "" {
			replaced[sg.Replaced] = true
		}
		if !s.isUnregisteredLocked(sg.VolumeID) {
			tapes[fmt.Sprintf("%d|%s", sg.VolumeID, normPath(sg.Dest))] = true
		}
	}
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded || len(cp.Segments) == 0 {
			continue
		}
		over := cp.CopyID != "" && replaced[cp.CopyID]
		if cp.CopyID == "" {
			for _, sg := range cp.Segments {
				over = over || tapes[fmt.Sprintf("%d|%s", sg.VolumeID, normPath(sg.Dest))]
			}
		}
		if over {
			c.stampIfBare(cp)
			cp.Superseded = true
		}
	}
	fresh := Copy{VolumeID: volumeID, Path: path, WrittenAt: &now, LastVerifiedAt: &now, VerifyOK: &ok, Segments: segs, CopyID: id}
	if id == "" { // some tape was written before copy IDs
		fresh.VerifyOK, fresh.Unidentified = nil, idNone
	}
	c.stampCopy(&fresh)
	c.Copies = append(c.Copies, fresh)
	_ = s.save()
}

// spanSetID is the ID the set being written carries: the one its written tapes hold,
// or "" when none is written yet.
func (s *Store) spanSetID(c *Chunk) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sg := range c.Segments {
		if (sg.Status == "WRITTEN" || sg.Status == "VERIFIED") && sg.CopyID != "" {
			return sg.CopyID
		}
	}
	return ""
}

// copyPathWithID is where the current copy with that ID is recorded, or "".
func (s *Store) copyPathWithID(c *Chunk, id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cp := range c.Copies {
		if !cp.Superseded && id != "" && cp.CopyID == id {
			return cp.Path
		}
	}
	return ""
}

// sameLocation is whether a check at at read the place l.
func sameLocation(l CopyLocation, at CopyAt) bool {
	return normPath(l.Path) == normPath(at.Path) && (l.VolumeID == 0 || at.VolumeID == 0 || l.VolumeID == at.VolumeID)
}

// CreditPackageCheck records a full check of a package folder on the copy it read,
// and returns how many copies it credited and, when none, why.
//
// A folder that holds a copy ID is the copy with that ID, wherever it is mounted now:
// a retired copy's ID credits nothing and blames nothing, and an ID another medium
// still holds is a conflict the user settles. A copy whose ID file is missing or
// unreadable earns no credit. A folder with no ID can only be a copy from before copy
// IDs recorded at exactly that folder; one found elsewhere is credited only when the
// drive is known by its serial and partition and the bytes read are that copy's.
func (s *Store) CreditPackageCheck(c *Chunk, at CopyAt, ok, repairable bool) (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	serialOf, partOf := map[int]string{}, map[int]string{}
	for _, v := range s.c.Volumes {
		serialOf[v.ID], partOf[v.ID] = v.Serial, v.PartitionID
	}
	contradicts := func(cp *Copy) bool {
		if at.VolumeID > 0 && cp.VolumeID != at.VolumeID {
			return true
		}
		if vs := serialOf[cp.VolumeID]; at.Serial != "" && vs != "" && !strings.EqualFold(vs, at.Serial) {
			return true
		}
		same, known := partitionCompare(partOf[cp.VolumeID], at.Partition)
		return known && !same
	}
	fits := func(cp *Copy) bool { return normPath(cp.Path) == normPath(at.Path) && !contradicts(cp) }
	now := time.Now().UTC()
	checked := func(i int) {
		v := ok
		c.Copies[i].LastCheckAt, c.Copies[i].LastCheckLevel, c.Copies[i].LastCheckOK = &now, VerifyB, &v
	}
	credit := func(i int) {
		checked(i)
		v := ok
		cp := &c.Copies[i]
		cp.LastVerifiedAt, cp.VerifyOK, cp.Unidentified, cp.Repairable = &now, &v, "", !ok && repairable
	}
	mark := func(i int, why string) {
		checked(i)
		cp := &c.Copies[i]
		cp.VerifyOK, cp.Unidentified, cp.Repairable = nil, why, false
	}
	done := func(n int, why string) (int, string) {
		_ = s.save()
		return n, why
	}

	if at.IDState == "ok" {
		var match []int
		retired := false
		for i := range c.Copies {
			if c.Copies[i].CopyID == at.CopyID {
				if c.Copies[i].Superseded {
					retired = true
				} else {
					match = append(match, i)
				}
			}
		}
		switch {
		case len(match) == 0 && retired:
			return 0, "this medium holds a copy that has since been replaced"
		case len(match) == 0:
			marked := false
			for i := range c.Copies {
				if cp := &c.Copies[i]; !cp.Superseded && fits(cp) {
					mark(i, idMismatch)
					marked = true
				}
			}
			if !marked {
				return 0, "this medium holds a copy ID the catalog has no record of"
			}
			return done(0, "this folder holds another copy's ID")
		case len(match) > 1:
			for _, i := range match {
				mark(i, idConflict)
			}
			return done(0, "more than one copy on record has this ID")
		}
		i := match[0]
		cp := &c.Copies[i]
		for _, l := range cp.SetAside {
			if sameLocation(l, at) {
				return 0, "this medium was set aside as a duplicate of the copy; write a new copy to it to give it its own ID"
			}
		}
		if cp.Conflict != nil {
			return 0, "two media hold this copy's ID; choose which one is the copy"
		}
		if fits(cp) {
			credit(i)
			return done(1, "")
		}
		if at.RecordedHolds || contradicts(cp) {
			cp.Conflict = &CopyLocation{VolumeID: at.VolumeID, Path: at.Path}
			mark(i, idConflict)
			return done(0, "another medium holds this copy's ID; choose which one is the copy")
		}
		cp.Path = at.Path // the drive is mounted somewhere else now
		credit(i)
		return done(1, "")
	}

	// No readable ID in the folder.
	var legacy []int
	marked := false
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded || !fits(cp) {
			continue
		}
		if cp.CopyID != "" || at.IDState == idUnreadable {
			mark(i, nonEmpty(at.IDState, idMissing))
			marked = true
		} else {
			legacy = append(legacy, i)
		}
	}
	if marked && len(legacy) == 0 {
		return done(0, "the copy ID file in this folder is "+nonEmpty(at.IDState, idMissing))
	}
	if len(legacy) == 0 {
		// A copy from before copy IDs on a drive now mounted somewhere else: only the
		// one the drive's serial and partition and the bytes read all point to.
		if !ok || at.Hash == "" || at.Serial == "" || at.Partition == "" || filepath.Base(at.Path) != c.Name {
			return 0, "no copy on record was read here"
		}
		found := -1
		for i := range c.Copies {
			cp := &c.Copies[i]
			if cp.Superseded || cp.CopyID != "" || len(cp.Segments) > 0 || (at.VolumeID > 0 && cp.VolumeID != at.VolumeID) {
				continue
			}
			vs := serialOf[cp.VolumeID]
			same, known := partitionCompare(partOf[cp.VolumeID], at.Partition)
			if vs == "" || !strings.EqualFold(vs, at.Serial) || !known || !same || c.valuesOf(cp).PayloadHash != at.Hash {
				continue
			}
			if found >= 0 {
				return 0, "the drive holds more than one copy this could be"
			}
			found = i
		}
		if found < 0 {
			return 0, "no copy on record was read here"
		}
		c.Copies[found].Path = at.Path
		credit(found)
		return done(1, "")
	}
	if len(legacy) > 1 {
		var same []int
		for _, i := range legacy {
			vals := c.valuesOf(&c.Copies[i])
			if (at.Hash != "" && vals.PayloadHash == at.Hash) || (at.Note != "" && vals.Sidecars["RESTORE.txt"] == at.Note) {
				same = append(same, i)
			}
		}
		if len(same) == 0 {
			if !ok {
				clearCopyCredit(c, legacy)
				return done(0, "more than one copy is recorded here and nothing says which was read")
			}
			return 0, "more than one copy is recorded here and nothing says which was read"
		}
		legacy = same
	}
	for _, i := range legacy {
		credit(i)
	}
	return done(len(legacy), "")
}

// SettleCopyConflict settles a copy whose ID two media hold (see App.SettleCopyConflict).
func (s *Store) SettleCopyConflict(c *Chunk, copyID, keep string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		cp := &c.Copies[i]
		if cp.Superseded || copyID == "" || cp.CopyID != copyID {
			continue
		}
		if cp.Conflict == nil {
			return fmt.Errorf("the copy with ID %s has no conflict to settle", copyID)
		}
		switch keep {
		case "recorded":
			cp.SetAside = append(cp.SetAside, *cp.Conflict)
		case "found":
			cp.SetAside = append(cp.SetAside, CopyLocation{VolumeID: cp.VolumeID, Path: cp.Path})
			cp.Path = cp.Conflict.Path
			if cp.Conflict.VolumeID > 0 {
				cp.VolumeID = cp.Conflict.VolumeID
			}
		default:
			return fmt.Errorf("keep must be \"recorded\" or \"found\", not %q", keep)
		}
		cp.Conflict, cp.Unidentified, cp.VerifyOK = nil, idUnchecked, nil
		return s.save()
	}
	return fmt.Errorf("no current copy of %s has ID %s", c.Name, copyID)
}
