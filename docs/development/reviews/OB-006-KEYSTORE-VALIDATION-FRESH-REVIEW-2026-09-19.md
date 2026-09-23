# OB-006 keystore validation review - 2026-09-19

REVIEW_INCOMPLETE - fresh-session independence not established; substantive checks finished

AI review, not human certification. Fresh-session independence is NOT established: this conversation retains the authoring context. This report will distinguish new execution and source assessment from author-reported evidence.

Root: C:\Users\nsott\source\repos\obelisk
Branch: fix/ob-006-keystore-validation
HEAD / explicit review base: 0dc7d5399c6889e015aebc9ba02e694a909d6e9c
Index empty; no merge/rebase/cherry-pick/sequencer operation found. FETCH_HEAD alone is not an active operation. No applicable AGENTS.md found in repository or ancestor chain. No existing OB-006 fresh review found.

Evidence directory: C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919
Starting hashes for every tracked/untracked nonignored file: starting-identities.json. Production diff against explicit base is pipeline.go; additional candidate source/tests are the three new keystore_validation*.go files. Existing modified NEXT_ACTIONS.md/OB_STATUS.md and untracked baseline/review/handoff/ledger/Figma are preserved.

## Candidate SHA-256 identities

- pipeline.go: a0cbb5a031afdb869cdb7ae74277f40fd393303c84c121619e00be391a49dec6
- keystore_validation.go: 29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593
- keystore_validation_test.go: 10bb54072f30fe7ce4d088d42bad941ed084c28f1ef6438a6f6d748522c9d7a0
- keystore_validation_boundary_test.go: 2b0122a48d6969336e77fb8657d918c7fe2e1390e6310f0cf394c07c7032db24

All four match the implementation report and retained green source bytes.

Check list-target: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\list-target-execution.json; raw logs logs/list-target.stdout and .stderr.

Check list-safety: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\list-safety-execution.json; raw logs logs/list-safety.stdout and .stderr.

Check list-red: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\list-red-execution.json; raw logs logs/list-red.stdout and .stderr.

Check target: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\target-execution.json; raw logs logs/target.stdout and .stderr.

Check red: FINISHED, native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\red-execution.json; raw logs logs/red.stdout and .stderr.

Check safety: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\safety-execution.json; raw logs logs/safety.stdout and .stderr.

Check build: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\build-execution.json; raw logs logs/build.stdout and .stderr.

Check vet: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\vet-execution.json; raw logs logs/vet.stdout and .stderr.

Check format: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\format-execution.json; raw logs logs/format.stdout and .stderr.

Check version: FINISHED, native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\ob006-review-20260919\version-execution.json; raw logs logs/version.stdout and .stderr.

## Source assessment and contract dispositions

No implementation blocker found in this bounded slice. The six dispositions below are technical acceptances based on the source and new executions described here, not fresh-session independence or human certification.

| Contract | Disposition | Source and regression evidence |
|---|---|---|
| All-participant validation before keystore mutation | ACCEPTED | pipeline.go:451; keystore_validation.go:18; ReadBoundaryNoMutation, MissingMutationBoundary, ParticipantSnapshot |
| Conflict detection independent of participant order | ACCEPTED | keystore_validation.go:131; ConflictBothOrders, ConflictMutationBoundary, ExactSecretsAndLocalDuplicates |
| Truthful status | ACCEPTED within replica-validation contract | keystore_validation.go:165; InvalidFinalParticipant, ConflictBothOrders, OfflineRecovery, FirstUseCompatibility |
| Offline recovery without observed-secret ambiguity | ACCEPTED for readable, structurally valid stores | pipeline.go:422; OfflineRecovery, LookupIgnoresUnrelatedConflict, MetadataConflictRefused |
| First-use compatibility | ACCEPTED for JSON generation and Windows build precheck | pipeline.go:385 and :968; FirstUseCompatibility, FirstBuildInitializationPrecheck; encryption not executed |
| Accurate partial-publication failure reporting | ACCEPTED | pipeline.go:365 and :476; main.go:2158; PublicationBoundary and PublicationFailureAPI |

### Validation and schema

SyncKeystores copies one LoadConfig participant list. Every listed store must pass readExistingKeystore before merge; every merge must finish before the first writeStoreObserved. Early refusal need not inspect later files once a prerequisite already failed. Zero paths refuse. Missing replicas are not enrollment; errors request reconnection or separate provisioning. Error-plus-bytes reads are rejected. The strict helper has no filesystem mutation: reading, parsing and building maps are its only work. LoadConfig error suppression remains unchanged; a snapshot neither fixes that issue nor excludes other writers.

The helper rejects malformed/trailing JSON, duplicate object fields, nonobject roots, unknown/case-aliased top-level fields, bad markers, negative/future schema versions, nil records, absent/nonstring/empty reference or passphrase, and unsupported present algorithms. It accepts either marker=1 and absent/zero legacy schemas; omitted/null key lists remain compatible with the existing writer. Unknown key-record metadata remains round-trippable. The persisted schema is a LIST of records, not an outer reference-indexed map, so there is no outer-key/inner-reference mismatch to validate. Inventing a K- prefix or deriving the reference from a fingerprint would be a new requirement: GenerateKey uses a random reference independent of the secret hash.

Whole-store validity applies to lookup too: an invalid sibling record or unsupported extension causes that participant to be skipped. This conservative compatibility boundary is documented in the implementation report and does not claim to salvage partially invalid stores. It can reduce availability for nonconforming/manual files; supported generated/legacy records remain accepted.

### Secrets, metadata and status

Merge keys by exact decoded reference and compares exact string secrets before field union. Conflicts within one store or in a final participant fail before publication. No case folding, trimming, secret fingerprinting, or arbitrary first/last selection is introduced. Compatible metadata fields are unioned; differing overlapping values refuse sync, including conservative differences in json.Number spelling. Nested metadata uses DeepEqual; large integers are not converted to float64. Sorting by reference makes publication deterministic. Unknown top-level fields refuse rather than disappear; recognized marker/schema stamping follows the existing writer contract.

Status preserves ok/reason/min_required/stores. Unavailable/invalid replicas, unequal reference sets, or observed secret/metadata conflicts prevent ok. Same-secret metadata disagreement is explicitly distinct from secret ambiguity. Counting paths still does not prove physical independence or distinct files; duplicate/aliased paths are a named residual.

Passphrase scans all valid available participants for the requested reference, returning no secret on observed disagreement. It may stop as soon as a conflict proves refusal. Unrelated references do not block an unambiguous requested key. Missing or invalid redundant replicas do not independently block a valid available key, and successful lookup is not advertised as global consistency.

Error strings carry classification and filesystem causes, not parsed records, secrets or fingerprints. OS path diagnostics remain paths, not redacted arbitrary user data. Tests use synthetic secrets, generic mismatch diagnostics, and never print snapshot hashes or maps containing secrets. New retained execution output was checked for the explicit synthetic secret literals. Secret-returning generation and QR/recovery artifacts are existing intended interfaces, separate from errors/status/logs.

### Callers, first use and publication

POST /api/keys/sync (main.go:2158) calls synchronously, returns HTTP 400/error and exits before key_count on failure. It is not a runJob path. UI ui/index.html:2686 only shows the count on success and catches errors. GET/PUT config, GET keys, and computePreflight consume public strict status. GenerateKey alone and BuildChunk's generation precheck pass allowMissing=true. Public missing-store status is false while direct generation provisions valid new stores. The Keys UI does not disable a generation API based on strict status; normal generation is automatic during build. The unchanged Windows containment refusal still precedes helper lookup, staging and generation. The build-precheck regression proves reaching that refusal, not encryption success.

Lookup callers: main.go:2168 returns a QR error; writer.go:704 and adopt.go:190 propagate lookup failure before decrypt; adopt.go:96 skips failed candidates and tries other references; privacy.go:38 refuses the encrypted-manifest operation; recoverykit.go:142 omits unavailable/ambiguous key material and returns warnings. Those last two retain imprecise missing-key wording (optional improvement below). None selects a secret after a lookup conflict.

writeStoreObserved observes before MkdirAll, then keeps the existing 0600 temporary-file write and os.Rename. No delete-first, direct destination truncation, retry deletion or rollback was added. Failure returns count zero with the original wrapped cause and explicit warning that earlier participants may have changed. PublicationBoundary checks errors.As(PathError), two observed attempts, union in the first file and unchanged second bytes. PublicationFailureAPI exercises the actual mux and asserts 400/no completed count. Application config/catalog/job bookkeeping is outside the no-keystore-mutation claim.

### Test seams and regression quality

Both new files were read explicitly, including all helpers. newKV finishes filesystem/configuration setup before the per-App observer/read injection and snapshot. The observer is at the production mutation boundary; the same observer then detects both successful real writes in the positive control. Injection supplies the actual read result to ordinary parsing/classification, not a test-only refusal. Directory-entry and file-content snapshots supplement observation and include parents/staging artifacts beneath the dedicated keystore root.

Hooks belong to each test's App, with no global registry, parallel hook writes or escaped shared fixture. The read hook is cleared for the positive control; other hooks end with the fixture lifetime. Leaving a hook set on an unreachable test-local App has no concrete interference path. Snapshot-test config mutation deliberately demonstrates that the original participant set remains authoritative for publication. Real reopen assertions check mappings, and partial-publication tests inspect resulting files before cleanup.

The portable test file is identical on red and candidate. Candidate-only mutation-boundary tests are not claimed to run on the parent. The older persistence probe source was read: it explicitly expects rejected participants to be overwritten and lookup precedence to change. Its PASS is evidence of the old defect, not the repair.

## Earlier Windows sharing failure

The author's initial suite and count=20 control logs were read. Parent and candidate each had 19 passing executions and one failure of TestDurableCompletion_C_UnrecordedTerminalStateIsQualified at durable_completion_test.go:296 opening catalog.json, with the same Windows sharing-violation signature. This is not enough to estimate failure rates or prove that every timing effect is unchanged.

The source provides additional evidence: waitTerminalJob (durable_completion_test.go:102) returns when terminal state becomes visible; FinishJob (store.go:3688) publishes that state and releases the job lock before runJob calls noteUnrecordedJob (main.go:605). That function then invokes Store.Log (store.go:1581), which can still save/rename catalog.json while the test immediately calls OpenStore. Neither this path nor its test changed, and the test executes scanning/job completion rather than keystore methods. This supports classification as a pre-existing timing/environment limitation, with a plausible audit-log/reopen interleaving. The exact Windows cause is not fully instrumented; no statistical claim or complete race diagnosis is made. The newly executed 39-test safety selection passed. The old failure remains visible rather than erased by this pass.

## Optional improvements and retained scope

Optional: preserve conflict-specific lookup wording at privacy.go:40 and recoverykit.go:144. Today a known ambiguity can be described as no reachable key, although the operation still refuses or omits the material; the recovery-kit warning includes the underlying conflict. A future narrow diagnostic improvement could preserve the cause and assert conflict wording through those callers. This does not compromise the bounded no-winner/no-mutation contract and is not a blocker.

Open scopes remain multi-file transaction/rollback, prior generations, filesystem identities, concurrent writers/validation-to-publication races, GenerateKey partial publication and dropped AddKeyMeta persistence failures, permission/ACL policy, and config/jobs load errors. No issue status was updated and no prior PR-01/02/03 behavior was reclassified as fully repaired.


## Newly executed verification
Go version execution confirms go1.26.8 windows/amd64. Process-selected environment is recorded in every manifest; CGO_ENABLED=0, GOWORK=off, GOFLAGS=-mod=readonly, isolated TEMP/TMP/GNUPGHOME, existing module/build cache, GOPROXY=off. No GOTMPDIR or global environment modification. No helper installation or personal data.
Before running targeted selectors, list-target/list-safety/list-red completed with native exit 0. Go -list also listed TestCatalogScale despite -skip; it was explicitly excluded from the subsequent safety execution and emitted no test outcome. Exact names and outcomes follow.
| Command | Native exit | Top-level outcomes | Subtests |
|---|---|---|---|
| target | 0 | {"pass": 20} | {"pass": 30} |
| red | 1 | {"pass": 2, "fail": 7} | {"fail": 13} |
| safety | 0 | {"pass": 39} | {"pass": 10} |
| build | 0 | {} | {} |
| vet | 0 | {} | {} |
| format | 0 | {} | {} |
| version | 0 | {} | {} |

Both candidate selections have no failures/skips: 20 top-level + 30 subtests for focused keystore/affected checks; 39 top-level + 10 subtests for prior safety. Red has 2 top-level passes, 7 failures and 13 failing subtests. Its failures specifically show invalid/missing replicas mutated and same-reference ambiguity accepted in both orders. This is a newly executed unsafe-parent control, not inherited defect-probe PASS. Parent copy red/ was independently reconstructed from 208 Git blobs; red-identities.json records blob IDs/SHA-256, with only the common regression file added. Build/vet succeeded; formatting was read-only (-l), with empty stdout/stderr. Native test exits are from subprocess.wait and persisted, not inferred from shell status.

### target exact executed top-level names

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

### safety exact executed top-level names

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

### red exact executed top-level names

- TestKeystoreValidation_UnionAndReopen: PASS
- TestKeystoreValidation_InvalidFinalParticipant: FAIL
- TestKeystoreValidation_MissingDoesNotProvision: FAIL
- TestKeystoreValidation_ConflictBothOrders: FAIL
- TestKeystoreValidation_IdenticalAndMetadata: FAIL
- TestKeystoreValidation_MetadataConflictRefused: FAIL
- TestKeystoreValidation_OfflineRecovery: PASS
- TestKeystoreValidation_PublicationFailureAPI: FAIL
- TestKeystoreValidation_FirstUseCompatibility: FAIL

## Reported-only evidence and limits

No full uncached suite was rerun in this review: the author reports 226 top-level pass / 0 fail / 39 skip, separately 40 passing subtests. The retained suite-ready execution manifest records FINISHED/native exit 0. These figures are not relabeled as this review's execution. The author excluded three _SystemDisk$ probes. Missing GPG/PAR2 continues to limit encryption/archive/repair/restore integration; JSON lookups and QR/backup fixtures are not decryption evidence. Windows race, ACL enforcement, crash durability, CI, Docker and hardware were not executed. Cross-process safety is not established.

## Final preservation and disposition

All 217 original tracked/untracked nonignored files match their starting SHA-256 hashes. Source, tests, dependencies, original implementation/persistence/baseline reports, handoff/ledger/status/next-actions and Figma bytes are unchanged. Figma SHA-256: 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. Branch/HEAD/index unchanged; git diff --check passed. Only this review report was added in the checkout. External review logs/runner/parent copy and final-preservation.json are retained in the evidence directory. Nothing staged, committed, pushed, merged or fixed.

REVIEW_INCOMPLETE solely for the requested fresh-session independence. The substantive code/test review is finished, with no code blocker found and all six bounded contracts technically accepted above. This is an AI review performed in a conversation retaining authoring context, NOT a completed fresh-session independent review or human certification. No source correction is requested by these findings.

ONE next action: review these exact candidate hashes in a new conversation without the authoring history, using this report as disclosed prior evidence rather than independent certification. No next-task work was executed.
