package main

// mirror_previous.go — a mirror never destroys the older version in place.
//
// When a mirror (full or "back up changes") would put a file where a different
// one already is, the one there is kept as a previous version under
// _obelisk-previous/<run time>/<same path> on the same drive, and only then is
// the new copy published:
//
//  1. the new copy is staged and verified (nothing at its name changes yet);
//  2. the old file gets a second name in the run's folder: a hard link, or, on
//     a filesystem without links (exFAT, FAT32), a copy that is read back and
//     checked before it counts;
//  3. the new copy is published over the name, which leaves the old object in
//     place under its kept name.
//
// So the path always holds one version or the other, never neither, and a crash
// between 2 and 3 leaves the old file in place and also findable in the run's
// folder. The run's folder holds RUN.json, written "incomplete" before the first
// file is kept and "complete" at the end; the catalog's run record (a
// BackupSession) says the same. A kept version never counts as a copy toward
// protection: it is recorded in that run record as an earlier version, with
// where it went. The folder is never removed on its own; removing a run's kept
// versions goes through the delete gate ("mirror-previous-remove").
//
// An identical file already there is left alone and credited. The mirror's own
// drive walks skip _obelisk-previous (skipDockDir), so kept versions are never
// treated as strays or copied back.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// mirrorKeepCrash, when set (tests only), stops a keep at a step as a crash
// would: returning true there returns at once, with no cleanup.
var mirrorKeepCrash func(step string) bool

// mirrorNoLinks, when set (tests only), makes the keep take the no-link path a
// filesystem like exFAT takes.
var mirrorNoLinks bool

var errMirrorCrashed = errors.New("mirror stopped part-way (test crash)")

// keptPathLimit, when above zero (tests only), is the longest kept path.
var keptPathLimit int

// keepDiskFree is the free-space reading for the keep check; a test seam.
var keepDiskFree = diskFree

// planMirrorKeeps plans a run over files at mrels on dest (skipping paths two
// files share), checks the kept paths and the space they need, and reports an
// earlier run that stopped part-way. What is there is Obelisk's own earlier
// copy when the volume's mirror record holds this file at that path with that
// hash.
func (a *App) planMirrorKeeps(dest *DestRoot, collectionID, volumeID int, files []*File, mrels []string, shared map[int]bool) (*keepPlan, error) {
	earlier := map[int]ChunkFileRef{}
	if prior := a.mirrorChunkOn(collectionID, volumeID); prior != nil {
		priorFiles, err := a.Store.ChunkFiles(prior.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range priorFiles {
			earlier[r.FileID] = r
		}
	}
	want, size, byRel := map[string]string{}, map[string]int64{}, map[string]*File{}
	var rels []string
	for i, f := range files {
		if shared[i] {
			continue
		}
		want[mrels[i]], size[mrels[i]], byRel[mrels[i]] = f.Hash, f.SizeBytes, f
		rels = append(rels, mrels[i])
	}
	p, err := planKeeps(dest, rels, want, size, func(mrel, hash string) bool {
		f := byRel[mrel]
		prev, ok := earlier[f.ID]
		return ok && prev.RelPath == f.RelPath && strings.EqualFold(prev.Hash, hash)
	})
	if err != nil {
		return nil, err
	}
	if err := checkKeepSpace(dest, p); err != nil {
		return nil, err
	}
	return p, nil
}

// keepPlan is what a mirror run will keep, worked out before anything is copied.
type keepPlan struct {
	run      string                 // the run's folder name, a UTC time
	existing map[string]string      // mrel -> hash of the file there now
	sizes    map[string]int64       // mrel -> its size
	own      map[string]bool        // mrel -> it is Obelisk's own earlier copy
	same     map[string]bool        // mrel -> identical to what would be copied
	keep     []string               // mrels whose file there will be kept
	kept     []KeptVersion          // kept so far, in order
	started  bool                   // RUN.json written
	session  *BackupSession         // the catalog's run record, once started
	keptBy   map[string]KeptVersion // mrel -> kept
	newBytes int64                  // bytes of new copies to write
	keepByte int64                  // bytes of the versions to keep
}

// planKeeps looks at each destination path the run would write: nothing there,
// the same content (left alone), or something else (kept). want maps each mrel
// to the catalog hash it would get, size to its size, and own reports whether
// what is there is Obelisk's own earlier copy (by recorded hash).
func planKeeps(dest *DestRoot, mrels []string, want map[string]string, size map[string]int64, own func(mrel, hash string) bool) (*keepPlan, error) {
	p := &keepPlan{run: time.Now().UTC().Format("20060102T150405Z"), existing: map[string]string{}, sizes: map[string]int64{},
		own: map[string]bool{}, same: map[string]bool{}, keptBy: map[string]KeptVersion{}}
	var tooLong []string
	for _, mrel := range mrels {
		fi, err := dest.Stat(mrel)
		if err != nil {
			p.newBytes += size[mrel]
			continue
		}
		if !fi.Mode().IsRegular() {
			continue // a folder or link there: the copy itself refuses it
		}
		h, err := dest.HashRel(mrel)
		if err != nil {
			return nil, fmt.Errorf("reading %s on the destination, to keep it before it is replaced: %w", mrel, err)
		}
		p.existing[mrel], p.sizes[mrel] = h, fi.Size()
		if strings.EqualFold(h, want[mrel]) {
			p.same[mrel] = true
			continue
		}
		p.own[mrel] = own(mrel, h)
		p.keep = append(p.keep, mrel)
		p.newBytes += size[mrel]
		p.keepByte += fi.Size()
		if !keptPathFits(dest.Canonical(), p.keptRel(mrel)) {
			tooLong = append(tooLong, mrel)
		}
	}
	if len(tooLong) > 0 {
		shown := tooLong
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return nil, fmt.Errorf("nothing was copied: %d file(s) here would be replaced, and the path to keep the earlier version (under %s) would be too long for this drive (%s). Shorten those folder names, or mirror to a folder nearer the drive's root",
			len(tooLong), mirrorPreviousDir, strings.Join(shown, ", "))
	}
	return p, nil
}

// keptRel is where the version at mrel is kept.
func (p *keepPlan) keptRel(mrel string) string { return path.Join(mirrorPreviousDir, p.run, mrel) }

// keptPathFits reports whether the kept path can be made on this system: each
// name within 255 units and the whole path within the system's limit.
func keptPathFits(root, rel string) bool {
	full := filepath.Join(root, filepath.FromSlash(rel))
	limit := 4095
	units := func(s string) int { return len(s) }
	if runtime.GOOS == "windows" {
		limit = 32000 // long paths are opened with the \\?\ form
		units = func(s string) int { return len(utf16.Encode([]rune(s))) }
	}
	if keptPathLimit > 0 {
		limit = keptPathLimit
	}
	if units(full) > limit {
		return false
	}
	for _, part := range strings.Split(rel, "/") {
		if units(part) > 255 {
			return false
		}
	}
	return true
}

// checkKeepSpace refuses a run whose new copies and kept versions will not fit.
// A kept version made by a hard link takes no new space; one copied (on a
// drive without links) takes its size again, so the check counts it.
func checkKeepSpace(dest *DestRoot, p *keepPlan) error {
	free, err := keepDiskFree(dest.Canonical())
	if err != nil {
		return nil // best effort, as the run's own space check
	}
	need := p.newBytes + p.keepByte
	if uint64(need) > uint64(free) {
		return fmt.Errorf("destination too small: the new copies need %.1f GB and keeping the earlier version of %d file(s) it would replace needs up to %.1f GB more, but %.1f GB is free. Nothing was copied. Free space, or remove earlier kept versions in %s",
			float64(p.newBytes)/1e9, len(p.keep), float64(p.keepByte)/1e9, float64(free)/1e9, mirrorPreviousDir)
	}
	return nil
}

// runMarker is RUN.json in a run's folder.
type runMarker struct {
	Status   string        `json:"status"` // incomplete | complete
	Started  time.Time     `json:"started"`
	Finished *time.Time    `json:"finished,omitempty"`
	Volume   string        `json:"volume"`
	Kept     []KeptVersion `json:"kept"`
	Note     string        `json:"note"`
}

func (p *keepPlan) writeMarker(dest *DestRoot, vol *Volume, status string) error {
	m := runMarker{Status: status, Started: time.Now().UTC(), Volume: vol.Label, Kept: p.kept,
		Note: "Earlier versions a mirror kept instead of replacing them, each at its old path under this folder. They are not counted as copies. Obelisk never removes them on its own."}
	if p.session != nil {
		m.Started = p.session.At
	}
	if status == "complete" {
		now := time.Now().UTC()
		m.Finished = &now
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return dest.WriteFileAtomic(path.Join(mirrorPreviousDir, p.run, "RUN.json"), b, 0o644)
}

// start marks the run incomplete, on the drive and in the catalog, before the
// first version is kept.
func (a *App) startKeeping(dest *DestRoot, vol *Volume, collectionID int, mode string, p *keepPlan) error {
	if p.started {
		return nil
	}
	p.session = a.Store.AddBackupSession(&BackupSession{CollectionID: collectionID, VolumeID: vol.ID, VolumeLabel: vol.Label,
		Base: "kept", Mode: mode, Dest: dest.Name(), KeptRun: path.Join(mirrorPreviousDir, p.run), Status: "incomplete",
		Name: fmt.Sprintf("Earlier versions kept on %s — %s (in progress)", vol.Label, p.run)})
	if err := p.writeMarker(dest, vol, "incomplete"); err != nil {
		return fmt.Errorf("marking the run's kept versions on the drive: %w", err)
	}
	p.started = true
	return nil
}

// finishKeeping marks the run complete, with every version it kept.
func (a *App) finishKeeping(dest *DestRoot, vol *Volume, p *keepPlan) error {
	if !p.started {
		return nil
	}
	err := p.writeMarker(dest, vol, "complete")
	var bytes int64
	for _, k := range p.kept {
		bytes += k.Size
	}
	p.session.Status, p.session.KeptVersions = "complete", p.kept
	p.session.Files, p.session.Bytes = len(p.kept), bytes
	p.session.Name = fmt.Sprintf("Earlier versions kept on %s — %d file(s), %s, %s", vol.Label, len(p.kept), humanBytes(bytes), p.run)
	return errors.Join(err, a.Store.UpdateBackupSession(p.session))
}

// keepThenPublish keeps what is at mrel (when the plan keeps it), then publishes
// the staged, verified copy tmpRel over mrel.
func (p *keepPlan) keepThenPublish(dest *DestRoot, tmpRel, mrel string, fileID int) error {
	if _, keep := p.existing[mrel]; !keep || p.same[mrel] {
		return dest.PublishNew(tmpRel, mrel)
	}
	kept := p.keptRel(mrel)
	if err := dest.MkdirAll(path.Dir(kept)); err != nil {
		return fmt.Errorf("making the folder to keep the earlier %s: %w", mrel, err)
	}
	if err := keepAside(dest, mrel, kept, p.existing[mrel]); err != nil {
		return fmt.Errorf("keeping the earlier %s: %w; it was not replaced", mrel, err)
	}
	if mirrorKeepCrash != nil && mirrorKeepCrash("kept") {
		return errMirrorCrashed
	}
	// The name moves to the new copy; the earlier file stays under its kept name.
	if err := dest.Publish(tmpRel, mrel); err != nil {
		return err
	}
	k := KeptVersion{FileID: fileID, RelPath: mrel, KeptRel: kept, Hash: p.existing[mrel], Size: p.sizes[mrel], OwnCopy: p.own[mrel]}
	p.kept = append(p.kept, k)
	p.keptBy[mrel] = k
	if mirrorKeepCrash != nil && mirrorKeepCrash("published") {
		return errMirrorCrashed
	}
	return nil
}

// keepAside gives the file at rel a second name, kept, without touching rel: a
// hard link where the drive has them, otherwise a copy checked against want.
func keepAside(dest *DestRoot, rel, kept, want string) error {
	r, err := destRel(rel)
	if err != nil {
		return err
	}
	k, err := destRel(kept)
	if err != nil {
		return err
	}
	if !mirrorNoLinks {
		err := dest.root.Link(filepath.FromSlash(r), filepath.FromSlash(k))
		if err == nil {
			return nil
		}
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s is already there", kept)
		}
	}
	// No links here (exFAT, FAT32): a copy, published only once it reads back.
	in, err := dest.Open(r)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, out, err := dest.stagingFor(k)
	if err != nil {
		return err
	}
	h, _, err := mirrorCopyStream(in, out, &throttler{}, func(int64) {})
	if err == nil && !strings.EqualFold(h, want) {
		err = fmt.Errorf("the earlier file changed while it was being kept")
	}
	if err == nil {
		err = dest.VerifyStage(tmp, h)
	}
	if err == nil {
		err = dest.PublishNew(tmp, k)
	}
	if err != nil {
		_ = dest.Discard(tmp)
	}
	return err
}

// KeptSummary is what a mirror run kept instead of replacing, for its report.
type KeptSummary struct {
	// KeptRun is the folder on the destination holding this run's kept versions.
	KeptRun string `json:"kept_run,omitempty"`
	// KeptOwn are Obelisk's own earlier copies, kept as previous versions.
	KeptOwn []string `json:"kept_own,omitempty"`
	// KeptOther are files that were at those paths and were not Obelisk's own
	// earlier copies: kept the same way, and reported apart.
	KeptOther []string `json:"kept_other,omitempty"`
	// KeptBytes is the space the kept versions use until they are removed.
	KeptBytes int64 `json:"kept_bytes,omitempty"`
	// Identical are files already there with the same content: left alone.
	Identical int `json:"identical,omitempty"`
}

// summary fills a run report's KeptSummary.
func (p *keepPlan) summary(dest *DestRoot) KeptSummary {
	own, other, bytes := p.keptReport()
	s := KeptSummary{KeptBytes: bytes, Identical: len(p.same)}
	for _, k := range own {
		s.KeptOwn = append(s.KeptOwn, k.RelPath)
	}
	for _, k := range other {
		s.KeptOther = append(s.KeptOther, k.RelPath)
	}
	if len(p.kept) > 0 {
		s.KeptRun = filepath.Join(dest.Canonical(), mirrorPreviousDir, p.run)
	}
	return s
}

// copyVerifyKeeping copies srcPath to rel beneath dest with the full
// copy-then-verify discipline (copyVerifyToRootAs), and publishes it by the
// plan: over nothing, or after keeping what is there.
func copyVerifyKeeping(dest *DestRoot, p *keepPlan, srcPath, rel string, fileID int, th *throttler, onBytes func(int64)) (string, int64, error) {
	return copyVerifyToRootAs(dest, srcPath, rel, "", nil, th, onBytes, func(tmpRel, v string) error {
		return p.keepThenPublish(dest, tmpRel, v, fileID)
	})
}

// keptReport splits what a run kept: Obelisk's own earlier copies, and other
// files that were at those paths.
func (p *keepPlan) keptReport() (own, other []KeptVersion, bytes int64) {
	for _, k := range p.kept {
		bytes += k.Size
		if k.OwnCopy {
			own = append(own, k)
		} else {
			other = append(other, k)
		}
	}
	return
}
