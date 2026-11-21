//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// ioctlVolumeGetVolumeDiskExtents is IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS.
const ioctlVolumeGetVolumeDiskExtents = 0x00560000

// physicalDiskOf names the physical disk holding folder dir ("disk:<number>"),
// from the disk extents of its volume. ok is false when that cannot be read
// (a network share, for example).
func physicalDiskOf(dir string) (string, bool) {
	n, err := volumeDiskNumber(dir)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("disk:%d", n), true
}

// volumeDiskNumber is the number of the one physical disk holding the volume
// that holds path. A variable so a test can present a volume without hardware.
var volumeDiskNumber = func(path string) (uint32, error) {
	vol, err := volumeDevicePath(path)
	if err != nil {
		return 0, err
	}
	// The volume is opened with no access asked for: reading its extents needs none.
	h, err := openForQuery(vol)
	if err != nil {
		return 0, fmt.Errorf("opening the volume of %s: %w", path, err)
	}
	defer windows.CloseHandle(h)
	var buf [512]byte
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlVolumeGetVolumeDiskExtents, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return 0, fmt.Errorf("reading the disk of %s: %w", path, err)
	}
	return diskFromExtents(buf[:n])
}
