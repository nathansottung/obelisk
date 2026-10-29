package main

// deviceid_parse.go — parsing of the platform tools' identity output, kept free
// of build tags so it is tested on every platform.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
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
		id.Partition = linuxPartitionID(mounted.PartUUID)
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

// ---- partition IDs ------------------------------------------------------
//
// A partition ID is the one stored on the disk, written the same way on every
// OS, so a drive read on another computer or OS is still the same partition:
//
//	gpt:<partition GUID, lower case>       a GPT partition's unique GUID
//	mbr:<disk signature>-<partition number> an MBR partition (Linux's PARTUUID form)
//
// Anything else is unknown (""). IDs of different kinds say nothing about each
// other, so they compare as unknown, never as different partitions.

var (
	guidForm = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	mbrForm  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{2}$`)
)

// gptPartitionID is the ID of a GPT partition with this unique GUID, braces
// and case as any OS prints it.
func gptPartitionID(guid string) string {
	g := strings.ToLower(strings.Trim(strings.TrimSpace(guid), "{}"))
	if !guidForm.MatchString(g) || g == "00000000-0000-0000-0000-000000000000" {
		return ""
	}
	return "gpt:" + g
}

// mbrPartitionID is the ID of partition number num on an MBR disk with this
// signature.
func mbrPartitionID(sig, num uint32) string {
	if sig == 0 || num == 0 || num > 0xff {
		return ""
	}
	return fmt.Sprintf("mbr:%08x-%02x", sig, num)
}

// linuxPartitionID is the ID for a Linux PARTUUID: a GPT GUID, or an MBR
// "signature-number".
func linuxPartitionID(partuuid string) string {
	p := strings.ToLower(strings.TrimSpace(partuuid))
	switch {
	case guidForm.MatchString(p):
		return gptPartitionID(p)
	case mbrForm.MatchString(p) && p[:8] != "00000000":
		return "mbr:" + p
	}
	return ""
}

// partitionCompare compares two partition IDs: known is false when either is
// unknown or they are of different kinds; same is meaningful only when known.
func partitionCompare(a, b string) (same, known bool) {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" || partitionKind(a) != partitionKind(b) {
		return false, false
	}
	return a == b, true
}

func partitionKind(id string) string {
	if i := strings.IndexByte(id, ':'); i >= 0 {
		return id[:i]
	}
	return ""
}

// diskutilPartitionID is the partition ID in `diskutil info` output: for a
// volume on a GPT disk, its GPT partition GUID ("Disk / Partition UUID");
// unknown on any other disk, where macOS makes the value up.
func diskutilPartitionID(vol, whole map[string]string) string {
	if !strings.Contains(whole["Content (IOContent)"], "GUID_partition_scheme") {
		return ""
	}
	return gptPartitionID(vol["Disk / Partition UUID"])
}

// mbrLinuxNumber numbers the MBR partition starting at offset the way Linux
// does in its PARTUUID, from a Windows DRIVE_LAYOUT_INFORMATION_EX: a primary
// partition is its slot (1-4), and logical partitions are 5, 6, … in the order
// of the extended boot records. Windows' own PartitionNumber skips empty slots
// and the extended container, so it differs whenever either is present. 0
// when the partition is not found.
//
// Layout: PartitionStyle u32 @0, PartitionCount u32 @4, then entries of 144
// bytes from @48. Each entry: StartingOffset i64 @8, and for MBR the partition
// type byte @32. Entries 0-3 are the primary slots; after them come groups of
// four per extended boot record, holding a logical partition and a link.
func mbrLinuxNumber(layout []byte, offset int64) uint32 {
	const head, size = 48, 144
	if len(layout) < head {
		return 0
	}
	count := int(binary.LittleEndian.Uint32(layout[4:8]))
	logical := uint32(4)
	for i := 0; i < count && head+(i+1)*size <= len(layout); i++ {
		e := layout[head+i*size : head+(i+1)*size]
		typ := e[32]
		used := typ != 0 && typ != 0x05 && typ != 0x0f && typ != 0x85
		if i >= 4 && used {
			logical++
		}
		if !used || int64(binary.LittleEndian.Uint64(e[8:16])) != offset {
			continue
		}
		if i < 4 {
			return uint32(i + 1)
		}
		return logical
	}
	return 0
}
