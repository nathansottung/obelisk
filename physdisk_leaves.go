package main

// physdisk_leaves.go — the physical disks under a Linux block device. Kept free of
// build tags so it is tested on every platform against a fixture tree.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// mountSourceFrom reads /proc/self/mountinfo text and returns the device name (no
// /dev/) of the longest mount holding abs, or "" when that mount is not on a device.
func mountSourceFrom(info, abs string) string {
	best, name := -1, ""
	for _, line := range strings.Split(info, "\n") {
		left, right, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		l, r := strings.Fields(left), strings.Fields(right)
		if len(l) < 5 || len(r) < 2 {
			continue
		}
		mp := strings.ReplaceAll(l[4], `\040`, " ")
		if !mountMatches(mp, abs) || len(mp) <= best {
			continue
		}
		best, name = len(mp), ""
		if strings.HasPrefix(r[1], "/dev/") {
			name = strings.TrimPrefix(r[1][strings.LastIndex(r[1], "/"):], "/")
		}
	}
	return name
}

// sysFsBtrfs lists each mounted Btrfs volume and the devices under it.
const sysFsBtrfs = "/sys/fs/btrfs"

// btrfsMembers names every device of the Btrfs volume that device name belongs to,
// sorted, or nil when it belongs to none. mountinfo names only one of them.
func btrfsMembers(root, name string) []string {
	volumes, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, v := range volumes {
		devices, err := os.ReadDir(filepath.Join(root, v.Name(), "devices"))
		if err != nil {
			continue
		}
		names := make([]string, 0, len(devices))
		found := false
		for _, d := range devices {
			names = append(names, d.Name())
			found = found || d.Name() == name
		}
		if found {
			return names
		}
	}
	return nil
}

// leafDisks walks /sys/class/block (root) from device name down through its "slaves"
// (LVM, RAID, encryption) to the whole disks underneath, as "disk:<major:minor>".
// wholeDisk names the disk a partition belongs to, or "" when name is not a partition.
// A loop or a device that cannot be read is an error: the disks are then unknown.
func leafDisks(root, name string, wholeDisk func(string) string) ([]string, error) {
	seen := map[string]bool{}
	onPath := map[string]bool{}
	var out []string
	var walk func(string) error
	walk = func(dev string) error {
		if onPath[dev] {
			return fmt.Errorf("block device %s is built on itself", dev)
		}
		if p := wholeDisk(dev); p != "" {
			dev = p
		}
		if seen[dev] {
			return nil
		}
		onPath[dev] = true
		defer delete(onPath, dev)
		entries, err := os.ReadDir(filepath.Join(root, dev, "slaves"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if len(entries) == 0 {
			b, err := os.ReadFile(filepath.Join(root, dev, "dev"))
			if err != nil {
				return err
			}
			seen[dev] = true
			out = append(out, "disk:"+strings.TrimSpace(string(b)))
			return nil
		}
		for _, e := range entries {
			if err := walk(e.Name()); err != nil {
				return err
			}
		}
		seen[dev] = true
		return nil
	}
	if err := walk(name); err != nil {
		return nil, err
	}
	return out, nil
}
