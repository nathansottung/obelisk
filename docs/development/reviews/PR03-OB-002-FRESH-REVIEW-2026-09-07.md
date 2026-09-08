# PR-03 / OB-002 — fresh-session source review

**Verdict: NEEDS_CHANGES** — three required corrections, all small and local. The
persistence layer itself is sound and the central claim of the patch is real: I
independently reproduced the pre-fix failures and the post-fix passes. What fails is
the *last mile* of the completion contract — the qualification of an unrecorded
completion is not published atomically with the status, no in-tree consumer reads it,
and the restart behaviour is documented as something it demonstrably is not.

**Provenance.** This is a fresh-session AI review performed by Claude (Opus 5) on
2026-09-07 at the user's request. It is **not** a third-party human certification and
carries no approval authority. Everything labelled "verified" below was executed on
this machine; everything else is labelled as read, inferred, or NOT TESTED.

**Date:** September 7, 2026
**Reviewer:** fresh-session AI review (no prior context from the implementation session)
**Preserves:** all prior reports; no issue status updated on the author's behalf.

---

## 1. Patch identity — verified, no mismatch

| Item | Expected | Observed |
|---|---|---|
| Git root | `…/Mnemosyne/mnemo-go` | `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go` ✓ |
| Branch | `fix/ob-002-durable-completion` | same ✓ |
| HEAD / review base | `f98eedf381253aae9a94dcfcc80f6bab2aec9317` | same ✓ |
| Staging area | empty | empty ✓ |
| In-progress Git op | none | none (no rebase / merge / cherry-pick / bisect state) ✓ |

The patch is entirely **uncommitted working-tree change against `f98eedf3…`**, exactly
as the implementation report states. `git diff --stat` vs that base: 14 tracked files,
+484 / −92 (12 source/test files plus `OB_STATUS.md` and `NEXT_ACTIONS.md`), plus the
untracked `durable_completion_test.go`. Compared against the explicit PR-03 base, not
`main`. No fetch, pull, switch, reset, stash, clean, restore, stage, commit, push or
merge was performed at any point.

**Project instructions:** none found — there is no `CLAUDE.md`, `AGENTS.md` or
`CONTRIBUTING.md` in the repository. `.claude/settings.local.json` contains only a
Bash permission allowlist, no behavioural instructions.

### Reviewed file identities (SHA-256, first 16 hex; re-verified unchanged after review)

| SHA-256 (16) | Lines | File |
|---|---|---|
| `88b14fa4e0e37dbe` | 514 | `adopt.go` |
| `df6e8966adab7d31` | 317 | `appbackup_test.go` |
| `9fcb4cc84cd138be` | 751 | `dock.go` |
| `1ad5ef43a5b7a8c2` | 566 | `exports.go` |
| `b654f207dd1be7cd` | 533 | `incremental.go` |
| `3b57de8726836a09` | 2940 | `main.go` |
| `efe6da1b372a7325` | 478 | `mirror.go` |
| `34fc4d8ad04fe5f7` | 1542 | `pipeline.go` |
| `ec94787b2895b267` | 937 | `plans.go` |
| `78faaebe153910ba` | 747 | `profiles.go` |
| `c4ab93160be9d892` | 287 | `seeing_what_happened_test.go` |
| `077aca52c5bc3e19` | 3712 | `store.go` |
| `0c910b703b891487` | 593 | `durable_completion_test.go` (untracked, read in full) |
| `4809a6c2f6b6811a` | 333 | `docs/development/reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md` |
| `084b185c2a708e41` | 998 | `docs/development/OB_STATUS.md` |
| `b93377c6d03be190` | 566 | `docs/development/NEXT_ACTIONS.md` |

`ui/index.html` (4319 lines) was read as the consumer of the new field; it is
**unmodified by this patch**, which is itself one of the findings (§4).

Also read: the PR-03 prompt
(`docs/OBELISK_POST_PR02_FIX_PROMPTS_2026-09-07/03_PR03_DURABLE_COMPLETION.md`), the
implementation report, and the `OB_STATUS.md` / `NEXT_ACTIONS.md` deltas.

---

## 2. Batch completion — re-traced, correct

### Batch owners: nine, enumerated not assumed

`grep BeginBatch()` over the whole tree returns exactly ten call sites: nine
production owners and `catalog_scale_test.go:108` (test-only, deliberately zeroes
`batchDepth`). All nine are converted to a named error result plus
`defer endBatchInto(a.Store, &err)`:

| # | Owner | Anchor |
|---|---|---|
| 1 | `AdoptMedia` | `adopt.go:237` |
| 2 | `AdoptFolder` | `adopt.go:400` |
| 3 | `IngestDrive` | `dock.go:163` |
| 4 | `ImportStructure` | `exports.go:344` |
| 5 | `ImportPlan` | `exports.go:512` |
| 6 | `BackupChanges` | `incremental.go:308` |
| 7 | `MirrorToVolume` | `mirror.go:144` |
| 8 | `ScanFolder` | `pipeline.go:520` |
| 9 | `ExecutePlanFromDrive` | `plans.go:709` |

**No production `EndBatch()` call remains outside `endBatchInto`.** I independently
verified the no-nesting claim by resolving the call graph of all nine: each is called
from exactly one HTTP handler in `main.go` and none calls another. So `batchDepth > 1`
arises only from concurrent jobs — the report's characterisation is accurate.

### The checked-flush contract holds

- **Final flush errors reach the caller and the job before success escapes.**
  `store.go:1491` returns `writeCatalog()`'s error; `endBatchInto` (`store.go:1510`)
  folds it into the owner's named result; `runJob` routes a non-nil `fn` error to
  `FinishJob(…, "FAILED", …)` (`main.go:561`). Verified end-to-end by
  `TestDurableCompletion_A_FailedFlushJobIsNotCompleted`.
- **The two causes stay separately discoverable.** `endBatchInto` wraps the
  *operation* error with `%w` and appends the flush cause with `%v`, so `errors.Is`
  on the original keeps working while the flush text survives. Verified.
- **Failed persistence does not clear dirty state.** `s.dirty` is cleared only at
  `store.go:1445`, after `os.Rename` succeeds; every failure path in `writeCatalog`
  returns before it.
- **A finishing job cannot depend on another open batch.** The `batchDepth == 0`
  precondition is gone: `EndBatch` now flushes whenever `s.dirty`. Verified by
  `TestDurableCompletion_D_*` in **both** directions, with the success case asserting
  against a genuine `OpenStore` reopen, not a counter.
- **Bookkeeping.** `batchDepth` still decrements only when `> 0`, so an unbalanced
  `EndBatch` cannot drive it negative (covered by `_G_BatchDepthBookkeeping`).
- **No new lock-order hazard.** `EndBatch` takes `s.mu` for exactly one catalog write
  and never spans a job. The jobs sidecar is guarded by the separate `s.jobs.mu`, and
  no code path holds one while acquiring the other. No unprotected mutation was
  introduced.

### The checkpoint, described accurately

The report is right not to call this a per-job transaction, and one point in the
patch's favour deserves reinforcing: **flushing the shared catalog mid-batch
introduces no new exposure**, because `save()` already wrote the whole document —
including other jobs' partially-applied state — every `batchInterval` (default 3s) at
`store.go:1370`. The change makes an existing behaviour deterministic at completion
rather than creating one. The stated cost (one extra full-catalog write per
overlapping job, at completion, not per mutation) matches the code.

**On the OB-007 question:** I agree with the author, and checked rather than accepted
it. `EndBatch` reads and writes only `batchDepth`/`dirty` under the existing `s.mu`;
it hands out no pointers and holds no lock across a job. There is no concrete
live-pointer dependency that invalidates the completion guarantee here, and no reason
to demand a Store rewrite.

---

## 3. Job creation and file publication — correct, including the rollback

`NewJob` (`store.go:3508`) increments the counter, appends the row, persists, and on
failure truncates the row and decrements `next` — all under a single `s.jobs.mu` hold,
so concurrent creators are serialised and the truncated row is always the caller's
own. `runJob` returns before launching the goroutine (`main.go:504`); `started.write`
(`main.go:477`) emits **503 with no `job_id`**, and no earlier success response can
have been written because the acknowledgement is the same single call. Verified by
`_F_UnrecordableJobStartsNoWork`, which proves both that the body never ran and that
the refused attempt did not burn an ID.

### Failure-stage map for the jobs sidecar (`store.go:3416`)

| Stage | Error returned? | Published? | Previous record |
|---|---|---|---|
| fault seam / `s.jobs.path == ""` | yes / n-a | no | intact |
| `json.MarshalIndent` | yes | no | intact |
| `os.OpenFile` (temp) | yes | no | intact |
| `f.Write` | yes | no | intact |
| `f.Sync` | yes | no | intact |
| `f.Close` | yes | no | intact |
| `os.Rename` | yes | **no** | intact |
| `syncDir` (parent) | **no — discarded** | yes | replaced |

**This is the key result for the review's rollback question, and it is favourable.**
The only stage that runs *after* publication is `syncDir`, whose error is deliberately
discarded. Therefore **every error `saveJobs` can return is pre-publication**, and
`NewJob`'s row/ID rollback can never orphan or reuse an ID for a record that actually
landed. This is a real property of the code, not an inference from the helper's name.
Previous-record preservation follows from the same structure and is directly asserted
by `_SaveJobsReportsFailure`.

**Limitations, stated rather than inferred:** `syncDir` is a no-op on Windows, so the
rename's own durability under power loss is not guaranteed on this platform — only the
file contents, via `Sync`. A rename that *returns* an error is not proof the new bytes
did not land; nothing here claims otherwise. `catalog.json` and `jobs.json` remain two
files and two writes. The report states all three limits correctly in its §7. A failed
rename also leaves a `.tmp` file behind, matching `writeCatalog`'s pre-existing
behaviour.

---

## 4. Terminal outcome and API visibility — **two blockers**

The additive-field approach is accepted. The prompt permits a minimal additive
diagnostic outcome, and keeping the four status values is the right call given that
`waitJob` and `jobStamp` would hang or mis-bucket on a new string. The implementation
is **not** rejected for retaining `COMPLETED`. It is rejected because the *rest* of
the contract around `COMPLETED + persist_error` does not hold.

`FinishJob` (`store.go:3568`) is genuinely a good change: status, label, artifacts and
result are filled in memory and published in **one** checked write while `s.jobs.mu`
is held, so a reader polling during a *pending* terminal write blocks and cannot see a
half-written outcome. That half of the snapshot requirement is met.

### BLOCKER 1 — an unqualified `COMPLETED` is observable after the terminal write has already failed

**Anchors:** `store.go:3568` (`FinishJob`), `store.go:3612` (`NoteJobUnrecorded`),
`main.go:571` and `main.go:585` (`noteUnrecordedJob`), `main.go:2910` (`GET /api/jobs`).

`FinishJob` sets `Status = "COMPLETED"` and **releases `s.jobs.mu`** when it returns
the write error. `PersistError` is then set by a **separate, later** acquisition of
the same mutex inside `NoteJobUnrecorded`. Between those two lock holds the row is a
plain, unqualified `COMPLETED` — and `Store.Jobs()` (the exact call behind
`GET /api/jobs`) will hand it out.

**Failure scenario.** `jobs.json` becomes unwritable. A scan finishes; `FinishJob`
publishes `COMPLETED` in memory, `saveJobs` fails, `FinishJob` returns and unlocks.
The UI's 1-second `/api/jobs` poll (`ui/index.html:2891`) or any scripted client lands
in the gap and receives `{"status":"COMPLETED"}` with no `persist_error` and no
`NOT RECORDED` label, for a job whose record on disk still says `RUNNING`. This is
precisely the falsehood the patch exists to remove, and it contradicts the patch's own
comment at `main.go:582` ("never an unqualified recorded COMPLETED").

**Verified, not theorised.** In a disposable copy I drove the production sequence
(`FinishJob` → read `Store.Jobs()` → `noteUnrecordedJob`) with the real
`failSaveJobs` seam:

```
BETWEEN FinishJob AND noteUnrecordedJob, GET /api/jobs shows:
    status="COMPLETED" persist_error="" label="probe"
AFTER noteUnrecordedJob:
    status="COMPLETED" persist_error="jobs.json unwritable (injected)"
```

Note that the shipped `_C_` test is itself exposed to this: `waitTerminalJob`
(`durable_completion_test.go:100`) polls `Store.Job(id)` for a terminal status and
*then* reads `PersistError`, so it can in principle observe the same gap and flake.

**Smallest correction.** Set the qualification inside `FinishJob`, under the mutex it
already holds, when `saveJobs()` returns an error — so status and qualification are
published in the same critical section and are never separately observable. Leave
`NoteJobUnrecorded` / `noteUnrecordedJob` for the logging and audit side. **Required
test:** assert that immediately after `FinishJob` returns an error — with no
intervening call — `Store.Jobs()` already reports `persist_error` for that row.

### BLOCKER 2 — the UI ignores `persist_error` and reports the job as success

**Anchors:** `ui/index.html:2893` (`jobStamp`), `ui/index.html:2985` (`waitJob`),
`ui/index.html:2840` (`jobArtifact`), `ui/index.html:4132` (one of the success toasts).

`persist_error` appears **nowhere** in `ui/index.html` — the whole tree was grepped;
the only non-Go, non-doc occurrence is the compiled `obelisk.exe`. Concretely, for a
job carrying `persist_error`:

- `jobStamp('COMPLETED')` returns the CSS class **`VERIFIED`**, so both the Jobs list
  and the job-detail header render the *success* stamp reading `COMPLETED`.
- `waitJob` resolves successfully on `status === 'COMPLETED'`, so callers announce
  plain success — e.g. `toast('Destination adopted')` at line 4132, `ccRender` at
  3186, `showKitResult` at 2729.
- `jobArtifact` renders the result summary as it would for any clean completion.

The only qualification that reaches a person is the mutated `Label` string
(`store.go:3621`), which appends `— NOT RECORDED: …`. That is real mitigation and not
nothing — but the strongest signal on the row is a green `VERIFIED` stamp, and the
automated `waitJob` path drops the qualification entirely. The review contract
requires that the existing UI not report durably recorded success; today it does. The
implementation report's §6 "API/schema surface" discusses status compatibility but
never states that no consumer reads the new field.

**Smallest correction.** Have `jobStamp` (or its two call sites) take the job rather
than the bare status and return a non-success class when `j.persist_error` is set,
render the reason next to the stamp, and make `waitJob` surface the qualification to
its callers instead of resolving as a clean success. This is a handful of lines and
does not touch the status enum.

### Snapshot integrity — checked, holds today, with one latent note

`Jobs()` (`store.go:3696`) and `Job()` (`store.go:3658`) return `cp := *j` — a
**shallow** copy, so `cp.Result` aliases the live map and `cp.Artifacts` the live
backing array, and the JSON encoder walks them after the mutex is released. Every
mutator was checked: after this patch **`SetJobResult`, `AppendJobArtifact` and
`SetJobArtifacts` have no non-test callers at all** (`runJob` now publishes through
`FinishJob` instead), and `FinishJob` assigns wholesale under the lock before the
terminal state exists. So no production path mutates a result map or artifact slice
after publication, and the snapshot is not undermined today. Two caveats worth
recording: the comment at `store.go:3699` ("Hand out copies, not the live rows")
overstates the isolation it actually provides, and there is **no race-detector
evidence** on this platform to back the analysis (§8).

### Two-file honesty — correct

When the catalog commits and the terminal job write fails, nothing successfully
written is rolled back: the copied files and catalog data stay, and only the
bookkeeping is reported as missing. `_C_` asserts the catalog still holds its 2 files
after a genuine reopen. The patch does not portray the two files as one atomic
transaction, in code or in the report.

---

## 5. Next-write and restart sequence — **blocker (documentation), behaviour defensible**

The review asked what is actually stored and shown for a job A whose terminal write
failed, after a later unrelated job operation successfully saves the collection and
the application restarts. **No shipped test covers this** — `_C_` reopens the store
without any intervening successful job write — so the sequence was executed in a
disposable copy against the real code:

```
STEP 1  in-memory A            : status="COMPLETED" persist_error="jobs.json unwritable (injected)"
                                 label="Scan … — NOT RECORDED: the job board could not be written"
STEP 2  on-disk A (right after): status="RUNNING"   persist_error=""
STEP 3  unrelated job B created; the board saves successfully
STEP 4  on-disk A (after B)    : status="COMPLETED" persist_error="jobs.json unwritable (injected)"
                                 label="… — NOT RECORDED: the job board could not be written"
STEP 5  AFTER RESTART A        : status="COMPLETED" persist_error="…" label="… — NOT RECORDED: …"
        => restart reports "COMPLETED", NOT INTERRUPTED
```

**What is right about this.** The qualification is *not* silently lost — the forbidden
outcome. `persist_error` and the `NOT RECORDED` label are serialised (the field is
`omitempty`, not `json:"-"`) and read straight back by `loadJobs`, so there is no
window in which A becomes an *unqualified* completion. A later successful
serialisation legitimately recording the terminal result is explicitly permitted by
the review, and A is not required to stay `INTERRUPTED` forever.

### BLOCKER 3 — the restart behaviour is documented as something it demonstrably is not

**Anchors:** `store.go:716-723` (the `PersistError` doc comment), `store.go:3612`
(`NoteJobUnrecorded`'s doc comment), implementation report §3 (the outcome table and
the prose beneath it).

Three statements in the patch are contradicted by the code's own behaviour:

1. `store.go:719` — "the next start reports the job as **INTERRUPTED**". Step 5 shows
   `COMPLETED`.
2. `store.go:721` — "It is **never read back from the sidecar as a stored fact**,
   because by definition the record carrying it was never stored." Steps 4 and 5 show
   it written to and read back from the sidecar.
3. Report §3, table row 4 — "Restart shows: **INTERRUPTED**", asserted
   unconditionally, and repeated in the prose ("the durable record stays `RUNNING`,
   which `loadJobs` reports as `INTERRUPTED` on the next start"). True only if no
   later job write ever succeeds — which is the *unlikely* branch, since any
   subsequent job on a recovered disk saves the whole collection.

There is also a coherence gap the patch does not address or mention: once step 4 has
happened the record **is** durable, yet it permanently reports `NOT RECORDED` and
carries `persist_error`. That is a *historical* persistence error being presented as
the *current* recording state. It errs conservative — it understates durability rather
than overstating it, so it is not a false completion — but the review asks
specifically that the two be distinguished, and they are not.

**Smallest correction.** Correct the two source comments and the report table (a
qualified "restart reports `INTERRUPTED` **unless a later job write has since
persisted the row**"), and state the historical-vs-current distinction explicitly. If
the author wants the record to read cleanly once it is genuinely durable, the natural
place is `saveJobs`/`loadJobs` — but that is a design choice, not something this review
requires. **Required test:** the step 1-5 sequence above, asserting whatever behaviour
is chosen.

---

## 6. Failure reporting itself — reliable, but the fallback is weaker than described

Traced `noteUnrecordedJob` (`main.go:585`) → `Store.NoteJobUnrecorded` → `log.Print` →
`Store.Log` (`store.go:1559`) → `save()` → `writeCatalog()`.

| Question | Answer |
|---|---|
| Same failing resource? | **Yes** — `Store.Log` writes the catalog, on the same volume. |
| Same lock? | **No.** `NoteJobUnrecorded` releases `s.jobs.mu` before `Store.Log` takes `s.mu`. No path holds one while taking the other. |
| Can it recurse? | **No.** `Store.Log` never touches the jobs sidecar; `NoteJobUnrecorded` never saves. |
| Can it deadlock? | **No.** Called from the `runJob` goroutine with no lock held; `EndBatch` released `s.mu` before `fn` returned. |
| Is its own persistence result checked? | **No** — `_ = s.save()` at `store.go:1563` (pre-existing). |
| Infinite retry? | **No.** `NoteJobUnrecorded` deliberately never re-attempts the sidecar. Correct, and correctly reasoned. |

This satisfies the review's standard: no retry loop is required, and a truthful
process-visible failure is valid when storage is unavailable. The failure is **always**
reported to the process log, unconditionally, before the best-effort audit attempt.

The audit half, however, is weaker than the source comment implies. Measured:

- **Both files unwritable** (the realistic whole-volume failure): `job-unrecorded`
  entries on disk after reopen = **0**.
- **Another job holds a batch open**: entries on disk = **0** — `Store.Log` → `save()`
  takes the `batchDepth > 0` branch, marks the catalog dirty and returns without
  writing.

So in both cases the audit trail is an in-memory append only. The report's §3 does say
"best-effort", which is honest; the *code* comment at `main.go:581-583` ("reports the
failure … to the catalog audit trail — a **DIFFERENT** file, which is the point")
reads as a durability claim it does not deliver. **Non-blocking, but worth one word of
correction.** Crucially, nothing falsely claims durable audit *history* — the entry is
either written or it is not there — so this is an accuracy issue, not a truthfulness
failure.

**What remains observable when both fail:** the process log line (always), the
in-memory `persist_error` and `NOT RECORDED` label via `/api/jobs` (subject to
Blockers 1 and 2), and `RUNNING` → `INTERRUPTED` on restart. That is an honest set.

---

## 7. Tests against the requirements

All ten new regressions were read in full and executed. They drive real production
paths (`ScanFolder`, `runJob`, `Store`) against disposable temp stores, assert on
**bytes on disk** (`readJobsFile`) or a **genuine `OpenStore` reopen** (`reopenFiles`),
use per-`Store` fault seams rather than shared mutable globals, use deterministic
channel barriers with 20-second bounds rather than sleeps for the concurrency cases,
and use no `os.Chmod`. `_E_` and `_G_` are genuine positive controls: both were
confirmed to pass **with and without** the fix.

**The red/green matrix was independently reproduced.** In a disposable copy `EndBatch`
was reverted to the dropped error plus `batchDepth == 0` condition and `saveJobs` to
swallowing every failure, then the suite was run. The result matches the report line
for line — 8 fail for the intended reason, `_E_` and `_G_` pass:

```
FAIL _A_FinalFlushFailureIsReported             "ScanFolder reported success (2 files) although the catalog could never be written"
FAIL _A_FailedFlushJobIsNotCompleted            status = "COMPLETED", want FAILED
FAIL _B_BothCausesSurvive                       "the flush failure was discarded"; "a lone flush failure must be reported; got <nil>"
FAIL _C_UnrecordedTerminalStateIsQualified      no persist_error; label unqualified
FAIL _D_FinishingJobDoesNotWaitForAnotherBatch  "the catalog on disk holds 0 file(s)"
FAIL _D_FlushFailureSurfacesWithAnotherBatchOpen "must report the failed flush even though another batch was open"
PASS _E_SuccessfulJobIsRecorded                 (control — passes both ways)
FAIL _F_UnrecordableJobStartsNoWork             "runJob must refuse when the job board cannot record the job"
PASS _G_BatchDepthBookkeeping                   (control — passes both ways)
FAIL _SaveJobsReportsFailure                    "SetJob must return the sidecar write failure"
```

**On case B, honestly assessed.** `_B_BothCausesSurvive` calls `endBatchInto` directly
rather than through one of the nine owners. The author states this rather than hiding
it, and the stated reason checks out: no owner has a deterministically reachable
operation error occurring *after* the catalog has been dirtied. The test does prove
**error composition** — `errors.Is` on the original still holds and the flush text
survives — at the exact point where masking would occur. It does **not** establish
caller wiring, and must not be described as end-to-end coverage. Caller wiring is
established separately, and adequately, by source tracing (all nine sites enumerated
in §2) plus the integrated `_A_` and `_D_` tests. That combination is sufficient here.

**Count check.** Nine batch owners: **confirmed by enumeration**, all nine updated.
`runJob` call sites: the report says 25; there are **24** — 19 `startedOn(w).write(…)`
plus 5 `resp, jerr := …`, which is also what the report's own breakdown adds up to.
Every one of the 24 handles the error (no site discards `jerr`); the discrepancy is a
counting error in the report, not a missed call site. `recomputeJob`
(`profiles.go:724`) is the separate inline job path and is handled.

**Coverage gaps** (all reachable by small additions, listed with the blockers above):
no test that a reader cannot observe an unqualified `COMPLETED` after a failed
terminal write; no test of the later-successful-save + restart sequence; no test that
any consumer reads `persist_error`.

**Earlier work preserved — verified by execution, not by file identity.** PR-01
(`TestOpenStore*`, `TestSchema*`, `TestAppBackup*`), PR-02 (`TestAtomicRename_*`,
`TestExportAppBackup_*`, `TestRestoreAppBackup_*`), OBX-006 (`TestContainment_*`) and
`TestSeeingWhatHappened_*` all pass. `openStore`'s read classification, the
recovery-save path and `writeCatalog` are untouched by the diff; `atomicRename` and
`mirror.go`'s replacement logic are untouched. The two adapted test files are
**signature-only and strictly stronger** — silent discards became `t.Fatal`, and every
existing assertion is unchanged.

---

## 8. Independent checks executed

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, existing toolchain, real system temp,
**no `GOTMPDIR` override**, no tool installed, no global setting changed, no platform
substituted, no real catalog / key / backup / hardware touched. Disposable data
throughout.

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **PASS**, exit 0 |
| Vet | `go vet ./...` | **PASS**, exit 0 |
| Format | `gofmt -l .` | clean apart from two **pre-existing** files under `docs/…/reproducers/` |
| OB-002 regressions | `go test -count=1 -v -run '^TestDurableCompletion_' .` | **10/10 PASS**, exit 0, verified by name |
| PR-01 / PR-02 / OBX-006 / restart | `-run '^(TestOpenStore.*\|TestSchema.*\|TestAppBackup.*\|TestAtomicRename_.*\|TestExportAppBackup_.*\|TestRestoreAppBackup_.*\|TestContainment_\|TestSeeingWhatHappened_.*)$'` | **PASS**, exit 0 |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 230 pass / 7 fail / 4 skip**, 19 sub-pass / 5 sub-fail |
| Pre-fix reproduction | guards reverted in a **disposable copy** | 8 RED / 2 control PASS — matrix reproduced exactly |
| Blocker probes 1-4 | disposable copy, real seams | all four behaviours reproduced (§4, §5, §6) |
| Windows race | `go test -race` | **NOT TESTED** — `-race` requires cgo; `CGO_ENABLED=0`, no gcc on PATH |
| CI | — | **NOT RUN. No CI evidence is claimed by this review.** |

**The author's reported 230 / 7 / 4 is confirmed by independent execution**, and the
seven failures are the seven named Windows Unicode compatibility cases by identity:
`TestBuildRestore_HostileFilenamesRoundTrip`, `TestBuildFilelist_IsNulDelimited`,
`TestBuildChunk_UnicodeFilenames_ExactMembers`,
`TestBuildChunk_WrongFileSelection_LookalikeNeighbour`,
`TestBuildChunk_UnicodeSourceRoot`, `TestBuildChunk_UnicodeStagingDir`,
`TestBuildRestore_UnicodeRoundTrip`. The four skips (`TestCardCheck_UnlistableDirBlocksFormat`,
`TestCatalogScale`, `TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`) are the
pre-existing environmental ones. **No new failure and no new skip is attributable to
this patch.**

The disposable copy lived entirely in the session scratchpad. **The working checkout
was never reverted, and all thirteen source/test file hashes are byte-identical to
their values at the start of this review** (§1 table re-verified at the end); branch,
HEAD and the empty staging area are likewise unchanged.

---

## 9. Verdict and required changes

### **NEEDS_CHANGES**

The persistence work is good and should not be reduced. The batch layer, the `NewJob`
rollback, the single-write `FinishJob` and the fault-seam design are all correct, and
the evidence behind them survived independent re-execution. Three corrections stand
between this and owner review, none of them architectural.

| # | Blocker | Anchor | Smallest correction |
|---|---|---|---|
| 1 | Unqualified `COMPLETED` observable via `/api/jobs` after the terminal write failed | `store.go:3568`, `store.go:3612`, `main.go:571` | Set `PersistError`/label inside `FinishJob` under the mutex it already holds; keep `noteUnrecordedJob` for logging. Test: `Store.Jobs()` shows `persist_error` immediately after `FinishJob` errors. |
| 2 | UI ignores `persist_error` and renders a `VERIFIED` success stamp; `waitJob` resolves as clean success | `ui/index.html:2893`, `ui/index.html:2985` | Make `jobStamp` and its two call sites qualify on `j.persist_error`; surface it from `waitJob`. No status-enum change. |
| 3 | "Restart shows INTERRUPTED" and "never read back from the sidecar" are false once a later write succeeds | `store.go:716-723`, `store.go:3612`, report §3 | Qualify the two comments and the report row; state the historical-vs-current-state distinction. Test: the step 1-5 sequence. |

### Optional improvements (not blocking)

- Correct "25 `runJob` call sites" to **24** in the report (§6).
- Soften the audit-fallback comment at `main.go:581-583` to match the measured
  best-effort reality (0 durable entries when the disk fails or a batch is open).
- Note at `store.go:3699` that `Jobs()`/`Job()` copies are **shallow** (`Result` and
  `Artifacts` alias the live row); safe today only because no production mutator
  remains after publication.
- `save()` does not set `dirty` when an **unbatched** direct write fails
  (`store.go:1370`), so that change is not retried by a later flush (pre-existing).
- `FinishJob` returns `nil` for an unknown ID — a caller with a stale ID gets silent
  success.

### Pre-existing limitations — explicitly NOT grounds for blocking

The seven Windows Unicode failures; the unavailable Windows race detector; the
deferred native tar writer; the absence of cross-file atomicity between `catalog.json`
and `jobs.json`; `syncDir`'s Windows no-op; no panic recovery in `runJob`'s goroutine
(pre-existing — a panicking job leaves `RUNNING` → `INTERRUPTED`, which is truthful);
OB-007 live-pointer ownership elsewhere in the Store. None of these is caused or
worsened by this patch, and none blocks it.

### Explicit assessment against the review's checklist

| Item | Assessment |
|---|---|
| Checked final flushes and overlapping batches | **Correct.** Nine owners enumerated and converted; both causes survive; dirty preserved on failure; verified in both directions. |
| Initial job persistence and ID rollback | **Correct.** Every `saveJobs` error is pre-publication, so the rollback cannot orphan or reuse an ID. |
| Terminal publication ordering | **Correct.** Status, artifacts and result publish in one checked write under one lock hold. |
| `COMPLETED` + `persist_error` through API/UI consumers | **BLOCKERS 1 and 2.** Qualification is not published atomically with the status, and no in-tree consumer reads it. |
| Later successful saves and restart behaviour | **BLOCKER 3.** Behaviour is defensible and loses nothing; the documentation of it is false. |
| Audit-fallback reliability | **Acceptable.** No deadlock, recursion, retry loop or false history; the process log always fires. One comment overstates durability. |
| Coverage limitations | Three named gaps, each fixable with a small test; no Windows race evidence; no CI. |

---

*Fresh-session AI review, 2026-09-07. No files were staged, committed, pushed or
merged; no issue status was updated; no implementation change was made. Prior reports
are preserved.*
