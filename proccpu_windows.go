//go:build windows

package main

// proccpu_windows.go — process CPU time via kernel32!GetProcessTimes, CGO-free
// (stdlib syscall + NewLazyDLL, the same idiom as meminfo_windows.go / diskfree_windows.go).

import (
	"syscall"
	"time"
	"unsafe"
)

// fileTime mirrors Win32 FILETIME: a 64-bit count of 100-nanosecond intervals split
// into two 32-bit halves.
type fileTime struct {
	low  uint32
	high uint32
}

func (f fileTime) duration() time.Duration {
	ticks := uint64(f.high)<<32 | uint64(f.low)
	return time.Duration(ticks) * 100 // each tick is 100ns
}

// processCPUTime returns kernel+user CPU time consumed by this process since start.
func processCPUTime() (time.Duration, bool) {
	k := syscall.NewLazyDLL("kernel32.dll")
	getCurrentProcess := k.NewProc("GetCurrentProcess")
	getProcessTimes := k.NewProc("GetProcessTimes")
	h, _, _ := getCurrentProcess.Call()
	var creation, exit, kernel, user fileTime
	r, _, _ := getProcessTimes.Call(h,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return 0, false
	}
	return kernel.duration() + user.duration(), true
}
