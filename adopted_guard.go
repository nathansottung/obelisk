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

// adoptedPathKey is p as the adopted guard compares it.
func adoptedPathKey(p string) string {
	p = filepath.Clean(p)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

// adoptedCopyAt reports whether dir is where a copy of adopted package c is, or
// a folder inside one.
func adoptedCopyAt(c *Chunk, dir string) bool {
	if c == nil || !c.Adopted {
		return false
	}
	want := adoptedPathKey(dir)
	on := func(p string) bool {
		k := adoptedPathKey(p)
		return k == want || canonicalWithin(k, want)
	}
	for i := range c.Copies {
		if c.Copies[i].Path != "" && on(c.Copies[i].Path) {
			return true
		}
	}
	return c.WrittenDest != "" && on(c.WrittenDest)
}

// refuseAdoptedRewrite refuses a re-write of adopted package c's copy (always),
// or a write by c over dir when dir is where any adopted backup's copy is, or a
// folder inside one: an adopted backup is never written over, by its own
// package or by another.
func (a *App) refuseAdoptedRewrite(c *Chunk, dir string, always bool) error {
	if c == nil {
		return nil
	}
	if c.Adopted && (always || adoptedCopyAt(c, dir)) {
		return fmt.Errorf("package %s was adopted as found; Obelisk never re-writes or writes over an adopted backup. Write a new copy to another place instead", c.Name)
	}
	if dir == "" {
		return nil
	}
	for _, o := range a.Store.Chunks(0) {
		if o.ID != c.ID && adoptedCopyAt(o, dir) {
			return fmt.Errorf("refusing to write package %s over %s: a copy of package %s, adopted as found, is there, and Obelisk never writes over an adopted backup. Choose another place", c.Name, dir, o.Name)
		}
	}
	return nil
}
