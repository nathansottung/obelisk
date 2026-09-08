# OB issue status at the pinned baseline

**Pinned SHA: `d97809b2f730e09960632e2943e562531fa095a3`**

Revalidated 2026-09-06 on branch `audit/obelisk-pr00-d97809b2`. Source register:
`docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/backlog/issues/`.

Original OB IDs, priorities and evidence classifications are preserved exactly. Nothing is
renumbered. New observations found during this audit receive new `OBX-` IDs at the end.

Every classification below rests on source read at this commit. Line anchors were confirmed by
direct reading, not only by search. Anchors are valid for this SHA only.

## Classifications used

| Value | Meaning |
|---|---|
| `CONFIRMED_AT_BASELINE` | The described defect is present at this SHA, with a traced failure path |
| `PARTIALLY_FIXED` | Part of the original finding has been repaired; a named remainder is still present |
| `ALREADY_FIXED` | The described defect is no longer present at this SHA |
| `NEEDS_INVESTIGATION` | Reachability or impact not established by source reading alone |
| `NOT_YET_REVALIDATED` | Not examined in this session |
| `NOT_APPLICABLE` | The described subject does not exist in this codebase |

---

## Summary

| ID | Priority | Classification | One-line status |
|---|---|---|---|
| OB-001 | P0 | `CONFIRMED_AT_BASELINE` → `PATCH_PENDING_REVIEW` | Read-error fall-through reproduced by execution and repaired on `fix/ob-001-catalog-open`; zero-length sub-claim corrected; initialization-intent residual still open |
| OB-002 | P0 | `CONFIRMED_AT_BASELINE` | `EndBatch` cannot return the final flush error; jobs report COMPLETED regardless |
| OB-003 | P0 | `CONFIRMED_AT_BASELINE` | `atomicRename` deletes the existing good destination after any rename failure |
| OB-004 | P0 | `CONFIRMED_AT_BASELINE` | Source guard is a lexical string prefix test, preflight only |
| OB-005 | P0 | `PARTIALLY_FIXED` | `ScanFolder` now records walk errors; unknown extent, bounding, persistence and `AdoptFolder` remain |
| OB-006 | P0 | `CONFIRMED_AT_BASELINE` | Unreadable keystore is skipped, then overwritten with a union computed without it |
| OB-007 | P1 | `CONFIRMED_AT_BASELINE` | No process lock of any kind; accessors hand out live catalog pointers |
| OB-008 | P1 | `PARTIALLY_FIXED` | Build list, `--` and BagIt encoding fixed; member validation and extraction confinement absent |
| OB-009 | P1 | `CONFIRMED_AT_BASELINE` | PAR2 repairs the archival original in place; extracted outputs are never re-hashed |
| OB-010 | P1 | `CONFIRMED_AT_BASELINE` | App-backup restore writes attacker-named members with no allowlist; live Store swapped mid-flight |
| OB-011 | P1 | `CONFIRMED_AT_BASELINE` (part `NOT_APPLICABLE`) | Recovery-kit secrets written 0644; no support-export feature exists |
| OB-012 | P1 | `CONFIRMED_AT_BASELINE` | Bare channel operations; a writer failure strands the producer forever |
| OB-013 | P1 | `CONFIRMED_AT_BASELINE` | Starvation counter is capped at 1 by its nesting; no wait duration measured |
| OB-014 … OB-036, OPT-001 | various | `NOT_YET_REVALIDATED` | Out of scope for this session — see "Unreviewed areas" |

Three of the thirteen have had real work land since the register was written (OB-005 and OB-008
partially, and the final-flush half of OB-012). That is exactly why revalidation was required
before implementation.

---

## OB-001 — Fail closed when an existing catalog cannot be read

**Priority P0. Original evidence: CODE_OBSERVED. Classification: `CONFIRMED_AT_BASELINE`
(read-error path), `PATCH_PENDING_REVIEW` on branch `fix/ob-001-catalog-open`.**

> **Update 2026-09-06 (PR-01).** The read-error path is now an **executed reproduction**, not
> only source-traced, and one sub-claim below has been **corrected**. See
> [reviews/PR01-OB-001-2026-09-06.md](reviews/PR01-OB-001-2026-09-06.md) for the patch, the
> red/green evidence and the residuals. Summary of the change in status:
>
> - **Read errors other than not-found — confirmed by execution and now repaired.** Against the
>   pre-fix logic, an injected permission-denied read made `OpenStore` return a usable Store and
>   `nil` error, replaced a 3,476-byte catalog holding one collection with a 3,259-byte empty
>   one, and wrote that day's `.bak-` sidecar from the **empty** bytes. Reopening then found 0
>   collections. A pristine-baseline probe using a real directory in place of `catalog.json`
>   reproduced the same fall-through (`store != nil`, `err == nil`, 3 seeded profiles).
> - **Zero-length catalog — the PR-00 claim below was WRONG and is corrected here.** A
>   zero-length `catalog.json` did **not** reach new-catalog initialization at baseline. The
>   `json.Unmarshal` immediately after the `existed = len(b) > 0` line rejects empty input first,
>   so baseline already failed closed with `catalog.json is damaged: unexpected end of JSON
>   input`, left the file at 0 bytes and wrote no backup (verified against pristine `d97809b2`).
>   `existed = len(b) > 0` was a latent hazard, not a live defect. PR-01 still rejects the case
>   explicitly and earlier, with an actionable message naming the `.bak-` sidecar, but it must
>   **not** be credited with fixing a fall-through that did not exist.
> - **Residual, still open under this ID:** `fs.ErrNotExist` does not prove the operator intended
>   a new catalog. A previously initialized data directory whose drive is unmounted also reads as
>   absent, and `os.MkdirAll(dataDir, ...)` at the top of the open path will create the missing
>   directory before the read is even attempted. Distinguishing first use from missing expected
>   storage needs storage identity and is **not** delivered by this patch.

> **Update 2026-09-06 (independent review of PR-01).** The patch was reviewed against the source
> and by execution: the read-error repair, the zero-length correction and the identity residual
> were each independently reproduced. Outcome **NEEDS_CHANGES**, scoped to documentation and test
> completeness — **no production-logic change was requested**, and `store.go` is unchanged by the
> follow-up. See [reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md](reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md)
> for findings F-1 through F-5 and
> [reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md](reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md)
> for the response. Two corrections affect what is written above and elsewhere in this file:
>
> - **The observer's exclusion of `saveJobs` was justified wrongly.** `loadJobs` (`store.go:3370`)
>   does call `saveJobs` (`store.go:3350`) at `store.go:3400-3401` when it reconciles a `RUNNING`
>   job, so a successful startup can write `jobs.json`. The exclusion is correct only by control
>   flow: `s.loadJobs()` at `store.go:1283` sits after every refusal return (`1164`, `1172`,
>   `1183`), so a **rejected** open never reaches it. `loadJobs`/`LoadConfig` remain OBX-004.
> - **The classification above is unchanged.** OB-001 stays `PATCH_PENDING_REVIEW`, and the
>   initialization-identity residual stays **open under this ID**. The independent review
>   reproduced it: opening an absent, unmounted-like data directory creates the tree and writes a
>   fresh 3,259-byte catalog plus a daily backup, with no error.

> **Update 2026-09-06 (checkpoint committed).** The focused recheck
> ([reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md](reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md))
> closed F-1, F-3, F-4 and F-5. F-2's remaining half — the wrongly justified `saveJobs`
> exclusion in the `persistObserver` field comment — was then corrected in `store.go` as a
> **comment-only** edit; the full suite was not rerun after it. The patch is now committed to
> `fix/ob-001-catalog-open` as `20c425d9ca6a4c5c1bbd79d33235a788b8201fa4` (`store.go`,
> `catalog_open_test.go`), on source baseline `d97809b2f730e09960632e2943e562531fa095a3`.
>
> - **The classification is still unchanged.** OB-001 remains `PATCH_PENDING_REVIEW` and the
>   initialization-identity residual remains **open under this ID**. Committing the read-failure
>   repair is not a resolution of the whole issue, and no independent review or
>   production-readiness certification is claimed for the committed state.
> - The last full execution reported **204 pass / 2 fail / 4 skip**. The two Windows TAB-fixture
>   failures remain open under OBX-001; Windows race testing remains **NOT TESTED**.

**Anchors.** `store.go:1109` `OpenStore`; the defect is `store.go:1119`.

**Actual failure path.**

1. `store.go:1119` reads `if b, err := os.ReadFile(s.path); err == nil { ... }`. This is the only
   error handling in the open path. There is no `errors.Is(err, fs.ErrNotExist)` and no
   `os.IsNotExist` anywhere in `store.go`. Any read error that is not "absent" — EACCES, EIO, a
   Windows sharing violation from antivirus or a backup agent holding the handle, a data
   directory on a not-yet-mounted network share — falls through silently. No log line, no error.
2. `existed` stays `false` and the in-memory catalog `s.c` stays zero-valued.
3. Because `existed` is false, the schema gate at `store.go:1132-1148` is skipped entirely — so
   neither the forward-schema read-only latch nor `backupBeforeMigrate` runs.
4. `seedBuiltinProfilesLocked()` at `store.go:1198` returns true against a zero catalog, setting
   `recovered = true`. The template seeding at `store.go:1204` does the same.
5. `store.go:1212` calls `s.save()`, which reaches `writeCatalog` and renames a three-profile
   **empty** catalog over the real `catalog.json` at `store.go:1359`.
6. `dailyBackup` at `store.go:1366` then writes today's `.bak-YYYYMMDD` from those empty bytes
   (`s.lastBak` is empty in a fresh process) and prunes to the newest 14, consuming a slot that
   held a real backup.

**Affected data.** The entire catalog: archives, folders, files, versions, chunks, copies,
volumes, locations, events, plans, templates, key metadata. Everything the tool knows about where
data lives and whether it verified.

**Existing protection.** Partial and incidental. Corrupt JSON *is* handled correctly and fails
closed (`store.go:1122`, fatal at `main.go:71`) — note the asymmetry: a damaged file refuses to
boot, an unreadable file boots empty. The `.bak-YYYYMMDD` rotation gives up to 14 days of
recovery, but step 6 actively erodes it. If the path is unreadable *and* unwritable the write
fails and only logs, so the destructive window is specifically "read fails, write succeeds" —
which is the common Windows shape (a file-level ACL or an open handle, with the directory still
writable).

**Reproduction obtained.** Source-traced, not executed. A safe executable reproduction is
specified in `NEXT_ACTIONS.md` using a fault seam rather than a real chmod, so it is meaningful
on Windows.

**Dependencies.** None. This is the reason it is the recommended first fix.

**Constraint on the fix.** `TestOpenStore_RecoverySaveFailureIsNonFatal`
(`persistence_test.go:47`) pins a deliberate decision: a failed *recovery save* stays non-fatal
and returns a usable Store. Failing closed on a failed *read* must not regress that.

**Smallest complete fix.** In `OpenStore`, bind the read error and branch: proceed with an empty
catalog only on a positively identified not-found; return the error for every other case so
`main.go:71` refuses to boot; and treat a zero-length `catalog.json` as damaged rather than new,
because `existed = len(b) > 0` at `store.go:1120` currently maps a torn write onto the brand-new
path. No change to seeding, no change to the recovery-save decision. Model to copy: `readStore`
at `pipeline.go:339`, the one reader in the codebase that already distinguishes these cases.

**Regression tests needed.** Injected read failure fails startup; **no write or create is
attempted on the catalog path**; original bytes byte-identical afterward; genuinely-absent,
zero-length, invalid-JSON and forward-schema cases each produce a distinct outcome.

**Related, same shape, not part of the first fix.** `loadJobs` (`store.go:3293`) drops both read
and unmarshal errors, silently yielding an empty job board and resetting `next` to 0 so IDs
collide. `LoadConfig` (`pipeline.go:146`) drops both as well, and `SaveConfig`
(`pipeline.go:158`) begins by calling it — so **any settings write after a corrupt config read
persists the defaults over the user's real settings**, including `AuthToken`, `KeystorePaths` and
`StagingDir`. Tracked as OBX-004.

---

## OB-002 — Make commit acknowledgments truthful across batches and job persistence

**Priority P0. Original evidence: CODE_OBSERVED + HISTORICAL_REPORT. Classification:
`CONFIRMED_AT_BASELINE`.**

**Anchors.** `store.go:1293` `save`; `store.go:1307` `writeCatalog`; `store.go:1391` `EndBatch`;
`store.go:3268` `saveJobs`; `main.go:522` job completion.

**Actual failure path.**

- `store.go:1300` returns `nil` for every coalesced mutation while a batch is open and a write
  happened within `batchInterval`. Callers see success for in-memory-only work.
- `store.go:1391` `EndBatch()` **has no return value**, and discards the final flush at
  `store.go:1398` with `_ = s.writeCatalog()`. It is structurally impossible for a caller to
  observe it. The doc comment at `store.go:1389-1390` claims "the job's result is durable when it
  returns" — a claim the signature cannot support.
- All nine call sites are `defer a.Store.EndBatch()`: `pipeline.go:520`, `mirror.go:144`,
  `adopt.go:237`, `adopt.go:400`, `exports.go:344`, `exports.go:512`, `dock.go:163`,
  `incremental.go:308`, `plans.go:709`. `defer` runs after return values are set, so even a
  returning `EndBatch` could not be folded in without a named-return change at each site.
- `main.go:522` sets COMPLETED with no persistence check anywhere in the chain. `SetJob` calls
  `saveJobs`, which returns nothing (`store.go:3268`), has no `Sync`, and silently drops marshal,
  write and rename failures.

**Affected data.** The entire catalog delta of any batched job — a full scan, adopt, mirror,
dock, export or plan execution — while the UI shows a green completed job with artifacts.

**Existing protection.** `writeCatalog` itself is well built: temp file, `f.Sync()`, checked
`Close()`, rename, daily backup, and a `failSave` fault-injection seam. Two known gaps remain
inside it: the parent-directory sync error is suppressed (`_ = syncDir(...)`, `store.go:1364`)
and `syncDir` is a documented no-op on Windows. Also `s.c.SchemaVersion` is stamped at
`store.go:1321` before the write is known to succeed. There is genuine prior work here —
`TestWriteCatalog_SaveFailurePropagates` (`persistence_test.go:17`) and the two durability-gate
tests — but it covers the unbatched path, not the batched one.

**Dependencies.** OB-001.

**Smallest complete fix.** Give `EndBatch` an `error` return and make the nine call sites named
returns that fold it in; propagate the terminal-persistence result into the job outcome so
COMPLETED requires a proven committed generation. Broader receipt or checkpoint protocol work
belongs in a later PR.

**Regression tests needed.** Fail the final batch flush and assert the job does not report
COMPLETED; fail jobs-sidecar persistence independently; run two batched jobs concurrently and
assert neither inherits the other's flush assumption (the shared `batchDepth` counter means the
first `EndBatch` to fire decrements to 1 and skips the flush entirely).

---

## OB-003 — Remove the destructive fallback from atomic replacement

**Priority P0. Original evidence: PATTERN_REPRODUCED + CODE_OBSERVED. Classification:
`CONFIRMED_AT_BASELINE`.**

> **Update 2026-09-07 (PR-02 implemented, awaiting independent review).** The defect and all five
> caller anchors below were re-verified at the current tree and are **still exact**. The
> destructive fallback is removed on branch `fix/ob-003-safe-replacement` (uncommitted), with
> regressions in `atomic_replace_test.go`. Report:
> [reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md](reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md).
>
> - **The "smallest complete fix" text below is partly superseded.** It says "on Windows use a
>   replacement primitive that overwrites in one operation". No such primitive was needed:
>   `os.Rename` **already replaces an existing file** on this Windows host (Go issues `MoveFileEx`
>   with `MOVEFILE_REPLACE_EXISTING`), established by execution, not by reading. The old comment
>   claiming otherwise was the reasoning that produced the defect and is corrected in the patch.
> - Cross-device is handled by **failing with the underlying error and never copying**, rather
>   than by explicit detection: no current caller can produce it, since every one stages its
>   temporary in the destination directory.
> - **Classification stays `CONFIRMED_AT_BASELINE`** until an independent review lands. The
>   guarantee proven is preservation of the destination across a failed publication — **not**
>   crash durability, network-filesystem safety, concurrency safety or ACL preservation, and the
>   injected failure causes prove handling rather than OS occurrence. The missing directory sync
>   noted below is **unchanged and still open**.
> - Full suite after the patch: **212 pass / 2 fail / 4 skip** (+8 = the new tests). The two
>   failures are the same OBX-001 Windows TAB-fixture pair, unrelated and untouched. Windows race
>   testing remains **NOT TESTED**.

> **Update 2026-09-07 (review findings F-1/F-2 applied).** The same-session adversarial review
> ([reviews/PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md](reviews/PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md))
> accepted the bounded replacement correction and asked for two things, both now done
> ([reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md](reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md)):
> the restore regression now asserts the staging file was actually created and is then gone
> (positive not-found), and the `appbackup.go` cleanup comment no longer reads as a guarantee.
> That comment was the **only** production change — `mirror.go` is untouched. Cleanup can still
> fail with its error discarded (an OBX-004-family instance at `appbackup.go:401`, recorded, not
> fixed), removal is not secure erasure, and the fixture covers the **catalog** staging file only,
> not keystores. **Classification stays `CONFIRMED_AT_BASELINE`**: neither review to date is
> independent of the authoring session.

> **Update 2026-09-07 (external review accepted; PR-02 checkpointed on its fix branch).** The
> bounded replacement repair was reviewed by a reviewer outside the authoring session and
> **accepted by the owner for publication**, and the branch `fix/ob-003-safe-replacement` is now
> committed and pushed. Report:
> [reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md](reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md).
> **F-1 and F-2 are closed.** Publication is a checkpoint of the reviewed artifact, not a merge,
> a release, or a claim of production readiness.
>
> What that external review is, exactly: a **separate AI source review** plus **isolated Linux
> probes** of the extracted production code — not a human audit or certification. Its evidence
> boundary carries forward unchanged:
>
> - Full repository **build, vet and tests were dependency-blocked** in that sandbox (no network
>   for `go mod download`); no dependency was stubbed to manufacture a pass.
> - The isolated Linux **race** run succeeded, but that is a narrow harness result — **not** a
>   full-product race result and **not Windows race evidence**. Windows race testing remains
>   **NOT TESTED**.
> - **212 pass / 2 fail / 4 skip** remains *reported* execution evidence from the local Windows
>   authoring/review sessions. It was **not re-executed during publication**, and no new test run
>   or CI pass is implied by this checkpoint.
> - The 2 failures are the same OBX-001 Windows TAB-fixture pair, unrelated and untouched.
>
> **Still open, unchanged by PR-02:** the Windows TAB fixture failures; cleanup-error
> observability (including `appbackup.go:401`); keystore/key file permissions; staging-path
> aliasing and ownership against a racing or hostile process; crash durability (no directory
> fsync here, unlike `writeCatalog`); and PR-01's initialization-identity residual.
> **Classification stays `CONFIRMED_AT_BASELINE`.**

**Anchor.** `mirror.go:318-324`, quoted in full:

```go
func atomicRename(tmp, final string) error {
	if err := os.Rename(tmp, final); err == nil {
		return nil
	}
	_ = os.Remove(final)
	return os.Rename(tmp, final)
}
```

**Actual failure path.** The comment describes the Windows target-exists case, but the code
deletes on **any** first-rename failure: cross-device `EXDEV`, permission denial, a destination
on a failing or read-only mount, a sharing violation from another handle, no space for the
directory entry. If the retry then also fails, the old bytes are gone and the new bytes were
never published. Every caller's error handler then runs `_ = os.Remove(tmp)`, so on the
double-failure path **both** the previous good file and its replacement are destroyed.
`atomicRename` also performs no directory sync, unlike `writeCatalog`.

**Affected data.** Five production callers:

| Anchor | What it publishes |
|---|---|
| `mirror.go:252` | Each mirrored file on the destination volume |
| `mirror.go:308` | `copyVerifyToDest` — the shared landing path for full mirrors and incremental runs |
| `plans.go:764` | Each file published by plan execution |
| `appbackup.go:220` | The app-backup `.tar` bundle |
| `appbackup.go:396` | **`catalog.json`, `jobs.json`, `formats.json` and `keystores/*` during app-state restore** |

The last one compounds OB-001: the helper that can delete a good file without replacing it is
also what publishes the catalog during recovery. Note too that auto-export reuses stable
per-period names (`appbackup.go:469-479`), so a failed rename there deletes the previous period's
already-good bundle.

**Existing protection.** The callers do copy-then-verify before renaming, so a corrupt copy is
caught. That protects against bad *content*; it does nothing about a failed *publication*.

**Dependencies.** None recorded.

**Smallest complete fix.** Delete the `os.Remove(final)` fallback. On Windows use a replacement
primitive that overwrites in one operation; elsewhere plain `os.Rename` already replaces. Handle
cross-device explicitly rather than by retry. Never remove an existing destination as a generic
response to a rename failure.

**Regression tests needed.** Missing temporary source with an existing final: replacement fails
and the existing final survives byte-identical. Plus permission, sharing and cross-device
injection with OS-specific evidence — the handoff is explicit that OS-specific behavior needs
OS-specific evidence, and the local suite currently cannot run POSIX permission tests at all.

---

## OB-004 — Enforce source and destination boundaries on actual filesystem objects

**Priority P0. Original evidence: PATTERN_REPRODUCED + CODE_OBSERVED. Classification:
`CONFIRMED_AT_BASELINE`.**

**Anchors.** `store.go:1724-1753` `AssertOutsideSources`; `store.go:26-39` `normPath`;
`store.go:1712` `SourceRoots`.

**Actual failure path.** The decision is `np == rp || strings.HasPrefix(np, rp+"/")` at
`store.go:1748`, over `normPath`-cleaned strings. `filepath.Abs` is purely lexical — no `Stat`,
no `Lstat`, no `EvalSymlinks`. Repo-wide there is **no** `os.SameFile`, **no** `os.Lstat`, and
**no** `os.Root`/`os.OpenRoot`; `filepath.EvalSymlinks` appears only in `deviceid_unix.go:28` and
`smart_unix.go:24`, both resolving device nodes. So a destination reaching a source through a
symlink, junction or bind mount passes the guard. Case folding is applied only when
`runtime.GOOS == "windows"` (`store.go:35`), leaving case-insensitive macOS volumes unprotected.
Windows 8.3 short names, UNC-versus-drive-letter and `\\?\` prefixes are not normalized.

All 20 call sites are preflight at function entry. The per-file write loops never re-check:
`mirror.go:210-256`, `incremental.go:341-364`, `plans.go:740-775`, `writer.go:359` and
`writer.go:390`. `destPath` is rebuilt inside those loops from catalog `RelPath` and never
re-validated against `destDir`.

**Affected data.** Registered source originals — the data the tool exists to protect.

**Existing protection.** The guard does catch the plain case, is applied at 20 entry points
including config validation, and `SourceRoots` returns a fresh copy so the invariant input is not
racy. `quarantine.go:140` is the one per-path caller, though it gates a move source rather than a
write destination.

**Dependencies.** None recorded. **Constraint:** `os.Root` requires Go 1.24 and `go.mod` declares
`go 1.22.2`, so adopting it is a deliberate toolchain decision, not a drop-in.

**Smallest complete fix.** Resolve both sides to filesystem identity before comparing, and
re-validate at each leaf mutation rather than only at entry.

**Regression tests needed.** Source alias via symlink and via Windows junction; nested output
escape; existing destination link; ancestor rename between preflight and write; disappearing
mount. Assert the registered source is unchanged in every case.

---

## OB-005 — Record scan completeness, failures and unknowable subtrees

**Priority P0. Original evidence: CODE_OBSERVED + HISTORICAL_REPORT. Classification:
`PARTIALLY_FIXED`.**

**What is already fixed.** The original finding says traversal failures are discarded. **That is
no longer true for `ScanFolder`.** `pipeline.go:535-539` records a `Kind:"walk"` `ScanProblem`
and keeps scanning; `parallelHash` at `pipeline.go:589-643` classifies stat, hash and
non-regular-file outcomes. `scan_problems_test.go:18` covers the stat and hash kinds and proves
problem files are excluded from the success count. `CardCheck` has the fuller treatment, with
exact `Skipped` and `WalkErrors` counts and a real unreadable-directory test at
`cardcheck_test.go:162`.

**What remains open.**

1. **No unknown-extent representation exists.** An unreadable directory hides an unknown number
   of descendants, and nothing in `package main` records that. Searching for
   `UnknownSubtrees`, `ScopeComplete`, `MandatoryProblems` or `CompletionFacts` returns matches
   only in the handoff's `examples/go-contracts/completion.go`, never in the product.
   `ScanFolder` returns `(int, []ScanProblem, error)` with no result struct and no byte counter.
2. **The problems slice is unbounded** (`pipeline.go:527-532`), against `maxCardProblems = 100`
   in `cardcheck.go:33`.
3. **Problems are never persisted to the catalog.** They reach only the job result map at
   `main.go:950-966` and therefore `jobs.json`. Nothing downstream — protection, completion,
   coverage — consumes them.
4. **Classification uses symlink-following `os.Stat`.** There is no `Lstat`, no `ModeSymlink` and
   no `os.Readlink` anywhere in the repository, so a symlink to a regular file outside the scan
   root is hashed and cataloged as an in-tree file at the link's relative path.
5. **`AdoptFolder` was not included in the fix.** `adopt.go:405-445` still returns `nil` on walk
   errors and calls `parallelHash` **without** the reporter, surfacing only the arithmetic
   `unreadable := len(paths) - len(hashed)` with no paths, kinds or reasons. The same
   drop-the-walk-error shape also lives at `dock.go:321`, `inference.go:91`, `plans.go:617`,
   `drift.go:151`, `quarantine.go:302` and `quarantine.go:450`.

**Coverage gap.** `scan_problems_test.go` does not cover `Kind:"walk"`, `Kind:"unsupported"`, any
bound, persistence, symlinks, `AdoptFolder` or `scanAdoptCandidates`. And the one end-to-end
unreadable-directory test that exists skips on Windows (`cardcheck_test.go:147`).

**Dependencies.** OB-002.

---

## OB-006 — Make keystore reconciliation fail closed and conflict-aware

**Priority P0. Original evidence: CODE_OBSERVED. Classification: `CONFIRMED_AT_BASELINE`.**

**Anchors.** `pipeline.go:469-496` `SyncKeystores`; `pipeline.go:357` `writeStore`;
`pipeline.go:370` `KeystoreStatus`; `pipeline.go:419` `GenerateKey`; `pipeline.go:451`
`Passphrase`.

**Actual failure path.** `pipeline.go:473` reads `if ks, err := readStore(p); err == nil` and so
skips every failed participant — permission denial, transient I/O error, unmounted drive, corrupt
JSON, and a forward schema version. The second loop at `pipeline.go:490` then iterates **the same
list unconditionally** and writes the merged union to every path, **including the one that just
failed to read**. Keys held only by that store are destroyed. `writeStore` is temp-plus-rename,
so the overwrite is atomic and total, and `SyncKeystores` returns `(count, nil)` with no
indication that any path was skipped.

Conflicts are silent last-wins: `merged[r] = k` at `pipeline.go:476` keys on `key_ref` only, with
no comparison of the passphrase. Two records sharing a reference with different secrets resolve
by config order. `KeystoreStatus` compares key-reference set membership only
(`pipeline.go:394-415`), so that pair reports `consistent == true` and `ok == true`. Worse, the
precedences disagree: `SyncKeystores` is last-wins while `Passphrase` (`pipeline.go:451`) returns
the **first** match found.

**Affected data.** Key material. Losing it makes otherwise healthy encrypted media permanently
unrecoverable — the largest blast radius of any finding in this register.

**Existing protection.** `readStore` (`pipeline.go:339`) is well written and does distinguish
not-found, unreadable, not-a-keystore and forward-schema. `writeStore` creates at `0o600`. The
problem is entirely in how `SyncKeystores` uses them. Unlike the catalog, which keeps both
`.pre-schema-vN` and `.bak-YYYYMMDD` generations, `writeStore` keeps **no** prior generation.

**Also noted.** `GenerateKey` (`pipeline.go:419-449`) is fail-fast, not atomic: if the second
keystore write fails, the first already holds the new key, `AddKeyMeta` is never reached, and the
caller gets `("", "", "", err)` so the passphrase is lost from memory. And because `readStore`
returns a fresh empty store for not-found, a keystore whose drive is unmounted is silently
recreated as a one-key file at the mountpoint.

**Dependencies.** OB-002, OB-003.

**Smallest complete fix.** Block the write phase entirely when any configured participant could
not be validated; detect same-reference/different-secret as an explicit conflict rather than
resolving it; retain a prior generation before overwrite.

**Regression tests needed.** Malformed, unreadable and newer-schema stores remain byte-identical
after a rejected sync. Same-reference/different-secret produces a conflict, never an
order-dependent result. Partial publication failure is visible and recoverable. No test prints a
secret value.

---

## OB-007 — Establish real writer ownership and immutable read boundaries

**Priority P1. Original evidence: CODE_OBSERVED + HISTORICAL_REPORT. Classification:
`CONFIRMED_AT_BASELINE`.**

**No process lock exists.** Searches for `syscall.Flock`, `LockFileEx`, `O_EXCL`, pidfile
patterns and `Getpid` return nothing in non-test Go source. `OpenStore` (`store.go:1109`) goes
straight from `os.MkdirAll` to reading the catalog. Two processes on one `-data` directory both
open it, hold divergent in-memory copies, and both rename over `catalog.json` — last writer wins
and the other session is erased. Both also use the same `<path>.tmp` (`store.go:1343`), so
concurrent writes can interleave and publish a spliced file, which then makes `OpenStore` refuse
to boot at `store.go:1122`.

The default `-listen 127.0.0.1:7821` gives accidental partial protection, but `main.go` binds
*after* `OpenStore` and after the seeding save, so a second instance has already written before
it discovers the port is taken — and a second instance on a different port with the same `-data`
has no protection at all. `appbackup.go:438` and `migrate.go:114` open additional `*Store`
instances inside the running process.

Sub-clause note: there is no `--force` flag in the codebase (`main.go:57-59` defines only
`-listen`, `-port`, `-data`; the `force` token at `main.go:2503` is an unrelated finalize
override). So "do not let `--force` bypass a live owner" is a design constraint for the lock that
gets built, not a present defect.

**Live mutable state escapes.** Roughly 14 exported accessors return live pointers into `s.c` —
`Collection` (`store.go:1479`), `Chunk` (2153), `DriftReport` (2207), `Volume` (2313), `Event`
(2587), `DockSession` (2857), `Profile` (3066), `FileByID` (1941) among them. The design depends
on it: `UpdateVolume` (`store.go:2357`), `UpdateEvent` (2599), `UpdateDockSession` (2888),
`UpdateBurnQueue` (`burner.go:79`) and `UpdatePlan` (`plans.go:66`) **ignore their argument
entirely** and only trigger a save — the caller has already mutated the live object outside the
lock. That mutation races `json.Marshal(&s.c)` inside `writeCatalog`, which walks the same
objects under `s.mu`. For a map-valued field such as `Plan.Satisfied` (assigned at
`plans.go:713`) the failure mode is a hard "concurrent map iteration and map write" panic, not
merely torn data.

A further ~18 accessors return shallow slice copies — a fresh backing array whose elements are
still live pointers (`Collections` 1473, `Volumes` 2307, `AllFiles` 1933 and others).
`AllFiles` documents the convention at `store.go:1931` ("Caller treats elements as read-only")
with no enforcement. `Jobs()` (`store.go:3434`, commit `93c166e`) copies each row struct, which
is real progress, but `cp := *j` copies headers only, so `Job.Result` and `Job.Artifacts` stay
shared.

**Concurrency shape.** `runJob` (`main.go:466`) is the only job launcher and serves all 25 job
endpoints. It imposes no concurrency limit and no per-resource serialization. Two concurrent
batched jobs share one `batchDepth`, which is the OB-002 interaction noted above.

**Dependencies.** OB-002.

**Evidence caveat.** The race detector could not run on this platform (see `BASELINE`, section
3.5). It would not settle this in any case: the `Update*`-with-unused-argument pattern is an
ownership defect visible in the source regardless of scheduling.

---

## OB-008 — Make archive namespaces unambiguous and untrusted

**Priority P1. Original evidence: CODE_OBSERVED + HISTORICAL_REPORT. Classification:
`PARTIALLY_FIXED`.**

**What is already fixed.** The build member list is NUL-delimited and passed `--null -T`
(`pipeline.go:971-983`, `pipeline.go:1037`, commit `2cf3181`). Both restore branches place `--`
before caller-supplied member names (`writer.go:693`, `writer.go:719`). BagIt manifest paths are
percent-encoded per RFC 8493 (`bagit.go:47-64`, commit `347389f`). `TestBuildRestore_
HostileFilenamesRoundTrip` and `TestBuildFilelist_IsNulDelimited` cover the round trip.

**Caveat on that coverage.** Both of those tests **fail on Windows at this commit** for an
unrelated fixture defect (OBX-001), and CI is Linux-only. The fix is real; its Windows evidence
is currently missing.

**What remains open.**

1. **`members` is unvalidated.** It arrives as raw request JSON (`main.go:1987-1996`) and is
   never checked against `c.Files`, never normalized, never rejected for `..` or absolute paths
   before being handed to external `tar`.
2. **No extraction confinement exists in the product.** The only guard in `RestoreChunk` is the
   destination check `AssertOutsideSources(outputDir)` at `writer.go:630`. There is no inspection
   of archive entry names or link targets, and no `filepath.IsLocal` or `..` guard anywhere in
   the codebase. Confinement rests entirely on the external tar's defaults — and the tar on this
   machine is bsdtar, not GNU tar, while `cfg.Tools["tar"]` lets an operator point at any binary.
   The handoff warns against exactly this: "Do not reduce this issue to inserting `--` while
   leaving extraction links unconstrained."
3. **`parseTarTOC` parses untrusted human-readable `tar -tvf` text** (`adopt.go:134-166`). A
   member name containing a newline splits into two entries; one containing `" -> "` is truncated
   at that substring; a name shaped like a listing line can inject a fabricated entry. These refs
   are written into the catalog as `Chunk.Files` at `adopt.go:332`. Tracked as OBX-005.

**Coverage gap.** `tar_names_test.go` does not cover any member name containing `..`, any
absolute member name, any symlink or hardlink member, or `parseTarTOC` against a newline-bearing
name.

**Dependencies.** OB-004, OB-005.

---

## OB-009 — Restore through private repair/staging and verify the recovered files

**Priority P1. Original evidence: CODE_OBSERVED. Classification: `CONFIRMED_AT_BASELINE`.**

**Anchor.** `writer.go:672-685`, and the extraction tail at `writer.go:687-741`.

**Actual failure path.** `enc` is resolved from the medium (`writer.go:638`,
`findPayload(sourceDir, c)` defaulting to `c.WrittenDest`), so `par2f := enc + ".par2"` also sits
on the medium. `run(par2Bin, "", "repair", par2f)` at `writer.go:678` therefore **rewrites the
payload on the archival medium during what the operator invoked as a restore**. Nothing between
`writer.go:621` and `writer.go:678` copies anything to scratch. `par2 repair` may also leave
`<payload>.1` backup files in the same directory.

Two further defects in the same block:

- The `else if` at `writer.go:683` means that when a `.par2` file **is** present, the ciphertext
  is never compared against `c.EncHash` at all. A successful `par2 verify` is accepted as
  sufficient, and after a repair the repaired bytes are never checked against the catalog digest.
  A self-consistent but wrong par2 set is therefore accepted.
- No requested output is re-hashed after extraction. Both branches return
  `restoreResult(...)` immediately after `tar` exits. Nothing confirms the requested members
  actually appeared in `outputDir`.

Every `ChunkFileRef` already carries the expected digest, and the BagIt payload manifest ships as
tar member one specifically so members can be verified on extract — `RestoreChunk` reads neither.

**Existing protection.** The write path does this correctly: `writer.go:397-400` reads the
payload back and compares to `c.EncHash`. And `versions.go:352-369` verifies single-file version
restore against the recorded hash — but softly: `if h, herr := hashFileHex(restored); herr == nil`
means an unopenable restored file skips the check entirely and the function returns success with
`hash_verified: false` rather than an error.

**Dependencies.** OB-002, OB-003, OB-004, OB-008.

---

## OB-010 — Restore application state as a coherent generation

**Priority P1. Original evidence: CODE_OBSERVED + HISTORICAL_REPORT. Classification:
`CONFIRMED_AT_BASELINE`.**

**Anchors.** `appbackup.go:265` `readTarMembers`; `appbackup.go:294` `verifyAppBackup`;
`appbackup.go:363` `RestoreAppBackup`.

**Actual failure path.**

1. **No member allowlist.** The `default:` arm at `appbackup.go:410-413` writes
   `filepath.Join(a.DataDir, m.Name)` with a manifest-supplied name. `filepath.Join` cleans, so
   `../../...` resolves outside the data directory, and `writeFile`'s `os.MkdirAll`
   (`appbackup.go:389`) creates the intervening directories to get there. The comment on that
   line names three files; the code enforces nothing. Only the `keystores/` arm at
   `appbackup.go:405` is safe, and only because it applies `filepath.Base`.
2. **Verification is self-referential.** `verifyAppBackup` checks each member against
   `man.Members[i].SHA256` — but `man` comes out of the same bundle. The one external check, the
   `.sha256` sidecar, is skipped entirely when absent (`if sc, err := os.ReadFile(...); err == nil`),
   so deleting the sidecar downgrades verification to self-consistency. `m.Name` is never
   validated.
3. **No resource limits.** `readTarMembers` calls `io.ReadAll` per member into a
   `map[string][]byte`, holding the entire bundle in memory with no size or count cap, before any
   validation runs. Duplicate member names resolve silently last-wins — note `verifyTarContents`
   at `pipeline.go:866` **does** reject duplicates for packages, so the asymmetry is an oversight.
4. **Per-file replacement, not a generation cutover.** The loop at `appbackup.go:399-415` writes
   one file at a time through `atomicRename` (which is OB-003). A failure on the third member
   returns with the first two already swapped in and `config.json` never written. Nothing stages
   to `<data>/restore-<stamp>/` and renames a directory. The `pre-restore-<stamp>` copies at
   `appbackup.go:373-384` are best-effort with discarded errors, are not fsynced, and **do not
   include `keystores/`** — so a clobbered keystore has no pre-restore copy.
5. **Live Store swapped mid-flight.** `a.Store = ns` at `appbackup.go:442` is a bare pointer
   assignment on a shared `*App` with no mutex and no request quiescence. Every HTTP handler
   dereferences `app.Store` concurrently. A job still running against the orphaned old Store
   keeps writing to the same path, so it can overwrite the just-restored `catalog.json` after the
   restore reports success.
6. The restored `config.json` is trusted wholesale (`appbackup.go:419-434`) — `KeystorePaths`,
   `StagingDir` and `Tools` (which resolve the `tar`/`gpg`/`par2` binaries) — with no
   `AssertOutsideSources` re-validation, unlike the `PUT /api/config` path.

**Existing protection.** Non-regular tar typeflags are skipped (`appbackup.go:282`), member
hashes and sizes are checked against the manifest, the format marker and schema gate are
enforced, and a pre-restore copy of four files is attempted.

**Dependencies.** OB-001, OB-002, OB-003, OB-004, OB-007, OB-008.

---

## OB-011 — Audit secret-bearing output permissions and redaction

**Priority P1. Original evidence: HISTORICAL_REPORT + PARTIAL_CODE_OBSERVATION. Classification:
`CONFIRMED_AT_BASELINE` for permissions; `NOT_APPLICABLE` for support-export redaction.**

**Permissions — confirmed.** Exactly one artifact in the tree is created `0o600`: the live
keystore, at `pipeline.go:363`. There is **no non-test `os.Chmod` anywhere in the repository**
(the only two hits are in `cardcheck_test.go`), so no pre-existing file ever has its permissions
corrected.

Secret-bearing artifacts created world-readable at `0o644`:

| Artifact | Anchor | Secret content |
|---|---|---|
| `<kit>/keys/K-*.png` | `recoverykit.go:152` | QR payload is `MNEMO1\|<key_ref>\|<passphrase>` |
| `<kit>/keys/K-*.sheet.txt` | `recoverykit.go:156` | The typable passphrase |
| `<kit>/keys/KEYS.html` | `recoverykit.go:168` | QR plus typable grid, every key |
| `config.json` | `pipeline.go:178` | `AuthToken` |
| Restored `keystores/*` | `appbackup.go:393` | **Downgrades the 0600 that `writeStore` created** |
| App-backup tar with `include_keys` | `appbackup.go:187`, `:231` | Whole keystores; tar member mode fixed at `0o644` (`appbackup.go:195`) |

The kit's own banner text at `recoverykit.go:28` says "Anyone holding this folder can decrypt
every encrypted package." The permission bits do not match that prose. `keysDir` itself is
created `0o755`.

**Redaction — not applicable.** There is **no diagnostics or support-export feature** in this
codebase. Every `diagnostic` hit is tape-drive hardware diagnostics; there are zero `redact`
hits. The nearest analogue is the app-backup bundle, which does correctly scrub `AuthToken` on
export (`appbackup.go:110-117`, covered by `appbackup_test.go:141`). Not redacted from that
bundle: absolute source roots, staging paths, keystore paths, tool paths, volume serials, and —
with `include_keys` — the keystores in full plaintext. Separately, `GET /api/config`
(`main.go:649`) returns the whole `Config` including `AuthToken`.

**Windows scope.** The handoff asks for an explicit Windows ACL scope. Mode bits are largely
advisory on NTFS, so a Windows fix means ACL work, and the tests must assert intended ACL
handling or record an explicit unsupported status. That scoping decision is open.

**Dependencies.** OB-006, OB-010.

---

## OB-012 — Make buffered copying cancellation-safe and durably completed

**Priority P1. Original evidence: CODE_OBSERVED. Classification: `CONFIRMED_AT_BASELINE`, with
the final-flush half `ALREADY_FIXED`.**

**Already fixed.** Commit `7e380fe` landed `finalizeWrite` (`writer.go:196-215`), called before
the hash is produced (`writer.go:176`), so a failed `Sync` or `Close` fails the write and returns
no hash. Sidecars route through it too (`writer.go:779`). Covered by
`writer_flush_test.go:27`, `:62`, `:88`. The short-read half is covered by
`ringcopy_shortread_test.go`.

**Still confirmed.** `writer.go:100` is a bare `ch <- b[:n]` and `writer.go:129` a bare
`for b := range ch`. `ringCopy` takes no `context.Context` and has no done channel; the only
`select` in the file (`writer.go:166`) is a non-blocking `errCh` poll that runs *after* the loop
has fully drained.

When `out.Write` fails at `writer.go:143` the function returns and nothing ever drains `ch`. The
producer fills the remaining buffer slots and then **blocks permanently** on the send — a
goroutine leak holding `depth × block` bytes, on the order of a gigabyte at the default
`bufferGB`, never reclaimed. The same return fires `defer in.Close()` (`writer.go:58`) while that
goroutine may still be inside `io.ReadFull` (`writer.go:97`). Because the producer is stuck on
the send, it cannot even report the resulting error through `errCh`.

Two related points. A producer *read* error is only observed at `writer.go:166`, after the
consumer has already written every queued block to the medium — there is no early abort. And
`ringCopy` writes straight to the final payload path (`os.Create(dst)`, `writer.go:70`), with no
temp-then-publish, so a mid-stream failure leaves a truncated file under the real payload name;
`mediumFail` (`writer.go:344`) does not remove it.

**Dependencies.** OB-002, OB-003, OB-007.

**Regression tests needed.** Inject a destination failure after N bytes and assert with a bounded
deadline that the producer goroutine exits and memory is released. Cancel during prefill, queue
wait, read, write and pacing.

---

## OB-013 — Replace misleading buffer telemetry with measured quantities

**Priority P1. Original evidence: PATTERN_REPRODUCED + CODE_OBSERVED. Classification:
`CONFIRMED_AT_BASELINE`.**

**Anchor.** `writer.go:135-141`:

```go
if written > 0 && atomic.LoadInt32(&readerDone) == 0 {
    if fill < stats.MinFill {
        stats.MinFill = fill
        if fill == 0 {
            stats.StarvedEvents++
        }
    }
}
```

`StarvedEvents++` sits inside `if fill < stats.MinFill`. Once `MinFill` reaches 0, that condition
can never be true again because `fill` cannot go negative, so **the counter is capped at 1 for
the entire copy** regardless of how many times the buffer drains and refills. It counts "the
first time the ring hit a new low that happened to be zero", not distinct wait intervals.

Further measurement defects in the same block:

- **No wait duration is measured anywhere.** `fill := len(ch)` at `writer.go:130` is sampled
  *after* the receive has already succeeded, so time spent blocked is never observed. A writer
  that waited 30 seconds and one that never waited both record `fill == 0` once.
- **Prefill is excluded rather than recorded separately** — the `written > 0` term drops the
  first sample and `readerDone == 0` drops the entire tail.
- **`MinFill` initialises optimistically** to `depth` (`writer.go:52`), so a copy too short to
  sample reports a perfectly fed buffer with zero starves.
- `RingStats` (`writer.go:20-29`) has no wait-duration field, no prefill field, and no
  provenance distinction between producer-read rate, host-accepted write rate and
  device-reported rate. `WriteMBps` is computed over `written`, which accumulates the *intended*
  length (`writer.go:146`), not the accepted length.

**Surfacing.** Persisted at `store.go:225`, set at `writer.go:368` and `span.go:195`, rendered at
`ui/index.html:510` as `N starve(s)` with no provenance and no unavailable state. A write that
starved fifty times displays as `1 starve`; one that was never sampled displays as `0 starves`.
Note `perf.go:57` uses `-1` for "not a ring destination", so the codebase already has the
unavailable-versus-zero idea in one place — `RingStats` just lacks it.

**One clause already satisfied.** The acceptance criterion "no fabricated zero-backhitch score is
emitted" holds: `backhitch` appears nowhere in compiled Go or in `ui/index.html`, only in the
handoff documents. There is no LTFS index-sync counter and no device head-speed metric. The
remaining exposure is the honesty of the counters that do exist.

**Dependencies.** OB-012.

---

## New observations from this audit

New IDs, not renumbered OB issues. These were found while establishing the baseline.

| ID | Severity | Finding |
|---|---|---|
| OBX-001 | High (process) | `tar_names_test.go:39` places `"tab\there.txt"` in the unconditional fixture list while the `runtime.GOOS != "windows"` guard at line 41 covers only newline names and the long path. TAB is illegal in Win32 filenames, so `TestBuildRestore_HostileFilenamesRoundTrip` and `TestBuildFilelist_IsNulDelimited` **fail on Windows**, aborting at `tar_names_test.go:68`. These are the regression tests for the OB-008 tar work, so that fix has no Windows evidence. CI is `ubuntu-latest` only, so this is invisible there. Fix: move the tab name behind the existing POSIX guard, and add a Windows CI job. |
| OBX-002 | Low | The pictograph gate in `docs/CONTRIBUTING.md` fails on three tracked lines: `ui/index.html:3214` uses U+2715 where only ✗ U+2717 is permitted; `bagit.go:48` uses U+00A7; `pipeline.go:502` uses U+2208. The check is not enforced in CI. |
| OBX-003 | Low | The same script crashes with `UnicodeEncodeError` on a Windows cp1252 console before finishing, and reports 144 false positives from the untracked handoff bundle while it is extracted in-tree. It needs UTF-8 output and a scope exclusion to be runnable as documented. |
| OBX-004 | High | `LoadConfig` (`pipeline.go:146`) discards both read and unmarshal errors and returns `defaultConfig()`. `SaveConfig` (`pipeline.go:158`) starts by calling it, so **any settings write following a corrupt or unreadable config persists the defaults over the user's real settings** — including `AuthToken` (the non-localhost bind gate), `KeystorePaths`, `StagingDir` and `Par2Redundancy`. Same fail-open family as OB-001; `loadJobs` (`store.go:3293`) shares it. Settings persistence is also a bare `os.WriteFile` with no temp-and-rename and no fsync (`pipeline.go:179`), weaker than both `writeCatalog` and `saveJobs`, and `out, _ := json.MarshalIndent(...)` drops the marshal error. |
| OBX-005 | Medium | `parseTarTOC` (`adopt.go:134-166`) parses untrusted human-readable `tar -tvf` output line by line. A member name containing a newline splits into two catalog entries; one containing `" -> "` is truncated there; a name shaped like a listing line can inject a fabricated entry. Results are written to the catalog as `Chunk.Files` (`adopt.go:332`). Belongs with OB-008. |
| OBX-006 | High | **Windows bsdtar filename-list handling fails for tested Unicode paths.** Found 2026-09-07 when the OBX-001 fixture repair let these tests reach product code on Windows for the first time. `BuildChunk` fails for the tested Unicode names: `pipeline.go:976-981` writes `filelist.txt` as UTF-8, and `pipeline.go:1035` passes it as `--null -T` to `C:\Windows\System32\tar.exe` (bsdtar 3.8.4 / libarchive 3.8.4, active ANSI codepage CP1252). Probed in isolation: `café.txt` fails with a UTF-8 list and **succeeds** with a CP1252 list; `ünïcødé★ 日本語.txt` fails; ASCII-only lists pass with and without a trailing NUL. The evidence supports this build decoding the list in the active ANSI codepage rather than UTF-8. Re-encoding alone is not sufficient — `日本語` has no CP1252 form. **Not established:** that every non-ASCII name fails, that every Windows tar behaves this way, behavior under other codepages (including 65001), GNU/MSYS tar, or POSIX behavior. A separate **unconfirmed security hypothesis** is noted in the report: one probe's diagnostic text contained path-like bytes not derived from the input, which *may* indicate uninitialized-buffer handling upstream; **no disclosure has been demonstrated**, and the raw output is **kept local and excluded from the published checkpoint**. Belongs with OB-008 — these are that fix's regression tests. Fix: a separate bounded Windows Unicode archive-build compatibility repair. |

> **Update 2026-09-07 (OBX-001: the Windows fixture half is repaired; the CI half is not).**
> The TAB name now sits behind the existing `runtime.GOOS != "windows"` guard on branch
> `fix/obx-001-windows-fixture` (test-only, parent `406ed2365b074398f4ef80094951753b961cf757`),
> and the guard's comment now names the two rules actually tested — Windows filename rules
> exclude TAB and newline — without claiming anything broader. Report:
> [reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md](reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md).
>
> **Both affected tests now reach product code on Windows for the first time — and both still
> fail, on a real failure the broken fixture had been hiding.** That is filed separately as
> **OBX-006** and is **not fixed here**.
>
> The suite reads **212 pass / 2 fail / 4 skip** before and after, which understates the change:
> the same two tests fail, but previously they aborted at fixture creation having exercised
> nothing, and now `BuildChunk` fails. **The suite is not green, and no count is predicted for a
> future OBX-006 fix.** Build and vet pass. **Windows race testing remains NOT TESTED** — verified,
> not assumed (`-race` needs cgo; `CGO_ENABLED=0` and no gcc). **No CI ran.** The
> `windows-latest` CI job remains outstanding, so **OBX-001 is not closed**.

> **Update 2026-09-07 (OBX-006: boundary diagnosed, regressions added, `DESIGN_DECISION_REQUIRED`).**
> Branch `fix/obx-006-windows-unicode-tar`, test-only, parent
> `c880c7d3afd7e61e367b8ee3aa64068af540c768`. One new file, `tar_unicode_names_test.go`.
> **No production change** — none of the five boundaries can be repaired losslessly at the
> helper-invocation boundary with the pinned helper. Report:
> [reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md](reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md).
>
> **The mechanism is pinned.** `C:\Windows\System32\tar.exe` (bsdtar 3.8.4, SHA-256
> `9B77D4C9…AE86`, no embedded manifest) decodes the `--null -T` list in the process ANSI code
> page (ACP 1252), not UTF-8. Same file, same directory: a UTF-8 list for `café.txt` **exits 0
> having archived `cafÃ©.txt`**; a CP1252 list archives `café.txt`. Separately, `-C`, `-f` and any
> **directory component** lose anything outside CP1252, while argv *leaf* names (including
> `日本語.txt` and a supplementary-plane name) survive intact — so the boundaries genuinely differ
> and only `-T` plus ANSI path resolution are at fault. Archive **headers are not the defect**;
> they record correct UTF-8 for whatever the helper opened.
>
> **Rejected with evidence:** UTF-8+BOM, UTF-16LE, `-T -` on stdin, absolute paths in the list,
> `--options hdrcharset=UTF-8` (output-only, no effect on `-T`), `LANG`/`LC_ALL`/`LC_CTYPE` on the
> child only (no effect — the Windows CRT ignores them), `\\?\` prefixes, CP1252 re-encoding
> (lossy; `日本語` has no form), and moving the member list to argv (cannot carry a non-CP1252
> directory component, and would need an unbounded command line). 8.3 short paths **do** fix `-C`
> and `-f` losslessly but fix no filename. Nothing machine-wide was changed.
>
> **New exposure recorded, not introduced:** the wrong-file build is caught by
> `verifyTarContents` (`pipeline.go:834-885`) only under `build_verify` `full`/`contents`. The
> default is `full`, so shipped defaults fail safe — but under the **`FAST` preset
> (`build_verify: none`) a wrongly named neighbour would reach the medium unchecked.**
>
> Suite now **212 pass / 7 fail / 4 skip** (uncached, `-count=1 -v ./...`): the 2 pre-existing
> `BuildChunk` failures plus the 5 added regressions. Build, vet and `gofmt` pass. **Windows race
> remains NOT TESTED** (verified: `-race` needs cgo, `CGO_ENABLED=0`, no gcc). **No CI ran.**
> **OBX-006 stays open** and now needs a design decision — see NEXT_ACTIONS.

> **Update 2026-09-07 (OBX-006: decision recorded; FAST containment implemented, pending review).**
> Same branch, same parent `c880c7d3`. Report:
> [reviews/OBX-006-CONTAINMENT-2026-09-07.md](reviews/OBX-006-CONTAINMENT-2026-09-07.md);
> the decision itself is recorded as §10a of the investigation report.
>
> **Investigation complete for the pinned helper** (`C:\Windows\System32\tar.exe`, bsdtar 3.8.4).
> **Native writer direction recorded, implementation deferred** to its own scope — replace Windows
> tar *construction* only, shared explicit member contract, streaming, existing manifest placement
> and package layout preserved, explicit metadata/entry-type support and rejection, separate
> finalisation and file-completion checks, verification kept on during initial deployment,
> helper-selection transparency, external-reader interoperability validated, Unicode restore tracked
> separately. **Not built here.**
>
> **New-build containment implemented, pending review.** `assertWindowsTarBuildVerifiable`
> (`pipeline.go`) refuses a Windows external-tar build whose **effective, normalised** tier is
> `none` — decided on `effectiveIntegrity`, not a preset label or raw config string, so a per-archive
> FAST override on a `full` global is caught too. It runs before tool resolution, staging creation,
> the `BUILDING` write, key generation and every tar call; the package stays `PLANNED`, no saved
> setting is altered, and the message says Contents/Full is required, why, and that **enabling
> verification does not fix Unicode**. No bypass, no version-string allowlist, no code-page
> workaround. Non-Windows behaviour and Contents/Full semantics are unchanged.
>
> Two existing tests that expected an unverified Windows build to succeed had **only their
> Windows-specific expectation** changed to the explicit refusal
> (`TestBuildVerify_FastModeSkipsAndWarns`, `TestFastArchiveAttestsReducedIntegrity`); neither is
> skipped and their other-platform coverage is intact.
>
> Suite **220 pass / 7 fail / 4 skip** (uncached): **+8** containment passes, the **same 7**
> compatibility failures preserved, **no new skips**. Build, vet, `gofmt` pass. **Windows race
> remains NOT TESTED.** **No CI ran.**
>
> **Not closed:** Unicode support is not fixed; **packages built unverified before this guard may
> exist and are NOT retroactively made safe** — their eligibility for later write/rewrite/span needs
> a separate assessment; and `verifyTarContents` is not a namespace-security proof (member-name
> validation, source aliasing, missing hashes and restore confinement remain **OB-008**, **OBX-005**,
> **OB-009**). **OBX-006 remains open.** **PR-03 / OB-002 remains the next substantial workstream.**

> **Update 2026-09-07 (OBX-006: containment reviewed, accepted and checkpointed).** Owner accepted
> the fresh-session review
> [reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md](reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md)
> (verdict `READY_FOR_OWNER_REVIEW`, no blocker) and authorised committing and pushing this
> bounded safety change to `fix/obx-006-windows-unicode-tar` only. **Not authorised and not done:**
> merge, any change to `main`, a release, the native tar writer, or PR-03.
>
> The review confirmed independently, against base `c880c7d3`, that the guard decides on the
> **normalised effective** configuration — per-archive override else global, legacy `"fast"`
> already mapped to `none` — and refuses a new Windows external-tar build at the `none` tier
> **before** tool resolution, staging-directory creation, the `BUILDING` transition, key
> generation and any `tar` invocation. It verified `BuildChunk` is the only new-package
> construction path, that no saved global or per-archive setting is mutated by a refusal, that the
> platform predicate is unreachable from configuration or API, and — by removing only the guard
> call in a **disposable copy** — that exactly the intended refusal assertions fail without it and
> nothing else moves. In that guardless copy an unverified build reached `STAGED` holding the
> wrong look-alike member, which is the path this guard closes.
>
> **Job bookkeeping still occurs on refusal** — a FAILED job and a log line are recorded. The claim
> is only that the unsafe *build operation* is refused; **no "zero catalog activity" guarantee is
> made and job durability is not addressed by this change.**
>
> **Prior executed review evidence (not a new publication run): 220 pass / 7 fail / 4 skip**,
> reproduced by the reviewer. The **seven failures are the known Unicode compatibility failures**,
> preserved deliberately. **Windows race testing and CI remain NOT TESTED.** The suite is not
> green, the branch is **not** release-ready, and **OBX-006 is not resolved**.
>
> **Optional test follow-ups from this review** (non-blocking, deliberately **not** implemented in
> the checkpoint so the accepted candidate stayed byte-identical; these belong to
> `OBX-006-CONTAINMENT-REVIEW-2026-09-07.md` and are unrelated to similarly numbered findings from
> PR-01/PR-02):
> - **F1** — add in-tree coverage for a *verifying* archive override under a `FAST`/`none` global
>   (verified correct out-of-tree, but unpinned in the repository), and assert that a refusal
>   leaves the **per-archive** override intact as well as the global config.
> - **F2** — the tool-independent refusal tests inherit `nativeTools`' skip they do not need.
>
> **Still open, unchanged by this checkpoint:** Unicode compatibility itself; **packages built
> unverified before this guard, which were not reassessed, altered or made safe**; the native
> Windows tar writer, which is an **approved future direction, not implemented functionality**;
> and **OB-008**, **OBX-005**, **OB-009** and the other separately tracked boundaries.
> **Refs OBX-006. PR-03 / OB-002 remains the next substantial implementation workstream**, and the
> published tip of this branch — not `c880c7d3` — is its intended parent.

> **Update 2026-09-07 (OB-002 / PR-03: truthful completion implemented, pending review).**
> Branch `fix/ob-002-durable-completion`, parent `f98eedf381253aae9a94dcfcc80f6bab2aec9317`.
> **Uncommitted and unpushed.** Report:
> [reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md](reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md).
>
> **Five falsehoods, all still open at the parent and all traced in current source, not from
> the baseline's anchors:** `EndBatch` discarded its final catalog write error
> (`_ = s.writeCatalog()`); `EndBatch` wrote only when `batchDepth == 0`, and that counter is
> shared by every concurrent job, so a job finishing while another held a batch wrote
> **nothing** and depended on that unrelated job flushing later; `saveJobs` returned nothing
> and swallowed marshal, write and rename failures with no fsync; `NewJob` ignored its own
> persistence failure and handed back an ID the system called recorded; and `runJob`
> published `COMPLETED` ahead of the artifacts and result describing it, in three unchecked
> writes. `GET /api/jobs` serves memory directly, so a reader could observe `COMPLETED`
> before any of the required writes succeeded.
>
> **Fixed, bounded.** `EndBatch() error` returns its failure and flushes whenever the catalog
> is dirty — a finishing job always writes its own work; all **nine** current batch owners
> (re-enumerated, none nested) fold that error into their result via `endBatchInto`, which
> joins rather than substitutes so an operation error and a flush error never mask each other.
> `saveJobs() error` is a checked, fsynced, atomically published write that preserves the
> previous good record on failure. `NewJob` persists first and rolls back the row **and the ID
> counter** on failure; `runJob` returns 503 and starts no work. `FinishJob` publishes the
> terminal status with its artifacts and result in one checked write.
>
> **Truthful, not atomic.** `catalog.json` and `jobs.json` remain two files. When the catalog
> commits and the terminal job write fails, the data and catalog are **kept** — nothing
> successfully written is rolled back — and the job reads `COMPLETED` **plus** an additive
> `persist_error` field and a `NOT RECORDED` label, with a restart reporting `INTERRUPTED`.
> The failing sidecar is never retried; the failure is reported to the process log and the
> catalog audit trail instead. `syncDir` is still a **no-op on Windows**: this is not
> zero-loss crash durability, and no rollback after an ambiguous rename is claimed.
>
> One additive schema field (`persist_error`, `omitempty`); no job status value added or
> removed. `runJob` now returns `(map, error)` and all 25 call sites were updated.
>
> Suite **230 pass / 7 fail / 4 skip** (uncached): **+10** — exactly the new regressions —
> the **same 7** Unicode compatibility failures by identity and cause, **no new skips**.
> Red/green proven in a disposable copy (8 fail without the fix; the all-succeed and
> batch-bookkeeping controls pass in both). Build, vet, `gofmt` pass. **Windows race remains
> NOT TESTED** (verified: `-race` needs cgo, `CGO_ENABLED=0`, no gcc). **No CI ran.**
>
> **PR-01 fail-closed startup, PR-02 safe replacement, the OBX-001 fixture and the OBX-006
> containment are behaviourally preserved**, with their regressions passing unchanged; two
> test files were adapted for the new signatures only and both were made stricter.
> **OB-002 awaits review. OBX-006 remains open.**

---

## Unreviewed areas

Recorded so the gap is visible rather than implied closed.

- **OB-014 through OB-036 and OPT-001:** `NOT_YET_REVALIDATED`. Not examined in this session.
- **Repository-wide file coverage ledger:** not produced. `NEXT_REVIEW_PROTOCOL.md` section 2
  asks for a per-file ledger over all 130 tracked Go files; this audit covered the OB-001..013
  anchor files in depth and the rest only incidentally.
- **HTTP, authentication and origin behavior:** not audited. `main.go` is 2,768 lines with 162
  canonical routes; only the job runner and a few handlers were read.
- **Migration and dock, incremental backup, finalization, export formats, scheduler and alerts,
  device identity:** the protocol names these as high-risk modules needing independent
  validation. Not done.
- **Tape, optical and Docker:** `NOT TESTED`. No helpers installed, no hardware, out of scope.
- **Cross-platform behavior:** everything here is windows/amd64. Linux and macOS behavior for
  OB-003, OB-004 and OB-011 needs its own evidence, and the handoff is explicit that OS-specific
  behavior requires OS-specific evidence.
- **Race detector on this platform:** `NOT TESTED`, no C compiler.

---

## OB-002 — follow-up patch, 2026-09-07 (appended; nothing above edited)

The fresh source review
(`docs/development/reviews/PR03-OB-002-FRESH-REVIEW-2026-09-07.md`) returned
`NEEDS_CHANGES` with three blockers. All three are now addressed in the working tree at
base `f98eedf3…`; the review's own text and every earlier report are preserved unchanged.

- **Blocker 1 — atomic qualification.** `FinishJob` published `COMPLETED`, released
  `s.jobs.mu`, and the qualification was set by a *later* acquisition, so `/api/jobs`
  could serve an unqualified `COMPLETED` for a job whose record on disk said `RUNNING`.
  It is now set by `markJobUnrecordedLocked` **inside the same lock hold** that publishes
  the status, artifacts and result. `Store.NoteJobUnrecorded` was removed;
  `App.noteUnrecordedJob` only reports, outside every jobs lock. `Store.Job`/`Store.Jobs`
  now hand out deep snapshots (`Result` map and `Artifacts` slice copied), closing the
  escaped-pointer note.
- **Blocker 2 — the UI.** `jobStamp` takes the job; an unrecorded completion gets an
  amber `NOT RECORDED` stamp, never `VERIFIED`, plus "Work finished — completion not
  recorded". `waitJob` resolves only for a clean recorded success and rejects on a
  distinct warning path carrying the job, so the result stays reachable and no caller can
  emit an ordinary success toast. All four in-tree consumers updated (jobs list, job
  detail, `adoptDest`, `dockIngest`, card-check poll). Terminal polling always stops.
- **Blocker 3 — current state vs history.** Two additive fields with one meaning each:
  `unrecorded` (this snapshot is not on disk; cleared by `saveJobs` **before** marshalling
  so bytes and memory agree, restored if the write fails) and `persist_error` (the last
  failure's cause, retained as history). The `— NOT RECORDED:` label mutation is gone. A
  later ordinary jobs write records the row; a later *failed* write cannot. No retry loop.
  Records from older builds decode as "recorded, no history", which is correct.

Also corrected in the same pass: the two false source comments about restart and the
sidecar, the audit-fallback comment (measured **0** durable `job-unrecorded` entries both
when the volume fails and when a batch is open — an attempt is not a durable entry), and
the `runJob` call count, re-enumerated as **24** (the report's 25 counted one line of
prose). The implementation report received a **dated addendum**; its original text was not
rewritten.

Suite **238 pass / 7 fail / 4 skip** (uncached) — **+8**, exactly the new tests; the
**same seven** Windows Unicode compatibility failures by identity, **no new skips**, none
weakened, native tar writer not implemented. Build, vet and `gofmt` pass (`gofmt` clean
apart from the two pre-existing `docs/…/reproducers/` files). All three blockers were
reproduced against the pre-follow-up implementation in a disposable copy; the working
checkout was never reverted. **Windows race remains NOT TESTED** (`-race` needs cgo;
`CGO_ENABLED=0`, no gcc). **No CI ran.** "Recorded" means the checked publication contract
only — `syncDir` is still a **no-op on Windows** and `catalog.json`/`jobs.json` are still
two files.

**OB-002 awaits a focused recheck. OBX-006 remains open.**

### Addendum — 7 September 2026, UI status-precedence correction

The focused recheck closed Blockers 1 and 3 and closed Blocker 2 for `COMPLETED`, and found
one **regression introduced by the follow-up**: the three job UI helpers branched on
`unrecorded` before `status`, so a `FAILED` job whose failure record also could not be
written rendered as an amber `NOT RECORDED` stamp under a sentence claiming the operation
had finished and its results were real. Fixed in `ui/index.html` only (**+56 / −11** lines
this pass): the execution outcome is now primary in `jobStamp`, `jobStampText` and
`jobRecordingNote` — `FAILED` and `INTERRUPTED` keep their own stamp and styling, only a
genuine `COMPLETED` can be downgraded to `UNRECORDED`, and a failed job's note says *"The
job failed. Its failure record could not be saved."* rather than the completion sentence.
`waitJob` already rejected failures on the ordinary error path (so no failed job could
reach a success toast); it now also carries the recording clause, on its own
`recordUnsaved` field rather than on `unrecorded`, which routes callers into the
show-the-results path. `INTERRUPTED` + unrecorded is **not produced by the application**
(`loadJobs` leaves those rows unflagged); that branch is defensive and its test is labelled
synthetic. **No Go production file was touched** — `store.go` and `main.go` are
byte-unchanged.

Coverage added for the gap the recheck named: `TestJobsUI_ExecutionOutcomeTakesPrecedence`
(same node harness, real `ui/index.html` script, cases A–E incl. the composed `vJobs` /
`vJobDetail` output) and `TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded` (the
backend premise, through the real `/api/jobs` handler). The new assertions were run against
the **pre-edit** page in a disposable copy first and failed on exactly the reported
regression, with the A/C/D/E controls passing; they pass against the corrected page, as does
the pre-existing UI block.

**Superseded diff figures.** `+756 / −109` and `+853 / −109` are both stale. Measured now:
tracked **15 files, +900 / −110** (source/UI **+712 / −110**, docs **+188**), `ui/index.html`
alone **+128 / −13**. Suite **240 pass / 7 fail / 4 skip** uncached — **+2**, exactly the two
new tests; the **same seven** Windows Unicode compatibility failures by identity, no new
skips. Build, vet, `gofmt` pass. **Windows race remains NOT TESTED** (`-race` needs cgo;
`CGO_ENABLED=0`, no gcc). **No CI ran.** The nested-`Result` aliasing and input-ownership
observations remain the recheck's documented **nonblockers** — not fixed here, and general
deep immutability is not established.

**Report:** `docs/development/reviews/PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md`.
**OB-002 awaits a targeted recheck of this correction. OBX-006 remains open.**

### Owner acceptance and checkpoint — 7 September 2026, PR-03 / OB-002

**The owner accepted the PR-03 review chain and the final targeted recheck for publication
of the bounded OB-002 completion-recording repair, and it is now committed on
`fix/ob-002-durable-completion`.** Implementation commit `f98310cb3209f374cf883f7df2f3c31b300b3f0a`,
parent `f98eedf381253aae9a94dcfcc80f6bab2aec9317`.

**This is owner acceptance of a reviewed development checkpoint. It is not a claim of
production readiness, of complete concurrency safety, or of universal crash durability.**
Nothing was merged, no release was published, `main` was not moved, and no next workstream
was started.

**Accepted sub-scope — what the checkpoint does establish.** Checked final catalog flushes
propagate failure. A finishing batch no longer depends on an unrelated batch's eventual flush,
under the implemented shared-catalog checkpoint contract. Failed initial job persistence
prevents starting the work. Execution outcome and current recording qualification are exposed
consistently: `unrecorded` describes the **current snapshot's** recording state, `persist_error`
preserves **earlier** recording-failure history, and a later successful ordinary jobs save may
record a previously unrecorded terminal result. `FAILED` remains `FAILED` when saving its
failure record also fails, and the UI presents execution failure and recording failure
separately.

**Limits carried forward unchanged — none of these is closed by this acceptance.** Catalog data
and `jobs.json` remain **two files, not one atomic transaction**. "Recorded" refers to the
checked publication contract implemented here, **not** a guarantee against all power-loss
scenarios, and completion still never asserts that written bytes were read back and verified.
`syncDir` remains a **no-op on Windows**, so rename durability is not guaranteed on this
platform. The nested-`Result` aliasing and input-ownership observations remain documented
**nonblockers**, deliberately not addressed; general deep immutability is **not** established
and no repository-wide audit was completed. The recovery-kit caller's pre-existing
rejection-handling limitation (`ui/index.html:2735`) is **unchanged** and remains an optional
improvement, as does the unused `recordUnsaved` marker. The known **Windows Unicode
compatibility failures** and the deferred native tar writer are unchanged. OBX-006 remains
open. Earlier baseline classifications stand; **no broader issue is closed by this
checkpoint** — the work is referenced as `Refs OB-002`, not closed.

**Test evidence, stated as it actually stands.** The latest reported **full** suite is
**240 pass / 7 fail / 4 skip** — author-reported. **The final targeted reviewer did not rerun
it.** That reviewer independently executed: the enumerated **20-test** selection
(`^(TestJobsUI_|TestDurableCompletion_)`, 20/20 PASS), the **31** prior-safety tests (31/31
PASS), the two **Node-backed UI logic tests** (both executed, not skipped), and a **labelled
bounded mutation experiment** (RED exit 1 / 22 failures, GREEN exit 0). Those sets are **not**
combined here into a full-suite figure and are **not** assumed disjoint; read the reports for
the exact commands and identities. The seven full-suite failures remain the known Windows
Unicode cases. **Windows `-race` remains NOT TESTED** (needs cgo; `CGO_ENABLED=0`, no gcc) and
**no CI ran**. The Node harness is script-logic and emitted-HTML checking only — **not**
browser rendering and **not** accessibility validation. `go build`, `go vet` and `gofmt` were
re-run at publication and pass (`gofmt` clean apart from the two pre-existing frozen
`docs/…/reproducers/` files, which are not published). The full suite was **not** rerun for an
unchanged publication candidate.

**Documentation correction.** A dated erratum was appended (append-only) to
`docs/development/reviews/PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md`: its pre-edit
hashes never preserved the corresponding blobs, so its RED run stands as author-reported and
the later mutation test is **not** a byte-identical replay of it; its two post-edit test blobs
were transposed and are corrected with full hashes; and the distinction between files with
established historical byte-identity and files for which only current identities were recorded
is preserved. No review document was rewritten.

**Reports:** `PR03-OB-002-IMPLEMENTATION-2026-09-07.md`, `PR03-OB-002-FRESH-REVIEW-2026-09-07.md`,
`PR03-OB-002-REVIEW-FOLLOWUP-2026-09-07.md`, `PR03-OB-002-FOCUSED-RECHECK-2026-09-07.md`,
`PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md` (+ erratum),
`PR03-OB-002-UI-TARGETED-RECHECK-2026-09-07.md`, all under `docs/development/reviews/`.
