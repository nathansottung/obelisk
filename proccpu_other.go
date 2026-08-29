//go:build !windows && !linux && !darwin

package main

// proccpu_other.go — fallback for platforms without a CGO-free process-CPU read; the
// strip then shows CPU as unavailable rather than lying.

import "time"

func processCPUTime() (time.Duration, bool) { return 0, false }
