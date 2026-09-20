# GUI reference alignment implementation - 2026-09-20

GUI_REFERENCE_ALIGNMENT_READY_FOR_REVIEW

Author implementation and validation in the current Codex session. Owner visual review has not occurred. Existing branch `feat/gui-preview`; unchanged HEAD/backend parent `05cf50afc004803e1c0de20a6ce444f6a538c24d`. No new branch, staging, commit, push, merge, automatic review or backend repair.

## Provenance and inputs

Controlling instruction: owner-submitted `C:\Users\nsott\.codex\attachments\a3f01766-e45c-4c29-9e29-e7e7bf747027\pasted-text.txt`, retained byte-for-byte as `submitted-prompt.txt` in the evidence directory below, with hash in the final manifest. The package's older `08_CONTINUE_GUI_FROM_PDF_REFERENCES.md` was hash-verified as package content, not executed as another task. Prior scaffold and safety reports are preserved. The prior scaffold report/living records and repository contribution/style guidance were consulted; no AGENTS.md was found by repository search. The supplied ZIP is new authorized reference input, not a reason to recreate the candidate.

Evidence directory: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-alignment-20260920-001358`.

Input: `docs/OBELISK_Readable_Design_References.zip`, SHA-256 `72a1f3308e2c4421b62c8bf286f9fab7651393a1dbbbbe1520774feefb331aca`, matching the owner-provided hash. The existing `.gitignore` line 10 already ignores `*.zip`; no ignore rule changed. Original retained in place, untracked. Original Figma remains 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda`, untracked. Root `Untitled.pdf` was also present on entry and preserved; its hash matches the package PDF. No .fig decoding or attachment-folder search was repeated.

Built-in .NET ZIP enumeration and extraction wrote only to the new `references` subdirectory. Each destination was resolved and required to stay beneath that directory; directories created separately, per-entry size capped at 50 MB, no overwrite extraction. Eight archive entries recorded in `zip-entries.json`. README and manifest read. All seven files listed in the manifest match their SHA-256 hashes, including all four PNGs and the PDF. No embedded content executed or uploaded.

| Page / file | SHA-256 | Actual use |
|---|---|---|
| 1 / `01-library-teal-2x.png` | `28f5feb46e5d42af39c44494bc7e3533bc7c29c32328f6d6497c099ee0e00a9d` | Visually inspected; shell, Library header/search/modes, storage, attention, projects and demo footer |
| 2 / `02-library-blue-light-sidebar-2x.png` | `16e7102c47c0568e210d95dd9a2481c641e12733f695cbceb82bf18803b3845d` | Visually inspected alternative; not implemented or blended |
| 3 / `03-library-blue-dark-sidebar-2x.png` | `51ba559725307f9efbd47d9ce02314692c62e9de12a7e5fc80bb36b3aeef3e1f` | Visually inspected alternative; not a disclosure state or implemented theme |
| 4 / `04-project-comparison-evidence-inspector-2x.png` | `537b6e6fd720acb0ae5992c4f665123a8b16fc6119caf95c8a86c94807f095f5` | Visually inspected; Smith Wedding comparison and selected-file inspector |
| `source/Untitled.pdf` | `3c17b2b53335c2cf1e5d69bd616e4d6e3fa4613cf0dad0dfb21b0b786c8b7a86` | Preserved source export, 3698553 bytes; PNGs used for visual comparison |

These are readable full-page renders, not the previously recovered 400x187 thumbnail. Filenames are package-assigned descriptions, not recovered Figma node names. Pages 1 and 4 supply a provisional reversible teal/light baseline, not owner approval of a final theme. All three Library alternatives show Studio selected; no theme selector was created.

## Implemented behavior and boundaries

Plain HTML/CSS/JavaScript retained in the existing preview entrypoint. New screens use semantic tables, headings, buttons, links, search fields and locally drawn inline SVGs; no screenshot background or hidden click map. No framework, dependency or asset download. Existing `server.mjs` and fixture adapter remain byte-identical. Server still allows only four fixed static assets, GET/HEAD, exact loopback host, with CSP denying connections/forms/external assets. No reference ZIP/PDF/image, repository file or evidence file is served.

- Library reproduces the four storage rows, dates/capacities/statuses, attention blocks, three projects, Studio mode control, search and footer. Storage selection and inventory search work in memory. Photography NAS is initially highlighted. The original synthetic content inspector remains available through an explicitly additional disclosure below the aligned surface; Find retains its original fixture search/filter/selection and empty/loading/error/partial cases.
- Smith Wedding is reachable from the project name, Fix issues and Compare copies. Its Library breadcrumb returns to the Library. This hash-route wiring is an implementation choice, not a recovered Figma prototype. Other project choices and operational-looking controls return explicit demo-only explanations. No registration, scan, verification, comparison, deletion, clipboard operation or device discovery is performed.
- Photography NAS is explicitly a user-selected comparison reference with 1,000 expected paths. HDD-017 keeps 994 matching, 4 absent, 2 differing, 0 unresolved; 12 destination-only files stay outside that denominator. Expanded explanatory details can be collapsed/reopened. HDD-023 stays offline with 800 matching, dashes for absent/differing and 200 unresolved, with the older Aug 3, 2025 scan. No unknown is converted to zero or older inventory presented as a fresh verification.
- The source action text `Review 6 unresolved items` is preserved. Its demo response explicitly notes the source ambiguity versus the matrix's narrower zero-Unresolved category and moves focus to the inspector. No recategorization algorithm was invented.
- Inspector retains IMG_4587.CR2, both physical roots, 28.4/28.1 MB, Sep 12/Sep 15 2025 observations and the displayed abbreviated checksums. Project search filters this one shown fixture and gives an explicit no-results state. Operational-looking buttons explain their demo-only status without changing evidence. Static roots do not imply an implemented path-mapping algorithm; the app does not choose the newest or delete either copy.
- Studio defaults to the visible reference selection. Existing disclosure behavior remains presentation-only; mode buttons retain core evidence and warn that unshown layouts are unspecified. Find is explicitly marked visually unspecified. Back Up, Archive, shared utilities, dialogs and extra disclosure/responsive designs are not newly claimed designed screens.

## Visual comparison and deliberate differences

Compared actual Chrome screenshots with page 1 and page 4 at 1440x1024 CSS pixels and device scale 2, yielding 2880x2048 PNGs. This is the package's PDF-based comparison assumption, not recovered Figma viewport metadata. Final screenshots are under `browser-final-v2`:

- `library-reference-aligned.png`
- `project-reference-aligned.png`
- `find-occurrences.png`, `find-desktop.png`, `find-error-desktop.png`, `activity-desktop.png`
- `library-narrow.png` at 390x844, scale 1; smoke evidence only, not responsive-design approval.

Visual inspection compared full-page proportions, sidebar/header positions, panel/table columns, selected storage treatment, inspector content and footer. First rendering showed excessive table/inspector height, incorrect table-column distribution and right-aligned expanded notes caused by the shared last-cell rule. Those were corrected and screenshots recaptured. The final layout follows the supplied composition and preserves readable source content. No pixel-perfect claim.

Deliberate clipping fix: page 4's rightmost offline count overflows toward the inspector. The implementation sizes the matrix columns and keeps `200` inside its own cell and panel; browser geometry assertions verify separation from the inspector. The reference PNG is untouched. Line wrapping of the offline note differs.

Remaining differences: Arial/Segoe UI and Consolas substitute for unprovided authoritative font assets. Locally drawn SVGs and plain category counts substitute for some source icons; checksum copy icons are omitted rather than wiring clipboard behavior. Small type-weight, padding, table-height and footer-position differences remain. The additional original Library inspection disclosure and demo status responses are implementation additions. Keyboard focus outlines intentionally remain visible during interaction; the comparison screenshot clears heading focus after separately asserting it. At narrow widths tables scroll within their panels; no new mobile design is claimed. No automated image-similarity score or full accessibility certification.

## Checks actually executed in this task

Selected tools: Node v24.19.0; installed Chrome 153.0.8010.48 on Windows. No installation or upgrade. Current results are separate from the historical scaffold's 7 Node tests / 33 browser assertions.

`node --test --test-isolation=none scripts/gui-preview/preview.test.mjs`: 7 top-level passes, 0 failures/skips, no subtests. Final exact-byte log `node-tests-final.txt`; earlier current-task run retained as `node-tests.txt`, also 7 passes. Existing tests exercise search/filter/state semantics and no-I/O guards; server rejection cases now explicitly include ZIP/PDF/reference/evidence paths; CLI still launches on an ephemeral loopback port, refuses an occupied port and is stopped/waited. Final CLI PID 17176 served port 43336; earlier PID 296 served port 43260. Both received forced SIGTERM cleanup and were waited, exit null; not a manual Ctrl+C test. These repeated runs are not summed.

`node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-alignment-20260920-001358\browser-final-v2'`: 48 browser assertions passed, no page runtime/console errors, no production or external page requests observed. Checks include navigation/return, keyboard focus, storage/project searches, exact counts/dates, category ambiguity response, HDD expansion, inspector demo actions, original Library inspection, all prior Find behavior, modes, placeholder navigation and narrow page overflow. Accessibility tree retained with named searchbox evidence. Browser PID/version/exit/signal and successful stop/wait/server close recorded in `browser-results.json`.

Earlier current-task browser runs are retained: `browser-1` stopped at an old navigation-focus assertion because Find was already active; the harness was corrected to navigate away and back. `browser-2` passed 45 assertions after layout corrections. `browser-final` passed 48 assertions; `browser-final-v2` reran those after category-symbol substitutions and footer SVG cleanup. These overlapping executions are not summed or relabeled as independent review. Browser source records each assertion by name. All runs use a new disposable profile and a 90-second deadline; launched processes were stopped/waited.

All five JavaScript modules passed `node --check`; final whitespace/new-file checks and exact identities are in `final-checks.json`. No Go/backend/shared production UI source changed; no full Go suite, build/vet, safety review, CI, hardware, race, Docker, other-platform runtime or media qualification was run or inferred.

## Candidate, preservation and launch

This alignment modifies six existing preview files: `scripts/gui-preview/app.mjs`, `index.html`, `style.css`, `browser-check.mjs`, `preview.test.mjs`, `README.md`. It appends current status to the four living records and adds this report. `fixtures.mjs`, `server.mjs`, all backend/production UI/dependency files, historical reports, original Figma, source PDF and reference ZIP remain unchanged. Existing uncommitted scaffold stays in place. No staging/commit/push or Git setup change.

`before.json` records actual entry identities, including the pre-existing untracked PDF and scaffold. The ZIP was already ignored and is verified separately. `final-identities.json` records the whole final preview candidate and living/report paths; `final-checks.json` records manifest digest, input hashes, source-reference verification, preservation differences, HEAD/index and process checks. Exact candidate copies and the actual submitted prompt are retained externally. No self-hash cycle is introduced in this report. Raw evidence and source references are local only, not authorized for publication.

From `C:\Users\nsott\source\repos\obelisk`:

```powershell
node scripts/gui-preview/server.mjs 0
```

Open the exact printed `http://127.0.0.1:<assigned-port>/` URL. Port 0 selects an available port; a requested occupied port fails without disturbing its listener. Stop with Ctrl+C in that terminal and wait for the prompt; the existing 60-minute automatic shutdown remains. No preview server is left running by validation.

Next action: owner visual review of this bounded uncommitted alignment. No automatic technical review, publication, production integration or next workstream.
