//go:build !guionly

package main

// atomic_replace_test.go — OB-003: publishing a replacement must never destroy the
// file it was meant to replace.
//
// The defect: atomicRename responded to ANY first-rename failure by deleting the
// existing destination and retrying:
//
//	if err := os.Rename(tmp, final); err == nil { return nil }
//	_ = os.Remove(final)          // <- the good bytes, gone
//	return os.Rename(tmp, final)  // <- and this can fail too
//
// The comment above it justified the delete with the Windows target-exists case, but
// the code did not distinguish causes. A cross-device link, a permission denial, a
// sharing violation from another handle, a read-only or failing mount — each one took
// the delete path. When the retry then also failed, the previous good file was gone
// and the replacement had never been published; every caller's error handler then
// removed the temporary as well, so BOTH copies were destroyed.
//
// What these tests pin is a preservation property, not an atomicity claim: after a
// failed publication the destination is byte-identical to what it was before. See the
// scope comment on atomicRename for what "atomic" does and does not assert here.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withRenameFailure installs a fault-injecting rename for the duration of one test.
//
// An injected error proves how atomicRename HANDLES that error. It is not evidence
// that this operating system produces that error in the circumstance the sentinel is
// named after — that claim would need a real cross-device or real locked-handle setup,
// which these tests do not attempt. The names describe the simulated cause only.
func withRenameFailure(t *testing.T, err error) {
	t.Helper()
	prev := renameFile
	renameFile = func(string, string) error { return err }
	t.Cleanup(func() { renameFile = prev })
}

// mustBytes fails the test unless path holds exactly want.
func mustBytes(t *testing.T, path, want, whenWhat string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: destination unreadable after the operation (%v) — it must still be there", whenWhat, err)
	}
	if string(got) != want {
		t.Fatalf("%s: destination bytes changed.\n got: %q\nwant: %q", whenWhat, got, want)
	}
}

// ---- A. failure with an existing destination ------------------------------------

// TestAtomicRename_MissingTempPreservesDestination is the core OB-003 regression: a
// replacement that cannot be published must fail AND leave the previous file intact.
// A missing temporary is the cheapest way to make a real (uninjected) rename fail on
// every platform, so this case needs no fault injection at all.
func TestAtomicRename_MissingTempPreservesDestination(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "destination.bin")
	tmp := filepath.Join(dir, "destination.bin.mnemo_tmp") // deliberately never created

	const good = "the previous good bytes — these must survive"
	mustWrite(t, final, good)

	err := atomicRename(tmp, final)
	if err == nil {
		t.Fatal("atomicRename must report an error when the temporary source does not exist")
	}
	// The underlying cause stays inspectable through any wrapping.
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error should still identify the missing source; got %v", err)
	}
	mustBytes(t, final, good, "missing temporary source")
}

// ---- B. success with no existing destination ------------------------------------

func TestAtomicRename_PublishesWhenDestinationAbsent(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "fresh.bin")
	tmp := final + ".mnemo_tmp"

	const payload = "freshly staged bytes"
	mustWrite(t, tmp, payload)

	if err := atomicRename(tmp, final); err != nil {
		t.Fatalf("publishing to an absent destination must succeed: %v", err)
	}
	mustBytes(t, final, payload, "publish with no prior destination")
	if fileExists(tmp) {
		t.Error("the temporary must not remain after a successful publication")
	}
}

// ---- C. success replacing an existing destination -------------------------------

// TestAtomicRename_ReplacesExistingDestination runs on whatever host executes the
// suite. On Windows it is the load-bearing case: it establishes by execution that a
// single os.Rename DOES replace an existing file here, which is what makes the
// delete-first fallback unnecessary rather than merely unsafe. The old comment
// claimed the opposite; this test is the evidence for correcting it.
func TestAtomicRename_ReplacesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "published.bin")
	tmp := final + ".mnemo_tmp"

	mustWrite(t, final, "the older published bytes")
	const replacement = "the newer bytes that must win"
	mustWrite(t, tmp, replacement)

	if err := atomicRename(tmp, final); err != nil {
		t.Fatalf("replacing an existing destination must succeed on this host: %v", err)
	}
	mustBytes(t, final, replacement, "replace an existing destination")
	if fileExists(tmp) {
		t.Error("the temporary must not remain after a successful replacement")
	}
}

// ---- D. injected rename failures --------------------------------------------------

// TestAtomicRename_InjectedFailuresPreserveDestination covers the failure causes the
// deleted fallback used to swallow. Each sentinel simulates a cause; see
// withRenameFailure for what that does and does not establish.
func TestAtomicRename_InjectedFailuresPreserveDestination(t *testing.T) {
	cases := []struct {
		name     string
		injected error
	}{
		{"permission denied", fs.ErrPermission},
		{"cross-device link", errors.New("rename: cross-device link")},
		{"sharing violation", errors.New("rename: the process cannot access the file because it is being used by another process")},
		{"read-only filesystem", errors.New("rename: read-only file system")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			final := filepath.Join(dir, "destination.bin")
			tmp := final + ".mnemo_tmp"

			const good = "previous good bytes"
			mustWrite(t, final, good)
			mustWrite(t, tmp, "the replacement that will never be published")

			withRenameFailure(t, tc.injected)

			err := atomicRename(tmp, final)
			if err == nil {
				t.Fatal("a failing rename must be reported, never absorbed")
			}
			if !errors.Is(err, tc.injected) {
				t.Errorf("the underlying error identity must survive; got %v", err)
			}
			// The whole point of OB-003: no destructive fallback ran.
			mustBytes(t, final, good, tc.name)
		})
	}
}

// TestAtomicRename_FailureDoesNotConsumeTemporary records a deliberate division of
// labour: atomicRename leaves the temporary alone on failure, and each caller decides
// whether to discard it. If that ever changes, a caller's cleanup could become a
// double-delete of something it no longer owns.
func TestAtomicRename_FailureDoesNotConsumeTemporary(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "destination.bin")
	tmp := final + ".mnemo_tmp"
	mustWrite(t, final, "previous good bytes")
	mustWrite(t, tmp, "staged replacement")

	withRenameFailure(t, fs.ErrPermission)

	if err := atomicRename(tmp, final); err == nil {
		t.Fatal("expected the injected failure to be reported")
	}
	if !fileExists(tmp) {
		t.Error("atomicRename must not delete the temporary; that is the caller's decision")
	}
}

// ---- E. a real caller's failure-and-cleanup path ----------------------------------

// TestExportAppBackup_FailedPublishPreservesPreviousBundle exercises the hazard
// OB_STATUS calls out by name: auto-export reuses stable per-period file names, so a
// failed rename over an existing bundle used to delete the previous period's good
// backup and then — via the caller's own `_ = os.Remove(tmp)` — the new one too,
// leaving the operator with neither.
//
// This is the caller-level half of the guarantee: a safe helper is not enough if the
// caller cleans up the wrong file afterwards.
func TestExportAppBackup_FailedPublishPreservesPreviousBundle(t *testing.T) {
	app, _ := compatApp(t)
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "obelisk-appbackup-weekly.tar")

	// A good bundle from a previous run.
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatalf("seeding the previous bundle: %v", err)
	}
	before, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatalf("reading the seeded bundle: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("the seeded bundle is empty; the fixture proves nothing")
	}

	// The next run cannot publish.
	withRenameFailure(t, fs.ErrPermission)

	if _, err := app.exportAppBackupTo(tarPath, false); err == nil {
		t.Fatal("the export must report the failed publication rather than claiming success")
	}

	after, rerr := os.ReadFile(tarPath)
	if rerr != nil {
		t.Fatalf("the previous bundle must survive a failed export, but it is gone: %v", rerr)
	}
	if string(after) != string(before) {
		t.Fatal("the previous bundle must be byte-identical after a failed export")
	}
	// And the caller still tidies up its own staging file.
	if fileExists(tarPath + ".tmp") {
		t.Error("the export's temporary should not be left behind after a reported failure")
	}
}

// TestRestoreAppBackup_FailedPublishPreservesExistingFile covers the second
// appbackup caller — the one that publishes catalog.json, jobs.json, formats.json and
// keystores during app-state restore, and so is the caller where a destructive
// fallback would compound OB-001.
func TestRestoreAppBackup_FailedPublishPreservesExistingFile(t *testing.T) {
	app, dataDir := compatApp(t)
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "obelisk-appbackup-restore.tar")
	if _, err := app.exportAppBackupTo(tarPath, false); err != nil {
		t.Fatalf("building a backup to restore: %v", err)
	}

	// Known bytes at a destination the restore will try to replace.
	catalog := filepath.Join(dataDir, "catalog.json")
	const sentinel = `{"schema_version":1,"note":"the live catalog — must survive a failed restore"}`
	mustWrite(t, catalog, sentinel)

	// Fail every publication, recording which staging files the restore had actually
	// created at the moment each rename failed. Without that record, the "it is gone
	// afterwards" assertion below could pass vacuously — a file never created is also
	// a file not found.
	stagedAtFailure := map[string]bool{}
	prevRename := renameFile
	renameFile = func(tmp, final string) error {
		stagedAtFailure[tmp] = fileExists(tmp)
		return fs.ErrPermission
	}
	t.Cleanup(func() { renameFile = prevRename })

	if _, err := app.RestoreAppBackup(tarPath); err == nil {
		t.Fatal("a restore that cannot publish must report the failure")
	}
	mustBytes(t, catalog, sentinel, "failed app-state restore")

	// F-1: the staging file this restore created must not be left behind. The restore
	// returns at its first failed member, and gatherMembers always emits catalog.json
	// first (config.json is handled last), so with includeKeys=false the file exercised
	// here is the CATALOG staging file. Keystore staging is the same code path but is
	// NOT covered by this fixture — no keystore member exists in this bundle.
	catalogTmp := catalog + ".tmp"
	if !stagedAtFailure[catalogTmp] {
		t.Fatalf("fixture proves nothing: %s was never created before publication failed (observed: %v)",
			filepath.Base(catalogTmp), stagedAtFailure)
	}
	// Require a positive not-found. A permission or other stat error must fail the test
	// rather than be read as successful cleanup.
	if _, err := os.Lstat(catalogTmp); err == nil {
		t.Errorf("%s must be removed after a failed publication, but it is still there", filepath.Base(catalogTmp))
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("could not establish that %s is gone: %v", filepath.Base(catalogTmp), err)
	}
}

// ---- the correction is stated where the code is read ------------------------------

// TestAtomicRename_CommentDoesNotClaimDeleteFirst guards the comment that justified
// the defect. The claim "on Windows it fails if the target exists, so remove first" is
// contradicted by TestAtomicRename_ReplacesExistingDestination; if it ever returns,
// the reasoning that produced OB-003 has returned with it.
func TestAtomicRename_CommentDoesNotClaimDeleteFirst(t *testing.T) {
	src, err := os.ReadFile("mirror.go")
	if err != nil {
		t.Fatalf("reading mirror.go: %v", err)
	}
	for _, banned := range []string{"so remove first", "os.Remove(final)"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("mirror.go still contains %q — OB-003 is about not deleting the destination", banned)
		}
	}
}
