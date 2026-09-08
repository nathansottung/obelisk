# PR-03 / OB-002 — UI status-precedence correction, closeout

**Scope: the one remaining blocker from the focused recheck — nothing else.** The recheck
(`PR03-OB-002-FOCUSED-RECHECK-2026-09-07.md`) closed Blockers 1 and 3 and closed Blocker 2
for `COMPLETED`. Its §5 remainder was a regression the follow-up itself introduced: the
three job UI helpers branched on `unrecorded` **before** `status`, so a `FAILED` job whose
failure record also could not be written rendered as an amber `NOT RECORDED` stamp under a
sentence asserting the operation had finished and its results were real.

That is fixed here, in `ui/index.html` only, with the regression coverage the recheck said
was missing at both levels.

**This is an editing pass, not a review, and it carries no approval authority.** It is the
same session lineage that wrote the patch. Everything marked *executed* ran on this machine
today, 7 September 2026; everything else is labelled read, inferred, or NOT TESTED.

---

## 1. Candidate identity — before and after

| Item | Before this pass | After this pass |
|---|---|---|
| Branch | `fix/ob-002-durable-completion` | **unchanged** |
| HEAD | `f98eedf381253aae9a94dcfcc80f6bab2aec9317` | **unchanged** |
| Staging area | empty | **empty** |
| In-progress Git op | none | **none** |

All six file identities at the start of this pass were **byte-identical to the values the
focused recheck recorded at its close** — the candidate had not moved.

| File | Blob before | Blob after | Changed |
|---|---|---|---|
| `ui/index.html` | `1bac5a50d6907287e1a7d5543fe69619cb68b0c4` | `c3aae212746ecb9b6f94c081a1f87808bcc50a1a` | **yes** |
| `durable_completion_ui_test.go` | `b957492d1428abb719c250f2cfcf01942205c815` | `446df4babf1482bb918cebca1c665d0cd1f4f40a` | **yes** |
| `durable_completion_followup_test.go` | `77ab90b1d333c1a82d61e11daa37a987938e61b7` | `a6e8fa422d5d15a8ee5c02cf9205d79a0806d705` | **yes** |
| `store.go` | `8e8b1d1b89028f1eca2e2501382163ecf123bd59` | `8e8b1d1b89028f1eca2e2501382163ecf123bd59` | no |
| `main.go` | `05b5eec0b6253615bf5ca97b1b8f59b1b9ab5b18` | `05b5eec0b6253615bf5ca97b1b8f59b1b9ab5b18` | no |
| `durable_completion_test.go` | `8d43c743baa193d1ea61828f0cfadc6263c1f00d` | `8d43c743baa193d1ea61828f0cfadc6263c1f00d` | no |

**No Go production file was touched.** Re-hashed after the edits: `store.go`, `main.go`,
`adopt.go`, `dock.go`, `exports.go`, `incremental.go`, `mirror.go`, `pipeline.go`,
`plans.go`, `profiles.go` — all unchanged. No persistence behaviour, job enum, lock,
snapshot ownership or earlier safety fix was modified. The focused recheck and every
baseline/handoff document are byte-unchanged; this report is the only document added.

---

## 2. The actual diff

**This pass alone** changed three files:

| File | Delta | What |
|---|---|---|
| `ui/index.html` | **+56 / −11** lines | `jobStamp`, `jobStampText`, `jobRecordingNote`, `waitJob`'s `FAILED` branch |
| `durable_completion_ui_test.go` | 227 → **445** lines | new `uiPrecedenceAssertions` block, shared `runUIHarness`, second test |
| `durable_completion_followup_test.go` | 497 → **570** lines | `_K_FailedJobCanAlsoBeUnrecorded` (backend premise) |

**Whole PR-03 candidate against the base, measured now — these supersede the stale
`+756 / −109` and the recheck's `+853 / −109`, both of which predate this pass:**

| Slice | Figure |
|---|---|
| Tracked, vs `f98eedf3…` | **15 files, +900 / −110** |
| — source/UI (13 files) | **+712 / −110** |
| — status documents (2 files) | **+188 / −0** |
| `ui/index.html` alone | **+128 / −13** |
| Untracked tests | `durable_completion_test.go` 603, `durable_completion_followup_test.go` 570, `durable_completion_ui_test.go` 445 lines |

Do not carry these forward as a target either; they are a measurement of this moment.

---

## 3. What changed in the page

The rule now stated at the helpers and enforced by the tests: **the execution outcome is
primary and always survives; `unrecorded` is an additional qualification that never
overwrites it and never upgrades a failure into a completion.**

- **`jobStamp`** — reads `status` first. `FAILED` returns `FAILED`, `INTERRUPTED` returns
  `INTERRUPTED`, and only a genuine `COMPLETED` can be downgraded to the amber
  `UNRECORDED`. The failure stamp and its existing `.stamp.FAILED` red styling are
  untouched.
- **`jobStampText`** — substitutes `NOT RECORDED` **only** for `COMPLETED`. Every other
  status reads as itself.
- **`jobRecordingNote`** — the "the operation itself finished and its results are real"
  sentence is now reachable only for `COMPLETED`. A non-`COMPLETED` terminal job that was
  also not recorded gets its own sentence, outcome first:
  > **The job failed. Its failure record could not be saved.** The outcome above stands;
  > what is missing is the saved record of it, so the job board may not show this outcome
  > after a restart. It is not retried and the operation is not re-run: the next ordinary
  > job update saves the board and records it. *(cause)*

  It does not say the work finished, that any output exists, or that any result is usable.
  The recording cause stays visible, separate from the operation's own error.
- **`waitJob`'s `FAILED` branch** — already rejected on the ordinary failure path (it tests
  `status==='COMPLETED'` before `unrecorded`), so no failed job could ever reach a success
  toast, and that was not the regression. What was missing is the recording qualification:
  the rejection now appends " — this failure record could not be saved (*cause*)" and
  carries `recordUnsaved` and `job`. It deliberately does **not** set `unrecorded`, because
  `unrecorded` means *finished successfully but not saved* and routes callers
  (`dockIngest`, the recovery kit, `adoptDest`) into the show-the-results path — a failed
  job has no results to show.

Both surfaces that use these helpers — the jobs list (`vJobs`) and job detail
(`vJobDetail`) — get the correction, since both call the same three helpers. `ccRender`'s
consumer likewise. No status enum, no new backend contract, no framework, no redesign.

**`INTERRUPTED` + unrecorded is not manufactured.** `loadJobs` is the only producer of
`INTERRUPTED` and deliberately leaves those rows unflagged, so the combination is not
produced by the application. The helper branch for it is defensive only, is commented as
such in the page, and the test that supplies it is labelled `SYNTHETIC`.

---

## 4. Execution-status × recording-state matrix

Every row below was **executed** against the real `ui/index.html` script under node.
"Synthetic" marks the one combination the backend does not produce.

| status | `unrecorded` | `persist_error` | `jobStamp` | `jobStampText` | note | `waitJob` |
|---|---|---|---|---|---|---|
| `COMPLETED` | false | — | `VERIFIED` | `COMPLETED` | none | resolves |
| `COMPLETED` | **true** | set | `UNRECORDED` | `NOT RECORDED` | "Work finished — completion not recorded"; results stay reachable; says *not verified history* | rejects, `unrecorded=true`, carries job |
| `COMPLETED` | false | **set (history)** | `VERIFIED` | `COMPLETED` | "Recorded. An earlier attempt … failed" | resolves |
| `FAILED` | false | — | `FAILED` | `FAILED` | none | rejects with the operation error |
| **`FAILED`** | **true** | set | **`FAILED`** | **`FAILED`** | **"The job failed. Its failure record could not be saved."** | rejects with the operation error **+** the recording clause; `recordUnsaved=true`; `unrecorded` **not** set |
| `INTERRUPTED` | false | — | `INTERRUPTED` | `INTERRUPTED` | none | rejects; poll stops |
| `INTERRUPTED` | true *(synthetic)* | set | `INTERRUPTED` | `INTERRUPTED` | "The job was interrupted. Its record could not be saved." | — |

Completion is never rendered as a content-verification claim in any row.

---

## 5. Red / green evidence for `FAILED` + unrecorded

The new assertions were written **first** and run against the **pre-edit page** in a
disposable copy (`git hash-object` of the copy: `1bac5a50d6907287e1a7d5543fe69619cb68b0c4`
— byte-identical to the page before the edit). The working checkout was **not** reverted
for this comparison.

**RED — pre-edit page, 26 failures, all of them the reported regression:**

```
 - a FAILED job whose record was not saved is still a FAILED job - the recording
   state must not displace the execution outcome
 - the stamp must still read FAILED, not NOT RECORDED
 - the note for a FAILED job must not assert that the work finished or that its
   results are real; got: <b>Work finished — completion not recorded.</b>
   The operation itself finished and its results are real and still available here …
 - the Jobs list must render the FAILED stamp for a failure whose record was not saved
 - the word FAILED must appear in the composed jobs list
 - the job detail stamp must stay FAILED
 - the recording qualification must remain available on the rejection; got: source unreadable
 …
=> harness exit 1 (RED)
```

The `A` (ordinary `FAILED`), `C` (`COMPLETED` + unrecorded), `D` (recorded + history) and
`E` (`INTERRUPTED`) controls **passed on the pre-edit page**, so the red run isolates the
regression rather than a general failure of the block.

**GREEN — corrected page:** `PASS`, harness exit 0. The pre-existing
`uiAssertions` block also still passes against the corrected page, so the `COMPLETED`
behaviour the recheck accepted is intact.

One assertion of mine was wrong on the first green attempt and was corrected in the test,
not in the page: a regex for verification claims matched the note's own *denial* ("is not
verified history"), which is the wording that should be there. It now tests for affirmative
verification claims and separately asserts the denial is retained.

### Coverage added

`durable_completion_ui_test.go` — new `TestJobsUI_ExecutionOutcomeTakesPrecedence`, a
second assertion block through the **same** harness, extracting and running the real
`<script>` from `ui/index.html`. No test-only re-implementation of the helpers. Cases:

- **A** `FAILED` + `unrecorded:false` — stamp, text and empty note unchanged; `waitJob`
  rejects with its own error and no recording clause.
- **B** `FAILED` + `unrecorded:true` — stamp `FAILED`, text `FAILED`, no `stamp UNRECORDED`
  and no `stamp VERIFIED` in the **composed** `vJobs` and `vJobDetail` output; the original
  `— ERROR: source unreadable` still present in both; a separate recording warning present;
  the note asserts no completion and no usable results; `waitJob` rejects carrying both
  facts and not `unrecorded`; through `adoptDest`, no `Destination adopted` toast, every
  toast on the `bad` path, and the operation error reaches the user.
- **C** `COMPLETED` + `unrecorded:true` — the existing warning survives; its sentence
  differs from the failed one; no clean-success toast.
- **D** `COMPLETED` + recorded + historical `persist_error` — no `NOT RECORDED` anywhere in
  the composed list; `VERIFIED`; history line still present.
- **E** `INTERRUPTED` — `INTERRUPTED` stamp and text; **counted `/api/jobs` fetches** show
  the list does not re-poll after 1.4 s, and `waitJob` decides on the first poll (exactly
  one additional fetch — no retry, no re-run).

`durable_completion_followup_test.go` — new `TestDurableCompletion_K_FailedJobCanAlsoBe`
`Unrecorded`, the backend premise the recheck also asked for: a failing-shaped
`FinishJob(…, "FAILED", nil, nil)` with the sidecar seam armed yields `status=FAILED`,
`unrecorded=true`, the operation error in the label and the recording cause in
`persist_error` — **the two causes kept separate**, no artifacts, no result — and both
facts survive the real `/api/jobs` handler chain and JSON encoder.

---

## 6. Commands and results — executed today

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `node v24.14.1` (pre-existing). Existing
toolchain, real system temp, **no `GOTMPDIR` override**, nothing installed, no global
setting changed, no platform substituted, no real catalog / key / backup / hardware
touched. Red/green work happened in a disposable copy under the session scratchpad; the
working checkout was never reverted.

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **PASS**, exit 0 |
| Vet | `go vet ./...` | **PASS**, exit 0 |
| Format | `gofmt -l .` | clean apart from the two **pre-existing frozen** `docs/…/reproducers/semantics_test.go` files |
| OB-002 + both UI tests | `go test -count=1 -v -run '^(TestJobsUI_\|TestDurableCompletion_)' .` | **20/20 PASS**, exit 0 |
| Frontend harness actually ran | same, `-v` | `TestJobsUI_HonoursRecordingState` **PASS (2.52s)**, `TestJobsUI_ExecutionOutcomeTakesPrecedence` **PASS (3.90s)** — executed, not skipped |
| Prior safety set | the recheck's `^(…OpenStore…\|Schema…\|AppBackup…\|AtomicRename…\|Export…\|Restore…\|Containment…\|SeeingWhatHappened…)$` pattern | **31/31 PASS**, exit 0 |
| RED, pre-edit page | disposable copy + new assertions | **exit 1**, 26 failures, all the reported regression |
| GREEN, corrected page | same assertions | **PASS**, exit 0 |
| Existing UI block vs corrected page | `uiAssertions` | **PASS**, exit 0 |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 240 pass / 7 fail / 4 skip**, 19 sub-pass / 5 sub-fail |
| Windows race | `go test -race` | **NOT TESTED** — requires cgo; `CGO_ENABLED=0`, no gcc on PATH |
| CI | — | **NOT RUN. No CI evidence is claimed.** |

**Correction to the recheck's own test-command row.** The recheck reported that pattern as
"OB-002 + UI + prior safety, 31/31". Executed here, its trailing `$` makes the
`TestDurableCompletion_` and `TestJobsUI_` alternatives unmatchable, so the 31 are the
**prior-safety tests only**; the OB-002 and UI tests were covered by its separate
`-run '^TestJobsUI_'` row. The 31 is real, its label was too broad. The unanchored pattern
in the row above is what actually runs the OB-002 and UI tests.

**Suite delta, measured not assumed.** Previously reported **238 / 7 / 4**; now
**240 / 7 / 4**. The **+2** is exactly the two tests added here. The seven failures are the
**same Windows Unicode compatibility cases by identity** —
`TestBuildRestore_HostileFilenamesRoundTrip`, `TestBuildFilelist_IsNulDelimited`,
`TestBuildChunk_UnicodeFilenames_ExactMembers`,
`TestBuildChunk_WrongFileSelection_LookalikeNeighbour`, `TestBuildChunk_UnicodeSourceRoot`,
`TestBuildChunk_UnicodeStagingDir`, `TestBuildRestore_UnicodeRoundTrip` — none weakened,
none skipped, native tar writer still not implemented. Four skips unchanged. **No new
failure and no new skip.**

---

## 7. Remaining limitations

**Of this fix.** The node harness exercises logic and emitted HTML only. It is **not**
browser-rendering validation: no real DOM, layout or CSS (so the amber-vs-red stamp is
verified as a class name and text, not as a rendered colour); no events or navigation; no
fetch/auth stack; no timer firing (polling is judged by fetch counts and the arming
condition). The `FAILED` + unrecorded wording is asserted for what it must **not** claim
and for its key phrases; it has not been read by a user.

**Deliberately not addressed here, and still the recheck's documented nonblockers.**
Nested `Result` values and `Result["artifacts"]` still alias stored state, and input
ownership is still undefended. Neither was fixed and neither is claimed fixed; general deep
immutability is **not** established. They remain optional improvements with a concrete type
but no reachable mutator today.

**Pre-existing, unchanged by this pass.** Seven Windows Unicode failures; no Windows race
evidence (no cgo/gcc); no CI; deferred native tar writer; no cross-file atomicity between
`catalog.json` and `jobs.json`; `syncDir`'s Windows no-op; `FinishJob` returning `nil` for
an unknown ID; `save()` not setting `dirty` on a failed unbatched write; OB-007 live-pointer
ownership elsewhere in the Store. "Recorded" still means only the checked publication
contract — not crash-safe or power-loss-safe — and job completion still never asserts that
written content was read back and verified.

---

## 8. State at close

Branch `fix/ob-002-durable-completion`, HEAD `f98eedf381253aae9a94dcfcc80f6bab2aec9317`,
staging **empty**, no in-progress Git operation. Nothing was fetched, pulled, switched,
reset, stashed, cleaned, restored, staged, committed, pushed or merged, and no next
workstream was started. No issue status was updated on anyone's behalf.

**READY_FOR_TARGETED_RECHECK** for this specific regression — the `FAILED` + unrecorded
status precedence in `jobStamp`, `jobStampText`, `jobRecordingNote` and `waitJob`. That is a
statement that the reported defect is fixed and covered, **not** independent approval of
PR-03.

*UI status-precedence closeout, 7 September 2026 — same session lineage as the patch. Not
an independent review.*

---

## Erratum — 7 September 2026, owner-acceptance publication pass

**Append-only.** Everything above is unmodified. This section corrects this document's own
bookkeeping and nothing else. The pass that added it changed **no** executable code, source
comment, test, UI logic, dependency or toolchain setting, and rewrote **no** review document
— not the fresh review, not the follow-up, not the focused recheck, not the targeted
recheck, not BASELINE.

### A. §1's and §5's pre-edit hashes did not preserve the corresponding blobs

The "before" column in §1 and the `1bac5a50d6907287e1a7d5543fe69619cb68b0c4` cited in §5
were produced by `git hash-object` **without `-w`**, on working-tree files. They were
therefore never written to the object database. Re-checked at publication, `git cat-file -e`
reports all three **ABSENT**:

- `1bac5a50d6907287e1a7d5543fe69619cb68b0c4` (`ui/index.html`, pre-edit) — ABSENT
- `b957492d1428abb719c250f2cfcf01942205c815` (`durable_completion_ui_test.go`, pre-edit) — ABSENT
- `77ab90b1d333c1a82d61e11daa37a987938e61b7` (`durable_completion_followup_test.go`, pre-edit) — ABSENT

They remain valid **records of what was measured at the time**; they are not retrievable
artifacts, and no attempt was made to reconstruct them.

Two consequences, both preserved rather than papered over:

1. **§5's RED run — 26 failures against the pre-edit page — is author-reported evidence
   only.** It was not independently reproduced and, the pre-edit blob being absent, cannot
   now be reproduced byte-identically.
2. **The later targeted recheck did not replay it.**
   `PR03-OB-002-UI-TARGETED-RECHECK-2026-09-07.md` §4 independently executed a **bounded
   mutation test**: a disposable copy of the *current* page in which only the three helpers
   were reverted to the documented defect shape, driven by the harness's own
   `uiPrelude` / `uiPrecedenceAssertions` extracted verbatim — **RED exit 1, 22 failures;
   GREEN exit 0**. That is a mutation experiment, **not a byte-identical replay** of the run
   recorded here. Its 22 failures and this document's 26 are counts from two different
   artifacts and are **not** comparable; neither figure supersedes or validates the other.

### B. §1's two post-edit test blobs are transposed — corrected

§1 assigns `446df4ba…` to `durable_completion_ui_test.go` and `a6e8fa42…` to
`durable_completion_followup_test.go`. Measured at publication with `git hash-object`, and
as staged in the implementation commit, the assignment is the reverse. The corrected full
mappings:

| File | Pre-edit blob (recorded, now absent) | Post-edit blob (measured) | Lines |
|---|---|---|---|
| `durable_completion_ui_test.go` | `b957492d1428abb719c250f2cfcf01942205c815` | `a6e8fa422d5d15a8ee5c02cf9205d79a0806d705` | 227 → **445** |
| `durable_completion_followup_test.go` | `77ab90b1d333c1a82d61e11daa37a987938e61b7` | `446df4babf1482bb918cebca1c665d0cd1f4f40a` | 497 → **570** |

The blob **set** in §1 was correct; only the two row assignments were swapped. §2's line
counts already corroborated the corrected mapping (the 445-line file hashes to
`a6e8fa42…`), as the targeted recheck observed. The **pre-edit** column is *not* transposed:
it agrees with the focused recheck's §1 table, which recorded `b957492d…` at 227 lines and
`77ab90b1…` at 497 lines. Because those two blobs are absent (A), the pre-edit column rests
on that line-count agreement and **cannot be re-measured**. No full hash here was inferred
or expanded from an abbreviated value.

**No code implication.** `ui/index.html`'s two rows are unaffected, and every other §1 row
was re-measured unchanged.

### C. Which files have historical byte-identity, and which have only current identities

This distinction is preserved exactly as the targeted recheck drew it, with one verified
addition, and is **not** collapsed into a general claim.

**Historical byte-identity established** — current contents are byte-identical to a blob
recorded by an earlier document, re-measured at publication:

| File | Blob | Established against |
|---|---|---|
| `store.go` | `8e8b1d1b89028f1eca2e2501382163ecf123bd59` | focused recheck §1; unchanged row of §1 above |
| `main.go` | `05b5eec0b6253615bf5ca97b1b8f59b1b9ab5b18` | focused recheck §1; unchanged row of §1 above |
| `durable_completion_test.go` | `8d43c743baa193d1ea61828f0cfadc6263c1f00d` | focused recheck §1; unchanged row of §1 above |
| `ui/index.html` | `c3aae212746ecb9b6f94c081a1f87808bcc50a1a` | §1 post-edit (this document) and targeted recheck §1 |
| `durable_completion_ui_test.go` | `a6e8fa422d5d15a8ee5c02cf9205d79a0806d705` | targeted recheck §1 (assignment corrected in B) |
| `durable_completion_followup_test.go` | `446df4babf1482bb918cebca1c665d0cd1f4f40a` | targeted recheck §1 (assignment corrected in B) |
| `appbackup_test.go` | `23106924955faa52c67b4153baebc2f3e17d1d7b` | implementation report §1 |
| `seeing_what_happened_test.go` | `993cf52fe71aa274b0bebb4a13c2f735ee6bfbc9` | implementation report §1 |

**Verified addition, recorded at publication, not claimed by any reviewer.** The targeted
recheck's §1 limitation — that byte-identity was *not* established for the other eight
modified Go files, because neither the focused recheck nor this closeout published blobs for
them — is accurate as written about those two documents. Re-measured at publication, however,
all eight match the git blobs the **implementation report's §1 table** recorded, so
byte-identity **back to the implementation-pass candidate** is established for them:

`adopt.go e35f16ad3ad405d5ef7163c296ad72de6e5de3fe`,
`dock.go 53d5603e33ed1003dc77ddd2907b45401184352b`,
`exports.go 155f2a6d247b3a28c2d39cf6bd9406483317cb4b`,
`incremental.go a230e9d6c5ae32a6cc949dc65338bda45b9edae1`,
`mirror.go 177834d0a33786daa4111b63e7b9b2ebfe86049e`,
`pipeline.go 8ff679fefdd9af0928a154a234b465c872d4d4ae`,
`plans.go aa90932342308cc0f519ae2fb0d22159b2f48d16`,
`profiles.go fa73d70f0278892821f81381653b8c5aef35dccd`.

This is an owner-side measurement against an existing published table. It does **not**
retroactively give the focused recheck or this closeout evidence they did not record, it does
**not** establish anything about the *intermediate* states of those files between the
implementation pass and now, and it is **not** a repository-wide audit.

### D. Diff figures at publication

§2's `+900 / −110` was, as §2 itself warned, a measurement of its moment. Measured against
`f98eedf381253aae9a94dcfcc80f6bab2aec9317` at publication: tracked **15 files, +963 / −110**
— source/UI **13 files, +712 / −110** (unchanged from §2, and from the targeted recheck) and
status documents **+251 / −0** (§2 recorded +188; the two documents grew afterwards).
`ui/index.html` alone remains **+128 / −13**. No implementation file is implicated by the
difference.

*Erratum appended during the owner-acceptance publication pass, 7 September 2026. It
corrects bookkeeping only and confers no additional approval.*
