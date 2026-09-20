package main

// store.go — the catalog.
//
// v1 deliberately uses a single JSON file with atomic writes instead of
// SQLite: zero CGO, zero external services, trivially inspectable with
// any text editor 30 years from now. The Store interface surface is
// small; swapping in SQLite (modernc.org/sqlite, pure Go) later touches
// only this file.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// normPath canonicalises a filesystem path for comparison: cleaned, forward-
// slashed, and case-folded on case-insensitive platforms (Windows). This lets
// copy matching survive separator/case differences between how a copy's path
// was stored and how a verify destination is later typed (e.g. T:/ vs T:\).
func normPath(p string) string {
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(filepath.Clean(p))
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// pathRelated reports whether a and b name the same location or one contains the
// other, comparing at path-segment boundaries (so /a/b never matches /a/bc).
func pathRelated(a, b string) bool {
	a, b = normPath(a), normPath(b)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// underScopePrefix reports whether a file's full (folder-joined) path falls within a
// folder-scope prefix. An empty prefix means "whole archive" — everything matches.
// Matching is at path-segment boundaries via normPath, so a prefix ".../2019" never
// captures ".../2019-raw". This is the single membership test behind every
// folder-scoped action (back up, mirror, plan, rescan): a filter over the catalog,
// never a change to it.
func underScopePrefix(full, prefix string) bool {
	if strings.TrimSpace(prefix) == "" {
		return true
	}
	nf, np := normPath(full), normPath(prefix)
	return nf == np || strings.HasPrefix(nf, np+"/")
}

// scopeDisplay renders a folder-scope prefix as a short human path for job labels
// and history names: the owning scanned folder's leaf name plus the remainder
// beneath it (e.g. "Personal Photos/2019"). An empty prefix returns "". Must be
// called without s.mu held (it takes the lock via FoldersOf).
func (s *Store) scopeDisplay(collectionID int, prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	P := strings.TrimRight(filepath.ToSlash(prefix), "/")
	nP := normPath(P)
	root := ""
	for _, fo := range s.FoldersOf(collectionID) {
		fp := filepath.ToSlash(fo.Path)
		nf := normPath(fp)
		if (nP == nf || strings.HasPrefix(nP, nf+"/")) && len(fp) > len(root) {
			root = fp
		}
	}
	if root == "" {
		return lastSeg(P)
	}
	if len(P) > len(root) {
		return lastSeg(root) + "/" + P[len(root)+1:]
	}
	return lastSeg(root)
}

// pathExt returns a file's lowercased extension (".nef"), for the search filter.
func pathExt(rel string) string { return strings.ToLower(filepath.Ext(rel)) }

// Archive kinds. SOURCED (default/legacy blank) archives have source folders on a
// main store (NAS) that scan/drift compare against. SOURCELESS archives have NO
// source folders — their file list is the deduped union (by hash) of everything
// found on the media adopted into them; the union IS the truth (see AdoptFolder).
const (
	ArchiveSourced    = "SOURCED"
	ArchiveSourceless = "SOURCELESS"
)

type Collection struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	// Kind is SOURCED (blank legacy = sourced) or SOURCELESS. Blank tolerated as
	// SOURCED so pre-Kind catalogs keep their current behavior.
	Kind string `json:"kind,omitempty"`
	// Integrity is an optional per-archive override of the global integrity preset
	// (nil = inherit the global setting). It sits beside the archive's protection
	// Profile: the Profile says how many copies, Integrity says how hard each copy
	// is proven.
	Integrity *Integrity `json:"integrity,omitempty"`
	// Retired hides the archive as an entity from Home, the Archives table, dashboards,
	// and search defaults — it stops asking for attention. RetireHidden is the firmer
	// tier that ALSO excludes it from the global Files/Data-known totals. Both are fully
	// reversible; in both tiers the dedup index still recognizes its files (so a drive
	// holding them never reads as mystery). Only Remove makes the app truly forget.
	Retired      bool       `json:"retired,omitempty"`
	RetireHidden bool       `json:"retire_hidden,omitempty"`
	RetiredAt    *time.Time `json:"retired_at,omitempty"`
}

// IsSourceless reports whether this archive is defined by adopted media alone (no
// source folders; scan/drift disabled).
func (c *Collection) IsSourceless() bool { return strings.EqualFold(c.Kind, ArchiveSourceless) }

// IsRetired reports whether this archive has been retired (either tier).
func (c *Collection) IsRetired() bool { return c.Retired }

type Folder struct {
	ID           int    `json:"id"`
	CollectionID int    `json:"collection_id"`
	Path         string `json:"path"`
}

type File struct {
	ID           int    `json:"id"`
	CollectionID int    `json:"collection_id"`
	FolderID     int    `json:"folder_id"`
	RelPath      string `json:"rel_path"`
	SizeBytes    int64  `json:"size_bytes"`
	HashAlg      string `json:"hash_alg"`
	Hash         string `json:"hash"`
	// Blake3 is a fast, media-decoupled content-identity hash computed in the same
	// read pass as Hash during scans/drift/dock. It is CATALOG-ONLY — it accelerates
	// hot-loop comparisons and must NEVER be written to a medium (see hashing.go /
	// docs/ARCHITECTURE.md). SHA-256 (Hash) remains the sole on-media hash.
	Blake3 string `json:"blake3,omitempty"`
	// Archival version retention (prompt 50): the fields above describe the CURRENT
	// content; ModTime/FirstSeen timestamp it, and Versions is the append-only
	// history of superseded prior content. A rescan that finds new bytes moves the
	// outgoing {hash,size,mtime,first_seen} into Versions instead of discarding it —
	// old packages still hold those bytes, so the catalog stops pretending they are
	// gone. Empty/zero on legacy catalogs; backfilled lazily on next UpsertFile.
	ModTime   time.Time     `json:"mtime,omitempty"`
	FirstSeen time.Time     `json:"first_seen,omitempty"`
	Versions  []FileVersion `json:"versions,omitempty"` // superseded prior versions, oldest→newest
	// Media metadata (prompt: Templates + Events). ShotAt is the EXIF capture time,
	// Role the workflow classification (RAW/EDITED-EXPORT/…), CameraSerial the EXIF
	// body serial — all best-effort, filled during scan/adopt/dock when the file is
	// a still image the stdlib EXIF reader understands (see exif.go / classifyRole).
	// Empty/zero on non-images and legacy rows; they drive event clustering, the
	// routing tokens, and the date/role/event search filters.
	ShotAt       time.Time `json:"shot_at,omitempty"`
	Role         string    `json:"role,omitempty"`
	CameraSerial string    `json:"camera_serial,omitempty"`
	// EventID is the Event this file belongs to (0 = unassigned). Set by confirming a
	// clustered/harvested Event or accepting a magnet suggestion; the sole membership
	// record (Events don't store file lists). See events.go.
	EventID int `json:"event_id,omitempty"`
}

// FileVersion is a superseded prior state of a file, retained so the catalog
// remembers what earlier sealed packages still protect. Append-only; the current
// version lives on the File itself. Content is located by Hash — package and
// mirror membership is content-addressed (ChunkFileRef.Hash), so every retained
// version remains findable even after the file's current bytes moved on.
type FileVersion struct {
	Hash         string    `json:"hash"`
	HashAlg      string    `json:"hash_alg,omitempty"`
	SizeBytes    int64     `json:"size_bytes"`
	ModTime      time.Time `json:"mtime,omitempty"`
	FirstSeen    time.Time `json:"first_seen,omitempty"`
	SupersededAt time.Time `json:"superseded_at"`
}

type ChunkFileRef struct {
	FileID     int    `json:"file_id"`
	RelPath    string `json:"rel_path"`
	SizeBytes  int64  `json:"size_bytes"`
	Hash       string `json:"hash,omitempty"`        // full SHA-256 (level-B truth; for drift/reconcile)
	SampleHash string `json:"sample_hash,omitempty"` // SHA-256 of size + first & last 4 MiB (the level-C baseline)
}

type Chunk struct {
	ID              int                `json:"id"`
	CollectionID    int                `json:"collection_id"`
	Name            string             `json:"name"`
	Status          string             `json:"status"` // PLANNED BUILDING STAGED WRITING WRITTEN VERIFIED FAILED
	MediaKind       string             `json:"media_kind"`
	TargetBytes     int64              `json:"target_bytes"`
	DataBytes       int64              `json:"data_bytes"`
	EncBytes        int64              `json:"enc_bytes"`
	FileCount       int                `json:"file_count"`
	SrcRoot         string             `json:"src_root"`
	HashAlg         string             `json:"hash_alg"`
	TarHash         string             `json:"tar_hash"`
	EncHash         string             `json:"enc_hash"` // hash of the payload file as written to media (ciphertext when encrypted, tar when not)
	Encrypted       bool               `json:"encrypted"`
	KeyRef          string             `json:"key_ref"`
	PrivateManifest bool               `json:"private_manifest,omitempty"` // medium carries manifest.json.gpg, no plaintext listing
	Par2            int                `json:"par2_redundancy"`
	StagedDir       string             `json:"staged_dir"`
	WrittenDest     string             `json:"written_dest"`
	VerifyOK        *bool              `json:"verify_ok"`
	Error           string             `json:"error"`
	Files           []ChunkFileRef     `json:"files"`
	BuildTimings    map[string]float64 `json:"build_timings,omitempty"`   // per-stage seconds: tar, hash, stage_verify, encrypt/stage, crypt_verify, par2
	BuildVerified   *BuildVerified     `json:"build_verified,omitempty"`  // build-time proof the payload faithfully contains the source
	VerifyEvents    []VerifyEvent      `json:"verify_events,omitempty"`   // append-only integrity-check log
	RingStats       *RingStats         `json:"ring_stats,omitempty"`      // last write's ring-buffer telemetry
	BuildWarnings   []string           `json:"build_warnings,omitempty"`  // non-fatal build issues the operator should see (e.g. BagIt tag files that failed to write)
	Spanned         bool               `json:"spanned"`                   // payload split across several media
	Segments        []Segment          `json:"segments,omitempty"`        // one per medium/tape when Spanned
	Copies          []Copy             `json:"copies,omitempty"`          // physical copies of this chunk on registered volumes
	Adopted         bool               `json:"adopted,omitempty"`         // cataloged from pre-existing media, not built here
	Mirror          bool               `json:"mirror,omitempty"`          // content-hash-matched loose files on a drive (dock mirror adoption), not a packaged payload
	ListingUnknown  bool               `json:"listing_unknown,omitempty"` // adopted without a manifest/TOC — contents unenumerated
	CreatedAt       time.Time          `json:"created_at"`
	WrittenAt       *time.Time         `json:"written_at,omitempty"`
	VerifiedAt      *time.Time         `json:"verified_at,omitempty"`
}

// Statuses a chunk can hold. ADOPTED-VERIFIED marks a package cataloged from
// pre-existing media (payload hashed and recorded as truth) rather than built.
const StatusAdoptedVerified = "ADOPTED-VERIFIED"

// Segment is one medium's worth of a spanned chunk: a byte range of the
// finished payload (or the par2 set on its own tape when Par2 is set). The
// per-segment Hash is the SHA-256 of exactly those bytes as written, so each
// tape's read-back proves it holds its verified share; concatenating the
// segments in order reproduces the payload (whose whole-file hash is EncHash).
type Segment struct {
	Index    int    `json:"index"`               // 1-based
	Bytes    int64  `json:"bytes"`               // length of this segment
	Hash     string `json:"hash"`                // SHA-256 of the bytes as written (filled at write time)
	Status   string `json:"status"`              // PENDING WRITING WRITTEN VERIFIED FAILED
	Dest     string `json:"dest"`                // base destination mount last used for this segment
	VolumeID int    `json:"volume_id,omitempty"` // registered volume this segment's tape belongs to
	Par2     bool   `json:"par2,omitempty"`      // this "segment" is the par2 set on its own tape
}

// Volume is a physical medium the operator can hold and locate: a tape, a
// drive, a disc. Barcodes come straight off a USB scanner (which types like a
// keyboard). This is the "where do the Smiths' photos physically live?" record.
// Location is a first-class physical place a volume can live — e.g. "Shoe Box #1"
// (onsite) or "Grandma's house" (offsite). It is the single source of truth for
// the "1" in 3-2-1: a volume's offsite-ness derives from its Location's Offsite
// flag (see volumeOffsite). The legacy per-volume Offsite bool is kept working as
// a fallback for any volume not yet assigned a Location.
type Location struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Offsite   bool      `json:"offsite,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Volume struct {
	ID       int    `json:"id"`
	Label    string `json:"label"`
	Barcode  string `json:"barcode"`
	Kind     string `json:"kind"` // TAPE HDD SSD OPTICAL OTHER
	Location string `json:"location"`
	// LocationID points at the Location this volume lives in (0 = unassigned). It is
	// the modern replacement for the free-text Location + Offsite bool below; when
	// set, offsite math reads the Location's Offsite flag. Zero on pre-Location
	// catalogs until the schema-v2 migration attaches one.
	LocationID int `json:"location_id,omitempty"`
	// Offsite is the LEGACY "1" flag, superseded by LocationID → Location.Offsite.
	// Kept and still read (as a fallback) so old catalogs and any unassigned volume
	// keep working; the schema-v2 migration mirrors it into a Location. Default false.
	Offsite   bool      `json:"offsite,omitempty"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	// Physical device identity, best-effort resolved from a mounted path at
	// register/adopt time (Get-Disk on Windows, lsblk/diskutil on unix). Empty
	// when it could not be resolved — external docks/USB bridges sometimes mask
	// the true serial; DeviceNote records that caveat when detected.
	Serial     string     `json:"serial,omitempty"`
	Model      string     `json:"model,omitempty"`
	DeviceSize int64      `json:"device_size,omitempty"` // total device capacity in bytes
	DeviceNote string     `json:"device_note,omitempty"` // e.g. "USB bridge — serial may be the bridge's, not the drive's"
	DeviceAt   *time.Time `json:"device_at,omitempty"`   // when identity was last resolved
	// Drive-mortality (SMART) snapshots, append-only, newest last. A COMPLEMENT to
	// hash verification — SMART hints at failure risk, it never proves data intact.
	SmartHistory []SmartSnapshot `json:"smart_history,omitempty"`
	// Finalize / seal state — the "close the box and label it" ceremony. A SEALED
	// volume is declared DONE and vault-ready; further writes are refused until an
	// explicit, audit-logged unseal. Finalizations is the append-only ceremony
	// history (SEALED / UNSEALED events), newest last.
	Sealed        bool           `json:"sealed,omitempty"`
	SealedAt      *time.Time     `json:"sealed_at,omitempty"`
	Finalizations []Finalization `json:"finalizations,omitempty"`
	// Drive-level (hardware) AES — e.g. an LTO drive's built-in encryption set via
	// `stenc`. This sits ENTIRELY OUTSIDE Obelisk's par2→gpg→tar restore story:
	// the bytes on the tape are ciphertext only the drive+key can decrypt, so a lost
	// drive key means the tape is unreadable by ANYTHING — gpg included. Obelisk
	// does not set it; this flag records that it WAS set (so inventories and the
	// Recovery Kit can shout about it). DriveEncNote records where the drive key lives.
	DriveEncrypted bool   `json:"drive_encrypted,omitempty"`
	DriveEncNote   string `json:"drive_enc_note,omitempty"`
}

// Finalization is one entry in a volume's seal ceremony history: who did it,
// when, what the volume held, and — for a forced seal — which preconditions were
// overridden and why (audit trail). Action is SEALED or UNSEALED.
type Finalization struct {
	At          time.Time `json:"at"`
	By          string    `json:"by"`
	Action      string    `json:"action"` // SEALED | UNSEALED
	Packages    int       `json:"packages,omitempty"`
	Bytes       int64     `json:"bytes,omitempty"`
	FreeBytes   int64     `json:"free_bytes,omitempty"`
	TotalBytes  int64     `json:"total_bytes,omitempty"`
	Forced      bool      `json:"forced,omitempty"`
	ForceReason string    `json:"force_reason,omitempty"` // typed confirmation / unseal reason
	Overrides   []string  `json:"overrides,omitempty"`    // precondition failures overridden by a forced seal
	Sidecar     string    `json:"sidecar,omitempty"`      // sidecar folder written on the volume
}

// SmartSnapshot is one read of a drive's SMART self-report (smartctl -j). It is a
// mortality SIGNAL, never a data-integrity guarantee: a PASSED drive can still
// hold a bit-rotted file, and only the custody-chain hashes prove otherwise.
// Fields are best-effort across ATA and NVMe; zero/omitted where a device does
// not report them. Advisory=true means "migrate copies off this volume".
type SmartSnapshot struct {
	At           time.Time `json:"at"`
	Device       string    `json:"device,omitempty"` // smartctl device arg, e.g. /dev/pd0, /dev/sda
	Type         string    `json:"type,omitempty"`   // ata | nvme | scsi
	Passed       *bool     `json:"passed,omitempty"` // SMART overall-health self-assessment
	TempC        int       `json:"temp_c,omitempty"` // current temperature °C
	PowerOnHours int64     `json:"power_on_hours,omitempty"`
	Reallocated  int64     `json:"reallocated_sectors"`       // ATA attr 5
	Pending      int64     `json:"pending_sectors"`           // ATA attr 197
	MediaErrors  int64     `json:"media_errors,omitempty"`    // NVMe media/data-integrity errors
	PercentUsed  int       `json:"percent_used,omitempty"`    // NVMe endurance used
	SpareLeft    int       `json:"available_spare,omitempty"` // NVMe available spare %
	SpareThresh  int       `json:"spare_threshold,omitempty"` // NVMe spare threshold %
	Advisory     bool      `json:"advisory"`                  // migrate copies off this volume
	AdvisoryWhy  string    `json:"advisory_why,omitempty"`    // human reason for the advisory
	Note         string    `json:"note,omitempty"`            // parse/collection note
}

// Copy is one physical instance of a chunk on a registered volume. Two verified
// copies on volumes in different locations is the redundancy goal.
//
// Superseded marks a copy retained only for history: when a failed copy is
// re-written to the same volume, the old record is kept (superseded=true) so the
// verify trail is not lost, while a fresh Copy takes its place. Superseded copies
// never count toward redundancy or the "current" per-volume copy.
type Copy struct {
	VolumeID  int        `json:"volume_id"`
	Path      string     `json:"path"`
	WrittenAt *time.Time `json:"written_at,omitempty"`
	// LastVerifiedAt / VerifyOK reflect only LEVEL B (full-content) verification —
	// the qualifying bar for protection and the verify-due clock. Level A/C checks
	// are advisory and are recorded in LastCheck* WITHOUT touching these.
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`
	VerifyOK       *bool      `json:"verify_ok,omitempty"`
	// LastCheck* record the most recent verify at ANY level (A/B/C) for display
	// ("checked (C, sample) · 2026-07-08"). A/C never satisfy COMPLETE or refresh
	// verify-due; only B does.
	LastCheckAt    *time.Time `json:"last_check_at,omitempty"`
	LastCheckLevel string     `json:"last_check_level,omitempty"`
	LastCheckOK    *bool      `json:"last_check_ok,omitempty"`
	Superseded     bool       `json:"superseded,omitempty"`
}

// BuildVerified is the build-time attestation that the package faithfully
// contains the source and (when encrypted) decrypts back to the verified tar —
// the two custody-chain links that used to be fingerprinted but never proven.
// It rides into the on-medium manifest.json so the media carry the proof.
//
//   - Contents (stage_verify): the staged tar was stream-read and every member
//     hashed and matched byte-exact against the catalog's source-file hashes.
//   - DecryptRoundtrip (crypt_verify): the ciphertext was decrypted to a hasher
//     (no plaintext to disk) and the result matched tar_hash. For a plaintext
//     package this is true by identity — the payload IS the verified tar.
//
// Mode is "full" (contents + decrypt round-trip), "contents" (contents only), or
// "none" (both skipped, Contents and DecryptRoundtrip false, Warning set) — the
// three build-verify tiers of the integrity presets.
//
// The remaining fields ATTEST the effective integrity settings this package was
// built with (preset, par2 %, routine verify level, verify-due window, and the
// always-on read-back), so the on-medium manifest self-documents how much
// assurance the media were created with.
type BuildVerified struct {
	Mode               string `json:"mode"`
	Contents           bool   `json:"contents"`
	DecryptRoundtrip   bool   `json:"decrypt_roundtrip"`
	Warning            string `json:"warning,omitempty"`
	Preset             string `json:"preset,omitempty"`               // ARCHIVAL | BALANCED | FAST | Custom
	Par2Percent        int    `json:"par2_percent,omitempty"`         // par2 redundancy the payload carries
	RoutineVerifyLevel string `json:"routine_verify_level,omitempty"` // routine re-verify level (B | C)
	VerifyDueMonths    int    `json:"verify_due_months,omitempty"`    // re-verify window
	ReadbackAfterWrite bool   `json:"readback_after_write,omitempty"` // always true — writing unverified media is refused
}

// DockDrive is one legacy drive processed in a dock session: which volume it
// resolved to, its physical serial, the mount it came up on, the content-match
// stats from mirror adoption, and when it finished. Mode records whether this
// pass adopted the drive fresh or re-verified an already-known one.
type DockDrive struct {
	VolumeID     int        `json:"volume_id"`
	Serial       string     `json:"serial,omitempty"`
	Label        string     `json:"label,omitempty"`
	Letter       string     `json:"letter,omitempty"` // mount path / drive letter this drive came up on
	Mode         string     `json:"mode"`             // "adopt" | "reverify"
	Matched      int        `json:"matched"`          // files whose content matches a current cataloged source file
	MatchedBytes int64      `json:"matched_bytes"`
	Historical   int        `json:"historical"` // files matching an older/packaged version, not the current source
	Other        int        `json:"other"`      // readable files matching nothing in the selected archives
	Unreadable   int        `json:"unreadable"`
	Sidecar      string     `json:"sidecar,omitempty"` // path of the inventory sidecar written onto the drive
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	Note         string     `json:"note,omitempty"`
}

// DockSession is a guided, resumable ingest of a stack of legacy backup drives
// through a dock, one at a time, reconciled against one or more Archives. It
// persists so the operator can close the app and resume across days; drives are
// matched by physical serial so a re-inserted drive is recognized, not
// re-adopted. Baseline holds the mounts present when the session started, so the
// watcher can diff for newly-appearing drives.
type DockSession struct {
	ID         int         `json:"id"`
	ArchiveIDs []int       `json:"archive_ids"`
	Baseline   []string    `json:"baseline_mounts,omitempty"`
	StartedAt  time.Time   `json:"started_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	Drives     []DockDrive `json:"drives_processed"`
	Status     string      `json:"status"` // ACTIVE | CLOSED
}

// SnapFile is one file recorded in a drive's SNAPSHOT: everything known about it
// from the ingest read pass, so the catalog can describe the drive's whole tree
// with the drive unplugged. RelPath is drive-root-relative (forward-slashed).
// Hash is the on-media SHA-256; Blake3 is the catalog-internal fast hash. Role
// classifies the file's purpose (see classifyRole); ShotAt/CameraSerial are the
// best-effort EXIF capture stamp and camera body serial (empty when unparseable).
type SnapFile struct {
	RelPath      string    `json:"rel_path"`
	SizeBytes    int64     `json:"size_bytes"`
	ModTime      time.Time `json:"mtime,omitempty"`
	Hash         string    `json:"hash,omitempty"`   // SHA-256 (durable, on-media identity)
	Blake3       string    `json:"blake3,omitempty"` // fast content hash, catalog-only
	Role         string    `json:"role,omitempty"`   // RAW | EDITED-EXPORT | SIDECAR | CATALOG | VIDEO | OTHER
	Critical     bool      `json:"critical,omitempty"`
	ShotAt       time.Time `json:"shot_at,omitempty"`       // EXIF DateTimeOriginal, best-effort
	CameraSerial string    `json:"camera_serial,omitempty"` // EXIF BodySerialNumber, best-effort
}

// VolumeSnapshot is the complete, offline-queryable record of what a drive holds,
// captured in one dock ingest read pass: every file (tree, size, mtime, both
// hashes, role, EXIF), the drive's SMART state at capture, and its physical
// identity. It is what lets the catalog answer "what is on DRIVE-04" — sizes,
// tree, hashes, dates, roles — with the drive unplugged and back in the box.
//
// Unlike a mirror Chunk (which records only files that content-match a cataloged
// source), a snapshot records EVERY file on the drive. One snapshot per volume;
// a re-ingest replaces it in place. Keyed by VolumeID.
type VolumeSnapshot struct {
	VolumeID   int            `json:"volume_id"`
	CapturedAt time.Time      `json:"captured_at"`
	SessionID  int            `json:"session_id,omitempty"`
	Label      string         `json:"label,omitempty"`
	Serial     string         `json:"serial,omitempty"`
	Model      string         `json:"model,omitempty"`
	DeviceSize int64          `json:"device_size,omitempty"`
	LocationID int            `json:"location_id,omitempty"` // where the volume lived at capture (for mirror-pair verdicts)
	Smart      *SmartSnapshot `json:"smart,omitempty"`       // drive-mortality state at capture
	Files      []SnapFile     `json:"files"`                 // EVERY file on the drive
	TotalFiles int            `json:"total_files"`
	TotalBytes int64          `json:"total_bytes"`
	Unreadable int            `json:"unreadable,omitempty"`
	// Role rollups (denormalized for the offline view — the per-file Role is the truth).
	RoleFiles map[string]int   `json:"role_files,omitempty"`
	RoleBytes map[string]int64 `json:"role_bytes,omitempty"`
}

// Event is a first-class named happening — a wedding, a baptism, a family shoot —
// that a set of files belong to. It is the unit people actually search for
// ("Smith"), the unit protection rolls up to ("this wedding: 1 copy — at
// risk"), and the {event}/{event_type} a routing template fills. Membership is
// recorded on the File (File.EventID), not here, so an event is a small record.
// CaptureStart/End is the EXIF date span of its members (or a harvested folder's),
// and doubles as the magnet range that pulls in unassigned files by capture date.
type Event struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	EventType    string    `json:"event_type,omitempty"`
	Year         int       `json:"year,omitempty"`
	CaptureStart time.Time `json:"capture_start,omitempty"`
	CaptureEnd   time.Time `json:"capture_end,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CollectionID int       `json:"collection_id,omitempty"` // archive this event's files live in (0 = any)
	FolderHint   string    `json:"folder_hint,omitempty"`   // leaf folder it was harvested/named from
	Source       string    `json:"source,omitempty"`        // HARVESTED | CLUSTERED | MANUAL
	CreatedAt    time.Time `json:"created_at"`
}

// Template is a small named document describing WHERE files should live: an
// editable event-type vocabulary plus one destination pattern per file role,
// using a fixed, tiny token set ({year} {date} {event_type} {event}
// {camera_serial} {orig_name}). It is intentionally NOT a wall of options —
// exceptions are handled by drag-in-tree, not more knobs. Routes maps a role
// (RAW/EDITED-EXPORT/SIDECAR/CATALOG/VIDEO/OTHER) to its destination pattern; a
// role with no route is "unrouted" in the preview.
type Template struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// EventTypes is the template's editable CATEGORY vocabulary (the guesser matches
	// folder names against it). Named "event_types" for schema back-compat; a
	// photographer's is wedding/portrait/…, a filmmaker's is short/feature/…, etc.
	EventTypes []string          `json:"event_types"`
	Routes     map[string]string `json:"routes"`
	// GroupNoun is what this template calls a grouping of files in the UI — "Event"
	// for photographers, "Session"/"Collection"/"Project" for other disciplines.
	// Blank = "Event" (the neutral default). Purely a display label; the underlying
	// model is always Event.
	GroupNoun string    `json:"group_noun,omitempty"`
	BuiltIn   bool      `json:"built_in,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Conflict statuses and classes. A conflict is two (or more) catalog files that
// are the SAME logical file but hold DIFFERENT bytes, with no source of truth to
// arbitrate — surfaced for a human decision, never auto-preferred.
const (
	ConflictOpen     = "OPEN"
	ConflictResolved = "RESOLVED"
	// ClassSameMeta (b): identical capture metadata AND camera serial, different hash.
	ClassSameMeta = "SAME-META"
	// ClassNoEXIF (c): same path/name, no EXIF to arbitrate, different hash.
	ClassNoEXIF = "NO-EXIF"
	// Resolutions.
	ResolveCanonical = "CANONICAL" // one version wins; the other(s) fold into its retained history
	ResolveKeepBoth  = "KEEP-BOTH" // both stay as independent files; the plan renames on placement
)

// Conflict is one true, human-arbitrated disagreement: N catalog files sharing a
// logical identity but differing in content hash. Signature (collection|key|sorted
// hashes) makes detection idempotent — a resolved conflict is never re-opened when
// the same bytes are re-scanned. Resolution feeds the per-file version-retention
// history (File.Versions); nothing is ever discarded.
type Conflict struct {
	ID           int        `json:"id"`
	CollectionID int        `json:"collection_id"`
	Class        string     `json:"class"` // SAME-META | NO-EXIF
	Key          string     `json:"key"`   // logical grouping key (display)
	RelPath      string     `json:"rel_path"`
	Role         string     `json:"role,omitempty"`
	RawAlert     bool       `json:"raw_alert,omitempty"` // any involved file is a RAW original
	Signature    string     `json:"signature"`
	FileIDs      []int      `json:"file_ids"`
	Status       string     `json:"status"`               // OPEN | RESOLVED
	Resolution   string     `json:"resolution,omitempty"` // CANONICAL | KEEP-BOTH
	Canonical    int        `json:"canonical_file_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// Plan statuses. A plan is authored against snapshots (drives unplugged), gated by
// a dry-run compile, then executed serial-bound and in any order over weeks.
const (
	PlanDraft     = "DRAFT"     // being authored; mapping computed live
	PlanCompiled  = "COMPILED"  // dry-run passed; mapping frozen, ready to execute
	PlanExecuting = "EXECUTING" // at least one drive's work has run
	PlanClosed    = "CLOSED"    // finished (or abandoned); final report recorded
)

// Plan is the keystone: a persisted, future file structure authored against drive
// SNAPSHOTS while the drives sit unplugged, executed later in any order. It maps
// every in-scope file BY CONTENT HASH to a planned destination path (so a hash
// shared across drives is copied once), remembers manual drag overrides that
// survive template edits, and tracks which hashes are already satisfied so
// execution is resumable across restarts and weeks.
type Plan struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	ArchiveIDs      []int  `json:"archive_ids,omitempty"`  // scope (empty = every snapshot)
	ScopePrefix     string `json:"scope_prefix,omitempty"` // folder-tree scope: restrict to files under this path prefix (empty = whole archive)
	TemplateID      int    `json:"template_id"`
	DestinationRoot string `json:"destination_root"` // may not exist yet — validated only at execution
	Status          string `json:"status"`
	DestVolumeID    int    `json:"dest_volume_id,omitempty"` // volume the reorganized copies are recorded on
	// Overrides are manual placements (hash → planned rel path) from dragging in the
	// virtual tree; they win over the template and PERSIST across template edits.
	Overrides map[string]string `json:"overrides,omitempty"`
	// Parked hashes are explicitly excluded from the "unrouted must be 0" compile gate
	// (the operator chose to leave them out of this move).
	Parked map[string]bool `json:"parked,omitempty"`
	// Mapping is the frozen hash → planned rel path, written at compile time so the
	// plan is stable even if the template later changes. Empty until compiled.
	Mapping map[string]string `json:"mapping,omitempty"`
	// Satisfied marks hashes whose bytes are now at the destination (first drive to
	// carry a shared hash satisfies it; later drives confirm). Drives execution
	// resumability and the coverage math.
	Satisfied  map[string]bool `json:"satisfied,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	CompiledAt *time.Time      `json:"compiled_at,omitempty"`
	ClosedAt   *time.Time      `json:"closed_at,omitempty"`
}

// TapeAlertFlag is one active TapeAlert bit reported by a tape drive, rendered
// in plain language. Severity is one of: clean, warn, error (info flags are
// dropped). TapeAlert is the drive's own self-report — a diagnostic SIGNAL, like
// SMART; it never proves the data on a cartridge is intact.
type TapeAlertFlag struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Severity string `json:"severity"` // clean | warn | error
	Text     string `json:"text"`
}

// TapeHealth is one read of a tape drive's diagnostics (TapeAlert + LOG SENSE),
// via an external tool (ITDT / tapeinfo / sg_logs / HPE L&TT). STRICTLY outside
// the write path and read-only toward the drive: never issues movement or write
// commands. A COMPLEMENT to hash verification, never a substitute.
type TapeHealth struct {
	At                  time.Time       `json:"at"`
	Tool                string          `json:"tool"`   // itdt | tapeinfo | sg_logs | hpe-ltt
	Device              string          `json:"device"` // e.g. \\.\Tape0, /dev/nst0
	Vendor              string          `json:"vendor,omitempty"`
	Product             string          `json:"product,omitempty"`
	Serial              string          `json:"serial,omitempty"`
	Alerts              []TapeAlertFlag `json:"alerts,omitempty"`
	CleaningRecommended bool            `json:"cleaning_recommended"`
	Severity            string          `json:"severity"` // ok | clean | warn | error
	Summary             string          `json:"summary"`  // plain-language one-liner
	PowerOnHours        int64           `json:"power_on_hours,omitempty"`
	BytesWritten        int64           `json:"bytes_written,omitempty"` // lifetime, from LOG SENSE when exposed
	BytesRead           int64           `json:"bytes_read,omitempty"`
	Note                string          `json:"note,omitempty"`
}

// VerifyEvent is one integrity check of a chunk's payload against its
// recorded enc_hash — logged by write read-back, media verify, burn verify,
// and verify campaigns. Append-only history; media is never modified.
type VerifyEvent struct {
	At    time.Time `json:"at"`
	OK    bool      `json:"ok"`
	Path  string    `json:"path"`
	Note  string    `json:"note"`            // e.g. "write read-back", "media verify", "burn verify", "campaign"
	Level string    `json:"level,omitempty"` // "A" census · "B" full · "C" sample; blank legacy events are level B
	// Advisory marks a level-A/C check: it records intact-so-far evidence but does
	// NOT satisfy COMPLETE or refresh verify-due — only a level-B pass does.
	Advisory bool `json:"advisory,omitempty"`
}

// UnmarshalJSON defaults Encrypted to true when the field is absent, so
// catalogs written before encryption became optional (every chunk was
// encrypted) load with the correct meaning.
func (c *Chunk) UnmarshalJSON(b []byte) error {
	type alias Chunk
	aux := &struct {
		Encrypted *bool `json:"encrypted"`
		*alias
	}{alias: (*alias)(c)}
	if err := json.Unmarshal(b, aux); err != nil {
		return err
	}
	c.Encrypted = aux.Encrypted == nil || *aux.Encrypted
	return nil
}

type KeyMeta struct { // secrets are NEVER here — keystore files only
	Ref         string    `json:"key_ref"`
	Fingerprint string    `json:"fingerprint"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type Job struct {
	ID         int        `json:"id"`
	Kind       string     `json:"kind"`
	Label      string     `json:"label"`
	Status     string     `json:"status"` // RUNNING COMPLETED FAILED INTERRUPTED
	Progress   float64    `json:"progress"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"` // set when the job reaches a terminal state
	// Live telemetry for byte-moving jobs (copy/write/mirror): current throughput
	// and estimated time remaining, surfaced on the job row instead of a bare %.
	RateMBps   float64        `json:"rate_mbps,omitempty"`
	ETASeconds float64        `json:"eta_seconds,omitempty"`
	BytesDone  int64          `json:"bytes_done,omitempty"`
	BytesTotal int64          `json:"bytes_total,omitempty"`
	FilesDone  int64          `json:"files_done,omitempty"`  // count-based progress (scan/drift/dock/mirror)
	FilesTotal int64          `json:"files_total,omitempty"` // total files this job will touch (0 = unknown)
	Result     map[string]any `json:"result,omitempty"`      // the finished job's artifact/summary (path, counts, …)
	// Artifacts is the structured list of concrete things the job produced —
	// packages staged, files copied, labels/kits generated, catalog records touched.
	// Each carries a Show action (a view + id the UI routes to). Recorded as the job
	// produces them and persisted alongside the job, so a finished job can always
	// answer "what did I make, and where can I see it?" (the show-the-artifact rule,
	// made structural). See Artifact.
	Artifacts []Artifact `json:"artifacts,omitempty"`
	// Unrecorded and PersistError are the OB-002 recording-state contract. They are
	// two DIFFERENT facts and must not be collapsed into one:
	//
	//   Unrecorded   — CURRENT state. True means THIS terminal snapshot (the status,
	//                  artifacts and result now in memory) has not been successfully
	//                  written to jobs.json. Status still describes what the WORK did;
	//                  Unrecorded says the bookkeeping did not land. It is cleared the
	//                  moment an ordinary jobs write successfully serialises this row —
	//                  see saveJobs, which clears it BEFORE marshalling so the bytes
	//                  and the in-memory row always agree.
	//   PersistError — HISTORY. The cause of the most recent recording failure. It is
	//                  retained after recovery, so "this completion was once at risk"
	//                  stays discoverable without claiming the record is still missing.
	//
	// So the three states a reader must be able to tell apart are:
	//
	//   unrecorded=true,  persist_error set   → not recorded right now (warn)
	//   unrecorded=false, persist_error set   → recorded, with an earlier failure (history)
	//   unrecorded=false, persist_error empty → ordinary recorded completion
	//
	// Both are additive and omitempty, so an ordinary job serialises exactly as
	// before. A record written by an older build has neither field, which decodes to
	// the third row above — correct, because a row that is IN the file is recorded.
	//
	// Unrecorded is never true in a successfully written file (saveJobs clears it
	// before the bytes are produced), and loadJobs clears it again on open for the
	// same reason. PersistError IS written and read back: it is history, and history
	// survives.
	Unrecorded   bool   `json:"unrecorded,omitempty"`
	PersistError string `json:"persist_error,omitempty"`
}

// Artifact is one concrete output a Job produced. The Show* fields locate it in
// the UI so a job's detail view can open the thing itself (the package's detail,
// the archive explorer scoped to what a scan covered, the volume, an export path).
type Artifact struct {
	Kind  string `json:"kind"`  // package | file | label | kit | catalog | export | volume | drift
	Label string `json:"label"` // human name ("NSP-C0003", "142 files cataloged")
	Path  string `json:"path,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Count int    `json:"count,omitempty"`
	// Show action target — the view + id the UI routes to. ShowView is a UI view
	// name ("packages"→chunk, "explore"→collection, "volumes"→volume, "drift"→
	// collection); ShowPath optionally scopes the explorer to a sub-path.
	ShowView string `json:"show_view,omitempty"`
	ShowID   int    `json:"show_id,omitempty"`
	ShowPath string `json:"show_path,omitempty"`
}

type Audit struct {
	At     time.Time `json:"at"`
	Action string    `json:"action"`
	Detail string    `json:"detail"`
}

// BackupSession records one incremental "back up changes" run: the delta that was
// landed onto one volume, when, and how. Stateless by design — nothing downstream
// depends on it to compute the NEXT delta (that's always recomputed from the catalog);
// it is history + recognition only. Name is the ready-to-show one-liner
// ("Incremental to ARCH-03 — 214 files, 8.1 GB, 2026-07-09").
type BackupSession struct {
	ID           int       `json:"id"`
	CollectionID int       `json:"collection_id"`
	VolumeID     int       `json:"volume_id"`
	VolumeLabel  string    `json:"volume_label"`
	Base         string    `json:"base"` // "volume" (not yet on this volume) | "protection" (below COMPLETE)
	Mode         string    `json:"mode"` // "mirror" | "package"
	Files        int       `json:"files"`
	Bytes        int64     `json:"bytes"`
	FolderIDs    []int     `json:"folder_ids,omitempty"` // folder scope (empty = whole archive)
	At           time.Time `json:"at"`
	Name         string    `json:"name"`
	Dest         string    `json:"dest,omitempty"` // mirror destination path (mirror mode)
}

// DriftItem is one file that differs between the source folders now and what
// the collection's chunks hold. MISSING/MODIFIED carry a restore pointer
// (which chunk + which volumes hold the backed-up version).
type DriftItem struct {
	State         string   `json:"state"` // NEW MODIFIED MISSING MOVED (UNCHANGED is counted, not listed). NEW renders as UNARCHIVED in the UI/docs; the stored value stays "NEW".
	Path          string   `json:"path"`
	Ext           string   `json:"ext"`
	Hash          string   `json:"hash,omitempty"`
	MovedFrom     string   `json:"moved_from,omitempty"`
	Chunk         string   `json:"chunk,omitempty"`   // backing chunk for MISSING/MODIFIED
	Volumes       []string `json:"volumes,omitempty"` // "LABEL (location)" restore-from pointers
	NeedsBackup   bool     `json:"needs_backup,omitempty"`
	Informational bool     `json:"informational,omitempty"`
	// Version retention (prompt 50): for a MODIFIED file the backed-up bytes are now
	// a RETAINED prior version. PriorVersion is its one-line restore-source locator
	// ("v2 · 2024-03-12 · in NSP-C0003 on tape LTO-0007"); FileID/PriorHash let the
	// UI open the full version history / restore that exact version.
	FileID       int    `json:"file_id,omitempty"`
	PriorHash    string `json:"prior_hash,omitempty"`
	PriorVersion string `json:"prior_version,omitempty"`
}

// ExtDrift is the per-file-type headline row (".NEF: 2 missing, 0 modified").
type ExtDrift struct {
	Ext           string `json:"ext"`
	Missing       int    `json:"missing"`
	Modified      int    `json:"modified"`
	New           int    `json:"new"`
	Moved         int    `json:"moved"`
	Informational bool   `json:"informational"`
}

// DriftReport is the persisted result of a Rescan & compare for one collection.
type DriftReport struct {
	At           time.Time      `json:"at"`
	CollectionID int            `json:"collection_id"`
	Counts       map[string]int `json:"counts"`      // alarm totals: unchanged,new,modified,missing,moved
	InfoCounts   map[string]int `json:"info_counts"` // informational-extension totals (excluded from alarms)
	ByExt        []ExtDrift     `json:"by_ext"`
	Items        []DriftItem    `json:"items"` // only the changed files (not UNCHANGED)
}

// Changes returns the number of non-informational changes (the alarm total).
func (r *DriftReport) Changes() int {
	if r == nil {
		return 0
	}
	return r.Counts["new"] + r.Counts["modified"] + r.Counts["missing"] + r.Counts["moved"]
}

// currentSchemaVersion is THE catalog schema version this build understands. It is
// stamped into catalog.json on every save. The forward-compatibility contract:
//   - file version  == current → proceed normally
//   - file version  <  current → run ordered, idempotent migrations (backup first)
//   - file version  >  current → REFUSE TO WRITE (a newer app wrote it); reads only
//
// Evolving the schema is governed by hard rules in docs/CONTRIBUTING.md
// ("Schema versioning"): persisted fields are append-only, removal needs a
// migration + a major bump, and every field must tolerate being absent.
const currentSchemaVersion = 8

// schemaMigration transforms the in-memory catalog UP TO the version named by To.
// Each MUST be idempotent (safe to run twice) and is applied in ascending To order.
type schemaMigration struct {
	To int
	Fn func(c *catalog)
}

// schemaMigrations is the ordered registry. To evolve the schema: bump
// currentSchemaVersion, APPEND a migration whose To equals the new version, and
// NEVER edit or remove an existing entry (old catalogs still replay it).
var schemaMigrations = []schemaMigration{
	// 0 → 1: the first versioned schema. Catalogs written before schema versioning
	// have no schema_version field (reads as 0) but are STRUCTURALLY identical to v1
	// — every field was already append-only and absent-tolerant — so this only
	// stamps the version. Idempotent by construction (empty body).
	{To: 1, Fn: func(c *catalog) {}},
	// 1 → 2: Locations became first-class. Create a Location per distinct prior
	// (free-text label, offsite) pairing among volumes and attach each volume to it,
	// so 3-2-1 offsite math can read Location.Offsite. The legacy per-volume Offsite
	// bool is preserved (reads keep working); this only ADDS Locations + location_id.
	{To: 2, Fn: migrateLocations},
	// 2 → 3: drive SNAPSHOTS became a first-class catalog record (the full offline
	// inventory of every dock-ingested drive). Purely additive — pre-v3 catalogs
	// simply have no snapshots yet; the field is absent-tolerant. The bump exists so
	// an OLDER build refuses to write a snapshot-bearing catalog (it would silently
	// drop the snapshots on save). Empty body — nothing to transform.
	{To: 3, Fn: func(c *catalog) {}},
	// 3 → 4: Events + Templates became first-class, and File gained shot_at/role/
	// camera_serial/event_id. All additive and absent-tolerant; the bump makes older
	// builds refuse to write (they'd drop events/templates/media-metadata). Empty
	// body — nothing to transform.
	{To: 4, Fn: func(c *catalog) {}},
	// 4 → 5: Conflicts (same-logical-file/different-bytes review queue) became a
	// first-class record. Additive; the bump makes older builds refuse to write (they
	// would drop pending/resolved conflict decisions). Empty body.
	{To: 5, Fn: func(c *catalog) {}},
	// 5 → 6: Plans (template-driven cold-drive reorganization) became first-class.
	// Additive; the bump makes older builds refuse to write (they'd drop plans and
	// their execution progress). Empty body.
	{To: 6, Fn: func(c *catalog) {}},
	// 6 → 7: managed-territory Quarantine (staged, reversible, never-completable
	// isolation of files inside destinations Obelisk populated) became first-class.
	// Additive; the bump makes older builds refuse to write (they'd drop the
	// _deleted staging records and their restore/history state). Empty body.
	{To: 7, Fn: func(c *catalog) {}},
	// 7 → 8: the file-role taxonomy generalized from photography-specific
	// (RAW/EDITED-EXPORT/SIDECAR/CATALOG/VIDEO) to discipline-neutral
	// (ORIGINALS/DELIVERABLES/SIDECARS/PROJECT-FILES). This remaps stored role
	// strings on Files, snapshot files, and template route keys. Idempotent —
	// already-new values pass through unchanged. VIDEO maps to DELIVERABLES as the
	// common case (a rescan re-derives roles by extension).
	{To: 8, Fn: migrateRolesV8},
}

// remapLegacyRole maps a pre-v8 (photography-specific) role string to the neutral
// taxonomy. Already-migrated / OTHER / blank values pass through unchanged, so it
// is safe to run twice.
func remapLegacyRole(r string) string {
	switch strings.ToUpper(strings.TrimSpace(r)) {
	case "RAW":
		return RoleOriginals
	case "EDITED-EXPORT":
		return RoleDeliverables
	case "SIDECAR":
		return RoleSidecars
	case "CATALOG":
		return RoleProject
	case "VIDEO":
		return RoleDeliverables // best-effort; a rescan reclassifies by extension
	}
	return r
}

// migrateRolesV8 rewrites every stored legacy role: on File rows, on snapshot files
// and their per-role tallies, and on template route keys. Content-addressed history
// (File.Versions) carries no role, so nothing else needs touching.
func migrateRolesV8(c *catalog) {
	for _, f := range c.Files {
		f.Role = remapLegacyRole(f.Role)
	}
	for _, snap := range c.Snapshots {
		for i := range snap.Files {
			snap.Files[i].Role = remapLegacyRole(snap.Files[i].Role)
		}
		snap.RoleFiles = remapRoleMapInt(snap.RoleFiles)
		snap.RoleBytes = remapRoleMapInt64(snap.RoleBytes)
	}
	for _, t := range c.Templates {
		if len(t.Routes) == 0 {
			continue
		}
		// Iterate legacy keys in sorted order so a collision (e.g. both EDITED-EXPORT
		// and VIDEO map to DELIVERABLES) resolves DETERMINISTICALLY — the migration
		// must produce identical bytes on every run.
		keys := make([]string, 0, len(t.Routes))
		for k := range t.Routes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		nr := make(map[string]string, len(t.Routes))
		for _, k := range keys {
			nk := remapLegacyRole(k)
			if _, ok := nr[nk]; ok {
				continue // first (lowest-sorting) legacy key wins the merged route
			}
			nr[nk] = t.Routes[k]
		}
		t.Routes = nr
	}
}

func remapRoleMapInt(m map[string]int) map[string]int {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[remapLegacyRole(k)] += v
	}
	return out
}

func remapRoleMapInt64(m map[string]int64) map[string]int64 {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[remapLegacyRole(k)] += v
	}
	return out
}

// migrateLocations is the 1→2 step: fold every volume's free-text Location + Offsite
// bool into first-class Location rows. Idempotent — a volume that already has a
// location_id is left alone, and existing Location rows are reused by (name, offsite).
func migrateLocations(c *catalog) {
	if c.NextID == nil {
		c.NextID = map[string]int{}
	}
	type key struct {
		name    string
		offsite bool
	}
	idByKey := map[key]int{}
	for _, l := range c.Locations {
		idByKey[key{strings.ToLower(strings.TrimSpace(l.Name)), l.Offsite}] = l.ID
	}
	now := time.Now().UTC()
	for _, v := range c.Volumes {
		if v.LocationID != 0 {
			continue
		}
		name := strings.TrimSpace(v.Location)
		if name == "" {
			if v.Offsite {
				name = "Offsite"
			} else {
				name = "Onsite"
			}
		}
		k := key{strings.ToLower(name), v.Offsite}
		id, ok := idByKey[k]
		if !ok {
			c.NextID["location"]++
			id = c.NextID["location"]
			c.Locations = append(c.Locations, &Location{ID: id, Name: name, Offsite: v.Offsite,
				Notes: "migrated from the per-volume offsite flag", CreatedAt: now})
			idByKey[k] = id
		}
		v.LocationID = id
	}
}

type catalog struct {
	// SchemaVersion MUST stay the first field. Absent (0) on pre-versioning catalogs;
	// migrated up to currentSchemaVersion on load. See currentSchemaVersion.
	SchemaVersion int            `json:"schema_version"`
	NextID        map[string]int `json:"next_id"`
	Collections   []*Collection  `json:"collections"`
	Folders       []*Folder      `json:"folders"`
	Files         []*File        `json:"files"`
	Chunks        []*Chunk       `json:"chunks"`
	Keys          []*KeyMeta     `json:"keys"`
	BurnQueues    []*BurnQueue   `json:"burn_queues"`
	Volumes       []*Volume      `json:"volumes"`
	Locations     []*Location    `json:"locations"` // physical places volumes live (the "1" in 3-2-1)
	Drift         []*DriftReport `json:"drift"`     // latest reconcile report per collection
	DockSessions  []*DockSession `json:"dock_sessions"`
	// Snapshots is the complete offline inventory of every dock-ingested drive
	// (one per volume, keyed by VolumeID). It is what lets Volumes/Treemap describe
	// an unplugged drive. See VolumeSnapshot.
	Snapshots []*VolumeSnapshot `json:"volume_snapshots,omitempty"`
	// Events (named happenings files belong to) and Templates (role→destination
	// routing documents). Additive in schema v4. See Event / Template.
	Events    []*Event    `json:"events,omitempty"`
	Templates []*Template `json:"templates,omitempty"`
	// Conflicts: true same-logical-file/different-bytes disagreements awaiting (or
	// carrying) a human decision. Additive in schema v5. See Conflict.
	Conflicts []*Conflict `json:"conflicts_review,omitempty"`
	// Plans: template-driven cold-drive reorganizations authored against snapshots and
	// executed serial-bound in any order. Additive in schema v6. See Plan.
	Plans      []*Plan      `json:"plans,omitempty"`
	TapeChecks []TapeHealth `json:"tape_checks"` // append-only tape-drive diagnostic snapshots, newest last
	// Protection profiles (the 3-2-1 model), their per-archive/per-folder
	// assignments, and the latest recomputed status summary per collection.
	Profiles    []*Profile           `json:"profiles"`
	Assignments []*Assignment        `json:"assignments"`
	Protection  []*ProtectionSummary `json:"protection"`
	Audit       []Audit              `json:"audit"`
	// Quarantine: files/folders staged into <destination_root>/_deleted inside managed
	// territory — never deleted by the tool, reversible via un-quarantine. Additive in
	// schema v7. See QuarantineEntry / quarantine.go.
	Quarantine []*QuarantineEntry `json:"quarantine,omitempty"`
	// BackupSessions: the append-only history of incremental "back up changes" runs —
	// each a named, stateless snapshot of "what was new that day" onto one volume. They
	// carry no execution state (the delta is always recomputed from the catalog); they
	// exist to show a Backup History and to feed Home's periodic-backup recognition
	// natively. Additive; tolerated absent on older catalogs. See incremental.go.
	BackupSessions []*BackupSession `json:"backup_sessions,omitempty"`
}

// compactThreshold is the file count above which the catalog is saved as compact
// (non-indented) JSON — see save(). Below it, catalogs stay pretty-printed and
// text-editor-inspectable.
const compactThreshold = 50_000

type Store struct {
	mu      sync.Mutex
	path    string
	c       catalog
	lastBak string // YYYYMMDD of the most recent daily backup written
	// readOnly latches when catalog.json was written by a NEWER schema than this
	// build understands. Every write is then refused (silently dropping fields the
	// newer app added would be data loss); reads/viewing still work. See OpenStore.
	readOnly       bool
	readOnlyReason string
	// fileIdx indexes Files by (collection|folder|relpath) so UpsertFile is O(1)
	// instead of a linear scan — the difference between an O(n) and an O(n²) scan
	// at a million files. Rebuilt from the catalog on open; kept in sync by
	// UpsertFile. Nil until first built.
	fileIdx map[string]*File
	// versionsRetained caps the per-file retained-version history (0 = unlimited,
	// the default). The App sets it from config before a scan/reconcile so UpsertFile
	// can prune under the lock. Capping only forgets the catalog's POINTER to old
	// bytes — the bytes stay sealed in their packages; it never deletes media.
	versionsRetained int
	// Batched persistence: during a long job (scan/adopt/mirror) the App brackets
	// the work in BeginBatch/EndBatch; while batchDepth>0, save() coalesces writes
	// (at most one every batchInterval) and marks the catalog dirty, then EndBatch
	// forces a final write. Crash-safety is preserved because those jobs are
	// idempotent re-runs (see BeginBatch).
	batchDepth    int
	dirty         bool
	lastSave      time.Time
	batchInterval time.Duration
	// failSave is a test-only fault-injection seam: nil in production. When set, it is
	// consulted at the top of writeCatalog so a test can simulate a disk-write failure
	// and prove persistence errors propagate instead of being silently dropped.
	failSave func() error
	// failSaveJobs is the same seam for the JOBS sidecar: nil in production, consulted
	// at the top of saveJobs. It is a per-Store field rather than a package global on
	// purpose — OB-002's regressions run concurrent jobs, and a shared mutable hook
	// would let one test's injected fault reach another's store.
	failSaveJobs func() error
	// persistObserver is a test-only OBSERVATION seam: nil in production, installed
	// only through openStore. Every attempted authority write reports itself here via
	// notePersist BEFORE the gates that could turn it into an early return, so a test
	// can prove a refused startup attempted no write at all — an absent catalog.json.tmp
	// proves nothing, because a successful rename removes that name too. Covers
	// writeCatalog, dailyBackup (create and prune) and backupBeforeMigrate. Jobs-sidecar
	// writes are outside this observer's scope: loadJobs can call saveJobs during startup
	// reconciliation, but a rejected catalog read returns before loadJobs is reached. The
	// dataDir MkdirAll precedes the read and is also outside this observer's scope.
	persistObserver func(op, path string)
	jobs            struct {
		mu      sync.Mutex
		next    int
		rows    []*Job
		loaded  bool   // a validated or successfully published sidecar has existed
		loadErr error  // refused reload: blocks every later jobs publication
		path    string // jobs.json sidecar (persists the board across restarts)
	}
}

// fileKey is the UpsertFile index key: a file is unique per (collection, folder,
// rel path).
func fileKey(collectionID, folderID int, rel string) string {
	return strconv.Itoa(collectionID) + "|" + strconv.Itoa(folderID) + "|" + rel
}

// buildFileIndexLocked (re)builds fileIdx from the catalog. Caller holds no lock
// during OpenStore (single-threaded); UpsertFile builds it lazily under the lock.
func (s *Store) buildFileIndexLocked() {
	idx := make(map[string]*File, len(s.c.Files))
	for _, f := range s.c.Files {
		idx[fileKey(f.CollectionID, f.FolderID, f.RelPath)] = f
	}
	s.fileIdx = idx
}

// openStoreFailSave, when non-nil, seeds the new Store's failSave seam at the top of
// OpenStore — the only way a test can inject a write failure into the recovery save
// OpenStore performs before returning. nil in production. See the failSave field.
var openStoreFailSave func() error

// decodeCatalogJSON is the shared pure native decoder. Startup policy stays in openStore.
func decodeCatalogJSON(raw []byte, c *catalog) error { return json.Unmarshal(raw, c) }

func OpenStore(dataDir string) (*Store, error) {
	return openStore(dataDir, os.ReadFile, nil)
}

// openStore is OpenStore with its two boundaries injected, so a test can drive the
// REAL decision logic below instead of standing in a mock that already behaves.
// readFile supplies the catalog and jobs read RESULTS — bytes and error together — that the
// classification switches on, so an injected failure and a genuine one travel the
// identical branch. persistObserver, when non-nil, records attempted authority
// writes (see the Store field). Production has exactly one caller, OpenStore, which
// passes os.ReadFile and nil. Keeping these as parameters rather than package-level
// hooks means no shared mutable test state, no cleanup, and no way for one test's
// injected fault to reach another.
func openStore(dataDir string, readFile func(string) ([]byte, error), persistObserver func(op, path string)) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dataDir, "catalog.json")}
	s.failSave = openStoreFailSave
	s.persistObserver = persistObserver
	s.jobs.path = filepath.Join(dataDir, "jobs.json")
	s.c.NextID = map[string]int{}
	existed := false
	var raw []byte
	// Catalog-read classification (OB-001). Three outcomes, deliberately distinct,
	// because only ONE of them may end in a new catalog:
	//
	//   read succeeded  -> this is the existing catalog; parse it.
	//   not-found       -> the only case that may initialize a new one.
	//   anything else   -> refuse to start.
	//
	// The last branch is the fix. Permission denial, an I/O error, a directory in the
	// way, a share held by a backup agent or antivirus, a data dir whose mount is not
	// up: every one of those used to fall through here with err != nil and leave an
	// EMPTY in-memory catalog. Seeding below then set recovered=true and saved, which
	// renames a three-profile empty catalog over the unread original — and dailyBackup
	// went on to spend a backup slot on those empty bytes. An unavailable authority
	// must not be replaced by an apparently new one.
	b, readErr := readFile(s.path)
	switch {
	case readErr == nil:
		// A zero-length catalog is the classic torn-write/power-loss artifact. Treating
		// it as "brand new" (the old len(b) > 0 test) skipped the schema gate and the
		// pre-migration backup and then overwrote it. It is damaged input, not consent.
		if len(b) == 0 {
			return nil, fmt.Errorf("%s is empty (0 bytes) — that is a damaged catalog, not a new one. "+
				"Recover it from a catalog.json.bak-YYYYMMDD sidecar in the same folder, or move the "+
				"empty file aside if you really do want to start a new catalog here. "+
				"Refusing to overwrite it", s.path)
		}
		existed = true
		raw = b
		if err := decodeCatalogJSON(b, &s.c); err != nil {
			return nil, fmt.Errorf("catalog.json is damaged: %w", err)
		}
	case errors.Is(readErr, fs.ErrNotExist):
		// Genuinely absent: initialize a new catalog, exactly as before. Note what this
		// does NOT establish — that the operator INTENDED a new catalog. A previously
		// initialized data directory whose drive is not mounted also reads as absent.
		// Telling those apart needs storage identity, which this change does not add;
		// the residual is recorded against OB-001.
	default:
		// Any bytes handed back alongside an error are discarded on purpose: a partial
		// read must never be parsed as if it were the whole catalog.
		return nil, fmt.Errorf("cannot read %s: %w — refusing to start so an unreadable "+
			"catalog is never replaced by a new empty one. Check the file's permissions, "+
			"and that the drive holding the data folder is connected", s.path, readErr)
	}
	if s.c.NextID == nil {
		s.c.NextID = map[string]int{}
	}
	// Schema-version gate — the forward-compatibility guarantee (see
	// currentSchemaVersion). A brand-new catalog is simply stamped at current on its
	// first save; only an EXISTING file gets migrated or read-only-latched.
	if existed {
		switch {
		case s.c.SchemaVersion > currentSchemaVersion:
			// A newer app wrote this. Refuse to save: we'd silently drop fields we
			// don't understand. Viewing is fine; every write is blocked in writeCatalog.
			s.readOnly = true
			s.readOnlyReason = fmt.Sprintf(
				"catalog.json was created by a newer version of Obelisk (catalog schema v%d; this build understands v%d). "+
					"Upgrade the app to write to it — refusing to save so newer fields aren't silently dropped. Read-only viewing is allowed.",
				s.c.SchemaVersion, currentSchemaVersion)
		case s.c.SchemaVersion < currentSchemaVersion:
			// Older schema: back up the exact on-disk bytes, then replay migrations.
			from := s.c.SchemaVersion
			s.backupBeforeMigrate(raw, from)
			s.runMigrations(from)
		}
	}
	// Reboot recovery: a disc caught mid-burn/mid-verify when the process
	// died is unknowable — the physical disc may be a coaster. Send it back
	// to PENDING so the operator re-burns on a fresh blank.
	recovered := false
	for _, q := range s.c.BurnQueues {
		for _, d := range q.Discs {
			if d.Status == "BURNING" || d.Status == "VERIFYING" {
				d.Detail = "reset to PENDING after restart (was " + d.Status + ") — the disc may be a coaster; re-burn on a fresh blank"
				d.Status = "PENDING"
				recovered = true
			}
		}
	}
	// Interrupted-job recovery: jobs are in-memory, so a chunk left mid-flight
	// (BUILDING/WRITING) when the process died is the only trace of an orphaned
	// job. Reset each to its prior stable state with an explanatory error, the
	// same spirit as the burn-queue recovery above.
	for _, c := range s.c.Chunks {
		switch c.Status {
		case "BUILDING":
			c.Status, c.Error = "PLANNED", "interrupted by shutdown mid-build — re-run Build"
			recovered = true
		case "WRITING":
			c.Status, c.Error = "STAGED", "interrupted by shutdown mid-write — re-run Write"
			recovered = true
		}
		// A spanned segment caught mid-write is an unknown partial file on the
		// medium; send it back to PENDING so the operator re-writes that tape.
		for i := range c.Segments {
			if c.Segments[i].Status == "WRITING" || c.Segments[i].Status == "WRITTEN" {
				c.Segments[i].Status = "PENDING"
				recovered = true
			}
		}
	}
	// Migration: pre-Volumes catalogs recorded only written_dest. Attach that as
	// a Copy on an auto-created "(unregistered)" volume so old data keeps working
	// and shows up in the Volumes/redundancy views.
	for _, c := range s.c.Chunks {
		if c.WrittenDest != "" && len(c.Copies) == 0 {
			v := s.ensureUnregisteredLocked()
			c.Copies = append(c.Copies, Copy{VolumeID: v.ID, Path: c.WrittenDest,
				WrittenAt: c.WrittenAt, LastVerifiedAt: c.VerifiedAt, VerifyOK: c.VerifyOK})
			recovered = true
		}
	}
	// Built-in profiles: ensure the three shipped profiles exist and stay
	// canonical (they are immutable, so we overwrite any drifted copy). Custom
	// user profiles are never touched. New catalogs start with just these three.
	if s.seedBuiltinProfilesLocked() {
		recovered = true
	}
	// Built-in routing template: seed the starter template set on a catalog with no
	// templates (never re-added once the operator deletes all of them within a run —
	// but a fresh open with zero templates re-seeds, matching the profiles spirit).
	if len(s.c.Templates) == 0 {
		s.seedBuiltinTemplateLocked()
		recovered = true
	}
	if recovered {
		// Loud non-fatal: recovery/seeding already applied in memory, so a boot that can't
		// persist it still beats no boot — it re-applies on the next successful save. Warn
		// the operator rather than refuse to open over a transient write failure.
		if err := s.save(); err != nil {
			log.Printf("WARNING: startup recovery could not be persisted (%v) — recovered "+
				"state may not survive another restart until the next successful save", err)
		}
	}
	// Load the persisted job board (sidecar), reconciling any job left RUNNING —
	// its goroutine died with the previous process — to INTERRUPTED so it is picked
	// up and visible with its partial artifacts rather than silently lost. The work
	// itself isn't auto-resumed; resumable ops (burn, span) are re-triggered by the
	// operator, and the chunk-status recovery above already reset mid-flight builds.
	if err := s.loadJobsFrom(readFile); err != nil {
		return nil, fmt.Errorf("open job board: %w", err)
	}
	return s, nil
}

// seedBuiltinProfilesLocked inserts any missing built-in profiles and refreshes
// existing ones to their canonical definition (built-ins are immutable). Returns
// true if anything changed. Caller holds no lock — OpenStore runs single-threaded.
func (s *Store) seedBuiltinProfilesLocked() bool {
	changed := false
	for _, bp := range builtinProfiles() {
		found := false
		for i, p := range s.c.Profiles {
			if p.ID == bp.ID {
				found = true
				if !p.Builtin || p.Name != bp.Name || p.RequiredCopies != bp.RequiredCopies ||
					p.RequiredDistinctMediaKinds != bp.RequiredDistinctMediaKinds ||
					p.RequiredOffsiteCopies != bp.RequiredOffsiteCopies || p.VerifyDueMonths != bp.VerifyDueMonths {
					s.c.Profiles[i] = bp
					changed = true
				}
				break
			}
		}
		if !found {
			s.c.Profiles = append(s.c.Profiles, bp)
			changed = true
		}
	}
	return changed
}

// ReadOnly reports whether the store refuses writes because catalog.json was
// created by a newer schema, and why. Callers (startup banner, preflight, the UI)
// surface the reason so the operator knows to upgrade rather than lose data.
func (s *Store) ReadOnly() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readOnly, s.readOnlyReason
}

// runMigrations replays every registered migration whose target version is above
// the file's version and at/below what this build supports, in ascending order,
// then stamps the catalog at the current version. Each migration is idempotent, so
// a re-run (e.g. after a crash mid-migration) reproduces the same result.
func (s *Store) runMigrations(from int) {
	for _, m := range schemaMigrations {
		if m.To > from && m.To <= currentSchemaVersion {
			m.Fn(&s.c)
		}
	}
	s.c.SchemaVersion = currentSchemaVersion
}

// backupBeforeMigrate writes the EXACT pre-migration bytes to a timestamped sidecar
// so a schema upgrade is always reversible. Best-effort: a failed backup logs but
// never blocks the migration (the daily .bak also exists), though a successful one
// is the safety net we want before rewriting the catalog in a new shape.
func (s *Store) backupBeforeMigrate(raw []byte, from int) {
	if len(raw) == 0 {
		return
	}
	name := fmt.Sprintf("%s.pre-schema-v%d-%s", s.path, from, time.Now().Format("20060102-150405"))
	s.notePersist("pre-schema-backup", name)
	_ = os.WriteFile(name, raw, 0o644)
}

// save persists the catalog. During a batched job it COALESCES writes: it marks
// the catalog dirty and skips the write if one happened within batchInterval,
// so a scan/adoption that mutates thousands of times pays a handful of full-file
// writes instead of thousands (each of which marshals the whole catalog — O(n)
// per write, O(n·writes) total). EndBatch forces the final write. Callers hold
// s.mu. Crash-safety is preserved: see BeginBatch.
func (s *Store) save() error {
	if s.batchDepth > 0 {
		s.dirty = true
		iv := s.batchInterval
		if iv <= 0 {
			iv = 3 * time.Second
		}
		if !s.lastSave.IsZero() && time.Since(s.lastSave) < iv {
			return nil // coalesced — a real write happened recently
		}
	}
	return s.writeCatalog()
}

func (s *Store) writeCatalog() error {
	// Record the ATTEMPT first — ahead of the fault seam and the read-only gate below,
	// either of which would otherwise return early and hide it from an observer.
	s.notePersist("catalog-write", s.path)
	// Test-only fault-injection seam (nil in production): simulate a write failure so
	// tests can prove persistence errors propagate rather than being dropped.
	if s.failSave != nil {
		if err := s.failSave(); err != nil {
			return err
		}
	}
	// Forward-compatibility gate: never overwrite a catalog a newer app created.
	if s.readOnly {
		return fmt.Errorf("%s", s.readOnlyReason)
	}
	// Stamp the schema version on every write — this is what makes the file
	// self-describing and lets a future build know how to read it.
	s.c.SchemaVersion = currentSchemaVersion
	// Pretty-print small catalogs (human-inspectable with any text editor — the
	// whole point of the JSON store) but switch to COMPACT JSON once the catalog
	// gets large: at 1M files the indentation alone is ~90 MB of whitespace, and
	// compact form parses meaningfully faster on load. A big catalog isn't
	// text-editor-inspectable anyway; `go test`/tools can pretty-print on demand.
	var b []byte
	var err error
	if len(s.c.Files) > compactThreshold {
		b, err = json.Marshal(&s.c)
	} else {
		b, err = json.MarshalIndent(&s.c, "", " ")
	}
	if err != nil {
		return err
	}
	// Crash durability: os.Rename swaps the directory entry atomically, but the
	// rename does NOT guarantee the temp file's CONTENTS reached stable storage —
	// after a power loss the entry can point at a file whose data blocks were never
	// flushed (a zero-length or torn catalog). So fsync the temp file's bytes BEFORE
	// the rename, then fsync the parent directory AFTER so the rename itself is
	// durable. On Windows syncDir is a no-op (directory fsync is unsupported there).
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	// Best-effort; gracefully skipped where the platform/filesystem does not support
	// a directory fsync (the file content is already durable via f.Sync above).
	_ = syncDir(filepath.Dir(s.path))
	s.lastSave, s.dirty = time.Now(), false
	s.dailyBackup(b)
	return nil
}

// BeginBatch enters batched-save mode: while active, save() coalesces catalog
// writes (at most one per batchInterval) instead of writing on every mutation.
// The App brackets scan / adoption / mirror jobs with BeginBatch/EndBatch.
//
// Crash-safety reasoning: these jobs are IDEMPOTENT RE-RUNS. A scan re-hashes the
// same source tree and UpsertFile replaces entries by (collection, folder, path);
// an adoption re-hashes the same media and skips payloads whose hash is already
// cataloged; a mirror re-writes the same files copy-then-verified. So if the
// process dies mid-job before the final write, re-running the job reproduces the
// same catalog state — nothing is silently lost, unlike a half-finished build or
// write (which are NOT batched and still persist immediately). Batching trades a
// few seconds of not-yet-persisted progress on a crash for avoiding an O(n·m)
// write storm; the trade is safe precisely because the work is replayable.
func (s *Store) BeginBatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchDepth++
}

// EndBatch leaves batched-save mode and forces the final write if anything is
// pending, RETURNING the error so a caller cannot report success for work that was
// never persisted (OB-002).
//
// Two deliberate properties:
//
//   - The error is returned, not dropped. Coalesced progress is allowed to sit in
//     memory during a batch, but the moment a job claims completion its catalog
//     changes must have reached the disk. Every caller folds this into its own error
//     via endBatchInto.
//   - It flushes whenever the catalog is dirty, NOT only when the depth reaches
//     zero. batchDepth is shared by every concurrent job, so the old
//     `batchDepth == 0` condition meant a job finishing while ANOTHER job still held
//     a batch wrote nothing and simply hoped that other job would flush later —
//     someone else's unrelated work became the precondition for this job's
//     durability. A finishing job now always writes.
//
// The tradeoff: when two jobs overlap, the flush also persists the other job's
// partially-applied state. That is safe — the catalog is one document written under
// s.mu, so it is always internally consistent, and every batched job is an
// idempotent re-run (see BeginBatch), so persisting its progress early can only save
// work on a replay. The cost is one extra full-catalog write per overlapping job.
func (s *Store) EndBatch() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.batchDepth > 0 {
		s.batchDepth--
	}
	if !s.dirty {
		return nil
	}
	return s.writeCatalog()
}

// endBatchInto ends a batched job and folds a failed final catalog write into the
// caller's error return. Used as `defer endBatchInto(a.Store, &err)` from every
// batch owner, which is why those functions name their error result.
//
// Neither cause is allowed to mask the other (OB-002): if the operation already
// failed, the flush failure is joined onto it rather than replacing it, and the
// original error stays unwrapped-to so existing errors.Is/As checks keep working.
func endBatchInto(s *Store, err *error) {
	ferr := s.EndBatch()
	if ferr == nil {
		return
	}
	if *err != nil {
		*err = fmt.Errorf("%w; additionally, the catalog could not be saved: %v", *err, ferr)
		return
	}
	*err = fmt.Errorf("the work finished but the catalog could not be saved, so it is NOT recorded: %w", ferr)
}

// dailyBackup writes catalog.json.bak-YYYYMMDD once per calendar day (best
// effort — a backup must never fail a save) and prunes to the newest 14.
func (s *Store) dailyBackup(b []byte) {
	day := time.Now().Format("20060102")
	if s.lastBak == day {
		return
	}
	s.lastBak = day
	bak := s.path + ".bak-" + day
	if _, err := os.Stat(bak); err != nil {
		s.notePersist("backup-create", bak)
		_ = os.WriteFile(bak, b, 0o644)
	}
	matches, _ := filepath.Glob(s.path + ".bak-*")
	sort.Strings(matches) // YYYYMMDD suffix sorts chronologically
	for len(matches) > 14 {
		s.notePersist("backup-prune", matches[0])
		_ = os.Remove(matches[0])
		matches = matches[1:]
	}
}

// notePersist reports an attempted authority write to the observer openStore
// installed. nil in production, so every call site pays one nil check. Callers report
// the attempt BEFORE performing it, and before any gate that could skip it, so "zero
// attempts" is a statement about what the code tried, not only about what survived.
func (s *Store) notePersist(op, path string) {
	if s.persistObserver != nil {
		s.persistObserver(op, path)
	}
}

func (s *Store) next(kind string) int {
	s.c.NextID[kind]++
	return s.c.NextID[kind]
}

func (s *Store) Log(action, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Audit = append(s.c.Audit, Audit{At: time.Now().UTC(), Action: action, Detail: detail})
	_ = s.save()
}

// LastAudit returns the most recent audit-log entry (nil if the log is empty) —
// the "last activity" the Home overview shows.
func (s *Store) LastAudit() *Audit {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.c.Audit) == 0 {
		return nil
	}
	a := s.c.Audit[len(s.c.Audit)-1]
	return &a
}

// ---- collections / folders / files -----------------------------------

func (s *Store) AddCollection(name string) *Collection {
	return s.AddCollectionKind(name, ArchiveSourced)
}

// AddCollectionKind creates an archive of a given kind (SOURCED / SOURCELESS). Both
// get the canonical 3-2-1 rule by default — protection counts copies the same way
// for both; a sourceless archive's file list just comes from adopted media.
func (s *Store) AddCollectionKind(name, kind string) *Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.EqualFold(kind, ArchiveSourceless) {
		kind = ArchiveSourced
	} else {
		kind = ArchiveSourceless
	}
	c := &Collection{ID: s.next("collection"), Name: name, Kind: kind, CreatedAt: time.Now().UTC()}
	s.c.Collections = append(s.c.Collections, c)
	// New Archives default to the canonical 3-2-1 rule (archive-level assignment).
	if s.profileLocked(DefaultProfileID) != nil {
		s.c.Assignments = append(s.c.Assignments, &Assignment{CollectionID: c.ID, Path: "", ProfileID: DefaultProfileID})
	}
	_ = s.save()
	return c
}

func (s *Store) Collections() []*Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Collection{}, s.c.Collections...)
}

func (s *Store) Collection(id int) *Collection {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.c.Collections {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// SetCollectionIntegrity sets (or clears, with nil) an archive's integrity
// override.
func (s *Store) SetCollectionIntegrity(id int, iv *Integrity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.c.Collections {
		if c.ID == id {
			c.Integrity = iv
			return s.save()
		}
	}
	return fmt.Errorf("archive %d not found", id)
}

// SetCollectionRetired retires or unretires an archive. retired=false clears both
// tiers (unretire). retired=true with hidden=false is the soft tier ("keep counting
// it"); hidden=true also hides its numbers from the global totals. Reversible; the
// transition is audit-logged. Never touches files on disk — only what the app shows.
func (s *Store) SetCollectionRetired(id int, retired, hidden bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.c.Collections {
		if c.ID == id {
			was := c.Retired
			c.Retired = retired
			c.RetireHidden = retired && hidden
			if retired {
				if !was {
					now := time.Now().UTC()
					c.RetiredAt = &now
				}
			} else {
				c.RetiredAt = nil
			}
			action, tier := "retire", "keep counting it"
			if !retired {
				action, tier = "unretire", "active"
			} else if hidden {
				tier = "hide its numbers"
			}
			s.c.Audit = append(s.c.Audit, Audit{At: time.Now().UTC(), Action: action,
				Detail: fmt.Sprintf("%s (id %d) — %s; files on disk untouched", c.Name, id, tier)})
			return s.save()
		}
	}
	return fmt.Errorf("archive %d not found", id)
}

// RemoveCollection permanently forgets an archive: it deletes the Collection row and
// every catalog record keyed to it (files, folders, drift, events, conflicts, profile
// assignments, protection summaries, backup sessions) and prunes its id out of any
// plan/dock scope. It deliberately KEEPS volumes, snapshots, keys, and the package /
// copy rows — a medium still self-describes, so its packages remain on the volume view
// as "from a removed archive" (their now-dangling CollectionID is that marker). It
// never deletes anything on disk. Returns what was forgotten vs. kept. The App-layer
// RemoveArchive enforces the confirm-name / export guards before calling this.
func (s *Store) RemoveCollection(id int) (RemoveCounts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var counts RemoveCounts
	found := false
	for _, c := range s.c.Collections {
		if c.ID == id {
			found = true
			break
		}
	}
	if !found {
		return counts, fmt.Errorf("archive %d not found", id)
	}

	// Files / folders (the records being forgotten).
	files := s.c.Files[:0]
	for _, f := range s.c.Files {
		if f.CollectionID == id {
			counts.Files++
		} else {
			files = append(files, f)
		}
	}
	s.c.Files = files
	folders := s.c.Folders[:0]
	for _, fo := range s.c.Folders {
		if fo.CollectionID == id {
			counts.Folders++
		} else {
			folders = append(folders, fo)
		}
	}
	s.c.Folders = folders

	// Other collection-keyed records — dropped (never resurfaced; the media keeps its
	// own truth). Packages/copies (Chunks) are KEPT on purpose.
	drift := s.c.Drift[:0]
	for _, d := range s.c.Drift {
		if d.CollectionID != id {
			drift = append(drift, d)
		}
	}
	s.c.Drift = drift
	events := s.c.Events[:0]
	for _, e := range s.c.Events {
		if e.CollectionID != id {
			events = append(events, e)
		}
	}
	s.c.Events = events
	conflicts := s.c.Conflicts[:0]
	for _, cf := range s.c.Conflicts {
		if cf.CollectionID != id {
			conflicts = append(conflicts, cf)
		}
	}
	s.c.Conflicts = conflicts
	assigns := s.c.Assignments[:0]
	for _, as := range s.c.Assignments {
		if as.CollectionID != id {
			assigns = append(assigns, as)
		}
	}
	s.c.Assignments = assigns
	prot := s.c.Protection[:0]
	for _, p := range s.c.Protection {
		if p.CollectionID != id {
			prot = append(prot, p)
		}
	}
	s.c.Protection = prot
	sessions := s.c.BackupSessions[:0]
	for _, b := range s.c.BackupSessions {
		if b.CollectionID != id {
			sessions = append(sessions, b)
		}
	}
	s.c.BackupSessions = sessions

	// Prune the id out of multi-archive scopes (plans, dock sessions) without deleting
	// those entities.
	for _, p := range s.c.Plans {
		p.ArchiveIDs = pruneID(p.ArchiveIDs, id)
	}
	for _, ds := range s.c.DockSessions {
		ds.ArchiveIDs = pruneID(ds.ArchiveIDs, id)
	}

	// Count kept packages + the distinct volumes they sit on (for the audit line).
	vols := map[int]bool{}
	for _, ch := range s.c.Chunks {
		if ch.CollectionID != id {
			continue
		}
		counts.Packages++
		for _, cp := range ch.Copies {
			if !cp.Superseded {
				vols[cp.VolumeID] = true
			}
		}
		for _, sg := range ch.Segments {
			if sg.VolumeID != 0 {
				vols[sg.VolumeID] = true
			}
		}
	}
	counts.Volumes = len(vols)

	// Finally drop the Collection row and rebuild the file index (stale keys gone).
	colls := s.c.Collections[:0]
	for _, c := range s.c.Collections {
		if c.ID != id {
			colls = append(colls, c)
		}
	}
	s.c.Collections = colls
	s.buildFileIndexLocked()
	if err := s.save(); err != nil {
		return counts, err
	}
	return counts, nil
}

// pruneID returns ids with every occurrence of drop removed (nil-safe, order-stable).
func pruneID(ids []int, drop int) []int {
	if len(ids) == 0 {
		return ids
	}
	out := ids[:0]
	for _, v := range ids {
		if v != drop {
			out = append(out, v)
		}
	}
	return out
}

func (s *Store) AddFolder(collectionID int, path string) *Folder {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Folders { // idempotent per (collection, path)
		if f.CollectionID == collectionID && f.Path == path {
			return f
		}
	}
	f := &Folder{ID: s.next("folder"), CollectionID: collectionID, Path: path}
	s.c.Folders = append(s.c.Folders, f)
	_ = s.save()
	return f
}

func (s *Store) FoldersOf(collectionID int) []*Folder {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Folder
	for _, f := range s.c.Folders {
		if f.CollectionID == collectionID {
			out = append(out, f)
		}
	}
	return out
}

// SourceRoots returns every registered Archive source folder (across all
// collections) — the directories Obelisk only ever OPENS FOR READING.
func (s *Store) SourceRoots() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.c.Folders))
	for _, f := range s.c.Folders {
		if strings.TrimSpace(f.Path) != "" {
			out = append(out, f.Path)
		}
	}
	return out
}

// AssertOutsideSources enforces Obelisk's core invariant — it NEVER writes into
// source data. Any WRITABLE destination (staging, write/span target, restore or
// recovery-kit output, keystore path, …) is refused when it resolves to a path
// at or beneath a registered source root. Empty paths pass (callers do their own
// required-ness check); resolution is best-effort so a not-yet-created
// destination is still checked by its intended absolute path.
func (s *Store) AssertOutsideSources(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	np := normPath(abs)
	for _, root := range s.SourceRoots() {
		rabs, err := filepath.Abs(root)
		if err != nil {
			rabs = root
		}
		rp := normPath(rabs)
		if rp == "" {
			continue
		}
		if np == rp || strings.HasPrefix(np, rp+"/") {
			return fmt.Errorf("refusing: %s is inside source root %s; Obelisk never writes into source data", path, root)
		}
	}
	return nil
}

// UpsertFile replaces a prior entry for (collection, folder, rel_path).
func (s *Store) UpsertFile(f File) *File {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fileIdx == nil {
		s.buildFileIndexLocked()
	}
	now := time.Now().UTC()
	key := fileKey(f.CollectionID, f.FolderID, f.RelPath)
	if e := s.fileIdx[key]; e != nil { // O(1) replace of a prior entry
		if e.FirstSeen.IsZero() { // backfill legacy rows so history has a start date
			e.FirstSeen = now
		}
		// Content changed → RETAIN the outgoing version instead of discarding it.
		// The old bytes are still sealed in whatever package(s) hold e.Hash; keeping
		// the record means the catalog can still point a restore at them.
		if e.Hash != "" && f.Hash != "" && e.Hash != f.Hash {
			e.Versions = append(e.Versions, FileVersion{
				Hash: e.Hash, HashAlg: e.HashAlg, SizeBytes: e.SizeBytes,
				ModTime: e.ModTime, FirstSeen: e.FirstSeen, SupersededAt: now,
			})
			if s.versionsRetained > 0 && len(e.Versions) > s.versionsRetained {
				// Forget only the OLDEST catalog pointers; the bytes remain on media.
				e.Versions = append([]FileVersion(nil), e.Versions[len(e.Versions)-s.versionsRetained:]...)
			}
			e.FirstSeen = now // the new current version is first seen now
		}
		e.SizeBytes, e.HashAlg, e.Hash = f.SizeBytes, f.HashAlg, f.Hash
		if f.Blake3 != "" {
			e.Blake3 = f.Blake3
		}
		if !f.ModTime.IsZero() {
			e.ModTime = f.ModTime
		}
		// Media metadata is refreshed when re-supplied (a later scan may parse EXIF the
		// first pass skipped); never cleared. EventID is preserved — membership is the
		// operator's, not the scanner's, to change.
		if !f.ShotAt.IsZero() {
			e.ShotAt = f.ShotAt
		}
		if f.Role != "" {
			e.Role = f.Role
		}
		if f.CameraSerial != "" {
			e.CameraSerial = f.CameraSerial
		}
		return e
	}
	nf := f
	nf.ID = s.next("file")
	if nf.FirstSeen.IsZero() {
		nf.FirstSeen = now
	}
	s.c.Files = append(s.c.Files, &nf)
	s.fileIdx[key] = &nf
	return &nf
}

// unionFile is one loose file being folded into a SOURCELESS archive's union.
// ShotAt/Role/CameraSerial carry the best-effort EXIF/role captured during the
// adopt walk, so union files become event-clusterable and routable.
type unionFile struct {
	RelPath      string
	Hash         string
	Size         int64
	ShotAt       time.Time
	Role         string
	CameraSerial string
}

// UpsertUnionFiles folds a batch of loose files into a SOURCELESS archive's file
// list, deduped by CONTENT HASH: a file whose hash already exists in the archive
// reuses that File (so the same content on two drives is ONE union entry with two
// copies). Returns the File IDs aligned with items, plus how many were NEW to the
// union (the rest are duplicates already present). One lock, O(files).
func (s *Store) UpsertUnionFiles(collectionID int, items []unionFile) (ids []int, added int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	byHash := map[string]*File{}
	for _, f := range s.c.Files {
		if f.CollectionID == collectionID && f.Hash != "" {
			if _, ok := byHash[f.Hash]; !ok {
				byHash[f.Hash] = f
			}
		}
	}
	now := time.Now().UTC()
	ids = make([]int, len(items))
	for i, it := range items {
		if it.Hash != "" {
			if e := byHash[it.Hash]; e != nil {
				ids[i] = e.ID
				// Backfill EXIF/role onto the shared union entry if this drive's copy
				// carried metadata the first-seen copy lacked.
				if e.ShotAt.IsZero() && !it.ShotAt.IsZero() {
					e.ShotAt = it.ShotAt
				}
				if e.Role == "" && it.Role != "" {
					e.Role = it.Role
				}
				if e.CameraSerial == "" && it.CameraSerial != "" {
					e.CameraSerial = it.CameraSerial
				}
				continue
			}
		}
		nf := &File{ID: s.next("file"), CollectionID: collectionID, FolderID: 0,
			RelPath: it.RelPath, SizeBytes: it.Size, HashAlg: "SHA256", Hash: it.Hash, FirstSeen: now,
			ShotAt: it.ShotAt, Role: it.Role, CameraSerial: it.CameraSerial}
		s.c.Files = append(s.c.Files, nf)
		if it.Hash != "" {
			byHash[it.Hash] = nf
		}
		ids[i] = nf.ID
		added++
	}
	_ = s.save()
	return ids, added
}

// CollectionFileCount is the number of catalog file rows in an archive — for a
// sourceless archive, the size of the deduped union.
func (s *Store) CollectionFileCount(collectionID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, f := range s.c.Files {
		if f.CollectionID == collectionID {
			n++
		}
	}
	return n
}

// SetVersionsRetained sets the per-file retained-version cap (0 = unlimited).
// Called before a scan/reconcile from config so UpsertFile prunes consistently.
func (s *Store) SetVersionsRetained(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < 0 {
		n = 0
	}
	s.versionsRetained = n
}

func (s *Store) Flush() { s.mu.Lock(); defer s.mu.Unlock(); _ = s.save() }

// FilesWithVersions returns every file that has retained at least one superseded
// version (more than one known content version), sorted by path — the input to
// the Recovery Kit's multi-version note.
func (s *Store) FilesWithVersions() []*File {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*File
	for _, f := range s.c.Files {
		if len(f.Versions) > 0 {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out
}

func (s *Store) FilesOf(collectionID int) []*File {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*File
	for _, f := range s.c.Files {
		if f.CollectionID == collectionID {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out
}

// AllFiles returns every catalog file (live records), for cross-archive scans like
// the event magnet. Caller treats elements as read-only unless holding intent.
func (s *Store) AllFiles() []*File {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*File, len(s.c.Files))
	copy(out, s.c.Files)
	return out
}

func (s *Store) FileByID(id int) *File {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Files {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// ExtTally counts files and bytes per extension for a collection (0 = all),
// computed under the lock so the format census never copies the whole file list.
func (s *Store) ExtTally(collectionID int) *extTally {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := newExtTally()
	for _, f := range s.c.Files {
		if collectionID != 0 && f.CollectionID != collectionID {
			continue
		}
		t.add(f.RelPath, f.SizeBytes)
	}
	return t
}

// SearchQuery bundles the search filters. Empty fields are ignored, so a bare
// path query behaves exactly as before.
type SearchQuery struct {
	Text         string    // substring of the file's rel path (case-insensitive)
	Hash         string    // exact or prefix match of the SHA-256 (case-insensitive)
	Ext          string    // file extension filter, e.g. ".nef" (leading dot optional)
	Status       string    // protection status filter (COMPLETE, PARTIAL, …)
	CollectionID int       // 0 = all archives; else scope to this archive
	Role         string    // media role filter (RAW / EDITED-EXPORT / VIDEO / …)
	EventID      int       // event membership filter (0 = ignore; -1 = only unassigned)
	From         time.Time // capture-date lower bound (inclusive) on EXIF ShotAt
	To           time.Time // capture-date upper bound (inclusive)
	Limit        int
}

// Search answers "which chunk / which medium holds this file?" and the inverse
// queries — is THIS hash backed up, show me every unprotected .NEF in an archive.
func (s *Store) Search(qr SearchQuery) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(strings.TrimSpace(qr.Text))
	hash := strings.ToLower(strings.TrimSpace(qr.Hash))
	ext := strings.ToLower(strings.TrimSpace(qr.Ext))
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	status := strings.ToUpper(strings.TrimSpace(qr.Status))
	role := strings.ToUpper(strings.TrimSpace(qr.Role))
	limit := qr.Limit
	if limit <= 0 {
		limit = 200
	}
	eventName := map[int]string{}
	for _, e := range s.c.Events {
		eventName[e.ID] = e.Name
	}
	loc := map[int]*Chunk{} // fileID -> chunk
	for _, ch := range s.c.Chunks {
		for _, cf := range ch.Files {
			loc[cf.FileID] = ch
		}
	}
	vol := map[int]*Volume{}
	for _, v := range s.c.Volumes {
		vol[v.ID] = v
	}
	locs := s.locationsMapLocked()
	// Per-file protection status (the six-status model) so search speaks the same
	// language as the dashboard and protection tree.
	fileCopies := map[int]map[string]physCopy{}
	for _, ch := range s.c.Chunks {
		if ch.Status == "FAILED" {
			continue
		}
		pcs := chunkPhysCopies(ch, vol, locs)
		if len(pcs) == 0 {
			continue
		}
		for _, cf := range ch.Files {
			m := fileCopies[cf.FileID]
			if m == nil {
				m = map[string]physCopy{}
				fileCopies[cf.FileID] = m
			}
			for sig, pc := range pcs {
				m[sig] = pc
			}
		}
	}
	folderPath := map[int]string{}
	for _, fo := range s.c.Folders {
		folderPath[fo.ID] = filepath.ToSlash(fo.Path)
	}
	// By default, retired archives are excluded from search (they've stepped back from
	// attention). Selecting a specific archive (CollectionID > 0) still searches it,
	// retired or not — the filter only applies to the "all archives" default.
	retiredColl := map[int]bool{}
	if qr.CollectionID <= 0 {
		for _, c := range s.c.Collections {
			if c.Retired {
				retiredColl[c.ID] = true
			}
		}
	}
	var out []map[string]any
	for _, f := range s.c.Files {
		if qr.CollectionID > 0 && f.CollectionID != qr.CollectionID {
			continue
		}
		if retiredColl[f.CollectionID] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(f.RelPath), q) {
			continue
		}
		if hash != "" && !strings.HasPrefix(strings.ToLower(f.Hash), hash) {
			continue // prefix match — a full 64-char hash reduces to an exact match
		}
		if ext != "" && !strings.EqualFold(pathExt(f.RelPath), ext) {
			continue
		}
		if role != "" && !strings.EqualFold(f.Role, role) {
			continue
		}
		if qr.EventID > 0 && f.EventID != qr.EventID {
			continue
		}
		if qr.EventID == -1 && f.EventID != 0 { // -1 = only files with no event
			continue
		}
		if !qr.From.IsZero() && (f.ShotAt.IsZero() || f.ShotAt.Before(qr.From)) {
			continue
		}
		if !qr.To.IsZero() && (f.ShotAt.IsZero() || f.ShotAt.After(qr.To)) {
			continue
		}
		pst, pdet, pprof := s.fileProtectionLocked(f, fileCopies, folderPath)
		if status != "" && pst != status {
			continue
		}
		row := map[string]any{"file_id": f.ID, "rel_path": f.RelPath, "size_bytes": f.SizeBytes, "hash": f.Hash, "ext": pathExt(f.RelPath)}
		row["protection_status"], row["protection_detail"] = pst, pdet
		if f.Role != "" {
			row["role"] = f.Role
		}
		if !f.ShotAt.IsZero() {
			row["shot_at"] = f.ShotAt
		}
		if f.EventID != 0 {
			row["event_id"] = f.EventID
			if n := eventName[f.EventID]; n != "" {
				row["event"] = n
			}
		}
		if len(f.Versions) > 0 { // total known content versions (current + retained history)
			row["version_count"] = len(f.Versions) + 1
		}
		if pprof != nil {
			row["profile_name"] = pprof.Name
		}
		if ch, ok := loc[f.ID]; ok {
			row["chunk"] = ch.Name
			row["chunk_status"] = ch.Status
			row["written_dest"] = ch.WrittenDest
			row["key_ref"] = ch.KeyRef
			// The on-medium payload filename to look for (<name>.tar / <name>.tar.gpg).
			row["payload"] = payloadName(ch)
			// "which volumes, verified when?" — the whole point of the feature.
			// Superseded copies are failed-then-rewritten history, not restore
			// sources, so they are omitted here.
			copies := make([]map[string]any, 0, len(ch.Copies))
			for _, cp := range ch.Copies {
				if cp.Superseded {
					continue
				}
				e := map[string]any{"path": cp.Path, "verify_ok": cp.VerifyOK, "last_verified_at": cp.LastVerifiedAt,
					"last_check_level": cp.LastCheckLevel, "last_check_ok": cp.LastCheckOK, "last_check_at": cp.LastCheckAt}
				if v := vol[cp.VolumeID]; v != nil {
					e["volume_label"], e["location"], e["kind"], e["barcode"] = v.Label, v.Location, v.Kind, v.Barcode
				}
				copies = append(copies, e)
			}
			row["copies"] = copies
			row["verified_copies"] = ch.VerifiedCopyCount()
		}
		out = append(out, row)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ---- chunks -----------------------------------------------------------

func (s *Store) AddChunk(c Chunk) *Chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	nc := c
	nc.ID = s.next("chunk")
	nc.CreatedAt = time.Now().UTC()
	s.c.Chunks = append(s.c.Chunks, &nc)
	_ = s.save()
	return &nc
}

func (s *Store) Chunk(id int) *Chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.c.Chunks {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (s *Store) Chunks(collectionID int) []*Chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Chunk
	for _, c := range s.c.Chunks {
		if collectionID == 0 || c.CollectionID == collectionID {
			out = append(out, c)
		}
	}
	return out
}

// ChunkedFileHashes maps fileID -> the hash recorded when it was chunked (from
// ChunkFileRef). A file whose current hash still matches is genuinely backed up;
// a mismatch means the on-disk version changed and needs re-chunking.
func (s *Store) ChunkedFileHashes(collectionID int) map[int]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[int]string{}
	for _, c := range s.c.Chunks {
		if c.CollectionID == collectionID && c.Status != "FAILED" {
			for _, cf := range c.Files {
				m[cf.FileID] = cf.Hash // later (newer) chunk wins for a re-chunked file
			}
		}
	}
	return m
}

// ReplaceDriftReport stores r as the latest report for its collection.
func (s *Store) ReplaceDriftReport(r *DriftReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.c.Drift[:0]
	for _, d := range s.c.Drift {
		if d.CollectionID != r.CollectionID {
			out = append(out, d)
		}
	}
	s.c.Drift = append(out, r)
	_ = s.save()
}

func (s *Store) DriftReport(collectionID int) *DriftReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.c.Drift {
		if d.CollectionID == collectionID {
			return d
		}
	}
	return nil
}

func (s *Store) DriftReports() []*DriftReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*DriftReport{}, s.c.Drift...)
}

func (s *Store) ChunkedFileIDs(collectionID int) map[int]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := map[int]bool{}
	for _, c := range s.c.Chunks {
		if c.CollectionID == collectionID && c.Status != "FAILED" {
			for _, cf := range c.Files {
				m[cf.FileID] = true
			}
		}
	}
	return m
}

// UpdateChunkErr persists the caller's in-memory edits to c and returns any save
// error. UpdateChunk is the legacy void wrapper kept so existing call sites compile
// unchanged; error-aware callers use UpdateChunkErr.
func (s *Store) UpdateChunkErr(c *Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

func (s *Store) UpdateChunk(c *Chunk) { _ = s.UpdateChunkErr(c) }

// AppendVerifyEventErr records one integrity check and persists. Callers set any
// status/verified_at fields on c first; this single save captures them too. If the
// save fails the appended event is rolled back before the error is returned, so a
// verify result that never reached disk is never left looking recorded in memory.
func (s *Store) AppendVerifyEventErr(c *Chunk, ev VerifyEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ev.Level == "" {
		ev.Level = "B" // legacy/always-full checks record as level B
	}
	c.VerifyEvents = append(c.VerifyEvents, ev)
	if err := s.save(); err != nil {
		c.VerifyEvents = c.VerifyEvents[:len(c.VerifyEvents)-1]
		return err
	}
	return nil
}

// AppendVerifyEvent is the legacy void wrapper kept so existing call sites compile
// unchanged; error-aware callers use AppendVerifyEventErr.
func (s *Store) AppendVerifyEvent(c *Chunk, ev VerifyEvent) { _ = s.AppendVerifyEventErr(c, ev) }

// ---- volumes + copies --------------------------------------------------

func (s *Store) ensureUnregisteredLocked() *Volume {
	for _, v := range s.c.Volumes {
		if v.Label == "(unregistered)" {
			return v
		}
	}
	v := &Volume{ID: s.next("volume"), Label: "(unregistered)", Kind: "OTHER",
		Notes: "auto-created for media written before volumes were tracked", CreatedAt: time.Now().UTC()}
	s.c.Volumes = append(s.c.Volumes, v)
	return v
}

func (s *Store) EnsureUnregistered() *Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.ensureUnregisteredLocked()
	_ = s.save()
	return v
}

func (s *Store) AddVolume(v Volume) *Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	nv := v
	nv.ID = s.next("volume")
	nv.CreatedAt = time.Now().UTC()
	if nv.Kind == "" {
		nv.Kind = "OTHER"
	}
	s.c.Volumes = append(s.c.Volumes, &nv)
	_ = s.save()
	return &nv
}

func (s *Store) Volumes() []*Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Volume{}, s.c.Volumes...)
}

func (s *Store) Volume(id int) *Volume {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.c.Volumes {
		if v.ID == id {
			return v
		}
	}
	return nil
}

func (s *Store) VolumeByBarcode(barcode string) *Volume {
	barcode = strings.TrimSpace(barcode)
	if barcode == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.c.Volumes {
		if strings.EqualFold(v.Barcode, barcode) {
			return v
		}
	}
	return nil
}

// VolumeBySerial finds a volume by its resolved physical serial — the key to
// recognizing a re-inserted drive across dock sessions. Empty serial never
// matches (many volumes have none).
func (s *Store) VolumeBySerial(serial string) *Volume {
	serial = strings.TrimSpace(serial)
	if serial == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.c.Volumes {
		if strings.EqualFold(strings.TrimSpace(v.Serial), serial) {
			return v
		}
	}
	return nil
}

func (s *Store) UpdateVolume(v *Volume) { s.mu.Lock(); defer s.mu.Unlock(); _ = s.save() }

// ---- locations ---------------------------------------------------------

// volumeOffsite is the single source of truth for whether a volume counts as the
// "1" in 3-2-1: its Location's Offsite flag when assigned, else the legacy
// per-volume Offsite bool (so unassigned/pre-migration volumes keep working).
func volumeOffsite(v *Volume, locs map[int]*Location) bool {
	if v == nil {
		return false
	}
	if v.LocationID != 0 {
		if l := locs[v.LocationID]; l != nil {
			return l.Offsite
		}
	}
	return v.Offsite
}

// locationsMapLocked indexes locations by ID for offsite resolution. Caller holds s.mu.
func (s *Store) locationsMapLocked() map[int]*Location {
	m := make(map[int]*Location, len(s.c.Locations))
	for _, l := range s.c.Locations {
		m[l.ID] = l
	}
	return m
}

func (s *Store) AddLocation(name string, offsite bool, notes string) *Location {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := &Location{ID: s.next("location"), Name: strings.TrimSpace(name), Offsite: offsite,
		Notes: strings.TrimSpace(notes), CreatedAt: time.Now().UTC()}
	s.c.Locations = append(s.c.Locations, l)
	_ = s.save()
	return l
}

func (s *Store) Locations() []*Location {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Location{}, s.c.Locations...)
}

func (s *Store) Location(id int) *Location {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.c.Locations {
		if l.ID == id {
			return l
		}
	}
	return nil
}

// UpdateLocation edits a location's name/offsite/notes in place. Because offsite
// math reads through the Location, flipping Offsite here re-homes every volume in
// it at once — exactly the point of first-class locations.
func (s *Store) UpdateLocation(id int, name string, offsite bool, notes string) *Location {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.c.Locations {
		if l.ID == id {
			l.Name, l.Offsite, l.Notes = strings.TrimSpace(name), offsite, strings.TrimSpace(notes)
			_ = s.save()
			return l
		}
	}
	return nil
}

// SetVolumeLocation reassigns a volume to a location (0 = clear). It also mirrors
// the location's offsite/label back onto the legacy Volume fields so any code (or
// export) still reading them stays consistent.
func (s *Store) SetVolumeLocation(volumeID, locationID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var vol *Volume
	for _, v := range s.c.Volumes {
		if v.ID == volumeID {
			vol = v
			break
		}
	}
	if vol == nil {
		return fmt.Errorf("volume %d not found", volumeID)
	}
	if locationID == 0 {
		vol.LocationID = 0
		return s.save()
	}
	var loc *Location
	for _, l := range s.c.Locations {
		if l.ID == locationID {
			loc = l
			break
		}
	}
	if loc == nil {
		return fmt.Errorf("location %d not found", locationID)
	}
	vol.LocationID = loc.ID
	vol.Offsite, vol.Location = loc.Offsite, loc.Name // keep legacy fields in sync
	return s.save()
}

// LocationStats returns per-location volume counts + total device bytes, for the
// Locations view. Volumes with no location are grouped under id 0 ("Unassigned").
func (s *Store) LocationStats() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	type agg struct {
		volumes int
		bytes   int64
	}
	byLoc := map[int]*agg{}
	for _, v := range s.c.Volumes {
		if v.Label == "(unregistered)" { // skip the synthetic bucket for pre-volumes media
			continue
		}
		a := byLoc[v.LocationID]
		if a == nil {
			a = &agg{}
			byLoc[v.LocationID] = a
		}
		a.volumes++
		a.bytes += v.DeviceSize
	}
	out := []map[string]any{}
	for _, l := range s.c.Locations {
		a := byLoc[l.ID]
		if a == nil {
			a = &agg{}
		}
		out = append(out, map[string]any{"id": l.ID, "name": l.Name, "offsite": l.Offsite,
			"notes": l.Notes, "volumes": a.volumes, "bytes": a.bytes})
	}
	if a := byLoc[0]; a != nil && a.volumes > 0 {
		out = append(out, map[string]any{"id": 0, "name": "Unassigned", "offsite": false,
			"notes": "volumes not yet placed in a location", "volumes": a.volumes, "bytes": a.bytes})
	}
	return out
}

// AppendSmartSnapshot records a drive-health reading in the volume's history
// (newest last), capped to the most recent 50 so trends stay visible across dock
// sessions without unbounded growth. Persists.
func (s *Store) AppendSmartSnapshot(v *Volume, snap SmartSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.SmartHistory = append(v.SmartHistory, snap)
	if len(v.SmartHistory) > 50 {
		v.SmartHistory = v.SmartHistory[len(v.SmartHistory)-50:]
	}
	_ = s.save()
}

// ---- drive snapshots ---------------------------------------------------

// PutVolumeSnapshot stores (or replaces) the offline inventory for a volume. One
// snapshot per volume, keyed by VolumeID — a re-ingest overwrites in place.
// Persists (coalesced under an active batch).
func (s *Store) PutVolumeSnapshot(snap *VolumeSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, ex := range s.c.Snapshots {
		if ex.VolumeID == snap.VolumeID {
			s.c.Snapshots[i] = snap
			_ = s.save()
			return
		}
	}
	s.c.Snapshots = append(s.c.Snapshots, snap)
	_ = s.save()
}

// VolumeSnapshot returns the offline inventory for a volume, or nil if the drive
// has never been ingested. The returned pointer is the live catalog record —
// treat it as read-only.
func (s *Store) VolumeSnapshot(volumeID int) *VolumeSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, snap := range s.c.Snapshots {
		if snap.VolumeID == volumeID {
			return snap
		}
	}
	return nil
}

// VolumeSnapshots returns every drive snapshot (for cross-drive duplicate/mirror
// comparison). The slice is a fresh copy; the elements are live records.
func (s *Store) VolumeSnapshots() []*VolumeSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*VolumeSnapshot, len(s.c.Snapshots))
	copy(out, s.c.Snapshots)
	return out
}

// ---- events ------------------------------------------------------------

// AddEvent creates an event (stamping ID + CreatedAt) and persists.
func (s *Store) AddEvent(e *Event) *Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.ID = s.next("event")
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	s.c.Events = append(s.c.Events, e)
	_ = s.save()
	return e
}

// Events returns events scoped to a collection (0 = all), newest-created first.
func (s *Store) Events(collectionID int) []*Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*Event{}
	for _, e := range s.c.Events {
		if collectionID == 0 || e.CollectionID == collectionID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Event returns one event by ID (live record), or nil.
func (s *Store) Event(id int) *Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.c.Events {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// UpdateEvent persists in-place edits to an event record.
func (s *Store) UpdateEvent(e *Event) { s.mu.Lock(); defer s.mu.Unlock(); _ = s.save() }

// DeleteEvent removes an event and clears the membership of any files pointing at
// it (their EventID resets to 0 — unassigned, not deleted).
func (s *Store) DeleteEvent(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.c.Events[:0]
	for _, e := range s.c.Events {
		if e.ID != id {
			out = append(out, e)
		}
	}
	s.c.Events = out
	for _, f := range s.c.Files {
		if f.EventID == id {
			f.EventID = 0
		}
	}
	_ = s.save()
}

// AssignFilesToEvent sets the EventID on the given files (0 to unassign). Returns
// how many rows changed. One lock; persists once.
func (s *Store) AssignFilesToEvent(fileIDs []int, eventID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	want := map[int]bool{}
	for _, id := range fileIDs {
		want[id] = true
	}
	n := 0
	for _, f := range s.c.Files {
		if want[f.ID] && f.EventID != eventID {
			f.EventID = eventID
			n++
		}
	}
	if n > 0 {
		_ = s.save()
	}
	return n
}

// FilesOfEvent returns the files whose membership points at an event.
func (s *Store) FilesOfEvent(eventID int) []*File {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*File{}
	for _, f := range s.c.Files {
		if f.EventID == eventID {
			out = append(out, f)
		}
	}
	return out
}

// ---- templates ---------------------------------------------------------

// AddTemplate creates a routing template and persists.
func (s *Store) AddTemplate(t *Template) *Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	t.ID = s.next("template")
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	s.c.Templates = append(s.c.Templates, t)
	_ = s.save()
	return t
}

// Templates returns all routing templates (built-in first, then by ID).
func (s *Store) Templates() []*Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Template, len(s.c.Templates))
	copy(out, s.c.Templates)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BuiltIn != out[j].BuiltIn {
			return out[i].BuiltIn
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Template returns one template by ID (live record), or nil.
func (s *Store) Template(id int) *Template {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.c.Templates {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// UpdateTemplate persists in-place edits to a template.
func (s *Store) UpdateTemplate(t *Template) { s.mu.Lock(); defer s.mu.Unlock(); _ = s.save() }

// DeleteTemplate removes a template (built-ins included; a fresh open re-seeds the
// built-in only when NO templates exist).
func (s *Store) DeleteTemplate(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.c.Templates[:0]
	for _, t := range s.c.Templates {
		if t.ID != id {
			out = append(out, t)
		}
	}
	s.c.Templates = out
	_ = s.save()
}

// ---- backup sessions (incremental history) -----------------------------

// AddBackupSession appends an incremental-run record (newest last) and persists.
func (s *Store) AddBackupSession(b *BackupSession) *BackupSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	b.ID = s.next("backup")
	if b.At.IsZero() {
		b.At = time.Now().UTC()
	}
	s.c.BackupSessions = append(s.c.BackupSessions, b)
	_ = s.save()
	return b
}

// BackupSessions returns the run history for a collection (0 = all), newest first.
func (s *Store) BackupSessions(collectionID int) []*BackupSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*BackupSession
	for _, b := range s.c.BackupSessions {
		if collectionID == 0 || b.CollectionID == collectionID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// starterTemplates is the built-in template SET installed on a fresh catalog — one
// per creative discipline, so every user has a discipline-shaped starting point to
// clone and edit (photography is now one profile among peers, not the assumption).
// All route over the SAME discipline-neutral role taxonomy and Event model, using
// the aliased grouping tokens ({event}/{collection}/{session}/{project}).
// CreatedAt is deliberately left zero: built-ins are canonical, not operator-
// authored, so serialization stays deterministic across catalogs.
func starterTemplates() []*Template {
	return []*Template{
		{
			Name: "Photographer", BuiltIn: true, GroupNoun: "Event",
			EventTypes: append([]string(nil), photographerVocabulary...),
			Routes: map[string]string{
				RoleOriginals:    "photos/{year}/{event_type}/{event}/",
				RoleSidecars:     "photos/{year}/{event_type}/{event}/",
				RoleProject:      "photos/{year}/{event_type}/{event}/",
				RoleDeliverables: "Finalized Images/{year}/{event_type}/{event}/",
			},
		},
		{
			Name: "Musician", BuiltIn: true, GroupNoun: "Project",
			EventTypes: append([]string(nil), musicianVocabulary...),
			Routes: map[string]string{
				RoleOriginals:    "music/{year}/{project}/stems/",
				RoleDeliverables: "music/{year}/{project}/masters/",
				RoleSidecars:     "music/{year}/{project}/",
				RoleProject:      "music/{year}/{project}/project/",
			},
		},
		{
			Name: "Filmmaker", BuiltIn: true, GroupNoun: "Project",
			EventTypes: append([]string(nil), filmmakerVocabulary...),
			Routes: map[string]string{
				RoleOriginals:    "footage/{year}/{project}/",
				RoleProject:      "projects/{year}/{project}/",
				RoleDeliverables: "deliverables/{year}/{project}/",
				RoleSidecars:     "footage/{year}/{project}/",
			},
		},
		{
			Name: "General", BuiltIn: true, GroupNoun: "Collection",
			EventTypes: append([]string(nil), generalVocabulary...),
			Routes: map[string]string{
				RoleOriginals:    "{year}/{category}/{collection}/",
				RoleDeliverables: "{year}/{category}/{collection}/",
				RoleSidecars:     "{year}/{category}/{collection}/",
				RoleProject:      "{year}/{category}/{collection}/",
			},
		},
	}
}

// seedBuiltinTemplateLocked installs the starter template SET on a catalog that has
// none. Caller holds s.mu (OpenStore is single-threaded at call time).
func (s *Store) seedBuiltinTemplateLocked() {
	if len(s.c.Templates) > 0 {
		return
	}
	for _, t := range starterTemplates() {
		t.ID = s.next("template")
		s.c.Templates = append(s.c.Templates, t)
	}
}

// ---- tape diagnostics --------------------------------------------------

// AddTapeCheck records a tape-drive diagnostic snapshot (newest last), capped to
// the most recent 50. Persists.
func (s *Store) AddTapeCheck(t TapeHealth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.TapeChecks = append(s.c.TapeChecks, t)
	if len(s.c.TapeChecks) > 50 {
		s.c.TapeChecks = s.c.TapeChecks[len(s.c.TapeChecks)-50:]
	}
	_ = s.save()
}

// LastTapeCheck returns the most recent tape-drive snapshot, or nil if none.
func (s *Store) LastTapeCheck() *TapeHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.c.TapeChecks) == 0 {
		return nil
	}
	t := s.c.TapeChecks[len(s.c.TapeChecks)-1]
	return &t
}

// TapeChecks returns a copy of the snapshot history (newest last).
func (s *Store) TapeChecks() []TapeHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]TapeHealth{}, s.c.TapeChecks...)
}

// ---- dock sessions -----------------------------------------------------

func (s *Store) AddDockSession(ds *DockSession) *DockSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	ds.ID = s.next("dock")
	now := time.Now().UTC()
	ds.StartedAt, ds.UpdatedAt = now, now
	if ds.Status == "" {
		ds.Status = "ACTIVE"
	}
	s.c.DockSessions = append(s.c.DockSessions, ds)
	_ = s.save()
	return ds
}

func (s *Store) DockSession(id int) *DockSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ds := range s.c.DockSessions {
		if ds.ID == id {
			return ds
		}
	}
	return nil
}

// ActiveDockSession returns the most recent ACTIVE session (the one a reopened
// app resumes), or nil when none is open.
func (s *Store) ActiveDockSession() *DockSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out *DockSession
	for _, ds := range s.c.DockSessions {
		if ds.Status == "ACTIVE" && (out == nil || ds.ID > out.ID) {
			out = ds
		}
	}
	return out
}

func (s *Store) DockSessions() []*DockSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*DockSession{}, s.c.DockSessions...)
}

func (s *Store) UpdateDockSession(ds *DockSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ds.UpdatedAt = time.Now().UTC()
	_ = s.save()
}

// RecordDockDrive appends (or, when the same volume is re-processed, replaces)
// a drive's result in the session and persists. Matching by volume keeps a
// re-verify from duplicating the row for a drive already in the session.
func (s *Store) RecordDockDrive(ds *DockSession, d DockDrive) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	d.FinishedAt = &now
	for i := range ds.Drives {
		if ds.Drives[i].VolumeID == d.VolumeID {
			ds.Drives[i] = d
			ds.UpdatedAt = now
			_ = s.save()
			return
		}
	}
	ds.Drives = append(ds.Drives, d)
	ds.UpdatedAt = now
	_ = s.save()
}

// barcodeSeq matches a "<PREFIX>-<digits>" barcode (dash optional) so the next
// number in a scheme is max(existing)+1 — no stored counter to drift out of sync.
var barcodeSeq = regexp.MustCompile(`^(.*?)-?(\d+)$`)

// NextBarcode returns the next unused barcode for prefix, formatted
// "<prefix>-NNNN" (4-digit, zero-padded, growing past 9999). It scans existing
// volume barcodes sharing that prefix, takes the highest trailing number, and
// adds one — so NSP-0001, NSP-0002, … stay gap-free and never collide.
func (s *Store) NextBarcode(prefix string) string {
	prefix = strings.TrimRight(strings.TrimSpace(prefix), "-")
	if prefix == "" {
		prefix = "NSP"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	max := 0
	for _, v := range s.c.Volumes {
		m := barcodeSeq.FindStringSubmatch(strings.TrimSpace(v.Barcode))
		if m == nil || !strings.EqualFold(strings.TrimRight(m[1], "-"), prefix) {
			continue
		}
		if n, err := strconv.Atoi(m[2]); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s-%04d", prefix, max+1)
}

// RecordCopy adds or refreshes the current (non-superseded) copy of chunk c on
// the given volume and persists. verifiedOK reflects the read-back that just
// happened. A superseded copy on the same volume is left untouched as history.
func (s *Store) RecordCopy(c *Chunk, volumeID int, path string, verifiedOK bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	ok := verifiedOK
	for i := range c.Copies {
		if c.Copies[i].VolumeID == volumeID && !c.Copies[i].Superseded {
			c.Copies[i].Path = path
			c.Copies[i].LastVerifiedAt = &now
			c.Copies[i].VerifyOK = &ok
			_ = s.save()
			return
		}
	}
	c.Copies = append(c.Copies, Copy{VolumeID: volumeID, Path: path, WrittenAt: &now, LastVerifiedAt: &now, VerifyOK: &ok})
	_ = s.save()
}

// UpdateCopyVerify records a LEVEL-B verify result on the current copy matching
// path (back-compat: all existing callers are full-content checks).
func (s *Store) UpdateCopyVerify(c *Chunk, path string, ok bool) {
	s.UpdateCopyVerifyLevel(c, path, ok, "B")
}

// UpdateCopyVerifyLevel refreshes a copy's verify state at the given level. Level
// B updates last_verified_at/verify_ok (the qualifying, verify-due-refreshing
// bar); levels A and C only update the advisory last_check_* fields, so they can
// never satisfy COMPLETE or reset the verify-due clock.
func (s *Store) UpdateCopyVerifyLevel(c *Chunk, path string, ok bool, level string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	apply := func(i int) {
		v := ok
		lvl := level
		c.Copies[i].LastCheckAt, c.Copies[i].LastCheckLevel, c.Copies[i].LastCheckOK = &now, lvl, &v
		if level == "B" {
			c.Copies[i].LastVerifiedAt, c.Copies[i].VerifyOK = &now, &v
		}
	}
	matched := false
	current := 0
	var soleIdx int
	for i := range c.Copies {
		if c.Copies[i].Superseded {
			continue
		}
		current++
		soleIdx = i
		// base-mount vs chunk-subfolder both match (one contains the other),
		// tolerant of separator/case differences across platforms.
		if pathRelated(c.Copies[i].Path, path) {
			apply(i)
			matched = true
		}
	}
	if !matched && current == 1 {
		apply(soleIdx)
	}
	_ = s.save()
}

// SupersedeCopy marks the current (non-superseded) copy of c on volumeID as
// superseded — retained in history — and returns its destination folder so a
// re-write can target the same medium. Returns "" if there is no current copy.
func (s *Store) SupersedeCopy(c *Chunk, volumeID int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range c.Copies {
		if c.Copies[i].VolumeID == volumeID && !c.Copies[i].Superseded {
			c.Copies[i].Superseded = true
			p := c.Copies[i].Path
			_ = s.save()
			return p
		}
	}
	return ""
}

// VerifiedCopyCount returns how many current copies last verified OK (spanned
// chunks count as one copy once fully written+verified). Superseded copies —
// history of failed-then-rewritten media — are excluded.
func (c *Chunk) VerifiedCopyCount() int {
	n := 0
	for _, cp := range c.Copies {
		if !cp.Superseded && cp.VerifyOK != nil && *cp.VerifyOK {
			n++
		}
	}
	return n
}

// CurrentCopyCount returns how many non-superseded copies exist (verified or
// not) — the number of live physical instances the catalog tracks.
func (c *Chunk) CurrentCopyCount() int {
	n := 0
	for _, cp := range c.Copies {
		if !cp.Superseded {
			n++
		}
	}
	return n
}

// ---- protection profiles + assignments ---------------------------------

func (s *Store) Profiles() []*Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]*Profile{}, s.c.Profiles...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Builtin != out[j].Builtin {
			return out[i].Builtin // built-ins first
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *Store) Profile(id string) *Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profileLocked(id)
}

func (s *Store) profileLocked(id string) *Profile {
	for _, p := range s.c.Profiles {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// AddProfile persists a new custom profile. builtin is always forced false; the
// id is slugified from the name (uniqued with a numeric suffix) unless provided.
func (s *Store) AddProfile(p Profile) *Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Builtin = false
	if strings.TrimSpace(p.ID) == "" {
		p.ID = s.uniqueProfileIDLocked(slugify(p.Name))
	} else if s.profileLocked(p.ID) != nil {
		p.ID = s.uniqueProfileIDLocked(p.ID)
	}
	np := p
	s.c.Profiles = append(s.c.Profiles, &np)
	_ = s.save()
	return &np
}

func (s *Store) uniqueProfileIDLocked(base string) string {
	if base == "" {
		base = "profile"
	}
	id := base
	for n := 2; s.profileLocked(id) != nil; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// UpdateProfile replaces a custom profile's editable fields. Built-in profiles
// are immutable and refused.
func (s *Store) UpdateProfile(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.profileLocked(p.ID)
	if cur == nil {
		return fmt.Errorf("profile %q not found", p.ID)
	}
	if cur.Builtin {
		return fmt.Errorf("%q is a built-in profile and cannot be edited — duplicate it to make a custom copy", cur.Name)
	}
	cur.Name = p.Name
	cur.Description = p.Description
	cur.RequiredCopies = p.RequiredCopies
	cur.RequiredDistinctMediaKinds = p.RequiredDistinctMediaKinds
	cur.RequiredOffsiteCopies = p.RequiredOffsiteCopies
	cur.MediaKindsAllowed = p.MediaKindsAllowed
	cur.VerifyDueMonths = p.VerifyDueMonths
	return s.save()
}

// DeleteProfile removes a custom profile. Built-in profiles are refused, and a
// profile still assigned to any archive/folder is refused with the list of what
// uses it so the operator can reassign first.
func (s *Store) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.profileLocked(id)
	if cur == nil {
		return fmt.Errorf("profile not found")
	}
	if cur.Builtin {
		return fmt.Errorf("%q is a built-in profile and cannot be deleted", cur.Name)
	}
	if users := s.profileUsersLocked(id); len(users) > 0 {
		return fmt.Errorf("%q is still assigned to %d place(s): %s — reassign them first", cur.Name, len(users), strings.Join(users, "; "))
	}
	out := s.c.Profiles[:0]
	for _, p := range s.c.Profiles {
		if p.ID != id {
			out = append(out, p)
		}
	}
	s.c.Profiles = out
	return s.save()
}

// profileUsersLocked returns human-readable descriptions of every assignment
// that references profile id ("Photography" / "Photography › To-Delete-2020").
func (s *Store) profileUsersLocked(id string) []string {
	name := map[int]string{}
	for _, c := range s.c.Collections {
		name[c.ID] = c.Name
	}
	var out []string
	for _, a := range s.c.Assignments {
		if a.ProfileID != id {
			continue
		}
		desc := name[a.CollectionID]
		if desc == "" {
			desc = fmt.Sprintf("archive#%d", a.CollectionID)
		}
		if a.Path != "" {
			desc += " › " + a.Path
		}
		out = append(out, desc)
	}
	return out
}

func (s *Store) Assignments() []*Assignment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Assignment{}, s.c.Assignments...)
}

func (s *Store) AssignmentsOf(collectionID int) []*Assignment {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Assignment
	for _, a := range s.c.Assignments {
		if a.CollectionID == collectionID {
			out = append(out, a)
		}
	}
	return out
}

// SetAssignment upserts the profile for (collection, path); an empty profileID
// removes the assignment (so the node falls back to inheriting an ancestor's).
// path "" is the archive-level assignment. Comparison is path-normalised so a
// re-typed drive letter/case does not create a duplicate node assignment.
func (s *Store) SetAssignment(collectionID int, path, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if profileID != "" && s.profileLocked(profileID) == nil {
		return fmt.Errorf("profile %q not found", profileID)
	}
	np := normPath(path)
	for i, a := range s.c.Assignments {
		if a.CollectionID == collectionID && normPath(a.Path) == np {
			if profileID == "" {
				s.c.Assignments = append(s.c.Assignments[:i], s.c.Assignments[i+1:]...)
			} else {
				s.c.Assignments[i].ProfileID = profileID
			}
			return s.save()
		}
	}
	if profileID != "" {
		s.c.Assignments = append(s.c.Assignments, &Assignment{CollectionID: collectionID, Path: path, ProfileID: profileID})
	}
	return s.save()
}

// resolveProfileLocked returns the nearest-ancestor-wins profile for a logical
// path in a collection: the assignment whose path is the longest ancestor of
// fullPath (an empty path is the archive-level root, ancestor of everything),
// ultimately the archive's assignment. Returns nil when nothing resolves
// (UNASSIGNED). Caller holds s.mu.
func (s *Store) resolveProfileLocked(collectionID int, fullPath string) *Profile {
	bestLen := -1
	var best *Assignment
	for _, a := range s.c.Assignments {
		if a.CollectionID != collectionID {
			continue
		}
		if !pathIsAncestor(a.Path, fullPath) {
			continue
		}
		if l := len(normPath(a.Path)); l > bestLen {
			bestLen, best = l, a
		}
	}
	if best == nil {
		return nil
	}
	return s.profileLocked(best.ProfileID)
}

// ---- keys (metadata only) ----------------------------------------------

func (s *Store) AddKeyMeta(k KeyMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c.Keys = append(s.c.Keys, &k)
	_ = s.save()
}
func (s *Store) KeyMetas() []*KeyMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*KeyMeta{}, s.c.Keys...)
}

// ---- jobs (persisted to a jobs.json sidecar; in-progress ones are picked up as
// INTERRUPTED on the next open — see loadJobs) --------------------------------

// saveJobs persists the job board to the sidecar (atomic temp-swap, same shape as
// writeCatalog). Called ONLY on meaningful transitions — create, artifact change,
// and terminal status — never on progress/telemetry ticks (those fire many times a
// second and stay in-memory; they carry nothing worth surviving a restart). Caller
// holds s.jobs.mu.
//
// OB-002: this RETURNS its error and actually checks every step. It previously
// swallowed all three failure modes — a marshal error returned silently, a failed
// temp write skipped the rename without a word, and the rename's own error was
// discarded — so "the job is COMPLETED" could be a purely in-memory claim while
// jobs.json still said RUNNING. It now also fsyncs the bytes before the rename, the
// same durability writeCatalog already had: a rename publishes a directory entry,
// not the data blocks behind it.
//
// A failed publication leaves the PREVIOUS good jobs.json untouched: the file is
// only ever replaced by a completed, synced temp file.
//
// It is also where an earlier recording failure is FORGIVEN. This write serialises
// every row, so a write that succeeds records the current terminal snapshot of every
// job on the board — including one whose own terminal write failed earlier. That row
// is therefore recorded now, and must stop saying otherwise. See writeJobsRows for
// why the flags are cleared before the bytes are produced rather than after.
func (s *Store) saveJobs() error {
	if s.jobs.loadErr != nil {
		return fmt.Errorf("job board unavailable after refused load: %w", s.jobs.loadErr)
	}
	if s.jobs.path == "" {
		return nil
	}
	// Test-only fault-injection seam (nil in production), consulted before any work
	// so a test can prove the error reaches the caller rather than being dropped.
	// Checked first, before any flag is touched: a seam failure must leave the board
	// exactly as it found it.
	if s.failSaveJobs != nil {
		if err := s.failSaveJobs(); err != nil {
			return err
		}
	}
	// Clear the not-recorded qualification on every flagged row BEFORE marshalling,
	// and put it back if the write fails.
	//
	// Clearing first is what keeps the serialised bytes and the published in-memory
	// state in agreement: if the write lands, the file says "recorded" and so does
	// memory; if it does not, both say "not recorded". Clearing AFTER a successful
	// write would leave `unrecorded: true` in the very file that proves otherwise,
	// and a restart would read that stale claim back.
	//
	// This is not optimistic publication. The caller holds s.jobs.mu for the whole
	// window, and every reader (Job, Jobs) takes the same mutex, so the cleared-but-
	// unwritten state is not observable outside this function: a reader either blocks
	// or sees the state from before the attempt. PersistError is deliberately NOT
	// cleared — that is history, and it is written out with the row.
	var forgiven []*Job
	for _, j := range s.jobs.rows {
		if j.Unrecorded {
			j.Unrecorded = false
			forgiven = append(forgiven, j)
		}
	}
	if err := s.writeJobsRows(); err != nil {
		for _, j := range forgiven {
			j.Unrecorded = true
		}
		return err
	}
	return nil
}

// writeJobsRows is saveJobs' actual file publication: marshal, write to a temp file,
// fsync, rename. Split out so saveJobs' recording-state bookkeeping wraps exactly the
// bytes it is responsible for. Caller holds s.jobs.mu.
//
// Every error it can return is PRE-publication: the only step that runs after the
// rename is the parent-directory sync, whose error is deliberately discarded. That is
// what makes NewJob's row/ID rollback safe — a returned error means nothing landed.
func (s *Store) writeJobsRows() error {
	b, err := json.MarshalIndent(struct {
		Next int    `json:"next"`
		Rows []*Job `json:"rows"`
	}{s.jobs.next, s.jobs.rows}, "", " ")
	if err != nil {
		return err
	}
	tmp := s.jobs.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.jobs.path); err != nil {
		return err
	}
	// Best-effort, and a no-op on Windows: the bytes are already durable via Sync
	// above, this only makes the rename itself survive a power loss where supported.
	_ = syncDir(filepath.Dir(s.jobs.path))
	s.jobs.loaded = true
	return nil
}

// NewJob creates a job row and PERSISTS it before returning (OB-002). If the board
// cannot be written the row is rolled back — including the ID counter, so the failed
// attempt does not burn an ID — and the error is returned, because work must not be
// launched under an ID the system would then describe as recorded. runJob turns this
// into a truthful 503 and starts nothing.
func (s *Store) NewJob(kind, label string) (*Job, error) {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if s.jobs.loadErr != nil {
		return nil, fmt.Errorf("job board unavailable after refused load: %w", s.jobs.loadErr)
	}
	if s.jobs.next == int(^uint(0)>>1) {
		return nil, fmt.Errorf("job identity counter exhausted; no work was started")
	}
	s.jobs.next++
	j := &Job{ID: s.jobs.next, Kind: kind, Label: label, Status: "RUNNING", CreatedAt: time.Now().UTC()}
	s.jobs.rows = append(s.jobs.rows, j)
	if err := s.saveJobs(); err != nil {
		s.jobs.rows = s.jobs.rows[:len(s.jobs.rows)-1]
		s.jobs.next--
		return nil, fmt.Errorf("the job board could not be written, so no work was started: %w", err)
	}
	return j, nil
}

// SetJob updates a job row, persisting only on a terminal transition. It returns
// that write's error (OB-002): a caller that publishes COMPLETED must know whether
// the completion was actually recorded. Progress-only calls stay in memory and
// return nil.
func (s *Store) SetJob(id int, progress float64, label, status string) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	var row *Job
	terminal := false
	for _, j := range s.jobs.rows {
		if j.ID == id {
			row = j
			if progress >= 0 {
				j.Progress = progress
			}
			if label != "" {
				j.Label = label
			}
			if status != "" {
				j.Status = status
				if status == "COMPLETED" || status == "FAILED" || status == "INTERRUPTED" {
					terminal = true
					now := time.Now().UTC()
					j.FinishedAt = &now
				}
			}
		}
	}
	// Persist only when a job reaches a terminal state — progress-only updates stay
	// in-memory (the sidecar carries outcomes, not live progress).
	if terminal {
		err := s.saveJobs()
		// Same rule as FinishJob: the qualification is published in the SAME lock hold
		// as the terminal status it qualifies, never by a later call.
		markJobUnrecordedLocked(row, err)
		return err
	}
	return nil
}

// markJobUnrecordedLocked records that a job's CURRENT terminal snapshot could not be
// written, in the same critical section that published that snapshot. Caller holds
// s.jobs.mu.
//
// This is the whole of Blocker 1's fix: the status and the qualification of the
// status become visible together or not at all. It used to be a separate exported
// call made after the lock was released, which left a window in which GET /api/jobs
// handed out a plain, unqualified COMPLETED for a job whose record on disk still said
// RUNNING — exactly the falsehood OB-002 exists to remove.
//
// It sets both halves of the contract: Unrecorded (current state) and PersistError
// (history). It does NOT touch the label — a mutated label cannot be un-mutated when
// a later write records the row, and appending to it twice would double the prefix.
// Deliberately does nothing when cause is nil, so the ordinary success path costs a
// nil check and clears nothing it should not.
//
// It never saves and never logs: the sidecar is the thing that just failed, and
// process logging and the catalog audit fallback belong outside this mutex (see
// App.noteUnrecordedJob).
func markJobUnrecordedLocked(j *Job, cause error) {
	if j == nil || cause == nil {
		return
	}
	j.Unrecorded = true
	j.PersistError = cause.Error()
}

// FinishJob publishes a job's terminal state together with the artifacts and result
// that describe it, in ONE checked sidecar write (OB-002).
//
// Ordering matters and is the reason this exists. runJob used to set COMPLETED
// first and then attach the artifacts and result in two further writes, so a reader
// could see a completed job whose outputs were not yet recorded, and the terminal
// write was not the last word about the job. Here the descriptive fields are filled
// in memory and the status is published with them in a single write, so the record
// that reaches jobs.json is complete or is not there at all.
//
// The error is still the caller's to handle — runJob logs it and attempts the audit
// fallback — but the row's own truthfulness no longer depends on the caller doing so:
// when the write fails, this marks the row not-recorded BEFORE releasing the mutex, so
// no reader can ever observe an unqualified COMPLETED for a job whose record did not
// land. Everything a reader needs — execution outcome, artifacts, result, current
// recording state, and any recording-failure qualification — is established inside
// this one critical section.
func (s *Store) FinishJob(id int, progress float64, label, status string, arts []Artifact, result map[string]any) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	var row *Job
	found := false
	for _, j := range s.jobs.rows {
		if j.ID != id {
			continue
		}
		found = true
		row = j
		if progress >= 0 {
			j.Progress = progress
		}
		if label != "" {
			j.Label = label
		}
		if len(arts) > 0 {
			j.Artifacts = arts
		}
		if result != nil {
			j.Result = result
		}
		j.RateMBps, j.ETASeconds = 0, 0
		if status != "" {
			j.Status = status
			now := time.Now().UTC()
			j.FinishedAt = &now
		}
	}
	if !found {
		return nil
	}
	err := s.saveJobs()
	// Still inside the same s.jobs.mu hold that published the status above: the
	// terminal snapshot and its qualification become visible together.
	markJobUnrecordedLocked(row, err)
	return err
}

// There is deliberately no exported "note this job unrecorded" call. It existed, and
// it was the bug: setting the qualification from outside meant re-taking s.jobs.mu
// after FinishJob had already published and released, leaving a window in which
// GET /api/jobs served an unqualified COMPLETED. The qualification is now set by
// markJobUnrecordedLocked inside the write's own lock hold, and a late external call
// would be worse than useless — by then an ordinary save may legitimately have
// recorded the row, and re-flagging it would report a missing record that is present.
//
// It is also why nothing retries here. The sidecar is the thing that just failed;
// writing the failure into it would either fail again or loop. Recovery is ordinary,
// not scheduled: the next successful jobs write — a new job, another job finishing —
// serialises the whole board including this row, and saveJobs clears the flag as part
// of producing those bytes. Until that happens the durable record stays whatever was
// last written (RUNNING), which loadJobs reports as INTERRUPTED on the next start.

// AppendJobArtifact records one artifact on a running job and persists it, so a
// job's detail view fills in as the work produces outputs. The write error is
// returned; a running job's artifact list is progress, so callers may reasonably
// ignore it, but no caller is forced to.
func (s *Store) AppendJobArtifact(id int, a Artifact) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	for _, j := range s.jobs.rows {
		if j.ID == id {
			j.Artifacts = append(j.Artifacts, a)
			return s.saveJobs()
		}
	}
	return nil
}

// SetJobArtifacts replaces a job's artifact list (used when a job reports all its
// artifacts at once on completion) and persists.
func (s *Store) SetJobArtifacts(id int, arts []Artifact) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	for _, j := range s.jobs.rows {
		if j.ID == id {
			j.Artifacts = arts
			return s.saveJobs()
		}
	}
	return nil
}

// snapshot returns a job row a caller can keep and read after s.jobs.mu is released.
//
// It is a DEEP copy of the parts that are not plain values. A bare `cp := *j` shares
// the Result map and the Artifacts backing array with the live row, so the returned
// "copy" would still be aliasing mutable state the store owns — and the /api/jobs
// encoder walks exactly those fields after the mutex is gone. No production path
// mutates a result map or an artifact slice after publication today (FinishJob
// assigns both wholesale under the lock), so this fixes no live bug; it removes the
// dependence on that staying true, which is not a property a reader can check.
//
// The map's VALUES are not deep-copied: they are the `map[string]any` a job returned
// once, and no code mutates their interiors after FinishJob publishes them. That is
// the limit of the isolation this provides, stated rather than implied.
func (j *Job) snapshot() *Job {
	cp := *j
	if j.Artifacts != nil {
		cp.Artifacts = append([]Artifact(nil), j.Artifacts...)
	}
	if j.Result != nil {
		cp.Result = make(map[string]any, len(j.Result))
		for k, v := range j.Result {
			cp.Result[k] = v
		}
	}
	return &cp
}

// Job returns a single job by ID (nil if unknown).
func (s *Store) Job(id int) *Job {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	for _, j := range s.jobs.rows {
		if j.ID == id {
			return j.snapshot()
		}
	}
	return nil
}

// SetJobTelemetry updates a running job's live throughput/ETA (0s clear it).
func (s *Store) SetJobTelemetry(id int, rateMBps, etaSeconds float64, bytesDone, bytesTotal, filesDone, filesTotal int64) {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	for _, j := range s.jobs.rows {
		if j.ID == id {
			j.RateMBps, j.ETASeconds, j.BytesDone, j.BytesTotal = rateMBps, etaSeconds, bytesDone, bytesTotal
			j.FilesDone, j.FilesTotal = filesDone, filesTotal
		}
	}
}

// SetJobResult attaches a finished job's artifact/summary so the UI can show it
// (the "show the artifact" rule) — clearing any live telemetry.
func (s *Store) SetJobResult(id int, result map[string]any) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	for _, j := range s.jobs.rows {
		if j.ID == id {
			j.Result = result
			j.RateMBps, j.ETASeconds = 0, 0
		}
	}
	return s.saveJobs()
}

func (s *Store) Jobs() []*Job {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	// Hand out copies, not the live rows: a running job's progress/telemetry is
	// updated under this lock while a reader (the /api/jobs encoder) is still
	// walking the result. snapshot copies the Result map and Artifacts slice too, so
	// what escapes this mutex is genuinely the caller's — including the recording
	// state, which every row carries with the status it qualifies.
	out := make([]*Job, 0, len(s.jobs.rows))
	for _, j := range s.jobs.rows {
		out = append(out, j.snapshot())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}
