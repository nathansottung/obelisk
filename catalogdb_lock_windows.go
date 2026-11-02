//go:build windows && !guionly

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

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
