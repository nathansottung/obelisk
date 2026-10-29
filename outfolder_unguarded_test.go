//go:build !guionly

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Where the destination guard is not available (macOS, the BSDs) an output
// folder is checked by real location and by identity and written through a
// handle. The guard is switched off here so this runs on every platform.
func TestOutFolder_WithoutGuard(t *testing.T) {
	orig := destGuardAvailable
	destGuardAvailable = false
	t.Cleanup(func() { destGuardAvailable = orig })
	app := dockApp(t)
	src, link, untouched := sourceBehindLink(t, app)

	for name, dir := range map[string]string{
		"a link to the source":       link,
		"a folder inside the source": filepath.Join(link, "out"),
		"the source's parent":        filepath.Dir(src),
	} {
		if o, err := app.openOutFolder(dir); err == nil {
			o.Close()
			t.Errorf("%s: not refused", name)
		}
	}
	untouched()

	dir := filepath.Join(t.TempDir(), "new")
	o, err := app.openOutFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	victim := plantAtStageName(t, filepath.Join(dir, "sub", "f.txt"))
	if err := o.WriteFile("sub/f.txt", []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "c.bin")
	mustWrite(t, other, "COPIED")
	if _, err := o.CopyIn(other, "c.bin", 0o644); err != nil {
		t.Fatal(err)
	}
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, victim, "SOMEONE ELSE'S")
	assertFileBytes(t, filepath.Join(dir, "sub", "f.txt"), "NEW")
	assertFileBytes(t, filepath.Join(dir, "c.bin"), "COPIED")
	if got := treeOf(t, dir); !equalStrings(got, []string{"c.bin", "sub", "sub/f.txt"}) {
		t.Errorf("staging files left behind: %v", got)
	}
}

// Without the guard, a failed publish also keeps the previous file and leaves
// no staging file (the same publish fault seam is honoured).
func TestOutFolder_WithoutGuardFailedPublishKeepsPrevious(t *testing.T) {
	orig := destGuardAvailable
	destGuardAvailable = false
	t.Cleanup(func() { destGuardAvailable = orig })
	app := dockApp(t)
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "f.txt"), "OLD")
	o, err := app.openOutFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	origHook := beforeDestPublish
	beforeDestPublish = func(*os.Root, string, string) error { return fs.ErrPermission }
	t.Cleanup(func() { beforeDestPublish = origHook })
	if err := o.WriteFile("f.txt", []byte("NEW"), 0o644); err == nil {
		t.Fatal("a failed publish was reported as written")
	}
	assertFileBytes(t, filepath.Join(dir, "f.txt"), "OLD")
	if got := treeOf(t, dir); !equalStrings(got, []string{"f.txt"}) {
		t.Errorf("staging file left behind: %v", got)
	}
}
