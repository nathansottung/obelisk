//go:build linux

package main

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplace renames from to to, between directory descriptors, failing
// with EEXIST (fs.ErrExist) if to exists. A filesystem without RENAME_NOREPLACE
// falls back to link-then-unlink, which refuses an existing name the same way.
func renameNoReplace(fromDir int, from string, toDir int, to string) error {
	err := unix.Renameat2(fromDir, from, toDir, to, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
		return linkThenUnlink(fromDir, from, toDir, to)
	}
	return err
}
