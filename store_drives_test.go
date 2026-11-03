//go:build !guionly

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

var bothEngines = []string{"json", "sqlite"}

// testDrive is a snapshot of n files whose paths and hashes depend on seed.
func testDrive(volumeID, n, seed int) *VolumeSnapshot {
	snap := &VolumeSnapshot{VolumeID: volumeID, CapturedAt: time.Date(2025, 3, 1, 0, 0, seed, 0, time.UTC),
		Label: fmt.Sprintf("DRIVE-%02d", volumeID), Serial: fmt.Sprintf("SER%d", volumeID), Scan: ScanComplete,
		RoleFiles: map[string]int{RoleOriginals: n}, UnreadableFolders: []ScanGap{{RelPath: "locked", Reason: "denied"}}}
	for i := 0; i < n; i++ {
		snap.Files = append(snap.Files, SnapFile{RelPath: fmt.Sprintf("shoot/%04d/IMG_%06d.NEF", i%37, i),
			SizeBytes: int64(1000 + i), ModTime: time.Date(2024, 5, 1, 0, 0, i, 0, time.UTC), Hash: fakeHash(i + seed),
			Blake3: fakeHash(i + seed + 1_000_000), Role: RoleOriginals, CameraSerial: "CAM1"})
	}
	snap.TotalFiles = n
	return snap
}

// totalChanges is how many rows the database's writer connection has changed.
func totalChanges(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.be.(*dbBackend).d.w.QueryRow(`SELECT total_changes()`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A drive snapshot's files are stored with it and read back a page at a time, in
// order, on both engines; on the database engine they are not kept in memory,
// and what a getter returns is a copy without them.
func TestDrives_SnapshotFilesReadPageByPage(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			s := openEngineStore(t, engine, dir)
			vol := s.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD"})
			n := 2*snapFilePageSize + 17
			snap := testDrive(vol.ID, n, 0)
			if err := s.PutVolumeSnapshot(snap); err != nil {
				t.Fatal(err)
			}
			snap.Files[0].RelPath = "changed by the caller afterwards"
			if engine == "sqlite" && s.c.Snapshots[0].Files != nil {
				t.Fatalf("the database engine keeps %d snapshot files in memory", len(s.c.Snapshots[0].Files))
			}
			got := s.VolumeSnapshot(vol.ID)
			if got == nil || got.Files != nil || got.TotalFiles != n || got.Label != "DRIVE-01" {
				t.Fatalf("VolumeSnapshot = %+v, want the header without files", got)
			}
			got.Label, got.RoleFiles[RoleOriginals], got.UnreadableFolders[0].RelPath = "X", 0, "Y"
			if again := s.VolumeSnapshot(vol.ID); again.Label != "DRIVE-01" || again.RoleFiles[RoleOriginals] != n ||
				again.UnreadableFolders[0].RelPath != "locked" {
				t.Fatalf("changing a returned snapshot changed the catalog: %+v", again)
			}
			want := testDrive(vol.ID, n, 0).Files
			check := func(s *Store) {
				t.Helper()
				files := testSnapFiles(t, s, vol.ID)
				if !reflect.DeepEqual(files, want) {
					t.Fatalf("%d files read back, want %d in order (first %+v)", len(files), len(want), files[:1])
				}
			}
			check(s)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s = openEngineStore(t, engine, dir)
			check(s)
			if engine == "sqlite" && s.c.Snapshots[0].Files != nil {
				t.Fatal("reopening the database read the snapshot files into memory")
			}
			// A second scan replaces the files; a shorter list leaves none behind.
			if err := s.PutVolumeSnapshot(testDrive(vol.ID, 3, 5)); err != nil {
				t.Fatal(err)
			}
			if files := testSnapFiles(t, s, vol.ID); !reflect.DeepEqual(files, testDrive(vol.ID, 3, 5).Files) {
				t.Fatalf("after a rescan: %+v", files)
			}
		})
	}
}

// A snapshot replaced while it is being read ends the read with an error instead
// of mixing two scans.
func TestDrives_SnapshotReplacedWhileReadIsAnError(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			s := openEngineStore(t, engine, t.TempDir())
			vol := s.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD"})
			if err := s.PutVolumeSnapshot(testDrive(vol.ID, snapFilePageSize+1, 0)); err != nil {
				t.Fatal(err)
			}
			read, replaced := 0, false
			var gotErr error
			for _, err := range s.SnapshotFiles(vol.ID) {
				if err != nil {
					gotErr = err
					break
				}
				read++
				if !replaced {
					replaced = true
					if err := s.PutVolumeSnapshot(testDrive(vol.ID, 2, 9)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if gotErr == nil || !strings.Contains(gotErr.Error(), "changed while it was being read") || read != snapFilePageSize {
				t.Fatalf("read %d files, error %v; want the first page, then the error", read, gotErr)
			}
		})
	}
}

// A snapshot file whose hash is not in the catalog's form is refused on both
// engines alike.
func TestDrives_SnapshotWithANonCanonicalHashRefused(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			s := openEngineStore(t, engine, t.TempDir())
			vol := s.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD"})
			snap := testDrive(vol.ID, 2, 0)
			snap.Files[1].Hash = strings.ToUpper(snap.Files[1].Hash)
			if err := s.PutVolumeSnapshot(snap); err == nil || !strings.Contains(err.Error(), "hexadecimal") {
				t.Fatalf("PutVolumeSnapshot = %v, want a refusal", err)
			}
			if s.VolumeSnapshot(vol.ID) != nil {
				t.Fatal("the refused snapshot was stored")
			}
		})
	}
}

// Saving a plan's header, or marking one hash satisfied, writes that row and
// nothing else of a plan with thousands of entries.
func TestPlans_ChangeWritesOnlyWhatChanged(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	s := openEngineStore(t, "sqlite", t.TempDir())
	mapping := map[string]string{}
	for i := 0; i < 5000; i++ {
		mapping[fakeHash(i)] = fmt.Sprintf("dest/%05d.NEF", i)
	}
	p, err := s.AddPlanErr(&Plan{Name: "Move", DestinationRoot: "D:/dest", Status: PlanCompiled, Mapping: mapping})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FlushPending(); err != nil {
		t.Fatal(err)
	}
	if got := s.c.Plans[0]; got.Mapping != nil || got.Satisfied != nil {
		t.Fatalf("the database engine keeps a plan's maps in memory: %d mapped", len(got.Mapping))
	}
	before := totalChanges(t, s)
	p.Name = "Renamed"
	if err := s.ChangePlan(p, PlanChange{Satisfied: map[string]bool{fakeHash(7): true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.FlushPending(); err != nil {
		t.Fatal(err)
	}
	if n := totalChanges(t, s) - before; n > 6 {
		t.Fatalf("renaming a plan and marking one hash satisfied changed %d rows", n)
	}
	if got := s.c.Plans[0]; got.Mapping != nil || got.Satisfied != nil {
		t.Fatal("a change put the plan's maps in memory")
	}
	m := testPlanMaps(t, s, p.ID)
	if len(m.Mapping) != 5000 || !m.Satisfied[fakeHash(7)] || len(m.Satisfied) != 1 || s.Plan(p.ID).Name != "Renamed" {
		t.Fatalf("after the change: %d mapped, satisfied %v, name %q", len(m.Mapping), m.Satisfied, s.Plan(p.ID).Name)
	}
}

// The same changes leave the same maps on both engines, after reopening too, and
// every plan handed out is a copy without its maps.
func TestPlans_MapsAgreeAcrossEngines(t *testing.T) {
	run := func(engine string) (*PlanMaps, *Plan) {
		dir := t.TempDir()
		s := openEngineStore(t, engine, dir)
		p, err := s.AddPlanErr(&Plan{Name: "Move", DestinationRoot: "D:/dest", ArchiveIDs: []int{1},
			Overrides: map[string]string{fakeHash(1): "keep/one"}, Parked: map[string]bool{fakeHash(2): true, fakeHash(3): false}})
		if err != nil {
			t.Fatal(err)
		}
		p.Status = PlanCompiled
		if err := s.ChangePlan(p, PlanChange{Mapping: map[string]string{fakeHash(1): "a", fakeHash(4): "b", fakeHash(7): "d"}}); err != nil {
			t.Fatal(err)
		}
		// A new mapping replaces the old one whole.
		if err := s.ChangePlan(p, PlanChange{Mapping: map[string]string{fakeHash(1): "a", fakeHash(4): "b", fakeHash(5): "c"}}); err != nil {
			t.Fatal(err)
		}
		if err := s.ChangePlan(p, PlanChange{Satisfied: map[string]bool{fakeHash(4): true, fakeHash(5): true},
			Overrides: map[string]string{fakeHash(1): "", fakeHash(6): "moved/six"}, Parked: map[string]bool{fakeHash(2): false}}); err != nil {
			t.Fatal(err)
		}
		if err := s.ChangePlan(p, PlanChange{Satisfied: map[string]bool{fakeHash(5): false}}); err != nil {
			t.Fatal(err)
		}
		got := s.Plan(p.ID)
		got.Name, got.ArchiveIDs[0] = "changed by the caller", 99
		if s.Plan(p.ID).Name != "Move" || s.Plan(p.ID).ArchiveIDs[0] != 1 {
			t.Fatal("changing a returned plan changed the catalog")
		}
		if got.Mapping != nil || got.Satisfied != nil || got.Overrides != nil || got.Parked != nil {
			t.Fatalf("a returned plan carries its maps: %+v", got)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openEngineStore(t, engine, dir)
		return testPlanMaps(t, s, p.ID), s.Plan(p.ID)
	}
	jm, jp := run("json")
	dm, dp := run("sqlite")
	want := &PlanMaps{Overrides: map[string]string{fakeHash(6): "moved/six"}, Parked: map[string]bool{},
		Mapping:   map[string]string{fakeHash(1): "a", fakeHash(4): "b", fakeHash(5): "c"},
		Satisfied: map[string]bool{fakeHash(4): true}}
	if !reflect.DeepEqual(jm, want) || !reflect.DeepEqual(dm, want) {
		t.Fatalf("maps after the changes:\n json   %+v\n sqlite %+v\n want   %+v", jm, dm, want)
	}
	if jp.Status != PlanCompiled || dp.Status != PlanCompiled || jp.Name != dp.Name {
		t.Fatalf("headers: json %+v, sqlite %+v", jp, dp)
	}
}

// A plan entry whose hash is not in the catalog's form is refused on both engines.
func TestPlans_NonCanonicalHashRefused(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			s := openEngineStore(t, engine, t.TempDir())
			p := s.AddPlan(&Plan{Name: "Move", DestinationRoot: "D:/dest"})
			err := s.ChangePlan(p, PlanChange{Overrides: map[string]string{"not-a-hash": "x"}})
			if err == nil || !strings.Contains(err.Error(), "not a content hash") {
				t.Fatalf("ChangePlan = %v, want a refusal", err)
			}
			if _, err := s.AddPlanErr(&Plan{Name: "Bad", Mapping: map[string]string{strings.ToUpper(fakeHash(1)): "x"}}); err == nil {
				t.Fatal("a plan with an upper-case hash was added")
			}
			if len(s.Plans()) != 1 || len(testPlanMaps(t, s, p.ID).Overrides) != 0 {
				t.Fatal("a refused change was kept")
			}
		})
	}
}

// A drift report's items are stored with it and read when asked for; on the
// database engine they are not kept in memory.
func TestDrift_ItemsReadWhenAskedFor(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			s := openEngineStore(t, engine, dir)
			coll := s.AddCollection("Photos")
			items := []DriftItem{{State: "MISSING", Path: "a.nef", Ext: ".nef", Hash: fakeHash(1), Chunk: "NSP-C00001",
				Volumes: []string{"LTO-1 (vault)"}, NeedsBackup: true}, {State: "NEW", Path: "b.nef", Ext: ".nef"}}
			rep := &DriftReport{At: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC), CollectionID: coll.ID,
				Counts: map[string]int{"missing": 1, "new": 1}, Items: items}
			if err := s.ReplaceDriftReport(rep); err != nil {
				t.Fatal(err)
			}
			rep.Items[0].Path = "changed by the caller"
			if engine == "sqlite" && s.c.Drift[0].Items != nil {
				t.Fatal("the database engine keeps drift items in memory")
			}
			if got := s.DriftReport(coll.ID); got == nil || got.Items != nil || got.Changes() != 2 {
				t.Fatalf("DriftReport = %+v, want the header without items", got)
			}
			want := []DriftItem{{State: "MISSING", Path: "a.nef", Ext: ".nef", Hash: fakeHash(1), Chunk: "NSP-C00001",
				Volumes: []string{"LTO-1 (vault)"}, NeedsBackup: true}, {State: "NEW", Path: "b.nef", Ext: ".nef"}}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s = openEngineStore(t, engine, dir)
			if got := noErr[[]DriftItem](t)(s.DriftItems(coll.ID)); !reflect.DeepEqual(got, want) {
				t.Fatalf("items after reopening: %+v", got)
			}
			full := noErr[*DriftReport](t)(s.DriftReportWithItems(coll.ID))
			if full == nil || !reflect.DeepEqual(full.Items, want) || full.Counts["missing"] != 1 {
				t.Fatalf("DriftReportWithItems = %+v", full)
			}
		})
	}
}

// Comparison records have their own table: dropping the oldest removes one row,
// and the runs read back as saved.
func TestCompareRuns_KeptInTheirTable(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	for i := 1; i <= 3; i++ {
		run := &CompareRun{At: time.Date(2025, 1, i, 0, 0, 0, 0, time.UTC), RefScanID: i, RefLabel: "A", OtherScanID: 10 + i,
			OtherLabel: "B", OtherComplete: true, Counts: map[string]int{ClassSame: i}, Total: i, ResultFile: fmt.Sprintf("compare-%d.json.gz", i),
			ResultSHA256: fakeHash(i)}
		dropped, err := s.AddCompareRun(run, 2)
		if err != nil {
			t.Fatal(err)
		}
		if i == 3 && (len(dropped) != 1 || dropped[0].RefScanID != 1) {
			t.Fatalf("dropped %+v, want the oldest", dropped)
		}
	}
	var rows int
	if err := s.be.(*dbBackend).d.w.QueryRow(`SELECT count(*) FROM compare_run WHERE ref_label = 'A' AND total > 1`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("compare_run holds %d of the kept runs as rows, want 2", rows)
	}
	want := s.CompareRuns()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	if got := s.CompareRuns(); !reflect.DeepEqual(got, want) || len(got) != 2 || got[0].Counts[ClassSame] != 3 {
		t.Fatalf("runs after reopening: %+v", got)
	}
}

// A scan's file list is read file by file, the same files LoadScanFiles reads;
// a list changed by one byte, or cut short, is refused.
func TestScanFiles_StreamedAndChecked(t *testing.T) {
	app := dockApp(t)
	vol := app.Store.AddVolume(Volume{Label: "DRIVE-01", Kind: "HDD"})
	snap := testDrive(vol.ID, 300, 0)
	rec, err := app.recordScan(snap, 5)
	if err != nil {
		t.Fatal(err)
	}
	var streamed []SnapFile
	list, err := app.StreamScanFiles(rec, func(f SnapFile) error {
		streamed = append(streamed, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(streamed, snap.Files) || list.Files != nil || len(list.UnreadableFolders) != 1 {
		t.Fatalf("streamed %d files, gaps %+v", len(streamed), list.UnreadableFolders)
	}
	// The same document decoded whole gives the same list.
	path := filepath.Join(app.scansDir(), rec.FileList)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var whole ScanFileList
	if err := json.NewDecoder(zr).Decode(&whole); err != nil {
		t.Fatal(err)
	}
	list.Files = streamed
	if !reflect.DeepEqual(&whole, list) {
		t.Fatalf("streamed list differs from the whole document")
	}
	for name, damage := range map[string]func([]byte) []byte{
		"one byte changed": func(b []byte) []byte { c := append([]byte(nil), b...); c[len(c)/2] ^= 1; return c },
		"cut short":        func(b []byte) []byte { return b[:len(b)/2] },
	} {
		if err := os.WriteFile(path, damage(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := app.LoadScanFiles(rec); err == nil || !strings.Contains(err.Error(), "does not match its record") {
			t.Errorf("%s: LoadScanFiles = %v, want the list refused as damaged", name, err)
		}
	}
}

// driveScenario runs the same drives, scans, comparisons, cross-drive lists and
// plan on one engine and returns what each produced, without IDs, times or file
// names that differ from run to run.
func driveScenario(t *testing.T, engine string) map[string]any {
	t.Setenv(catalogEngineEnv, engine)
	prev := snapFilePageSize
	snapFilePageSize = 7 // many pages, so reading them is tested too
	defer func() { snapFilePageSize = prev }()
	app := dockApp(t)
	s := app.Store
	coll := s.AddCollection("Photos")
	fo := s.AddFolder(coll.ID, "/srv/photos")
	for i := 0; i < 400; i++ {
		if _, err := s.UpsertFileErr(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: fmt.Sprintf("shoot/%04d/IMG_%06d.NEF", i%37, i),
			SizeBytes: int64(1000 + i), HashAlg: "SHA256", Hash: fakeHash(i), Role: RoleOriginals}); err != nil {
			t.Fatal(err)
		}
	}
	var recs []*ScanRecord
	for k := 0; k < 3; k++ {
		vol := s.AddVolume(Volume{Label: fmt.Sprintf("DRIVE-%02d", k+1), Kind: "HDD", Serial: fmt.Sprintf("SER%d", k+1)})
		snap := testDrive(vol.ID, 400-50*k, 0)
		for i := range snap.Files {
			switch {
			case k == 1 && i%10 == 1:
				snap.Files[i].Hash = fakeHash(i + 5000) // changed
			case k == 1 && i%10 == 2:
				snap.Files[i].RelPath = "moved/" + snap.Files[i].RelPath
			case k == 2 && i%7 == 0:
				snap.Files[i].RelPath = strings.ToUpper(snap.Files[i].RelPath) // case only
			}
		}
		snap.UnreadableFolders = nil
		if k == 2 {
			snap.Scan, snap.UnreadableFolders = ScanPartial, []ScanGap{{RelPath: "shoot/0003", Reason: "denied"}}
		}
		if err := s.PutVolumeSnapshot(snap); err != nil {
			t.Fatal(err)
		}
		rec, err := app.recordScan(snap, 5)
		if err != nil {
			t.Fatal(err)
		}
		recs = append(recs, rec)
	}
	out := map[string]any{}
	results := func(run *CompareRun) []ClassResult {
		res, err := app.LoadCompareResults(run)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	for name, c := range map[string]struct {
		ref   CompareRef
		other int
	}{"scan-scan": {CompareRef{ScanID: recs[0].ID}, recs[1].ID}, "drive-partial": {CompareRef{VolumeID: recs[0].VolumeID}, recs[2].ID},
		"archive": {CompareRef{ArchiveID: coll.ID}, recs[1].ID}} {
		run, err := app.CompareScans(c.ref, c.other)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = map[string]any{"counts": run.Counts, "total": run.Total, "results": results(run)}
	}
	lists, err := app.CrossDriveLists(recs[0].VolumeID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range lists.Scans {
		lists.Scans[i].ScanID, lists.Scans[i].At = 0, time.Time{}
	}
	out["cross-drive"] = lists

	tmpl := s.AddTemplate(&Template{Name: "By role", Routes: map[string]string{RoleOriginals: "photos/{orig_name}"}})
	plan := s.AddPlan(&Plan{Name: "Move", TemplateID: tmpl.ID, ArchiveIDs: []int{coll.ID}, DestinationRoot: t.TempDir()})
	files, err := app.planFiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	out["plan-files"] = files
	if err := app.SetParked(plan.ID, fakeHash(3), true); err != nil {
		t.Fatal(err)
	}
	if err := app.SetOverride(plan.ID, fakeHash(4), "special/four.NEF"); err != nil {
		t.Fatal(err)
	}
	rep, err := app.CompilePlan(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	out["plan-report"] = rep
	cov, err := app.PlanCoverage(s.Plan(plan.ID))
	if err != nil {
		t.Fatal(err)
	}
	out["plan-coverage"] = cov
	m := testPlanMaps(t, s, plan.ID)
	out["plan-maps"] = m
	work, err := app.plansPendingForVolume(recs[0].VolumeID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range work {
		work[i].PlanID = 0
	}
	out["plan-work"] = work
	report, err := app.driveReport(s.VolumeSnapshot(recs[1].VolumeID))
	if err != nil {
		t.Fatal(err)
	}
	for i := range report.Peers {
		report.Peers[i].VolumeID = 0
	}
	report.VolumeID = 0
	out["drive-report"] = report
	return out
}

// Comparisons, the cross-drive lists, the drive report and a plan's files,
// compile report, coverage and maps come out the same on both engines.
func TestDrives_ResultsUnchangedBetweenEngines(t *testing.T) {
	j := driveScenario(t, "json")
	d := driveScenario(t, "sqlite")
	var keys []string
	for k := range j {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		jb, _ := json.Marshal(j[k])
		db, _ := json.Marshal(d[k])
		if !bytes.Equal(jb, db) {
			t.Errorf("%s differs between engines:\n json   %.400s\n sqlite %.400s", k, jb, db)
		}
	}
	if c := j["scan-scan"].(map[string]any)["counts"].(map[string]int); c[ClassChanged] == 0 || c[ClassMoved] == 0 {
		t.Fatalf("the scenario compares nothing interesting: %v", c)
	}
}
