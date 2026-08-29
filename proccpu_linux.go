//go:build linux

package main

// proccpu_linux.go — process CPU time from /proc/self/stat (utime+stime), the same
// pure file-read style as meminfo_linux.go. CGO-free; USER_HZ is 100 on effectively
// every Linux, so we avoid a sysconf/cgo call.

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const linuxUserHZ = 100

// processCPUTime returns user+system CPU time consumed by this process since start.
// /proc/self/stat is "pid (comm) state ...": comm can contain spaces and parentheses,
// so we parse the fixed fields AFTER the final ')'. utime is field 14, stime field 15.
func processCPUTime() (time.Duration, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	s := string(b)
	rp := strings.LastIndexByte(s, ')')
	if rp < 0 || rp+2 >= len(s) {
		return 0, false
	}
	// fields[0] is `state` (field 3); utime (14) -> index 11, stime (15) -> index 12.
	fields := strings.Fields(s[rp+2:])
	if len(fields) < 13 {
		return 0, false
	}
	utime, err1 := strconv.ParseInt(fields[11], 10, 64)
	stime, err2 := strconv.ParseInt(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return time.Duration(utime+stime) * time.Second / linuxUserHZ, true
}
