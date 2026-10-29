//go:build windows && !guionly

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procWideCharToMultiByte = windows.NewLazySystemDLL("kernel32.dll").NewProc("WideCharToMultiByte")

// fitsANSICodePage reports whether name can be written exactly in the system's
// ANSI code page, the one the Windows tar converts names to.
func fitsANSICodePage(name string) bool {
	w, err := windows.UTF16FromString(name)
	if err != nil {
		return false
	}
	const cpACP, wcNoBestFitChars = 0, 0x400
	buf := make([]byte, 4*len(w)+4)
	var usedDefault int32
	n, _, _ := procWideCharToMultiByte.Call(cpACP, wcNoBestFitChars,
		uintptr(unsafe.Pointer(&w[0])), uintptr(len(w)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
		0, uintptr(unsafe.Pointer(&usedDefault)))
	return n != 0 && usedDefault == 0
}
