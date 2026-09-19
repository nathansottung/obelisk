# OB-006 keystore validation implementation - 2026-09-19

READY_FOR_REVIEW - uncommitted candidate; OB-006 remains partially repaired

Parent: 0dc7d5399c6889e015aebc9ba02e694a909d6e9c
Branch: fix/ob-006-keystore-validation

Scope: validate all existing sync participants before writes; detect secret conflicts; align status and read-only lookup, preserving first-use initialization. No transactional multi-store publish, prior generations, ACL, config/job repair or GenerateKey/catalog atomicity. No UI change.

Starting identities, retained parent source, raw logs and later candidate artifacts: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9

Prior reproduction: persistence-20260919-131458-90213423/source/persistence_review_probe_test.go and logs/probes.jsonl; defect-reproduction PASS is not repaired behavior. Starting phase was regression and implementation; completed evidence follows.

Check red: FINISHED; native exit 1. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\red-execution.json.

Check targeted: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\targeted-execution.json.

Check targeted-final: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\targeted-final-execution.json.

Check safety: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\safety-execution.json.

Check build: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\build-execution.json.

Check vet: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\vet-execution.json.

Check format: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\format-execution.json.

Check suite: FINISHED; native exit 1. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\suite-execution.json.

Check parent-reopen: FINISHED; native exit 1. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\parent-reopen-execution.json.

Check candidate-reopen: FINISHED; native exit 1. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\candidate-reopen-execution.json.

Check targeted-ready: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\targeted-ready-execution.json.

Check build-ready: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\build-ready-execution.json.

Check vet-ready: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\vet-ready-execution.json.

Check format-ready: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\format-ready-execution.json.

Check suite-ready: FINISHED; native exit 0. Manifest/logs: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\suite-ready-execution.json.

## Final behavior and caller contract

SyncKeystores (pipeline.go) copies one configured participant list, reads and validates every existing file, then merges validated records before its first writeStoreObserved call. No missing parent, temporary file, keystore or backup is created on validation refusal. Per-file temp-plus-rename remains unchanged; later publication errors return count 0 with the original OS cause wrapped and a warning that earlier participants may already have changed. The HTTP sync handler is synchronous, not runJob: POST /api/keys/sync returns 400/error rather than key_count on failure. The existing UI catches the error and only reports a count on success. Normal unrelated application/error bookkeeping is outside the no-keystore-mutation claim.

KeystoreStatus retains ok/reason/min_required/stores without adding response fields. It now treats missing/unreadable/invalid participants and observed secret/metadata conflicts as not OK. Replica set mismatch still advises sync. Paths are not claimed to be physically independent; duplicate/aliased participants remain an identity residual. GET /api/config, PUT /api/config, GET /api/keys and Preflight consume this status. No UI rewrite or endpoint alias change.

Passphrase reads all configured participants it can validate and compares every occurrence of the requested reference. Available equal secrets remain recoverable with an offline or invalid replica; observed unequal secrets for that reference return an error and no material. Unrelated references' conflicts do not block an unambiguous requested secret. Success establishes availability only: it does not assert the unreadable/unsupported replicas agree. QR, recovery-kit and writer decryption callers keep their existing result/error contracts. No secret, record, key reference or fingerprint is added to conflict diagnostics; structural errors do not echo JSON decoder tokens.

The strict reader is separate from unchanged readStore. Legacy first-use readStore still treats not-found as empty. GenerateKey and the encrypted BuildChunk precheck use the same validation/conflict check with missing allowed, preserving existing first-time key-generation workflows. This small caller change was necessary because BuildChunk previously consulted public KeystoreStatus before reaching GenerateKey. The helper-free Windows regression reaches the unchanged build containment refusal before tool lookup, with missing stores still absent; it is NOT an encryption round-trip test. Public status and ordinary sync never use this missing-allowed mode. Initialization still cannot distinguish deliberate enrollment from a disappeared expected location; that pre-existing residual remains.

## Validation and deterministic metadata rules

Existing files must have a recognized Obelisk or legacy marker, supported schema, and key records with nonempty string references and nonempty string secrets; present algorithms must be GPG-AES256. Legacy omitted/empty key lists remain supported, including null emitted by existing writeStore for an empty store. Unknown top-level fields are refused rather than erased. Exact top-level spelling prevents encoding/json case aliases from replacing keys. Duplicate JSON object fields are rejected at every level. Key-record extension metadata is preserved; UseNumber prevents large integer precision loss. No secret trimming, case folding or other normalization is performed.

Same reference/equal secret is not a secret conflict. Metadata is merged by field union; overlapping equal JSON values are retained. Different values for the same metadata field cause explicit metadata-conflict refusal requiring reconciliation, even if secrets match. This conservative rule never silently chooses a note/date/algorithm by path order; lookup may still return an identical secret despite a metadata disagreement. Numeric metadata spellings that decode to different json.Number values are conservatively different. Merged records sort by reference rather than the old unstable created_at ties. Schema stamping and recognized legacy-marker read compatibility retain the existing writer contract; no new persisted schema fields are introduced.

## Mutation-boundary evidence

The per-App read-result seam injects bytes AND errors into the actual production classification, including valid-looking bytes returned with permission failure. It never supplies a test-only early rejection. The per-App observer reaches the actual writeStoreObserved boundary BEFORE MkdirAll/temp writing, and is armed only after fixture creation/configuration. Hooks are nil in production; tests own distinct Apps and configure hooks before use, without global overrides or parallel mutation. Snapshot checks inventory directory entries and file hashes, not just final bytes or missing .tmp names. Real successful union/reopen is the positive control for the SAME observer: both publication attempts are seen. Real directory obstacles exercise later publication failures; error wrapping preserves the underlying PathError. Conflict tests include both orders, a conflicting last participant, local duplicate references, case differences and significant spaces. The publication test inspects the advanced first replica and unchanged failed second replica, and the API test verifies HTTP 400 without completed count. These probes need no GPG/PAR2.

## Explicit residuals

This is NOT all of OB-006 resolved: no retained prior generations, cross-file rollback/transaction, fsync/ACL redesign, cross-process ownership, filesystem identity/alias protection or race-free validation-to-publication guarantee. Files can change after preflight. GenerateKey still publishes sequentially, returns an error after partial writes, and can report success after AddKeyMeta drops catalog persistence failure. Its random reference collision policy is unchanged. Config load (OBX-004), job load, general metadata mutators and full backup/restore generation safety remain untouched. Windows ACL/race/hardware and crash durability are NOT TESTED; no claim is inherited from build success. Missing PR-04 remains unavailable; reconcile identity/ownership overlap when transferred. Figma and feature-matrix work remain later tasks.

## Earlier suite failure retained

The first candidate suite exited 1: 224 top-level pass / 1 fail / 39 skip, separately 40 subtest pass. The failing unchanged TestDurableCompletion_C_UnrecordedTerminalStateIsQualified hit a Windows catalog reopen sharing violation at durable_completion_test.go:296. A bounded count=20 comparison reproduced the same signature on both pristine parent production and candidate: each 19 pass / 1 fail, native exit 1. This establishes pre-existence; source inspection suggests asynchronous audit logging/reopen interleaving, but does not prove a complete race diagnosis. No prior safety code/test was weakened. The final suite is rerun because the encrypted-build first-use caller changed afterward, not to erase this failure. Prior logs and all intermediate command manifests are retained.

## Final identities and validation

Exact parent: 0dc7d5399c6889e015aebc9ba02e694a909d6e9c. Active branch: fix/ob-006-keystore-validation. HEAD remains the parent; source changes are an UNCOMMITTED CANDIDATE, not covered by the earlier pinned-source review.

| Changed source/test file | Candidate SHA-256 |
|---|---|
| pipeline.go | a0cbb5a031afdb869cdb7ae74277f40fd393303c84c121619e00be391a49dec6 |
| keystore_validation.go | 29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593 |
| keystore_validation_test.go | 10bb54072f30fe7ce4d088d42bad941ed084c28f1ef6438a6f6d748522c9d7a0 |
| keystore_validation_boundary_test.go | 2b0122a48d6969336e77fb8657d918c7fe2e1390e6310f0cf394c07c7032db24 |

Full retained final source: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\green. Pristine parent plus the identical portable safety regression file: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-20260919-133203-d7aa49d9\red. The common keystore_validation_test.go has the same bytes in both copies; the per-App seam/first-build boundary tests exist only in the candidate and are not claimed as RED-executed on the parent. All parent production files in red are unchanged Git blobs. Starting identities and final candidate identities are retained as JSON. Intermediate runs preceded final caller/test refinements and are not substituted for final-candidate evidence.

| Run | Native exit | Top-level outcomes | Subtest outcomes |
|---|---|---|---|
| red | 1 | {"fail": 7, "pass": 2} | {"fail": 13} |
| targeted-ready | 0 | {"pass": 20} | {"pass": 30} |
| safety | 0 | {"pass": 39} | {"pass": 10} |
| build-ready | 0 | {} | {} |
| vet-ready | 0 | {} | {} |
| format-ready | 0 | {} | {} |
| suite | 1 | {"fail": 1, "pass": 224, "skip": 39} | {"pass": 40} |
| parent-reopen | 1 | {"fail": 1, "pass": 19} | {} |
| candidate-reopen | 1 | {"fail": 1, "pass": 19} | {} |
| suite-ready | 0 | {"pass": 226, "skip": 39} | {"pass": 40} |

All listed commands have FINISHED state, observed native exits and terminal package results where applicable. The repeated reopen rows count 20 executions of ONE named test, not 20 distinct tests. Focused safety ran before the final first-build compatibility adjustment; the final targeted check and uncached suite exercise the final source and retain prior safety tests. Final format stdout and stderr are empty (zero flagged files). Full-suite Go default 10m timeout retained; targeted/repetition timeout 3m. No baseline count was used as a pass target. Suite tool sessions: initial 34803, final 86910, both polled to native completion without duplicate launch. Runner records exact executable/arguments/cwd/environment/start/finish/PID and separate stdout/stderr for each command.

The parent RED portable assertions: 2 top-level pass / 7 fail; 13 failing subtests; package FAIL, native exit 1. The meaningful failures include destructive sync of invalid/missing replicas, conflict-order behavior, metadata loss, and newly required honest status/publication qualification. This is distinct from the old defect-reproduction PASS. Final targeted: 17 new JSON/precheck regressions plus 3 affected existing tests, all pass; 30 subtests pass. No GPG/PAR2 invocation is needed by these tests.

## Exact selected test names

### targeted-ready

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
### safety

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

## Final suite skips and evidence limits

- TestIntegration_AdoptHandMadeTar
- TestIntegration_DeepAdoptEnumeratesContents
- TestIntegration_AdoptOwnChunkIsDuplicate
- TestBuildVerify_CatchesCorruptTar
- TestBuildVerify_CatchesBadEncryption
- TestBuildVerify_FullAttestation
- TestBuildVerify_FastModeSkipsAndWarns
- TestContainment_RefusesWindowsBuildWhenVerificationDisabled
- TestContainment_RefusesBeforeKeyGeneration
- TestContainment_WatchDetectsTarInvocation
- TestContainment_VerifyingTiersStillBuild
- TestContainment_NonWindowsPathUnaffected
- TestContainment_WrongFileCannotCompleteUnverified
- TestContainment_VerifierStillRejectsMismatchedArchive
- TestCardCheck_UnlistableDirBlocksFormat
- TestCatalogScale
- TestCopyLevelVerifyAndRewrite
- TestBuildChunk_TerminalStageWriteIsDurabilityGate
- TestIntegration_KeystoreEnforcement
- TestIntegration_FullChainCorruptRepairRestore
- TestIntegration_PlaintextPackage
- TestIntegration_PrivacyMode
- TestIntegration_Spanning
- TestIntegration_SourceSafetyRefusals
- TestIntegration_Throttle
- TestIntegration_VersionRetentionRoundTrip
- TestFastArchiveAttestsReducedIntegrity
- TestPayloadNamingEndToEnd
- TestSpannedPayloadNaming
- TestSeeingWhatHappened_BuildArtifact
- TestBuildRestore_HostileFilenamesRoundTrip
- TestBuildFilelist_IsNulDelimited
- TestBuildChunk_UnicodeFilenames_ExactMembers
- TestBuildChunk_WrongFileSelection_LookalikeNeighbour
- TestBuildChunk_UnicodeSourceRoot
- TestBuildChunk_UnicodeStagingDir
- TestBuildRestore_UnicodeRoundTrip
- TestTreeExpansionBudget
- TestWriteChunk_FailedFinalFlushRecordsNoCopy

39 top-level skips: 36 missing-helper gates (both GPG/PAR2 still unavailable; the first reported helper varies with map iteration), one POSIX directory-permission test, and opt-in scale/performance tests. Three explicit system-disk probes remain excluded with -skip _SystemDisk$ and produce no emitted outcome. Encryption, Unicode tar compatibility, full repair/restore integration, Windows race/ACL, hardware, Docker and CI remain unvalidated. The retained initial suite failure and matched parent reproduction remain a separate prior timing/validation issue; the final PASS does not erase it.

## Review request

ONE next action: an independent review of this candidate's participant validation, secret/metadata conflict policy, initialization-versus-status behavior and no-mutation evidence. Do not stage/commit/push or start another fix during this task. Unresolved GenerateKey partial-publication and catalog-metadata-loss findings remain explicitly open. No broader issue is closed, no GUI implemented, and repository-wide review remains incomplete.
