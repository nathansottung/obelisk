//go:build !guionly

package main

// source_gate_recheck_test.go: a stalled source delays nobody for long, every way
// into a folder is decided against the sources, a package being read is not rebuilt,
// and a refusal leaves nothing behind and says so truthfully.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func shortSourceLimit(t *testing.T) {
	t.Helper()
	prev := sourceLookupLimit
	sourceLookupLimit = 150 * time.Millisecond
	t.Cleanup(func() { sourceLookupLimit = prev })
}

// An unrelated job finishes its decision, and the stalled source stays protected by
// what was last known about it.
func TestGate_StalledSourceDoesNotStopAnUnrelatedDecision(t *testing.T) {
	shortSourceLimit(t)
	app := dockApp(t)
	src := t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	if _, err := app.Store.sourceSnapshot(); err != nil {
		t.Fatal(err)
	}
	full, err := filepath.EvalSymlinks(src)
	if err != nil {
		t.Fatal(err)
	}
	_, unblock := stallSource(t, src)
	defer unblock()
	var got error
	if !finishes(func() {
		r, err := app.Store.reserveOutputPath(t.TempDir())
		if err == nil {
			r()
		}
		got = err
	}) {
		t.Fatal("an unrelated output waited on a stalled source")
	}
	if got != nil {
		t.Fatalf("an unrelated output: %v", got)
	}
	var inside error
	if !finishes(func() {
		r, err := app.Store.reserveOutputPath(filepath.Join(full, "new"))
		if err == nil {
			r()
		}
		inside = err
	}) {
		t.Fatal("the decision waited on a stalled source")
	}
	if inside == nil {
		t.Fatal("an output inside a stalled source was allowed")
	}
}

// With nothing known about a stalled source, a decision is refused, not guessed.
func TestGate_StalledSourceWithNothingKnownRefuses(t *testing.T) {
	shortSourceLimit(t)
	app := dockApp(t)
	src := t.TempDir()
	f := app.Store.addFolderRecord(app.Store.AddCollection("Live").ID, src)
	app.Store.mu.Lock()
	f.VolumeID = ""
	app.Store.mu.Unlock()
	_, unblock := stallSource(t, src)
	defer unblock()
	var got error
	if !finishes(func() {
		r, err := app.Store.reserveOutputPath(t.TempDir())
		if err == nil {
			r()
		}
		got = err
	}) {
		t.Fatal("the decision waited on a stalled source")
	}
	if got == nil || !strings.Contains(got.Error(), "not answering") {
		t.Fatalf("got %v", got)
	}
}

func TestMigrate_TargetInsideASourceIsRefused(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	app.Store.addFolderRecord(app.Store.AddCollection("Live").ID, src)
	target := filepath.Join(src, "newdata")
	if _, err := app.migrateDataDir(app.DataDir, target); err == nil {
		t.Fatal("the data folder was moved into a source")
	}
	assertEmptyFolder(t, src)
}

func TestRegisterSource_AnotherSpellingKeepsOneRecord(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	src := t.TempDir()
	first, err := app.Store.RegisterSource(coll.ID, src)
	if err != nil {
		t.Fatal(err)
	}
	again, err := app.Store.RegisterSource(coll.ID, src+string(filepath.Separator))
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || len(app.Store.sourceFolders()) != 1 {
		t.Fatalf("%d records for one source", len(app.Store.sourceFolders()))
	}
}

// Writes of one package can run together, but a rebuild waits for them.
func TestWrite_HoldsItsPackageAgainstARebuild(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "HELD-WHILE-READ")
	var rebuild, together error
	_, err := app.writeChunk(nil, c.ID, t.TempDir(), 0, 0, 0, 0, noProg2, func() {
		rebuild = app.BuildChunk(c.ID, noProg)
		var second func()
		if second, together = app.Store.shareWork(packageWorkKey(c.ID), "busy"); together == nil {
			second()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if rebuild == nil || !strings.Contains(rebuild.Error(), "already") {
		t.Fatalf("a rebuild during a write: %v", rebuild)
	}
	if together != nil {
		t.Fatalf("a second write of the package: %v", together)
	}
	after := app.Store.Chunk(c.ID)
	after.Status = "FAILED"
	app.Store.UpdateChunk(after)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("a rebuild after the writes: %v", err)
	}
}

func noProg2(float64, string) {}

func TestSpanWrite_RefusedWhileItsPackageIsBeingBuilt(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "SPAN-HELD")
	c.Spanned = true
	c.Segments = []Segment{{Status: "PENDING"}}
	app.Store.UpdateChunk(c)
	release, err := app.Store.claimWork(packageWorkKey(c.ID), "busy")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := app.SpanWriteNext(c.ID, t.TempDir(), 0, 0, 0, 0, noProg2); err == nil || !strings.Contains(err.Error(), "already") {
		t.Fatalf("got %v", err)
	}
}

// A keystore set already in use keeps working with a warning if its disks cannot be
// read; a set being entered is still refused.
func TestKeystores_UnknownDiskInUseWarnsAndStillWorks(t *testing.T) {
	if !diskLookupSupported {
		t.Skip("this system does not read disks")
	}
	a, b := t.TempDir(), t.TempDir()
	prevDev, prevDisks, prevNet, prevVol := keystoreDeviceOf, keystoreDisks, keystoreOnNetwork, keystoreVolumeOf
	t.Cleanup(func() {
		keystoreDeviceOf, keystoreDisks, keystoreOnNetwork, keystoreVolumeOf = prevDev, prevDisks, prevNet, prevVol
	})
	keystoreDeviceOf = realKeystoreDevice
	keystoreOnNetwork = func(string) bool { return false }
	keystoreDisks = func(string) ([]string, bool) { return nil, false }
	keystoreVolumeOf = func(p string) (string, error) { return p, nil }
	app := dockApp(t)
	paths := []string{filepath.Join(a, "k.json"), filepath.Join(b, "k.json")}
	if err := checkKeystorePaths(paths); err == nil {
		t.Fatal("a new set with unknown disks was accepted")
	}
	st := app.keystoreStatus(paths, true)
	if st["ok"] != true {
		t.Fatalf("a set in use stopped working: %v", st["reason"])
	}
	if w, _ := st["warning"].(string); !strings.Contains(w, "could not be read") {
		t.Fatalf("no warning: %q", w)
	}
	keystoreVolumeOf = func(string) (string, error) { return "one-volume", nil }
	if st := app.keystoreStatus(paths, true); st["ok"] == true {
		t.Fatal("two keystores on one volume passed")
	}
}

// If the private manifest cannot be moved aside, the burn must not run.
func TestBurn_StopsWhenThePrivateManifestCannotBeMoved(t *testing.T) {
	burnWithFailedMove(t, os.ErrPermission)
}

// A staging folder that is gone is a failed move, not a manifest that is not there.
func TestBurn_StopsWhenTheManifestHasNowhereToGo(t *testing.T) {
	burnWithFailedMove(t, os.ErrNotExist)
}

// Decisions never go back to a stalled drive, whichever path they take.
func TestGate_NewFolderDecisionDoesNotTouchAStalledSource(t *testing.T) {
	shortSourceLimit(t)
	app := dockApp(t)
	src := t.TempDir()
	f := app.Store.addFolderRecord(app.Store.AddCollection("Live").ID, src)
	app.Store.mu.Lock()
	f.VolumeID = "another-volume" // so the volume rule does not answer first
	app.Store.mu.Unlock()
	_, unblock := stallSource(t, src)
	defer unblock()
	release := make(chan struct{})
	defer close(release)
	prev := sourceCanonicalOfMissing
	sourceCanonicalOfMissing = func(p string) string {
		if strings.EqualFold(p, src) {
			<-release
		}
		return prev(p)
	}
	t.Cleanup(func() { sourceCanonicalOfMissing = prev })
	if !finishes(func() {
		if r, err := app.Store.reserveOutputPath(filepath.Join(t.TempDir(), "new")); err == nil {
			r()
		}
	}) {
		t.Fatal("deciding a new folder went back to the stalled source")
	}
}

func TestQuarantine_DoesNotTouchAStalledSource(t *testing.T) {
	shortSourceLimit(t)
	app := dockApp(t)
	src := t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	if _, err := app.Store.sourceSnapshot(); err != nil {
		t.Fatal(err)
	}
	full, err := filepath.EvalSymlinks(src)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "unrelated.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	defer close(release)
	prev := sourceIdentityOf
	sourceIdentityOf = func(p string) (fileIdent, error) {
		if normPath(p) == normPath(full) {
			<-release
		}
		return prev(p)
	}
	t.Cleanup(func() { sourceIdentityOf = prev })
	if !finishes(func() { _, _ = app.Store.resolveQuarantine(file) }) {
		t.Fatal("quarantining an unrelated file waited on a stalled source")
	}
}

func burnWithFailedMove(t *testing.T, moveErr error) {
	t.Helper()
	app, qid, _ := burnSetupForTest(t, "Disc")
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	c := app.Store.Chunk(app.Store.BurnQueue(qid).Discs[0].ChunkID)
	c.PrivateManifest = true
	app.Store.UpdateChunk(c)
	plain := filepath.Join(c.StagedDir, c.Name+".manifest.json")
	if _, err := os.Stat(plain); err != nil {
		if err := os.WriteFile(plain, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prevMove, prevBurn := moveManifestAside, runBurnProgram
	moveManifestAside = func(string, string) error { return moveErr }
	runBurnProgram = func([]string) error {
		t.Error("the burn ran with the plaintext manifest in its folder")
		return errStopBurnForTest
	}
	t.Cleanup(func() { moveManifestAside, runBurnProgram = prevMove, prevBurn })
	if _, err := app.BurnNext(qid, func(float64, string) {}); err == nil {
		t.Fatal("the burn did not stop")
	}
}

// failingRemove lets the attempt create and rename, but never delete.
type failingRemove struct{ pathProbe }

func (failingRemove) Remove(string) error { return os.ErrPermission }

// A test file that could not be removed is named, and "nothing was written" is not said.
func TestWriteAccess_LeftoverProbeIsNamed(t *testing.T) {
	dir := t.TempDir()
	err := probeResult(dir, destWriteUse, runWriteProbe(failingRemove{pathProbe{dir}}))
	var wa *WriteAccessError
	if !errors.As(err, &wa) {
		t.Fatalf("got %v", err)
	}
	if strings.Contains(err.Error(), "Nothing was written") {
		t.Errorf("the refusal denies the file it left: %v", err)
	}
	entries, rerr := os.ReadDir(dir)
	if rerr != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, rerr)
	}
	if !strings.Contains(err.Error(), entries[0].Name()) {
		t.Errorf("the refusal does not name %s: %v", entries[0].Name(), err)
	}
}

// Folders made for an output that is then refused are removed again.
func TestWriteAccess_UnguardedRefusalRemovesWhatItMade(t *testing.T) {
	withoutDestGuard(t)
	app := dockApp(t)
	parent := t.TempDir()
	failProbe(t)
	o, err := app.openOutFolder(filepath.Join(parent, "a", "b"))
	if err == nil {
		o.Close()
		t.Fatal("the output folder was opened")
	}
	assertEmptyFolder(t, parent)
}

func TestWriteAccess_GuardedRefusalRemovesWhatItMade(t *testing.T) {
	if !destGuardAvailable {
		t.Skip("no destination guard here")
	}
	app := dockApp(t)
	parent := t.TempDir()
	target := filepath.Join(parent, "new")
	prev := writeAccessFacts
	writeAccessFacts = func(p string, newFolder bool) (writeFacts, error) {
		if strings.EqualFold(filepath.Base(p), "new") {
			return writeFacts{CannotChange: true}, nil
		}
		return prev(p, newFolder)
	}
	t.Cleanup(func() { writeAccessFacts = prev })
	in := app.Store.CheckDestination(target)
	c, err := app.Store.CreateDestination(target, in.Folder, true)
	if err == nil && c.Status != destStatusRefuse {
		t.Fatalf("status %q", c.Status)
	}
	assertEmptyFolder(t, parent)
}

// A keystore in a folder not made yet gets the real attempt before any is written,
// and the folders made for it are removed if the set is refused.
func TestWriteAccess_KeystoreInNewFolderIsProved(t *testing.T) {
	app := dockApp(t)
	first, second := t.TempDir(), t.TempDir()
	calls := 0
	prev := writeProbeStep
	writeProbeStep = func(s string) error {
		if s == "create" {
			if calls++; calls == 2 {
				return os.ErrPermission
			}
		}
		return nil
	}
	t.Cleanup(func() { writeProbeStep = prev })
	r, err := app.Store.reserveOutputFiles([]string{
		filepath.Join(first, "new", "k.json"), filepath.Join(second, "new", "deep", "k.json")})
	if err == nil {
		r()
		t.Fatal("the keystores were reserved")
	}
	assertWriteRefusal(t, err, "cannot change")
	assertEmptyFolder(t, first)
	assertEmptyFolder(t, second)
}

// Every member of a multi-device Btrfs volume counts as a disk under it.
func TestBtrfsMembers_NamesEveryDevice(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"11-aa/devices/sdb1", "11-aa/devices/sdc", "22-bb/devices/sdd1"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := btrfsMembers(root, "sdb1")
	if len(got) != 2 || got[0] != "sdb1" || got[1] != "sdc" {
		t.Fatalf("got %v", got)
	}
	if got := btrfsMembers(root, "sde1"); got != nil {
		t.Fatalf("a device on no Btrfs volume: %v", got)
	}
}
