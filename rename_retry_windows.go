//go:build windows

package main

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

// Win32 error codes a rename reports while another process holds the file.
const (
	errorAccessDenied     syscall.Errno = 5
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

// transientRenameError reports whether a rename failed only because the file was
// briefly held open by someone else, so that trying again shortly may succeed.
// It understands both a Win32 error (from os.Rename) and an NTSTATUS (from a
// handle rename through NtSetInformationFile), which is mapped to its Win32 code.
func transientRenameError(err error) bool {
	var errno syscall.Errno
	var status windows.NTStatus
	switch {
	case errors.As(err, &errno):
	case errors.As(err, &status):
		errno = status.Errno()
	default:
		return false
	}
	switch errno {
	case errorAccessDenied, errorSharingViolation, errorLockViolation:
		return true
	}
	return false
}
