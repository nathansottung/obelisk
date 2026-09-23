# Windows comparison package substantive review — 2026-09-21

**WINDOWS_COMPARISON_ALPHA_PACKAGE_READY_FOR_OWNER_REVIEW**

No material packaging defect was found in the reviewed local Windows,
unsigned, generated-data scope. This verdict applies to the original ZIP and
uncommitted source identities below. It is not owner acceptance, public-release
approval, second-machine qualification or security certification. No fixes were
made. Only this report was added to the checkout during the review.

## Authority, provenance and checkpoint

The owner separately submitted Prompt 29 from attachment
`c3842e0a-787d-4f90-8528-d345bffb96c2/pasted-text.txt`. Its verbatim copy is in the
review evidence directory. This is a same-conversation Codex reviewer pass with
author context available, not an independent human or context-isolated review.
No delegation was used. Historical prompts were context, not executable tasks.
No completed matching comparison-package review existed at entry.

Review evidence root, abbreviated **R**:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-review-20260921-193716`

Author evidence root, abbreviated **A**:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-alpha-20260921-175648`

Branch: `feat/windows-comparison-alpha-package`.
HEAD and complete packaging-patch parent:
`e3d9bef998a20dff78dc67463dfb8f848aad76ce`.
Accepted comparison implementation in its ancestry:
`87b4fcf3443bf18bd64671896f911c73dcbdc22a`.
Index empty; no merge/rebase/cherry-pick/revert marker at entry. Candidate work
remains uncommitted and unstaged. No branch creation/switch or remote operation.
No applicable AGENTS.md was found. The controlling implementation report,
retained Prompt 28, packaging source/tests, current handoff/next-actions/status,
coverage/release records and relevant prior package/runtime reports informed the
review. Those historical executions are not reviewer test results.

The candidate manifest contains exactly these 17 changed/new paths, all verified
against their raw working-byte hashes before execution and at closeout:

```text
RELEASE_CHECKLIST.md
docs/development/CODEX_HANDOFF.md
docs/development/NEXT_ACTIONS.md
docs/development/OB_STATUS.md
docs/development/REVIEW_COVERAGE.csv
docs/development/reviews/WINDOWS-COMPARISON-ALPHA-PACKAGE-IMPLEMENTATION-2026-09-21.md
scripts/gui-preview/README.md
scripts/windows-alpha/BUG-REPORT.md
scripts/windows-alpha/QUICKSTART.md
scripts/windows-alpha/SUPPORTED.md
scripts/windows-alpha/browser-rehearsal.mjs
scripts/windows-alpha/build.mjs
scripts/windows-alpha/comparison-browser-checks.mjs
scripts/windows-alpha/launcher.mjs
scripts/windows-alpha/package-files.mjs
scripts/windows-alpha/package.test.mjs
scripts/windows-alpha/tutorial-fixtures.mjs
```

`R/reviewed-candidate-identities.json` is a retained copy of
`A/candidate-identities.json`, SHA-256
`44f01813dc8074428c816b04194da996c1b9612dba75d90181e0cd46755a31fe`.
`R/starting.diff`, `starting-status.txt`, `checkpoint.json`, `reviewed-source/`
and `final-preservation.json` retain reconciliation evidence. Seventeen is the
candidate change count, not a whole-repository audit count or staging allowlist.
Accepted native/server/UI/protocol/comparer files are unchanged from the parent;
the preview README is documentation, not a runtime change. Existing untracked
PDF/Figma and ignored design ZIP remain untouched. No living record was edited.

## Exact original artifact and separate build control

Primary archive executed:

`A\build\0.9.0-dev-comparison-package.e3d9bef998a2-windows-amd64-local.zip`

14,431,445 bytes; SHA-256
**`33d1d9e5087c35aca23606957777f3d13ff8267de798c9848c79ada4697f00d2`**.

Packaged manifest SHA-256:
`9ab8ac95a56cc16686b778070667dbd180a58a5062e21e6699e0c7fb7196c28c`.
Native executable SHA-256:
`a7e179bfbf6bac7d8f013058b954e0a2195e605b0f64c5fb77d2b046517feb2f`.
Native build-source manifest SHA-256:
`18d3943c151038f28d554ee103aba1425ecdedc72c9bdd732a54e66d7d20a3b6`.

Listed the ZIP before extraction. Its 22 entries have no absolute/traversing
names, duplicate/case-colliding destinations, links or unintended files. Actual
names are ordinary ASCII destinations from the explicit allowlist. Two new .NET
extracts under `R/rehearsal/extracted-one` and
`R/rehearsal/relocated café O'Brien & +%#` match the packaged/staging manifest and
allowlist. Manifest hashes cover 21 content entries; the enclosing ZIP hash and
separate manifest hash identify the manifest without an impossible self-hash.

Delivered content: `bin/obelisk.exe`; the 12 preview runtime files (`server.mjs`,
`stop-command.mjs`, `catalog-adapter.mjs`, `index.html`, `app.mjs`, `fixtures.mjs`,
`style.css`, `catalog-ui.mjs`, `catalog-protocol.mjs`, `catalog-names.mjs`,
`recorded-comparison.mjs`, `comparison-ui.mjs`); `Launch.ps1`, `launcher.mjs`,
`tutorial-fixtures.mjs`, `QUICKSTART.md`, `SUPPORTED.md`, `BUG-REPORT.md`,
`LICENSE`, `THIRD-PARTY-NOTICES.txt`, and `package-manifest.json`.
No design originals/exports, previous packages, catalogs/private state, history,
test evidence, screenshots, caches, Node/browser or incidental executable.

The inspected builder reads committed native and preview blobs from the exact
parent and uncommitted packaging files from their identified working bytes. It
does not select an old executable from evidence. New output-directory creation
and exclusive final-file creation prevent replacement. A reviewer control rerun
against its own existing build directory refused with exit 1 before mutation;
the artifact record stayed unchanged. Failed builds retain partial output and do
not become successful artifacts. No automatic cleanup/retry is claimed.

Separate fresh offline build:

`R\control-build\0.9.0-dev-comparison-package.e3d9bef998a2-windows-amd64-local.zip`

Its directory distinguishes it from the original; the internal package ID was
not restamped. It is **byte-identical** to the original ZIP in this environment,
including executable, manifests and shipped inputs. This is one observed control,
not a universal reproducibility guarantee. The original ZIP, not this control,
was used for all primary package/browser/lifecycle execution.

Build command (repo working directory; all outputs external):

```text
node scripts/windows-alpha/build.mjs R\control-build
  C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\go.exe
  C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod
```

This displayed command is wrapped for readability; `control-build/commands.json`
contains exact executable/argument arrays and exits. Go 1.26.8 windows/amd64,
Node v24.19.0 x64; `-trimpath`, `-buildvcs=false`, version ldflag, CGO=0,
windows/amd64; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, readonly modules,
retained cached modules and fresh cache/temp. No downloads or tool changes.
`control-build/build-source.json`, `binary-build-info.txt`, `artifact.json` and
command stdout/stderr retain provenance. The build metadata names all four linked
dependencies represented in the notice selection (barcode, go-qrcode, blake3,
cpuid), plus Go's runtime notice; no new third-party asset was introduced. This
review does not provide blanket redistribution/legal clearance.

The filename suffix identifies the committed runtime parent, not all packaging
bytes or an official 0.9.0 release. The full native binary retains its accepted
embedded UI and broader commands. The wrapper is not a security sandbox.

The OLD frozen inventory ZIP is separate:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225\build-complete\0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`

Its checked SHA-256 remains
`f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`.
Neither archive, author stage, previous extracts, owner workspace nor historical
report was replaced or updated.

## Source findings and independent expectations

No material finding requiring implementation correction. Reviewed
`launcher.mjs` argument validation, `generatePair`/`generate`/`workspacePath`,
native command dispatch and child waits alongside unchanged `Launch.ps1` array
forwarding. No shell command-text evaluation. One/two path ordering is preserved;
missing/excess/duplicate paths, nonexistent input, missing/wrong packaged component
and byte-identical aliases refuse. Either failed reader invalidates the pair;
intentional static remains a separately requested action. The no-argument path
only prints help. Node version/platform/architecture requirements are explicit.

Generation requires a new separate ordinary workspace. A reviewer-created partial
ALPHA directory with a sentinel refused without merging, deleting or resetting it.
Generation is sequential, not transactional: an I/O failure after mkdir can leave
a partial new tree. The code does not print the final ALPHA/BETA completion line
unless both sides finish, and a subsequent run refuses the existing destination.
No rollback/atomic-pair guarantee is advertised; no injected mid-write failure was
needed to establish the collision boundary.

Both original-ZIP relocations generated actual new ALPHA/BETA source files, then
native inventories. Reviewer enumeration independently checked exact relative
paths and computed each included source file's SHA-256 and byte length against
the native catalog, without using comparison output as the expected value.
Eleven source files per side; ON retains nine records and leaves all source files
on disk. The `.DS_Store` directory child and `.DS_Store.bak` remain included.
No snapshots were rewritten to falsify native observation provenance.

| Inputs/reference | Agreement | Difference | Inconclusive | Reference only | Counterpart only | Union |
|---|---:|---:|---:|---:|---:|---:|
| ALPHA OFF / BETA OFF | 9 | 1 | 0 | 1 | 1 | 12 |
| ALPHA OFF / BETA ON | 7 | 1 | 0 | 3 | 1 | 12 |
| BETA ON / ALPHA OFF | 7 | 1 | 0 | 1 | 3 | 12 |

The distinct-content key is `nested/O'Brien & +%# note.txt`; side-only keys are
`ALPHA-only.txt` and `BETA-only.txt`. Equal content at different paths remains
separate. The ordinary producer supplies no inconclusive pair; zero is expected.
Malformed scope/encoding, duplicate artifact and failed-reader refusal controls
were executed instead of fabricating an inconclusive native tutorial observation.
The large-ID/control-name protocol fixture is explicitly synthetic test input.

Reference/root alignment remains an explicit user action. Source scope/time,
exact IDs/full digests/sizes and A/B attribution remain tied to each input through
startup reversal and reference reversal. Documentation accurately distinguishes
recorded agreement and recorded-set absence from live verification, physical loss,
backup health and proof of independent copies. Compatible scoped reader and legacy
UNKNOWN requirements, disjoint new outputs, bounds and stop/reopen are retained.

## Fresh reviewer execution and limitations of each observation

PowerShell **5.1.19041.6456**, Node **24.19.0 x64**, Chrome
**153.0.8010.48**. Test names were selected before execution in
`R/selected-tests.txt`; inspected assertions were run unchanged:

- `node --test scripts/windows-alpha/package.test.mjs`: **10 passed, 0 failed**,
  exit 0. `OBELISK_ALPHA_BUILD=A/build`, `OBELISK_ALPHA_TEST=R/rehearsal` make
  this suite extract the original identified archive into new reviewer locations.
- `node --test scripts/windows-alpha/boundary.test.mjs`: **1 passed, 0 failed**,
  exit 0, with `OBELISK_ALPHA_INPUTS=R/rehearsal/rehearsal-inputs.json`.
- `browser-rehearsal.mjs` against those inputs: **9 sessions**, all normal stops,
  exit 0. Named check counts: off 8; on 8; foreign 8; refused 2; static 3; pair 18;
  pair-reversed 18; pair-scope 19; pair-reopen 18. These overlapping checks are not
  102 unique tests. Accessibility-tree assertion also executed, but this is not
  a human screen-reader test. `browser-selection.json`, `browser.log`,
  `browser/summary.json` and per-case browser JSON retain actual execution.
- `independent-controls.mjs`: exit 0; independently computed file identities,
  partial-workspace sentinel preservation, existing-build refusal and source/
  artifact pairing. Unsupported Node major was tested using explicitly simulated
  `process.versions.node=22.0.0` metadata under Node 24, not an installed Node 22
  execution. Missing Node was exercised by the package suite's bounded PATH.
- `git diff --check`: passed; line-ending normalization advisories were emitted.
  No formatter, vet or full native archival test suite was run for unchanged runtime.

Package tests execute the PowerShell wrapper from `rehearsal/unrelated cwd`, with
PATH limited to installed Node and Windows System32. Browser tests execute the
extracted Node launcher with the same bounded runtime environment. Native producer
and reader paths resolve inside each extracted package. This demonstrates local
relocation/dependency isolation, not a clean/second machine. No checkout rename or
owner tool removal was used. Review build/browser tools are reviewer dependencies,
not bundled end-user prerequisites.

The tests cover literal package/tutorial commands, single/static mode, both
relocations, output/workspace collisions, exact control names, int64 ID precision,
scope rejection/UNKNOWN, bad second reader, duplicate artifacts, complete pair
counts, reversals, mixed scope and immutable stop/reopen. Browser checks cover
same-basename labels in A/B/All, both-source exact Find, explicit root acceptance,
details, filters, delayed old success/error suppression, keyboard/skip and history.
Each browser session has a fresh review-owned profile and finite lifecycle.

Screenshots at 1440x1024 CSS pixels, scale 2, are in `R/browser/`:
`pair/find-both.png`, `pair/library-all.png`, `pair/comparison-summary.png`,
`pair/comparison-detail.png`, `pair/comparison-swapped.png`,
`pair-scope/scope-qualified.png`, and `refused/scope-refused.png`, plus other
single/static/reversal views. The mixed-scope screenshot was visually inspected:
it explicitly qualifies `.DS_Store` as outside the counterpart policy and does not
invent an absent-side record or claim current physical absence. No new design
fidelity or polish claim is made.

## Read-only boundary and exact lifecycle evidence

Source inspection: `gui_catalog.go` reads the selected catalog file into the
bounded projection; recorded source paths are evidence strings. `catalog-adapter`
spawns only the fixed reader with selected catalog arguments; complete responses
are validated before adoption/enumeration. `recorded-comparison.mjs` operates on
frames without filesystem APIs. `server.mjs` serves the fixed asset map and
loopback data endpoints, with no path/executable picker or producer route.
Incomplete/dead reader contribution throws/refuses rather than yielding empty
success. Current package/query failure and stale-response checks substantiate
those unchanged boundaries without a blanket engine audit.

The separate reviewer lifecycle probe locked the actual 22 files in the second
generated pair using exclusive Windows handles. A second read-open was attempted
for **every** file and all 22 were denied before launching. Locks remained held
through comparison and the traced sessions; after release all source hashes
matched. Locks substantiate no file-content reads for this selected session, not
absence of directory enumeration or general ACL/hostile-filesystem safety.

`R/trace.mjs` is external process-scoped instrumentation, not a package edit. It
records Node starts/exits and spawned server/native-reader IDs and exits. All
required command-driven cases used actual `Launch.ps1`, stdin held open after
the complete token, and waited for the PowerShell wrapper, Node launcher, server
and readers. `lifecycle-results.json` and individual trace/output files retain:

| Probe | PowerShell PID | Server PID | Reader PIDs | Outcome |
|---|---:|---:|---|---|
| split `st`, `op\r`, `\n` | 14028 | 16544 | 16668, 13200 | Listener remained live before LF; all normal exits 0 |
| captured second-reader failure | 18608 | 17708 | 16840, 9160 | Comparison 503 after induced termination; first reader and server waited on stop |
| reopen, whole `stop\r\n` | 2708 | 16644 | 13484, 13300 | Same unchanged pair, successful comparison, all normal exits 0 |

The deliberately killed reader is an induced abnormal exit, not a normal-stop
pass. Wrapper/server code 0 after stop is not misrepresented as successful reader
health; the trace and comparison 503 distinguish it. No owner process was killed.

**Retained reviewer harness failure:** the optional fourth case sent empty EOF and
incorrectly expected automatic shutdown. `stop-command.mjs` and the server's
`onEnd` instead flush a pending command; empty EOF does not invoke stop. The probe
guard could no longer write a stop token after ending stdin. Only the trace-bound,
command-line-verified review reader PID 2744 and server PID 6368 were terminated
and waited, allowing launcher/PowerShell to finish with nonzero status. This was
safety cleanup, not command-driven success or a packaging defect. The aggregate
`lock-results.json` correctly retains exit 1; the first three successful rows are
not erased. `eof-harness-cleanup.json` and original logs retain the failure.

A separate `eof-token.mjs` then exercised supported unterminated `stop` followed
by EOF in a BETA-only session: exit 0, normal server/reader/launcher waits and
listener closure, retained in `eof-token-results.json`. It does not rewrite the
failed empty-EOF case or claim empty EOF is a supported stop command.

**Actual console:** automated terminal input `stop` plus Enter through a fresh
review-owned ConsoleHost, `IsInputRedirected=False`, PowerShell 5.1.19041.6456.
PowerShell PID 14504, Node launcher 12828, server 13068, readers 6004/14008;
both reader, server, launcher and outer PowerShell exits 0. `console-trace.jsonl`,
`console-transcript.txt`, `console-exit.json` retain evidence. This was automated
console interaction, not human typing or reuse of the author's console pass.
It does not qualify every terminal or an unexecuted Ctrl+C variant.

Observed application requests stayed on the isolated session loopback origin;
no browser runtime exceptions. Package tests reject raw package/adapter/catalog/
API routes. Chrome background diagnostics are retained separately and do not
establish application egress or blanket browser-network silence. No public/remote
security certification is inferred. Source/catalog identities, canonical extracts
and command results are retained; all owned validation processes are stopped and
waited, including the abnormal EOF cleanup. No owner process/profile/port was used.

## Working package commands, preservation and next action

From the original ZIP's new extracted directory, using a NEW reviewer workspace
under the actual evidence root (the executed suite used `R/rehearsal/pair-one`;
do not regenerate that existing workspace):

```powershell
.\Launch.ps1 check
$p = Join-Path $env:LOCALAPPDATA 'ObeliskDev\NEW-unused-review-pair'
.\Launch.ps1 generate-pair $p
.\Launch.ps1 inventory "$p\ALPHA" off.json
.\Launch.ps1 inventory "$p\BETA" off.json
.\Launch.ps1 inventory "$p\BETA" on.json --ignore-ds-store
.\Launch.ps1 view "$p\ALPHA\catalogs\off.json"
# Type stop, Enter; wait for reader/server and Packaged child waited.
.\Launch.ps1 view "$p\ALPHA\catalogs\off.json" "$p\BETA\catalogs\off.json"
```

Choose Compare recorded snapshots, deliberately choose the reference, inspect
roots/scope/time, then accept root alignment and compare. Type stop and Enter,
wait for exit, and repeat the final view command to reopen the SAME catalogs.
Do not regenerate or inventory to reopen. Reverse the two arguments to reverse
session A/B; choose reference separately. Use BETA `on.json` for mixed scope.
Commands require existing Windows x64, Node 24 x64, Windows PowerShell and browser;
no checkout, Go or Git is needed by the user. Existing destinations refuse.

Final checks in `R/final-preservation.json`: original and control ZIP identities,
author stage and both canonical reviewer extracts, the 17 candidate paths,
accepted runtime/build source, design inputs, branch/HEAD/index and historical
files remain as identified. The only added checkout file is this report. The
author's broader evidence/owner workspaces were not modified or reused.

Prompt20 remains **DEFERRED_BY_OWNER**. Original security-alert classification
remains unresolved; this review encountered no new security block and changed no
protection. Separate limits remain: second machine/downloaded-file policy,
public signing/reputation/distribution, other terminals/platforms, production
data, scale, ACL/races/power loss, backup/recovery and physical media. No install,
download, package transfer, binary publication or clean-machine claim.

**One next action:** owner acceptance and a separately authorized SOURCE/PACKAGE
CHECKPOINT for source commits/publication while preserving this exact new ZIP.
Do not automatically upload binaries, rebuild/restamp with a new commit label,
replace the old frozen package or start a new feature. Stop here.
