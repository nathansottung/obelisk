//go:build !guionly

package main

// destauth_test.go — the portable half of the destination-authority
// regressions. The Windows half (junction alias, nested junction, root
// substitution, extended-length and 8.3 spellings) is in destauth_windows_test.go,
// because those are Windows link and naming mechanisms, not a general "links" case.
//
// Each test drives a REAL production workflow — MirrorToVolume or BackupChanges —
// rather than asserting that a predicate returns false. A boundary that only holds
// when called directly is not a boundary.
//
// IDENTITY IS CAPTURED BEFORE THE MUTATION, from an open handle. On Windows the
// identity fields of an os.FileInfo are filled in LAZILY by reopening the name, so
// a FileInfo taken before a mutation and compared with os.SameFile afterwards can
// report "same file" about a DIFFERENT object. handleIdentity forces the answer at
// the moment of the call. Identical bytes are not proof an original survived — a
// replaced file can hold identical bytes — so every "the source is untouched"
// claim here checks object identity as well as contents.
//
// All fixtures are synthetic files under t.TempDir(). Nothing is elevated, nothing
// is mounted, and no real catalog, keystore, NAS original, tape or optical medium
// is involved.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// treeOf lists a directory's contents as slash-separated relative paths, so a test
// can assert DIRECTORY MEMBERSHIP and not merely that a named file still reads the
// same. A leaked sidecar or staging file shows up here.
func treeOf(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(out)
	return out
}

// assertNoStaging fails if any staging file survives anywhere under root.
func assertNoStaging(t *testing.T, root string) {
	t.Helper()
	for _, rel := range treeOf(t, root) {
		if strings.HasSuffix(rel, mirrorTmpSuffix) {
			t.Errorf("staging file left behind under %s: %s", root, rel)
		}
	}
}

// mirrorSetup registers one source folder in its own archive and returns the app,
// the archive id, the source root and a volume to mirror onto.
func mirrorSetup(t *testing.T, name string, files map[string]string) (*App, int, string, *Volume) {
	t.Helper()
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, files)
	coll := app.Store.AddCollection(name)
	if _, _, err := app.ScanFolder(coll.ID, src, noProgMirror); err != nil {
		t.Fatalf("scan %s: %v", src, err)
	}
	vol := app.Store.AddVolume(Volume{Label: name + "-VOL", Kind: "HDD"})
	return app, coll.ID, src, vol
}

// registerProtected scans a second, separate folder into its own archive so it is a
// REGISTERED SOURCE ROOT without being part of the archive under test.
func registerProtected(t *testing.T, app *App, files map[string]string) string {
	t.Helper()
	prot := t.TempDir()
	writeTree(t, prot, files)
	coll := app.Store.AddCollection("Protected")
	if _, _, err := app.ScanFolder(coll.ID, prot, noProgMirror); err != nil {
		t.Fatalf("scan protected %s: %v", prot, err)
	}
	return prot
}

// ---- 1. positive control -----------------------------------------------------

// An ordinary, unrelated destination still works: files land, the sidecar is
// written, the source keeps its object identity and its exact directory contents,
// and no staging survives. Any tightening of the boundary must keep this green.
func TestDestAuth_OrdinarySafeDestination(t *testing.T) {
	app, collID, src, vol := mirrorSetup(t, "Photos", map[string]string{
		"a.txt":          "alpha\n",
		"sub/b.txt":      "bravo in a subfolder\n",
		"sub/deep/c.bin": "charlie deep binary payload\n",
	})
	dest := t.TempDir()

	beforeIdent := identOf(t, filepath.Join(src, "a.txt"))
	beforeTree := treeOf(t, src)

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("mirror to an ordinary destination must succeed: %v", err)
	}
	if res.Mirrored != 3 || res.Failed != 0 || res.Skipped != 0 {
		t.Fatalf("result: mirrored=%d failed=%d skipped=%d, want 3/0/0", res.Mirrored, res.Failed, res.Skipped)
	}
	for _, rel := range []string{"a.txt", "sub/b.txt", "sub/deep/c.bin"} {
		want, _ := hashFileHex(filepath.Join(src, filepath.FromSlash(rel)))
		got, gerr := hashFileHex(filepath.Join(dest, filepath.FromSlash(rel)))
		if gerr != nil || want != got {
			t.Errorf("mirrored %s: hash %q vs %q err=%v", rel, want, got, gerr)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, dockSidecarDir, "INVENTORY.md")); err != nil {
		t.Errorf("inventory sidecar missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, dockSidecarDir, "catalog_snapshot.json")); err != nil {
		t.Errorf("catalog snapshot missing: %v", err)
	}
	assertNoStaging(t, dest)

	if got := identOf(t, filepath.Join(src, "a.txt")); got != beforeIdent {
		t.Errorf("source leaf is a different object after a safe mirror: %+v -> %+v", beforeIdent, got)
	}
	if got := treeOf(t, src); !equalStrings(beforeTree, got) {
		t.Errorf("source tree changed: %v -> %v", beforeTree, got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---- 2. the ancestor direction ----------------------------------------------

// A destination that CONTAINS registered sources is refused BEFORE any protected
// data is republished. No links, no aliases, no race: two registered folders under
// a common parent, mirrored to that parent, reconstruct their own source paths
// exactly, because a multi-folder mirror labels each subtree with the source
// folder's base name.
//
// The old pathname guard permits this — that is asserted here too, so the
// difference between the preflight check and the boundary stays visible.
func TestDestAuth_AncestorDestinationRefused(t *testing.T) {
	app := dockApp(t)
	parent := t.TempDir()
	alpha := filepath.Join(parent, "Alpha")
	bravo := filepath.Join(parent, "Bravo")
	writeTree(t, alpha, map[string]string{"a.txt": "ALPHA ORIGINAL\n"})
	writeTree(t, bravo, map[string]string{"b.txt": "BRAVO ORIGINAL\n"})

	coll := app.Store.AddCollection("Two")
	for _, root := range []string{alpha, bravo} {
		if _, _, err := app.ScanFolder(coll.ID, root, noProgMirror); err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	vol := app.Store.AddVolume(Volume{Label: "ANC", Kind: "HDD"})

	// The preflight pathname check ALLOWS the ancestor — one direction only.
	if err := app.Store.AssertOutsideSources(parent); err != nil {
		t.Fatalf("precondition: the preflight check is expected to allow the ancestor here, got %v", err)
	}

	aIdent := identOf(t, filepath.Join(alpha, "a.txt"))
	bIdent := identOf(t, filepath.Join(bravo, "b.txt"))
	aTree, bTree := treeOf(t, alpha), treeOf(t, bravo)

	_, err := app.MirrorToVolume(coll.ID, nil, parent, vol.ID, 0, "", noProgMirror)
	if err == nil {
		t.Fatal("mirroring to a folder that CONTAINS registered sources must be refused")
	}
	if !strings.Contains(err.Error(), "CONTAINS") {
		t.Errorf("the refusal must say the destination contains a source, got: %v", err)
	}
	if !strings.Contains(err.Error(), alpha) && !strings.Contains(err.Error(), bravo) {
		t.Errorf("the refusal must name the source that caused it, got: %v", err)
	}

	if got := identOf(t, filepath.Join(alpha, "a.txt")); got != aIdent {
		t.Errorf("Alpha original is a different object after a refused run: %+v -> %+v", aIdent, got)
	}
	if got := identOf(t, filepath.Join(bravo, "b.txt")); got != bIdent {
		t.Errorf("Bravo original is a different object after a refused run: %+v -> %+v", bIdent, got)
	}
	if b, _ := os.ReadFile(filepath.Join(alpha, "a.txt")); string(b) != "ALPHA ORIGINAL\n" {
		t.Errorf("Alpha original content changed: %q", b)
	}
	if got := treeOf(t, alpha); !equalStrings(aTree, got) {
		t.Errorf("Alpha membership changed: %v -> %v", aTree, got)
	}
	if got := treeOf(t, bravo); !equalStrings(bTree, got) {
		t.Errorf("Bravo membership changed: %v -> %v", bTree, got)
	}
	assertNoStaging(t, parent)
}

// The same refusal must hold for the incremental caller, which has its own
// destination effects (payload, sidecar, session record) and not merely a shared
// helper.
func TestDestAuth_AncestorDestinationRefusedForBackupChanges(t *testing.T) {
	app := dockApp(t)
	parent := t.TempDir()
	alpha := filepath.Join(parent, "Alpha")
	bravo := filepath.Join(parent, "Bravo")
	writeTree(t, alpha, map[string]string{"a.txt": "ALPHA ORIGINAL\n"})
	writeTree(t, bravo, map[string]string{"b.txt": "BRAVO ORIGINAL\n"})
	coll := app.Store.AddCollection("TwoInc")
	for _, root := range []string{alpha, bravo} {
		if _, _, err := app.ScanFolder(coll.ID, root, noProgMirror); err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	vol := app.Store.AddVolume(Volume{Label: "ANC-INC", Kind: "HDD"})
	aIdent := identOf(t, filepath.Join(alpha, "a.txt"))

	_, err := app.BackupChanges(coll.ID, nil, vol.ID, BaseVolume, ModeMirror, parent, 0, "", noProgMirror)
	if err == nil {
		t.Fatal("BackupChanges to a folder that CONTAINS registered sources must be refused")
	}
	if got := identOf(t, filepath.Join(alpha, "a.txt")); got != aIdent {
		t.Errorf("Alpha original is a different object after a refused run: %+v -> %+v", aIdent, got)
	}
	if _, serr := os.Stat(filepath.Join(parent, dockSidecarDir)); serr == nil {
		t.Error("a refused run must not write the inventory sidecar")
	}
	assertNoStaging(t, parent)
}

// ---- 3. hard-linked destination leaf ----------------------------------------

// A legitimate destination whose leaf is already a HARD LINK to a registered
// source original. Root confinement alone does not settle this: the destination
// name is inside the authorized root, and it is the SHARED OBJECT behind it that
// must survive. Stage-and-publish replaces the NAME; a truncating open would
// rewrite the original through the link.
func TestDestAuth_HardLinkedDestinationLeaf(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "Payload", map[string]string{"payload.txt": "NEW DESTINATION BYTES\n"})
	prot := registerProtected(t, app, map[string]string{"orig.txt": "ORIGINAL SOURCE BYTES\n"})
	dest := t.TempDir()

	origPath := filepath.Join(prot, "orig.txt")
	linkPath := filepath.Join(dest, "payload.txt")
	if err := os.Link(origPath, linkPath); err != nil {
		t.Skipf("SKIP (NOT TESTED): hard links unavailable on this host/filesystem: %v", err)
	}
	origIdent := identOf(t, origPath)
	if identOf(t, linkPath) != origIdent {
		t.Fatal("precondition: the destination leaf must be the same object as the protected original")
	}
	protTree := treeOf(t, prot)

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	if res.Mirrored != 1 {
		t.Fatalf("mirrored=%d failed=%d, want 1 mirrored", res.Mirrored, res.Failed)
	}
	// The protected original keeps its bytes AND its identity.
	if b, _ := os.ReadFile(origPath); string(b) != "ORIGINAL SOURCE BYTES\n" {
		t.Errorf("protected original was overwritten through the hard link: %q", b)
	}
	if got := identOf(t, origPath); got != origIdent {
		t.Errorf("protected original is a different object now: %+v -> %+v", origIdent, got)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("protected tree changed: %v -> %v", protTree, got)
	}
	// The destination now holds the NEW bytes in a DIFFERENT object — the publish
	// replaced the name, it did not write through the shared inode.
	if b, _ := os.ReadFile(linkPath); string(b) != "NEW DESTINATION BYTES\n" {
		t.Errorf("destination leaf should hold the copied bytes, got %q", b)
	}
	if identOf(t, linkPath) == origIdent {
		t.Error("destination leaf is still the same object as the protected original; the publish did not break the link")
	}
	assertNoStaging(t, dest)
}

// ---- 4. failure completion ---------------------------------------------------

// A publication that fails must leave the previous good destination byte-identical
// and the same object, remove only the staging this operation created, and report
// the failure. This is the no-delete-destination-fallback guarantee carried
// through the root-relative publish.
func TestDestAuth_PublishFailurePreservesPriorDestination(t *testing.T) {
	app, collID, src, vol := mirrorSetup(t, "Keep", map[string]string{"keep.txt": "V2 BYTES\n"})
	dest := t.TempDir()
	prior := filepath.Join(dest, "keep.txt")
	if err := os.WriteFile(prior, []byte("V1 GOOD BYTES\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	priorIdent := identOf(t, prior)

	injected := errors.New("injected publish failure")
	orig := beforeDestPublish
	beforeDestPublish = func(*os.Root, string, string) error { return injected }
	defer func() { beforeDestPublish = orig }()

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("the job itself should complete and report the per-file failure: %v", err)
	}
	if res.Mirrored != 0 || res.Failed != 1 {
		t.Errorf("job must report the actual failure: mirrored=%d failed=%d skipped=%d, want 0/1/0", res.Mirrored, res.Failed, res.Skipped)
	}
	if b, _ := os.ReadFile(prior); string(b) != "V1 GOOD BYTES\n" {
		t.Errorf("a failed publish must leave the previous copy intact, got %q", b)
	}
	if got := identOf(t, prior); got != priorIdent {
		t.Errorf("a failed publish must not replace the previous object: %+v -> %+v", priorIdent, got)
	}
	assertNoStaging(t, dest)
	if got := treeOf(t, src); len(got) != 1 || got[0] != "keep.txt" {
		t.Errorf("source must be untouched, got %v", got)
	}
}

// The error a failed publish returns must name the operation, not be swallowed.
func TestDestAuth_PublishFailureReportsTheCause(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	writeTree(t, src, map[string]string{"x.txt": "bytes\n"})
	dest, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer dest.Close()

	injected := errors.New("injected publish failure")
	orig := beforeDestPublish
	beforeDestPublish = func(*os.Root, string, string) error { return injected }
	defer func() { beforeDestPublish = orig }()

	_, _, cerr := copyVerifyToRoot(dest, filepath.Join(src, "x.txt"), "x.txt", nil, nil)
	if cerr == nil {
		t.Fatal("a failed publish must surface as an error")
	}
	if !errors.Is(cerr, injected) {
		t.Errorf("the underlying cause must be preserved, got %v", cerr)
	}
	if !strings.Contains(cerr.Error(), "publish") {
		t.Errorf("the error should say what failed, got %v", cerr)
	}
	if errors.Is(cerr, errCopySource) {
		t.Error("a destination-side failure must not be reported as a source read failure")
	}
	assertNoStaging(t, dest.Name())
}

// ---- 5. sidecar and cleanup obey the same authority --------------------------

// Sidecars and failure cleanup used to travel rebuilt absolute pathnames. They now
// go through the same validated, root-relative path as payload writes: the same
// rejection for an escaping relative path, and the sidecar lands under the
// authorized destination and nowhere else.
func TestDestAuth_SidecarAndCleanupObeyAuthority(t *testing.T) {
	app, collID, src, vol := mirrorSetup(t, "Side", map[string]string{"a.txt": "alpha\n"})
	dest := t.TempDir()

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil || res.Mirrored != 1 {
		t.Fatalf("mirror: mirrored=%d err=%v", res.Mirrored, err)
	}
	if _, serr := os.Stat(filepath.Join(dest, dockSidecarDir, "INVENTORY.md")); serr != nil {
		t.Errorf("sidecar must land under the authorized destination: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(src, dockSidecarDir)); serr == nil {
		t.Error("a sidecar must never appear in a registered source")
	}

	d, err := app.Store.AuthorizeDestination(dest)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer d.Close()

	escapes := []string{
		"../escape.md",
		"sub/../../escape.md",
		"/etc/escape.md",
		"",
		"./escape.md",
	}
	for _, rel := range escapes {
		if err := d.WriteFileAtomic(rel, []byte("x"), 0o644); err == nil {
			t.Errorf("sidecar write must refuse %q", rel)
		} else if !errors.Is(err, errRelEscape) {
			t.Errorf("sidecar write of %q must be refused as unconfined, got %v", rel, err)
		}
		if err := d.MkdirAll(rel); err == nil && rel != "" && rel != "./escape.md" {
			t.Errorf("MkdirAll must refuse %q", rel)
		}
		if err := d.Discard(rel); err == nil {
			t.Errorf("cleanup must refuse %q", rel)
		} else if !errors.Is(err, errRelEscape) {
			t.Errorf("cleanup of %q must be refused as unconfined, got %v", rel, err)
		}
		if err := d.Publish(rel, "ok.txt"); err == nil {
			t.Errorf("publish must refuse staging name %q", rel)
		}
		if err := d.Publish("ok.tmp", rel); err == nil {
			t.Errorf("publish must refuse final name %q", rel)
		}
	}
	// A drive-qualified relative path is refused too — the catalog is not trusted to
	// supply only well-formed relative names.
	if err := d.WriteFileAtomic(`C:/absolute.md`, []byte("x"), 0o644); err == nil {
		t.Error("a drive-qualified path must be refused")
	}
	assertNoStaging(t, dest)
}

// Staging is uniquely owned and never follows an existing leaf: a file that merely
// has the staging suffix is not truncated, and two staging requests never collide.
func TestDestAuth_StagingIsUniquelyOwned(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer d.Close()

	relA, fa, err := d.stagingFor("f.txt")
	if err != nil {
		t.Fatalf("stagingFor: %v", err)
	}
	relB, fb, err := d.stagingFor("f.txt")
	if err != nil {
		t.Fatalf("stagingFor (second): %v", err)
	}
	if relA == relB {
		t.Errorf("two staging files for the same target must not share a name: %s", relA)
	}
	if !strings.HasSuffix(relA, mirrorTmpSuffix) || !strings.HasSuffix(relB, mirrorTmpSuffix) {
		t.Errorf("staging names must keep the %s suffix: %s / %s", mirrorTmpSuffix, relA, relB)
	}
	if _, err := d.root.OpenFile(relA, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); err == nil {
		t.Error("staging must not open an existing file")
	}
	if _, err := fa.Write([]byte("OWNED A")); err != nil {
		t.Fatal(err)
	}
	if _, err := fb.Write([]byte("OWNED B")); err != nil {
		t.Fatal(err)
	}
	if got, err := d.HashRel(relA); err != nil || got != sha256Hex([]byte("OWNED A")) {
		t.Fatalf("owned stage changed: %q %v", got, err)
	}

	_ = d.Discard(relA)
	_ = d.Discard(relB)
}

// ---- 6. symlink aliases — a DIFFERENT mechanism from junctions ---------------

// Symbolic links resolve by different rules than directory junctions, so they get
// their own fixture. Where symlink creation is not permitted, this SKIPS with a
// reason; it never reports a pass it did not earn.
func TestDestAuth_SymlinkAliasOfSourceRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "SymSrc", map[string]string{"a.txt": "alpha\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	alias := filepath.Join(t.TempDir(), "dest-alias")
	if err := os.Symlink(prot, alias); err != nil {
		t.Skipf("SKIP (NOT TESTED): symbolic links cannot be created on this host: %v", err)
	}
	protTree := treeOf(t, prot)
	protIdent := identOf(t, filepath.Join(prot, "keepme.txt"))

	if _, err := app.MirrorToVolume(collID, nil, alias, vol.ID, 0, "", noProgMirror); err == nil {
		t.Fatal("a symlink alias of a registered source must be refused as a destination")
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("protected tree changed through a symlink alias: %v -> %v", protTree, got)
	}
	if got := identOf(t, filepath.Join(prot, "keepme.txt")); got != protIdent {
		t.Errorf("protected original replaced: %+v -> %+v", protIdent, got)
	}
}

// A symlink NESTED below a genuinely safe destination must not redirect a leaf
// write. The root is correct; the escape is one level down, which no entry-point
// pathname check can answer.
func TestDestAuth_NestedSymlinkLeafRefused(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "SymNest", map[string]string{"sub/inner.txt": "INNER\n"})
	prot := registerProtected(t, app, map[string]string{"keepme.txt": "PROTECTED\n"})
	dest := t.TempDir()
	if err := os.Symlink(prot, filepath.Join(dest, "sub")); err != nil {
		t.Skipf("SKIP (NOT TESTED): symbolic links cannot be created on this host: %v", err)
	}
	protTree := treeOf(t, prot)

	res, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("the destination root is genuinely safe, so the job should run: %v", err)
	}
	if res.Failed != 1 || res.Mirrored != 0 {
		t.Errorf("the leaf write through a nested symlink must fail: mirrored=%d failed=%d", res.Mirrored, res.Failed)
	}
	if got := treeOf(t, prot); !equalStrings(protTree, got) {
		t.Errorf("a leaf write escaped into the protected source: %v -> %v", protTree, got)
	}
}

// ---- 7. the boundary is reached through the real workflows -------------------

// BackupChanges (mirror mode) lands through the same authorized root, sidecar
// included — the incremental caller is covered by its own destination effects, not
// by sharing a helper.
func TestDestAuth_BackupChangesLandsThroughAuthorizedRoot(t *testing.T) {
	app, collID, src, vol := mirrorSetup(t, "Inc", map[string]string{"a.txt": "alpha\n", "b.txt": "bravo\n"})
	dest := createdForTest(t, app, filepath.Join(t.TempDir(), "not-created-yet", "mirror"))

	beforeIdent := identOf(t, filepath.Join(src, "a.txt"))
	res, err := app.BackupChanges(collID, nil, vol.ID, BaseVolume, ModeMirror, dest, 0, "", noProgMirror)
	if err != nil {
		t.Fatalf("backup changes: %v", err)
	}
	if res.Files != 2 || res.Failed != 0 {
		t.Fatalf("files=%d failed=%d, want 2/0", res.Files, res.Failed)
	}
	// The bootstrap created the missing components through an authorized ancestor.
	for _, rel := range []string{"a.txt", "b.txt", dockSidecarDir + "/INVENTORY.md"} {
		if _, serr := os.Stat(filepath.Join(dest, filepath.FromSlash(rel))); serr != nil {
			t.Errorf("expected %s under the bootstrapped destination: %v", rel, serr)
		}
	}
	if got := identOf(t, filepath.Join(src, "a.txt")); got != beforeIdent {
		t.Errorf("source leaf replaced by an incremental run: %+v -> %+v", beforeIdent, got)
	}
	assertNoStaging(t, dest)
}

// A destination that IS a registered source, and one INSIDE a registered source,
// stay refused — the directions the old check did cover must not regress.
func TestDestAuth_EqualAndInsideSourceStillRefused(t *testing.T) {
	app, collID, src, vol := mirrorSetup(t, "Same", map[string]string{"x.txt": "hi\n"})
	for _, dest := range []string{src, filepath.Join(src, "mirror"), filepath.Join(src, "deep", "mirror")} {
		_, err := app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
		assertSourceRefusal(t, "mirror to "+dest, err)
		assertSourceRefusal(t, "check and create "+dest, refusedForTest(t, app, dest))
	}
	if got := treeOf(t, src); len(got) != 1 || got[0] != "x.txt" {
		t.Errorf("refused runs must leave the source alone, got %v", got)
	}
}

// A destination whose identity cannot be established is refused rather than
// guessed at: destRel rejects the shapes that would otherwise be handed to the
// filesystem to adjudicate.
func TestDestAuth_RelPathValidation(t *testing.T) {
	ok := []string{"a.txt", "sub/a.txt", "sub/deep/a.bin", "OBELISK_DOCK/INVENTORY.md"}
	for _, r := range ok {
		if _, err := destRel(r); err != nil {
			t.Errorf("destRel(%q) must be accepted, got %v", r, err)
		}
	}
	bad := []string{"", "  ", ".", "..", "../x", "a/../../x", "/abs", "//server/share/x", `C:\x`, "C:/x", "a//b", "a/./b"}
	for _, r := range bad {
		if _, err := destRel(r); err == nil {
			t.Errorf("destRel(%q) must be refused", r)
		} else if !errors.Is(err, errRelEscape) {
			t.Errorf("destRel(%q) must be refused as unconfined, got %v", r, err)
		}
	}
}

// A destination that names an existing FILE is a clear refusal, not a surprise
// later in the run.
func TestDestAuth_FileAsDestinationRefused(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "not-a-folder.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Store.AuthorizeDestination(p); err == nil {
		t.Fatal("a file must not be authorized as a destination")
	} else if !strings.Contains(err.Error(), "not a folder") {
		t.Errorf("the refusal should say what is wrong, got %v", err)
	}
}

// The canonical name an authorization decides about comes from the OPEN HANDLE,
// and the operator's spelling is kept only for messages and records.
func TestDestAuth_CanonicalComesFromTheHandle(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	defer d.Close()
	if d.Name() != dir {
		t.Errorf("Name() should keep the operator's spelling: %q vs %q", d.Name(), dir)
	}
	if d.Canonical() == "" {
		t.Fatal("Canonical() must be established")
	}
	// Same object, however it is spelled.
	if normPath(d.Canonical()) != normPath(mustEvalSymlinks(t, dir)) {
		t.Errorf("canonical %q does not match the resolved location %q", d.Canonical(), mustEvalSymlinks(t, dir))
	}
	fmt.Fprintf(os.Stderr, "authorized %s as %s\n", d.Name(), d.Canonical())
}

func mustEvalSymlinks(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", p, err)
	}
	a, err := filepath.Abs(r)
	if err != nil {
		t.Fatalf("Abs(%s): %v", r, err)
	}
	return a
}
