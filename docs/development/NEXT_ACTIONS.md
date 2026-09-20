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

> **Status update 2026-09-07 (OBX-001 fixture done; it uncovered OBX-006).** The one-line move
> above is **implemented and test-only** on `fix/obx-001-windows-fixture`, branched from the
> published PR-02 checkpoint `406ed2365b074398f4ef80094951753b961cf757`. Report:
> [reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md](reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md).
>
> **It does not make the suite green on Windows, and the paragraph above was wrong to expect
> that.** With the fixture fixed, both tests run real product code for the first time on Windows
> and fail there, in `BuildChunk`. That failure is filed as **OBX-006** (Windows bsdtar
> filename-list handling for the tested Unicode paths, related to OB-008) and is left unfixed by
> design — it needs its own scoped change and review.
>
> Suite: **212 pass / 2 fail / 4 skip** — **not green**, and no count is promised for a future fix.
> Build and vet clean. **Windows race still NOT TESTED** (verified: `-race` requires cgo,
> `CGO_ENABLED=0`, no gcc). **No CI ran.** The `windows-latest` CI job is **still not done**, so
> only the fixture sub-scope of OBX-001 is complete.
>
> **Next:** a separate bounded **Windows Unicode archive-build compatibility repair** for
> OBX-006, with **PR-03 queued immediately after** it. Neither is started.

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

---

## Update 2026-09-07 — OBX-006 needs a design decision before any code lands

Branch `fix/obx-006-windows-unicode-tar` (test-only, parent `c880c7d3`) diagnosed the Windows
Unicode archive-build failure and added its regressions. Full evidence:
[reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md](reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md).

**The demonstrated boundary.** With `C:\Windows\System32\tar.exe` (bsdtar 3.8.4) on an ACP-1252
host the build path can carry ASCII and CP1252-representable names, and — via argv only — any
*leaf* filename. It cannot carry a non-CP1252 character in a **directory component**, a **source
root** or a **staging/output path**, and cannot carry one at all through **`-T`**, the mechanism
the product uses. Every invocation-local remedy was tested and rejected with evidence; none is
both lossless and sufficient.

**Decide before implementing.** Options, smallest first:

1. **Windows-only Go `archive/tar` writer** behind the existing helper-selection boundary,
   external tar retained elsewhere and for all reads. Removes the failure entirely. Costs: two
   build paths, and `--format=posix` equivalence, manifest-first ordering and bounded-memory
   streaming must be proven byte-comparable; `cfg.Tools["tar"]` stops selecting the builder on
   Windows.
2. **Extend it to restore** (Go reader for selected members). This is what actually closes the
   round trip, and it would also give **OB-008** the extraction confinement it currently lacks —
   today confinement rests entirely on the external tar's defaults.
3. **Ship a known-good helper.** Packaging, provenance, signing, update and deployment burden on
   a 30-year archival tool. Not recommended without a decision.
4. **Refuse early and document** — reject non-representable names at plan time rather than at
   `tar`. A stopgap, not a fix; worth doing alongside 1 either way.

**Recommended next scope:** option 1 only, with the five new regressions as acceptance criteria
and an ASCII-package byte comparison against the helper to prove format equivalence.

**Queue discipline.** This does **not** displace **PR-03's P0 durability work**, which stays next.
OBX-006 is not started beyond the tests, and no archive-engine rewrite is authorised.

**Also queued, from the same evidence:** under the **`FAST` preset (`build_verify: none`)** nothing
compares tar members to the catalogue, so a wrongly named neighbour could reach a medium
unchecked. Pre-existing, surfaced here, unfixed. Track with OB-008/OB-009.

**Still outstanding from OBX-001:** the `windows-latest` CI lane. No CI has run.

---

## Update 2026-09-07 (later) — OBX-006 decision taken; FAST containment landed, pending review

Same branch `fix/obx-006-windows-unicode-tar`, same parent `c880c7d3`. Report:
[reviews/OBX-006-CONTAINMENT-2026-09-07.md](reviews/OBX-006-CONTAINMENT-2026-09-07.md).

- **Investigation completed for the pinned helper.** `C:\Windows\System32\tar.exe`
  (bsdtar 3.8.4) diagnosed; no further probing is queued.
- **Native writer direction recorded, implementation deferred.** Approved as the future
  direction with an explicit constraint list (investigation report §10a). It needs its own
  implementation scope covering the member-input contract, metadata and entry-type support
  (sparse files, links, ACLs and alternate data streams decided deliberately, not inherited),
  finalisation vs file-completion checks, external-reader interoperability and restore
  compatibility. **Do not start it as part of PR-03.**
- **New-build FAST containment implemented, pending review.** Windows external-tar builds
  whose *effective* tier disables content verification are refused before tar, key generation
  or staging. Contents/Full and non-Windows behaviour unchanged. Two existing tests had only
  their Windows expectation updated to the refusal.
- **Unicode compatibility remains open.** The seven compatibility failures are preserved
  deliberately; containment is not a fix and must not be described as one.
- **Existing-artifact exposure remains open.** Packages built unverified *before* this guard
  may exist. They are untouched and are **not** retroactively safe; whether they may still be
  written, rewritten or spanned needs its own assessment. Not scheduled here.
- **Verifier scope stays bounded.** `verifyTarContents` is not a namespace-security proof —
  member-name validation, source aliasing, missing hashes and restore confinement remain
  **OB-008**, **OBX-005** and **OB-009**.

**PR-03 / OB-002 remains the next substantial production workstream, and is not started.**

---

## Update 2026-09-07 (checkpoint) — OBX-006 containment accepted and published to its fix branch

The fresh-session review
[reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md](reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md)
returned `READY_FOR_OWNER_REVIEW` with **no blocker**, and the owner accepted it for
checkpointing. The change is committed and pushed to `fix/obx-006-windows-unicode-tar` **only**.
**Not authorised, not done:** merge, any change to `main`, a release, the native tar writer, or
PR-03.

**What is now published, stated precisely.**

- The guard decides on the **normalised effective** build-verify tier — the archive's own
  override if set, else the global, with legacy `"fast"` already normalised to `none`. It reads
  no preset label and no raw config string.
- It **refuses a new Windows external-tar build whose effective tier is `none`** before tool
  resolution, staging-directory creation, the `BUILDING` transition, key generation and every
  `tar` invocation. The package stays `PLANNED`; no staging directory, key, tar or payload is
  produced; no saved global or per-archive setting is altered.
- **Job failure bookkeeping may still occur.** Through the API a refusal records a FAILED job and
  a log line. **No claim of zero catalog activity is made, and this change does not fix job
  durability.**
- **Contents/Full still detect the reproduced wrong-file mismatch** — that check is the thing the
  containment depends on, and it was re-executed, not assumed.
- **Unicode compatibility itself remains unresolved.** The seven compatibility failures are
  preserved unchanged, are expected to fail, and must not be described as fixed.
- **Previously built unverified packages were not reassessed, altered or made safe** by this
  guard. Their eligibility for later write, rewrite or span operations still needs its own
  assessment, which has not been scheduled.
- **The native Windows tar writer is an approved future direction, not implemented
  functionality.**
- **OB-008**, **OBX-005**, **OB-009** and the other separately tracked boundaries remain open.

**Test evidence.** Prior executed review evidence, **not a new publication run**:
**220 pass / 7 fail / 4 skip**. The seven failures are the known Unicode compatibility failures.
**Windows race testing and CI remain NOT TESTED.** The suite is not green and the branch is not
release-ready.

**Optional test follow-ups from this specific review** — non-blocking, deliberately **not**
applied during publication so the accepted candidate stayed byte-identical. They belong to
`OBX-006-CONTAINMENT-REVIEW-2026-09-07.md` and are **not** the similarly numbered findings from
the PR-01 or PR-02 reviews:

1. **F1 — coverage gap.** Add a permanent in-tree test for a *verifying* archive override under a
   `FAST`/`none` global (correct today, but pinned only by an out-of-tree probe), and assert that
   a refusal preserves the **per-archive** override as well as the global config file.
2. **F2 — cosmetic.** The refusal tests inherit `nativeTools`' toolchain skip although the refusal
   path resolves no tool, so on a machine without the toolchain they would skip silently.

**Refs OBX-006 — the issue is not closed.**

**PR-03 / OB-002 remains the next substantial implementation workstream.** The native writer does
**not** move ahead of it. The published tip of `fix/obx-006-windows-unicode-tar` — not `c880c7d3`
— is the intended parent for the PR-03 branch.

---

## Update 2026-09-07 (later) — PR-03 / OB-002 implemented, pending review

Branch `fix/ob-002-durable-completion`, parent `f98eedf381253aae9a94dcfcc80f6bab2aec9317`
(the published OBX-006 containment checkpoint). Report:
[reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md](reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md).
**Uncommitted and unpushed** — implementation and evidence only.

**What it fixes.** Five open falsehoods on the completion path, all still present at the
parent: `EndBatch` discarded its final catalog write error; `EndBatch` wrote only at
`batchDepth == 0`, so a job finishing while another job held a batch wrote nothing and
depended on that unrelated job flushing later; `saveJobs` swallowed every failure and never
fsynced; `NewJob` ignored its own persistence failure; and `runJob` published `COMPLETED`
before the artifacts and result that describe it.

**The contract now.** Batch finalization returns its error and all nine batch owners fold it
into their own result; a finishing job always writes its own work; the jobs sidecar is a
checked, fsynced, atomically published write; a job whose initial record cannot be written
starts no work and returns 503; and the terminal status is published with its artifacts and
result in one checked write.

**Deliberately NOT claimed.** `catalog.json` and `jobs.json` remain two files and two
writes — this is not one atomic transaction. When the catalog commits and the job record
does not, the data and catalog are kept, the job is reported `COMPLETED` **plus** an
additive `persist_error` and a `NOT RECORDED` label, and a restart reports `INTERRUPTED`;
the failing sidecar is never retried. `syncDir` is still a **no-op on Windows**, so this is
not zero-loss crash durability.

**Evidence.** Ten new regressions in `durable_completion_test.go` drive the real production
paths and assert on bytes on disk or a genuine reopen. Red/green confirmed in a disposable
copy: eight fail without the fix for the intended reason; the all-succeed and
batch-bookkeeping controls pass in both. Full uncached suite **230 pass / 7 fail / 4 skip**
— **+10** (exactly the new tests), the **same seven** Unicode compatibility failures by
identity, **no new skips**. Build, vet and `gofmt` pass. **Windows race remains NOT TESTED**
(verified: `-race` needs cgo, `CGO_ENABLED=0`, no gcc). **No CI ran.**

**PR-01, PR-02, OBX-001 and OBX-006 behaviours are preserved** and their regressions pass
unchanged. Two test files were adapted for the new signatures only, and both were made
stricter rather than weaker.

**Next:** one substantive review of this patch. Not started and not authorised here: the
native tar writer, the containment review's optional F1/F2 tests, and PR-04.

---

## 2026-09-07 — OB-002 / PR-03 follow-up: the three review blockers are closed

Appended; nothing above is edited. Base unchanged: `f98eedf3…` on
`fix/ob-002-durable-completion`, still entirely uncommitted working-tree change.

**What changed.** The qualification of an unrecorded completion is now published in the
*same* lock hold as the terminal status (Blocker 1); the existing job UI and every in-tree
`waitJob` consumer read it and warn instead of reporting recorded success (Blocker 2); and
"not recorded right now" is a separate field from "a recording failure happened earlier",
so a job a later save legitimately records stops claiming its record is missing
(Blocker 3). Two false source comments about restart, the audit-fallback comment, and the
`runJob` call count (25 → **24**) were corrected in the same pass.

**The contract, in one line.** `unrecorded` is current state and is never present in a
successfully written file; `persist_error` is history and survives recovery and restart.
`saveJobs` clears the flag **before** marshalling and restores it if the write fails, so
the bytes and the published in-memory state always agree and nothing is marked recorded
optimistically.

**Deliberately NOT claimed.** Recovery is ordinary, not scheduled — there is **no retry
loop**; the next successful jobs write records the row, and until then the durable record
stays what was last written (`RUNNING` → `INTERRUPTED` on restart). `catalog.json` and
`jobs.json` are still two files and two writes. `syncDir` is still a **no-op on Windows**,
so "recorded" means the checked publication contract, not zero-loss power-failure
durability. The catalog audit fallback is an **attempt**, not a durable entry — measured
0 entries on disk both when the volume fails and when another job holds a batch open.
Job completion still never means the written bytes were read back and verified.

**Evidence.** Eight new regressions: seven in `durable_completion_followup_test.go`
(visibility gap direct + concurrent through the real HTTP handlers, later-successful
write, later-failed write, restart without recovery publication, combined
jobs+catalog failure, audit coalescing under an open batch) and one in
`durable_completion_ui_test.go`, which executes the **real** `<script>` block from
`ui/index.html` under `node` with a thin stub — no framework, nothing installed, and it
*skips with a named coverage gap* if `node` is absent. All ten earlier regressions are
retained and pass. Full uncached suite **238 pass / 7 fail / 4 skip** — **+8** (exactly
the new tests), the **same seven** Unicode compatibility failures by identity, **no new
skips**. Build, vet, `gofmt` pass. All three blockers were reproduced against the
pre-follow-up implementation in a disposable copy. **Windows race NOT TESTED** (`-race`
needs cgo; `CGO_ENABLED=0`, no gcc). **No CI ran.**

**Report:** `docs/development/reviews/PR03-OB-002-REVIEW-FOLLOWUP-2026-09-07.md`.

**Next:** one focused recheck of these three areas. Not started and not authorised here:
the native tar writer, the containment review's optional F1/F2 tests, and PR-04.

### Addendum — 7 September 2026, UI status-precedence correction

The focused recheck's one remaining blocker is fixed: `jobStamp`, `jobStampText` and
`jobRecordingNote` no longer let `unrecorded` displace the execution outcome, so a `FAILED`
job whose failure record also could not be written keeps the red `FAILED` stamp and gets
*"The job failed. Its failure record could not be saved."* as a separate qualification
instead of the completion sentence. `waitJob` carries the recording clause on its own
`recordUnsaved` field. `ui/index.html` only, **+56 / −11** this pass; **no Go production
file touched**. New coverage: `TestJobsUI_ExecutionOutcomeTakesPrecedence` (A–E, real page
script, composed list and detail output) and
`TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded`. Red against the pre-edit page in a
disposable copy, green against the corrected one.

Current figures, superseding `+756 / −109` and `+853 / −109`: tracked **15 files,
+900 / −110**; suite **240 pass / 7 fail / 4 skip** — **+2**, exactly the new tests, same
seven Unicode compatibility failures by identity. Build, vet, `gofmt` pass. **Windows race
NOT TESTED** (needs cgo). **No CI ran.**

**Report:** `docs/development/reviews/PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md`.

**Next:** a targeted recheck of this correction. Still not started and not authorised here:
the nested-`Result` aliasing and input-ownership nonblockers, the native tar writer, the
containment review's optional F1/F2 tests, and PR-04.

### Owner acceptance and checkpoint — 7 September 2026, PR-03 / OB-002

The PR-03 review chain and the final targeted recheck were **accepted by the owner for
publication**, and the bounded OB-002 completion-recording repair is committed on
`fix/ob-002-durable-completion` (implementation commit
`f98310cb3209f374cf883f7df2f3c31b300b3f0a`, parent `f98eedf381253aae9a94dcfcc80f6bab2aec9317`).
**A reviewed development checkpoint — not production readiness, not complete concurrency
safety, not universal crash durability.** Not merged; `main`, the earlier fix branches and the
tags were not moved; no release; no next workstream started.

**Accepted sub-scope.** Checked final catalog flushes propagate failure; a finishing batch no
longer depends on an unrelated batch's eventual flush under the implemented shared-catalog
checkpoint contract; failed initial job persistence prevents starting the work; execution
outcome and current recording qualification are exposed consistently (`unrecorded` = the
current snapshot's recording state, `persist_error` = earlier recording-failure history, and a
later successful ordinary jobs save may record a previously unrecorded terminal result);
`FAILED` stays `FAILED` when saving its failure record also fails, with execution failure and
recording failure presented separately in the UI.

**Deliberately still not started and not authorised by this acceptance:** removal or redesign
of the unused `recordUnsaved` marker; the recovery-kit caller's pre-existing
rejection-handling limitation; nested-`Result` aliasing and input-ownership hardening; the
native Windows tar writer; the containment review's optional F1/F2 tests; PR-04. Catalog data
and `jobs.json` remain two files, not one atomic transaction; "recorded" means the checked
publication contract only, not power-loss safety; `syncDir` is still a **no-op on Windows**.
The known Windows Unicode compatibility failures stand and **OBX-006 remains open**. No
baseline classification was erased and **no broader issue was closed** — `Refs OB-002` only.

**Evidence, as it actually stands.** Full suite **240 / 7 / 4** is **author-reported and was
not rerun** by the final targeted reviewer, who instead independently executed the enumerated
20-test selection (20/20), the 31 prior-safety tests (31/31), the two Node-backed UI logic
tests (executed, not skipped) and a labelled bounded mutation experiment (RED exit 1 / 22
failures, GREEN exit 0). **These sets are not combined into a full-suite figure and are not
assumed disjoint.** Windows `-race` **NOT TESTED**; **no CI ran**; the Node harness is not
browser-rendering or accessibility validation. Build, vet and `gofmt` re-run at publication and
pass.

**Next (not authorised here):** the next safety workstream should branch from the **published
PR-03 head of `fix/ob-002-durable-completion`**, not from `f98eedf3…`, which is no longer the
branch tip.


### 2026-09-19 - New-desktop baseline recorded

On setup/windows-nsott at 0dc7d5399c6889e015aebc9ba02e694a909d6e9c, Go 1.26.8 build/vet/format passed; uncached tests: 209 top-level pass / 0 fail / 39 skip, separately 10 subtests pass, and 3 system-disk probes excluded. Missing GPG/PAR2 accounts for 36 skips; this does not close Unicode/archive integration issues. Windows race, CI, Docker and hardware remain NOT TESTED. See [baseline](NEW_DESKTOP_BASELINE-2026-09-19.md) and [current handoff](CODEX_HANDOFF.md). PR-04 transfer and Figma design context remain pending. Repository-wide review, coverage ledger and feature matrix are not completed.

ONE next action: begin a bounded persistence source-review pass at this pinned SHA and initialize the review ledger. Preserve this baseline; do not rerun it as setup or recreate the earlier aborted reviewer reports.


### 2026-09-19 - Bounded persistence review complete

Published source 0dc7d5399c6889e015aebc9ba02e694a909d6e9c remains unchanged. [Review](PERSISTENCE_REVIEW-2026-09-19.md) and [coverage ledger](REVIEW_COVERAGE.csv): 208 tracked files inventoried; 4 fully reviewed, 13 partial, 144 inventoried only, 47 excluded. One disposable probe run: 13 top-level and 5 subtest passes, native exit 0, reproducing known unsafe behavior with positive controls. Baseline was not rerun. OBX-004/OB-006 confirmed; accepted PR-01/02/03 scopes preserved. PR-04/Figma remain separately pending.

ONE next action: prepare the bounded OB-006 validation-before-write repair and conflict/refused-participant regressions in a separately authorized task; sync currently overwrites rejected key stores. Keep partial publication and previous-generation retention as explicit residuals if outside that patch. Remaining repository review and feature matrix are not complete.


### 2026-09-19 - OB-006 validation candidate ready for review

On fix/ob-006-keystore-validation, parent 0dc7d5399c6889e015aebc9ba02e694a909d6e9c, strict participant validation and conflict-safe sync/status/lookup are implemented, uncommitted. First-use generation/build behavior is preserved. See [report](reviews/OB-006-KEYSTORE-VALIDATION-IMPLEMENTATION-2026-09-19.md). Final targeted 20 pass; build/vet/format pass; suite 226 pass / 0 fail / 39 skip, with 40 separately counted passing subtests and 3 excluded system-disk probes. Prior intermittent reopen failure reproduced on parent and candidate and retained. This is not all OB-006 solved: prior generations, partial publication, GenerateKey/catalog atomicity, ACL and ownership remain open.

ONE next action: independent review of the candidate and its no-mutation/conflict/first-use evidence. Nothing staged, committed, pushed or merged; no next fix started.


### 2026-09-19 - Accepted OB-006 checkpoint; next base

Implementation d147a823262757065d7817a233c0327523916e2a is the owner-accepted, unchanged validation/conflict slice; [accepted review](reviews/OB-006-KEYSTORE-VALIDATION-DETACHED-REVIEW-2026-09-19.md). Refs OB-006. Scope/residuals and local-only raw evidence are recorded in [handoff](CODEX_HANDOFF.md). Prior-generation, multi-store/concurrency, GenerateKey/catalog and ACL work remain open. No new full-suite run for publication.

ONE next action after branch publication: use the final evidence commit containing this note as the exact parent for the separately authorized configuration-loading sub-scope of OBX-004. Do not use 0dc7d539 as the repair base. Job-state loading, including loadJobs null-row handling, remains a subsequent sub-scope. Neither repair was begun here.


### 2026-09-19 - OBX-004 configuration candidate ready for review

Branch fix/obx-004-config-read-safety; exact parent 26918c5b8ed01ea301fc9a4c7658e19054a73632. [Implementation report](reviews/OBX-004-CONFIG-READ-SAFETY-IMPLEMENTATION-2026-09-19.md) records fail-closed config reads/updates, explicit first-use initialization, checked publication and caller propagation. Final targeted 39 top-level / 62 subtest passes; uncached suite 240 pass / 0 fail / 39 skip, plus 72 passing subtests; build/vet/format pass. Earlier catalog-reopen sharing failure is retained and reproduced on the exact accepted parent. Three system-disk probes excluded; missing helpers and platform limitations remain explicit.

ONE next action: separate review of this uncommitted candidate, especially first-use compatibility, caller propagation, mutation-observer evidence and publication-phase reporting. No stage, commit, push, merge or next fix here. Job-state loading/loadJobs null-row handling remains the next separate OBX-004 implementation sub-scope after acceptance. Broader repository review, PR-04 and Figma access remain separately pending.


### 2026-09-19 - R1/R2 correction ready for focused recheck

On fix/obx-004-config-read-safety, exact HEAD/base 26918c5b8ed01ea301fc9a4c7658e19054a73632. All 224 reviewed candidate hashes matched before edits; full pre-edit copies retained. [Follow-up report](reviews/OBX-004-CONFIG-READ-SAFETY-REVIEW-FOLLOWUP-2026-09-19.md) maps both controlling-review blockers to changes, pre/post identities and actual verification. The controlling review and original implementation report remain unchanged historical evidence.

R1: explicit initialization now links the checked staging file into an absent destination; it cannot replace an arriving file, directory or link. Unsupported hard links refuse without fallback. Post-link cleanup/directory-sync failures preserve Published=true and the final entry. Ordinary updates retain replacement. Deterministic late-arrival tests fail on the pre-follow-up candidate and pass now, including real Windows symlink coverage. R2: README/handbook document explicit Docker and Compose bootstrap, loopback/no published ports, full arguments/token/same storage, stop-and-wait, then normal startup without the flag. Deployment file changes are comments only. Defaults alongside catalog/key state do not recover lost settings.

Current executed evidence: target 52 top-level / 74 subtest passes; prior safety 43 / 10 passes; one uncached suite 244 top-level passes / 0 failures / 39 skips, with 84 passing subtests. Build/vet/format and Git Bash syntax checks pass. Actual-binary CLI lifecycle runs cover five natural refusals and four serving-then-forced-stop cases per execution, with exact exits retained. Docker runtime was unavailable and not executed. Missing GPG/PAR2, platform/opt-in skips, three excluded system-disk probes, Windows race/ACL/power-loss, other-platform runtime and CI limitations remain explicit. The previously recorded Windows sharing issue was not repaired; no such failure occurred in these runs.

READY_FOR_FOCUSED_RECHECK is an implementation status, not independent approval. ONE next action: focused recheck of R1/R2 and their retained evidence. No staging, commits, pushes, merges, branch changes or job-loading repair. loadJobs/null-row work remains a separate later sub-scope; broader concurrency, storage identity, malicious same-principal interference, cross-file transactions, PR-04 and Figma access remain deferred. Figma bytes are preserved.


## 2026-09-19 - OBX-004 configuration publication authorized

The owner requested completion of configuration publication after the [OBX-004-CONFIG-READ-SAFETY-FOCUSED-RECHECK-2026-09-19.md](reviews/OBX-004-CONFIG-READ-SAFETY-FOCUSED-RECHECK-2026-09-19.md) closed R1 and R2 as READY_FOR_OWNER_REVIEW. Implementation commit: `3ea444ae528af7343818142c867a8f955f9252aa` on `fix/obx-004-config-read-safety`, parent `26918c5b8ed01ea301fc9a4c7658e19054a73632`. The reviewed source, tests, deployment files and first-use documentation were committed unchanged. Prior implementation/review reports are preserved as historical evidence. This is a bounded branch checkpoint, not a merge, release or repository-wide approval.

Accepted scope: failed existing configuration reads never become defaults; ordinary updates preserve validated settings and unknown fields through checked replacement; explicit initialization publishes complete staged bytes with a no-replace hard link, refuses late entries, and reports post-publication errors truthfully. Startup/callers propagate failure. Docker/Compose first use explicitly initializes and serves on persistent state, then stops and waits before ordinary startup without the initialization flag. Hard-link support is required on the configuration/application-state filesystem; it is not imposed on backup media.

Evidence remains distinct: the author reports 244 top-level passes / 39 skips and 84 passing subtests. The focused recheck executed 39 top-level passes and 48 subtest passes, plus a separate dangling-symlink probe with two passing subtests, and reproduced the old overwrite against preserved pre-follow-up source. Its build, vet, formatting and shell-syntax checks passed. These overlapping results are not a new full-suite count. Publication reverified candidate hashes, exact staged content and whitespace; no new tests or CI run are claimed. Raw AppData/Temp logs and disposable fixtures remain local only.

Residuals: job loading/loadJobs null rows, broader storage identity, concurrent updates, cross-file transactions, retained generations and ACL work remain open. Docker runtime, Windows race, power loss, other-platform runtime and hardware are unverified; missing GPG/PAR2 still limits native integration. The known catalog-reopen sharing issue is not repaired by this checkpoint. Figma is preserved and excluded from publication.

The separate evidence commit containing this note is the intended parent for later job-load work. Do not use the former base `26918c5b8ed01ea301fc9a4c7658e19054a73632` or the implementation commit as that parent. Publication is complete only when `publication-receipt.md` records a successful push and a live origin branch tip equal to the full evidence-commit SHA. The receipt is a local post-push artifact, kept outside the evidence commit to avoid a self-referential SHA. No job-load branch or implementation is created by this publication task.


## 2026-09-19 - OBX-004 job-loading candidate awaiting review

READY_FOR_JOB_LOADING_REVIEW on `fix/obx-004-job-load-safety`, exact published parent `da22f1d9895d5350142a6f9b05ac41f0a920e170`. Configuration publication is complete: implementation `3ea444ae528af7343818142c867a8f955f9252aa`, evidence `da22f1d9895d5350142a6f9b05ac41f0a920e170`; the existing verified receipt is `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-publication-20260919\publication-receipt.md`. No publication steps were replayed for this local task. R1/R2 remain closed in their configuration scope.

The uncommitted [job-loading implementation](reviews/OBX-004-JOB-LOAD-SAFETY-IMPLEMENTATION-2026-09-19.md) validates the complete sidecar before adopting/reconciling rows, propagates failed reads/decodes/identity checks through OpenStore, and latches refused reloads against stale publication. Valid missing-sidecar first use, historical empty forms, single case aliases, optional fields, counter recovery and interrupted-job semantics remain supported. Startup refuses before serving; restore/migration callers retain errors and cannot save their stale board over detected rejected job state. NewJob refuses exhausted counters rather than wrapping IDs. Catalog recovery still precedes the jobs gate; restore and migration remain nontransactional across files.

Author execution in the current shared Codex session: preserved exact-parent source plus a portable regression produced 2 top-level failures / 20 failing subtests and a passing compatibility control. Initial focused candidate run: 85 top-level passes / 117 passing subtests. Final code: one native full-suite run, 254 top-level passes / 0 failures / 39 skips, separately 122 passing subtests; three system-disk probes excluded. Native build/vet and final formatting pass; Linux/amd64 and macOS/arm64 cross-build/vet pass without runtime qualification. Earlier configuration author/reviewer results are historical, separate evidence. No separate-context reviewer or automated review was run.

Evidence: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-load-20260919-212813`, containing actual parent/candidate source, prompt, commands, raw JSON/logs, CLI stop/wait records and identities. Figma and unrelated/historical files are preserved. Missing GPG/PAR2, Windows permission fixtures, opt-in performance skips, race/ACL, Docker, CI, power loss, browser and hardware limitations remain explicit. The prior Windows sharing interleaving is not repaired or disproved by this passing run.

Missing sidecar is still assumed optional on a fresh Store; storage identity and external-writer coordination remain open. Refusal latching is not continuous disk monitoring. Cross-file transactions, generation retention, schema migration/recovery tooling and broader repository review remain separate. Product direction is unchanged: generation-independent buffering/preservation; intended capability-based LTO-1 through LTO-10 and future extensions, with LTO-8 first for physical qualification rather than a generation limit or tested-support claim. Blu-ray and Figma work remain separate.

ONE next action: one substantive review of this exact uncommitted candidate and retained evidence. No staging, commits, pushes, merges, GUI/media work or automatic reviewer execution.


## 2026-09-19 - job restore-member correction awaiting focused recheck

READY_FOR_JOB_LOADING_FOCUSED_RECHECK on `fix/obx-004-job-load-safety`, HEAD/base `da22f1d9895d5350142a6f9b05ac41f0a920e170`. The substantive [NEEDS_CHANGES review](reviews/OBX-004-JOB-LOAD-SAFETY-REVIEW-2026-09-19.md) remains unchanged. Its R1/P1 (restore validates a spelling but publishes a destination) is addressed by the author, not closed by a reviewer. This entry supersedes the earlier pending-substantive-review next action and the unqualified claim about all incoming job members.

The [follow-up](reviews/OBX-004-JOB-LOAD-SAFETY-REVIEW-FOLLOWUP-2026-09-19.md) restricts regular app-backup members to exact state-file names and flat portable keystore filenames; rejects duplicate tar names before map insertion and duplicate manifest entries before publication; retains exact payload/hash binding and job validation. Aliases are rejected even with valid payloads. Canonical current/legacy formats and valid export/restore/migration pass. `keystores/jobs.json` remains distinct. No schema, transactional restore or general filesystem-alias guarantee is introduced.

Current author evidence: exact reviewed uncommitted snapshot reproduces both reported clobbers; final native suite 257 top-level passes / 0 failures / 39 skips, separately 152 passing subtests. Earlier focused correction selection: 71 top-level / 112 subtest passes; retained probes rerun as author checks: 3 top-level / 3 subtest passes. Overlapping results are not summed. Prior author 254/39/122 and substantive reviewer 90/0/122 plus its separate probes remain historical. No new targeted reviewer execution or acceptance has occurred.

Evidence: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-followup-20260919-222503`, including pre-correction bytes, captured red tar/before/after fixtures, commands/logs and final manifest. Missing helpers, optional performance and Windows permission skips remain; race/ACL, Docker/CI, other-platform runtime, power-loss and hardware qualification are not implied. Figma, historical reports, branch/HEAD and empty index remain preserved. Configuration publication stays complete and was not replayed.

ONE next action: targeted recheck of R1 aliases, duplicate/collision ordering, validation before publication, original-store/latch preservation, valid restore/migration and affected safety controls on this exact candidate. No automatic review, staging, commits, push, GUI or media work. Generation-independent architecture and LTO-8 as the first physical qualification target remain unchanged.


## 2026-09-19 - owner-accepted job-loading checkpoint

Owner acceptance recorded 2026-09-19T23:09:31.174870-04:00 (America/New_York). The [substantive review](reviews/OBX-004-JOB-LOAD-SAFETY-REVIEW-2026-09-19.md) and [closing focused recheck](reviews/OBX-004-JOB-LOAD-SAFETY-FOCUSED-RECHECK-2026-09-19.md) establish the accepted bounded scope; restore-member R1 is CLOSED. Implementation commit: `58711f5d13132246fe048cd8f749cf16d67c1fc1`, direct parent `da22f1d9895d5350142a6f9b05ac41f0a920e170`, branch `fix/obx-004-job-load-safety`. This entry supersedes the pending-review next actions above. Historical reports retain their original verdicts, dates and bytes.

Accepted: complete job-state validation before adoption/reconciliation, read/error propagation, serialized refused-load stale-write protection, and rejection of noncanonical restore member names and duplicates before publication. The policy is rejection, not alias normalization or recovery. Valid first-use/history/restore/migration and existing FAILED/current-recording/history semantics remain; completion is not verification. This does not close all OBX-004 or all persistence concerns.

Evidence layers remain separate: initial job author 254 top-level passes / 39 skips, separately 122 passing subtests; substantive reviewer 90 top-level passes / no skips and 122 subtests, with separate probes (2 top passes / 1 failure; 1 subtest pass / 2 failures); correction author 257 top-level passes / 39 skips and 152 subtests; closing focused reviewer 33 top-level passes / no failures or skips and 81 subtests, plus separate probes 6 top-level / 12 subtest passes. The closing review executed build, vet, formatting and whitespace checks as well as 238-file preservation checks. Both job reviewers disclose same-session Codex provenance and a shared repository, with disposable probe copies; no independent agent/context isolation or human certification is claimed. Overlapping counts are not summed.

This publication task performs identity, documentation, staging/tree, commit and remote checks only; no historical test campaign is rerun. No active required Git hook was found; any later hook execution must be separately recorded. Retained evidence preserves the correction's capture-filename collision and subsequent corrected capture, not an invented clean history. Missing GPG/PAR2 and permission/performance skips, integration limits, the pre-existing Windows sharing interleaving, race/ACL/power-loss, Docker/CI, other-platform runtime and hardware limitations remain. Cross-compilation is not target-platform execution. Storage identity, external/concurrent writers, cross-file transactions, retained generations and broader namespace/security remain deferred.

Publication is PENDING until the external `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-publication-20260919-230517\publication-receipt.md` records the actual evidence SHA and verified equality of local HEAD, origin tracking ref and live `refs/heads/fix/obx-004-job-load-safety` at `https://github.com/nathansottung/obelisk.git`. The evidence commit must not name its own SHA or claim an unperformed push. Only that successful receipt establishes NEXT_TASK_BASE_SHA for later authorized work. Configuration publication remains complete and unchanged.

Immediate next action: complete the authorized evidence commit and scoped publication verification; no additional implementation or review. After publication, wait for a separate owner task. The isolated Figma preview remains proposed and unimplemented, not automatically authorized. Preserve generation-independent buffering/preservation; LTO-8 remains the first physical qualification target rather than a generation limit, other backends/generations need explicit qualification, and Blu-ray remains separate. Figma stays unchanged and excluded; no GUI/media work is performed.
