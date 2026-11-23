//go:build !windows

package main

import "os"

// publishAbsent gives the finished file from the name final only if final does
// not exist. A rename would replace it here, so a hard link is the atomic
// no-replace publish. The caller removes the staging name.
func publishAbsent(from, final string) error { return os.Link(from, final) }
