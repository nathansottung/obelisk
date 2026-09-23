# OBX-004 focused R1/R2 recheck - 2026-09-19

READY_FOR_OWNER_REVIEW

| Finding | Disposition |
|---|---|
| R1 / P1: initialization overwrites late-arriving configuration | CLOSED |
| R2 / P2: container first-use instructions omit initialization | CLOSED |

Reviewer provenance: Codex, fresh source inspection and native execution in the shared working repository. The controlling detached review and author follow-up were inputs. No human certification, separate model context, independent agent, or context isolation is claimed. REVIEW ONLY. No implementation changes.

No remaining blocker was found in this focused response. Closure is bounded to the no-replace initialization contract and documented container lifecycle; it is not repository-wide certification or container-runtime approval.

## Candidate and scope

Branch: `fix/obx-004-config-read-safety`. Full HEAD and overall patch base: `26918c5b8ed01ea301fc9a4c7658e19054a73632`. Index empty; no merge/rebase/cherry-pick/revert/bisect/sequencer operation found. No AGENTS.md found in the ancestor chain or repository, including hidden paths. The starting inventory contains 229 tracked/untracked files. All match the author's retained `final-identities.json`, including the three new initialization tests and follow-up report. The full working-byte SHA-256 manifest is appended below.

Controlling report: `OBX-004-CONFIG-READ-SAFETY-DETACHED-REVIEW-2026-09-19.md`. Response: `OBX-004-CONFIG-READ-SAFETY-REVIEW-FOLLOWUP-2026-09-19.md`. Both actual repository reports were read and preserved. Overall diff was captured against the explicit base, never main. Follow-up comparison used the retained pre-edit candidate under `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\red`, verified against every entry in `pre-identities.json`. These are the prior UNCOMMITTED implementation bytes, not simply the Git parent.

The observed follow-up list agrees with the report: config.go; three new config_init tests; README and first-run handbook; Dockerfile/Compose comments; CODEX_HANDOFF, NEXT_ACTIONS and REVIEW_COVERAGE; and the new author follow-up report. Dockerfile and Compose nonblank non-comment lines exactly match their pre-follow-up copies. No further decoding, ordinary-update or caller-propagation implementation change was found in that comparison. The previously accepted broader audit was not restarted.

## R1: CLOSED

Anchors: `config.go:142` (InitializeConfig), `config.go:268` (publisher), `config.go:310` (real link), `config.go:320` (checked staging removal), `config.go:324` (ordinary rename), `pipeline.go:149` (configuration path); `config_init_safety_test.go:14` and `:105`, `config_init_fault_test.go:10`, `config_init_cli_test.go:21`, `config_boundary_test.go:191`.

Initialization serializes defaults, creates unique staging in the configuration's own directory, checks full Write, Sync and Close, then calls `os.Link(f.Name(), a.configPath())`. The exact successful-publication point is successful hard-link creation: the complete closed staging file acquires the final config.json pathname. No second existence check substitutes for publication, and there is no replacement/deletion/truncation fallback. Go 1.26.8's actual Windows implementation (`src/os/file_windows.go:308`) converts the destination/source arguments and calls `syscall.CreateHardLink(newname, oldname, 0)`; the Unix implementation (`file_unix.go:404`) delegates to `syscall.Link(oldname, newname)`. This inspection does not constitute Unix runtime execution.

Real filesystem tests passed for late valid and malformed files, directory entries and symlinks, preserving bytes or entry identity and leaving no staging artifacts. They coordinate arrival through createTemp after the real absence read, then execute production writes/sync/close/link. The absent-destination positive control also passed. The symbolic-link case executed; it was not skipped.

A separate reviewer test in a disposable byte-preserved candidate copy exercised dangling symlinks both already present at the initial read and arriving during createTemp. Both passed using the real publication operation. Although target-following ReadFile reports not-exist for the dangling target, hard-link creation refuses the existing directory entry. The link text survives, the missing target remains missing, no staging remains, Config is zero, Published=false, and runtime hash acceleration is unchanged. Only `reviewer_dangling_test.go` was added to that disposable copy; repository tests were not edited.

Pre-publication create/write/short-write/sync/close and unsupported-link failures passed. The unsupported-link case is an injected OS-operation failure, not a mounted unsupported-filesystem experiment. It preserves the wrapped cause, produces zero Config with Published=false, does not adopt defaults, and never enters rename. Real collisions independently prove destination preservation.

After successful linking, checked removal targets only the additional staging pathname, not config.json. A cleanup failure returns Published=true with a specific wrapped cleanup cause; a subsequent deferred best-effort removal may succeed. A persistent OS cleanup refusal can leave the owned staging name, as the author documents. No test is claimed to reproduce persistent OS cleanup refusal. Directory-sync failure also returns Published=true. Both fault cases retain a readable published configuration and update runtime preference only because publication actually occurred. They do not remove config.json to simulate rollback. Returning zero Config with an error does not erase the explicit Published classification or imply no file exists. Windows syncDir remains a no-op; no power-loss guarantee is established.

Ordinary SaveConfig and restored-config updates still call the replacement branch. Existing successful replacement, failure stages, field preservation, reopen and restore regression tests passed. Existing valid sequential initialization returns the read configuration without publication; actual CLI tests preserve non-default bytes. Damaged, empty and directory-obstructed configuration refuses initialization before catalog creation. The obstruction case is not an ACL test.

### Storage support boundary

The hard-link requirement applies to the application-state/configuration filesystem selected by `-data` (container `/data`), because staging and config.json share that directory. It is not imposed on source disks, backup destinations, Blu-ray or tape. Existing valid configuration returns before the link operation; ordinary updates retain their rename requirements. There is no universal rule that every launch or backup destination requires hard links.

README:181-183 states the first-use requirement and safe unsupported-storage refusal; handbook:60-61 also states it in its container guidance. For Compose this applies to the filesystem exposed through `./data:/data`; for docker run it applies to the named-volume filesystem. Host filesystem branding alone does not prove that a container bind-mount implementation supports hard links. Unsupported mounts are not certified by these examples. The primitive and same-volume restriction agree with [Microsoft CreateHardLinkW documentation](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-createhardlinkw), inspected during this recheck. No second publication backend is required for this bounded closure.

Optional documentation improvement, not a blocker: repeat the storage prerequisite next to the native first-launch command, since its current prominent explanation sits in the container sections. The implemented scope and the existing first-use requirement are clear enough to establish the supported boundary; no storage backend is silently advertised as having a fallback.

## R2: CLOSED

Anchors: README:168-245; handbook:48-112; Dockerfile:37-40; docker-compose.yml:12-34; main.go:56-78 and startup authentication selection; config_init_cli_test.go:21.

Both documents build this checkout into the exact image tag used by Compose and specify `--pull never`; they do not assume published latest includes the feature. Both bootstrap paths supply full `-listen 127.0.0.1:7821 -data /data -init-config` arguments and the supported synthetic-in-tests/private-.env-in-guidance token mechanism. The existing ENTRYPOINT accepts these arguments. Bootstrap has no published ports. The text explicitly says initialization continues serving, instructs `docker stop obelisk-init` in another terminal, and requires waiting for the foreground command to return before normal startup.

The docker-run examples reuse mnemo-data:/data and mnemo-staging:/staging. Compose uses the actual obelisk service and its ./data:/data binding from the same project directory; the documentation explicitly distinguishes this from named-volume storage. Normal commands retain data/listen/auth settings without -init-config. Compose's ordinary restart policy remains unchanged; the one-off run uses --rm. Command override, inherited volumes, suppressed service ports, and --rm overriding restart behavior agree with the [official Compose run reference](https://docs.docker.com/reference/cli/docker/compose/run/), inspected during this recheck. No permanent initialization flag or volume deletion is introduced.

The documents distinguish deliberate defaults beside catalog/key files from recovery of original auth, helpers, keystore paths and staging preferences. The CLI existing-state case confirms config is added while catalog/key bytes survive and old auth/keystore choices are not rediscovered. Reconnecting storage or restoring original configuration remains the recovery guidance.

Docker is absent from PATH. No Docker installation, container build/run, Compose execution or new filesystem mount occurred. Source inspection and passing shell syntax checks establish documentation consistency, not deployed container behavior.

## Executed reviewer evidence

Local evidence directory: `C:\Users\nsott\AppData\Local\Temp\obx004-recheck-mvu85i_7`.

`runner.py` and per-command JSON retain exact argv, cwd, environment overrides, start/finish times, PID and native exit; logs retain stdout/stderr. Process-selected Go 1.26.8 windows/amd64, GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off. Existing approved module/toolchain cache was read. TEMP/TMP/GNUPGHOME/CLI evidence and the successful runs' fresh GOCACHE reside under the disposable evidence directory. No GOTMPDIR override, installations, global settings, WSL switch, real catalog/key/media or personal keyring access.

| Check | Native exit | Result |
|---|---:|---|
| Selected Go version | 0 | go1.26.8 windows/amd64 |
| Initial target enumeration using author's build-cache path | 1 | Access denied opening cache entry; setup failure, no tests executed |
| Target enumeration with fresh disposable cache | 0 | 39 selected names enumerated before execution |
| Focused candidate tests | 0 | 39 top-level passes; 48 passing subtests; no failures/skips |
| Pre-follow-up red enumeration / execution | 0 / 1 | Positive control and directory arrival pass; valid/damaged/symlink arrivals reproduce overwrite |
| Reviewer dangling-entry enumeration / execution | 0 / 0 | 1 top-level pass; 2 subtest passes; no skips |
| Standalone build | 0 | Passed |
| go vet ./... | 0 | Passed |
| Selected-toolchain gofmt -l, four follow-up Go files | 0 | Empty output; read-only |
| git diff --check | 0 | Passed; line-ending notices retained |
| Initial Git Bash -n, both extracted example scripts | 3221225794 each | Sandbox signal-pipe creation failed before syntax validation |
| Same Bash -n checks outside sandbox | 0 each | Both passed; syntax only, no script execution |

The cache/setup and shell-launch failures are retained, not counted as candidate test failures or discarded. The shell retry used the already installed Git Bash through approved escalation; no tool was installed.

Target selector: `^(TestConfig|TestSettings|TestApplySetup|TestSetup|TestAtomicRename_|TestRestoreAppBackup_Failed|TestAppBackup_)`. Enumeration: `go test -list <selector> .`. Execution: `go test -count=1 -json -timeout 3m -run <selector> .`. Pre-follow-up selector: `^TestConfigInit_`; reviewer selector: `^TestReviewer_`, each enumerated then run with `-count=1 -json -timeout 2m`. Standalone build: `go build -o <evidence>/build/obelisk.exe .`. Vet: `go vet ./...`. Exact expanded commands follow below.

The red copy contains all 225 pre-follow-up inventoried files with matching SHA-256, plus the current portable config_init_safety_test.go. This is a byte-preserved pre-follow-up production replay with an added regression, not a reconstruction by reverting current code or mutating the publisher. Its one failing top-level test has three failing arrival subtests; the positive control and directory subtest pass. The test's runtime-preference assertion text says unpublished defaults, but on the red implementation replacement actually published those defaults: the decisive safety failure is clobbering the arriving settings. Current real-link behavior passes all four arrivals.

### Actual CLI evidence

The repository regression builds the real binary using the selected toolchain. Each child gets an explicit disposable -data path, ephemeral 127.0.0.1 address, synthetic token and GNUPGHOME. It checks HTTP root 200 twice for serving cases, imposes bounded observation, kills the exact child, and waits for completion before reusing its data. Nine JSON/log pairs under cli/lifecycle-* retain exact arguments, PIDs, timestamps, exits, readiness and before/after inventories.

| Case | Actual result |
|---|---|
| Fresh ordinary startup | Natural exit 1; no config/catalog created |
| Fresh -init-config | HTTP 200, continues serving; config/catalog created; forced stop and wait |
| Ordinary restart | HTTP 200; fixture hashes unchanged; forced stop and wait |
| Existing non-default valid config with -init-config | HTTP 200; fixture hashes unchanged; forced stop and wait |
| Malformed / empty config with -init-config | Each natural exit 1; original bytes unchanged |
| Directory obstruction with -init-config | Natural exit 1; obstruction unchanged; no catalog created |
| Existing synthetic catalog/key state, missing config | Ordinary launch naturally exits 1; existing hashes unchanged |
| Same existing state with deliberate -init-config | HTTP 200; only defaults config added; forced stop and wait |

All four serving child processes returned native exit 1 after forced termination on this Windows run. These are not natural successful exits. All five refusals naturally returned 1 with forced_stop=false. No recorded CLI PID remained alive at final inspection. This confirms native serving/stop/wait/restart semantics, not Docker signal handling or runtime deployment.

## Reported-only evidence and residuals

The author's 244 top-level passes / 39 skips and 84 passing subtests are reported evidence only; this recheck did not repeat the full suite or sum overlapping selections into one. Earlier review runs and sharing-failure investigations remain historical evidence. No new conclusion about absence of Windows timing issues follows from these passes.

Hard-link unsupported-filesystem behavior was injected, not exercised on removable/network filesystems. Persistent cleanup denial, Windows ACL enforcement, race detector, power loss, CI, Linux/macOS execution, Docker runtime and hardware remain unverified. Missing GPG/PAR2 integration coverage remains limited; JSON/configuration tests are not native encryption/recovery integration evidence. Broader storage identity, malicious same-principal interference, multi-process update coordination, retained generations, cross-file restore/setup transactions, job loading and tape remain outside scope. No optional comment-only review loop or additional backend is requested.

## Exact candidate test names

All PASS:

```
TestAppBackup_ExportRestoreRoundTrip
TestAppBackup_IncludeKeys
TestAppBackup_TamperedMemberRefused
TestAppBackup_NewerSchemaRefused
TestAtomicRename_MissingTempPreservesDestination
TestAtomicRename_PublishesWhenDestinationAbsent
TestAtomicRename_ReplacesExistingDestination
TestAtomicRename_InjectedFailuresPreserveDestination
TestAtomicRename_FailureDoesNotConsumeTemporary
TestRestoreAppBackup_FailedPublishPreservesExistingFile
TestAtomicRename_CommentDoesNotClaimDeleteFirst
TestConfigBoundary_RefusedReadsObserveNoMutation
TestConfigBoundary_ExplicitInitialization
TestConfigBoundary_CheckedPublication
TestConfigBoundary_RealReadError
TestConfigBoundary_BackgroundAndKeystoreRefusal
TestConfigBoundary_JobRecordsReadFailure
TestConfigBoundary_OptionalToolsUseValidatedSnapshot
TestConfigBoundary_RestoreUsesCheckedPublisher
TestConfigBoundary_HTTPPublicationFailure
TestConfigBoundary_InvalidRequestNoMutation
TestConfigBoundary_StartupRefusesBeforeCatalogOrBind
TestConfigBoundary_OptionalDefaultsAndReopen
TestConfigInitCLI_Lifecycle
TestConfigInit_CheckedFailureStages
TestConfigInit_LateArrivalPreserved
TestConfigInit_NoReplacePositiveControl
TestConfigReadSafety_SettingsRejectDamagedPrerequisite
TestConfigReadSafety_ValidUpdatePreservesFields
TestSettingsConfigFields
TestSettingsUIReachable
TestApplySetup_SkipDefaults
TestApplySetup_DataKindTemplate
TestApplySetup_LocationArchiveAndNav
TestApplySetup_TargetRequirements
TestApplySetup_IntegrityPresetKnobs
TestSetupState_RoundTrip
TestApplySetup_PreservesUnrelated
TestApplySetup_MatrixCoherent
```

Exact test/subtest terminal events are retained in target-outcomes.json, red-outcomes.json and probe-outcomes.json.

## Exact executed commands

- `version`: `"C:\Program Files\Go\bin\go.exe" version`; native exit 0.
- `target-list`: `"C:\Program Files\Go\bin\go.exe" test -list ^(TestConfig|TestSettings|TestApplySetup|TestSetup|TestAtomicRename_|TestRestoreAppBackup_Failed|TestAppBackup_) .`; native exit 1.
- `target-list-v2`: `"C:\Program Files\Go\bin\go.exe" test -list ^(TestConfig|TestSettings|TestApplySetup|TestSetup|TestAtomicRename_|TestRestoreAppBackup_Failed|TestAppBackup_) .`; native exit 0.
- `target`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 3m -run ^(TestConfig|TestSettings|TestApplySetup|TestSetup|TestAtomicRename_|TestRestoreAppBackup_Failed|TestAppBackup_) .`; native exit 0.
- `red-list`: `"C:\Program Files\Go\bin\go.exe" test -list ^TestConfigInit_ .`; native exit 0.
- `red`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 2m -run ^TestConfigInit_ .`; native exit 1.
- `probe-list`: `"C:\Program Files\Go\bin\go.exe" test -list ^TestReviewer_ .`; native exit 0.
- `probe`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 2m -run ^TestReviewer_ .`; native exit 0.
- `build`: `"C:\Program Files\Go\bin\go.exe" build -o C:\Users\nsott\AppData\Local\Temp\obx004-recheck-mvu85i_7\build\obelisk.exe .`; native exit 0.
- `vet`: `"C:\Program Files\Go\bin\go.exe" vet ./...`; native exit 0.
- `format`: `C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\gofmt.exe -l config.go config_init_safety_test.go config_init_fault_test.go config_init_cli_test.go`; native exit 0.
- `whitespace`: `git diff --check`; native exit 0.
- `syntax-readme`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/Temp/obx004-recheck-mvu85i_7/readme.sh`; native exit 3221225794.
- `syntax-handbook`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/Temp/obx004-recheck-mvu85i_7/handbook.sh`; native exit 3221225794.
- `syntax-readme-v2`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/Temp/obx004-recheck-mvu85i_7/readme.sh`; native exit 0.
- `syntax-handbook-v2`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/Temp/obx004-recheck-mvu85i_7/handbook.sh`; native exit 0.

## Full reviewed candidate identities

SHA-256 of actual starting working bytes, all matching the author final manifest. The new reviewer report is excluded from its own manifest.

```json
{
  ".dockerignore": "ed6ca17f188919dcd94f021ed5d689c0483096c21158439690caaf2ffc41682c",
  ".gitattributes": "b7c753f47a9f1a383d9ddf6e2bc284cc54f3054c4226131bc29b75b7f21eb795",
  ".github/workflows/ci.yml": "ae6e091287aa9102c65cc943cb7bda250019bf4a517c56ff2a23c637885ce787",
  ".github/workflows/release.yml": "f57ac7fd9df8f391ae900d6a3efef25cb143fcc21adcadec71cf2bfb061f0573",
  ".gitignore": "6ae89a2e30171579e45edf2146d378a118093f94c40114d43ecbd3ed2e124fde",
  ".vscode/extensions.json": "fcaabcc9af285f129e11c3db4677c9413bc1545c12327997e4b931de14f7d9d3",
  ".vscode/launch.json": "9e986e720e070dfcaa67c539bd893fb6c8ec8d5810bd90864e17bc0ea8b1fc08",
  "Dockerfile": "4f53143f12d36591529e2f8bdcdea6cafb60d1a026f226e784702775a222faf4",
  "LICENSE": "9038ef6382d6b579257b5f25b30aa122e0de8c5491eb99e432818514dccd216d",
  "README.md": "cf4942c5769d068ef41b876f0fbfd8f59170fdf241a28728d906c66760640d4c",
  "RELEASE_CHECKLIST.md": "73e6f073a3f257dd62ea46c5c4b3d56bc1510f0a174af8df38e43795f960f16c",
  "adopt.go": "c993c006d12f4d9859e04af12f8713c535babf0e9d0917dba322b67d4792422e",
  "adopt_test.go": "2d052655aa4b8c1fd3be57c2d74ad77a0870bc22534617e44a281220b050b7fc",
  "apiguard_test.go": "95cb88f23d0e2e992910127de85d5e14df24f4160e1bd31f5573a13774963202",
  "appbackup.go": "544e653da8956c16f3f8886a6ce69f1e432bfc8a0400843690f23fb837f8894c",
  "appbackup_test.go": "36f29bf5dd536923ec16dd9ad0d4d7b06d6b1ed4b0ea070d1b2620f7f804a3b7",
  "atomic_replace_test.go": "b46875d17a29c716ec5308f236feae47471d1755c5ce124dc3d0728cee1f0bf5",
  "auth_test.go": "e4f65a66ebeb987500e21497c1450ce55498d98d700e1ce1618c5d83eaa63e80",
  "bagit.go": "d61c043b210f90bd5229bc170c58c3639b945f978431e49cbf7c5f47f5aa6e00",
  "bagit_test.go": "f973ef4eb725f3bcc455810077d828a224a32a0c78604ead8c7bbc1a1de4a509",
  "browse.go": "6919e26d8ed56e0879900474584dbcc72aabc02d95f203d431c1beaaa1658340",
  "build_verify_test.go": "d12acce608c680c61e442581b11bf4dba3facb4c2a41b1013fd38cba408bb091",
  "build_verify_windows_containment_test.go": "73b8343b61b2434789414ec6eb4de8621a18b2d8d1078b9d39e84f0ee4d63bfa",
  "burner.go": "62e7fd9e9fecb2b34fc459caf1f5a7057865be7add85a875d370e04e912308d2",
  "cardcheck.go": "7fdcd6ac234c6422149ccd123fc05b9634d9065e867cd029b9e8206c0e488bfc",
  "cardcheck_test.go": "c6d689dc4b1d11b57af224b69b051f42a3ccb743100b44efb009527e04d7fc5a",
  "catalog_open_test.go": "13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3",
  "catalog_scale_test.go": "c99b4b201f82ae4793c2f8f0fd444649924c1a8e54e61b3078a11a65ded74c73",
  "config.go": "fcfb81fe8c317522806e0cc0742688a9650d0958983c7fb95f9c0e1adaae1789",
  "config_boundary_test.go": "5e70cd6fed9fab23d7033a8149f9498b27858645a9e38045beb36629bc72e897",
  "config_init_cli_test.go": "17e8a8ec07c5b1c90a598010ee3aaadd32bb251fd76f3d5544f571ec55eed3de",
  "config_init_fault_test.go": "de8dee21c086f823d7f21f27c89f7fe73c6c1bd4ff4c7a66d961c0deba9f193d",
  "config_init_safety_test.go": "ead694d9843336530f34b8049b72edcc5a3e8216d84eb66c15e49eb90330ca73",
  "config_read_safety_test.go": "93ca9201dd4d0d7d4fae63abddfa143de841ee55f17b7b8fb09ddcee59b6bc9e",
  "config_test_helpers_test.go": "747129bac34f36f514b665fff335c663de48242e1968af4cb5ed57d190f98e96",
  "conflicts.go": "c6d353310ddd4c71b79d6821d5cbdfa9fe95d45f1987c6e71cffddbde245f007",
  "conflicts_test.go": "d116585e4dd13ca877dcc5218309946a0efcbb2a097cb8b02cab3343ed6df090",
  "copy_health_test.go": "18bc11f70c6cd6cc678886c713d576985104518afa1dfd89135ff8219e6a7877",
  "datamap.go": "d4c8d0d7ce26eb5885e9de65a87e09af52be6ded66a03d1d78b1043e9d8b3105",
  "datamap_test.go": "299196163b678ebb55c44934f16817e5c220393a69bf515a92ccb1850dee9b81",
  "deviceid.go": "15ae93b4db80b592d6ff382e24baf02de2e55a690b6273a9915098a1cfe943a1",
  "deviceid_unix.go": "7bcbea76b28d89176fed89cfe9ad3ed27b261bc1e0baf85bd1eadf8f3dfb571d",
  "deviceid_windows.go": "09784f2fc73bea033ebb0d8caaa3d86a124759c8f0bfd6ea5ace6cf043637216",
  "dirsync_unix.go": "3479a216926bf995714f12afecd6b88f6ed6374b66c62b14a6d046dea5a78495",
  "dirsync_windows.go": "5fea9c891ce63e8ad169723778c57e529ec0e9614b202d3d531ddc4c0270a4c3",
  "diskfree_unix.go": "8b6e4f56829bbcbc7caad6a75551f31968ac9375e4b542827484e00336a9bbc4",
  "diskfree_windows.go": "4ca61bf8ed12f9853ea7498957ad668784ed565aac061b858e074c128e5c3270",
  "dock.go": "af84d0bea8797be35047fc1eee93b4302417c9a1505707d12f6d4ffa83b5ed52",
  "dock_mounts_unix.go": "b5eb84b2ea8cb47648d0d7bbf28bfd591f353911012b44dd057714ef00218b40",
  "dock_mounts_windows.go": "057a0a6e915b24b27c672d590c6465d9617e5d145459d0f6ad1e53d986bf281f",
  "dock_test.go": "579dd3dafb12b418876c7f22abf056ecc9b3e7fde1d6387aa01e3b25bfcdf0f3",
  "docker-compose.yml": "026480e6e8e37fbae52b45f7479e6586f328451b98e6264fd083afd8aafe9a4d",
  "docs/ARCHITECTURE.md": "63c19810b5f03bf31c292cfdc271411a82efc65156023eeda2333c7ee8ea133e",
  "docs/COMPARISON.md": "31337747bbdfa2c6121ed481329e91cf62f795ad20e53ce58772acb76bfd2ce6",
  "docs/CONTRIBUTING.md": "9b478da3f9ced95a9f131badae2aa85e7c616ca4d44d1f4219e9c30500069fda",
  "docs/Obelisk.fig": "69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda",
  "docs/RESTORE_RUNBOOK.md": "7aea44781f700d38b249590fb793c55f35ceb9defcd5dfe23284960efac9128c",
  "docs/development/BASELINE-2026-09-06.md": "57988c0b857c69f3b166dff77a95d86b6e86b38ec2262a24e73b56737552608e",
  "docs/development/CODEX_HANDOFF.md": "d72ddd352579c351ab9311b70e63c23a8dcf7d8bed0b4d86583fcf12af39e21d",
  "docs/development/NEW_DESKTOP_BASELINE-2026-09-19.md": "4a54cc02eba76e52ab3cdafac17855c3ad17b876c789f9e7b90dfc866d1b8061",
  "docs/development/NEXT_ACTIONS.md": "942c0019835143bd87b04d07343bf79ef6af42335c4a7fa15f8c028c338e5422",
  "docs/development/OB_STATUS.md": "3da881637a09f3061cb25fd4241bf35f7f33b5b659d36b9ca6dd7bde87293541",
  "docs/development/PERSISTENCE_REVIEW-2026-09-19.md": "8aa24fecdabd09843f8644fd3aee947a34652a7eb2d66d4ead9c5c56be3d998a",
  "docs/development/REVIEW_COVERAGE.csv": "4737419e9623af14b3526bb8434c94fa0a30147419b338d9e41aa79e7f7100ff",
  "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-DETACHED-REVIEW-2026-09-19.md": "ce903017e74c1e5cbff906a02aae5e655572ccd6c2805a4c5d21af3c6b41e154",
  "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-FRESH-REVIEW-2026-09-19.md": "7b0b349578910b4edfa8fad5edf0cbf889e872df3a0385d5254c2bf87c1c9ed2",
  "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-IMPLEMENTATION-2026-09-19.md": "7b91b221977d8482408538c3fa2f04aeec437358c927949ce4441725312eaf44",
  "docs/development/reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md": "b6281c6b4a1d2758bb99cfaf467da94f5bbd9f4f1465aaf2e9a5acf0540a3f1a",
  "docs/development/reviews/OBX-004-CONFIG-READ-SAFETY-DETACHED-REVIEW-2026-09-19.md": "0d3585014f6fab7d4f41c87b1c4c9da2fb5893830535776fb0b7944e65c68e76",
  "docs/development/reviews/OBX-004-CONFIG-READ-SAFETY-IMPLEMENTATION-2026-09-19.md": "34acd295d541824c57fc6d3f47b150db638a92d9abc2c588817ed2e9e6318f9e",
  "docs/development/reviews/OBX-004-CONFIG-READ-SAFETY-REVIEW-FOLLOWUP-2026-09-19.md": "e67e8ef0105794ccbc81733f5913c1dcec24fb95fadade761fb2a4ad360e1ac4",
  "docs/development/reviews/OBX-006-CONTAINMENT-2026-09-07.md": "b27a7d3b54ab67f6472912ed295ae02129d60e942a9048dafdf7cbc1c4deb262",
  "docs/development/reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md": "faa18d13253186b81af1f5a71b0b11498197c57e6e09edac1882faa8f09e63ba",
  "docs/development/reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md": "803232e8edf1eb65c35be7d2be83045c11b3aadc826f7ab0087b4656cd4485e5",
  "docs/development/reviews/PR01-OB-001-2026-09-06.md": "78a45fc095c9dadbb19bb7764ac13f9fd86b9e2497499c124d6122c0d2a36f25",
  "docs/development/reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md": "2009e5fff98f2426c49a2a6195035622f381a549b274d6b3f641512065f2cd37",
  "docs/development/reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md": "1b5cc6042dce58acf015debb18f96bfaf6139d3a3c9fc1a80a2f5fe25beab33b",
  "docs/development/reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md": "60d93d467c90b2c6268f3b351a6f37ef7970f260e0078830c8f126e56995c670",
  "docs/development/reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md": "83d25498c75698d03553c2c08f719a2415809b55bb89db432902d2cfa37cc0a0",
  "docs/development/reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md": "adcac2c082e9e2daa0f63030913d4e04a300f9c2bda03d7d267628c54107fc2f",
  "docs/development/reviews/PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md": "774bab36e0dc794e7662b44e8879d907b0d4c3f6acce9b67859068e4d2453bd9",
  "docs/development/reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md": "b4eaf0c610e51c6007d73aa50f697249eb50b8923a042417e9f65da156b631b2",
  "docs/development/reviews/PR03-OB-002-FOCUSED-RECHECK-2026-09-07.md": "267705583889cd36343e732cf4a76cf6dc6236e106da13b0b1463c3c170decc7",
  "docs/development/reviews/PR03-OB-002-FRESH-REVIEW-2026-09-07.md": "18df0387b5af04c1dc99fda5e4b55db2099e001351d1a276ff499c91e1738c77",
  "docs/development/reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md": "6b734a3c86b1e093351bddcb2ca5b3a35843da23b152567e7848d614791d235e",
  "docs/development/reviews/PR03-OB-002-REVIEW-FOLLOWUP-2026-09-07.md": "253da4e9ae95f6556ad63a49a34d7543ac9ddff9dea3b0a26e44897e64de947a",
  "docs/development/reviews/PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md": "aa65cd466110aecf4dd7a3740837f90458a9e727c3c70cac1b56429e5ba889f9",
  "docs/development/reviews/PR03-OB-002-UI-TARGETED-RECHECK-2026-09-07.md": "8d51293848e0fff31cf9d8ed18a934d4aeb38953c8c82e828fe52ad0b51c1a6d",
  "docs/handbook/00-what-is-this.md": "70029afe6542883eac60f739d4dfcecd6ace5373eac856240efb09fac7eba810",
  "docs/handbook/01-install-and-first-run.md": "6ef9ce7d9b8aee58b8095f02592984b0a571ffabe65ba156f760ffd5ae0bf14e",
  "docs/handbook/02-set-up-safely.md": "d3677cd196d73b53c85a7817f4bfe2e81a7d74a130bb5a504f5588375b5f542f",
  "docs/handbook/03-your-first-backup.md": "4fa4f9b3e72990c65c8768308c3b897d14373d0efa18efd86cf7f270effefe1b",
  "docs/handbook/04-the-3-2-1-setup.md": "b32cce6f075eb540ba20138eb09e85a4204f39e9aeca2fccb2dad1fac2b28b9e",
  "docs/handbook/05-tape.md": "485a989a4c94cf7dfc3e8736faa9a057b0bddd3d5a057a03730cf4996b53fddc",
  "docs/handbook/06-discs-and-drives.md": "a8d876ac25812f654dedb470cdabc5bbb10bfde53535b5094eed5e746b98e490",
  "docs/handbook/07-checking-on-your-archive.md": "2cf4f96fef6831a552045aca9729629458cef20cedf62d357e6afd12f47278d8",
  "docs/handbook/08-getting-files-back.md": "898e707654eeb7356547d335ccb59cb615271635f988f82e2c694d660b973a86",
  "docs/handbook/09-the-recovery-kit.md": "6d477fb62f917d720419cfe469d84b1f1816a4a47839dae43c0c38c392577563",
  "docs/handbook/10-where-your-data-lives.md": "e2b0a963cea4f1b76d1e74a5559e8d80b318a46a8461578da2e7bb2c2a55d0ac",
  "docs/handbook/11-keeping-a-backup-current.md": "f2e21e268cc839575ad9a13f8d959398b398b86699949e0f8eff477af01da218",
  "docs/handbook/12-moving-to-a-new-computer.md": "fc2a2e8dc0d6436cb7d2f18706d8fd65e35a360488598a351b644e6a38e12b73",
  "docs/handbook/README.md": "ae757cc11317d0d9d4b7ac83d99e3a93d965ebec0d1f9eca410a02668309730e",
  "docs/handbook/glossary.md": "2533a13f5c54f0fa964db4c00417c139bfa73af64ce50f4cbb13dd2811ea87a3",
  "docs/handbook/troubleshooting.md": "9b67ae046a5638e92848336e8c337a80cf31b069475b54d4378052c8a5914788",
  "docs/img/README.md": "9c6ceddaef19ee07f41260a4b30b16a57f0f2184a68437248cb64a48e0e63bbf",
  "drift.go": "ee80fe0f96d0f95661f7c46784cd1266ccb588ff47b9ba7c6cff76c0448523d9",
  "durability_gate_test.go": "3973a7a0f8d7cfb082d0e9a87d71045737198a30edcb94dc9c3d82a480855ab5",
  "durable_completion_followup_test.go": "0e22ecc5124f41b97bf5edd3bc8743da96f398200d50f99e7bdd06f1e3e21dec",
  "durable_completion_test.go": "3a4837e343aa00d878d6daf32b6f5bc752564c403c315ef2bcc0cc999768351f",
  "durable_completion_ui_test.go": "c9746527013416cf74b03895248d45a34b0cb678739df4ad7948d1993e9f93f7",
  "dvdisaster.go": "b056e271efb878f6b867f52958412581e7b6a7592e1a7b6fc23dfc6fe63911d1",
  "dvdisaster_test.go": "9d4f744cd754b7d5878b093e8dceea14281a1c66ba307c374d3e8c1e7ac270af",
  "escrow.go": "1f266b780e27e5f561744c93ef6214cce68fdecd35a7c10deeb71952a1fe4d26",
  "escrow/PLACEHOLDER.txt": "d52b7f5972e12ce4b7720e1293e186f742fbf6408c1fa8f1e1a9ecd01a3e28c1",
  "escrow/obelisk-src.tar.gz": "ce3eeca6bff24f3a0e6cf6d4065a7395bd27740ccddf4b3a44afa0e4a22254ec",
  "escrow_docs.go": "6b8ae87baba575c724b2c84182d605b92854ddefd1233b2f784be403620fa5a0",
  "escrow_manifest.json": "7208612ba0640eb7ed90996713e89aa6f14e0e3984f6afccb4e9f5a4751242dc",
  "escrow_test.go": "0c7765ed8d895d72cdbcf12a35136d3804442c96e85bba88cf23d8d721ec997c",
  "events.go": "b3e4839bbd9d6ec3163532d381cd4442a501ae8ac92cd6766a5cd3943a4f6789",
  "events_test.go": "5a22e26e495eb59f7e5206dab1377764af0423f82d6521a4004b839b4c9b3a87",
  "exif.go": "dc04f54ac598b7c0924fe14f37e0f343f1e37a5a7c63ebe9eedde4bc1464b0a1",
  "exports.go": "1ad5ef43a5b7a8c2d550f69763c0a7db8a1ffa7c234c43a09c97db681aabc196",
  "exports_test.go": "bd3db4109259bdf40490b7df62d773a1f958b48085c9fd252531b746facbfb1f",
  "features_5253_test.go": "aa79ee9b0b3ffa7812108d6cae00aa98814f1cd50eb0cb00290d14224932f7ef",
  "finalize.go": "9e5512861d589a9716a39ef4d140f2f998a6baaf538c3b0d6d1b520db5a98b34",
  "finalize_test.go": "0a77eb30e658317a6b1e62d3bd046a0b8c9f4d8019107794582c2002bdb38f3e",
  "format_census_test.go": "1e1fa9b92681a2eed7e71dc345b9f79259e384aaef1c0fd018c56b8cebb4678f",
  "formats.go": "ec8d134b7565a7330f7f602ad95977f137a6d16ff0c993f42f4b985b1bb2f3f6",
  "formats.json": "a9272719772744a403ee488794c33aeba48b16d38d78ad4758072a74123d7509",
  "generalize_test.go": "3519b41e10801fe13f14b014ce3bf6e83ac474052edfacc753cf237b7df2760f",
  "go.mod": "b95a14e02c8faf2a4aac73901b843bc79ba154fd15aeaeb6161391bfaeb2c2bc",
  "go.sum": "9d169ad00514ad13d9a10e1b1ac4723171c020a23b792c76721d97ccd47a1993",
  "hashing.go": "2ab02f40830ce6abb6390bae8040aa277359f35fd8764214ba524fa57c3268e9",
  "hashing_test.go": "daf9c7ea3f6dc1a65fbde2c0902fa9e5f7b1f814448813366945aad717210e81",
  "home.go": "99678d7955ec0ae22091b64783b4df0a93d00bbf9981ddba4e2920c743e1a04a",
  "home_test.go": "dc566cf7ba73b57de2d33c57cefad5104da8fe2c04e6c4ed8bf0d34940cd5c74",
  "incremental.go": "96b3b250451e8197e8882a37a9f9a7c5bcd0cac7c91665f701b19a4e23df72a6",
  "incremental_test.go": "fad33655a945b6917d2de9ababd5dc3a6c61afcca57407008e40824942a3b05a",
  "inference.go": "5036685260b539657ff0ebdeb3b1faa3af830317267d724045f0ccd13c9e040c",
  "integration_test.go": "20d1023e214b17c8bfc0d37d584c782229fb934fd81785185354422e8ec7f1c8",
  "integrity.go": "e7f0cb521f0d81d88719957acfa53d94ca6fc109e0415b11348d43e6dbe8b4ec",
  "integrity_test.go": "3e72e3938a1db5054396a75f64f738b0eee56a19ca8e7b0fec6ce6a43e2fa919",
  "keystore_validation.go": "29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593",
  "keystore_validation_boundary_test.go": "2b0122a48d6969336e77fb8657d918c7fe2e1390e6310f0cf394c07c7032db24",
  "keystore_validation_test.go": "878452c2cd3d7e5f3afafcbbde4aa41ae1c3c957e9242f7fd1114e5cad90b86b",
  "label.go": "db31c7636192ae9fbc4d31a39c1440be5db87abac0267d5ade76cf948962833b",
  "ltfs_unix.go": "2e3ab33767ebbbacf9600672c52a63396f8e516e78cfd77ab532b6c35f28dca6",
  "ltfs_windows.go": "6cc84197baaf97b9bc981687656d5d1d52e453c767a560d20d7393ae09024a19",
  "main.go": "02706c9299b08d5cdb30d251f26655e89472f324cd6dadc0f04497ff6df27b22",
  "meminfo.go": "6b5ba49f26d857bf622a052dd54c9a850a289547b48fc9b4f3ffdcdf4fa1a24a",
  "meminfo_darwin.go": "e4a9a0e82fff49d82355421b67bf4343cc8ec3a75cfbe70b370a0db57c7c2e1d",
  "meminfo_linux.go": "746c245014ccda0625a1d60cd28883c585101e50aa3581651e289d799986f3cb",
  "meminfo_other.go": "f158dabddd7ae7bf867f196c1168cbfee1e86d20eebc0c1c7e4e434259d46d0a",
  "meminfo_windows.go": "9626face4212bb23b3278059c4bff3b53ea39986287096dafbf04851b91ad38d",
  "metadata.go": "e78a2784ee0d679e16c13e820eca8747dd8bf2f9f5b011ece05b64cbd5efb00e",
  "migrate.go": "e10ec332db58e90305cb774b0615192a1af7ad2edf4ef0785933df357d290db4",
  "mirror.go": "1957dfc6254d0ca14f13fbe8e9878fe7fb4f4ea07eec2c38ef66b15eeef86fec",
  "mirror_test.go": "900181d4eb964de1c502a30c79a82ea9cfae11f3fa0fbc1ddf3ac1190a8fd854",
  "optical.go": "ea4827e9c575012d650fd4a623510988ba275dd5ec0947e10d8d3541a12fa36f",
  "paperkey.go": "d99a78a3e0af4e0be737ba5b4f770f0135b343dc69850189f635bd4eab6401e6",
  "paperkey_test.go": "83ca9ea596a9686a921e1943f453cc5f024b89eab8cbeac8ef31288af19aeac7",
  "payload_naming_test.go": "c8ca561f87f254f0d7b1a071c6f69784d1b683a3d808c35ae99bf4e0ab6ee28c",
  "perf.go": "56fb5f54bf87f414551c9d3598ba1736816f4e29e0eeefb3fc7fece6a53027e3",
  "perf_test.go": "c284955f30916954cfbcf3b2681e4211833f4d19b025f5f4b828dc08546c38be",
  "persistence_test.go": "0509567d2d7803f0da6c463da827dc9ed5ef0b2985b53f81f177e4c0378af975",
  "pipeline.go": "09056e6275873afec4cf2f43b58bb993107813614de76455b426eabb4e7dceb4",
  "plans.go": "0d9829ef09e26041e7e684b3a7c7857d93c2141d370c12116eb918e2c10c99d5",
  "plans_test.go": "8b8989cb5eaee533a3bde2648f44b8bd84a1b5eb53a9b49082c437fda42a592a",
  "privacy.go": "dae7e14fa45e735bddfe0be62a55a189591c665d5c89a184a068e01738762f56",
  "proccpu_darwin.go": "710ef28aaa647eed06dafc552a0a6ebd0b7e54508c0198830ece0a45c25aef0c",
  "proccpu_linux.go": "0114d4c5bf5d769431c7f477e8bfc6965162d6ebcd1faafe9d5ff3a0a425d5ac",
  "proccpu_other.go": "db2da07bf3bd675e15e57862257b945d26729b037c33ef94477cf6eafa60a581",
  "proccpu_windows.go": "c7a1f09323ecbe7cc5f3a325bb1bcfb0a191295edb83eca63982e3a0dd3c765d",
  "profiles.go": "78faaebe153910ba1c99cffad4ec98cd1c0b4118f72854eb98b6206937cbb154",
  "protection_test.go": "5a70e156960d87c735ae1936dc3521a9b2806c851e257264860ca26d98d40f87",
  "quarantine.go": "b440cd67b607eec50392123c86ff08c570cdb3490e3f3ea6a490eb9869136486",
  "quarantine_test.go": "0b59d71fe8646617441b1889d156ecddb512ccf838c1b7db6fcd660b24d0c7b7",
  "recoverykit.go": "d9b2f61c206705bf54093c077db62d1b7e1198fee72a7a3f631413c792e06fdd",
  "removal.go": "c9edb9d6fe13eac499510dcb3023d35568f9cc738ecaaa234ca1b7af6b571acf",
  "removal_test.go": "3156295ff2134330df6c388f3ace625810c942d5595ac0cddeba8168f2a8f46f",
  "rename_compat_test.go": "491764e168c1d336f84da0ba07d9d59124562a656eb30ede783c470cbe1042e7",
  "ringcopy_shortread_test.go": "3ed9be8cc9c1a6821ebe11e12262eac6502dd65b553645e234f33882457f43d1",
  "scan_problems_test.go": "ebeda532427af8fceb6c83a8746cbe3cda70df071c9fd82e8c62fe41a1786a43",
  "schema_test.go": "002df0dde5face40db725562b38e3dd24124c407d6349a890ebfd8f2b706fe73",
  "scripts/banned-phrases.txt": "c6e7ac9c1fc5b5d2ab16615256accdaabb17f8af05da5f2580c6a81889a97953",
  "scripts/prose-lint": "5e7d1117cb48fd607c1d754c839f01ec096bfd4fb4a8b621d5d77e089fe0771f",
  "seeing_what_happened_test.go": "86a775030bc82fb1404071a430bc238430b9d72e8064d46d53e69a3b64575d18",
  "settings_test.go": "ef5c91bc629adfbaa305d479937e6723245d2a28951104b69ed9faf02331c668",
  "setup.go": "7be4fb9f09a33c39fe1f505e9721979474a1cd60b814d6c52a9f5a4c1ec2bce9",
  "setup_test.go": "eefe9bcb661f3891823a47fa41095c78fd5eb151f022eb8bf430f172027b156d",
  "smart.go": "19b3b3f6239aeef4588ec4498518562fb5c3a2758658a98e6f09a43512d5bba3",
  "smart_test.go": "a05f3523aa76f09961ee02b1179786ab7cea273f024c654a9fda46128f0644f3",
  "smart_unix.go": "d3c98a10cfd7f868557b913e2b002949f5b263c81b7fb758e495f3133cc91f93",
  "smart_windows.go": "1d3efcf5ab87be4ebe40602f45022e63e97ed59de69cee1e5c8e9a3f7ec23122",
  "snapshot.go": "76ab5a779c8d7c3bc684d421403c775d32e6a7a28c50fed25ce8b97db1a13e00",
  "snapshot_test.go": "0b45e1e13cddb603951819df13aa1c4cb24090df13aeaef4d5067a3ce95f4a4f",
  "sourceless_test.go": "49a4d9819047374e98bfcda5fd9cc03bdbba3332d0e1ed4777b64a0b8b0c6f16",
  "space.go": "bbe0b4e55ed48c6132e6ac8f1589c62ecd4807d0dee6125edd175077f6c6888c",
  "span.go": "6a47340de6c5b6a862660f9c032e8b041cb970e254166ba6c85d6058b50783b6",
  "stenc.go": "982e6c74c9741c48bc462171acf8bb6c7a3ed76fd04a8fc178ce1cddc4307571",
  "stenc_test.go": "154662ab4c32c11e4d2522d5fe71d332218f1630dc07019300dce6893318ab8d",
  "store.go": "e91f658d6733302aa87d7ce720163bc9a88df71ed7bc55a9ea48625286d374a7",
  "tape.go": "2919ff9bc9d03dbe2c9a4c89587bc8a74186ee15b8a70a5df381b8621978de8b",
  "tape_parsers.go": "5425ffe8a3ec915e204b763eb3fecd5d3b9a43606215607c65aebf02a837e565",
  "tape_test.go": "504be41bd4d1f12f29705b4f6c7a13a0fd6235e135b48be87b738e9cfe882c0b",
  "tar_names_test.go": "ae9cf03ccaa1d054fa7dd1db974a3467c5dfde91b627a4bf13d675546dc568e2",
  "tar_unicode_names_test.go": "0a9c662bc2a6e6e43a104b0903f6056ffbe912bd72d029ac6e5f503d9fdabc83",
  "templates.go": "dfce374db3b4907616ee0b42a7fb0cd37ae7b6843738862f7a83caa8fecc3f06",
  "testdata/catalog_current.json": "0a225a5c3beabe0914e685760688274079b4438707f4f0f8c626249ebe36cf57",
  "testdata/catalog_schema1.json": "da3104c2cbd1698879d2bcdb0b53f7db0ffd5bbfe84fc36935bcaf2f6d2a9e7c",
  "testdata/tape_hpltt.txt": "a8f001303ebc8a36af71127665a43df12afc34b1e515710f12eff97cd48c0763",
  "testdata/tape_itdt.txt": "2045b5d6a64eef66a4be173184f6a2d0a463ade187bdf38efb323bfa5ebfb25f",
  "testdata/tape_sg_logs_alerts.txt": "b54a57e43d3d272a2466fbb3b63b16b2eef976ca61d9976fdef8013dc1f1bfaf",
  "testdata/tape_sg_logs_stats.txt": "fac0060b323284afe5bbed5b142b85167ea4f41c07f6843a79429e88b1891209",
  "testdata/tape_tapeinfo.txt": "a426f917b823eac9069b51a0b69bbec39ecd9b05c5f8982e890698eaf3fd5f5c",
  "testdata/tape_tapeinfo_clean.txt": "519860a9a451e9ff7845efd8599e4efa313391e801e330a298da8521e150c2d6",
  "tools.go": "033c3118c62895d619843a880f834ff6b65c279e1de0663475ad7c9c54f9bb9d",
  "tree_test.go": "7fac45e28cf3fdf121ea0c8c4c679d33d30919bd401a31234af8d1c1f5d553b8",
  "treemap.go": "c93d967973a7f8808d7cc2d558127c8f054e9a8618844252f110ca698e59501b",
  "treemap_test.go": "4df18871ecf0c2bed641c843ba671bad5f00511b51a79953bcd0190d35a36667",
  "ui/index.html": "7c40475011aa6935273f14ffb3ae97b8e667b087f528f011d0d74d9a227e1656",
  "uimode_test.go": "c9bcea36317ec710c10f311dd2f9feada3b434e62c13f03aeb952c5061e4fc15",
  "verify_levels.go": "787b1d2bd602ae5a9a5b0c947005aa47ceb6bd32af753a512cb2be6aee786c0a",
  "verify_levels_test.go": "6a0ec5b763c1a0e92a1b2ab466965b790e78b41e6c30a1d04e46504c4b465c3b",
  "versions.go": "3cdd6491bd72b032a2efd46db3bcc011bbd6b6301708243bf3b813c0a4e20b72",
  "versions_test.go": "81a0b28ea4077b0c8e7f0e83021238667e416b96c6f9d0bcf5261caa366e5a2d",
  "volume_identity_test.go": "8774ea98e908e0570b6d177b463d0cb1bdd2ed8df296a5716e9bde052b7a5217",
  "writer.go": "6f049544015b84ef05de41ed87fe5985a8de50b455d60fb7dd81a42b2fff8294",
  "writer_flush_test.go": "257123cd088ced2762ed440f322c2f1bb7cbef6e43471f5db5dc11a80b0e6f85"
}
```

## Final preservation verification

PASS. All 229 pre-existing inventoried files retain their starting SHA-256, including source, tests, dependencies, deployment configuration, Figma and every prior report. Only this new review report was added. Branch/full HEAD and the empty index are unchanged; status differs only by this report. No operation marker or live recorded CLI process remains. Figma remains 219717 bytes / SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. See preservation-final.json, operations-after.json, cli-processes-after.json and status-before/status-after.txt in local evidence.

No fixes, staging, commits, pushes, merges, installs, job-loading repair or tape work performed. R1 and R2 are CLOSED; READY_FOR_OWNER_REVIEW. Stop after review.
