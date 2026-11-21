//go:build unix && !guionly

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ensurePrivateDir creates path as 0700, or confirms that an existing folder is a
// real folder owned by this user with no group or other access.
func ensurePrivateDir(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a plain folder", path)
	}
	return checkOwnerOnlyInfo(fi, path)
}

// verifyPrivateFile confirms path is a plain file owned by this user, with no group
// or other access.
func verifyPrivateFile(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s is not a plain file", path)
	}
	return checkOwnerOnlyInfo(fi, path)
}

func checkOwnerOnlyInfo(fi os.FileInfo, path string) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read the owner of %s", path)
	}
	if int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s belongs to another account", path)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s can be reached by another account", path)
	}
	return nil
}
