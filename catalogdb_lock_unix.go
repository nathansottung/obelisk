//go:build (linux || darwin) && !guionly

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockHeldElsewhere reports whether a lockFileExclusive error means another holder
// has the lock, rather than that the lock could not be taken at all.
func lockHeldElsewhere(err error) bool { return errors.Is(err, unix.EWOULDBLOCK) }

// lockFileExclusive takes an exclusive lock on f without waiting. The system drops
// the lock when the process ends, so a crash never leaves the folder locked.
func lockFileExclusive(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func unlockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_UN) }
