//go:build windows

package main

// syncDir is a no-op on Windows: unlike POSIX, Windows does not support fsync on
// a directory handle, so there is no directory-entry flush to perform. Content
// durability still holds — writeCatalog fsyncs the temp file before the rename;
// the directory fsync is a POSIX-only belt-and-suspenders step we skip here.
func syncDir(string) error { return nil }
