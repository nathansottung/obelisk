# Codex handoff - 2026-09-19

OB-006 OWNER-ACCEPTED CHECKPOINT

Implementation checkpoint: d147a823262757065d7817a233c0327523916e2a (owner accepted; separate evidence commit follows)
Branch: fix/ob-006-keystore-validation
Root: C:\Users\nsott\source\repos\obelisk

PR-04 candidate not yet transferred. Figma design context pending. Repository-wide review not completed. Neither missing item blocks this baseline.

Execution directory: C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9
Manifest: C:\Users\nsott\AppData\Local\ObeliskDev\baseline-20260919-125535-898459a9\execution-manifest.json

Baseline checks and final summary complete. The bounded persistence source review is now complete; see the current review entry below.

Report: [NEW_DESKTOP_BASELINE-2026-09-19.md](NEW_DESKTOP_BASELINE-2026-09-19.md).

Read the manifest before resuming; check the process identity before duplicating any RUNNING command. Repository-wide review, REVIEW_COVERAGE.csv and FEATURE_MATRIX.csv remain next-stage work. No substantive reviewer reports are recoverable from the earlier aborted delegation. Preserve accepted fixes and residual scopes in OB_STATUS.md. Only authorized current documentation and local execution outputs may change; no source/UI/test/dependency changes, helpers, commits, pushes, merges or hardware operations.

Results: build/vet/format exit 0; no formatting differences. Uncached tests exit 0: 209 top-level pass, 0 fail, 39 skip; separately 10 subtests pass. Three system-disk probes excluded. 36 skips require GPG/PAR2; Unicode compatibility remains unvalidated. Windows race, CI, Docker and hardware not tested. Full names/reasons are in the report; raw output and native results are in the execution directory above.

## Current review - 2026-09-19

BOUNDED PERSISTENCE PASS COMPLETE at 0dc7d5399c6889e015aebc9ba02e694a909d6e9c on setup/windows-nsott. Report: [PERSISTENCE_REVIEW-2026-09-19.md](PERSISTENCE_REVIEW-2026-09-19.md). Ledger: [REVIEW_COVERAGE.csv](REVIEW_COVERAGE.csv).

208 tracked files inventoried, plus 3 local artifacts. Four entire files source-reviewed, 13 partial, 144 inventoried only, 47 excluded with reasons. Large store.go/pipeline.go/main.go remain PARTIAL. No repository-wide review or feature matrix completed.

One bounded disposable probe run: 13 top-level pass / 0 fail / 0 skip, separately 5 passing subtests; native exit 0. These observations reproduce unresolved failure modes, not repaired behavior. Evidence: C:\Users\nsott\AppData\Local\ObeliskDev\persistence-20260919-131458-90213423 (source/probe source retained; probe-execution.json; logs/probes.jsonl). No full baseline rerun and no repository tests/source changed.

OBX-004 config/jobs and OB-006 key reconciliation confirmed. OB-011 permission policy is source-observed; Windows ACL behavior not validated. PR-01/02/03 accepted scopes stand, with documented residuals. Catalog semantic-shape/counter observations await broader-register reconciliation; no speculative new OBX ID. PR-04 remains unavailable and Figma context pending.

ONE next action: in a separately authorized task, prepare the bounded OB-006 validation-before-write repair with conflict/refused-participant regressions. Rejected participants are currently overwritten by sync, threatening otherwise recoverable encrypted media. Keep prior-generation/partial-publication residuals explicit. Do not restart baseline or claim the entire source was audited.

## Current implementation - 2026-09-19

READY_FOR_REVIEW: bounded OB-006 validation/conflict candidate on fix/ob-006-keystore-validation, parent 0dc7d5399c6889e015aebc9ba02e694a909d6e9c. See [implementation report](reviews/OB-006-KEYSTORE-VALIDATION-IMPLEMENTATION-2026-09-19.md) for exact candidate hashes, caller map, red/green copies and logs.

Strict sync validates every existing participant before any keystore mutation. Status rejects missing/invalid/conflicting replicas; lookup permits offline recovery but rejects observed requested-key secret conflicts. First-use GenerateKey and encrypted-build precheck are preserved. Metadata field union is deterministic; differing same-field values refuse sync. No UI or endpoint change.

Final build/vet/format pass. Targeted: 20 top-level and 30 subtest passes. Final uncached suite: 226 top-level pass / 0 fail / 39 skip, separately 40 subtest passes; three hardware probes excluded. Earlier suite had one unchanged reopen sharing failure, reproduced on parent and candidate (each 19/20 passes); evidence retained. No independent review is claimed. New code/test rows are CANDIDATE_PENDING_REVIEW, not silently covered by the persistence audit.

Open: previous generations, transactional publication, GenerateKey partial writes/catalog metadata, ACL, config/jobs, cross-process/identity safety, unavailable PR-04 and Figma. ONE next action: independent review of this candidate. No stage/commit/push/merge, new fix or GUI work performed.


## 2026-09-19 - Owner acceptance and OB-006 checkpoint

Owner accepted the bounded candidate after [the detached-review execution](reviews/OB-006-KEYSTORE-VALIDATION-DETACHED-REVIEW-2026-09-19.md). Implementation commit: d147a823262757065d7817a233c0327523916e2a on fix/ob-006-keystore-validation, based on 0dc7d5399c6889e015aebc9ba02e694a909d6e9c. Reviewed source/test bytes are unchanged. Refs OB-006; the whole issue remains partially repaired.

Accepted scope: synchronization validates every participant before keystore mutation; observed same-reference secret conflicts are rejected independently of order; status and lookup distinguish replica consistency from recovery availability; legitimate first-use initialization remains supported; partial-publication errors reach callers accurately.

Residual scope: multi-store atomicity, concurrent writers, retained generations, GenerateKey/catalog coordination, and broader ACL/key-security validation. Configuration and job-state loading remain OBX-004 work; neither was implemented in this publication.

Evidence remains distinct: the implementation report's final full run was 226 top-level passes / 0 failures / 39 skips, plus 40 passing subtests. The detached review separately ran 20 targeted tests and 39 prior-safety tests, plus 40 subtests; build/vet/format passed and seven expected parent-version regression failures reproduced. These overlapping selections are not a new full-suite result. Missing GPG/PAR2 limits integration coverage; Windows race, broader ACL, crash-durability and hardware evidence remain unestablished. Publication checks inspect Git scope, hashes and whitespace; no suite rerun or new review.

Historical baseline, persistence, implementation and both review reports are preserved unchanged. The baseline and persistence review are included because these reports directly reference them and their contents were inspected for publication. Raw test logs, manifests, disposable red/green copies and scratch probes referenced under AppData remain LOCAL ONLY and are not included in this commit.

The separate evidence commit containing this acceptance note is the intended base for the next configuration-repair branch. Use its full SHA after publication, not the old 0dc7d539 checkpoint. No configuration repair, merge or release is authorized during this publication.


## 2026-09-19 - OBX-004 configuration candidate ready for separate review

READY_FOR_REVIEW on fix/obx-004-config-read-safety, exact parent 26918c5b8ed01ea301fc9a4c7658e19054a73632 (published OB-006 evidence checkpoint; accepted implementation d147a823262757065d7817a233c0327523916e2a). No commits, index changes or publication in this slice. See [implementation report](reviews/OBX-004-CONFIG-READ-SAFETY-IMPLEMENTATION-2026-09-19.md) for exact file identities, caller map, commands and local raw evidence.

Existing configuration reads/updates now fail closed for missing/unreadable/damaged input. Deliberate first use requires startup -init-config; existing valid optional fields retain defaults and unknown extension fields survive settings updates. Updates use checked staged publication; errors distinguish before-replacement failure from published-but-directory-durability-unconfirmed. Startup/auth, settings/setup, jobs/background, keystore and helper callers propagate errors or use a validated snapshot. This is a new implementation candidate, not a separately reviewed fix.

Final current-source evidence: targeted 39 top-level passes and 62 passing subtests; uncached suite 240 top-level passes / 0 failures / 39 skips, plus 72 passing subtests. Go 1.26.8 Windows/amd64 build, vet and formatting passed. Earlier safety run hit the known Windows catalog-reopen sharing failure; exact accepted-parent reproduction 19/20 passes, current-candidate follow-up 20/20. Retained failures were not removed. Final suite passes do not establish the absence of that pre-existing timing issue. Missing GPG/PAR2 account for 36 skips; permission and opt-in scale/performance checks account for three more. Three system-disk probes excluded. Race/ACL/hardware/crash durability remain unestablished. Raw logs/candidate copies under AppData/Local/ObeliskDev/obx004-config-20260919 are LOCAL ONLY.

Config-only OBX-004 scope is ready; loadJobs and its null-row panic are unchanged and remain the next separately authorized sub-scope. Multi-process exclusion, cross-file/setup/restore atomicity, retained generations, PR-04/source reconstruction and broader ACL work remain outside this patch. OB-006 storage-validation source is unchanged; its fixture explicitly initializes config and all 17 bounded keystore tests pass in the final suite. Existing Figma bytes and historical reports are preserved. Repository-wide review and the feature matrix are not thereby completed.

ONE next action: separate review of this uncommitted configuration candidate and its retained evidence. Do not start job-loading work as part of that review.


## 2026-09-19 - R1/R2 correction ready for focused recheck

On fix/obx-004-config-read-safety, exact HEAD/base 26918c5b8ed01ea301fc9a4c7658e19054a73632. All 224 reviewed candidate hashes matched before edits; full pre-edit copies retained. [Follow-up report](reviews/OBX-004-CONFIG-READ-SAFETY-REVIEW-FOLLOWUP-2026-09-19.md) maps both controlling-review blockers to changes, pre/post identities and actual verification. The controlling review and original implementation report remain unchanged historical evidence.

R1: explicit initialization now links the checked staging file into an absent destination; it cannot replace an arriving file, directory or link. Unsupported hard links refuse without fallback. Post-link cleanup/directory-sync failures preserve Published=true and the final entry. Ordinary updates retain replacement. Deterministic late-arrival tests fail on the pre-follow-up candidate and pass now, including real Windows symlink coverage. R2: README/handbook document explicit Docker and Compose bootstrap, loopback/no published ports, full arguments/token/same storage, stop-and-wait, then normal startup without the flag. Deployment file changes are comments only. Defaults alongside catalog/key state do not recover lost settings.

Current executed evidence: target 52 top-level / 74 subtest passes; prior safety 43 / 10 passes; one uncached suite 244 top-level passes / 0 failures / 39 skips, with 84 passing subtests. Build/vet/format and Git Bash syntax checks pass. Actual-binary CLI lifecycle runs cover five natural refusals and four serving-then-forced-stop cases per execution, with exact exits retained. Docker runtime was unavailable and not executed. Missing GPG/PAR2, platform/opt-in skips, three excluded system-disk probes, Windows race/ACL/power-loss, other-platform runtime and CI limitations remain explicit. The previously recorded Windows sharing issue was not repaired; no such failure occurred in these runs.

READY_FOR_FOCUSED_RECHECK is an implementation status, not independent approval. ONE next action: focused recheck of R1/R2 and their retained evidence. No staging, commits, pushes, merges, branch changes or job-loading repair. loadJobs/null-row work remains a separate later sub-scope; broader concurrency, storage identity, malicious same-principal interference, cross-file transactions, PR-04 and Figma access remain deferred. Figma bytes are preserved.


## 2026-09-19 - OBX-004 configuration publication authorized

The owner requested completion of configuration publication after the [OBX-004-CONFIG-READ-SAFETY-FOCUSED-RECHECK-2026-09-19.md](reviews/OBX-004-CONFIG-READ-SAFETY-FOCUSED-RECHECK-2026-09-19.md) closed R1 and R2 as READY_FOR_OWNER_REVIEW. Implementation commit: `3ea444ae528af7343818142c867a8f955f9252aa` on `fix/obx-004-config-read-safety`, parent `26918c5b8ed01ea301fc9a4c7658e19054a73632`. The reviewed source, tests, deployment files and first-use documentation were committed unchanged. Prior implementation/review reports are preserved as historical evidence. This is a bounded branch checkpoint, not a merge, release or repository-wide approval.

Accepted scope: failed existing configuration reads never become defaults; ordinary updates preserve validated settings and unknown fields through checked replacement; explicit initialization publishes complete staged bytes with a no-replace hard link, refuses late entries, and reports post-publication errors truthfully. Startup/callers propagate failure. Docker/Compose first use explicitly initializes and serves on persistent state, then stops and waits before ordinary startup without the initialization flag. Hard-link support is required on the configuration/application-state filesystem; it is not imposed on backup media.

Evidence remains distinct: the author reports 244 top-level passes / 39 skips and 84 passing subtests. The focused recheck executed 39 top-level passes and 48 subtest passes, plus a separate dangling-symlink probe with two passing subtests, and reproduced the old overwrite against preserved pre-follow-up source. Its build, vet, formatting and shell-syntax checks passed. These overlapping results are not a new full-suite count. Publication reverified candidate hashes, exact staged content and whitespace; no new tests or CI run are claimed. Raw AppData/Temp logs and disposable fixtures remain local only.

Residuals: job loading/loadJobs null rows, broader storage identity, concurrent updates, cross-file transactions, retained generations and ACL work remain open. Docker runtime, Windows race, power loss, other-platform runtime and hardware are unverified; missing GPG/PAR2 still limits native integration. The known catalog-reopen sharing issue is not repaired by this checkpoint. Figma is preserved and excluded from publication.

The separate evidence commit containing this note is the intended parent for later job-load work. Do not use the former base `26918c5b8ed01ea301fc9a4c7658e19054a73632` or the implementation commit as that parent. Publication is complete only when `publication-receipt.md` records a successful push and a live origin branch tip equal to the full evidence-commit SHA. The receipt is a local post-push artifact, kept outside the evidence commit to avoid a self-referential SHA. No job-load branch or implementation is created by this publication task.
