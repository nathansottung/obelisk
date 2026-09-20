# GUI reference alignment substantive review - 2026-09-20

NEEDS_CHANGES

One material interaction defect was reproduced: the keyboard Skip to content link replaces Smith Wedding or Find with Library. Reference alignment and the isolated synthetic-data boundary otherwise have no material blocker in the reviewed slice. No correction was implemented.

## Scope, checkpoint and provenance

Reviewed the combined uncommitted scaffold and design-alignment candidate on `feat/gui-preview`, HEAD/overall GUI base `05cf50afc004803e1c0de20a6ce444f6a538c24d`. Index empty; no in-progress merge, rebase, cherry-pick, revert or bisect. No existing completed substantive GUI review was found. The two implementation reports are author evidence, not a prior review. No branch switch, remote lookup or completed safety/publication work was repeated.

This source/browser review was performed by Codex in the **same conversation/session that authored the candidate**, with the author's context available and a shared checkout. It is a separate review execution, not an independent-agent or separate-context review, and not human certification. The unchanged repository suite ran against the checkout. Browser execution and extra server probes used a byte-identified disposable copy of all eight preview files. Only the added reviewer harness/probes differ in that copy; no application bytes were edited. No sub-agent or automatic review service was used.

Controlling submitted instruction: `C:\Users\nsott\.codex\attachments\ac4b148d-d45f-4c31-a2b0-c49e436309c9\pasted-text.txt`, retained as `submitted-prompt.txt` in review evidence. Read the current living handoff/status/next-actions/coverage entries, both GUI implementation reports, preview README, source and runners. Applicable repository contribution guidance was already available from this same session; no repository AGENTS.md was found. Historical pending-task instructions did not supersede this read-only scope.

Review evidence root, abbreviated **E** below:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-review-20260920-003545`.

Starting inventory: **251 files**, including tracked files, untracked GUI files, `docs/Obelisk.fig`, root `Untitled.pdf`, and the already-ignored supplied reference ZIP. This inventory is preservation evidence, not a staging allowlist or whole-repository source audit. All **14 entries** of the alignment author's `final-identities.json` matched: 0 missing, 0 changed. The other 237 inventoried paths are listed separately as outside that scoped manifest, not misclassified as unexpected candidate changes. No additional preview implementation file was found beyond the eight known files.

Cumulative scope versus the explicit base: eight new files under `scripts/gui-preview`, the two new GUI implementation reports, and changes to four living records. The six author-reported alignment files are a subset: app.mjs, index.html, style.css, browser-check.mjs, preview.test.mjs and README.md. The server and fixtures were introduced in the scaffold and were included in this review. `tracked-cumulative.diff` and `untracked-cumulative.diff` retain both kinds of candidate content; a HEAD-only checkout would not contain this preview. Complete scoped identities appear at the end of this report and in the starting manifest.

## Reference identities and rendered comparison

Reused the retained extraction at `C:\Users\nsott\AppData\Local\ObeliskDev\gui-alignment-20260920-001358\references`. README/manifest read; all seven manifest-listed files hash-verified again. Embedded historical prompt was not executed. ZIP SHA-256: `72a1f3308e2c4421b62c8bf286f9fab7651393a1dbbbbe1520774feefb331aca`. No new extraction or fig-kiwi decoding was needed.

| Input | SHA-256 |
|---|---|
| Page 1 / 01-library-teal-2x.png | `28f5feb46e5d42af39c44494bc7e3533bc7c29c32328f6d6497c099ee0e00a9d` |
| Page 4 / 04-project-comparison-evidence-inspector-2x.png | `537b6e6fd720acb0ae5992c4f665123a8b16fc6119caf95c8a86c94807f095f5` |
| Page 2 alternative | `16e7102c47c0568e210d95dd9a2481c641e12733f695cbceb82bf18803b3845d` |
| Page 3 alternative | `51ba559725307f9efbd47d9ce02314692c62e9de12a7e5fc80bb36b3aeef3e1f` |
| source/Untitled.pdf and root Untitled.pdf | `3c17b2b53335c2cf1e5d69bd616e4d6e3fa4613cf0dad0dfb21b0b786c8b7a86` |
| Original docs/Obelisk.fig, 219717 bytes | `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda` |

Visually compared primary reference pages 1 and 4 with **fresh reviewer screenshots** `E\browser\library-reference-aligned.png` and `E\browser\project-reference-aligned.png`. The author's two screenshots remain available and untouched; their separate hashes are in `author-screenshots.json`. They were not relabeled as reviewer captures.

Reviewer Chrome 153.0.8010.48, Windows, 1440x1024 CSS pixels at device scale 2; PNG headers confirm **2880x2048**. Captures are **viewport-only**, using captureBeyondViewport=false, without stretching or CSS changes. This is the PDF-based comparison convention, not recovered Figma metadata. The runner also captured a 390x844/scale-1 narrow viewport for smoke testing only.

The shell, selected Library navigation, title/search/Studio control, storage table, attention blocks, project table and footer follow page 1's composition. The project breadcrumb/reference description, matrix, expanded HDD-017, offline HDD-023, registration/review actions and selected-file inspector follow page 4. Main content remains readable at the comparison viewport. The 200 unresolved count is visible inside its cell, and geometry assertions show it stays within the matrix and outside the inspector. The permitted clipping fix does not hide another category or change the denominator.

Nonblocking visual differences: substitute Arial/system and Consolas typography; simplified/missing status and checksum-copy icons; small column, padding and panel-height differences; different offline-note wrapping; additional Library content-inspection disclosure; visible keyboard focus treatment. These do not materially undermine this provisional composition. Narrow tables scroll within their panels rather than establishing a new approved mobile design. Teal/light remains provisional. Pages 2/3 are alternatives, not disclosure screens. Find, other workspaces, unshown dialogs and additional mode layouts have no supplied design and are not blockers for the bounded alignment. No pixel-perfect, formal accessibility or owner aesthetic acceptance claim.

## Dispositions

| Area | Disposition |
|---|---|
| Reference alignment | No material blocker in supplied page-1/page-4 slice; substitutions and clipping correction acceptable for owner review |
| Interactions and data semantics | **R1 requires correction**; other exercised navigation, filtering, selection, disclosure, demo and category behavior passed |
| Preview isolation | No material blocker found in complete source flow, observed page requests and bounded negative probes |
| Documentation and evidence | Candidate/reference identities, launch instructions and author/reviewer separation supported; historical reports preserved. Existing browser coverage misses R1 |

Photography NAS remains expressly user-selected, with 1,000 expected paths. HDD-017 retains 994/4/2/0 and excludes 12 destination-only files from the denominator. HDD-023 retains older observations, 800 matching, dashes for unknown absent/differing and 200 unresolved. Inspector paths, dates, sizes and abbreviated checksums are synthetic static reference data. No mapping, verification or authoritative-original algorithm was inferred. Registration and inspector controls explain their demo-only state and do not claim a real operation. The instructed source wording "Review 6 unresolved items" remains a documented ambiguity, not an implementation category defect and not a second blocker.

## Consolidated blocker

### R1 / P2 - Skip to content changes workspace instead of focusing its content

Source anchors: [index.html:11](../../../scripts/gui-preview/index.html:11), [app.mjs:113](../../../scripts/gui-preview/app.mjs:113), [app.mjs:155](../../../scripts/gui-preview/app.mjs:155). The skip link targets `#main`; the hash router treats that fragment as an unknown route, defaults to Library at line 116, re-renders, and focuses its h1.

Reproduction on the unchanged candidate:

1. Launch the preview and open Smith Wedding through the Library, or navigate to Find.
2. Focus the visible-on-focus Skip to content link and activate it with Enter. Reviewer probes programmatically focused the real link, then dispatched actual Chromium Enter key events; screenshots record its visible focus state.
3. Observe that the workspace changes to Library, location.hash becomes `#main`, and focus lands on `H1`, not `main`.

Expected: bypass navigation while keeping the current workspace/content/state and moving focus to the main content. Actual: both project and Find are replaced by Library. This makes the supplied keyboard shortcut perform an unintended navigation; it is material to the usable keyboard navigation requirement. It is present in the cumulative scaffold, not necessarily introduced by alignment.

Evidence: `E\browser\reviewer-probes.json` records both before/after states. `skip-before-smith-wedding.png`, `skip-after-smith-wedding.png`, `skip-before-find.png`, `skip-after-find.png` show the visible change. Both probes expected the original title and main focus and failed. Existing browser tests check heading focus for ordinary navigation, not the skip-link action; their 48 passes do not cover this behavior.

Smallest complete correction: handle the skip action separately from workspace routing, focusing/scrolling the existing main without replacing the current route or rendering Library. Add focused browser regressions for Library, Smith Wedding and Find that activate the visible skip control via keyboard, retain route/content/selection, and check destination focus. Preserve ordinary hash navigation and back/return behavior. Do not redesign the shell or change data categories. No correction was made in this review.

## Source and execution evidence

Complete preview flow inspected: server.mjs imports only Node HTTP/file-read/URL helpers, preloads exactly four assets relative to itself, binds 127.0.0.1, validates Host, refuses non-GET/HEAD methods, and performs exact Map lookup on req.url. It has no arbitrary filesystem route, proxy, backend import or operation endpoint. HTML loads local stylesheet/app only. app.mjs imports fixtures.mjs only; fixture data and reference rows are literals with in-memory UI state. Text uses textContent/createTextNode; SVG paths are fixed literals. No fetch, XMLHttpRequest, WebSocket, browser storage, clipboard, device or production adapter is wired into the application. Browser/Node test scripts are not served.

Fresh page request log contains only `http://127.0.0.1:43425/`, `/style.css`, `/app.mjs` and `/fixtures.mjs`, including reviewer demo/search/skip actions. No console/runtime errors were recorded. This observed request log, exact route implementation and active negative probes jointly support isolation; console silence alone would not. Reading packaged preview assets is expected and does not contradict the archival-data demo notice.

The existing definitions were enumerated in `check-inventory.txt` before execution. Exact commands are in `commands.txt`:

| Fresh reviewer execution | Results | Evidence |
|---|---|---|
| Unchanged Node suite: `node --test --test-isolation=none scripts/gui-preview/preview.test.mjs` | 7 top-level pass, 0 fail/skip; no subtests; exit 0 | node-tests.txt |
| Established browser checks executed from copied runner with appended reviewer observations | 48 assertions pass | browser/browser-results.json |
| Additional reviewer browser probes | 3 pass, 2 fail; both failures reproduce R1 | browser/reviewer-probes.json |
| `node --test --test-isolation=none E\negative-probes.mjs` | 2 top-level pass, 0 fail/skip; no subtests; 19 raw HTTP requests; exit 0 | negative-probes.txt; negative-request-results.json |
| All five repository .mjs syntax checks; tracked diff whitespace; explicit preview-file trailing whitespace | Pass | executed-checks.json; whitespace.txt |

The reviewer browser harness uses the established CDP setup and 48 assertions without changing copied application files. Appended observations test visible Expert-button behavior, inert search markup, disclosure persistence across return navigation, and the two skip-link cases. It deliberately records probe failures without aborting cleanup, so process exit 0 / completed=true does **not** mean the independent probes all passed. The verdict uses their recorded 3/2 outcomes. Screenshots and request records remain attributed to this review execution.

Independent server probes used synthetic sentinel files beside and one level above the copied preview. Raw traversal, encoded slash/dot, backslash, double-slash, .git/reference/source names and query paths were refused with 404 without returning sentinel content. HEAD of the allowed page returned 200 and no body; mutation methods returned 405; foreign Host returned 403. Sentinel hashes remained unchanged. No real secret, production directory or unrelated service was probed.

Selected Node v24.19.0; no installation, upgrade or global changes. CLI PID 16272 served ephemeral loopback port 43421, then was terminated and waited, signal SIGTERM / exit null. Browser PID 17448 and its review server were stopped/waited/closed; recorded browser signal SIGTERM / exit null. Only review-launched processes were touched. The browser harness has a 90-second deadline and CLI tests have explicit stop timers. Manual Ctrl+C, physical mobile, screen-reader, backend, full Go safety suite, CI, other-platform, hardware or media qualification was not executed or inferred.

Historical author results remain separate: scaffold 7 Node / 33 browser; latest alignment 7 Node / 48 browser. These overlapping runs are not summed with reviewer execution. A PowerShell evidence-collection command initially stopped when Git's existing line-ending warning was treated as a terminating native stderr error; the read-only whitespace command was then recorded with native exit 0. No file was normalized or changed.

## Preservation and next action

All 251 starting files match their starting working-byte identities after execution. The sole new repository path is this review report. Source/tests/fixtures, all backend and production UI files, living records, historical reports, Figma, supplied ZIP and both PDF copies remain unchanged. Figma stays untracked; ZIP remains ignored by the pre-existing rule. Branch/HEAD remain exact, index empty, no active Git operation and no review-launched preview process remains. Final details and report hash are retained in `E\final-preservation.json`. No stage/commit/push/merge, new branch or implementation edit.

Next action: one bounded correction of R1 and then a targeted recheck; neither is automatically started. Owner visual acceptance and any publication remain separate decisions.

Existing inspection launch from `C:\Users\nsott\source\repos\obelisk`:

```powershell
node scripts/gui-preview/server.mjs 0
```

Open the printed loopback URL. Stop with Ctrl+C in that terminal and wait for the prompt.

## Complete scoped candidate identities

The following are the 14 verified pre-review candidate entries, unchanged by review. The new review report is identified separately in the external final preservation record.

| Path | SHA-256 |
|---|---|
| scripts/gui-preview/app.mjs | edde115de59990cce45271c77374ad4ff4cdbec032a37b9e10f69dfe22c12f5d |
| scripts/gui-preview/index.html | bd3c41bc59bb3c0363b77f8a58495fb36d037902cadf524504ece47d27588a9a |
| scripts/gui-preview/style.css | bd109653e1a17fa75df32973a5115fcf18668cd7d827b50cd3f6d27a5e40dab0 |
| scripts/gui-preview/browser-check.mjs | 431e6d42454dc0b45df28eb31d99b16b321f7afcbd2d719bafcf8adba7b9ea54 |
| scripts/gui-preview/preview.test.mjs | 2e808f9b27c90a2da69a4fc26f3f6135b49b9df02c614223b7f8698e0794b901 |
| scripts/gui-preview/README.md | f36f48d7957766476a7a698fc0f0d59a86dce7b136fc9d3161f50aa8a3da6d6c |
| docs/development/CODEX_HANDOFF.md | 7e7bd1affdb82215d7e298eed0066c830db73da52bd9fc3d098a26fb4f018d49 |
| docs/development/NEXT_ACTIONS.md | f55519c07f1aa870975920a4670a987edf2993d28ee1b911689ab481807840e0 |
| docs/development/OB_STATUS.md | 9b784e840ea952e2fe9d208f88c5f9b6d47e852279c8df92d43be640644b377f |
| docs/development/REVIEW_COVERAGE.csv | fb4264be89127c66701cc9472fbb2e6cdfe6bdfb1df6b7e0ff7b5a009a6508a4 |
| scripts/gui-preview/server.mjs | 16bbff4d0ab8379cb2616282572e04a5f60daaa4e530c1e20ca7f3a37d257984 |
| scripts/gui-preview/fixtures.mjs | 0c1c7901dec126e906352d9759e31627e2b8d173995343387d382f0f3c980628 |
| docs/development/reviews/GUI-PREVIEW-IMPLEMENTATION-2026-09-19.md | 2abee26eef291ee07470bc31e58c581409595332ee788b4d0575a199cad66e2b |
| docs/development/reviews/GUI-REFERENCE-ALIGNMENT-IMPLEMENTATION-2026-09-20.md | 14af791c847d2d17ffbb040f26219ad3df8ab94aa1ce0c541525f0f2f6de6014 |
