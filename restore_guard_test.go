//go:build !guionly

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// builtPlainPackageForTest builds and writes one plaintext package of files and
// returns its id and its folder on the medium.
func builtPlainPackageForTest(s *itServer, files map[string][]byte) (int, string) {
	s.t.Helper()
	s.setConfig(nil)
	src := s.makeSource(files)
	c := s.scanPlanBuild(src, 1, false, 10)
	pid, name := int(c["id"].(float64)), c["name"].(string)
	medium := s.t.TempDir()
	vid := s.addVolume("RESTORE-"+name, "lab")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": medium, "volume_id": vid}))
	return pid, filepath.Join(medium, name)
}

// restoreRefusedForTest asks for a restore and requires it to be refused, either
// up front or by the job, returning the reason.
func restoreRefusedForTest(s *itServer, pid int, pkgDir, out string) string {
	s.t.Helper()
	m := s.obj("POST", fmt.Sprintf("/api/chunks/%d/restore", pid), map[string]any{"source_dir": pkgDir, "output_dir": out})
	if _, ok := m["job_id"]; ok {
		return s.jobFailure(m)
	}
	if s.status(m) < 400 {
		s.t.Fatalf("restore neither started nor refused: %v", m)
	}
	reason, _ := m["error"].(string)
	return reason
}

// A restore never replaces a file: if any name it would write is already taken
// in the output folder, the whole restore is refused before anything is
// written, and the existing file is left as it was.
func TestRestore_RefusesNameCollisions(t *testing.T) {
	s := newIT(t)
	pid, pkgDir := builtPlainPackageForTest(s, map[string][]byte{
		"a.txt": []byte("from the package\n"), "sub/b.txt": []byte("also from the package\n"),
	})
	out := t.TempDir()
	writeTree(t, out, map[string]string{"a.txt": "SOMEONE ELSE'S"})

	reason := restoreRefusedForTest(s, pid, pkgDir, out)
	if !strings.Contains(reason, "a.txt") {
		t.Errorf("the refusal does not name the file already there: %q", reason)
	}
	assertFileBytes(t, filepath.Join(out, "a.txt"), "SOMEONE ELSE'S")
	if got := treeOf(t, out); !equalStrings(got, []string{"a.txt"}) {
		t.Errorf("a refused restore wrote into the output folder: %v", got)
	}
}

// The output folder is checked by where it really is: a folder link to a
// registered source is refused, and nothing is written into the source.
func TestRestore_RefusesOutputThatIsASource(t *testing.T) {
	s := newIT(t)
	pid, pkgDir := builtPlainPackageForTest(s, map[string][]byte{"a.txt": []byte("from the package\n")})
	src := t.TempDir()
	writeTree(t, src, map[string]string{"orig.txt": "ORIG"})
	s.app.Store.AddFolder(s.app.Store.AddCollection("Live").ID, src)
	link := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, src)

	reason := restoreRefusedForTest(s, pid, pkgDir, link)
	if !strings.Contains(reason, "source") {
		t.Errorf("refused for another reason: %q", reason)
	}
	if got := treeOf(t, src); !equalStrings(got, []string{"orig.txt"}) {
		t.Errorf("a restore wrote into a registered source: %v", got)
	}
}

// A clean restore leaves only the restored files: no work folder behind.
func TestRestore_LeavesOnlyTheRestoredFiles(t *testing.T) {
	s := newIT(t)
	files := map[string][]byte{"a.txt": []byte("one\n"), "sub/deep/b.txt": []byte("two\n")}
	pid, pkgDir := builtPlainPackageForTest(s, files)
	out := t.TempDir()
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/restore", pid), map[string]any{"source_dir": pkgDir, "output_dir": out}))
	if got := treeOf(t, out); !equalStrings(got, []string{"a.txt", "manifest-sha256.txt", "sub", "sub/deep", "sub/deep/b.txt"}) {
		t.Errorf("output after restore: %v", got)
	}
	for rel, b := range files {
		assertFileBytes(t, filepath.Join(out, filepath.FromSlash(rel)), string(b))
	}
}

// A damaged payload is repaired on a copy in the restore's work folder: the
// medium is only read, so its bytes (damage included) and its file list are
// exactly as they were, and the restore still comes out whole.
func TestRestore_RepairsACopyNeverTheMedium(t *testing.T) {
	s := newIT(t)
	s.setConfig(nil)
	files := map[string][]byte{
		"a.txt":     bytes.Repeat([]byte("the quick brown fox\n"), 2000),
		"sub/b.bin": bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 5000),
	}
	src := s.makeSource(files)
	c := s.scanPlanBuild(src, 1, false, 40)
	pid, name := int(c["id"].(float64)), c["name"].(string)
	medium := t.TempDir()
	vid := s.addVolume("REPAIR-1", "lab")
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/write", pid), map[string]any{"dest_dir": medium, "volume_id": vid}))
	pkgDir := filepath.Join(medium, name)
	payload := filepath.Join(pkgDir, name+".tar")
	corruptRun(t, payload, 24)
	damaged, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	before := treeOf(t, pkgDir)

	out := t.TempDir()
	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/restore", pid), map[string]any{"source_dir": pkgDir, "output_dir": out}))
	assertTreeMatches(t, src, out)

	if after, _ := os.ReadFile(payload); !bytes.Equal(after, damaged) {
		t.Error("the payload on the medium was changed: repair must work on a copy")
	}
	if after := treeOf(t, pkgDir); !equalStrings(after, before) {
		t.Errorf("files on the medium changed from %v to %v", before, after)
	}
	for _, e := range treeOf(t, out) {
		if strings.HasPrefix(e, ".obelisk-restore-") {
			t.Errorf("the restore left its work folder behind: %s", e)
		}
	}
}
