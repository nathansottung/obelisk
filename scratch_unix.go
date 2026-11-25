//go:build !windows

package main

import "golang.org/x/sys/unix"

// unlinkOwnAt removes name in the folder dirFd, a name this operation made and
// has just checked by identity (deststage_unix.go, deststage_noreplace_unix.go).
// flags is 0 for a file or unix.AT_REMOVEDIR for an empty folder.
func unlinkOwnAt(dirFd int, name string, flags int) error {
	return unix.Unlinkat(dirFd, name, flags)
}
