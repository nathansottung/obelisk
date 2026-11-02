//go:build (linux || darwin) && !guionly

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFileExclusive takes an exclusive lock on f without waiting. The system drops
// the lock when the process ends, so a crash never leaves the folder locked.
func lockFileExclusive(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func unlockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
