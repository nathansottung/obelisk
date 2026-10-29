//go:build !windows && !linux

package main

// physicalDiskOf is not read on this system; keystores are placed by volume.
func physicalDiskOf(dir string) (string, bool) { return "", false }
