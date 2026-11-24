//go:build !guionly

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// keptScenario is an archive of two files mirrored once to a volume, with one
// source file changed since and a file of someone else's at the other's path.
func keptScenario(t *testing.T) (app *App, collID int, vol *Volume, src, dest string) {
	t.Helper()
	return keptScenarioAt(t, t.TempDir())
}

// keptScenarioAt is keptScenario mirroring into dest.
func keptScenarioAt(t *testing.T, dest string) (app *App, collID int, vol *Volume, src, _ string) {
	t.Helper()
	app = dockApp(t)
	src = t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "alpha v1\n", "sub/b.txt": "bravo\n"})
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProgMirror); err != nil {
		t.Fatal(err)
	}
	vol = app.Store.AddVolume(Volume{Label: "MIR-K", Kind: "HDD"})
	if _, err := app.MirrorToVolume(coll.ID, nil, dest, vol.ID, 0, "", noProgMirror); err != nil {
		t.Fatal(err)
	}
	// The source changes and is scanned again; someone puts their own file where
	// the other one was mirrored.
	writeTree(t, src, map[string]string{"a.txt": "alpha v2, changed\n"})
	if _, _, err := app.ScanFolder(coll.ID, src, noProgMirror); err != nil {
		t.Fatal(err)
	}
	return app, coll.ID, vol, src, dest
}

func keptRunDir(t *testing.T, dest string) string {
	t.Helper()
	es, err := os.ReadDir(filepath.Join(dest, mirrorPreviousDir))
	if err != nil || len(es) != 1 {
		t.Fatalf("want one kept-version run on the drive, got %v (%v)", es, err)
	}
	return filepath.Join(dest, mirrorPreviousDir, es[0].Name())
}

func runStatus(t *testing.T, run string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(run, "RUN.json"))
	if err != nil {
		t.Fatalf("RUN.json: %v", err)
	}
	var m runMarker
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m.Status
}

// A mirror never destroys the older version: it is kept under
// _obelisk-previous, recorded as an earlier version and not counted as a copy,
// and a file that was not Obelisk's own is reported apart. An identical file is
// left alone.
func TestMirrorKeep_KeepsTheOlderVersion(t *testing.T) {
	for _, noLinks := range []bool{false, true} { // true: as on exFAT, which has no links
		name := "links"
		if noLinks {
			name = "no links (exFAT)"
		}
		t.Run(name, func(t *testing.T) {
			mirrorNoLinks = noLinks
			t.Cleanup(func() { mirrorNoLinks = false })
			app, collID, vol, _, dest := keptScenario(t)
			mustWrite(t, filepath.Join(dest, "sub", "b.txt"), "someone else's bravo\n")
			bInfo, _ := os.Stat(filepath.Join(dest, "sub", "b.txt"))
			unchanged := filepath.Join(dest, "sub", "b.txt")
			_ = unchanged

			res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
			if err != nil {
				t.Fatal(err)
			}
			if got := mustRead(t, filepath.Join(dest, "a.txt")); got != "alpha v2, changed\n" {
				t.Errorf("the new copy is not in place: %q", got)
			}
			run := keptRunDir(t, dest)
			if got := mustRead(t, filepath.Join(run, "a.txt")); got != "alpha v1\n" {
				t.Errorf("the earlier version was not kept: %q", got)
			}
			if got := mustRead(t, filepath.Join(run, "sub", "b.txt")); got != "someone else's bravo\n" {
				t.Errorf("the other file there was not kept: %q", got)
			}
			if len(res.KeptOwn) != 1 || res.KeptOwn[0] != "a.txt" || len(res.KeptOther) != 1 || res.KeptOther[0] != "sub/b.txt" {
				t.Errorf("own and other kept files not reported apart: own %v, other %v", res.KeptOwn, res.KeptOther)
			}
			if res.KeptBytes == 0 || res.KeptRun == "" {
				t.Errorf("the report does not say how much is kept or where: %+v", res.KeptSummary)
			}
			if runStatus(t, run) != "complete" {
				t.Error("a finished run is not marked complete")
			}
			_ = bInfo
			// Recorded as earlier versions, with where they went.
			var session *BackupSession
			for _, s := range app.Store.KeptVersionRuns(collID) {
				if s.Base == "kept" {
					session = s
				}
			}
			if session == nil || session.Status != "complete" || len(session.KeptVersions) != 2 {
				t.Fatalf("the catalog does not record the kept versions: %+v", session)
			}
			for _, k := range session.KeptVersions {
				if !strings.HasPrefix(k.KeptRel, mirrorPreviousDir+"/") || k.Hash == "" {
					t.Errorf("kept version recorded without where it went: %+v", k)
				}
			}
			// Never a current copy: the mirror record holds only the new content.
			for _, c := range mirrorChunksFor(app, collID, vol.ID) {
				refs, _ := app.Store.ChunkFiles(c.ID)
				for _, r := range refs {
					for _, k := range session.KeptVersions {
						if r.Hash == k.Hash {
							t.Errorf("a kept version counts as a copy: %+v", r)
						}
					}
				}
			}
		})
	}
}

// A file already there with the same content is left alone, not rewritten.
func TestMirrorKeep_IdenticalFileLeftAlone(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "same\n"})
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProgMirror); err != nil {
		t.Fatal(err)
	}
	vol := app.Store.AddVolume(Volume{Label: "MIR-S", Kind: "HDD"})
	dest := t.TempDir()
	mustWrite(t, filepath.Join(dest, "a.txt"), "same\n")
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(dest, "a.txt"), old, old)
	res, err := app.MirrorToVolume(coll.ID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(filepath.Join(dest, "a.txt"))
	if !fi.ModTime().Equal(old) || res.Identical != 1 || res.Mirrored != 1 {
		t.Errorf("an identical file was rewritten or not credited: mtime %v, %+v", fi.ModTime(), res.KeptSummary)
	}
	if _, err := os.Stat(filepath.Join(dest, mirrorPreviousDir)); err == nil {
		t.Error("an identical file was kept as a previous version")
	}
}

// A crash at either step leaves the path with one version or the other, the
// earlier version findable in the run's folder, and the run marked incomplete
// on the drive and in the catalog.
func TestMirrorKeep_CrashAtEachStep(t *testing.T) {
	for _, noLinks := range []bool{false, true} {
		for _, step := range []string{"kept", "published"} {
			t.Run(step, func(t *testing.T) {
				mirrorNoLinks = noLinks
				t.Cleanup(func() { mirrorNoLinks = false })
				app, collID, vol, _, dest := keptScenario(t)
				mirrorKeepCrash = func(s string) bool { return s == step }
				_, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
				mirrorKeepCrash = nil
				if err == nil {
					t.Fatal("setup: the crash was not injected")
				}
				got := mustRead(t, filepath.Join(dest, "a.txt"))
				if got != "alpha v1\n" && got != "alpha v2, changed\n" {
					t.Fatalf("the path holds neither version: %q", got)
				}
				if step == "kept" && got != "alpha v1\n" {
					t.Error("a crash before the publish changed the path")
				}
				run := keptRunDir(t, dest)
				if kept := mustRead(t, filepath.Join(run, "a.txt")); kept != "alpha v1\n" {
					t.Errorf("the earlier version is not findable in the run's folder: %q", kept)
				}
				if runStatus(t, run) != "incomplete" {
					t.Error("the interrupted run is not marked incomplete on the drive")
				}
				var incomplete bool
				for _, s := range app.Store.KeptVersionRuns(collID) {
					incomplete = incomplete || s.Base == "kept" && s.Status == "incomplete"
				}
				if !incomplete {
					t.Error("the interrupted run is not marked incomplete in the catalog")
				}
			})
		}
	}
}

// A kept path too long to make refuses the run before anything changes.
func TestMirrorKeep_KeptPathTooLongRefusesTheRun(t *testing.T) {
	app, collID, vol, _, dest := keptScenario(t)
	keptPathLimit = len(filepath.Join(dest, "a.txt")) + 5 // the kept path is longer
	t.Cleanup(func() { keptPathLimit = 0 })
	_, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("a run whose kept path cannot be made: %v; want a refusal", err)
	}
	if got := mustRead(t, filepath.Join(dest, "a.txt")); got != "alpha v1\n" {
		t.Errorf("the refused run changed the file: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, mirrorPreviousDir)); err == nil {
		t.Error("the refused run made a kept-version folder")
	}
}

// A run whose new copies and kept versions will not fit is refused first.
func TestMirrorKeep_SpaceForKeptVersionsIsCheckedFirst(t *testing.T) {
	app, collID, vol, _, dest := keptScenario(t)
	keepDiskFree = func(string) (int64, error) { return 10, nil }
	t.Cleanup(func() { keepDiskFree = diskFree })
	_, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err == nil || !strings.Contains(err.Error(), "keeping the earlier version") {
		t.Fatalf("a run without room for kept versions: %v; want a refusal", err)
	}
	if got := mustRead(t, filepath.Join(dest, "a.txt")); got != "alpha v1\n" {
		t.Errorf("the refused run changed the file: %q", got)
	}
}

// The mirror's own drive walks skip the kept folder: it is never listed as a
// file on the drive, so it is never a stray and never copied back.
func TestMirrorKeep_DriveWalksSkipTheKeptFolder(t *testing.T) {
	app, collID, vol, _, dest := keptScenario(t)
	if _, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror); err != nil {
		t.Fatal(err)
	}
	if !skipDockDir(mirrorPreviousDir) || !cardSkipDir(mirrorPreviousDir) {
		t.Fatal("the drive walks do not skip the kept folder")
	}
	cfg, _ := app.LoadConfig()
	files, _, _, err := app.scanTree(dest, cfg, func(int, int) {}, func(SnapFile) {})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f.RelPath, mirrorPreviousDir) {
			t.Errorf("a drive scan lists a kept version: %s", f.RelPath)
		}
	}
	// A second mirror does not copy kept versions anywhere or keep them again.
	if _, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror); err != nil {
		t.Fatal(err)
	}
	if es, _ := os.ReadDir(filepath.Join(dest, mirrorPreviousDir)); len(es) != 1 {
		t.Errorf("a second identical mirror kept something again: %d runs", len(es))
	}
}

// Removing a run's kept versions goes only through the gate, with "Delete".
func TestMirrorKeep_RemovedOnlyThroughTheGate(t *testing.T) {
	app, collID, vol, _, dest := keptScenario(t)
	if _, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror); err != nil {
		t.Fatal(err)
	}
	run := keptRunDir(t, dest)
	runs, total, err := app.MirrorKeptRuns(dest)
	if err != nil || len(runs) != 1 || total == 0 {
		t.Fatalf("kept runs not listed with their size: %v %d %v", runs, total, err)
	}
	p := prepareForTest(t, app, "mirror-previous-remove", map[string]any{"dest": dest, "run": filepath.Base(run)})
	if _, err := app.ExecuteDestructive(testSession, p.Token, "Delete"); err != nil {
		t.Fatal(err)
	}
	scratchGone(t, run, "kept versions after Delete")
	scratchKept(t, filepath.Join(dest, "a.txt"), "the current copy")
}
