# PR-03 / OB-002 — targeted recheck of the UI status-precedence correction

**Verdict: READY_FOR_OWNER_REVIEW.**
**The `FAILED` + unrecorded UI blocker is CLOSED.**

## Provenance

**This is a fresh-session AI review.** It is a new session with no continuity with the
sessions that wrote the patch, the follow-up, the focused recheck or the closeout. It read
those documents but re-derived every claim from the source and from execution on this
machine. It is **not** owner review and carries no merge authority.

**Scope, as instructed:** the one remaining blocker — `FAILED` + unrecorded status
precedence in `ui/index.html` — and its immediate regression surface. The repository-wide
review and the persistence design were **not** restarted.

**Date of execution:** 7 September 2026. Everything marked *executed* ran here today;
everything else is labelled read, reported, inferred or NOT TESTED.

**Nothing was modified.** No fetch, pull, switch, reset, stash, clean, restore, stage,
commit, push or merge. No implementation file was touched. The checkout was never reverted.
This report is the only file added.

No completed targeted review of these exact contents exists — the five prior PR-03 documents
all predate the current `ui/index.html`, `durable_completion_ui_test.go` and
`durable_completion_followup_test.go` contents.

There is **no `CLAUDE.md`** in this repository; `.claude/settings.local.json` is a tool
allowlist with no behavioural instructions.

---

## 1. Reviewed candidate identity — executed

| Item | Expected | Observed |
|---|---|---|
| Git root | `…/Mnemosyne/mnemo-go` | matches ✓ |
| Branch | `fix/ob-002-durable-completion` | same ✓ |
| HEAD | `f98eedf381253aae9a94dcfcc80f6bab2aec9317` | same ✓ |
| Staging area | empty | empty ✓ (`git diff --cached --name-only` returns nothing) |
| In-progress Git op | none | none ✓ (no `rebase-merge`, `rebase-apply`, `MERGE_HEAD`, `CHERRY_PICK_HEAD`, `BISECT_LOG`) |
| Stash | — | empty ✓ |

### File identities (`git hash-object`), measured at the start and re-measured at the close — unchanged throughout

| File | Blob | Lines |
|---|---|---|
| `ui/index.html` | `c3aae212746ecb9b6f94c081a1f87808bcc50a1a` | — |
| `durable_completion_ui_test.go` *(untracked, read in full)* | `a6e8fa422d5d15a8ee5c02cf9205d79a0806d705` | 445 |
| `durable_completion_followup_test.go` *(untracked, read in full)* | `446df4babf1482bb918cebca1c665d0cd1f4f40a` | 570 |
| `durable_completion_test.go` *(untracked)* | `8d43c743baa193d1ea61828f0cfadc6263c1f00d` | 603 |
| `store.go` | `8e8b1d1b89028f1eca2e2501382163ecf123bd59` | — |
| `main.go` | `05b5eec0b6253615bf5ca97b1b8f59b1b9ab5b18` | — |

Untracked tests were inspected **directly**, not through `git diff`, which omits them.

### Distinguishing this correction from the whole PR-03 patch

`ui/index.html`, `durable_completion_ui_test.go` and `durable_completion_followup_test.go`
match the closeout's recorded **post-edit** blobs exactly, so the file set this correction
touched is confirmed. Measured now against `f98eedf3…`:

| Slice | Figure |
|---|---|
| Tracked, vs HEAD | 15 files, **+963 / −110** |
| — source/UI (13 files) | **+712 / −110** — matches the closeout exactly |
| — status documents (2 files) | **+251 / −0** — closeout recorded +188 |
| `ui/index.html` alone | **+128 / −13** — matches the closeout exactly |

The only divergence from the closeout is the two **status documents**
(`docs/development/OB_STATUS.md`, `docs/development/NEXT_ACTIONS.md`), which grew by 63 lines
after the closeout measured. Every source/UI figure matches. No implementation file is
implicated.

### The "no Go production changes" claim — verified where evidence exists, limited where it does not

- **Verified.** `store.go` (`8e8b1d1b…`) and `main.go` (`05b5eec0…`) are **byte-identical**
  to the blobs the *focused recheck* recorded for the previously reviewed candidate, and to
  the closeout's unchanged rows. `durable_completion_test.go` (`8d43c743…`) likewise. For
  these three the no-change claim is established, not taken on report.
- **Limitation.** The focused recheck did not record blobs for the other eight modified Go
  files (`adopt.go`, `dock.go`, `exports.go`, `incremental.go`, `mirror.go`, `pipeline.go`,
  `plans.go`, `profiles.go`), and the closeout's re-hash values for them are not published.
  Their current blobs are recorded here as a forward baseline — `adopt.go e35f16ad…`,
  `dock.go 53d5603e…`, `exports.go 155f2a6d…`, `incremental.go a230e9d6…`,
  `mirror.go 177834d0…`, `pipeline.go 8ff679fe…`, `plans.go aa909323…`,
  `profiles.go fa73d70f…` — but **byte-identity to the pre-correction candidate is not
  established for them and is not claimed.** The corroborating evidence is indirect: the
  source/UI diff totals match the closeout exactly except in `ui/index.html`, which is
  consistent with the claim.

### Correction to the closeout's bookkeeping (documentation only)

The closeout's §1 table **transposes the two post-edit test blobs**: it assigns `446df4ba…`
to `durable_completion_ui_test.go` and `a6e8fa42…` to `durable_completion_followup_test.go`.
Measured here it is the reverse, and the closeout's own line counts corroborate the reverse
(`ui_test` 227→**445**, `followup_test` 497→**570**; the 445-line file hashes to
`a6e8fa42…`). The blob *set* is correct; only the row assignment is swapped. **No code
implication.** Not a blocker.

### Pre-edit artifacts are unavailable — stated, not worked around

The closeout's pre-edit blobs (`1bac5a50…`, `b957492d…`, `77ab90b1…`) were produced by
`git hash-object` without `-w` on working-tree files, so they were **never written to the
object database**. `git cat-file -e` confirms all three are **ABSENT**. The closeout's exact
pre-edit RED run is therefore **not byte-identically reproducible**, and this review does not
claim to have replayed it. A bounded equivalent was run instead — see §4.

---

## 2. The UI and its actual consumers — inspected and executed

Anchors: `jobStamp` `ui/index.html:2921`, `jobStampText` `ui/index.html:2933`,
`jobRecordingNote` `ui/index.html:2939`, `waitJob` `ui/index.html:3068`.
Consumers: `vJobs` `ui/index.html:2885` (row composed at `:2891-2893`), `vJobDetail`
`ui/index.html:3014` and `:3017`, `ccRender` `ui/index.html:3312`, `dockIngest`
`ui/index.html:3191-3205`, `adoptDest` `ui/index.html:4246-4247`, the recovery-kit caller
`ui/index.html:2734-2735`.

**The precedence rule holds in the source.** `jobStamp` tests `status` first and returns
`FAILED` / `INTERRUPTED` before `unrecorded` is consulted; only a genuine `COMPLETED` can be
downgraded to `UNRECORDED`. `jobStampText` substitutes `NOT RECORDED` **only** when
`unrecorded && status==='COMPLETED'`. `jobRecordingNote` gates the "the operation itself
finished and its results are real" sentence on `status==='COMPLETED'` and gives every other
unrecorded terminal status its own outcome-first sentence.

### `FAILED` + unrecorded — all seven required properties verified

| Required | Verified | Evidence |
|---|---|---|
| Primary status text and styling remain `FAILED` | ✓ | `jobStamp` returns `'FAILED'` → class `stamp FAILED` (its existing red styling, unchanged); `jobStampText` returns `'FAILED'`. Executed: `stamp FAILED` present and `stamp UNRECORDED` / `stamp VERIFIED` / the string `NOT RECORDED` absent in **composed** `vJobs` **and** `vJobDetail` output |
| Original operation error remains available | ✓ | `— ERROR: source unreadable` present in composed list **and** detail output; also in `waitJob`'s rejection message |
| Recording failure appears as a separate qualification | ✓ | `jobRecordingNote` emits a distinct block: "**The job failed. Its failure record could not be saved.**" plus the `persist_error` cause in its own `<span class="dim mono">` |
| No sentence claims successful completion or valid results | ✓ | Executed against the composed list, the composed detail view and the note: none matches `/results are real\|operation itself finished\|Work finished/` or `/results are (real\|valid\|usable)\|still available here/` |
| Polling stops | ✓ | `vJobs` arms `jobsTimer` only when some job is `RUNNING` (`:3056`); `vJobDetail` arms `jobDetailTimer` only when `running` (`:3038`). Executed on a terminal job: **counted** `/api/jobs` fetches unchanged across 1.4 s |
| No plain-success toast, no automatic rerun | ✓ | `waitJob`'s `FAILED` branch throws; the `for(;;)` loop exits on the first poll. Executed: exactly **one** additional `/api/jobs` fetch, and through the real `adoptDest` caller no `Destination adopted` toast — every toast on that path is on the `bad` (error) channel and carries the operation error |
| `recordUnsaved` does not route the failure into the completed-results path | ✓ | See below |

### Assessment of `recordUnsaved` through the consumers

`waitJob`'s `FAILED` branch sets `err.recordUnsaved=true` and `err.job=j`, and deliberately
does **not** set `err.unrecorded`. That asymmetry is the load-bearing part, because
`unrecorded` is the field every consumer branches on:

- **`dockIngest` (`:3192`)** — `catch(e){if(e&&e.unrecorded){unrecorded=e;j=e.job}else throw e}`.
  A `FAILED` rejection has no `.unrecorded`, so it **re-throws** to the outer
  `catch(e){toast(e.message,1)}`. It never reaches the show-the-results path, never sets
  `dockBanner`, and never produces the "DONE — safe to eject" message. ✓
- **`adoptDest` (`:4247`)** — `.catch(e=>{toast(e.message,1);if(e&&e.unrecorded)vPlanDetail(id)})`.
  Error toast only; the `Destination adopted` toast sits on the `.then` chain and is
  unreachable. ✓ Executed.
- **Recovery kit (`:2735`)** — `.catch(e=>{if(e&&e.unrecorded){…}})`. A `FAILED` rejection
  falls through with no action. This is **identical to the ordinary `FAILED` behaviour that
  predates this change** and is not a regression; the user is still informed, because the
  dialog has already navigated to `#jobs` (`:2730`), where the `FAILED` stamp and the
  recording note render. Recorded as an optional improvement in §6.
- **`ccRender` (`:3312`)** — calls `jobRecordingNote(job)` and renders it above the verdict;
  it inherits the corrected note with no change of its own. ✓

**`recordUnsaved` is set but read by no consumer.** For `FAILED` jobs the recording
qualification therefore reaches the user through two channels that were verified: the
rejection **message** text (which the toast paths render) and `jobRecordingNote` in the jobs
list and job detail. That is sufficient for the contract; the unread field is a forward seam,
noted in §6.

### The other required combinations

| Combination | Verified |
|---|---|
| **`COMPLETED` + unrecorded** | Warning still works — `jobStamp`→`UNRECORDED`, `jobStampText`→`NOT RECORDED`, the "Work finished — completion not recorded" sentence present with the cause. Results remain accessible with the qualification (artifacts and result render; `dockIngest` assigns `j=e.job` and continues; the kit caller still shows the kit). No ordinary recorded-success notification — executed through `adoptDest`: no `Destination adopted`, warning toast present ✓ |
| **`COMPLETED` + recorded + historical `persist_error`** | Not falsely shown as currently unrecorded — `jobStamp`→`VERIFIED`, `jobStampText`→`COMPLETED`, and the string `NOT RECORDED` is **absent** from the composed list. History stays distinct: "Recorded. An earlier attempt to record this job failed (…); a later save succeeded." Established successful polling behaviour intact — `waitJob` resolves ✓ |
| **Ordinary `FAILED`** | `stamp FAILED`, text `FAILED`, empty recording note, and the composed list contains no `could not be saved` clause. `waitJob` rejects with its own error, `recordUnsaved` unset, message **not** extended ✓ |
| **Ordinary `COMPLETED`** | Positive control: `VERIFIED`, `COMPLETED`, empty note, `waitJob` resolves, success toast ✓ |
| **`INTERRUPTED`** | `stamp INTERRUPTED`, text `INTERRUPTED`, empty note. Terminal polling stops: `waitJob` decides on the **first** poll (fetch delta exactly 1) and the list does not re-poll ✓ |
| **`INTERRUPTED` + unrecorded** | **SYNTHETIC — labelled as such, and not claimed to be produced by the current backend.** Verified independently: `markJobUnrecordedLocked` (`store.go:3663`) has exactly two callers, `SetJob` (`store.go:3638`) and `FinishJob` (`store.go:3724`), both on terminal-write paths; `loadJobs` sets `INTERRUPTED` directly and flags nothing. The helper branch is defensive and degrades correctly (`INTERRUPTED` survives, no success claim) |

**A `RUNNING` job cannot be flagged `unrecorded`** — both `markJobUnrecordedLocked` callers
are gated on a terminal status, so `jobStamp`'s `unrecorded`-before-`BUILDING` fallthrough is
unreachable in production. Checked independently of the report.

### Partial artifacts on a failed job

Not assumed away. **Diagnostic access is preserved:** `vJobDetail` renders `j.artifacts` with
**no status gate** (`ui/index.html:3025-3033`), so a failed job carrying partial artifacts
still lists them, and `waitJob`'s rejection carries `err.job` so a caller can reach them.
**Nothing presents them as verified results:** the jobs-list summary chip `jobArtifact`
(`ui/index.html:2846`) is gated on `status==='COMPLETED'` — pre-existing and unchanged — and
the failed-job note asserts no output exists and no result is usable. Both halves of the
instruction are satisfied.

---

## 3. The two new tests and the test selector

### Selector — enumerated first, and the previous mistake reproduced

`go test -list '^(TestJobsUI_|TestDurableCompletion_)' .` enumerates **20** functions
(18 `TestDurableCompletion_*`, 2 `TestJobsUI_*`).

**The previous selector defect is real and was reproduced here.**
`go test -list '^(TestDurableCompletion_|TestJobsUI_)$' .` printed **no test names at all** —
the trailing `$` makes both alternatives unmatchable, since every actual name continues past
those prefixes. The closeout's correction of the focused recheck's row is accurate. The
unanchored pattern above is what actually selects them, and its verbose output was inspected
for RUN/PASS entries rather than trusting the exit code.

### The UI test is not a substitute model

`TestJobsUI_ExecutionOutcomeTakesPrecedence` (`durable_completion_ui_test.go:439`) goes
through `runUIHarness` (`:404`), which reads `ui/index.html`, extracts the **real** `<script>`
block verbatim via `uiScript` (`:28`), concatenates the browser stub + that script + the
assertion block, and runs it under `node`. **No helper is re-implemented in the test.** Both
UI tests share the one harness, so neither can drift onto a different page. Its assertions
run against the page's own `jobStamp`, `jobStampText` and `jobRecordingNote`, against the
**composed** `vJobs` / `vJobDetail` `innerHTML`, and against `waitJob` and the real
`adoptDest` consumer with counted fetches. Verified by reading, and independently by §4:
mutating the page alone flips the result.

### The backend test exercises the real handler

`TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded`
(`durable_completion_followup_test.go:513`) establishes the premise with the real
`Store.FinishJob(id, 0, "… — ERROR: source unreadable", "FAILED", nil, nil)` and the
`failSaveJobs` seam armed, then asserts: status stays `FAILED`; the operation error stays in
the label; `Unrecorded` is set and `PersistError` carries the recording cause; the two causes
are **not** merged (it asserts `persist_error` does **not** contain `source unreadable`); and
no artifacts and no result are invented. It then re-reads through `dcJobsAPI`
(`durable_completion_followup_test.go:44`), which registers the production `api(mux, app)`
routes on an `httptest.NewServer` and issues a real `GET /api/jobs` — **the reported real
handler**, not a stub — confirming both facts survive the handler chain and the JSON encoder.

### Tests executed — exact names and outcomes

Command: `go test -count=1 -v -run '^(TestJobsUI_|TestDurableCompletion_)' .` → **20/20 PASS**, `ok`, exit 0.

| Test | Result |
|---|---|
| `TestJobsUI_ExecutionOutcomeTakesPrecedence` | **PASS (3.90s)** — executed under node, **not skipped** |
| `TestJobsUI_HonoursRecordingState` | **PASS (2.49s)** — executed under node, **not skipped** |
| `TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded` | **PASS (0.02s)** |
| `TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable` | PASS (0.02s) |
| `TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified` | PASS (0.22s) |
| `TestDurableCompletion_I_LaterSuccessfulWriteRecords` | PASS (0.04s) |
| `TestDurableCompletion_I_LaterFailedWriteDoesNotRecord` | PASS (0.03s) |
| `TestDurableCompletion_I_RestartWithoutRecoveryPublication` | PASS (0.03s) |
| `TestDurableCompletion_J_CombinedFailureStaysObservable` | PASS (0.03s) |
| `TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch` | PASS (0.02s) |
| `TestDurableCompletion_A_FinalFlushFailureIsReported` | PASS (0.02s) |
| `TestDurableCompletion_A_FailedFlushJobIsNotCompleted` | PASS (0.03s) |
| `TestDurableCompletion_B_BothCausesSurvive` | PASS (0.02s) |
| `TestDurableCompletion_C_UnrecordedTerminalStateIsQualified` | PASS (0.04s) |
| `TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch` | PASS (0.03s) |
| `TestDurableCompletion_D_FlushFailureSurfacesWithAnotherBatchOpen` | PASS (0.02s) |
| `TestDurableCompletion_E_SuccessfulJobIsRecorded` | PASS (0.03s) |
| `TestDurableCompletion_F_UnrecordableJobStartsNoWork` | PASS (0.17s) |
| `TestDurableCompletion_G_BatchDepthBookkeeping` | PASS (0.03s) |
| `TestDurableCompletion_SaveJobsReportsFailure` | PASS (0.03s) |

The two new tests were also run in isolation with an exact-name anchored selector —
`TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded` **PASS (0.02s)**,
`TestJobsUI_ExecutionOutcomeTakesPrecedence` **PASS (3.89s)**, exit 0.

**Directly affected PR-03 / prior-safety set**, the focused recheck's pattern
(`^(TestOpenStore.*|TestSchema.*|TestAppBackup.*|TestAtomicRename_.*|TestExportAppBackup_.*|TestRestoreAppBackup_.*|TestContainment_.*|TestSeeingWhatHappened_.*)$`):
**31/31 PASS**, exit 0 — 4 `AppBackup`, 6 `AtomicRename`, 1 `ExportAppBackup`,
1 `RestoreAppBackup`, 8 `Containment`, 8 `OpenStore`, 3 `SeeingWhatHappened`. No
`TestSchema*` function exists, which is why the pattern yields 31 and not more.

---

## 4. Red/green evidence — reported vs re-established here

**The closeout's exact RED run is NOT reproducible** — its pre-edit blobs are absent from the
object database (§1), so its "26 failures against blob `1bac5a50…`" is **reported evidence
only**. It was not replayed, and byte-identity was not established.

**A bounded equivalent was executed instead, in a disposable copy under the session
scratchpad; the working checkout was never reverted.** The harness's own `uiPrelude` and
`uiPrecedenceAssertions` were extracted verbatim from `durable_completion_ui_test.go` and
composed with a copy of the page in which **only** the three helpers were reverted to the
documented defect shape — `jobStamp` consulting `unrecorded` first, `jobStampText`
substituting `NOT RECORDED` for any unrecorded status, and `jobRecordingNote`'s completion
sentence made unconditional on `unrecorded`. This is a **mutation test, not a byte-identical
replay**, and is labelled as such.

- **RED — defect-shaped page: exit 1, 22 failures**, every one of them the reported
  regression, including `jobStamp` returning `UNRECORDED`, the stamp reading `NOT RECORDED`,
  and the note rendering *"**Work finished — completion not recorded.** The operation itself
  finished and its results are real and still available here…"* over a `FAILED` job — in the
  helper, in the composed `vJobs` output and in the composed `vJobDetail` output.
- **Controls passed on the defect-shaped page**: ordinary `FAILED`, the healed/history case,
  plain `INTERRUPTED`, the polling-count assertions, every `waitJob` assertion and both
  `adoptDest` cases. The RED run therefore **isolates the regression** rather than failing
  generally. (That `waitJob` passed under the mutation independently corroborates the
  closeout's statement that `waitJob`'s `FAILED` branch was never the regression.)
- **GREEN — the current page through the same extracted harness: `PASS`, exit 0.**

Because only the page changed between the two runs, the assertions demonstrably detect the
defect and are not tautological against the current page.

### The affirmative-vs-denial distinction — verified, not assumed

This was the specific trap flagged for checking, and the tests handle it correctly.

- The verification-claim guard is
  `/contents? (were |was )?verified|verified against|checked the (files|bytes|contents)|read back and (checked|verified)/i`,
  asserted **negative** on the `COMPLETED` + unrecorded note. Probed directly: it returns
  **false** against the note's own denial *"…is not verified history"* and **true** against an
  affirmative sentence. It is **not** an overbroad `/verified/` substring.
- The denial is separately asserted **positive**: `/not verified history/i` must match — so
  removing the denial would fail the suite rather than pass it.
- The success-claim guard `claimsSuccess = /results are real|operation itself finished|Work finished/`
  returns **false** against the `FAILED` note's outcome-first wording ("The outcome above
  stands; what is missing is the saved record of it"), which is denial-shaped, and **true**
  against the completion sentence. It does not misfire on the denial.

The closeout's disclosure that one of its own assertions initially matched the denial and was
corrected **in the test rather than in the page** is consistent with the code as it stands.

---

## 5. Other checks executed

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `node v24.14.1` — the pre-existing
environment. Nothing installed, no platform substituted, **no `GOTMPDIR` override**, no
global setting changed, no real catalog, key, backup, archive or hardware touched. All
disposable work stayed in the session scratchpad.

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **PASS**, exit 0 |
| Vet | `go vet ./...` | **PASS**, exit 0 |
| Format | `gofmt -l .` | clean except the two **pre-existing frozen** `docs/…/reproducers/semantics_test.go` files (excluded by instruction). Both new/edited test files are gofmt-clean |
| Test-name enumeration | `go test -list …` | executed before running anything selected |
| Full suite | — | **NOT RERUN.** The author's **240 pass / 7 fail / 4 skip** stands as **reported evidence only**. Not mandatory for a targeted recheck, per instruction |
| Windows `-race` | — | **NOT TESTED** |
| CI | — | **NOT RUN. No CI evidence is claimed** |
| Browser rendering / accessibility | — | **NOT TESTED** — see §6 |

---

## 6. Verdict

### **READY_FOR_OWNER_REVIEW**

### The `FAILED` + unrecorded UI blocker is **CLOSED**

| Blocker | Status | Basis |
|---|---|---|
| 1 — atomic visibility of the recording failure | **CLOSED** (unchanged; closed by the focused recheck, not re-litigated here) | — |
| 2 — existing UI and polling honour the contract | **CLOSED** | `COMPLETED` was already closed. The `FAILED` remainder is closed on: the source precedence in all four sites; execution against the real page across the full status × recording-state matrix, including composed `vJobs` and `vJobDetail` output and the real `adoptDest` consumer; polling stopping on counted fetches; `recordUnsaved` verified through every consumer as **not** routing into the completed-results path; and a RED/GREEN mutation pair proving the new assertions detect the defect |
| 3 — current recording state vs error history | **CLOSED** (unchanged; re-confirmed incidentally by the healed-case assertions) | — |

**No blockers are raised by this review.**

Nested `Result` aliasing and input ownership are **not reopened**. They remain documented
non-blockers; this UI change touches no Go production code and makes no new failure through
them reachable.

### Optional improvements — not blockers

1. **`recordUnsaved` is written but never read.** No consumer branches on it; the recording
   qualification reaches the user only via the rejection message text and
   `jobRecordingNote`. Adequate today, but the field is currently a seam with no consumer —
   either use it or mark it reserved.
2. **The recovery-kit caller (`ui/index.html:2735`) silently swallows any `FAILED`
   rejection.** Pre-existing and unchanged by this pass, and the user still sees the failure
   because the dialog has already navigated to `#jobs`. A one-line
   `else toast(e.message,1)` would close it.
3. **Closeout bookkeeping.** The two post-edit test blobs are transposed in its §1 table
   (§1 above). Documentation only.

### Existing limitations, carried forward unchanged

- **The node harness is script-logic and emitted-HTML testing only.** It is **not** browser
  rendering and **not** accessibility validation: no real DOM, layout or CSS — so the red
  `FAILED` stamp is verified as a class name and text, **not as a rendered colour or
  contrast**; no events or navigation; no fetch/auth stack; timers silenced, so polling is
  judged by counted fetches and the arming condition. A **skip** in this harness would be
  non-coverage, not a pass — here both UI tests **executed** (2.49 s and 3.90 s).
- **Full suite reported, not rerun here**: 240 / 7 / 4. The seven failures are the known
  Windows Unicode compatibility cases; the deferred native tar writer is unchanged.
- **Windows race and CI: NOT TESTED / NOT RUN.**
- **`INTERRUPTED` + unrecorded is synthetic**; the current backend does not produce it.
- Unchanged by this pass: no cross-file atomicity between `catalog.json` and `jobs.json`;
  `syncDir`'s Windows no-op, so rename durability is not guaranteed on this platform;
  "recorded" means only the checked publication contract, not crash- or power-loss-safety;
  and completion never asserts that written content was read back and verified.

---

## 7. State at close — re-verified after the review

Branch `fix/ob-002-durable-completion`; HEAD `f98eedf381253aae9a94dcfcc80f6bab2aec9317`;
staging **empty**; no in-progress Git operation. `git status --porcelain` listed **27**
entries at the start of this review and **28** at its close — the pre-existing 27 unchanged
entry for entry, plus the single new `??` entry for this report. All six file blobs in §1
re-measured **unchanged**.

Every prior report is preserved byte-unchanged (SHA-256):
`…FOCUSED-RECHECK…` `26770558…`, `…UI-PRECEDENCE-CLOSEOUT…` `f8d359ac…`,
`…FRESH-REVIEW…` `18df0387…`, `…REVIEW-FOLLOWUP…` `253da4e9…`,
`…IMPLEMENTATION…` `6b734a3c…`. No issue status was altered on anyone's behalf.

**This report is the only file added.** No fix, commit, push, merge or next-task work was
performed.

*Targeted UI recheck, 7 September 2026 — fresh-session AI review, no session continuity with
the patch or closeout authors. Not owner review.*
