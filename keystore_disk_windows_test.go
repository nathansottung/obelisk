//go:build windows && !guionly

package main

import (
	"strings"
	"testing"
)

// The device rule is about losing a device, so on Windows keystores are placed
// by the physical disk under their volume: two volumes (partitions) of one
// disk are one device.
func TestKeystoreDevice_IsThePhysicalDisk(t *testing.T) {
	dev, err := realKeystoreDevice(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dev, "disk:") {
		t.Fatalf("keystore device %q is a volume, not a physical disk", dev)
	}
}
