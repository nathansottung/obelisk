//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// resolveDeviceIdentity maps a path to its physical disk and partition, both from
// the volume that holds the path. A drive mounted in a folder is a different volume
// from the drive letter its path starts with, so the letter is never used. The disk
// number comes from the volume's extents, then one read-only PowerShell/CIM call
// (Get-Disk -Number) describes that disk. A volume across several disks, or any
// failed lookup, leaves the disk unknown: nothing is guessed. The caller treats an
// error as non-fatal.
func resolveDeviceIdentity(path string) (DeviceIdentity, error) {
	disk, diskErr := volumeDiskNumber(path)
	if errors.Is(diskErr, errVolumeSpansDisks) {
		return DeviceIdentity{}, diskErr // no single disk or partition to record
	}
	// The partition is read from the same volume, so it is kept when only the
	// description of its disk could not be read.
	part, _ := resolvePartitionID(path)
	var id DeviceIdentity
	err := diskErr
	if err == nil {
		id, err = describeDisk(disk)
	}
	if err != nil {
		if part != "" {
			return DeviceIdentity{Partition: part}, nil
		}
		return DeviceIdentity{}, err
	}
	id.Partition = part
	return id, nil
}

// describeDisk is what Get-Disk reports about one disk number.
func describeDisk(disk uint32) (DeviceIdentity, error) {
	// One statement, structured output. Casts force the BusType enum and (possibly
	// padded) serial to plain strings so the JSON is stable across PowerShell versions.
	script := fmt.Sprintf(`$ErrorActionPreference='Stop';`+
		`$d=Get-Disk -Number %d;`+
		`[pscustomobject]@{Serial=[string]$d.SerialNumber;Model=[string]$d.Model;`+
		`Friendly=[string]$d.FriendlyName;Size=[int64]$d.Size;Bus=[string]$d.BusType}|ConvertTo-Json -Compress`, disk)

	out, err := runDiskQuery(script)
	if err != nil {
		return DeviceIdentity{}, fmt.Errorf("Get-Disk for disk %d: failed to resolve device (%v)", disk, err)
	}
	var raw struct {
		Serial, Model, Friendly, Bus string
		Size                         int64
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &raw); err != nil {
		return DeviceIdentity{}, fmt.Errorf("parsing Get-Disk output: %w", err)
	}
	model := strings.TrimSpace(raw.Model)
	if model == "" {
		model = strings.TrimSpace(raw.Friendly)
	}
	id := DeviceIdentity{
		Serial:    strings.TrimSpace(raw.Serial),
		Model:     model,
		SizeBytes: raw.Size,
		Bus:       strings.TrimSpace(raw.Bus),
	}
	id.Note = busCaveat(id.Bus)
	return id, nil
}

// runDiskQuery runs one read-only PowerShell statement. A variable so a test can
// answer it without hardware.
var runDiskQuery = func(script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return helperCommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
}
