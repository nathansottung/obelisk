# OBX-004 job restore-member focused recheck - 2026-09-19

READY_FOR_OWNER_REVIEW

**R1 / P1: restore validates a spelling, but publishes a destination — CLOSED.** No blocker remains within this correction and its directly affected contracts. This closure supplements the earlier substantive review's passing scope; it is not owner acceptance, publication, repository-wide safety or transactional restore certification.

Provenance: Codex, fresh source inspection and executions in the same shared session that authored the correction and earlier review. No independent agent, isolated context/filesystem or human certification is claimed. Tests used the repository and a byte-preserved external copy as identified below. REVIEW ONLY: no fixes or repository test changes.

## Identity and scope

Repository: `C:\Users\nsott\source\repos\obelisk`. Branch: `fix/obx-004-job-load-safety`. Full HEAD and overall job-loading base: `da22f1d9895d5350142a6f9b05ac41f0a920e170`. This HEAD does not include the uncommitted candidate. Configuration publication was not replayed; no network lookup occurred.

Evidence root: `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-recheck-20260919-224651`. `initial-identities.json` records all **238** pre-existing tracked/untracked working files, including the new job_restore_members_test.go, all other job source/tests, dependencies, build inputs, living records, historical reports and Figma. Manifest SHA-256: `eb62641e7ce3fa598070c4b4122ca1607ee43d61d366304b157f3ff6fb7913c3`. Comparison with the correction author's `obx004-job-followup-20260919-222503/final-identities.json` found **zero changed, missing or additional files**. See author-comparison.json.

The author's retained before/ files were checked against all 236 entries in its pre-identities.json and match. This is the reviewed uncommitted pre-correction source, not the Git parent. Correction delta: appbackup.go; new job_restore_members_test.go; four living records; follow-up report. Other source/tests and historical reports are unchanged from that snapshot. `delta-identities.json` records the actual changes; `correction.diff` retains the production correction; `overall.txt` retains the full tracked diff against da22f1d...; candidate/ retains all starting working bytes, including untracked files. No completed job restore-member focused recheck existed at start.

No AGENTS.md was found in the repository or ancestors. Applicable contribution guidance and the three reports, current handoff/next-actions/status/coverage entries, restore code and tests were read; unchanged shared-session inspection was reused where identities matched. Index was empty, with no active Git operation. Local execution date is September 19; UTC timestamps cross September 20.

## Finding disposition and publication trace

The correction consistently chooses **rejection**, not normalization or alias support:

- appbackup.go:276 permits literal state-file names and flat keystores/<filename> regular members. Root job aliases are not accepted. Keystore leaves exclude separator, colon, dot/parent and trailing dot/space forms; a nested keystores/jobs.json remains a distinct supported destination. The exporter checks included keystore names at :143, before export publication. Deliberately excluded nonportable leaf spellings are documented in the follow-up; this is not universal filename portability.
- readTarMembers at :290 consumes the complete archive before returning. Unsupported regular names and exact duplicate regular records refuse before map insertion. No first/last payload winner is selected. Non-regular tar members are not entered in the map and are never extracted; a manifest reference with no matching regular member fails as missing.
- verifyAppBackup at :327 checks format/schema, the existing optional whole-tar checksum and manifest size/hashes. At :367 it also rejects invalid/self-publishing/duplicate manifest names. At :389 it validates the canonical jobs bytes. Validation uses the same exact map entry selected later by the manifest name, without a second name normalization or reread from an external file.
- RestoreAppBackup at :408 returns a verification error immediately. Its first directory mutation is the pre-restore backup creation at :452, after verification and config preparation. Extraction uses the verified map bytes at :484; :472 is the ordinary member rename publication boundary. Config still follows its checked final publication path, and runtime Store adoption remains after successful reopen at :511. No protected-state alias can take a different dispatch path under the accepted naming policy.

Thus a late invalid name or duplicate cannot be discovered only after an earlier member has published. Rejected incoming archives do not read-fail the original job board, reset its latch, adopt incoming rows or disable otherwise valid later writes. Conversely, a pre-existing refused-load latch remains set when an incoming alias is rejected.

| Contract | Focused disposition and evidence |
|---|---|
| Reported aliases | PASS: ./jobs.json and Jobs.json, with invalid and otherwise-valid payloads, refuse during member preflight. Original bytes, full fixture inventory, runtime Store, rows and counter are preserved. |
| Canonical validation and compatibility | PASS: invalid canonical jobs remains refused; canonical exporter round-trip, included keys, legacy format and valid migration controls pass. The external valid restore/migration probe confirms adopted rows, historical error semantics and identity continuation. |
| Duplicate/collision ordering | PASS: actual repeated tar records, canonical-plus-alias and reversed valid/invalid orders refuse. Additional reviewer probes establish rejection for two distinct valid job payloads in both orders, after a publishable formats member. Duplicate manifest entries refuse too. |
| Publication ordering and cleanup | PASS for these preflight refusals. InspectAppBackup and RestoreAppBackup yield the same refusal; recursive inventories show no pre-restore directory or staging artifact. Production creates that directory before extraction and contains no rollback that removes it. This source ordering plus native observations establishes prevention, not transient replacement followed by restoration. |
| Ordinary saves and existing latch | PASS: legitimate NewJob/reopen after incoming refusal persists original/new jobs, never rejected IDs. Existing failed-load state is retained. Reload/recovery, completion-lock and FAILED/current-recording/history controls pass. |
| Startup/shared dispatch | PASS in the affected scope. Actual job CLI reaches the job gate using valid prerequisites. Configuration checked-publication/restore tests and atomic replacement controls pass; their accepted implementations are not reopened as separate audits. |

No new source-only blocker or optional hardening demand is assigned. Broader namespace/security, concurrent writers, symlinks, filesystem aliases, keystore case collisions and multi-file transaction concerns remain explicitly outside this closure. Later I/O failures can still leave partial restore effects; these preflight results do not promise all-store rollback.

## Executed evidence

The permanent regression was read in full. job_restore_members_test.go:18 writes and re-reads ordered tar records, including duplicates; its manifest describes the last payload for an exact repeated name, matching the former reader's integrity interpretation. It does not silently deduplicate the tar. At :74 its cases exercise the real restore entry point with valid archive/hash prerequisites, invalid/valid aliases, canonical invalid jobs, order variants and duplicate manifest entries. At :150 it checks pre-existing refusal, and at :179 it distinguishes a legitimate keystore basename. Recognizable existing history and full snapshots precede every attempted restore.

Additional probes exist only in external candidate/:

- reviewer_job_probe_test.go was copied unchanged from the substantive review, after checking SHA-256 `296d76af63f5edbf7c4924082e9ea2b5266307e4bbba2a91576ba6e38a7c5e8c`. It reruns reported aliases, reload/completion mutex ordering with successful recovery, and valid restore/migration.
- reviewer_member_probe_test.go, SHA-256 `198fb4ac816d5f756e5abaed85c2dd3b90b42e8f25690d543c206daec807d753`, adds two-valid-record competitors (six order/name cases), symlink/hardlink/directory tar members referenced by a manifest (three cases), and a native SameFile observation. Link records are tar fixtures only; no filesystem links or new mounts were created.

All 238 original files in candidate/ still match the starting manifest; only those two probe files were added there. Native observation: this disposable Windows filesystem resolves jobs.json and Jobs.json to the same file. Portable preflight refusal does not depend on that behavior. No claim is made about another volume or Unix runtime.

Author historical red artifacts remain in the follow-up evidence: exact pre-correction source, red-fixtures-v2 tar files and before/after bytes, plus logs. Their reported canonical preservation and both alias clobbers agree with the retained summary. The first capture's case-colliding evidence filenames and corrected capture are disclosed in the follow-up. No historical red replay was executed in this focused review; it is not relabeled as current execution or a Git-parent replay.

Process-selected Go **1.26.8 windows/amd64** was verified. GOTOOLCHAIN=go1.26.8, GOWORK=off, CGO_ENABLED=0, GOFLAGS=-mod=readonly, GOPROXY=off; existing approved module cache, fresh GOCACHE/TEMP/TMP and synthetic GNUPGHOME under the evidence root. No GOTMPDIR override, installations, global changes, WSL or security-control bypass.

Both selections were enumerated before execution. The main selector was:

```text
^(TestJobRestoreMembers|TestJobLoad|TestAppBackup|TestRestoreAppBackup|TestAtomicRename|TestRenameCompat_(MigrateCopiesVerifiesAndSwitches|AppBackupLegacyFormat)$|TestConfigBoundary_(CheckedPublication|RestoreUsesCheckedPublisher)$|TestDurableCompletion_(H_NoUnqualifiedCompletedIsObservable|K_FailedJobCanAlsoBeUnrecorded|I_LaterSuccessfulWriteRecords)$|TestJobsUI)
```

Executed `go test -count=1 -json -timeout 3m -run <selector> .` in the repository; external probe command was `go test -count=1 -json -timeout 2m -run '^TestReviewer(Job|Member)_' .`.

| This focused reviewer | Native exit | Top-level pass / fail / skip | Subtest pass / fail / skip |
|---|---:|---|---|
| Selected repository tests | 0 | 33 / 0 / 0 | 81 / 0 / 0 |
| External probes | 0 | 6 / 0 / 0 | 12 / 0 / 0 |

`target-outcomes.json`, `probe-outcomes.json`, enumeration logs and raw JSON retain exact names/results. `*-execution.json` and runner.py retain commands, cwd, overrides, timestamps and native child exits. No setup failures or retries. Native build to evidence/build/obelisk.exe, go vet ./..., selected-toolchain read-only gofmt -l of the two correction files and git diff --check all exited 0. Formatting output was empty; Git emitted only existing line-ending conversion notices. No full suite or redundant cross-build was run now.

Seven CLI records are retained under cli/: four natural exit-1 job refusals with unchanged seeded fixtures, plus three HTTP-200 serving controls explicitly stopped and waited. Explicit disposable -data, ephemeral loopback address, synthetic token, bounded lifetime and native exit/forced-stop distinction are established by the unchanged regression. Final recorded CLI process count: zero.

Evidence attribution remains separate: original author 254 top-level passes / 39 skips plus 122 subtests; substantive reviewer 90 passes / no skips plus 122 subtests, with separate probes (2 top passes / 1 fail; 1 subtest pass / 2 failures); correction author 257 passes / 39 skips plus 152 subtests and reported build/vet/cross checks. None is this focused execution; overlapping selections and top/subtests are not summed.

## Preservation, limits and next action

All 238 pre-existing inventoried repository files remain byte-identical, including source/tests, dependencies, four living records and historical reports. Figma remains untracked, 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda`. Only this new report is added. Branch/full HEAD unchanged, index empty, no active Git operation; see preservation-final.json and status-after.txt in evidence. No fixes, staging, commits, fetch/pull, push, merge or branch changes.

Missing-helper integration limits remain. No race-detector, ACL, Docker/CI, browser, other-platform runtime, power-loss or hardware qualification follows from these checks. Accepted ordinary job semantics stand: FAILED stays primary, recording qualification differs from history, and completion is not file verification. Configuration publication remains complete. GUI/Figma, generation-independent buffering, physical media and separate Blu-ray work were not undertaken; LTO-8 remains the first physical qualification target, not a generation limit.

ONE next action: **OWNER ACCEPTANCE AND SCOPED JOB-LOADING PUBLICATION**, as a separately authorized task. This focused closure requires no additional review round. Implementation/evidence commits and destination-specific push authorization belong to that later task and were not executed here.
