# Next actions after PR-00

**Pinned SHA: `d97809b2f730e09960632e2943e562531fa095a3`**

Derived from [BASELINE-2026-09-06.md](BASELINE-2026-09-06.md) and [OB_STATUS.md](OB_STATUS.md).
Original OB IDs and PR numbers from the handoff's `02_FIRST_PULL_REQUESTS.md` are preserved.

---

> **Status update 2026-09-06.** PR-01 has been **implemented and tested**, uncommitted, on branch
> `fix/ob-001-catalog-open`. The report is
> [reviews/PR01-OB-001-2026-09-06.md](reviews/PR01-OB-001-2026-09-06.md).
>
> **The regression test proposed in this document was NOT used.** The PR-00 review addendum
> identified three defects in it, all correct: its `catalog.json.bak-*` assertion would have
> failed against its own fixture (setup legitimately writes a daily backup); an absent
> `catalog.json.tmp` proves nothing, because a successful rename removes that name too; and a
> fault hook returning early would bypass the branch under repair. The shipped tests in
> `catalog_open_test.go` use snapshot-and-compare of catalog and backup bytes, an observation
> seam at the authority-write boundary with a positive control, and read-result injection through
> the production decision branch. The block below is retained for provenance — **do not lift it**.
>
> One claim in the section below is also **wrong** and is corrected in `OB_STATUS.md`: a
> zero-length `catalog.json` did not reach new-catalog initialization at baseline; `json.Unmarshal`
> already rejected it.

> **Status update 2026-09-06 (independent review).** PR-01 has been independently reviewed:
> [reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md](reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md).
> Outcome **NEEDS_CHANGES**, scoped to documentation and test completeness; the reviewer verified
> the production decision logic by execution and **requested no production-logic change**. The
> response — corrected test comments, a `backup-prune` positive control, and a whole-directory
> fixture comparison, all in `catalog_open_test.go` with `store.go` untouched — is
> [reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md](reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md).
> PR-01 is still **uncommitted** and PR-02 is still **not started**. The acceptance line below
> that reads "195 pass / 2 fail / 4 skip" is the PR-00 baseline and is **not** a target to hold
> after adding tests; compare failure identities and causes instead, as that paragraph already says.

> **Status update 2026-09-06 (checkpoint committed).** The focused recheck
> ([reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md](reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md))
> closed F-1, F-3, F-4 and F-5. The remaining half of F-2 — the `persistObserver` comment that
> justified the `saveJobs` exclusion by claiming `OpenStore` only ever reads the sidecar — was
> corrected in `store.go` as a **comment-only** edit, recorded in section 11 of the follow-up
> report; the full suite was not rerun after it. PR-01 is therefore **no longer uncommitted**: it
> is committed to `fix/ob-001-catalog-open` as
> `20c425d9ca6a4c5c1bbd79d33235a788b8201fa4` (`store.go`, `catalog_open_test.go`) on baseline
> `d97809b2f730e09960632e2943e562531fa095a3`. **PR-02 is still not started**, nothing is merged,
> and OB-001 is **not** resolved as a whole — the initialization-identity residual stays open, as
> `OB_STATUS.md` records. Everything above this block is retained as written, including the
> superseded regression-test proposal and the corrections to it.

> **Status update 2026-09-07 (PR-02 started and implemented).** PR-01 is published; **PR-02 /
> OB-003 is now implemented and uncommitted** on branch `fix/ob-003-safe-replacement`, cut from
> the published PR-01 checkpoint `4cd867b2ddb26c945f7c74faee8ef32780743be7`. The destructive
> delete-destination-and-retry fallback in `atomicRename` is gone; a failed publication now
> leaves the existing file byte-identical and returns the underlying error. Changed:
> `mirror.go`, `appbackup.go`, and the new `atomic_replace_test.go`. PR-01's `store.go` and
> `catalog_open_test.go` are **verified byte-identical** and were not touched. Report:
> [reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md](reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md).
>
> Awaiting **one substantive independent review**; nothing is staged, committed or pushed, and
> **PR-03 is not started**. Full suite: 212 pass / 2 fail / 4 skip — the +8 are the new tests, and
> the 2 failures remain the OBX-001 Windows TAB fixture pair. The section below describes PR-01,
> which is complete; it is retained as written for provenance.

> **Status update 2026-09-07 (PR-02 review findings applied).** F-1 and F-2 from the same-session
> review are applied and verified; see
> [reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md](reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md).
> The production delta for that work was **comment-only**. PR-02 remains **uncommitted** on
> `fix/ob-003-safe-replacement` and now awaits a **fresh review by someone who has not seen it** —
> both existing PR-02 reviews came from the authoring session. **PR-03 is not started.**

> **Status update 2026-09-07 (PR-02 externally reviewed, owner accepted, published as a branch
> checkpoint).** PR-02 / OB-003 received the required review from **outside the authoring
> session**, the owner accepted it for publication, and the reviewed patch is now **committed and
> pushed on `fix/ob-003-safe-replacement`** (stacked on the unmerged PR-01). **F-1/F-2 closed.**
> Report:
> [reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md](reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md).
> This is a checkpoint of the reviewed artifact only — **not merged, not released, not a
> production-readiness claim.**
>
> The review was a **separate AI source review plus isolated Linux probes**, not a human audit.
> Full repository **build/vet/tests were dependency-blocked** in that sandbox. The isolated Linux
> race success is **not** full-product or Windows race evidence, and **Windows race testing
> remains NOT TESTED**. The Windows **212 pass / 2 fail / 4 skip** figure remains *reported*
> evidence from the local authoring/review sessions — **not re-executed during publication**, and
> **no new test run or CI pass is implied** by this push.
>
> **Open and untouched:** the two OBX-001 Windows TAB-fixture failures, cleanup-error
> observability, key/keystore permissions, staging-path aliasing, crash durability, and PR-01's
> initialization-identity residual. **PR-03 is not started.**

## The one exact next implementation target

**PR-01 / OB-001 — an existing catalog cannot silently become a new catalog.**

It is first because it is the only confirmed path at this commit that destroys the **entire**
catalog unattended, it has **no dependencies**, and the fix is small and local. Every other P0
either depends on it or has a narrower blast radius per occurrence.

### The change

In `OpenStore` (`store.go:1109`), replace the swallow at `store.go:1119`:

```go
if b, err := os.ReadFile(s.path); err == nil {
```

with an explicit three-way branch:

1. **Positively identified not-found** (`errors.Is(err, fs.ErrNotExist)`) — this is a genuinely
   new catalog. Continue exactly as today: `existed` stays false, seeding runs, the recovery save
   persists it.
2. **Any other read error** — return it wrapped, so `main.go:71` refuses to boot. Nothing is
   written. This is the whole fix.
3. **Read succeeded but the file is zero-length** — treat as damaged, not new. Today
   `existed = len(b) > 0` at `store.go:1120` routes a torn write onto the brand-new path, which
   skips the schema gate and `backupBeforeMigrate` and then overwrites.

Invalid JSON already fails closed at `store.go:1122` and needs no change.

**Do not change:** the seeding logic, the recovery-save decision, or `writeCatalog`. Keep the
diff inside `OpenStore`.

**Model to copy:** `readStore` at `pipeline.go:339` — the one reader in the codebase that already
distinguishes not-found, unreadable, not-a-keystore and forward-schema.

### Non-scope for PR-01

The same fail-open shape in `loadJobs` (`store.go:3293`) and `LoadConfig` (`pipeline.go:146`) is
OBX-004 and belongs in its own change. Do not widen PR-01 to cover them — `LoadConfig` in
particular has a live-behavior consequence (`SaveConfig` persisting defaults over real settings)
that deserves its own tests.

### The regression test

Written out here, **not applied**. Nothing was added to the Go package this session, so the build
and the 201-test suite are unchanged. Lift this verbatim into a new
`catalog_failclosed_test.go` in PR-01.

Two design notes. First, it uses a **read fault seam** rather than `os.Chmod`, because chmod 000
is a no-op on Windows — `cardcheck_test.go:147` already skips for exactly that reason, and a test
that skips on the maintainer's own platform is not coverage. Adding the seam is part of PR-01;
it mirrors the existing `failSave` and `openStoreFailSave` seams and stays nil in production.
Second, the strongest assertion is not "an error was returned" but **"no write was attempted"** —
the handoff calls this out specifically.

```go
package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// A read fault seam alongside the existing failSave / openStoreFailSave seams:
//
//	var catalogReadFault func(path string) error   // in store.go, nil in production
//
// consulted at the top of the OpenStore read so a test can inject EACCES/EIO
// without needing platform-specific permission behavior.

// TestOpenStore_UnreadableCatalogFailsClosed is the OB-001 regression: a catalog that
// cannot be READ (as opposed to one that is genuinely absent) must stop startup and must
// not be replaced by a new, empty authority.
func TestOpenStore_UnreadableCatalogFailsClosed(t *testing.T) {
	dir := t.TempDir()

	// A real catalog with real content.
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	c := st.AddCollection("Irreplaceable")
	path := filepath.Join(dir, "catalog.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("fixture produced an empty catalog")
	}
	beforeInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Injected permission failure on the read — not a not-found.
	catalogReadFault = func(string) error { return fs.ErrPermission }
	defer func() { catalogReadFault = nil }()

	st2, err := OpenStore(dir)
	if err == nil {
		t.Fatal("OpenStore must fail closed when the catalog cannot be read; it returned no error")
	}
	if st2 != nil {
		t.Fatal("OpenStore must not return a usable Store built on an unread catalog")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("error should wrap the underlying read failure, got %v", err)
	}

	// The bytes on disk are untouched: no empty replacement, no seeding save.
	after, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("catalog must still exist after a refused startup: %v", rerr)
	}
	if string(after) != string(before) {
		t.Fatal("catalog bytes changed after a refused startup — the original was overwritten")
	}
	afterInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Error("catalog was rewritten (mtime changed) during a startup that should have written nothing")
	}

	// No write was ATTEMPTED: the temp file writeCatalog would use must not exist, and no
	// daily backup may have been created from empty bytes.
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("writeCatalog was attempted during a refused startup (catalog.json.tmp exists)")
	}
	baks, _ := filepath.Glob(path + ".bak-*")
	if len(baks) > 0 {
		t.Errorf("a daily backup was written during a refused startup: %v", baks)
	}

	// And the catalog is genuinely intact once the fault clears.
	catalogReadFault = nil
	st3, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("reopen after clearing the fault: %v", err)
	}
	if st3.Collection(c.ID) == nil {
		t.Fatal("the original archive is gone — the catalog did not survive the refused startup")
	}
}

// TestOpenStore_ZeroLengthCatalogIsDamagedNotNew covers the torn-write case: an empty
// catalog.json is the classic power-loss artifact, and treating it as "brand new" skips the
// schema gate and the pre-migrate backup, then overwrites it.
func TestOpenStore_ZeroLengthCatalogIsDamagedNotNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenStore(dir); err == nil {
		t.Fatal("a zero-length catalog.json must be reported as damaged, not initialized as new")
	}
	if b, err := os.ReadFile(path); err != nil || len(b) != 0 {
		t.Errorf("the zero-length file must be left exactly as found, got len=%d err=%v", len(b), err)
	}
}

// TestOpenStore_AbsentCatalogStillInitializes pins the other side of the branch: a genuinely
// missing catalog is still a legitimate new-catalog case and must keep working.
func TestOpenStore_AbsentCatalogStillInitializes(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("an absent catalog must initialize normally: %v", err)
	}
	if len(st.Profiles()) == 0 {
		t.Error("a new catalog must still be seeded with the built-in profiles")
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Errorf("the seeding save must still persist a new catalog: %v", err)
	}
}
```

**Also required in PR-01:** confirm `TestOpenStore_RecoverySaveFailureIsNonFatal`
(`persistence_test.go:47`) and the four `schema_test.go` tests still pass unchanged. That first
test pins a deliberate decision — a failed *recovery save* stays non-fatal — and failing closed
on a failed *read* must not regress it. The two are compatible: one is a read, the other a write.

### Acceptance for PR-01

- The three tests above pass; `persistence_test.go` and `schema_test.go` pass unchanged.
- `go build ./...`, `go vet ./...`, `gofmt -l` over tracked files stay clean.
- Suite result is no worse than the recorded baseline (195 pass / 2 fail / 4 skip — the two
  failures are OBX-001 and unrelated).
- The PR description carries: scope and non-scope, OB-001, before and after SHA, the UI message
  shown when startup is refused, and the exact commands run.

---

## First five bounded PRs

Default handoff sequence retained. Current evidence supports it rather than overriding it, so
nothing is reordered. Two annotations are recorded below where evidence adds something.

| PR | Issues | Scope | Depends on | Acceptance test |
|---|---|---|---|---|
| **PR-01** | OB-001 | Fail closed on a catalog read that is not a positive not-found; treat zero-length as damaged | None | Injected read failure: startup refused, **no write attempted**, bytes byte-identical; absent / zero-length / invalid-JSON / forward-schema each distinct |
| **PR-02** | OB-003 | Remove the `os.Remove(final)` fallback from `atomicRename`; use a real replacement primitive | None | Missing temp source with an existing final: replacement fails, existing final survives unchanged. Permission, sharing and cross-device injected, with OS-specific evidence |
| **PR-03** | OB-002 | `EndBatch` returns its final flush error; the nine `defer` sites fold it into their result; terminal job persistence gates the reported outcome | OB-001 | Fail the final batch flush and the jobs sidecar separately: no job reports COMPLETED. Two concurrent batched jobs do not share a flush assumption |
| **PR-04** | OB-004, OB-007 prerequisite slices | Resolve filesystem identity instead of comparing strings; re-validate at each leaf mutation | None (toolchain decision on `os.Root`) | Symlink and junction source aliases, nested output escape, ancestor rename between preflight and write: registered sources unchanged |
| **PR-05** | OB-005 remainder | Bound the problems slice; persist problems and completeness with the committed capture; represent unknown subtree extent; bring `AdoptFolder` up to `ScanFolder`'s standard | OB-002 | Denied directory records unknown extent and never upgrades to complete coverage; a failed problem write fails the archival result; `AdoptFolder` reports paths and kinds, not just an integer |

### Annotations on the sequence

**PR-02 ranks second on its own merits, and there is a compounding reason.** `atomicRename` is
also what publishes `catalog.json`, `jobs.json` and `keystores/*` during app-state restore
(`appbackup.go:396`). The helper that can delete a good file without replacing it sits directly
in the catalog recovery path, which is the same asset PR-01 protects.

**OB-006 is the one candidate for pulling forward.** It is sequenced at PR-07 in the handoff, but
its blast radius is the largest in the register: `SyncKeystores` (`pipeline.go:490`) overwrites a
keystore that was skipped because it could not be read, and lost key material makes otherwise
healthy encrypted media permanently unrecoverable. Recommendation: keep the default order, but if
PR-04 stretches (the `os.Root` toolchain decision could make it), take OB-006 before it rather
than letting it wait behind three PRs. Not acted on in this session — recorded as the documented
alternative.

**A cheap, unblocked fix that is not in the sequence:** OBX-001. Moving `"tab\there.txt"` behind
the existing `runtime.GOOS != "windows"` guard at `tar_names_test.go:41` is a one-line change that
makes the suite green on Windows and restores the OB-008 regression coverage there. It touches
only a test file, so it does not belong in any of the five PRs above; it is worth doing first or
alongside PR-01. Adding a `windows-latest` CI job is the durable version of that fix.

---

## Follow-on, in dependency order after the first five

| Handoff PR | Issues | Blocked until |
|---|---|---|
| PR-06 | OB-007 | Needs a platform lock design decision; the accessor and `Update*` ownership work can start earlier |
| PR-07 | OB-006, OB-011 scoped | See the pull-forward note above; OB-011 needs an explicit Windows ACL scope first |
| PR-08 | OB-008 remainder, parts of OB-010, OBX-005 | OB-004, OB-005 |
| PR-09 | OB-012, OB-013 (and OB-019, OB-022 slices) | OB-002, OB-003, OB-007 |
| PR-10 | OB-009, OB-010, OB-021 | OB-002, OB-003, OB-004, OB-008 |

---

## Gaps this session did not close

Listed so nothing is implied complete.

1. **OB-014 through OB-036 and OPT-001 are `NOT_YET_REVALIDATED.**` Out of scope by the session
   brief. They must not be scheduled as work until revalidated — three of the thirteen items that
   *were* checked had already been partly fixed.
2. **No repository-wide file coverage ledger.** `NEXT_REVIEW_PROTOCOL.md` section 2 asks for one
   row per tracked file across all 130 Go files. This audit went deep on the OB-001..013 anchor
   files and did not account for the rest. The protocol says not to call an audit complete while
   unreviewed files are unexplained — by that standard, the repository-wide audit is still open.
3. **HTTP, authentication and origin behavior unaudited.** 162 canonical routes; only the job
   runner and a few handlers were read.
4. **High-risk modules named by the protocol but not independently validated:** migration and
   dock, incremental backup, finalization and coverage, hardware and optical writers, device
   identity, export formats, scheduler and alerts, config and tool-runner helpers.
5. **No cross-platform evidence.** Everything here is windows/amd64. OB-003, OB-004 and OB-011
   all have platform-specific behavior needing platform-specific evidence.
6. **No race evidence for this platform** (no C compiler) and **no tape, optical, Docker or
   hardware validation** (no helpers, no devices). A skipped hardware-dependent test is not a
   passing validation.
7. **The UI feature map is a route and navigation inventory, not a behavioral audit.** See
   [UI_FEATURE_MAP-2026-09-06.md](UI_FEATURE_MAP-2026-09-06.md).

---

## Working rules carried forward

- Reproduce or invalidate at HEAD before changing anything. Write the failing regression first.
- Make the smallest semantics-preserving repair. Do not replace working code to match a proposed
  implementation.
- Keep semantic fixes separate from mechanical extraction.
- An unavailable check is NOT TESTED with its cause, never PASS. A tool-gated skip is not a pass.
- A passing race run is evidence about the schedules exercised, not proof of concurrency safety.
- Never exercise a failure test against real originals, production catalogs, keystores, archival
  drives or media.
