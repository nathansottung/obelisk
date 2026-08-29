package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// findStream returns the sampled row for a stream path, or nil.
func findStream(s PerfSample, path string) *PerfStreamOut {
	for i := range s.Streams {
		if s.Streams[i].Path == path {
			return &s.Streams[i]
		}
	}
	return nil
}

// TestPerfMeterMovingAverage checks the trailing-window rate math and idle decay,
// driven by explicit timestamps so it never sleeps or depends on real CPU/RAM.
func TestPerfMeterMovingAverage(t *testing.T) {
	m := NewPerfMeter()
	base := time.Unix(1_700_000_000, 0)
	const dst = "/mnt/vol"

	// 10 MB observed at t=0 (baseline), t=1s, t=2s. The trailing-window rate is the
	// counter delta over the spanned time: (30-10)MB over (2-0)s = 10 MB/s.
	m.Observe("dst:"+dst, "dest", "VOL-1", dst, 0, 10_000_000, base)
	m.Observe("dst:"+dst, "dest", "VOL-1", dst, 0, 10_000_000, base.Add(1*time.Second))
	m.Observe("dst:"+dst, "dest", "VOL-1", dst, 0, 10_000_000, base.Add(2*time.Second))

	s := m.Sample(base.Add(2 * time.Second))
	row := findStream(s, dst)
	if row == nil {
		t.Fatalf("destination stream missing from sample: %+v", s.Streams)
	}
	if row.MBps < 9.9 || row.MBps > 10.1 {
		t.Fatalf("moving-average MB/s wrong: got %v, want ~10", row.MBps)
	}
	if row.Label != "VOL-1" || row.Kind != "dest" {
		t.Fatalf("row identity wrong: %+v", row)
	}
	if row.PeakMBps < row.MBps {
		t.Fatalf("peak (%v) should be >= current (%v)", row.PeakMBps, row.MBps)
	}

	// IDLE DECAY: advance past the window with no further bytes → the rate reads 0
	// (the row is still present until the eviction horizon).
	s2 := m.Sample(base.Add(5 * time.Second))
	if row2 := findStream(s2, dst); row2 == nil || row2.MBps != 0 {
		t.Fatalf("idle rate should decay to 0, got %+v", row2)
	}

	// EVICTION: well past the evict horizon, the dead row is dropped entirely.
	s3 := m.Sample(base.Add(30 * time.Second))
	if findStream(s3, dst) != nil {
		t.Fatalf("stale stream should have been evicted: %+v", s3.Streams)
	}
}

// TestPerfMeterThrottleMarker checks a throttled destination pinned near its cap is
// flagged (the paused-style marker) and the cap is reported.
func TestPerfMeterThrottleMarker(t *testing.T) {
	m := NewPerfMeter()
	base := time.Unix(1_700_000_000, 0)
	const dst = "/mnt/slow"
	capMBps := 20.0                 // MB/s
	throttleBps := capMBps * 1e6    // as passed by the copy sites
	perTick := int64(capMBps * 1e6) // ~cap MB per 1s tick → rate ≈ cap
	m.Observe("dst:"+dst, "dest", "TAPE", dst, throttleBps, perTick, base)
	m.Observe("dst:"+dst, "dest", "TAPE", dst, throttleBps, perTick, base.Add(1*time.Second))
	m.Observe("dst:"+dst, "dest", "TAPE", dst, throttleBps, perTick, base.Add(2*time.Second))

	row := findStream(m.Sample(base.Add(2*time.Second)), dst)
	if row == nil {
		t.Fatal("throttled stream missing")
	}
	if row.ThrottleMBps < 19.9 || row.ThrottleMBps > 20.1 {
		t.Fatalf("throttle cap wrong: %v", row.ThrottleMBps)
	}
	if !row.Throttled {
		t.Fatalf("stream at the cap should be flagged throttled: rate=%v cap=%v", row.MBps, row.ThrottleMBps)
	}
}

// TestPerfEndpointFastNoCatalogLock proves GET /api/perf never waits on the catalog
// mutex: it must answer promptly WHILE s.mu is held. A future regression that took the
// catalog lock inside the handler would block here and trip the timeout.
func TestPerfEndpointFastNoCatalogLock(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	app := &App{DataDir: t.TempDir(), Store: st, Perf: NewPerfMeter()}
	handler := func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, app.Perf.Sample(time.Now()))
	}

	// Hold the catalog mutex for the whole call.
	st.mu.Lock()
	defer st.mu.Unlock()

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/perf", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		done <- time.Since(start)
	}()

	select {
	case d := <-done:
		if d > 5*time.Millisecond {
			t.Fatalf("/api/perf took %v while catalog lock held; budget is 5ms (does it touch s.mu?)", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("/api/perf blocked while the catalog mutex was held — it must never take s.mu")
	}
}
