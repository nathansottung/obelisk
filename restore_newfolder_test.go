//go:build !guionly

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// withoutDestGuard runs the rest of the test as on a platform where the
// destination guard is not available (macOS, the BSDs).
func withoutDestGuard(t *testing.T) {
	t.Helper()
	prev := destGuardAvailable
	destGuardAvailable = false
	t.Cleanup(func() { destGuardAvailable = prev })
}

// Without the destination guard, a restore goes only into a new folder outside
// every source: Obelisk creates it and restores into it; an existing folder, or
// one that would be inside or hold a source, is refused and nothing is created.
func TestRestore_WithoutGuardOnlyIntoANewFolder(t *testing.T) {
	s := newIT(t)
	files := map[string][]byte{"a.txt": []byte("one\n"), "sub/b.txt": []byte("two\n")}
	pid, pkgDir := builtPlainPackageForTest(s, files)
	withoutDestGuard(t)

	// An existing folder, even an empty one, is refused.
	existing := t.TempDir()
	if reason := restoreRefusedForTest(s, pid, pkgDir, existing); !strings.Contains(reason, "new folder") {
		t.Errorf("restore into an existing folder: refused for another reason: %q", reason)
	}
	if got := treeOf(t, existing); len(got) != 0 {
		t.Errorf("a refused restore wrote into %s: %v", existing, got)
	}

	// A new folder inside a source, also reached through a folder link, is refused.
	parent := t.TempDir()
	src := filepath.Join(parent, "photos")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	s.app.Store.AddFolder(s.app.Store.AddCollection("Live").ID, src)
	link := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, src)
	for _, out := range []string{filepath.Join(src, "restored"), filepath.Join(link, "restored")} {
		if reason := restoreRefusedForTest(s, pid, pkgDir, out); !strings.Contains(reason, "source") {
			t.Errorf("restore into %s: refused for another reason: %q", out, reason)
		}
	}
	if got := treeOf(t, parent); !equalStrings(got, []string{"photos", "photos/orig.nef"}) {
		t.Errorf("a refused restore created something: %v", got)
	}

	// A new folder outside every source: created, restored into, nothing else left.
	out := filepath.Join(t.TempDir(), "restored")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/restore", pid), map[string]any{"source_dir": pkgDir, "output_dir": out}))
	if got := treeOf(t, out); !equalStrings(got, []string{"a.txt", "manifest-sha256.txt", "sub", "sub/b.txt"}) {
		t.Errorf("output after restore: %v", got)
	}
	for rel, b := range files {
		assertFileBytes(t, filepath.Join(out, filepath.FromSlash(rel)), string(b))
	}

	// The check the UI reads tells it to skip the guard's questions.
	if c := s.app.Store.CheckDestination(filepath.Join(t.TempDir(), "next")); !c.NewFolderOnly {
		t.Errorf("the check does not say only a new folder is possible: %+v", c)
	}
}

// The new restore folder is matched against the sources by identity, on its
// own: with the path check switched off, a new folder whose parent is a source
// (reached here through a folder link, as a different letter case or a
// firmlink reaches it on macOS) is still refused, and nothing is created.
func TestRestore_NewFolderMatchesSourcesByIdentity(t *testing.T) {
	s := newIT(t)
	pid, pkgDir := builtPlainPackageForTest(s, map[string][]byte{"a.txt": []byte("one\n")})
	withoutDestGuard(t)
	prev := newRestoreFolderPathCheck
	newRestoreFolderPathCheck = func(*Store, string) error { return nil }
	t.Cleanup(func() { newRestoreFolderPathCheck = prev })

	src := t.TempDir()
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	s.app.Store.AddFolder(s.app.Store.AddCollection("Live").ID, src)
	link := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, src)

	for _, out := range []string{filepath.Join(link, "restored"), filepath.Join(link, "deeper")} {
		if reason := restoreRefusedForTest(s, pid, pkgDir, out); !strings.Contains(reason, "source") {
			t.Errorf("restore into %s: refused for another reason: %q", out, reason)
		}
		// The job's own check, past the route.
		if _, err := s.app.RestoreChunk(pid, pkgDir, out, nil, func(float64, string) {}); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("restore job into %s: got %v, want a refusal naming the source", out, err)
		}
	}
	if got := treeOf(t, src); !equalStrings(got, []string{"orig.nef"}) {
		t.Errorf("a refused restore created something in the source: %v", got)
	}
}
