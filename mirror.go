package main

// mirror.go — native MIRROR backup: copy an Archive's files to a Volume as PLAIN
// FILES, preserving the source tree. Mirrors are the un-encrypted, browse-anywhere
// complement to packages: packages are sealed/verified/encrypted units for tape &
// optical; mirrors are live, directly-browsable copies for spinning drives.
//
// Each file is copied with v1's copy-then-verify discipline: staged to
// a uniquely created object, its bytes hashed as they are written, read back
// through the retained file and compared to the source, then renamed into place —
// so a partially-written or corrupted file never appears under its real name. The
// result is recorded exactly like an adopted mirror (Prompt 23): a verified
// file-level Copy on the volume, via the same Chunk.Mirror record, so drift and
// coverage count it as a real copy with no special-casing. The volume inventory
// sidecar is refreshed afterward.
//
// Multiple mirror jobs run concurrently — one job per volume — which v1 proved is
// the right shape for feeding several spinning drives at once. Writer-side speed
// honors throttle_mbps (thermal control), paced across the WHOLE job.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const mirrorTmpSuffix = ".mnemo_tmp"

// throttler paces cumulative written bytes against wall-clock to hold a steady
// MB/s cap across an entire mirror job (not per-file), the same self-correcting
// scheme the ring-buffer writer uses. bps <= 0 disables pacing.
type throttler struct {
	bps     float64
	start   time.Time
	written int64
}

func (t *throttler) pace(n int) {
	if t == nil || t.bps <= 0 {
		return
	}
	t.written += int64(n)
	target := time.Duration(float64(t.written) / t.bps * float64(time.Second))
	if el := time.Since(t.start); target > el {
		time.Sleep(target - el)
	}
}

// errCopySource marks a failure reading the SOURCE, as opposed to the destination.
// Mirror runs count the two differently: an unreadable source is "skipped", a
// destination that would not take the bytes is "failed".
var errCopySource = errors.New("source read failed")

// mirrorCopyFile streams src -> dst, hashing the bytes on the way through
// (so the returned hash proves what left the source), pacing the writer, and
// fsyncing before return so a subsequent read-back sees the medium, not cache.
//
// The source is opened before dst is created: os.Create truncates an existing dst,
// including through a hard link, so a source that cannot be read must not cost the
// destination anything.
func mirrorCopyFile(src, dst string, th *throttler, onBytes func(int64)) (string, int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", errCopySource, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return "", 0, err
	}
	hash, n, err := mirrorCopyStream(in, out, th, onBytes)
	return hash, n, errors.Join(err, out.Close())
}

// mirrorCopyStream streams an open source into an open destination, hashing the
// bytes on the way through, pacing the writer, and syncing before it returns. The
// caller owns and closes both files, on failure too.
func mirrorCopyStream(in, out *os.File, th *throttler, onBytes func(int64)) (string, int64, error) {
	h := sha256.New()
	buf := make([]byte, 1<<20)
	var written int64
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return "", written, werr
			}
			h.Write(buf[:n])
			written += int64(n)
			if onBytes != nil {
				onBytes(int64(n))
			}
			th.pace(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "", written, fmt.Errorf("%w: %w", errCopySource, rerr)
		}
	}
	if err := out.Sync(); err != nil {
		return "", written, err
	}
	return hex.EncodeToString(h.Sum(nil)), written, nil
}

// MirrorResult is the per-volume outcome (also the API/job return shape).
type MirrorResult struct {
	VolumeID int      `json:"volume_id"`
	Volume   string   `json:"volume"`
	Dest     string   `json:"dest"`
	Mirrored int      `json:"mirrored"`
	Bytes    int64    `json:"bytes"`
	Changed  int      `json:"changed"` // copied, but source content differs from the catalog hash (source drifted)
	Failed   int      `json:"failed"`  // copy or verify failed; not finalized
	Skipped  int      `json:"skipped"` // source unreadable/missing
	Sidecar  string   `json:"sidecar,omitempty"`
	Coverage Coverage `json:"coverage"`
	// DestinationNotes lists warnings the destination was accepted with and checks
	// that could not run on its filesystem (for example on exFAT or FAT32).
	DestinationNotes []string `json:"destination_notes,omitempty"`
}

// MirrorToVolume copies the selected folders of an archive to destDir as plain
// files (copy-then-verify each), records a verified mirror Copy on the volume,
// and refreshes the volume inventory sidecar. One call == one volume; run several
// concurrently for multi-volume mirroring.
func (a *App) MirrorToVolume(collectionID int, folderIDs []int, destDir string, volumeID int, throttleMbps float64, scopePrefix string, progress func(float64, string)) (out *MirrorResult, err error) {
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return nil, cfgErr
	}
	coll := a.Store.Collection(collectionID)
	if coll == nil {
		return nil, fmt.Errorf("archive %d not found", collectionID)
	}
	if strings.TrimSpace(destDir) == "" {
		return nil, fmt.Errorf("dest_dir (the mirror target on the volume) required")
	}
	// SOURCE-SAFETY. A mirror WRITES files and a sidecar
	// into destDir. The destination is authorized ONCE, on the identity of the
	// object actually opened, and every write below goes through the returned
	// handle — payload, read-back, sidecar and failure cleanup alike. Nothing here
	// reopens an absolute pathname after this point.
	//
	// This replaces the entry-time AssertOutsideSources string test, which could
	// not see an alias of a source, a destination that CONTAINS a source, a nested
	// link below a correct root, or a root substituted after the check.
	//
	// The authorization also creates the destination when it does not exist yet,
	// root-relative through an authorized ancestor — so no unrestricted MkdirAll
	// runs before the decision.
	dest, err := a.Store.AuthorizeDestination(destDir)
	if err != nil {
		return nil, err
	}
	defer dest.Close()
	vol := a.Store.Volume(volumeID)
	if vol == nil {
		return nil, fmt.Errorf("volume %d not found", volumeID)
	}
	if throttleMbps <= 0 {
		throttleMbps = cfg.ThrottleMbps
	}
	// Batch catalog writes across the mirror job (idempotent copy-then-verify).
	a.Store.BeginBatch()
	defer endBatchInto(a.Store, &err)

	// Resolve the source files (optionally limited to chosen folders) and the
	// folder roots they hang off.
	folderPath := map[int]string{}
	for _, f := range a.Store.FoldersOf(collectionID) {
		folderPath[f.ID] = f.Path
	}
	want := map[int]bool{}
	for _, id := range folderIDs {
		want[id] = true
	}
	var files []*File
	usedFolders := map[int]bool{}
	var totalBytes int64
	for _, f := range a.Store.FilesOf(collectionID) {
		if len(want) > 0 && !want[f.FolderID] {
			continue
		}
		root := folderPath[f.FolderID]
		if root == "" {
			continue // orphan file with no source folder — cannot locate on disk
		}
		if scopePrefix != "" {
			full := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(f.RelPath)))
			if !underScopePrefix(full, scopePrefix) {
				continue // outside the selected folder-tree scope
			}
		}
		files = append(files, f)
		usedFolders[f.FolderID] = true
		totalBytes += f.SizeBytes
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no cataloged files to mirror (scan a folder into the archive first, or widen the folder selection)")
	}
	if err := dest.CheckFileSizes(fileSizes(files)); err != nil {
		return nil, err
	}

	// Preserve the source tree. With a single source folder, files land at their
	// bare rel path; with several, each folder gets a unique subtree so trees from
	// different roots never collide.
	label := mirrorFolderLabels(usedFolders, folderPath)
	multi := len(usedFolders) > 1

	// Space pre-flight against the destination (best-effort). A read-only query, so
	// it may use the authorized canonical name; nothing is mutated through it.
	if free, err := diskFree(dest.Canonical()); err == nil && free < totalBytes {
		return nil, fmt.Errorf("destination too small: need %.1f GB, free %.1f GB", float64(totalBytes)/1e9, float64(free)/1e9)
	}

	res := &MirrorResult{VolumeID: vol.ID, Volume: vol.Label, Dest: destDir, DestinationNotes: dest.Notes()}
	logDestinationNotes(a.Store, "mirror", dest)
	th := &throttler{bps: throttleMbps * 1e6, start: time.Now()}
	throttleBps := throttleMbps * 1e6 // fed to the Performance strip's destination row
	var doneBytes, doneFiles int64
	report := func(msg string) {
		frac := 0.0
		if totalBytes > 0 {
			frac = float64(doneBytes) / float64(totalBytes)
		}
		progress(0.02+frac*0.92, progStats(doneBytes, totalBytes, doneFiles, int64(len(files)), msg))
	}

	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	refs := make([]ChunkFileRef, 0, len(files))
	lastTick := time.Now() // paces mid-file progress so live MB/s updates on big files too
	for i, f := range files {
		doneFiles = int64(i + 1) // file currently being mirrored (matches the "i+1/len" message)
		srcRoot := folderPath[f.FolderID]
		srcPath := filepath.Join(srcRoot, filepath.FromSlash(f.RelPath))
		mrel := f.RelPath
		if multi {
			mrel = label[f.FolderID] + "/" + f.RelPath
		}
		if i%25 == 0 || len(files) < 50 {
			report(fmt.Sprintf("mirroring %d/%d — %s", i+1, len(files), f.RelPath))
		}
		// One confined landing: parent creation, staging, read-back, publication and
		// failure cleanup all happen through the authorized handle, relative to mrel.
		streamHash, n, cerr := copyVerifyToRoot(dest, srcPath, mrel, th, func(d int64) {
			doneBytes += d
			now := time.Now()
			// Feed the Performance strip: a copy reads and writes the same bytes, so the
			// source root and destination volume rows both advance by d.
			a.Perf.Observe("src:"+srcRoot, "source", "", srcRoot, 0, d, now)
			a.Perf.Observe("dst:"+destDir, "dest", vol.Label, destDir, throttleBps, d, now)
			if now.Sub(lastTick) > 700*time.Millisecond {
				lastTick = now
				report(fmt.Sprintf("mirroring %d/%d — %s", i+1, len(files), f.RelPath))
			}
		})
		if cerr != nil {
			// Same split as before the conversion: an unreadable or mid-copy-failing
			// SOURCE is skipped; a destination that would not take, verify or publish
			// the bytes is a failure.
			if errors.Is(cerr, errCopySource) {
				res.Skipped++
			} else {
				res.Failed++
			}
			continue
		}
		if streamHash != f.Hash {
			res.Changed++ // faithful copy of the CURRENT source, which drifted from the catalog
		}
		// Record the level-C sample baseline (size + first/last 4 MiB) so a cheap
		// sample re-verify has something to compare against later.
		sample, _ := dest.SampleHashRel(mrel)
		refs = append(refs, ChunkFileRef{FileID: f.ID, RelPath: f.RelPath, SizeBytes: n, Hash: streamHash, SampleHash: sample})
		res.Mirrored++
		res.Bytes += n
	}

	// Record the verified file-level copies exactly like an adopted mirror, so
	// drift and coverage treat them as real copies (Prompt 23 records).
	if len(refs) > 0 {
		sort.Slice(refs, func(i, j int) bool { return refs[i].RelPath < refs[j].RelPath })
		a.upsertMirrorChunk(collectionID, vol, destDir, refs, "mirror")
	}

	progress(0.96, "refreshing volume inventory sidecar")
	if sc, err := a.writeVolumeInventoryTo(dest, vol); err == nil {
		res.Sidecar = sc
	}
	res.Coverage = a.archiveCoverage([]int{collectionID})
	a.Store.Log("mirror", fmt.Sprintf("%s → %s (%s): %d files (%.1f GB), %d changed, %d failed, %d skipped",
		coll.Name, vol.Label, destDir, res.Mirrored, float64(res.Bytes)/1e9, res.Changed, res.Failed, res.Skipped))
	progress(1.0, fmt.Sprintf("mirrored %d file(s) to %s", res.Mirrored, vol.Label))
	return res, nil
}

// copyVerifyToRoot copies srcPath to rel BENEATH AN AUTHORIZED DESTINATION with the
// full copy-then-verify discipline — stage into a uniquely owned .mnemo_tmp created
// with O_EXCL (hashing on the way), read the bytes back off the destination through
// the same handle and confirm they match, then publish by rename — so a partial or
// corrupted file never appears under its real name. Returns the streamed SHA-256 and
// byte count. The single verified landing path shared by full mirrors and incremental
// "back up changes" runs.
//
// Every step is relative to the authorized root: the parent directory creation, the
// staging file, the read-back, the publication and the cleanup. rel is validated as a
// confined relative path before it reaches the filesystem, so a catalog RelPath or a
// folder label cannot address anything outside the destination.
//
// Failure discipline: publication never removes the previous
// destination file, so a failed publish leaves the existing copy byte-identical; and
// cleanup follows the platform ownership contract in deststage_*.go. Unix
// private staging requires trusted same-UID processes and retains empty containers.
//
// A source-side failure is wrapped with errCopySource so callers can keep counting
// "skipped" (unreadable source) apart from "failed" (destination would not take it).
func copyVerifyToRoot(dest *DestRoot, srcPath, rel string, th *throttler, onBytes func(int64)) (string, int64, error) {
	v, err := destRel(rel)
	if err != nil {
		return "", 0, err
	}
	if dir := path.Dir(v); dir != "." {
		if err := dest.MkdirAll(dir); err != nil {
			return "", 0, err
		}
	}
	tmpRel, out, err := dest.stagingFor(v)
	if err != nil {
		return "", 0, err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		_ = dest.Discard(tmpRel)
		return "", 0, fmt.Errorf("%w: %w", errCopySource, err)
	}
	defer in.Close()
	streamHash, n, err := mirrorCopyStream(in, out, th, onBytes)
	if err != nil {
		_ = dest.Discard(tmpRel)
		return "", n, err
	}
	// Read the bytes back off the destination and confirm byte-identity before the
	// file gets its real name.
	if rerr := dest.VerifyStage(tmpRel, streamHash); rerr != nil {
		_ = dest.Discard(tmpRel)
		return "", n, fmt.Errorf("read-back check failed for %s: the copy on the drive does not match the source. The drive may be failing, or the file changed while copying. Copy it again; if it keeps failing, check the drive's health", path.Base(v))
	}
	if err := dest.Publish(tmpRel, v); err != nil {
		_ = dest.Discard(tmpRel)
		return "", n, err
	}
	return streamHash, n, nil
}

// renameFile is the one rename primitive behind renameReplacing (and so behind
// atomicRename), and a test-only fault-injection seam: production always holds
// os.Rename. Tests replace it to drive the REAL failure handling; an injected error
// demonstrates how a failure is handled, not that a given OS produces that error in
// the simulated circumstance.
var renameFile = os.Rename

// atomicRename publishes the staged file tmp under its real name final, replacing
// whatever is already there, in a single rename.
//
// It never removes final. A rename that fails leaves the existing file exactly as it
// was and reports the error, so a failed publication cannot also destroy the copy it
// was meant to replace (OB-003). The previous version deleted final after ANY first
// rename failure and retried; when the retry failed too, the good bytes were gone and
// the new bytes had never been published. The delete was justified by the claim that
// a rename cannot replace an existing file on Windows. That claim is false for
// os.Rename, which issues MoveFileEx with MOVEFILE_REPLACE_EXISTING there, and
// TestAtomicRename_ReplacesExistingDestination pins the replacing behavior by
// execution on the host that runs the suite.
//
// What "atomic" claims here, precisely: the swap of an existing final for tmp is one
// filesystem operation, so a reader never observes a half-written file under the real
// name, and a failure leaves the previous contents whole. It does NOT claim crash
// durability — no directory fsync is performed, unlike writeCatalog — nor correctness
// across network filesystems, nor safety against another process writing final
// concurrently. None of those are tested, so none are asserted.
//
// Assumptions, all satisfied by the five current callers, which each build tmp as
// final plus a suffix in the same directory: tmp and final are distinct paths on the
// same filesystem, and tmp is a staging file this process owns. Publication across
// filesystems is not supported; the rename simply fails and that error is returned,
// rather than being silently downgraded to a copy that would have to delete or
// overwrite final to finish. On failure tmp is left alone — discarding it is each
// caller's decision, since only the caller knows whether it can be rebuilt.
//
// On Windows a rename blocked by a transient lock is retried for a bounded time; see
// renameReplacing.
func atomicRename(tmp, final string) error {
	if err := renameReplacing(tmp, final); err != nil {
		// Wrapped for context, %w so callers can still inspect the cause.
		return fmt.Errorf("publish %s: %w", final, err)
	}
	return nil
}

// fileSizes lists the cataloged sizes of files, for a destination size check.
func fileSizes(files []*File) []int64 {
	out := make([]int64, len(files))
	for i, f := range files {
		out[i] = f.SizeBytes
	}
	return out
}

// logDestinationNotes records in the activity log what a job's destination was
// accepted with, so it is visible after the job result is gone.
func logDestinationNotes(s *Store, action string, dest *DestRoot) {
	for _, n := range dest.Notes() {
		s.Log(action, dest.Name()+": "+n)
	}
}

// mirrorFolderLabels assigns each source folder a unique, filesystem-safe subtree
// name (its base name, disambiguated by folder id on collision) for multi-folder
// mirrors.
func mirrorFolderLabels(used map[int]bool, folderPath map[int]string) map[int]string {
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := map[int]string{}
	seen := map[string]bool{}
	for _, id := range ids {
		base := safeName(filepath.Base(strings.TrimRight(folderPath[id], `/\`)))
		if base == "" {
			base = fmt.Sprintf("folder-%d", id)
		}
		if seen[base] {
			base = fmt.Sprintf("%s-%d", base, id)
		}
		seen[base] = true
		out[id] = base
	}
	return out
}

// writeVolumeInventoryTo refreshes the OBELISK_DOCK sidecar on an ALREADY
// AUTHORIZED destination, describing everything the catalog says this volume
// holds — a self-documenting inventory that survives the catalog.
//
// The sidecar obeys exactly the authority the payload writes obey. It used to
// re-run the AssertOutsideSources string test and then os.MkdirAll / os.WriteFile a
// rebuilt absolute path, which followed whatever that name resolved to — in the
// recorded reproduction it wrote the inventory INSIDE a registered source through a
// destination alias. Now it writes root-relative, staged and published like a
// payload file, and re-checking the pathname is neither needed nor trusted.
func (a *App) writeVolumeInventoryTo(dest *DestRoot, vol *Volume) (string, error) {
	if err := dest.MkdirAll(dockSidecarDir); err != nil {
		return "", err
	}
	dir := filepath.Join(dest.Name(), dockSidecarDir) // display only; never reopened
	now := time.Now().UTC()

	type invFile struct {
		RelPath string `json:"rel_path"`
		Hash    string `json:"hash"`
		Size    int64  `json:"size_bytes"`
	}
	type invArchive struct {
		ArchiveID int            `json:"archive_id"`
		Archive   string         `json:"archive"`
		Chunk     string         `json:"chunk"`
		Mirror    bool           `json:"mirror"`
		Integrity *BuildVerified `json:"integrity,omitempty"` // built packages attest their effective integrity
		Files     []invFile      `json:"files"`
	}
	var archives []invArchive
	var totalFiles int
	var totalBytes int64
	tally := newExtTally() // format census of exactly what this volume holds
	for _, c := range a.Store.Chunks(0) {
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
		files := make([]invFile, 0, len(c.Files))
		for _, ref := range c.Files {
			files = append(files, invFile{RelPath: ref.RelPath, Hash: ref.Hash, Size: ref.SizeBytes})
			totalBytes += ref.SizeBytes
			tally.add(ref.RelPath, ref.SizeBytes)
		}
		sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
		totalFiles += len(files)
		name := ""
		if coll := a.Store.Collection(c.CollectionID); coll != nil {
			name = coll.Name
		}
		archives = append(archives, invArchive{ArchiveID: c.CollectionID, Archive: name, Chunk: c.Name, Mirror: c.Mirror, Integrity: c.BuildVerified, Files: files})
	}

	census := a.censusFromTally(tally)
	snap := map[string]any{
		"obelisk_volume_inventory": 1, "generated_utc": now.Format(time.RFC3339),
		"volume": vol, "total_files": totalFiles, "total_bytes": totalBytes, "archives": archives,
		"formats": census,
	}
	sb, _ := json.MarshalIndent(snap, "", "  ")
	if err := dest.WriteFileAtomic(dockSidecarDir+"/catalog_snapshot.json", sb, 0o644); err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("# Obelisk — volume inventory\n\n")
	b.WriteString(fmt.Sprintf("Generated %s. This medium holds **%d file(s)** (%s) across %d archive(s).\n\n",
		now.Format(time.RFC3339), totalFiles, humanBytes(totalBytes), len(archives)))
	b.WriteString(fmt.Sprintf("- **Volume:** %s\n", vol.Label))
	if vol.Serial != "" {
		b.WriteString(fmt.Sprintf("- **Serial:** `%s`\n", vol.Serial))
	}
	b.WriteString("\nMirror files are stored as **plain files** — browse or copy them with any tool; no Obelisk, key, or unpack step is needed. Each was copy-then-verified (SHA-256) against its source.\n")
	for _, ar := range archives {
		kind := "package"
		if ar.Mirror {
			kind = "mirror"
		}
		b.WriteString(fmt.Sprintf("\n## %s — %s (%d file(s))\n\n", ar.Archive, kind, len(ar.Files)))
		for i, f := range ar.Files {
			if i >= 2000 {
				b.WriteString(fmt.Sprintf("_…and %d more (see catalog_snapshot.json)_\n", len(ar.Files)-i))
				break
			}
			b.WriteString(fmt.Sprintf("- `%s`\n", f.RelPath))
		}
	}
	b.WriteString("\n---\n\n")
	b.WriteString(fmt.Sprintf("**Formats:** %.0f%% of these bytes are in OPEN or DOCUMENTED formats.\n\n", census.SafePct))
	b.WriteString("```\n" + readersReference(census) + "```\n")
	if err := dest.WriteFileAtomic(dockSidecarDir+"/INVENTORY.md", []byte(b.String()), 0o644); err != nil {
		return dir, err
	}
	return dir, nil
}
