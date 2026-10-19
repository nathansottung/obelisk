//go:build windows

package main

import (
	"errors"
	"syscall"
)

// Win32 error codes a rename reports while another process holds the file.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

// transientRenameError reports whether a rename failed only because the file was
// briefly held open by someone else, so that trying again shortly may succeed.
func transientRenameError(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case errorAccessDenied, errorSharingViolation, errorLockViolation:
		return true
	}
	return false
}
