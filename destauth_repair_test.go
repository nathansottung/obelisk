//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// real authorization only at the filesystem root, never a root mutation.
func TestDestAuth_FilesystemRootOverlap(t *testing.T) {
	app, _, src, _ := mirrorSetup(t, "Root", map[string]string{"sentinel": "ORIGINAL"})
	id := identOf(t, filepath.Join(src, "sentinel"))
	volumeRoot := filepath.VolumeName(src) + string(filepath.Separator)
	if d, err := app.Store.AuthorizeDestination(volumeRoot); err == nil {
		d.Close()
		t.Error("authorized filesystem root containing a registered source")
	}
	whole := dockApp(t)
	c := whole.Store.AddCollection("Whole volume")
	whole.Store.AddFolder(c.ID, volumeRoot)
	absent := filepath.Join(t.TempDir(), "absent")
	for _, dest := range []string{volumeRoot, t.TempDir(), absent} {
		d, err := whole.Store.AuthorizeDestination(dest)
		if err == nil {
			d.Close()
		}
		assertSourceRefusal(t, "authorize "+dest+" beneath a whole-volume source", err)
		assertSourceRefusal(t, "check and create "+dest+" beneath a whole-volume source", refusedForTest(t, whole, dest))
	}
	if _, serr := os.Lstat(absent); !os.IsNotExist(serr) {
		t.Errorf("a folder was created inside the whole-volume source: %v", serr)
	}
	if id != identOf(t, filepath.Join(src, "sentinel")) {
		t.Fatal("source object changed")
	}
	if got := treeOf(t, src); !equalStrings(got, []string{"sentinel"}) {
		t.Fatal(got)
	}
	assertFileBytes(t, filepath.Join(src, "sentinel"), "ORIGINAL")
	// Positive sibling and shared-prefix controls.
	for _, dest := range []string{t.TempDir(), src + "-sibling"} {
		d, err := app.Store.AuthorizeDestination(createdForTest(t, app, dest))
		if err != nil {
			t.Fatalf("safe sibling: %v", err)
		}
		d.Close()
	}
}

func assertFileBytes(t *testing.T, p, want string) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil || string(b) != want {
		t.Fatalf("%s: bytes=%q err=%v, want %q", p, b, err, want)
	}
}

// names, distinct object identities, contents, catalog references and coverage.
func TestDestAuth_ExactWhitespaceNames(t *testing.T) {
	for _, mode := range []string{"mirror", "incremental"} {
		t.Run(mode, func(t *testing.T) {
			members := map[string]string{" a.txt": "LEADING SPACE", "a.txt": "PLAIN"}
			app, cid, src, vol := mirrorSetup(t, "Names", members)
			before := map[string]fileIdent{}
			for name := range members {
				before[name] = identOf(t, filepath.Join(src, name))
			}
			dest := t.TempDir()
			if mode == "mirror" {
				res, err := app.MirrorToVolume(cid, nil, dest, vol.ID, 0, "", noProgMirror)
				if err != nil || res.Mirrored != 2 || res.Failed != 0 || res.Skipped != 0 {
					t.Fatalf("result=%+v err=%v", res, err)
				}
				if res.Coverage.CoveredFiles != 2 || res.Coverage.Pct != 100 {
					t.Fatal(res.Coverage)
				}
			} else {
				res, err := app.BackupChanges(cid, nil, vol.ID, BaseVolume, ModeMirror, dest, 0, "", noProgMirror)
				if err != nil || res.Files != 2 || res.Failed != 0 {
					t.Fatalf("result=%+v err=%v", res, err)
				}
			}
			entries, err := os.ReadDir(dest)
			if err != nil {
				t.Fatal(err)
			}
			names := map[string]bool{}
			for _, e := range entries {
				if !e.IsDir() {
					names[e.Name()] = true
				}
			}
			if len(names) != 2 || !names[" a.txt"] || !names["a.txt"] {
				t.Fatalf("actual names: %v", names)
			}
			if identOf(t, filepath.Join(dest, " a.txt")) == identOf(t, filepath.Join(dest, "a.txt")) {
				t.Fatal("names still alias one object")
			}
			for name, want := range members {
				assertFileBytes(t, filepath.Join(dest, name), want)
				assertFileBytes(t, filepath.Join(src, name), want)
				if before[name] != identOf(t, filepath.Join(src, name)) {
					t.Fatal("source replaced")
				}
			}
			chunks := mirrorChunksFor(app, cid, vol.ID)
			if len(chunks) != 1 || chunks[0].FileCount != 2 || len(chunks[0].Files) != 2 {
				t.Fatalf("catalog chunks=%+v", chunks)
			}
			seen := map[string]bool{}
			for _, ref := range chunks[0].Files {
				want, ok := members[ref.RelPath]
				if !ok || seen[ref.RelPath] || ref.Hash != sha256Hex([]byte(want)) || ref.SizeBytes != int64(len(want)) {
					t.Fatalf("false catalog reference: %+v", ref)
				}
				seen[ref.RelPath] = true
			}
			cov := app.archiveCoverage([]int{cid})
			if cov.CoveredFiles != 2 || cov.TotalFiles != 2 || cov.Pct != 100 {
				t.Fatal(cov)
			}
			assertNoStaging(t, dest)
		})
	}
}

// the review's exact after-verification/before-publication interleaving.
// Windows must refuse the substitution. Unix detects this same-UID intrusion
// into its private namespace; this ordered test does not prove race isolation.
func TestDestAuth_PublicationSubstitution(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "publish", true: "failure_cleanup"}[fail], func(t *testing.T) {
			app, cid, src, vol := mirrorSetup(t, "Stage", map[string]string{"a.txt": "VERIFIED BYTES"})
			dest := t.TempDir()
			prior := filepath.Join(dest, "a.txt")
			writeTree(t, dest, map[string]string{"a.txt": "PRIOR"})
			priorID := identOf(t, prior)
			sourceID := identOf(t, filepath.Join(src, "a.txt"))
			old := beforeDestPublish
			defer func() { beforeDestPublish = old }()
			attempted, substituted := false, false
			var replacement string
			var replacementID fileIdent
			beforeDestPublish = func(r *os.Root, tmp, final string) error {
				if final != "a.txt" {
					return nil
				}
				attempted = true
				if err := r.Rename(tmp, tmp+".saved"); err != nil {
					if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) {
						t.Fatalf("substitution not exercised by the expected sharing mechanism: %v", err)
					}
					t.Logf("MECHANISM: Windows refused staging substitution while owned handle is open: %v", err)
				} else {
					substituted = true
					if err := r.WriteFile(tmp, []byte("UNVERIFIED REPLACEMENT"), 0o644); err != nil {
						t.Fatal(err)
					}
					replacement = filepath.Join(dest, filepath.FromSlash(tmp))
					replacementID = identOf(t, replacement)
					t.Log("MECHANISM: substitution exercised inside private staging; ordered violation must be refused, replacement retained")
				}
				if fail {
					return errors.New("injected failure after substitution attempt")
				}
				return nil
			}
			res, err := app.MirrorToVolume(cid, nil, dest, vol.ID, 0, "", noProgMirror)
			if err != nil || !attempted {
				t.Fatalf("barrier=%v result=%+v err=%v", attempted, res, err)
			}
			if fail || substituted {
				if res.Mirrored != 0 || res.Failed != 1 || res.Coverage.CoveredFiles != 0 {
					t.Fatal(res)
				}
				assertFileBytes(t, prior, "PRIOR")
				if identOf(t, prior) != priorID {
					t.Fatal("failed publication replaced prior object")
				}
				if len(mirrorChunksFor(app, cid, vol.ID)) != 0 {
					t.Fatal("false catalog credit")
				}
			} else {
				if res.Mirrored != 1 || res.Failed != 0 || res.Coverage.CoveredFiles != 1 {
					t.Fatal(res)
				}
				assertFileBytes(t, prior, "VERIFIED BYTES")
			}
			if substituted {
				assertFileBytes(t, replacement, "UNVERIFIED REPLACEMENT")
				if replacementID != identOf(t, replacement) {
					t.Fatal("cleanup touched unrelated substituted object")
				}
			} else {
				assertNoStaging(t, dest)
			}
			assertFileBytes(t, filepath.Join(src, "a.txt"), "VERIFIED BYTES")
			if identOf(t, filepath.Join(src, "a.txt")) != sourceID {
				t.Fatal("source replaced")
			}
		})
	}
}

func TestDestAuth_SidecarMetadataAndCleanup(t *testing.T) {
	app := dockApp(t)
	dest, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	prot := t.TempDir()
	original := filepath.Join(prot, "sentinel")
	writeTree(t, prot, map[string]string{"sentinel": "PROTECTED"})
	beforeID := identOf(t, original)
	beforeStat, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	substituted, attempted := false, false
	var tmp string
	oldMetadata, oldPublish := beforeDestMetadata, beforeDestPublish
	defer func() { beforeDestMetadata, beforeDestPublish = oldMetadata, oldPublish }()
	beforeDestMetadata = func(r *os.Root, rel string) error {
		tmp, attempted = rel, true
		if err := r.Rename(tmp, tmp+".saved"); err == nil {
			if err := os.Link(original, filepath.Join(dest.Name(), filepath.FromSlash(tmp))); err != nil {
				t.Fatalf("hard-link substitution not exercised: %v", err)
			}
			substituted = true
			t.Log("MECHANISM: hard-link substitution exercised before production sidecar metadata")
		} else if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(32)) {
			t.Logf("MECHANISM: Windows refused staging substitution: %v", err)
		} else {
			t.Fatalf("substitution not exercised: %v", err)
		}
		return nil
	}
	beforeDestPublish = func(*os.Root, string, string) error { return errors.New("injected sidecar failure after metadata") }
	if err := dest.WriteFileAtomic("sidecar", []byte("SIDECAR"), 0o400); err == nil {
		t.Fatal("expected sidecar publication failure")
	}
	if !attempted {
		t.Fatal("metadata barrier did not execute")
	}
	assertFileBytes(t, original, "PROTECTED")
	afterStat, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	if afterStat.Mode() != beforeStat.Mode() || beforeID != identOf(t, original) {
		t.Fatal("original metadata or identity changed")
	}
	if substituted {
		p := filepath.Join(dest.Name(), filepath.FromSlash(tmp))
		if identOf(t, p) != beforeID {
			t.Fatal("cleanup removed unrelated hard link")
		}
		assertFileBytes(t, p, "PROTECTED")
	}
}

func TestDestAuth_PortableDriveNames(t *testing.T) {
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, rel := range []string{"C:/x", `C:\x`, "c:relative", "Z:/x", `\\server\share\x`} {
		if _, err := destRel(rel); !errors.Is(err, errRelEscape) {
			t.Errorf("accepted %q: %v", rel, err)
		}
		if err := d.WriteFileAtomic(rel, []byte("wrong"), 0o644); !errors.Is(err, errRelEscape) {
			t.Errorf("write %q: %v", rel, err)
		}
	}
	if got := treeOf(t, d.Name()); len(got) != 0 {
		t.Fatalf("invalid names mutated destination: %v", got)
	}
	for _, rel := range []string{" a.txt", "sub/ b.txt", "plain.txt"} {
		got, err := destRel(rel)
		if err != nil || got != rel {
			t.Fatalf("name rewritten: %q -> %q %v", rel, got, err)
		}
	}
	if runtime.GOOS == "windows" {
		for _, rel := range []string{"a.txt ", "dir /a.txt", "a.txt.", "a.txt:stream"} {
			if _, err := destRel(rel); err == nil {
				t.Errorf("ambiguous Windows name accepted: %q", rel)
			}
		}
	}
}
