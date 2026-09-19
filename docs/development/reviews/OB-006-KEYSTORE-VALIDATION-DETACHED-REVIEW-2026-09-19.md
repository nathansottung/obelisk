# OB-006 keystore validation - separate review execution, 2026-09-19

READY_FOR_OWNER_REVIEW

Provenance: AI review with new source inspection and command executions under the current user request, continuing the existing conversation. Not a separate model context or human certification. The previous same-session review is preserved as evidence; its verdict is not adopted as this review conclusion.

Root: C:\Users\nsott\source\repos\obelisk
Branch: fix/ob-006-keystore-validation
Full HEAD / explicit review base: 0dc7d5399c6889e015aebc9ba02e694a909d6e9c
No staged entries or Git operation found. No AGENTS.md found in the repository or ancestor chain.

Evidence directory: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919

## Candidate identities (SHA-256)

- pipeline.go: a0cbb5a031afdb869cdb7ae74277f40fd393303c84c121619e00be391a49dec6
- keystore_validation.go: 29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593
- keystore_validation_test.go: 10bb54072f30fe7ce4d088d42bad941ed084c28f1ef6438a6f6d748522c9d7a0
- keystore_validation_boundary_test.go: 2b0122a48d6969336e77fb8657d918c7fe2e1390e6310f0cf394c07c7032db24

All four match the implementation report and retained green bytes. Hashes identify artifacts only. Explicit-base production diff is pipeline.go; the other three files are new/untracked and were read explicitly, including both complete test files. Existing tracked documentation changes: NEXT_ACTIONS.md and OB_STATUS.md. Existing untracked artifacts: Figma, baseline, persistence review, handoff, coverage ledger, implementation report, previous same-session review, and the three candidate Go files. Full starting inventory/hashes: starting-identities.json.

The retained parent red/ copy was verified against all 208 explicit-base Git blobs; its extra portable regression file matches the candidate. That disposable copy will be used for new parent-control execution, with fresh temporary directories/logs. No parent source or checkout reversal.

Check list-target: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\list-target-execution.json; raw logs logs/list-target.stdout and .stderr.

Check list-safety: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\list-safety-execution.json; raw logs logs/list-safety.stdout and .stderr.

Check list-red: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\list-red-execution.json; raw logs logs/list-red.stdout and .stderr.

Check target: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\target-execution.json; raw logs logs/target.stdout and .stderr.

Check safety: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\safety-execution.json; raw logs logs/safety.stdout and .stderr.

Check red: FINISHED, native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\red-execution.json; raw logs logs/red.stdout and .stderr.

Check build: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\build-execution.json; raw logs logs/build.stdout and .stderr.

Check vet: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\vet-execution.json; raw logs logs/vet.stdout and .stderr.

Check format: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\format-execution.json; raw logs logs/format.stdout and .stderr.

Check version: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\version-execution.json; raw logs logs/version.stdout and .stderr.

## Contract review

The following conclusions come from the source/caller/test reads and new executions for this request. No substantive blocker was found.

| Contract | Assessment | Anchors and reasoning |
|---|---|---|
| 1. Validate participants before mutation | ACCEPTED | pipeline.go:451 copies one configured path list, reads each through keystore_validation.go:18, completes merge, then reaches pipeline.go:469. An invalid final participant returns before any publication. ReadBoundaryNoMutation and MissingMutationBoundary combine an actual publication observer with directory/file snapshots. |
| 2. Order-independent conflicts and secret handling | ACCEPTED | keystore_validation.go:131 compares exact decoded strings for the same reference; local duplicates and conflicts in either participant order refuse. Equal secrets permit deterministic metadata reconciliation. ConflictBothOrders, ConflictMutationBoundary, ExactSecretsAndLocalDuplicates and IdenticalAndMetadata exercise this. No secret/record/fingerprint is interpolated into new diagnostics. |
| 3. Truthful status | ACCEPTED within documented scope | keystore_validation.go:165 requires valid available participants, matching reference sets and no secret/metadata conflicts before ok. InvalidFinalParticipant, ConflictBothOrders, OfflineRecovery and FirstUseCompatibility check refusal. Path count is not proof of independent devices. |
| 4. Offline lookup without ambiguity | ACCEPTED for valid available replicas | pipeline.go:422 scans all successfully validated stores for the requested reference and returns no secret on observed disagreement. Missing/invalid replicas may be skipped; a usable unambiguous key remains available. OfflineRecovery, LookupIgnoresUnrelatedConflict and MetadataConflictRefused establish the intended distinction. Lookup never asserts global consistency. |
| 5. First use separate from ordinary sync | ACCEPTED | Unchanged readStore retains not-found initialization. GenerateKey (pipeline.go:385) and BuildChunk's precheck (:968) alone allow missing stores. FirstUseCompatibility really generates/reopens files; FirstBuildInitializationPrecheck reaches the existing Windows containment refusal before helper lookup. Ordinary sync refuses absent expected replicas. |
| 6. Truthful publication failure | ACCEPTED | pipeline.go:365 preserves temp-plus-rename; :469 returns zero and a wrapped OS cause on failure, warning that earlier replicas may be updated. main.go:2158 returns HTTP 400 rather than key_count. PublicationBoundary and PublicationFailureAPI inspect the advanced first store and unchanged failed second store; no rollback is asserted. |

### Validation, metadata and mutation boundary

The strict reader is distinct from initialization defaults. It handles absent/read-failed files before parsing, including a read that returns both bytes and an error. Parsing rejects duplicate fields, trailing JSON, wrong root/record shapes, unsupported top-level names and case aliases, bad markers, unsupported schema versions, invalid references/secrets and unsupported present algorithms. Nil records are safely rejected via type assertions. Missing/zero legacy schema and omitted/null key lists remain compatible with existing writer output; either recognized marker is accepted.

The schema is an array of records, not a map indexed by reference. There is no second outer reference to mismatch. Random references are not derived from secret fingerprints; imposing a new reference format or derivation rule would change compatibility. Unknown record metadata is retained. Unknown top-level fields refuse publication rather than disappear. UseNumber preserves integer precision. Same-secret metadata is unioned by field; differing overlapping values cause explicit refusal, with numeric spelling differences conservatively treated as different. Sorting records by reference makes merged output deterministic. This policy is intentionally stricter than silently picking a note/date from one store.

The validator reads only; it never creates directories, keys, staging or backup artifacts. writeStoreObserved invokes its per-App observer before MkdirAll and the temporary write. Tests arm hooks after setup, observe the actual production write path, and prove that the same observer sees both writes in a valid positive control. Snapshots cover all directory entries/file hashes beneath dedicated synthetic key roots, not merely the final destination bytes. Read injection enters ordinary validation logic. Each App belongs to its own fixture; no global hooks, shared parallel mutation or concrete cross-test interference was found. Hooks left on an unreachable fixture do not escape its lifetime.

LoadConfig's ignored errors and the validation-to-publication race remain separate limitations. Copying paths prevents rereading a changing configuration list during sync; it neither guarantees correct config loading nor locks files against other writers. Normal catalog/config/job bookkeeping is outside the no-keystore-mutation guarantee.

### Caller trace and secret output

Sync has a synchronous HTTP handler, not a job wrapper. main.go:2158 returns immediately on error; ui/index.html:2686 shows a count only on success and catches the error. GET/PUT config, GET keys and computePreflight consume strict status.

BuildChunk failures propagate through main.go:1925 into runJob. RestoreChunk lookup failure at writer.go:704 propagates via main.go:2093. runJob at main.go:556 marks execution FAILED on an operation error and preserves the separate recording-state qualification if FinishJob fails. These pre-existing PR-03 semantics were not edited. The build initialization exception still precedes the unchanged containment gate; the JSON/precheck tests do not establish GPG encryption.

QR lookup at main.go:2168 returns an error before encoding on conflict. adopt.go:96 skips unusable references when trying an encrypted manifest, while adopt.go:190 propagates the requested-reference failure. privacy.go:38 refuses encrypted-manifest lookup failure. recoverykit.go:142 omits ambiguous/unavailable key QR/sheet output and returns a warning. Existing intended generation/QR/recovery artifacts can contain secrets; that is distinct from errors/logs/status. New errors use generic classifications and OS paths/causes, without printing parsed values. Test mismatch messages do not dump secret mappings or snapshot hashes. The explicit synthetic secret literals were absent from this review's raw command output.

### Nonblocking observation

privacy.go:40 translates any lookup failure into wording that says no reachable store holds the key; recoverykit.go:144 uses similar wording but also includes the actual conflict cause. With conflicting readable stores, the text is imprecise even though the operation safely refuses or omits the key. Optional future improvement: preserve conflict-specific wording and exercise those caller diagnostics. This is not a synchronization-success, ambiguity-selection or secret-disclosure blocker, and no change was made.

### Publication and retained scope

Safe per-file replacement was not replaced with delete-first, direct destination truncation, destructive retry or rollback. Existing temporary-file behavior remains; after preflight a later failure can leave earlier stores updated. The wrapped error retains PathError information, with completed count zero. This does not establish multi-file atomicity, fsync/crash safety or cross-process exclusion.

Open scopes remain multi-store atomicity, concurrent writers and aliases, retained generations, GenerateKey partial publication/catalog coordination, broader permissions/ACLs, and config/jobs loading. Init cannot distinguish deliberate enrollment from every disappearance; that is pre-existing and does not weaken ordinary sync's strict contract. No issue was marked resolved as a whole.

## Inspected prior evidence

Read the implementation report, handoff and previous same-session review as historical inputs. Independently matched all candidate file hashes to the report and retained green bytes. Verified all 208 retained red parent files against explicit-base Git blobs and checked the portable regression file is identical to the candidate before re-executing it. Retained red and targeted-ready logs/manifests were also inspected: old red had 7 top-level failures/2 passes and 13 failing subtests; green targeted had 20/30 passes. Those are inspected-only results distinct from the new results below.

The author's suite-ready manifest/logs record native exit 0, 226 top-level passes / 0 failures / 39 skips and 40 passing subtests. No full suite was rerun for this request. The earlier parent-reopen and candidate-reopen logs each contain 19 passing executions and one Windows sharing failure at durable_completion_test.go:296. Source inspection shows the unchanged terminal-state wait may return before runJob's subsequent noteUnrecordedJob catalog audit write completes (main.go:605). The test then immediately reopens the catalog. This provides a plausible pre-existing audit-write/reopen interleaving beyond simply observing one parent failure; it is not a complete OS race diagnosis or proof about failure rates. It does not execute the changed keystore methods. The old failure remains a validation/environment limitation, not evidence of a newly introduced keystore regression.

## Remaining execution limits

Synthetic JSON/keystore/reopen/API checks do not prove encryption or full recovery integration. No GPG/PAR2 installation or invocation was needed for these selectors. Missing helpers still limit integration coverage. Windows race, ACL enforcement, crash durability, CI, Docker and hardware were not executed. Previously skipped tests are not reported as passes.


## Commands and results executed for this review

Process-selected Go 1.26.8 windows/amd64 was confirmed by go version. Command manifests record executable, arguments, cwd, process PID, start/finish, native subprocess.wait exit, and process-only settings. Existing Go cache/modules were reused; GOPROXY=off, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, fresh TEMP/TMP/GNUPGHOME under this evidence directory. No GOTMPDIR/global configuration change. The existing inspected harness was copied into this new evidence directory and pointed only at this new report; no old logs were overwritten.

All three list commands completed before tests. Exact enumerated names are in logs/list-target.stdout, list-safety.stdout and list-red.stdout. Go -list includes TestCatalogScale even with -skip; the actual safety run excluded it and emitted no outcome.

    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -list ^(TestKeystoreValidation_|TestRenameCompat_KeystoreMarkers$|TestRecoveryKitWritesKeyPages$|TestAppBackup_IncludeKeys$) .
    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -list ^(TestOpenStore_|TestAtomicRename_|TestDurableCompletion_|TestJobsUI_|TestCatalog|TestWriteCatalog_) -skip TestCatalogScale$ .
    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -list ^TestKeystoreValidation_ .
    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestKeystoreValidation_|TestRenameCompat_KeystoreMarkers$|TestRecoveryKitWritesKeyPages$|TestAppBackup_IncludeKeys$) -timeout 3m .
    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestOpenStore_|TestAtomicRename_|TestDurableCompletion_|TestJobsUI_|TestCatalog|TestWriteCatalog_) -skip TestCatalogScale$ -timeout 3m .
    "C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^TestKeystoreValidation_ -timeout 3m .
    "C:\Program Files\Go\bin\go.exe" build -mod=readonly -o C:\Users\nsott\AppData\Local\ObeliskDev\ob006-detached-review-20260919\build\obelisk.exe .
    "C:\Program Files\Go\bin\go.exe" vet -mod=readonly ./...
    gofmt -l [all root Go files; exact list in format-execution.json]
    "C:\Program Files\Go\bin\go.exe" version

Candidate checks ran in the repository; parent red/list-red ran in the verified disposable author evidence red directory. Build output went to this review evidence directory, not the checkout.

| Check | Native exit | Top-level outcomes | Subtests |
|---|---|---|---|
| list-target | 0 | {} | {} |
| list-safety | 0 | {} | {} |
| list-red | 0 | {} | {} |
| target | 0 | {"pass": 20} | {"pass": 30} |
| safety | 0 | {"pass": 39} | {"pass": 10} |
| red | 1 | {"pass": 2, "fail": 7} | {"fail": 13} |
| build | 0 | {} | {} |
| vet | 0 | {} | {} |
| format | 0 | {} | {} |
| version | 0 | {} | {} |

Both candidate selections passed with zero failures/skips. The parent failures are expected evidence that the new safety assertions detect the old destructive/ambiguous behavior: rejected/missing replicas are changed, conflict lookup returns material, and sync chooses a result instead of refusing. They are not unexplained candidate failures. The common file is unchanged in both copies; candidate-only boundary tests were not claimed to execute on parent. Read-only formatting produced empty stdout/stderr.

### target executed test names

- TestAppBackup_IncludeKeys: PASS
- TestKeystoreValidation_ReadBoundaryNoMutation: PASS
- TestKeystoreValidation_ConflictMutationBoundary: PASS
- TestKeystoreValidation_MissingMutationBoundary: PASS
- TestKeystoreValidation_PublicationBoundary: PASS
- TestKeystoreValidation_ParticipantSnapshot: PASS
- TestKeystoreValidation_LookupIgnoresUnrelatedConflict: PASS
- TestKeystoreValidation_ExactSecretsAndLocalDuplicates: PASS
- TestKeystoreValidation_FirstBuildInitializationPrecheck: PASS
- TestKeystoreValidation_UnionAndReopen: PASS
- TestKeystoreValidation_InvalidFinalParticipant: PASS
- TestKeystoreValidation_MissingDoesNotProvision: PASS
- TestKeystoreValidation_ConflictBothOrders: PASS
- TestKeystoreValidation_IdenticalAndMetadata: PASS
- TestKeystoreValidation_MetadataConflictRefused: PASS
- TestKeystoreValidation_OfflineRecovery: PASS
- TestKeystoreValidation_PublicationFailureAPI: PASS
- TestKeystoreValidation_FirstUseCompatibility: PASS
- TestRecoveryKitWritesKeyPages: PASS
- TestRenameCompat_KeystoreMarkers: PASS

### safety executed test names

- TestAtomicRename_MissingTempPreservesDestination: PASS
- TestAtomicRename_PublishesWhenDestinationAbsent: PASS
- TestAtomicRename_ReplacesExistingDestination: PASS
- TestAtomicRename_InjectedFailuresPreserveDestination: PASS
- TestAtomicRename_FailureDoesNotConsumeTemporary: PASS
- TestAtomicRename_CommentDoesNotClaimDeleteFirst: PASS
- TestOpenStore_UnreadableCatalogFailsClosed: PASS
- TestOpenStore_ZeroLengthCatalogIsDamagedNotNew: PASS
- TestOpenStore_InvalidJSONStillFailsWithDamagedMessage: PASS
- TestOpenStore_NewerSchemaStillOpensReadOnly: PASS
- TestOpenStore_FreshDirectoryStillInitializes: PASS
- TestOpenStore_NotExistShapesAllInitialize: PASS
- TestOpenStore_ExistingCatalogReopensIntact: PASS
- TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable: PASS
- TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified: PASS
- TestDurableCompletion_I_LaterSuccessfulWriteRecords: PASS
- TestDurableCompletion_I_LaterFailedWriteDoesNotRecord: PASS
- TestDurableCompletion_I_RestartWithoutRecoveryPublication: PASS
- TestDurableCompletion_J_CombinedFailureStaysObservable: PASS
- TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch: PASS
- TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded: PASS
- TestDurableCompletion_A_FinalFlushFailureIsReported: PASS
- TestDurableCompletion_A_FailedFlushJobIsNotCompleted: PASS
- TestDurableCompletion_B_BothCausesSurvive: PASS
- TestDurableCompletion_C_UnrecordedTerminalStateIsQualified: PASS
- TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch: PASS
- TestDurableCompletion_D_FlushFailureSurfacesWithAnotherBatchOpen: PASS
- TestDurableCompletion_E_SuccessfulJobIsRecorded: PASS
- TestDurableCompletion_F_UnrecordableJobStartsNoWork: PASS
- TestDurableCompletion_G_BatchDepthBookkeeping: PASS
- TestDurableCompletion_SaveJobsReportsFailure: PASS
- TestJobsUI_HonoursRecordingState: PASS
- TestJobsUI_ExecutionOutcomeTakesPrecedence: PASS
- TestWriteCatalog_SaveFailurePropagates: PASS
- TestOpenStore_RecoverySaveFailureIsNonFatal: PASS
- TestCatalogCurrentRoundTrip: PASS
- TestCatalogMigrateV1ToV2: PASS
- TestCatalogLegacyMigratesAndBacksUp: PASS
- TestCatalogNewerSchemaIsReadOnly: PASS

### red executed test names

- TestKeystoreValidation_UnionAndReopen: PASS
- TestKeystoreValidation_InvalidFinalParticipant: FAIL
- TestKeystoreValidation_MissingDoesNotProvision: FAIL
- TestKeystoreValidation_ConflictBothOrders: FAIL
- TestKeystoreValidation_IdenticalAndMetadata: FAIL
- TestKeystoreValidation_MetadataConflictRefused: FAIL
- TestKeystoreValidation_OfflineRecovery: PASS
- TestKeystoreValidation_PublicationFailureAPI: FAIL
- TestKeystoreValidation_FirstUseCompatibility: FAIL

## Final preservation and verdict

All 218 pre-existing nonignored tracked/untracked files are byte-identical to the starting hashes, including source, tests, dependencies/configuration, Figma and every prior working document/report. Figma SHA-256: 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. Branch and full HEAD unchanged; staging empty; git diff --check passed. Only this new review report was added to the checkout. Evidence: final-preservation.json and starting-identities.json outside the checkout.

READY_FOR_OWNER_REVIEW. All six bounded contracts accepted; no concrete blocking violation found. The diagnostic wording observation is optional. This is a separate review execution continuing the same conversation, with disclosed historical inputs; it is not a claim of separate model context or human certification. No unresolved access/evidence/execution limitation prevents this scoped verdict. Broader key-management residuals remain open. No implementation/test/configuration edits, issue-status updates, commits, pushes, merges, branch switches or next-task work were performed.
