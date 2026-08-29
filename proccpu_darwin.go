//go:build darwin

package main

// proccpu_darwin.go — process CPU time via getrusage(RUSAGE_SELF), CGO-free (stdlib
// syscall, the same family of sysctl/syscall reads as meminfo_darwin.go).

import (
	"syscall"
	"time"
)

// processCPUTime returns user+system CPU time consumed by this process since start.
func processCPUTime() (time.Duration, bool) {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0, false
	}
	tv := func(t syscall.Timeval) time.Duration {
		return time.Duration(t.Sec)*time.Second + time.Duration(t.Usec)*time.Microsecond
	}
	return tv(ru.Utime) + tv(ru.Stime), true
}
