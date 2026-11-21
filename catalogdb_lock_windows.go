//go:build windows && !guionly

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockHeldElsewhere reports whether a lockFileExclusive error means another holder
// has the lock, rather than that the lock could not be taken at all.
func lockHeldElsewhere(err error) bool {
	return errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING)
}

// lockFileExclusive takes an exclusive lock on f without waiting. Windows drops
// the lock when the process ends, so a crash never leaves the folder locked.
func lockFileExclusive(f *os.File) error {
	var ol windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
}

func unlockFile(f *os.File) error {
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
