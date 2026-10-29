//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// resolvePartitionID is the volume GUID of the volume holding path, as
// "{xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx}". It works for a volume mounted at a
// drive letter or in a folder. Read-only, no PowerShell.
func resolvePartitionID(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return "", err
	}
	mount := make([]uint16, windows.MAX_LONG_PATH)
	if err := windows.GetVolumePathName(p, &mount[0], uint32(len(mount))); err != nil {
		return "", fmt.Errorf("finding the volume of %s: %w", path, err)
	}
	name := make([]uint16, 64)
	if err := windows.GetVolumeNameForVolumeMountPoint(&mount[0], &name[0], uint32(len(name))); err != nil {
		return "", fmt.Errorf("reading the volume GUID of %s: %w", windows.UTF16ToString(mount), err)
	}
	vn := windows.UTF16ToString(name) // \?\Volume{GUID}\
	i, j := strings.IndexByte(vn, '{'), strings.IndexByte(vn, '}')
	if i < 0 || j < i {
		return "", fmt.Errorf("unexpected volume name %q", vn)
	}
	return strings.ToLower(vn[i : j+1]), nil
}
