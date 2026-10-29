//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// physicalDiskOf names the whole disk holding folder dir ("disk:<major:minor>"),
// from sysfs: a partition's entry sits inside its disk's. ok is false when dir
// is on no block device (tmpfs, overlay, a network share).
func physicalDiskOf(dir string) (string, bool) {
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
	if _, err := os.Stat(filepath.Join(entry, "partition")); err == nil {
		entry = filepath.Dir(entry)
	}
	b, err := os.ReadFile(filepath.Join(entry, "dev"))
	if err != nil {
		return "", false
	}
	return "disk:" + strings.TrimSpace(string(b)), true
}
