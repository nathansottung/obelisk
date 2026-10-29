//go:build !windows

package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// resolveDeviceIdentity maps a mounted path to its physical device: Linux shells
// `lsblk -J` (structured JSON) and finds the disk whose subtree owns the path's
// mountpoint; macOS shells `diskutil info`. No CGO, read-only, non-fatal on any
// failure (tool absent, path not on a block device, network mount).
func resolveDeviceIdentity(path string) (DeviceIdentity, error) {
	abs := absEval(path)
	switch runtime.GOOS {
	case "linux":
		return resolveLinux(abs)
	case "darwin":
		return resolveDarwin(abs)
	}
	return DeviceIdentity{}, fmt.Errorf("device identity not supported on %s", runtime.GOOS)
}

// resolvePartitionID is the ID of the partition holding path: the partition
// UUID on Linux, the GPT partition GUID on macOS (see deviceid_parse.go).
func resolvePartitionID(path string) (string, error) {
	abs := absEval(path)
	var id DeviceIdentity
	var err error
	switch runtime.GOOS {
	case "linux":
		id, err = resolveLinux(abs)
	case "darwin":
		id, err = resolveDarwin(abs)
	default:
		return "", fmt.Errorf("partition identity not supported on %s", runtime.GOOS)
	}
	if err != nil {
		return "", err
	}
	if id.Partition == "" {
		return "", fmt.Errorf("no partition ID for %s", abs)
	}
	return id.Partition, nil
}

func absEval(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if p, e := filepath.EvalSymlinks(abs); e == nil {
		abs = p
	}
	return abs
}

// ---- Linux: lsblk -------------------------------------------------------

func resolveLinux(abs string) (DeviceIdentity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := helperCommandContext(ctx, "lsblk", "-J", "-b", "-o", lsblkColumns).Output()
	if err != nil {
		return DeviceIdentity{}, fmt.Errorf("lsblk: %v", err)
	}
	return parseLsblk(out, abs)
}

// ---- macOS: diskutil ----------------------------------------------------

var diskutilBytes = regexp.MustCompile(`\((\d+) Bytes\)`)

func resolveDarwin(abs string) (DeviceIdentity, error) {
	info, err := diskutilInfo(abs)
	if err != nil {
		return DeviceIdentity{}, err
	}
	vol := info
	id := DeviceIdentity{}
	// "Part of Whole: disk2" points at the physical disk, which carries the
	// media name (model), full capacity and partition scheme; the volume node
	// often does not.
	if whole := info["Part of Whole"]; whole != "" {
		if wi, e := diskutilInfo(whole); e == nil {
			info = wi
		}
	}
	id.Partition = diskutilPartitionID(vol, info)
	if m := firstNonEmpty(info["Device / Media Name"], info["Media Name"]); m != "" {
		id.Model = m
	}
	for _, k := range []string{"Disk Size", "Total Size", "Volume Used Space"} {
		if v := info[k]; v != "" {
			if b := diskutilBytes.FindStringSubmatch(v); b != nil {
				id.SizeBytes, _ = strconv.ParseInt(b[1], 10, 64)
				break
			}
		}
	}
	// diskutil rarely exposes a hardware serial for the volume; capture it when
	// present under any of the keys different macOS versions have used.
	id.Serial = firstNonEmpty(info["Disk / Partition UUID"], info["Volume UUID"])
	if strings.Contains(strings.ToLower(info["Protocol"]), "usb") {
		id.Bus = "USB"
		id.Note = busCaveat("USB")
	}
	if !id.resolved() {
		return id, fmt.Errorf("diskutil reported no identity for %s", abs)
	}
	return id, nil
}

func diskutilInfo(target string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := helperCommandContext(ctx, "diskutil", "info", target).Output()
	if err != nil {
		return nil, fmt.Errorf("diskutil info %s: %v", target, err)
	}
	m := map[string]string{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		parts := strings.SplitN(string(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return m, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
