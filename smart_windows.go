//go:build windows

package main

import "fmt"

// smartDeviceNode maps a path to the smartctl device argument on Windows: the one
// physical disk of the volume holding the path, as /dev/pd<N> (smartmontools'
// physical-drive syntax). A drive mounted in a folder is its own volume, so the
// path's drive letter is not used. Read-only.
func smartDeviceNode(path string) (string, error) {
	n, err := volumeDiskNumber(path)
	if err != nil {
		return "", fmt.Errorf("resolving the disk of %s: %w", path, err)
	}
	return fmt.Sprintf("/dev/pd%d", n), nil
}
