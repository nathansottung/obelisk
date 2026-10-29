package main

// xdrive_lists.go — the cross-drive lists: files found on only one drive, and
// files found on two or more, from each drive's latest complete scan. A drive
// whose scans are all partial is left out (and listed as left out), since what it
// did not read could hold anything. Files are matched by content (SHA-256), so
// the same file at different paths on different drives is one entry. Reports
// only: nothing here changes a copy or a count.

import (
	"fmt"
	"sort"
	"time"
)

// CrossDrivePlace is one place a file was found.
type CrossDrivePlace struct {
	VolumeID int    `json:"volume_id"`
	Label    string `json:"label"`
	Path     string `json:"path"`
}

// CrossDriveEntry is one file content and every place it was found.
type CrossDriveEntry struct {
	Hash   string            `json:"hash"`
	Size   int64             `json:"size"`
	Drives int               `json:"drives"`
	Places []CrossDrivePlace `json:"places"`
}

// CrossDriveScan is the scan used for a drive.
type CrossDriveScan struct {
	VolumeID int       `json:"volume_id"`
	Label    string    `json:"label"`
	ScanID   int       `json:"scan_id"`
	At       time.Time `json:"at"`
}

// CrossDriveLeftOut is a drive with no complete scan.
type CrossDriveLeftOut struct {
	VolumeID int    `json:"volume_id"`
	Label    string `json:"label"`
	Reason   string `json:"reason"`
}

// CrossDriveLists is the result of CrossDriveLists.
type CrossDriveLists struct {
	VolumeID    int                 `json:"volume_id,omitempty"` // the drive OnlyOn is for
	OnlyOn      []CrossDriveEntry   `json:"only_on"`
	OnTwoOrMore []CrossDriveEntry   `json:"on_two_or_more"`
	Scans       []CrossDriveScan    `json:"scans"`
	LeftOut     []CrossDriveLeftOut `json:"left_out,omitempty"`
}

// CrossDriveLists builds the lists. OnlyOn is for volumeID (empty when 0).
func (a *App) CrossDriveLists(volumeID int) (*CrossDriveLists, error) {
	out := &CrossDriveLists{VolumeID: volumeID, OnlyOn: []CrossDriveEntry{}, OnTwoOrMore: []CrossDriveEntry{}, Scans: []CrossDriveScan{}}
	seenVol := map[int]bool{}
	var vols []int
	for _, r := range a.Store.AllScanRecords() {
		if !seenVol[r.VolumeID] {
			seenVol[r.VolumeID] = true
			vols = append(vols, r.VolumeID)
		}
	}
	sort.Ints(vols)
	byHash := map[string]*CrossDriveEntry{}
	onDrive := map[string]map[int]bool{}
	for _, vid := range vols {
		label := a.volumeLabel(vid)
		rec := a.Store.latestCompleteScan(vid)
		if rec == nil {
			out.LeftOut = append(out.LeftOut, CrossDriveLeftOut{VolumeID: vid, Label: label, Reason: "no complete scan of the whole drive"})
			continue
		}
		list, err := a.LoadScanFiles(rec)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		out.Scans = append(out.Scans, CrossDriveScan{VolumeID: vid, Label: label, ScanID: rec.ID, At: rec.At})
		for _, f := range list.Files {
			if f.Hash == "" {
				continue
			}
			e := byHash[f.Hash]
			if e == nil {
				e = &CrossDriveEntry{Hash: f.Hash, Size: f.SizeBytes}
				byHash[f.Hash] = e
				onDrive[f.Hash] = map[int]bool{}
			}
			e.Places = append(e.Places, CrossDrivePlace{VolumeID: vid, Label: label, Path: f.RelPath})
			onDrive[f.Hash][vid] = true
		}
	}
	for h, e := range byHash {
		e.Drives = len(onDrive[h])
		switch {
		case e.Drives >= 2:
			out.OnTwoOrMore = append(out.OnTwoOrMore, *e)
		case volumeID > 0 && onDrive[h][volumeID]:
			out.OnlyOn = append(out.OnlyOn, *e)
		}
	}
	firstPath := func(e CrossDriveEntry) string { return e.Places[0].Path }
	sort.Slice(out.OnlyOn, func(i, j int) bool { return firstPath(out.OnlyOn[i]) < firstPath(out.OnlyOn[j]) })
	sort.Slice(out.OnTwoOrMore, func(i, j int) bool { return firstPath(out.OnTwoOrMore[i]) < firstPath(out.OnTwoOrMore[j]) })
	return out, nil
}
