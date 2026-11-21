package main

// cardcheck.go — "How many backup copies does this card's content have?" Plug in an
// SD/CF/USB card, and Obelisk reads it (NEVER writes, NEVER registers it as a source)
// and looks up every file BY CONTENT. For each file it reports how many separate,
// verified backup copies exist, where each one lives, and when each was last checked.
//
// It never gives a verdict on formatting, erasing or reusing a card. That is a permanent rule:
// the tool reports copies and dates, and the person decides.
//
// What counts as a copy is deliberately narrow. A copy counts only when it is a
// package copy or an adopted backup copy (not a source-scan record, an earlier-version
// record, or a drive snapshot on its own), is not superseded, last verified OK, and
// sits on a different physical device from the card. A file the catalog knows only
// from a scan or a snapshot is reported as recorded, with no backup copy.
//
// Read-only by construction: it only os.Opens files to hash them, mutates nothing in
// the catalog, and adds no source root. Copies are matched by SHA-256, the on-media
// hash; BLAKE3 is used only to recognise a file the catalog has recorded.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxCardProblems caps how many individual unreadable paths the result carries back
// for display; the Skipped / WalkErrors counts below are always exact.
const maxCardProblems = 100

// maxCardFileRows caps each per-file list (files with copies, files without) the
// result carries back; the counts are always exact.
const maxCardFileRows = 500

// cardDateTime is the date and time format the card check reports in, in local time.
const cardDateTime = "01/02/2006 15:04:05"

// cardDeviceIdentity resolves the physical device behind the card. A seam so tests
// can name the card's device without real hardware.
var cardDeviceIdentity = resolveDeviceIdentity

// cardOnlineVolumes reports which registered volumes are connected right now. A seam
// for tests; the live probe shells a tool per mount.
var cardOnlineVolumes = func(a *App) map[int]bool { return a.onlineVolumeIDsCached() }

// CardCopy is one counted backup copy of a card file's content.
type CardCopy struct {
	Where          string    `json:"where"`   // volume label (or labels, for a spanned copy)
	Package        string    `json:"package"` // package name
	Kind           string    `json:"kind"`    // "package" | "adopted"
	LastVerifiedAt time.Time `json:"last_verified_at"`
	// Offline: the volume is not connected now, so this copy was not rechecked; its
	// date is from the last time it was.
	Offline bool `json:"offline,omitempty"`
	vols    []int
}

// CardFileResult is one file on the card and the backup copies its content has.
type CardFileResult struct {
	Rel    string     `json:"rel"`
	Size   int64      `json:"size"`
	Hash   string     `json:"hash,omitempty"`
	Role   string     `json:"role,omitempty"`
	Copies []CardCopy `json:"copies,omitempty"`
	// RecordedAt / RecordedIn: for a file with no backup copy that the catalog has
	// still seen (a source scan, an earlier version, a drive snapshot), when and where.
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	RecordedIn string     `json:"recorded_in,omitempty"`
	// NotCounted lists copies that were found but do not count, and why.
	NotCounted []string `json:"not_counted,omitempty"`
}

// CardExtRow is a per-type tally of the card, split into no-copy vs total.
type CardExtRow struct {
	Ext         string `json:"ext"`
	Total       int    `json:"total"`
	NoCopy      int    `json:"no_copy"`
	NoCopyBytes int64  `json:"no_copy_bytes"`
}

// CardExcluded is a hidden or system file or folder the check did not read.
type CardExcluded struct {
	Path   string `json:"path"`
	Folder bool   `json:"folder,omitempty"`
}

// CardCheckResult is the whole report. It carries counts and dates, never a verdict.
type CardCheckResult struct {
	MountPath     string `json:"mount_path"`
	TotalFiles    int    `json:"total_files"`
	TotalBytes    int64  `json:"total_bytes"`
	BackedUpFiles int    `json:"backed_up_files"` // files with at least one counted copy
	BackedUpBytes int64  `json:"backed_up_bytes"`
	NoCopyFiles   int    `json:"no_copy_files"` // files with no counted copy
	NoCopyBytes   int64  `json:"no_copy_bytes"`
	RecordedOnly  int    `json:"recorded_only"` // of NoCopyFiles: known from a record, with no backup copy
	// FewestCopies is the lowest copy count among files that have a copy.
	FewestCopies int `json:"fewest_copies"`
	// AsOf is the oldest last-verified time among every copy counted: the report is
	// only as fresh as its stalest proof. Nil when no copy was counted.
	AsOf *time.Time `json:"as_of,omitempty"`

	Skipped    int `json:"skipped"`     // files listed but not hashed (I/O error)
	WalkErrors int `json:"walk_errors"` // folders/entries that could not be listed at all
	// Excluded counts hidden (dot-prefixed) and system files and folders that were not
	// read. A folder counts once; what is inside it was never seen.
	Excluded      int            `json:"excluded"`
	ExcludedItems []CardExcluded `json:"excluded_items,omitempty"` // capped
	// Incomplete is true when anything on the card was not read: an unreadable file,
	// an unlistable folder, or an excluded item.
	Incomplete bool `json:"incomplete"`

	Problems          []ScanProblem    `json:"problems,omitempty"` // what could not be read, capped (kind walk|stat|hash|unsupported)
	Where             []string         `json:"where,omitempty"`    // distinct places counted copies live
	NoCopy            []CardFileResult `json:"no_copy,omitempty"`  // files with no backup copy (capped)
	NoCopyTruncated   int              `json:"no_copy_truncated,omitempty"`
	BackedUp          []CardFileResult `json:"backed_up,omitempty"` // files with copies, with each copy (capped)
	BackedUpTruncated int              `json:"backed_up_truncated,omitempty"`
	ByExt             []CardExtRow     `json:"by_ext,omitempty"`
	Summary           string           `json:"summary"`
}

// cardCopyIndex is what the catalog can say about content: the backup copies that
// count, the ones that do not (and why), and the records that only show it was seen.
type cardCopyIndex struct {
	copies   map[string][]*cardPkg // SHA-256 → packages holding it
	recorded map[string]cardRecord // SHA-256 or BLAKE3 → earliest record of it
}

// cardPkg is one package's copies, judged once against the card.
type cardPkg struct {
	counted    map[string]CardCopy // keyed by physical device, so one device counts once
	notCounted []string
}

type cardRecord struct {
	at time.Time
	in string
}

// cardDevice is what is known of the card's physical device.
type cardDevice struct {
	mount    string
	serial   string
	volumeID int // the registered volume the card is, if any
}

// placeOf reports whether v is on the card's device (true) or a different one
// (false), and whether that could be told at all. Two partitions of one disk share
// a serial, so they are one device.
func (d cardDevice) placeOf(v *Volume, copyPath string) (onCard, known bool) {
	if d.volumeID != 0 && v.ID == d.volumeID {
		return true, true
	}
	vs := strings.TrimSpace(v.Serial)
	if d.serial != "" && vs != "" {
		return strings.EqualFold(vs, d.serial), true
	}
	// Without both serials the device cannot be told apart. A copy recorded under the
	// card's own mount is treated as on the card; anything else is unknown.
	if pathAtOrUnder(copyPath, d.mount) {
		return true, true
	}
	return false, false
}

// pathAtOrUnder reports whether p is root or inside it, at path-segment boundaries.
// It handles a drive root such as E: as well as a folder.
func pathAtOrUnder(p, root string) bool {
	np, nr := strings.TrimSuffix(normPath(p), "/"), strings.TrimSuffix(normPath(root), "/")
	if np == "" || nr == "" {
		return false
	}
	return np == nr || strings.HasPrefix(np, nr+"/")
}

func (a *App) cardCopies(dev cardDevice) (*cardCopyIndex, error) {
	return retrySnapshotRead(func() (*cardCopyIndex, error) { return a.cardCopiesOnce(dev) })
}

func (a *App) cardCopiesOnce(dev cardDevice) (*cardCopyIndex, error) {
	idx := &cardCopyIndex{copies: map[string][]*cardPkg{}, recorded: map[string]cardRecord{}}
	vols := map[int]*Volume{}
	for _, v := range a.Store.Volumes() {
		vols[v.ID] = v
	}
	label := func(v *Volume) string { return nonEmpty(v.Label, fmt.Sprintf("vol#%d", v.ID)) }
	devKey := func(v *Volume) string {
		if s := strings.TrimSpace(v.Serial); s != "" {
			return "dev:" + strings.ToUpper(s) // two partitions of one disk are one device
		}
		return "vol:" + strconv.Itoa(v.ID)
	}

	for _, c := range a.Store.Chunks(0) {
		if c.ListingUnknown {
			continue // contents never listed: nothing can be matched to it
		}
		kind := "package"
		if c.Adopted || c.Mirror {
			kind = "adopted"
		}
		pkg := &cardPkg{counted: map[string]CardCopy{}}
		judge := func(v *Volume, cpPath string, verifyOK *bool, verifiedAt *time.Time, checkOK *bool, checkAt *time.Time) (CardCopy, string) {
			if v == nil {
				return CardCopy{}, "the drive this copy was on is no longer registered"
			}
			name := label(v)
			onCard, known := dev.placeOf(v, cpPath)
			switch {
			case onCard:
				return CardCopy{}, name + ": on this card"
			case !known:
				return CardCopy{}, name + ": could not tell its drive apart from this card"
			case verifyOK == nil:
				return CardCopy{}, name + ": never verified"
			case !*verifyOK:
				return CardCopy{}, name + ": last verification failed"
			case verifiedAt == nil:
				return CardCopy{}, name + ": no verification date"
			case checkOK != nil && !*checkOK && checkAt != nil && checkAt.After(*verifiedAt):
				return CardCopy{}, name + ": a later check failed"
			}
			return CardCopy{Where: name, Package: c.Name, Kind: kind, LastVerifiedAt: *verifiedAt, vols: []int{v.ID}}, ""
		}
		if c.Spanned {
			// One logical copy spread over its segment media: it counts only when every
			// segment is verified and none is on the card.
			var names, keys []string
			var ids []int
			reason := ""
			done := len(c.Segments) > 0
			for _, sg := range c.Segments {
				if sg.Status != "VERIFIED" {
					done = false
				}
				v := vols[sg.VolumeID]
				if v == nil {
					reason = "a segment's drive is no longer registered"
					continue
				}
				if onCard, known := dev.placeOf(v, sg.Dest); onCard {
					reason = label(v) + ": a segment is on this card"
				} else if !known && reason == "" {
					reason = label(v) + ": could not tell its drive apart from this card"
				}
				names = append(names, label(v))
				keys = append(keys, devKey(v))
				ids = append(ids, v.ID)
			}
			switch {
			case reason != "":
				pkg.notCounted = append(pkg.notCounted, reason)
			case !done || c.VerifiedAt == nil:
				pkg.notCounted = append(pkg.notCounted, c.Name+": not every segment is verified")
			default:
				sort.Strings(keys)
				pkg.counted["span:"+strings.Join(keys, "+")] = CardCopy{Where: strings.Join(names, " + "),
					Package: c.Name, Kind: kind, LastVerifiedAt: *c.VerifiedAt, vols: ids}
			}
		} else {
			for _, cp := range c.Copies {
				if cp.Superseded {
					continue // history only
				}
				v := vols[cp.VolumeID]
				cc, why := judge(v, cp.Path, cp.VerifyOK, cp.LastVerifiedAt, cp.LastCheckOK, cp.LastCheckAt)
				if why != "" {
					pkg.notCounted = append(pkg.notCounted, why)
					continue
				}
				k := devKey(v)
				if prev, ok := pkg.counted[k]; !ok || cc.LastVerifiedAt.After(prev.LastVerifiedAt) {
					pkg.counted[k] = cc
				}
			}
		}
		if len(pkg.counted) == 0 && len(pkg.notCounted) == 0 {
			continue // no copies at all: it holds nothing for anyone
		}
		listed, err := a.Store.ChunkFiles(c.ID)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, ref := range listed {
			if ref.Hash == "" || seen[ref.Hash] {
				continue
			}
			seen[ref.Hash] = true
			idx.copies[ref.Hash] = append(idx.copies[ref.Hash], pkg)
		}
	}

	record := func(hash string, at time.Time, in string) {
		if hash == "" {
			return
		}
		if cur, ok := idx.recorded[hash]; ok && (at.IsZero() || (!cur.at.IsZero() && !at.Before(cur.at))) {
			return
		}
		idx.recorded[hash] = cardRecord{at: at, in: in}
	}
	for _, coll := range a.Store.Collections() {
		in := "Archive: " + coll.Name
		for f, err := range a.Store.FilesOf(coll.ID) {
			if err != nil {
				return nil, err
			}
			record(f.Hash, f.FirstSeen, in)
			record(f.Blake3, f.FirstSeen, in)
			for _, v := range f.Versions {
				record(v.Hash, v.FirstSeen, in+" (earlier version)")
			}
		}
	}
	for _, snap := range a.Store.VolumeSnapshots() {
		in := "Drive: " + nonEmpty(snap.Label, fmt.Sprintf("vol#%d", snap.VolumeID))
		for sf, err := range a.Store.SnapshotFiles(snap.VolumeID) {
			if err != nil {
				return nil, err
			}
			record(sf.Hash, snap.CapturedAt, in)
			record(sf.Blake3, snap.CapturedAt, in)
		}
	}
	return idx, nil
}

// cardSkipDir names the tool's own sidecar folders, which a card walk passes over
// without counting: they hold the tool's records, not anyone's files.
func cardSkipDir(name string) bool {
	switch name {
	case dockSidecarDir, sealSidecarDir, dockSidecarDirLegacy, sealSidecarDirLegacy:
		return true
	}
	return false
}

// cardExcluded names hidden (dot-prefixed) and system entries the check does not
// read. They are counted and listed, and they make the result incomplete.
func cardExcluded(name string) bool {
	return strings.HasPrefix(name, ".") || skipBrowseDir(name)
}

// CardCheck reads a mounted card/drive and reports, per file, how many backup copies
// its content has, where, and when each was last verified. Read-only.
func (a *App) CardCheck(mountPath string, progress func(float64, string)) (*CardCheckResult, error) {
	info, err := os.Stat(mountPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a readable folder/drive: %s", mountPath)
	}
	progress(0.01, "reading inventory")
	dev := cardDevice{mount: mountPath}
	if id, err := cardDeviceIdentity(mountPath); err == nil {
		dev.serial = strings.TrimSpace(id.Serial)
		if v := a.Store.VolumeByIdentity(id.Serial, id.Partition); v != nil {
			dev.volumeID = v.ID
		}
	}
	idx, err := a.cardCopies(dev)
	if err != nil {
		return nil, err
	}

	res := &CardCheckResult{MountPath: mountPath}
	var mu sync.Mutex
	// Whatever the card will not give us is COUNTED, never dropped. A folder we cannot
	// list and a file we cannot hash are independent losses — a file inside an unlistable
	// folder never reaches `paths`, so it can never show up in Skipped — so each gets its
	// own tally, and either one makes the result incomplete.
	addProblem := func(p, kind, msg string) {
		mu.Lock()
		defer mu.Unlock()
		if kind == "walk" {
			res.WalkErrors++
		}
		if len(res.Problems) < maxCardProblems {
			res.Problems = append(res.Problems, ScanProblem{Path: p, Kind: kind, Err: msg})
		}
	}
	exclude := func(p string, folder bool) {
		res.Excluded++
		if len(res.ExcludedItems) < maxCardProblems {
			rel, e := filepath.Rel(mountPath, p)
			if e != nil {
				rel = filepath.Base(p)
			}
			res.ExcludedItems = append(res.ExcludedItems, CardExcluded{Path: filepath.ToSlash(rel), Folder: folder})
		}
	}

	progress(0.03, "listing files on the card")
	var paths []string
	if err := filepath.WalkDir(mountPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			addProblem(p, "walk", err.Error()) // could not list this entry — keep going, but remember
			return nil
		}
		if p == mountPath {
			return nil
		}
		if d.IsDir() {
			if cardSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			if cardExcluded(d.Name()) {
				exclude(p, true)
				return filepath.SkipDir
			}
			return nil
		}
		if cardExcluded(d.Name()) {
			exclude(p, false)
			return nil
		}
		paths = append(paths, p)
		return nil
	}); err != nil {
		addProblem(mountPath, "walk", err.Error())
	}

	reg := a.formatRegistry()
	extAgg := map[string]*CardExtRow{}
	whereSet := map[string]bool{}
	total := len(paths)
	var online map[int]bool
	if total > 0 && len(idx.copies) > 0 {
		online = cardOnlineVolumes(a)
	}

	if total > 0 {
		parallelHash(paths, func(done int) {
			progress(0.05+0.9*float64(done)/float64(total), progStats(0, 0, int64(done), int64(total), "hashing card files"))
		}, func(p, sha, b3 string, size int64, _ time.Time) {
			rel, e := filepath.Rel(mountPath, p)
			if e != nil {
				rel = filepath.Base(p)
			}
			rel = filepath.ToSlash(rel)
			role, _ := classifyRole(reg, rel)
			fr := CardFileResult{Rel: rel, Size: size, Hash: sha, Role: role}
			counted := map[string]CardCopy{}
			for _, pkg := range idx.copies[sha] {
				for k, cc := range pkg.counted {
					if prev, ok := counted[k]; !ok || cc.LastVerifiedAt.After(prev.LastVerifiedAt) {
						counted[k] = cc
					}
				}
				fr.NotCounted = append(fr.NotCounted, pkg.notCounted...)
			}
			for _, cc := range counted {
				cc.Offline = !cardCopyOnline(cc.vols, online)
				fr.Copies = append(fr.Copies, cc)
			}
			sort.Slice(fr.Copies, func(i, j int) bool { return fr.Copies[i].Where < fr.Copies[j].Where })
			fr.NotCounted = dedupStrings(fr.NotCounted)
			if len(fr.Copies) == 0 {
				rec, ok := idx.recorded[sha]
				if !ok && b3 != "" {
					rec, ok = idx.recorded[b3]
				}
				if ok {
					fr.RecordedIn = rec.in
					if !rec.at.IsZero() {
						at := rec.at
						fr.RecordedAt = &at
					}
				}
			}

			mu.Lock()
			defer mu.Unlock()
			res.TotalFiles++
			res.TotalBytes += size
			ext := strings.ToLower(path.Ext(rel))
			er := extAgg[ext]
			if er == nil {
				er = &CardExtRow{Ext: ext}
				extAgg[ext] = er
			}
			er.Total++
			if n := len(fr.Copies); n > 0 {
				res.BackedUpFiles++
				res.BackedUpBytes += size
				if res.FewestCopies == 0 || n < res.FewestCopies {
					res.FewestCopies = n
				}
				for _, cc := range fr.Copies {
					whereSet[cc.Where] = true
					if res.AsOf == nil || cc.LastVerifiedAt.Before(*res.AsOf) {
						at := cc.LastVerifiedAt
						res.AsOf = &at
					}
				}
				if len(res.BackedUp) < maxCardFileRows {
					res.BackedUp = append(res.BackedUp, fr)
				} else {
					res.BackedUpTruncated++
				}
			} else {
				res.NoCopyFiles++
				res.NoCopyBytes += size
				if fr.RecordedIn != "" {
					res.RecordedOnly++
				}
				er.NoCopy++
				er.NoCopyBytes += size
				if len(res.NoCopy) < maxCardFileRows {
					res.NoCopy = append(res.NoCopy, fr)
				} else {
					res.NoCopyTruncated++
				}
			}
		}, addProblem)
	}

	res.Skipped = total - res.TotalFiles // listed but never hashed (stat/read errors)
	res.Incomplete = res.Skipped > 0 || res.WalkErrors > 0 || res.Excluded > 0

	for l := range whereSet {
		res.Where = append(res.Where, l)
	}
	sort.Strings(res.Where)
	if len(res.Where) > 10 {
		res.Where = res.Where[:10]
	}
	for _, er := range extAgg {
		res.ByExt = append(res.ByExt, *er)
	}
	sort.Slice(res.ByExt, func(i, j int) bool {
		if res.ByExt[i].NoCopy != res.ByExt[j].NoCopy {
			return res.ByExt[i].NoCopy > res.ByExt[j].NoCopy // most uncopied types first
		}
		return res.ByExt[i].Ext < res.ByExt[j].Ext
	})
	sort.Slice(res.NoCopy, func(i, j int) bool { return res.NoCopy[i].Rel < res.NoCopy[j].Rel })
	sort.Slice(res.BackedUp, func(i, j int) bool { return res.BackedUp[i].Rel < res.BackedUp[j].Rel })

	res.Summary = cardCheckSummary(res)
	progress(1.0, res.Summary)
	return res, nil
}

// cardCopyOnline reports whether every volume a copy is on is connected now.
func cardCopyOnline(vols []int, online map[int]bool) bool {
	if len(vols) == 0 {
		return false
	}
	for _, id := range vols {
		if !online[id] {
			return false
		}
	}
	return true
}

func dedupStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// cardCheckSummary is the report in one line: how many files have no backup copy
// (first, when any), how many are backed up and as of when, the fewest copies any
// file has, and what was not read. It never says whether to format.
func cardCheckSummary(res *CardCheckResult) string {
	var parts []string
	tot := commaInt(res.TotalFiles)
	if res.NoCopyFiles > 0 {
		parts = append(parts, fmt.Sprintf("%s of %s files have no backup copy.", commaInt(res.NoCopyFiles), tot))
	}
	if res.BackedUpFiles > 0 && res.AsOf != nil {
		parts = append(parts, fmt.Sprintf("%s of %s files backed up as of %s. Fewest copies: %d.",
			commaInt(res.BackedUpFiles), tot, res.AsOf.Local().Format(cardDateTime), res.FewestCopies))
	}
	if res.TotalFiles == 0 && !res.Incomplete {
		parts = append(parts, "No files found.")
	}
	if res.Incomplete {
		var gaps []string
		if res.Skipped > 0 {
			gaps = append(gaps, fmt.Sprintf("%s file(s) could not be read", commaInt(res.Skipped)))
		}
		if res.WalkErrors > 0 {
			gaps = append(gaps, fmt.Sprintf("%s folder(s) could not be listed", commaInt(res.WalkErrors)))
		}
		if res.Excluded > 0 {
			gaps = append(gaps, fmt.Sprintf("%s hidden or system item(s) were not checked", commaInt(res.Excluded)))
		}
		parts = append(parts, "Incomplete: "+strings.Join(gaps, ", ")+".")
	}
	return strings.Join(parts, " ")
}
