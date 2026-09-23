# Focused source-label recheck — 2026-09-21

**GUI_MULTI_SNAPSHOT_READY_FOR_OWNER_REVIEW**

**R1/P2 — Source-label ambiguity: CLOSED.** The corrected same-basename inputs are visibly distinguishable in Library, Find, source summaries, counts, selectors and inspector attribution. Fresh browser execution confirms that the labels and selected evidence remain attached to the correct startup input. No remaining blocking finding was identified within this focused scope.

This is an AI-assisted same-session focused reviewer recheck under the submitted Prompt 23B, with the earlier implementation/review context available. It is not a context-isolated agent review, human certification, another substantive whole-candidate audit, owner acceptance or publication.

## Exact reviewed candidate

- Branch: `feat/gui-multi-snapshot-readonly`.
- Full HEAD and overall uncommitted multi-snapshot patch parent: `a099ddc7530d81a9f3206e426b81def5172b16ec`.
- Index empty and operation markers absent at entry and final preservation check. No branch change or remote operation.
- Correction evidence (`A`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-source-label-correction-20260921-143056`.
- New reviewer evidence (`R`): `C:\Users\nsott\AppData\Local\ObeliskDev\gui-source-label-recheck-20260921-144637`.
- Controlling corrected manifest: `A/candidate-identities.json`, SHA-256 **`efff5677db62e05f42835891de468f935b64f6e83d7d0201e07d193c8e5e270b`**. All **314** listed working files matched; no extra visible paths or unexplained drift. This inventory includes new source/tests, prior reports and preserved design references, not 314 audited code files.
- The author follow-up is included in that manifest: SHA-256 `7689398dd1ebca462b4da98c584e5ca582ebaca51681213882943a04ce5b2ede`.
- `R/reviewed-identities.json` retains the exact manifest; `R/source` retains the verified source bytes used for this execution. `initial-checkpoint.json` retains full tracked/untracked status, branch, HEAD, index and markers. The new closing report is accounted for separately in `final-preservation.json`.

No earlier completed focused R1/P2 recheck was found. The prior NEEDS_CHANGES review and AUTHOR_ADDRESSED report were not treated as reviewer closure. Applicable instructions, the actual controlling review, author follow-up, original implementation report, relevant README and current four living-record entries were consulted using the established source mapping. The exact submitted instruction is retained as `R/submitted-prompt.txt`.

The correction delta is distinct from the full existing multi-snapshot implementation: UI labels and scoped CSS; browser regression routing/new label test; README; four living-record appends; and the author follow-up. `R/correction.diff` preserves the correction's existing-file delta; the full new label module and report are in the verified source inventory. No checkout of the published parent was substituted for the working candidate. Native Go/module files match the retained pre-correction identities; reader, adapter/protocol identity semantics and native schema were not changed by the label correction.

## R1 evidence and source disposition

The original defect was presentation ambiguity, not established wrong-record retrieval. The correction closes that defect without replacing the existing snapshot-qualified authority.

| Surface or boundary | Inspected anchors and fresh evidence |
| --- | --- |
| Session association | `scripts/gui-preview/catalog-ui.mjs:11`–14 creates a map once from startup handles to `Snapshot A/B — basename`; subsequent rendering looks up the handle. `catalog-adapter.mjs:91` and its adoption/query code bind validated reader projections and responses to those handles. Result order, current filter, record ID and response arrival do not assign labels. |
| Library and selector | UI lines 48, 56–60 use the shared source label for options, headings, summaries and View buttons. Forward/reversed browser cases compare actual visible text, scope/time and handles to independently derived input expectations and the direct server bootstrap. Collection/storage attribution at lines 74/77 uses the same helper. |
| Results, totals and inspector | UI lines 104, 132 and 134 qualify inspector, per-source counts and result button text. Six same-basename recorded entries remain separate in All, including overlapping ID/path evidence. Both result buttons are distinguishable before selecting or opening details. Browser accessibility-tree button names contain the distinct qualifiers. |
| Evidence authority | `catalog-protocol.mjs:72` validates requested snapshot groups and exact IDs; UI lookup still uses snapshot handle plus native ID. External reviewer checks compare input digest/full record expectations to `/mode.mjs`, then compare native `/catalog-query` group handles/IDs/counts. Browser selection checks compare full hash, bytes, path and first-seen values against fixture expectations, not merely another label. |
| Scope/time and stable navigation | UNKNOWN remains unknown; native OFF and ON scopes, counts and recorded offset time remain attached to their input. A/B/All, navigation, Back/Forward, keyboard focus and delayed former-All response checks preserve association. New launches may reverse letters; within-session identity stays fixed. No sorting control exists in this catalog workflow, and none was invented for the recheck. |
| Long/special names | `style.css:23`–24 bounds multi-snapshot columns and allows wrapping. At 1440×1024 scale 2, the leading qualifier remains visible with the long `café & +%# ... .json` basename in both argument orders. Text-shaped markup remains inert. `displayName` and textContent preserve the supported name-display policy; no new path disclosure or lookup key was introduced. |
| No results/failure | Per-source zero counts remain qualified. Existing one-reader-failure simulation clears results/inspector, latches error and never reports an unqualified successful All total or demo fallback. The error is whole-session and contains no individual basename label to misattribute; the correction does not infer a specific failing source or add partial-success behavior. |
| Shared controls | Fresh exact large-ID, control-character/exact-search, duplicate refusal, empty/excluded, static and single-catalog controls pass. Skip-link and Tab/Shift+Tab preserve the semantic filter and Find workspace. Browser accessibility inspection is not a claim of screen-reader execution. |

The author regression (`source-label-browser-checks.mjs:9` onward) reads independent fixture expectations, checks visible attribution and selected evidence, and runs through the real server/native reader. The external reviewer wrapper adds direct bootstrap/native response comparisons and retains `server-bootstrap.json`, `native-all-response.json` and `accessibility-final.json`. All reviewer additions live outside the repository and outside the pristine `R/source` copy; implementation files and repository tests were not modified.

## Baseline, fixtures and screenshots

The retained pre-fix failure was inspected, not replayed: `A/before-regression.log` records the expected failed Library source-label assertion and exit 1. Its runner targets the complete pre-correction runtime under `A/before`, not only the parent commit. The pre-fix UI hash matches the substantive review's retained runtime: `78a258f6789ecde27ef4066deea880a54de7d4a763750dccee4992f2e473973e`. The historical ambiguous result screenshot and fixture expectations remain intact. This is reuse of valid red evidence, not a new failed reviewer run.

Fresh review-owned input copies live under `R/label-inputs`; setup and expectation files are retained. No native inventory/source scan was rerun. The supported generated fixture bytes were copied from existing evidence, with new reader processes, ephemeral ports and browser profiles.

Same-basename inputs:

| Input | SHA-256 | Independent record-1 expectation |
| --- | --- | --- |
| `label-inputs/same/0/snapshot.json` | `5307642a99c351ffc7d92a61e03a658380d31ac8b2c15476e42896a0a541a5c8` | `shared.txt`, 1111 bytes, hash `a` × 64, first-seen `2021-02-03T04:05:06+02:00` |
| `label-inputs/same/1/snapshot.json` | `dc4c7577376e623639dbe21b235602dbf23d4acf8cbf05d23d483cb4d1322085` | `shared.txt`, 3333 bytes, hash `c` × 64, first-seen `2022-03-04T05:06:07-05:00` |

Each has three records and historical UNKNOWN inventory time/scope. Both remain valid distinct inputs. The long-name pair uses copied native-generated A and explicitly synthetic `b-aged.json` fixtures with OFF/ON, three/two records and different inventory observation times, including `2024-02-03T04:05:06-05:00`. `setup-labels.mjs` derives expectations from the small-ID fixture bytes independently of the UI. Large-ID fixtures use the existing Go-generated exact representations rather than JavaScript numeric reconstruction.

`label-cases.json` and `same[-reverse]-expectations.json` / `long[-reverse]-expectations.json` identify exact launch arguments and expected input order. Every label session's `source-associations.json` records its generated handle-to-input/digest/label mapping, validated against the direct bootstrap. Reversed launches use the input-specific expectations in reversed order; durable A/B identity is not claimed.

Inspected fresh screenshots include `browser-same/labels-all.png` and `browser-long-reverse/labels-all.png`; associated Library, inspector and no-result images are retained alongside them. They show both qualifiers in the actual result/source context without hover or later inspection. Captures use the established 1440×1024 desktop viewport at scale 2 (2880×2048 PNG). Static controls additionally run 390×844. Shared harness metadata lists both viewports even for desktop-only pair cases; no broader responsive or Figma fidelity claim follows.

## Execution performed in this recheck

Environment: Windows x64, installed Node **24.19.0**, Chrome **153.0.8010.48**. `environment.json` records the actual Node executable. No installs, updates, global settings, security workarounds or new detection occurred.

The reader copied into `R/output/reviewer-reader.exe` hashes to **`d7260541e248ea69a76cbeb1b281fc2e3f13e1df3507933fa8d930536040ae15`**. Its source/build association is the retained substantive-review source manifest and explicit offline Go 1.26.8 build log; the relevant native source remains identical. The reader-double hash is separately retained in `reader-identities.json` and used only for the labeled runtime-failure case. No native build, vet, 26/242 suite rerun or package-reader replacement was justified or performed.

Selected Node test names were enumerated before execution in `node-selection.txt`. `run-controls.ps1` retains exact process-scoped fixture/reader environment and commands:

```text
node --test scripts/gui-preview/stop-command.test.mjs scripts/gui-preview/preview.test.mjs scripts/gui-preview/catalog.test.mjs scripts/gui-preview/catalog-correction.test.mjs scripts/gui-preview/multi-snapshot.test.mjs
node <R>/cli-stop.mjs <R>
```

Result: **31 Node tests pass, 0 fail/cancel/skip/todo**, exit 0 (`node-tests.log`, `node-exit.json`). Explicit CLI stop probe also exits 0 (`cli-stop-exit.json`).

`run-browsers.ps1` and `browser-exits.json` retain each exact argv/exit. The external `review-browser.mjs` serves unchanged runtime from `R/source`, routes label scenarios through `review-labels.mjs`, and uses the unchanged candidate controls otherwise. **12 reviewer browser sessions completed, all command exits 0; no failed attempt or retry in this recheck.**

| Fresh reviewer selection | Sessions / logged checks |
| --- | --- |
| Same-basename forward/reverse | 2 × 18 |
| Long Unicode/punctuation basename forward/reverse | 2 × 18 |
| Native pair/keyboard/delayed response | 1 × 17 |
| Large exact IDs | 1 × 16 |
| Special/control/exact names | 1 × 9 |
| Empty/all-excluded | 1 × 4 |
| Duplicate refusal | 1 × 2 |
| Simulated reader runtime failure | 1 × 4 |
| Single catalog | 1 × 22 |
| Static | 1 × 55 |

Direct wrapper/accessibility assertions also execute without inflating logged check counts. Browser sessions are not unique assertion totals. The delayed-response seam and reader double are simulations; normal label selection/evidence and the additional bootstrap/query probes use the verified native reader. `git diff --check` exits 0. Only the single closing report is added to the checkout.

Historical layers remain separate: original substantive reviewer 26 native top-level tests/242 subtests, 31 Node passes, 13 completed browser sessions with its harness failure/retry; correction author 31 Node passes, 12 completed corrected sessions and the expected red baseline. Those historical runs were inspected as provenance and are not counted as fresh reviewer execution. The earlier author's overlapping seven-test follow-up is not an additional unique total.

## Processes, preservation and limits

All browser contexts have bounded deadlines and retained browser PID/stop/wait, server-close and reader-exit records. Browser harness SIGTERM is forced cleanup followed by wait, not command-driven browser shutdown. The failure-double lifecycle is likewise separated from normal native-reader exits.

The explicit command check launches the actual development CLI, writes `stop\n`, keeps stdin open, waits and checks listener closure separately. `cli-stop.json`: server **4840**, readers **17168/9136**, all exit **0**, no forced timeout, listener closed. The selected Node pair control also covers split stop with stdin open. No unrelated owner/dogfood process was attached to or stopped.

Before closing, every pre-existing repository manifest entry and every retained source-copy entry still matches. `preservation-before-report.json` also verifies native-source preservation and the red-runtime identity. `final-preservation.json` records final branch/HEAD/index/markers/status, zero pre-existing drift, the sole added closing-report path and its SHA-256 separately. README, four living records, implementation/tests, historical reports and all design/reference inputs remain unchanged during this recheck. Historical evidence was read selectively and not overwritten.

Frozen ZIP SHA-256 remains **`f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`**. It was hashed only, not extracted, executed, rebuilt, repackaged, renamed or published. Existing package/dogfood directories and manifests were not modified.

Scope remains generated-data, two-fixed-snapshot development viewing. No production catalogs/media, source scanning, registry/history, operational comparison, platform/scale/ACL/power-loss or hardware qualification. Prompt 20 remains **DEFERRED_BY_OWNER**. Optional visual polish and deferred work do not reopen R1. Closure relies on the prior substantive review plus this focused correction/binding evidence; it is not a fresh audit of unrelated behavior.

**ONE next action:** owner acceptance and separately authorized scoped publication of the **complete multi-snapshot source candidate**, including its original implementation and correction. No publication, frozen-package refresh, new audit or next feature is started by this report.
