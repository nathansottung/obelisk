package main

import (
	"encoding/binary"
	"testing"
)

// volumeExtents builds a VOLUME_DISK_EXTENTS answer for the given disk numbers.
func volumeExtents(disks ...uint32) []byte {
	b := make([]byte, 8+24*len(disks))
	binary.LittleEndian.PutUint32(b, uint32(len(disks)))
	for i, d := range disks {
		binary.LittleEndian.PutUint32(b[8+24*i:], d)
	}
	return b
}

func TestDiskFromExtents(t *testing.T) {
	for _, c := range []struct {
		name string
		buf  []byte
		want uint32
		ok   bool
	}{
		{"one extent", volumeExtents(3), 3, true},
		{"two extents on one disk", volumeExtents(2, 2), 2, true},
		{"a volume across two disks", volumeExtents(1, 2), 0, false},
		{"no extents", volumeExtents(), 0, false},
		{"an answer cut short", volumeExtents(1, 1)[:20], 0, false},
		{"empty", nil, 0, false},
	} {
		got, err := diskFromExtents(c.buf)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("%s: got %d, %v; want disk %d", c.name, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: got disk %d, want it unknown", c.name, got)
		}
	}
}
