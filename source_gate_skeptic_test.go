//go:build !guionly

package main

// source_gate_skeptic_test.go: the gate does no disk work while locked, one package
// or queue has one worker, and identity is compared for outputs not there yet and
// for keystores.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// stallSource makes opening src block until the returned function is called.
func stallSource(t *testing.T, src string) (entered <-chan struct{}, unblock func()) {
	t.Helper()
	in, out := make(chan struct{}), make(chan struct{})
	var inOnce, outOnce sync.Once
	prev := openSourceRoot
	openSourceRoot = func(p string) (*os.File, error) {
		if strings.EqualFold(p, src) {
			inOnce.Do(func() { close(in) })
			<-out
		}
		return prev(p)
	}
	unblock = func() { outOnce.Do(func() { close(out) }) }
	t.Cleanup(func() { unblock(); openSourceRoot = prev })
	return in, unblock
}

func finishes(fn func()) bool {
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(3 * time.Second):
		return false
	}
}

// A source drive that stops answering delays only the job that is looking at it.
func TestGate_AStalledSourceDoesNotHoldUpOtherJobs(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	stalled := t.TempDir()
	app.Store.AddFolder(coll.ID, stalled)
	first, err := app.Store.reserveOutputPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entered, unblock := stallSource(t, stalled)
	waiting := make(chan struct{})
	go func() {
		defer close(waiting)
		if r, err := app.Store.reserveOutputPath(t.TempDir()); err == nil {
			r()
		}
	}()
	<-entered
	if !finishes(first) {
		unblock()
		<-waiting
		t.Fatal("a finished job could not release its folder while another job waited on a stalled source")
	}
	if !finishes(func() { _ = app.Store.liveOutputCount() }) {
		unblock()
		<-waiting
		t.Fatal("the gate stayed locked during disk work")
	}
	unblock()
	<-waiting
}

// A source added while an output is being decided is still seen: the decision is
// made again rather than recorded on a stale list.
func TestGate_SourceAddedDuringTheDecisionIsSeen(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	existing := t.TempDir()
	app.Store.AddFolder(coll.ID, existing)
	out := t.TempDir()
	prev := openSourceRoot
	var once sync.Once
	var regErr error
	openSourceRoot = func(p string) (*os.File, error) {
		once.Do(func() {
			ok := finishes(func() { regErr = tryRegisterSource(app.Store, coll.ID, out) })
			if !ok {
				regErr = errors.New("registering a source waited on the decision")
			}
		})
		return prev(p)
	}
	t.Cleanup(func() { openSourceRoot = prev })
	r, err := app.Store.reserveOutputPath(out)
	if regErr != nil {
		if err == nil {
			r()
		}
		t.Fatal(regErr)
	}
	if err == nil {
		r()
		t.Fatal("an output was recorded over a source added during its decision")
	}
}

// sameObjectAs makes the source at alias report the identity of real, as one network
// share does under two server names.
func sameObjectAs(t *testing.T, alias, real string) {
	t.Helper()
	id, err := pathIdentity(real)
	if err != nil {
		t.Fatal(err)
	}
	prev := sourceIdentityOf
	sourceIdentityOf = func(p string) (fileIdent, error) {
		if normPath(p) == normPath(alias) {
			return id, nil
		}
		return prev(p)
	}
	t.Cleanup(func() { sourceIdentityOf = prev })
}

func TestGate_NewOutputInsideASourceByAnotherNameIsRefused(t *testing.T) {
	app := dockApp(t)
	src, share := t.TempDir(), t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	sameObjectAs(t, src, share)
	if r, err := app.Store.reserveOutputPath(filepath.Join(share, "staging", "new")); err == nil {
		r()
		t.Fatal("a new output inside a source, reached by another name, was allowed")
	}
}

func TestGate_NewOutputBesideASourceIsAllowed(t *testing.T) {
	app := dockApp(t)
	parent := t.TempDir()
	src := filepath.Join(parent, "photos")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	r, err := app.Store.reserveOutputPath(filepath.Join(parent, "new", "output"))
	if err != nil {
		t.Fatalf("an output beside a source: %v", err)
	}
	r()
}

// An output reserved before it existed is compared by identity once it does.
func TestGate_OutputMadeAfterItsReservationIsComparedByIdentity(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	out := filepath.Join(t.TempDir(), "new")
	r, err := app.Store.reserveOutputPath(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r()
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, alias, out)
	if err := tryRegisterSource(app.Store, coll.ID, t.TempDir()); err != nil {
		t.Fatalf("an unrelated source: %v", err)
	}
	app.Store.srcGate.mu.Lock()
	defer app.Store.srcGate.mu.Unlock()
	for o := range app.Store.srcGate.live {
		if len(o.where.chain) == 0 {
			t.Fatal("the output kept no identity after it was created")
		}
	}
}

func TestKeystore_InsideASourceByAnotherNameIsRefused(t *testing.T) {
	app := dockApp(t)
	src, share := t.TempDir(), t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	sameObjectAs(t, src, share)
	key := filepath.Join(share, "keys.json")
	if err := app.Store.CheckFileOutsideSources(key); err == nil {
		t.Error("the configuration check allowed a keystore inside a source by another name")
	}
	if r, err := app.Store.reserveOutputFiles([]string{key}); err == nil {
		r()
		t.Error("a keystore inside a source by another name was reserved")
	}
}

func TestKeystore_BesideASourceIsAllowed(t *testing.T) {
	app := dockApp(t)
	parent := t.TempDir()
	src := filepath.Join(parent, "photos")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	r, err := app.Store.reserveOutputFiles([]string{filepath.Join(parent, "keys.json")})
	if err != nil {
		t.Fatalf("a keystore beside a source: %v", err)
	}
	r()
}

// One package has one worker: a second build must not clear the first one's folder.
func TestBuild_SecondBuildOfOnePackageIsRefused(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "ONE-WORKER")
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	var second error
	ran := false
	afterPathReserved = func(string) {
		if !ran {
			ran = true
			second = app.BuildChunk(c.ID, noProg)
		}
	}
	t.Cleanup(func() { afterPathReserved = nil })
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if !ran || second == nil || !strings.Contains(second.Error(), "already") {
		t.Fatalf("ran %v, the second build: %v", ran, second)
	}
}

func TestBurn_SecondBurnOfOneQueueIsRefused(t *testing.T) {
	app, qid, _ := burnSetupForTest(t, "Disc")
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	var second error
	ran, nested := false, false
	prev := runBurnProgram
	runBurnProgram = func([]string) error {
		if nested {
			return errStopBurnForTest
		}
		ran, nested = true, true
		_, second = app.BurnNext(qid, func(float64, string) {})
		return errStopBurnForTest
	}
	t.Cleanup(func() { runBurnProgram = prev })
	_, _ = app.BurnNext(qid, func(float64, string) {})
	if !ran || second == nil || !strings.Contains(second.Error(), "already") {
		t.Fatalf("ran %v, the second burn: %v", ran, second)
	}
}

// The way forward names what to do, and another spelling of a recorded source is
// still that source.
func TestRegisterSource_DataFolderRefusalGivesAWayForward(t *testing.T) {
	app := dockApp(t)
	err := tryRegisterSource(app.Store, app.Store.AddCollection("Live").ID, filepath.Dir(app.DataDir))
	if err == nil || !strings.Contains(err.Error(), "add the folders inside it") {
		t.Fatalf("no way forward: %v", err)
	}
}

func TestRegisterSource_RecordedSourceBySpellingIsNotLockedOut(t *testing.T) {
	app := dockApp(t)
	coll := app.Store.AddCollection("Live")
	holder := filepath.Dir(app.DataDir)
	app.Store.addFolderRecord(coll.ID, holder)
	if err := tryRegisterSource(app.Store, coll.ID, holder+string(filepath.Separator)); err != nil {
		t.Fatalf("a recorded source with a trailing separator: %v", err)
	}
}

// ---- device identity ------------------------------------------------------------

const lsblkSpanning = `{"blockdevices":[
 {"name":"sda","kname":"sda","serial":"A","model":"M","size":100,"mountpoint":null,"type":"disk","tran":"sata","partuuid":null,
  "children":[{"name":"sda1","kname":"sda1","size":90,"mountpoint":null,"type":"part","partuuid":"0a1b2c3d-01",
   "children":[{"name":"vg-data","kname":"dm-0","size":180,"mountpoint":"/data","type":"lvm"}]}]},
 {"name":"sdb","kname":"sdb","serial":"B","model":"M","size":100,"mountpoint":null,"type":"disk","tran":"sata","partuuid":null,
  "children":[{"name":"sdb1","kname":"sdb1","size":90,"mountpoint":"/other","type":"part","partuuid":"0a1b2c3e-01",
   "children":[{"name":"vg-data","kname":"dm-0","size":180,"mountpoint":"/data","type":"lvm"}]}]}]}`

func TestParseLsblk_SpanningVolumeIsUnknown(t *testing.T) {
	for _, kname := range []string{"", "dm-0"} {
		id, err := parseLsblkDevice([]byte(lsblkSpanning), "/data/photos", kname)
		if !errors.Is(err, errVolumeSpansDisks) {
			t.Errorf("kname %q: got %+v, %v", kname, id, err)
		}
		if id.Serial != "" || id.Partition != "" {
			t.Errorf("kname %q: a disk was recorded: %+v", kname, id)
		}
	}
}

// The opened object's device decides, not the path's longest mount.
func TestParseLsblk_DeviceBeatsMountPath(t *testing.T) {
	id, err := parseLsblkDevice([]byte(lsblkSpanning), "/other/x", "sda1")
	if err != nil || id.Serial != "A" || id.Partition != "mbr:0a1b2c3d-01" {
		t.Fatalf("got %+v, %v", id, err)
	}
	if _, err := parseLsblkDevice([]byte(lsblkSpanning), "/other/x", "gone"); err == nil {
		t.Fatal("a device lsblk does not list was given an identity")
	}
}

func TestMountSource_FromMountInfo(t *testing.T) {
	// The root mount is listed last, so only the longest match can pick /home.
	const info = "22 21 0:31 /home /home rw shared:1 - btrfs /dev/sdb1 rw\n" +
		"23 21 0:32 / /run rw - tmpfs tmpfs rw\n" +
		"21 1 0:30 / / rw - btrfs /dev/sda2 rw\n"
	for abs, want := range map[string]string{"/home/me": "sdb1", "/var/x": "sda2", "/run/x": "", "/homely": "sda2"} {
		if got := mountSourceFrom(info, abs); got != want {
			t.Errorf("%s: got %q, want %q", abs, got, want)
		}
	}
}

// A failed disk lookup is not proof of two devices.
func TestKeystores_UnknownDiskIsNotProofOfSeparation(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	prevDev, prevDisks, prevNet := keystoreDeviceOf, keystoreDisks, keystoreOnNetwork
	keystoreDeviceOf = realKeystoreDevice
	keystoreOnNetwork = func(string) bool { return false }
	keystoreDisks = func(p string) ([]string, bool) {
		if strings.HasPrefix(p, a) {
			return []string{"disk:1"}, true
		}
		return nil, false
	}
	t.Cleanup(func() { keystoreDeviceOf, keystoreDisks, keystoreOnNetwork = prevDev, prevDisks, prevNet })
	err := checkKeystorePaths([]string{filepath.Join(a, "k.json"), filepath.Join(b, "k.json")})
	if diskLookupSupported && (err == nil || !strings.Contains(err.Error(), "cannot tell which device")) {
		t.Fatalf("got %v", err)
	}
	// A network share has no local disk to read, and stays allowed.
	keystoreOnNetwork = func(p string) bool { return strings.HasPrefix(p, b) }
	if err := checkKeystorePaths([]string{filepath.Join(a, "k.json"), filepath.Join(b, "k.json")}); err != nil {
		t.Fatalf("a keystore on a network share: %v", err)
	}
}
