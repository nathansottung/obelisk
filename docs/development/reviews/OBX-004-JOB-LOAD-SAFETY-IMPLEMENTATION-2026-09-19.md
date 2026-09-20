# OBX-004 job-state loading implementation - 2026-09-19

READY_FOR_JOB_LOADING_REVIEW

Author provenance: Codex, implementation and native testing in the existing shared session. No separate reviewer, separate model context, or context-isolated execution is claimed. This candidate is uncommitted and awaits one substantive review; it is not accepted or published.

## Starting checkpoint and authorization

Repository: `C:\Users\nsott\source\repos\obelisk`. Created `fix/obx-004-job-load-safety` at exact published evidence tip `da22f1d9895d5350142a6f9b05ac41f0a920e170`. Actual configuration implementation: `3ea444ae528af7343818142c867a8f955f9252aa`. The configuration branch's old base `26918c5b8ed01ea301fc9a4c7658e19054a73632` was not used as the new task parent.

The supplied prompt's final job-loading authorization superseded its pasted historical configuration prompts. It authorized branch creation, bounded implementation, testing and documentation only. The existing publication receipt at `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-publication-20260919\publication-receipt.md` was read and agrees with the local chain. Configuration publication and R1/R2 closure remain complete. No fetch, live remote lookup, push approval or publication replay occurred.

No AGENTS.md was found in the ancestor chain or repository, including hidden paths. Relevant handoff, next-actions, status, coverage, persistence findings, accepted completion/UI review records, source and tests were read; docs/CONTRIBUTING.md supplied the build/vet/format and cross-compilation checks. Initial tracked worktree and index were clean, no Git operation active, and the task branch did not exist. Figma was the sole untracked file. The new branch was created through normal permission approval. No staging, commits or amendments occurred.

Evidence root: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-load-20260919-212813`. Execution date is 2026-09-19 in America/New_York; corresponding UTC timestamps cross into 2026-09-20. `submitted-prompt.txt` preserves the actual input, without invented prompt identifiers.

## Confirmed defects and baseline reproduction

Parent store.go:3530-3582 ignored unreadable, empty and malformed jobs.json, then returned a usable Store. NewJob could serialize an empty/stale board over that rejected history. Null rows panicked at the unconditional Unrecorded dereference. Duplicate/nonpositive/missing IDs and competing identity aliases were not validated even though Job returns the first matching ID and mutation methods walk matches. Reload assigned the counter before finishing row traversal; errors/panics could therefore leave inconsistent adoption. These are the paths repaired here.

The `parent/` evidence copy retains the actual 230 pre-edit working files at the clean published parent, including embedded inputs, reports and the excluded Figma file. Every pre-existing byte matches `pre-identities.json`; the only added parent test is the portable `job_load_safety_test.go`. No parent production mutation or checkout reversal was used. The original regression is retained in its pre-gofmt form; selected-toolchain gofmt output equals the final portable test byte-for-byte, with identical assertions.

After enumeration, native red execution exited 1: `TestJobLoad_RejectExisting` and `TestJobLoad_RefusedReloadCannotSaveStaleBoard` failed, with 20 failing subtests. All 18 invalid-existing cases produced the expected red safety failures; the two reload cases showed stale overwrite or null-row panic. `TestJobLoad_ValidAndReconcile` and its four empty-form subtests passed. These are real runtime safety reproductions, not compile failures or a mutated publisher experiment. Panic is caught by the regression solely to record the failure and allow other cases to run; production parent behavior still panics.

Already-safe behavior retained: typed decode failures did not themselves adopt partially unmarshaled rows; current code now returns those errors instead of silently continuing. Maximum observed row ID already raised the counter. Valid RUNNING rows already became INTERRUPTED, optional sidecar first use already worked, and established write/completion qualification and UI precedence were accepted. This patch does not claim those as new inventions.

## Implementation contract and compatibility

The job sidecar remains the writer's unversioned object `{next, rows}`. There is no new schema version, migration, initialization flag, quarantine or recovery feature.

1. Read failure discards any accompanying bytes. Only os.ErrNotExist on initial Store loading means optional first use. This is a supported missing-file assumption, not proof that storage is connected or that old history never existed.
2. Decode and validate the entire candidate before adopting its rows/counter or reconciling any job. Empty bytes, malformed/trailing JSON, non-object/null top level, duplicate JSON keys, invalid UTF-8, wrong typed fields/timestamps, null rows and ambiguous identities refuse loading. Diagnostics give fixed reasons and row indices without echoing row contents; OS causes remain wrapped.
3. Positive unique integer row IDs reflect NewJob's established allocator and Job/SetJob lookup relationship. Missing/null/zero/negative IDs, duplicates and competing aliases cannot select a winner or receive invented IDs. Negative/null next counters refuse. Counter recovery to the maximum actual ID remains; a valid MaxInt counter/ID can be viewed but NewJob returns an exhaustion error rather than wrapping.
4. Existing reader-compatible empty `{}`, absent next/rows, rows:null and rows:[] remain valid. The actual writer emits rows:null for a nil slice and [] for a nonnil empty slice. A single case alias such as Next/Rows/ID remains accepted. Optional fields, omitted status/kind/timestamps, unknown fields and arbitrary status vocabulary retain the prior typed-reader policy; no blanket zero-value or status-schema validation was introduced. Unknown job fields are still ignored and can be dropped on a later write, as before; this is not configuration-style unknown-field preservation.
5. Read/validate/adopt is serialized by jobs.mu. On failure, existing rows and counter remain intact and a loadErr is latched before unlock. NewJob refuses before ID allocation or worker launch; all normal sidecar saves refuse before recording flags or publication are touched. Existing workers may still update their in-memory outcomes and return recording errors, preserving FAILED/current-unrecorded/history semantics. A later fully validated load can clear the latch. Deleting a previously loaded/published board, or deleting a rejected file after a failed reload, cannot clear it into empty success. There is no public reload endpoint and no continuous on-disk monitoring; this is a guard on actual read/refusal paths, not external-writer coordination.
6. Valid RUNNING jobs become INTERRUPTED with rate/ETA zeroed; artifacts remain. Work is not resumed or inferred complete. PersistError remains history; loaded Unrecorded is cleared according to the accepted OB-002 contract. Reconciliation save remains best-effort and deterministically re-derived on reopen. An injected reconciliation-write failure is tested without claiming a new result was recorded or lost.

## Source/caller map and changed paths

| Path | Bounded change / anchors |
|---|---|
| store.go | Store.jobs adds loadErr/loaded; openStore:1322 propagates failure; saveJobs:3448 gates stale writes; writeJobsRows:3501 marks successful publication; NewJob:3540 gates load failure and counter overflow |
| job_load.go (new) | jobField:21; decodeJobBoard:33; readJobBoard:91; loadJobs:103; retained-Store guard:108; loadJobsFrom:123 validates, latches, adopts and reconciles |
| appbackup.go | verifyAppBackup:352 validates jobs member before restore publication; RestoreAppBackup:476 handles rejected reopen and retained Store |
| migrate.go | migrateDataDir:114 propagates failed target reopen, checks the retained source board, and tells the caller to resolve rejected state before retrying |
| job_load_safety_test.go (new) | Portable parent/green rejection, valid/first-use/reconcile and refused-reload regressions |
| job_load_boundary_test.go (new) | Read faults, native obstruction, save/worker latching, reconciliation fault, exhausted identities, restore/migration callers, identity/optional compatibility |
| job_load_cli_test.go (new) | Actual built-program startup refusal, first use, valid reconciliation and ordinary restart, with retained bounded process evidence |
| docs/development/CODEX_HANDOFF.md | Append current job candidate, completed configuration publication, scope/evidence and next review |
| docs/development/NEXT_ACTIONS.md | Append one substantive review as the next action |
| docs/development/OB_STATUS.md | Append pending job candidate without closing broader OBX-004 |
| docs/development/REVIEW_COVERAGE.csv | Update bounded changed-source rows and add new candidate/test/report rows; not whole-file certification |
| this report (new) | Author evidence and review handoff |

All production OpenStore callers were traced: main startup, RestoreAppBackup and migrateDataDir. main.go's existing fatal OpenStore-error branch runs before background export, bind and API/worker startup, so no main change was needed. runJob already checks NewJob before launching its goroutine and routes failures through the existing error contract. No UI change is needed to preserve the accepted FAILED-primary/current-recording/history distinctions.

Restore now rejects a correctly hashed but structurally invalid jobs member before publishing any member. On a later failed reopen, the retained Store validates its current jobs file without adopting/reconciling rows; if rejected, it latches subsequent job writes. Migration likewise returns failure without swapping DataDir/Store, checks the retained source board and latches it if invalid. The new tests verify both actual caller paths and a subsequent NewJob refusal. These checks do not prevalidate all archive namespaces or redesign restore/migration transactions.

### Mutation boundaries and residuals

Catalog reading, migration/recovery/seeding and any corresponding catalog writes still precede the jobs load in OpenStore. A fresh-directory jobs obstruction test explicitly observes catalog.json creation before rejection; it verifies the obstruction survives. Seeded CLI refusal fixtures have unchanged recursive inventories. Do not generalize those fixture-specific no-mutation results to all startup paths.

A jobs-omitting restore can create a pre-restore backup and publish other members before failing job reopen; its returned error explicitly says members may already be restored. Migration may leave verified copies at its target after refusing adoption. Tests preserve job bytes and prevent later stale publication; they do not promise cross-file rollback. Broader storage identity, multiple processes, concurrent restore/worker transactions, retained generations and permissions remain deferred. The existing jobs temp-swap/directory-sync implementation was not redesigned; Windows directory sync remains a no-op and no power-loss durability claim follows.

## Current execution evidence

Process-selected Go 1.26.8 windows/amd64; GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off. Existing approved module/toolchain cache reused; fresh GOCACHE and TEMP/TMP/GNUPGHOME beneath this evidence root. No GOTMPDIR override, global tool/configuration changes, installation, WSL, real keyring/catalog/original/media access. Commands writing fixtures/evidence under ObeliskDev used normal permission escalation.

`runner.py`, `*-execution.json` and `logs/` retain exact commands, cwd, settings, timestamps, exits and raw output. Source was formatted only in the seven changed Go files, using the selected gofmt binary; final gofmt -l output is empty. Final source remained unchanged after full-suite validation; subsequent changes were development records only.

| Execution | Exit | Top-level pass / fail / skip | Subtest pass / fail / skip |
|---|---:|---|---|
| Exact-parent portable red | 1 | 1 / 2 / 0 | 4 / 20 / 0 |
| Initial focused candidate | 0 | 85 / 0 / 0 | 117 / 0 / 0 |
| Final native full suite, excluding three system-disk probes | 0 | 254 / 0 / 39 | 122 / 0 / 0 |

Each selection was enumerated before execution. The focused run preceded the additional optional-field/identity regression, CLI test hash-preference restoration, and a diagnostic/comment clarification; the full suite covers the final code and all 10 new top-level job tests. There was one native full-suite run, no retry-until-green and no discarded failure. Overlapping selections are not summed. Version, build, vet, formatting and whitespace checks passed. Linux/amd64 and macOS/arm64 cross-build/vet passed as requested by contribution guidance; these are compilation/static checks only, not OS runtime evidence.

The 39 explicit skips comprise 36 missing GPG/PAR2 integration cases, one POSIX permission fixture unavailable on Windows, and two opt-in scale/performance tests. TestSmartDeviceNode_SystemDisk, TestVolumeHealth_SystemDisk and TestDeviceIdentityAndLabel_SystemDisk were enumerated but excluded by -skip, not executed or counted as emitted skips. Full test/subtest names and outcomes are in full-outcomes.json; raw reasons remain in logs/full.stdout and full-skip-output.json. Earlier configuration author 244/39/84 and reviewer 39/48 results remain historical; they were not relabeled as this task's execution.

### Actual CLI outcomes

The new regression builds the actual program with the selected Go toolchain (90-second build bound). Every child uses an explicit disposable -data path, ephemeral 127.0.0.1 listen address, synthetic token and isolated GNUPGHOME. Inherited auth variables are removed. Each observation is bounded to eight seconds plus HTTP request timeout; each process is killed if still running and Wait completes before fixture reuse. HTTP root 200 establishes serving. PIDs, exact arguments, native exits, before/after snapshots, readiness, forced_stop and logs are retained in cli/jobs-lifecycle-*.

| Case, in each targeted/full execution | Result |
|---|---|
| Malformed jobs JSON | Natural exit 1, no HTTP readiness, no panic, seeded fixture unchanged |
| Valid row followed by null | Natural exit 1, no HTTP readiness/panic or job reconciliation write, seeded fixture unchanged |
| Duplicate IDs | Natural exit 1, seeded fixture unchanged |
| Directory at jobs.json | Natural exit 1, directory retained; not an ACL test |
| Valid config with absent optional jobs | HTTP 200; bounded forced stop and wait |
| Valid RUNNING job | HTTP 200; job becomes INTERRUPTED; bounded forced stop and wait |
| Ordinary restart | HTTP 200; fixture hashes unchanged; bounded forced stop and wait |

Four natural refusals and three serving-then-forced-stop cases ran per matrix. Serving processes also report native exit 1 after forced termination on Windows; those are not naturally successful exit codes. Config CLI lifecycle tests ran too, retaining their own separate logs. No recorded CLI process remains running at final inspection.

No Windows race, effective ACL enforcement, Docker runtime, CI, Linux/macOS runtime, browser rendering, power loss or hardware qualification is claimed. Node-based existing UI logic regressions passed without browser qualification. Read-permission/I/O injections and native directory obstruction are distinct evidence, not ACL enforcement. Native encryption/recovery integration remains limited by missing helpers. The prior catalog-reopen Windows sharing interleaving is unchanged; passing this run does not establish its absence.

## Exact executed commands

The following are actual retained executions (Go runs use the settings above; cross checks record their own GOOS/GOARCH overrides). Whitespace and source-format writes were also executed directly through the shell tool and are recorded separately in the final evidence summary.

- `build-execution.json`: `"C:\Program Files\Go\bin\go.exe" build -o C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-load-20260919-212813\build\obelisk.exe .`; exit 0.
- `darwin-build-execution.json`: `"C:\Program Files\Go\bin\go.exe" build -o C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-load-20260919-212813\build\obelisk-darwin-arm64 .`; exit 0.
- `darwin-vet-execution.json`: `"C:\Program Files\Go\bin\go.exe" vet ./...`; exit 0.
- `format-execution.json`: `C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\gofmt.exe -l store.go appbackup.go migrate.go job_load.go job_load_safety_test.go job_load_boundary_test.go job_load_cli_test.go`; exit 0.
- `full-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 5m -skip _SystemDisk$ ./...`; exit 0.
- `full-list-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -list . .`; exit 0.
- `linux-build-execution.json`: `"C:\Program Files\Go\bin\go.exe" build -o C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-load-20260919-212813\build\obelisk-linux-amd64 .`; exit 0.
- `linux-vet-execution.json`: `"C:\Program Files\Go\bin\go.exe" vet ./...`; exit 0.
- `red-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 2m -run ^TestJobLoad_ .`; exit 1.
- `red-list-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -list ^TestJobLoad_ .`; exit 0.
- `target-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 3m -run ^(TestJobLoad|TestDurableCompletion|TestJobsUI|TestSeeingWhatHappened_Interrupted|TestConfig|TestOpenStore|TestKeystoreValidation|TestAppBackup|TestAtomicRename|TestRestoreAppBackup|TestWriteCatalog) .`; exit 0.
- `target-list-execution.json`: `"C:\Program Files\Go\bin\go.exe" test -list ^(TestJobLoad|TestDurableCompletion|TestJobsUI|TestSeeingWhatHappened_Interrupted|TestConfig|TestOpenStore|TestKeystoreValidation|TestAppBackup|TestAtomicRename|TestRestoreAppBackup|TestWriteCatalog) .`; exit 0.
- `version-execution.json`: `"C:\Program Files\Go\bin\go.exe" version`; exit 0.
- `vet-execution.json`: `"C:\Program Files\Go\bin\go.exe" vet ./...`; exit 0.

## Candidate identities and retained artifacts

The table below records SHA-256 of current source/test/build inputs. `pre-identities.json` covers all 230 initial files. `final-identities.json` and `candidate/` retain every final tracked/untracked working file and its actual bytes, including new tests and the finished report. The full final manifest is external to the report, avoiding a self-hash cycle. Parent bytes are retained in parent/, not represented by hashes alone. No historical report was rewritten.

| Path | SHA-256 |
|---|---|
| store.go | 18b43c41a396671c7ee874210b015127fcef81b8747d86adc2a9a97b45754217 |
| job_load.go | b41fee17f7d0b56e50c5906032bc5b2af01f9bf7dc971eeec7025908f60aec29 |
| appbackup.go | 2645bd55e3d058b72973aa6da1030719b685f990387ea3a658b2517abd930e0e |
| migrate.go | 6fe1d7ba44f83b3c3f85942c14b71311c7f798cbe92485acefa3bd077a25fc1a |
| job_load_safety_test.go | 15e64496b0a33c480d8f87aa6c74674d7a0c3d08b8869b5a4a01e1912df0645c |
| job_load_boundary_test.go | 7e8b9bef507f24ab7b11537c48c56cd5a8c63e1f29b341451256ccd718fe5dd7 |
| job_load_cli_test.go | dcc3111bc7b373e3f520990b35122216788939391c14c522ad6aeeccaa2ae0e3 |
| main.go | 02706c9299b08d5cdb30d251f26655e89472f324cd6dadc0f04497ff6df27b22 |
| config.go | fcfb81fe8c317522806e0cc0742688a9650d0958983c7fb95f9c0e1adaae1789 |
| keystore_validation.go | 29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593 |
| ui/index.html | 7c40475011aa6935273f14ffb3ae97b8e667b087f528f011d0d74d9a227e1656 |
| go.mod | b95a14e02c8faf2a4aac73901b843bc79ba154fd15aeaeb6161391bfaeb2c2bc |
| go.sum | 9d169ad00514ad13d9a10e1b1ac4723171c020a23b792c76721d97ccd47a1993 |

## Final preservation and next action

The final preservation manifest checks every initial file against the explicit change list. Only the three existing production files and four living development records are modified; four new Go files and this implementation report are added. Figma remains untracked, 219717 bytes, SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. Configuration implementation, dependencies, UI, accepted keystore/job-completion tests and historical reviews remain byte-identical to the published starting checkout. Branch is fix/obx-004-job-load-safety, HEAD remains da22f1d9895d5350142a6f9b05ac41f0a920e170, index empty, no active Git operation. See preservation-final.json and final status in evidence.

Next action: one substantive review of this exact uncommitted job-loading candidate, especially complete validation before adoption, refusal latching and actual caller propagation, compatibility/identity decisions, and truthful mutation boundaries. Do not replay configuration review or automatically publish this candidate.

Product direction remains generation-independent buffering/preservation, intended LTO-1 through LTO-10/future capability extensions and LTO-8 as first physical qualification target, not a generation limit or tested-support claim. Blu-ray and Figma implementation remain separate. No GUI/framework, PR-04 reconstruction, media operation or subsequent workstream was started. No staging, commits, pushes, merges, installs or automated reviewer execution. Stop for review.
