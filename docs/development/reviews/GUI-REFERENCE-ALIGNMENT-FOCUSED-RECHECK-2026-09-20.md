# GUI skip-link focused recheck - 2026-09-20

GUI_PREVIEW_READY_FOR_OWNER_REVIEW — R1 CLOSED

Post-reboot checkpoint verification and targeted reviewer execution against the corrected uncommitted preview. No implementation, test or living-record edits. This is the same Codex conversation/session with author context available, not an independent agent/context or human review. No sub-agent was used. No prior completed closing recheck or interrupted focused recheck was found; the original NEEDS_CHANGES review and author follow-up were not mistaken for reviewer closure.

## Checkpoint and exact scope

Repository `C:\Users\nsott\source\repos\obelisk`; branch `feat/gui-preview`; full HEAD/overall GUI base `05cf50afc004803e1c0de20a6ce444f6a538c24d`. Read-only Git root, branch, HEAD, porcelain status and staged-diff commands all exited 0. Index empty; four living records modified and preview/reports/Figma/root PDF untracked as expected. No inspected index/HEAD/packed-refs lock, merge, rebase, cherry-pick, revert or bisect marker. No cleanup, normalization, reset, branch change or remote lookup.

Read the controlling [R1 review](GUI-REFERENCE-ALIGNMENT-REVIEW-2026-09-20.md), [author follow-up](GUI-REFERENCE-ALIGNMENT-REVIEW-FOLLOWUP-2026-09-20.md), latest four living-record entries, actual source/tests and runner README. Applicable contribution guidance remains as read earlier in this conversation; repository instruction search found no AGENTS.md. Earlier configuration/job tasks were not resumed.

Author evidence root **A**: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-skip-fix-20260920-005527`.

New reviewer evidence root **E**: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-skip-recheck-20260920-110358`.

Controlling instruction `C:\Users\nsott\.codex\attachments\3f9fda85-f26e-401e-b9d5-97d7e24dd6af\pasted-text.txt` is retained byte-for-byte as `E\submitted-prompt.txt`. Historical instructions were not replayed.

The actual `A\final-identities.json` contains 16 paths: eight preview files, four living records, the correction report, and three earlier GUI reports. Every current size/hash matches. It includes the correction report, also separately verified as SHA-256 `54812775fb774388e30b413d21d346f7a6399bbc4dcf00bcb1d7c3a4e730bad7`. Principal corrected identities:

| Path | SHA-256 |
|---|---|
| scripts/gui-preview/app.mjs | f4b555e851406f17c7dd6a6d01759ae352edde7b05b4f2ad01586261a252db3c |
| scripts/gui-preview/browser-check.mjs | e04540cbf378ab486e1901582c862aaf849e59c30d1a6896169820b22ff3e1c8 |
| scripts/gui-preview/index.html | bd3c41bc59bb3c0363b77f8a58495fb36d037902cadf524504ece47d27588a9a |
| scripts/gui-preview/style.css | bd109653e1a17fa75df32973a5115fcf18668cd7d827b50cd3f6d27a5e40dab0 |

Path reconciliation uses the actual manifests, not subtraction between unrelated totals. The original 251-path inventory gained the substantive-review report to become the author's 252 starting paths. Of those, the author preserved 246 and intentionally changed six: app.mjs, browser-check.mjs, CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md and REVIEW_COVERAGE.csv. The new correction report yields 253 paths. Overlaying the 16 final identities on the 252 starting identities produces exactly those 253 expected paths, all matching now: no missing or unexpected mutation. The separately extracted seven reference entries also match; they are not seven additional repository files. The new submitted prompt is external evidence, not a candidate file. Figma, original ZIP and root PDF are already in the repository inventory.

Before execution, saved all eight preview files, including necessary imports/assets, to a non-overwriting `E\preview` snapshot; all eight hashes match the corrected candidate. `E\before.json` is the full reconciled baseline, `snapshot.json` verifies the backup, and `git-before.json` retains native commands/exits/status. The original pre-fix copy's eight hashes were verified against `A\pre-fix-identities.json`; author red logs and terminal exits remain available. `reviewed-correction.diff` retains the author's scoped source/test diff. No historical evidence was modified.

## R1 disposition and test adequacy

**R1/P2 CLOSED**: “Skip to content changes workspace instead of focusing its content.” The real link at [index.html:11](../../../scripts/gui-preview/index.html:11) remains keyboard-accessible with href #main. Main has tabindex -1. [app.mjs:155](../../../scripts/gui-preview/app.mjs:155) intercepts only this link's click, prevents default fragment navigation and focuses the current main. The ordinary hashchange renderer at line 160 remains active. The link listener is installed once on the stable shell, resolves its target at activation and does not render or change application state. No broad delegated anchor interception, positive tabindex ordering, removed navigation or router rewrite.

Inspected [browser-check.mjs:56](../../../scripts/gui-preview/browser-check.mjs:56) through its skip/navigation assertions. These checks would detect the original defect: they compare the actual URL, title, content, input values, selection, history length and performance time origin, require real main focus, and require the original first content node to survive. They do not merely locate a heading or observe scroll. The retained author red execution records all four strict cases failing, including both reported workspace resets. Library kept its title but changed URL/history and rebuilt content, so its strict control also failed. That historical red evidence is reused with author attribution; no red replay or mutation was needed now.

## Fresh execution

Installed Node v24.19.0 at `C:\Program Files\nodejs\node.exe`; fresh CDP browser reports Chrome 153.0.8010.48, launched from `C:\Program Files\Google\Chrome\Application\chrome.exe`. Commands executed once from the repository root:

```powershell
node --test --test-isolation=none scripts/gui-preview/preview.test.mjs
node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-skip-recheck-20260920-110358\browser'
```

Both exited 0. Node: **7 top-level passes, 0 failures/skips/cancellations, no subtests**. Browser: **55 checks passed, 0 failures/skipped checks**, including four focused keyboard cases and three directly affected navigation assertions. Existing complete suites were the practical unchanged harness; this was not a repeated substantive visual review. `check-inventory.txt`, `node-tests.txt`, `browser-run.txt`, `exits.json` and `browser/browser-results.json` retain definitions and outcomes. No setup interruption or retry in this execution.

Each keyboard case deterministically focuses the first real primary-nav link, then uses actual Shift+Tab to reach the visible skip link and Enter to activate it. The settled before snapshot is taken after this setup. No direct main focus, mocked router, test-created control or handler patch substitutes for activation.

| Case | State retained across skip | Actual focus and sequence |
|---|---|---|
| Library | Library URL/content/control values and content node | MAIN#main, Tab to Guided, Shift+Tab to Settings, Tab to Guided |
| Find | #find; SIM-OCC-03 query; one selected Field notes v1 result and occurrence details | MAIN#main, Tab to Guided; reverse/forward usable |
| Smith Wedding | #smith-wedding; IMG_4587 query; IMG_4587.CR2 selected; HDD-017 expanded after toggle setup | MAIN#main, Tab to Library breadcrumb; reverse to Settings and forward usable |
| Find after navigation and Expert rerender | Find query/result/selection and Expert disclosure | Same preserved-state and focus checks pass after rendering lifecycle |

All four cases preserved the full before/after URL, history length, time origin and content-node identity. This, source inspection and actual Back/Forward behavior support no unintended workspace/history transition or reload; history length alone was not treated as proof. The real project link, Library breadcrumb, normal Back to Library and Forward to project pass, as do the existing search/filter/selection/disclosure/demo/navigation controls. Initial focused cases use route setup; ordinary real links and keyboard project navigation are separately exercised by the same runner. No persistence across restart or unrelated normal navigation is promised.

`browser/skip-link-results.json` retains every predicate, before/after state, active focus, next Tab and reverse target. Fresh screenshots are `browser/skip-link-{library,find,smith-wedding,find-rerender}.png` and `skip-destination-*.png`, viewport-only 1440x1024 CSS pixels/scale 2. Find's focused shortcut and Smith Wedding's focused main were visually inspected; outlines are visible and content remains present. Screenshots supplement actual activeElement evidence. Existing ancillary narrow captures are not a new mobile-design acceptance.

Page request observations contain only `http://127.0.0.1:53063/`, style.css, app.mjs and fixtures.mjs. No runtime/console errors or external/API requests observed. Existing Node isolation tests pass; no new broad server campaign, backend startup, catalog/key/media access, installation or global setting change.

Node CLI PID 15600 served newly assigned loopback port 53061, then was stopped and waited (SIGTERM, exit null). Fresh task browser PID 7448 and its port-53063 server were stopped/waited/closed; browser result records SIGTERM, exit null, browserStoppedAndWaited=true and serverStopped=true. No pre-reboot PID or port was reused to control a process; no unrelated process was stopped.

## Attribution, preservation and next action

Separate historical layers: alignment author 7 Node/48 browser; original substantive reviewer 7 Node/48 browser plus separate 3-pass/2-fail probes and 2 isolation passes; duplicate review identity-only; correction author red reproduction and 7 Node/55 browser passes. This task freshly executed 7 Node/55 browser once; overlapping runs are not summed. This is a targeted closure in the same conversation with author context, not independent-context certification.

Final verification: all 253 pre-existing repository paths remain byte-identical to this task's verified baseline, including source/tests, four living records, historical reports, Figma/ZIP/PDF and backend/production UI. All seven extracted reference entries remain identical. The sole new repository file is this closing report. Snapshot bytes also remain unchanged. `E\final-preservation.json` records final identities, report hash, Git/index/markers and process results; `evidence-identities.json` identifies prompt/logs/screenshots. No staged changes, commits, pushes, merges or branch changes.

Limitations: no screen-reader, accessibility-standard, cross-browser/platform or owner visual certification; no repeat full Figma comparison or missing Find-design resolution. Old living records intentionally remain at author-addressed status because this review authorizes only a new report. This report supplies the closing disposition.

ONE next action: owner visual acceptance and separately authorized scoped preview publication (Prompt 10). No automatic commit or further recheck. Prompt 11 disposable-catalog integration remains gated on publication at the actual new SHA.
