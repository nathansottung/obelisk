//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// publishAbsent moves the finished file from to the name final only if final does
// not exist. The move is atomic and, without MOVEFILE_REPLACE_EXISTING, never
// replaces an entry.
//
// It is a move and not a hard link because a file that has just been hard-linked
// can refuse to be replaced for several seconds: the kernel holds it open, and
// both a plain and a POSIX-style rename over it get "Access is denied", for longer
// than the rename retry waits. The staging name is gone when this returns.
func publishAbsent(from, final string) error {
	src, err := windows.UTF16PtrFromString(extendedPath(from))
	if err != nil {
		return &os.LinkError{Op: "move", Old: from, New: final, Err: err}
	}
	dst, err := windows.UTF16PtrFromString(extendedPath(final))
	if err != nil {
		return &os.LinkError{Op: "move", Old: from, New: final, Err: err}
	}
	if err := windows.MoveFileEx(src, dst, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return &os.LinkError{Op: "move", Old: from, New: final, Err: err}
	}
	return nil
}

// extendedPath gives an absolute path the \\?\ prefix, so a data folder with a
// long path works here as it does through the os package. Any other path is
// returned unchanged.
func extendedPath(p string) string {
	if !filepath.IsAbs(p) || strings.HasPrefix(p, `\\?\`) {
		return p
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, `\\`) {
		return `\\?\UNC\` + clean[2:]
	}
	return `\\?\` + clean
}
