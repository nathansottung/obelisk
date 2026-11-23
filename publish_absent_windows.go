//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// publishAbsentHasFallback: the move below works on every Windows filesystem, so
// a failure is final and never falls through to a replacing rename.
const publishAbsentHasFallback = false

// extendedPathFrom is the path length, in UTF-16 units, from which a path needs
// the \\?\ prefix. It is the limit the os package uses.
const extendedPathFrom = 248

// publishAbsent moves the finished file from to the name final only if final does
// not exist. The move is atomic and, without MOVEFILE_REPLACE_EXISTING, never
// replaces an entry. A stage briefly held by a scanner is retried as a rename is.
//
// It is a move and not a hard link because a file that has just been hard-linked
// can refuse to be replaced for several seconds: the kernel holds it open, and
// both a plain and a POSIX-style rename over it get "Access is denied", for longer
// than the rename retry waits. The staging name is gone when this succeeds.
func publishAbsent(from, final string) error {
	fail := func(err error) error { return &os.LinkError{Op: "move", Old: from, New: final, Err: err} }
	src, err := windows.UTF16PtrFromString(extendedPath(from))
	if err != nil {
		return fail(err)
	}
	dst, err := windows.UTF16PtrFromString(extendedPath(final))
	if err != nil {
		return fail(err)
	}
	return retryTransientRename(func() error {
		if err := windows.MoveFileEx(src, dst, windows.MOVEFILE_WRITE_THROUGH); err != nil {
			return fail(err)
		}
		return nil
	})
}

// extendedPath gives a long absolute path the \\?\ prefix, so a data folder with a
// long path works here as it does through the os package. A shorter path, a
// relative path and one already in device form are returned unchanged, so Win32
// normalizes them the same way it does for every other file call.
func extendedPath(p string) string {
	if len(utf16.Encode([]rune(p))) < extendedPathFrom || !filepath.IsAbs(p) {
		return p
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, `\\?\`) || strings.HasPrefix(clean, `\\.\`) {
		return p
	}
	if strings.HasPrefix(clean, `\\`) {
		return `\\?\UNC\` + clean[2:]
	}
	return `\\?\` + clean
}
