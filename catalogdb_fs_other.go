//go:build !windows && !linux && !darwin && !guionly

package main

// checkCatalogDBLocal has no way to tell a network filesystem on this platform;
// the release targets are Windows, Linux and macOS.
func checkCatalogDBLocal(string) error { return nil }
