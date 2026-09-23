# PR-01 / OB-001 — independent review

**Outcome: NEEDS_CHANGES**

The required changes are **documentation and test-completeness only**. The production decision
logic is correct and I would not change a line of it. `READY_FOR_OWNER_REVIEW` is withheld solely
because three factual claims that ship *inside the patch itself* (two source comments and one
report table) are wrong, and correcting them is cheap. This outcome is not a rejection of the fix,
and in either case it is not permission to commit, merge, or a production-readiness claim.

Review-only. No source or test file in the working checkout was created, edited, staged, committed
or pushed. All probing was done in disposable copies under the session scratchpad.

---

## 1. Reviewed identity

Verified before anything else, and re-verified at the end (section 9).

| Property | Observed | Matches author's report |
|---|---|---|
| Git root | `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go` | ✓ |
| Branch | `fix/ob-001-catalog-open` | ✓ |
| `HEAD` | `d97809b2f730e09960632e2943e562531fa095a3` | ✓ equals reported base; nothing committed |
| Staged content | none (`git diff --cached` empty) | ✓ |
| In-progress git operation | none (`MERGE_HEAD`, `REBASE_HEAD`, `rebase-merge`, `rebase-apply`, `CHERRY_PICK_HEAD`, `BISECT_LOG`, `REVERT_HEAD`, `SEQUENCER` all absent) | ✓ |
| `store.go` | modified, **+80 / −3** | ✓ exactly as reported |
| `catalog_open_test.go` | new, untracked, **509 lines** | ✗ **reported as 431** — see F-5 |

Exact contents reviewed:

| File | git blob (`hash-object`) | SHA-256 |
|---|---|---|
| `store.go` (working tree, patched) | `04f0b5051a83a43aa5d9a8c1611884497e484808` | `8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56` |
| `catalog_open_test.go` (untracked) | `f873ef27ad30bc38fc55f36ebe81234a4e10752a` | `69972b5d7f1e00cd84b31ad2abbccdccc3a23e98fd51f5b840c35a4f207d200c` |
| `store.go` at baseline `HEAD:store.go` | `00d47561962b4c59f3c4c9fbc8245da726675ec0` | — |

`catalog_open_test.go` is untracked and therefore absent from `git diff`; it was read in full
directly from disk (509 lines), not through a diff.

**No material divergence from the reported checkpoint.** The one discrepancy — the 431 vs 509 line
count — is a miscount in the report's metadata table, not a different artifact: the report's own
red-run evidence cites `catalog_open_test.go:306` and `:359`, and in the 509-line file on disk line
306 is exactly `t.Fatal("openStore must fail closed when the catalog cannot be read; it returned no
error")` and line 359 is exactly `t.Errorf("the error should say the file is empty, got %q", err)`.
The evidence in the report is about this file.

### Extraction problem — resolved

`docs/development/PR01_REVIEW_ADDENDUM/` does indeed contain the wrong bundle: it is a byte-level
duplicate of the implementation handoff (`00_START_HERE.md`, `backlog/`, `design/`,
`source-materials/`, …), not a two-document addendum. Its contents were **not** used and **not**
modified.

The real archive at `docs/OBELISK_PR00_REVIEW_ADDENDUM_2026-09-06.zip` (11 019 bytes) was extracted
to a disposable scratch directory. It holds exactly three entries and both manifest hashes validate:

| Entry | Manifest SHA-256 | Computed | Bytes |
|---|---|---|---|
| `OBELISK_PR00_REVIEW_ADDENDUM_2026-09-06.md` | `465f3af5…5ea3db` | `465f3af5…5ea3db` ✓ | 15 855 ✓ |
| `OBELISK_PR01_CORRECTED_PROMPT_2026-09-06.md` | `8153653b…6e11469` | `8153653b…6e11469` ✓ | 8 156 ✓ |
| `MANIFEST.json` | (not self-listed) | `4c91dfa8…293d37` | 379 |

This review uses the **corrected prompt**, not the regression test proposed in `NEXT_ACTIONS.md`.
The misfiled `PR01_REVIEW_ADDENDUM/` directory was left exactly as found; repairing it is not part
of this review, and is reported here rather than fixed.

---

## 2. Verdict on the central question

> Does this bounded patch correctly prevent a catalog read failure from falling through into
> new-catalog initialization and overwriting existing catalog/backup state?

**Yes.** Verified by execution against the real baseline blob, not by reading the author's report.

**The defect is real at baseline.** In a disposable copy of the package with `store.go` restored
from `HEAD:store.go` (`00d47561`), placing a directory where `catalog.json` belongs produces a
genuine non-not-found read error on Windows, and:

```
BASELINE read error is: read ...\catalog.json: Incorrect function.
BASELINE unreadable: store!=nil=true err=<nil>
BASELINE unreadable: profiles seeded in memory = 3
BASELINE unreadable: collections = 0
2026/09/06 21:21:12 WARNING: startup recovery could not be persisted (rename ...catalog.json.tmp
  ...catalog.json: Access is denied.) — recovered state may not survive another restart
```

Baseline `OpenStore` returned a **usable Store and a nil error** on an unreadable catalog, with an
empty three-profile catalog seeded in memory and a publication actually attempted. That is the
fall-through, confirmed independently.

**The patch closes it.** With the working-tree `store.go`, the same shape is refused:

```
rejected open: store!=nil=false err=cannot read ...\catalog.json: open ...\catalog.json:
  permission denied — refusing to start so an unreadable catalog is never replaced by a new
  empty one. Check the file's permissions, and that the drive holding the data folder is connected
before entries: [. catalog.json catalog.json.bak-20260906]
after  entries: [. catalog.json catalog.json.bak-20260906]
```

The before/after listing is a **full `filepath.Walk` of the whole data directory**, written by me,
independent of the patch's own assertions — stronger evidence than the shipped tests provide
(see F-4).

### Section 3 checklist from the review brief

| Requirement | Verdict | Anchor / evidence |
|---|---|---|
| A read error other than not-found cannot reach seeding, recovery saves, catalog replacement, daily-backup creation, or backup pruning | **Confirmed.** The `default` arm returns at [store.go:1183](../../../store.go#L1183), before the schema gate (1191), the recovery loops (1207+), seeding (1258, 1266), the recovery `save()` (1275) and `loadJobs()` (1283) | [store.go:1157-1187](../../../store.go#L1157-L1187) |
| Rejected startup returns an error and no usable Store | **Confirmed.** All three refuse arms `return nil, …` | 1164, 1172, 1183 |
| Error wrapping preserves underlying error identity | **Confirmed.** `%w` on `readErr`; `errors.Is(err, fs.ErrPermission)` passes in test | [store.go:1183](../../../store.go#L1183), [catalog_open_test.go:311](../../../catalog_open_test.go#L311) |
| Public `OpenStore` uses the same decision branch as the injected tests | **Confirmed by construction** — `OpenStore` is a one-line delegate with no logic of its own | [store.go:1119-1121](../../../store.go#L1119-L1121). See O-3 for making this evidence rather than inspection |
| Partial bytes alongside a read error are not accepted | **Confirmed.** `b` is simply not read in the `default` arm; covered by the `partial_bytes_with_an_error` subtest | [store.go:1180-1187](../../../store.go#L1180-L1187), [catalog_open_test.go:286-293](../../../catalog_open_test.go#L286-L293) |
| Malformed-input and schema behavior remain compatible | **Confirmed.** Invalid JSON keeps the identical `"catalog.json is damaged: %w"` message; the newer-schema read-only latch is untouched. `TestCatalogCurrentRoundTrip`, `TestCatalogMigrateV1ToV2`, `TestCatalogLegacyMigratesAndBacksUp`, `TestCatalogNewerSchemaIsReadOnly` all pass unmodified | [store.go:1170-1172](../../../store.go#L1170-L1172), section 6 |
| Legitimate fresh-directory initialization still works | **Confirmed**, through both `openStore` and exported `OpenStore` | [catalog_open_test.go:442-462](../../../catalog_open_test.go#L442-L462) |
| Existing recovery-save behavior unchanged | **Confirmed.** The `if recovered { if err := s.save(); … log … }` block is byte-identical to baseline; `TestOpenStore_RecoverySaveFailureIsNonFatal` passes unmodified | [store.go:1271-1279](../../../store.go#L1271-L1279), [persistence_test.go:47](../../../persistence_test.go#L47) |

The established recovery-save policy was **not** changed by the patch and is **not** questioned by
this review.

---

## 3. Findings, by severity

### Blocking

**None.** No finding here justifies withholding the production change.

### F-1 — Medium. The corrected zero-length claim was fixed in the report but not in the shipped test comments

**Anchors:** [catalog_open_test.go:10-11](../../../catalog_open_test.go#L10-L11),
[catalog_open_test.go:334-336](../../../catalog_open_test.go#L334-L336)

The author's correction is **right**, and I verified it independently against baseline blob
`00d47561`:

```
BASELINE zero-length: store!=nil=false err=catalog.json is damaged: unexpected end of JSON input
BASELINE zero-length: catalog is now 0 bytes
BASELINE zero-length: baks before=[...catalog.json.bak-20260906] after=[...catalog.json.bak-20260906]
```

A zero-length catalog already failed closed, left the file at 0 bytes, and created no backup —
because `json.Unmarshal` sits *inside* the baseline `err == nil` block and rejects empty input
before `existed` is ever consulted. `existed = len(b) > 0` was a latent hazard, never a live defect.

But the correction never reached the code. The test file still asserts the disproved claim twice:

- line 10-11: *"A zero-length catalog.json (the torn-write artifact) took the same path"* — it did not.
- line 334-336: *"An empty catalog.json used to satisfy `len(b) > 0 == false` and take the brand-new path, **which skips the schema gate and the pre-migration backup and then overwrites the file**"* — it never overwrote the file.

**Failure scenario:** a maintainer touching this area in six months reads `catalog_open_test.go`
(not `reviews/PR01-…md`), believes the zero-length branch is load-bearing against catalog
destruction, and either over-weights it during a refactor or, on finding the rejection
inconvenient, is unable to judge that relaxing it costs only diagnostics. The report's §1 correction
is invisible from the code.

Related: my red run shows `TestOpenStore_ZeroLengthCatalogIsDamagedNotNew` fails against the old
logic **only** on `strings.Contains(err.Error(), "empty")` — it is a **message-wording pin, not a
behavioral regression test**. That should be said in the test.

**Required correction:** reword both comments to state that baseline already failed closed via
`json.Unmarshal`, and that PR-01's contribution here is an earlier, explicit rejection with an
actionable message — not the prevention of an overwrite.

### F-2 — Medium. The observer's stated boundary gives a wrong reason for excluding `saveJobs`

**Anchors:** [store.go:1086-1088](../../../store.go#L1086-L1088) (source comment); PR-01 report §2.3
("Not covered, and why")

Both say the observer omits `saveJobs` because *"`OpenStore` only ever **reads** the sidecar, via
`loadJobs`, so no jobs write is reachable in the startup path."*

That is false. [`loadJobs`](../../../store.go#L3320) calls
[`saveJobs`](../../../store.go#L3355) whenever it reconciles a job left `RUNNING` to `INTERRUPTED`:

```go
	if changed {
		s.saveJobs()
	}
```

`saveJobs` writes `jobs.json.tmp` and renames it ([store.go:3362-3363](../../../store.go#L3362-L3363)).
So a *successful* startup after an unclean shutdown **does** write the sidecar, uninstrumented.

**The conclusion for OB-001 nevertheless holds**, and I checked it rather than assuming it:
`s.loadJobs()` is at [store.go:1283](../../../store.go#L1283), the last statement before
`return s, nil`, and every refuse arm returns at 1164/1172/1183. A **rejected** open cannot reach
`loadJobs`, so no jobs write is reachable on the path the "zero attempts" claim covers.

**Failure scenario:** the observer is reused for a later issue — OBX-004 explicitly targets the same
fail-open shape in `loadJobs` — and the reviewer of *that* change trusts this comment, concludes the
jobs sidecar is read-only during startup, and writes a "zero attempted writes" assertion that is
silently blind to the very write OBX-004 is about.

**Required correction:** restate the exclusion by *reachability*, not by a false claim about what
`OpenStore` does: `saveJobs` is uninstrumented and **is** reachable on a successful open via
`loadJobs`, but is unreachable on a rejected open because `loadJobs` runs only after the
classification has already succeeded. Fix the source comment and report §2.3 together.

### F-3 — Low/Medium. One of the four instrumented paths has no positive control

**Anchors:** [catalog_open_test.go:208-252](../../../catalog_open_test.go#L208-L252) (the control),
[store.go:1483](../../../store.go#L1483) (`backup-prune`)

`TestPersistObserver_SeesRealMutations` demonstrates the observer fires for `catalog-write`,
`backup-create` and `pre-schema-backup`. It never exercises `backup-prune`, which needs more than 14
`.bak-*` sidecars to fire. The report §2.3 tabulates **four** covered paths; **three** are
demonstrated wired.

This does not weaken claim A — pruning is reachable only from `dailyBackup`, itself reachable only
from `writeCatalog`, which a rejected open never enters. It is a completeness gap between the
report's claim and its evidence, and exactly the class the corrected prompt's item F warns about
("Zero counts from an unwired spy prove nothing").

**Required correction (either is acceptable):** add a subtest that seeds 15 `catalog.json.bak-*`
files in a temp dir and asserts a `backup-prune` op is recorded; **or** change the report and the
`store.go:1082-1088` comment to say three paths are control-verified and `backup-prune` is
instrumented but not covered by the control.

### F-4 — Low. The on-disk assertion is narrower than the property it backs

**Anchor:** [catalog_open_test.go:119-138](../../../catalog_open_test.go#L119-L138)
(`requireUnchanged`), globbing set at [:101](../../../catalog_open_test.go#L101)

`requireUnchanged` compares `catalog.json` plus `.bak-*` and `.pre-schema-*` matches. It would not
notice a stray `catalog.json.tmp`, a newly created `jobs.json`, or any other new entry — so the
"independent protection evidence" the corrected prompt asks for (item C) is confined to the paths
the author already thought to instrument. The header comment at
[:22-26](../../../catalog_open_test.go#L22-L26) meanwhile says the tests prove "a refused startup
attempted **no write at all**", which reaches past what this assertion measures.

The patch is fine today — I confirmed with a whole-directory `filepath.Walk` before/after that
membership is byte-identical across a rejected open. But the shipped test does not.

**Suggested (not required):** replace the two globs with a full `filepath.Walk` snapshot. It is a
few lines, costs nothing, and upgrades the file-level evidence from "the paths we enumerated" to
"anything at all" — which is precisely the gap between claim A and claim B (section 4).

### F-5 — Low. Report metadata line count is wrong

**Anchor:** PR-01 report, changed-files table

`catalog_open_test.go` is 509 lines (`wc -l`), not 431. The report's own cited line anchors match
the 509-line file, so this is a miscount, not a stale artifact. Correct the table so the deliverable
is identifiable by size as well as by hash.

### F-6 — Informational. The residual is correctly recorded, and I reproduced it

Covered in section 5.

---

## 4. Claim A vs claim B — exactly what is and is not observed

I enumerated **every** filesystem-mutating call in `store.go` rather than trusting the report's
table:

| Site | Operation | Instrumented? | Reachable on a **rejected** open? |
|---|---|---|---|
| [store.go:1133](../../../store.go#L1133) | `os.MkdirAll(dataDir, 0o755)` | **No** | **Yes** — it runs *before* the read |
| [store.go:1346](../../../store.go#L1346) | `os.WriteFile` (pre-schema backup) | Yes — `pre-schema-backup` @1345 | No (requires `existed`) |
| [store.go:1409](../../../store.go#L1409), [:1424](../../../store.go#L1424) | `os.OpenFile` + `os.Rename` (catalog publication) | Yes — `catalog-write` @1372, first statement of `writeCatalog` | No |
| [store.go:1478](../../../store.go#L1478) | `os.WriteFile` (daily backup) | Yes — `backup-create` @1477 | No |
| [store.go:1484](../../../store.go#L1484) | `os.Remove` (backup prune) | Yes — `backup-prune` @1483 | No |
| [store.go:3362-3363](../../../store.go#L3362-L3363) | `os.WriteFile` + `os.Rename` (`saveJobs`) | **No** | No — `loadJobs` @1283 runs only after a successful classification |

Nine mutation sites, four instrumented, two uninstrumented. Migration functions are pure
(`Fn: func(c *catalog)`, no I/O) and `store.go` starts **no goroutines**, so nothing can mutate
asynchronously behind the observer or pollute the shared spy.

- **Claim A — "no catalog or backup mutation was attempted after a rejected read": ESTABLISHED.**
  Every catalog/backup authority write in `store.go` is instrumented; the observer reports before
  any gate that could hide the attempt; and I independently confirmed whole-directory membership is
  unchanged.
- **Claim B — "no filesystem mutation of any kind was attempted during `OpenStore`": FALSE**, and
  the report does not make it. `os.MkdirAll` at [store.go:1133](../../../store.go#L1133) mutates the
  filesystem before the read. The report's §2.3 wording — "zero application-initiated authority
  mutations in the traced startup path" — is correctly scoped and should be kept. Only the test-file
  header at [:22-26](../../../catalog_open_test.go#L22-L26) drifts toward B ("no write at all"); see F-4.

**No unprotected mutation path exists on the rejected-open path** other than the `MkdirAll` that
precedes the read by design. The documented observation boundary is sufficient for this bounded fix.
**I do not recommend introducing a general filesystem abstraction** to widen it — fix the two
inaccurate sentences (F-2, F-3) and, optionally, take the whole-directory snapshot (F-4).

---

## 5. Preserved scope corrections

### The zero-length correction holds — do not credit PR-01 with a fall-through fix

Checked against baseline source and by execution (section 3, F-1). Baseline rejected a zero-length
catalog via `json.Unmarshal`, left it at 0 bytes and wrote no backup. **PR-01 delivers an earlier
and clearer error here, which is a different change from preventing initialization.** The red run
confirms the shipped test fails at baseline *only* on the message assertion.

### The initialization-identity residual remains open — reproduced, not merely accepted

`fs.ErrNotExist` does not establish intentional first use, and `os.MkdirAll(dataDir, 0o755)` at
[store.go:1133](../../../store.go#L1133) runs **before** the read. I reproduced the consequence
against the patched tree:

```
OpenStore on an absent (unmounted-like) root: store!=nil=true err=<nil>
RESIDUAL CONFIRMED: MkdirAll created ...\NotMounted\obelisk-data during OpenStore
RESIDUAL CONFIRMED: a brand-new catalog.json (3259 bytes) was written there
dir now contains: [. catalog.json catalog.json.bak-20260906]
```

Pointing the app at a data directory whose storage is not mounted still creates the directory tree,
classifies the catalog as absent, and initializes a brand-new catalog plus a daily backup — with no
error. **This patch does not address it and does not claim to.**

The precise residual, stated without expanding PR-01 into a storage-identity redesign:

> `OpenStore` cannot distinguish a genuinely first-use data directory from a previously initialized
> one whose storage is absent, because (a) `MkdirAll` manufactures the directory before the read and
> (b) `fs.ErrNotExist` is accepted as sufficient evidence of first use. Resolving it requires
> persisted storage identity — out of scope for PR-01.

This is the unfinished half of OB-001's own original text ("Preserve path/mount identity checks; do
not auto-create over an uncertain path"). **The broader initialization/identity guarantee must not
be marked resolved.** Only the read-failure branch is delivered. I recommend splitting the residual
into a named successor (e.g. OB-001b) so PR-01 can close cleanly without the register implying the
whole guarantee shipped. I made **no** status changes to `OB_STATUS.md` or `NEXT_ACTIONS.md`.

---

## 6. Tests actually run

Everything below was executed by me on this checkout, against disposable `t.TempDir()` fixtures and
synthetic data only. No production catalog, real keystore, NAS original, backup destination, tape or
optical medium was touched. No tool was installed, the Go baseline was unchanged, and WSL was not used.

Environment: `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, `gcc` **not found**, module `go 1.22.2`.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `go build ./...` | **0** | PASS |
| 2 | `go vet ./...` | **0** | PASS |
| 3 | `gofmt -l $(git ls-files '*.go') catalog_open_test.go` | **0** | PASS — no file flagged, new untracked test included |
| 4 | `go test -count=1 -run 'TestOpenStore_\|TestPersistObserver_\|TestCatalog\|TestWriteCatalog_' -v .` | **0** | PASS (see below) |
| 5 | `go test ./... -count=1 -v` (full, uncached) | **1** | **203 pass / 2 fail / 4 skip**, 74.6 s |
| 6 | Red run: `go test -count=1 -run 'TestOpenStore_\|TestPersistObserver_' -v .` in a disposable copy with the decision logic reverted, seam retained | **1** | Fails for the intended reasons (below) |
| 7 | Baseline probes vs `HEAD:store.go` (`00d47561`) in a disposable copy | 0 | Reproduced the defect and the zero-length correction |
| 8 | Claim A/B + residual probes vs the patched tree in a disposable copy | 0 | Reproduced whole-directory invariance and the `MkdirAll` residual |
| — | `go test -race` | — | **NOT TESTED**, unchanged. `CGO_ENABLED=0` and no `gcc`. Not a pass |

### Run 4 — targeted, green

```
--- PASS: TestPersistObserver_SeesRealMutations
--- PASS: TestOpenStore_UnreadableCatalogFailsClosed
    --- PASS: .../permission_denied  .../io_error  .../partial_bytes_with_an_error
--- PASS: TestOpenStore_ZeroLengthCatalogIsDamagedNotNew
--- PASS: TestOpenStore_InvalidJSONStillFailsWithDamagedMessage
--- PASS: TestOpenStore_NewerSchemaStillOpensReadOnly
--- PASS: TestOpenStore_FreshDirectoryStillInitializes
--- PASS: TestOpenStore_NotExistShapesAllInitialize
    --- PASS: .../bare_sentinel  .../PathError  .../wrapped
--- PASS: TestOpenStore_ExistingCatalogReopensIntact
--- PASS: TestWriteCatalog_SaveFailurePropagates          [pre-existing, unmodified]
--- PASS: TestOpenStore_RecoverySaveFailureIsNonFatal     [pre-existing, unmodified]
--- PASS: TestCatalogCurrentRoundTrip                     [pre-existing, unmodified]
--- PASS: TestCatalogMigrateV1ToV2                        [pre-existing, unmodified]
--- PASS: TestCatalogLegacyMigratesAndBacksUp             [pre-existing, unmodified]
--- PASS: TestCatalogNewerSchemaIsReadOnly                [pre-existing, unmodified]
```

### Run 6 — red run, independently reconstructed

I did not take the author's red run on trust. I built a separate disposable copy, kept the seam
verbatim, and reverted **only** the classification to the baseline `if b, err := readFile(s.path);
err == nil { existed = len(b) > 0; … }`. Result:

```
--- PASS: TestPersistObserver_SeesRealMutations                       <- control still passes
--- FAIL: TestOpenStore_UnreadableCatalogFailsClosed
    --- FAIL: .../permission_denied            catalog_open_test.go:306
    --- FAIL: .../io_error                     catalog_open_test.go:306
    --- FAIL: .../partial_bytes_with_an_error  catalog_open_test.go:306
        "openStore must fail closed when the catalog cannot be read; it returned no error"
--- FAIL: TestOpenStore_ZeroLengthCatalogIsDamagedNotNew
    catalog_open_test.go:359: the error should say the file is empty, got
    "catalog.json is damaged: unexpected end of JSON input"
--- PASS: TestOpenStore_InvalidJSONStillFailsWithDamagedMessage       <- preservation test
--- PASS: TestOpenStore_NewerSchemaStillOpensReadOnly                 <- preservation test
--- PASS: TestOpenStore_FreshDirectoryStillInitializes                <- not failing indiscriminately
--- PASS: TestOpenStore_NotExistShapesAllInitialize
--- PASS: TestOpenStore_ExistingCatalogReopensIntact
--- PASS: TestOpenStore_RecoverySaveFailureIsNonFatal
```

This matches the author's reported red run **exactly**, including line numbers. The evidence
identifies the relevant behavioral change, not a test-seam failure: the observer control and all
five preservation/compatibility tests pass in red, and the three failures are on the branch under
repair. The zero-length failure is a **message** failure, which is itself the evidence behind the
correction in F-1.

### Run 5 — full suite, compared by identity

**203 pass / 2 fail / 4 skip.** This reproduces the author's reported counts exactly. More
importantly, the failures are the same two **identities** with the same **cause**:

```
tar_names_test.go:80:  TestBuildRestore_HostileFilenamesRoundTrip
tar_names_test.go:145: TestBuildFilelist_IsNulDelimited
  cannot stage "tab\there.txt": open ...\tab<TAB>here.txt:
  The filename, directory name, or volume label syntax is incorrect.
```

That is OBX-001 — the Windows-invalid TAB fixture, unrelated to this patch and correctly **not**
bundled into it. The four skips are the same four, on the same gates:
`TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`, `TestVolumeHealth_SystemDisk`,
`TestTreeExpansionBudget`.

**The suite is not green and must not be described as green.** It is *no worse than baseline*: the
+8 over baseline's 195 is exactly the eight new top-level test functions in `catalog_open_test.go`.

**Methodology note against my own run.** My first full-suite attempt set `GOTMPDIR` into the
scratchpad and reported 7 failures. Five of those were an artifact of that override —
`par2.exe` rejects the relocated temp path ("Ignoring out of basepath source file"). Re-run with the
default temp directory, the count is 203/2/4. The five extra failures were mine, not the patch's.

No handoff example test (`docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/examples/go-contracts/`)
was counted as product regression evidence; those live in a separate module and are outside `./...`
for this package.

### Remaining limitations

- **windows/amd64 only.** No Linux or macOS evidence for this path. The injected error shapes are
  platform-independent, but the real triggers (ACL denial, sharing violations, `Incorrect function.`)
  are Windows-specific.
- **Race: NOT TESTED**, unchanged, for the recorded reason (no C compiler). Not a pass.
- **No hardware, tape, optical or Docker validation**, and none is warranted by this patch.
- The observer bounds the "no writes" evidence to the four instrumented paths (section 4).

---

## 7. Answers to the five questions in report §7

**1. Is the three-way classification right, and is discarding bytes that arrive with an error the
behavior you want?**

Yes to both, and I would not change the logic. The switch at
[store.go:1157-1187](../../../store.go#L1157-L1187) is exhaustive and correctly ordered: success is
matched first, `fs.ErrNotExist` is the sole initialization licence, and everything else refuses.
Using `errors.Is` rather than a type assertion is the right call and is pinned by
`TestOpenStore_NotExistShapesAllInitialize` in three shapes, so the classification cannot silently
narrow if a future Go release or an interposed layer rewraps the error.

Discarding bytes returned alongside an error is not merely defensible, it is the only safe option.
`os.ReadFile` can return a partial buffer with a non-nil error on a short read. If such a prefix
happened to terminate at a structurally complete boundary, `json.Unmarshal` would accept it and the
app would proceed on a **silently truncated catalog with no error at all** — losing archives with no
diagnostic, which is strictly worse than the failure OB-001 describes. Keep it.

**2. Is a parameterized `openStore` the right seam shape versus the package-level fault vars the
codebase already uses in three places?**

Yes, and it is an improvement on the existing convention. The package-level seams (`openStoreFailSave`,
`Store.failSave`) require defer-restore discipline at every use, leak across tests if a `defer` is
missed, and force every test that touches them to forgo `t.Parallel`. The parameterized form has no
shared mutable state, nothing to restore, and no way for one test's injected fault to reach another
— exactly what the corrected prompt's item E asked for.

The cost is small and well contained: two parameters on an **unexported** function, with the
exported `OpenStore` signature and behavior untouched, and all three production callers
([main.go:71](../../../main.go#L71), [appbackup.go:438](../../../appbackup.go#L438),
[migrate.go:114](../../../migrate.go#L114)) unmodified. Production has exactly one caller passing
`os.ReadFile, nil`. I checked all three: each propagates the new error correctly, and `main.go`
turns it into `log.Fatalf`, so no listener starts.

Recommendation: when OBX-004 addresses the same fail-open shape in `loadJobs` and `LoadConfig`,
reuse this shape rather than adding a fourth package-level hook.

**3. Is the observer's coverage sufficient for the "zero attempted writes" claim to mean what the
tests say it means?**

**Yes for the claim OB-001 needs (claim A); no for the broader phrasing that appears in the test
header.** Section 4 has the full enumeration: I independently found nine filesystem-mutating sites
in `store.go`, of which four are instrumented and two are not. Of the uninstrumented pair, only
`MkdirAll` is reachable on a rejected open, and it runs before the read by design; `saveJobs` is
unreachable there because `loadJobs` sits after the classification.

So "zero attempted authority writes after a rejected read" means what it says. Two accuracy defects
must be fixed before that boundary is trustworthy to a later reader: **F-2** (the `saveJobs`
exclusion is justified by a false statement) and **F-3** (`backup-prune` is claimed as covered but
has no positive control). I would also take **F-4** — the whole-directory snapshot — because it is
nearly free and gives file-level evidence that does not depend on having enumerated the paths
correctly.

I do **not** recommend a general filesystem abstraction here. The documented boundary is
proportionate to a bounded fix; the problem is two wrong sentences, not the seam's design.

**4. Are the two new error messages the wording you want operators to see?**

The substance is right — both name the file, preserve the cause, and give an action — and this is
ultimately the owner's call, not a defect. Two observations:

- The unreadable message interpolates `%w` mid-sentence, so the operator sees the path twice with
  the OS text wedged between path and guidance:
  `cannot read C:\…\catalog.json: open C:\…\catalog.json: Access is denied. — refusing to start so…`.
  Putting the cause last, or dropping the duplicated path, would read better at a terminal.
- The zero-length message's escape hatch ("move the empty file aside if you really do want to start
  a new catalog here") is good and worth keeping — it converts a hard stop into a decision the
  operator can act on.

Both arrive through `log.Fatalf` as a single unwrapped line of 200+ characters. Acceptable for a CLI;
worth a thought only if you care about terminal wrapping.

**5. The initialization-intent residual: accept as scoped-out here, or hold PR-01 until it is
addressed?**

**Accept as scoped out. Do not hold PR-01.** The patch strictly reduces risk on the path it touches
and introduces no new failure mode — build, vet, format and the full suite are all no worse than
baseline, and every pre-existing behavioral contract in this area still passes unmodified. Holding
it would keep a *confirmed, reproduced, unattended total-catalog-loss* path open while a larger
storage-identity design is settled, which trades a certain present risk for an uncertain future one.

Two conditions on accepting it, both documentation-only: the residual stays **open** under OB-001
(section 5) with the reproduction recorded, and `OB_STATUS.md` marks only the read-failure branch
resolved, never the initialization/identity guarantee. Splitting the residual into a named successor
issue is the cleanest way to let PR-01 close without the register overstating what shipped.

---

## 8. Required corrections and optional suggestions

### Required before this patch is committed (documentation and tests only — no production logic change)

| # | Change | Anchor |
|---|---|---|
| R-1 | Reword the two zero-length comments to say baseline already failed closed via `json.Unmarshal`; PR-01 contributes an earlier, clearer rejection. Note that `TestOpenStore_ZeroLengthCatalogIsDamagedNotNew` pins a message, not a fall-through | [catalog_open_test.go:10-11](../../../catalog_open_test.go#L10-L11), [:334-336](../../../catalog_open_test.go#L334-L336) |
| R-2 | Restate the `saveJobs` exclusion by reachability. It **is** reachable on a successful open via `loadJobs`→`saveJobs`; it is unreachable on a **rejected** open. Fix source comment and report §2.3 together | [store.go:1086-1088](../../../store.go#L1086-L1088) |
| R-3 | Either add a `backup-prune` positive control (15 `.bak-*` sidecars) or downgrade the "four covered paths" claim to three demonstrated + one instrumented | [catalog_open_test.go:208-252](../../../catalog_open_test.go#L208-L252) |
| R-4 | Correct the report's line count for `catalog_open_test.go` (509, not 431) | report metadata table |

### Recommended regression tests

| # | Test | Why |
|---|---|---|
| O-1 | `backup-prune` positive control (satisfies R-3 as a test rather than a wording change) | Closes the last unverified observer path |
| O-2 | Full `filepath.Walk` snapshot in `requireUnchanged` instead of the two globs (F-4) | Independent file-level evidence that does not depend on having enumerated the write paths correctly |
| O-3 | A rejected-open test through the **exported** `OpenStore` using a **real** fault — a directory placed where `catalog.json` belongs. I verified this produces a genuine non-not-found read error on Windows (`Incorrect function.`) and it is portable (`EISDIR` on Unix) | Today "the public path uses the same branch" rests on inspection of the one-line delegate. This makes it evidence, with no injection at all |

### Optional maintainability

- `faultyReadAt` ([catalog_open_test.go:191](../../../catalog_open_test.go#L191)) takes `t *testing.T`
  but only calls `t.Helper()` and can never fail. The parameter can go.
- Consider asserting in the rejected-read subtests that `errors.Is(err, fs.ErrNotExist)` is **false**,
  pinning that a not-found can never arrive via the refuse branch.
- The `persistSpy.live` / `sawAnything` pair is used once, in `seedRealCatalog`. Fine as is; just
  noting it is the only consumer if the spy is ever trimmed.

---

## 9. Scope status

### Can be considered fixed by this patch

- A catalog read error that is **not** a positively identified not-found refuses startup: no usable
  `Store`, a wrapped error preserving the cause, and no seeding, recovery save, catalog replacement,
  daily-backup creation or backup pruning. Verified through the production decision branch by
  injection **and** by whole-directory before/after comparison.
- Bytes returned alongside a read error are discarded, never parsed as a catalog.
- A zero-length catalog is rejected earlier, explicitly, with an actionable message — **a
  diagnostics improvement, not the repair of a fall-through** (which never existed).
- Preserved unchanged and re-verified: the invalid-JSON `"catalog.json is damaged"` contract; the
  newer-schema read-only latch; fresh-directory initialization through both `openStore` and
  `OpenStore`; and the recovery-save-non-fatal policy.

### Remains open

- **The initialization-identity residual under OB-001** — `fs.ErrNotExist` does not establish
  intentional first use, and `MkdirAll` precedes the read. Reproduced in section 5. **OB-001 must
  not be marked resolved as a whole.**
- Non-Windows evidence for this path: none.
- Race detection: NOT TESTED (no cgo/gcc), unchanged.
- `backup-prune` instrumented but not control-verified (R-3).
- OBX-001 (the two Windows TAB failures), OBX-004, OB-002 and OB-003: untouched and open, correctly
  kept out of this patch.

---

## 10. Post-review state verification

Re-checked after all analysis and test execution. Everything matches section 1 exactly:

| Property | Value | Unchanged |
|---|---|---|
| Branch | `fix/ob-001-catalog-open` | ✓ |
| `HEAD` | `d97809b2f730e09960632e2943e562531fa095a3` | ✓ |
| Staged content | none | ✓ |
| `store.go` blob | `04f0b5051a83a43aa5d9a8c1611884497e484808` | ✓ |
| `store.go` SHA-256 | `8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56` | ✓ |
| `catalog_open_test.go` blob | `f873ef27ad30bc38fc55f36ebe81234a4e10752a` | ✓ |
| `catalog_open_test.go` SHA-256 | `69972b5d7f1e00cd84b31ad2abbccdccc3a23e98fd51f5b840c35a4f207d200c` | ✓ |
| Working-tree status | ` M store.go`, `?? catalog_open_test.go` | ✓ |

No pull, fetch, branch switch, reset, stash, clean, restore, stage, commit, push or merge was
performed. No production source or test file was edited. All probes ran in throwaway package copies
under the session scratchpad. `BASELINE-2026-09-06.md`, `OB_STATUS.md`, `NEXT_ACTIONS.md`, the
existing `reviews/PR01-OB-001-2026-09-06.md`, the misfiled `PR01_REVIEW_ADDENDUM/` directory and the
untracked handoff were all left exactly as found; no issue status was changed. This file is the only
addition.

PR-02 was not started.
