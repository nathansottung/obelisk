# PR-02 / OB-003 — external source review

**Outcome: READY_FOR_OWNER_REVIEW**  
**Review date:** September 7, 2026  
**Reviewer:** ChatGPT in a separate review context from the Claude implementation session. This is an external AI source review, not a human audit or certification.  
**Input:** `OBELISK_PR02_REVIEW_INPUT_2026-09-07.zip`  
**Reported PR-02 base:** `4cd867b2ddb26c945f7c74faee8ef32780743be7`  
**Reported working branch:** `fix/ob-003-safe-replacement`

## Decision

The bounded destructive-fallback correction is accepted for owner review. I found **no blocking defect introduced by this patch** within its stated replacement/caller-cleanup scope. The two required follow-up items are closed:

- **F-1 CLOSED:** the submitted restore test now proves that the catalog staging file existed at the failed-publication boundary and requires a genuine not-found result afterward, while retaining the prior-destination preservation assertion.
- **F-2 CLOSED:** the saved application-restore comment now states that cleanup is best effort, its failure is discarded, and removal is not secure erasure.

No additional F-1/F-2 implementation cycle is required. The next workflow step can be a scoped checkpoint commit/push of this exact reviewed patch, subject to owner acceptance and a local artifact-identity check. This is **not** a main-branch merge, release approval, whole-repository safety approval, or closure of unrelated archival issues.

The accepted guarantee is narrow:

> Obelisk no longer deletes the existing destination as its own fallback after a failed rename. In the tested missing-source, injected-error and local Linux cross-filesystem failure cases, the prior destination survives unchanged. The helper returns the underlying failure through `%w`; caller cleanup targets the staging pathname, subject to the documented ownership/aliasing assumptions.

Do not generalize this into power-loss durability, secure erasure, cross-platform atomic visibility, concurrency safety, or a guarantee about every possible filesystem failure.

## 1. Evidence boundary — important

This review includes direct reading of the uploaded base/candidate source, all five helper callers, the submitted new test file, relevant test scheduling/fixtures, and the review chain. It also includes independently executed **isolated production-code probes** on Linux.

It does **not** include a successful full-repository build or test run in this environment. The module cache lacked the declared third-party dependencies. An actual `go mod download` attempt failed because network/DNS access was unavailable. Subsequent build, vet and targeted-repository attempts confirmed the dependency blocker with module lookup disabled. No dependency was replaced by a stub or mock merely to manufacture a repository pass.

Therefore:

- The author's Windows **212 pass / 2 fail / 4 skip** remains **reported evidence**, not my reproduced full-suite result.
- The two reported Windows TAB-fixture failures remain OBX-001; I did not execute Windows tests.
- Windows race testing remains NOT TESTED.
- A successful **Linux isolated-harness** race run is additional, narrower evidence. It is not a full-package race result and not Windows evidence.
- No CI result, current GitHub HEAD, remote branch, local Windows index, or live workstation state was independently queried. This review identifies the uploaded bytes, not a remote branch that might later move.
- No LTO, optical, Docker, NAS, live catalogs, real keys, or production backup data were exercised.

These limitations do not reveal a source-level blocker in the bounded fix. They must accompany its checkpoint and subsequent CI evaluation.

## 2. Package and patch identity

Input ZIP SHA-256:

```text
4ded21c8b89d080fd96409aba14299084e21a5f24841fabfd92aa82857290219
```

Archive inspection found 392 entries, no duplicate/unsafe paths or symbolic-link entries, and no CRC failure. Both supplied all-file SHA-256 manifests verified with zero missing, additional or mismatched files:

| Tree | Files | Go files |
|---|---:|---:|
| Supplied base | 188 | 131 |
| Candidate | 189 | 132 |

The candidate/base difference is exactly:

```text
modified  mirror.go
modified  appbackup.go
added     atomic_replace_test.go
modified  docs/development/OB_STATUS.md
modified  docs/development/NEXT_ACTIONS.md
```

The package README explains that its base combines the tracked working-tree representation with the changed paths restored from their reported base blobs. Some unrelated non-Go files retain working-tree CRLF rather than raw Git-blob LF. I verified internal manifests and the precise diff, but this is **not** an independently authenticated `git archive` of the entire named commit. No `.git` history was provided. That packaging qualification does not obscure the actual Go-source delta.

### Reviewed candidate identities

| File | SHA-256 | Git blob SHA-1 | Lines |
|---|---|---|---:|
| `mirror.go` | `51801765bab2d2d868c7aa802f8527fb8fa893c3be9864054a2e46a6ec56a211` | `c10a447dbcd4f4e9929ddf7e75eb664908a00ab0` | 478 |
| `appbackup.go` | `c088995ee6dba916c1b3f6657f84e9a7d8b435d187fc0643fa3fb8b060df27d9` | `f007d878b86ef83b8ce40c118cfb81d9d76f4060` | 525 |
| `atomic_replace_test.go` | `b46875d17a29c716ec5308f236feae47471d1755c5ce124dc3d0728cee1f0bf5` | `097a61e7f1aee356717aebee735c0547c2bae69c` | 304 |

PR-01 preservation was checked directly between the supplied trees:

| File | Candidate/base blob | Result |
|---|---|---|
| `store.go` | `bfce18705e877c09d00d6c318767682d9128cf70` | byte-identical |
| `catalog_open_test.go` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` | byte-identical |
| `plans.go` | `9b4ccbf7cdad255ec6929781ee99fa9485ed6e92` | byte-identical |

All input and execution-copy source files were rechecked against the original manifests at the end. **None changed.** Only separate review/probe files and scratch caches were created. The uploaded ZIP also remains unchanged.

## 3. Production correction and caller map

### Helper — `mirror.go:315–354`

Production binds `renameFile = os.Rename`. `atomicRename` calls it once, wraps a failure with `fmt.Errorf("publish %s: %w", final, err)`, and otherwise returns nil. The old `os.Remove(final)` and second rename are gone. There is no direct-overwrite copy fallback or rollback sequence.

All five callers were read:

| Caller | Staging construction | Error behavior and cleanup |
|---|---|---|
| `MirrorToVolume`, `mirror.go:218–255` | `destPath + ".mnemo_tmp"` | Best-effort remove staging; increment `Failed`; do not record that file as mirrored. |
| `copyVerifyToDest`, `mirror.go:292–312` | `destPath + ".mnemo_tmp"` | Best-effort remove staging; return publication error. |
| `ExecutePlan`, `plans.go:739–772` | `dest + ".mnemo_tmp"` | Best-effort remove staging; increment `Unreadable`; do not set `Satisfied` for the failed item. |
| `exportAppBackupTo`, `appbackup.go:183–233` | `tarPath + ".tmp"` | Best-effort remove staging; return publication error before writing the new sidecar. |
| `RestoreAppBackup.writeFile`, `appbackup.go:388–423` | `dest + ".tmp"` | New best-effort remove of staging; return the original publication error, wrapped with the member context by the caller. |

These are lexical same-directory constructions. They do not establish exclusive file ownership against malicious aliases, a racing process, or a real user file whose name collides with the deterministic staging suffix. Those are existing, separately recorded boundary problems, not solved by this patch.

Within the ordinary owned-staging contract, no failure cleanup removes the final pathname. The two collection-level loops continue returning counters rather than a rich per-file problem record. That existing operator-visibility limitation remains outside the replacement fix.

The application restore still operates one member at a time and can be partially applied before a later member fails. The prior-generation coherence and source-confinement issues remain open. PR-02 improves each publication boundary; it does not make the whole restore transaction atomic.

## 4. F-1 and F-2 closeout

### F-1 — closed by direct source inspection; narrower independent probe executed

`atomic_replace_test.go:239–285` retains the old-destination assertion and now records `fileExists(tmp)` **inside** the injected rename call. The test requires that observation for `catalog.json.tmp` before accepting the subsequent not-found result.

`gatherMembers` (`appbackup.go:103–108`) appends `catalog.json` first; restore processes that member before its final configuration step. The fixture passes `includeKeys=false`, so its coverage is correctly described as **catalog staging**, not restored-keystore security.

The final check uses `os.Lstat` and `errors.Is(err, fs.ErrNotExist)`; a permission/error outcome cannot silently masquerade as removal success. A failure to observe the staging file is fatal, so the absence assertion is not satisfied merely because no staging was ever created.

I could not run the whole submitted application test because its package could not resolve dependencies. I did mechanically extract the exact production `writeFile` function-literal body, assign it a top-level name without changing its body, and run a separate probe against real disposable files. That probe observed staging with the replacement bytes, injected publication failure, verified the prior catalog bytes, and required staging absence. It passed.

In a separate mutation copy, removing **only** `_ = os.Remove(tmp)` made that isolated probe fail on the absence assertion. The fixture-presence and prior-destination checks still passed. This independently confirms that the cleanup line matters, but is **not** represented as execution of the entire `RestoreAppBackup` path or the original repository test.

### F-2 — closed

`appbackup.go:397–401` now says removal is best effort, can fail, has its error discarded, and is not secure erasure. The executable cleanup remains `_ = os.Remove(tmp); return err`.

The same-session review report and follow-up were preserved with their true provenance. I do not retroactively describe them as independent.

## 5. Shared test hook and map ownership

The new `renameFile` seam is package-level mutable state. I parsed all 132 candidate Go files and found **no executable call to `Parallel`**. The only assignments to the hook are the two install/restore pairs in `atomic_replace_test.go:42–43` and `:258–262`.

The new tests and their table subtests run serially, restore the previous function with `t.Cleanup`, and invoke export/restore synchronously. `compatApp` creates a disposable Store/App without starting the main application's background loop. The `stagedAtFailure` map is written and read on that synchronous path; I found no concurrent accessor in this fixture.

The existing multi-volume mirror test starts goroutines but joins them with `WaitGroup.Wait` before returning; it does not install the hook. Production may concurrently read the unchanged hook, but there is no production assignment to it.

**Result: no concrete hook or map interference defect demonstrated in this submitted use.** This is not an endorsement of arbitrary future parallel tests or detached jobs using the same global seam. A concise nonparallel-test convention comment would be useful, not a blocker.

Official `testing` documentation describes the subtest/cleanup ordering relied on here. The runtime probe's race pass covers only the isolated harness, not every scheduling path in the product.

## 6. Independently executed checks

### Environment

- Linux/amd64 sandbox; local scratch files on overlay filesystem, `/dev/shm` on tmpfs.
- Go **1.23.2**; the provided module declares `go 1.22.2`.
- GCC **14.2.0** available; probes executed as unprivileged UID 1000, not root.
- Declared external Go dependencies absent; retrieval failed on DNS/network access.
- No toolchain installed or upgraded, no `go.mod` or `go.sum` changes, no dependency stubs.
- No `GOTMPDIR` override. Isolated home/module/build caches were used in review scratch space.

### Product checks versus isolated checks

| Check | Result | Evidence meaning |
|---|---|---|
| Input archive CRC/path checks and both all-file manifests | PASS | Uploaded bytes are internally consistent. |
| Base/candidate diff and PR-01 identity | PASS | Only the intended source/test and living-doc delta is present. |
| `gofmt -l mirror.go appbackup.go atomic_replace_test.go` | PASS, no output | Actual candidate formatting; no rewrite. |
| Repository `go build` | BLOCKED before compilation | Missing dependencies; **not** a product build failure finding. |
| Repository `go vet` | BLOCKED | Same module blocker; not a vet pass. |
| Targeted repository `go test` | BLOCKED | Original caller tests were not executed here. |
| Full repository suite | NOT RUN | Same unresolved dependency boundary. |
| Isolated candidate source probes | PASS | 8 top-level tests plus 4 injected-error subcases. |
| Exact old-helper red probes | EXPECTED FAIL | Missing-source and real Linux EXDEV probes lose the old destination; two normal-success cases and the tested permission case pass. |
| Isolated cleanup-line removal | EXPECTED FAIL | Catalog staging remains, and the required-absence assertion detects it. |
| Isolated `go vet` | PASS | Applies only to extracted harness. |
| Isolated Linux `-race -count=25 -shuffle=732451` | PASS | 200 top-level executions plus 100 subcases; no detected race in that harness. |

### What was actually executed without dependencies

Go's parser extracted these candidate declarations without changing their function bodies:

- `renameFile` and `atomicRename` from `mirror.go`;
- the `RestoreAppBackup.writeFile` body, given a top-level function name for the harness;
- five existing helper-level test functions and their small fixture helpers.

Three review-owned probes add a real unprivileged permission failure, a real Linux cross-device failure between scratch filesystems, and the extracted restore-closure cleanup check. The standalone package has no third-party imports.

These are **not replacements for the application's integration suite**. The report and filenames label them isolated. A module-resolution failure was not worked around by stubbing the application.

### Key red/green observations

| Scenario | Exact old helper | Candidate helper |
|---|---|---|
| Missing temporary source, good existing final | FAIL: final deleted | PASS: final unchanged and missing-source error retained |
| New publication, no previous final | PASS | PASS |
| Replace existing final | PASS | PASS |
| Real same-directory Linux permission failure | PASS: removal itself is denied | PASS |
| Real Linux cross-device rename failure | FAIL: final deleted | PASS: final unchanged, staging retained, `EXDEV` preserved |

The permission test passing against the old helper is expected: this particular denied directory also prevents its bad delete fallback. It is not a discriminating regression for OB-003. The missing-source and EXDEV cases are discriminating.

The cross-filesystem probe deliberately exercises an unsupported helper input, not a reachable regular same-directory product workflow. It establishes safe error handling on that Linux case, not cross-filesystem publication support.

### Retained Windows evidence

The supplied implementation/follow-up reports record 212 passing top-level tests, two Windows TAB-fixture failures and four skips. Raw full-suite output was not retained in the upload. I read the reports but did not independently reproduce those Windows results.

No statement in this report upgrades the known Windows failures, Windows race limitation, unavailable hardware or unrun CI to a pass.

## 7. Answers to the five implementation questions

**Q1 — keep the `%w` error wrapping? Yes.** It adds final-path context and preserves `errors.Is` through the helper and application restore's outer wrapping. I found no current caller or test switching on exact helper error strings or using a legacy `os.Is*` predicate on this wrapped result. Do not infer that those older predicates universally unwrap arbitrary errors.

**Q2 — keep the narrow caller cleanup in this patch? Yes.** It handles a staging artifact created immediately before publication in the same closure, returns the original error, and now has meaningful test coverage and accurate wording. Cleanup failure is still discarded and key permissions remain a separate issue. That limitation does not justify removing useful cleanup or claiming it always succeeds.

**Q3 — add explicit cross-device detection? No additional implementation is needed for this patch.** All regular callers construct a sibling staging path. A rename error is returned rather than converted to destructive copy/retry. The added real Linux EXDEV probe confirms the bounded failure behavior. The patch does not promise cross-device support or control against topology changes.

**Q4 — keep the file-wide comment/substring guard? Acceptable as a supplemental guard, not as safety proof.** The behavioral tests are the evidence. The grep-style test can reject an innocent future comment or fail to detect a differently spelled destructive path; narrowing it later would improve maintainability. It is not a blocker in this patch.

**Q5 — POSIX execution before making a general claim? This review adds actual Linux execution for the extracted helper.** That supports Linux helper behavior, not a full Linux application pass, macOS validation, arbitrary filesystem behavior, or an atomicity guarantee on Windows. Keep release claims platform- and test-specific.

## 8. Nonblocking observations and scope cautions

### R-N1 — the new helper comment still overstates cross-platform atomic visibility

At `mirror.go:334–339`, one rename call is used to justify that a reader can never observe a half-written file. Go's public `os.Rename` contract explicitly does **not** promise atomic rename on non-Unix platforms, even within one directory. Single API invocation and public cross-platform atomicity are not equivalent.

This does **not** invalidate removing the destructive fallback, nor does it require a Windows wrapper or another implementation cycle. Accept the bounded guarantee in this review instead of that broader sentence. A future routine documentation edit should say, for example:

```go
// Publication uses one os.Rename call and never deletes final as a fallback.
// Atomic visibility and failure semantics depend on the OS/filesystem; this
// helper adds no cross-platform atomicity or crash-durability guarantee.
```

No source change was made during this review. This is an optional wording improvement, not an outstanding F-2 requirement or a blocking product defect.

### R-N2 — document the serial-test assumption

The hook is sound in the inspected serial tests. A one-line warning against adding `t.Parallel` to hook-using tests would preserve the existing convention. No concrete race was found, and no new injection framework is required.

### R-N3 — keep existing residuals visible

- Best-effort temporary-file deletion may fail without separate reporting.
- Deterministic suffix collisions and hostile aliases are not eliminated.
- Restore generation coherence, member confinement and restored-secret modes remain open.
- Mirror/plan counters do not provide full per-file operator-visible errors.
- Directory sync and power-loss tests remain unprovided.
- PR-01's missing-storage/initialization-identity residual remains open.
- The two Windows OBX-001 test failures and Windows race gap remain open.

These are pre-existing or explicitly deferred matters. They should not be silently treated as fixed, but do not block this scoped loss-prevention repair.

## 9. Reviewer disposition and next action

**READY_FOR_OWNER_REVIEW — no required implementation changes from this review.**

On owner acceptance:

1. Verify the local working artifact hashes match this review.
2. Commit the PR-02 production/test changes on `fix/ob-003-safe-replacement` using an explicit file allowlist.
3. Commit selected review/status evidence separately, with its provenance and execution limits intact.
4. Push only that fix branch. Do not change main, tags or releases.
5. If a stacked PR is opened while PR-01 remains unmerged, compare against `fix/ob-001-catalog-open`, not main, so only PR-02 is presented as new work.
6. Evaluate actual CI when it runs; this review does not assert a CI result.
7. Move to PR-03 / OB-002 as the next implementation workstream. Do not restart PR-02's F-1/F-2 loop without a genuinely different patch or new defect.

Publication is a versioned checkpoint, not automatic merge approval or a claim that all archival guarantees are complete.

## 10. Evidence index and technical references

The companion ZIP includes:

- `evidence/source-identities.json` and `package-verification.json`;
- raw logs for the dependency blocker, read-only formatting, isolated green, old-helper red, cleanup mutation red, isolated race repetitions and isolated vet;
- `evidence/execution-ledger.json` with exact commands, exit statuses and classifications;
- `evidence/source-scheduling-scan.json` from parsing all 132 Go files;
- `evidence/input-preservation-check.json`;
- parser-based extractor, probe assembly script, selected production excerpts and standalone probe packages;
- source-anchor excerpts for the helper, callers and submitted F-1 assertions;
- a bounded publication prompt for the owner's local Claude session.

Raw original Windows test logs were not in the supplied package; they have not been manufactured here.

Primary technical references consulted to interpret the supplied code, not to replace it:

- Go `os.Rename` documentation: https://pkg.go.dev/os#Rename — replacement behavior and the non-Unix atomicity limitation.
- Go `testing` documentation: https://pkg.go.dev/testing#T.Run and https://pkg.go.dev/testing#T.Cleanup — test/subtest sequencing and cleanup lifetime.

The source/caller findings are based on the supplied candidate with the identities above. No live-repository freshness claim is made.
