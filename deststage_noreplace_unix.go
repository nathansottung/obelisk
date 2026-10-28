//go:build !windows

package main

import "golang.org/x/sys/unix"

// linkThenUnlink gives the staged file the name to without replacing anything:
// linkat refuses an existing name with EEXIST, and only then is the staging name
// removed. If the removal fails the file is still published under to.
func linkThenUnlink(fromDir int, from string, toDir int, to string) error {
	if err := unix.Linkat(fromDir, from, toDir, to, 0); err != nil {
		return err
	}
	_ = unix.Unlinkat(fromDir, from, 0)
	return nil
}
