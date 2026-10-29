//go:build !guionly

package main

import (
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

// lsblk output for one disk with two partitions: the partition UUID is the
// mounted partition's, the serial is the disk's.
func TestParseLsblk_PartitionUUID(t *testing.T) {
	out := []byte(`{"blockdevices":[{"name":"sdb","serial":"WD-123","model":"WD Elements","size":"4000787030016","mountpoint":null,"type":"disk","tran":"usb","partuuid":null,
	  "children":[{"name":"sdb1","serial":null,"model":null,"size":1000,"mountpoint":"/media/a","type":"part","tran":null,"partuuid":"1111-aaaa"},
	              {"name":"sdb2","serial":null,"model":null,"size":2000,"mountpoint":"/media/b","type":"part","tran":null,"partuuid":"2222-bbbb"}]}]}`)
	id, err := parseLsblk(out, "/media/b/photos")
	if err != nil {
		t.Fatal(err)
	}
	if id.Serial != "WD-123" || id.Partition != "2222-bbbb" {
		t.Errorf("got serial %q partition %q, want WD-123 / 2222-bbbb", id.Serial, id.Partition)
	}
	id, _ = parseLsblk(out, "/media/a")
	if id.Partition != "1111-aaaa" {
		t.Errorf("partition at /media/a = %q, want 1111-aaaa", id.Partition)
	}
}

// diskutil names the mounted volume's UUID; the partition ID is that UUID.
func TestDiskutilPartitionID(t *testing.T) {
	info := map[string]string{"Volume UUID": "0A1B2C3D-0000-1111-2222-333344445555", "Disk / Partition UUID": "FFFF0000-0000-1111-2222-333344445555"}
	if got := diskutilPartitionID(info); got != "0A1B2C3D-0000-1111-2222-333344445555" {
		t.Errorf("partition ID %q, want the Volume UUID", got)
	}
}

// On Windows the partition ID is the volume GUID, the same for any folder on
// the volume.
func TestResolvePartitionID_WindowsVolumeGUID(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the volume GUID is a Windows identity")
	}
	root := os.Getenv("SystemDrive") + `\`
	id, err := resolvePartitionID(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "{") || !strings.HasSuffix(id, "}") || len(id) != 38 {
		t.Errorf("partition ID %q, want a {GUID}", id)
	}
	sub, err := resolvePartitionID(filepath.Join(os.Getenv("SystemRoot"), "System32"))
	if err != nil || sub != id {
		t.Errorf("a folder on the same volume: %q (%v), want %q", sub, err, id)
	}
}
