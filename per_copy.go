package main

// per_copy.go — each copy on a medium keeps the key and hashes it was written with.
// A rebuild gives the package new values and leaves every copy alone, so old copies
// keep verifying and restoring. This file holds the app-level work that needs those
// values: finding which values a payload has, the par2 repairability check, and
// refilling staging from a verified copy.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// writeAfterPayloadHook is a test seam: a failure after a write's payload is on the
// medium and before its sidecars are.
var writeAfterPayloadHook func() error

// checkCopySidecarsAt reads back the sidecars recorded for one copy beside a payload
// read from dir on a medium. It checks only copies with recorded hashes, and only in
// a folder of the package's own name, where RESTORE.txt is its own.
func checkCopySidecarsAt(c *Chunk, sidecars map[string]string, dir string) error {
	if len(sidecars) == 0 || filepath.Base(dir) != c.Name {
		return nil
	}
	names := make([]string, 0, len(sidecars))
	for name := range sidecars {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h, err := hashFileHex(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("sidecar %s is missing on the medium", name)
		}
		if h != sidecars[name] {
			return fmt.Errorf("sidecar %s on the medium does not match the build", name)
		}
	}
	return nil
}

// valuesForPayload finds the recorded values of the payload at path. With one set on
// record no read is needed unless verifyBytes is set.
func valuesForPayload(c *Chunk, path string, verifyBytes bool) (CopyValues, error) {
	all := recordedValues(c)
	if len(all) == 1 && !verifyBytes {
		return all[0], nil
	}
	h, err := hashFileHex(path)
	if err != nil {
		return CopyValues{}, err
	}
	for _, v := range all {
		if v.PayloadHash == h {
			return v, nil
		}
	}
	return CopyValues{}, fmt.Errorf("its hash matches no copy on record")
}

// par2CanRepair says whether the par2 set beside a damaged payload can repair it.
// It only reads: par2's verify exit code 1 means damaged and repairable.
func (a *App) par2CanRepair(payload string) bool {
	par2File := payload + ".par2"
	if _, err := os.Stat(par2File); err != nil {
		return false
	}
	bin, err := a.tool("par2")
	if err != nil {
		return false
	}
	var exit *exec.ExitError
	return errors.As(helperCommand(bin, "verify", par2File).Run(), &exit) && exit.ExitCode() == 1
}

// priorBuildAt records which build a package folder on a medium holds, before a write
// to it. The RESTORE.txt says so cheaply; only when it names no build on record are
// the payload or segment files read.
func priorBuildAt(c *Chunk, dir string) priorBuild {
	p := priorBuild{Note: restoreNoteHash(dir)}
	id, state := readCopyID(dir)
	if state == "ok" {
		p.CopyID = id
	}
	payload := payloadPathIn(dir, c)
	// Read by name, not by pattern: a mount path may hold pattern characters.
	var segs []string
	par2s := 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, c.Name) {
			continue
		}
		if strings.HasPrefix(n, c.Name+".seg") {
			segs = append(segs, filepath.Join(dir, n))
		}
		if strings.HasSuffix(n, ".par2") {
			par2s++
		}
	}
	p.Present = p.Note != "" || payload != "" || len(segs) > 0 || par2s > 0 || state != idMissing
	for _, v := range recordedValues(c) {
		if p.Note != "" && v.Sidecars["RESTORE.txt"] == p.Note {
			return p
		}
	}
	if payload != "" {
		if h, err := hashFileHex(payload); err == nil {
			p.PayloadHash = h
		}
	}
	for _, sp := range segs {
		if h, err := hashFileHex(sp); err == nil {
			if p.SegHashes == nil {
				p.SegHashes = map[string]bool{}
			}
			p.SegHashes[h] = true
		}
	}
	return p
}

// goneCopyFolders is each folder of the package's current copies that is not there
// now (normPath). It reads drives, so it runs outside the catalog lock.
func (a *App) goneCopyFolders(c *Chunk) map[string]bool {
	gone := map[string]bool{}
	for _, p := range a.Store.copyFolders(c) {
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			gone[normPath(p)] = true
		}
	}
	return gone
}

// identifyRead fills in what the copy ID file in the folder a check read holds, and
// whether the copy recorded with that ID, somewhere else, is there too.
func (a *App) identifyRead(c *Chunk, at *CopyAt) {
	at.CopyID, at.IDState = copyIDAt(c, at.Path)
	if at.IDState != "ok" {
		return
	}
	if p := a.Store.copyPathWithID(c, at.CopyID); p != "" && normPath(p) != normPath(at.Path) {
		id, state := readCopyID(p)
		at.RecordedHolds = state == "ok" && id == at.CopyID
		if at.RecordedHolds {
			at.RecordedSerial, at.RecordedPartition = driveSerialAt(p), partitionIDAt(p)
		}
	}
}

// keysCopiesNeedMissing names each key a copy on record needs that no configured
// keystore holds.
func (a *App) keysCopiesNeedMissing() []string {
	cfg, err := a.LoadConfig()
	if err != nil {
		return nil
	}
	held := map[string]bool{}
	for _, p := range cfg.KeystorePaths {
		ks, err := readStore(p)
		if err != nil {
			continue
		}
		for _, k := range ks.Keys {
			if ref, ok := k["key_ref"].(string); ok {
				held[ref] = true
			}
		}
	}
	var missing []string
	for _, ref := range a.Store.CopyKeyRefs() {
		if !held[ref] {
			missing = append(missing, ref)
		}
	}
	return missing
}

// stagingIntact is whether the package's staging holds its current build unchanged.
func (a *App) stagingIntact(c *Chunk) bool {
	if chunkStagedForWrite(c) != nil {
		return false
	}
	h, err := hashFileHex(payloadPathIn(c.StagedDir, c))
	return err == nil && h == c.EncHash
}

// RefillStaging restores a package's staging from a verified copy on volumeID, for
// when staging is damaged or gone. sourceDir overrides where the copy is read, and
// is required for a spanned copy (every tape's files in one folder or in one
// subfolder each). The refill is built beside staging, checked against the copy's
// own hashes, and only then takes staging's place: a refill that fails changes
// nothing. The package then describes the copy that was read.
func (a *App) RefillStaging(id, volumeID int, sourceDir string, progress func(float64, string), opts ...PackageWriteOptions) (map[string]any, error) {
	cfg, err := a.LoadConfig()
	if err != nil {
		return nil, err
	}
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", id)
	}
	release, err := a.Store.claimWork(packageWorkKey(id), fmt.Sprintf("package %s is being built, written or burned; wait for that job to finish", c.Name))
	if err != nil {
		return nil, err
	}
	defer release()
	if a.stagingIntact(c) {
		return nil, fmt.Errorf("the staging of %s is intact; nothing to refill", c.Name)
	}
	cp, ok := a.Store.verifiedCopyOn(c, volumeID)
	if !ok {
		return nil, fmt.Errorf("%s has no verified copy on volume %d to refill from", c.Name, volumeID)
	}
	vals := c.valuesOf(&cp)
	if c.PrivateManifest && len(vals.Sidecars) > 0 {
		private := false
		for name := range vals.Sidecars {
			private = private || strings.HasSuffix(name, ".manifest.json.gpg")
		}
		if !private {
			return nil, fmt.Errorf("this copy of %s carries a plain file list and the package is private, so writing from it would put file names on a new medium; rebuild the package instead", c.Name)
		}
	}
	src := sourceDir
	if src == "" {
		if len(cp.Segments) > 0 {
			return nil, fmt.Errorf("%s is spanned: copy every tape's files into one folder and give it as the source", c.Name)
		}
		src = cp.Path
	}
	if cfg.StagingDir == "" {
		return nil, fmt.Errorf("staging_dir is not configured (Settings)")
	}
	if err := a.Store.CheckOutsideSources(cfg.StagingDir); err != nil {
		return nil, err
	}
	// The refill puts the encrypted package in staging, as a build does.
	keyNotice, err := a.checkKeyPlacement(c, cfg.StagingDir, cfg.KeystorePaths, packageWriteOptions(opts).AllowKeyDrive)
	if err != nil {
		return nil, err
	}
	if keyNotice != "" {
		progress(0.01, keyNotice)
	}
	releaseStaging, err := a.Store.reserveOutputPath(cfg.StagingDir)
	if err != nil {
		return nil, err
	}
	defer releaseStaging()
	if err := proveWriteAccess(cfg.StagingDir, "the staging folder"); err != nil {
		return nil, err
	}
	work := filepath.Join(cfg.StagingDir, c.Name)
	if rel, err := filepath.Rel(cfg.StagingDir, work); err != nil || rel != filepath.Base(work) || rel == "." || rel == ".." {
		return nil, fmt.Errorf("package name %q does not make a folder inside staging", c.Name)
	}
	stage, err := os.MkdirTemp(cfg.StagingDir, "."+c.Name+".refill-*")
	if err != nil {
		return nil, err
	}
	stageMade, err := madeDir(stage)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stageMade.Remove() }() // the refill folder, by identity, if it was not put in place

	view := c.withSegments(cp.Segments) // the package as this copy recorded it
	progress(0.05, "reading the copy")
	var payload string
	if len(cp.Segments) > 0 {
		if payload, _, err = rejoinSegments(src, stage, view, progress); err != nil {
			return nil, err
		}
		if err := copyNamedFrom(src, stage, vals.Sidecars); err != nil {
			return nil, err
		}
	} else {
		from := findPayload(src, c)
		if from == "" {
			return nil, fmt.Errorf("payload for %s not found under %s", c.Name, src)
		}
		// A package's own folder holds only its files. At a medium's root, other
		// packages sit beside it, so only this package's files are taken.
		if dir := filepath.Dir(from); filepath.Base(dir) == c.Name {
			if err := copyFolderInto(dir, stage); err != nil {
				return nil, err
			}
			// Staging is not a copy: the next write gives its copy a new ID.
			if err := removeOwnedByID(nil, []fileIdent{stageMade.id}, stage, copyIDFile, kindCopyID); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		} else if err := copyOwnFiles(c, dir, stage, filepath.Base(from), vals.Sidecars); err != nil {
			return nil, err
		}
		payload = filepath.Join(stage, filepath.Base(from))
	}
	// A refill that could not be written from is no refill.
	for _, name := range requiredSidecars(view) {
		if _, err := os.Stat(filepath.Join(stage, name)); err != nil {
			return nil, fmt.Errorf("the copy at %s has no %s of its own for %s; staging was not changed", src, name, c.Name)
		}
	}
	progress(0.75, "checking the refill against the copy's hash")
	h, err := hashFileHex(payload)
	if err != nil {
		return nil, err
	}
	if h != vals.PayloadHash {
		return nil, fmt.Errorf("the payload read from %s does not match that copy's hash; staging was not changed. Verify the copy, or refill from another", src)
	}
	for name, want := range vals.Sidecars {
		got, err := hashFileHex(filepath.Join(stage, name))
		if err != nil {
			return nil, fmt.Errorf("sidecar %s is missing from the copy; staging was not changed", name)
		}
		if got != want {
			return nil, fmt.Errorf("sidecar %s does not match that copy's hash; staging was not changed", name)
		}
	}
	st, err := os.Stat(payload)
	if err != nil {
		return nil, err
	}
	progress(0.9, "replacing staging")
	// The damaged staging is moved aside, not removed, until the refill is in
	// place and recorded; it is recorded by identity before it moves.
	var prev *madeEntry
	if fi, err := os.Lstat(work); err == nil {
		if isLinkMode(fi.Mode()) || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a folder Obelisk made; move it out of staging and refill again", work)
		}
		if prev, err = madeDir(work); err != nil {
			return nil, err
		}
		aside := stage + ".old"
		if err := renameNew(work, aside); err != nil {
			return nil, fmt.Errorf("moving the damaged staging aside: %w", err)
		}
		prev.Moved(aside)
	}
	if err := renameNew(stage, work); err != nil {
		if prev != nil {
			if rerr := renameNew(prev.Path(), work); rerr == nil {
				prev.Moved(work)
			}
		}
		return nil, fmt.Errorf("putting the refill in place: %w", err)
	}
	stageMade.Moved(work)
	stageMade = nil // in place: not removed
	if err := a.Store.RefilledFrom(c, vals, st.Size(), work, cp.Segments); err != nil {
		return nil, fmt.Errorf("the refill is in staging, but the catalog could not record it: %w", err)
	}
	if prev != nil {
		_ = prev.Remove() // the replaced staging, by identity, now the refill is recorded
	}
	if c.Status == "FAILED" {
		c.Status = "STAGED"
	}
	a.refreshChunkStatus(c)
	if err := a.Store.UpdateChunkErr(c); err != nil {
		return nil, err
	}
	a.Store.Log("refill", fmt.Sprintf("%s: staging refilled from the copy on volume %d", c.Name, volumeID))
	progress(1.0, "staging refilled")
	res := map[string]any{"chunk": c.Name, "staged_dir": work, "status": c.Status, "from_volume": volumeID}
	if keyNotice != "" {
		res["key_drive_notice"] = keyNotice
	}
	return res, nil
}

// copyFolderInto copies the plain files directly inside from into to.
func copyFolderInto(from, to string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if err := copyFile(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
			return fmt.Errorf("copying %s: %w", e.Name(), err)
		}
	}
	return nil
}

// noteIsFor is whether a RESTORE.txt's title names this package.
func noteIsFor(path, name string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(string(b), "\n")
	for _, word := range strings.Fields(first) {
		if word == name {
			return true
		}
	}
	return false
}

// copyOwnFiles copies a package's payload, its par2 set and its sidecars from a folder
// shared with other packages. With no recorded sidecar hashes (an adopted package),
// its manifest is known by name and its RESTORE.txt by the package named in its title.
func copyOwnFiles(c *Chunk, from, to, payload string, sidecars map[string]string) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		n := e.Name()
		if !e.Type().IsRegular() || (n != payload && !(strings.HasPrefix(n, payload+".") && strings.HasSuffix(n, ".par2"))) {
			continue
		}
		if err := copyFile(filepath.Join(from, n), filepath.Join(to, n)); err != nil {
			return fmt.Errorf("copying %s: %w", n, err)
		}
	}
	if len(sidecars) > 0 {
		return copyNamedFrom(from, to, sidecars)
	}
	for _, name := range requiredSidecars(c) {
		p := filepath.Join(from, name)
		if _, err := os.Stat(p); err != nil || (name == "RESTORE.txt" && !noteIsFor(p, c.Name)) {
			continue
		}
		if err := copyFile(p, filepath.Join(to, name)); err != nil {
			return fmt.Errorf("copying %s: %w", name, err)
		}
	}
	return nil
}

// copyNamedFrom copies each named sidecar from src, or from a subfolder of src (one
// per tape), into to.
func copyNamedFrom(src, to string, names map[string]string) error {
	for name := range names {
		from := filepath.Join(src, name)
		if _, err := os.Stat(from); err != nil {
			m := filesNamed(src, true, name, name)
			if len(m) == 0 {
				return fmt.Errorf("sidecar %s not found under %s", name, src)
			}
			sort.Strings(m)
			from = m[0]
		}
		if err := copyFile(from, filepath.Join(to, name)); err != nil {
			return fmt.Errorf("copying %s: %w", name, err)
		}
	}
	return nil
}
