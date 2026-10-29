//go:build !guionly

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Two partitions of one disk share the disk's serial. Each is its own volume,
// told apart by its partition ID.
func TestVolumeByIdentity_PartitionsOfOneDisk(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p1 := st.AddVolume(Volume{Label: "P1", Serial: "SER-1", PartitionID: "PART-1"})
	p2 := st.AddVolume(Volume{Label: "P2", Serial: "SER-1", PartitionID: "PART-2"})
	if v := st.VolumeByIdentity("SER-1", "part-2"); v == nil || v.ID != p2.ID {
		t.Errorf("SER-1/PART-2 matched %+v, want P2", v)
	}
	if v := st.VolumeByIdentity("SER-1", "PART-1"); v == nil || v.ID != p1.ID {
		t.Errorf("SER-1/PART-1 matched %+v, want P1", v)
	}
	if v := st.VolumeByIdentity("SER-1", "PART-3"); v != nil {
		t.Errorf("a third partition matched %s, want none", v.Label)
	}
	if v := st.VolumeByIdentity("SER-1", ""); v != nil {
		t.Errorf("with the partition unknown, matched %s of two partitions, want none", v.Label)
	}
	// A masked serial: the partition ID alone is enough.
	if v := st.VolumeByIdentity("", "PART-1"); v == nil || v.ID != p1.ID {
		t.Errorf("PART-1 without a serial matched %+v, want P1", v)
	}
	// A volume recorded before partitions were: the one volume with that serial.
	old := st.AddVolume(Volume{Label: "OLD", Serial: "SER-OLD"})
	if v := st.VolumeByIdentity("SER-OLD", "PART-9"); v == nil || v.ID != old.ID {
		t.Errorf("an older volume with no partition recorded matched %+v, want OLD", v)
	}
	if v := st.VolumeByIdentity("SER-OLD", ""); v == nil || v.ID != old.ID {
		t.Errorf("an older volume with the partition unknown matched %+v, want OLD", v)
	}
}

// A check is never credited to a copy on another partition, even at the same
// path and on the same disk.
func TestCreditCopyCheck_NeverCrossesPartitions(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v := st.AddVolume(Volume{Label: "P1", Serial: "SER-1", PartitionID: "PART-1"})
	c := st.AddChunk(Chunk{Name: "PKG"})
	st.RecordCopy(c, v.ID, "E:/PKG", false)
	if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/PKG", Serial: "SER-1", Partition: "PART-2"}, true, VerifyB); n != 0 {
		t.Errorf("a read of PART-2 credited %d copies on PART-1", n)
	}
	if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/PKG", Serial: "SER-1", Partition: "PART-1"}, true, VerifyB); n != 1 {
		t.Errorf("a read of PART-1 credited %d copies, want 1", n)
	}
}

// A drive mounted somewhere new is credited by identity only when the partition
// read is the copy's partition. The disk serial alone is shared by every
// partition of the disk.
func TestCreditCopyCheck_MovedDriveNeedsItsPartition(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v := st.AddVolume(Volume{Label: "P1", Serial: "SER-1", PartitionID: "PART-1"})
	legacy := st.AddVolume(Volume{Label: "OLD", Serial: "SER-2"})
	c := st.AddChunk(Chunk{Name: "PKG"})
	st.RecordCopy(c, v.ID, "E:/PKG", false)
	d := st.AddChunk(Chunk{Name: "PKG2"})
	st.RecordCopy(d, legacy.ID, "E:/PKG2", false)

	if n := st.CreditCopyCheck(c, CopyAt{Path: "F:/PKG", Serial: "SER-1", Partition: "PART-2"}, true, VerifyB); n != 0 {
		t.Errorf("another partition of the disk, at a new path: credited %d, want 0", n)
	}
	if n := st.CreditCopyCheck(c, CopyAt{Path: "F:/PKG", Serial: "SER-1"}, true, VerifyB); n != 0 {
		t.Errorf("partition unknown, at a new path: credited %d, want 0", n)
	}
	if n := st.CreditCopyCheck(d, CopyAt{Path: "F:/PKG2", Serial: "SER-2", Partition: "PART-X"}, true, VerifyB); n != 0 {
		t.Errorf("a volume with no partition recorded, at a new path: credited %d, want 0", n)
	}
	if n := st.CreditCopyCheck(c, CopyAt{Path: "F:/PKG", Serial: "SER-1", Partition: "PART-1"}, true, VerifyB); n != 1 {
		t.Errorf("the copy's own partition at a new path: credited %d, want 1", n)
	}
	if got := copyOn(c, v.ID).Path; normPath(got) != normPath("F:/PKG") {
		t.Errorf("copy path %s, want it moved to F:/PKG", got)
	}
}

// Docking two partitions of one disk registers two volumes, not one.
func TestIngestDrive_TwoPartitionsAreTwoVolumes(t *testing.T) {
	app := dockApp(t)
	a, b := t.TempDir(), t.TempDir()
	writeTree(t, a, map[string]string{"one.txt": "one\n"})
	writeTree(t, b, map[string]string{"two.txt": "two\n"})
	orig := partitionIDAt
	partitionIDAt = func(p string) string {
		if normPath(p) == normPath(a) {
			return "PART-A"
		}
		return "PART-B"
	}
	defer func() { partitionIDAt = orig }()
	ds, err := app.StartDockSession(nil)
	if err != nil {
		t.Fatal(err)
	}
	ra, err := app.IngestDrive(ds.ID, a, "SER-DISK", "A", "", "", false, noProg)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := app.IngestDrive(ds.ID, b, "SER-DISK", "B", "", "", false, noProg)
	if err != nil {
		t.Fatal(err)
	}
	if ra["volume_id"] == rb["volume_id"] {
		t.Fatalf("both partitions were taken as volume %v", ra["volume_id"])
	}
	va, vb := app.Store.VolumeByIdentity("SER-DISK", "PART-A"), app.Store.VolumeByIdentity("SER-DISK", "PART-B")
	if va == nil || vb == nil || va.PartitionID != "PART-A" || vb.PartitionID != "PART-B" {
		t.Errorf("volumes %+v / %+v, want each partition recorded on its own volume", va, vb)
	}
}

// A mirror verify at an overriding mount refuses a different partition of the
// volume's disk.
func TestMirrorVerify_OverrideRefusesAnotherPartition(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.jpg": "A"})
	vol := app.Store.AddVolume(Volume{Label: "M1", Kind: "HDD", Serial: "SER-1", PartitionID: "PART-1"})
	if _, err := app.AdoptFolder(drive, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	origS, origP := driveSerialAt, partitionIDAt
	driveSerialAt = func(string) string { return "SER-1" }
	partitionIDAt = func(string) string { return "PART-2" }
	defer func() { driveSerialAt, partitionIDAt = origS, origP }()
	_, err := app.VerifyMirrorVolume(vol.ID, drive, VerifyB, nil)
	if err == nil || !strings.Contains(err.Error(), "partition") {
		t.Errorf("a verify of another partition at an overriding mount: %v, want refused", err)
	}
}

// lsblk output for one disk with two partitions: the partition ID is the
// mounted partition's on-disk ID, the serial is the disk's.
func TestParseLsblk_PartitionUUID(t *testing.T) {
	out := []byte(`{"blockdevices":[{"name":"sdb","serial":"WD-123","model":"WD Elements","size":"4000787030016","mountpoint":null,"type":"disk","tran":"usb","partuuid":null,
	  "children":[{"name":"sdb1","serial":null,"model":null,"size":1000,"mountpoint":"/media/a","type":"part","tran":null,"partuuid":"0A1B2C3D-01"},
	              {"name":"sdb2","serial":null,"model":null,"size":2000,"mountpoint":"/media/b","type":"part","tran":null,"partuuid":"6F1D2E3C-AAAA-4BBB-8CCC-0123456789AB"}]}]}`)
	id, err := parseLsblk(out, "/media/b/photos")
	if err != nil {
		t.Fatal(err)
	}
	if id.Serial != "WD-123" || id.Partition != "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab" {
		t.Errorf("got serial %q partition %q, want WD-123 / the GPT partition GUID", id.Serial, id.Partition)
	}
	id, _ = parseLsblk(out, "/media/a")
	if id.Partition != "mbr:0a1b2c3d-01" {
		t.Errorf("partition at /media/a = %q, want mbr:0a1b2c3d-01", id.Partition)
	}
}

// The partition ID is the one stored on the disk, written the same way on
// every OS: a GPT partition GUID, or an MBR disk signature and partition
// number. Anything else is unknown.
func TestPartitionIDForms(t *testing.T) {
	g := "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab"
	for in, want := range map[string]string{
		"{6F1D2E3C-AAAA-4BBB-8CCC-0123456789AB}": g,
		"6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab":   g,
		"not-a-guid":                             "",
		"":                                       "",
	} {
		if got := gptPartitionID(in); got != want {
			t.Errorf("gptPartitionID(%q) = %q, want %q", in, got, want)
		}
	}
	if got := mbrPartitionID(0x0a1b2c3d, 1); got != "mbr:0a1b2c3d-01" {
		t.Errorf("mbrPartitionID = %q", got)
	}
	if got := mbrPartitionID(0, 1); got != "" {
		t.Errorf("an MBR disk with no signature: %q, want unknown", got)
	}
	if got := linuxPartitionID("6F1D2E3C-AAAA-4BBB-8CCC-0123456789AB"); got != g {
		t.Errorf("linux GPT PARTUUID: %q", got)
	}
	if got := linuxPartitionID("0a1b2c3d-01"); got != "mbr:0a1b2c3d-01" {
		t.Errorf("linux MBR PARTUUID: %q", got)
	}
	if same, known := partitionCompare(g, "mbr:0a1b2c3d-01"); known || same {
		t.Errorf("IDs of different kinds compared as known (%v, %v), want unknown", same, known)
	}
	if same, known := partitionCompare(g, strings.ToUpper(g[:4])+g[4:]); !known || !same {
		t.Error("the same GPT ID did not compare equal")
	}
}

// IDs of different kinds say nothing about each other: a check is credited by
// its path as if the partition were unknown.
func TestCreditCopyCheck_OtherKindOfPartitionIDIsUnknown(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v := st.AddVolume(Volume{Label: "P1", Serial: "SER-1", PartitionID: "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab"})
	c := st.AddChunk(Chunk{Name: "PKG"})
	st.RecordCopy(c, v.ID, "E:/PKG", false)
	if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/PKG", Serial: "SER-1", Partition: "mbr:0a1b2c3d-01"}, true, VerifyB); n != 1 {
		t.Errorf("credited %d, want the copy at its own path credited", n)
	}
	if v2 := st.VolumeByIdentity("SER-1", "mbr:0a1b2c3d-01"); v2 == nil || v2.ID != v.ID {
		t.Errorf("an ID of another kind matched %+v, want the one volume with the serial", v2)
	}
}

// macOS names a GPT partition's on-disk GUID "Disk / Partition UUID". On a disk
// that is not GPT the partition ID is unknown.
func TestDiskutilPartitionID(t *testing.T) {
	vol := map[string]string{"Volume UUID": "0A1B2C3D-0000-1111-2222-333344445555", "Disk / Partition UUID": "6F1D2E3C-AAAA-4BBB-8CCC-0123456789AB"}
	gpt := map[string]string{"Content (IOContent)": "GUID_partition_scheme"}
	if got := diskutilPartitionID(vol, gpt); got != "gpt:6f1d2e3c-aaaa-4bbb-8ccc-0123456789ab" {
		t.Errorf("partition ID %q, want the GPT partition GUID", got)
	}
	if got := diskutilPartitionID(vol, map[string]string{"Content (IOContent)": "FDisk_partition_scheme"}); got != "" {
		t.Errorf("an MBR disk on macOS: %q, want unknown", got)
	}
}

// On Windows the partition ID is read from the disk through the volume, the
// same for any folder on the volume.
func TestResolvePartitionID_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("reads a Windows volume")
	}
	root := os.Getenv("SystemDrive") + `\`
	id, err := resolvePartitionID(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "gpt:") && !strings.HasPrefix(id, "mbr:") {
		t.Errorf("partition ID %q, want gpt: or mbr:", id)
	}
	sub, err := resolvePartitionID(filepath.Join(os.Getenv("SystemRoot"), "System32"))
	if err != nil || sub != id {
		t.Errorf("a folder on the same volume: %q (%v), want %q", sub, err, id)
	}
}

// A volume recorded before partitions were kept takes the partition read only
// when all its copies are found on that partition. Otherwise it is matched as
// before, by its serial, and stays without a partition.
func TestIngestDrive_LegacyVolumeBindsOnlyWithItsCopies(t *testing.T) {
	app := dockApp(t)
	a, b := t.TempDir(), t.TempDir()
	writeTree(t, a, map[string]string{"PKG/x.txt": "x"})
	writeTree(t, b, map[string]string{"other.txt": "o"})
	legacy := app.Store.AddVolume(Volume{Label: "OLD", Kind: "HDD", Serial: "SER-OLD"})
	c := app.Store.AddChunk(Chunk{Name: "PKG"})
	app.Store.RecordCopy(c, legacy.ID, filepath.Join(a, "PKG"), true)
	orig := partitionIDAt
	partitionIDAt = func(p string) string {
		if normPath(p) == normPath(a) {
			return "gpt:00000000-0000-0000-0000-00000000000a"
		}
		return "gpt:00000000-0000-0000-0000-00000000000b"
	}
	defer func() { partitionIDAt = orig }()
	ds, err := app.StartDockSession(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, b, "SER-OLD", "B", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	if v := app.Store.Volume(legacy.ID); v.PartitionID != "" {
		t.Fatalf("a partition without the volume's copies was bound to it: %s", v.PartitionID)
	}
	if _, err := app.IngestDrive(ds.ID, a, "SER-OLD", "A", "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	if v := app.Store.Volume(legacy.ID); v.PartitionID != "gpt:00000000-0000-0000-0000-00000000000a" {
		t.Errorf("the partition holding the volume's copies was not bound: %q", v.PartitionID)
	}
}

// An MBR partition is numbered as Linux numbers it: primaries by slot, logical
// partitions from 5, whatever Windows' own numbering.
func TestMbrLinuxNumber(t *testing.T) {
	const head, size = 48, 144
	type ent struct {
		off int64
		typ byte
	}
	layout := func(es ...ent) []byte {
		b := make([]byte, head+len(es)*size)
		binary.LittleEndian.PutUint32(b[4:8], uint32(len(es)))
		for i, e := range es {
			binary.LittleEndian.PutUint64(b[head+i*size+8:], uint64(e.off))
			b[head+i*size+32] = e.typ
		}
		return b
	}
	// Slot 1 empty, slot 2 NTFS, slot 3 extended; two logical partitions, each
	// followed by its link entry.
	l := layout(ent{0, 0}, ent{1 << 20, 0x07}, ent{100 << 20, 0x05}, ent{0, 0},
		ent{101 << 20, 0x07}, ent{200 << 20, 0x05}, ent{0, 0}, ent{0, 0},
		ent{201 << 20, 0x0c}, ent{0, 0}, ent{0, 0}, ent{0, 0})
	for off, want := range map[int64]uint32{1 << 20: 2, 101 << 20: 5, 201 << 20: 6, 999: 0} {
		if got := mbrLinuxNumber(l, off); got != want {
			t.Errorf("partition at %d: number %d, want %d", off, got, want)
		}
	}
}
