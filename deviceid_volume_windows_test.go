//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// fakeVolumeDisk answers the disk lookups without hardware.
func fakeVolumeDisk(t *testing.T, disk uint32, diskErr error) *[]string {
	t.Helper()
	var scripts []string
	oldNum, oldRun := volumeDiskNumber, runDiskQuery
	volumeDiskNumber = func(string) (uint32, error) { return disk, diskErr }
	runDiskQuery = func(script string) ([]byte, error) {
		scripts = append(scripts, script)
		return []byte(`{"Serial":"INNER-SERIAL","Model":"Inner Disk","Friendly":"","Size":1000,"Bus":"USB"}`), nil
	}
	t.Cleanup(func() { volumeDiskNumber, runDiskQuery = oldNum, oldRun })
	return &scripts
}

// A folder on C: can be the mount point of another disk. The disk must come from
// the volume holding the path, never from the outer drive letter.
func TestDeviceIdentity_DiskComesFromTheVolumeNotTheLetter(t *testing.T) {
	scripts := fakeVolumeDisk(t, 7, nil)
	id, err := resolveDeviceIdentity(`C:\mnt\usb`)
	if err != nil {
		t.Fatal(err)
	}
	if id.Serial != "INNER-SERIAL" {
		t.Fatalf("serial %q", id.Serial)
	}
	if len(*scripts) != 1 || !strings.Contains((*scripts)[0], "Get-Disk -Number 7") || strings.Contains((*scripts)[0], "DriveLetter") {
		t.Fatalf("the disk was not looked up by the volume's disk number: %q", *scripts)
	}
}

// When the volume's disk is unknown, nothing is guessed from the drive letter.
func TestDeviceIdentity_UnknownDiskIsNotGuessed(t *testing.T) {
	for name, diskErr := range map[string]error{
		"a volume across disks": errVolumeSpansDisks,
		"a failed lookup":       errors.New("no answer"),
	} {
		t.Run(name, func(t *testing.T) {
			scripts := fakeVolumeDisk(t, 0, diskErr)
			id, _ := resolveDeviceIdentity(`C:\mnt\usb`)
			if id.Serial != "" || id.Model != "" || id.SizeBytes != 0 {
				t.Fatalf("a disk was recorded: %+v", id)
			}
			if len(*scripts) != 0 {
				t.Fatalf("a disk was looked up anyway: %q", *scripts)
			}
		})
	}
}

// A volume across disks has no single partition to record either.
func TestDeviceIdentity_SpanningVolumeRecordsNoPartition(t *testing.T) {
	fakeVolumeDisk(t, 0, errVolumeSpansDisks)
	id, _ := resolveDeviceIdentity(`C:\`)
	if id.Partition != "" {
		t.Fatalf("a partition was recorded for a volume across disks: %q", id.Partition)
	}
}

func TestPhysicalDiskOf_UnknownForASpanningVolume(t *testing.T) {
	fakeVolumeDisk(t, 0, errVolumeSpansDisks)
	if d, ok := physicalDiskOf(`C:\`); ok {
		t.Fatalf("got %q for a volume across disks", d)
	}
}

// Drive health must read the same disk the identity names.
func TestSmartDevice_UsesTheVolumesDisk(t *testing.T) {
	fakeVolumeDisk(t, 7, nil)
	if got, err := smartDeviceNode(`C:\mnt\usb`); err != nil || got != "/dev/pd7" {
		t.Fatalf("got %q, %v", got, err)
	}
	fakeVolumeDisk(t, 0, errVolumeSpansDisks)
	if got, err := smartDeviceNode(`C:\mnt\usb`); err == nil {
		t.Fatalf("got %q for a volume across disks", got)
	}
}

// The real lookup on this PC agrees with Windows about the system drive.
func TestVolumeDiskNumber_MatchesWindowsForTheSystemDrive(t *testing.T) {
	got, err := volumeDiskNumber(`C:\`)
	if err != nil {
		t.Skipf("this PC does not report disk extents here: %v", err)
	}
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`(Get-Partition -DriveLetter C | Get-Disk).Number`).Output()
	if err != nil {
		t.Skipf("Get-Disk is not available: %v", err)
	}
	want, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Skipf("unexpected answer %q", out)
	}
	if int(got) != want {
		t.Fatalf("volume lookup says disk %d, Windows says %d", got, want)
	}
}

// TestManual_FolderMountedDrive is the manual check in
// docs/development/MANUAL_CHECK_FOLDER_MOUNT.md. It reads only.
func TestManual_FolderMountedDrive(t *testing.T) {
	mount := os.Getenv("OBELISK_MANUAL_MOUNT")
	if mount == "" {
		t.Skip("manual: set OBELISK_MANUAL_MOUNT to a folder a drive is mounted in")
	}
	const script = `$ErrorActionPreference='Stop';` +
		`$m=$env:OBELISK_MANUAL_MOUNT.TrimEnd('\')+'\';` +
		`$p=@(Get-Partition | Where-Object { $_.AccessPaths -contains $m });` +
		`if($p.Count -ne 1){throw "no single partition is mounted at $m"};` +
		`$d=$p[0] | Get-Disk;` +
		`$c=(Get-Partition -DriveLetter C | Get-Disk).Number;` +
		`[pscustomobject]@{Number=[int]$d.Number;Serial=[string]$d.SerialNumber;Size=[int64]$d.Size;` +
		`Guid=[string]$p[0].Guid;SystemDisk=[int]$c}|ConvertTo-Json -Compress`
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("asking Windows about %s: %v\n%s", mount, err, out)
	}
	var want struct {
		Number, SystemDisk int
		Serial, Guid       string
		Size               int64
	}
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if want.Number == want.SystemDisk {
		t.Fatalf("%s is on the system disk. Mount a different drive here so the check can tell them apart", mount)
	}
	n, err := volumeDiskNumber(mount)
	if err != nil {
		t.Fatalf("Obelisk could not read the disk of %s: %v", mount, err)
	}
	id, err := resolveDeviceIdentity(mount)
	if err != nil {
		t.Fatalf("Obelisk could not identify %s: %v", mount, err)
	}
	t.Logf("Windows:  disk %d, serial %q, size %d, partition %s", want.Number, strings.TrimSpace(want.Serial), want.Size, want.Guid)
	t.Logf("Obelisk:  disk %d, serial %q, size %d, partition %s", n, id.Serial, id.SizeBytes, id.Partition)
	if int(n) != want.Number {
		t.Errorf("disk: Obelisk %d, Windows %d", n, want.Number)
	}
	if id.Serial != strings.TrimSpace(want.Serial) {
		t.Errorf("serial: Obelisk %q, Windows %q", id.Serial, strings.TrimSpace(want.Serial))
	}
	if id.SizeBytes != want.Size {
		t.Errorf("size: Obelisk %d, Windows %d", id.SizeBytes, want.Size)
	}
	if g := strings.ToLower(strings.Trim(want.Guid, "{}")); g != "" && !strings.Contains(strings.ToLower(id.Partition), g) {
		t.Errorf("partition: Obelisk %q, Windows %q", id.Partition, want.Guid)
	}
}
