//go:build !windows

package main

// transientRenameError is always false here: POSIX renames are not blocked by other
// processes holding the file open, so a failed rename is not retried.
func transientRenameError(error) bool { return false }
