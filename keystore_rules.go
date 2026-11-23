package main

// keystore_rules.go — keystores must be separate files on separate devices.
//
// Keys are kept in at least two keystores so that losing one device loses no
// key. Two entries that are the same file (the same path, a letter-case variant
// of it, or another name for the same file) or two keystores on one device do
// not give that, so such a set is refused.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// keystoreDeviceOf names the device a keystore path is on. A variable so tests,
// whose keystores are all on one temp drive, can treat folders as devices.
var keystoreDeviceOf = realKeystoreDevice

// keystoreDisks and keystoreOnNetwork read the disks under a folder and whether it is
// on a network share. Variables so tests can present a failed lookup.
var (
	keystoreDisks     = physicalDisksOf
	keystoreOnNetwork = pathOnNetwork
	keystoreVolumeOf  = realKeystoreVolume
)

// errKeystoreDiskUnknown: disks can be read on this system, but not this one's.
var errKeystoreDiskUnknown = errors.New("its disk could not be read, so Obelisk cannot show it is separate from the other keystores")

// realKeystoreDevice is the device of the nearest existing folder holding path:
// the physical disks under its volume (so two partitions of one disk are one device).
// A volume across several disks names each one, joined by deviceSeparator. Where
// disks can be read and this one could not be, that is an error: a failed lookup is
// not proof of a separate device. A network share has no local disk and is placed by
// its volume, as is everything on a system whose disks are not read.
func realKeystoreDevice(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// A keystore that exists is a file: it is on its folder's device.
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}
	dir, _, err := deepestExistingDir(abs)
	if err != nil {
		return "", err
	}
	if disks, ok := keystoreDisks(dir); ok {
		return strings.Join(disks, deviceSeparator), nil
	}
	if diskLookupSupported && !keystoreOnNetwork(dir) {
		return "", errKeystoreDiskUnknown
	}
	return volumeOfFolder(dir)
}

// realKeystoreVolume is the volume of the nearest existing folder holding path.
func realKeystoreVolume(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(abs); err == nil && !fi.IsDir() {
		abs = filepath.Dir(abs)
	}
	dir, _, err := deepestExistingDir(abs)
	if err != nil {
		return "", err
	}
	return volumeOfFolder(dir)
}

func volumeOfFolder(dir string) (string, error) {
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
	_, err := checkKeystoreSet(paths, false)
	return err
}

// checkKeystoresInUse is checkKeystorePaths for a set already configured. A keystore
// whose disk cannot be read is placed by its volume and named in a warning, so a
// working set does not lose encryption because a driver does not report its disks.
func checkKeystoresInUse(paths []string) (warning string, err error) {
	return checkKeystoreSet(paths, true)
}

func checkKeystoreSet(paths []string, inUse bool) (string, error) {
	var unread []string
	seenName := map[string]string{}
	seenFile := map[fileIdent]string{}
	seenDev := map[string]string{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("keystore_paths: %q: %w", p, err)
		}
		key := strings.ToLower(filepath.Clean(abs))
		if prev, ok := seenName[key]; ok {
			return "", fmt.Errorf("keystore_paths: %q and %q are the same keystore; each keystore must be a separate file", prev, p)
		}
		seenName[key] = p
		if f, err := os.Open(abs); err == nil {
			id, idErr := handleIdentity(f)
			f.Close()
			if idErr == nil && id.idx != 0 {
				if prev, ok := seenFile[id]; ok {
					return "", fmt.Errorf("keystore_paths: %q and %q are the same file; each keystore must be a separate file", prev, p)
				}
				seenFile[id] = p
			}
		}
		dev, err := keystoreDeviceOf(abs)
		if errors.Is(err, errNoExistingParent) {
			// Nothing on its path exists (its drive is not plugged in): its device
			// cannot be told, so it is placed by its drive letter or share.
			// Reachability is reported by the keystore status, not refused here.
			dev = "root:" + strings.ToLower(keystoreRootOf(abs))
			err = nil
		}
		if inUse && errors.Is(err, errKeystoreDiskUnknown) {
			if v, verr := keystoreVolumeOf(abs); verr == nil {
				dev, err = "volume:"+v, nil
				unread = append(unread, p)
			}
		}
		if err != nil {
			return "", fmt.Errorf("keystore_paths: cannot tell which device %q is on: %w", p, err)
		}
		// Sharing any one disk means losing that disk loses both.
		parts := strings.Split(dev, deviceSeparator)
		for _, d := range parts {
			if prev, ok := seenDev[d]; ok {
				return "", fmt.Errorf("keystore_paths: %q and %q are on the same device; keep each keystore on a different device, so losing one loses no key", prev, p)
			}
		}
		for _, d := range parts {
			seenDev[d] = p
		}
	}
	if len(unread) > 0 {
		return fmt.Sprintf("The disk under %s could not be read, so Obelisk can only show these keystores are on different volumes, not different physical disks. Check that they are, or move them to drives Obelisk can read.", strings.Join(unread, ", ")), nil
	}
	return "", nil
}

// deviceSeparator joins the disks of a volume that lies on more than one.
const deviceSeparator = "\x00"

// keystoreRootOf is the drive a path is under, for a keystore whose device
// cannot be read: its volume name (a drive letter or share), or its top folder
// where paths have none.
func keystoreRootOf(abs string) string {
	if v := filepath.VolumeName(abs); v != "" {
		return v
	}
	rest := strings.TrimLeft(filepath.ToSlash(abs), "/")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return "/" + rest
}
