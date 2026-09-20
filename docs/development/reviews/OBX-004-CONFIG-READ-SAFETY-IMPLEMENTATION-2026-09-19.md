# OBX-004 configuration read safety - 2026-09-19

READY_FOR_REVIEW - final bounded candidate; separate review pending. Earlier progress entries below are retained chronologically.

Parent: 26918c5b8ed01ea301fc9a4c7658e19054a73632
Branch: fix/obx-004-config-read-safety

Scope: fail-closed existing configuration reads/updates, explicit first-use initialization, checked staging/publication, actual caller error propagation. Preserve OB-006 and PR-01/02/03. Job loading/null-row recovery, source-boundary reconstruction, keystore transactions, ACL overhaul and tar writer remain outside scope.

Evidence directory: C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919 (raw evidence local only). Starting working-file hashes and exact pinned parent copy retained there. Figma preserved, index empty.

Planned initialization contract: ordinary LoadConfig/SaveConfig require existing valid JSON; explicit InitializeConfig / startup -init-config provides deliberate first use and refuses damaged existing input. This is a narrow startup compatibility change: a missing file alone is no longer permission to initialize. Optional-field defaults and unknown extension fields in valid input will be preserved.

Check compile-01: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\compile-01-execution.json. Raw logs local only.

Check compile-02: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\compile-02-execution.json. Raw logs local only.

Check format-write-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-write-01-execution.json. Raw logs local only.

Check target-list-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\target-list-01-execution.json. Raw logs local only.

Check red-list-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\red-list-01-execution.json. Raw logs local only.

Check red-probe-01: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\red-probe-01-execution.json. Raw logs local only.

Check target-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\target-01-execution.json. Raw logs local only.

Check format-write-02: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-write-02-execution.json. Raw logs local only.

Check all-list-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\all-list-01-execution.json. Raw logs local only.

Check target-02: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\target-02-execution.json. Raw logs local only.

Check safety-01: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\safety-01-execution.json. Raw logs local only.

## Implementation and compatibility (validation in progress)

LoadConfig now returns (Config,error). A failed existing read returns zero Config; SaveConfig performs its checked read before serialization, source-path validation, or publication. Missing expected config is an error. InitializeConfig, exposed only through the explicit startup -init-config flag, enrolls deliberate first use and returns existing valid configuration unchanged. Startup reads before OpenStore, auth-token selection, background export, or binding. An environment token does not bypass the configuration gate.

Supported optional fields retain defaults; optional null maps/slices remain accepted. Unknown extension fields survive updates, without inventing a schema-version gate. Known-field type errors, null scalars/string elements, duplicate JSON fields and competing case aliases are refused: accepting these previously let ambiguous/damaged input turn into partial defaults. One historical case-insensitive alias remains supported and is canonicalized on write. Explicit empty optional values are serialized so reopening does not resurrect a nonempty default. Settings payload shape is unchanged; invalid/truncated/oversize request bodies now fail instead of being treated as an empty update. First-use command instructions were updated narrowly in README and the installation handbook.

The per-App mutex serializes ordinary settings updates. Publication uses a unique same-directory 0600 temporary file, checked full write, file Sync, Close, rename over the destination, then directory sync. Cleanup only removes the staging file. ConfigPublicationError.Published distinguishes failure before replacement from a subsequent directory-sync error. No Config result is returned as successfully saved on either error. Runtime hash preference follows actually published bytes, including a post-publication error. Windows directory sync remains a no-op; no crash-durability guarantee, cross-process lock, retained generation or cross-file transaction is established. Tests inject each boundary into this actual publisher. Restore validates its archived config before member mutation and uses this publisher; a later restore failure can leave other members already restored. Setup still has two sequential settings publications, not an atomic transaction.

Retained persistence-review probes were inspected from persistence-20260919-131458-90213423/source/persistence_review_probe_test.go (ConfigHealthyAndInvalid, ConfigUnreadableAndMissing, ConfigHTTP); their PASS represented observed defects. The new signature-portable config_read_safety_test.go was copied byte-identically into the exact parent Git-blob reconstruction. red-probe-01 exited 1: both top-level tests and all six damaged/missing settings subcases failed as expected, including destructive replacement. The valid-update probe also detected loss of unknown fields/explicit clear. target-01 exited 0 on the candidate. Later expanded runs remain separately recorded below; no historical baseline is relabeled as current evidence.

New boundary tests arm the real mutation observer only after fixture setup, prove it fires on a successful update, inject read failures through the ordinary read decision, and compare recursive directory-entry/file-hash snapshots. Real directory-as-config read obstruction supplements injected access/I/O failures. Subprocess tests call actual main and assert exit 1 before catalog creation/binding, without printing auth values. HTTP mux, job dispatcher, auto-export, keystore, recovery kit and restore paths are exercised. Prior tests only receive checked explicit fixture initialization/signature adaptations; assertions remain intact.

## Interim executed results

- target-01: 28 top-level passes; 31 passing subtests.
- target-02: 53 top-level passes; 31 passing subtests, native exit 0. Expanded settings, setup matrix, caller, background/job and restore tests.
- safety-01: 64 top-level passes / 1 failure / 7 skips; 40 passing subtests, native exit 1. Failure: TestDurableCompletion_C_UnrecordedTerminalStateIsQualified catalog.json reopen denied by Windows sharing. This is the same unchanged test/failure retained during OB-006 (parent and candidate previously each 19/20); exact accepted-parent and current-candidate follow-up evidence will be retained. No job persistence fix is included. Seven containment cases need the missing native helper stack and did not execute.

Raw logs/manifests remain authoritative; full names are in all-list-01.stdout, and each JSON run records every selected test/subtest. A final uncached suite, build/vet/format and preservation checks remain pending.

Check format-write-03: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-write-03-execution.json. Raw logs local only.

Check red-reopen-01: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\red-reopen-01-execution.json. Raw logs local only.

Check candidate-reopen-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\candidate-reopen-01-execution.json. Raw logs local only.

Check suite-01: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\suite-01-execution.json. Raw logs local only.

Check format-write-final: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-write-final-execution.json. Raw logs local only.

Check all-list-final: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\all-list-final-execution.json. Raw logs local only.

Check target-final: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\target-final-execution.json. Raw logs local only.

Check format-write-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-write-ready-execution.json. Raw logs local only.

Check all-list-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\all-list-ready-execution.json. Raw logs local only.

Check target-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\target-ready-execution.json. Raw logs local only.

Check vet-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\vet-ready-execution.json. Raw logs local only.

Check build-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\build-ready-execution.json. Raw logs local only.

Check version-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\version-ready-execution.json. Raw logs local only.

Check suite-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\suite-ready-execution.json. Raw logs local only.

Check format-check-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\format-check-ready-execution.json. Raw logs local only.

Check whitespace-ready: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\whitespace-ready-execution.json. Raw logs local only.

## Final assessment and caller map

The bounded configuration-loading repair is complete as an uncommitted candidate. Actual repository is C:\Users\nsott\source\repos\obelisk, branch fix/obx-004-config-read-safety, HEAD/parent 26918c5b8ed01ea301fc9a4c7658e19054a73632. This parent includes accepted OB-006 implementation d147a823262757065d7817a233c0327523916e2a and its separate evidence commit. Starting state was an empty index and no tracked changes, with only docs/Obelisk.fig untracked. The new branch was created at that explicit parent; no published ref moved, fetch/pull/reset/stash/clean/rebase/merge/stage/commit/push was used. No configuration redesign or job-loading repair was performed.

| Caller / current source anchor | Expected read/update behavior | Error handling and visible result |
|---|---|---|
| main.go:72; config.go loadStartupConfig | Existing state before catalog, auth or server work; initialize only with -init-config | Fatal exit on failed read; env token cannot bypass. Explicit first use publishes checked defaults. |
| main.go:749,771 config GET/PUT | Return valid config; parse update and read prerequisite before writing | GET 503, PUT 400 on error; no saved config result on publication error. HTTP body limit 1 MiB now enforced with a non-success response. |
| setup.go ApplySetup, SetupState; main.go setup routes | Read valid existing state; preserve unrelated fields through preset/answer updates | Both writes checked; failed read/publication returns non-success. Two-write setup remains sequential, not all-or-nothing. |
| integrity.go integrityView/applyGlobalIntegrity/applyArchiveIntegrity | Checked global snapshot; compute optional archive override; validate before clearing override | Errors reach integrity routes; global failed save returns zero Integrity and error, no success log. |
| home.go HomeOverview; datamap.go DataMap; space.go SpaceAdvice; browse.go browseRoots | Derive views from validated settings | Fallible signatures, API jsonResult or existing browse error response; no default-based view on failed read. |
| main.go collection creation, barcode assignment/preview, label, finalize assessment | Read default profile/barcode/label/finalization settings before using them | 503 on failed read, collection creation guard moved before catalog mutation. |
| pipeline.go tool/Preflight/computePreflight; tools.go ToolsView | Resolve helper overrides from checked snapshot; check current config even before cached readiness | Read error returned or ok:false/error status; optional resolution uses resolveConfigTool(snapshot), never a discarded second config read. |
| smart.go VolumeHealth; main.go volume-detail/health routes; dock.go IngestDrive | Check config before helper availability and volume registration; reuse snapshot for optional health | API read failure 503; helper/device limitations remain explicit optional health results. No fallback defaults from a failed config read. |
| stenc.go stencBin/StencStatus/DriveEncryptionStatus/SetDriveKey/ClearDriveKey; main.go drive-key route | Checked snapshot before helper/device/key operations | Error or available:false/error status; no key operation after failed config read. noteTapeDriveEncryption uses the writer's validated snapshot. |
| tape.go TapeAvailable/TapeToolStatus/TapeCheck | Derive availability/device from checked snapshot | Fallible availability propagated to route; status error explicitly unavailable; check returns error. Pure status helpers prevent hidden rereads. |
| pipeline.go KeystoreStatus/GenerateKey/Passphrase/SyncKeystores | Read participant paths successfully before OB-006 validation/recovery | Config error -> not-ready status or caller error; no keystore mutation. Existing offline-recovery/conflict distinctions unchanged. |
| pipeline.go ScanFolder/Plan/BuildChunk; metadata.go extractMediaMeta | Read versions/integrity/staging settings before dependent work; metadata uses snapshot | Read errors reach API/job; no accidental defaults. Existing build, key and durability guards retained. |
| mirror.go MirrorToVolume; incremental.go BackupChanges/planDeltaPackages; plans.go ExecutePlanFromDrive | Checked throttle/planning snapshot before dependent work | Errors returned through callers to job outcome; effectiveIntegrity now takes checked Config. |
| drift.go ReconcileCollection; burner.go BurnNext; writer.go WriteChunk; span.go SpanWriteNext; finalize.go FinalizeVolume | Validate config before paths/buffers/drive/finalization decisions | Existing error-bearing return paths reach job/API boundary. No new default paths on failed read. |
| adopt.go AdoptFolder; dock.go IngestDrive/mirrorAdopt; inference.go InferStructure/leafDateRange | Read valid config before metadata helper use; pass snapshot to best-effort metadata | Failed prerequisite propagated. Missing optional metadata tools remain optional after a successful config read. |
| escrow.go FetchEscrowCache; main.go escrow GET/fetch | Read cache settings before writing; planEscrow/escrowCacheDir take snapshot | Error returned to route/job; no default cache location after failed read. |
| recoverykit.go BuildRecoveryKit | Load before output-directory creation or any kit member write | Failed read returns job error before output. Later unavailable individual keys remain warnings per existing recovery contract. |
| appbackup.go gatherMembers/RestoreAppBackup/maybeAutoExport; main.go export ticker | Export validated config; validate current/restored config; checked config replacement; validate auto-export settings | Export/restore/API/job errors propagated; automatic export returns and logs config failure. Restore publication errors explicitly warn that other members may already have restored. |
| Indirect a.tool callers: adopt.go readAdoptManifest/payloadTOC, privacy.go ReadMediumManifest, writer.go RestoreChunk; dvdisaster.go helper resolution | Existing error-returning helper resolution now carries LoadConfig failure | Required helper paths propagate error; optional device/ECC diagnostics keep their documented error/warning behavior. No helper invocation using fabricated settings. |

All direct LoadConfig/SaveConfig occurrences were enumerated against current source. Existing Config payload field names remain unchanged. Internal signature changes include fallible view functions and snapshot arguments for pure helpers; affected tests use explicit checked first-use fixtures. keystore_validation.go and keystore_validation_boundary_test.go remain byte-identical; keystore_validation_test.go changes only fixture initialization. No test assertions or safety predicates were removed.

## Final executed evidence

All commands below are completed native executions, not starts, and all manifests have FINISHED state. Go version: go1.26.8 windows/amd64; GOTOOLCHAIN=go1.26.8, CGO_ENABLED=0, GOWORK=off, GOFLAGS=-mod=readonly, GOPROXY=off; existing approved module/toolchain cache retained. No GOTMPDIR override, helper installation, toolchain/dependency upgrade, global setting or platform switch. Exact executable, argv, cwd, process settings, PID and start/finish are in each <name>-execution.json; logs/<name>.stdout and .stderr hold raw output. These raw files and candidate copies are LOCAL ONLY, not included in the patch. all-list-ready.stdout enumerates the complete final inventory before execution.

| Run | Native exit | Top-level pass/fail/skip | Subtest pass/fail/skip |
|---|---:|---|---|
| red-probe-01 | 1 | 0 / 2 / 0 | 0 / 6 / 0 |
| target-01 | 0 | 28 / 0 / 0 | 31 / 0 / 0 |
| target-02 | 0 | 53 / 0 / 0 | 31 / 0 / 0 |
| safety-01 | 1 | 64 / 1 / 7 | 40 / 0 / 0 |
| red-reopen-01 | 1 | 19 / 1 / 0 | 0 / 0 / 0 |
| candidate-reopen-01 | 0 | 20 / 0 / 0 | 0 / 0 / 0 |
| suite-01 | 0 | 239 / 0 / 39 | 72 / 0 / 0 |
| target-ready | 0 | 39 / 0 / 0 | 62 / 0 / 0 |
| suite-ready | 0 | 240 / 0 / 39 | 72 / 0 / 0 |

The repeated reopen checks are 20 executions of ONE unchanged test, not 20 distinct tests. Exact accepted-parent red-reopen-01 reproduced the earlier sharing failure (19 pass / 1 fail); candidate follow-up passed 20/20. This establishes pre-existence, not a complete diagnosis or a fix. No job/PR-03 code was altered. suite-01 passed before the final pure-helper snapshot correction; suite-ready is the final source result. Final targeted/suite outcomes overlap and must not be summed as unique coverage.

Build-ready, vet-ready, version-ready, format-check-ready and whitespace-ready each exit 0. Final gofmt -l stdout/stderr are empty (no unformatted files). Git diff --check reports no whitespace errors; Git's ordinary CRLF/LF conversion notices are retained in stderr. compile-01 found two return-shape and one test-helper signature errors; compile-02 passed. The late all-list-final/target-final attempt could not compile a redeclared handler error variable; no tests executed there. It was corrected, then all-list-ready, target-ready, build/vet and suite-ready passed. All failed/intermediate artifacts are retained rather than overwritten.

Final suite: 240 top-level passes, 0 failures, 39 skips; 72 passing subtests. 36 skips require missing GPG/PAR2; one Unix-permission fixture is unavailable on Windows; two large-scale/performance checks are opt-in. Three explicitly excluded system-disk probes: TestSmartDeviceNode_SystemDisk, TestVolumeHealth_SystemDisk, TestDeviceIdentityAndLabel_SystemDisk. Windows race (CGO disabled/no GCC), ACL enforcement, live tape/optical hardware, crash/power-loss durability, CI/Docker and unavailable integration paths are NOT established. Passing JSON-keystore checks do not turn skipped native integration tests into passes.

### Exact Go commands

Each line uses C:\Program Files\Go\bin\go.exe with the recorded process-selected settings. Names beginning red run in the reconstructed parent copy; other commands run in this checkout. The red copy contains only pinned parent blobs plus byte-identical config_read_safety_test.go.

- `all-list-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -list . .`; native exit 0.
- `all-list-final`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -list . .`; native exit 1.
- `all-list-ready`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -list . .`; native exit 0.
- `build-ready`: `"C:\Program Files\Go\bin\go.exe" build -mod=readonly -o C:\Users\nsott\AppData\Local\ObeliskDev\obx004-config-20260919\build\obelisk.exe .`; native exit 0.
- `candidate-reopen-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=20 -json -run ^TestDurableCompletion_C_UnrecordedTerminalStateIsQualified$ .`; native exit 0.
- `compile-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -run ^$ -gcflags=-e .`; native exit 1.
- `compile-02`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -run ^$ -gcflags=-e .`; native exit 0.
- `red-list-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -list ^TestConfigReadSafety_ .`; native exit 0.
- `red-probe-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^TestConfigReadSafety_ .`; native exit 1.
- `red-reopen-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=20 -json -run ^TestDurableCompletion_C_UnrecordedTerminalStateIsQualified$ .`; native exit 1.
- `safety-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestOpenStore_|TestPersistObserver_|TestAtomicRename_|TestExportAppBackup_Failed|TestRestoreAppBackup_Failed|TestDurableCompletion_|TestJobsUI_|TestKeystoreValidation_|TestAuth|TestAPIGuard|TestContainment_) .`; native exit 1.
- `suite-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -skip _SystemDisk$ ./...`; native exit 0.
- `suite-ready`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -skip _SystemDisk$ ./...`; native exit 0.
- `target-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestConfig|TestSetup|TestSettings|TestTools|TestIntegrity|TestAppBackup|TestAutoExport|TestTape|TestStenc|TestHome|TestDataMap|TestUIMode|TestBrowse) .`; native exit 0.
- `target-02`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestConfig|TestApplySetup|TestSetup|TestSettings|TestTools|TestToolBrowse|TestIntegrity|TestEffectiveIntegrity|TestAppBackup|TestAutoExport|TestTape|TestStenc|TestHome|TestDataMap|TestUIMode|TestBrowse|TestDock|TestInferStructure|TestMirror|TestPlan_|TestResolveTapeTool|TestHashAccel|TestRecoveryKit) .`; native exit 0.
- `target-final`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestConfig|TestStenc|TestSetDriveKey|TestNoteTape|TestResolveTapeTool|TestTools|TestDock|TestInferStructure|TestKeystoreValidation_) .`; native exit 1.
- `target-list-01`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -list ^(TestConfig|TestSetup|TestSettings|TestTools|TestIntegrity|TestAppBackup|TestAutoExport|TestTape|TestStenc|TestHome|TestDataMap|TestUIMode|TestBrowse) .`; native exit 0.
- `target-ready`: `"C:\Program Files\Go\bin\go.exe" test -mod=readonly -count=1 -json -run ^(TestConfig|TestStenc|TestSetDriveKey|TestNoteTape|TestResolveTapeTool|TestTools|TestDock|TestInferStructure|TestKeystoreValidation_) .`; native exit 0.
- `version-ready`: `"C:\Program Files\Go\bin\go.exe" version`; native exit 0.
- `vet-ready`: `"C:\Program Files\Go\bin\go.exe" vet -mod=readonly ./...`; native exit 0.

Formatting commands use the approved cached Go 1.26.8 gofmt.exe. Their exact file argv, including intermediate -w invocations and final -l, is retained in the corresponding format-*-execution.json; the final selected source/test file list is exactly the 64 Go paths in the identity table below. No other formatter was installed. The report's incremental Check entries were written as each command finished.

## Remaining scope and provenance

This is implementation-session evidence, not an independent review, human certification, or closure of all OBX-004. Separate review is the next action. loadJobs malformed/missing/null-row handling and its startup panic remain unchanged as the next separately authorized OBX-004 sub-scope; catalog missing/null/counter and related persistence observations remain recorded in the historical review. No job-loading fix is hidden here. Config first-use intent does not establish storage identity; concurrent processes can still replace state between validation and publication. Setup/restore are multi-publication operations; retained generations, cross-file atomicity, broader ACL protections, GenerateKey/catalog coordination, native Windows tar and PR-04 source-boundary reconstruction remain outside scope. Figma frame access and repository-wide feature/source review remain separate unfinished programs.

## Final candidate identities

Full parent: 26918c5b8ed01ea301fc9a4c7658e19054a73632. Hashes below are SHA-256 of actual working-file bytes, not normalized Git blobs. The 64 source/test files were copied to candidate-files and frozen in tested-source-identities.json, then verified unchanged after validation. The parent red copy is checked against every tracked Git blob. Current docs and the report itself are included in the external final-artifact-identities.json; the report does not embed its own recursive hash. Starting-identities.json retains every starting tracked file and Figma hash. No staging or commit was performed.

| Changed file | SHA-256 working bytes |
|---|---|
| README.md | 696f7c9fdaa3f5270edd125e117daf4245f7fbb23ea5224295c445f636d37571 |
| adopt.go | c993c006d12f4d9859e04af12f8713c535babf0e9d0917dba322b67d4792422e |
| appbackup.go | 544e653da8956c16f3f8886a6ce69f1e432bfc8a0400843690f23fb837f8894c |
| appbackup_test.go | 36f29bf5dd536923ec16dd9ad0d4d7b06d6b1ed4b0ea070d1b2620f7f804a3b7 |
| browse.go | 6919e26d8ed56e0879900474584dbcc72aabc02d95f203d431c1beaaa1658340 |
| build_verify_windows_containment_test.go | 73b8343b61b2434789414ec6eb4de8621a18b2d8d1078b9d39e84f0ee4d63bfa |
| burner.go | 62e7fd9e9fecb2b34fc459caf1f5a7057865be7add85a875d370e04e912308d2 |
| config.go | b3adaa80451eac6db939d82718ce166d6d81a45e6c4a2f7f4361b672bc2c77c3 |
| config_boundary_test.go | 5e70cd6fed9fab23d7033a8149f9498b27858645a9e38045beb36629bc72e897 |
| config_read_safety_test.go | 93ca9201dd4d0d7d4fae63abddfa143de841ee55f17b7b8fb09ddcee59b6bc9e |
| config_test_helpers_test.go | 747129bac34f36f514b665fff335c663de48242e1968af4cb5ed57d190f98e96 |
| copy_health_test.go | 18bc11f70c6cd6cc678886c713d576985104518afa1dfd89135ff8219e6a7877 |
| datamap.go | d4c8d0d7ce26eb5885e9de65a87e09af52be6ded66a03d1d78b1043e9d8b3105 |
| datamap_test.go | 299196163b678ebb55c44934f16817e5c220393a69bf515a92ccb1850dee9b81 |
| dock.go | af84d0bea8797be35047fc1eee93b4302417c9a1505707d12f6d4ffa83b5ed52 |
| dock_test.go | 579dd3dafb12b418876c7f22abf056ecc9b3e7fde1d6387aa01e3b25bfcdf0f3 |
| docs/development/CODEX_HANDOFF.md | 61a542ab6cb48e36ddc7de26379adbb95564b9d3005f1276ef2e6c692e12df74 |
| docs/development/NEXT_ACTIONS.md | 539fce8744b31da871e3882e9d2111a989d2c7738f2890e13c61fc99c8cfe682 |
| docs/development/REVIEW_COVERAGE.csv | a2381c647ac5d934e58d3f7557ff51b2042cf138691126f5929a2189776e0241 |
| docs/handbook/01-install-and-first-run.md | 80f4354112a4f482fb765927372541bf02e5e08c065fe6126980e513620445d7 |
| drift.go | ee80fe0f96d0f95661f7c46784cd1266ccb588ff47b9ba7c6cff76c0448523d9 |
| escrow.go | 1f266b780e27e5f561744c93ef6214cce68fdecd35a7c10deeb71952a1fe4d26 |
| escrow_test.go | 0c7765ed8d895d72cdbcf12a35136d3804442c96e85bba88cf23d8d721ec997c |
| exports_test.go | bd3db4109259bdf40490b7df62d773a1f958b48085c9fd252531b746facbfb1f |
| finalize.go | 9e5512861d589a9716a39ef4d140f2f998a6baaf538c3b0d6d1b520db5a98b34 |
| finalize_test.go | 0a77eb30e658317a6b1e62d3bd046a0b8c9f4d8019107794582c2002bdb38f3e |
| format_census_test.go | 1e1fa9b92681a2eed7e71dc345b9f79259e384aaef1c0fd018c56b8cebb4678f |
| generalize_test.go | 3519b41e10801fe13f14b014ce3bf6e83ac474052edfacc753cf237b7df2760f |
| home.go | 99678d7955ec0ae22091b64783b4df0a93d00bbf9981ddba4e2920c743e1a04a |
| home_test.go | dc566cf7ba73b57de2d33c57cefad5104da8fe2c04e6c4ed8bf0d34940cd5c74 |
| incremental.go | 96b3b250451e8197e8882a37a9f9a7c5bcd0cac7c91665f701b19a4e23df72a6 |
| incremental_test.go | fad33655a945b6917d2de9ababd5dc3a6c61afcca57407008e40824942a3b05a |
| inference.go | 5036685260b539657ff0ebdeb3b1faa3af830317267d724045f0ccd13c9e040c |
| integration_test.go | 20d1023e214b17c8bfc0d37d584c782229fb934fd81785185354422e8ec7f1c8 |
| integrity.go | e7f0cb521f0d81d88719957acfa53d94ca6fc109e0415b11348d43e6dbe8b4ec |
| integrity_test.go | 3e72e3938a1db5054396a75f64f738b0eee56a19ca8e7b0fec6ce6a43e2fa919 |
| keystore_validation_test.go | 878452c2cd3d7e5f3afafcbbde4aa41ae1c3c957e9242f7fd1114e5cad90b86b |
| main.go | 02706c9299b08d5cdb30d251f26655e89472f324cd6dadc0f04497ff6df27b22 |
| metadata.go | e78a2784ee0d679e16c13e820eca8747dd8bf2f9f5b011ece05b64cbd5efb00e |
| mirror.go | 1957dfc6254d0ca14f13fbe8e9878fe7fb4f4ea07eec2c38ef66b15eeef86fec |
| payload_naming_test.go | c8ca561f87f254f0d7b1a071c6f69784d1b683a3d808c35ae99bf4e0ab6ee28c |
| perf_test.go | c284955f30916954cfbcf3b2681e4211833f4d19b025f5f4b828dc08546c38be |
| pipeline.go | 09056e6275873afec4cf2f43b58bb993107813614de76455b426eabb4e7dceb4 |
| plans.go | 0d9829ef09e26041e7e684b3a7c7857d93c2141d370c12116eb918e2c10c99d5 |
| plans_test.go | 8b8989cb5eaee533a3bde2648f44b8bd84a1b5eb53a9b49082c437fda42a592a |
| quarantine_test.go | 0b59d71fe8646617441b1889d156ecddb512ccf838c1b7db6fcd660b24d0c7b7 |
| recoverykit.go | d9b2f61c206705bf54093c077db62d1b7e1198fee72a7a3f631413c792e06fdd |
| removal_test.go | 3156295ff2134330df6c388f3ace625810c942d5595ac0cddeba8168f2a8f46f |
| rename_compat_test.go | 491764e168c1d336f84da0ba07d9d59124562a656eb30ede783c470cbe1042e7 |
| seeing_what_happened_test.go | 86a775030bc82fb1404071a430bc238430b9d72e8064d46d53e69a3b64575d18 |
| settings_test.go | ef5c91bc629adfbaa305d479937e6723245d2a28951104b69ed9faf02331c668 |
| setup.go | 7be4fb9f09a33c39fe1f505e9721979474a1cd60b814d6c52a9f5a4c1ec2bce9 |
| setup_test.go | eefe9bcb661f3891823a47fa41095c78fd5eb151f022eb8bf430f172027b156d |
| smart.go | 19b3b3f6239aeef4588ec4498518562fb5c3a2758658a98e6f09a43512d5bba3 |
| smart_test.go | a05f3523aa76f09961ee02b1179786ab7cea273f024c654a9fda46128f0644f3 |
| space.go | bbe0b4e55ed48c6132e6ac8f1589c62ecd4807d0dee6125edd175077f6c6888c |
| span.go | 6a47340de6c5b6a862660f9c032e8b041cb970e254166ba6c85d6058b50783b6 |
| stenc.go | 982e6c74c9741c48bc462171acf8bb6c7a3ed76fd04a8fc178ce1cddc4307571 |
| stenc_test.go | 154662ab4c32c11e4d2522d5fe71d332218f1630dc07019300dce6893318ab8d |
| tape.go | 2919ff9bc9d03dbe2c9a4c89587bc8a74186ee15b8a70a5df381b8621978de8b |
| tape_test.go | 504be41bd4d1f12f29705b4f6c7a13a0fd6235e135b48be87b738e9cfe882c0b |
| tar_unicode_names_test.go | 0a9c662bc2a6e6e43a104b0903f6056ffbe912bd72d029ac6e5f503d9fdabc83 |
| tools.go | 033c3118c62895d619843a880f834ff6b65c279e1de0663475ad7c9c54f9bb9d |
| treemap_test.go | 4df18871ecf0c2bed641c843ba671bad5f00511b51a79953bcd0190d35a36667 |
| uimode_test.go | c9bcea36317ec710c10f311dd2f9feada3b434e62c13f03aeb952c5061e4fc15 |
| verify_levels_test.go | 6a0ec5b763c1a0e92a1b2ab466965b790e78b41e6c30a1d04e46504c4b465c3b |
| versions_test.go | 81a0b28ea4077b0c8e7f0e83021238667e416b96c6f9d0bcf5261caa366e5a2d |
| volume_identity_test.go | 8774ea98e908e0570b6d177b463d0cb1bdd2ed8df296a5716e9bde052b7a5217 |
| writer.go | 6f049544015b84ef05de41ed87fe5985a8de50b455d60fb7dd81a42b2fff8294 |

Preserved Figma: docs/Obelisk.fig, 219717 bytes, SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. Historical baseline/persistence/OB-006 implementation and both review reports remain byte-identical. Final preservation verification is retained in preservation-final.json, alongside final-artifact-identities.json and the final status/ref inventory. Only the scoped candidate and current handoff/coverage/next-action/first-use guidance have changed.

ONE next action: separate review of this exact uncommitted OBX-004 configuration candidate. Stop here; do not stage/commit/push or start the job-loading sub-scope.
