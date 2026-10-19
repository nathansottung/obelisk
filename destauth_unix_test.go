//go:build !windows && !guionly

package main

import (
	"os"
	"path"
	"path/filepath"
	"testing"
)

// a dangling relative symlink inside the broad ancestor must not turn
// bootstrap into creation in protected/missing. Source-derived pre-repair defect;
// this test requires actual Unix execution, not cross-compilation.
func TestDestAuth_DanglingBootstrap(t *testing.T) {
	app := dockApp(t)
	base := t.TempDir()
	prot := filepath.Join(base, "protected")
	writeTree(t, prot, map[string]string{"sentinel": "PROTECTED"})
	c := app.Store.AddCollection("Protected")
	app.Store.AddFolder(c.ID, prot)
	id := identOf(t, filepath.Join(prot, "sentinel"))
	before := treeOf(t, prot)
	if err := os.Symlink("protected/missing", filepath.Join(base, "alias")); err != nil {
		t.Fatalf("Unix symlink fixture unavailable (NOT TESTED): %v", err)
	}
	if d, err := app.Store.AuthorizeDestination(filepath.Join(base, "alias", "child")); err == nil {
		d.Close()
		t.Fatal("dangling bootstrap authorized")
	}
	if got := treeOf(t, prot); !equalStrings(before, got) {
		t.Fatalf("bootstrap mutated protected tree: %v -> %v", before, got)
	}
	assertFileBytes(t, filepath.Join(prot, "sentinel"), "PROTECTED")
	if id != identOf(t, filepath.Join(prot, "sentinel")) {
		t.Fatal("sentinel replaced")
	}
	// Ordinary missing sibling creation remains supported on Unix.
	d, err := app.Store.AuthorizeDestination(filepath.Join(base, "safe", "child"))
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

func TestDestAuth_BootstrapComponentSubstitution(t *testing.T) {
	for _, when := range []string{"before_mkdir", "after_mkdir"} {
		t.Run(when, func(t *testing.T) {
			app := dockApp(t)
			base := t.TempDir()
			prot := filepath.Join(base, "protected")
			writeTree(t, prot, map[string]string{"sentinel": "PROTECTED"})
			c := app.Store.AddCollection("Protected")
			app.Store.AddFolder(c.ID, prot)
			id := identOf(t, filepath.Join(prot, "sentinel"))
			before := treeOf(t, prot)
			old := bootstrapMkdir
			defer func() { bootstrapMkdir = old }()
			fired := false
			bootstrapMkdir = func(r *os.Root, component string) error {
				if fired {
					return old(r, component)
				}
				fired = true
				if when == "after_mkdir" {
					if err := old(r, component); err != nil {
						return err
					}
					if err := r.Rename(component, component+".saved"); err != nil {
						t.Fatal(err)
					}
				}
				if err := r.Symlink("protected", component); err != nil {
					t.Fatal(err)
				}
				if when == "after_mkdir" {
					return nil
				}
				return old(r, component)
			}
			if d, err := app.Store.AuthorizeDestination(filepath.Join(base, "absent", "child")); err == nil {
				d.Close()
				t.Fatal("substituted component authorized")
			}
			if !fired {
				t.Fatal("barrier never fired")
			}
			if got := treeOf(t, prot); !equalStrings(before, got) {
				t.Fatalf("protected mutation: %v -> %v", before, got)
			}
			assertFileBytes(t, filepath.Join(prot, "sentinel"), "PROTECTED")
			if id != identOf(t, filepath.Join(prot, "sentinel")) {
				t.Fatal("sentinel replaced")
			}
		})
	}
}

func TestDestAuth_MissingSourceUnderSymlink(t *testing.T) {
	app := dockApp(t)
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	writeTree(t, realParent, map[string]string{"sentinel": "KEEP"})
	if err := os.Symlink("real", filepath.Join(base, "alias")); err != nil {
		t.Fatalf("Unix symlink fixture unavailable (NOT TESTED): %v", err)
	}
	c := app.Store.AddCollection("Absent")
	app.Store.AddFolder(c.ID, filepath.Join(base, "alias", "missing", "descendant"))
	id := identOf(t, filepath.Join(realParent, "sentinel"))
	before := treeOf(t, realParent)
	for _, dest := range []string{filepath.Join(realParent, "missing"), filepath.Join(realParent, "missing", "descendant"), filepath.Join(realParent, "missing", "descendant", "inside")} {
		if d, err := app.Store.AuthorizeDestination(dest); err == nil {
			d.Close()
			t.Errorf("authorized %s", dest)
		}
	}
	if got := treeOf(t, realParent); !equalStrings(before, got) {
		t.Fatal(got)
	}
	if id != identOf(t, filepath.Join(realParent, "sentinel")) {
		t.Fatal("sentinel replaced")
	}
	assertFileBytes(t, filepath.Join(realParent, "sentinel"), "KEEP")
	d, err := app.Store.AuthorizeDestination(filepath.Join(realParent, "safe"))
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
}

// replace the public staging-container name while leaving its inaccessible
// interior alone. Descriptor-relative publication must follow the original
// directory. This is distinct from the same-UID private-entry intrusion test.
func TestDestAuth_PrivateContainerSubstitution(t *testing.T) {
	app, cid, _, vol := mirrorSetup(t, "Private", map[string]string{"a.txt": "VERIFIED"})
	dest := t.TempDir()
	old := beforeDestPublish
	defer func() { beforeDestPublish = old }()
	fired := false
	var replacement string
	var replacementID fileIdent
	beforeDestPublish = func(r *os.Root, tmp, final string) error {
		if final != "a.txt" {
			return nil
		}
		fired = true
		container := path.Dir(tmp)
		if err := r.Rename(container, container+".saved"); err != nil {
			t.Fatal(err)
		}
		if err := r.WriteFile(container, []byte("UNRELATED"), 0o644); err != nil {
			t.Fatal(err)
		}
		replacement = filepath.Join(dest, filepath.FromSlash(container))
		replacementID = identOf(t, replacement)
		return nil
	}
	res, err := app.MirrorToVolume(cid, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil || !fired || res.Mirrored != 1 || res.Failed != 0 {
		t.Fatalf("barrier=%v result=%+v err=%v", fired, res, err)
	}
	assertFileBytes(t, filepath.Join(dest, "a.txt"), "VERIFIED")
	assertFileBytes(t, replacement, "UNRELATED")
	if replacementID != identOf(t, replacement) {
		t.Fatal("unrelated container-name replacement removed")
	}
}

func TestDestAuth_UnixSpaceEndingDirectory(t *testing.T) {
	app, cid, _, vol := mirrorSetup(t, "Spaces", map[string]string{"dir / a.txt": "EXACT", "dir/a.txt": "OTHER"})
	dest := t.TempDir()
	res, err := app.MirrorToVolume(cid, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil || res.Mirrored != 2 || res.Failed != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	assertFileBytes(t, filepath.Join(dest, "dir ", " a.txt"), "EXACT")
	assertFileBytes(t, filepath.Join(dest, "dir", "a.txt"), "OTHER")
}
