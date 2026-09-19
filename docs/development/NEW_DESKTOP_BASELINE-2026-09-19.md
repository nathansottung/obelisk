# New-desktop Windows baseline - 2026-09-19

BASELINE RECORDED

Source: 0dc7d5399c6889e015aebc9ba02e694a909d6e9c
Branch: setup/windows-nsott
Root: C:\Users\nsott\source\repos\obelisk

PR-04 candidate not yet transferred. Figma design context pending. Repository-wide review not completed. Neither missing item blocks this baseline.

Execution directory: C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9
Manifest: C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\execution-manifest.json

All baseline checks and final summary complete; next stage is bounded persistence source review.

Initial checkout: no staged/tracked changes; only untracked docs/Obelisk.fig (219717 bytes), SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda.

Selected environment: process-scoped Go 1.26.8; actual GOVERSION/GOROOT/GOMOD/GOWORK/CGO_ENABLED in environment.stdout. go.mod directive 1.22.2; CI selects stable; Dockerfile selects golang:1.22-alpine. No dependency upgrades or global changes.

Settings:
~~~json
{
  "GOTOOLCHAIN": "go1.26.8",
  "GOMODCACHE": "C:\\Users\\nsott\\AppData\\Local\\ObeliskDev\\audit-2026-09-19\\go-mod",
  "GOCACHE": "C:\\Users\\nsott\\AppData\\Local\\ObeliskDev\\baseline-20260919-125535-898459a9\\go-cache",
  "GOFLAGS": "-mod=readonly",
  "CGO_ENABLED": "0",
  "GOPROXY": "https://proxy.golang.org",
  "GOSUMDB": "sum.golang.org",
  "GONOSUMDB": "",
  "GONOPROXY": "",
  "GOPRIVATE": "",
  "GOWORK": "off",
  "GOOS": "windows",
  "GOARCH": "amd64",
  "TEMP": "C:\\Users\\nsott\\AppData\\Local\\ObeliskDev\\baseline-20260919-125535-898459a9\\tmp",
  "TMP": "C:\\Users\\nsott\\AppData\\Local\\ObeliskDev\\baseline-20260919-125535-898459a9\\tmp",
  "GNUPGHOME": "C:\\Users\\nsott\\AppData\\Local\\ObeliskDev\\baseline-20260919-125535-898459a9\\gnupg",
  "GOTMPDIR": null,
  "GOROOT": null,
  "OBELISK_PERF": null,
  "OBELISK_SCALE": null,
  "OBELISK_FILES": null,
  "OBELISK_MIRROR": null,
  "OBELISK_WIDE": null
}
~~~

| Check | State | Native exit | stdout | stderr |
|---|---|---|---|---|
| environment | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\environment.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\environment.stderr |
| packages | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\packages.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\packages.stderr |
| build | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\build.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\build.stderr |
| vet | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\vet.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\vet.stderr |
| format | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\format.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\format.stderr |
| tests | FINISHED | 0 | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\tests.stdout | C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\logs\tests.stderr |

Safety: no TestMain found. newIT/nativeTools/newTestApp use temporary synthetic catalogs, staging, keystores and source fixtures; TEMP/TMP and GNUPGHOME are isolated. No app main launch. Tape/ECC cases use parsers or absent-helper refusal. Synthetic volume tests may query read-only OS device metadata; no raw device or personal-file access. Three explicit hardware probes are excluded with -skip=_SystemDisk$: TestSmartDeviceNode_SystemDisk, TestVolumeHealth_SystemDisk, TestDeviceIdentityAndLabel_SystemDisk. They are NOT TESTED, not counted as emitted skips. Default Go test timeout remains 10m. GOTMPDIR is absent. No helper installation/substitution.

Helper paths and test pins:
~~~json
{
  "git": "C:\\Program Files\\Git\\cmd\\git.EXE",
  "go": "C:\\Program Files\\Go\\bin\\go.EXE",
  "tar": "C:\\Windows\\system32\\tar.EXE",
  "gpg": null,
  "par2": null,
  "node": "C:\\Program Files\\nodejs\\node.EXE",
  "gcc": null,
  "smartctl": null,
  "stenc": null,
  "dvdisaster": null,
  "ffprobe": null,
  "exiftool": null,
  "docker": null,
  "xorriso": null,
  "ltfs": null,
  "mt": null,
  "sg_logs": null,
  "gpg_test_pin_exists": false,
  "par2_test_pin_exists": false
}
~~~

Windows race NOT TESTED (GCC absent; CGO_ENABLED=0). No CI, Docker, browser, Blu-ray, LTO or hardware-write result is implied. Missing GPG/PAR2 limits integration coverage. Historical 240/7/4 is not this machine's result.

## Recorded results

Baseline execution is complete. No build, vet or test failures were observed. All four requested checks exited 0. Formatting stdout/stderr are empty: zero flagged filenames across 137 tracked Go files. A passing process does not make skipped coverage pass.

Top-level tests: 209 PASS / 0 FAIL / 39 SKIP (248 emitted outcomes). Subtests: 10 PASS / 0 FAIL / 0 SKIP, counted separately. Three system-disk tests deliberately excluded; no result emitted. All started tests have terminal outcomes. One package PASS, elapsed 44.531 seconds; process exit 0 observed. No package/build failures; tests.stderr is empty. Tool session 77782 was polled to completion, never duplicated.

36 helper skips (33 first reported GPG, 3 first reported PAR2); both tools are missing, and nativeTools uses a map, so the first missing helper is not the only unmet prerequisite. Remaining skips: POSIX permission behavior, opt-in catalog scale, opt-in tree performance. Known OBX-006 Unicode build/restore tests skipped before reaching their historical failure paths: this is not evidence of repair. Missing helpers also block corrupt-repair-restore, encryption/privacy, spanning, adoption, keystore enforcement, full build attestations, source safety refusals and copy/write integration gates. Node-backed job UI tests did execute, but this is not browser rendering or accessibility evidence.

Environment observations: Windows version API reports Microsoft Windows NT 10.0.19045.0; windows/amd64. Git 2.55.0.windows.3; Node v24.19.0; native tar C:\Windows\system32\tar.exe is bsdtar 3.5.2 / libarchive 3.5.2 (zlib/1.2.5.f-ipp). Windows ANSI code page 1252, distinct from terminal UTF-8 encoding. Default installed Go was 1.27.0; all baseline Go commands explicitly selected 1.26.8 (verified in environment.stdout).

Exact commands, arguments, working directory, process settings, start/finish timestamps and native PIDs/exits are in execution-manifest.json. The Python runner was invoked from PowerShell; subprocess.wait() captured the native child return code directly, independently of logging or the runner's exit. RUNNING was persisted before spawning; FINISHED only after wait returned. Each command updated this report and the handoff before the next began. Tool orchestration sessions: packages 63920, build 46564, tests 77782.

## Preservation and next stage

All 208 starting tracked file SHA-256 identities matched before the authorized NEXT_ACTIONS append, including production source, tests, UI, build/CI configuration and go.mod/go.sum. Final verification permits only that append among originally tracked files. Branch, full HEAD, staging and Figma SHA-256 remain unchanged. Old audit-2026-09-19/logs/toolchain.txt was never opened for writing. Initial and final identities are retained locally. No historical review report was edited.

Minimum prerequisites for further integration validation: real GPG and PAR2, then a separately authorized helper-dependent validation run; no installation was performed here. Windows race additionally needs a compatible C compiler and CGO enabled in its own process. Permission coverage needs appropriate platform execution; scale/performance need their explicit gates. Hardware probes/writes, Docker and CI remain NOT TESTED.

ONE next action: begin a bounded persistence source-review pass at this same pinned checkpoint and start the review ledger; do not restart this baseline. PR-04 transfer and Figma frame access remain separately pending. No review coverage ledger or feature matrix was created, and test execution is not substantive source review.

## Top-level test outcomes

| Test | Result | Skip/failure reason |
|---|---|---|
| TestIntegration_AdoptHandMadeTar | SKIP | adopt_test.go:49: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_DeepAdoptEnumeratesContents | SKIP | adopt_test.go:105: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_AdoptOwnChunkIsDuplicate | SKIP | adopt_test.go:156: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestAPIGuard_SameOriginWithHeaderPasses | PASS |  |
| TestAPIGuard_SameOriginPostPasses | PASS |  |
| TestAPIGuard_MissingHeaderForbidden | PASS |  |
| TestAPIGuard_ForeignOriginForbidden | PASS |  |
| TestAPIGuard_ForeignRefererForbidden | PASS |  |
| TestAPIGuard_SameOriginNavigationDownloadAllowed | PASS |  |
| TestAPIGuard_CrossOriginNavigationForbidden | PASS |  |
| TestAPIGuard_NonAPIPassesThrough | PASS |  |
| TestAppBackup_ExportRestoreRoundTrip | PASS |  |
| TestAppBackup_IncludeKeys | PASS |  |
| TestAppBackup_TamperedMemberRefused | PASS |  |
| TestAppBackup_NewerSchemaRefused | PASS |  |
| TestAtomicRename_MissingTempPreservesDestination | PASS |  |
| TestAtomicRename_PublishesWhenDestinationAbsent | PASS |  |
| TestAtomicRename_ReplacesExistingDestination | PASS |  |
| TestAtomicRename_InjectedFailuresPreserveDestination | PASS |  |
| TestAtomicRename_FailureDoesNotConsumeTemporary | PASS |  |
| TestExportAppBackup_FailedPublishPreservesPreviousBundle | PASS |  |
| TestRestoreAppBackup_FailedPublishPreservesExistingFile | PASS |  |
| TestAtomicRename_CommentDoesNotClaimDeleteFirst | PASS |  |
| TestIsLocalhostAddr | PASS |  |
| TestAuthMiddleware | PASS |  |
| TestWriteBagItTags | PASS |  |
| TestExportBagConformant | PASS |  |
| TestExportPackageBag | PASS |  |
| TestBagEncodePath_EncodesOnlyWhatRFC8493Requires | PASS |  |
| TestBagPayloadManifest_HostileNamesStayOneLinePerFile | PASS |  |
| TestWriteBagItTags_ManifestOnDiskIsEncoded | PASS |  |
| TestBuildVerify_CatchesCorruptTar | SKIP | build_verify_test.go:101: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildVerify_CatchesBadEncryption | SKIP | build_verify_test.go:131: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildVerify_FullAttestation | SKIP | build_verify_test.go:155: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildVerify_FastModeSkipsAndWarns | SKIP | build_verify_test.go:202: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_RefusesWindowsBuildWhenVerificationDisabled | SKIP | build_verify_windows_containment_test.go:96: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_RefusesBeforeKeyGeneration | SKIP | build_verify_windows_containment_test.go:174: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_WatchDetectsTarInvocation | SKIP | build_verify_windows_containment_test.go:214: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_VerifyingTiersStillBuild | SKIP | build_verify_windows_containment_test.go:237: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_NonWindowsPathUnaffected | SKIP | build_verify_windows_containment_test.go:267: required tool "par2" not found (pin C:/Tools/par2/par2.exe missing, not on PATH) |
| TestContainment_WrongFileCannotCompleteUnverified | SKIP | build_verify_windows_containment_test.go:295: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_VerifierStillRejectsMismatchedArchive | SKIP | build_verify_windows_containment_test.go:331: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestContainment_ProductionPredicateFollowsGOOS | PASS |  |
| TestCardCheck_BackedUpVsNew | PASS |  |
| TestCardCheck_SafeToFormat | PASS |  |
| TestCardCheck_UnhashableFileBlocksFormat | PASS |  |
| TestCardCheck_UnlistableDirBlocksFormat | SKIP | cardcheck_test.go:147: POSIX directory permissions |
| TestCardCheck_MatchesDriveSnapshot | PASS |  |
| TestPersistObserver_SeesRealMutations | PASS |  |
| TestPersistObserver_SeesBackupPrune | PASS |  |
| TestOpenStore_UnreadableCatalogFailsClosed | PASS |  |
| TestOpenStore_ZeroLengthCatalogIsDamagedNotNew | PASS |  |
| TestOpenStore_InvalidJSONStillFailsWithDamagedMessage | PASS |  |
| TestOpenStore_NewerSchemaStillOpensReadOnly | PASS |  |
| TestOpenStore_FreshDirectoryStillInitializes | PASS |  |
| TestOpenStore_NotExistShapesAllInitialize | PASS |  |
| TestOpenStore_ExistingCatalogReopensIntact | PASS |  |
| TestCatalogScale | SKIP | catalog_scale_test.go:22: set OBELISK_SCALE=1 to run the large-catalog benchmark |
| TestConflictClasses_DetectResolveAndPlanGate | PASS |  |
| TestConflictResolution_NotReopened | PASS |  |
| TestCopyLevelVerifyAndRewrite | SKIP | copy_health_test.go:21: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestDataMap_WritesAndNever | PASS |  |
| TestDataMap_MissingFlags | PASS |  |
| TestDataMap_VerifyPointersAreReal | PASS |  |
| TestWhereScreenWiredInUI | PASS |  |
| TestDockIngest_TwoDrivesSequentialAndReinsert | PASS |  |
| TestBuildChunk_TerminalStageWriteIsDurabilityGate | SKIP | durability_gate_test.go:21: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestAppendVerifyEventErr_RollsBackOnSaveFailure | PASS |  |
| TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable | PASS |  |
| TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified | PASS |  |
| TestDurableCompletion_I_LaterSuccessfulWriteRecords | PASS |  |
| TestDurableCompletion_I_LaterFailedWriteDoesNotRecord | PASS |  |
| TestDurableCompletion_I_RestartWithoutRecoveryPublication | PASS |  |
| TestDurableCompletion_J_CombinedFailureStaysObservable | PASS |  |
| TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch | PASS |  |
| TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded | PASS |  |
| TestDurableCompletion_A_FinalFlushFailureIsReported | PASS |  |
| TestDurableCompletion_A_FailedFlushJobIsNotCompleted | PASS |  |
| TestDurableCompletion_B_BothCausesSurvive | PASS |  |
| TestDurableCompletion_C_UnrecordedTerminalStateIsQualified | PASS |  |
| TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch | PASS |  |
| TestDurableCompletion_D_FlushFailureSurfacesWithAnotherBatchOpen | PASS |  |
| TestDurableCompletion_E_SuccessfulJobIsRecorded | PASS |  |
| TestDurableCompletion_F_UnrecordableJobStartsNoWork | PASS |  |
| TestDurableCompletion_G_BatchDepthBookkeeping | PASS |  |
| TestDurableCompletion_SaveJobsReportsFailure | PASS |  |
| TestJobsUI_HonoursRecordingState | PASS |  |
| TestJobsUI_ExecutionOutcomeTakesPrecedence | PASS |  |
| TestNormBurnEcc | PASS |  |
| TestEccMethodArgAndFileName | PASS |  |
| TestEccGenArgs | PASS |  |
| TestDeriveOpticalDevice | PASS |  |
| TestBurnEccConfigPredicates | PASS |  |
| TestGenerateDiscEcc_MissingToolIsNonFatal | PASS |  |
| TestNormEscrowMode | PASS |  |
| TestManifestExcludesLTFS | PASS |  |
| TestPlanBinariesOnlyVsFull | PASS |  |
| TestPlanReaderSelectionAndGating | PASS |  |
| TestWriteBundleAssemblesAndVerifies | PASS |  |
| TestWriteBundleOffIsSkipped | PASS |  |
| TestSidecarEscrowBudgetsHonestly | PASS |  |
| TestRecoveryKitCarriesEscrow | PASS |  |
| TestInferStructure_OrganizedTree | PASS |  |
| TestClusterEvents_ChaoticDrive | PASS |  |
| TestMagnet_SuggestsStrayIntoHarvestedEvent | PASS |  |
| TestExportImport_StructureAndPlanRoundTrip | PASS |  |
| TestOpticalRestoreNote | PASS |  |
| TestDefaultBurnerIsXorriso | PASS |  |
| TestDriveEncryptionWarnsInKit | PASS |  |
| TestFinalizeSealsAndWritesSidecar | PASS |  |
| TestFinalizeBlockedUnlessForced | PASS |  |
| TestFormatCensusTiers | PASS |  |
| TestFormatRegistryUserOverride | PASS |  |
| TestMusicianProject_RolesCriticalAndRouting | PASS |  |
| TestHashFileBothMatchesSHA256 | PASS |  |
| TestScanRecordsBlake3 | PASS |  |
| TestBlake3NeverOnMedia | PASS |  |
| TestHome_NASOnly | PASS |  |
| TestHome_ShoeboxOnly | PASS |  |
| TestHome_Mixed | PASS |  |
| TestIncremental_VolumeBaseOnlyDelta | PASS |  |
| TestIncremental_ProtectionBaseExcludesComplete | PASS |  |
| TestIncremental_FeedsHomeRecognition | PASS |  |
| TestIntegration_KeystoreEnforcement | SKIP | integration_test.go:264: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_FullChainCorruptRepairRestore | SKIP | integration_test.go:286: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_PlaintextPackage | SKIP | integration_test.go:323: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_PrivacyMode | SKIP | integration_test.go:361: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_Spanning | SKIP | integration_test.go:396: required tool "par2" not found (pin C:/Tools/par2/par2.exe missing, not on PATH) |
| TestIntegration_SourceSafetyRefusals | SKIP | integration_test.go:500: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_Throttle | SKIP | integration_test.go:553: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestIntegration_VersionRetentionRoundTrip | SKIP | integration_test.go:615: required tool "par2" not found (pin C:/Tools/par2/par2.exe missing, not on PATH) |
| TestIntegrityPresetLabels | PASS |  |
| TestEffectiveIntegrityArchiveOverride | PASS |  |
| TestFastArchiveAttestsReducedIntegrity | SKIP | integrity_test.go:69: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestMirror_CopyVerifyTreeAndCoverage | PASS |  |
| TestMirror_RefusesSourceDest | PASS |  |
| TestMirror_ConcurrentMultiVolume | PASS |  |
| TestMirror_IdempotentAndMultiFolderTree | PASS |  |
| TestLineCodeIsCRC16 | PASS |  |
| TestRecoveryKitWritesKeyPages | PASS |  |
| TestKeyPageHTMLPrintable | PASS |  |
| TestKeySheetRoundTrip | PASS |  |
| TestKeySheetCatchesTypo | PASS |  |
| TestPayloadNamingEndToEnd | SKIP | payload_naming_test.go:112: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestSpannedPayloadNaming | SKIP | payload_naming_test.go:246: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestPerfMeterMovingAverage | PASS |  |
| TestPerfMeterThrottleMarker | PASS |  |
| TestPerfEndpointFastNoCatalogLock | PASS |  |
| TestWriteCatalog_SaveFailurePropagates | PASS |  |
| TestOpenStore_RecoverySaveFailureIsNonFatal | PASS |  |
| TestPlan_ExecuteReverseAcrossRestart | PASS |  |
| TestPlan_DriveDiffersFromSnapshot | PASS |  |
| TestCanonicalPhotographyExample | PASS |  |
| TestProfileResolutionNearestAncestor | PASS |  |
| TestBuiltinProfilesImmutableAndInUseGuard | PASS |  |
| TestQuarantine_ManagedTerritoryRoundTrip | PASS |  |
| TestQuarantine_AbsentOnAdoptedAndRefusedForSources | PASS |  |
| TestQuarantine_ReconcileHumanRemoved | PASS |  |
| TestRetireRoundTrip | PASS |  |
| TestRemoveEmptyArchive | PASS |  |
| TestRemoveGuardedKeepsVolumes | PASS |  |
| TestRenameCompat_DataDirFallback | PASS |  |
| TestRenameCompat_MigrateCopiesVerifiesAndSwitches | PASS |  |
| TestRenameCompat_KeystoreMarkers | PASS |  |
| TestRenameCompat_ManifestMarkers | PASS |  |
| TestRenameCompat_SidecarDirsRecognized | PASS |  |
| TestRenameCompat_StructurePlanImportMarkers | PASS |  |
| TestRenameCompat_AppBackupLegacyFormat | PASS |  |
| TestRenameCompat_LegacyEncryptedPayloadRecognized | PASS |  |
| TestRenameCompat_RestoreTxtNotRequired | PASS |  |
| TestRingCopy_ShortSourceSurfacesReadError | PASS |  |
| TestRingCopy_WholeFileCopyIsClean | PASS |  |
| TestScanFolder_CollectsProblemsAndExcludesFromCount | PASS |  |
| TestCatalogCurrentRoundTrip | PASS |  |
| TestCatalogMigrateV1ToV2 | PASS |  |
| TestCatalogLegacyMigratesAndBacksUp | PASS |  |
| TestCatalogNewerSchemaIsReadOnly | PASS |  |
| TestSeeingWhatHappened_ScanArtifactsAndValidation | PASS |  |
| TestSeeingWhatHappened_InterruptedReconcile | PASS |  |
| TestSeeingWhatHappened_BuildArtifact | SKIP | seeing_what_happened_test.go:250: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestSystemMemory_RealNumbers | PASS |  |
| TestBrowseWithFiles | PASS |  |
| TestToolsCatalog | PASS |  |
| TestToolBrowseToBinary | PASS |  |
| TestHashAccelToggle | PASS |  |
| TestSettingsConfigFields | PASS |  |
| TestLabelSizeParts | PASS |  |
| TestSettingsUIReachable | PASS |  |
| TestApplySetup_SkipDefaults | PASS |  |
| TestApplySetup_DataKindTemplate | PASS |  |
| TestApplySetup_LocationArchiveAndNav | PASS |  |
| TestApplySetup_TargetRequirements | PASS |  |
| TestApplySetup_IntegrityPresetKnobs | PASS |  |
| TestSetupState_RoundTrip | PASS |  |
| TestApplySetup_PreservesUnrelated | PASS |  |
| TestApplySetup_MatrixCoherent | PASS |  |
| TestParseSmart_AtaBadSectorsAdvises | PASS |  |
| TestParseSmart_FailingRaisesAdvisory | PASS |  |
| TestParseSmart_HealthyNvmeNoAdvisory | PASS |  |
| TestParseSmart_UnusableIsError | PASS |  |
| TestDriveSnapshot_OfflineBrowseMirrorAndLocationVerdict | PASS |  |
| TestExtractShotMeta_JPEG | PASS |  |
| TestSourcelessArchiveUnionAndLocations | PASS |  |
| TestLocationOffsiteFlipRehomesVolumes | PASS |  |
| TestParseStenc_On | PASS |  |
| TestParseStenc_Off | PASS |  |
| TestParseStenc_KeyLoadedNotEncrypting | PASS |  |
| TestParseStenc_Unrecognised | PASS |  |
| TestEncValueOnAndFirstInt | PASS |  |
| TestStencInstallHintIsOSAware | PASS |  |
| TestStencAvailability | PASS |  |
| TestNoteTapeDriveEncryption_NonFatalWhenAbsent | PASS |  |
| TestSetDriveKey_RequiresTool | PASS |  |
| TestParseTapeinfo | PASS |  |
| TestParseTapeinfo_HealthyNoFlags | PASS |  |
| TestParseSgLogs | PASS |  |
| TestParseITDT | PASS |  |
| TestParseHpLtt | PASS |  |
| TestParseTape_GarbageIsError | PASS |  |
| TestResolveTapeTool_ConfigOverride | PASS |  |
| TestClassifyFlag_UnknownIsWarn | PASS |  |
| TestBuildRestore_HostileFilenamesRoundTrip | SKIP | tar_names_test.go:80: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildFilelist_IsNulDelimited | SKIP | tar_names_test.go:145: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildChunk_UnicodeFilenames_ExactMembers | SKIP | tar_unicode_names_test.go:239: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildChunk_WrongFileSelection_LookalikeNeighbour | SKIP | tar_unicode_names_test.go:265: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildChunk_UnicodeSourceRoot | SKIP | tar_unicode_names_test.go:292: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildChunk_UnicodeStagingDir | SKIP | tar_unicode_names_test.go:310: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestBuildRestore_UnicodeRoundTrip | SKIP | tar_unicode_names_test.go:330: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |
| TestFolderTreeLevels | PASS |  |
| TestFolderTreePaginationWindow | PASS |  |
| TestTreeExpansionBudget | SKIP | tree_test.go:116: set OBELISK_PERF=1 to run the tree-expansion budget test |
| TestTreemapLevelsAndSizes | PASS |  |
| TestTreemapWorstStatusRollupAndDrift | PASS |  |
| TestTreemapFoldsSmallChildren | PASS |  |
| TestUIMode_DefaultAndMergePersist | PASS |  |
| TestSampleLevelMissesMiddleCorruption | PASS |  |
| TestLevelBSatisfiesFullVerify | PASS |  |
| TestUpsertFileRetainsVersions | PASS |  |
| TestVersionsRetainedCap | PASS |  |
| TestFileVersionsLocate | PASS |  |
| TestReconcileModifiedShowsPriorVersion | PASS |  |
| TestSelectVersionAsOf | PASS |  |
| TestRetainedVersionsMD | PASS |  |
| TestNextBarcode | PASS |  |
| TestVolumeLabelHTML | PASS |  |
| TestRingCopy_FailedFinalFlushIsNotSuccess | PASS |  |
| TestCopyFile_FailedFinalFlushIsNotSuccess | PASS |  |
| TestWriteChunk_FailedFinalFlushRecordsNoCopy | SKIP | writer_flush_test.go:89: required tool "gpg" not found (pin C:/Program Files/GnuPG/bin/gpg.exe missing, not on PATH) |

## Subtest outcomes (separate)

| Subtest | Result |
|---|---|
| TestAtomicRename_InjectedFailuresPreserveDestination/permission_denied | PASS |
| TestAtomicRename_InjectedFailuresPreserveDestination/cross-device_link | PASS |
| TestAtomicRename_InjectedFailuresPreserveDestination/sharing_violation | PASS |
| TestAtomicRename_InjectedFailuresPreserveDestination/read-only_filesystem | PASS |
| TestOpenStore_UnreadableCatalogFailsClosed/permission_denied | PASS |
| TestOpenStore_UnreadableCatalogFailsClosed/io_error | PASS |
| TestOpenStore_UnreadableCatalogFailsClosed/partial_bytes_with_an_error | PASS |
| TestOpenStore_NotExistShapesAllInitialize/bare_sentinel | PASS |
| TestOpenStore_NotExistShapesAllInitialize/PathError | PASS |
| TestOpenStore_NotExistShapesAllInitialize/wrapped | PASS |
