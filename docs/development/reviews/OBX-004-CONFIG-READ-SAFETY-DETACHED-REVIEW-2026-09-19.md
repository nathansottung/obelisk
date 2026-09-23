# OBX-004 configuration safety review - 2026-09-19

NEEDS_CHANGES

Reviewer provenance: Codex, reviewing the shared working repository with fresh source inspection and native executions. Shared handoff and historical reports were inputs. No human certification, separate model context, or context isolation is claimed. REVIEW ONLY: no implementation changes.

The candidate substantially repairs failed existing reads and ordinary updates, but explicit initialization can overwrite a configuration that appears after its absence check. Container first-use instructions also omit the newly required initialization step. These are two consolidated blockers; no job-loader work is requested here.

## Blockers

### R1 / P1: explicit initialization uses replacement publication

Anchors: config.go:140-159 (InitializeConfig), config.go:252-305 (publishConfig), especially config.go:301-305; config_boundary_test.go:136 (helper-only initialization coverage), main.go:60 and :72 (flag).

After readConfig returns not-exist, InitializeConfig prepares defaults and calls publishConfig(..., true). The initialize boolean only adds MkdirAll. Publication still uses os.Rename over config.json. A second process/operator can create or restore valid settings between the read and rename; initialization replaces those settings with defaults, including losing the auth token, helper overrides and key paths. The per-App mutex cannot protect that other writer, and startup creates its own App instance. This violates initialization's no-overwrite contract independently of a broader multi-process locking design.

New reviewer reproduction: in a disposable copy, TestReviewer_InitMustNotClobberLateArrival installs a createTemp hook that writes a valid synthetic configuration after the real absence read, then delegates to real os.CreateTemp. Production Write/Sync/Close/os.Rename run unchanged. The test requires refusal and byte preservation; it fails (native exit 1). Inspection confirms initialization succeeds and replaces the arriving file. This is a deterministic interleaving demonstration on Windows, not a race-detector or stress-test result.

Smallest complete correction: give initialization a distinct atomic no-replace publication operation, preserving checked staging/write/sync/close and truthful directory-sync error reporting. If any destination entry already exists at publication, preserve it and return a clear refusal (or deliberately reread and return a valid existing configuration). Do not substitute another existence check, destination truncation, or delete-and-retry. Ordinary settings replacement can retain its current publisher. Add the late-arrival regression, including byte preservation and no stale staging artifacts, plus actual CLI -init-config coverage of first use, existing valid/damaged input, and repeat ordinary startup. Existing helper tests and the startup-refusal subprocess never pass -init-config, so they cannot guard that CLI contract. Keep broader concurrent updates and storage identity explicitly deferred.

### R2 / P2: documented container first use now fails

Anchors: README.md:169-179 (Docker quick run/Compose instructions), docker-compose.yml:3-5 and :18 (launch contract), Dockerfile:42-43 (default arguments); the patch's added first-use guidance is README.md:229 and docs/handbook/01-install-and-first-run.md:27.

The Docker quick run and Compose instructions start a fresh /data without -init-config. With this candidate they exit at the configuration gate even when the required environment token is supplied. Compose's restart policy repeats that failure. The new native-binary paragraph does not explain how to initialize the same container volume, preserve the full listen/data arguments when overriding CMD, or stop the initialization launch before normal service startup. No launch configuration silently enables initialization, which is good; the missing bootstrap instructions are the compatibility gap.

Smallest complete correction: document an explicit one-time container/Compose launch against the same persistent /data, with the required token and complete arguments, followed by normal launches without -init-config. Explain that the current flag initializes AND SERVES, so bootstrap must be stopped before normal service launch. Do not put -init-config permanently into CMD, Compose command, or restart scripts. Document that deliberately creating defaults over existing catalog/key state does not recover the original settings. Extend the CLI regression to establish the documented continue-serving/stop/restart contract. Docker execution is not required on this machine; the deployment finding is source-inspected, supported by native execution of the same missing-config gate.

## Separate contract assessments

| Area | Assessment and evidence |
|---|---|
| Fail-closed existing reads | PASS within bounded scope. readConfig returns zero Config/error on missing/access/I/O errors. decodeConfig refuses empty/malformed/non-object/null input, invalid UTF-8, wrong known types, duplicate fields, competing aliases, null scalars and null string entries. No failed read becomes successful defaults. |
| Update/publication preservation | PASS for serialized per-App updates and tested failure stages. SaveConfig reads before mutation, merges the validated snapshot, preserves unknown raw fields and explicit optional clears, and refuses an absent prerequisite. Unique same-directory staging replaces direct truncation. Serialization/create/write/short-write/sync/close/rename failures preserve old bytes; directory-sync error correctly reports Published=true. No delete-destination fallback or unsafe rollback. |
| Explicit initialization/repeat startup | NEEDS_CHANGES: R1. Sequential behavior is correct: missing ordinary startup refuses; deliberate initialization creates defaults and serves; existing valid initialization is a byte-preserving config no-op and serves; damaged/unreadable existing configuration refuses; subsequent ordinary startup serves. Actual CLI evidence below. |
| Startup/API caller propagation | PASS for inspected paths. main validates config before OpenStore, token selection, background export or bind. Environment auth cannot bypass the read gate. GET config/setup/views return non-success on read failure; PUT returns non-success without a saved config on failure. Jobs record dependent-operation failure; auto-export logs/returns errors. Availability endpoints may return an explicit unavailable/error payload rather than HTTP failure; no default-based helper invocation is introduced. |
| Security-sensitive settings | PASS for ordinary valid updates. Auth token, staging, keystore paths, helper settings and other fields survive partial updates/reopen. Startup uses one checked snapshot for auth and retains non-loopback token enforcement. Middleware continues capturing its startup token; live token replacement was not introduced. R1 remains an initialization exception. |
| Compatibility/deployment | Optional omitted fields, optional null maps/slices, a single historical case alias, unknown extensions and explicit clears remain supported. No new schema-version requirement was invented, and no concrete supported unambiguous configuration was found incorrectly rejected. R2 blocks container first-use documentation. |
| Regression validity | Existing new read/publication tests are meaningful: hooks enter production decisions, observers are armed after fixture setup, successful publication proves the observer works, and recursive entry/file-hash snapshots check preservation. Read directory obstruction supplements injected permission/I/O faults. Red portable tests fail on parent as expected. Missing actual successful -init-config coverage and late-arrival coverage belong to R1/R2, not another separate comment-only finding. |

Source scope: read config.go and all three new untracked test files explicitly; inspected explicit-base production and fixture diffs, Config/defaultConfig, uniqueJSON, normalization, publication helpers, setup/integrity routes, startup/auth, settings request parsing, and all direct LoadConfig/SaveConfig occurrences. Traced helper/view, scan/build/write, ingest/adopt/inference, finalize, mirror/incremental/plans, escrow, keystore, recovery-kit, export/restore and background paths. Required errors propagate; optional metadata/health probes consume already validated snapshots. keystore_validation.go and its boundary tests remain unchanged. Accepted OB-006 strict sync versus offline recovery/generation distinctions remain intact. This is a bounded review, not whole-file or repository-wide certification.

Publication stages: update serialization occurs before staging; initialization serialization precedes MkdirAll. os.CreateTemp supplies a unique same-directory file with requested mode 0600, followed by checked full Write, Sync, Close, rename, then syncDir. Deferred cleanup targets only the operation's temp name. ConfigPublicationError unwraps OS causes and distinguishes publication phase. Failed saves return zero Config; no success response is sent. SaveConfig and InitializeConfig update runtime hash acceleration only if bytes were published, including directory-sync error. Windows syncDir is a no-op: no Windows power-loss/directory-durability claim. Restore validates current/archive config before member mutation and uses checked publication, but other members may already have changed on failure; its error says so. Restore's live hash preference behavior is not newly repaired by this patch and must not be included in a universal claim that every restore updates all runtime preferences. Setup remains two sequential saves, not a transaction.

Decode errors intentionally avoid echoing values; read/publication errors retain wrapped OS causes. A nonblocking diagnostic improvement would preserve safe field names/JSON offsets without values for malformed/type errors. No secret disclosure was observed in failure logs/responses.

## Executed reviewer evidence

Evidence directory (local only): C:\Users\nsott\AppData\Local\Temp\obx004-review-24c622789f7d45e197f09218bab5efd9

Process-selected Go 1.26.8 windows/amd64; GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off. Existing approved module cache reused, fresh GOCACHE beneath evidence; synthetic GNUPGHOME beneath evidence. No installations, global configuration, GOTMPDIR override, WSL, personal keyring, real backup data or hardware operations. checks.ps1 retains exact selectors/arguments. Native exits and timestamps are in per-command JSON; stdout/stderr are retained. target-list.log enumerates names before execution; safety-list.stdout does likewise (TestCatalogScale was listed but explicitly skipped by selector, not executed).

| Reviewer command | Native exit | Result |
|---|---:|---|
| go version | 0 | go1.26.8 windows/amd64 |
| target test enumeration | 0 | exact names in target-list.log |
| go test -count=1 -json -timeout 2m -run target-selector . | 0 | 73 top-level passes; 66 passing subtests; no failures/skips |
| safety test enumeration | 0 | safety-list.stdout |
| go test -count=1 -json -timeout 2m -run safety-selector -skip TestCatalogScale$ . | 0 | 18 top-level passes; 6 passing subtests; no failures/skips emitted |
| go build -o <evidence>/obelisk.exe . | 0 | build passed |
| go vet ./... | 0 | passed |
| selected-toolchain gofmt -l on root Go files | 0 | empty output; read-only |
| git diff --check | 0 | passed |
| disposable no-clobber probe enumeration / test | 0 / 1 | one expected failing safety assertion reproduces R1 |

Exact executed test/subtest names and outcomes are retained in target-outcomes.json and safety-outcomes.json and raw JSON logs; top-level names are appended below. These focused selections are not a full-suite run. Author-reported final 240 top-level passes / 39 skips and 72 passing subtests remain reported evidence, not reviewer execution.

The first disposable probe copy omitted embedded documentation and failed to compile; probe.stdout/probe.json retain that failure, which is not counted as a safety reproduction. After copying the required docs, probe2-list.log enumerated the test successfully and probe2.stdout/probe2.json record the actual assertion failure. Only reviewer_probe_test.go was added to the disposable copy; production candidate bytes were copied unchanged.

CLI probes used the newly built binary, explicit disposable -data paths and ephemeral 127.0.0.1 ports. Each serving process was bounded to five seconds and explicitly killed after HTTP root 200. cli-v2/*.json retains exact arguments, native exit, before/after file hashes and whether the reviewer stopped it. Server exits -1 are forced stops, not successful natural exits. Initial cli*.json had missing ExitCode due to the PowerShell process collector; those incomplete logs remain, and the corrected System.Diagnostics.Process collector reran the matrix under cli-v2. No earlier exit was invented.

| Actual CLI case | Outcome |
|---|---|
| Fresh disposable directory, ordinary startup | Exit 1; no config/catalog created |
| Same directory, -init-config | HTTP 200 and continues serving; defaults/config/catalog created; reviewer stops process |
| Ordinary startup after initialization | HTTP 200; existing file hashes unchanged; stopped |
| Existing valid config with -init-config | HTTP 200; existing file hashes unchanged; stopped |
| Existing malformed / empty config with -init-config | Each exit 1; original bytes unchanged; no catalog created |
| Unreadable config (directory obstruction) with -init-config | Exit 1; obstruction retained; no catalog created; not an ACL test |
| Existing synthetic catalog/key file, missing config, ordinary startup | Exit 1; both file hashes unchanged |
| Same existing state, deliberate -init-config | HTTP 200; config added, catalog/key bytes unchanged; stopped |

The existing-state initialization is deliberate creation of DEFAULT settings, not recovery of old auth/tool/key-path/staging choices. Catalog/key files are not erased or reinitialized in the tested case, but key paths are not rediscovered and original settings are not recovered. Reconnect/restore the original config for normal recovery; choosing defaults requires subsequent intentional setup. Missing-storage identity is still unresolved.

Help was inspected from source before probes and from the built binary (-data explicitly supplied even for -help). Help does not say initialization exits; source and runtime show it serves. Native first-use docs are consistent with serving, but the existing tests do not establish that behavior. Docker is absent; Dockerfile/Compose were source-inspected only. No additional service/start script was found in the repository inventory.

## Retained red/green and Windows sharing evidence

Read the implementation report, CODEX_HANDOFF.md, bounded persistence findings and accepted OB-006 detached review. All implementation-report candidate hashes matched initial working bytes. The portable config_read_safety_test.go also matches the retained parent copy byte-for-byte (SHA-256 93ca9201dd4d0d7d4fae63abddfa143de841ee55f17b7b8fb09ddcee59b6bc9e). Retained red-probe-01 records both top-level failures plus all six damaged/missing settings subcase failures; retained green outcomes are historical evidence, supplemented by this review's target run.

Retained safety-01 and exact-parent red-reopen-01 logs both show TestDurableCompletion_C_UnrecordedTerminalStateIsQualified failing at durable_completion_test.go:296 because catalog.json was in use. Parent follow-up reports 19/20, candidate 20/20; those are repeated executions of one test. Independently inspected unchanged runJob/FinishJob/noteUnrecordedJob and the test: terminal-state observation can precede the subsequent best-effort catalog audit write, while the test immediately reopens the catalog. This is a plausible existing sharing interleaving, not a complete OS diagnosis. Current added ScanFolder config read does not publish catalog/config on that path; job recording and catalog error handling are unchanged, and unreadable reopen correctly refuses. No new destructive handling or new sharing cause was identified. This review's focused run passed the test once; neither that nor parent reproduction establishes absence of timing issues.

## Deferred and optional work

No blockers are assigned to job loading/null rows, complete multi-process coordination, concurrent update identity, retained generations, cross-file restore/setup transactions, broader catalog missing-storage identity, ACL overhaul or unavailable PR-04/source reconstruction. Windows race detector, ACL enforcement, power loss, Docker runtime, CI and hardware were not executed. Missing GPG/PAR2 still limits native integration; JSON and CLI passes are not encryption/recovery integration evidence. Optional richer sanitized decode diagnostics can accompany future functional work; no separate comment-only review loop is required.

## Candidate preservation and identities

No AGENTS.md found in the ancestor chain or repository (including hidden paths). Branch and full HEAD match fix/obx-004-config-read-safety / 26918c5b8ed01ea301fc9a4c7658e19054a73632. Explicit-base diff was used, never main. Index was empty; no merge/rebase/cherry-pick/revert/bisect operation marker found. Candidate includes 65 modified tracked files, four new Go source/test files, the implementation report and preserved untracked Figma. No prior completed review of these contents existed. This report was created IN PROGRESS before test/build commands. Full initial file SHA-256 identities follow, including source/tests/dependencies and historical reports; final comparison is recorded after them.

Full initial candidate/file identities:
```json
{
    "proccpu_other.go":  "db2da07bf3bd675e15e57862257b945d26729b037c33ef94477cf6eafa60a581",
    "ltfs_unix.go":  "2e3ab33767ebbbacf9600672c52a63396f8e516e78cfd77ab532b6c35f28dca6",
    "recoverykit.go":  "d9b2f61c206705bf54093c077db62d1b7e1198fee72a7a3f631413c792e06fdd",
    "catalog_open_test.go":  "13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3",
    "apiguard_test.go":  "95cb88f23d0e2e992910127de85d5e14df24f4160e1bd31f5573a13774963202",
    "finalize_test.go":  "0a77eb30e658317a6b1e62d3bd046a0b8c9f4d8019107794582c2002bdb38f3e",
    "integrity.go":  "e7f0cb521f0d81d88719957acfa53d94ca6fc109e0415b11348d43e6dbe8b4ec",
    "go.mod":  "b95a14e02c8faf2a4aac73901b843bc79ba154fd15aeaeb6161391bfaeb2c2bc",
    "docs/development/NEXT_ACTIONS.md":  "539fce8744b31da871e3882e9d2111a989d2c7738f2890e13c61fc99c8cfe682",
    "deviceid.go":  "15ae93b4db80b592d6ff382e24baf02de2e55a690b6273a9915098a1cfe943a1",
    ".vscode/launch.json":  "9e986e720e070dfcaa67c539bd893fb6c8ec8d5810bd90864e17bc0ea8b1fc08",
    "docs/development/reviews/PR03-OB-002-UI-PRECEDENCE-CLOSEOUT-2026-09-07.md":  "aa65cd466110aecf4dd7a3740837f90458a9e727c3c70cac1b56429e5ba889f9",
    "diskfree_unix.go":  "8b6e4f56829bbcbc7caad6a75551f31968ac9375e4b542827484e00336a9bbc4",
    "scripts/banned-phrases.txt":  "c6e7ac9c1fc5b5d2ab16615256accdaabb17f8af05da5f2580c6a81889a97953",
    "docs/development/PERSISTENCE_REVIEW-2026-09-19.md":  "8aa24fecdabd09843f8644fd3aee947a34652a7eb2d66d4ead9c5c56be3d998a",
    "appbackup.go":  "544e653da8956c16f3f8886a6ce69f1e432bfc8a0400843690f23fb837f8894c",
    "rename_compat_test.go":  "491764e168c1d336f84da0ba07d9d59124562a656eb30ede783c470cbe1042e7",
    "docs/development/reviews/PR02-OB-003-EXTERNAL-REVIEW-2026-09-07.md":  "83d25498c75698d03553c2c08f719a2415809b55bb89db432902d2cfa37cc0a0",
    "docs/development/OB_STATUS.md":  "3da881637a09f3061cb25fd4241bf35f7f33b5b659d36b9ca6dd7bde87293541",
    "hashing_test.go":  "daf9c7ea3f6dc1a65fbde2c0902fa9e5f7b1f814448813366945aad717210e81",
    "inference.go":  "5036685260b539657ff0ebdeb3b1faa3af830317267d724045f0ccd13c9e040c",
    "home.go":  "99678d7955ec0ae22091b64783b4df0a93d00bbf9981ddba4e2920c743e1a04a",
    "removal_test.go":  "3156295ff2134330df6c388f3ace625810c942d5595ac0cddeba8168f2a8f46f",
    "docs/development/REVIEW_COVERAGE.csv":  "a2381c647ac5d934e58d3f7557ff51b2042cf138691126f5929a2189776e0241",
    "docs/development/reviews/PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md":  "774bab36e0dc794e7662b44e8879d907b0d4c3f6acce9b67859068e4d2453bd9",
    "perf.go":  "56fb5f54bf87f414551c9d3598ba1736816f4e29e0eeefb3fc7fece6a53027e3",
    "docs/CONTRIBUTING.md":  "9b478da3f9ced95a9f131badae2aa85e7c616ca4d44d1f4219e9c30500069fda",
    "docs/development/reviews/PR02-OB-003-IMPLEMENTATION-2026-09-07.md":  "adcac2c082e9e2daa0f63030913d4e04a300f9c2bda03d7d267628c54107fc2f",
    "meminfo_windows.go":  "9626face4212bb23b3278059c4bff3b53ea39986287096dafbf04851b91ad38d",
    "dock.go":  "af84d0bea8797be35047fc1eee93b4302417c9a1505707d12f6d4ffa83b5ed52",
    "integration_test.go":  "20d1023e214b17c8bfc0d37d584c782229fb934fd81785185354422e8ec7f1c8",
    "escrow/obelisk-src.tar.gz":  "ce3eeca6bff24f3a0e6cf6d4065a7395bd27740ccddf4b3a44afa0e4a22254ec",
    "versions_test.go":  "81a0b28ea4077b0c8e7f0e83021238667e416b96c6f9d0bcf5261caa366e5a2d",
    "formats.go":  "ec8d134b7565a7330f7f602ad95977f137a6d16ff0c993f42f4b985b1bb2f3f6",
    "config.go":  "b3adaa80451eac6db939d82718ce166d6d81a45e6c4a2f7f4361b672bc2c77c3",
    "verify_levels_test.go":  "6a0ec5b763c1a0e92a1b2ab466965b790e78b41e6c30a1d04e46504c4b465c3b",
    "versions.go":  "3cdd6491bd72b032a2efd46db3bcc011bbd6b6301708243bf3b813c0a4e20b72",
    "docs/handbook/04-the-3-2-1-setup.md":  "b32cce6f075eb540ba20138eb09e85a4204f39e9aeca2fccb2dad1fac2b28b9e",
    "smart_unix.go":  "d3c98a10cfd7f868557b913e2b002949f5b263c81b7fb758e495f3133cc91f93",
    "docs/development/BASELINE-2026-09-06.md":  "57988c0b857c69f3b166dff77a95d86b6e86b38ec2262a24e73b56737552608e",
    "docs/development/reviews/PR03-OB-002-UI-TARGETED-RECHECK-2026-09-07.md":  "8d51293848e0fff31cf9d8ed18a934d4aeb38953c8c82e828fe52ad0b51c1a6d",
    "ltfs_windows.go":  "6cc84197baaf97b9bc981687656d5d1d52e453c767a560d20d7393ae09024a19",
    "mirror_test.go":  "900181d4eb964de1c502a30c79a82ea9cfae11f3fa0fbc1ddf3ac1190a8fd854",
    "auth_test.go":  "e4f65a66ebeb987500e21497c1450ce55498d98d700e1ce1618c5d83eaa63e80",
    "docs/development/reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md":  "803232e8edf1eb65c35be7d2be83045c11b3aadc826f7ab0087b4656cd4485e5",
    "dvdisaster.go":  "b056e271efb878f6b867f52958412581e7b6a7592e1a7b6fc23dfc6fe63911d1",
    "docs/development/reviews/OBX-001-WINDOWS-FIXTURE-2026-09-07.md":  "b6281c6b4a1d2758bb99cfaf467da94f5bbd9f4f1465aaf2e9a5acf0540a3f1a",
    "docs/development/reviews/PR03-OB-002-IMPLEMENTATION-2026-09-07.md":  "6b734a3c86b1e093351bddcb2ca5b3a35843da23b152567e7848d614791d235e",
    "keystore_validation_test.go":  "878452c2cd3d7e5f3afafcbbde4aa41ae1c3c957e9242f7fd1114e5cad90b86b",
    "docs/handbook/06-discs-and-drives.md":  "a8d876ac25812f654dedb470cdabc5bbb10bfde53535b5094eed5e746b98e490",
    "datamap_test.go":  "299196163b678ebb55c44934f16817e5c220393a69bf515a92ccb1850dee9b81",
    "smart.go":  "19b3b3f6239aeef4588ec4498518562fb5c3a2758658a98e6f09a43512d5bba3",
    "exports.go":  "1ad5ef43a5b7a8c2d550f69763c0a7db8a1ffa7c234c43a09c97db681aabc196",
    "writer_flush_test.go":  "257123cd088ced2762ed440f322c2f1bb7cbef6e43471f5db5dc11a80b0e6f85",
    "docs/handbook/11-keeping-a-backup-current.md":  "f2e21e268cc839575ad9a13f8d959398b398b86699949e0f8eff477af01da218",
    "seeing_what_happened_test.go":  "86a775030bc82fb1404071a430bc238430b9d72e8064d46d53e69a3b64575d18",
    "generalize_test.go":  "3519b41e10801fe13f14b014ce3bf6e83ac474052edfacc753cf237b7df2760f",
    "escrow_docs.go":  "6b8ae87baba575c724b2c84182d605b92854ddefd1233b2f784be403620fa5a0",
    "docs/img/README.md":  "9c6ceddaef19ee07f41260a4b30b16a57f0f2184a68437248cb64a48e0e63bbf",
    "escrow_test.go":  "0c7765ed8d895d72cdbcf12a35136d3804442c96e85bba88cf23d8d721ec997c",
    "tar_names_test.go":  "ae9cf03ccaa1d054fa7dd1db974a3467c5dfde91b627a4bf13d675546dc568e2",
    "durable_completion_followup_test.go":  "0e22ecc5124f41b97bf5edd3bc8743da96f398200d50f99e7bdd06f1e3e21dec",
    ".dockerignore":  "ed6ca17f188919dcd94f021ed5d689c0483096c21158439690caaf2ffc41682c",
    "scripts/prose-lint":  "5e7d1117cb48fd607c1d754c839f01ec096bfd4fb4a8b621d5d77e089fe0771f",
    "config_read_safety_test.go":  "93ca9201dd4d0d7d4fae63abddfa143de841ee55f17b7b8fb09ddcee59b6bc9e",
    "tape_parsers.go":  "5425ffe8a3ec915e204b763eb3fecd5d3b9a43606215607c65aebf02a837e565",
    "docs/handbook/00-what-is-this.md":  "70029afe6542883eac60f739d4dfcecd6ace5373eac856240efb09fac7eba810",
    "docs/development/reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md":  "60d93d467c90b2c6268f3b351a6f37ef7970f260e0078830c8f126e56995c670",
    "docs/handbook/02-set-up-safely.md":  "d3677cd196d73b53c85a7817f4bfe2e81a7d74a130bb5a504f5588375b5f542f",
    "docs/handbook/README.md":  "ae757cc11317d0d9d4b7ac83d99e3a93d965ebec0d1f9eca410a02668309730e",
    "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-FRESH-REVIEW-2026-09-19.md":  "7b0b349578910b4edfa8fad5edf0cbf889e872df3a0385d5254c2bf87c1c9ed2",
    "testdata/catalog_current.json":  "0a225a5c3beabe0914e685760688274079b4438707f4f0f8c626249ebe36cf57",
    "docs/development/NEW_DESKTOP_BASELINE-2026-09-19.md":  "4a54cc02eba76e52ab3cdafac17855c3ad17b876c789f9e7b90dfc866d1b8061",
    "mirror.go":  "1957dfc6254d0ca14f13fbe8e9878fe7fb4f4ea07eec2c38ef66b15eeef86fec",
    "drift.go":  "ee80fe0f96d0f95661f7c46784cd1266ccb588ff47b9ba7c6cff76c0448523d9",
    "copy_health_test.go":  "18bc11f70c6cd6cc678886c713d576985104518afa1dfd89135ff8219e6a7877",
    "config_test_helpers_test.go":  "747129bac34f36f514b665fff335c663de48242e1968af4cb5ed57d190f98e96",
    "perf_test.go":  "c284955f30916954cfbcf3b2681e4211833f4d19b025f5f4b828dc08546c38be",
    ".gitignore":  "6ae89a2e30171579e45edf2146d378a118093f94c40114d43ecbd3ed2e124fde",
    "docs/development/CODEX_HANDOFF.md":  "61a542ab6cb48e36ddc7de26379adbb95564b9d3005f1276ef2e6c692e12df74",
    "meminfo.go":  "6b5ba49f26d857bf622a052dd54c9a850a289547b48fc9b4f3ffdcdf4fa1a24a",
    "docs/development/reviews/OBX-006-CONTAINMENT-2026-09-07.md":  "b27a7d3b54ab67f6472912ed295ae02129d60e942a9048dafdf7cbc1c4deb262",
    "span.go":  "6a47340de6c5b6a862660f9c032e8b041cb970e254166ba6c85d6058b50783b6",
    "cardcheck_test.go":  "c6d689dc4b1d11b57af224b69b051f42a3ccb743100b44efb009527e04d7fc5a",
    "incremental.go":  "96b3b250451e8197e8882a37a9f9a7c5bcd0cac7c91665f701b19a4e23df72a6",
    "proccpu_linux.go":  "0114d4c5bf5d769431c7f477e8bfc6965162d6ebcd1faafe9d5ff3a0a425d5ac",
    "dock_mounts_unix.go":  "b5eb84b2ea8cb47648d0d7bbf28bfd591f353911012b44dd057714ef00218b40",
    "quarantine.go":  "b440cd67b607eec50392123c86ff08c570cdb3490e3f3ea6a490eb9869136486",
    "dirsync_unix.go":  "3479a216926bf995714f12afecd6b88f6ed6374b66c62b14a6d046dea5a78495",
    "paperkey.go":  "d99a78a3e0af4e0be737ba5b4f770f0135b343dc69850189f635bd4eab6401e6",
    "docs/handbook/10-where-your-data-lives.md":  "e2b0a963cea4f1b76d1e74a5559e8d80b318a46a8461578da2e7bb2c2a55d0ac",
    "incremental_test.go":  "fad33655a945b6917d2de9ababd5dc3a6c61afcca57407008e40824942a3b05a",
    "ui/index.html":  "7c40475011aa6935273f14ffb3ae97b8e667b087f528f011d0d74d9a227e1656",
    "docs/development/reviews/PR02-OB-003-REVIEW-FOLLOWUP-2026-09-07.md":  "b4eaf0c610e51c6007d73aa50f697249eb50b8923a042417e9f65da156b631b2",
    "docs/handbook/08-getting-files-back.md":  "898e707654eeb7356547d335ccb59cb615271635f988f82e2c694d660b973a86",
    "cardcheck.go":  "7fdcd6ac234c6422149ccd123fc05b9634d9065e867cd029b9e8206c0e488bfc",
    "escrow_manifest.json":  "7208612ba0640eb7ed90996713e89aa6f14e0e3984f6afccb4e9f5a4751242dc",
    "meminfo_other.go":  "f158dabddd7ae7bf867f196c1168cbfee1e86d20eebc0c1c7e4e434259d46d0a",
    "atomic_replace_test.go":  "b46875d17a29c716ec5308f236feae47471d1755c5ce124dc3d0728cee1f0bf5",
    "tape.go":  "2919ff9bc9d03dbe2c9a4c89587bc8a74186ee15b8a70a5df381b8621978de8b",
    "migrate.go":  "e10ec332db58e90305cb774b0615192a1af7ad2edf4ef0785933df357d290db4",
    "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-DETACHED-REVIEW-2026-09-19.md":  "ce903017e74c1e5cbff906a02aae5e655572ccd6c2805a4c5d21af3c6b41e154",
    "go.sum":  "9d169ad00514ad13d9a10e1b1ac4723171c020a23b792c76721d97ccd47a1993",
    "testdata/tape_sg_logs_stats.txt":  "fac0060b323284afe5bbed5b142b85167ea4f41c07f6843a79429e88b1891209",
    "formats.json":  "a9272719772744a403ee488794c33aeba48b16d38d78ad4758072a74123d7509",
    "volume_identity_test.go":  "8774ea98e908e0570b6d177b463d0cb1bdd2ed8df296a5716e9bde052b7a5217",
    "docs/Obelisk.fig":  "69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda",
    "hashing.go":  "2ab02f40830ce6abb6390bae8040aa277359f35fd8764214ba524fa57c3268e9",
    "scan_problems_test.go":  "ebeda532427af8fceb6c83a8746cbe3cda70df071c9fd82e8c62fe41a1786a43",
    "plans_test.go":  "8b8989cb5eaee533a3bde2648f44b8bd84a1b5eb53a9b49082c437fda42a592a",
    "templates.go":  "dfce374db3b4907616ee0b42a7fb0cd37ae7b6843738862f7a83caa8fecc3f06",
    "label.go":  "db31c7636192ae9fbc4d31a39c1440be5db87abac0267d5ade76cf948962833b",
    "tar_unicode_names_test.go":  "0a9c662bc2a6e6e43a104b0903f6056ffbe912bd72d029ac6e5f503d9fdabc83",
    "diskfree_windows.go":  "4ca61bf8ed12f9853ea7498957ad668784ed565aac061b858e074c128e5c3270",
    "stenc.go":  "982e6c74c9741c48bc462171acf8bb6c7a3ed76fd04a8fc178ce1cddc4307571",
    "smart_test.go":  "a05f3523aa76f09961ee02b1179786ab7cea273f024c654a9fda46128f0644f3",
    "exports_test.go":  "bd3db4109259bdf40490b7df62d773a1f958b48085c9fd252531b746facbfb1f",
    "docs/ARCHITECTURE.md":  "63c19810b5f03bf31c292cfdc271411a82efc65156023eeda2333c7ee8ea133e",
    "testdata/tape_itdt.txt":  "2045b5d6a64eef66a4be173184f6a2d0a463ade187bdf38efb323bfa5ebfb25f",
    "snapshot_test.go":  "0b45e1e13cddb603951819df13aa1c4cb24090df13aeaef4d5067a3ce95f4a4f",
    "docs/handbook/03-your-first-backup.md":  "4fa4f9b3e72990c65c8768308c3b897d14373d0efa18efd86cf7f270effefe1b",
    "dirsync_windows.go":  "5fea9c891ce63e8ad169723778c57e529ec0e9614b202d3d531ddc4c0270a4c3",
    "sourceless_test.go":  "49a4d9819047374e98bfcda5fd9cc03bdbba3332d0e1ed4777b64a0b8b0c6f16",
    "docs/development/reviews/OB-006-KEYSTORE-VALIDATION-IMPLEMENTATION-2026-09-19.md":  "7b91b221977d8482408538c3fa2f04aeec437358c927949ce4441725312eaf44",
    "testdata/tape_tapeinfo.txt":  "a426f917b823eac9069b51a0b69bbec39ecd9b05c5f8982e890698eaf3fd5f5c",
    "adopt.go":  "c993c006d12f4d9859e04af12f8713c535babf0e9d0917dba322b67d4792422e",
    "snapshot.go":  "76ab5a779c8d7c3bc684d421403c775d32e6a7a28c50fed25ce8b97db1a13e00",
    "tools.go":  "033c3118c62895d619843a880f834ff6b65c279e1de0663475ad7c9c54f9bb9d",
    "testdata/tape_hpltt.txt":  "a8f001303ebc8a36af71127665a43df12afc34b1e515710f12eff97cd48c0763",
    "treemap.go":  "c93d967973a7f8808d7cc2d558127c8f054e9a8618844252f110ca698e59501b",
    "catalog_scale_test.go":  "c99b4b201f82ae4793c2f8f0fd444649924c1a8e54e61b3078a11a65ded74c73",
    "events_test.go":  "5a22e26e495eb59f7e5206dab1377764af0423f82d6521a4004b839b4c9b3a87",
    "events.go":  "b3e4839bbd9d6ec3163532d381cd4442a501ae8ac92cd6766a5cd3943a4f6789",
    ".vscode/extensions.json":  "fcaabcc9af285f129e11c3db4677c9413bc1545c12327997e4b931de14f7d9d3",
    "conflicts.go":  "c6d353310ddd4c71b79d6821d5cbdfa9fe95d45f1987c6e71cffddbde245f007",
    "docs/development/reviews/PR01-OB-001-2026-09-06.md":  "78a45fc095c9dadbb19bb7764ac13f9fd86b9e2497499c124d6122c0d2a36f25",
    "meminfo_darwin.go":  "e4a9a0e82fff49d82355421b67bf4343cc8ec3a75cfbe70b370a0db57c7c2e1d",
    "docs/development/reviews/PR03-OB-002-FOCUSED-RECHECK-2026-09-07.md":  "267705583889cd36343e732cf4a76cf6dc6236e106da13b0b1463c3c170decc7",
    "burner.go":  "62e7fd9e9fecb2b34fc459caf1f5a7057865be7add85a875d370e04e912308d2",
    "writer.go":  "6f049544015b84ef05de41ed87fe5985a8de50b455d60fb7dd81a42b2fff8294",
    "store.go":  "e91f658d6733302aa87d7ce720163bc9a88df71ed7bc55a9ea48625286d374a7",
    "docs/development/reviews/OBX-004-CONFIG-READ-SAFETY-IMPLEMENTATION-2026-09-19.md":  "34acd295d541824c57fc6d3f47b150db638a92d9abc2c588817ed2e9e6318f9e",
    "testdata/tape_sg_logs_alerts.txt":  "b54a57e43d3d272a2466fbb3b63b16b2eef976ca61d9976fdef8013dc1f1bfaf",
    "setup.go":  "7be4fb9f09a33c39fe1f505e9721979474a1cd60b814d6c52a9f5a4c1ec2bce9",
    "persistence_test.go":  "0509567d2d7803f0da6c463da827dc9ed5ef0b2985b53f81f177e4c0378af975",
    "privacy.go":  "dae7e14fa45e735bddfe0be62a55a189591c665d5c89a184a068e01738762f56",
    "proccpu_windows.go":  "c7a1f09323ecbe7cc5f3a325bb1bcfb0a191295edb83eca63982e3a0dd3c765d",
    "deviceid_unix.go":  "7bcbea76b28d89176fed89cfe9ad3ed27b261bc1e0baf85bd1eadf8f3dfb571d",
    "space.go":  "bbe0b4e55ed48c6132e6ac8f1589c62ecd4807d0dee6125edd175077f6c6888c",
    ".github/workflows/release.yml":  "f57ac7fd9df8f391ae900d6a3efef25cb143fcc21adcadec71cf2bfb061f0573",
    "docs/handbook/01-install-and-first-run.md":  "80f4354112a4f482fb765927372541bf02e5e08c065fe6126980e513620445d7",
    "exif.go":  "dc04f54ac598b7c0924fe14f37e0f343f1e37a5a7c63ebe9eedde4bc1464b0a1",
    "Dockerfile":  "4ebf210eb94d59ad2a8df079ae345273ea8e6d26ca710f91409ad6628f9e5a45",
    "treemap_test.go":  "4df18871ecf0c2bed641c843ba671bad5f00511b51a79953bcd0190d35a36667",
    "verify_levels.go":  "787b1d2bd602ae5a9a5b0c947005aa47ceb6bd32af753a512cb2be6aee786c0a",
    "testdata/tape_tapeinfo_clean.txt":  "519860a9a451e9ff7845efd8599e4efa313391e801e330a298da8521e150c2d6",
    "uimode_test.go":  "c9bcea36317ec710c10f311dd2f9feada3b434e62c13f03aeb952c5061e4fc15",
    "finalize.go":  "9e5512861d589a9716a39ef4d140f2f998a6baaf538c3b0d6d1b520db5a98b34",
    "build_verify_windows_containment_test.go":  "73b8343b61b2434789414ec6eb4de8621a18b2d8d1078b9d39e84f0ee4d63bfa",
    "paperkey_test.go":  "83ca9ea596a9686a921e1943f453cc5f024b89eab8cbeac8ef31288af19aeac7",
    "docs/handbook/12-moving-to-a-new-computer.md":  "fc2a2e8dc0d6436cb7d2f18706d8fd65e35a360488598a351b644e6a38e12b73",
    "conflicts_test.go":  "d116585e4dd13ca877dcc5218309946a0efcbb2a097cb8b02cab3343ed6df090",
    "docs/development/reviews/PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md":  "1b5cc6042dce58acf015debb18f96bfaf6139d3a3c9fc1a80a2f5fe25beab33b",
    "optical.go":  "ea4827e9c575012d650fd4a623510988ba275dd5ec0947e10d8d3541a12fa36f",
    "metadata.go":  "e78a2784ee0d679e16c13e820eca8747dd8bf2f9f5b011ece05b64cbd5efb00e",
    "setup_test.go":  "eefe9bcb661f3891823a47fa41095c78fd5eb151f022eb8bf430f172027b156d",
    "config_boundary_test.go":  "5e70cd6fed9fab23d7033a8149f9498b27858645a9e38045beb36629bc72e897",
    "README.md":  "696f7c9fdaa3f5270edd125e117daf4245f7fbb23ea5224295c445f636d37571",
    "docs/development/reviews/OBX-006-CONTAINMENT-REVIEW-2026-09-07.md":  "faa18d13253186b81af1f5a71b0b11498197c57e6e09edac1882faa8f09e63ba",
    "docs/handbook/05-tape.md":  "485a989a4c94cf7dfc3e8736faa9a057b0bddd3d5a057a03730cf4996b53fddc",
    "docs/development/reviews/PR03-OB-002-FRESH-REVIEW-2026-09-07.md":  "18df0387b5af04c1dc99fda5e4b55db2099e001351d1a276ff499c91e1738c77",
    "integrity_test.go":  "3e72e3938a1db5054396a75f64f738b0eee56a19ca8e7b0fec6ce6a43e2fa919",
    "meminfo_linux.go":  "746c245014ccda0625a1d60cd28883c585101e50aa3581651e289d799986f3cb",
    "testdata/catalog_schema1.json":  "da3104c2cbd1698879d2bcdb0b53f7db0ffd5bbfe84fc36935bcaf2f6d2a9e7c",
    "durable_completion_ui_test.go":  "c9746527013416cf74b03895248d45a34b0cb678739df4ad7948d1993e9f93f7",
    "keystore_validation_boundary_test.go":  "2b0122a48d6969336e77fb8657d918c7fe2e1390e6310f0cf394c07c7032db24",
    "appbackup_test.go":  "36f29bf5dd536923ec16dd9ad0d4d7b06d6b1ed4b0ea070d1b2620f7f804a3b7",
    "schema_test.go":  "002df0dde5face40db725562b38e3dd24124c407d6349a890ebfd8f2b706fe73",
    "docs/RESTORE_RUNBOOK.md":  "7aea44781f700d38b249590fb793c55f35ceb9defcd5dfe23284960efac9128c",
    "browse.go":  "6919e26d8ed56e0879900474584dbcc72aabc02d95f203d431c1beaaa1658340",
    "docs/COMPARISON.md":  "31337747bbdfa2c6121ed481329e91cf62f795ad20e53ce58772acb76bfd2ce6",
    "keystore_validation.go":  "29bba5f7557fbecb7fefe9dcef0bcf62dce7d28f060e165bcf3306d9f3ac0593",
    "datamap.go":  "d4c8d0d7ce26eb5885e9de65a87e09af52be6ded66a03d1d78b1043e9d8b3105",
    "dvdisaster_test.go":  "9d4f744cd754b7d5878b093e8dceea14281a1c66ba307c374d3e8c1e7ac270af",
    "adopt_test.go":  "2d052655aa4b8c1fd3be57c2d74ad77a0870bc22534617e44a281220b050b7fc",
    "bagit_test.go":  "f973ef4eb725f3bcc455810077d828a224a32a0c78604ead8c7bbc1a1de4a509",
    "home_test.go":  "dc566cf7ba73b57de2d33c57cefad5104da8fe2c04e6c4ed8bf0d34940cd5c74",
    "dock_mounts_windows.go":  "057a0a6e915b24b27c672d590c6465d9617e5d145459d0f6ad1e53d986bf281f",
    "escrow/PLACEHOLDER.txt":  "d52b7f5972e12ce4b7720e1293e186f742fbf6408c1fa8f1e1a9ecd01a3e28c1",
    "docs/handbook/troubleshooting.md":  "9b67ae046a5638e92848336e8c337a80cf31b069475b54d4378052c8a5914788",
    "build_verify_test.go":  "d12acce608c680c61e442581b11bf4dba3facb4c2a41b1013fd38cba408bb091",
    "main.go":  "02706c9299b08d5cdb30d251f26655e89472f324cd6dadc0f04497ff6df27b22",
    "proccpu_darwin.go":  "710ef28aaa647eed06dafc552a0a6ebd0b7e54508c0198830ece0a45c25aef0c",
    "docs/handbook/09-the-recovery-kit.md":  "6d477fb62f917d720419cfe469d84b1f1816a4a47839dae43c0c38c392577563",
    "stenc_test.go":  "154662ab4c32c11e4d2522d5fe71d332218f1630dc07019300dce6893318ab8d",
    "ringcopy_shortread_test.go":  "3ed9be8cc9c1a6821ebe11e12262eac6502dd65b553645e234f33882457f43d1",
    "docs/handbook/glossary.md":  "2533a13f5c54f0fa964db4c00417c139bfa73af64ce50f4cbb13dd2811ea87a3",
    "format_census_test.go":  "1e1fa9b92681a2eed7e71dc345b9f79259e384aaef1c0fd018c56b8cebb4678f",
    "smart_windows.go":  "1d3efcf5ab87be4ebe40602f45022e63e97ed59de69cee1e5c8e9a3f7ec23122",
    "tape_test.go":  "504be41bd4d1f12f29705b4f6c7a13a0fd6235e135b48be87b738e9cfe882c0b",
    "docs/development/reviews/PR01-OB-001-FOCUSED-RECHECK-2026-09-06.md":  "2009e5fff98f2426c49a2a6195035622f381a549b274d6b3f641512065f2cd37",
    "protection_test.go":  "5a70e156960d87c735ae1936dc3521a9b2806c851e257264860ca26d98d40f87",
    "settings_test.go":  "ef5c91bc629adfbaa305d479937e6723245d2a28951104b69ed9faf02331c668",
    "pipeline.go":  "09056e6275873afec4cf2f43b58bb993107813614de76455b426eabb4e7dceb4",
    "quarantine_test.go":  "0b59d71fe8646617441b1889d156ecddb512ccf838c1b7db6fcd660b24d0c7b7",
    "docker-compose.yml":  "feba39b90700e22168f53bd4646465abc1754f72bc742b2d4b48bb1f75101ee8",
    "features_5253_test.go":  "aa79ee9b0b3ffa7812108d6cae00aa98814f1cd50eb0cb00290d14224932f7ef",
    "plans.go":  "0d9829ef09e26041e7e684b3a7c7857d93c2141d370c12116eb918e2c10c99d5",
    "docs/handbook/07-checking-on-your-archive.md":  "2cf4f96fef6831a552045aca9729629458cef20cedf62d357e6afd12f47278d8",
    "profiles.go":  "78faaebe153910ba1c99cffad4ec98cd1c0b4118f72854eb98b6206937cbb154",
    "dock_test.go":  "579dd3dafb12b418876c7f22abf056ecc9b3e7fde1d6387aa01e3b25bfcdf0f3",
    "deviceid_windows.go":  "09784f2fc73bea033ebb0d8caaa3d86a124759c8f0bfd6ea5ace6cf043637216",
    "removal.go":  "c9edb9d6fe13eac499510dcb3023d35568f9cc738ecaaa234ca1b7af6b571acf",
    "tree_test.go":  "7fac45e28cf3fdf121ea0c8c4c679d33d30919bd401a31234af8d1c1f5d553b8",
    ".gitattributes":  "b7c753f47a9f1a383d9ddf6e2bc284cc54f3054c4226131bc29b75b7f21eb795",
    "LICENSE":  "9038ef6382d6b579257b5f25b30aa122e0de8c5491eb99e432818514dccd216d",
    "durable_completion_test.go":  "3a4837e343aa00d878d6daf32b6f5bc752564c403c315ef2bcc0cc999768351f",
    "durability_gate_test.go":  "3973a7a0f8d7cfb082d0e9a87d71045737198a30edcb94dc9c3d82a480855ab5",
    "bagit.go":  "d61c043b210f90bd5229bc170c58c3639b945f978431e49cbf7c5f47f5aa6e00",
    "payload_naming_test.go":  "c8ca561f87f254f0d7b1a071c6f69784d1b683a3d808c35ae99bf4e0ab6ee28c",
    "RELEASE_CHECKLIST.md":  "73e6f073a3f257dd62ea46c5c4b3d56bc1510f0a174af8df38e43795f960f16c",
    ".github/workflows/ci.yml":  "ae6e091287aa9102c65cc943cb7bda250019bf4a517c56ff2a23c637885ce787",
    "docs/development/reviews/PR03-OB-002-REVIEW-FOLLOWUP-2026-09-07.md":  "253da4e9ae95f6556ad63a49a34d7543ac9ddff9dea3b0a26e44897e64de947a",
    "escrow.go":  "1f266b780e27e5f561744c93ef6214cce68fdecd35a7c10deeb71952a1fe4d26"
}
```


## Exact reviewer top-level test outcomes

target (all PASS):
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
TestAuthMiddleware
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
TestConfigReadSafety_SettingsRejectDamagedPrerequisite
TestConfigReadSafety_ValidUpdatePreservesFields
TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable
TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified
TestDurableCompletion_I_LaterSuccessfulWriteRecords
TestDurableCompletion_I_LaterFailedWriteDoesNotRecord
TestDurableCompletion_I_RestartWithoutRecoveryPublication
TestDurableCompletion_J_CombinedFailureStaysObservable
TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch
TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded
TestDurableCompletion_A_FinalFlushFailureIsReported
TestDurableCompletion_A_FailedFlushJobIsNotCompleted
TestDurableCompletion_B_BothCausesSurvive
TestDurableCompletion_C_UnrecordedTerminalStateIsQualified
TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch
TestDurableCompletion_D_FlushFailureSurfacesWithAnotherBatchOpen
TestDurableCompletion_E_SuccessfulJobIsRecorded
TestDurableCompletion_F_UnrecordableJobStartsNoWork
TestDurableCompletion_G_BatchDepthBookkeeping
TestDurableCompletion_SaveJobsReportsFailure
TestRecoveryKitCarriesEscrow
TestKeystoreValidation_ReadBoundaryNoMutation
TestKeystoreValidation_ConflictMutationBoundary
TestKeystoreValidation_MissingMutationBoundary
TestKeystoreValidation_PublicationBoundary
TestKeystoreValidation_ParticipantSnapshot
TestKeystoreValidation_LookupIgnoresUnrelatedConflict
TestKeystoreValidation_ExactSecretsAndLocalDuplicates
TestKeystoreValidation_FirstBuildInitializationPrecheck
TestKeystoreValidation_UnionAndReopen
TestKeystoreValidation_InvalidFinalParticipant
TestKeystoreValidation_MissingDoesNotProvision
TestKeystoreValidation_ConflictBothOrders
TestKeystoreValidation_IdenticalAndMetadata
TestKeystoreValidation_MetadataConflictRefused
TestKeystoreValidation_OfflineRecovery
TestKeystoreValidation_PublicationFailureAPI
TestKeystoreValidation_FirstUseCompatibility
TestRecoveryKitWritesKeyPages
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
safety (all PASS):
```
TestExportAppBackup_FailedPublishPreservesPreviousBundle
TestPersistObserver_SeesRealMutations
TestPersistObserver_SeesBackupPrune
TestOpenStore_UnreadableCatalogFailsClosed
TestOpenStore_ZeroLengthCatalogIsDamagedNotNew
TestOpenStore_InvalidJSONStillFailsWithDamagedMessage
TestOpenStore_NewerSchemaStillOpensReadOnly
TestOpenStore_FreshDirectoryStillInitializes
TestOpenStore_NotExistShapesAllInitialize
TestOpenStore_ExistingCatalogReopensIntact
TestJobsUI_HonoursRecordingState
TestJobsUI_ExecutionOutcomeTakesPrecedence
TestWriteCatalog_SaveFailurePropagates
TestOpenStore_RecoverySaveFailureIsNonFatal
TestCatalogCurrentRoundTrip
TestCatalogMigrateV1ToV2
TestCatalogLegacyMigratesAndBacksUp
TestCatalogNewerSchemaIsReadOnly
```

Final preservation verification: PASS. Every pre-existing inventoried source, test, dependency, documentation and historical-report file has the same SHA-256 as at review start. Only this new review report was added. Branch/full HEAD unchanged, index empty, no in-progress operation, and no reviewer server remains. Figma remains 219717 bytes / SHA-256 69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda. See preservation-final.json and status-before/status-after.txt in local evidence. No fixes, staging, commits, pushes, merges, installs or job-loader work performed. Stop after review.
