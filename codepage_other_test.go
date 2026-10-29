//go:build !windows && !guionly

package main

// fitsANSICodePage is true off Windows: the system tar takes names as UTF-8.
func fitsANSICodePage(string) bool { return true }
