//go:build !windows && !linux && !darwin && !guionly

package main

import "os"

// lockFileExclusive has no lock to take on this platform; the release targets are
// Windows, Linux and macOS.
func lockFileExclusive(*os.File) error { return nil }

func unlockFile(*os.File) error { return nil }
