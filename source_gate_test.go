//go:build !guionly

package main

// source_gate_test.go: a folder cannot be a source and a live output at once.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// tryRegisterSource registers path as a source the way a scan does.
func tryRegisterSource(s *Store, collectionID int, path string) error {
	_, err := s.RegisterSource(collectionID, path)
	return err
}

// overlapLayout is an output folder and each kind of source that overlaps it.
type overlapLayout struct {
	out     string
	sources map[string]string
}

func newOverlapLayout(t *testing.T) overlapLayout {
	t.Helper()
	base := t.TempDir()
	out := filepath.Join(base, "out")
	child := filepath.Join(out, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, alias, out)
	return overlapLayout{out: out, sources: map[string]string{
		"the same folder":             out,
		"a parent":                    base,
		"a child":                     child,
		"another name for the folder": alias,
	}}
}

// While an output is held, registering an overlapping source and writing to the
// output must not both succeed.
func TestLiveOutput_OverlappingSourceCannotProceed(t *testing.T) {
	for _, name := range []string{"the same folder", "a parent", "a child", "another name for the folder"} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			l := newOverlapLayout(t)
			coll := app.Store.AddCollection("Live")
			o, err := app.openOutFolder(l.out)
			if err != nil {
				t.Fatal(err)
			}
			defer o.Close()
			regErr := tryRegisterSource(app.Store, coll.ID, l.sources[name])
			writeErr := o.WriteFile("written-after.txt", []byte("output"), 0o644)
			if regErr == nil && writeErr == nil {
				t.Fatalf("%s became a source while the output was live, and the output was still written", name)
			}
		})
	}
}

// Once refused, nothing is recorded, the output still works, and the source can be
// added when the output is released.
func TestLiveOutput_RefusedSourceAddsNothingAndWorksAfterRelease(t *testing.T) {
	app := dockApp(t)
	l := newOverlapLayout(t)
	coll := app.Store.AddCollection("Live")
	o, err := app.openOutFolder(l.out)
	if err != nil {
		t.Fatal(err)
	}
	err = tryRegisterSource(app.Store, coll.ID, l.out)
	if err == nil || !strings.Contains(err.Error(), "a source and a live output") {
		t.Fatalf("registering the live output as a source: %v", err)
	}
	if got := app.Store.SourceRoots(); len(got) != 0 {
		t.Fatalf("a refused source was recorded: %v", got)
	}
	if err := o.WriteFile("still-written.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("the output stopped working: %v", err)
	}
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tryRegisterSource(app.Store, coll.ID, l.out); err != nil {
		t.Fatalf("after the output was released: %v", err)
	}
}

// A real scan is the path users take, and it refuses too.
func TestLiveOutput_ScanOfALiveOutputIsRefused(t *testing.T) {
	app := dockApp(t)
	l := newOverlapLayout(t)
	coll := app.Store.AddCollection("Live")
	o, err := app.openOutFolder(l.out)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	if _, _, err := app.ScanFolder(coll.ID, l.out, func(float64, string) {}); err == nil || !strings.Contains(err.Error(), "live output") {
		t.Fatalf("scan of a live output: %v", err)
	}
	if got := app.Store.SourceRoots(); len(got) != 0 {
		t.Fatalf("the scan registered a source: %v", got)
	}
}

// Every other writer that holds a folder keeps it reserved for as long as it runs.
func TestLiveOutput_PathWritersReserveTheirFolder(t *testing.T) {
	app := dockApp(t)
	l := newOverlapLayout(t)
	coll := app.Store.AddCollection("Live")
	release, err := app.Store.reserveOutputPath(l.out)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryRegisterSource(app.Store, coll.ID, l.sources["a child"]); err == nil {
		t.Fatal("a source inside a reserved staging folder was accepted")
	}
	release()
	release() // safe twice
	if n := app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("%d outputs still live", n)
	}
	if err := tryRegisterSource(app.Store, coll.ID, l.sources["a child"]); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Store.reserveOutputPath(l.out); err == nil {
		t.Fatal("a folder holding a source was reserved as an output")
	}
}

// Without the guard (macOS), output folders and new restore folders reserve too.
func TestLiveOutput_UnguardedWritersReserve(t *testing.T) {
	withoutDestGuard(t)
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	out := filepath.Join(t.TempDir(), "out")
	o, err := app.openOutFolder(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryRegisterSource(app.Store, coll.ID, out); err == nil {
		t.Error("a source over an unguarded output was accepted")
	}
	o.Close()
	if n := app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("closing the output left %d live", n)
	}
	restoreDir := filepath.Join(t.TempDir(), "restored")
	tgt, err := app.openRestoreTarget(restoreDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryRegisterSource(app.Store, coll.ID, restoreDir); err == nil {
		t.Error("a source over a new restore folder was accepted")
	}
	app.closeRestoreTarget(tgt, true)
	if n := app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("closing the restore left %d live", n)
	}
}

// A restore or move replaces the catalog's sources, so it waits for live outputs.
func TestLiveOutput_CatalogSwapWaits(t *testing.T) {
	app := dockApp(t)
	l := newOverlapLayout(t)
	o, err := app.openOutFolder(l.out)
	if err != nil {
		t.Fatal(err)
	}
	if _, done, err := app.holdForSwap("a test swap"); err == nil {
		done()
		t.Fatal("the catalog could be swapped under a live output")
	}
	o.Close()
	_, done, err := app.holdForSwap("a test swap")
	if err != nil {
		t.Fatalf("after the output closed: %v", err)
	}
	done()
}

// Two requests racing on one folder: never both.
func TestLiveOutput_RaceHasOneWinner(t *testing.T) {
	for i := 0; i < 25; i++ {
		app := dockApp(t)
		l := newOverlapLayout(t)
		coll := app.Store.AddCollection("Live")
		var wg sync.WaitGroup
		var out *outFolder
		var outErr, srcErr error
		wg.Add(2)
		go func() { defer wg.Done(); out, outErr = app.openOutFolder(l.out) }()
		go func() { defer wg.Done(); srcErr = tryRegisterSource(app.Store, coll.ID, l.out) }()
		wg.Wait()
		if out != nil {
			out.Close()
		}
		if (outErr == nil) == (srcErr == nil) {
			t.Fatalf("round %d: output error %v, source error %v; exactly one must win", i, outErr, srcErr)
		}
	}
}

// The normal path: unrelated sources and outputs do not get in each other's way.
func TestLiveOutput_UnrelatedFoldersStillWork(t *testing.T) {
	app := dockApp(t)
	base := t.TempDir()
	outA, outB, src, sibling := filepath.Join(base, "outA"), filepath.Join(base, "outB"), filepath.Join(base, "source"), filepath.Join(base, "outAB")
	for _, d := range []string{outA, outB, src, sibling} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	coll := app.Store.AddCollection("Live")
	a, err := app.openOutFolder(outA)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := tryRegisterSource(app.Store, coll.ID, src); err != nil {
		t.Fatalf("an unrelated source: %v", err)
	}
	// A name that merely starts with the output's name is a different folder.
	if err := tryRegisterSource(app.Store, coll.ID, sibling); err != nil {
		t.Fatalf("a folder whose name starts the same: %v", err)
	}
	if err := a.WriteFile("f.txt", []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A source whose drive is away blocks only an output on that drive.
func TestLiveOutput_OfflineSourceBlocksOnlyItsOwnVolume(t *testing.T) {
	app := dockApp(t)
	out := t.TempDir()
	here := pathVolumeID(out)
	if here == "" {
		t.Skip("this filesystem reports no volume identity")
	}
	coll := app.Store.AddCollection("Live")
	f := app.Store.AddFolder(coll.ID, filepath.Join(unattachedPathForTest(t), "Photos"))
	other := filepath.Join(out, "elsewhere")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	setFolderVolumeForTest(app.Store, f.ID, "volume-that-is-away")
	d, err := app.Store.AuthorizeDestination(other)
	if err != nil {
		t.Fatalf("an output on another volume was blocked by an offline source: %v", err)
	}
	d.Close()
	// The same source recorded on this output's volume blocks it. Decided directly:
	// on Linux, a path under / cannot be made to look like a detached drive.
	own := &sourceSnapshot{srcs: []sourceRef{{registered: f.Path, offline: true, volumeID: here, abs: f.Path}}}
	if err := own.decideOffline(here, other, other, other); err == nil {
		t.Fatal("an output on the offline source's own volume was allowed")
	}
}

func setFolderVolumeForTest(s *Store, id int, volume string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.c.Folders {
		if f.ID == id {
			f.VolumeID = volume
		}
	}
}

// The only code that may add to the catalog's list of sources is the gate, the
// catalog loader, and the disposable preview catalog.
func TestSourcePaths_EveryAdditionGoesThroughTheGate(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]int{
		"source_gate.go":    1, // insertSource
		"catalogdb_rows.go": 1, // loading the catalog
		"gui_inventory.go":  1, // a disposable catalog of its own
		"store.go":          1, // forgetting an archive removes its folders
	}
	adds := regexp.MustCompile(`\.Folders\s*=`)
	calls := regexp.MustCompile(`\.insertSource\(`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if n := len(adds.FindAll(b, -1)); n != allowed[file] {
			t.Errorf("%s assigns the source list %d time(s), want %d: a new path must use RegisterSource", file, n, allowed[file])
		}
		want := 0
		if file == "source_gate.go" {
			want = 1
		}
		if n := len(calls.FindAll(b, -1)); n != want {
			t.Errorf("%s calls insertSource %d time(s), want %d", file, n, want)
		}
	}
}

// A source registered after the output was checked, and before it is held, is seen.
func TestAuthorize_SeesASourceRegisteredSinceItsCheck(t *testing.T) {
	for _, name := range []string{"the same folder", "a parent", "a child", "another name for the folder"} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			l := newOverlapLayout(t)
			coll := app.Store.AddCollection("Live")
			destBeforeReserve = func() {
				destBeforeReserve = nil
				if err := tryRegisterSource(app.Store, coll.ID, l.sources[name]); err != nil {
					t.Errorf("registering before the output is held: %v", err)
				}
			}
			t.Cleanup(func() { destBeforeReserve = nil })
			d, err := app.Store.AuthorizeDestination(l.out)
			if err == nil {
				d.Close()
				t.Fatalf("the output was authorized although %s had become a source", name)
			}
		})
	}
}
