//go:build !windows && !linux

package main

// physicalDisksOf: disks are not read on this system; keystores are placed by volume,
// so two volumes on one disk are not detected here.
func physicalDisksOf(dir string) ([]string, bool) {
	if d, ok := physicalDiskOf(dir); ok {
		return []string{d}, true
	}
	return nil, false
}
