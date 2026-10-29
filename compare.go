package main

// compare.go — comparing drive scans. One drive or archive can be marked as the
// reference. A comparison classifies another scan's files against a reference
// scan (or the reference archive's current files) with the shared classifier
// (classify.go), and each run is saved: a CompareRun record in the catalog with
// its counts, and its per-file results as gzip-compressed JSON in the compares
// folder in the data folder. Comparisons are reports only: they change no
// package, copy or count.

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReferenceMark is the drive (Kind "volume") or archive (Kind "archive") other
// scans are compared against by default.
type ReferenceMark struct {
	Kind string `json:"kind"`
	ID   int    `json:"id"`
}

// CompareRef says what to compare against: a scan by ID, a drive (its latest
// complete scan), an archive (its current files), or, when all are zero, the
// marked reference.
type CompareRef struct {
	ScanID    int `json:"scan_id,omitempty"`
	VolumeID  int `json:"volume_id,omitempty"`
	ArchiveID int `json:"archive_id,omitempty"`
}

// CompareRun is one saved comparison.
type CompareRun struct {
	ID            int            `json:"id"`
	At            time.Time      `json:"at"`
	RefScanID     int            `json:"ref_scan_id,omitempty"`
	RefVolumeID   int            `json:"ref_volume_id,omitempty"`
	RefArchiveID  int            `json:"ref_archive_id,omitempty"`
	RefLabel      string         `json:"ref_label"`
	RefComplete   bool           `json:"ref_complete"`
	OtherScanID   int            `json:"other_scan_id"`
	OtherVolumeID int            `json:"other_volume_id"`
	OtherLabel    string         `json:"other_label"`
	OtherComplete bool           `json:"other_complete"`
	Counts        map[string]int `json:"counts"`
	Total         int            `json:"total"`
	ResultFile    string         `json:"result_file"`
	ResultSHA256  string         `json:"result_sha256"`
}

type compareResultFile struct {
	Format  string        `json:"format"`
	Version int           `json:"version"`
	Results []ClassResult `json:"results"`
}

const (
	compareFileFormat   = "obelisk-compare-results"
	compareRunsRetained = 50
	compareFilePrefix   = "compare-"
	compareFileSuffix   = ".json.gz"
)

func (a *App) comparesDir() string { return filepath.Join(a.DataDir, "compares") }

func validCompareFileName(name string) bool {
	return name == filepath.Base(name) && strings.HasPrefix(name, compareFilePrefix) &&
		strings.HasSuffix(name, compareFileSuffix) && !strings.ContainsAny(name, "/\\:")
}

// scanSide is a scan's files as one side of a comparison, with what the scan
// could not read.
func scanSide(list *ScanFileList) ClassSide {
	gaps := &VolumeSnapshot{UnreadableFolders: list.UnreadableFolders, UnreadableFiles: list.UnreadableFiles, SkippedFolders: list.SkippedFolders}
	side := ClassSide{Known: gaps.Known}
	for _, f := range list.Files {
		side.Files = append(side.Files, ClassFile{Path: f.RelPath, Hash: f.Hash, Size: f.SizeBytes})
	}
	return side
}

// latestCompleteScan is the drive's newest complete scan, or nil.
func (s *Store) latestCompleteScan(volumeID int) *ScanRecord {
	for _, r := range s.ScanRecords(volumeID) {
		if r.Scan == ScanComplete {
			return r
		}
	}
	return nil
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

func (a *App) volumeLabel(id int) string {
	if v := a.Store.Volume(id); v != nil {
		return v.Label
	}
	return fmt.Sprintf("volume %d", id)
}

// CompareScans compares the scan otherScanID against ref and saves the run.
func (a *App) CompareScans(ref CompareRef, otherScanID int) (*CompareRun, error) {
	otherRec := a.Store.scanByID(otherScanID)
	if otherRec == nil {
		return nil, fmt.Errorf("scan %d not found", otherScanID)
	}
	if ref == (CompareRef{}) {
		m := a.Store.Reference()
		if m == nil {
			return nil, fmt.Errorf("no reference is marked: choose a reference drive, archive or scan")
		}
		if m.Kind == "archive" {
			ref.ArchiveID = m.ID
		} else {
			ref.VolumeID = m.ID
		}
	}
	run := &CompareRun{At: time.Now().UTC(), OtherScanID: otherRec.ID, OtherVolumeID: otherRec.VolumeID,
		OtherLabel: a.volumeLabel(otherRec.VolumeID), OtherComplete: otherRec.Scan == ScanComplete}
	var refSide ClassSide
	switch {
	case ref.ArchiveID > 0:
		coll := a.Store.Collection(ref.ArchiveID)
		if coll == nil {
			return nil, fmt.Errorf("archive %d not found", ref.ArchiveID)
		}
		for _, f := range a.Store.FilesOf(coll.ID) {
			refSide.Files = append(refSide.Files, ClassFile{Path: f.RelPath, Hash: f.Hash, Size: f.SizeBytes})
		}
		run.RefArchiveID, run.RefLabel, run.RefComplete = coll.ID, "archive "+coll.Name, true
	default:
		var rec *ScanRecord
		if ref.ScanID > 0 {
			if rec = a.Store.scanByID(ref.ScanID); rec == nil {
				return nil, fmt.Errorf("scan %d not found", ref.ScanID)
			}
		} else if rec = a.Store.latestCompleteScan(ref.VolumeID); rec == nil {
			return nil, fmt.Errorf("%s has no complete scan to compare against", a.volumeLabel(ref.VolumeID))
		}
		list, err := a.LoadScanFiles(rec)
		if err != nil {
			return nil, err
		}
		refSide = scanSide(list)
		run.RefScanID, run.RefVolumeID, run.RefLabel, run.RefComplete = rec.ID, rec.VolumeID, a.volumeLabel(rec.VolumeID), rec.Scan == ScanComplete
	}
	otherList, err := a.LoadScanFiles(otherRec)
	if err != nil {
		return nil, err
	}
	cls := Classify(refSide, scanSide(otherList), HostPathRules())
	run.Counts, run.Total = cls.Counts, len(cls.Results)
	if err := a.saveCompareRun(run, cls.Results); err != nil {
		return nil, err
	}
	a.Store.Log("compare", fmt.Sprintf("compared %s (scan %d) with %s: %v", run.OtherLabel, run.OtherScanID, run.RefLabel, run.Counts))
	return run, nil
}

// saveCompareRun writes the results file, then the record (to disk at once),
// then removes the result files of runs beyond the cap.
func (a *App) saveCompareRun(run *CompareRun, results []ClassResult) error {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(compareResultFile{Format: compareFileFormat, Version: 1, Results: results}); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	tok := make([]byte, 6)
	if _, err := rand.Read(tok); err != nil {
		return err
	}
	name := fmt.Sprintf("%s%s-%s%s", compareFilePrefix, run.At.Format("20060102T150405Z"), hex.EncodeToString(tok), compareFileSuffix)
	dir := a.comparesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("saving the comparison: %w", err)
	}
	final := filepath.Join(dir, name)
	tmp, err := writeStage(final, buf.Bytes(), 0o600)
	if err != nil {
		return fmt.Errorf("saving the comparison: %w", err)
	}
	if err := renameReplacing(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saving the comparison: %w", err)
	}
	run.ResultFile, run.ResultSHA256 = name, sha256Hex(buf.Bytes())
	dropped, err := a.Store.AddCompareRun(run, compareRunsRetained)
	if err != nil {
		_ = os.Remove(final)
		return fmt.Errorf("saving the comparison record: %w", err)
	}
	for _, d := range dropped {
		if validCompareFileName(d.ResultFile) {
			_ = os.Remove(filepath.Join(dir, d.ResultFile))
		}
	}
	return nil
}

// LoadCompareResults reads a run's per-file results, checked against its record.
func (a *App) LoadCompareResults(run *CompareRun) ([]ClassResult, error) {
	if run == nil || !validCompareFileName(run.ResultFile) {
		return nil, fmt.Errorf("comparison has no result file")
	}
	b, err := os.ReadFile(filepath.Join(a.comparesDir(), run.ResultFile))
	if err != nil {
		return nil, fmt.Errorf("reading comparison %d: %w", run.ID, err)
	}
	if sha256Hex(b) != run.ResultSHA256 {
		return nil, fmt.Errorf("comparison %d's result file does not match its record (changed or damaged)", run.ID)
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	var f compareResultFile
	if err := json.NewDecoder(io.LimitReader(zr, 4<<30)).Decode(&f); err != nil {
		return nil, fmt.Errorf("reading comparison %d: %w", run.ID, err)
	}
	if f.Format != compareFileFormat {
		return nil, fmt.Errorf("comparison %d's result file is not a comparison", run.ID)
	}
	return f.Results, nil
}

// ---- store ----------------------------------------------------------------

// SetReference marks the reference (nil clears it) and saves.
func (s *Store) SetReference(m *ReferenceMark) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// AddCompareRun records a run, giving it its ID, and drops the oldest runs
// beyond keep. The catalog is written to disk at once; on failure nothing
// changes. It returns the dropped runs, whose result files the caller removes.
func (s *Store) AddCompareRun(run *CompareRun, keep int) ([]*CompareRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := append([]*CompareRun(nil), s.c.CompareRuns...)
	prevID := s.c.NextID["compare"]
	run.ID = s.next("compare")
	s.c.CompareRuns = append(s.c.CompareRuns, run)
	var dropped []*CompareRun
	if n := len(s.c.CompareRuns) - keep; n > 0 {
		dropped = append(dropped, s.c.CompareRuns[:n]...)
		s.c.CompareRuns = append([]*CompareRun(nil), s.c.CompareRuns[n:]...)
	}
	if err := s.writeCatalog(); err != nil {
		s.c.CompareRuns, s.c.NextID["compare"] = prev, prevID
		return nil, err
	}
	return dropped, nil
}

// CompareRuns returns copies of the saved runs, newest first.
func (s *Store) CompareRuns() []*CompareRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*CompareRun, 0, len(s.c.CompareRuns))
	for i := len(s.c.CompareRuns) - 1; i >= 0; i-- {
		c := *s.c.CompareRuns[i]
		out = append(out, &c)
	}
	return out
}

// CompareRun returns a saved run by ID, or nil.
func (s *Store) CompareRun(id int) *CompareRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.c.CompareRuns {
		if r.ID == id {
			c := *r
			return &c
		}
	}
	return nil
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
