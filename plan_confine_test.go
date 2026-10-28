//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// compiledPlanForTest is a compiled plan whose mapping sends the content of each
// file on the drive at mount to the planned path given for it, the way an
// imported plan arrives (no template is consulted).
func compiledPlanForTest(t *testing.T, app *App, mount, serial string, files map[string]string, planned map[string]string, destRoot string) *Plan {
	t.Helper()
	writeTree(t, mount, files)
	makeSnapshotDrive(t, app, mount, serial, "DRIVE-"+serial)
	mapping := map[string]string{}
	for rel, to := range planned {
		h, err := hashFileHex(filepath.Join(mount, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		mapping[h] = to
	}
	return app.Store.AddPlan(&Plan{Name: "Confine", DestinationRoot: destRoot, Status: PlanCompiled, Mapping: mapping})
}

// A planned path is relative to the plan's destination: one that climbs out of
// it, is rooted, or names a drive is refused, and nothing is written for it,
// inside or outside the destination.
func TestPlan_ExecuteConfinesPlannedPaths(t *testing.T) {
	app := dockApp(t)
	outer := t.TempDir()
	destRoot := createdForTest(t, app, filepath.Join(outer, "dest"))
	mount := t.TempDir()
	files := map[string]string{
		"1.jpg": "ONE", "2.jpg": "TWO", "3.jpg": "THREE", "4.jpg": "FOUR", "5.jpg": "FIVE", "6.jpg": "SIX",
	}
	planned := map[string]string{
		"1.jpg": "../escape1.jpg",
		"2.jpg": "a/../../escape2.jpg",
		"3.jpg": `a\..\..\escape3.jpg`,
		"4.jpg": filepath.ToSlash(filepath.Join(outer, "escape4.jpg")),
		"5.jpg": "C:/escape5.jpg",
		"6.jpg": "ok/six.jpg",
	}
	plan := compiledPlanForTest(t, app, mount, "CONF", files, planned, destRoot)

	r, err := app.ExecutePlanFromDrive(plan.ID, mount, "CONF", func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if r.Copied != 1 || r.Refused != 5 {
		t.Fatalf("copied %d, refused %d: want the one confined path copied and the 5 others refused", r.Copied, r.Refused)
	}
	if got := treeOf(t, outer); !equalStrings(got, []string{"dest", "dest/ok", "dest/ok/six.jpg"}) {
		t.Fatalf("files under the destination's parent: %v, want only dest/ok/six.jpg", got)
	}
	if _, err := os.Stat(`C:\escape5.jpg`); err == nil {
		t.Fatal("a drive-qualified planned path was written")
	}
}

// A file already at a planned path is never replaced: that file is refused and
// left exactly as it was, and its content is not marked as placed.
func TestPlan_ExecuteRefusesExistingFile(t *testing.T) {
	app := dockApp(t)
	destRoot := createdForTest(t, app, filepath.Join(t.TempDir(), "dest"))
	writeTree(t, destRoot, map[string]string{"photos/a.jpg": "SOMEONE ELSE'S"})
	mount := t.TempDir()
	plan := compiledPlanForTest(t, app, mount, "EXIST",
		map[string]string{"a.jpg": "AAA", "b.jpg": "BBB"},
		map[string]string{"a.jpg": "photos/a.jpg", "b.jpg": "photos/b.jpg"}, destRoot)

	r, err := app.ExecutePlanFromDrive(plan.ID, mount, "EXIST", func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if r.Copied != 1 || r.Refused != 1 {
		t.Fatalf("copied %d, refused %d: want b copied and a refused", r.Copied, r.Refused)
	}
	assertFileBytes(t, filepath.Join(destRoot, "photos", "a.jpg"), "SOMEONE ELSE'S")
	assertFileBytes(t, filepath.Join(destRoot, "photos", "b.jpg"), "BBB")
	ha, _ := hashFileHex(filepath.Join(mount, "a.jpg"))
	if app.Store.Plan(plan.ID).Satisfied[ha] {
		t.Fatal("the refused file's content was marked as placed")
	}
}

// An imported plan with a planned or source path that is not confined is
// refused as a whole, before anything is added to the catalog.
func TestPlan_ImportRefusesUnconfinedPaths(t *testing.T) {
	for _, bad := range []ExportPlanFile{
		{SrcRel: "a.jpg", Hash: "h1", Planned: "../a.jpg"},
		{SrcRel: "a.jpg", Hash: "h1", Planned: "/a.jpg"},
		{SrcRel: "a.jpg", Hash: "h1", Planned: `x\..\..\a.jpg`},
		{SrcRel: "../outside.jpg", Hash: "h1", Planned: "a.jpg"},
	} {
		app := dockApp(t)
		plans, vols := len(app.Store.Plans()), len(app.Store.Volumes())
		exp := PlanExport{Format: planFormat, Name: "Imported", DestinationRoot: t.TempDir(),
			Drives: []ExportPlanDrive{{Label: "D", Serial: "S1", Files: []ExportPlanFile{bad}}}}
		if _, err := app.ImportPlan(exp); err == nil {
			t.Errorf("imported a plan with %+v", bad)
		}
		if len(app.Store.Plans()) != plans || len(app.Store.Volumes()) != vols {
			t.Errorf("a refused import of %+v changed the catalog", bad)
		}
	}
}

// A run that is interrupted after placing a file but before recording it leaves
// that file at its planned path. The next run credits it (it is read, and has
// exactly the planned content) instead of refusing it, so execution resumes.
func TestPlan_ExecuteCreditsAFilePlacedButNotRecorded(t *testing.T) {
	app := dockApp(t)
	destRoot := createdForTest(t, app, filepath.Join(t.TempDir(), "dest"))
	mount := t.TempDir()
	plan := compiledPlanForTest(t, app, mount, "RESUME",
		map[string]string{"a.jpg": "AAA", "b.jpg": "BBB"},
		map[string]string{"a.jpg": "photos/a.jpg", "b.jpg": "photos/b.jpg"}, destRoot)
	if _, err := app.ExecutePlanFromDrive(plan.ID, mount, "RESUME", func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	// As if the run stopped before a.jpg was recorded.
	ha, _ := hashFileHex(filepath.Join(mount, "a.jpg"))
	p := app.Store.Plan(plan.ID)
	delete(p.Satisfied, ha)
	p.Status, p.ClosedAt = PlanExecuting, nil
	app.Store.UpdatePlan(p)

	r, err := app.ExecutePlanFromDrive(plan.ID, mount, "RESUME", func(float64, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if r.Refused != 0 || r.Adopted != 1 || r.Coverage.Pct != 100 {
		t.Fatalf("refused %d, adopted %d, coverage %.0f%%: want the placed file credited and the plan complete", r.Refused, r.Adopted, r.Coverage.Pct)
	}
	assertFileBytes(t, filepath.Join(destRoot, "photos", "a.jpg"), "AAA")
}

// Template routes are normalised when the plan's paths are worked out: a
// leading slash, "." parts and doubled slashes mean the same folder inside the
// destination, so the files are placed there. A route that would still leave
// the destination stops the compile, before anything runs.
func TestPlan_RoutesAreNormalisedAtCompile(t *testing.T) {
	app := dockApp(t)
	d1 := t.TempDir()
	writeTree(t, d1, map[string]string{"a.jpg": "AAA", "b.jpg": "BBB"})
	makeSnapshotDrive(t, app, d1, "NORM", "DRIVE-N")
	for route, want := range map[string]string{
		"/Photos/{orig_name}":        "Photos",
		"./out/./{orig_name}":        "out",
		`kept\{orig_name}`:           "kept",
		"deep//./nested/{orig_name}": "deep/nested",
	} {
		tmpl := app.Store.AddTemplate(&Template{Name: "R " + route, Routes: map[string]string{RoleDeliverables: route}})
		dest := createdForTest(t, app, filepath.Join(t.TempDir(), "dest"))
		plan := app.Store.AddPlan(&Plan{Name: "N " + route, TemplateID: tmpl.ID, DestinationRoot: dest})
		if rep, err := app.CompilePlan(plan.ID); err != nil || !rep.Compilable {
			t.Fatalf("route %q: compile %+v, %v", route, rep, err)
		}
		r, err := app.ExecutePlanFromDrive(plan.ID, d1, "NORM", func(float64, string) {})
		if err != nil {
			t.Fatal(err)
		}
		if r.Copied != 2 || r.Refused != 0 {
			t.Errorf("route %q: copied %d, refused %d, want both placed", route, r.Copied, r.Refused)
		}
		assertFileBytes(t, filepath.Join(dest, filepath.FromSlash(want), "a.jpg"), "AAA")
	}

	tmpl := app.Store.AddTemplate(&Template{Name: "Escape", Routes: map[string]string{RoleDeliverables: "../{orig_name}"}})
	plan := app.Store.AddPlan(&Plan{Name: "Escape", TemplateID: tmpl.ID, DestinationRoot: t.TempDir()})
	if _, err := app.CompilePlan(plan.ID); err == nil {
		t.Error("a plan whose route leaves the destination compiled")
	}
	if p := app.Store.Plan(plan.ID); p.Status == PlanCompiled {
		t.Error("the plan is marked compiled")
	}
}
