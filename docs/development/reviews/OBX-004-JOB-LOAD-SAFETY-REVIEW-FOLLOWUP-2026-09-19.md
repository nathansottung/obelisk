# OBX-004 job restore-member correction - 2026-09-19

READY_FOR_JOB_LOADING_FOCUSED_RECHECK

Author provenance: Codex in the existing shared implementation/review session. This is an authorized correction and author validation, not a targeted reviewer recheck, independent review, owner acceptance or publication. The substantive NEEDS_CHANGES review and original implementation report remain unchanged. Local execution date is September 19; UTC execution timestamps cross September 20.

## Checkpoint and reviewed baseline

Repository: `C:\Users\nsott\source\repos\obelisk`. Existing branch `fix/obx-004-job-load-safety`; full HEAD and overall patch base `da22f1d9895d5350142a6f9b05ac41f0a920e170`. Index empty; no active Git operation. No branch creation, remote lookup or configuration publication replay. No applicable AGENTS.md was found in the repository/ancestor chain. The actual reports, candidate helpers/tests and relevant living records/restore contracts were inspected; prior shared-session source inspection was reused where byte identities matched.

Controlling finding: **R1 / P1: restore validates a spelling, but publishes a destination**, in [the substantive review](OBX-004-JOB-LOAD-SAFETY-REVIEW-2026-09-19.md). The new exact `members["jobs.json"]` validation disagreed with extraction's path resolution. `./jobs.json` and native Windows `Jobs.json` could replace good history, then fail only at reopen. Returning that late error and latching later writes did not protect the original file.

Evidence root: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-followup-20260919-222503`. `review-comparison.json` compares actual starting bytes with the review's `initial-identities.json`: all 235 reviewed files match; the sole addition is the substantive review report. `pre-identities.json` inventories the actual 236-file starting checkout, including untracked candidate source/tests and Figma. Manifest SHA-256: `3f4d27edad5d7b1c2fd13372a441f37f71300152c2017a611b9da8865d3330a3`.

`before/` retains those exact working bytes, not a reconstruction from the published parent. It excludes .git, caches, credentials and live data. The only extra source in that external copy is the retained reviewer probe with explicitly described capture adaptations. `pre-diff.txt`, `status-before.txt`, `checkpoint.json` and `submitted-request.txt` retain the starting diff, state and actual prompt. No completed prior correction report existed.

## Correction and compatibility

Only appbackup.go changes production behavior in this correction:

- `validAppBackupMemberName` at :276 permits literal MANIFEST.json, catalog.json, config.json, jobs.json and formats.json, plus flat `keystores/<filename>` members. It does not normalize names or choose a winner. Keystore leaves exclude empty/dot/parent names, slash, backslash, colon and trailing dot/space spellings. Case inside legitimate keystore leaves is retained; `keystores/jobs.json` is not the root job board.
- `readTarMembers` at :290 rejects unsupported regular-member names and exact duplicate tar records before inserting into its map. Records can no longer silently replace earlier payloads during reading.
- `verifyAppBackup` at :327 applies the same name policy to manifest entries, refuses manifest self-publication and duplicate manifest entries, and preserves existing size/hash, format, schema and optional whole-tar integrity checks. The single canonical jobs payload is still decoded before RestoreAppBackup can mutate state. Exact validated map bytes are the bytes selected for extraction; there is no second alias interpretation.
- `gatherMembers` at :143 checks exported keystore names against the same policy before any export publication, so export does not silently produce a bundle this importer rejects because of such a filename. The error requests renaming the nonportable leaf before export.

Current exporter output uses the literal state names and flat keystore entries; the supported legacy format marker uses the same names in its compatibility fixture. Canonical current/legacy bundles, optional jobs omission, keys and valid export/restore/migration continue to work. Arbitrary archive paths and noncanonical state aliases previously accepted by generic extraction are now deliberately refused, including otherwise-valid alias payloads. Flat keystore filenames containing the excluded separator/stream/trailing characters are also deliberately refused, including imports created on a system that allowed those characters. This restriction prevents parent/path aliases through the keystore branch; it is not a claim that every platform filename is now portable.

All accepted root state destinations now have exactly one permitted spelling. A second canonical record or manifest entry is rejected, and every alias competitor is rejected regardless of order. No unrelated nested file is equated with jobs.json merely by basename. Configuration keeps its checked, last-publication path; catalog and keystore dispatch are unchanged after preflight. No decoder, refusal-latch, startup, migration, completion-lock or UI behavior was modified.

This is a naming/payload preflight policy, not a general symlink, filesystem-alias, short-name, hostile-concurrency or archive-security certification. Case collisions between distinct keystore leaves and broader restore transactions remain outside this correction. No new schema, automatic recovery or rollback was added.

## Decisive regression evidence

Before production edits, the retained reviewer TestReviewerJob_RestoreDestinationAliases ran against before/: native exit 1, one failing top-level test, separately one passing canonical subtest and two failing alias subtests. Both reported clobbers reproduced. This is the exact reviewed uncommitted implementation, not parent da22f1d... and not a mutated publisher.

A second red execution added only external harness capture of tar fixtures and before/after bytes; its assertions and production remained unchanged. Its output filenames `jobs.json` and `Jobs.json` themselves collided on this Windows filesystem, so those initial captures do not independently represent both cases. They and all logs remain preserved. A third bounded red execution used distinct labels in `cli/red-fixtures-v2/` (canonical, dot-alias, case-alias), retaining each actual tar and authoritative before/after bytes. All three red executions produced the same expected failures, with no compile/setup failures. They are repetitions, not additional distinct defect counts. `red-capture-v1-probe.go.txt` preserves the first capture harness; before/reviewer_job_probe_test.go retains the corrected capture harness. Original reviewer probe bytes remain in the prior review evidence and green/.

The native fixture observation is specific: the canonical jobs.json bytes survive; `./jobs.json` and `Jobs.json` each replace them with `{"rows":[null]}` before reopen reports null-row refusal. This establishes actual behavior on this machine's fixture filesystem, not a universal claim about all Windows/Unix filesystems.

New permanent tests in `job_restore_members_test.go`:

- :18 orderedJobRestoreBundle writes and rereads actual ordered tar records, including duplicate records. Its manifest binds each unique name to the final payload, matching the old map reader's integrity interpretation, rather than accidentally rejecting a duplicate solely because fixture hashes were wrong. Alias records retain their individual correct hashes/sizes.
- :74 TestJobRestoreMembers_RefusalBeforePublication covers canonical invalid jobs, invalid and valid alias payloads, slash/backslash/dot/stream/trailing and keystore-parent spellings, exact duplicate and canonical-plus-alias collisions in both orders, and duplicate manifest entries. Distinguishable valid incoming ID 70 and invalid payloads are used over recognizable original history.
- :150 TestJobRestoreMembers_PreservesExistingRefusal proves an incoming rejection does not clear a separately latched failure of the original store or permit stale publication.
- :179 TestJobRestoreMembers_NestedKeystoreIsNotJobBoard proves the supported nested keystore member is dispatched separately and is not job-decoded by basename.

Refusal tests call both read-only InspectAppBackup and real RestoreAppBackup, require the identical useful preflight error, compare the entire disposable data-directory inventory and in-memory snapshot/counter, and require the same Store. No pre-restore backup directory appears. Source creates that directory before extraction and never removes it as rollback; together with the identical read-only error, this establishes rejection before publication rather than replace-and-restore. Subsequent legitimate NewJob and reopen succeed on the originally valid store and never persist rejected incoming ID 70. The original-store latch test separately requires continuing refusal.

The retained three reviewer probes were then rerun as **author checks** against green/, a byte-preserved corrected source copy: all three top-level tests and three subtests passed. These cover the exact reported names, completion while reload holds jobs.mu, successful later reload/write recovery, and valid restore/migration with source preservation and identity continuation. They are not the pending targeted reviewer execution.

## Current author executions

Verified Go 1.26.8 windows/amd64. Reused approved process-scoped GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off and existing module cache. GOCACHE, TEMP/TMP, synthetic GNUPGHOME, build outputs and CLI evidence are under this task's evidence root. No GOTMPDIR override, installations, dependency upgrades, global settings, WSL or security-control changes.

Selections were enumerated before execution. Exact commands/settings/native exits/timestamps are in `*-execution.json`, runner.py/cross-runner.py and logs/. Raw JSON, exact test/subtest outcomes and skip output are retained separately. Runner shell success is not substituted for native child exit.

| Current execution | Native exit | Top-level pass / fail / skip | Subtest pass / fail / skip |
|---|---:|---|---|
| Exact reviewed red; each of three capture stages | 1 | 0 / 1 / 0 | 1 / 2 / 0 |
| Focused correction selection | 0 | 71 / 0 / 0 | 112 / 0 / 0 |
| Final full native suite | 0 | 257 / 0 / 39 | 152 / 0 / 0 |
| Retained probes on corrected copy, author rerun | 0 | 3 / 0 / 0 | 3 / 0 / 0 |

Focused command: `go test -count=1 -json -timeout 3m -run '^(TestJobRestoreMembers|TestJobLoad|TestAppBackup|TestRestoreAppBackup|TestRenameCompat|TestConfig|TestAtomicRename|TestDurableCompletion|TestJobsUI)' .`. It preceded four extra subcases for two keystore-parent spellings. Final full command: `go test -count=1 -json -timeout 5m -skip '_SystemDisk$' ./...`; one run, with all final source/tests. TestSmartDeviceNode_SystemDisk, TestVolumeHealth_SystemDisk and TestDeviceIdentityAndLabel_SystemDisk were enumerated and excluded, not executed or counted among emitted skips. The 39 skips retain missing-helper, native permission-fixture and opt-in scale/performance limitations.

Native build and vet passed; Linux/amd64 and macOS/arm64 build/vet passed with explicit process-local target overrides (compilation/static analysis only). Selected-toolchain gofmt was applied only to appbackup.go and the new test; subsequent read-only gofmt output is empty. git diff --check passed, with existing line-ending conversion notices in living records. No product source changed after the full run; documentation closeout followed.

Job/config CLI matrices used explicit disposable -data paths, ephemeral loopback addresses, synthetic auth, bounded lifetimes and stop/wait. Natural refusal exits remain distinct from forced Windows serving-process stops. Logs/PIDs/arguments/snapshots remain under cli/. Final process verification and summary are external; no recorded CLI child remains.

Evidence categories are separate: original author full suite 254 top-level passes / 39 skips plus 122 passing subtests; substantive reviewer focused 90 top-level passes / no skips plus 122 passing subtests, with its separate probes (2 top passes / 1 failure; 1 subtest pass / 2 failures); current correction counts above. Do not sum overlaps. No next targeted reviewer execution has occurred.

## Identities, preservation and next gate

Correction-only changes: appbackup.go; new job_restore_members_test.go; four living records CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md, REVIEW_COVERAGE.csv; this new follow-up report. The existing overall job candidate additionally contains its earlier store.go, migrate.go, job_load.go and three regression files, all unchanged by this correction.

| Source/test | Pre-correction SHA-256 | Final SHA-256 |
|---|---|---|
| appbackup.go | 2645bd55e3d058b72973aa6da1030719b685f990387ea3a658b2517abd930e0e | b1b6cf63645276cd4068dbd20cf85e7d6bb283b445b4871d198d7b3e96b85d3e |
| job_restore_members_test.go | absent | fae4b99e1ca0137baf29f13f8d6bdb2ad4a499e08746cfdde4583c82a4ab41f7 |

`final-identities.json` and `candidate/` retain all final working files, including this finished report and new tests. The manifest is external, so the report does not contain its own hash. `preservation-final.json` records exact authorized differences and verifies all other pre-existing files. `correction.diff` compares the retained pre-correction appbackup.go with current bytes; the whole candidate's explicit-base diff and status are retained separately. Original implementation/review/configuration reports, dependencies/build settings, UI and Figma remain unchanged. Figma remains untracked, 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda`. Branch/full HEAD unchanged; index empty; no active Git operation. Nothing staged, committed, pushed or merged.

Missing GPG/PAR2 still limits native integration. No race detector, effective ACL, Docker/CI, other-platform runtime, browser, power-loss or physical-media qualification. The prior Windows catalog sharing interleaving remains outside the correction. Restore remains sequential across files; naming refusal tests do not establish general transactional safety. Product direction remains generation-independent preservation, with LTO-8 first for physical qualification rather than a generation limit; other tape backends/generations and Blu-ray require separate qualification. Figma GUI work remains separate.

ONE next action: perform the targeted restore-member bypass recheck on this exact candidate: reported aliases, duplicate/collision order, actual validated-payload publication, original-store/latch preservation, canonical current/legacy export/restore, valid migration and affected safety controls. R1 is addressed by the author and pending that recheck, not declared CLOSED. Stop here; no automatic reviewer or publication task.
