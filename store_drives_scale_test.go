//go:build !guionly

package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

// peakHeap runs fn and reports how long it took and the most heap it held above
// what was live before it started, sampled every few milliseconds.
func peakHeap(fn func() error) (time.Duration, uint64, error) {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	var peak uint64
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			mu.Lock()
			if m.HeapAlloc > peak {
				peak = m.HeapAlloc
			}
			mu.Unlock()
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	}()
	t0 := time.Now()
	err := fn()
	took := time.Since(t0)
	close(done)
	mu.Lock()
	defer mu.Unlock()
	if peak < base {
		return took, 0, err
	}
	return took, peak - base, err
}

// liveHeap is the heap still in use after a collection.
func liveHeap() uint64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

func mib(n uint64) string { return fmt.Sprintf("%d MiB", n>>20) }

// scaleDrive is a drive of n files; shift changes which files it holds, so two
// drives built with different shifts differ in a known way.
func scaleDrive(volumeID, n, shift int) *VolumeSnapshot {
	snap := &VolumeSnapshot{VolumeID: volumeID, CapturedAt: time.Date(2025, 1, 1, 0, 0, shift, 0, time.UTC),
		Label: fmt.Sprintf("SCALE-%02d", volumeID), Serial: fmt.Sprintf("SCALE%d", volumeID), Scan: ScanComplete,
		Files: make([]SnapFile, 0, n)}
	for i := 0; i < n; i++ {
		rel := fmt.Sprintf("Clients/%04d/RAW/DSC_%08d.NEF", i%5000, i)
		h := fakeHash(i)
		if shift > 0 {
			switch i % 100 {
			case 1: // changed in place
				h = fakeHash(i + 3*n)
			case 2: // moved
				rel = fmt.Sprintf("Moved/%04d/DSC_%08d.NEF", i%5000, i)
			case 3: // new here, missing there
				rel, h = fmt.Sprintf("New/%08d.NEF", i), fakeHash(i+4*n)
			}
		}
		snap.Files = append(snap.Files, SnapFile{RelPath: rel, SizeBytes: 38_000_000 + int64(i%1000),
			ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Hash: h, Blake3: fakeHash(i + n), Role: RoleOriginals,
			ShotAt: time.Date(2023, 6, 1, 0, 0, i%86400, 0, time.UTC), CameraSerial: "4012345"})
		snap.TotalBytes += snap.Files[i].SizeBytes
	}
	snap.TotalFiles = n
	return snap
}

// TestDriveCompareScale1M stores two drives of a million files each and an
// archive of the same million, then times a scan-to-scan comparison, a
// comparison with the archive and the cross-drive lists, with the most memory
// each held. SKIPPED by default:
//
//	OBELISK_SCALE=1 OBELISK_SCALE_ENGINE=json|sqlite go test -run TestDriveCompareScale1M -v -timeout 60m
func TestDriveCompareScale1M(t *testing.T) {
	if os.Getenv("OBELISK_SCALE") == "" {
		t.Skip("set OBELISK_SCALE=1 to run the million-file drive benchmark")
	}
	n := envInt("OBELISK_FILES", 1_000_000)
	prev := fileCopyGuard
	fileCopyGuard = nil
	defer func() { fileCopyGuard = prev }()
	engine := os.Getenv("OBELISK_SCALE_ENGINE")
	if engine == "" {
		engine = "sqlite"
	}
	dir := t.TempDir()
	s := openEngineStore(t, engine, dir)
	app := initializedTestApp(t, &App{DataDir: dir, Store: s})
	coll := s.AddCollection("Scale")
	fo := s.AddFolder(coll.ID, "/srv/scale")
	s.BeginBatch()
	for i := 0; i < n; i++ {
		if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID,
			RelPath:   fmt.Sprintf("Clients/%04d/RAW/DSC_%08d.NEF", i%5000, i),
			SizeBytes: 38_000_000 + int64(i%1000), HashAlg: "SHA256", Hash: fakeHash(i),
			ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Role: RoleOriginals}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.EndBatch(); err != nil {
		t.Fatal(err)
	}
	before := liveHeap()
	var scans [2]*ScanRecord
	var store [2]time.Duration
	for k := range scans {
		vol := s.AddVolume(Volume{Label: fmt.Sprintf("SCALE-%02d", k+1), Kind: "HDD", Serial: fmt.Sprintf("SCALE%d", k+1)})
		snap := scaleDrive(vol.ID, n, k)
		t0 := time.Now()
		if err := s.PutVolumeSnapshot(snap); err != nil {
			t.Fatal(err)
		}
		rec, err := app.recordScan(snap, 10)
		if err != nil {
			t.Fatal(err)
		}
		store[k], scans[k] = time.Since(t0), rec
	}
	held := liveHeap()
	if held < before {
		held = before
	}
	var run *CompareRun
	scanTook, scanPeak, err := peakHeap(func() error {
		var err error
		run, err = app.CompareScans(CompareRef{ScanID: scans[0].ID}, scans[1].ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.Counts[ClassSame] == 0 || run.Counts[ClassChanged] == 0 || run.Counts[ClassMoved] == 0 ||
		run.Counts[ClassNew] == 0 || run.Counts[ClassMissing] == 0 {
		t.Errorf("scan comparison counts %v, want every kind", run.Counts)
	}
	archTook, archPeak, err := peakHeap(func() error {
		_, err := app.CompareScans(CompareRef{ArchiveID: coll.ID}, scans[1].ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	var lists *CrossDriveLists
	xTook, xPeak, err := peakHeap(func() error {
		var err error
		lists, err = app.CrossDriveLists(0)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lists.OnTwoOrMore) == 0 {
		t.Error("no file on two drives")
	}
	t.Logf("\n%s engine, two drives of %s files each, %s archive files\n"+
		"store snapshot + scan    %10s, %s\n"+
		"snapshots held in memory %10s\n"+
		"compare scan with scan   %10s, peak %s\n"+
		"compare scan with archive%10s, peak %s\n"+
		"cross-drive lists        %10s, peak %s\n",
		engine, comma(n), comma(n), round(store[0]), round(store[1]), mib(held-before),
		round(scanTook), mib(scanPeak), round(archTook), mib(archPeak), round(xTook), mib(xPeak))
}
