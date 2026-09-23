# PR-03 / OB-002 — focused recheck of the follow-up patch

**Verdict: NEEDS_CHANGES** — one blocker remains, and it is a **regression introduced by
the follow-up itself**. Blockers 1 and 3 are closed on the evidence below. Blocker 2 is
closed for the `COMPLETED` case it was written about, but its fix mis-states a `FAILED`
job whose record also failed to write. The correction is a few lines in one place.

## Provenance — read this first

**This is a self-recheck, not an independent review.** The same session that wrote the
follow-up patch also performed this recheck. It carries **no approval authority** and is
not a substitute for owner or third-party review. Its value is that it re-derived every
claim from the source and from execution rather than from the follow-up report, and it
found a defect the follow-up report did not disclose.

**Date:** September 7, 2026. Everything marked *verified* was executed on this machine
today; everything else is labelled read, inferred, or NOT TESTED.

**Preserved:** BASELINE, the independent review, the fresh review, the implementation
report (original body untouched; its dated addendum evaluated as an addendum) and every
frozen handoff document. No implementation file was modified by this recheck. Nothing was
fetched, pulled, switched, reset, stashed, cleaned, restored, staged, committed, pushed or
merged.

---

## 1. Review target

| Item | Expected | Observed |
|---|---|---|
| Git root | `…/Mnemosyne/mnemo-go` | matches ✓ |
| Branch | `fix/ob-002-durable-completion` | same ✓ |
| HEAD / overall PR-03 base | `f98eedf381253aae9a94dcfcc80f6bab2aec9317` | same ✓ |
| Staging area | empty | empty ✓ |
| In-progress Git op | none | none ✓ |

The whole PR-03 patch is uncommitted working-tree change against `f98eedf3…`, compared
against that base and **not** against `main`. `git diff --stat` vs the base:
**15 tracked files, +853 / −109**, split as **13 source/UI files (+665 / −109)** and
**2 status documents (+188)**. Untracked: `durable_completion_test.go`,
`durable_completion_followup_test.go`, `durable_completion_ui_test.go`, the three review
documents, and the frozen handoff directories.

### The follow-up delta is exactly reconstructible — verified, not assumed

The previously reviewed candidate's blob IDs were recorded before the follow-up. Applying
the follow-up's own edit list in reverse to the current files reproduced **byte-identical**
matches for all four changed files:

| File | reconstructed pre-follow-up blob | recorded blob | match |
|---|---|---|---|
| `store.go` | `0389ec05e2a686b10a8f42bfeaba4f37425ff101` | same | ✓ |
| `main.go` | `0c40ff149635cf2c7819954081acab3b8148b9ca` | same | ✓ |
| `durable_completion_test.go` | `4e090b8ce129a92cbaef82591242654ffcbebdf8` | same | ✓ |
| `ui/index.html` | `b412feb75bafb8a98c062fb78b399bcff40ac334` | same | ✓ |

So the follow-up delta is known exactly and **no part of the HEAD-to-working-tree diff was
treated as follow-up work when it was not**. The follow-up changed only these four files
plus two new test files and three documents. Delta size: `store.go` 223 changed lines
(**58 functional**, the rest comment), `main.go` 67, `ui/index.html` 95,
`durable_completion_test.go` 22.

### Files examined, identities at the end of this recheck (unchanged throughout)

| File | blob | Lines |
|---|---|---|
| `store.go` | `8e8b1d1b89028f1eca2e2501382163ecf123bd59` | 3855 |
| `main.go` | `05b5eec0b6253615bf5ca97b1b8f59b1b9ab5b18` | 2983 |
| `ui/index.html` | `1bac5a50d6907287e1a7d5543fe69619cb68b0c4` | 4388 |
| `durable_completion_test.go` | `8d43c743baa193d1ea61828f0cfadc6263c1f00d` | 603 |
| `durable_completion_followup_test.go` (untracked, read in full) | `77ab90b1d333c1a82d61e11daa37a987938e61b7` | 497 |
| `durable_completion_ui_test.go` (untracked, read in full) | `b957492d1428abb719c250f2cfcf01942205c815` | 227 |
| `…/PR03-OB-002-FRESH-REVIEW-2026-09-07.md` | SHA-256 `18df0387b5af04c1` | unchanged |

There is **no `CLAUDE.md`** in this repository; `.claude/settings.local.json` is a Bash
allowlist with no behavioural instructions.

---

## 2. Blocker 1 — atomic visibility of the recording failure: **CLOSED**

**Contract:** terminal outcome, artifacts/result and the recording qualification become
observable as one snapshot; on failure the qualification is established before the lock is
released; no caller needs a later acquisition to repair an already-visible result.

**Verified by source:**

- `FinishJob` (`store.go:3688`) fills status, label, artifacts and result, calls
  `saveJobs()`, then calls `markJobUnrecordedLocked(row, err)` at `store.go:3724` — all
  inside one `defer s.jobs.mu.Unlock()` scope. `SetJob`'s terminal path does the same at
  `store.go:3638`. `markJobUnrecordedLocked` (`store.go:3663`) sets `Unrecorded` and
  `PersistError` and is a no-op on `nil` cause, so the success path clears nothing.
- **Every** access to `s.jobs.rows` / `s.jobs.next` lives in `store.go` and is inside a
  function that takes `s.jobs.mu` — enumerated all 20 access sites against all 10
  `s.jobs.mu.Lock()` holders. **There is no unlocked reader**, so the cleared-but-unwritten
  window inside `saveJobs` is genuinely unobservable; a reader blocks or sees the prior
  safe snapshot.
- `markJobUnrecordedLocked` has exactly two callers, both under the mutex.

**Removed mechanism has no surviving caller.** `grep` over all `.go` and `.html` outside
`docs/` finds `NoteJobUnrecorded` only in a *comment* in
`durable_completion_followup_test.go:109`. No code path can apply a stale qualification
after a later successful save. `App.noteUnrecordedJob` now only reports; it is called from
`main.go:563`, `main.go:573` and `profiles.go:742`, and none of the three touches job
state.

**Logging / audit fallback — no deadlock, recursion or retry.** `noteUnrecordedJob` runs
with no jobs lock held, logs unconditionally, then makes one unchecked `Store.Log` call
(which takes the separate `s.mu`). Executed with **both** seams armed in a disposable copy:
it returned, the process log line fired, and the job's qualification was independent of it.
No path retries the sidecar.

**Executed:** `_H_NoUnqualifiedCompletedIsObservable` (read on the very next line after the
failed write, through `Store.Job`, `GET /api/jobs` **and** `GET /api/jobs/{id}`) and
`_H_ConcurrentReaderNeverSeesUnqualified` (three readers on the real HTTP handlers across
a barrier-held save). The concurrency case was run **3× consecutively** — no flake.

---

## 3. Blocker 3 — flag restoration, later saves, restart: **CLOSED**

**The restore is a single choke point and covers every returnable stage.** `saveJobs`
(`store.go:3444`) clears flags, then calls `writeJobsRows()`; **all** of marshal, `OpenFile`,
`Write`, `Sync`, `Close` and `Rename` return through the one
`if err := s.writeJobsRows(); err != nil` branch, which restores. The only step after the
rename is `syncDir`, whose error is discarded — so **every returnable error is still
pre-publication** and the previously accepted `NewJob` row/ID rollback assumption is intact
and unchanged.

**The restore is per-job, not blanket** — verified by execution, not by reading.
`forgiven` holds only the rows that were actually flagged and restores exactly those:

```
after B's good save:             A.unrecorded=false  B.unrecorded=false
after an unrelated FAILED save:  A.unrecorded=true   B.unrecorded=false
A.persist_error="fail-A2"
```

B is not collaterally flagged, and A keeps its own value and its own latest cause.

**A real (non-seam) failure restores too.** Redirecting the sidecar to an uncreatable path
and driving a genuine `OpenFile` failure: `A.unrecorded` stayed `true`. So the behaviour is
not an artefact of the injected seam.

**A successful save acknowledges only what it serialised.** `writeJobsRows` marshals
`s.jobs.rows` in full, so every row in memory is in the bytes; there is no subset write.
Memory and file agree after publication (asserted on actual `jobs.json` bytes by
`_I_LaterSuccessfulWriteRecords`).

**Sequence A–E, all covered and all correct:**

| | Case | Outcome | Evidence |
|---|---|---|---|
| A | terminal save fails | `COMPLETED` + `unrecorded` + `persist_error`, published together | `_H_`, `_I_` setup |
| B | later unrelated save also fails | A **not** acknowledged; flag and cause intact; `jobs.json` untouched | `_I_LaterFailedWriteDoesNotRecord` + my per-job probe |
| C | later unrelated save succeeds | A recorded as part of the whole collection; `unrecorded` absent in the bytes; `persist_error` retained; artifacts + result present | `_I_LaterSuccessfulWriteRecords` |
| D | reopen after B | `INTERRUPTED` — the file that actually survived; 0 artifacts, nil result, no fabricated finish time | my probe (executed) |
| E | reopen after C | `COMPLETED` + history + artifacts | `_I_LaterSuccessfulWriteRecords` |

Error history survives C without claiming the result is still absent — the UI's third state
says "Recorded. An earlier attempt … failed", verified by execution in §4.

**Older records.** A hand-written sidecar was decoded through the real `OpenStore`:

```
row 1 (plain     ): status="COMPLETED" unrecorded=false persist_error=""
row 2 (old       ): status="COMPLETED" unrecorded=false persist_error="written by an older build"
row 3 (impossible): status="COMPLETED" unrecorded=false persist_error="stale claim"
```

Row 2 is the previous build's record: it decodes as *recorded, with history*, which is
correct because it is in the file. Row 3 is the case a good save can never produce; the
defensive clear in `loadJobs` handles it. ✓

Not claimed and not inferred here: `catalog.json` and `jobs.json` are **not** an atomic
transaction, and nothing about power-loss durability or general audit storage was
expanded into.

---

## 4. The new deep-snapshot boundary — accepted, with two recorded limits

Executed against the real `Store` in a disposable copy:

| Probe | Result |
|---|---|
| (a) mutate returned `Result` top-level key / `Artifacts[0]` | **isolated** — store still `files=2`, `"original-artifact"` ✓ |
| (b) mutate a **nested** map inside `Result` | **not isolated** — store changed |
| (c) mutate `Result["artifacts"]` (same slice `runJob` also passes as `arts`) | **not isolated** — store's `Artifacts[0].Label` changed |
| (d) caller mutates the map it handed to `FinishJob`, after recording | **not isolated** — store showed `files=4242` |

**These are limits, not blockers, and the reason is the standard this review was given:
a blocker needs a concrete produced type *and* a reachable mutation path.**

- (c) has a concrete produced type — `runJob` (`main.go:571`) does
  `arts, _ := res["artifacts"].([]Artifact)` and `FinishJob` stores **both** `j.Artifacts =
  arts` and `j.Result = res`, so the copied slice and the aliased one coexist.
- But **no reachable mutator exists**: the only non-test consumers of `Store.Job` /
  `Store.Jobs` are the two API handlers, which JSON-encode and never mutate. Checked the
  input side too — `IngestDrive` (`dock.go:250`) and the other job bodies construct their
  result map immediately before returning and retain no reference, so (d) has no live
  path either.

**No swallowed errors:** `snapshot()` has no failure mode; it cannot turn an error into an
empty result presented as success.

**No API type change:** nil and empty `Artifacts`/`Result` serialise identically (both
omitted under `omitempty`) — verified by encoding both shapes:

```
nil-both    {"id":1,...,"status":"COMPLETED","progress":1,...}
empty-both  {"id":2,...,"status":"COMPLETED","progress":1,...}
```

**Same logical snapshot:** the terminal status, artifacts and result are assigned together
under one lock hold and copied together by `snapshot()`.

---

## 5. Blocker 2 — real UI and polling: **STILL_OPEN** (narrowly)

Cases A, B and C are closed. Case D is not, and the defect was **introduced by this
follow-up**.

### Closed — A, B, C, verified by executing the real page script

| Case | Result |
|---|---|
| **A** `COMPLETED`, `unrecorded:true` | amber `UNRECORDED` stamp reading `NOT RECORDED`; the sentence "Work finished — completion not recorded" is rendered; **no** `stamp VERIFIED`; `waitJob` rejects with `unrecorded===true` carrying the job; no success toast from `adoptDest`; polling stops (job is terminal, `jobsTimer` not armed) |
| **B** `COMPLETED`, clean | positive control passes — `stamp VERIFIED`, success toast, `waitJob` resolves |
| **C** `COMPLETED`, recorded, historical `persist_error` | `jobStamp=VERIFIED`, text `COMPLETED`, list does **not** say "not recorded", no re-poll, `waitJob` resolves cleanly, and the history line is shown separately ✓ |

**Results/artifacts stay genuinely reachable**, not merely attached to a rejected promise:
the recovery-kit caller shows the kit from `e.job.result`; `dockIngest` assigns `j = e.job`
and continues with the real inventory result; `adoptDest` still refreshes the plan view;
the card-check loop passes the job into `ccRender` and the verdict is rendered above the
warning; and both the Jobs list and job detail keep rendering artifacts and the result
summary for an unrecorded completion. Verified by assertions on visible text (`2 records`,
`2 file(s) cataloged`, `Safe to format`), not only CSS class names.

**Completion is not verification:** `COMPLETED` is never rendered as a content-verification
claim, and a comment at the helpers says so explicitly.

### BLOCKER 2 remainder — a `FAILED` job whose record also failed is reported as "NOT RECORDED", and the note contradicts it

**Anchors:** `ui/index.html:2914` (`jobStamp`), `ui/index.html:2920` (`jobStampText`),
`ui/index.html:2924` (`jobRecordingNote`). Reached from `main.go:562`, where a failing job
is finished with `FinishJob(…, "FAILED", nil, nil)` — and `markJobUnrecordedLocked` now sets
`Unrecorded` on **any** terminal status whose write failed, including `FAILED`.

All three helpers branch on `unrecorded` **before** they look at `status`, so the execution
outcome is displaced. Executed against the current page:

```
--- D. FAILED + unrecorded ---
  jobStamp      = UNRECORDED   (control, plain FAILED: FAILED)
  jobStampText  = "NOT RECORDED"   (control: "FAILED")
  note claims "operation itself finished and its results are real": true
  note mentions the execution error at all: false
  jobs list shows the word FAILED anywhere: false
  job detail shows stamp text NOT RECORDED: true
  waitJob rejected: "source unreadable" unrecordedFlag=false
```

**Failure scenario.** A scan fails because the source is unreadable, and the same disk
condition prevents the `FAILED` record from being written. The Jobs list shows an **amber**
`NOT RECORDED` stamp — not the red `FAILED` stamp — and underneath it the sentence *"Work
finished — completion not recorded. The operation itself finished and its results are real
and still available here."* Both halves are false: the operation did **not** finish, and
there are no results. The job's `— ERROR: source unreadable` label text is still rendered,
so the failure is not completely hidden, but the status stamp and the explanatory sentence
now assert the opposite of what happened.

**Contract violated.** The correction brief for this patch required that the fix must not
"confus[e] the operation's own error with a recording error", and this recheck's §5 D
requires `FAILED`/`INTERRUPTED` to "preserve the execution failure/interruption **and** any
distinct recording problem". Today the recording problem overwrites the execution failure.
This is a **regression**: before the follow-up, `jobStamp('FAILED')` returned `FAILED` and
the qualification lived in a label suffix, so the failure survived.

**Smallest correction.** Let status win for non-`COMPLETED` terminal states, and make the
note's wording conditional — roughly:

- `jobStamp`: return `'UNRECORDED'` only when `j.status === 'COMPLETED'`; otherwise keep
  `FAILED` / `INTERRUPTED`.
- `jobStampText`: substitute `NOT RECORDED` only for `COMPLETED`; otherwise keep the status
  and show the recording problem beside it.
- `jobRecordingNote`: use the "the operation itself finished and its results are real"
  sentence only for `COMPLETED`; for a failed job say that the **failure record** could not
  be written, without asserting the work finished.

No status enum change, no redesign, no new framework. **Required test:** a `FAILED` +
`unrecorded` case in `durable_completion_ui_test.go` (which currently has none), plus a Go
case asserting the backend row — there is no `FAILED` + `unrecorded` test at either level
today.

**Scope note:** `INTERRUPTED` + `unrecorded` is **not reachable in production** — the only
producer of `INTERRUPTED` is `loadJobs` (`store.go:3559`), which sets it directly on rows
and deliberately does not flag them. So the required fix is the `FAILED` case.

### The Node harness — genuine, with its non-coverage recorded

The harness extracts the real `<script>` block and calls the page's own `jobStamp`,
`jobStampText`, `jobRecordingNote`, `vJobs`, `vJobDetail`, `waitJob`, `adoptDest` and
`ccRender`. It is **not** a test-only model, and there is direct proof: the same harness run
against the reconstructed pre-follow-up page reproduces the *old* behaviour
(`stamp VERIFIED`, `waitJob` resolves). Confirmed running, not skipping: **PASS in 2.48s**.

What it does **not** exercise, recorded as a coverage gap rather than glossed:
real DOM, layout and CSS (elements are inert stubs and `innerHTML` is just a string);
events, clicks and navigation (`addEventListener` is a no-op); the browser fetch stack and
the 401/auth retry; the perf-strip path (`getElementById` returns `null`); and timer firing
(`setInterval` is silenced, so polling is judged by the arming condition, not by observing
a tick). Logic and emitted HTML only — this is **not** browser-rendering validation.

---

## 6. Documentation corrections — verified

- **False unconditional restart claims are gone.** The old `PersistError` comment
  ("the next start reports the job as INTERRUPTED", "never read back from the sidecar as a
  stored fact") no longer appears anywhere in `store.go`. The single surviving mention of
  `INTERRUPTED`-on-restart (`store.go:3741`) is properly conditioned — it is the tail of
  "Until that happens the durable record stays whatever was last written", i.e. explicitly
  the no-later-write branch. ✓
- **Audit wording separates all three things** (`main.go:579-606`): process logging
  ("ALWAYS happens"), the catalog audit append ("BEST EFFORT and frequently does not reach
  a file", with both the whole-volume and open-batch reasons named and the measured zero),
  and the explicit rule that "an audit ATTEMPT is not evidence of a durable audit entry".
  Nothing describes an in-memory append as a durable audit entry. ✓
- **`runJob` count re-enumerated at this candidate, independently.** `grep` over the
  non-test tree returns 25 lines; `main.go:467` is prose inside `started.write`'s doc
  comment. Actual call sites: **24** = 19 `startedOn(w).write(…)` + 5 `resp, jerr := …`
  (`main.go:719, 1179, 1246, 2222, 2278`). The corrected figure is right; the old 25 was
  not preserved. ✓
- **History preserved, corrections dated.** The implementation report's original §3 table
  row is intact at line 144 (still saying `INTERRUPTED` unconditionally), and the dated
  addendum at line 337 supersedes it at line 356 with both branches. That is the requested
  shape — the historical text was not rewritten. The fresh review is byte-unchanged
  (SHA-256 `18df0387b5af04c1`). ✓

---

## 7. Checks executed

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `node v24.14.1` (pre-existing), existing
toolchain, real system temp, **no `GOTMPDIR` override**, nothing installed, no global
setting changed, no platform substituted, no real catalog / key / backup / hardware
touched. All mutation and red/green work happened in disposable copies under the session
scratchpad; **the working checkout was never reverted or modified.**

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **PASS**, exit 0 |
| Vet | `go vet ./...` | **PASS**, exit 0 |
| Format | `gofmt -l .` | clean outside `docs/`; the only two entries are the **pre-existing frozen** `docs/…/reproducers/semantics_test.go` files, excluded by instruction |
| OB-002 + UI + prior safety | `-run '^(TestDurableCompletion_\|TestJobsUI_\|TestOpenStore.*\|TestSchema.*\|TestAppBackup.*\|TestAtomicRename_.*\|TestExportAppBackup_.*\|TestRestoreAppBackup_.*\|TestContainment_.*\|TestSeeingWhatHappened_.*)$'` | **31/31 PASS**, exit 0 |
| Frontend harness actually ran | `-run '^TestJobsUI_' -v` | **PASS (2.48s)** — executed, not skipped |
| Concurrency stability | `-count=3 -run '^TestDurableCompletion_H_ConcurrentReader…'` | **PASS ×3**, no flake |
| Snapshot ownership, per-job restore, real-failure restore | disposable copy, 4 probes | executed; results in §3–§4 |
| Sequence D, older-record decoding, audit fallback | disposable copy, 3 probes | executed; results in §3 and §2 |
| Case C/D UI behaviour | real page script under node, disposable harness | executed; results in §5 |
| Pre-follow-up reconstruction | reverse-applied edits, `git hash-object` | 4/4 **byte-identical** to recorded blobs |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 238 pass / 7 fail / 4 skip**, 19 sub-pass / 5 sub-fail |
| Windows race | `go test -race` | **NOT TESTED** — requires cgo; `CGO_ENABLED=0`, no gcc on PATH |
| CI | — | **NOT RUN. No CI evidence is claimed.** |

**Independently reproduced, not taken on report.** The author reported 238 / 7 / 4; my own
uncached run produced **238 / 7 / 4** with the same sub-counts. The seven failures are the
same Windows Unicode compatibility cases by identity —
`TestBuildRestore_HostileFilenamesRoundTrip`, `TestBuildFilelist_IsNulDelimited`,
`TestBuildChunk_UnicodeFilenames_ExactMembers`,
`TestBuildChunk_WrongFileSelection_LookalikeNeighbour`, `TestBuildChunk_UnicodeSourceRoot`,
`TestBuildChunk_UnicodeStagingDir`, `TestBuildRestore_UnicodeRoundTrip` — none weakened,
none skipped, native tar writer not implemented. The four skips are the pre-existing
environmental ones. **No new failure and no new skip.**

### The completion / recording guarantee actually accepted here

"Recorded" means, and only means: the whole jobs collection was marshalled, written to a
temp file, `fsync`ed, and renamed into place, and that sequence returned success — after
which the in-memory rows and the bytes agree. It does **not** mean crash-safe or
power-loss-safe: `syncDir` is a no-op on Windows, so the rename's own durability is not
guaranteed on this platform. `catalog.json` and `jobs.json` remain two files and two
writes with no cross-file atomicity. Job completion never asserts that written content was
read back and verified.

---

## 8. Verdict

### **NEEDS_CHANGES**

| Blocker | Status | Note |
|---|---|---|
| 1 — atomic visibility of the recording failure | **CLOSED** | qualification set inside the publishing lock; no unlocked reader anywhere; removed API has no surviving caller; fallback cannot deadlock, recurse or loop |
| 2 — existing UI and polling honour the contract | **STILL_OPEN** | closed for `COMPLETED` (cases A/B/C verified by executing the real page). A `FAILED` job whose record also failed renders as amber `NOT RECORDED` with a note asserting the operation finished and produced results. §5 has the anchors, the executed evidence and the smallest correction |
| 3 — current recording state vs error history | **CLOSED** | per-job restore across every returnable stage incl. a real non-seam failure; A–E all verified; older records decode correctly; `NewJob` rollback assumptions intact |

### Optional improvements — explicitly not blockers

- **Nested `Result` values and `Result["artifacts"]` still alias stored state** (§4 b/c).
  Concrete type, no reachable mutator today. Worth either deep-copying one level further or
  narrowing the `snapshot()` comment, which currently says the values "are not deep-copied"
  but does not mention that `Result["artifacts"]` defeats the `Artifacts` copy.
- **Input ownership is undefended** (§4 d): a job body that retained its result map could
  silently change a recorded terminal result. No current job body does. A one-line comment
  on `FinishJob` stating that it takes ownership would close it.
- **The follow-up report's own `+756 / −109`** is stale by construction — it was measured
  before the `OB_STATUS.md` / `NEXT_ACTIONS.md` appends that the same pass then made. Actual
  is `+853 / −109` (source/UI `+665 / −109`, docs `+188`). Cosmetic.
- **No test covers `FAILED` + `unrecorded` at either level** — it is what let the §5
  regression through, and it should land with that fix.

### Pre-existing limitations — not grounds for blocking

Seven Windows Unicode failures; no Windows race evidence (no cgo/gcc); no CI; deferred
native tar writer; no cross-file atomicity between `catalog.json` and `jobs.json`;
`syncDir`'s Windows no-op; `_B_BothCausesSurvive` still driving `endBatchInto` directly
rather than through an owner; `FinishJob` returning `nil` for an unknown ID; `save()` not
setting `dirty` on a failed unbatched write; OB-007 live-pointer ownership elsewhere in the
Store. None is caused or worsened by the follow-up.

### Not reopened

The accepted batch-completion and job-creation work was not re-audited: the follow-up did
not touch `EndBatch`, the nine batch owners, or `NewJob`'s rollback, and the only relevant
connection — whether splitting `saveJobs` invalidated the pre-publication assumption behind
the rollback — was checked and holds (§3). No repository-wide audit was repeated.

---

**Final state check.** Branch `fix/ob-002-durable-completion`, HEAD
`f98eedf381253aae9a94dcfcc80f6bab2aec9317`, staging **empty**. The working tree held 25
entries at the start of this recheck and holds 26 now — the one addition is this report.
Every implementation file, test and pre-existing report is byte-unchanged: `store.go`
`8e8b1d1b…`, `main.go` `05b5eec0…`, `ui/index.html` `1bac5a50…`,
`durable_completion_test.go` `8d43c743…`, `durable_completion_followup_test.go`
`77ab90b1…`, `durable_completion_ui_test.go` `b957492d…` — identical to their values when
this recheck began. No issue status was updated on the
author's behalf. Nothing was staged, committed, pushed or merged, and no next workstream
was started.

*Focused recheck, 2026-09-07 — self-review by the patch's author session. Not independent
approval.*
