//go:build !windows && !linux

package main

// renameNoReplace renames from to to, between directory descriptors, failing
// with EEXIST (fs.ErrExist) if to exists.
func renameNoReplace(fromDir int, from string, toDir int, to string) error {
	return linkThenUnlink(fromDir, from, toDir, to)
}
