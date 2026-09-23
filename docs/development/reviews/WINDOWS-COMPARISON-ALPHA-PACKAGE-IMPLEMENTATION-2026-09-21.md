# Windows comparison alpha package implementation — 2026-09-21

Status: **WINDOWS_COMPARISON_ALPHA_PACKAGE_READY_FOR_REVIEW**. This is an
uncommitted, unstaged packaging candidate, not package acceptance or publication.

## Authorization and checkpoint

The owner submitted Prompt 28 from attachment
`9b78ce0e-4e90-4e19-aeeb-e3a21763346a/pasted-text.txt`. Its verbatim copy is
`submitted-prompt.txt` in the external task directory below. Prompt 28 is a
conversation label, not a repository change ID. No historical prompt was executed.

Verified starting branch `feat/gui-recorded-snapshot-comparison`, clean index and
tracked worktree, no Git operation in progress, and HEAD/parent
`e3d9bef998a20dff78dc67463dfb8f848aad76ce`. Its comparison implementation parent is
`87b4fcf3443bf18bd64671896f911c73dcbdc22a`. Reused the identifiable publication
receipt at `gui-recorded-comparison-publish-20260921-172333/publication-receipt.md`
under ObeliskDev; no network lookup or repeat push. Created
`feat/windows-comparison-alpha-package` at that parent. HEAD remains unchanged.
`Untitled.pdf`, `docs/Obelisk.fig`, and the ignored readable-design ZIP remain
in place with unchanged bytes. No accepted runtime source was modified.

Evidence root (abbreviated **T** below):

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-alpha-20260921-175648`

`checkpoint.json`, `starting-identities.json`, and `starting-source/` retain the
entry checkpoint. `candidate-identities.json` identifies the final changed/new
source files without a self-referential hash; `candidate-source/` retains those
bytes. `final-preservation.json` records the final preservation checks and manifest
hashes. All build outputs, inputs, logs and screenshots remain outside the repo.

## Package and provenance

New unsigned archive, built once and tested after extraction:

`T\build\0.9.0-dev-comparison-package.e3d9bef998a2-windows-amd64-local.zip`

- ZIP: **14,431,445 bytes**, SHA-256
  `33d1d9e5087c35aca23606957777f3d13ff8267de798c9848c79ada4697f00d2`.
- Package manifest SHA-256:
  `9ab8ac95a56cc16686b778070667dbd180a58a5062e21e6699e0c7fb7196c28c`.
- Fresh `bin/obelisk.exe`: 14,283,264 bytes, SHA-256
  `a7e179bfbf6bac7d8f013058b954e0a2195e605b0f64c5fb77d2b046517feb2f`.
- Runtime source is committed `e3d9bef998a20dff78dc67463dfb8f848aad76ce`;
  packaging/launcher/tutorial inputs are separately hashed uncommitted files.
  The source commit alone does not identify this packaging candidate.
- `build/artifact.json`, `build/build-source.json`,
  `build/stage/package-manifest.json`, `build/binary-build-info.txt`,
  `build/commands.json`, command output files and `build-invocation.json` retain
  exact source/dependency identities, commands, flags, tools and results.

Selected retained Go 1.26.8 windows/amd64 explicitly from
`ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\go.exe`;
Node v24.19.0 x64 from `C:\Program Files\nodejs\node.exe`. Fresh native build used
the existing cached modules, isolated cache/temp, GOTOOLCHAIN=local, GOPROXY=off,
GOSUMDB=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0, GOOS=windows and GOARCH=amd64.
No installation, download, global setting change or source-runtime repair.

The explicit 22-entry allowlist is:

```text
bin/obelisk.exe
preview/server.mjs
preview/stop-command.mjs
preview/catalog-adapter.mjs
preview/index.html
preview/app.mjs
preview/fixtures.mjs
preview/style.css
preview/catalog-ui.mjs
preview/catalog-protocol.mjs
preview/catalog-names.mjs
preview/recorded-comparison.mjs
preview/comparison-ui.mjs
Launch.ps1
launcher.mjs
tutorial-fixtures.mjs
QUICKSTART.md
SUPPORTED.md
BUG-REPORT.md
LICENSE
THIRD-PARTY-NOTICES.txt
package-manifest.json
```

Both comparison modules are taken from the same committed runtime as the server,
reader and GUI. No new third-party asset/dependency was introduced. Existing Go
and linked dependency notices are retained. No Node/browser, design exports,
historical catalogs, test evidence, .git, caches or keys are packaged. The native
binary retains its existing broader command surface: this wrapper is convenience,
not a security sandbox. Build information is provenance, not signing or security
certification. No reproducibility claim was inferred from this single build.

Original accepted inventory ZIP remains distinct and unchanged:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225\build-complete\0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`

SHA-256 `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`,
verified at entry and closeout. Its reviewed source-manifest identity
`60ea696a91f4c467b543b381a93e6294813ed5030e557386722e55b1de6713a0`
is historical provenance, not the source identity of this new package. No old
staging, extracts, manifests, review reports or owner workspaces were modified.

## Bounded implementation and tutorial

`launcher.mjs` now forwards one or two fixed absolute catalog paths, in argument
order, to the existing repeated `--catalog` server contract. Missing/relative/
duplicate paths, more than two inputs, bad options and missing package components
refuse explicitly. Duplicate artifact bytes at distinct paths refuse through the
accepted reader/session contract. The existing single `generate`, `inventory`,
single `view`, `check` and explicitly chosen `static` paths remain available.
No-argument help remains inert. No browser path picker or automatic comparison.

`generate-pair` creates a new ALPHA/BETA workspace only. Each side retains the
10-file international/punctuation/empty/equal-content/.DS_Store control set plus
one side-only file. BETA changes the contents of `nested/O'Brien & +%# note.txt`.
Native inventories observe those actual new files; no snapshot rewriting,
backdating or copied runtime catalogs. Explicit BETA ON inventory excludes only
the two regular exact `.DS_Store` files and preserves the near-name/directory
controls and all source files. Independent expected table:

| Inputs/reference | Agreement | Difference | Inconclusive | Reference only | Counterpart only | Union |
|---|---:|---:|---:|---:|---:|---:|
| ALPHA OFF / BETA OFF | 9 | 1 | 0 | 1 | 1 | 12 |
| ALPHA OFF / BETA ON | 7 | 1 | 0 | 3 | 1 | 12 |
| BETA ON / ALPHA OFF | 7 | 1 | 0 | 1 | 3 | 12 |

Ordinary producer fixtures supply no inconclusive pair; zero is the expected
result, not fabricated incomplete evidence. Equal-content distinct paths stay
distinct. Recorded agreement does not establish present source state;
only-recorded does not establish physical absence. Scope and observation time
remain per input. Reference/root acceptance remains an explicit UI action.

From a new extracted package in Windows PowerShell (Windows x64, Node 24 x64,
PowerShell 5.1 and a browser; no Go/Git/checkout needed):

```powershell
.\Launch.ps1 check
$pair = Join-Path $env:LOCALAPPDATA 'ObeliskDev\Comparison tutorial 01'
.\Launch.ps1 generate-pair $pair
$alpha = Join-Path $pair 'ALPHA'
$beta = Join-Path $pair 'BETA'
.\Launch.ps1 inventory $alpha off.json
.\Launch.ps1 inventory $beta off.json
.\Launch.ps1 inventory $beta on.json --ignore-ds-store
$a = Join-Path $alpha 'catalogs\off.json'
$b = Join-Path $beta 'catalogs\off.json'
.\Launch.ps1 view $a $b
```

Choose Compare recorded snapshots, choose the reference, inspect the root
alignment, then explicitly run. Type `stop` and Enter and wait for the reader and
packaged-child exit messages and prompt return. Reopen with the same `view $a $b`
command without generation/inventory. `view $a`, `view $b`, `view $b $a`, and
`view $a (Join-Path $beta 'catalogs\on.json')` exercise the other documented paths.
Use a different new workspace name if the tutorial directory already exists.
The shipped quickstart contains the complete sequence, scope qualifications,
limits, immutable-output rules, compatible-reader requirement and UNKNOWN legacy
scope. Supported/unsupported and sanitized-feedback documents were updated.

## Execution on this archive, not historical totals

Selections were retained before execution in `test-selection.txt` and
`browser-selection.json`. Both independent .NET extracts match all manifest
entries: `T/rehearsal/extracted-one` and
`T/rehearsal/relocated café O'Brien & +%#`. Launcher tests run via the actual
PowerShell wrapper from `rehearsal/unrelated cwd`, with PATH limited to installed
Node and Windows System32. Browser sessions run the extracted Node launcher.

- `node --test scripts/windows-alpha/package.test.mjs`: **10/10 passed**.
  Covers original single fixture/tutorial and new pair generation, native OFF/ON,
  collisions, inert/default and missing prerequisites/components, literal paths,
  both relocations, pair order/reference reversal, independent class table,
  exact control names/int64 IDs, scope-key/encoding refusal, legacy UNKNOWN,
  captured reader failure, duplicate artifact/bad second reader, static, and
  clean reopen. Each split `st` / `op\n` stop keeps stdin open until natural exit;
  wrapper/server and each reader exit are asserted, not listener closure alone.
- `node --test scripts/windows-alpha/boundary.test.mjs`: **1/1 passed**;
  missing selected input stays explicit failure without creating it or demo fallback.
- `browser-rehearsal.mjs`: **9 sessions**, all normally stopped. Named assertion
  counts: off 8, on 8, foreign 8, refused 2, static 3, pair 18, pair-reversed 18,
  pair-scope 19, pair-reopen 18. These are overlapping session checks, not 102
  unique tests. Actual Chrome version, requests and assertions are in each
  `browser/<case>/browser.json`. Checked visible same-basename A/B labels,
  A/B/All counts, exact Find, explicit reference/root action, independent totals,
  both sides' IDs/full hashes/exact bytes, filters, scope qualification, reference
  reversal, held late success/error responses, history and keyboard/skip focus.
- Real console: `console-transcript.txt` and `console-exit.json` retain
  ConsoleHost, `IsInputRedirected=False`, PowerShell PID 16600, typed `stop` plus
  Enter, reader PIDs 5880/9580 both code 0, packaged child code 0 and outer exit 0.
  This is a new console execution, not inference from pipe tests or old evidence.
- Additional `locked-source-boundary-final.mjs`: all **22 generated source
  files exclusively locked** throughout three pair sessions. Comparison still
  succeeds; POST is 405 and an actual invalid Host header is 403. Deliberately
  killing the captured second reader makes comparison 503; shutdown waits for
  both readers; a fresh session succeeds. The induced reader termination is
  separately labeled, not a normal-stop pass. Exact PIDs, args and exit results
  are in `locked-source-boundary-final.json`; exit 0 and lock count 22 in
  `locked-source-final-exit.json`.

Two supplemental probe harness attempts failed and remain retained: first used
a non-UUID comparison request (400 correctly refused); second used fetch's Host
override, which did not send the intended header (200). Corrected the harness to
use a UUID and raw Node HTTP for Host. Both failed attempts ran their finally
close/wait, then Node emitted an uncaught-assertion/libuv exit diagnostic. The
successful third attempt is separate evidence; neither failure caused a runtime
or package edit/rebuild. No false all-pass aggregation.

Browser screenshots at 1440x1024 CSS pixels, scale 2, include
`browser/pair/library-all.png`, `find-both.png`, `comparison-detail.png`,
`comparison-swapped.png`, `comparison-summary.png`, and
`browser/pair-scope/scope-qualified.png`; single/static/refusal screenshots are
retained too. These are package behavior evidence, not a new Figma fidelity claim.
The comparison detail screenshot was visually inspected: both source labels,
recorded roots, times, IDs, full hashes and exact sizes are legible.

Observed application requests stayed on the session loopback origin; no runtime
exceptions. Negative routes refuse package files, adapters, catalogs and APIs.
Chrome stderr contains a default background web-app install diagnostic for Gmail;
the CDP application-request check is not a claim that every browser background
component is network silent. No application upload/telemetry was added.

Accepted reader code operates on fixed catalog inputs; comparison operates on
recorded frames. Source-lock execution additionally demonstrates no source reads
for this pair. Source/catalog before-after hashes and canonical extract identities
are retained. All owned validation processes have completed and were waited;
browser summary includes launcher/browser PIDs and reader exit output. No owner
process was attached to or stopped.

## Exact changed paths and review boundary

Modified: `scripts/windows-alpha/{launcher.mjs,tutorial-fixtures.mjs,build.mjs,
package-files.mjs,package.test.mjs,browser-rehearsal.mjs,QUICKSTART.md,SUPPORTED.md,
BUG-REPORT.md}`; new `scripts/windows-alpha/comparison-browser-checks.mjs`.
Documentation: `docs/development/{CODEX_HANDOFF.md,NEXT_ACTIONS.md,OB_STATUS.md,
REVIEW_COVERAGE.csv}`, `RELEASE_CHECKLIST.md`, `scripts/gui-preview/README.md`, and
this new report. Final per-file hashes are in the external candidate manifest.
No existing historical report, native Go source, server, GUI, protocol or comparer
changed. There is no separate feature-matrix file to duplicate; the existing
living status/coverage/release records are updated with this bounded milestone.

Earlier author/reviewer counts and the original package review remain historical.
No full native archival suite was rerun for unchanged runtime. No second machine,
downloaded-file policy, arbitrary terminal, public signing/reputation/distribution,
real-data/large-scale/platform/ACL/power-loss/media qualification is claimed.
Prompt20 remains **DEFERRED_BY_OWNER**. The prior security alert remains unresolved
at its previously recorded classification; no new security block occurred and no
protection was disabled or bypassed. Generated-data use only, local Windows scope.

**One next action:** substantive package-delta review of these uncommitted source
identities and this exact ZIP. Do not infer owner acceptance, replace the original
package, publish source/binaries, or start another feature. No staging, commit,
push, merge, tag, signing, upload, installation or production/media operation.
