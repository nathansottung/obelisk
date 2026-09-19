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
