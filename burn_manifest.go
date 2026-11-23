package main

// burn_manifest.go — a burn copies its whole folder onto write-once media, so a
// private package's plaintext manifest is moved out first, and no other package's
// plaintext may be in there.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// hideBurnManifest makes c's staged folder safe to burn. putBack returns a moved
// manifest afterward. An error means the burn must not run.
func hideBurnManifest(c *Chunk) (putBack func() error, err error) {
	putBack = func() error { return nil }
	dir := filepath.Clean(c.StagedDir)
	parent := filepath.Dir(dir)
	if parent == dir {
		return nil, fmt.Errorf("%s is a drive root, so there is no place outside it for the private manifest", dir)
	}
	plain := filepath.Join(dir, c.Name+".manifest.json")
	hidden := filepath.Join(parent, "."+c.Name+".manifest.hidden")
	if c.PrivateManifest {
		// A burn that crashed left it aside. If a newer build made another, the move
		// below replaces the old copy.
		if _, err := os.Stat(hidden); err == nil {
			if _, perr := os.Stat(plain); errors.Is(perr, fs.ErrNotExist) {
				if err := moveManifestBack(hidden, plain); err != nil {
					return nil, fmt.Errorf("the manifest left by an earlier burn could not be put back: %w", err)
				}
			} else if perr != nil {
				return nil, perr
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	if err := refuseForeignManifests(dir, plain); err != nil {
		return nil, err
	}
	if !c.PrivateManifest {
		return putBack, nil
	}
	if _, err := os.Stat(plain); errors.Is(err, fs.ErrNotExist) {
		return putBack, nil // no plaintext manifest to keep off the disc
	} else if err != nil {
		return nil, err
	}
	if err := moveManifestAside(plain, hidden); err != nil {
		return nil, fmt.Errorf("the private manifest could not be moved out of the disc's folder: %w", err)
	}
	return func() error { return moveManifestBack(hidden, plain) }, nil
}

// refuseForeignManifests finds any plaintext manifest in dir other than own. A folder
// that cannot be read fully is refused, since what it holds would be burned.
func refuseForeignManifests(dir, own string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".manifest.hidden") ||
			(strings.HasSuffix(name, ".manifest.json") && p != own) {
			return fmt.Errorf("%s holds another package's plaintext manifest (%s), which would go on the disc. Move it out, or set the staging folder somewhere outside this package", dir, p)
		}
		return nil
	})
}
