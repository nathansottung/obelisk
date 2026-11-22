//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failProbe makes the real attempt fail, as a share's own permissions would.
func failProbe(t *testing.T) {
	t.Helper()
	prev := writeProbeStep
	writeProbeStep = func(s string) error {
		if s == "create" {
			return os.ErrPermission
		}
		return nil
	}
	t.Cleanup(func() { writeProbeStep = prev })
}

// A new data folder that fails the real attempt is removed again.
func TestWriteAccess_NewDataFolderFailingTheAttemptIsRemoved(t *testing.T) {
	parent := t.TempDir()
	failProbe(t)
	s, err := OpenStore(filepath.Join(parent, "a", "data"))
	if err == nil {
		s.Close()
		t.Fatal("a catalog was opened there")
	}
	assertWriteRefusal(t, err, "cannot change")
	assertEmptyFolder(t, parent)
}

// What only the real attempt can find still stops an output, and frees its hold.
func TestWriteAccess_OutputFailingTheAttemptIsRefused(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	failProbe(t)
	o, err := app.openOutFolder(dir)
	if err == nil {
		o.Close()
		t.Fatal("the output folder was opened")
	}
	assertWriteRefusal(t, err, "cannot change")
	assertEmptyFolder(t, dir)
	if n := app.Store.liveOutputCount(); n != 0 {
		t.Fatalf("%d outputs still held", n)
	}
}

// Where the guard is missing, a folder that cannot be written is not created first.
func TestWriteAccess_UnguardedOutputCreatesNothing(t *testing.T) {
	withoutDestGuard(t)
	app := dockApp(t)
	parent := t.TempDir()
	blockWrites(t, parent, writeFacts{CannotChange: true})
	o, err := app.openOutFolder(filepath.Join(parent, "new"))
	if err == nil {
		o.Close()
		t.Fatal("the output folder was opened")
	}
	assertWriteRefusal(t, err, "cannot change")
	assertEmptyFolder(t, parent)
}

func TestWriteAccess_UnguardedOutputFailingTheAttemptIsRefused(t *testing.T) {
	withoutDestGuard(t)
	app := dockApp(t)
	dir := t.TempDir()
	failProbe(t)
	o, err := app.openOutFolder(dir)
	if err == nil {
		o.Close()
		t.Fatal("the output folder was opened")
	}
	assertWriteRefusal(t, err, "cannot change")
}

// A burn is refused, and its disc left as it was, while its queue or package is busy.
func TestBurn_RefusedWhileItsQueueOrPackageIsBusy(t *testing.T) {
	for _, which := range []string{"queue", "package"} {
		t.Run(which, func(t *testing.T) {
			app, qid, _ := burnSetupForTest(t, "Disc")
			cfg := mustConfig(t, app)
			cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
			if err := writeConfigForTest(app, cfg); err != nil {
				t.Fatal(err)
			}
			prev := runBurnProgram
			runBurnProgram = func([]string) error {
				t.Error("the burn ran")
				return errStopBurnForTest
			}
			t.Cleanup(func() { runBurnProgram = prev })
			q := app.Store.BurnQueue(qid)
			key := "burn-queue:" + itoaForTest(qid)
			if which == "package" {
				key = packageWorkKey(q.Discs[0].ChunkID)
			}
			release, err := app.Store.claimWork(key, "busy")
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := app.BurnNext(qid, func(float64, string) {}); err == nil || !strings.Contains(err.Error(), "already") {
				t.Fatalf("got %v", err)
			}
			if st := app.Store.BurnQueue(qid).Discs[0].Status; st != "PENDING" {
				t.Fatalf("the disc became %s", st)
			}
		})
	}
}

func itoaForTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
