//go:build !guionly

package main

// writecheck_test.go: before Obelisk puts anything somewhere, it checks the account
// can really create, write, rename and delete there.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// blockWrites makes the access check report facts for anything at or under dir.
func blockWrites(t *testing.T, dir string, facts writeFacts) {
	t.Helper()
	// The checks see paths in full. A temp folder can be spelled with a short name.
	full, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	prev := writeAccessFacts
	writeAccessFacts = func(p string, newFolder bool) (writeFacts, error) {
		for _, d := range []string{dir, full} {
			if normPath(p) == normPath(d) || canonicalWithin(normPath(d), normPath(p)) {
				return facts, nil
			}
		}
		return prev(p, newFolder)
	}
	t.Cleanup(func() { writeAccessFacts = prev })
}

var writeBlocks = map[string]struct {
	facts writeFacts
	says  string
}{
	"a read-only share":      {writeFacts{ReadOnlyShare: true}, "network share is read-only"},
	"read-only media":        {writeFacts{ReadOnlyVolume: true}, "drive is read-only"},
	"a write-protected card": {writeFacts{WriteProtected: true}, "write-protected"},
	"a deny entry":           {writeFacts{DeniedBy: []string{"Example Group"}}, "deny entry for Example Group"},
	"read but not change":    {writeFacts{CannotChange: true}, "can read this folder but cannot change it"},
}

func assertWriteRefusal(t *testing.T, err error, says string) {
	t.Helper()
	var wa *WriteAccessError
	if !errors.As(err, &wa) {
		t.Fatalf("got %v, want a write-access refusal", err)
	}
	for _, want := range []string{says, "Nothing was written", "did not switch to another location"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func assertEmptyFolder(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%s holds %d entries after a refusal, first %q", dir, len(entries), entries[0].Name())
	}
}

func TestWriteAccess_NewCatalogRefusedAndNothingLeft(t *testing.T) {
	for name, b := range writeBlocks {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			blockWrites(t, parent, b.facts)
			s, err := OpenStore(filepath.Join(parent, "data"))
			if err == nil {
				s.Close()
				t.Fatal("a catalog was opened there")
			}
			assertWriteRefusal(t, err, b.says)
			assertEmptyFolder(t, parent)
		})
	}
}

func TestWriteAccess_ExistingDataFolderRefused(t *testing.T) {
	for name, b := range writeBlocks {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			blockWrites(t, dir, b.facts)
			if s, err := OpenStore(dir); err == nil {
				s.Close()
				t.Fatal("a catalog was opened there")
			} else {
				assertWriteRefusal(t, err, b.says)
			}
			assertEmptyFolder(t, dir)
		})
	}
}

func TestWriteAccess_BackupOutputRefused(t *testing.T) {
	for name, b := range writeBlocks {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			dir := t.TempDir()
			blockWrites(t, dir, b.facts)
			if destGuardAvailable {
				if c := app.Store.CheckDestination(dir); c.Status != destStatusRefuse || !strings.Contains(c.Reason, b.says) {
					t.Errorf("the check: status %q, reason %q", c.Status, c.Reason)
				}
			}
			o, err := app.openOutFolder(dir)
			if err == nil {
				o.Close()
				t.Fatal("the output folder was opened")
			}
			assertWriteRefusal(t, err, b.says)
			assertEmptyFolder(t, dir)
			if n := app.Store.liveOutputCount(); n != 0 {
				t.Fatalf("%d outputs still held after the refusal", n)
			}
		})
	}
}

func TestWriteAccess_KeystoreRefusedBeforeAnyIsWritten(t *testing.T) {
	for name, b := range writeBlocks {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			first, second := t.TempDir(), t.TempDir()
			blockWrites(t, second, b.facts)
			r, err := app.Store.reserveOutputFiles([]string{filepath.Join(first, "k.json"), filepath.Join(second, "k.json")})
			if err == nil {
				r()
				t.Fatal("the keystores were reserved")
			}
			assertWriteRefusal(t, err, b.says)
			assertEmptyFolder(t, first)
			assertEmptyFolder(t, second)
			if n := app.Store.liveOutputCount(); n != 0 {
				t.Fatalf("%d outputs still held after the refusal", n)
			}
		})
	}
}

func TestWriteAccess_StagingRefusedBeforeItIsCleared(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "NO-WRITE")
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	blockWrites(t, staging, writeFacts{CannotChange: true})
	before, err := os.ReadDir(filepath.Join(staging, c.Name))
	if err != nil {
		t.Fatal(err)
	}
	err = app.BuildChunk(c.ID, noProg)
	assertWriteRefusal(t, err, "cannot change")
	after, err := os.ReadDir(filepath.Join(staging, c.Name))
	if err != nil || len(after) != len(before) {
		t.Fatalf("the earlier build was cleared: %d entries, was %d (%v)", len(after), len(before), err)
	}
}

// What the static look cannot see (a share's own permissions) is found by really
// doing it, and nothing of the attempt stays.
func TestWriteAccess_RealAttemptFailureRefusesAndLeavesNothing(t *testing.T) {
	for step, want := range map[string]string{"create": "cannot change", "rename": "cannot change", "delete": "cannot change"} {
		t.Run(step, func(t *testing.T) {
			dir := t.TempDir()
			prev := writeProbeStep
			writeProbeStep = func(s string) error {
				if s == step {
					return os.ErrPermission
				}
				return nil
			}
			t.Cleanup(func() { writeProbeStep = prev })
			err := proveWriteAccess(dir, "backup output")
			assertWriteRefusal(t, err, want)
			writeProbeStep = prev
			assertEmptyFolder(t, dir)
		})
	}
}

func TestWriteAccess_NormalFolderPassesAndKeepsNothing(t *testing.T) {
	dir := t.TempDir()
	if err := proveWriteAccess(dir, "backup output"); err != nil {
		t.Fatal(err)
	}
	assertEmptyFolder(t, dir)
	missing := filepath.Join(dir, "not", "there")
	if err := proveWriteAccess(missing, "backup output"); err != nil {
		t.Fatalf("a folder that can be made: %v", err)
	}
	assertEmptyFolder(t, dir)
}
