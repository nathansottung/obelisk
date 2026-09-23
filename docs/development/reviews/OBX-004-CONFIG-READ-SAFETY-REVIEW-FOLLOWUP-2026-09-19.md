# OBX-004 R1/R2 implementation follow-up - 2026-09-19

READY_FOR_FOCUSED_RECHECK

Parent/HEAD: 26918c5b8ed01ea301fc9a4c7658e19054a73632. Branch: fix/obx-004-config-read-safety. Reviewed candidate identities match all 224 recorded files. Index empty; no operation active. Pre-edit copies and full SHA-256 manifest retained under C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919 (local only). Figma and controlling/historical reports preserved.

Scope: R1 no-replace initialization, deterministic collision and actual CLI lifecycle regressions; R2 explicit Docker/Compose bootstrap guidance. Existing reads/updates/callers accepted by review remain intact. No job-loader or general locking repair.

Check red-list: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\red-list-execution.json. Raw logs local only.

Check red-arrival: FINISHED; native exit 1; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\red-arrival-execution.json. Raw logs local only.

Check format-write: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\format-write-execution.json. Raw logs local only.

Check target-list: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\target-list-execution.json. Raw logs local only.

Check target: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\target-execution.json. Raw logs local only.

Check safety-list: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\safety-list-execution.json. Raw logs local only.

Check safety: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\safety-execution.json. Raw logs local only.

Check full-list: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\full-list-execution.json. Raw logs local only.

Check build: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\build-execution.json. Raw logs local only.

Check version: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\version-execution.json. Raw logs local only.

Check vet: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\vet-execution.json. Raw logs local only.

Check format-check: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\format-check-execution.json. Raw logs local only.

Check full: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\full-execution.json. Raw logs local only.

Check docker-readme-syntax: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\docker-readme-syntax-execution.json. Raw logs local only.

Check docker-handbook-syntax: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\docker-handbook-syntax-execution.json. Raw logs local only.

Check whitespace: FINISHED; native exit 0; C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\whitespace-execution.json. Raw logs local only.

## R1/P1 - no-replace initialization

The controlling review's overwrite finding is addressed in config.go, InitializeConfig/publishConfig. Serialization and existing validation remain unchanged. After unique same-directory staging creation, complete Write, file Sync and Close, initialization uses os.Link(staging, config.json). Successful link creation is the publication point: the already-complete file acquires the final name only when that destination entry is absent. A file, directory or link arriving after the initial absence read is preserved; the wrapped OS error yields ConfigPublicationError.Published=false and zero Config. No second existence check, placeholder, direct final-name write, destination deletion/rename-aside or replacement fallback was added. Existing valid state recognized at the initial read remains a no-op; damaged/unreadable state refuses.

Ordinary SaveConfig and restored-config replacement stay on the reviewed rename path. After successful initialization link creation, removal of the owned staging name is checked, then directory sync is checked. Either post-link error reports Published=true; the final entry is never removed to manufacture rollback. The error text now describes a post-publication step and wraps the specific cleanup/sync cause. InitializeConfig updates runtime hash acceleration only for actual publication, including post-publication errors. This does not claim that RestoreAppBackup refreshes every runtime preference.

The per-App link/removeTemp seams supplement real filesystem testing: nil uses os.Link/os.Remove. Existing create/write/sync/close/rename behavior is retained. Deferred cleanup still only removes this operation's staging name. A cleanup fault is reported even if deferred best-effort cleanup subsequently succeeds; a persistent OS refusal can leave owned staging. A collision leaves none in the exercised normal-cleanup cases. Windows directory sync remains a no-op; successful publication does not establish power-loss durability.

The approved Go 1.26.8 source was inspected: os/file_windows.go:308 delegates Link to syscall.CreateHardLink; os/file_unix.go:404 delegates to syscall.Link. This uses the platform hard-link primitive, not a general locking framework. Same-directory staging keeps both names on the same filesystem. Hard-link support is required; unsupported filesystems refuse without fallback. Microsoft documents NTFS/same-volume restrictions and unsupported ReFS/SMB variants in [CreateHardLinkW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-createhardlinkw). No NTFS ACL guarantee follows from the requested 0600 staging mode. Broader concurrent updates, directory/media identity, malicious same-principal interference and cross-file transactions remain deferred. Linux/macOS runtime was not executed; attempted Open Group page retrieval returned HTTP 403, so no fetched POSIX reference is claimed.

### Deterministic red/green evidence

Read the retained reviewer_probe_test.go from the controlling review's local copy. config_init_safety_test.go ports its createTemp-coordinated late arrival into a repository regression. The hook installs the arriving entry after the real missing read, then returns a real os.CreateTemp; production Write/Sync/Close and actual publication run. The test checks exact arriving bytes or directory/link identity, zero Config/error phase, unchanged runtime preference, and absence of stale staging. The valid fixture carries synthetic auth/helper/key-path settings; values are never printed. A real absent-destination positive control proves initialization can publish and reopen.

red-arrival runs against byte-preserved PRE-FOLLOW-UP candidate files, not the old Git parent that lacked configuration safety. It exited 1: valid, damaged and symbolic-link arrivals failed their safety assertions; the directory case and positive control passed. After correction all four arrivals and the positive control pass, including actual symbolic links on this Windows host. A future host lacking symlink-fixture permission explicitly skips only that case. This is deterministic interleaving evidence, not race-detector or stress evidence. The retained red test is its pre-gofmt version; checked gofmt output is byte-identical to the final repository regression, with no assertion changes. deployment-inspection.json records that comparison. Copies/hashes preserve both forms.

config_init_fault_test.go supplements real linking with create/write/short-write/sync/close, unsupported-link, staging-cleanup and directory-sync faults. A replacement hook fails the test if initialization enters rename. Pre-link failures leave no final config or staging and do not switch runtime defaults. Post-link cleanup/sync errors retain readable final config, report Published=true, and follow actual published runtime preferences. Existing TestConfigBoundary_CheckedPublication retains successful ordinary replacement and all reviewed update-failure stages.

## Actual CLI lifecycle regressions

config_init_cli_test.go builds the actual program using the selected test toolchain's go binary with build -mod=readonly -o <disposable>/obelisk.exe ., then runs that binary. It does not call only an initialization helper. Every child receives an explicit disposable -data, loopback ephemeral listen address, synthetic token, and synthetic GNUPGHOME; inherited bearer-token values are removed. Build lifetime is capped at 90 seconds; each serving/refusal observation at eight seconds. Readiness is actual HTTP root 200, checked again while the process remains running. Each child is stopped and Wait completes before the next same-directory launch. These tests preserve initializes-and-serves semantics without adding an exit-only command.

Raw CLI process logs and per-case JSON under cli/lifecycle-* retain exact executable/arguments, PID, timestamps, native exit, forced_stop, http_ready and before/after recursive directory/file-hash inventories. Paths belong only to disposable fixtures; token/config values are not included in JSON evidence. All serving cases in these Windows executions ended with native exit 1 AFTER test-forced termination. That is not a naturally successful process exit. Refusal cases naturally exited 1 with forced_stop=false. The forced-stop bit and observed readiness distinguish these outcomes.

| CLI case, per execution | Observed outcome |
|---|---|
| Fresh ordinary startup | Natural exit 1; no config/catalog creation |
| Fresh -init-config | HTTP 200, continues serving; checked config/catalog creation; forced stop and wait |
| Subsequent ordinary startup | HTTP 200; fixture hashes unchanged; forced stop and wait |
| Valid non-default config with -init-config | HTTP 200; all existing config/state bytes unchanged; forced stop and wait |
| Malformed / empty config with -init-config | Each natural exit 1; original bytes preserved; no catalog created |
| Directory obstruction at config with -init-config | Natural exit 1; entry preserved; no catalog created; not an ACL test |
| Existing synthetic catalog/key state, no config, ordinary startup | Natural exit 1; state hashes unchanged |
| Same state, deliberate -init-config | HTTP 200; only default config added, existing catalog/key hashes unchanged; forced stop and wait |

The matrix ran in both targeted and full-suite executions: nine child processes per matrix, five natural refusals and four serving processes subsequently stopped. The default-config check confirms no auth token or keystore paths were recovered from the existing catalog/key fixture. Existing older helper/startup/API and accepted OB-006 tests also ran; these are not substituted for the actual successful CLI cases.

## R2/P2 - explicit container bootstrap instructions

README.md's Docker quick start and docs/handbook/01-install-and-first-run.md now provide BOTH Docker run and Compose sequences. Each starts by building this corrected checkout as ghcr.io/nathansottung/obelisk:latest, the actual tag configured by docker-compose.yml; it does not assume the published latest already contains the repair. --pull never keeps the explicitly built local image. A private .env supplies the supported OBELISK_AUTH_TOKEN mechanism without an embedded credential.

Docker bootstrap mounts mnemo-data:/data and mnemo-staging:/staging, uses the real obelisk entrypoint with complete -listen 127.0.0.1:7821 -data /data -init-config arguments, and publishes no ports. After readiness, docker stop obelisk-init in another terminal and waiting for the foreground command to return precede normal startup on those SAME named volumes, without -init-config. The normal command supplies full listen/data arguments too.

Compose uses the actual obelisk service and ./data:/data bind from the same project directory. compose run --rm --no-deps --pull never --name obelisk-init obelisk supplies the full bootstrap arguments. No --service-ports is used; --rm overrides restart for the one-off container. The same stop-and-wait step precedes compose up -d --pull never --no-build obelisk with its existing ordinary command/restart policy. Docker's [compose run reference](https://docs.docker.com/reference/cli/docker/compose/run/) confirms service volumes are reused, command replacement, port suppression and --rm restart behavior. Named-volume and Compose bind-mount alternatives are explicitly distinguished; switching examples must not accidentally switch storage.

Dockerfile and docker-compose.yml changes are comments only, correcting the obsolete quick-start wording. Exact non-comment-line comparison against pre-edit copies passed: ENTRYPOINT/CMD, service image/name, mounts, token environment, ports and ordinary restart policy remain unchanged. No permanent initialization flag, fallback or restart script was added. Documentation says bootstrap continues serving; stop it before normal startup, retain data, and recover missing/damaged expected settings by restoring access/original config. Deliberate defaults beside catalog/key state do not reconstruct auth/helper/keystore/staging settings.

Docker is unavailable and no container build/run/Compose execution was attempted. Existing Git Bash ran -n on the extracted example blocks for both documents, exit 0; this checks shell syntax only. Service/argument/volume semantics were source-inspected and compared against official Compose documentation. Native CLI testing establishes the lifecycle, not successful container deployment, Docker image build, CI or other-platform runtime.

## Executed checks (this follow-up only)

All native commands listed below finished; none is merely a command start. The runner retains exact executable/argv/cwd/settings/PID/start/finish/exit in <name>-execution.json and raw logs/<name>.stdout/.stderr. Names were enumerated before target/safety/full execution. Go version is go1.26.8 windows/amd64; process-selected GOTOOLCHAIN=go1.26.8, CGO_ENABLED=0, GOWORK=off, GOFLAGS=-mod=readonly, GOPROXY=off, existing caches and isolated TEMP/TMP/GNUPGHOME. No tools installed, global configuration changed, GOTMPDIR override, WSL switch or real archival-data access.

| Check | Native exit | Top-level pass/fail/skip | Subtest pass/fail/skip |
|---|---:|---|---|
| red-arrival | 1 | 1 / 1 / 0 | 1 / 3 / 0 |
| target | 0 | 52 / 0 / 0 | 74 / 0 / 0 |
| safety | 0 | 43 / 0 / 0 | 10 / 0 / 0 |
| full | 0 | 244 / 0 / 39 | 84 / 0 / 0 |

Build, vet, selected-source gofmt -l, Git diff --check, and both Git Bash -n checks passed. Formatting output is empty. Git's line-ending notices are retained; they are not whitespace-check errors. There was exactly one uncached full-suite run. Selection overlap must not be summed as distinct test coverage. The earlier author suite (240 pass/39 skip/72 subtests) and detached reviewer selections (73+18 top-level/72 subtests) are historical evidence, not current results or pass-count targets.

Final suite has 39 explicit skips: 36 need missing GPG/PAR2, one permission fixture is unavailable on Windows, two scale/performance checks are opt-in. Three system-disk probes were excluded by selector: TestSmartDeviceNode_SystemDisk, TestVolumeHealth_SystemDisk, TestDeviceIdentityAndLabel_SystemDisk. No Windows sharing failure occurred in this follow-up; the previously reproduced catalog-reopen sharing issue remains unresolved and its old logs/reports remain unchanged. No retry-until-pass or discarded failure. Race, ACL enforcement, power loss, Docker runtime, CI, hardware and Linux/macOS runtime are unestablished. All 17 accepted JSON-keystore validation tests pass without implying skipped native integration coverage.

Exact recorded commands (red commands use the retained pre-follow-up copy; other commands use this checkout):

- `build-execution`: `"C:\Program Files\Go\bin\go.exe" build -o C:\Users\nsott\AppData\Local\ObeliskDev\obx004-followup-20260919\build\obelisk.exe .`; native exit 0.
- `docker-handbook-syntax-execution`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/ObeliskDev/obx004-followup-20260919/container-handbook.sh`; native exit 0.
- `docker-readme-syntax-execution`: `"C:\Program Files\Git\bin\bash.exe" -n C:/Users/nsott/AppData/Local/ObeliskDev/obx004-followup-20260919/container-readme.sh`; native exit 0.
- `format-check-execution`: `C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\gofmt.exe -l config.go config_init_safety_test.go config_init_fault_test.go config_init_cli_test.go`; native exit 0.
- `format-write-execution`: `C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\gofmt.exe -w config.go config_init_safety_test.go config_init_fault_test.go config_init_cli_test.go`; native exit 0.
- `full-execution`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 5m -skip _SystemDisk$ ./...`; native exit 0.
- `full-list-execution`: `"C:\Program Files\Go\bin\go.exe" test -list . .`; native exit 0.
- `red-arrival-execution`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 2m -run ^TestConfigInit_ .`; native exit 1.
- `red-list-execution`: `"C:\Program Files\Go\bin\go.exe" test -list ^TestConfigInit_ .`; native exit 0.
- `safety-execution`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 2m -run ^(TestOpenStore_|TestPersistObserver_|TestAtomicRename_|TestDurableCompletion_|TestJobsUI_|TestWriteCatalog_|TestCatalog(Current|Migrate|Legacy|Newer)|TestExportAppBackup_Failed|TestRestoreAppBackup_Failed) .`; native exit 0.
- `safety-list-execution`: `"C:\Program Files\Go\bin\go.exe" test -list ^(TestOpenStore_|TestPersistObserver_|TestAtomicRename_|TestDurableCompletion_|TestJobsUI_|TestWriteCatalog_|TestCatalog(Current|Migrate|Legacy|Newer)|TestExportAppBackup_Failed|TestRestoreAppBackup_Failed) .`; native exit 0.
- `target-execution`: `"C:\Program Files\Go\bin\go.exe" test -count=1 -json -timeout 3m -run ^(TestConfig|TestKeystoreValidation_|TestAuth|TestAPIGuard|TestApplySetup|TestSetup) .`; native exit 0.
- `target-list-execution`: `"C:\Program Files\Go\bin\go.exe" test -list ^(TestConfig|TestKeystoreValidation_|TestAuth|TestAPIGuard|TestApplySetup|TestSetup) .`; native exit 0.
- `version-execution`: `"C:\Program Files\Go\bin\go.exe" version`; native exit 0.
- `vet-execution`: `"C:\Program Files\Go\bin\go.exe" vet ./...`; native exit 0.
- `whitespace-execution`: `git diff --check`; native exit 0.

## Candidate identity and remaining scope

Full overall patch base/HEAD remains 26918c5b8ed01ea301fc9a4c7658e19054a73632 on fix/obx-004-config-read-safety. Pre-follow-up artifacts are the reviewed UNCOMMITTED candidate, not HEAD's source alone. All 224 identities recorded by the controlling review matched before editing. pre-identities.json covers all 225 then-present tracked/untracked files; red/ retains their actual bytes, including embedded assets and historical evidence. The only added red source is the portable regression. Final full working-file identities, retained post-edit copies, Git status/refs, and preservation checks are saved beside it in final-identities.json and preservation-final.json. Hashes are SHA-256 of actual working bytes, not normalized Git blobs.

| Follow-up file | Pre-follow-up SHA-256 | Post-follow-up SHA-256 |
|---|---|---|
| Dockerfile | 4ebf210eb94d59ad2a8df079ae345273ea8e6d26ca710f91409ad6628f9e5a45 | 4f53143f12d36591529e2f8bdcdea6cafb60d1a026f226e784702775a222faf4 |
| README.md | 696f7c9fdaa3f5270edd125e117daf4245f7fbb23ea5224295c445f636d37571 | cf4942c5769d068ef41b876f0fbfd8f59170fdf241a28728d906c66760640d4c |
| config.go | b3adaa80451eac6db939d82718ce166d6d81a45e6c4a2f7f4361b672bc2c77c3 | fcfb81fe8c317522806e0cc0742688a9650d0958983c7fb95f9c0e1adaae1789 |
| config_init_cli_test.go | NEW | 17e8a8ec07c5b1c90a598010ee3aaadd32bb251fd76f3d5544f571ec55eed3de |
| config_init_fault_test.go | NEW | de8dee21c086f823d7f21f27c89f7fe73c6c1bd4ff4c7a66d961c0deba9f193d |
| config_init_safety_test.go | NEW | ead694d9843336530f34b8049b72edcc5a3e8216d84eb66c15e49eb90330ca73 |
| docker-compose.yml | feba39b90700e22168f53bd4646465abc1754f72bc742b2d4b48bb1f75101ee8 | 026480e6e8e37fbae52b45f7479e6586f328451b98e6264fd083afd8aafe9a4d |
| docs/development/CODEX_HANDOFF.md | 61a542ab6cb48e36ddc7de26379adbb95564b9d3005f1276ef2e6c692e12df74 | d72ddd352579c351ab9311b70e63c23a8dcf7d8bed0b4d86583fcf12af39e21d |
| docs/development/NEXT_ACTIONS.md | 539fce8744b31da871e3882e9d2111a989d2c7738f2890e13c61fc99c8cfe682 | 942c0019835143bd87b04d07343bf79ef6af42335c4a7fa15f8c028c338e5422 |
| docs/development/REVIEW_COVERAGE.csv | a2381c647ac5d934e58d3f7557ff51b2042cf138691126f5929a2189776e0241 | 4737419e9623af14b3526bb8434c94fa0a30147419b338d9e41aa79e7f7100ff |
| docs/handbook/01-install-and-first-run.md | 80f4354112a4f482fb765927372541bf02e5e08c065fe6126980e513620445d7 | 6ef9ce7d9b8aee58b8095f02592984b0a571ffabe65ba156f760ffd5ae0bf14e |

The new follow-up report itself has no recursive hash in this table; its final hash is in final-identities.json. CODEX_HANDOFF/NEXT_ACTIONS/REVIEW_COVERAGE are working records updated narrowly. The controlling detached review, original implementation report, all historical baselines, accepted decoding/ordinary-update/caller code outside this initialization change, dependencies, Figma and unrelated work remain preserved. Figma remains 219717 bytes / SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. Branch/HEAD/refs/index and operation-state checks are retained; no branch change, stage, commit, push, merge or next sub-scope was performed.

R1 and R2 are addressed as an implementation candidate, not independently approved. Broader concurrent updates, directory/media identity, malicious same-principal interference, cross-file setup/restore transactions, GenerateKey/catalog coordination, ACL overhaul, PR-04, native tar, Figma access and repository-wide review remain outside this correction. loadJobs/null-row work is unchanged and remains the next separately authorized implementation sub-scope after acceptance.

ONE next action: focused recheck of R1/R2 on this exact corrected candidate and evidence. Stop without publication or job-loading work.
