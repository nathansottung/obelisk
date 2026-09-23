# OBX-004 job-loading substantive review - 2026-09-19

NEEDS_CHANGES

Reviewer provenance: Codex, source inspection and fresh native executions in the shared session that implemented this candidate. This is not an independent agent, isolated context, human certification, owner acceptance or publication. Review only: no implementation changes. Local execution date is September 19; UTC timestamps cross September 20.

The loader and ordinary refused-reload protection pass this bounded review. One restore validation bypass remains: an incoming member whose name resolves to jobs.json can replace the known-good board without receiving the new prepublication validation.

## R1 / P1: restore validates a spelling, but publishes a destination

Anchors: appbackup.go:354-358 (exact members["jobs.json"] lookup), :446-463 (manifest iteration and filepath.Join destination), :476-481 (late reopen/refusal); job_load_boundary_test.go:198 (canonical-name-only incoming rejection).

A correctly hashed bundle with member `./jobs.json` containing `{"rows":[null]}` passes verifyAppBackup because the map has no exact `jobs.json` key. RestoreAppBackup then joins that name to DataDir, replacing the existing jobs.json. OpenStore rejects the null row only after publication. `Jobs.json` has the same effect on this native Windows filesystem. This occurs without concurrency, external edits during restore, invalid hashes, traversal outside DataDir, or a later unrelated member failure.

Reviewer reproduction: external `candidate/reviewer_job_probe_test.go`, TestReviewerJob_RestoreDestinationAliases. Each case starts with an actual NewJob and saves its known-good bytes, constructs a correctly hashed synthetic bundle, calls production RestoreAppBackup, and requires byte preservation after refusal. Canonical `jobs.json` passes; `./jobs.json` and `Jobs.json` both fail the preservation assertion. Their returned error is `reopen catalog after restore (members may already be restored): open job board: invalid jobs.json: null row at index 0`. Native probe exit 1. Production sources are unchanged in the copy; no mock publisher or substituted decoder is used.

The retained-Store latch prevents subsequent stale saves after the late refusal, but cannot undo the invalid replacement already performed. A pre-restore copy may aid recovery; it does not satisfy validation before replacing live history. The generic archive-name handling predates this patch. The scoped defect is that the new job-member check does not cover the destinations that the existing publisher can write. This finding does not require a comprehensive archive namespace audit or cross-file atomic restore.

Smallest complete remedy: before any restore mutation, ensure every accepted member capable of publishing to the job-board destination receives job validation, or reject its noncanonical name. Validation and extraction must use one consistent name/destination policy, including platform aliases and multiple members resolving to the same job destination. Keep ordinary canonical valid restores supported. Add actual RestoreAppBackup regressions for these two names, a canonical-valid-plus-invalid-alias collision, byte preservation on refusal, and a valid control. Correct the current handoff/report claims as part of that functional correction; no separate documentation-only review loop is needed.

## Contract dispositions

| Contract | Disposition and evidence |
|---|---|
| Read/decode/validation and compatibility | PASS in the inspected scope. Read errors discard partial bytes and retain wrapped causes. The complete local candidate is validated before adoption/reconciliation. Null rows, malformed/non-object input, wrong typed fields/timestamps, invalid UTF-8, duplicate IDs/nonpositive IDs and competing identity/membership aliases refuse. IDs follow the actual positive allocator/lookup contract; counter recovery and exhaustion handling are coherent. Valid omitted fields, empty object, null/empty rows, optional values and single case aliases pass. Unknown extensions retain the existing ignore/drop-on-write policy; no new schema or preservation promise. No concrete supported unambiguous writer output was found rejected. |
| Refused reload and later stale writes | PASS for serialized operations on one Store. job_load.go:123 holds jobs.mu across read/validation/adoption and latches failure before unlock. store.go:3448 gates all normal saveJobs publication before recording flags; :3540 gates NewJob before allocation. Failed reload preserves prior rows/counter, repeated failure and subsequent absence do not clear refusal, and a fully validated reload permits real publication again. Existing worker outcomes can change in memory with truthful recording errors. |
| Startup and caller propagation | PASS. OpenStore returns nil/error on job refusal; main.go:76-79 exits before background export, bind or normal work. Real CLI fixtures reach this gate with valid configuration/catalog prerequisites. Catalog initialization/recovery can precede the job gate; a fresh-directory obstruction test observes catalog creation. Startup is not globally mutation-free. |
| Reconciliation and accepted completion/UI semantics | PASS. Valid RUNNING becomes INTERRUPTED, rate/ETA clear, artifacts survive, and work is not resumed. Current recording qualification remains distinct from historical PersistError; FAILED stays primary. Best-effort reconciliation save is unchanged and tested. Completion is not file verification. Existing completion and UI logic tests passed; no browser qualification. |
| Restore and migration | NEEDS_CHANGES for R1. Canonical invalid incoming jobs refuse before publication. Failed reopen latches the retained Store when its current job board is invalid. Valid restore and migration controls pass, including retained history, original source bytes, identity continuation and actual Store adoption. Migration may leave target copies; restore can publish other members before a late failure. These existing cross-file effects remain deferred. |

Source inspection covered the full production diff, complete new job_load.go and all three new regression files, persisted Job representation, uniqueJSON, all job read/write/refusal-state references, OpenStore startup ordering, saveJobs/writeJobsRows, NewJob, terminal/artifact/result mutations, Job/Jobs snapshots, runJob and its recording-error reporting, archive verification/extraction and migration. writeJobsRows has no production caller bypassing saveJobs. The only loadErr clearing assignment follows complete validation. Retained-Store checks do not silently clear an existing refusal. All production OpenStore callers were traced.

The external TestReviewerJob_ReloadCompletionOrdering additionally holds the production reload inside its read seam, verifies jobs.mu is held, starts a normal FinishJob call, then releases the invalid read. Reload and completion return errors; rejected disk bytes survive; no candidate row/counter is adopted; FAILED is qualified Unrecorded. A repeated refusal and subsequent valid reload followed by NewJob/reopen establish the negative and positive paths. This is a bounded lock-order probe, not a race-detector or broad concurrency qualification.

## Candidate identity and preservation

Repository: `C:\Users\nsott\source\repos\obelisk`. Branch: `fix/obx-004-job-load-safety`. Full HEAD and explicit patch base: `da22f1d9895d5350142a6f9b05ac41f0a920e170`. Configuration publication/R1/R2 were not reopened; no remote lookup occurred.

No AGENTS.md was found in the repository or ancestor chain. Read docs/CONTRIBUTING.md, author implementation report, relevant current handoff/next-actions/status/coverage records, bounded persistence findings and accepted completion/UI records. No completed substantive job-loading review was present; the implementation report was the only job-loading report at review start.

Seven tracked files differ from the explicit base: store.go, appbackup.go, migrate.go and four living development records (CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md, REVIEW_COVERAGE.csv). Tracked diff: 78 insertions, 69 deletions. New candidate files are job_load.go, job_load_safety_test.go, job_load_boundary_test.go, job_load_cli_test.go and the implementation report. Figma is separately untracked and excluded. Index empty; no active Git operation.

Evidence root: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-review-20260919-220757`.

- `initial-identities.json`: actual SHA-256 working-byte manifest of all 235 pre-existing tracked/untracked candidate files, including sources, tests, dependency/build/deployment inputs, development records, historical reports and Figma. Manifest file SHA-256: `c943fe1e4f036aad96c9de5de5cb34c43fbfa3aed98c17d6ca957ff17a96a2f3`.
- `author-comparison.json`: comparison with author `obx004-job-load-20260919-212813/final-identities.json`; zero added, missing or changed paths. No material drift or missing identity artifact.
- `candidate/`: all 235 byte-preserved working files, including untracked sources/tests; every original file still matches the manifest. Only reviewer_job_probe_test.go was added there. Probe SHA-256: `296d76af63f5edbf7c4924082e9ea2b5266307e4bbba2a91576ba6e38a7c5e8c`.
- `candidate.txt`: full tracked explicit-base diff; untracked bytes are separately retained in candidate/. `status-before.txt`, `head.txt`, `branch.txt`, `index.txt`, `checkpoint.json` retain starting state. `submitted-request.txt` retains the actual request.

The author manifest and report agree with the reviewed files. Historical exact-parent red execution is author evidence (2 top-level failures, 20 failing subtests, plus compatibility passes), not reviewer execution. No historical replay or production mutation experiment was performed in this review. The new restore failures are executions of the current byte-preserved candidate.

## Reviewer execution

Go 1.26.8 windows/amd64 was verified. Reused approved process-scoped settings: GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off, existing approved module cache. Fresh review GOCACHE, TEMP/TMP and synthetic GNUPGHOME were used. No GOTMPDIR override, installation, global configuration, WSL, personal keyring or real archive/media access. Exact commands, settings, native exits and timestamps are retained in `*-execution.json`; `runner.py` and `logs/` retain the runner and output. The runner's own shell exit is not substituted for its recorded native child exit.

Both selections were enumerated successfully before execution. Main selector:

```text
^(TestJobLoad|TestDurableCompletion|TestJobsUI|TestSeeingWhatHappened_Interrupted|TestConfig|TestOpenStore|TestKeystoreValidation|TestAppBackup|TestAtomicRename|TestRestoreAppBackup|TestWriteCatalog|TestCatalog)
```

Main execution: `go test -count=1 -json -timeout 3m -run <selector> -skip TestCatalogScale$ .`. CatalogScale was enumerated but deliberately excluded, not executed or counted as an emitted skip. Probe execution in external candidate/: `go test -count=1 -json -timeout 2m -run ^TestReviewerJob_ .`.

| Reviewer execution | Native exit | Top-level pass / fail / skip | Subtest pass / fail / skip |
|---|---:|---|---|
| Focused current candidate | 0 | 90 / 0 / 0 | 122 / 0 / 0 |
| External reviewer probes | 1 | 2 / 1 / 0 | 1 / 2 / 0 |

Exact selected names and results are in logs/target-list.stdout, target-outcomes.json, logs/probe-list.stdout and probe-outcomes.json. The probe failure is R1; its canonical-name control passes. The other two probes (reload/completion ordering and valid restore/migration) pass. No setup/test failures were discarded or retried until green. One exploratory rg command used a Windows-incompatible literal wildcard and was corrected for source inspection; it was not a test failure.

Native build (`go build -o <evidence>/build/obelisk.exe .`), `go vet ./...`, selected-toolchain read-only `gofmt -l` of all seven candidate Go files, and `git diff --check` each exited 0. Formatting output was empty; diff check emitted only existing line-ending conversion notices for OB_STATUS.md and REVIEW_COVERAGE.csv. No formatting writes occurred. No reviewer full-suite or cross-platform run; author cross-build/static results remain reported evidence only.

The actual job CLI matrix produced four natural exit-1 refusals (malformed JSON, late null row, duplicate IDs and native directory obstruction), each with no HTTP readiness and unchanged seeded fixture inventory. Valid optional-first-use, RUNNING reconciliation and ordinary restart reached HTTP 200 and were explicitly stopped and waited. The configuration lifecycle matrix also ran as an affected regression. All 16 child records are retained in cli-summary.json and cli/; forced Windows exit 1 is not a naturally successful exit. Each child used explicit disposable -data, ephemeral loopback address, synthetic auth and bounded lifetime. No recorded CLI process remained at the final check.

Author-only evidence remains distinct: the reported final full native suite is 254 top-level passes / 39 skips, separately 122 passing subtests; its retained test-summary.json agrees. These counts are not added to reviewer results. Missing GPG/PAR2 skips do not establish encryption/recovery integration. Native directory obstructions and injected permission/I/O errors are not ACL enforcement tests.

## Limits and final handoff

Optional jobs absence on a newly constructed Store remains a first-use assumption, not missing-storage identity or recovery. Refusal protection applies to actual load/check paths; it does not monitor arbitrary external edits after successful validation. Cross-process writes, concurrent restore/worker transactions, retained generations, cross-file rollback, broader catalog structure, permissions, archive namespace security beyond R1, and existing publication durability residuals remain deferred. No Docker, CI, race detector, Linux/macOS runtime, browser, power-loss or physical-media qualification. The historical Windows catalog sharing interleaving is neither repaired nor disproved by this passing selection. Figma/GUI and media work remain separate; generation-independent architecture and LTO-8 as first physical qualification target are unchanged.

Preservation check: all 235 pre-existing candidate files remain byte-identical, including historical reports and author evidence in the repository. Figma remains untracked, 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda`. Branch/HEAD unchanged, index empty, no Git operation; remaining recorded CLI processes: zero. Only this new review report is added to the repository. Final verification is retained externally in preservation-final.json and status-after.txt.

Handoff: NEEDS_CHANGES on `fix/obx-004-job-load-safety` at `da22f1d9895d5350142a6f9b05ac41f0a920e170`; candidate identified by initial-identities.json above. Validation, ordinary stale-write protection and startup pass; restore is blocked by R1. Reviewer totals remain 90 passing main tests plus a separate probe selection with 2 passes / 1 failure; subtests and limitations are reported separately above. Nothing fixed, staged, committed or pushed.

ONE next action: one consolidated correction of R1, followed by a targeted recheck of that correction. No publication or next implementation is authorized by this review.
