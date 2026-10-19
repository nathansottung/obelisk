//go:build linux

package main

// destauth_linux.go — handle-derived location and identity on Linux.
//
// The location comes from /proc/self/fd/<n>, which the kernel resolves for the
// OPEN DESCRIPTOR, not for the pathname that opened it — so a symlinked or
// bind-visible spelling reduces to the real location. Identity comes from fstat
// on the same descriptor.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// handleCanonicalPath returns where the object behind f actually lives.
func handleCanonicalPath(f *os.File) (string, error) {
	p, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
	if err != nil {
		return "", fmt.Errorf("cannot resolve /proc/self/fd for %s: %w", f.Name(), err)
	}
	// A descriptor whose object has been unlinked reads back with a " (deleted)"
	// suffix. That is an unresolved location, not a location.
	if len(p) == 0 || p[0] != '/' {
		return "", fmt.Errorf("cannot resolve a usable location for %s (got %q)", f.Name(), p)
	}
	if strings.HasSuffix(p, " (deleted)") {
		return "", fmt.Errorf("the folder behind %s has been removed", f.Name())
	}
	return filepath.Clean(p), nil
}

// fileIdent is a filesystem object's identity, taken from an OPEN DESCRIPTOR.
type fileIdent struct {
	vol uint64
	idx uint64
}

// handleIdentity reports the identity of the object behind f, via fstat on the
// descriptor — device plus inode, forced at the moment of the call.
func handleIdentity(f *os.File) (fileIdent, error) {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		return fileIdent{}, fmt.Errorf("fstat failed for %s: %w", f.Name(), err)
	}
	return fileIdent{vol: uint64(st.Dev), idx: uint64(st.Ino)}, nil
}

// volumeIDOf names the filesystem holding f by its UUID, found by matching the
// descriptor's device number against the block devices in /dev/disk/by-uuid.
// st_dev alone is not used: a removable drive can get a different device number
// each time it is attached. Filesystems without a block device there (Btrfs
// subvolumes, tmpfs, network mounts) have no recorded identity and return "".
func volumeIDOf(f *os.File) (string, error) {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		return "", fmt.Errorf("fstat failed for %s: %w", f.Name(), err)
	}
	ents, err := os.ReadDir("/dev/disk/by-uuid")
	if err != nil {
		return "", nil
	}
	for _, e := range ents {
		var dev syscall.Stat_t
		if syscall.Stat(filepath.Join("/dev/disk/by-uuid", e.Name()), &dev) != nil {
			continue
		}
		if dev.Mode&syscall.S_IFMT == syscall.S_IFBLK && uint64(dev.Rdev) == uint64(st.Dev) {
			return "fs-uuid:" + e.Name(), nil
		}
	}
	return "", nil
}
