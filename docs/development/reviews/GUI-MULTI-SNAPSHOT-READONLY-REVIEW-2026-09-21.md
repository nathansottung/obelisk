# Two-snapshot Library/Find substantive review — 2026-09-21

**Verdict: NEEDS_CHANGES.** One consolidated P2 finding: distinct snapshots with the same basename have indistinguishable visible and accessible result labels and source summaries. Underlying snapshot-qualified lookup remained correct in the exercised cases. No correction was made during this review.

This is a same-session Codex review under the owner's submitted Prompt 23, with implementation context available. It is not a context-isolated review, human certification, package acceptance, or qualification of other workflows. Prompt 23 is a conversation label, not a repository-issued change identifier.

## Candidate and evidence identity

- Branch: `feat/gui-multi-snapshot-readonly`.
- Full HEAD and complete patch parent: `a099ddc7530d81a9f3206e426b81def5172b16ec`.
- Index empty; no operation markers at entry. The candidate is uncommitted working bytes, not HEAD alone.
- Author evidence root (`A`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-20260921-130125`.
- Reviewer evidence root (`R`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-review-20260921-140103`.
- `A/candidate-identities.json` SHA-256: `481deb4373f7480b2fc2fa14f59c04583fcac31200d53afe503a87ddf69caa78`. All 308 listed current files matched. This is an identity inventory, not 308 audited implementation files.
- `R/source` preserves those 308 actual candidate files before reviewer execution. Final rehash found no source-copy drift. Builds and runtime execution used this copy; it has no `.git` directory. Build-path/VCS differences from the author's binary are not asserted reproducible.
- `R/complete-tracked.diff` preserves the complete tracked diff against the explicit parent. New files are preserved in `R/source`; `initial-context.json` and `initial-identities.json` retain the original checkout state.

Complete candidate path set: 11 modified tracked files:

```text
docs/development/CODEX_HANDOFF.md
docs/development/NEXT_ACTIONS.md
docs/development/OB_STATUS.md
docs/development/REVIEW_COVERAGE.csv
gui_catalog.go
scripts/gui-preview/README.md
scripts/gui-preview/browser-check.mjs
scripts/gui-preview/catalog-adapter.mjs
scripts/gui-preview/catalog-protocol.mjs
scripts/gui-preview/catalog-ui.mjs
scripts/gui-preview/server.mjs
```

Four new candidate files:

```text
docs/development/reviews/GUI-MULTI-SNAPSHOT-READONLY-IMPLEMENTATION-2026-09-21.md
gui_multi_snapshot_test.go
scripts/gui-preview/multi-snapshot-browser-checks.mjs
scripts/gui-preview/multi-snapshot.test.mjs
```

Pre-existing untracked references `Untitled.pdf` and `docs/Obelisk.fig`, and ignored `docs/OBELISK_Readable_Design_References.zip`, are outside that candidate manifest. All three are included in the review's 311-file preservation baseline. They were not converted, changed, or used to assert new visual fidelity. No already-completed substantive review of these exact two-snapshot contents was found.

Read provenance: the submitted Prompt 23 is retained as `R/submitted-prompt.txt`; the retained Prompt 22 and implementation contract in A, the current implementation report, GUI README, relevant handoff/status/next-action/coverage and provenance records, prior catalog/filename/scope closure evidence, and the relevant external `scope-reconciliation-20260921-113337` mapping were used. The 32-workflow reconciliation was not repeated. No applicable repository AGENTS instructions were found. The complete changed runtime/test surface and directly affected reader/protocol/UI/lifecycle code were inspected.

## Finding R1 — P2: same-basename sources lose distinguishable presentation identity

Primary anchor: `scripts/gui-preview/catalog-ui.mjs:130`. Related basename-only labels occur in source summaries/Library controls around lines 52–56 and per-source result totals at line 128. `catalog-adapter.mjs:startCatalogSession` deliberately supplies `path.basename(catalogs[i])` as the label. The selector adds a digest prefix, and the inspector adds the full digest and handle, but result buttons do not.

Smallest exercised reproduction:

1. Launch the real two-reader server with fresh review-owned `review-inputs/A/snapshot.json` and `review-inputs/B/snapshot.json` and the fresh reviewer reader.
2. Both valid, nonidentical catalogs contain record `1`, path `shared.txt`, and the same collection display text. A records 1111 bytes and SHA-256 `a` repeated 64 times; B records 3333 bytes and SHA-256 `c` repeated 64 times. Their first-seen values also differ. Each contains three records and has historical UNKNOWN scope/time.
3. Open Find with All selected. There are correctly six entries, but the two record-1 buttons have exactly the same text and accessible name: `snapshot.json · shared.txt · Review <img src=x onerror=alert(1)> · Record 1`. The markup-shaped collection name is inert text.
4. The status says `snapshot.json: 3; snapshot.json: 3`; collapsed source summaries and Library `View snapshot.json` controls also lack a distinct source label.

Expected: a user can distinguish each origin from its visible/accessibility label, including results in All and per-source totals. Actual: distinct `data-snapshot` values preserve internal authority but are not visible or accessible source names. Clicking and inspecting the full digest can disambiguate afterward; this does not identify the origin of the result before selection. The same failure reproduces with startup order reversed.

Evidence: `R/review-expectations.json`, `review-independent-corrected-labels.json`, `review-independent-reverse-labels.json`, and the independent browser result logs. Screenshot: `R/browser-independent-corrected/same-basename-result-ambiguity.png`; reversed-order screenshot is under `browser-independent-reverse`. `qualified-inspector.png` shows the corresponding correctly bound evidence. Expectations were generated independently of the UI and checked against full hash, bytes, path, source-folder text, first-seen value and catalog digest.

This violates the selected contract's requirement that distinct same-basename files remain distinguishable and that every result identifies its origin. It is a provenance/usability correctness defect, not observed cross-catalog data mixing or a cosmetic theme objection.

Smallest requested correction: use a consistent, unique within-session presentation identifier in result names, per-source totals, summaries and Library control names. Keep existing server-bound handles as authority. A collision-safe short digest label or another unambiguous session display label is sufficient; durable UUIDs/history are unnecessary. Add a same-basename, overlapping-record regression including reversed startup order. **Do not treat this report as authorization to implement that correction.**

## Boundary dispositions

| Boundary | Disposition and decisive evidence |
| --- | --- |
| Snapshot-qualified identity and duplicates | Internal binding passes exercised controls; presentation needs R1 correction. `startCatalogSession`, `validateSnapshots`, `validateMatches` and UI inspector lookup preserve `{snapshot,id}`. Independent A/B fixtures exercise equal ID with different evidence, equal path/hash as separate entries, A-only ID absent from B, both filters, All and reversed input order. A simulated B-qualified A-only response is refused by UI validation; the normal native B query also returns no such record. Same input and byte-identical copy are refused by raw-byte digest, not pathname or semantic JSON equivalence. Overlapping records in distinct artifacts are accepted. No persistent identity/deduplication claim. |
| Exact native identities | Fresh Go-generated large-ID fixtures retain `9007199254740991`, `9007199254740992`, `9007199254740993` and `9223372036854775807` as distinct strings through native query, protocol, browser selection and inspector. Fixture creation does not first round them in JavaScript. Browser same-ID byte evidence differs correctly between sources. |
| Scope, time and counts | Pass within supported representations. Native/Node tests and browser pairings cover OFF, ON with counts, UNKNOWN, known zero, empty and all-excluded. Validated inventory audit time is projected by `gui_catalog.go` around lines 285–289; missing historical time stays unavailable. Offset preservation is checked in native/Node aged fixtures; independent inspector expectations also retain different offset timestamps. Load time is not represented as new inventory verification. All totals describe recorded entries; exclusions remain per source and unavailable contributions never become zero. R1 covers ambiguity of the per-source labels, not count arithmetic. |
| Startup/runtime failures and stale work | Pass exercised controls. Missing/malformed/unsupported and malformed-scope inputs, both startup positions, duplicate artifacts and extra arguments are refused without a successful partial/static fallback. Fault doubles exercise second-reader exit, malformed/unknown response, timeout, overflow and incomplete frames. All fails as a whole; documented failure latching also refuses later healthy-source queries. Fresh valid restart succeeds. UI epoch/request guards withstand deterministic late native-result delivery, late injected rejection, X→Y, All→A, A→B, navigation away and Back/Forward without replacing a newer selection. |
| Input, response and resource bounds | Pass inspected/probed bounds. Reader accepts 1..4 MiB regular-file input, at most 1000 file rows/copy occurrences, 100 rows per ancillary table and bounded strings/rows. Aggregate accepts exactly 1000 records or copy occurrences and refuses 1001 records/1002 occurrences in independent native boundary fixtures. 4 MiB+1 input is refused. Reader framing caps 16 MiB; session projection caps 32 MiB; aggregate has one in-flight request, no unbounded query queue. Five-second response deadlines and bounded forced cleanup are exercised using explicit doubles. The 32 MiB projection guard was inspected, not separately saturated by a large browser campaign. |
| Read-only and HTTP isolation | Pass bounded evidence. Reader opens the selected files read-only, validates and copies data into a read-only in-memory Store; queries do not reopen catalog or source paths. HTTP accepts only fixed assets and constrained query fields/selectors; probes reject unknown handles, arbitrary path/executable parameters, unsupported methods, unknown routes and foreign Host. Review-owned source file was exclusively locked before startup and both catalog files after adoption; queries and cached bootstrap still succeeded. A watcher observed no input-directory changes. These supplement source inspection and byte evidence; they are not a host-wide syscall trace or hostile same-principal race qualification. |
| Two-reader lifecycle | Pass. Ordinary, refused-startup and runtime-failure paths retain owned reader exit records. Whole and split `stop` lines on the actual pair CLI both stop with stdin deliberately held open, server exit 0, both readers exit 0, and listener closure separately asserted. They are not EOF/timeout successes. Fault children requiring SIGTERM and browser harness SIGTERM are recorded as forced cleanup followed by wait, not normal reader shutdown. |
| Static/single compatibility and controls | Fresh static and single-catalog browser sessions pass, alongside their directly affected Node/native controls. Pair checks exercise semantic A/B/All selection, keyboard selection, Tab/Shift+Tab, skip-link focus without reset, Back/Forward, disclosures and Guided/Studio/Expert warnings. Browser exact-name controls retain special Unicode/markup text and distinct LF/CR/CRLF versus literal escapes. Control-character catalog fixtures are protocol/legacy representations, not claims that Windows created those filenames natively. R1 remains the accessible source-label defect. |

The runtime trace covered server startup parsing → sequential adoption of two readers → random session handles bound to validated digest/projection → handle-qualified queries and response validation → UI selection/inspector → shutdown. Data is adopted once; no hot reload or persistent registration was introduced. The unchanged native read-only reader was inspected in its new composition, not assumed safe solely because it was unchanged. Packaging dependencies remain source-only; no package refresh was performed.

## Fresh reviewer execution

Environment: Windows amd64; explicitly selected retained Go **1.26.8**, Node **24.19.0**, installed headless Chrome **153.0.8010.48**. `run-native.ps1` retains the explicit toolchain executable under `ObeliskDev/audit-2026-09-19/go-mod/golang.org/toolchain@v0.0.1-go1.26.8.windows-amd64/bin`. Offline `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, `GOWORK=off`; fresh review cache/temp directories and existing module cache. No GOTMPDIR override, PATH-Go substitution, installation, dependency download or security change.

Fresh adapter: `R/output/reviewer-reader.exe`, 14382080 bytes, SHA-256 **`d7260541e248ea69a76cbeb1b281fc2e3f13e1df3507933fa8d930536040ae15`**. Built from the byte-verified `R/source` using `go build -mod=readonly -o ... .`. Exact paths, commands and native exits are in `run-native.ps1`, `native-command-exits.json` and `build.log`.

`reviewer-binary-identities.json` also identifies the newly built candidate test double and reviewer-only fault emitter. `review-fault.go` lives outside the preserved source copy and adds timeout/overflow simulation to the existing double. It is never substituted for the native reader in native controls.

Fresh fixture mapping:

- `catalog-inputs`, `correction-inputs`, `multi-inputs`: regenerated by the selected native tests, with new disposable native inventory producer inputs where applicable. Existing author input directories/binaries were not reused for decisive execution.
- `review-inputs/A|B/snapshot.json`: independent valid synthetic catalog fixtures described in R1, built by `setup-review.mjs`; full expectations in `review-expectations.json`.
- `boundaries` and `boundaries-verified-exit`: native-readable deterministic row/copy/input-size boundary fixtures.
- `multi-faults`: explicitly synthetic protocol-double projections and cases, not native inventory results.
- `fixture-identities.json`: final exact bytes/hashes; input preservation checks occur in the selected tests, independent probes and watcher/lock record. This final manifest alone is not evidence against transient writes.

| Fresh execution | Actual result |
| --- | --- |
| Enumerate `^TestGUI`, then `go test -mod=readonly -count=1 -json -run '^TestGUI' -timeout 4m .` | **26 top-level tests and 242 subtests pass; 0 failures, 0 skips.** `native-selection.txt`, `native-tests.jsonl`. |
| Reader/double builds, `go vet -mod=readonly .`, read-only `gofmt -l gui_catalog.go gui_multi_snapshot_test.go` | Exit 0; formatting output empty. Exact exits/logs retained. |
| Enumerate and run five Node files: `stop-command.test.mjs`, `preview.test.mjs`, `catalog.test.mjs`, `catalog-correction.test.mjs`, `multi-snapshot.test.mjs` | **31 pass, 0 fail/cancel/skip/todo**, command exit 0. `node-selection.txt`, `run-node.ps1`, `node-tests.log`, `node-command-exit.json`. |
| Native supplemental API/boundary/lock/CLI probes | Verified run exit 0. `supplemental-results-verified-exit.json`, `read-only-observations-verified-exit.json`, retained script/stdout/stderr. Zero input-directory watcher events. |
| Independent protocol fault probes | Five completed cases: runtime timeout, response overflow, startup timeout, partial output/exit, malformed shape. `fault-probe-results.json` records individual elapsed times and reader exits; command exit 0. These are simulations. |
| Browser work | **14 attempts, 13 completed sessions**, detailed below. One external reviewer-harness failure retained; corrected rerun completed. A successful defect-reproduction assertion is not a passing product contract. |
| Checkout whitespace | `git diff --check` exit 0. Existing line-ending conversion warnings were emitted; no normalization performed. |

Completed browser sessions and recorded check counts (not unique test totals): native pair 17; large identities 16; control names 9; UNKNOWN/zero 4; empty/excluded 4; malformed-scope refusal 2; duplicate-copy refusal 2; reopen 17; simulated runtime failure 4; single catalog 22; static 55; independent same-basename/stale work 21; independent reversed order 21. Each has `browser-results.json`, screenshots and, for catalog modes, `catalog-process.json`; exact batch argv/exits are in `browser-command-exits.json` and `run-browser.ps1`.

Reviewer diagnostics retained separately:

- First independent browser attempt stopped after 13 recorded checks because the external review harness redeclared a CDP global `const q`. The candidate was not changed. `review-browser-checks-attempt1.mjs`, the failed result and log remain. The reviewer-only harness changed that declaration to `var`; corrected and reversed-order sessions completed 21 checks each.
- First supplemental probe completed its assertions but the PowerShell `Start-Process` wrapper returned a null numeric exit. That missing exit was not called zero. A separate retained coordinator used `System.Diagnostics.Process` and new output/fixture names; the repeated bounded probes returned an actual **0**. These overlapping executions are not additive coverage totals.
- No new security detection or approval rejection occurred.

Screenshots compare rendered semantics against fixture expectations, not a newly claimed Figma design. Decisive images include the same-basename ambiguity and qualified inspector screenshots, `browser-large/large-inspector-2.png`, native pair Library/inspectors, scope/empty states, and failure views. Pair captures use 1440×1024 at scale 2. Static controls also exercise 390×844; shared harness metadata lists both viewports even where a pair scenario did not run the narrow viewport. No additional responsive/Figma fidelity claim is made.

Historical author evidence remains separate: the implementation report reports 26 native top-level tests/242 subtests, 31 Node tests plus an overlapping seven-test follow-up, 21 completed browser sessions, build/vet/format/whitespace checks and retained setup retries. Those are not added to fresh reviewer totals or substituted for executions above.

## Preservation, limits and next action

`pre-report-preservation.json` rehashed all **311 pre-existing repository files** in the baseline: no drift. All 308 retained source-copy entries still match the author manifest. The only repository addition from review is this report; implementation, tests, living records, earlier reports and design inputs remain untouched. Final checkout/index/operation-marker checks and this report's SHA-256 are recorded separately in `R/final-preservation.json`, avoiding a self-referential report hash. The index remains empty and branch/HEAD remain those above.

The accepted frozen ZIP was only hashed, never extracted/rebuilt/updated. Its before/after SHA-256 remains `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`. The available historical package source manifest matches `60ea696a91f4c467b543b381a93e6294813ed5030e557386722e55b1de6713a0`. The author's reader and candidate manifest also retain their initial hashes. Author evidence was read selectively and never overwritten; historical directories were not exhaustively rehashed.

All 14 browser attempts record browser stop/wait and server stop, including the failed harness attempt. Per-reader exit records and independent CLI/fault lifecycle records are retained. Whole-stop server PID 11692 waited readers 3908/8280; split-stop server PID 18096 waited readers 8216/17440, all native reader exits 0. Fault-emitter nonzero exits/forced stops are preserved separately. No owner dogfood process was attached to or stopped. No operational backend, production catalog or owner media was accessed.

Residual limits: selected generated-data two-snapshot workflow only; no host-wide syscall tracing, hostile filesystem race qualification, scale/ACL/power-loss campaign, package distribution, clean-machine or broader-platform certification. Prompt 20 remains **DEFERRED_BY_OWNER**. Generation-independent architecture, LTO-8-first physical validation and separate Blu-ray work remain deferred unchanged. Missing new Figma frames and optional visual polish are not blockers. No additional substantiated implementation finding was identified within this review scope.

**One next action:** separately authorize the bounded R1 presentation-identity correction and a targeted same-basename/reversed-order recheck. Stop here; no fixes, staging, commits, pushes, merges, package refresh or automatic follow-on work.
