# GUI skip-link R1 correction - 2026-09-20

READY_FOR_GUI_SKIP_LINK_FOCUSED_RECHECK

AUTHOR-ADDRESSED, not reviewer-closed or owner-accepted. One bounded implementation/follow-up in the same Codex conversation/session as the author and substantive review. No separate reviewer or sub-agent was used. The ONE next action is targeted keyboard-navigation recheck of R1 and directly affected controls; that reviewer step has not been performed.

## Checkpoint and provenance

Existing branch `feat/gui-preview`; unchanged HEAD and overall GUI patch base `05cf50afc004803e1c0de20a6ce444f6a538c24d`. The full preview is uncommitted and absent from a HEAD-only checkout. The controlling [substantive review](GUI-REFERENCE-ALIGNMENT-REVIEW-2026-09-20.md) remains unchanged, including its NEEDS_CHANGES verdict and actual R1/P2 finding. Both GUI implementation reports, current living records and contribution instructions were consulted. No repository AGENTS.md was found; earlier recorded ancestor inspection also found none. No completed correction existed on entry.

Submitted instruction: `C:\Users\nsott\.codex\attachments\49bbb1a2-89c1-438f-8c40-5c27ddf57b83\pasted-text.txt`, retained byte-for-byte as `submitted-prompt.txt`, SHA-256 `0b785c87c4285affbdbc271b2ef2b787d306357e96fd6983cfb64ab1b549b3be`. Historical prompts were not replayed.

Evidence root **E**: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-skip-fix-20260920-005527`.

Verified the actual prior review's `before.json`: all 251 entries matched, with no drift. Added the existing controlling review to this task's starting inventory: 252 paths. Its hash remains `32b5f42cbeec526e8adb5c4a1f03f05664213580661466592d54003985c6aa2c`. `checkpoint.json` records full branch/HEAD/status, relevant untracked files, empty index and absent merge/rebase/cherry-pick/revert/bisect markers. The ignored ZIP is explicitly inventoried; this is preservation evidence, not a staging allowlist.

Before editing, copied all eight actual runnable preview source/test/README files into `E\pre-fix`. `pre-fix-identities.json` records their byte sizes and hashes; manifest SHA-256 `d26166094dd0eef393a2968fac8492ed11a07603ea58264870240d714ab82ea4`. Original app hash: `edde115de59990cce45271c77374ad4ff4cdbec032a37b9e10f69dfe22c12f5d`; browser harness: `431e6d42454dc0b45df28eb31d99b16b321f7afcbd2d719bafcf8adba7b9ea54`. The later additional probe files are separate from these preserved originals.

## Root cause and delta

The real link at [index.html:11](../../../scripts/gui-preview/index.html:11) has href `#main`. The actual hash router at [app.mjs:113](../../../scripts/gui-preview/app.mjs:113) defaults unknown fragments to Library, replaces content, and focuses h1 on hashchange. The review's original retained `browser/reviewer-probes.json` and source were read. There is no delegated anchor interceptor that already separates focus handling from routing.

Added five lines at [app.mjs:155](../../../scripts/gui-preview/app.mjs:155): a listener on this skip link alone prevents its fragment navigation and focuses the current `main`. Main already has `id=main` and `tabindex=-1`; existing CSS supplies visible keyboard focus. The listener queries the destination at activation, does not retain a rendered child, and does not call render or mutate application state/history. Keyboard Enter and ordinary pointer activation share the native link click handler. No URL scheme, router, HTML, CSS, data or layout change.

Integrated four keyboard cases and three navigation assertions in [browser-check.mjs:56](../../../scripts/gui-preview/browser-check.mjs:56). Each focuses the first real primary-navigation link for deterministic setup, then sends actual Shift+Tab to reach the real skip link and Enter to activate it. Neither direct skip/main focus nor a test-created replacement handler is used as activation evidence. The test observes focus-visible and on-screen link geometry, then records before/after route, history length, performance time origin, workspace text, control values, selected result/file, disclosure and active focus. It also verifies the same first content node survives, catching accidental rerendering even when text matches.

Find uses query `SIM-OCC-03`, one visible selected v1 result and occurrence details. Smith Wedding uses query `IMG_4587`, the existing selected file, and an explicitly collapsed/reopened HDD-017 disclosure. A fourth Find case repeats after ordinary navigation and the Expert-mode rerender. After skip, Tab reaches Guided for Library/Find or the Library breadcrumb for the project; Shift+Tab can return to Settings, then Tab advances again without trapping focus. Separate normal project-link Back/Forward and breadcrumb checks pass. Existing search, selection, disclosure, demo and navigation checks remain in the suite. No new ordinary-navigation state-persistence promise is made.

Follow-up delta: only `scripts/gui-preview/app.mjs`, `scripts/gui-preview/browser-check.mjs`, the four existing living records (CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md, REVIEW_COVERAGE.csv), and this new report. The overall GUI candidate additionally contains the other six original preview files, prior implementation/review reports and earlier living-record changes. It must not be confused with this two-code-file correction.

## Execution now, separate from historical evidence

Installed tools: `C:\Program Files\nodejs\node.exe`, Node v24.19.0; `C:\Program Files\Google\Chrome\Application\chrome.exe`, Chrome 153.0.8010.48 (CDP reports the same). No installation, platform switch or external asset request. Definitions were enumerated in `check-inventory.txt` before execution.

The new pre-fix probe uses the established browser setup against the byte-preserved preview. It is new regression evidence, not an exact replay of the historical reviewer harness. Initial `pre-fix/skip-probe.mjs` exited 1 after a test-only top-level JavaScript `const q` scope collision; cleanup ran, and `before-browser/browser-results.json` retains incomplete status. The probe was corrected by block-scoping setup evaluations and rerun as `skip-probe-v2.mjs`, SHA-256 `5550557e985650bc6e1ab716546bd0b17c509a7b5afc17fe80d19e814e657ddd`. No application bytes changed between these runs.

| Execution | Actual outcome |
|---|---|
| Pre-fix keyboard cases, `before-browser-v2/skip-link-results.json` | 0 pass, 4 fail; all four reach the visible link; Find and Smith Wedding reset to Library/#main/H1, including repeated Find |
| Pre-fix Library control | Title stays Library, but URL/history changes, content node is replaced and main is not focused; therefore the stronger preservation check fails rather than being reported as a passing control |
| Corrected Node suite, `node-tests.txt` | 7 top-level pass, 0 fail/skip/cancelled, no subtests; exit 0 |
| Corrected full browser suite, `after-browser/browser-results.json` | 55 checks pass, 0 fail; no skipped checks; exit 0. Includes original 48 plus 4 keyboard cases and 3 navigation assertions |
| Corrected keyboard detail, `after-browser/skip-link-results.json` | All 4 preserve state/URL/history/node and focus main; all forward/reverse Tab controls pass |
| Syntax and whitespace | app.mjs and browser-check.mjs syntax pass; tracked diff and changed preview trailing-whitespace checks pass |

Pre-fix probe deliberately logs all four failed case outcomes without throwing, allowing screenshots and cleanup. Its inherited console says "9 browser checks passed" because the four nonthrowing case labels are pushed alongside five successful supporting checks. That console line is not the verdict: the recorded case booleans are 0/4. The corrected durable harness asserts each result and fails the process on failure.

Exact commands, from the repository root (`E` denotes the absolute evidence root above):

```powershell
node E\pre-fix\skip-probe.mjs "C:\Program Files\Google\Chrome\Application\chrome.exe" E\before-browser
node E\pre-fix\skip-probe-v2.mjs "C:\Program Files\Google\Chrome\Application\chrome.exe" E\before-browser-v2
node --check scripts/gui-preview/app.mjs
node --check scripts/gui-preview/browser-check.mjs
node --test --test-isolation=none scripts/gui-preview/preview.test.mjs
node scripts/gui-preview/browser-check.mjs "C:\Program Files\Google\Chrome\Application\chrome.exe" E\after-browser
git diff --check
```

Full executable paths are retained in `E\commands.txt`. Corrected Node/browser suites ran once, with no post-pass runtime changes or suite repeats. PowerShell identity-record collection initially had a pipeline parse error (nothing executed); its corrected evidence-only command succeeded. No additional server-security campaign or Go suite was run. Existing Node isolation cases still pass; browser request records contain only the loopback page and its three packaged assets, with no runtime errors.

Historical layers, not summed: alignment author 7 Node/48 browser; original substantive review 7 Node/48 browser plus separate probes 3 pass/2 fail and 2 isolation-test passes; duplicate-review turn only verified identities and reran no tests. This report's validation is author follow-up evidence, not a closing review.

## Screenshots, identities and preservation

New viewport-only PNGs at 1440x1024 CSS pixels/scale 2 (2880x2048): `after-browser/skip-link-{library,find,smith-wedding,find-rerender}.png` and corresponding `skip-destination-*.png`. Find and project focused-link/destination pairs were visually inspected: visible shortcut outline becomes the existing main outline, with workspace/query/selected evidence intact. HTML/CSS are byte-unchanged; no unintended composition change was observed. The existing runner also captures its usual Library/project/narrow images; this task did not repeat the full Figma comparison. Pre-fix images remain under `before-browser-v2` with the same names.

Corrected app SHA-256: `f4b555e851406f17c7dd6a6d01759ae352edde7b05b4f2ad01586261a252db3c` (25755 bytes). Corrected browser harness: `e04540cbf378ab486e1901582c862aaf849e59c30d1a6896169820b22ff3e1c8` (22576 bytes). `candidate-identities.json` records all eight preview files and four living records; `final-identities.json` includes this report and preserved historical reports. `probe-identities.json`, `screenshot-identities.json`, `follow-up-source.diff` and `final-preservation.json` identify evidence and distinguish authorized changes from preserved files.

All seven extracted reference entries were rehashed against the retained actual reference manifest: seven matches, recorded in `references-final.json`. Figma, original ZIP, root PDF and package PDF/PNGs remain unchanged; no extraction, decoding, upload or media work. Final preservation compares all 252 starting paths: six authorized existing paths changed, 246 preserved, no missing files; this report is the sole added repository path. Backend, production UI, fixtures, server, original tests, HTML/CSS/README and all historical reports are preserved. Branch/HEAD unchanged, index empty, no Git operation markers. The original .fig stays untracked and ZIP remains ignored by its pre-existing rule.

Node CLI PID 17952 served loopback port 43647 and was terminated/waited (SIGTERM, exit null). Corrected browser PID 15484 and loopback server port 43649 were stopped/waited/closed. Both pre-fix attempts also closed their task processes; all result records retain cleanup status. No unrelated process was stopped. No staged content, commit, push, merge, branch operation, production integration or publication.

Launch for the targeted recheck: `node scripts/gui-preview/server.mjs 0`; open the printed loopback URL, Ctrl+C and wait for the terminal prompt to stop. Manual Ctrl+C, pointer-specific regression, screen readers, accessibility-standard certification, other browsers/platforms and owner visual acceptance are not claimed. Provisional teal/light styling, font/icon substitutions, clipping correction and unshown-screen limitations remain unchanged. Prompts 10/11 remain gated.
