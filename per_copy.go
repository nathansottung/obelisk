package main

// per_copy.go — each copy on a medium keeps the key and hashes it was written with.
// A rebuild gives the package new values and leaves every copy alone, so old copies
// keep verifying and restoring. This file holds the app-level work that needs those
// values: finding which values a payload has, the par2 repairability check, and
// refilling staging from a verified copy.

import (
	"errors"
	"fmt"
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

// recordedValues is every distinct set of values on record for this package: its
// current build first, then its copies.
func recordedValues(c *Chunk) []CopyValues {
	out := []CopyValues{{KeyRef: c.KeyRef, PayloadHash: c.EncHash, TarHash: c.TarHash, Sidecars: c.Sidecars}}
	seen := map[string]bool{c.EncHash: true}
	for i := range c.Copies {
		v := c.valuesOf(&c.Copies[i])
		if !seen[v.PayloadHash] {
			seen[v.PayloadHash] = true
			out = append(out, v)
		}
	}
	return out
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
func (a *App) RefillStaging(id, volumeID int, sourceDir string, progress func(float64, string)) (map[string]any, error) {
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
	defer os.RemoveAll(stage)

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
		if err := copyFolderInto(filepath.Dir(from), stage); err != nil {
			return nil, err
		}
		payload = filepath.Join(stage, filepath.Base(from))
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
	aside := ""
	if _, err := os.Lstat(work); err == nil {
		aside = stage + ".old"
		if err := renameReplacing(work, aside); err != nil {
			return nil, fmt.Errorf("moving the damaged staging aside: %w", err)
		}
	}
	if err := renameReplacing(stage, work); err != nil {
		if aside != "" {
			_ = renameReplacing(aside, work)
		}
		return nil, fmt.Errorf("putting the refill in place: %w", err)
	}
	if err := a.Store.RefilledFrom(c, vals, st.Size(), work, cp.Segments); err != nil {
		return nil, fmt.Errorf("the refill is in staging, but the catalog could not record it: %w", err)
	}
	if aside != "" {
		_ = os.RemoveAll(aside)
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
	return map[string]any{"chunk": c.Name, "staged_dir": work, "status": c.Status, "from_volume": volumeID}, nil
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

// copyNamedFrom copies each named sidecar from src, or from a subfolder of src (one
// per tape), into to.
func copyNamedFrom(src, to string, names map[string]string) error {
	for name := range names {
		from := filepath.Join(src, name)
		if _, err := os.Stat(from); err != nil {
			m, _ := filepath.Glob(filepath.Join(src, "*", name))
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
