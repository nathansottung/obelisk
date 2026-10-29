//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

// ioctlVolumeGetVolumeDiskExtents is IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS.
const ioctlVolumeGetVolumeDiskExtents = 0x00560000

// physicalDiskOf names the physical disk holding folder dir ("disk:<number>"),
// from the disk extents of its volume. ok is false when that cannot be read
// (a network share, for example).
func physicalDiskOf(dir string) (string, bool) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return "", false
	}
	var root [windows.MAX_PATH + 1]uint16
	if err := windows.GetVolumePathName(p, &root[0], uint32(len(root))); err != nil {
		return "", false
	}
	var name [windows.MAX_PATH + 1]uint16
	if err := windows.GetVolumeNameForVolumeMountPoint(&root[0], &name[0], uint32(len(name))); err != nil {
		return "", false
	}
	// The volume is opened without its trailing backslash, with no access
	// asked for: reading its extents needs none.
	vol, err := windows.UTF16PtrFromString(strings.TrimSuffix(windows.UTF16ToString(name[:]), `\`))
	if err != nil {
		return "", false
	}
	h, err := windows.CreateFile(vol, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(h)
	// VOLUME_DISK_EXTENTS: a count, then DISK_EXTENT entries (disk number,
	// offset, length) from byte 8. The first extent's disk names the device.
	var buf [256]byte
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlVolumeGetVolumeDiskExtents, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil || n < 12 {
		return "", false
	}
	if binary.LittleEndian.Uint32(buf[0:4]) == 0 {
		return "", false
	}
	return fmt.Sprintf("disk:%d", binary.LittleEndian.Uint32(buf[8:12])), true
}
