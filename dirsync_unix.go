//go:build !windows

package main

import "os"

// syncDir fsyncs a directory so that a rename INTO it is durable across power
// loss: on POSIX filesystems the directory entry created by os.Rename is only
// guaranteed on stable storage once the directory itself is fsynced. Opening the
// directory read-only and calling Sync is the standard idiom. Best-effort — some
// filesystems return an error for a directory fsync, which the caller treats as a
// graceful skip (the renamed file's contents were already fsynced separately).
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
