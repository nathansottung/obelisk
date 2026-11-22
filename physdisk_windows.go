//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// ioctlVolumeGetVolumeDiskExtents is IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS.
const ioctlVolumeGetVolumeDiskExtents = 0x00560000

// physicalDiskOf names the one physical disk holding folder dir ("disk:<number>"),
// from the disk extents of its volume. ok is false when that cannot be read (a
// network share, for example) or the volume lies on several disks.
func physicalDiskOf(dir string) (string, bool) {
	n, err := volumeDiskNumber(dir)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("disk:%d", n), true
}

// physicalDisksOf names every physical disk holding folder dir. ok is false when
// that cannot be read.
func physicalDisksOf(dir string) ([]string, bool) {
	disks, err := volumeDiskSet(dir)
	if err != nil || len(disks) == 0 {
		return nil, false
	}
	out := make([]string, len(disks))
	for i, d := range disks {
		out[i] = fmt.Sprintf("disk:%d", d)
	}
	return out, true
}

// volumeDiskNumber is the number of the one physical disk holding the volume that
// holds path. A variable so a test can present a volume without hardware.
var volumeDiskNumber = func(path string) (uint32, error) {
	disks, err := volumeDiskSet(path)
	if err != nil {
		return 0, err
	}
	if len(disks) != 1 {
		return 0, errVolumeSpansDisks
	}
	return disks[0], nil
}

// volumeDiskSet is every physical disk holding the volume that holds path. A
// variable so a test can present a volume without hardware.
var volumeDiskSet = func(path string) ([]uint32, error) {
	vol, err := volumeDevicePath(path)
	if err != nil {
		return nil, err
	}
	// The volume is opened with no access asked for: reading its extents needs none.
	h, err := openForQuery(vol)
	if err != nil {
		return nil, fmt.Errorf("opening the volume of %s: %w", path, err)
	}
	defer windows.CloseHandle(h)
	buf := make([]byte, 512)
	var n uint32
	err = windows.DeviceIoControl(h, ioctlVolumeGetVolumeDiskExtents, nil, 0, &buf[0], uint32(len(buf)), &n, nil)
	if errors.Is(err, windows.ERROR_MORE_DATA) { // the answer starts with how many there are
		buf = make([]byte, extentBufferFor(buf[:8]))
		err = windows.DeviceIoControl(h, ioctlVolumeGetVolumeDiskExtents, nil, 0, &buf[0], uint32(len(buf)), &n, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("reading the disks of %s: %w", path, err)
	}
	return disksFromExtents(buf[:n])
}
