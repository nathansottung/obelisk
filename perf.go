package main

// perf.go — the live "Performance strip" meter. It answers one question cheaply and
// honestly: how fast is OBELISK ITSELF moving bytes right now, and where is the
// bottleneck (a slow destination, a starved ring buffer, a throttle cap)? These are
// the APP's own transfer rates, NOT whole-system disk activity.
//
// The meter is app-level live state behind its OWN mutex — it NEVER touches the
// catalog mutex (Store.mu), so /api/perf can never contend with a running job for the
// store. Copy paths feed it byte deltas keyed by source/destination identity; the UI
// reads a snapshot via Sample(). Every method takes an explicit `now` so the
// moving-average math is deterministic and unit-testable without sleeps.

import (
	"runtime"
	"sort"
	"sync"
	"time"
)

const (
	perfWindow   = 2 * time.Second  // trailing moving-average window
	perfIdleZero = 2 * time.Second  // no bytes for this long → the rate reads 0
	perfEvict    = 10 * time.Second // no bytes for this long → drop the row entirely
)

// perfCumSample is a cumulative-bytes reading at a point in time; a trailing pair of
// these gives an unambiguous "counter over a window" rate.
type perfCumSample struct {
	t   time.Time
	cum int64
}

type perfStream struct {
	kind        string // "source" | "dest"
	label       string // volume label (dest) or "" (source)
	path        string
	throttleBps float64 // 0 = unthrottled
	cum         int64   // cumulative bytes observed on this stream
	samples     []perfCumSample
	mbps        float64
	peakMBps    float64
	lastSeen    time.Time
	bufPct      int // ring-buffer fill %, -1 when not a ring destination
	stalls      int // live starved-writer count for a ring destination
}

// PerfStreamOut is one ledger row in the sampled snapshot.
type PerfStreamOut struct {
	Kind         string  `json:"kind"`
	Label        string  `json:"label,omitempty"`
	Path         string  `json:"path"`
	MBps         float64 `json:"mbps"`
	PeakMBps     float64 `json:"peak_mbps"`
	ThrottleMBps float64 `json:"throttle_mbps,omitempty"` // 0 = unthrottled
	Throttled    bool    `json:"throttled,omitempty"`     // actively pinned at the cap (paused-style marker)
	BufferPct    int     `json:"buffer_pct"`              // -1 = not a ring destination
	Stalls       int     `json:"stalls,omitempty"`
}

// PerfSample is the whole strip in one JSON object — what GET /api/perf returns.
type PerfSample struct {
	CPUPercent    float64         `json:"cpu_percent"`     // machine-normalized 0..100 (share of ALL cores)
	CPUAvailable  bool            `json:"cpu_available"`   // false when this platform can't read process CPU
	RAMFreeBytes  int64           `json:"ram_free_bytes"`  // 0 = undetectable
	RAMTotalBytes int64           `json:"ram_total_bytes"` // 0 = undetectable
	Streams       []PerfStreamOut `json:"streams"`
	BufferPct     int             `json:"buffer_pct"` // active ring destination fill %, -1 = no ring write
	Stalls        int             `json:"stalls"`
	Throttled     bool            `json:"throttled"` // any destination pinned at its cap
	At            time.Time       `json:"at"`
}

// PerfMeter is the live transfer meter. Its mutex guards only this struct — never the
// catalog. Construct with NewPerfMeter and hang it off App.
type PerfMeter struct {
	mu      sync.Mutex
	streams map[string]*perfStream
	// process-CPU sampling state (delta between successive Samples)
	lastCPU   time.Duration
	lastCPUAt time.Time
	numCPU    int
}

func NewPerfMeter() *PerfMeter {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	return &PerfMeter{streams: map[string]*perfStream{}, numCPU: n}
}

// Observe records deltaBytes moved on a stream (id keys the row; a source/destination
// path, not a per-file id, keeps the row count bounded). kind is "source" or "dest";
// label is the volume label for a destination; throttleBps is the writer's cap (0 =
// none). Nil-meter and non-positive deltas are ignored.
func (m *PerfMeter) Observe(id, kind, label, path string, throttleBps float64, deltaBytes int64, now time.Time) {
	if m == nil || deltaBytes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.streams[id]
	if s == nil {
		s = &perfStream{kind: kind, bufPct: -1}
		m.streams[id] = s
	}
	// Identity fields can arrive on the first byte; keep them fresh.
	s.label, s.path, s.throttleBps = label, path, throttleBps
	s.cum += deltaBytes
	s.samples = append(s.samples, perfCumSample{t: now, cum: s.cum})
	s.lastSeen = now
	s.mbps = streamRate(s, now)
	if s.mbps > s.peakMBps {
		s.peakMBps = s.mbps
	}
}

// SetBuffer records the live ring-buffer fill percentage and running stall count for a
// destination stream (created on demand so it can be set alongside the first Observe).
func (m *PerfMeter) SetBuffer(id, kind, label, path string, fillPct, stalls int, now time.Time) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.streams[id]
	if s == nil {
		s = &perfStream{kind: kind, bufPct: -1}
		m.streams[id] = s
	}
	if label != "" {
		s.label = label
	}
	if path != "" {
		s.path = path
	}
	s.bufPct, s.stalls, s.lastSeen = fillPct, stalls, now
}

// streamRate computes the trailing-window MB/s for a stream: pruned to samples within
// perfWindow of now, the counter delta over the spanned time. Fewer than two samples in
// the window (i.e. no recent bytes) reads 0 — the idle decay. Caller holds m.mu.
func streamRate(s *perfStream, now time.Time) float64 {
	cutoff := now.Add(-perfWindow)
	i := 0
	for i < len(s.samples) && s.samples[i].t.Before(cutoff) {
		i++
	}
	if i > 0 {
		s.samples = append(s.samples[:0], s.samples[i:]...)
	}
	if len(s.samples) < 2 {
		return 0
	}
	first, last := s.samples[0], s.samples[len(s.samples)-1]
	span := last.t.Sub(first.t).Seconds()
	if span <= 0 {
		return 0
	}
	return round1(float64(last.cum-first.cum) / span / 1e6)
}

// Sample returns the current snapshot: per-stream rates (idle rows decayed to 0, dead
// rows evicted), machine RAM, and machine-normalized process CPU% since the last
// Sample. Takes no catalog lock.
func (m *PerfMeter) Sample(now time.Time) PerfSample {
	out := PerfSample{At: now, BufferPct: -1}

	m.mu.Lock()
	for id, s := range m.streams {
		if now.Sub(s.lastSeen) > perfEvict {
			delete(m.streams, id)
			continue
		}
		s.mbps = streamRate(s, now) // decays to 0 once the window empties
		row := PerfStreamOut{
			Kind: s.kind, Label: s.label, Path: s.path,
			MBps: s.mbps, PeakMBps: s.peakMBps, BufferPct: s.bufPct,
		}
		if s.throttleBps > 0 {
			row.ThrottleMBps = round1(s.throttleBps / 1e6)
			row.Throttled = s.mbps >= 0.85*row.ThrottleMBps
		}
		if s.kind == "dest" && s.bufPct >= 0 {
			row.Stalls = s.stalls
			if out.BufferPct < 0 || now.Sub(s.lastSeen) <= perfIdleZero {
				out.BufferPct, out.Stalls = s.bufPct, s.stalls
			}
		}
		if row.Throttled {
			out.Throttled = true
		}
		out.Streams = append(out.Streams, row)
	}
	m.mu.Unlock()

	// Stable ledger order: sources first, then destinations, then by label/path.
	sort.Slice(out.Streams, func(i, j int) bool {
		a, b := out.Streams[i], out.Streams[j]
		if a.Kind != b.Kind {
			return a.Kind == "source" // "source" sorts before "dest"
		}
		if a.Label != b.Label {
			return a.Label < b.Label
		}
		return a.Path < b.Path
	})

	if mem := SystemMemory(); mem.TotalBytes > 0 {
		out.RAMFreeBytes, out.RAMTotalBytes = mem.AvailableBytes, mem.TotalBytes
	}
	out.CPUPercent, out.CPUAvailable = m.cpuPercent(now)
	return out
}

// cpuPercent returns the app's CPU use as a share of the WHOLE machine (0..100),
// measured between successive Samples. The first call establishes the baseline and
// reports 0. Guarded by its own lock so it stays off the catalog mutex.
func (m *PerfMeter) cpuPercent(now time.Time) (float64, bool) {
	cpu, ok := processCPUTime()
	if !ok {
		return 0, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	prev, prevAt := m.lastCPU, m.lastCPUAt
	m.lastCPU, m.lastCPUAt = cpu, now
	if prevAt.IsZero() {
		return 0, true // baseline established; first delta comes next Sample
	}
	wall := now.Sub(prevAt).Seconds()
	if wall <= 0 {
		return 0, true
	}
	pct := (cpu - prev).Seconds() / (wall * float64(m.numCPU)) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return round1(pct), true
}
