//go:build windows && !guionly

package main

// destauth_windows_test.go — the destination-authority cases that are Windows link and naming
// mechanisms. These are NOT generic "symlink" tests: a directory junction is a
// reparse point with its own resolution rules, an 8.3 short name is a second real
// name for the same object, and the extended-length `\\?\` prefix changes how the
// name is parsed. Each is named for what it actually exercises. The symlink
// equivalents live in destauth_test.go and skip loudly where symlinks cannot be
// created without elevation.
//
// Every fixture is synthetic and disposable, under t.TempDir(). No elevation, no
// mounts, no real media.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

func TestDestAuth_MissingSourceUnderJunction(t *testing.T) {
	app := dockApp(t)
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	writeTree(t, realParent, map[string]string{"sentinel": "KEEP"})
	alias := filepath.Join(base, "alias")
	mustJunction(t, alias, realParent)
	c := app.Store.AddCollection("Absent")
	app.Store.AddFolder(c.ID, filepath.Join(alias, "missing-source", "descendant"))
	id := identOf(t, filepath.Join(realParent, "sentinel"))
	before := treeOf(t, realParent)
	for _, dest := range []string{filepath.Join(realParent, "missing-source", "descendant"), filepath.Join(realParent, "missing-source"), filepath.Join(realParent, "missing-source", "descendant", "inside")} {
		if err := refusedForTest(t, app, dest); err == nil {
			t.Errorf("authorized absent registered source relationship: %s", dest)
		}
	}
	if got := treeOf(t, realParent); !equalStrings(before, got) {
		t.Fatalf("absent source or descendant created: %v -> %v", before, got)
	}
	if id != identOf(t, filepath.Join(realParent, "sentinel")) {
		t.Fatal("ancestor sentinel replaced")
	}
	assertFileBytes(t, filepath.Join(realParent, "sentinel"), "KEEP")
	d, err := app.Store.AuthorizeDestination(createdForTest(t, app, filepath.Join(realParent, "safe-sibling")))
	if err != nil {
		t.Fatalf("safe sibling of absent source: %v", err)
	}
	d.Close()
	if _, err := os.Stat(filepath.Join(realParent, "missing-source")); !os.IsNotExist(err) {
		t.Fatalf("registered missing source changed: %v", err)
	}
}

// mustJunction creates a directory junction, or SKIPS with the reason. A host that
// cannot create junctions must not report these tests as passing.
func mustJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("SKIP (NOT TESTED): directory junctions unavailable on this host (mklink /J): %v — %s", err, strings.TrimSpace(string(out)))
	}
}

// shortPathName returns the 8.3 spelling of p, or "" when short-name generation is
// off for the volume.
func shortPathName(t *testing.T, p string) string {
	t.Helper()
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetShortPathNameW")
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return ""
	}
	buf := make([]uint16, 1024)
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(in)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 || int(r) >= len(buf) {
		return ""
	}
	return syscall.UTF16ToString(buf[:r])
}

// ---- junction alias of a registered source ----------------------------------

// A directory junction pointing AT a registered source root is a second name for
// the protected folder. The old pathname check refused the real path and permitted
// the junction in the same run. Deciding on the identity of the opened object
// gives both spellings the same verdict.
func TestDestAuth_JunctionAliasOfSourceRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "JAlias", map[string]string{"a.txt": "alpha\n", "sub/deep.txt": "deep\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	alias := filepath.Join(t.TempDir(), "dest-alias")
	mustJunction(t, alias, prot)

	// Control: the real path is refused by the preflight check...
	if err := app.Store.AssertOutsideSources(prot); err == nil {
		t.Fatal("precondition: the preflight check should refuse the real source path")
	}
	// ...and the alias is not, which is exactly the recorded bypass.
	if err := app.Store.AssertOutsideSources(alias); err != nil {
		t.Fatalf("precondition: the preflight check is expected to allow the alias, got %v", err)
	}

	protTree := treeOf(t, prot)
	protIdent := identOf(t, filepath.Join(prot, "keepme.txt"))

	_, err := app.MirrorToVolume(collID, nil, alias, vol.ID, 0, "", noProgMirror)
	if err == nil {
		t.Fatal("a junction alias of a registered source must be refused as a destination")
	}
	if !strings.Contains(err.Error(), prot) {
		t.Errorf("the refusal should name the source it resolved to, got: %v", err)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("files were written into the protected source through an alias: %v -> %v", protTree, got)
	}
	if got := identOf(t, filepath.Join(prot, "keepme.txt")); got != protIdent {
		t.Errorf("protected original replaced: %+v -> %+v", protIdent, got)
	}
}

// ---- nested junction below a genuinely safe root -----------------------------

// The destination root is correct and stays correct; the escape is one level
// below it, through a junction the catalog's relative path walks into. No
// entry-point check can answer this, however well it resolves the destination
// string. Root-relative operations refuse it.
func TestDestAuth_NestedJunctionLeafRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "JNest", map[string]string{"sub/inner.txt": "INNER\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	dest := t.TempDir()
	mustJunction(t, filepath.Join(dest, "sub"), prot)

	protTree := treeOf(t, prot)
	protIdent := identOf(t, filepath.Join(prot, "keepme.txt"))

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("the destination root is genuinely safe, so the job should run and fail the leaf: %v", err)
	}
	if res.Mirrored != 0 || res.Failed != 1 {
		t.Errorf("the leaf write through a nested junction must fail: mirrored=%d failed=%d skipped=%d", res.Mirrored, res.Failed, res.Skipped)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("a leaf write escaped the authorized root into the protected source: %v -> %v", protTree, got)
	}
	if got := identOf(t, filepath.Join(prot, "keepme.txt")); got != protIdent {
		t.Errorf("protected original replaced: %+v -> %+v", protIdent, got)
	}
}

// The SIDECAR obeys the same authority as payload writes: a junction planted at
// the sidecar's own folder name cannot carry the inventory into a source.
func TestDestAuth_SidecarThroughNestedJunctionRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "JSide", map[string]string{"a.txt": "alpha\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	dest := t.TempDir()
	mustJunction(t, filepath.Join(dest, dockSidecarDir), prot)

	protTree := treeOf(t, prot)

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	// The payload still lands — the destination root is safe.
	if res.Mirrored != 1 {
		t.Errorf("payload should still land: mirrored=%d failed=%d", res.Mirrored, res.Failed)
	}
	// The sidecar must NOT have been written through the junction.
	if res.Sidecar != "" {
		if _, serr := treeContains(t, prot, "INVENTORY.md"); serr {
			t.Error("the inventory sidecar was written into the protected source through a junction")
		}
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("the sidecar escaped into the protected source: %v -> %v", protTree, got)
	}
}

func treeContains(t *testing.T, root, name string) (string, bool) {
	t.Helper()
	for _, rel := range treeOf(t, root) {
		if filepath.Base(rel) == name {
			return rel, true
		}
	}
	return "", false
}

// ---- root substituted after authorization ------------------------------------

// The check was correct when it ran; the destination is swapped underneath it
// mid-job. The barrier is the progress callback — the swap happens after a known
// file has published, so there is no sleep and no timing assumption.
//
// TWO OUTCOMES ARE ACCEPTABLE, and the test records which one occurred:
//
//   - the OS refuses the substitution outright, because this process holds the
//     directory open (a Windows sharing-mode effect, NOT a portable guarantee and
//     NOT something the design relies on); or
//   - the substitution succeeds and the retained handle keeps writing to the
//     ORIGINAL object, so the replacement pathname is never followed.
//
// Either way the asserted security property is the same: nothing reaches the
// protected source, and the protected original keeps its identity.
func TestDestAuth_RootSubstitutedAfterAuthorization(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "Swap", map[string]string{
		"01.txt": "one\n", "02.txt": "two\n", "03.txt": "three\n", "04.txt": "four\n",
	})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	base := t.TempDir()
	dest := filepath.Join(base, "dest")
	writeTree(t, dest, map[string]string{"placeholder.txt": "x\n"})

	protTree := treeOf(t, prot)
	protIdent := identOf(t, filepath.Join(prot, "keepme.txt"))

	swapped := false
	var swapErr error
	barrier := func(_ float64, msg string) {
		if swapped || !strings.Contains(msg, "mirroring 2/4") {
			return
		}
		swapped = true
		// Deterministic: 01.txt has published by the time this message appears.
		if err := renameFile(dest, filepath.Join(base, "moved-away")); err != nil {
			swapErr = err
			return
		}
		out, err := exec.Command("cmd", "/c", "mklink", "/J", dest, prot).CombinedOutput()
		if err != nil {
			swapErr = err
			t.Logf("substitution: rename succeeded but junction creation failed: %v — %s", err, strings.TrimSpace(string(out)))
		}
	}

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", barrier)
	if err != nil {
		t.Logf("job returned: %v", err)
	}
	if !swapped {
		t.Fatal("the barrier never fired — the substitution was not attempted")
	}
	if swapErr != nil {
		t.Logf("MECHANISM: the OS refused the substitution while the authorized handle was open: %v", swapErr)
		t.Log("this is a Windows sharing-mode effect, not a portable guarantee; the assertion below is the property that matters")
	} else {
		t.Log("MECHANISM: the substitution succeeded; the retained handle must not have followed the replacement pathname")
	}
	if res != nil {
		t.Logf("result: mirrored=%d failed=%d skipped=%d", res.Mirrored, res.Failed, res.Skipped)
	}

	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("writes reached the protected source after a post-authorization substitution: %v -> %v", protTree, got)
	}
	if got := identOf(t, filepath.Join(prot, "keepme.txt")); got != protIdent {
		t.Errorf("protected original replaced: %+v -> %+v", protIdent, got)
	}
}

// ---- alternate Windows spellings ---------------------------------------------

// The extended-length prefix was a confirmed bypass: filepath.Abs leaves `\\?\` in
// place, so the normalised string matched no registered source while the OS
// resolved it to the protected folder. The canonical name now comes from the
// handle, with the prefix stripped, so both spellings reduce to one answer.
func TestDestAuth_ExtendedLengthPrefixRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "ExtLen", map[string]string{"a.txt": "alpha\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	absProt, err := filepath.Abs(prot)
	if err != nil {
		t.Fatal(err)
	}
	ext := `\\?\` + absProt

	// Control: the preflight check lets this spelling through.
	if perr := app.Store.AssertOutsideSources(ext); perr != nil {
		t.Fatalf("precondition: the preflight check is expected to allow the extended-length spelling, got %v", perr)
	}

	protTree := treeOf(t, prot)
	protIdent := identOf(t, filepath.Join(prot, "keepme.txt"))

	if _, merr := app.MirrorToVolume(collID, nil, ext, vol.ID, 0, "", noProgMirror); merr == nil {
		t.Fatal("an extended-length spelling of a registered source must be refused")
	} else {
		t.Logf("refused: %v", merr)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("files were written into the protected source through the extended-length spelling: %v -> %v", protTree, got)
	}
	if got := identOf(t, filepath.Join(prot, "keepme.txt")); got != protIdent {
		t.Errorf("protected original replaced: %+v -> %+v", protIdent, got)
	}
}

// An 8.3 short name is a second real name for the same folder. Short-name
// generation can be disabled per volume, so this SKIPS with a reason rather than
// passing when there is nothing to test.
func TestDestAuth_ShortNameSpellingRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "ShortName", map[string]string{"a.txt": "alpha\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	// A long, non-8.3 component is needed for a distinct short name to exist.
	longChild := filepath.Join(prot, "a-very-long-folder-name-for-8dot3")
	writeTree(t, longChild, map[string]string{"inner.txt": "INNER\n"})

	short := shortPathName(t, longChild)
	if short == "" || strings.EqualFold(short, longChild) {
		t.Skipf("SKIP (NOT TESTED): 8.3 short-name generation is not active for this volume (%s)", longChild)
	}
	t.Logf("short spelling: %s", short)

	protTree := treeOf(t, prot)
	if _, err := app.MirrorToVolume(collID, nil, short, vol.ID, 0, "", noProgMirror); err == nil {
		t.Fatal("an 8.3 spelling of a folder inside a registered source must be refused")
	} else {
		t.Logf("refused: %v", err)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("files were written into the protected source through the 8.3 spelling: %v -> %v", protTree, got)
	}
}
