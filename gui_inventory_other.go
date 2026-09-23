//go:build !windows

package main

import "os"

func inventoryIsLink(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }

// Other platforms retain explicit absolute/non-link policy. Local mount type,
// bind aliases and runtime behavior are not qualified by this Windows milestone.
func inventoryPlatformPath(string) error { return nil }

func inventoryDirectoryEncoding(string, int) error { return nil } // Raw Go names are checked before adoption.
