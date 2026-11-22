//go:build !windows

package main

// physicalDisksOf names the disk holding folder dir. On these systems a folder's
// device is read as one whole disk.
func physicalDisksOf(dir string) ([]string, bool) {
	if d, ok := physicalDiskOf(dir); ok {
		return []string{d}, true
	}
	return nil, false
}
