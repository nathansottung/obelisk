package main

// app_helpers.go — small helpers shared by the full build (main.go) and the
// launcher-only build (main_gui.go, -tags guionly). Untagged, so each exists
// exactly once in both builds.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// fileExists reports whether path names an existing regular file.
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func s(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func f(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

func bl(m map[string]any, k string) bool {
	v, _ := m[k].(bool)
	return v
}

// pathFree reports free bytes for the nearest existing ancestor of p, so it
// works for a destination folder that doesn't exist yet (e.g. a new burn dir).
func pathFree(p string) (int64, error) {
	for {
		if _, err := os.Stat(p); err == nil {
			return diskFree(p)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return diskFree(p)
		}
		p = parent
	}
}

// progBytes formats a progress message that also carries byte counters for live
// telemetry: runJob parses the "\x1f<done>\x1f<total>\x1f<human>" prefix into
// MB/s + ETA and displays only the human tail. Byte-moving jobs (write/mirror/
// span) emit this so the job row shows throughput, not just a percent.
// progStats encodes structured live telemetry into a progress message: byte
// counters (for throughput/ETA), file counters (for "X / Y files"), and a human
// step label — delimited by the unit-separator control char so runJob can parse
// them out. A plain (unencoded) message is shown verbatim. progBytes is the
// byte-only wrapper; count-based jobs pass 0 bytes and real file counts.
func progStats(bytesDone, bytesTotal, filesDone, filesTotal int64, human string) string {
	return fmt.Sprintf("\x1f%d\x1f%d\x1f%d\x1f%d\x1f%s", bytesDone, bytesTotal, filesDone, filesTotal, human)
}

func progBytes(done, total int64, human string) string {
	return progStats(done, total, 0, 0, human)
}

// noteUnrecordedJob REPORTS the one case the job board cannot report through itself:
// the work reached a terminal state but the sidecar write failed.
//
// It does not set the job's state. FinishJob already published the terminal snapshot
// and its not-recorded qualification together, under one hold of s.jobs.mu, before it
// returned this error — so by the time this runs, /api/jobs has been serving the
// qualified row all along. This function must not re-flag the row either: an ordinary
// jobs write may already have recorded it in the meantime, and saying "not recorded"
// about a row that is on disk would be the same class of falsehood in the other
// direction.
//
// What it does is get the failure OUT of the failing subsystem, in two steps of
// decreasing reliability, and it is worth being exact about which is which:
//
//   - The process log line ALWAYS happens. It is the durable-in-practice record here:
//     no disk of ours, no lock, nothing to fail.
//   - The catalog audit append is BEST EFFORT and frequently does not reach a file.
//     Store.Log appends in memory and calls save(), which (a) writes catalog.json —
//     a different file, but usually the same volume, so the realistic whole-volume
//     failure takes it out too, and (b) is a no-op write when another job holds a
//     batch open, in which case the entry is only marked dirty for a later flush that
//     may never come. Measured on both paths: zero entries on disk after a reopen. So
//     an audit ATTEMPT is not evidence of a durable audit entry, and nothing here or
//     downstream may describe it as one.
//
// Neither step is a retry of the sidecar: that is the thing that just failed, and
// retrying it is how this turns into a loop. Recovery is left to ordinary subsequent
// jobs writes (see saveJobs).
func (a *App) noteUnrecordedJob(id int, status string, cause error) {
	msg := fmt.Sprintf("job %d reached %s but its record could not be written: %v", id, status, cause)
	log.Print("jobs: " + msg)
	// Deliberately last, deliberately unchecked, and deliberately outside any jobs
	// lock: Store.Log takes s.mu, and holding s.jobs.mu across that would invent a
	// lock ordering this code does not otherwise have. A failure here changes nothing
	// that has already been reported above.
	a.Store.Log("job-unrecorded", msg)
}

func offsiteWord(off bool) string {
	if off {
		return "offsite"
	}
	return "onsite"
}
