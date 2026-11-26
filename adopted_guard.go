package main

// adopted_guard.go — Obelisk never encrypts or changes data in place on a
// device. An adopted backup is recorded as it was found: it is never encrypted,
// rebuilt, re-written or written over. To get an encrypted copy, build a new
// package from the source.

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// adoptedNotice is what the page shows for an adopted package that is not
// encrypted.
const adoptedNotice = "not encrypted, adopted as found"

// adoptedCopyAt reports whether dir is where a copy of adopted package c is.
func adoptedCopyAt(c *Chunk, dir string) bool {
	if c == nil || !c.Adopted {
		return false
	}
	key := func(p string) string {
		p = filepath.Clean(p)
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	want := key(dir)
	for i := range c.Copies {
		if c.Copies[i].Path != "" && key(c.Copies[i].Path) == want {
			return true
		}
	}
	return c.WrittenDest != "" && key(c.WrittenDest) == want
}

// refuseAdoptedRewrite refuses a re-write of adopted package c's copy, or a
// write over the folder an adopted copy is in.
func refuseAdoptedRewrite(c *Chunk, dir string, always bool) error {
	if c == nil || !c.Adopted {
		return nil
	}
	if always || adoptedCopyAt(c, dir) {
		return fmt.Errorf("package %s was adopted as found; Obelisk never re-writes or writes over an adopted backup. Write a new copy to another place instead", c.Name)
	}
	return nil
}
