package main

// catalog_open_test.go — OB-001: an existing catalog cannot silently become a new one.
//
// The defect: OpenStore classified the catalog read as "err == nil or nothing". Every
// other read outcome — permission denied, an I/O error, a directory in the way, a share
// held by antivirus, a data dir whose mount is not up — fell through with an EMPTY
// in-memory catalog. Seeding then flagged the store as recovered and saved, renaming a
// three-profile empty catalog over the unread original, after which dailyBackup spent a
// backup slot on the empty bytes.
//
// What the zero-length case actually did, corrected (independent review, F-1):
//
//	A zero-length catalog.json did NOT take that path at baseline. json.Unmarshal sits
//	inside the old `err == nil` block and rejects empty input before `existed` is ever
//	consulted, so d97809b2 already failed closed with "catalog.json is damaged:
//	unexpected end of JSON input", left the file at 0 bytes and wrote no backup. That
//	was verified by executing the pristine baseline, not by reading it.
//	`existed = len(b) > 0` was a LATENT HAZARD, never a live defect. This change gives
//	the case an earlier and more actionable diagnostic; it must NOT be credited with
//	preventing a zero-length overwrite, because no such overwrite happened. The test
//	below therefore pins a MESSAGE, not a repaired fall-through, and says so.
//
// How these tests are built, and why:
//
//   - They drive openStore, the production function, with its two boundaries injected.
//     They do NOT stand in a mock that already behaves correctly: the injected read
//     result flows through the same switch a real os.ReadFile result does.
//   - Injection is by PARAMETER, not by a package-level hook, so there is no shared
//     mutable state, nothing to restore, and no way for one test's fault to reach
//     another. (They still avoid t.Parallel: openStore reads the package-level
//     openStoreFailSave seam that persistence_test.go and durability_gate_test.go set.)
//   - "No write was attempted" is asserted with the persistObserver seam, NOT with the
//     absence of catalog.json.tmp. A successful rename removes that name, so its absence
//     afterwards is equally consistent with a write having happened. The observer records
//     the attempt before any gate can turn it into an early return; two positive controls
//     below prove the observer is wired to every path it claims to cover.
//   - Fixture setup legitimately writes a daily backup, so these tests snapshot the
//     fixture directory AFTER the fixture is persisted and require it to be identical
//     afterwards. They never assert that no backups exist.
//
// The observer's scope, stated without ambiguous arithmetic (F-2, F-3):
//
//	store.go has SIX filesystem call sites that mutate state, which group into FOUR
//	named operations the observer reports plus TWO it does not. Call sites and operation
//	groups are different counts; "four" refers to operations, not call sites.
//
//	  observed  catalog-write      writeCatalog:1372, ahead of the failSave seam and the
//	                               read-only gate; covers BOTH the os.OpenFile of
//	                               catalog.json.tmp (1409) and the os.Rename onto
//	                               catalog.json (1424) — two call sites, one operation
//	  observed  backup-create      dailyBackup:1477, before os.WriteFile (1478)
//	  observed  backup-prune       dailyBackup:1483, before os.Remove (1484)
//	  observed  pre-schema-backup  backupBeforeMigrate:1345, before os.WriteFile (1346)
//
//	  NOT observed  os.MkdirAll(dataDir) at openStore:1133
//	  NOT observed  saveJobs (os.WriteFile:3362 + os.Rename:3363)
//
//	Why saveJobs is excluded, corrected: NOT because "OpenStore only reads the sidecar".
//	It does not only read it. loadJobs (store.go:3370) calls saveJobs (store.go:3350) at
//	3400-3401 whenever it reconciles a job left RUNNING into INTERRUPTED, so a SUCCESSFUL
//	startup after an unclean shutdown does write jobs.json. The exclusion is justified by
//	CONTROL FLOW, not by read-only-ness: s.loadJobs() is at store.go:1283, the statement
//	before `return s, nil` at 1284, while every refusal returns earlier — 1164
//	(zero-length), 1172 (invalid JSON), 1183 (unreadable). A REJECTED open can therefore
//	never reach loadJobs, so no jobs write is reachable on the path these tests claim.
//	Nothing here modifies loadJobs, saveJobs or config loading; that fail-open family is
//	OBX-004 and stays separate.
//
// What the two kinds of evidence each establish, and what neither establishes:
//
//   - Observer evidence establishes ATTEMPTED operations, within the four groups above.
//   - Fixture-state comparison establishes PRESERVATION of the observed fixture state:
//     names, types and bytes of every entry in the disposable data directory.
//   - Neither an absent temp file nor matching final bytes alone proves no write was
//     attempted, which is why both are asserted and neither is asserted alone.
//   - The guarantee is NOT "no filesystem write anywhere during OpenStore".
//     os.MkdirAll(dataDir) at store.go:1133 runs BEFORE the catalog read and does mutate
//     the filesystem; it is outside this bounded claim by design. The claim is: after a
//     REJECTED catalog read, no catalog or backup mutation is attempted, and the fixture
//     directory is byte-identical.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// ---- helpers ---------------------------------------------------------------

// persistSpy records every attempted authority write openStore's observer reports.
type persistSpy struct {
	mu   sync.Mutex
	arm  bool
	ops  []string
	live bool // set once the spy has seen anything at all, armed or not
}

func (p *persistSpy) observer() func(op, path string) {
	return func(op, path string) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.live = true
		if p.arm {
			p.ops = append(p.ops, op+" "+filepath.Base(path))
		}
	}
}

// armAfterFixture starts recording. Called only once the fixture is fully persisted, so
// legitimate setup writes are never counted against the operation under test.
func (p *persistSpy) armAfterFixture() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.arm, p.ops = true, nil
}

func (p *persistSpy) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.ops...)
}

func (p *persistSpy) sawAnything() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.live
}

// fixtureEntry is one entry in the disposable fixture directory: what KIND of thing it
// is and, for a regular file, its exact content. Recording the kind as well as the bytes
// catches a file swapped for a directory or a symlink — which a bytes-only comparison
// would surface only as a read error, and a glob-based one would miss entirely.
type fixtureEntry struct {
	kind string // "file", "dir", "symlink", or "irregular" (device, socket, pipe, ...)
	size int64  // regular files only
	sum  string // regular files: SHA-256 of the exact bytes. symlinks: the raw link target.
}

func (e fixtureEntry) String() string {
	switch e.kind {
	case "file":
		return fmt.Sprintf("file %d bytes sha256:%s", e.size, e.sum[:16])
	case "symlink":
		return "symlink -> " + e.sum
	default:
		return e.kind
	}
}

// fixtureState is EVERY entry in the disposable fixture directory, keyed by its
// slash-separated path relative to that directory.
//
// This replaces an earlier snapshot that globbed only catalog.json plus .bak-* and
// .pre-schema-* (independent review, F-4). That was narrower than the claim it backed:
// it could not have seen a leftover catalog.json.tmp, a newly created jobs.json, or a
// new subdirectory. Enumerating the whole fixture directory means the comparison no
// longer depends on having guessed the write paths correctly — temporary and migration
// artifacts are inside the claimed scope and are covered by construction.
type fixtureState map[string]fixtureEntry

// snapshotFixture enumerates dir in full. It is deliberately strict: any enumeration or
// read failure is a t.Fatalf, never a skipped entry, so an artifact that cannot be
// inspected is reported instead of silently dropped from the comparison.
//
// filepath.WalkDir does NOT follow symlinks, so a link planted inside the fixture is
// recorded by its target string and never walked through. Traversal therefore stays
// within the disposable fixture directory and can never reach production storage.
func snapshotFixture(t *testing.T, dir string) fixtureState {
	t.Helper()
	st := fixtureState{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("enumerating %s: %w", p, walkErr)
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return fmt.Errorf("relativising %s: %w", p, rerr)
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil // the fixture root itself
		}
		switch {
		case d.IsDir():
			st[rel] = fixtureEntry{kind: "dir"}
		case d.Type()&fs.ModeSymlink != 0:
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return fmt.Errorf("reading link %s: %w", p, lerr)
			}
			st[rel] = fixtureEntry{kind: "symlink", sum: target}
		case d.Type().IsRegular():
			b, berr := os.ReadFile(p)
			if berr != nil {
				return fmt.Errorf("reading %s: %w", p, berr)
			}
			sum := sha256.Sum256(b)
			st[rel] = fixtureEntry{kind: "file", size: int64(len(b)), sum: hex.EncodeToString(sum[:])}
		default:
			// A device, socket or pipe. Recorded by kind rather than omitted, so its
			// appearance or disappearance still fails the comparison.
			st[rel] = fixtureEntry{kind: "irregular"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	if _, ok := st["catalog.json"]; !ok {
		t.Fatalf("snapshot %s: no catalog.json — the fixture is not what this test assumes", dir)
	}
	return st
}

// requireUnchanged proves the fixture directory is exactly what it was: same entries,
// same kinds, same bytes. Nothing added, nothing removed, nothing rewritten.
//
// This is PRESERVATION evidence about observed state. It is not, on its own, evidence
// that no write was attempted — a write that failed, or one that rewrote identical
// bytes, would leave this comparison happy. The persistObserver assertions carry the
// "attempted" half of the claim; the two are asserted together and neither alone.
func (before fixtureState) requireUnchanged(t *testing.T, dir, what string) {
	t.Helper()
	after := snapshotFixture(t, dir)

	for _, name := range sortedFixtureKeys(before) {
		got, ok := after[name]
		if !ok {
			t.Errorf("%s: %s was REMOVED (was %s)", what, name, before[name])
			continue
		}
		if got != before[name] {
			t.Errorf("%s: %s changed\n  before: %s\n  after:  %s", what, name, before[name], got)
		}
	}
	for _, name := range sortedFixtureKeys(after) {
		if _, ok := before[name]; !ok {
			t.Errorf("%s: %s was CREATED (%s) — the refused open wrote something", what, name, after[name])
		}
	}
}

func sortedFixtureKeys(m fixtureState) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// seedRealCatalog builds a genuine catalog through the production path and returns the
// directory, the collection id that must survive, and a spy already installed. The spy
// is deliberately NOT armed yet — this setup is allowed to write, including a daily
// backup.
func seedRealCatalog(t *testing.T) (dir string, collID int, spy *persistSpy) {
	t.Helper()
	dir = t.TempDir()
	spy = &persistSpy{}
	st, err := openStore(dir, os.ReadFile, spy.observer())
	if err != nil {
		t.Fatalf("fixture: openStore: %v", err)
	}
	c := st.AddCollection("Irreplaceable")
	if c == nil {
		t.Fatal("fixture: AddCollection returned nil")
	}
	st.mu.Lock()
	werr := st.writeCatalog()
	st.mu.Unlock()
	if werr != nil {
		t.Fatalf("fixture: writeCatalog: %v", werr)
	}
	if !spy.sawAnything() {
		t.Fatal("fixture: the persist observer saw no write at all — the seam is not wired")
	}
	return dir, c.ID, spy
}

// faultyReadAt returns a reader that fails for the catalog path only and reads normally
// everywhere else, so the injected fault is scoped to the fixture under test.
func faultyReadAt(t *testing.T, catPath string, b []byte, err error) func(string) ([]byte, error) {
	t.Helper()
	return func(p string) ([]byte, error) {
		if p != catPath {
			return os.ReadFile(p)
		}
		return b, err
	}
}

// ---- positive control ------------------------------------------------------

// TestPersistObserver_SeesRealMutations is the control for every "zero attempts"
// assertion below. A spy that is simply unwired also reports zero, which would prove
// nothing. This shows the observer fires on three of the four observed operation groups:
// the catalog write, the daily-backup creation, and the pre-migration backup.
//
// The fourth, backup-prune, needs more than 14 sidecars before the production path will
// fire at all, so it has its own control in TestPersistObserver_SeesBackupPrune below.
// Between the two, every operation the observer claims to cover is demonstrated wired.
func TestPersistObserver_SeesRealMutations(t *testing.T) {
	// Catalog write plus daily backup, on a fresh directory.
	dir := t.TempDir()
	spy := &persistSpy{}
	st, err := openStore(dir, os.ReadFile, spy.observer())
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	spy.armAfterFixture()
	st.AddCollection("observable")
	ops := spy.recorded()
	if len(ops) == 0 {
		t.Fatal("observer recorded nothing for a real catalog mutation — it is not wired")
	}
	if !hasOpPrefix(ops, "catalog-write") {
		t.Errorf("observer missed the catalog write; recorded %v", ops)
	}

	// The daily backup fires on the first write of the day in a fresh process. Confirm
	// the observer covers that path too, on a directory that has not been written yet.
	dir2 := t.TempDir()
	spy2 := &persistSpy{}
	spy2.armAfterFixture() // arm from the very start: we want the seeding save's backup
	if _, err := openStore(dir2, os.ReadFile, spy2.observer()); err != nil {
		t.Fatalf("openStore: %v", err)
	}
	if ops2 := spy2.recorded(); !hasOpPrefix(ops2, "backup-create") {
		t.Errorf("observer missed the daily-backup creation; recorded %v", ops2)
	}

	// Pre-migration backup, via a legacy (unversioned) catalog.
	dir3 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir3, "catalog.json"),
		[]byte(`{"next_id":{"collection":1},"collections":[{"id":1,"name":"Old"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	spy3 := &persistSpy{}
	spy3.armAfterFixture()
	if _, err := openStore(dir3, os.ReadFile, spy3.observer()); err != nil {
		t.Fatalf("openStore on a legacy catalog: %v", err)
	}
	if ops3 := spy3.recorded(); !hasOpPrefix(ops3, "pre-schema-backup") {
		t.Errorf("observer missed the pre-migration backup; recorded %v", ops3)
	}
}

func hasOpPrefix(ops []string, prefix string) bool {
	for _, o := range ops {
		if strings.HasPrefix(o, prefix) {
			return true
		}
	}
	return false
}

func opsWithPrefix(ops []string, prefix string) []string {
	var out []string
	for _, o := range ops {
		if strings.HasPrefix(o, prefix) {
			out = append(out, strings.TrimPrefix(o, prefix+" "))
		}
	}
	return out
}

func backupSidecars(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "catalog.json.bak-*"))
	if err != nil {
		t.Fatalf("glob backups in %s: %v", dir, err)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, filepath.Base(m))
	}
	sort.Strings(out)
	return out
}

// TestPersistObserver_SeesBackupPrune is the missing positive control for the fourth
// observed operation group (independent review, F-3). Without it, "the observer covers
// backup-prune" rested on reading store.go:1483 rather than on running it.
//
// It drives the REAL production path — dailyBackup's retention loop, reached through
// writeCatalog exactly as any ordinary save reaches it — and never calls the observer
// directly, because a test that invokes the notification itself would prove only that a
// function call works.
//
// Determinism, without touching the clock, the retention constant or production code:
//
//   - The 15 synthetic sidecars are dated in the far past (2020), and dailyBackup sorts
//     its glob by NAME (store.go:1481), so which files prune is fixed by their names.
//     Nothing depends on mtime resolution, on sleeping, or on what day it is.
//   - Pruning only runs on a Store's FIRST write of the day, because dailyBackup returns
//     early while s.lastBak already equals today (store.go:1470-1473). The fixture store
//     has already taken that slot, so the test opens a SECOND store over the same
//     directory — a fresh Store with an empty lastBak — and writes through that.
//   - The expected prune count is computed from the observed before-state rather than
//     hard-coded, so a midnight rollover between fixture and mutation (which would add
//     one backup-create) changes the arithmetic without breaking the test.
func TestPersistObserver_SeesBackupPrune(t *testing.T) {
	dir, _, spy := seedRealCatalog(t)

	// Synthetic sidecars, disposable and distinguishable. Far-past dates so they sort
	// below today's real one and are the ones retention drops.
	const synthetic = 15
	for i := 1; i <= synthetic; i++ {
		name := filepath.Join(dir, fmt.Sprintf("catalog.json.bak-202001%02d", i))
		if err := os.WriteFile(name, []byte(fmt.Sprintf("synthetic backup %02d\n", i)), 0o644); err != nil {
			t.Fatalf("fixture: write %s: %v", name, err)
		}
	}
	before := backupSidecars(t, dir)
	if len(before) != synthetic+1 { // 15 synthetic + the fixture's own daily backup
		t.Fatalf("fixture: expected %d sidecars, got %d: %v", synthetic+1, len(before), before)
	}

	// Fixture is complete; everything from here is the operation under test.
	spy.armAfterFixture()

	st, err := openStore(dir, os.ReadFile, spy.observer())
	if err != nil {
		t.Fatalf("reopen for the pruning write: %v", err)
	}
	st.AddCollection("triggers a save") // -> save -> writeCatalog -> dailyBackup -> prune

	ops := spy.recorded()
	pruned := opsWithPrefix(ops, "backup-prune")
	created := opsWithPrefix(ops, "backup-create")

	if len(pruned) == 0 {
		t.Fatalf("the observer recorded NO backup-prune for a write that must prune "+
			"%d sidecars down to 14; recorded %v", len(before), ops)
	}

	// What retention must have dropped, derived from the observed before-state.
	wantPrunes := len(before) + len(created) - 14
	if len(pruned) != wantPrunes {
		t.Errorf("observer recorded %d prune(s), want %d (%d sidecars before + %d created - 14 retained)\n  pruned: %v",
			len(pruned), wantPrunes, len(before), len(created), pruned)
	}

	// The pruning was real, not merely announced: every reported file is gone from disk.
	for _, name := range pruned {
		if _, serr := os.Stat(filepath.Join(dir, name)); serr == nil {
			t.Errorf("observer reported pruning %s, but the file is still on disk", name)
		} else if !errors.Is(serr, fs.ErrNotExist) {
			t.Fatalf("checking pruned %s: %v", name, serr)
		}
	}

	// And retention landed where it should: the oldest went, the newest stayed.
	after := backupSidecars(t, dir)
	if len(after) != 14 {
		t.Errorf("retention must leave exactly 14 sidecars, got %d: %v", len(after), after)
	}
	for _, gone := range []string{"catalog.json.bak-20200101", "catalog.json.bak-20200102"} {
		if containsString(after, gone) {
			t.Errorf("%s is the oldest sidecar and should have been pruned; still present in %v", gone, after)
		}
		if !containsString(pruned, gone) {
			t.Errorf("the observer did not report pruning %s; reported %v", gone, pruned)
		}
	}
	if !containsString(after, "catalog.json.bak-20200115") {
		t.Errorf("the newest synthetic sidecar must survive retention; remaining: %v", after)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---- the regression: read failures must fail closed ------------------------

// TestOpenStore_UnreadableCatalogFailsClosed covers the read-error shapes that used to
// fall through into new-catalog initialization. Each asserts the full contract: no
// usable Store, an error that preserves the cause, zero attempted authority writes, and
// byte-identical catalog and backup state.
func TestOpenStore_UnreadableCatalogFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		bytes   []byte
		err     error
		wantErr error
	}{
		{
			name:    "permission denied",
			err:     &fs.PathError{Op: "open", Path: "catalog.json", Err: fs.ErrPermission},
			wantErr: fs.ErrPermission,
		},
		{
			name:    "io error",
			err:     &fs.PathError{Op: "read", Path: "catalog.json", Err: errors.New("input/output error")},
			wantErr: nil, // no sentinel; checked by message below
		},
		{
			// The dangerous shape: a short read that hands back SOME bytes together with
			// an error. Parsing those would silently truncate the catalog.
			name:    "partial bytes with an error",
			bytes:   []byte(`{"schema_version":`),
			err:     &fs.PathError{Op: "read", Path: "catalog.json", Err: errors.New("unexpected end of device")},
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, collID, spy := seedRealCatalog(t)
			catPath := filepath.Join(dir, "catalog.json")
			before := snapshotFixture(t, dir)
			spy.armAfterFixture()

			st, err := openStore(dir, faultyReadAt(t, catPath, tc.bytes, tc.err), spy.observer())

			if err == nil {
				t.Fatal("openStore must fail closed when the catalog cannot be read; it returned no error")
			}
			if st != nil {
				t.Error("openStore must not return a usable Store built on an unread catalog")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error must preserve the underlying read failure; got %v", err)
			}
			if !strings.Contains(err.Error(), "cannot read") {
				t.Errorf("error should name the failure as a read problem, got %q", err)
			}
			if ops := spy.recorded(); len(ops) != 0 {
				t.Errorf("a refused startup attempted %d authority write(s): %v", len(ops), ops)
			}
			before.requireUnchanged(t, dir, "refused startup ("+tc.name+")")

			// And the catalog is genuinely intact: clear the fault, reopen, find the data.
			st2, err := openStore(dir, os.ReadFile, nil)
			if err != nil {
				t.Fatalf("reopen after clearing the fault: %v", err)
			}
			if st2.Collection(collID) == nil {
				t.Fatal("the original archive is gone — the catalog did not survive the refused startup")
			}
		})
	}
}

// TestOpenStore_ZeroLengthCatalogIsDamagedNotNew covers the torn-write artifact.
//
// Read the header note on F-1 before changing this test. Baseline d97809b2 ALREADY
// rejected a zero-length catalog.json: json.Unmarshal ran unconditionally inside the old
// `err == nil` block and failed with "unexpected end of JSON input", leaving the file at
// 0 bytes and writing no backup. So this test pins a MESSAGE and the point at which the
// refusal happens — it does not pin a repaired overwrite, because there was none. Run it
// against the old logic and it fails only on the strings.Contains(..., "empty") line,
// which is exactly the evidence for that correction.
//
// The value it carries is still real: rejecting before json.Unmarshal keeps the operator
// out of a dead end by naming the .bak-YYYYMMDD sidecar to recover from, and it pins
// `existed = len(b) > 0` shut so the latent hazard cannot be reintroduced by a later
// refactor that moves the parse.
func TestOpenStore_ZeroLengthCatalogIsDamagedNotNew(t *testing.T) {
	dir, _, spy := seedRealCatalog(t)
	catPath := filepath.Join(dir, "catalog.json")

	// Truncate the catalog in place, the way a power loss during publication would.
	if err := os.WriteFile(catPath, nil, 0o644); err != nil {
		t.Fatalf("truncate fixture catalog: %v", err)
	}
	before := snapshotFixture(t, dir)
	if got := before["catalog.json"]; got.size != 0 {
		t.Fatalf("fixture should be zero-length, got %d bytes", got.size)
	}
	spy.armAfterFixture()

	st, err := openStore(dir, os.ReadFile, spy.observer())
	if err == nil {
		t.Fatal("a zero-length catalog.json must be reported as damaged, not initialized as new")
	}
	if st != nil {
		t.Error("openStore must not return a usable Store for a damaged catalog")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("the error should say the file is empty, got %q", err)
	}
	if ops := spy.recorded(); len(ops) != 0 {
		t.Errorf("a refused startup attempted %d authority write(s): %v", len(ops), ops)
	}
	// The empty file is left exactly as found — including its backups, which still hold
	// the recoverable content.
	before.requireUnchanged(t, dir, "zero-length catalog")
}

// ---- preserved behavior ----------------------------------------------------

// TestOpenStore_InvalidJSONStillFailsWithDamagedMessage pins the pre-existing
// invalid-JSON contract. It already failed closed; this change must not reword or
// reclassify it.
func TestOpenStore_InvalidJSONStillFailsWithDamagedMessage(t *testing.T) {
	dir := t.TempDir()
	catPath := filepath.Join(dir, "catalog.json")
	garbage := []byte(`{"collections":[`)
	if err := os.WriteFile(catPath, garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	spy := &persistSpy{}
	spy.armAfterFixture()
	st, err := openStore(dir, os.ReadFile, spy.observer())
	if err == nil {
		t.Fatal("invalid JSON must still refuse to open")
	}
	if st != nil {
		t.Error("invalid JSON must not return a usable Store")
	}
	if !strings.Contains(err.Error(), "damaged") {
		t.Errorf("the established message says the catalog is damaged, got %q", err)
	}
	if ops := spy.recorded(); len(ops) != 0 {
		t.Errorf("a refused startup attempted %d authority write(s): %v", len(ops), ops)
	}
	after, rerr := os.ReadFile(catPath)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	if !bytes.Equal(garbage, after) {
		t.Error("an invalid catalog must be left exactly as found")
	}
}

// TestOpenStore_NewerSchemaStillOpensReadOnly guards the forward-compatibility contract
// against an over-broad reading of "fail closed". A newer-schema catalog must still
// OPEN, read-only — refusing it outright would be a regression of
// TestCatalogNewerSchemaIsReadOnly and would lock an operator out of viewing their data.
func TestOpenStore_NewerSchemaStillOpensReadOnly(t *testing.T) {
	dir := t.TempDir()
	catPath := filepath.Join(dir, "catalog.json")
	newer := []byte(`{"schema_version":999,"collections":[{"id":1,"name":"Future"}]}`)
	if err := os.WriteFile(catPath, newer, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := openStore(dir, os.ReadFile, nil)
	if err != nil {
		t.Fatalf("a newer-schema catalog must still open (read-only), got error: %v", err)
	}
	ro, why := st.ReadOnly()
	if !ro {
		t.Fatal("a newer-schema catalog must open read-only")
	}
	if !strings.Contains(why, "newer") {
		t.Errorf("the read-only reason should explain the newer-version refusal: %q", why)
	}
	if len(st.Collections()) != 1 {
		t.Error("read-only viewing must still work")
	}
	after, rerr := os.ReadFile(catPath)
	if rerr != nil {
		t.Fatalf("read back: %v", rerr)
	}
	if !bytes.Equal(newer, after) {
		t.Error("a newer-schema catalog must never be rewritten on disk")
	}
}

// TestOpenStore_FreshDirectoryStillInitializes pins initialization compatibility: the
// absent case is the one branch that may still create a catalog, and it must behave
// exactly as before.
func TestOpenStore_FreshDirectoryStillInitializes(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir, os.ReadFile, nil)
	if err != nil {
		t.Fatalf("a fresh directory must initialize normally: %v", err)
	}
	if st == nil {
		t.Fatal("a fresh directory must return a usable Store")
	}
	if len(st.Profiles()) == 0 {
		t.Error("a new catalog must still be seeded with the built-in profiles")
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Errorf("the seeding save must still persist a new catalog: %v", err)
	}
	// The exported entry point must behave identically.
	dir2 := t.TempDir()
	if _, err := OpenStore(dir2); err != nil {
		t.Errorf("OpenStore must still initialize a fresh directory: %v", err)
	}
}

// TestOpenStore_NotExistShapesAllInitialize confirms the not-found classification uses
// errors.Is rather than an exact type, so a wrapped or PathError-shaped not-found is
// still recognized as "absent" and not mistaken for an unreadable catalog.
func TestOpenStore_NotExistShapesAllInitialize(t *testing.T) {
	shapes := map[string]error{
		"bare sentinel": fs.ErrNotExist,
		"PathError":     &fs.PathError{Op: "open", Path: "catalog.json", Err: fs.ErrNotExist},
		"wrapped":       fmt.Errorf("stat layer: %w", fs.ErrNotExist),
	}
	for name, readErr := range shapes {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			catPath := filepath.Join(dir, "catalog.json")
			st, err := openStore(dir, faultyReadAt(t, catPath, nil, readErr), nil)
			if err != nil {
				t.Fatalf("a not-found read must still initialize a new catalog, got: %v", err)
			}
			if st == nil {
				t.Fatal("a not-found read must return a usable Store")
			}
		})
	}
}

// TestOpenStore_ExistingCatalogReopensIntact is the plain success path: a supported
// existing catalog opens, keeps its contents and ids, and is not rewritten by the act of
// opening it.
func TestOpenStore_ExistingCatalogReopensIntact(t *testing.T) {
	dir, collID, _ := seedRealCatalog(t)
	before := snapshotFixture(t, dir)

	st, err := openStore(dir, os.ReadFile, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	c := st.Collection(collID)
	if c == nil {
		t.Fatal("the collection must survive a reopen")
	}
	if c.Name != "Irreplaceable" {
		t.Errorf("collection name = %q, want %q", c.Name, "Irreplaceable")
	}
	// Not just catalog.json: opening a current-schema catalog must leave the whole data
	// directory alone — no rewrite, no new sidecar, no jobs.json conjured into being.
	before.requireUnchanged(t, dir, "reopening a current-schema catalog")
}
