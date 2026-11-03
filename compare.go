package main

// compare.go — comparing drive scans. One drive or archive can be marked as the
// reference. A comparison classifies another scan's files against a reference
// scan (or the reference archive's current files) with the shared classifier
// (classify.go), and each run is saved: a CompareRun record in the catalog with
// its counts, and its per-file results as gzip-compressed JSON in the compares
// folder in the data folder. Comparisons are reports only: they change no
// package, copy or count.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
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
	RefRoot       string         `json:"ref_root,omitempty"` // the folder a reference scan covers; empty for a whole drive
	OtherScanID   int            `json:"other_scan_id"`
	OtherVolumeID int            `json:"other_volume_id"`
	OtherLabel    string         `json:"other_label"`
	OtherComplete bool           `json:"other_complete"`
	OtherRoot     string         `json:"other_root,omitempty"`
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

// scanSide reads a scan's files as one side of a comparison, with what the scan
// could not read. Only each file's path, hash and size are kept.
func (a *App) scanSide(rec *ScanRecord) (ClassSide, error) {
	var side ClassSide
	list, err := a.StreamScanFiles(rec, func(f SnapFile) error {
		side.Files = append(side.Files, ClassFile{Path: f.RelPath, Hash: f.Hash, Size: f.SizeBytes})
		return nil
	})
	if err != nil {
		return ClassSide{}, err
	}
	gaps := &VolumeSnapshot{UnreadableFolders: list.UnreadableFolders, UnreadableFiles: list.UnreadableFiles, SkippedFolders: list.SkippedFolders}
	side.Known = gaps.Known
	return side, nil
}

// scanCovers says what a scan covers, for messages.
func scanCovers(r *ScanRecord) string {
	if r.Root != "" {
		return "the folder " + r.Root
	}
	return "the whole drive"
}

// scanLabel names a scan's drive, and its folder when it covers one.
func scanLabel(a *App, r *ScanRecord) string {
	if r.Root != "" {
		return a.volumeLabel(r.VolumeID) + " (folder " + r.Root + ")"
	}
	return a.volumeLabel(r.VolumeID)
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
		OtherLabel: scanLabel(a, otherRec), OtherComplete: otherRec.Scan == ScanComplete, OtherRoot: otherRec.Root}
	var refSide ClassSide
	switch {
	case ref.ArchiveID > 0:
		coll := a.Store.Collection(ref.ArchiveID)
		if coll == nil {
			return nil, fmt.Errorf("archive %d not found", ref.ArchiveID)
		}
		for f, err := range a.Store.FilesOf(coll.ID) {
			if err != nil {
				return nil, err
			}
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
			return nil, fmt.Errorf("%s has no complete scan of the whole drive to compare against", a.volumeLabel(ref.VolumeID))
		}
		// Paths in a folder's scan start at that folder, and in a drive's scan at
		// the drive's root: the two are not comparable path by path.
		if rec.Root != otherRec.Root {
			return nil, fmt.Errorf("scan %d is of %s and scan %d of %s; compare scans of the same folder, or of whole drives", rec.ID, scanCovers(rec), otherRec.ID, scanCovers(otherRec))
		}
		var err error
		if refSide, err = a.scanSide(rec); err != nil {
			return nil, err
		}
		run.RefScanID, run.RefVolumeID, run.RefLabel, run.RefComplete, run.RefRoot = rec.ID, rec.VolumeID, scanLabel(a, rec), rec.Scan == ScanComplete, rec.Root
	}
	otherSide, err := a.scanSide(otherRec)
	if err != nil {
		return nil, err
	}
	cls := Classify(refSide, otherSide, HostPathRules())
	refSide, otherSide = ClassSide{}, ClassSide{}
	run.Counts, run.Total = cls.Counts, len(cls.Results)
	if err := a.saveCompareRun(run, cls.Results); err != nil {
		return nil, err
	}
	a.Store.Log("compare", fmt.Sprintf("compared %s (scan %d) with %s: %v", run.OtherLabel, run.OtherScanID, run.RefLabel, run.Counts))
	return run, nil
}

// writeCompareResults writes a compareResultFile of results to w, compressed, one
// result at a time, so the document is never held whole.
func writeCompareResults(w io.Writer, results []ClassResult) error {
	bw := bufio.NewWriterSize(w, 1<<16)
	zw := gzip.NewWriter(bw)
	head, err := json.Marshal(compareFileFormat)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(zw, `{"format":%s,"version":1,"results":`, head); err != nil {
		return err
	}
	if results == nil {
		_, err = io.WriteString(zw, "null")
	} else {
		_, err = io.WriteString(zw, "[")
		for i := range results {
			if err != nil {
				break
			}
			var b []byte
			if b, err = json.Marshal(&results[i]); err != nil {
				break
			}
			if i > 0 {
				if _, err = io.WriteString(zw, ","); err != nil {
					break
				}
			}
			_, err = zw.Write(b)
		}
		if err == nil {
			_, err = io.WriteString(zw, "]")
		}
	}
	if err == nil {
		_, err = io.WriteString(zw, "}\n")
	}
	if err == nil {
		err = zw.Close()
	}
	if err == nil {
		err = bw.Flush()
	}
	return err
}

// saveCompareRun writes the results file, then the record (to disk at once),
// then removes the result files of runs beyond the cap.
func (a *App) saveCompareRun(run *CompareRun, results []ClassResult) error {
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
	stage, err := createStage(final, 0o600)
	if err != nil {
		return fmt.Errorf("saving the comparison: %w", err)
	}
	tmp := stage.Name()
	sum := sha256.New()
	err = writeCompareResults(io.MultiWriter(stage, sum), results)
	if err == nil {
		err = stage.Sync()
	}
	if cerr := stage.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saving the comparison: %w", err)
	}
	if err := renameReplacing(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saving the comparison: %w", err)
	}
	run.ResultFile, run.ResultSHA256 = name, hex.EncodeToString(sum.Sum(nil))
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
