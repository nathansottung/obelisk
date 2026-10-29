package main

// writer.go — RAM ring buffer (goroutines + a bounded channel), read-back
// verify, and the three-tool restore. This is the file that would host a
// raw-tape (/dev/nst0) backend later; everything above it is agnostic.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type RingStats struct {
	BufferBlocks  int     `json:"buffer_blocks"`
	BlockBytes    int     `json:"block_bytes"`
	MinFill       int     `json:"min_fill"`
	StarvedEvents int     `json:"starved_events"`
	Bytes         int64   `json:"bytes"`
	Seconds       float64 `json:"seconds"`
	ReadMBps      float64 `json:"read_mbps"`
	WriteMBps     float64 `json:"write_mbps"`
}

// ringCopy streams src -> dst through a bounded channel of blocks,
// hashing on the READ side (so the hash proves what left the source).
//
// throttleMbps > 0 paces the WRITER (drain) side only — the reader keeps
// filling the ring unpaced, which is the whole point of the buffer: read fast,
// write at a steady capped rate (e.g. to keep an SSD from overheating). Pacing
// is against cumulative bytes vs elapsed time, so it self-corrects into a
// smooth, steady rate instead of bursts.
// offset/length select a byte range of src; length <= 0 means "from offset to
// EOF". This lets a spanned chunk stream one segment's range through the same
// ring buffer as a whole-file write.
// live, when non-nil, is called each drained block with the current cumulative read
// (source) and written (destination) byte counts, the buffer fill percentage, and the
// live stall count — feeding the Performance strip's ring gauge. It must be cheap and
// non-blocking; pass nil when no live telemetry is wanted.
//
// dst is written as a new staging file beside it, created exclusively, and renamed
// to dst only once it is flushed, so a file already at dst (or a hard link to
// another file) is replaced as a name and never written through.
func ringCopy(src, dst string, offset, length int64, blockMB int, bufferGB, throttleMbps float64, progress func(done, total int64), live func(read, written int64, fillPct, stalls int)) (string, RingStats, error) {
	out, err := createStage(dst, 0o644)
	if err != nil {
		return "", RingStats{}, err
	}
	tmp := out.Name()
	hash, stats, err := ringCopyTo(src, out, offset, length, blockMB, bufferGB, throttleMbps, progress, live)
	if err == nil {
		// The payload is not written until the medium says it is (see finalizeWrite).
		err = finalizeWrite(out, dst)
	}
	if err == nil {
		err = renameReplacing(tmp, dst)
	}
	if err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return "", stats, err
	}
	return hash, stats, nil
}

// ringCopyTo is ringCopy into out, an open file the caller owns: it writes the
// stream and returns its hash, and leaves flushing, closing and naming the file to
// the caller.
func ringCopyTo(src string, out *os.File, offset, length int64, blockMB int, bufferGB, throttleMbps float64, progress func(done, total int64), live func(read, written int64, fillPct, stalls int)) (string, RingStats, error) {
	block := blockMB << 20
	depth := int(bufferGB * float64(1<<30) / float64(block))
	if depth < 2 {
		depth = 2
	}
	stats := RingStats{BufferBlocks: depth, BlockBytes: block, MinFill: depth}

	in, err := os.Open(src)
	if err != nil {
		return "", stats, err
	}
	defer in.Close()
	if offset > 0 {
		if _, err := in.Seek(offset, io.SeekStart); err != nil {
			return "", stats, err
		}
	}
	total := length
	if total <= 0 {
		if st, err := in.Stat(); err == nil {
			total = st.Size() - offset
		}
	}
	ch := make(chan []byte, depth)
	errCh := make(chan error, 1)
	h := sha256.New()
	var readBytes int64
	var readSecs float64
	var readerDone int32 // set when the reader has produced its last block

	go func() {
		t0 := time.Now()
		remaining := length // when > 0, read exactly this many bytes
		for {
			nb := block
			if length > 0 {
				if remaining <= 0 {
					break
				}
				if int64(nb) > remaining {
					nb = int(remaining)
				}
			}
			b := make([]byte, nb)
			n, err := io.ReadFull(in, b)
			if n > 0 {
				h.Write(b[:n])
				ch <- b[:n]
				atomic.AddInt64(&readBytes, int64(n)) // atomic: the writer loop reads it live
				remaining -= int64(n)
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// A bounded (byte-range) copy that reaches EOF before the requested
				// length has a TRUNCATED source — that is a read failure, not a clean
				// end. Surface it as the real I/O error so the caller reports the read
				// error, instead of silently streaming a short payload whose hash then
				// fails downstream and manifests as a misleading "hash mismatch".
				// (For a whole-file copy, length<=0, EOF is the normal end of stream.)
				if length > 0 && remaining > 0 {
					errCh <- fmt.Errorf("short read from %s: source ended %d byte(s) before the expected %d", src, remaining, length)
				}
				break
			}
			if err != nil {
				errCh <- err
				break
			}
		}
		readSecs = time.Since(t0).Seconds()
		atomic.StoreInt32(&readerDone, 1)
		close(ch)
	}()

	throttleBps := throttleMbps * 1e6
	start := time.Now()
	var written int64
	for b := range ch {
		fill := len(ch)
		// Sample buffer occupancy only in steady state: skip the first block
		// (buffer still warming) and everything after the reader has finished
		// (the tail always drains to empty — counting it would peg min at 0 and
		// hide whether the writer ever actually starved mid-stream).
		if written > 0 && atomic.LoadInt32(&readerDone) == 0 {
			if fill < stats.MinFill {
				stats.MinFill = fill
				if fill == 0 {
					stats.StarvedEvents++
				}
			}
		}
		if _, err := out.Write(b); err != nil {
			return "", stats, err
		}
		written += int64(len(b))
		if total > 0 {
			progress(written, total)
		}
		if live != nil {
			pct := 0
			if depth > 0 {
				pct = fill * 100 / depth
			}
			live(atomic.LoadInt64(&readBytes), written, pct, stats.StarvedEvents)
		}
		// Writer-side pacing only: sleep until cumulative bytes match the target
		// rate. Self-correcting against wall clock, so the rate stays smooth.
		if throttleBps > 0 {
			target := time.Duration(float64(written) / throttleBps * float64(time.Second))
			if el := time.Since(start); target > el {
				time.Sleep(target - el)
			}
		}
	}
	select {
	case err := <-errCh:
		return "", stats, err
	default:
	}
	secs := time.Since(start).Seconds()
	stats.Bytes, stats.Seconds = written, round2(secs)
	stats.WriteMBps = round1(float64(written) / 1e6 / secs)
	if readSecs > 0 {
		stats.ReadMBps = round1(float64(atomic.LoadInt64(&readBytes)) / 1e6 / readSecs)
	}
	return hex.EncodeToString(h.Sum(nil)), stats, nil
}

// writeFlushFaultHook is a test-only fault-injection seam (nil in production): when
// set, finalizeWrite consults it in place of the real Sync and, if it returns an
// error, fails the write exactly as a failed final flush would. It lets tests prove a
// destination that cannot be flushed is never reported as a successful write, without
// having to stage a genuinely full disk. See finalizeWrite.
var writeFlushFaultHook func(dst string) error

// finalizeWrite forces a freshly written file all the way to stable storage and
// reports any error from that final flush or close, so a caller may only treat a write
// as done once the medium has actually accepted every byte. Callers keep their
// deferred Close as the safety net for error paths — closing an already-closed file is
// harmless, and the deferred error is the one we can afford to drop.
func finalizeWrite(out *os.File, dst string) error {
	if writeFlushFaultHook != nil {
		if err := writeFlushFaultHook(dst); err != nil {
			out.Close()
			return fmt.Errorf("flushing %s: %w", dst, err)
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("flushing %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", dst, err)
	}
	return nil
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

// ringPerfLive builds the ringCopy `live` callback that feeds the Performance strip: a
// source (read) row, a destination (write) row labelled with the volume, and the live
// buffer fill/stall gauge on the destination. The volume label is resolved ONCE up
// front (never per block), and byte deltas are diffed from the cumulative counts. The
// meter's methods are nil-safe, so this is harmless when no strip is watching.
func (a *App) ringPerfLive(srcPath, destDir string, volumeID int, throttleMbps float64) func(read, written int64, fillPct, stalls int) {
	destLabel := ""
	if v := a.Store.Volume(volumeID); v != nil {
		destLabel = v.Label
	}
	srcID, dstID := "src:"+srcPath, "dst:"+destDir
	throttleBps := throttleMbps * 1e6
	var lastR, lastW int64
	return func(read, written int64, fillPct, stalls int) {
		now := time.Now()
		if d := read - lastR; d > 0 {
			a.Perf.Observe(srcID, "source", "", srcPath, 0, d, now)
			lastR = read
		}
		if d := written - lastW; d > 0 {
			a.Perf.Observe(dstID, "dest", destLabel, destDir, throttleBps, d, now)
			lastW = written
		}
		a.Perf.SetBuffer(dstID, "dest", destLabel, destDir, fillPct, stalls, now)
	}
}

// ---- chunk-level operations -------------------------------------------

// stagedPayloadPresent reports whether the package's staged payload still exists
// on disk — the local artifact a fresh copy can be re-written from.
func (a *App) stagedPayloadPresent(c *Chunk) bool {
	return c.StagedDir != "" && payloadPathIn(c.StagedDir, c) != ""
}

// refreshChunkStatus recomputes a package's lifecycle status from the best
// available evidence, per the multi-copy model: one bad copy never drags the
// whole package to FAILED while the staged artifact or another verified copy
// survives. FAILED stays reserved for build/staging failures (set directly by
// those paths); this never sets FAILED. PLANNED/BUILDING are pre-staging states
// owned by the build and left untouched (a write/verify can't run on them). The
// mid-write WRITING state IS resolved here — the write calls this at the end to
// transition to its evidence-based status.
func (a *App) refreshChunkStatus(c *Chunk) {
	switch c.Status {
	case "PLANNED", "BUILDING":
		return
	}
	switch {
	case c.VerifiedCopyCount() > 0:
		ok := true
		c.Status, c.Error, c.VerifyOK = "VERIFIED", "", &ok
	case c.CurrentCopyCount() > 0:
		// A copy exists but none verifies — written yet under-protected; the
		// per-copy state carries the failure, the package is not FAILED.
		bad := false
		c.Status, c.Error, c.VerifyOK = "WRITTEN", "", &bad
	case a.stagedPayloadPresent(c):
		c.Status, c.Error, c.VerifyOK = "STAGED", "", nil
	}
}

func (a *App) WriteChunk(id int, destDir string, bufferGB float64, blockMB int, throttleMbps float64, volumeID int, progress func(float64, string)) (map[string]any, error) {
	return a.writeChunk(nil, id, destDir, bufferGB, blockMB, throttleMbps, volumeID, progress, nil)
}

// chunkWritable is the check that a package can be written from staging now.
func chunkWritable(c *Chunk) error {
	if c.Spanned {
		return fmt.Errorf("package %s is spanned; use span-write (one tape at a time)", c.Name)
	}
	return chunkStagedForWrite(c)
}

// chunkStagedForWrite is the check, shared with span writes, that a package's
// last build succeeded and left its payload and sidecars in staging. A FAILED
// package (its build failed, or its staged payload was found damaged) is rebuilt
// first.
func chunkStagedForWrite(c *Chunk) error {
	if c.StagedDir == "" || (c.Status != "STAGED" && c.Status != "WRITTEN" && c.Status != "VERIFIED") {
		return fmt.Errorf("package %s is %s; build it first", c.Name, c.Status)
	}
	if payloadPathIn(c.StagedDir, c) == "" {
		return fmt.Errorf("staged payload missing under %s", c.StagedDir)
	}
	return checkStagedSidecars(c)
}

// writeChunk is WriteChunk into out when the caller already holds the output
// folder for destDir (a re-write), or into destDir opened here when out is nil.
// beforeWrite, when set, runs once every check has passed, just before the
// first byte is written (a re-write supersedes the old copy there).
func (a *App) writeChunk(out *outFolder, id int, destDir string, bufferGB float64, blockMB int, throttleMbps float64, volumeID int, progress func(float64, string), beforeWrite func()) (map[string]any, error) {
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return nil, cfgErr
	}
	if bufferGB <= 0 {
		bufferGB = cfg.BufferGB
	}
	if blockMB <= 0 {
		blockMB = cfg.BlockMB
	}
	if throttleMbps <= 0 { // 0 = use configured default (which is itself 0 = unthrottled)
		throttleMbps = cfg.ThrottleMbps
	}
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", id)
	}
	if err := chunkWritable(c); err != nil {
		return nil, err
	}
	enc := payloadPathIn(c.StagedDir, c)
	// The destination is decided by where it really is, and every file is written
	// beneath the folder that was decided about (see outfolder.go).
	if out == nil {
		var err error
		if out, err = a.openOutFolder(destDir); err != nil {
			return nil, err
		}
		defer out.Close()
	}
	// Preserve whatever name the payload was staged under (current <name>.tar /
	// <name>.tar.gpg, or the legacy .tar.gpg for a plaintext package built before
	// the rename) so the on-medium payload stays consistent with its sidecars.
	payloadBase := filepath.Base(enc)
	if err := out.MkdirAll(c.Name); err != nil {
		return nil, err
	}
	dest := out.Join(c.Name)
	var need int64
	entries, _ := os.ReadDir(c.StagedDir)
	for _, e := range entries {
		n := e.Name()
		// Skip staging-only files: dirs, the filelist, and the intermediate tar
		// that sits beside the ciphertext for encrypted packages. For a plaintext
		// package the tar IS the payload (n == payloadBase), so it is counted.
		if e.IsDir() || n == "filelist.txt" {
			continue
		}
		if n == c.Name+".tar" && n != payloadBase {
			continue
		}
		if st, err := e.Info(); err == nil {
			need += st.Size()
		}
	}
	if free, err := destFree(destDir); err == nil && free < need {
		return nil, fmt.Errorf("destination too small: need %.1f GB, free %.1f GB", float64(need)/1e9, float64(free)/1e9)
	}

	if beforeWrite != nil {
		beforeWrite()
	}
	c.Status = "WRITING"
	a.Store.UpdateChunk(c)
	// mediumFail: a write/medium error. The staged artifact is fine, so the
	// package falls back to its evidence-based status (STAGED, or still VERIFIED
	// via another copy) — never FAILED for a bad medium.
	mediumFail := func(err error) (map[string]any, error) {
		a.refreshChunkStatus(c)
		a.Store.UpdateChunk(c)
		return nil, err
	}
	// stagedFail: the staged artifact itself is corrupt (its bytes no longer hash
	// to enc_hash). That IS a package-level FAILED.
	stagedFail := func(err error) (map[string]any, error) {
		c.Status, c.Error = "FAILED", storedErrorText(err.Error())
		a.Store.UpdateChunk(c)
		return nil, err
	}

	progress(0.02, "writing payload")
	payloadRel := c.Name + "/" + payloadBase
	destPayload := out.Join(payloadRel)
	tmp, f, err := out.Create(payloadRel)
	if err != nil {
		return mediumFail(err)
	}
	streamHash, stats, err := ringCopyTo(enc, f, 0, 0, blockMB, bufferGB, throttleMbps,
		func(done, total int64) {
			frac := 0.0
			if total > 0 {
				frac = float64(done) / float64(total)
			}
			progress(0.02+frac*0.66, progBytes(done, total, "writing payload"))
		},
		a.ringPerfLive(enc, dest, volumeID, throttleMbps))
	c.RingStats = &stats // telemetry: proof the buffer decoupled read from a throttled write
	// A ringCopy read error (including a short/truncated source) must be reported as
	// the I/O error it is — checked BEFORE the hash comparison, so a read failure is
	// never masked as a stream-hash mismatch.
	if err != nil {
		out.Discard(tmp)
		return mediumFail(err)
	}
	if streamHash != c.EncHash {
		out.Discard(tmp)
		return stagedFail(fmt.Errorf("stream hash mismatch while writing (staged payload corrupted — rebuild)"))
	}

	// Read-back verify: the payload gets its name only once it reads back off the
	// medium as the staged bytes. A copy that reads back differently is not
	// published and is recorded as failed.
	progress(0.70, "read-back verify")
	ok := true
	rb := streamHash
	if err := out.Commit(tmp, payloadRel, 0o644, streamHash); err != nil {
		if !errors.Is(err, errStageMismatch) {
			return mediumFail(err)
		}
		ok, rb = false, "(differs from the staged payload)"
	}

	progress(0.78, "writing sidecars")
	for _, e := range entries {
		if !ok {
			break
		}
		n := e.Name()
		// Skip the payload (streamed above) and the staging-only intermediate tar
		// and filelist; copy every real sidecar (par2 set, manifest, RESTORE.txt).
		if e.IsDir() || n == payloadBase || n == c.Name+".tar" || n == "filelist.txt" {
			continue
		}
		if c.PrivateManifest && n == c.Name+".manifest.json" {
			continue // private: the ENCRYPTED manifest.json.gpg ships instead
		}
		sum, err := out.CopyIn(filepath.Join(c.StagedDir, n), c.Name+"/"+n, 0o644)
		if err != nil {
			return mediumFail(err)
		}
		if want, ok := c.Sidecars[n]; ok && sum != want {
			return mediumFail(fmt.Errorf("staged sidecar %s changed during the write; rebuild the package", n))
		}
	}

	now := time.Now().UTC()
	c.WrittenDest, c.WrittenAt = dest, &now
	if ok {
		c.VerifiedAt = &now
	}
	if volumeID <= 0 {
		volumeID = a.Store.EnsureUnregistered().ID
	}
	// Record the result on THIS copy; derive package status from all copies. A
	// bad read-back marks only this copy failed — the package stays healthy if
	// the staged payload or another verified copy is intact.
	a.Store.RecordCopy(c, volumeID, dest, ok)
	// Awareness: if this landed on a tape whose drive is actively encrypting (stenc),
	// record it on the volume so kits/inventories shout about the drive-key risk.
	a.noteTapeDriveEncryption(volumeID, cfg)
	note := "write read-back"
	if !ok {
		note = "write read-back: hash mismatch medium=" + rb
	}
	a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: ok, Path: destPayload, Note: note})
	a.refreshChunkStatus(c)
	a.Store.UpdateChunk(c)
	a.Store.Log("write", fmt.Sprintf("%s -> %s verify_ok=%v", c.Name, dest, ok))
	if ok {
		progress(1.0, "verified")
	} else {
		progress(1.0, "read-back MISMATCH — copy marked failed")
	}
	return map[string]any{"chunk": c.Name, "dest": dest, "verify_ok": ok,
		"status": c.Status, "verified_copies": c.VerifiedCopyCount(), "ring_buffer": stats}, nil
}

// RewriteCopy re-writes the package's copy on volumeID from staging to the same
// destination folder, superseding the existing (typically FAILED) copy. The old
// record is retained in history (superseded=true); the write creates a fresh
// Copy. This is the "Re-write this copy" affordance for a failed medium.
func (a *App) RewriteCopy(id, volumeID int, bufferGB float64, blockMB int, throttleMbps float64, progress func(float64, string)) (map[string]any, error) {
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", id)
	}
	if c.Spanned {
		return nil, fmt.Errorf("package %s is spanned; re-write its segments via span-write", c.Name)
	}
	var dest string
	for i := range c.Copies {
		if c.Copies[i].VolumeID == volumeID && !c.Copies[i].Superseded {
			dest = c.Copies[i].Path
			break
		}
	}
	if dest == "" {
		return nil, fmt.Errorf("no current copy of %s on volume %d to re-write", c.Name, volumeID)
	}
	// Check the package and open the folder (deciding it and taking its lease)
	// before the old copy is marked superseded, so a re-write that cannot run
	// changes nothing. WriteChunk rebuilds dest as destDir/<name>, so the copy
	// folder's parent is opened.
	if err := chunkWritable(c); err != nil {
		return nil, err
	}
	out, err := a.openOutFolder(filepath.Dir(dest))
	if err != nil {
		return nil, err
	}
	defer out.Close()
	// The old copy is superseded only when the write starts: a re-write refused
	// by a later check (free space, say) leaves it current.
	return a.writeChunk(out, id, filepath.Dir(dest), bufferGB, blockMB, throttleMbps, volumeID, progress,
		func() { a.Store.SupersedeCopy(c, volumeID) })
}

func (a *App) VerifyChunk(id int, destDir string) (map[string]any, error) {
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", id)
	}
	base := destDir
	if base == "" {
		base = c.WrittenDest
	}
	enc := findPayload(base, c)
	if enc == "" {
		return nil, fmt.Errorf("payload for %s not found under %s (looked for %s, flat or in a %s/ folder)",
			c.Name, base, strings.Join(payloadNameCandidates(c), " or "), c.Name)
	}
	rb, err := hashFileHex(enc)
	if err != nil {
		return nil, err
	}
	ok := rb == c.EncHash
	// A copy is the payload and its sidecars: they are read back too.
	sideErr := checkSidecarsAt(c, filepath.Dir(enc))
	if sideErr != nil {
		ok = false
	}
	now := time.Now().UTC()
	// Record the result on the copy recorded at the folder that was read, and only
	// that one; the package status derives from ALL copies, so a single bad medium
	// never marks the package FAILED while another copy or the staged payload is
	// intact. A read that matches no recorded copy (the staging folder, say)
	// credits nothing.
	credited := a.Store.CreditCopyCheck(c, CopyAt{Path: filepath.Dir(enc), Serial: driveSerialAt(base)}, ok, VerifyB)
	if ok && credited > 0 {
		c.VerifiedAt = &now
	}
	note := "media verify"
	switch {
	case sideErr != nil:
		note = "media verify: " + sideErr.Error()
	case !ok:
		note = "media verify: hash mismatch medium=" + rb
	}
	if credited == 0 {
		note += " · no recorded copy at " + filepath.Dir(enc) + " (or the drive could not be told apart), so no copy was credited"
	}
	a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: ok, Path: enc, Note: note, Advisory: credited == 0})
	a.refreshChunkStatus(c)
	a.Store.UpdateChunk(c)
	return map[string]any{"chunk": c.Name, "path": enc, "verify_ok": ok, "expected": c.EncHash, "actual": rb,
		"credited": credited, "status": c.Status, "verified_copies": c.VerifiedCopyCount()}, nil
}

// VerifyCampaign scans dest_dir for chunk folders/payloads whose names match
// cataloged chunks and re-verifies each against its enc_hash — "insert the
// tape/disc, verify everything on it in one click". Strictly read-only with
// respect to media; only the catalog's verify history/status is updated.
func (a *App) VerifyCampaign(destDir, level string, progress func(float64, string)) (map[string]any, error) {
	if strings.TrimSpace(destDir) == "" {
		return nil, fmt.Errorf("dest_dir required")
	}
	level = normLevel(level)
	byName := map[string]*Chunk{}
	for _, c := range a.Store.Chunks(0) {
		byName[c.Name] = c
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", destDir, err)
	}
	type cand struct {
		c    *Chunk
		path string
	}
	var cands []cand
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() { // dest/NAME/<payload> (current or legacy name)
			if c, ok := byName[name]; ok {
				if p := payloadPathIn(filepath.Join(destDir, name), c); p != "" {
					cands = append(cands, cand{c, p})
				}
			}
			continue
		}
		// flat payload: dest/NAME.tar or dest/NAME.tar.gpg
		var base string
		if strings.HasSuffix(name, ".tar.gpg") {
			base = strings.TrimSuffix(name, ".tar.gpg")
		} else if strings.HasSuffix(name, ".tar") {
			base = strings.TrimSuffix(name, ".tar")
		} else {
			continue
		}
		if c, ok := byName[base]; ok {
			cands = append(cands, cand{c, filepath.Join(destDir, name)})
		}
	}
	// Native mirror packages whose files live under this medium (level applies).
	var mirrorTargets []*Chunk
	for _, c := range a.Store.Chunks(0) {
		if !c.Mirror {
			continue
		}
		for _, cp := range c.Copies {
			// A mirror's copy is recorded at the mount it was written to.
			if !cp.Superseded && normPath(cp.Path) == normPath(destDir) {
				mirrorTargets = append(mirrorTargets, c)
				break
			}
		}
	}
	serial := driveSerialAt(destDir) // which drive is being read, when it can be found
	if len(cands) == 0 && len(mirrorTargets) == 0 {
		return nil, fmt.Errorf("no cataloged packages or mirrors found on %s (looked for NAME/<payload>, NAME.tar[.gpg], or a mirror rooted here)", destDir)
	}
	var okCount int
	results := make([]map[string]any, 0, len(cands))
	for i, cd := range cands {
		progress(float64(i)/float64(len(cands)+len(mirrorTargets)), "verify "+cd.c.Name)
		// Package payloads are ALWAYS level B — full content hash, no option.
		h, herr := hashFileHex(cd.path)
		ok := herr == nil && h == cd.c.EncHash
		note := "campaign"
		if herr != nil {
			note = "campaign: " + herr.Error()
		} else if !ok {
			note = "campaign: hash mismatch"
		} else if serr := checkSidecarsAt(cd.c, filepath.Dir(cd.path)); serr != nil {
			ok, note = false, "campaign: "+serr.Error()
		}
		now := time.Now().UTC()
		// Copy-level result, on the copy recorded where this payload was read and
		// only there; package status derives from all copies (a bad medium on this
		// campaign does not fail a package with other verified copies).
		credited := a.Store.CreditCopyCheck(cd.c, CopyAt{Path: filepath.Dir(cd.path), Serial: serial}, ok, VerifyB)
		if ok && credited > 0 {
			cd.c.VerifiedAt = &now
			okCount++
		}
		if credited == 0 {
			note += " · no recorded copy here, so no copy was credited"
		}
		a.Store.AppendVerifyEvent(cd.c, VerifyEvent{At: now, OK: ok, Path: cd.path, Note: note, Level: VerifyB, Advisory: credited == 0})
		a.refreshChunkStatus(cd.c)
		a.Store.UpdateChunk(cd.c)
		results = append(results, map[string]any{"chunk": cd.c.Name, "ok": ok, "path": cd.path, "level": VerifyB,
			"status": cd.c.Status, "verified_copies": cd.c.VerifiedCopyCount()})
	}
	// Mirrors: verified at the chosen level (A/C advisory, B satisfies COMPLETE).
	mirrorResults := make([]map[string]any, 0, len(mirrorTargets))
	for j, c := range mirrorTargets {
		progress(float64(len(cands)+j)/float64(len(cands)+len(mirrorTargets)), "verify mirror "+c.Name)
		ok, checked, bad, firstBad := verifyMirrorChunk(c, destDir, level)
		now := time.Now().UTC()
		note := fmt.Sprintf("campaign mirror (%s): %d/%d ok", levelTag(level), checked-bad, checked)
		if !ok {
			note += " · first bad: " + firstBad
		}
		credited := a.Store.CreditCopyCheck(c, CopyAt{Path: destDir, Serial: serial}, ok, level)
		if credited == 0 {
			note += " · the drive could not be told apart from another with a copy here, so no copy was credited"
		}
		a.Store.AppendVerifyEvent(c, VerifyEvent{At: now, OK: ok, Path: destDir, Note: note, Level: level, Advisory: !levelSatisfiesComplete(level) || credited == 0})
		if level == VerifyB {
			if ok && credited > 0 {
				c.VerifiedAt = &now
			}
			a.refreshChunkStatus(c)
			a.Store.UpdateChunk(c)
		}
		if ok {
			okCount++
		}
		mirrorResults = append(mirrorResults, map[string]any{"chunk": c.Name, "ok": ok, "level": level,
			"checked": checked, "bad": bad, "advisory": !levelSatisfiesComplete(level)})
	}
	if level == VerifyB && len(mirrorTargets) > 0 {
		a.Store.RecomputeProtection(nil)
	}
	total := len(cands) + len(mirrorTargets)
	progress(1.0, fmt.Sprintf("verified %d/%d ok", okCount, total))
	a.Store.Log("verify-campaign", fmt.Sprintf("%s: %d/%d ok (mirror level %s)", destDir, okCount, total, level))
	return map[string]any{"dest_dir": destDir, "checked": total, "ok": okCount, "level": level,
		"results": results, "mirror_results": mirrorResults}, nil
}

func (a *App) RestoreChunk(id int, sourceDir, outputDir string, members []string, progress func(float64, string)) (map[string]any, error) {
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", id)
	}
	if sourceDir == "" {
		sourceDir = c.WrittenDest
	}
	var restoreWarnings []string // non-fatal issues the operator must still see (e.g. a missing par2 set)
	// Restore writes only into an output folder authorized by the destination
	// guard: never a source, inside one or holding one, by real location. The
	// package is unpacked into a private work folder there, and each file then
	// gets its real name without ever replacing one (see restore_guard.go).
	// Where the guard is not available (macOS, the BSDs) a restore goes only into
	// a new folder that this restore creates (see newRestoreFolder).
	tgt, err := a.openRestoreTarget(outputDir)
	if err != nil {
		return nil, err
	}
	succeeded := false
	defer func() { a.closeRestoreTarget(tgt, succeeded) }()
	work := tgt.work
	restoreWarnings = append(restoreWarnings, tgt.notes...)
	enc := findPayload(sourceDir, c)
	if enc == "" {
		// Spanned restore drill: if the joined payload isn't present but the
		// segment files are (all copied into one scratch dir), rejoin them —
		// the same `cat seg* > payload` step RESTORE.txt documents by hand.
		if c.Spanned {
			joined, warns, jerr := rejoinSegments(sourceDir, work.abs, c, progress)
			if jerr != nil {
				return nil, jerr
			}
			for _, wmsg := range warns {
				a.Store.Log("restore", c.Name+": "+wmsg)
			}
			restoreWarnings = append(restoreWarnings, warns...)
			enc = joined
		} else {
			return nil, fmt.Errorf("payload for %s not found under %s (point source at the package folder on the medium)", c.Name, sourceDir)
		}
	}
	par2Bin, err := a.tool("par2")
	if err != nil {
		return nil, err
	}
	tarBin, err := a.tool("tar")
	if err != nil {
		return nil, err
	}
	var gpgBin string
	if c.Encrypted {
		if gpgBin, err = a.tool("gpg"); err != nil {
			return nil, err
		}
	}

	repaired := false
	par2f := enc + ".par2"
	progress(0.05, "par2 verify")
	if _, err := os.Stat(par2f); err == nil {
		if verr := run(par2Bin, "", "verify", par2f); verr != nil {
			// Repair works on a copy in the restore's work folder. The medium (and
			// whatever folder the package is read from, a source included) is only
			// ever read: par2 rewrites the file it repairs and leaves the damaged
			// original beside it.
			progress(0.10, "par2 repair (on a copy)")
			copied, cerr := copyForRepair(enc, filepath.Join(work.abs, "repair"))
			if cerr != nil {
				return nil, fmt.Errorf("copying the package for repair: %w", cerr)
			}
			if rerr := run(par2Bin, "", "repair", copied+".par2"); rerr != nil {
				return nil, fmt.Errorf("par2 repair failed: %v", rerr)
			}
			enc = copied
			repaired = true
		}
	} else if h, _ := hashFileHex(enc); h != c.EncHash {
		return nil, fmt.Errorf("no par2 present and ciphertext hash mismatch — data damaged")
	}

	// A new restore folder (no destination guard) is unpacked into by path: it
	// must still be the folder that was created and checked.
	if tgt.dest == nil {
		if err := tgt.stillTheFolder(); err != nil {
			return nil, err
		}
	}
	if !c.Encrypted {
		// Unencrypted: the payload is a plain tar; extract it directly, no gpg.
		progress(0.25, "extract")
		// "--" ends option parsing: members are archive member names chosen by the
		// caller, and a member beginning with "-" would otherwise be read as a tar
		// option rather than as the file to restore.
		targs := []string{"-xf", enc, "-C", tgt.into, "--"}
		targs = append(targs, members...)
		if err := run(tarBin, "", targs...); err != nil {
			return nil, fmt.Errorf("tar extract failed: %v", err)
		}
		res, err := a.finishRestore(c, tgt, outputDir, repaired, restoreWarnings, progress)
		succeeded = err == nil
		return res, err
	}

	progress(0.25, "decrypt + extract")
	pass, err := a.Passphrase(c.KeyRef)
	if err != nil {
		return nil, err
	}
	gpg := helperCommand(gpgBin, "--batch", "--yes", "--pinentry-mode", "loopback",
		"--passphrase-fd", "0", "-d", enc)
	gpg.Stdin = strings.NewReader(pass)
	pipe, err := gpg.StdoutPipe()
	if err != nil {
		return nil, err
	}
	targs := []string{"-xf", "-", "-C", tgt.into, "--"} // "--": member names are never options
	targs = append(targs, members...)
	tarc := helperCommand(tarBin, targs...)
	tarc.Stdin = pipe
	var tarErr strings.Builder
	tarc.Stderr = &tarErr
	var gpgErr strings.Builder
	gpg.Stderr = &gpgErr

	if err := gpg.Start(); err != nil {
		return nil, err
	}
	if err := tarc.Start(); err != nil {
		return nil, err
	}
	terr := tarc.Wait()
	gerr := gpg.Wait()
	if gerr != nil {
		return nil, fmt.Errorf("gpg decrypt failed: %s", tail(gpgErr.String(), 400))
	}
	if terr != nil {
		return nil, fmt.Errorf("tar extract failed: %s", tail(tarErr.String(), 400))
	}
	res, err := a.finishRestore(c, tgt, outputDir, repaired, restoreWarnings, progress)
	succeeded = err == nil
	return res, err
}

// finishRestore publishes what was extracted into the work folder to its real
// names in the output folder (refusing any name already taken) and reports.
func (a *App) finishRestore(c *Chunk, tgt *restoreTarget, outputDir string, repaired bool, warnings []string, progress func(float64, string)) (map[string]any, error) {
	progress(0.9, "placing restored files")
	n, warns, err := tgt.publish()
	if err != nil {
		if stop := a.stopOnDestinationChange("restore", err, n); stop != nil {
			return nil, stop
		}
		return nil, err
	}
	warnings = append(warnings, warns...)
	progress(1.0, "restored")
	a.Store.Log("restore", fmt.Sprintf("%s -> %s: %d file(s) (repaired=%v)", c.Name, outputDir, n, repaired))
	res := restoreResult(c, outputDir, repaired, warnings)
	res["files"] = n
	return res, nil
}

// restoreResult builds the restore job's result map, attaching any non-fatal
// warnings (e.g. a par2 set that couldn't be placed beside a rejoined payload) so
// the operator sees them on the job — never a silently incomplete restore.
func restoreResult(c *Chunk, outputDir string, repaired bool, warnings []string) map[string]any {
	res := map[string]any{"chunk": c.Name, "repaired": repaired, "output": outputDir}
	if len(warnings) > 0 {
		res["warnings"] = warnings
	}
	return res
}

// tail keeps the last n bytes of helper output, cut on a character boundary and
// cleaned for storage (see cleanToolText).
func tail(s string, n int) string {
	if len(s) > n {
		s = s[len(s)-n:]
		for len(s) > 0 && !utf8.RuneStart(s[0]) {
			s = s[1:]
		}
	}
	return cleanToolText(s, n)
}

// copyFile copies src to dst through a new staging file beside dst, created
// exclusively and renamed to dst once flushed, so a file already at dst is
// replaced as a name and never written through.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := createStage(dst, 0o644)
	if err != nil {
		return err
	}
	tmp := out.Name()
	_, err = io.Copy(out, in)
	if err == nil {
		// Same rule as the payload stream: a sidecar (par2 block, manifest,
		// RESTORE.txt) whose final flush fails is a sidecar that is not on the
		// medium, and the caller must hear about it rather than read a nil error.
		err = finalizeWrite(out, dst)
	}
	if err == nil {
		err = renameReplacing(tmp, dst)
	}
	if err != nil {
		out.Close()
		_ = os.Remove(tmp)
	}
	return err
}

// driveSerialAt is the serial of the drive holding path, or "" when it cannot
// be found. A variable so tests need not probe disks.
var driveSerialAt = func(path string) string {
	if id, err := resolveDeviceIdentity(path); err == nil {
		return id.Serial
	}
	return ""
}

// destFree is the free space at a write's destination. A variable so a test can
// make a medium full.
var destFree = diskFree
