package main

// physdisk_leaves.go — the physical disks under a Linux block device. Kept free of
// build tags so it is tested on every platform against a fixture tree.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
