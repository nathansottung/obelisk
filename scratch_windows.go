//go:build windows

package main

import (
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

// deleteByHandle marks the file f, which this operation created and holds
// open, for deletion through its own handle: the object removed is the one
// held, whatever its name now names. For a destination stage (deststage_windows.go).
func deleteByHandle(f *os.File) error {
	remove := byte(1)
	var iosb windows.IO_STATUS_BLOCK
	err := windows.NtSetInformationFile(windows.Handle(f.Fd()), &iosb, &remove, 1, windows.FileDispositionInformation)
	runtime.KeepAlive(f)
	return err
}
