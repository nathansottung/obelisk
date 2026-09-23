# Isolated GUI preview implementation - 2026-09-19

GUI_PREVIEW_SCAFFOLD_READY_DESIGN_INPUT_REQUIRED

Uncommitted candidate on `feat/gui-preview`, exact parent and unchanged HEAD `05cf50afc004803e1c0de20a6ce444f6a538c24d`. No staging, commit, push, merge, release or automatic reviewer. This is author implementation/validation in the current Codex session, not owner inspection or independent technical review.

## Authorization and starting evidence

The owner submitted the external attachment `C:\Users\nsott\.codex\attachments\5bd86cb1-47b1-4ba4-b2a1-84600a73b9f7\pasted-text.txt` as this task's instruction. Its bounded local preview authorization controls over historical pending-task text. The older revision-2 handoff was referenced by the prompt; it was not separately supplied/read here. No invented task identifier or design approval.

Starting branch was `fix/obx-004-job-load-safety`. HEAD and the local origin tracking ref both matched the expected evidence SHA. Direct local ancestry is `da22f1d9895d5350142a6f9b05ac41f0a920e170` -> implementation `58711f5d13132246fe048cd8f749cf16d67c1fc1` -> evidence `05cf50afc004803e1c0de20a6ce444f6a538c24d`. The existing external publication receipt at `C:\Users\nsott\AppData\Local\ObeliskDev\obx004-job-publication-20260919-230517\publication-receipt.md` was read and records ACCEPTED_AND_PUBLISHED plus prior live branch equality. No fresh network lookup or publication was performed here. Configuration and job-loading acceptance loops remain complete for their bounded scopes.

Index and tracked worktree were clean; the sole untracked file was `docs/Obelisk.fig`. No merge/rebase/cherry-pick/revert/bisect operation was present. The requested feature branch was absent and was created at the exact parent. Initial sandbox branch creation failed to obtain index.lock; the same explicitly authorized branch command succeeded with filesystem escalation. No reset, stash, clean, fetch, pull or Git setup change.

No applicable AGENTS.md was found in the repository or ancestor chain. Inspected current living records, the coverage ledger, repository contribution/build/style guidance, existing HTML/CSS/script, Go embedding/routing, existing UI/error tests and accepted job report context. `FEATURE_MATRIX.csv` does not exist; no repository-wide matrix or coverage claim was invented. The existing single coverage ledger receives bounded candidate entries.

## Design access and implementation

The selected local file is 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda` before and after. It remains untracked and byte-preserved. No frame exports or screenshots were found in the repository; no supported local Figma inspector was exposed by available tools. No parser installed, file uploaded, third-party authorization flow started or design URL invented. No Figma frame, typography specification or designed interaction state was observed. The file identity is preservation evidence only.

Observed reference: actual `ui/index.html` source, including the dark navigation shell, green/neutral palette, system typography, panels, focus styles and disclosure concepts. The preview uses those conventions provisionally, with the owner's written Library / Back Up / Archive / Find / Activity / Devices & locations / Settings architecture. Rendered preview screenshots were inspected; they are output evidence, not selected-design input. Every page labels the missing Figma reference and synthetic-only status.

Required design input: readable exports of the main shell, Library and Find frames, including selected-detail and Guided/Studio/Expert states where designed, and typography/color/spacing specifications where not visible in the exports. Until those are observed and applied, do not label this candidate Figma-faithful or READY_FOR_GUI_PREVIEW_OWNER_INSPECTION.

| Surface | Actual candidate behavior | Limits |
|---|---|---|
| Shell | Hash navigation, current-page state, heading focus, skip link, responsive layout | Provisional visual layout |
| Library / Find | Search name/path/hash/version/occurrence/medium; combine project/medium/availability filters; reset; select and inspect all known occurrences | Three invented identities, four intentional occurrences, two versions; totals confined to sample |
| Evidence details | Online/offline, dated observations, unknown inventory, verification date/scope, intentional copies, recovery planning text | Static evidence; no real recovery or current-media assertion |
| Fixture presentations | Ready, held loading, empty, error, partial information; no-results via search | Partial view retains sample rows with explicit unknown completeness; Ready simulates retry |
| Disclosure | Guided core details; Studio adds synthetic hash/inventory scope; Expert adds medium identifier | In-memory presentation only; no settings persistence or guarantee/permission changes |
| Back Up / Archive | Navigable recurring/incremental and package/queue/verification outlines | Disabled operations; schedules, retention, encryption and execution deferred |
| Activity | Pending/running/failed/completed examples; operation, recording, verification and recording history shown separately | Static simulations; FAILED primary when recording also fails; current unrecorded differs from recorded with historical errors |
| Devices / Settings | Sample locations and observations; disclosure explanation | Discovery/enrollment and operational settings unavailable |

All strings from fixtures use DOM textContent/setAttribute, never HTML insertion. Offline does not mean lost; unknown is not zero; copying/completion is not verification; enrollment is not copying/moving/reorganizing originals. Critical warnings are present at every disclosure level. Search matches the same occurrence for medium/availability constraints and displays all known sample occurrences of selected content, explicitly labeled as such.

## Isolation and launch

New files live under `scripts/gui-preview/`, outside Go's `//go:embed ui` tree. Existing normal UI, backend, tests and build/dependency files are unchanged. Plain HTML/CSS/JavaScript; preinstalled Node 24.19.0 provides the disposable static server. No framework, package install or build step. Server preloads a four-route asset allowlist; only GET/HEAD, exact loopback Host, no dynamic file lookup, proxy, store imports or API routes. Unknown routes return 404; mutation methods return 405. CSP denies browser connections, external assets, forms and framing. Fixtures/app have no I/O adapter or browser persistence. Validation harness is not served.

PowerShell working directory and launch:

```powershell
Set-Location 'C:\Users\nsott\source\repos\obelisk'
node scripts/gui-preview/server.mjs 0
```

Open the exact `http://127.0.0.1:<assigned-port>/` printed by the terminal. `0` assigns an available port; a requested fixed port (e.g. `4317`) fails if occupied. No public bind. Stop with Ctrl+C in the same terminal and wait for the prompt. Automatic shutdown after 60 minutes. No server is left running by this task. Owner has not yet launched/inspected it. See [launch guide](../../../scripts/gui-preview/README.md).

## Executed validation and limitations

Evidence root: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-preview-20260919-232330`. Initial identities, Figma preservation, raw Node logs, failed and successful browser runs, screenshots and final identities remain local only.

- Final Node command: `node --test --test-isolation=none scripts/gui-preview/preview.test.mjs`: **7 top-level passes, 0 failures, 0 skips, no subtests**. `node-tests-final-v2.txt`. Covers search, combined filters, semantic states, no-I/O source guard, actual server rejection boundary, CLI launch, occupied-port refusal and stop/wait. Final CLI served `http://127.0.0.1:42850/`, PID 17520; explicitly terminated and waited, signal SIGTERM, exit null. This is forced process cleanup, not manual Ctrl+C validation.
- Final real-browser command: `node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-preview-20260919-232330\browser-final-v2'`: **33 browser assertions passed**, separate from Node test counts. Actual browser version and PID/exit/signal in `browser-results.json`. Disposable profile; loopback preview; browser stopped/waited and server closed. Keyboard Enter activation/focus, heading focus, navigation, filters, no-results, safe markup-like filename, empty/loading/error/partial, all disclosure levels, failure precedence and placeholders exercised. Page requests were only local preview assets; no page runtime exceptions or production API/external requests observed.
- Browser viewports: 1365x1000 desktop and 390x844 narrow, device scale 1, Chromium desktop emulation (not physical mobile testing). Screenshots: `library-desktop.png`, `find-desktop.png`, `find-error-desktop.png`, `activity-desktop.png`, `library-narrow.png` under final browser directory. Desktop/narrow output visually inspected. Narrow page overflow check passed. Accessibility tree retained; named searchbox and focus assertions are limited evidence, not a full accessibility audit or screen-reader qualification.
- All five `.mjs` files pass `node --check`. Tracked diff whitespace and explicit new-file whitespace checks recorded in `final-checks.json`. No compilation stage exists for this plain JavaScript preview. No Go/shared UI/error-contract code changed, so no Go suite/build/vet or historic safety test rerun was needed for this isolated surface. No cross-platform, hardware, Docker, race, CI, ACL, crash/power-loss or backend integration qualification is claimed.

Earlier execution is retained, not silently replaced: default Node test isolation failed to spawn under the sandbox (EPERM); a supported in-process test mode was used. Initial six-test run passed. First browser attempt stopped on reused top-level CDP evaluation variable declarations; block scoping corrected the harness. Next browser run passed 30 assertions. Added keyboard check initially omitted the Enter character in the CDP event; corrected harness passed 33 assertions. Added CLI test initially treated signal termination's null exit code as failure; corrected assertion records signal and wait. Product source did not change for these harness corrections. Each browser attempt's cleanup records remain in its own directory; `completed` in final-format results distinguishes incomplete runs. No independent review was performed.

## Final candidate and next action

Added paths: `scripts/gui-preview/app.mjs`, `fixtures.mjs`, `index.html`, `style.css`, `server.mjs`, `preview.test.mjs`, `browser-check.mjs`, `README.md` (all under that directory), and this implementation record. Modified only the four living records: `docs/development/CODEX_HANDOFF.md`, `NEXT_ACTIONS.md`, `OB_STATUS.md`, `REVIEW_COVERAGE.csv`. Exact final SHA-256/byte identities for all candidate paths are in external `final-identities.json`; its digest and all preservation differences are in `final-checks.json` to avoid a self-hash cycle. External initial identities cover the actual published worktree, not an old 238-file staging inventory. No historical report was edited. Final Git snapshot, HEAD/index checks and unchanged Figma identity are retained externally.

Next: owner can inspect the runnable scaffold; supply the design exports above for a later fidelity pass. Separate scoped technical review/publication remains later. PR-04 recovery, underlying Windows tar Unicode compatibility, broader persistence/storage identity/atomicity/concurrency/keystore/ACL, helper/platform integration, streaming/ring-buffer architecture, LTO qualification (LTO-8 first physical target, not generation cap), Blu-ray, main integration and release remain their separate unresolved tracks. None was repaired, re-audited, integrated or qualified by this preview.
