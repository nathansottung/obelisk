package main

// dock.go — a guided, resumable mode for ingesting a stack of legacy backup
// drives through a dock, one at a time. The operator picks the Archive(s) to
// reconcile against; the view watches for a newly-inserted drive; a single job
// chain then does everything hands-off:
//
//   register/match the Volume by serial (idempotent across sessions) → capture
//   its physical identity → MIRROR-ADOPT it (hash every loose file, match by
//   CONTENT against the selected archives' cataloged source hashes) → record the
//   drive's snapshot and this scan in the catalog (the scan's file list goes to
//   the scans folder in the data folder) → record per-drive results → "DONE —
//   safe to eject. Insert the next drive."
//
// It is strictly READ-ONLY toward the drive and toward sources: nothing is ever
// written onto a docked drive, the archive folders on the NAS are only ever
// hashed for comparison, and a docked drive that is (by where it really is) a
// source, inside one or holding one is refused (CheckOutsideSources). The only
// writes are to the catalog and the scans folder. A drive is identified by its
// physical serial, so re-inserting one already processed is recognized and
// offered as a re-verify, not a re-adopt.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// dockSidecarDir is the single folder the dock writes onto each drive. The
// mirror-adoption walk skips it so the tool never re-inventories its own output.
// New sidecars are written under the Obelisk name; the legacy Mnemosyne folder is
// still recognized on read forever (drives written under the old name keep their
// MNEMOSYNE_DOCK sidecar — see the "Name compatibility" section in ARCHITECTURE.md).
const dockSidecarDir = "OBELISK_DOCK"
const dockSidecarDirLegacy = "MNEMOSYNE_DOCK"

// MountInfo is one mounted volume the dock watcher can see (platform-resolved).
type MountInfo struct {
	Path      string `json:"path"`
	Label     string `json:"label,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

// DockCandidate is a newly-appeared drive offered for ingest: where it mounted,
// what the OS says it is, and whether we've seen this physical drive before.
type DockCandidate struct {
	Path               string         `json:"path"`
	Label              string         `json:"label,omitempty"`
	SizeBytes          int64          `json:"size_bytes,omitempty"`
	Serial             string         `json:"serial,omitempty"`
	Model              string         `json:"model,omitempty"`
	VolumeID           int            `json:"volume_id,omitempty"`
	AlreadyProcessed   bool           `json:"already_processed"`    // known drive with mirror data for these archives → offer re-verify
	ProcessedInSession bool           `json:"processed_in_session"` // already ingested in THIS session
	PlanWork           []PlanWorkItem `json:"plan_work,omitempty"`  // compiled plans this drive can advance
}

// ---- session lifecycle -------------------------------------------------

// StartDockSession opens a session reconciling against archiveIDs, snapshotting
// the mounts present now so the watcher can diff for newly-inserted drives.
//
// With no archives the session only scans: each drive's snapshot and scan are
// recorded, and nothing is matched or adopted.
func (a *App) StartDockSession(archiveIDs []int) (*DockSession, error) {
	for _, id := range archiveIDs {
		if a.Store.Collection(id) == nil {
			return nil, fmt.Errorf("archive %d not found", id)
		}
	}
	var baseline []string
	for _, m := range enumerateMounts() {
		baseline = append(baseline, m.Path)
	}
	ds := a.Store.AddDockSession(&DockSession{ArchiveIDs: archiveIDs, Baseline: baseline, Status: "ACTIVE"})
	if len(archiveIDs) == 0 {
		a.Store.Log("dock", fmt.Sprintf("session %d started to scan drives only (no archive)", ds.ID))
	} else {
		a.Store.Log("dock", fmt.Sprintf("session %d started for archive(s) %v", ds.ID, archiveIDs))
	}
	return ds, nil
}

// CloseDockSession marks a session done (the operator finished the stack).
func (a *App) CloseDockSession(id int) (*DockSession, error) {
	ds := a.Store.DockSession(id)
	if ds == nil {
		return nil, fmt.Errorf("dock session %d not found", id)
	}
	ds.Status = "CLOSED"
	a.Store.UpdateDockSession(ds)
	a.Store.Log("dock", fmt.Sprintf("session %d closed (%d drive(s))", id, len(ds.Drives)))
	return ds, nil
}

// DockCandidates returns the drives that have appeared since the session started
// (current mounts minus the baseline), each annotated with its resolved identity
// and whether it has been seen before.
func (a *App) DockCandidates(sessionID int) ([]DockCandidate, error) {
	ds := a.Store.DockSession(sessionID)
	if ds == nil {
		return nil, fmt.Errorf("dock session %d not found", sessionID)
	}
	base := map[string]bool{}
	for _, p := range ds.Baseline {
		base[p] = true
	}
	inSession := map[int]bool{}
	for _, d := range ds.Drives {
		inSession[d.VolumeID] = true
	}
	out := []DockCandidate{}
	for _, m := range enumerateMounts() {
		if base[m.Path] {
			continue // present at session start — not a freshly-docked drive
		}
		c := DockCandidate{Path: m.Path, Label: m.Label, SizeBytes: m.SizeBytes}
		if id, err := resolveDeviceIdentity(m.Path); err == nil {
			c.Serial, c.Model = id.Serial, id.Model
			if c.SizeBytes == 0 {
				c.SizeBytes = id.SizeBytes
			}
		}
		if v := a.Store.VolumeBySerial(c.Serial); v != nil {
			c.VolumeID = v.ID
			c.AlreadyProcessed = a.volumeHasMirror(v.ID, ds.ArchiveIDs)
			c.ProcessedInSession = inSession[v.ID]
			c.PlanWork = a.plansPendingForVolume(v.ID) // offer "Execute plan work from this drive"
		}
		out = append(out, c)
	}
	return out, nil
}

// ---- ingest ------------------------------------------------------------

// IngestDrive is the hands-off job chain for one docked drive. serial/label may
// be supplied by the watcher (or a test); a blank serial is resolved from the
// live device. mode "" auto-selects adopt vs re-verify (a drive already holding
// mirror data for these archives re-verifies).
//
// confirm gates the long read behind the drive-mortality check: if the SMART
// snapshot raises a failure advisory (reallocated/pending sectors &c.) and confirm
// is false, IngestDrive returns EARLY with needs_confirmation=true and the SMART
// detail — BEFORE hashing the whole drive — so the operator can copy critical data
// off a dying disk first. Re-call with confirm=true to inventory it anyway (the
// read is non-destructive; nothing is ever written to the drive).
func (a *App) IngestDrive(sessionID int, mountPath, serial, label, mode, level string, confirm bool, progress func(float64, string)) (res map[string]any, err error) {
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return nil, cfgErr
	}
	ds := a.Store.DockSession(sessionID)
	if ds == nil {
		return nil, fmt.Errorf("dock session %d not found", sessionID)
	}
	if ds.Status != "ACTIVE" {
		return nil, fmt.Errorf("dock session %d is %s — start a new session", sessionID, ds.Status)
	}
	if strings.TrimSpace(mountPath) == "" {
		return nil, fmt.Errorf("mount_path (the docked drive) required")
	}
	if fi, err := os.Stat(mountPath); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("cannot read docked drive %s", mountPath)
	}
	// The drive's top folder must be listable, or there is nothing to scan.
	if _, err := os.ReadDir(mountPath); err != nil {
		return nil, fmt.Errorf("cannot read the docked drive %s: %s", mountPath, scanReason(err))
	}
	// A registered source is never a docked drive, by where it really is: checked
	// before any volume is registered.
	if err := a.Store.CheckOutsideSources(mountPath); err != nil {
		return nil, err
	}
	// Batch catalog writes across the ingest (idempotent: matched by content hash).
	a.Store.BeginBatch()
	defer endBatchInto(a.Store, &err)

	progress(0.02, "identifying drive")
	// An explicit serial (from the watcher, which already resolved the candidate,
	// or a caller) is authoritative — we trust it and skip re-probing the device.
	// Otherwise we resolve the drive's identity live from the mount.
	explicitSerial := strings.TrimSpace(serial) != ""
	serial = strings.TrimSpace(serial)
	if !explicitSerial {
		if id, err := resolveDeviceIdentity(mountPath); err == nil {
			serial = id.Serial
		}
	}
	// Match or register the Volume — idempotent by serial across sessions.
	var vol *Volume
	if serial != "" {
		vol = a.Store.VolumeBySerial(serial)
	}
	reinserted := vol != nil
	if vol == nil {
		vol = a.Store.AddVolume(Volume{Label: nonEmpty(label, mountLabel(mountPath)), Kind: "HDD",
			Serial: serial, Notes: "ingested via dock"})
	}
	// Capture / refresh the live physical identity (best-effort, non-fatal). Only
	// when the serial wasn't supplied — a supplied serial means identity is
	// already settled and re-probing a stand-in mount must not clobber it.
	if !explicitSerial {
		if _, changed := a.resolveVolumeIdentity(vol, mountPath); changed {
			a.Store.UpdateVolume(vol)
		}
	}

	// Drive-mortality snapshot — a COMPLEMENT to the content verification below,
	// never a substitute. Best-effort and silent: smartctl absent, a masked USB
	// bridge, or a permission wall just records nothing and never fails the
	// ingest. Snapshots accrue in the volume's history so trends show across
	// dock sessions.
	var health *SmartSnapshot
	if snap, herr := a.volumeHealth(vol, mountPath, cfg); herr == nil {
		health = snap
	}

	// Pre-flight failure gate — BEFORE the long full-drive read. If SMART is
	// shouting imminent failure (reallocated/pending sectors, FAILING self-test,
	// NVMe wear-out), stop and let the operator rescue critical data off the drive
	// first. This is advisory only: the inventory read is non-destructive and never
	// writes to the drive, so confirm=true proceeds normally. A drive with no SMART
	// (smartctl absent / USB bridge) has health==nil and is never gated.
	if health != nil && health.Advisory && !confirm {
		a.Store.Log("dock", fmt.Sprintf("session %d: %s SMART advisory — %s (awaiting operator confirmation before inventory)",
			ds.ID, vol.Label, health.AdvisoryWhy))
		return map[string]any{
			"needs_confirmation": true,
			"volume_id":          vol.ID, "serial": vol.Serial, "label": vol.Label,
			"health": health,
			"message": "Warning: This drive may be failing — " + health.AdvisoryWhy + ". " +
				"Copy critical data off it first? Or continue the inventory anyway — the read is non-destructive and never writes to the drive.",
		}, nil
	}

	// A drive we already hold mirror data for is a RE-VERIFY, not a re-adopt.
	hasMirror := a.volumeHasMirror(vol.ID, ds.ArchiveIDs)
	effMode := "adopt"
	if strings.EqualFold(mode, "reverify") || (mode == "" && reinserted && hasMirror) {
		effMode = "reverify"
	}

	// A re-verify may run at a cheaper level (A census / C sample) — a path-based
	// check of the known mirror instead of the full content re-hash. Adoption and
	// level B always do the full content match.
	var drive *DockDrive
	// err is the named result (see the signature) so a failed final catalog flush can
	// be folded into it by endBatchInto; it is no longer declared locally here.
	if effMode == "reverify" && normLevel(level) != VerifyB {
		drive, err = a.dockReverifyAtLevel(ds, mountPath, vol, normLevel(level), progress)
	} else {
		drive, err = a.mirrorAdopt(ds, mountPath, vol, effMode, health, progress)
	}
	if err != nil {
		return nil, err
	}
	drive.Letter = mountPath
	a.Store.RecordDockDrive(ds, *drive)
	a.Store.Log("dock", fmt.Sprintf("session %d: %s %s (%d matched, %d historical, %d unreadable)",
		ds.ID, effMode, vol.Label, drive.Matched, drive.Historical, drive.Unreadable))

	progress(1.0, "DONE — safe to eject")
	out := map[string]any{
		"volume_id": vol.ID, "serial": vol.Serial, "label": vol.Label, "mode": effMode,
		"reinserted": reinserted, "matched": drive.Matched, "matched_bytes": drive.MatchedBytes,
		"historical": drive.Historical, "other": drive.Other, "unreadable": drive.Unreadable,
		"note": drive.Note, "health": health,
		"message":  "DONE — safe to eject. Insert the next drive.",
		"coverage": a.archiveCoverage(ds.ArchiveIDs),
	}
	// Whether the drive was read in full. Absent on the cheap A/C re-verify, which
	// does not walk the drive.
	if drive.Scan != "" {
		out["scan"], out["complete"] = drive.Scan, drive.Scan == ScanComplete
		out["unreadable_folders"], out["unknown"] = drive.UnreadableFolders, drive.Unknown
	}
	// The per-drive ingest report: role breakdown, duplicates vs. already-cataloged
	// drives, folder-overlap, and drive-mirror detection with the location verdict.
	// Built from the snapshot mirrorAdopt just stored (nil on the cheap A/C re-verify
	// path, which does not re-walk the drive).
	if snap := a.Store.VolumeSnapshot(vol.ID); snap != nil {
		out["report"] = a.driveReport(snap)
	}
	return out, nil
}

// mirrorAdopt hashes EVERY loose file on the drive in one read pass and does two
// things with each: (1) records it in the drive's SNAPSHOT — the complete offline
// inventory (tree, size, mtime, SHA-256 + BLAKE3, workflow role, best-effort EXIF
// capture time + camera serial) that lets the catalog describe the drive unplugged;
// and (2) matches it by CONTENT against the selected archives, recording matches as
// an ADOPTED-VERIFIED mirror package per archive (with a verified copy on this
// volume). It does NOT write anything onto the drive: adopted media are read-only,
// so the inventory lives ONLY in the catalog (the snapshot), unlike tool-written
// media which still carry a sidecar (see docs/ARCHITECTURE.md on this asymmetry).
func (a *App) mirrorAdopt(ds *DockSession, mountPath string, vol *Volume, mode string, health *SmartSnapshot, progress func(float64, string)) (*DockDrive, error) {
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return nil, cfgErr
	}
	// Source-safety: refuse to treat a registered source folder as a docked drive.
	// (We no longer write to the drive at all, but adopting a NAS source AS a drive
	// would double-count it and is never intended — read-only or not.)
	if err := a.Store.CheckOutsideSources(mountPath); err != nil {
		return nil, err
	}

	// Content indexes from the selected archives. The dock first-pass matches by
	// BLAKE3 (the fast hot-loop hash) when the catalog has it, falling back to
	// SHA-256 for files scanned before BLAKE3 was recorded. Both keys point at the
	// same File, so a drive file need only match on one.
	currentByHash := map[string][]*File{}   // current source file(s) by SHA-256
	currentByBlake3 := map[string][]*File{} // ...and by BLAKE3 (fast path)
	archiveOfFile := map[int]int{}          // fileID -> archiveID
	histHashes := map[string]bool{}         // previously-packaged versions no longer current
	for _, aid := range ds.ArchiveIDs {
		for _, f := range a.Store.FilesOf(aid) {
			if f.Hash != "" {
				currentByHash[f.Hash] = append(currentByHash[f.Hash], f)
			}
			if f.Blake3 != "" {
				currentByBlake3[f.Blake3] = append(currentByBlake3[f.Blake3], f)
			}
			archiveOfFile[f.ID] = aid
		}
	}
	for _, c := range a.Store.Chunks(0) {
		if !containsInt(ds.ArchiveIDs, c.CollectionID) {
			continue
		}
		for _, ref := range c.Files {
			if ref.Hash == "" {
				continue
			}
			if _, cur := currentByHash[ref.Hash]; !cur {
				histHashes[ref.Hash] = true
			}
		}
	}

	// Walk and hash the drive (recording what it cannot read), and classify each
	// file by content against the archives.
	progress(0.06, "scanning drive")
	matchedByArchive := map[int]map[int]*File{} // archiveID -> fileID -> File
	var matchedBytes int64
	var hist, other int
	snapFiles, gaps, unreadable, serr := a.scanTree(mountPath, cfg,
		func(done, total int) {
			progress(0.06+float64(done)/float64(max(total, 1))*0.74, progStats(0, 0, int64(done), int64(total), "hashing drive files"))
		},
		func(sf SnapFile) {
			// BLAKE3 first (fast path), then SHA-256 for legacy catalog entries.
			fs, ok := currentByBlake3[sf.Blake3]
			if !ok {
				fs, ok = currentByHash[sf.Hash]
			}
			if ok {
				for _, f := range fs {
					aid := archiveOfFile[f.ID]
					if matchedByArchive[aid] == nil {
						matchedByArchive[aid] = map[int]*File{}
					}
					if _, seen := matchedByArchive[aid][f.ID]; !seen {
						matchedByArchive[aid][f.ID] = f
						matchedBytes += f.SizeBytes
					}
					// Backfill media metadata onto the matched archive file if a prior
					// scan/adopt didn't capture it (e.g. cataloged before this feature).
					if f.ShotAt.IsZero() && !sf.ShotAt.IsZero() {
						f.ShotAt = sf.ShotAt
					}
					if f.Role == "" && sf.Role != "" {
						f.Role = sf.Role
					}
					if f.CameraSerial == "" && sf.CameraSerial != "" {
						f.CameraSerial = sf.CameraSerial
					}
				}
			} else if histHashes[sf.Hash] {
				hist++
			} else {
				other++
			}
		})
	if serr != nil {
		return nil, fmt.Errorf("cannot read the docked drive %s: %s", mountPath, scanReason(serr))
	}

	snap := a.buildSnapshot(ds, vol, health, snapFiles, unreadable, gaps)
	// The scan's own record and file list are saved before the snapshot and the
	// mirror records change, so a scan that cannot be recorded fails the ingest and
	// leaves those as they were.
	if _, err := a.recordScan(snap, cfg.ScanHistoryRetained); err != nil {
		return nil, err
	}

	// What a partial scan could not read is unknown, not gone. A file an earlier
	// scan matched on this drive, and this scan did not see, is kept apart as
	// unknown (never counted as a copy or re-stamped as verified) unless the
	// previous scan saw that content only in places this scan did read, which shows
	// it is no longer there. Only the archive's current version is kept.
	knownAbsent := func(string) bool { return false }
	// Only a snapshot of the whole drive says where things were on it.
	if prev := a.Store.VolumeSnapshot(vol.ID); prev != nil && prev.Root == "" {
		seenAt := map[string]bool{} // hash -> every place the previous scan saw it is known now
		for _, f := range prev.Files {
			if f.Hash == "" {
				continue
			}
			known, ok := seenAt[f.Hash]
			seenAt[f.Hash] = (known || !ok) && snap.Known(f.RelPath)
		}
		knownAbsent = func(h string) bool { return seenAt[h] }
	}

	// Record the matches as a verified mirror package per archive.
	progress(0.84, "recording matches")
	totalMatched, unknownFiles := 0, 0
	for _, aid := range ds.ArchiveIDs {
		fm := matchedByArchive[aid]
		refs := make([]ChunkFileRef, 0, len(fm))
		for _, f := range fm {
			refs = append(refs, ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: f.SizeBytes, Hash: f.Hash})
		}
		var unknown []ChunkFileRef
		if prior := a.mirrorChunkOn(aid, vol.ID); prior != nil && snap.Scan == ScanPartial {
			current := map[int]string{}
			for _, f := range a.Store.FilesOf(aid) {
				current[f.ID] = f.Hash
			}
			seen := map[int]bool{}
			for _, r := range append(append([]ChunkFileRef(nil), prior.Files...), prior.UnknownFiles...) {
				if _, matched := fm[r.FileID]; matched || seen[r.FileID] {
					continue
				}
				seen[r.FileID] = true
				if h, ok := current[r.FileID]; ok && r.Hash != "" && h == r.Hash && !knownAbsent(r.Hash) {
					unknown = append(unknown, r)
				}
			}
		}
		if len(refs) == 0 {
			// None of this archive's files were found. What the drive's record held
			// is gone, or unknown where this scan could not read; the copy is not
			// verified again by a scan that found nothing.
			if p := a.mirrorChunkOn(aid, vol.ID); p != nil && (len(p.Files) > 0 || len(p.UnknownFiles) > 0) {
				gone := 0
				for _, r := range p.Files {
					if !containsRef(unknown, r.FileID) {
						gone++
					}
				}
				p.Files, p.FileCount, p.EncBytes, p.DataBytes = nil, 0, 0, 0
				p.UnknownFiles = unknown
				a.Store.UpdateChunk(p)
				a.Store.AppendVerifyEvent(p, VerifyEvent{At: time.Now().UTC(), OK: false, Path: mountPath, Advisory: true,
					Note: fmt.Sprintf("this scan found none of the archive's files here: %d no longer on the drive, %d unknown (in places it could not read)", gone, len(unknown))})
			}
			unknownFiles += len(unknown)
			continue
		}
		sort.Slice(refs, func(i, j int) bool { return refs[i].RelPath < refs[j].RelPath })
		sort.Slice(unknown, func(i, j int) bool { return unknown[i].RelPath < unknown[j].RelPath })
		c := a.upsertMirrorChunk(aid, vol, mountPath, refs, mode)
		c.UnknownFiles = unknown
		a.Store.UpdateChunk(c)
		if len(unknown) > 0 {
			a.Store.AppendVerifyEvent(c, VerifyEvent{At: time.Now().UTC(), OK: true, Path: mountPath, Advisory: true,
				Note: fmt.Sprintf("%d file(s) an earlier scan found here are unknown: this scan could not read where they were, so they are not counted until a scan reads them again", len(unknown))})
		}
		totalMatched += len(fm)
		unknownFiles += len(unknown)
	}

	d := &DockDrive{VolumeID: vol.ID, Serial: vol.Serial, Label: vol.Label, Mode: mode,
		Matched: totalMatched, MatchedBytes: matchedBytes, Historical: hist, Other: other, Unreadable: unreadable,
		Scan: snap.Scan, UnreadableFolders: len(snap.UnreadableFolders), Unknown: unknownFiles}
	if snap.Scan == ScanPartial {
		d.Note = fmt.Sprintf("partial scan: %d folder(s) and %d file(s) could not be read; what they hold is unknown", len(snap.UnreadableFolders), len(snap.UnreadableFiles))
	}

	// Store the drive's SNAPSHOT — the full offline inventory. NO sidecar is written
	// to the drive: adopted media are read-only, so the inventory lives in the
	// catalog alone (see this function's doc + docs/ARCHITECTURE.md).
	progress(0.92, "recording drive snapshot")
	if err := a.Store.PutVolumeSnapshot(snap); err != nil {
		return nil, fmt.Errorf("saving the drive snapshot: %w", err)
	}

	if mode == "reverify" {
		if d.Note != "" {
			d.Note += " · "
		}
		d.Note += "re-verified (drive recognized by serial)"
	}
	return d, nil
}

// buildSnapshot assembles a VolumeSnapshot from the walk's per-file records plus
// the drive's identity and SMART state, with role rollups denormalized for the
// offline view. Files are sorted by path for a stable, browsable tree.
func (a *App) buildSnapshot(ds *DockSession, vol *Volume, health *SmartSnapshot, files []SnapFile, unreadable int, gaps scanGaps) *VolumeSnapshot {
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	gaps.sort()
	snap := &VolumeSnapshot{
		VolumeID: vol.ID, CapturedAt: time.Now().UTC(), SessionID: ds.ID,
		Label: vol.Label, Serial: vol.Serial, Model: vol.Model, DeviceSize: vol.DeviceSize,
		LocationID: vol.LocationID, Smart: health, Files: files, Unreadable: unreadable,
		Scan: gaps.completeness(), UnreadableFolders: gaps.unreadableFolders,
		UnreadableFiles: gaps.unreadableFiles, SkippedFolders: gaps.skippedFolders,
		RoleFiles: map[string]int{}, RoleBytes: map[string]int64{},
	}
	for _, f := range files {
		snap.TotalFiles++
		snap.TotalBytes += f.SizeBytes
		role := f.Role
		if role == "" {
			role = RoleOther
		}
		snap.RoleFiles[role]++
		snap.RoleBytes[role] += f.SizeBytes
	}
	return snap
}

// dockWalkDir walks a docked drive. A variable so a test can make the walk fail.
var dockWalkDir = filepath.WalkDir

// scanTree walks root and hashes every file under it, read-only, recording each
// folder it cannot list, each file it cannot read and each system folder it skips.
// each is called for every file hashed, one at a time, with its snapshot entry
// (role, and best-effort media metadata). It returns the entries, the gaps and
// the number of files that could not be hashed; an error means root itself could
// not be read and nothing was scanned.
func (a *App) scanTree(root string, cfg Config, progress func(done, total int), each func(SnapFile)) ([]SnapFile, scanGaps, int, error) {
	var paths []string
	var gaps scanGaps
	if err := dockWalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err // the top folder itself cannot be read: no scan at all
			}
			gaps.unreadableFolders = append(gaps.unreadableFolders, ScanGap{RelPath: driveRel(root, p), Reason: scanReason(err)})
			return nil // the rest is still scanned
		}
		if d.IsDir() {
			if p != root && skipDockDir(d.Name()) {
				gaps.skippedFolders = append(gaps.skippedFolders, ScanGap{RelPath: driveRel(root, p), Reason: "system folder, not scanned"})
				return filepath.SkipDir
			}
			return nil
		}
		paths = append(paths, p)
		return nil
	}); err != nil {
		return nil, scanGaps{}, 0, err
	}
	reg := a.formatRegistry()
	var mu sync.Mutex
	files := make([]SnapFile, 0, len(paths))
	parallelHash(paths,
		func(done int) { progress(done, len(paths)) },
		func(p, sha, b3 string, size int64, mtime time.Time) {
			// Role, and for still-image roles the EXIF capture time + camera body
			// serial (bounded header read, outside the lock; unparseable = empty).
			rel := driveRel(root, p)
			role, crit := classifyRole(reg, rel)
			sf := SnapFile{RelPath: rel, SizeBytes: size, ModTime: mtime, Hash: sha, Blake3: b3, Role: role, Critical: crit}
			sf.ShotAt, sf.CameraSerial = a.extractMediaMeta(p, role, cfg)
			mu.Lock()
			defer mu.Unlock()
			files = append(files, sf)
			if each != nil {
				each(sf)
			}
		},
		func(p, kind, msg string) {
			mu.Lock()
			defer mu.Unlock()
			gaps.unreadableFiles = append(gaps.unreadableFiles, ScanGap{RelPath: driveRel(root, p), Reason: fileScanReason(kind)})
		})
	return files, gaps, len(paths) - len(files), nil
}

// scanGaps collects what a drive scan did not read.
type scanGaps struct {
	unreadableFolders, unreadableFiles, skippedFolders []ScanGap
}

func (g *scanGaps) sort() {
	for _, l := range [][]ScanGap{g.unreadableFolders, g.unreadableFiles, g.skippedFolders} {
		sort.Slice(l, func(i, j int) bool { return l[i].RelPath < l[j].RelPath })
	}
}

// completeness is ScanComplete when the scan read every folder and file it was
// meant to (system folders are skipped by design), else ScanPartial.
func (g scanGaps) completeness() string {
	if len(g.unreadableFolders) > 0 || len(g.unreadableFiles) > 0 {
		return ScanPartial
	}
	return ScanComplete
}

// scanReason is why a path could not be read, without the path itself.
func scanReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.Is(err, fs.ErrNotExist):
		return "disappeared during the scan"
	}
	return "read error"
}

// fileScanReason words a hashing problem kind (see parallelHash) for the record.
func fileScanReason(kind string) string {
	switch kind {
	case "unsupported":
		return "not a regular file"
	case "stat":
		return "cannot be opened"
	}
	return "read error"
}

// driveRel returns p as a forward-slashed path relative to the drive mount root,
// so snapshot paths are stable and browsable regardless of mount letter.
func driveRel(mountPath, p string) string {
	if rel, err := filepath.Rel(mountPath, p); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(p)
}

// dockReverifyAtLevel does a cheap path-based re-verify of the mirror(s) already
// recorded for this volume at level A or C — checking each cataloged file at
// mountPath/relpath, skipping the full content re-hash. Advisory only (never
// satisfies COMPLETE or refreshes verify-due); a level-B re-verify uses the full
// content-match path (mirrorAdopt) instead.
func (a *App) dockReverifyAtLevel(ds *DockSession, mountPath string, vol *Volume, level string, progress func(float64, string)) (*DockDrive, error) {
	progress(0.1, "re-verifying ("+levelName(level)+")")
	matched, bad := 0, 0
	for _, aid := range ds.ArchiveIDs {
		for _, c := range a.Store.Chunks(aid) {
			// A folder's record is checked at its folder, not at the drive root.
			if !c.Mirror || c.MirrorRoot != "" {
				continue
			}
			on := false
			for _, cp := range c.Copies {
				if cp.VolumeID == vol.ID && !cp.Superseded {
					on = true
					break
				}
			}
			if !on {
				continue
			}
			ok, checked, b, firstBad := verifyMirrorChunk(c, mountPath, level)
			matched += checked - b
			bad += b
			now := time.Now().UTC()
			note := fmt.Sprintf("dock re-verify (%s): %d/%d ok", levelTag(level), checked-b, checked)
			if !ok {
				note += " · first bad: " + firstBad
			}
			a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: ok, Path: mountPath, Note: note, Level: level, Advisory: true})
			a.Store.CreditCopyCheck(c, CopyAt{Path: mountPath, VolumeID: vol.ID}, ok, level)
		}
	}
	note := fmt.Sprintf("re-verified at level %s (advisory — only full/B satisfies protection)", level)
	if bad > 0 {
		note += fmt.Sprintf(" · %d file(s) failed the %s check", bad, levelName(level))
	}
	return &DockDrive{VolumeID: vol.ID, Serial: vol.Serial, Label: vol.Label, Mode: "reverify",
		Matched: matched, Note: note}, nil
}

// upsertMirrorChunk creates or refreshes the ADOPTED-VERIFIED mirror package for
// (archive, volume): the drive's content-matched files, with a verified copy on
// the volume. Idempotent — a re-ingest updates the same package in place.
func (a *App) upsertMirrorChunk(archiveID int, vol *Volume, mountPath string, refs []ChunkFileRef, mode string) *Chunk {
	return a.upsertMirrorChunkAt(archiveID, vol, "", mountPath, refs, mode)
}

// upsertMirrorChunkAt is upsertMirrorChunk for the record of one folder (root,
// as adopted) on the drive; "" is the record of the whole drive or destination.
// Each folder keeps its own record, so adopting one folder never replaces
// another's files.
func (a *App) upsertMirrorChunkAt(archiveID int, vol *Volume, root, mountPath string, refs []ChunkFileRef, mode string) *Chunk {
	var sum int64
	for _, r := range refs {
		sum += r.SizeBytes
	}
	now := time.Now().UTC()
	ok := true
	note := "dock mirror adopt"
	switch mode {
	case "reverify":
		note = "dock re-verify"
	case "mirror":
		note = "mirror backup written (copy-then-verified)"
	}
	if c := a.mirrorChunkAt(archiveID, vol.ID, root); c != nil {
		c.Files, c.FileCount, c.EncBytes, c.DataBytes = refs, len(refs), sum, sum
		c.WrittenDest, c.VerifiedAt, c.VerifyOK, c.Status = mountPath, &now, &ok, StatusAdoptedVerified
		a.Store.RecordCopy(c, vol.ID, mountPath, true)
		a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: true, Path: mountPath, Note: note})
		a.Store.UpdateChunk(c)
		return c
	}
	coll := a.Store.Collection(archiveID)
	safe := "ARCHIVE"
	if coll != nil {
		safe = safeName(coll.Name)
	}
	name := fmt.Sprintf("MIRROR-%s-V%d", safe, vol.ID)
	if root != "" {
		name += "-" + safeName(filepath.Base(root)) + "-" + sha256Hex([]byte(normPath(root)))[:6]
	}
	c := a.Store.AddChunk(Chunk{CollectionID: archiveID, MirrorRoot: root,
		Name: name, Status: StatusAdoptedVerified,
		MediaKind: nonEmpty(vol.Kind, "HDD"), EncBytes: sum, DataBytes: sum, FileCount: len(refs),
		HashAlg: "SHA256", Encrypted: false, Adopted: true, Mirror: true, WrittenDest: mountPath,
		Files: refs, WrittenAt: &now, VerifiedAt: &now, VerifyOK: &ok})
	a.Store.RecordCopy(c, vol.ID, mountPath, true)
	a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: true, Path: mountPath, Note: note})
	return c
}

// containsRef reports whether refs holds the file fileID.
func containsRef(refs []ChunkFileRef, fileID int) bool {
	for _, r := range refs {
		if r.FileID == fileID {
			return true
		}
	}
	return false
}

// mirrorChunkOn is the archive's mirror package with a current copy on the
// volume, or nil.
func (a *App) mirrorChunkOn(archiveID, volumeID int) *Chunk {
	return a.mirrorChunkAt(archiveID, volumeID, "")
}

// mirrorChunkAt is the archive's mirror record of folder root on the volume
// ("" for the whole drive), or nil.
func (a *App) mirrorChunkAt(archiveID, volumeID int, root string) *Chunk {
	for _, c := range a.Store.Chunks(archiveID) {
		if !c.Mirror || normPath(c.MirrorRoot) != normPath(root) {
			continue
		}
		for _, cp := range c.Copies {
			if cp.VolumeID == volumeID && !cp.Superseded {
				return c
			}
		}
	}
	return nil
}

// volumeHasMirror reports whether the volume already holds mirror data for any
// of the given archives (a current copy on a mirror package).
func (a *App) volumeHasMirror(volumeID int, archiveIDs []int) bool {
	for _, c := range a.Store.Chunks(0) {
		if !c.Mirror || !containsInt(archiveIDs, c.CollectionID) {
			continue
		}
		for _, cp := range c.Copies {
			if cp.VolumeID == volumeID && !cp.Superseded {
				return true
			}
		}
	}
	return false
}

// ---- coverage ----------------------------------------------------------

type CoverageArchive struct {
	ArchiveID    int     `json:"archive_id"`
	Name         string  `json:"name"`
	TotalFiles   int     `json:"total_files"`
	CoveredFiles int     `json:"covered_files"`
	Uncovered    int     `json:"uncovered_files"`
	Unknown      int     `json:"unknown_files,omitempty"` // not covered, but on a drive a partial scan could not read
	Pct          float64 `json:"pct"`
	TotalBytes   int64   `json:"total_bytes"`
	CoveredBytes int64   `json:"covered_bytes"`
}

type Coverage struct {
	Archives     []CoverageArchive `json:"archives"`
	TotalFiles   int               `json:"total_files"`
	CoveredFiles int               `json:"covered_files"`
	Uncovered    int               `json:"uncovered_files"`
	Pct          float64           `json:"pct"`
	TotalBytes   int64             `json:"total_bytes"`
	CoveredBytes int64             `json:"covered_bytes"`
	Unknown      int               `json:"unknown_files,omitempty"` // not covered, but on a drive a partial scan could not read
}

// archiveCoverage computes, for the selected archives, how many source files now
// have ≥1 verified copy anywhere (any chunk — packaged or mirror — with a
// verified copy). A file counts as covered by fileID or by content hash.
func (a *App) archiveCoverage(archiveIDs []int) Coverage {
	coveredIDs := map[int]bool{}
	coveredHashes := map[string]bool{}
	unknownIDs := map[int]bool{} // on a drive a partial scan could not read
	for _, c := range a.Store.Chunks(0) {
		if c.VerifiedCopyCount() == 0 {
			continue
		}
		for _, ref := range c.Files {
			if ref.FileID > 0 {
				coveredIDs[ref.FileID] = true
			}
			if ref.Hash != "" {
				coveredHashes[ref.Hash] = true
			}
		}
		for _, ref := range c.UnknownFiles {
			unknownIDs[ref.FileID] = true
		}
	}
	var cov Coverage
	for _, aid := range archiveIDs {
		coll := a.Store.Collection(aid)
		if coll == nil {
			continue
		}
		ca := CoverageArchive{ArchiveID: aid, Name: coll.Name}
		for _, f := range a.Store.FilesOf(aid) {
			ca.TotalFiles++
			ca.TotalBytes += f.SizeBytes
			if coveredIDs[f.ID] || (f.Hash != "" && coveredHashes[f.Hash]) {
				ca.CoveredFiles++
				ca.CoveredBytes += f.SizeBytes
			} else if unknownIDs[f.ID] {
				ca.Unknown++
			}
		}
		ca.Uncovered = ca.TotalFiles - ca.CoveredFiles
		if ca.TotalFiles > 0 {
			ca.Pct = round1(float64(ca.CoveredFiles) / float64(ca.TotalFiles) * 100)
		}
		cov.Archives = append(cov.Archives, ca)
		cov.TotalFiles += ca.TotalFiles
		cov.CoveredFiles += ca.CoveredFiles
		cov.TotalBytes += ca.TotalBytes
		cov.CoveredBytes += ca.CoveredBytes
		cov.Unknown += ca.Unknown
	}
	cov.Uncovered = cov.TotalFiles - cov.CoveredFiles
	if cov.TotalFiles > 0 {
		cov.Pct = round1(float64(cov.CoveredFiles) / float64(cov.TotalFiles) * 100)
	}
	return cov
}

// ---- session report ----------------------------------------------------
//
// Note the deliberate asymmetry: the dock writes NO inventory sidecar onto an
// adopted drive (adopted media are read-only — the inventory lives in the catalog
// snapshot alone). Tool-WRITTEN media still carry their sidecar at seal time. See
// docs/ARCHITECTURE.md.

// SessionReportMarkdown is the exportable documentation trail: every drive's
// serial/label/contents summary plus running coverage of the selected archives.
func (a *App) SessionReportMarkdown(sessionID int) (string, error) {
	ds := a.Store.DockSession(sessionID)
	if ds == nil {
		return "", fmt.Errorf("dock session %d not found", sessionID)
	}
	cov := a.archiveCoverage(ds.ArchiveIDs)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Dock session %d\n\n", ds.ID))
	b.WriteString(fmt.Sprintf("Started %s · status **%s** · %d drive(s) processed.\n\n",
		ds.StartedAt.Format(time.RFC3339), ds.Status, len(ds.Drives)))

	b.WriteString("## Archives reconciled\n\n")
	for _, ca := range cov.Archives {
		b.WriteString(fmt.Sprintf("- **%s** — %.1f%% covered (%d/%d files with ≥1 verified copy; %d still on 0 copies)\n",
			ca.Name, ca.Pct, ca.CoveredFiles, ca.TotalFiles, ca.Uncovered))
	}
	b.WriteString(fmt.Sprintf("\n**Overall:** %.1f%% — %d of %d files covered, **%d files still on 0 copies**.\n\n",
		cov.Pct, cov.CoveredFiles, cov.TotalFiles, cov.Uncovered))

	b.WriteString("## Drives processed\n\n")
	if len(ds.Drives) == 0 {
		b.WriteString("_None yet._\n\n")
	} else {
		b.WriteString("| # | Label | Serial | Pass | Matched | Matched GB | Historical | Unreadable | Finished |\n")
		b.WriteString("|--:|-------|--------|------|--------:|-----------:|-----------:|-----------:|----------|\n")
		for i, d := range ds.Drives {
			serial := d.Serial
			if serial == "" {
				serial = "—"
			}
			fin := ""
			if d.FinishedAt != nil {
				fin = d.FinishedAt.Format("2006-01-02 15:04")
			}
			b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %d | %.2f | %d | %d | %s |\n",
				i+1, mdCell(d.Label), mdCell(serial), d.Mode, d.Matched,
				float64(d.MatchedBytes)/1e9, d.Historical, d.Unreadable, fin))
		}
	}
	b.WriteString("\nObelisk treated every source folder as READ-ONLY: NAS paths were only hashed for comparison, never written.\n")
	return b.String(), nil
}

// ---- small helpers -----------------------------------------------------

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// mountLabel derives a fallback volume label from a mount path.
func mountLabel(mountPath string) string {
	base := filepath.Base(filepath.Clean(mountPath))
	if base == "" || base == "." || base == string(filepath.Separator) || strings.HasSuffix(base, ":") {
		return "DOCK-DRIVE"
	}
	return base
}

// safeName reduces an archive name to a compact token for a package name.
func safeName(name string) string {
	s := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)
	s = strings.Trim(s, "-")
	if len(s) > 24 {
		s = s[:24]
	}
	if s == "" {
		s = "ARCHIVE"
	}
	return s
}

// skipDockDir lists directory names the drive walk ignores: our own sidecar and
// common OS bookkeeping that is unreadable or irrelevant.
func skipDockDir(name string) bool {
	switch name {
	case dockSidecarDir, dockSidecarDirLegacy, sealSidecarDir, sealSidecarDirLegacy,
		"System Volume Information", "$RECYCLE.BIN", ".Trashes", ".Spotlight-V100", ".fseventsd":
		return true
	}
	return false
}
