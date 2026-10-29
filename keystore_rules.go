package main

// keystore_rules.go — keystores must be separate files on separate devices.
//
// Keys are kept in at least two keystores so that losing one device loses no
// key. Two entries that are the same file (the same path, a letter-case variant
// of it, or another name for the same file) or two keystores on one device do
// not give that, so such a set is refused.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// keystoreDeviceOf names the device a keystore path is on. A variable so tests,
// whose keystores are all on one temp drive, can treat folders as devices.
var keystoreDeviceOf = realKeystoreDevice

// realKeystoreDevice is the device of the nearest existing folder holding path:
// the physical disk under its volume where that can be read (so two partitions
// of one disk are one device), otherwise the volume, read from a handle on it.
func realKeystoreDevice(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, _, err := deepestExistingDir(abs)
	if err != nil {
		return "", err
	}
	if disk, ok := physicalDiskOf(dir); ok {
		return disk, nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return "", err
	}
	defer f.Close()
	id, err := handleIdentity(f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", id.vol), nil
}

// checkKeystorePaths refuses a keystore set with the same file twice (the same
// path, a letter-case variant, or an existing file reached by two names) or two
// keystores on one device.
func checkKeystorePaths(paths []string) error {
	seenName := map[string]string{}
	seenFile := map[fileIdent]string{}
	seenDev := map[string]string{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("keystore_paths: %q: %w", p, err)
		}
		key := strings.ToLower(filepath.Clean(abs))
		if prev, ok := seenName[key]; ok {
			return fmt.Errorf("keystore_paths: %q and %q are the same keystore; each keystore must be a separate file", prev, p)
		}
		seenName[key] = p
		if f, err := os.Open(abs); err == nil {
			id, idErr := handleIdentity(f)
			f.Close()
			if idErr == nil && id.idx != 0 {
				if prev, ok := seenFile[id]; ok {
					return fmt.Errorf("keystore_paths: %q and %q are the same file; each keystore must be a separate file", prev, p)
				}
				seenFile[id] = p
			}
		}
		dev, err := keystoreDeviceOf(abs)
		if err != nil {
			// Not plugged in, or not reachable: its device cannot be told, so the
			// device rule cannot be checked for it. Reachability is reported by
			// the keystore status, not refused here.
			continue
		}
		if prev, ok := seenDev[dev]; ok {
			return fmt.Errorf("keystore_paths: %q and %q are on the same device; keep each keystore on a different device, so losing one loses no key", prev, p)
		}
		seenDev[dev] = p
	}
	return nil
}
