# Windows inventory developer-alpha package - 2026-09-20

**WINDOWS_INVENTORY_ALPHA_PACKAGE_READY_FOR_REVIEW**

Local, unsigned packaging candidate only. No staging, commit, push, merge, tag, release, installation, upload or public distribution. No accepted runtime semantics were changed. This is not a release candidate for Obelisk's backup/archive product.

## Authority and checkpoint

The owner submitted Prompt 17 as the current task, authorizing this bounded local branch/packaging/tutorial/rehearsal milestone. Actual submitted text is retained externally as `submitted-prompt.txt`; historical prompts were not executed. The preceding publication receipt was read, not regenerated, and its helper was not rerun.

Initial branch `feat/gui-disposable-inventory`, full HEAD `bfbce891df78d529c6be2d2912dc8443597c007e`, empty index, clean tracked worktree and no merge/rebase/cherry-pick/revert/sequencer/index-lock markers were verified. Its parent is accepted runtime implementation `8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c`, whose parent is `5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`. No new live remote lookup or publication was performed.

Created **`feat/windows-inventory-alpha-package`** at **`bfbce891df78d529c6be2d2912dc8443597c007e`**. HEAD remains that full SHA. All candidate changes are uncommitted and unstaged. The old 290-file prepublication count was not used as a new acceptance requirement. No concurrent agent participated. No applicable AGENTS.md was found in the checkout/ancestor locations inspected; contributor, license, release and current handoff guidance was used.

Task directory, abbreviated **T** below:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225`

`initial-checkpoint.json`, the submitted instruction, build commands/source manifests, test enumeration/logs, browser evidence, console results and final candidate/artifact identities are retained there. Existing inventory reports and design inputs remain preserved.

## Security context and qualification limits

The owner reports a negative scan after an earlier Windows alert. That original detection remains unclassified. This task does not call it a false positive, attribute it to a tool, or certify this host/package malware-free. Read-only `Get-MpComputerStatus` and `Get-MpThreat` queries returned access denied, HRESULT `0x80041003`; no elevation of those queries, full/offline scan or forensic investigation followed. OS CIM information was also inaccessible; ordinary .NET/Node OS version information was available. These denied observations are not evidence of safety.

No security detection/quarantine or protection-blocked build/launch was reported in the commands executed here. Antivirus/firewall/script policy/TLS/sandbox/global PATH settings were not changed. No exclusion, allow-list, quarantine restore, encoded execution or policy bypass was used. The package tells testers to stop and report detections/permission blocks rather than retry in alternate forms. Original-alert classification, public signing/reputation/SmartScreen and distribution approval remain open gates.

## Implementation and feature freeze

New files are all under `scripts/windows-alpha/`:

- `Launch.ps1`, `launcher.mjs`: explicit help/check/generate/inventory/view/static actions, package-relative executable/assets, Node 24 x64 checks, package byte verification, argument-array transport, deliberate mode selection and child waiting. No user-controlled shell command construction. Defaults display help only.
- `tutorial-fixtures.mjs`: ten deterministic expendable files, including Unicode/punctuation, nesting, equal bytes at distinct paths, distinct content, an empty file, two exact regular `.DS_Store` controls, a near name, a sidecar and content beneath a same-named directory.
- `build.mjs`, `package-files.mjs`, `zip.mjs`: exact committed build-source selection and package entry allowlist; offline single-target build; stored-entry UTF-8 ZIP; non-overwriting build/package outputs; notices and machine-readable identities.
- `extract.ps1`: test/developer-only independent .NET ZIP extraction into a new destination, with unsafe-entry refusal.
- `package.test.mjs`, `boundary.test.mjs`, `browser-rehearsal.mjs`: bounded package/dependency/relocation/protocol/browser/shutdown checks. These are not shipped or served.
- `QUICKSTART.md`, `SUPPORTED.md`, `BUG-REPORT.md`: relative tester instructions, limits/compatibility, support matrix, protection guidance and optional redacted reporting without telemetry.

Documentation changes: this report; the four existing living records (`CODEX_HANDOFF.md`, `NEXT_ACTIONS.md`, `OB_STATUS.md`, `REVIEW_COVERAGE.csv`); `RELEASE_CHECKLIST.md`; and `scripts/gui-preview/README.md`. The tracked checkout has no existing FEATURE_MATRIX file; no competing master roadmap was created. The package's small supported/unsupported table is tester guidance, not a replacement project matrix.

The Go producer/reader/shared code, catalog schema/protocol, Node server/adapter/UI, serving allowlists, query/encoding/ID rules and shutdown parser remain unchanged. The launcher starts only the accepted finite producer, accepted read-only viewer or explicitly selected static demo. It neither starts the normal production server on missing arguments nor adds a browser scan/executable-selection endpoint. A selected catalog failure stays an explicit failure, never successful static/empty fallback.

The reusable executable is the full Obelisk binary. It includes other commands and existing embedded UI/documents/format metadata/development escrow placeholder. The wrapper is not a security sandbox; no backup/archive or release escrow qualification is implied. No framework migration, backend repair or installer was introduced.

## Build and package identity

Host observed: Windows version **10.0.19045.0**, **x64**. Target **windows/amd64**, PE machine **0x8664**. Installed Go **1.26.8 windows/amd64**, Node **24.19.0 x64**, Chrome **153.0.8010.48**. Windows PowerShell ConsoleHost was used. No other OS/architecture was executed or bundled.

Build-source commit: **`bfbce891df78d529c6be2d2912dc8443597c007e`**; accepted runtime implementation is its parent **`8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c`**. `build.mjs` reads the committed blobs for non-test root Go files, go.mod/go.sum, required embedded documents/formats/escrow inputs and UI files into a separate build source directory. No worktree packaging code is misrepresented as committed runtime. `build-source.json` records exact copied source bytes. Runtime preservation is checked separately from authorized new packaging/documentary changes.

Selected command, with the exact installed Go executable, working directory and absolute output retained in `T/build-complete/commands.json`:

```text
go build -trimpath -buildvcs=false -ldflags "-X main.appVersion=0.9.0-dev-inventory-package.bfbce891df78" -o <new-stage>/bin/obelisk.exe .
```

The existing `0.9.0-dev` convention is retained with an explicit development/source suffix; this is not a release tag/version claim. Build environment: `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-mod=readonly`, `CGO_ENABLED=0`, `GOOS=windows`, `GOARCH=amd64`, explicit existing module cache and new task-owned Go cache/temp, GOTMPDIR unset. No dependency registry/download/install occurred. Build and `go version -m` completed with exit 0. No new Go unit-test/vet campaign was run.

First build `T/build` failed with `bagit.go:37:12: pattern docs/COMPARISON.md: no matching files found`, exit 1, before any executable was produced. The packaging build-source allowlist omitted embedded documents/formats. A complete committed `go:embed` inspection identified `docs/COMPARISON.md`, `docs/RESTORE_RUNBOOK.md` and `formats.json`; only the packaging allowlist was corrected. First commands/stderr/source and `build-script-at-attempt.mjs` remain retained. The successful build used a distinct directory `build-complete`. This was a compiler missing-input failure, not a reported security detection or an alternate-form security retry.

Final ZIP:

`T/build-complete/0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`

| Artifact | Exact identity |
|---|---|
| ZIP | 14,394,065 bytes; SHA-256 `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de` |
| `bin/obelisk.exe` | 14,280,192 bytes; SHA-256 `bd03641cc098ae79a5c1cf5e6a86464f8e478ef4188f51870501234406435292` |
| `package-manifest.json` | SHA-256 `6fc6aca29d187b2487eef86a55859dbff6c1273975d81e113a1b99f7af02c98a` |

The internal manifest hashes 19 content entries and records source commit, uncommitted build/launcher/script identities, target/toolchain and runtime prerequisites. It does not hash itself. The external `artifact.json` hashes the finalized ZIP and internal manifest, and records all 20 entries. `candidate-identities.json` records the final uncommitted packaging/test/documentary bytes separately; the implementation report has a separate identity record. Hashes identify bytes, not signing, trust or malware status.

Exact ZIP entries:

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

Notices include Obelisk MIT and the actual cached licenses for barcode, go-qrcode, BLAKE3, cpuid and the Go runtime. Node/browser/fonts/third-party helper executables are not bundled. Public redistribution review remains pending; no blanket license clearance is asserted. No .git, designs, private catalogs, keys, raw reports/screenshots/evidence, historical executable, generated tutorial catalog, caches or full build environment appears in the ZIP.

## Fresh rehearsal and results

Relevant existing Node test groups and new package checks were enumerated before execution (`existing-test-enumeration.txt`, `package-test-enumeration.txt`, `boundary-test-enumeration.txt`). Existing inventory author/reviewer Go/Node/browser results remain historical, as recorded in the [accepted closing review](GUI-DISPOSABLE-INVENTORY-SCOPE-S1-R1-FOCUSED-RECHECK-2026-09-20.md). They were not replayed or counted as new package passes.

Successful unpacked rehearsal uses `T/rehearsal-corrected/extracted-one` and `T/rehearsal-corrected/relocated café O'Brien & +%#`. Both were independently extracted from the same unchanged ZIP through .NET ZipFile, with exact entry set, internal manifest and every content hash checked. Current working directory was the unrelated `T/rehearsal-corrected/unrelated cwd`. Runtime PATH contained only the installed Node directory and Windows System32; normal OS variables including PATHEXT were retained. No Go, Git, compiler, old evidence binary, build source/cache or checkout path was selected by runtime commands. Known executable/asset paths are recorded in command stdout. The harness itself is developer test code; it is not an end-user dependency.

| New execution | Result |
|---|---|
| Corrected package test selection | **7 passed, 0 failed, 0 skipped/cancelled**, 19,956.7338 ms; 34 recorded child commands |
| Focused missing-catalog supplement | **1 passed, 0 failed, 0 skipped/cancelled**, 323.8477 ms; separate from seven above |
| Initial browser catalog sessions | OFF **8**, ON **8**, foreign/control-name/large-ID **8**, malformed scope **2** named checks; four completed sessions |
| Static browser correction | One completed session, **3** named checks; separate retained initial failure below |
| Packaged CLI split stop | **10** sessions completed through `stop` split as `st` then `op\n`, stdin kept open through natural exit; listener closure asserted |
| Interactive console paths | Two nonredirected Windows ConsoleHost PTY checks: Ctrl+C and typed `stop` |
| Final process/listener check | No task-owned Node/Chrome/native processes remained; **18** recorded loopback URLs no longer responded |

Tests execute the packaged producer on two new generated workspaces. Each contains ten known files. OFF includes ten/excludes zero; ON includes eight/excludes two, with 13 visited entries and ten regular files. Same-named directories are traversed; near names and sidecars stay included. Equal hashes at distinct paths remain distinct file records. Six successful new snapshots were produced (OFF, ON and a separately named OFF per workspace); two attempts to replace an existing output refused with `published:false`, preserving bytes. Repeated generation refused existing workspaces. Source before/after manifests match for both ten-file trees.

The package byte check refuses missing Node and missing/wrong packaged adapter/assets without alternate binaries/downloads/static fallback. Read-only viewers load OFF/ON, query/select the exact punctuation name, stop and reopen existing snapshots without regeneration. Native/HTTP probes reject malformed canonical scope keys and malformed encoding, accept a supported unscoped protocol fixture with UNKNOWN, distinguish actual LF from literal backslash-n, and preserve ID `9007199254740993`. Missing catalog remains failed without creating a file. A test-induced termination of a specifically captured task-owned native reader invalidated `/mode.mjs` and returned query failure; `intentional-reader-fault.json` distinguishes this forced fault from normal shutdown.

Browser checks use the extracted launcher and installed Chrome, 1440x1024 CSS pixels, scale 2 (2880x2048 PNGs). They cover Library scope, Find, exact-name input via CDP, inspector selection, large IDs, disclosure state retention, keyboard skip-link/Tab focus, malformed-load refusal and deliberate static mode. Recorded page requests were loopback application requests only, with no runtime exceptions; this is bounded browser/runtime evidence, not OS-wide network tracing. Screenshots under `T/browser/{off,on,foreign,refused}` and `T/browser-static-corrected/static` are retained. The ON Find/inspector and refused-scope screenshots were visually inspected; no new design-fidelity or human GUI certification is claimed.

### Retained failed/setup attempts

The first package run (`T/rehearsal`, `package-tests.log`) omitted Windows PATHEXT from the bounded child environment. PowerShell consequently lost native output/wait/exit behavior: six top-level checks failed and a dependent reader-fault setup did not complete. The specifically identified task-owned test process PID 17976 was stopped after confirming it had no live children; that is forced cleanup, not a passing shutdown test. The first harness and command records are preserved. Controlled launcher `check` diagnostics with stdin closed/open reproduced empty output under that environment, while retaining PATHEXT produced the expected package identity in both cases. Only the test harness environment and its dependent-fault precondition were corrected. The ZIP/runtime/launcher were not rebuilt or modified for this retry.

The initial five-session browser run completed its four catalog sessions, then failed the static assertion because its readiness wait matched the initial HTML heading before module rendering. That failed session has zero completed named checks; its Chrome cleanup was forced and waited, while its server stopped normally. The original browser harness/summary remains preserved. Only the test wait was changed to the rendered projects panel, and **only static** was rerun in a new evidence directory. The four passing catalog sessions were not repeated. Overall: six browser attempts, five completed sessions, one retained setup failure; assertions and sessions are not interchangeable or added to historical totals.

### Console and dependency evidence

Tool PTY `ConsoleHost` reported `IsInputRedirected=false` and `IsOutputRedirected=false`. Ctrl+C (session 96900, URL port 50561) reached `Preview stopped (SIGINT)`; native reader PID 12772 exit 0 and packaged child exit 0 were printed/waited. The interrupted outer PowerShell returned 1, retained as such rather than a fabricated all-zero command. Typed `stop` plus carriage return (session 10345, port 50576) reached `stdin stop`; reader PID 13268, packaged child and outer PowerShell all exited 0. `console-capability.json` and `console-results.json` identify the captured tool output and scope. Both URLs were subsequently closed. These are actual console-path observations in this tool PTY, not human Windows Terminal, every console host or second-machine qualification. The quickstart retains a short owner check for broader console environments.

`artifact-validation.json` rechecks all package/extraction hashes, PE target and both generated source manifests after rehearsals. A targeted UTF-8/UTF-16LE byte scan of all package content found none of the development user/task/cache path needles; `-trimpath` and `go version -m` metadata were also inspected. Module identifiers, synthetic paths, compiled default strings and the existing embedded development escrow placeholder still exist. No exhaustive decompilation or absence-of-all-path-metadata claim is made.

## Accepted data/compatibility boundary

This is small, quiescent, generated Windows fixed-local-source inventory only. The accepted producer's explicit/disjoint ordinary source/output parents, absent output, hard-link support, bounds, reparse/special refusal, original-name handling, cancellation and publication point remain intact. Limits: 64 regular files/128 entries/depth 8, 512-byte relative/4096-byte absolute paths, 8 MiB per included file/32 MiB total, one growth-detection byte before refusal, 30-second cooperative deadline. No hostile-race confinement, ACL, power-loss, unsupported-storage or scale guarantee is added. Producer reads its selected generated files; the viewer reads the snapshot only and treats source/media paths as data.

Exclusion remains OFF by default and matches only exact classified regular `.DS_Store` basenames when enabled. Sources/sidecars are not cleaned; same-named directories are not pruned and historical occurrences are not hidden. Durable scope distinguishes OFF/ON/counts/valid zero/all-excluded/empty/legacy UNKNOWN, with strict required-key validation. Existing exact-search/ID/failure/loopback/serving/shutdown behavior is unchanged.

New scoped catalogs require the corrected compatible reader paired with the documented source/output/binary identities. Previously identified pre-correction reader SHA-256 `bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69` refuses populated scope, per historical reviewed compatibility evidence; it was not rerun here. Supported older unscoped catalogs retain UNKNOWN, never inferred OFF/zero. No scope stripping, downgrade, normalization or catalog migration is permitted. This newly built reader has its own recorded identity, not an assumed historical executable identity.

No clean Windows VM or second machine was available/used. Same-workstation fresh-directory relocation is dependency-isolation evidence only. Production, broader filesystems/names, other OS/architecture, scale, Docker/CI, packaging/public signing, hardware and backup/recovery remain unqualified. PR-04 and external-tar Unicode containment remain outstanding; broader persistence/concurrency, tape/ring buffer and Blu-ray remain separate. LTO-8 remains the first physical qualification target, not the generation limit.

## Review handoff

Reviewer should inspect exact package/source/script identities and entry/dependency/license boundaries; no-replace fixture generation and explicit actions; argument transport and runtime prerequisites; failure/compatibility/shutdown handling; independent extraction and relocation evidence; retained failures and precise console/clean-machine/security limitations. Reuse the accepted runtime findings rather than silently expanding production scope. Review working-tree packaging code against its recorded hashes, not merely HEAD.

ONE next action: **substantive package review** of this uncommitted local candidate. No automatic publication, release, next feature, production inventory or hardware work follows this report.
