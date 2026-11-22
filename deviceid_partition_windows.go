//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ioctlDiskGetPartitionInfoEx = 0x00070048
	ioctlDiskGetDriveLayoutEx   = 0x00070050
	ioctlStorageGetDeviceNumber = 0x002D1080
	partitionStyleMBR           = 0
	partitionStyleGPT           = 1
	partitionInfoExSize         = 144
	partitionInfoGPTIDOffset    = 48
	driveLayoutMBRSigOffset     = 8
	finalPathVolumeGUID         = 0x1 // VOLUME_NAME_GUID for GetFinalPathNameByHandle
)

// resolvePartitionID is the on-disk ID of the partition holding path (see
// deviceid_parse.go): its GPT partition GUID, or its MBR disk signature and
// partition number. It is read from the disk, so the same partition has the
// same ID on any computer, unlike the volume GUID Windows assigns per machine.
// Works for a volume at a drive letter or mounted in a folder. Read-only, no
// PowerShell.
func resolvePartitionID(path string) (string, error) {
	// A volume across several disks has no one partition. Windows may still answer.
	if _, err := volumeDiskNumber(path); errors.Is(err, errVolumeSpansDisks) {
		return "", err
	}
	vol, err := volumeDevicePath(path)
	if err != nil {
		return "", err
	}
	h, err := openForQuery(vol)
	if err != nil {
		return "", fmt.Errorf("opening the volume of %s: %w", path, err)
	}
	defer windows.CloseHandle(h)
	var info [partitionInfoExSize]byte
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlDiskGetPartitionInfoEx, nil, 0, &info[0], uint32(len(info)), &n, nil); err != nil {
		return "", fmt.Errorf("reading the partition of %s: %w", path, err)
	}
	switch binary.LittleEndian.Uint32(info[0:4]) {
	case partitionStyleGPT:
		g := (*windows.GUID)(unsafe.Pointer(&info[partitionInfoGPTIDOffset]))
		if id := gptPartitionID(g.String()); id != "" {
			return id, nil
		}
	case partitionStyleMBR:
		layout, err := mbrDriveLayout(h)
		if err != nil {
			return "", fmt.Errorf("reading the disk layout of %s: %w", path, err)
		}
		sig := binary.LittleEndian.Uint32(layout[driveLayoutMBRSigOffset : driveLayoutMBRSigOffset+4])
		start := int64(binary.LittleEndian.Uint64(info[8:16]))
		if id := mbrPartitionID(sig, mbrLinuxNumber(layout, start)); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("no on-disk partition ID for %s", path)
}

// volumeDevicePath is the \\?\Volume{GUID} device of the volume holding path. It
// opens the object and asks Windows which volume that handle is on, so a folder
// mount, junction or link is followed to where the object really is. A path not
// there yet is answered from its deepest existing folder.
func volumeDevicePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	h, err := openObjectForQuery(abs)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		ancestor, _, derr := deepestExistingDir(abs)
		if derr != nil {
			return "", fmt.Errorf("finding the volume of %s: %w", path, derr)
		}
		h, err = openObjectForQuery(ancestor)
	}
	if err != nil {
		return "", fmt.Errorf("opening %s to find its volume: %w", path, err)
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), finalPathVolumeGUID)
	if err != nil {
		return "", fmt.Errorf("finding the volume of %s: %w", path, err)
	}
	if int(n) >= len(buf) {
		return "", fmt.Errorf("finding the volume of %s: the path is too long", path)
	}
	vol := volumeFromFinalPath(windows.UTF16ToString(buf[:n]))
	if vol == "" {
		return "", fmt.Errorf("finding the volume of %s: Windows named no volume", path)
	}
	return vol, nil
}

// openObjectForQuery opens a file or folder, following links and mounts, with no
// read or write access asked for.
func openObjectForQuery(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

// volumeFromFinalPath is the \\?\Volume{GUID} prefix of a final path, or "".
func volumeFromFinalPath(final string) string {
	const prefix = `\\?\Volume{`
	if !strings.HasPrefix(final, prefix) {
		return ""
	}
	end := strings.Index(final, "}")
	if end < 0 {
		return ""
	}
	return final[:end+1]
}

// openForQuery opens a volume or disk device for queries only: no read or
// write access is asked for.
func openForQuery(dev string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(dev)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

// mbrDriveLayout is the DRIVE_LAYOUT_INFORMATION_EX of the MBR disk a volume
// is on.
func mbrDriveLayout(vol windows.Handle) ([]byte, error) {
	var num [12]byte // STORAGE_DEVICE_NUMBER: type, device number, partition number
	var n uint32
	if err := windows.DeviceIoControl(vol, ioctlStorageGetDeviceNumber, nil, 0, &num[0], uint32(len(num)), &n, nil); err != nil {
		return nil, err
	}
	disk, err := openForQuery(fmt.Sprintf(`\\.\PhysicalDrive%d`, binary.LittleEndian.Uint32(num[4:8])))
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(disk)
	layout := make([]byte, 48+128*partitionInfoExSize)
	if err := windows.DeviceIoControl(disk, ioctlDiskGetDriveLayoutEx, nil, 0, &layout[0], uint32(len(layout)), &n, nil); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(layout[0:4]) != partitionStyleMBR {
		return nil, fmt.Errorf("the disk is not MBR")
	}
	if n < 48 {
		return nil, fmt.Errorf("the disk layout came back short (%d bytes)", n)
	}
	return layout[:n], nil
}
