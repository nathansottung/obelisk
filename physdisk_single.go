//go:build !windows && !linux

package main

// diskLookupSupported: disks are not read here, so keystores are placed by volume.
const diskLookupSupported = false

// blockDeviceName: not read on this system.
func blockDeviceName(string) string { return "" }

// pathOnNetwork: not read on this system; keystores are placed by volume either way.
func pathOnNetwork(string) bool { return false }

// physicalDisksOf: disks are not read on this system; keystores are placed by volume,
// so two volumes on one disk are not detected here.
func physicalDisksOf(dir string) ([]string, bool) {
	if d, ok := physicalDiskOf(dir); ok {
		return []string{d}, true
	}
	return nil, false
}
