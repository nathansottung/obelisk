package main

// scanhistory.go — the history of drive scans. Each scan of a docked drive is
// its own ScanRecord in the catalog (ID, time, drive, complete or partial, counts),
// and its full file list is written as gzip-compressed JSON to the scans folder in
// the data folder, named in the record with its SHA-256. Only the newest
// scan_history_retained scans of a drive are kept. The catalog keeps the latest
// snapshot of each drive as before (VolumeSnapshot), for the views that use it.

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

// ScanRecord is one scan of a drive.
type ScanRecord struct {
	ID                int       `json:"id"`
	VolumeID          int       `json:"volume_id"`
	At                time.Time `json:"at"`
	SessionID         int       `json:"session_id,omitempty"`
	Root              string    `json:"root,omitempty"` // the folder scanned, when not the whole drive
	Label             string    `json:"label,omitempty"`
	Serial            string    `json:"serial,omitempty"`
	Scan              string    `json:"scan"` // ScanComplete or ScanPartial
	TotalFiles        int       `json:"total_files"`
	TotalBytes        int64     `json:"total_bytes"`
	UnreadableFolders int       `json:"unreadable_folders,omitempty"`
	UnreadableFiles   int       `json:"unreadable_files,omitempty"`
	SkippedFolders    int       `json:"skipped_folders,omitempty"`
	FileList          string    `json:"file_list"`        // its name in the scans folder
	FileListSHA256    string    `json:"file_list_sha256"` // of the compressed file
}

// ScanFileList is the content of a scan's compressed file list.
type ScanFileList struct {
	Format            string     `json:"format"`
	Version           int        `json:"version"`
	VolumeID          int        `json:"volume_id"`
	At                time.Time  `json:"at"`
	Scan              string     `json:"scan"`
	Root              string     `json:"root,omitempty"`
	Files             []SnapFile `json:"files"`
	UnreadableFolders []ScanGap  `json:"unreadable_folders,omitempty"`
	UnreadableFiles   []ScanGap  `json:"unreadable_files,omitempty"`
	SkippedFolders    []ScanGap  `json:"skipped_folders,omitempty"`
}

const (
	scanFileListFormat   = "obelisk-scan-files"
	defaultScansRetained = 10
	scanFilePrefix       = "scan-"
	scanFileSuffix       = ".json.gz"
)

func (a *App) scansDir() string { return filepath.Join(a.DataDir, "scans") }

// scansRetained is how many scans of each drive are kept.
func scansRetained(cfg Config) int {
	if cfg.ScanHistoryRetained > 0 {
		return cfg.ScanHistoryRetained
	}
	return defaultScansRetained
}

// validScanFileName accepts only names this file writes, so a record can never
// point a read or a removal outside the scans folder.
func validScanFileName(name string) bool {
	return name == filepath.Base(name) && strings.HasPrefix(name, scanFilePrefix) &&
		strings.HasSuffix(name, scanFileSuffix) && !strings.ContainsAny(name, "/\\:")
}

// recordScan stores snap as a new scan of its drive: the compressed file list
// first, then the record. Any failure is returned, so the ingest fails.
func (a *App) recordScan(snap *VolumeSnapshot, keep int) (*ScanRecord, error) {
	if keep <= 0 {
		keep = defaultScansRetained
	}
	list := ScanFileList{Format: scanFileListFormat, Version: 1, VolumeID: snap.VolumeID, At: snap.CapturedAt,
		Scan: snap.Scan, Root: snap.Root, Files: snap.Files, UnreadableFolders: snap.UnreadableFolders,
		UnreadableFiles: snap.UnreadableFiles, SkippedFolders: snap.SkippedFolders}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(list); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	tok := make([]byte, 6)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%sv%d-%s-%s%s", scanFilePrefix, snap.VolumeID,
		snap.CapturedAt.UTC().Format("20060102T150405Z"), hex.EncodeToString(tok), scanFileSuffix)
	dir := a.scansDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("saving the scan's file list: %w", err)
	}
	final := filepath.Join(dir, name)
	tmp, err := writeStage(final, buf.Bytes(), 0o600)
	if err != nil {
		return nil, fmt.Errorf("saving the scan's file list: %w", err)
	}
	if err := renameReplacing(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("saving the scan's file list: %w", err)
	}
	rec := &ScanRecord{VolumeID: snap.VolumeID, At: snap.CapturedAt, SessionID: snap.SessionID, Root: snap.Root, Label: snap.Label,
		Serial: snap.Serial, Scan: snap.Scan, TotalFiles: snap.TotalFiles, TotalBytes: snap.TotalBytes,
		UnreadableFolders: len(snap.UnreadableFolders), UnreadableFiles: len(snap.UnreadableFiles),
		SkippedFolders: len(snap.SkippedFolders), FileList: name, FileListSHA256: sha256Hex(buf.Bytes())}
	dropped, err := a.Store.AddScanRecord(rec, keep)
	if err != nil {
		_ = os.Remove(final) // no record points at it
		return nil, fmt.Errorf("saving the scan record: %w", err)
	}
	// The record is on disk; the dropped scans' file lists can go.
	for _, d := range dropped {
		if validScanFileName(d.FileList) {
			_ = os.Remove(filepath.Join(dir, d.FileList))
		}
	}
	a.sweepScanFiles(dir)
	return rec, nil
}

// sweepScanFiles removes file lists in the scans folder that no record points
// at (left by a crash, or by a catalog restored from an older backup). Only
// files older than an hour are removed, so a scan being recorded at the same
// moment keeps its file.
func (a *App) sweepScanFiles(dir string) {
	inUse := a.Store.ScanFileListsInUse()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || !validScanFileName(name) || inUse[name] {
			continue
		}
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > time.Hour {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

// LoadScanFiles reads a scan's file list, checking it against its recorded
// SHA-256.
func (a *App) LoadScanFiles(rec *ScanRecord) (*ScanFileList, error) {
	if rec == nil || !validScanFileName(rec.FileList) {
		return nil, fmt.Errorf("scan has no file list")
	}
	b, err := os.ReadFile(filepath.Join(a.scansDir(), rec.FileList))
	if err != nil {
		return nil, fmt.Errorf("reading scan %d's file list: %w", rec.ID, err)
	}
	if sha256Hex(b) != rec.FileListSHA256 {
		return nil, fmt.Errorf("scan %d's file list does not match its record (changed or damaged)", rec.ID)
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	var list ScanFileList
	if err := json.NewDecoder(io.LimitReader(zr, 8<<30)).Decode(&list); err != nil {
		return nil, fmt.Errorf("reading scan %d's file list: %w", rec.ID, err)
	}
	if list.Format != scanFileListFormat || list.VolumeID != rec.VolumeID {
		return nil, fmt.Errorf("scan %d's file list is not this scan's", rec.ID)
	}
	return &list, nil
}
