# PR-03 / OB-002 — truthful batch and terminal job completion

**Status: READY_FOR_REVIEW** — one bounded patch, implemented and evidenced. Not staged,
not committed, not pushed.

**Date:** September 7, 2026
**Branch:** `fix/ob-002-durable-completion`
**Parent (verified committed predecessor):** `f98eedf381253aae9a94dcfcc80f6bab2aec9317`
— the OBX-006 containment evidence commit, published and reviewed. Branched from that
explicit SHA, not from `main` and not from `c880c7d3`.

**Not in this patch:** the native tar writer, the containment review's optional F1/F2
test suggestions, the Windows fixture, PR-04. No SQLite, agents, new storage format,
transaction framework, source-boundary redesign, keystore/config reconciliation, UI
redesign or file moves.

---

## 1. Checkpoint

Verified before any edit: root `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go`,
branch `fix/obx-006-windows-unicode-tar` at HEAD `f98eedf3…`, **nothing staged**, no
in-progress Git operation, and a clean tree apart from the expected untracked handoff and
prompt bundles — which were left untouched and are not a reason to clean or stash.
`origin` is `nathansottung/obelisk`; `fix/ob-002-durable-completion` existed neither
locally nor on the remote, so nothing was overwritten or duplicated.

No fetch, pull, reset, stash, clean, rebase, merge or force-switch was run. `main`
(`d97809b2`) and the PR-01/PR-02/OBX-001/OBX-006 branch refs are untouched.

### Changed files (against `f98eedf3…`)

| File | Change | Blob |
|---|---|---|
| `store.go` | persistence layer: checked batch finalization + checked jobs sidecar | `0389ec05e2a686b10a8f42bfeaba4f37425ff101` |
| `main.go` | `runJob` truthfulness + 25 call sites | `0c40ff149635cf2c7819954081acab3b8148b9ca` |
| `profiles.go` | the second (inline) job path | `fa73d70f0278892821f81381653b8c5aef35dccd` |
| `adopt.go` | 2 batch owners → checked finalization | `e35f16ad3ad405d5ef7163c296ad72de6e5de3fe` |
| `dock.go` | 1 batch owner | `53d5603e33ed1003dc77ddd2907b45401184352b` |
| `exports.go` | 2 batch owners | `155f2a6d247b3a28c2d39cf6bd9406483317cb4b` |
| `incremental.go` | 1 batch owner | `a230e9d6c5ae32a6cc949dc65338bda45b9edae1` |
| `mirror.go` | 1 batch owner | `177834d0a33786daa4111b63e7b9b2ebfe86049e` |
| `pipeline.go` | 1 batch owner (`ScanFolder`) | `8ff679fefdd9af0928a154a234b465c872d4d4ae` |
| `plans.go` | 1 batch owner | `aa90932342308cc0f519ae2fb0d22159b2f48d16` |
| `durable_completion_test.go` | **new** — the OB-002 regression matrix | `4e090b8ce129a92cbaef82591242654ffcbebdf8` |
| `appbackup_test.go` | signature adaptation only (§6) | `23106924955faa52c67b4153baebc2f3e17d1d7b` |
| `seeing_what_happened_test.go` | signature adaptation only (§6) | `993cf52fe71aa274b0bebb4a13c2f735ee6bfbc9` |

`git diff --stat` vs the parent: 12 tracked files, +393 / −92, plus the new test file.

---

## 2. Current caller / failure matrix

Enumerated at this commit, not taken from the baseline's counts.

### Batch owners — nine, all still current, all `(…, error)` returns

| # | Owner | File | Was | Now |
|---|---|---|---|---|
| 1 | `AdoptMedia` | `adopt.go:236` | `defer a.Store.EndBatch()` | `defer endBatchInto(a.Store, &err)` |
| 2 | `AdoptFolder` | `adopt.go:399` | same | same |
| 3 | `IngestDrive` | `dock.go:162` | same | same |
| 4 | `ImportStructure` | `exports.go:343` | same | same |
| 5 | `ImportPlan` | `exports.go:511` | same | same |
| 6 | `BackupChanges` | `incremental.go:307` | same | same |
| 7 | `MirrorToVolume` | `mirror.go:143` | same | same |
| 8 | `ScanFolder` | `pipeline.go:519` | same | same |
| 9 | `ExecutePlanFromDrive` | `plans.go:708` | same | same |

The count matches the baseline's "nine", but it was re-derived. `catalog_scale_test.go:108`
is a tenth, test-only, caller that deliberately drops its pending write by zeroing
`batchDepth`; it is unaffected. **None of the nine nests inside another** — each is called
from exactly one HTTP handler — so `batchDepth > 1` arises only from *concurrent* jobs,
which is precisely the hazard.

### Job paths — two

| Path | Entry | Terminal |
|---|---|---|
| `runJob` (`main.go`), 25 handler call sites | `Store.NewJob` | `Store.FinishJob` |
| `recomputeJob` (`profiles.go`), inline | `Store.NewJob` | `Store.SetJob(…, "COMPLETED")` |

### Failures found — all still open at this commit, none previously repaired

| # | Defect | Anchor (parent) | Consequence |
|---|---|---|---|
| D1 | `EndBatch` discarded its final write error (`_ = s.writeCatalog()`) | `store.go:1458` | a batched job returned success for changes never written |
| D2 | `EndBatch` wrote only at `batchDepth == 0` | `store.go:1464` | a job finishing while another job held a batch wrote **nothing** and depended on that unrelated job flushing later |
| D3 | `saveJobs` returned nothing and swallowed marshal, temp-write and rename failures; no fsync | `store.go:3352` | terminal `COMPLETED` could be an in-memory claim while `jobs.json` still said `RUNNING` |
| D4 | `NewJob` ignored its own persistence failure | `store.go:3407` | work launched under an ID the system described as recorded |
| D5 | `runJob` published `COMPLETED` *before* artifacts and result, in three separate unchecked writes | `main.go:522-529` | a reader could see a completed job whose outputs were not recorded |

An API reader **could** observe `COMPLETED` before the required writes succeeded: `GET
/api/jobs` serves `Store.Jobs()` straight from memory, and none of D1–D5 gated it.

---

## 3. The contract implemented

Coalesced progress may still sit in memory during a batch — that is the whole point of
batching, and the batched jobs are idempotent re-runs. What changed is that **no
completion is acknowledged until its persistence boundary has actually been crossed.**

- `EndBatch() error` returns the final write's error, and every one of the nine owners
  folds it into its own error result through the shared `endBatchInto` helper.
- `EndBatch` now flushes **whenever the catalog is dirty**, not only at depth zero. A
  finishing job always writes its own work.
- `saveJobs() error` performs a genuinely checked write — marshal, open, write, **fsync**,
  close, rename, directory sync — matching the durability `writeCatalog` already had, and
  returns every failure.
- `NewJob` persists before returning and **rolls back** the row and the ID counter on
  failure, so no work starts under an unrecorded ID.
- `FinishJob` publishes the terminal status together with the job's artifacts and result
  in **one** checked write, so the record that reaches `jobs.json` is complete or absent.

### Overlapping batches — the chosen behaviour and its tradeoff

A finishing job writes the whole catalog, which necessarily also persists the *other*
job's partially-applied state. That is safe and deliberate: the catalog is a single
document written under `s.mu`, so it is always internally consistent, and every batched
job is an idempotent re-run, so persisting its progress early can only save work on a
replay. **The cost is one extra full-catalog write per overlapping job** — paid once per
job at completion, not per mutation, so the O(n·m) write storm batching exists to prevent
is unaffected.

The alternative — serialising jobs, or giving each an independent dirty set — would need
per-job change tracking inside the Store and is a larger change than the guarantee
requires. **No live-pointer/OB-007 dependency blocks this implementation**: the fix reads
and writes only `batchDepth`/`dirty` under the existing `s.mu`, and holds that mutex for
exactly one catalog write, never across a job.

### Failure ordering and API-visible outcomes

The catalog and `jobs.json` are separate files, and **this patch does not pretend they are
one transaction.** The order is: the operation's catalog changes commit at `EndBatch`,
then the job record is written.

| Situation | Catalog | `jobs.json` | API shows | Restart shows |
|---|---|---|---|---|
| all writes succeed | committed | terminal record | `COMPLETED` | `COMPLETED` |
| final catalog flush fails | not committed | `FAILED` recorded | `FAILED`, label carries the persistence cause | `FAILED` |
| operation fails *and* flush fails | not committed | `FAILED` | both causes in one error; the original still satisfies `errors.Is` | `FAILED` |
| catalog commits, terminal job write fails | **committed and kept** | still `RUNNING` | `COMPLETED` **plus `persist_error`** and a `— NOT RECORDED:` label | `INTERRUPTED` |
| initial job write fails | untouched | untouched | **HTTP 503**, no `job_id`, no work started | nothing |

The fourth row is the two-file case. Nothing successfully written is rolled back to make
the files agree: the copied data and the catalog stay, and only the bookkeeping is reported
as missing. `NoteJobUnrecorded` deliberately **does not retry** the sidecar — that is the
thing that just failed — so the durable record stays `RUNNING`, which `loadJobs` reports as
`INTERRUPTED` on the next start. That is the conservative state and it is the truth: the
process cannot prove the outcome survived it. The failure is reported once to the process
log and once to the catalog audit trail (a *different* file), best-effort.

`Job.PersistError` (`persist_error`, `omitempty`) is the one additive schema field. An
ordinary recorded job serialises byte-identically to before. The four status values are
unchanged — deliberately: the UI's `waitJob` polls for `COMPLETED`/`FAILED` and `jobStamp`
maps anything else to `BUILDING`, so a new status string would hang existing clients.

---

## 4. Red / green evidence

The pre-fix behaviour was restored in a **disposable copy** under the session scratchpad
(`EndBatch` reverted to the dropped error + depth-zero condition, `saveJobs` to swallowing
every failure). The working checkout was never reverted.

| Regression | Guard removed (RED) | Patch applied (GREEN) |
|---|---|---|
| A `…_FinalFlushFailureIsReported` | **FAIL** — "ScanFolder reported success (2 files) although the catalog could never be written" | PASS |
| A `…_FailedFlushJobIsNotCompleted` | **FAIL** — `status = "COMPLETED", want FAILED` | PASS |
| B `…_BothCausesSurvive` | **FAIL** — "the flush failure was discarded"; "a lone flush failure must be reported; got \<nil\>" | PASS |
| C `…_UnrecordedTerminalStateIsQualified` | **FAIL** — no `persist_error`, label unqualified | PASS |
| D `…_FinishingJobDoesNotWaitForAnotherBatch` | **FAIL** — "job A reported success but the catalog on disk holds 0 file(s)" | PASS |
| D `…_FlushFailureSurfacesWithAnotherBatchOpen` | **FAIL** — "must report the failed flush even though another batch was open" | PASS |
| E `…_SuccessfulJobIsRecorded` | PASS | PASS |
| F `…_UnrecordableJobStartsNoWork` | **FAIL** — "runJob must refuse when the job board cannot record the job" | PASS |
| G `…_BatchDepthBookkeeping` | PASS | PASS |
| `…_SaveJobsReportsFailure` | **FAIL** — "SetJob must return the sidecar write failure" | PASS |

Eight fail for the intended reason without the fix; **E and G pass in both**, which is what
makes them controls rather than restatements of the change — the all-succeed path and the
batch bookkeeping were already correct and had to stay correct.

### What the regressions actually check

Real production paths only — `ScanFolder`, `runJob`, `Store` — against disposable temp
stores. Assertions are on **bytes on disk or a genuine `OpenStore` reopen**, not on
observer counts or in-memory fields: `readJobsFile` parses the real `jobs.json`, and
`reopenFiles` opens the catalog fresh. The seams are the per-`Store` `failSave` /
`failSaveJobs` fields — owned by the store under test, never a shared mutable global, so
the concurrency cases cannot leak a fault into another test. No `os.Chmod` is used.

D uses deterministic channel barriers (job B signals its batch is open, then blocks until A
has finished) with 20-second bounds, and tests **both** the success and the failure
direction. F proves no work started by requiring a channel the job body would close to stay
closed, and additionally proves the refused attempt did not burn an ID.

**One scope note, stated rather than hidden:** case B exercises `endBatchInto` — the shared
finalization helper all nine owners defer to — directly rather than through one of them,
because none of the nine has a deterministically reachable operation error that occurs
*after* the catalog has been dirtied; their post-`BeginBatch` failures all return before any
mutation. A and D cover the integrated path; B pins the one decision they cannot reach, at
the exact point where masking would occur, and asserts the original error still satisfies
`errors.Is`.

---

## 5. Test results

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, real system temp (**no
`GOTMPDIR` override**), existing toolchain, PowerShell exit codes checked. No tool
installed, no global setting changed, no platform substituted.

| Check | Command | Result |
|---|---|---|
| OB-002 regressions | `go test -count=1 -v -run '^TestDurableCompletion_' .` | **10/10 PASS**, exit 0 |
| PR-01 startup + recovery-save + schema | `-run '^(TestOpenStore.*\|TestSchema.*\|TestAppBackup.*)$'` | **PASS** — fail-closed reads, read-only newer schema, non-fatal recovery save all intact |
| PR-02 safe replacement | `-run '^(TestAtomicRename_.*\|TestExportAppBackup_.*\|TestRestoreAppBackup_.*)$'` | **PASS** — including all four injected rename failures preserving the destination |
| OBX-006 containment | `-run '^TestContainment_'` | **8 + 4 subtests PASS** |
| Batch owners | `-run '^(TestMirror_.*\|TestScan.*\|TestIncremental.*\|TestDock.*\|TestPlan.*)$'` | **PASS**, incl. `TestMirror_ConcurrentMultiVolume` (the concurrent path the `EndBatch` change most affects) |
| Job restart reconciliation | `TestSeeingWhatHappened_InterruptedReconcile` | **PASS** |
| Build | `go build ./...` | **PASS** (exit 0) |
| Vet | `go vet ./...` | **PASS** (exit 0) |
| Format | `gofmt -l` over all non-`docs/` Go files | **clean** |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 230 pass / 7 fail / 4 skip**, plus 19 passing and 5 failing subtests |
| Windows race | `go test -race …` | **NOT TESTED — unavailable** |

**Against the 220 / 7 / 4 baseline: +10 passes (exactly the new regressions), the same
seven failures by identity and cause, no new skips, no new failures.**

The seven are the unresolved Windows **Unicode compatibility** failures preserved by
OBX-006 — `TestBuildRestore_HostileFilenamesRoundTrip`, `TestBuildFilelist_IsNulDelimited`,
`TestBuildChunk_UnicodeFilenames_ExactMembers`,
`TestBuildChunk_WrongFileSelection_LookalikeNeighbour`, `TestBuildChunk_UnicodeSourceRoot`,
`TestBuildChunk_UnicodeStagingDir`, `TestBuildRestore_UnicodeRoundTrip`. They are **not**
the old invalid-TAB fixture failures, and this patch neither fixes nor touches them. The
four skips (`TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`,
`TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`) are pre-existing and
environmental. The counts are reported, not treated as a target.

**Windows race testing remains NOT TESTED**, verified rather than assumed: `go test -race`
exits 2 with `-race requires cgo`, `CGO_ENABLED=0`, and no `gcc` on PATH. **No CI ran; no
CI or cross-platform result is claimed by this patch.**

---

## 6. Compatibility

**PR-01 behaviour preserved.** A rejected catalog read still cannot fall through into an
empty replacement, and the schema and recovery-save contracts are unchanged —
`writeCatalog`, `openStore`'s read classification and the recovery save were not modified.
Their regressions pass unchanged.

**PR-02 behaviour preserved.** A rename failure still never deletes the existing
destination. `atomicRename` and `mirror.go`'s replacement logic were not touched; the only
`mirror.go` change is the batch owner's signature and deferred finalization.

**OBX-001 and OBX-006 preserved.** `tar_names_test.go` and
`build_verify_windows_containment_test.go` are byte-unchanged, and the containment still
refuses effective-`none` Windows external-tar builds before any construction or
key-generation side effect.

**Unicode compatibility tests keep their intended-success assertions** — unchanged, still
failing, still meant to.

**Two test files were adapted, for signature only**, and both were made *stronger*, never
weaker:

- `appbackup_test.go` — `NewJob`, `AppendJobArtifact` and `SetJob` now return their write
  error. The fixture writes to a real temp store where they must succeed, so the test now
  `t.Fatal`s if one does not, instead of discarding it silently. What it asserts about the
  backup round-trip is untouched.
- `seeing_what_happened_test.go` — same two calls; this test *depends* on the `RUNNING` row
  reaching `jobs.json` before its reopen, so checking the errors makes a previously assumed
  precondition explicit. The restart assertions are untouched.

**API/schema surface.** One additive `Job` field (`persist_error`, `omitempty`). No status
value added or removed. `runJob` now returns `(map[string]any, error)`; all 25 call sites
were updated — 19 uniform `jsonOut(w, runJob(…))` sites became `startedOn(w).write(runJob(…))`
(a method on the writer, because Go forbids mixing a multi-value call with other arguments),
and 5 sites that post-process the response handle the error explicitly. The multi-volume
mirror handler reports a per-volume `{"status":"NOT_STARTED","error":…}` entry instead of a
job id for a volume whose job could not be recorded. `recomputeJob` returns its totals with
a `job_error` and no `job_id` when the board cannot record it — the recompute itself is a
pure in-memory pass over state the catalog already holds, so the totals remain correct.

**Two local variable declarations were removed** (`dock.go`'s `var err error`,
`pipeline.go`'s `var problems []ScanProblem`) because the named results now declare them.
Named results zero-initialise identically and every `return` statement is unchanged.

---

## 7. Durability limits — explicitly not claimed

- **This is not crash-proof, zero-loss durability.** `syncDir` is a **no-op on Windows**
  (directory fsync is unsupported there), so on this platform a rename's own durability
  after a power loss is not guaranteed — only the file contents are, via `Sync`. That
  documented platform residual is preserved, not papered over, and it applies to the jobs
  sidecar exactly as it already did to the catalog.
- **`catalog.json` and `jobs.json` are still two files and two writes.** A crash between
  them leaves the catalog committed and the job `RUNNING` → `INTERRUPTED` on restart. This
  patch makes that state *truthfully reported*; it does not make it impossible.
- **No rollback after an ambiguous publication.** If a rename returns an error the previous
  good file is preserved, but an error does not prove the new bytes did not land; nothing
  here claims otherwise.
- **Progress within a batch is still not durable**, by design. Only completion is gated.
- **Partial-result semantics are untouched.** Domain operations that legitimately report
  per-file failure counts with no fatal error still do so; the new guarantee is scoped to
  persistence acknowledgments, not to every pre-existing meaning of `COMPLETED`.
- **Concurrency evidence is limited to what ran.** The overlapping-batch tests are
  deterministic and passed, and `TestMirror_ConcurrentMultiVolume` passes, but **no race
  detector run backs this** on this machine.

## 8. Remaining acceptance criteria

None of the seven completion-boundary items in the prompt is left unmet, so this is **one
slice, not a partial fix**: final catalog flush, operation-plus-finalization error,
overlapping and nested batches, initial job-record persistence, terminal job-record
persistence, process-visible behaviour when persistence fails, and normal successful reopen
are each implemented and covered.

Open, and deliberately outside this patch:

- **Windows race evidence and CI** for these concurrent paths (`NOT TESTED`; OBX-001's
  `windows-latest` lane is still outstanding).
- **OB-007 live-pointer ownership** — not a blocker here (§3), but the Store still hands out
  live `*Chunk`/`*Folder` pointers elsewhere; unchanged by this patch.
- **OBX-006 Unicode compatibility** and the native tar writer.
- **Existing unverified packages** built before the OBX-006 containment — still unreassessed.
- The containment review's optional **F1/F2** test suggestions.

**Refs OB-002 / PR-03.** Ready for one substantive review.

---

## Addendum — dated correction, 2026-09-07 (follow-up patch)

Everything above is preserved **as it was written**, including the statements this
addendum corrects. Nothing in §1–§8 has been edited; read the corrections below as
superseding the specific claims they name.

### C1 — §3, table row 4: "Restart shows: `INTERRUPTED`" was unconditional and is not

The row asserted `INTERRUPTED` for the "catalog commits, terminal job write fails"
case with no qualification. That is only the branch in which **no later jobs write ever
succeeds**. Because `saveJobs` serialises the whole board, any subsequent ordinary jobs
write — a new job, another job finishing — records job A's terminal snapshot, after
which a restart correctly reports `COMPLETED`. Both branches are now covered by tests
(`_I_LaterSuccessfulWriteRecords`, `_I_RestartWithoutRecoveryPublication`).

Corrected row 4:

| Situation | Catalog | `jobs.json` | API shows | Restart shows |
|---|---|---|---|---|
| catalog commits, terminal job write fails | **committed and kept** | still `RUNNING` until a later write lands | `COMPLETED` **plus `unrecorded: true`** and `persist_error` | `INTERRUPTED` if no later jobs write succeeded; `COMPLETED` (with `persist_error` as history, `unrecorded` absent) if one did |

### C2 — §3: the `— NOT RECORDED:` label mutation is gone

The label is no longer mutated. A mutated label cannot be un-mutated when a later write
legitimately records the row, and repeated marking would double the prefix. The
qualification is the `unrecorded` field, rendered by the UI.

### C3 — §3: one additive field became two, with different meanings

`persist_error` alone was doing three jobs. The contract is now explicit:

- `unrecorded` (bool, `omitempty`) — **current** state: this terminal snapshot is not in
  `jobs.json`. Never written to a successful file, and cleared by `loadJobs` on open.
- `persist_error` (string, `omitempty`) — **history**: the last recording failure's
  cause, retained after recovery.

A record from an older build has neither, which decodes to "recorded, no history" —
correct, because a row that is in the file is recorded.

### C4 — §3: "reported … to the catalog audit trail (a *different* file)" overstates it

Measured on this machine: with the volume unwritable the durable `job-unrecorded` entry
count after a reopen is **0**, and with another job's batch open it is also **0**
(`Store.Log` → `save()` takes the dirty-and-return branch). The process log line always
fires; the catalog audit append is an **attempt**, and an attempt is not evidence of a
durable audit entry. Covered by `_J_CombinedFailureStaysObservable` and
`_J_UnrecordedSurvivesAConcurrentBatch`.

### C5 — §6: `runJob` call sites — 25 corrected to **24**

Re-enumerated at this candidate: `grep -n "runJob("` over the non-test tree returns 25
lines, one of which is the prose in `started.write`'s doc comment. The actual call sites
are **24** (19 `startedOn(w).write(…)` plus 5 `resp, jerr := …`), all of which handle the
error. `recomputeJob` (`profiles.go`) remains the separate inline job path. This was a
counting error in §6, not a missed call site.

### C6 — the visibility gap §3 did not describe

`FinishJob` published `COMPLETED` and released `s.jobs.mu`; `NoteJobUnrecorded` set the
qualification on a **later** acquisition of the same mutex. Between them, `GET /api/jobs`
served an unqualified `COMPLETED`. Reproduced in a disposable copy and now fixed inside
`FinishJob`'s own lock hold. See the follow-up review for the full account.

*Correction dated 2026-09-07. Prior text preserved verbatim; see
`PR03-OB-002-REVIEW-FOLLOWUP-2026-09-07.md` for the follow-up patch it describes.*
