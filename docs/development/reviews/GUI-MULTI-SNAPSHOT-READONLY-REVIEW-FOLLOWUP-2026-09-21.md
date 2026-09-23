# Same-basename source-label correction — 2026-09-21

**READY_FOR_GUI_MULTI_SNAPSHOT_LABEL_FOCUSED_RECHECK**

**SOURCE_LABEL_FINDING: AUTHOR_ADDRESSED — R1/P2** in [the controlling substantive review](GUI-MULTI-SNAPSHOT-READONLY-REVIEW-2026-09-21.md). This is implementation completion under the owner's submitted Prompt 23A, not reviewer closure or owner acceptance. No automatic recheck or publication followed.

## Candidate and preservation

Branch: `feat/gui-multi-snapshot-readonly`. Full unchanged HEAD/complete multi-snapshot patch parent: `a099ddc7530d81a9f3206e426b81def5172b16ec`. Index empty; no active operation markers. The parent does not include the uncommitted implementation being corrected.

Task evidence (`T`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-source-label-correction-20260921-143056`.

Historical reviewer evidence (`R`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-review-20260921-140103`.

Before edits, all 311 files in R's original baseline plus the controlling review report matched their recorded identities. Review report SHA-256: `a5eb2ef3462a8439549c19c7e1b44357244b6d1e7c42097ccfa9b282dcdb8d76`. `T/before-identities.json` records those 312 entries; `T/before` retains their actual bytes, including new/untracked source/tests. `initial-checkpoint.json` records branch/HEAD/index/status/markers and zero drift; `before-complete.diff` preserves the full existing tracked patch. No previous completed correction was found.

The actual review and reproduction, original implementation report, relevant README and four living records were consulted. Earlier Prompt 22/23 source mapping and contracts were reused rather than re-executed. Prompt 23A's exact submitted text is retained in `T/submitted-prompt.txt`. No new repository-issued change ID is invented.

The correction delta is eight existing paths:

```text
scripts/gui-preview/catalog-ui.mjs
scripts/gui-preview/style.css
scripts/gui-preview/multi-snapshot-browser-checks.mjs
scripts/gui-preview/README.md
docs/development/CODEX_HANDOFF.md
docs/development/NEXT_ACTIONS.md
docs/development/OB_STATUS.md
docs/development/REVIEW_COVERAGE.csv
```

Two new paths:

```text
scripts/gui-preview/source-label-browser-checks.mjs
docs/development/reviews/GUI-MULTI-SNAPSHOT-READONLY-REVIEW-FOLLOWUP-2026-09-21.md
```

This is additional to the reviewed overall uncommitted multi-snapshot patch, whose complete 11-modified/4-new implementation path set is listed in the controlling review. The controlling review itself was another pre-existing untracked report. The final manifest/status and complete parent diff distinguish all these from this correction's `correction.diff`. Unrelated PDF/Figma/ZIP references remain preserved.

`T/candidate-identities.json` and `T/after` retain post-correction bytes including the new test module and this report. `final-preservation.json` identifies all changed/added paths, final index/HEAD/markers, report hash separately, before-copy preservation, and frozen-package identity. It checks every pre-existing file against the authorized correction path set; historical reports, native/runtime files outside that set, and reference inputs must show no drift.

## Cause and correction

The reader/session already distinguished the two inputs correctly. UI code reused the basename-only `s.label` in results, counts and summaries. Two different `snapshot.json` inputs therefore had identical visible/accessibility source names despite different handles and correctly selected evidence.

`catalog-ui.mjs` now creates one presentation map keyed by each validated startup handle. The first startup source is **Snapshot A — basename** and the second is **Snapshot B — basename**. A common `sourceLabel` lookup supplies Library headings and View buttons, selector options, Find disclosure summaries, collection/storage attribution, result buttons, per-source totals and inspector attribution. Scope and recorded time remain inside their corresponding labeled source cards. Basenames use the existing `displayName` supported-character representation and render via textContent. No filename, catalog metadata, native ID, authority field or protocol format changes.

The mapping is initialized once for the fixed session. Filter/result order, selected record and delayed response arrival do not assign letters. Reversing the startup arguments intentionally reverses A/B on a new launch; there is no persistent device/catalog identity claim. Full digest/session details remain under the existing rules. No new absolute-path disclosure, field, endpoint or browsing control was added.

Three narrowly scoped CSS rules allow multi-snapshot text to wrap, keep split columns bounded, and constrain the selector width. The distinguishing qualifier is first, including when a long filename wraps or the native select clips its trailing text. Single/static styling is unaffected. The original stylesheet BOM was preserved during final diff cleanup; this encoding-only cleanup did not change browser semantics.

Existing failure behavior remains whole-session refusal/clearing. Its global error message has no individual basename renderer to replace; the failing reader is not guessed or relabeled as healthy. Source-specific scope/zero-result attribution uses the shared label before failure. No new partial-success/error-provenance feature was introduced. The catalog workflow has no supported sorting control, so no sort feature or simulated sort was added.

## Reproduction and focused regression

The historical before screenshot and fixture expectations are retained as `T/historical-before.png` and `historical-before-expectations.json`, copied from the controlling review without modifying it. They show two identical result/source names at 1440×1024, scale 2.

The new automated browser regression also ran against `T/before`'s exact reviewed runtime via `before-probe.mjs`. Only the test-harness import points to the new regression; the runtime is unchanged. It **fails as expected** on the first distinct Library-heading/View-control assertion (`before-regression.log`, exit 1). This is a deliberately failing pre-fix control, not a corrected-candidate failure or a replay of only the published parent. Its server/readers/browser were cleaned up and waited.

`setup-labels.mjs` creates new task-owned input copies and derives independent expectations from their catalog bytes, not from UI labels. `label-cases.json`, per-case expectation JSON and each browser's `source-associations.json` record the physical input paths, SHA-256, launch order, current handle, visible qualifier, scope/time and expected record evidence.

- Same-basename case: `label-inputs/same/0/snapshot.json` and `same/1/snapshot.json` copy the review's supported synthetic catalogs. Both have record 1/path `shared.txt`, but 1111 versus 3333 bytes, distinct full hashes and first-seen values. Both have historical UNKNOWN inventory scope/time. They remain valid distinct inputs; neither is renamed in response to the defect.
- Long-name case: separate directories contain the same supported basename beginning `café & +%# ` followed by nine `long-catalog-` segments and `.json`. Inputs copy the native-generated A and labeled synthetic `b-aged.json` fixtures: OFF versus ON scope, different counts, and the recorded `2024-02-03T04:05:06-05:00` historical timestamp. These are fresh copies of existing generated fixtures, not a claim that the native inventory producer ran again.
- Each case runs forward and reversed, with expectations reversed alongside input order. The native readers verify all input bytes; source/media path strings inside these fixtures are never opened. Exact large-ID/control-name controls use separate copied Go-generated fixtures and are not parsed through JavaScript numeric identity conversion.

The new `source-label-browser-checks.mjs` tests visible labels before selection, actual browser accessibility-tree result names, Library controls, selector names, scope/time binding, results/counts, full selected hash/size/path/first-seen evidence, A/B/All filtering, no-results labels, delayed former-All delivery, navigation/Back/Forward, Tab/Shift+Tab and skip-link preservation. The delay is an explicit client response-delivery simulation around normal native responses. It does not modify candidate runtime or reader behavior. Native semantic button text supplies accessible names; no screen-reader product was executed.

After screenshots: `browser-same/labels-library.png`, `labels-all.png`, `labels-inspector-0.png`, `labels-inspector-1.png`, `labels-no-results.png`, plus corresponding reversed/long-case images. Before and after use the existing 1440×1024 scale-2 viewport; scroll/selection states differ where named. Visual inspection confirms the leading A/B qualifier and wrapped long text. This is functional source-label evidence, not a new Figma/theme or universal accessibility claim.

## Execution actually performed now

Installed Node 24.19.0 and Chrome 153.0.8010.48 on Windows. No installation, dependency fetch, Go switch, global configuration or security change. Verified reader copied into `T/output/reviewer-reader.exe`: SHA-256 `d7260541e248ea69a76cbeb1b281fc2e3f13e1df3507933fa8d930536040ae15`. Its retained source/build association is the controlling review's verified source copy and offline Go 1.26.8 build. Native source is unchanged; **no native build/vet or 26/242 suite rerun was needed or performed**. The copied reviewer double is identified separately in `binary-identities.json` and used only for the labeled runtime-failure simulation.

Selected Node files were enumerated into `node-selection.txt` before execution:

```text
node --test scripts/gui-preview/stop-command.test.mjs scripts/gui-preview/preview.test.mjs scripts/gui-preview/catalog.test.mjs scripts/gui-preview/catalog-correction.test.mjs scripts/gui-preview/multi-snapshot.test.mjs
```

Result: **31 pass, 0 fail/cancel/skip/todo**, exit 0. `node-exit.json` records exact arguments and fixture/reader environment values; `node-tests.log` retains terminal results. These exercise directly affected composition, binding, duplicate, scope, filename, stop and compatibility controls. New label assertions are real-browser regressions, not merely internal-handle checks.

`run-browsers.ps1` retains exact commands and per-case output; `browser-exits.json` records all exits. **12 corrected browser sessions completed, all command exits 0**:

| Selection | Sessions / recorded checks |
| --- | --- |
| Same-basename forward and reverse | 2 × 17 |
| Long Unicode/punctuation basename forward and reverse | 2 × 17 |
| Native pair, selection, scope/time, keyboard and delayed response | 1 × 17 |
| Exact large IDs | 1 × 16 |
| Special names, LF/CR/CRLF and literal escapes | 1 × 9 |
| Genuine empty/all-excluded | 1 × 4 |
| Duplicate-copy refusal | 1 × 2 |
| Injected one-reader runtime failure | 1 × 4 |
| Single catalog | 1 × 22 |
| Static | 1 × 55 |

These are session/check counts, not unique assertions across all runs. The expected failing pre-fix session is separate: **13 total browser attempts, 12 corrected completions**. No corrected-run failure/retry occurred. Accessibility-tree assertions in the new module additionally execute directly and are not separately added to its check count. Shared harness metadata lists a narrow viewport too, but only the static control exercises 390×844; new pair scenarios use the desktop viewport.

Syntax checks for changed UI/new browser module and `git diff --check` returned 0. Existing Git line-ending warnings were informational; unrelated files were not normalized. Full host tracing, broader responsive/assistive technology, production/media and package qualification were not performed.

The first final-manifest script stopped when PowerShell treated Git's existing line-ending warning on stderr as a terminating error. The external script's Git-capture error handling was corrected and final preservation rerun; no runtime test or implementation correction was involved. The original script is retained as `finalize-attempt1.ps1`.

Historical totals remain historical: substantive review 26 native top-level/242 subtests, 31 Node passes, 13 completed browser sessions plus its separate reviewer-harness failure/retry; earlier author 31 Node and overlapping seven-test follow-up, 21 browser sessions and setup diagnostics. None were rerun or added into correction totals merely for identity checkpointing.

## Stop and preservation evidence

Each browser attempt records `browser-results.json` and `catalog-process.json`: server close and reader wait, plus browser harness SIGTERM/wait. Browser forced cleanup is not called normal command shutdown. The runtime fault double's failure/forced cleanup is likewise a simulation, not a normal native reader exit.

Separately, `cli-stop.mjs` launches the actual pair CLI, sends `stop\n` while keeping stdin open, waits for server exit **0** and both readers exit **0**, then verifies listener closure. `cli-stop.json`: server PID 12308, reader PIDs 9640 and 1616, no forced timeout. The selected Node pair CLI test also exercises split stop. No owner/dogfood process was attached to or stopped.

The frozen ZIP remains unchanged at SHA-256 `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`. No extraction, rebuild, signing, repackaging or publication occurred. Historical evidence/reports, original source snapshots, design inputs, owner work and native/backend code are preserved. Prompt 20 remains **DEFERRED_BY_OWNER**; second-machine and physical-media qualification are separate.

Development launch from the repository:

```powershell
$labels = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-source-label-correction-20260921-143056'
node scripts/gui-preview/server.mjs 0 --catalog "$labels\label-inputs\same\0\snapshot.json" --catalog "$labels\label-inputs\same\1\snapshot.json" --adapter "$labels\output\reviewer-reader.exe"
# Open the printed loopback URL. To stop, type stop and press Enter.
# Wait for both reader exits and the PowerShell prompt to return.
```

**One next action:** separately submit a focused R1 source-label recheck and directly affected binding controls on this exact uncommitted candidate. Stop here; no staging, commit, push, merge, automatic reviewer closure, package refresh or next feature.
