//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const sysClassBlock = "/sys/class/block"

// blockEntryOf is the sysfs entry of the block device holding folder dir. ok is false
// when dir is on no block device (tmpfs, overlay, a network share).
func blockEntryOf(dir string) (string, bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(dir, &st); err != nil {
		return "", false
	}
	dev := uint64(st.Dev)
	major := (dev>>8)&0xfff | (dev>>32)&^0xfff
	minor := dev&0xff | (dev>>12)&^0xff
	entry, err := filepath.EvalSymlinks(fmt.Sprintf("/sys/dev/block/%d:%d", major, minor))
	if err != nil {
		return "", false
	}
	return entry, true
}

// physicalDiskOf names the whole disk holding folder dir ("disk:<major:minor>"),
// from sysfs: a partition's entry sits inside its disk's. ok is false when dir
// is on no block device (tmpfs, overlay, a network share).
func physicalDiskOf(dir string) (string, bool) {
	entry, ok := blockEntryOf(dir)
	if !ok {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(entry, "partition")); err == nil {
		entry = filepath.Dir(entry)
	}
	b, err := os.ReadFile(filepath.Join(entry, "dev"))
	if err != nil {
		return "", false
	}
	return "disk:" + strings.TrimSpace(string(b)), true
}

// physicalDisksOf names every whole disk under folder dir, following LVM, RAID and
// encryption layers. ok is false when they cannot be read.
func physicalDisksOf(dir string) ([]string, bool) {
	name := blockDeviceName(dir)
	if name == "" {
		return nil, false
	}
	disks, err := leafDisks(sysClassBlock, name, sysfsWholeDisk)
	if err != nil || len(disks) == 0 {
		return nil, false
	}
	return disks, true
}

// diskLookupSupported: a keystore's disks are read here.
const diskLookupSupported = true

// blockDeviceName is the kernel name of the block device holding dir, or "". A Btrfs
// subvolume reports a device number with no block device, so its mount is read.
func blockDeviceName(dir string) string {
	if entry, ok := blockEntryOf(dir); ok {
		return filepath.Base(entry)
	}
	b, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	name := mountSourceFrom(string(b), abs)
	if name == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(sysClassBlock, name)); err == nil {
		return name
	}
	if real, err := filepath.EvalSymlinks(filepath.Join("/dev/mapper", name)); err == nil {
		return filepath.Base(real)
	}
	return ""
}

// pathOnNetwork reports whether dir is on a network filesystem.
func pathOnNetwork(dir string) bool {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return false
	}
	switch uint32(st.Type) {
	case 0x6969, 0xFF534D42, 0xFE534D42, 0x517B: // NFS, CIFS, SMB2, SMB
		return true
	}
	return false
}

// sysfsWholeDisk is the disk a partition belongs to, or "" for anything else.
func sysfsWholeDisk(name string) string {
	entry := filepath.Join(sysClassBlock, name)
	if _, err := os.Stat(filepath.Join(entry, "partition")); err != nil {
		return ""
	}
	real, err := filepath.EvalSymlinks(entry)
	if err != nil {
		return ""
	}
	return filepath.Base(filepath.Dir(real))
}
