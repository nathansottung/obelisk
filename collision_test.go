//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A plan that would put two different files at one path (the same name, or
// names that differ only in letter case or Unicode form) is not compiled.
func TestPlan_TwoFilesOnePathIsRefused(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"same name":  {"x/a.jpg": "A1", "y/a.jpg": "A2"},
		"case only":  {"x/a.jpg": "A1", "y/A.JPG": "A2"},
		"NFC vs NFD": {"x/café.jpg": "A1", "y/café.jpg": "A2"},
	} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			d := t.TempDir()
			writeTree(t, d, files)
			makeSnapshotDrive(t, app, d, "SER-C", "DRIVE-C")
			tmpl := app.Store.AddTemplate(&Template{Name: "Flat", Routes: map[string]string{RoleDeliverables: "photos/{orig_name}"}})
			plan := app.Store.AddPlan(&Plan{Name: "Flat move", TemplateID: tmpl.ID, DestinationRoot: filepath.Join(t.TempDir(), "NAS")})
			_, err := app.CompilePlan(plan.ID)
			if err == nil || !strings.Contains(err.Error(), "same path") {
				t.Errorf("compile: %v, want a refusal naming the shared path", err)
			}
			if p := app.Store.Plan(plan.ID); p.Status == PlanCompiled {
				t.Error("the plan was compiled")
			}
		})
	}
}

// A mirror that would put two different files at one path (here, two source
// folders whose names differ only in letter case) copies neither and credits
// neither, and says so.
func TestMirror_TwoFilesOnePathNeitherCredited(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Photos")
	a := filepath.Join(t.TempDir(), "Photos")
	b := filepath.Join(t.TempDir(), "photos")
	writeTree(t, a, map[string]string{"x.jpg": "FIRST", "only-a.jpg": "A"})
	writeTree(t, b, map[string]string{"x.jpg": "SECOND"})
	for _, src := range []string{a, b} {
		if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
			t.Fatal(err)
		}
	}
	dest := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "MIR-1", Kind: "HDD"})
	res, err := app.MirrorToVolume(coll.ID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatal(err)
	}
	if res.Mirrored != 1 || len(res.Collisions) != 2 {
		t.Errorf("mirrored %d, collisions %v; want only-a.jpg mirrored and both x.jpg reported", res.Mirrored, res.Collisions)
	}
	for _, r := range mirrorRefsOn(app, coll.ID, vol.ID) {
		if strings.HasSuffix(r, "x.jpg") {
			t.Errorf("a colliding file was credited: %s", r)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "Photos", "x.jpg")); err == nil {
		if b, _ := os.ReadFile(filepath.Join(dest, "Photos", "x.jpg")); string(b) == "SECOND" {
			t.Error("the second file overwrote the first")
		}
	}
}

// Two folder adopts on one drive keep separate records, and a dock of the whole
// drive keeps its own: none replaces another's files.
func TestAdoptFolder_SeparateRecordsPerFolder(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"W2019/a.jpg": "A", "W2020/b.jpg": "B"})
	vol := app.Store.AddVolume(Volume{Label: "OLD-9", Kind: "HDD"})
	for _, f := range []string{"W2019", "W2020"} {
		if _, err := app.AdoptFolder(filepath.Join(drive, f), arch.ID, vol.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	n := 0
	for _, c := range app.Store.Chunks(arch.ID) {
		if c.Mirror {
			n++
			for _, r := range c.Files {
				got[r.RelPath] = true
			}
		}
	}
	if n != 2 || !got["a.jpg"] || !got["b.jpg"] {
		t.Errorf("%d mirror records with %v; want one per folder, each with its file", n, got)
	}
	if cov := app.archiveCoverage([]int{arch.ID}); cov.CoveredFiles != 2 {
		t.Errorf("coverage %d of 2", cov.CoveredFiles)
	}
}
