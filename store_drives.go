package main

// store_drives.go — drive snapshots, scan records, comparison records and drift
// reports, reached only through these methods.
//
// A drive snapshot can list a million files, and a drift report every changed
// file of an archive. On the JSON engine those lists stay on their records; on
// the database engine they live only in the snap_file and drift_item tables
// (catalogdb_drives.go), and only each record's header is in memory. Every
// record a method returns is the caller's own copy, without its list: a
// snapshot's files are read a page at a time with SnapshotFiles, and a report's
// items with DriftItems. Scan and comparison records are small and capped, and
// are kept in memory on both engines; their file lists and results stay as
// compressed files in the data folder. Outside the store nothing reaches these
// records or lists as fields; TestPackages_OnlyTheStoreTouchesCatalogRecords
// checks it.

import (
	"errors"
	"fmt"
	"iter"
)

// errSnapshotChanged is the error SnapshotFiles ends with when the snapshot it is
// reading is replaced (the drive scanned again) between two pages.
var errSnapshotChanged = errors.New("changed while it was being read")

// retrySnapshotRead runs read, and runs it again from the start (three tries in
// all) while it fails only because a drive snapshot it was reading was replaced.
// read must keep nothing from a try that fails. Views that only read use it, so a
// drive scanned while one is shown does not fail the view.
func retrySnapshotRead[T any](read func() (T, error)) (T, error) {
	var v T
	var err error
	for try := 0; try < 3; try++ {
		if v, err = read(); !errors.Is(err, errSnapshotChanged) {
			return v, err
		}
	}
	return v, err
}

// snapFilePageSize is how many snapshot files SnapshotFiles reads per page (a
// variable so tests can read in small pages).
var snapFilePageSize = 5000

// headerOfSnapshot is a copy of a drive snapshot without its files.
func headerOfSnapshot(s *VolumeSnapshot) *VolumeSnapshot {
	c := *s
	c.Files = nil
	if s.Smart != nil {
		sm := *s.Smart
		c.Smart = &sm
	}
	c.UnreadableFolders = append([]ScanGap(nil), s.UnreadableFolders...)
	c.UnreadableFiles = append([]ScanGap(nil), s.UnreadableFiles...)
	c.SkippedFolders = append([]ScanGap(nil), s.SkippedFolders...)
	if s.RoleFiles != nil {
		c.RoleFiles = make(map[string]int, len(s.RoleFiles))
		for k, v := range s.RoleFiles {
			c.RoleFiles[k] = v
		}
	}
	if s.RoleBytes != nil {
		c.RoleBytes = make(map[string]int64, len(s.RoleBytes))
		for k, v := range s.RoleBytes {
			c.RoleBytes[k] = v
		}
	}
	return &c
}

// canonicalHash reports whether h is a content hash as the catalog stores it:
// exactly 64 lowercase hexadecimal characters.
func canonicalHash(h string) bool {
	if len(h) != 64 {
		return false
	}
	for i := 0; i < len(h); i++ {
		if c := h[i]; !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// checkSnapshotHashes refuses a snapshot file whose hash is neither empty nor 64
// lowercase hexadecimal characters, on both engines alike.
func checkSnapshotHashes(snap *VolumeSnapshot) error {
	for _, f := range snap.Files {
		for _, h := range []string{f.Hash, f.Blake3} {
			if h != "" && !canonicalHash(h) {
				return fmt.Errorf("the snapshot of volume %d lists %s with a hash that is not 64 lowercase hexadecimal characters (%q)",
					snap.VolumeID, f.RelPath, h)
			}
		}
	}
	return nil
}

func (s *Store) snapshotLocked(volumeID int) (*VolumeSnapshot, int) {
	for i, snap := range s.c.Snapshots {
		if snap.VolumeID == volumeID {
			return snap, i
		}
	}
	return nil, len(s.c.Snapshots)
}

// PutVolumeSnapshot stores (or replaces) the offline inventory for a volume, files
// and all. One snapshot per volume, keyed by VolumeID — a re-ingest overwrites in
// place; earlier scans are kept as ScanRecords. The store keeps its own copy, so
// the caller may drop or change snap afterwards. Persists (coalesced under an
// active batch) and returns the save's error.
func (s *Store) PutVolumeSnapshot(snap *VolumeSnapshot) error {
	if err := checkSnapshotHashes(snap); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := headerOfSnapshot(snap)
	_, ord := s.snapshotLocked(snap.VolumeID)
	if s.be != nil {
		s.dirty = true
		if err := s.be.snapshotPut(rec, ord, snap.Files); err != nil {
			return err
		}
	} else if snap.Files != nil {
		rec.Files = append(make([]SnapFile, 0, len(snap.Files)), snap.Files...)
	}
	if ord < len(s.c.Snapshots) {
		s.c.Snapshots[ord] = rec
	} else {
		s.c.Snapshots = append(s.c.Snapshots, rec)
	}
	return s.save()
}

// VolumeSnapshot returns a copy of a volume's offline inventory without its
// files (read them with SnapshotFiles), or nil if the drive has never been
// ingested.
func (s *Store) VolumeSnapshot(volumeID int) *VolumeSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap, _ := s.snapshotLocked(volumeID); snap != nil {
		return headerOfSnapshot(snap)
	}
	return nil
}

// VolumeSnapshots returns a copy of every drive snapshot, without files.
func (s *Store) VolumeSnapshots() []*VolumeSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*VolumeSnapshot, 0, len(s.c.Snapshots))
	for _, snap := range s.c.Snapshots {
		out = append(out, headerOfSnapshot(snap))
	}
	return out
}

// SnapshotFiles yields every file of a volume's snapshot, in order, reading a
// page at a time and holding the store lock only while it reads one, so the
// loop body may call Store methods. A snapshot replaced while it is being read
// (the drive scanned again) ends the loop with an error, never with a mix of the
// two. A volume with no snapshot yields nothing.
func (s *Store) SnapshotFiles(volumeID int) iter.Seq2[SnapFile, error] {
	return func(yield func(SnapFile, error) bool) {
		var started *VolumeSnapshot
		after := -1
		for {
			s.mu.Lock()
			cur, _ := s.snapshotLocked(volumeID)
			if started == nil {
				started = cur
			}
			if cur == nil || cur != started {
				s.mu.Unlock()
				if started != nil {
					yield(SnapFile{}, fmt.Errorf("the snapshot of volume %d %w; read it again", volumeID, errSnapshotChanged))
				}
				return
			}
			var page []SnapFile
			var err error
			if s.be != nil {
				page, after, err = s.be.snapFilePage(volumeID, after, snapFilePageSize)
			} else {
				start := after + 1
				end := min(start+snapFilePageSize, len(cur.Files))
				if start < end {
					page = append([]SnapFile(nil), cur.Files[start:end]...)
				}
				after = end - 1
			}
			s.mu.Unlock()
			if err != nil {
				yield(SnapFile{}, err)
				return
			}
			for _, f := range page {
				if !yield(f, nil) {
					return
				}
			}
			if len(page) < snapFilePageSize {
				return
			}
		}
	}
}

// SnapshotFileList collects every file of a volume's snapshot.
func (s *Store) SnapshotFileList(volumeID int) ([]SnapFile, error) {
	var out []SnapFile
	for f, err := range s.SnapshotFiles(volumeID) {
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// ---- scan records ------------------------------------------------------

// AddScanRecord records a drive scan, giving it its ID, and drops the oldest
// records of that drive beyond keep. The catalog is written to disk at once, even
// inside a batch, so the caller may remove the dropped records' file lists only
// after this returns them. If the write fails, the record is not added, nothing is
// dropped, and the error is returned.
func (s *Store) AddScanRecord(rec *ScanRecord, keep int) ([]*ScanRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prevScans := append([]*ScanRecord(nil), s.c.Scans...)
	prevID := s.c.NextID["scan"]
	c := *rec
	c.ID = s.next("scan")
	s.c.Scans = append(s.c.Scans, &c)
	n := 0
	for _, r := range s.c.Scans {
		if r.VolumeID == rec.VolumeID {
			n++
		}
	}
	var dropped []*ScanRecord
	kept := make([]*ScanRecord, 0, len(s.c.Scans))
	for _, r := range s.c.Scans { // oldest first
		if r.VolumeID == rec.VolumeID && n > keep {
			d := *r
			dropped = append(dropped, &d)
			n--
			continue
		}
		kept = append(kept, r)
	}
	s.c.Scans = kept
	if err := s.writeCatalog(); err != nil {
		s.c.Scans, s.c.NextID["scan"] = prevScans, prevID
		return nil, err
	}
	rec.ID = c.ID
	return dropped, nil
}

// ScanFileListsInUse is the set of file-list names the scan records point at.
func (s *Store) ScanFileListsInUse() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.c.Scans))
	for _, r := range s.c.Scans {
		out[r.FileList] = true
	}
	return out
}

// scanFileLists maps each scan record's file-list name to its recorded SHA-256.
func (s *Store) scanFileLists() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.c.scanListSums()
	for name := range out {
		if !validScanFileName(name) {
			delete(out, name)
		}
	}
	return out
}

// scanListSums maps each scan record's file-list name, as recorded, to its
// recorded SHA-256 (for a catalog read from an app backup, too).
func (c *catalog) scanListSums() map[string]string {
	out := make(map[string]string, len(c.Scans))
	for _, r := range c.Scans {
		out[r.FileList] = r.FileListSHA256
	}
	return out
}

// ScanRecords returns copies of a drive's scan records, newest first.
func (s *Store) ScanRecords(volumeID int) []*ScanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*ScanRecord
	for i := len(s.c.Scans) - 1; i >= 0; i-- {
		if r := s.c.Scans[i]; r.VolumeID == volumeID {
			c := *r
			out = append(out, &c)
		}
	}
	return out
}

// AllScanRecords returns copies of every scan record, newest first.
func (s *Store) AllScanRecords() []*ScanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*ScanRecord, 0, len(s.c.Scans))
	for i := len(s.c.Scans) - 1; i >= 0; i-- {
		c := *s.c.Scans[i]
		out = append(out, &c)
	}
	return out
}

// scanByID finds a scan record by its ID.
func (s *Store) scanByID(id int) *ScanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.c.Scans {
		if r.ID == id {
			c := *r
			return &c
		}
	}
	return nil
}

// latestCompleteScan is the drive's newest complete scan of the whole drive, or
// nil. A folder adopt's scan (Root set) covers one folder, not the drive.
func (s *Store) latestCompleteScan(volumeID int) *ScanRecord {
	for _, r := range s.ScanRecords(volumeID) {
		if r.Scan == ScanComplete && r.Root == "" {
			return r
		}
	}
	return nil
}

// ---- reference and comparison records ------------------------------------

// SetReference marks the reference (nil clears it) and saves.
func (s *Store) SetReference(m *ReferenceMark) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m != nil {
		c := *m
		m = &c
	}
	s.c.Reference = m
	return s.save()
}

// Reference is the marked reference, or nil.
func (s *Store) Reference() *ReferenceMark {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.c.Reference == nil {
		return nil
	}
	m := *s.c.Reference
	return &m
}

func cloneCompareRun(r *CompareRun) *CompareRun {
	c := *r
	if r.Counts != nil {
		c.Counts = make(map[string]int, len(r.Counts))
		for k, v := range r.Counts {
			c.Counts[k] = v
		}
	}
	return &c
}

// AddCompareRun records a run, giving it its ID, and drops the oldest runs
// beyond keep. The catalog is written to disk at once; on failure nothing
// changes. It returns the dropped runs, whose result files the caller removes.
func (s *Store) AddCompareRun(run *CompareRun, keep int) ([]*CompareRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := append([]*CompareRun(nil), s.c.CompareRuns...)
	prevID := s.c.NextID["compare"]
	rec := cloneCompareRun(run)
	rec.ID = s.next("compare")
	s.c.CompareRuns = append(s.c.CompareRuns, rec)
	var dropped []*CompareRun
	if n := len(s.c.CompareRuns) - keep; n > 0 {
		for _, d := range s.c.CompareRuns[:n] {
			dropped = append(dropped, cloneCompareRun(d))
		}
		s.c.CompareRuns = append([]*CompareRun(nil), s.c.CompareRuns[n:]...)
	}
	if err := s.writeCatalog(); err != nil {
		s.c.CompareRuns, s.c.NextID["compare"] = prev, prevID
		return nil, err
	}
	run.ID = rec.ID
	return dropped, nil
}

// CompareRuns returns copies of the saved runs, newest first.
func (s *Store) CompareRuns() []*CompareRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*CompareRun, 0, len(s.c.CompareRuns))
	for i := len(s.c.CompareRuns) - 1; i >= 0; i-- {
		out = append(out, cloneCompareRun(s.c.CompareRuns[i]))
	}
	return out
}

// CompareRun returns a saved run by ID, or nil.
func (s *Store) CompareRun(id int) *CompareRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.c.CompareRuns {
		if r.ID == id {
			return cloneCompareRun(r)
		}
	}
	return nil
}

// ---- drift reports -------------------------------------------------------

// headerOfDrift is a copy of a drift report without its items.
func headerOfDrift(r *DriftReport) *DriftReport {
	c := *r
	c.Items = nil
	cp := func(m map[string]int) map[string]int {
		if m == nil {
			return nil
		}
		out := make(map[string]int, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	c.Counts, c.InfoCounts = cp(r.Counts), cp(r.InfoCounts)
	c.ByExt = append([]ExtDrift(nil), r.ByExt...)
	c.Unread = append([]string(nil), r.Unread...)
	return &c
}

// ReplaceDriftReport stores r, items and all, as the latest report for its
// collection, and returns the save's error. The store keeps its own copy.
func (s *Store) ReplaceDriftReport(r *DriftReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A new slice, put in place only once the report is written: a write the
	// database refuses leaves the previous report.
	out := make([]*DriftReport, 0, len(s.c.Drift)+1)
	for _, d := range s.c.Drift {
		if d.CollectionID != r.CollectionID {
			out = append(out, d)
		}
	}
	rec := headerOfDrift(r)
	if s.be != nil {
		s.dirty = true
		if err := s.be.driftPut(rec, len(out), r.Items); err != nil {
			return err
		}
	} else if r.Items != nil {
		rec.Items = append(make([]DriftItem, 0, len(r.Items)), r.Items...)
	}
	s.c.Drift = append(out, rec)
	return s.save()
}

func (s *Store) driftLocked(collectionID int) *DriftReport {
	for _, d := range s.c.Drift {
		if d.CollectionID == collectionID {
			return d
		}
	}
	return nil
}

// DriftReport returns a copy of an archive's latest reconcile report without its
// items (see DriftItems), or nil.
func (s *Store) DriftReport(collectionID int) *DriftReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d := s.driftLocked(collectionID); d != nil {
		return headerOfDrift(d)
	}
	return nil
}

// DriftReports returns a copy of every archive's latest report, without items.
func (s *Store) DriftReports() []*DriftReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DriftReport, 0, len(s.c.Drift))
	for _, d := range s.c.Drift {
		out = append(out, headerOfDrift(d))
	}
	return out
}

// DriftItems returns the changed files of an archive's latest report, in order
// (nil when there is no report).
func (s *Store) DriftItems(collectionID int) ([]DriftItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.driftItemsLocked(collectionID)
}

func (s *Store) driftItemsLocked(collectionID int) ([]DriftItem, error) {
	d := s.driftLocked(collectionID)
	if d == nil {
		return nil, nil
	}
	if s.be != nil {
		return s.be.driftItems(collectionID)
	}
	return append([]DriftItem(nil), d.Items...), nil
}

// DriftReportWithItems returns a copy of an archive's latest report with its
// items, or nil.
func (s *Store) DriftReportWithItems(collectionID int) (*DriftReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.driftLocked(collectionID)
	if d == nil {
		return nil, nil
	}
	out := headerOfDrift(d)
	items, err := s.driftItemsLocked(collectionID)
	if err != nil {
		return nil, err
	}
	out.Items = items
	return out, nil
}
