package main

// deviceid_parse.go — parsing of the platform tools' identity output, kept free
// of build tags so it is tested on every platform.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// flexInt unmarshals a JSON number OR a quoted number (lsblk -b emits either
// depending on version) into an int64.
type flexInt int64

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil // unparseable size is non-fatal; leave zero
	}
	*n = flexInt(v)
	return nil
}

type lsblkNode struct {
	Name       string      `json:"name"`
	Serial     string      `json:"serial"`
	Model      string      `json:"model"`
	Size       flexInt     `json:"size"`
	Mountpoint string      `json:"mountpoint"`
	Type       string      `json:"type"`
	Tran       string      `json:"tran"` // transport: usb, sata, nvme, …
	PartUUID   string      `json:"partuuid"`
	Children   []lsblkNode `json:"children"`
}

// lsblkColumns are the columns parseLsblk reads.
const lsblkColumns = "NAME,SERIAL,MODEL,SIZE,MOUNTPOINT,TYPE,TRAN,PARTUUID"

// parseLsblk finds, in `lsblk -J -b -o lsblkColumns` output, the disk whose
// subtree holds the longest mountpoint matching abs. The serial, model, size and
// transport are the disk's; the partition is the partition UUID of the node
// mounted there.
func parseLsblk(out []byte, abs string) (DeviceIdentity, error) {
	var doc struct {
		BlockDevices []lsblkNode `json:"blockdevices"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return DeviceIdentity{}, fmt.Errorf("parsing lsblk JSON: %w", err)
	}
	best := -1
	var pick lsblkNode
	var mounted *lsblkNode
	for i := range doc.BlockDevices {
		if l, n := deepestMountMatch(&doc.BlockDevices[i], abs); l > best {
			best, pick, mounted = l, doc.BlockDevices[i], n
		}
	}
	if best < 0 {
		return DeviceIdentity{}, fmt.Errorf("no block device mounts %s", abs)
	}
	id := DeviceIdentity{
		Serial:    strings.TrimSpace(pick.Serial),
		Model:     strings.TrimSpace(pick.Model),
		SizeBytes: int64(pick.Size),
		Bus:       strings.ToUpper(strings.TrimSpace(pick.Tran)),
	}
	if mounted != nil {
		id.Partition = strings.TrimSpace(mounted.PartUUID)
	}
	id.Note = busCaveat(id.Bus)
	return id, nil
}

// deepestMountMatch returns the length of the longest mountpoint at or under
// node that is a path-prefix of abs, and the node mounted there, or -1 if none
// matches.
func deepestMountMatch(node *lsblkNode, abs string) (int, *lsblkNode) {
	best, at := -1, (*lsblkNode)(nil)
	if mountMatches(node.Mountpoint, abs) && len(node.Mountpoint) > best {
		best, at = len(node.Mountpoint), node
	}
	for i := range node.Children {
		if l, n := deepestMountMatch(&node.Children[i], abs); l > best {
			best, at = l, n
		}
	}
	return best, at
}

func mountMatches(mp, abs string) bool {
	if mp == "" {
		return false
	}
	if abs == mp {
		return true
	}
	return strings.HasPrefix(abs, strings.TrimRight(mp, "/")+"/")
}

// diskutilPartitionID is the partition ID in `diskutil info` output for a
// mounted volume: its Volume UUID.
func diskutilPartitionID(info map[string]string) string {
	return strings.TrimSpace(info["Volume UUID"])
}
