package main

// burn_manifest.go — what stays off the medium. A burn copies its whole folder onto
// write-once media, so files that are only for staging are moved out first and put
// back afterward, and nothing from another package may be in there.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// stagedOnlyFile reports whether a file in a staged folder stays off the medium: the
// file list, the plaintext tar beside an encrypted payload, and on private media
// everything that names the files. Restoring needs none of them: par2 covers the
// payload, and the file list is inside the tar and in the encrypted manifest.
func stagedOnlyFile(c *Chunk, name string) bool {
	switch name {
	case "filelist.txt":
		return true
	case c.Name + ".tar":
		return name != payloadName(c)
	}
	if !c.PrivateManifest {
		return false
	}
	return name == c.Name+".manifest.json" || isBagItTagFile(name)
}

// burnHoldDir is where a burn keeps staged-only files, beside the burned folder: same
// volume, never inside it. Named by a hash, as a package name need not suit a path.
func burnHoldDir(c *Chunk, parent string) string {
	return filepath.Join(parent, ".obelisk-burn-hold-"+sha256Hex([]byte(c.Name))[:16])
}

// holdStagedOnlyFiles makes c's staged folder safe to burn. putBack returns what was
// moved. An error means the burn must not run.
func holdStagedOnlyFiles(c *Chunk) (putBack func() error, err error) {
	dir := filepath.Clean(c.StagedDir)
	parent := filepath.Dir(dir)
	if parent == dir {
		return nil, fmt.Errorf("%s is a drive root, so there is no place outside it for files that must stay off the disc", dir)
	}
	hold := burnHoldDir(c, parent)
	if err := recoverBurnHold(c, dir, parent, hold); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		lower := strings.ToLower(e.Name())
		switch {
		case !e.IsDir() && stagedOnlyFile(c, e.Name()):
			names = append(names, e.Name())
		case e.IsDir(), strings.HasSuffix(lower, ".manifest.hidden"),
			strings.HasSuffix(lower, ".manifest.json") && e.Name() != c.Name+".manifest.json":
			return nil, fmt.Errorf("%s holds %s, which is not part of this package and would go on the disc. Move it out, or set the staging folder somewhere outside this package", dir, e.Name())
		}
	}
	var moved []string
	putBack = func() error {
		var errs []error
		for _, n := range moved {
			if err := moveManifestBack(filepath.Join(hold, n), filepath.Join(dir, n)); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", n, err))
			}
		}
		if len(errs) == 0 && len(moved) > 0 {
			_ = removeEmptyOwnFolder(parent, filepath.Base(hold), kindBurnHold) // only ever removes an empty folder
		}
		return errors.Join(errs...)
	}
	if len(names) == 0 {
		return putBack, nil
	}
	if err := os.MkdirAll(hold, 0o700); err != nil {
		return nil, fmt.Errorf("the folder for files that must stay off the disc could not be made: %w", err)
	}
	for _, n := range names {
		if err := moveManifestAside(filepath.Join(dir, n), filepath.Join(hold, n)); err != nil {
			if perr := putBack(); perr != nil {
				err = errors.Join(err, perr)
			}
			return nil, fmt.Errorf("%s could not be moved out of the disc's folder: %w", n, err)
		}
		moved = append(moved, n)
	}
	return putBack, nil
}

// recoverBurnHold puts back what a crashed burn left aside. A file that a newer build
// has made again stays in the hold: the burn then refuses to move the new one over
// it, and the hold is listed to the user as a leftover to review.
func recoverBurnHold(c *Chunk, dir, parent, hold string) error {
	// Earlier versions kept the private manifest here.
	if c.PrivateManifest {
		old := filepath.Join(parent, "."+c.Name+".manifest.hidden")
		if _, err := os.Stat(old); err == nil {
			if err := os.MkdirAll(hold, 0o700); err != nil {
				return err
			}
			if err := renameNew(old, filepath.Join(hold, c.Name+".manifest.json")); err != nil {
				return fmt.Errorf("the manifest left by an earlier burn could not be recovered: %w", err)
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	entries, err := os.ReadDir(hold)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	left := false
	for _, e := range entries {
		target := filepath.Join(dir, e.Name())
		_, serr := os.Stat(target)
		switch {
		case errors.Is(serr, fs.ErrNotExist):
			if err := moveManifestBack(filepath.Join(hold, e.Name()), target); err != nil {
				return fmt.Errorf("%s, left by an earlier burn, could not be put back: %w", e.Name(), err)
			}
		case serr != nil:
			return serr
		default:
			left = true
		}
	}
	if !left {
		_ = removeEmptyOwnFolder(parent, filepath.Base(hold), kindBurnHold) // only ever removes an empty folder
	}
	return nil
}
