# PR-03 / OB-002 — follow-up patch: all three blockers addressed

**Status: READY_FOR_FOCUSED_RECHECK**

This is an **implementation response**, not an independent review. It was produced by
the agent that made the changes, so it carries **no approval authority** and does not
certify its own work. Everything labelled *verified* below was executed on this machine
on 2026-09-07; everything else is labelled as read, inferred, or NOT TESTED.

**Date:** September 7, 2026
**Addresses:** `docs/development/reviews/PR03-OB-002-FRESH-REVIEW-2026-09-07.md`
(Blockers 1, 2 and 3, plus its five non-blocking corrections).
**Preserves:** BASELINE, the independent review, the fresh review, the PR-03
implementation report and every frozen handoff document. Nothing was rewritten; the
implementation report gained a **dated addendum** rather than an edit.

---

## 1. Identity — parent, before, after

| Item | Value |
|---|---|
| Git root | `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go` |
| Branch | `fix/ob-002-durable-completion` ✓ |
| Parent / PR-03 base (HEAD, unchanged) | `f98eedf381253aae9a94dcfcc80f6bab2aec9317` ✓ |
| Staging area | empty, before and after ✓ |
| In-progress Git op | none (no rebase / merge / cherry-pick / bisect state) ✓ |

No fetch, pull, switch, reset, stash, clean, restore, stage, commit, push, merge or
amend was performed at any point. The patch remains **entirely uncommitted working-tree
change against `f98eedf3…`**.

### Candidate before this response (the reviewed candidate)

Re-verified against the fresh review's §1 table before any edit: branch, HEAD, empty
staging and clean in-progress state all matched, `git diff --stat` was 14 tracked files
/ +484 / −92, and the untracked `durable_completion_test.go` was present. **No material
difference from the reviewed candidate was found**, so no divergence report was needed.

Git blob IDs of the files this response changed, as they stood at entry:

| File | blob (before) |
|---|---|
| `store.go` | `0389ec05e2a686b10a8f42bfeaba4f37425ff101` |
| `main.go` | `0c40ff149635cf2c7819954081acab3b8148b9ca` |
| `ui/index.html` | `b412feb75bafb8a98c062fb78b399bcff40ac334` |
| `durable_completion_test.go` | `4e090b8ce129a92cbaef82591242654ffcbebdf8` |
| whole working diff vs base | `46e1b929911a7e49eed00f44e33752238b419639` |

### Candidate after this response

| File | SHA-256 (16) | Lines |
|---|---|---|
| `store.go` | `e91f658d6733302a` | 3855 |
| `main.go` | `988b9937b387febf` | 2983 |
| `ui/index.html` | `24e4ba278b46c9f1` | 4388 |
| `durable_completion_test.go` | `3a4837e343aa00d8` | 603 |
| `durable_completion_followup_test.go` **(new)** | `560c7bac999122bf` | 497 |
| `durable_completion_ui_test.go` **(new)** | `99a7daf327cc5ed1` | 227 |
| `…/PR03-OB-002-IMPLEMENTATION-2026-09-07.md` | `6b734a3c86b1e093` | 401 |

`git diff --stat` vs the base is now **15 tracked files, +756 / −109**, plus three
untracked test files (`durable_completion_test.go`,
`durable_completion_followup_test.go`, `durable_completion_ui_test.go`). The other ten
source and test files touched by PR-03 (`adopt.go`, `dock.go`, `exports.go`, `incremental.go`,
`mirror.go`, `pipeline.go`, `plans.go`, `profiles.go`, `appbackup_test.go`,
`seeing_what_happened_test.go`) are **unchanged by this follow-up**.

---

## 2. The recording-state contract

The heart of the follow-up. `persist_error` alone was being asked to mean three
different things; it now means exactly one, and a second additive field means the other.

| Fields on a job | Meaning | UI |
|---|---|---|
| `unrecorded: true` + `persist_error` set | **Current**: this terminal snapshot is not in `jobs.json` | amber `NOT RECORDED` stamp + "Work finished — completion not recorded" |
| `unrecorded` absent + `persist_error` set | **History**: recorded now; a recording failure happened earlier | green `VERIFIED` stamp + a dim "Recorded. An earlier attempt … failed" line |
| both absent | ordinary recorded completion | green `VERIFIED` stamp, no note |

Anchors: the field and its contract at `store.go:716-746`; `markJobUnrecordedLocked` at
`store.go:3663`; the clearing rule in `saveJobs` at `store.go:3444`; the API contract at
`main.go:2932`; the UI helpers at `ui/index.html:2899`.

### How a later save records the row — and why it is not optimistic

`saveJobs` serialises **every** row, so a write that lands records the current terminal
snapshot of every job on the board. It therefore clears `Unrecorded` on every flagged
row **before marshalling**, and restores the flags if the write fails:

- **Bytes and memory agree.** Clearing *after* a successful write would leave
  `unrecorded: true` in the very file that disproves it, and a restart would read that
  stale claim back.
- **Nothing is exposed early.** The caller holds `s.jobs.mu` across the whole window and
  every reader (`Store.Job`, `Store.Jobs`) takes the same mutex, so the cleared-but-
  unwritten state is not observable: a reader either blocks or sees the pre-attempt
  state.
- **A failed save acknowledges nothing.** The restore puts every flag back, so an
  unrelated failed write cannot mark job A recorded or clear its warning.
- **`persist_error` is never cleared.** History is written out with the row and survives
  the restart.

There is **no retry loop and no scheduler**. Recovery is ordinary: the next successful
jobs write — a new job, another job finishing — records the row as a side effect of
being a complete serialisation. Until that happens, the durable record stays what was
last written.

### Older records

A record written by an older build carries neither field, which decodes to "recorded, no
history" — correct, because a row that is *in* the file was recorded by the write that
produced the file. `loadJobs` additionally clears `Unrecorded` on open for the same
reason, which also gives the right reading to a `persist_error` written by the previous
(pre-follow-up) build.

### `saveJobs`' failure-stage assumptions, rechecked

The publication steps were moved into `writeJobsRows` unchanged; the flag bookkeeping
wraps them. Every error `saveJobs` can return is still **pre-publication** — the only
step after `os.Rename` is `syncDir`, whose error is deliberately discarded. So
`NewJob`'s row/ID rollback is still a rollback of something that never landed, and was
**not** turned into an unsafe rollback after a published write. `NewJob` and `EndBatch`
are otherwise untouched by this follow-up.

---

## 3. Blocker 1 — atomic, process-visible qualification

**Was:** `FinishJob` set `Status = "COMPLETED"` and released `s.jobs.mu`; `PersistError`
was set by a separate later acquisition inside `NoteJobUnrecorded`. Between the two
holds, `GET /api/jobs` served a plain, unqualified `COMPLETED` for a job whose record on
disk still said `RUNNING`.

**Now:** `FinishJob` calls `markJobUnrecordedLocked(row, err)` **inside the same lock
hold** that published the status, artifacts and result. Execution outcome, artifacts,
result, current recording state and the failure qualification are all established in one
critical section. `SetJob`'s terminal path does the same. A reader may block during the
save, or see the prior safe snapshot; it can never see an unqualified `COMPLETED`.

`Store.NoteJobUnrecorded` was **removed**. Setting the qualification from outside was the
bug, and a late external call would now be actively wrong: by then an ordinary save may
legitimately have recorded the row. `App.noteUnrecordedJob` remains, and now only
reports (process log, then the best-effort audit attempt) — outside every jobs lock, so
no lock combination, deadlock or recursion is introduced.

**Returned data ownership, not assumed.** The review's latent note was closed rather than
relied on: `Store.Job`/`Store.Jobs` previously returned `cp := *j`, a shallow copy whose
`Result` map and `Artifacts` slice still aliased the live row, walked by the JSON encoder
after the mutex was released. Both now go through `Job.snapshot()`, which copies the map
and the slice. Stated limit: the map's *values* are not deep-copied — they are the
`map[string]any` a job returned once, and nothing mutates their interiors after
publication.

| Change | Anchor |
|---|---|
| qualification set under the publishing lock | `store.go:3724` (`FinishJob`), `store.go:3638` (`SetJob`) |
| `markJobUnrecordedLocked` | `store.go:3663` |
| `NoteJobUnrecorded` removed, rationale kept in place | `store.go:3728` |
| reporting-only fallback | `main.go:607` |
| deep snapshot for readers | `store.go:3786` |

**Tests:** `_H_NoUnqualifiedCompletedIsObservable` (the read on the very next line, with
no intervening call, through `Store.Job`, `GET /api/jobs` and `GET /api/jobs/{id}`) and
`_H_ConcurrentReaderNeverSeesUnqualified` (three readers hammering the real HTTP handler
across a save boundary held open by a channel barrier; every observation collected and
checked; bounded, no sleeps as evidence).

---

## 4. Blocker 2 — the existing UI and polling honour the contract

Narrow changes to `ui/index.html` only. No framework, no navigation change, no styling
work beyond one new stamp class.

| Consumer | Change |
|---|---|
| `jobStamp` | takes the **job**, not the bare status; returns `UNRECORDED` (amber) instead of `VERIFIED` when `j.unrecorded` |
| `jobStampText` (new) | the stamp's words: `NOT RECORDED`, never a bare `COMPLETED` |
| `jobRecordingNote` (new) | three states, three sentences: the warning, the history line, or nothing |
| Jobs list (`vJobs`) | new stamp + the warning line under the label; the result summary stays |
| Job detail (`vJobDetail`) | new stamp + the warning above "Writes:"; artifacts stay reachable |
| `waitJob` | resolves **only** for a clean recorded success; rejects with `e.unrecorded === true` and `e.job` for an unrecorded one; also stops on terminal `INTERRUPTED` instead of polling forever |
| recovery-kit caller | shows the kit **and** the warning; no ordinary success wording |
| `adoptDest` | no "Destination adopted" toast on the unrecorded path; warning toast, view still refreshed |
| `dockIngest` | keeps and uses the inventory result; banner says not-recorded instead of "DONE" |
| card-check poll → `ccRender` | the warning renders **above** the safe-to-format verdict; the verdict itself is not discarded |
| CSS | `.stamp.UNRECORDED` — deliberately not `.VERIFIED` |

`waitJob` rejecting (rather than resolving with a flag) is deliberate: a caller nobody
updated fails loudly instead of announcing success quietly. All four in-tree callers were
updated to keep the useful outcome while warning.

Terminal polling stops in every path: `vJobs`/`vJobDetail` only re-poll while `RUNNING`,
and `waitJob` now exits on all three terminal states. Nothing re-runs the operation.

**Job completion is not verification.** A comment at the helpers states it: `COMPLETED`
says the operation ran to the end and its record was written — never that the bytes it
wrote were read back and checked. That claim belongs to verify/check jobs and their own
results.

### Endpoint consistency and the compatibility limit

`GET /api/jobs` and `GET /api/jobs/{id}` both serialise the whole `Job` from the same
snapshot, so their machine-readable qualification is identical and matches the UI —
asserted directly in `_H_NoUnqualifiedCompletedIsObservable`. The contract is documented
at `main.go:2932`, including the limit stated plainly: **adding a field does not make an
existing client persistence-aware.** Anything branching on `status == "COMPLETED"` alone
keeps working and keeps being wrong about the unrecorded case. Every in-tree consumer was
updated; external clients must add the check themselves.

---

## 5. Blocker 3 — current state vs error history, in code and in words

The contract in §2 is the fix. The review's sequence A–F is now a test
(`_I_LaterSuccessfulWriteRecords`) and behaves as required:

| Step | Behaviour, verified |
|---|---|
| A. operation + catalog work complete | catalog committed and kept |
| B. A's terminal jobs write fails | `FinishJob` returns the error |
| C. A visible as completed but NOT RECORDED | `unrecorded: true` + `persist_error`, published under the same lock |
| D. an unrelated ordinary jobs update saves | the bytes it writes contain A's terminal snapshot |
| E. A accurately represented as recorded, failure retained as history | on disk: `COMPLETED`, no `unrecorded`, `persist_error` kept, artifacts and result present; memory and API agree |
| F. restart reconstructs what is in that file | `COMPLETED` + history, artifacts intact |

And the other branch, which the old comments asserted unconditionally
(`_I_RestartWithoutRecoveryPublication`): with **no** later write containing A's terminal
result, a restart recovers only the older record that actually survived — the `RUNNING`
row, reported `INTERRUPTED`, with **no** fabricated artifacts, **no** fabricated result
and **no** invented completion time. The pre-existing reconciliation behaviour is
unchanged.

### Source comments corrected

1. `store.go:716-746` — the `PersistError` doc comment. Was: "the next start reports the
   job as INTERRUPTED" and "It is never read back from the sidecar as a stored fact,
   because by definition the record carrying it was never stored." Both were false once
   a later write succeeded. Replaced with the two-field contract, the three states, the
   older-record rule, and the explicit statement that `PersistError` **is** written and
   read back because it is history.
2. `store.go:3728` (formerly `NoteJobUnrecorded`'s doc comment) — the same unconditional
   restart claim. Replaced with why there is no external marker, why nothing retries, and
   the actual recovery rule ("the next successful jobs write … until that happens the
   durable record stays whatever was last written").

### Audit-fallback explanation corrected

`main.go:607`. Was: "reports the failure … to the catalog audit trail — a **DIFFERENT**
file, which is the point", which reads as a durability claim. Now states the two steps in
decreasing reliability: the **process log line always happens**; the **catalog audit
append is best effort and frequently reaches no file** — measured **0** durable
`job-unrecorded` entries after a reopen both when the volume is unwritable and when
another job holds a batch open (`Store.Log` → `save()` takes the dirty-and-return
branch). An audit *attempt* is not evidence of a durable audit entry, and nothing claims
one. The original persistence failure is preserved on the job either way. General audit
persistence was **not** redesigned.

### Call-count correction

Re-enumerated at this candidate: `grep -n "runJob("` over the non-test tree returns 25
lines, one of which is prose inside `started.write`'s doc comment. **24** actual call
sites (19 `startedOn(w).write(…)` + 5 `resp, jerr := …`), all handling the error;
`recomputeJob` is the separate inline path. The implementation report's "25" is corrected
in a **dated addendum** appended to it — its original §1–§8 text is untouched.

---

## 6. Tests added and changed

New: `durable_completion_followup_test.go` (7 tests), `durable_completion_ui_test.go`
(1 test driving the real page). Changed: three assertions in
`durable_completion_test.go` (`_C_` now asserts the field pair and asserts the label is
**not** mutated; `_E_` gained `unrecorded` checks in memory and on disk). All ten
original regressions are retained and pass.

| Requirement | Test | What it actually drives |
|---|---|---|
| A. terminal-write failure + concurrent reader | `_H_NoUnqualifiedCompletedIsObservable`, `_H_ConcurrentReaderNeverSeesUnqualified` | real `failSaveJobs` seam, real `FinishJob`, `Store.Jobs`, and the real `api(mux, app)` handlers over `httptest`; channel barrier, no sleeps as evidence |
| B. existing UI behaviour | `TestJobsUI_HonoursRecordingState` | the **real** `<script>` block from `ui/index.html`, executed by node: `jobStamp`, `jobStampText`, `jobRecordingNote`, `vJobs`, `vJobDetail`, `waitJob`, `adoptDest`, `ccRender`. Positive controls throughout (clean completion still renders `VERIFIED`, still toasts success, `waitJob` still resolves) |
| C. later successful write | `_I_LaterSuccessfulWriteRecords` | reads actual `jobs.json` bytes and a genuine `OpenStore` reopen |
| D. later failed write | `_I_LaterFailedWriteDoesNotRecord` | a failed `NewJob` write must not mark A recorded, then a successful one must |
| E. restart without recovery publication | `_I_RestartWithoutRecoveryPublication` | reopen; asserts `INTERRUPTED` and asserts nothing was fabricated |
| F. combined failure | `_J_CombinedFailureStaysObservable`, `_J_UnrecordedSurvivesAConcurrentBatch` | both seams armed; bounded so a hang is a failure; asserts no durable audit entry is readable back |
| G. earlier accepted work | `_A_`, `_B_`, `_C_`, `_D_` ×2, `_E_`, `_F_`, `_G_`, `_SaveJobsReportsFailure` | final batch-flush propagation, dual-error preservation, overlapping batches, `NewJob` no-launch-on-save-failure — all retained, all pass |

**On the frontend test's honesty.** It executes the page's own code; it does not
re-implement it. What it does **not** exercise is a real browser: layout, CSS, events and
actual DOM behaviour are stubbed. It proves the *logic and the emitted HTML*, not the
rendering. If `node` is absent the test **skips with an explicit message naming the
coverage that is then missing** rather than passing on source inspection.

The review's note that `waitTerminalJob` could in principle flake (poll for terminal,
then read `PersistError`) no longer applies: the two are now published together.

---

## 7. Commands and actual results

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `node v24.14.1` (already present),
existing toolchain, real system temp, **no `GOTMPDIR` override**, nothing installed, no
machine-wide change, no WSL, no real catalog / key / backup / hardware touched.

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **PASS**, exit 0 |
| Vet | `go vet ./...` | **PASS**, exit 0 |
| Format | `gofmt -l .` | clean apart from the two **pre-existing** `docs/…/reproducers/semantics_test.go` files |
| JS syntax | `node --check` on the extracted script | **PASS** |
| OB-002 + UI regressions | `go test -count=1 -v -run '^(TestDurableCompletion_\|TestJobsUI_)' .` | **18/18 PASS**, exit 0, verified by name |
| PR-01 / PR-02 / OBX-006 / restart | `-run '^(TestOpenStore.*\|TestSchema.*\|TestAppBackup.*\|TestAtomicRename_.*\|TestExportAppBackup_.*\|TestRestoreAppBackup_.*\|TestContainment_.*\|TestSeeingWhatHappened_.*)$'` | **PASS**, exit 0 |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 238 pass / 7 fail / 4 skip**, 19 sub-pass / 5 sub-fail |

**238 / 7 / 4.** The previously reported 230 / 7 / 4 is historical evidence, not a
target; the +8 is exactly the eight new tests. The seven failures are the **same seven
Windows Unicode compatibility tests by identity** — `TestBuildRestore_HostileFilenamesRoundTrip`,
`TestBuildFilelist_IsNulDelimited`, `TestBuildChunk_UnicodeFilenames_ExactMembers`,
`TestBuildChunk_WrongFileSelection_LookalikeNeighbour`, `TestBuildChunk_UnicodeSourceRoot`,
`TestBuildChunk_UnicodeStagingDir`, `TestBuildRestore_UnicodeRoundTrip` — none weakened,
none skipped, and the native tar writer was **not** implemented. The four skips are the
pre-existing environmental ones. **No new failure and no new skip.**

### Red evidence against the pre-follow-up implementation

In a **disposable copy** under the session scratchpad, this follow-up's source edits were
reverse-applied exactly (every replacement inverted, each requiring a unique match), the
copy was built, and the behaviour was measured. The working checkout was never reverted.

Blocker 1 and Blocker 3, driven through the pre-fix API:

```
BETWEEN FinishJob AND NoteJobUnrecorded: status="COMPLETED" persist_error="" label="probe"
  => REPRODUCED: an unqualified COMPLETED is observable after the terminal write failed
AFTER NoteJobUnrecorded:                 status="COMPLETED" persist_error="jobs.json unwritable (injected)"

ON DISK after a later successful save:   status="COMPLETED" persist_error="jobs.json unwritable (injected)"
                                         label="Scan A — NOT RECORDED: the job board could not be written"
  => REPRODUCED: a row that IS recorded on disk still carries the not-recorded qualification
AFTER RESTART:                           status="COMPLETED" persist_error="…" label="… — NOT RECORDED: …"
```

Blocker 2, driving the pre-fix `ui/index.html` with the same harness:

```
jobStamp(COMPLETED) = VERIFIED
jobs list renders "stamp VERIFIED" : true
jobs list mentions persist_error   : false
job detail renders "stamp VERIFIED": true
waitJob resolves as clean success  : true
```

`TestJobsUI_HonoursRecordingState` also fails against the pre-fix page (the new helpers
do not exist there); the probe above is the behavioural statement of the same thing.

---

## 8. Limitations — unchanged, and still stated

**What "recorded" means here.** The publication contract that is actually checked: the
bytes were marshalled, written to a temp file, `fsync`ed, and renamed into place, and
that sequence returned success. It is **not** a new guarantee of zero-loss power-failure
durability.

- **`syncDir` is a no-op on Windows.** File *contents* are durable via `Sync`; the
  rename's own durability under power loss is not guaranteed on this platform. Unchanged
  by this patch and explicitly not claimed.
- **`catalog.json` and `jobs.json` remain two files and two writes.** No cross-file
  atomicity. When the catalog commits and the job record does not, the data stays and
  only the bookkeeping is reported missing.
- **A rename that returns an error is not proof the new bytes did not land**; nothing
  here claims otherwise.
- **Windows race testing: NOT TESTED.** `-race` requires cgo; `CGO_ENABLED=0`, no gcc on
  PATH. The concurrency analysis in §3 is source-level plus the barrier test, not
  race-detector evidence.
- **CI: NOT RUN.** No CI evidence is claimed.
- **The frontend test is not a browser.** Logic and emitted HTML only; layout, CSS and
  real DOM/event behaviour are stubbed.
- **`_B_BothCausesSurvive`** still drives `endBatchInto` directly, not through one of the
  nine owners; it proves error composition, not caller wiring, which is established
  separately by enumeration plus `_A_`/`_D_`. Unchanged, and restated here rather than
  quietly inherited.
- **Seven Windows Unicode compatibility failures** and the deferred native tar writer:
  untouched, and not grounds for blocking this patch.
- **OB-007 live-pointer ownership elsewhere in the Store** is unchanged; only the job
  rows' own snapshot isolation was tightened.
- `save()` still does not set `dirty` when an **unbatched** direct write fails
  (`store.go:1370`) — pre-existing, out of scope here.
- `FinishJob` still returns `nil` for an unknown ID — pre-existing, out of scope here.

---

## 9. Verdict

### **READY_FOR_FOCUSED_RECHECK**

All three blockers are addressed in code, tests and wording, and comments, tests, UI
wording and the implementation report now agree:

| # | Blocker | Addressed by |
|---|---|---|
| 1 | unqualified `COMPLETED` observable after a failed terminal write | `markJobUnrecordedLocked` inside `FinishJob`/`SetJob`'s own lock hold; `NoteJobUnrecorded` removed; deep reader snapshots. Tests `_H_` ×2 |
| 2 | UI ignored the field and reported success | `jobStamp`/`jobStampText`/`jobRecordingNote`, list + detail, `waitJob`'s warning path, all four callers, the new stamp class. Test `TestJobsUI_HonoursRecordingState` |
| 3 | current recording state conflated with error history; false restart comments | the `unrecorded` / `persist_error` contract; clear-before-marshal in `saveJobs`; both source comments and the audit comment corrected; report addendum. Tests `_I_` ×3, `_J_` ×2 |

This is the implementer's own account and must not be read as independent approval.
Nothing was staged, committed, pushed or merged; no issue status was updated on the
author's behalf; PR-04 was not started.

*Follow-up implementation response, 2026-09-07.*
