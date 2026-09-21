# Windows inventory developer-alpha package review - 2026-09-21

**WINDOWS_INVENTORY_ALPHA_PACKAGE_READY_FOR_OWNER_REVIEW**

No material blocker was found in this bounded local review of the exact uncommitted packaging candidate and the owner's original ZIP. This is not owner acceptance, clean-machine qualification, public distribution approval, or a safety certification. No corrections were applied.

## Authority, review provenance and identities

The owner separately submitted Prompt 18. This was one substantive Codex review in the current conversation, without a separate agent, isolated review context or human certification. Historical prompts were read as context only. No existing substantive package review for these exact identities was found. Applicable contributor guidance, the current packaging additions to the release checklist and four living records, preview compatibility documentation, submitted Prompt 17, implementation report, packaging code and tests were inspected. No applicable AGENTS.md was found in the checked repository/ancestor locations.

Branch: **`feat/windows-inventory-alpha-package`**.

Full HEAD and complete packaging-patch parent: **`bfbce891df78d529c6be2d2912dc8443597c007e`**. Its parent is accepted runtime implementation **`8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c`**. The new packaging files are uncommitted; the filename suffix identifies the committed runtime parent, not a commit containing this packaging work or a published 0.9.0 release.

Reviewer evidence directory, abbreviated **R**:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-package-review-20260921-090146`

Author evidence directory, abbreviated **T**:

`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225`

| Reviewed input | Verified identity |
|---|---|
| Original ZIP, `T/build-complete/0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip` | 14,394,065 bytes; SHA-256 **`f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`** |
| Source candidate manifest, `T/candidate-identities.json`, copied unchanged to R | SHA-256 **`60ea696a91f4c467b543b381a93e6294813ed5030e557386722e55b1de6713a0`**; all 20 listed working files matched |
| Original ZIP's `package-manifest.json` | SHA-256 **`6fc6aca29d187b2487eef86a55859dbff6c1273975d81e113a1b99f7af02c98a`**; matched author stage |
| Original ZIP's `bin/obelisk.exe` | 14,280,192 bytes; SHA-256 **`bd03641cc098ae79a5c1cf5e6a86464f8e478ef4188f51870501234406435292`** |
| Implementation report | 20,328 bytes; SHA-256 **`6b1cdf4d4bb11ec757e8f2a28b068b3ecadbf68f1c88991f223035ee0a2cf36d`** |

The 20-file candidate consists of thirteen new files under `scripts/windows-alpha`, six modified existing documentary files, and the new implementation report. It is not an audited-file total or staging allowlist. The index was empty, no operation/lock markers were found, and the full untracked listing was retained. Existing untracked `Untitled.pdf` and `docs/Obelisk.fig`, and the ignored design ZIP, were identified separately. R retains the submitted review instruction, initial status/diff, exact raw copies of all 20 candidate files, source identities, reviewer probe sources and execution evidence.

## Original archive and build provenance

The original ZIP hash was verified before extraction or execution. Its actual central entries were listed in `R/zip-central-entries.json` before .NET extraction to a new directory. All 20 names were relative, within the explicit allowlist, without traversal, duplicate/case-colliding destinations, reserved device names or link attributes. Each entry had zero external attributes. The archive contained no unexpected directories or content entries.

The exact entries are the binary; ten `preview/` runtime assets; `Launch.ps1`, `launcher.mjs`, `tutorial-fixtures.mjs`, `QUICKSTART.md`, `SUPPORTED.md`, `BUG-REPORT.md`; `LICENSE`; `THIRD-PARTY-NOTICES.txt`; and `package-manifest.json`. The inspected `package-files.mjs` supplies the explicit runtime/launcher lists. The internal manifest hashes nineteen content entries and intentionally excludes itself; the external artifact record hashes the manifest and finalized ZIP. No cryptographic self-defense or signing claim is required or inferred.

Every extracted content entry matched its manifest and the author stage. All ten preview assets matched committed blobs at the stated parent. All nine manifest-listed packaging script identities matched the uncommitted working bytes. All 92 selected build-source files, including required embedded UI/documents/formats/escrow inputs, matched the committed source and the author's retained build source. Working text comparisons explicitly allowed Git CRLF/LF representation without rewriting files; binary comparison was exact. Runtime source was not changed by packaging or review.

`build.mjs` reads committed runtime blobs and separately identified working packaging files. It creates a new output directory, refuses an existing destination, uses explicit file lists and writes new outputs exclusively. Failed commands stop generation; no stale binary or historical executable is selected. Build flags are `-trimpath -buildvcs=false -ldflags "-X main.appVersion=0.9.0-dev-inventory-package.bfbce891df78"`. The build uses installed Go 1.26.8, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-mod=readonly`, `CGO_ENABLED=0`, `GOOS=windows`, `GOARCH=amd64`, and separate Go cache/temp directories. No dependency download occurred.

After inspecting the packager, the reviewer ran it once into **`R/reviewer-build`**, using:

```text
C:\Program Files\nodejs\node.exe scripts/windows-alpha/build.mjs R\reviewer-build C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\go.exe C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod
```

Here and below R/T are the full directories defined above; arguments were passed separately. Exact build subprocess commands, times, exits and outputs are retained in `R/reviewer-build/commands.json` and adjacent files. The build exited 0. Its 92-source manifest, entry set, all text/assets, internal manifest, executable and complete ZIP were **byte-identical** to the original. The separate reviewer ZIP consequently has the same full SHA-256. This is an observed same-machine rebuild result, not a general reproducible-build guarantee or replacement of the primary original artifact. The PE machine is `0x8664`; Go build metadata confirms windows/amd64 and the recorded dependency versions.

The original LICENSE and third-party notices were compared with the actual committed/license-cache inputs: Obelisk MIT, barcode, go-qrcode, BLAKE3, cpuid and Go runtime notices. Their contents matched the inspected builder and included dependency records. Node, Chrome, helper executables and fonts are not bundled. No designs, `.git`, private catalogs/keys, generated tutorial snapshots, evidence/screenshots or caches are package entries. The full binary includes the accepted embedded production UI and development escrow placeholder; the guide acknowledges this and does not claim a minimal-command sandbox or qualified recovery release. Public redistribution clearance remains a separate gate.

## Launcher, prerequisites and workspace boundaries

`Launch.ps1:1` forwards the remaining argument array to the selected `node.exe`, uses its own script directory, and returns the native launcher exit code. `launcher.mjs` resolves package content from `import.meta.url`, spawns argument arrays with `shell:false`, verifies declared content identities before operational actions, and waits for its child. It has no default production action, auto-installation, alternate adapter selection or implicit catalog generation.

Actual environment: Windows **10.0.19045.0 x64**; Node **24.19.0 x64** at `C:\Program Files\nodejs\node.exe`; Windows PowerShell **5.1.19041.6456 Desktop / ConsoleHost**; Chrome **153.0.8010.48**. The real-console token was not an administrator token. PowerShell 7 and other editions were not executed. Existing policy was not modified. Runtime test PATH retained only the installed Node directory and Windows System32, alongside ordinary OS variables including PATHEXT. Go/Git/cache/checkout paths were not runtime dependencies. The source checkout remained in place; process resolution and inspected import paths, rather than changed cwd alone, establish the tested package-relative behavior.

Fresh original-ZIP extractions were exercised at `R/rehearsal/extracted-one` and `R/rehearsal/relocated café O'Brien & +%#`, from `R/rehearsal/unrelated cwd`. Help was inert. Missing Node failed with an actionable prerequisite message. Missing adapter, changed adapter bytes and missing web asset were tested only in labeled altered extraction copies and refused before startup. The authoritative ZIP and original extraction copies were preserved.

The unsupported-major guard was additionally exercised through PowerShell with a reviewer-only Node preload setting `process.versions.node` to `22.0.0`. The guard refused with exit 1 and no successful check response. **This is a simulated prerequisite-input control executed by Node 24, not execution under installed Node 22.** Only Node 24 was found through command selection; no other runtime was installed or downloaded. The preload never changed package bytes.

Generation requires an explicit new local workspace under ObeliskDev, separate from the package. Ancestors are checked as ordinary directories; documentation retains the quiescent fixed-local assumption rather than promising hostile-race containment. The marker selects tutorial workspaces; it is not an authorization boundary against a malicious local caller. Repeated generation preserved populated workspaces, a separate existing directory sentinel and an existing file sentinel. Four traversal/absolute/installation-style output names were refused before a native spawn. An existing directory named like a catalog retained its sentinel; the native producer exited 1 with `published:false`. Existing catalog collisions also preserved exact bytes, while fresh allowed names succeeded.

The producer's stdout/stderr are inherited and its exit is returned without parsing/reclassifying publication status. The wrapper does not roll back a published result. Post-publication cleanup/status-failure behavior is supported here by source inspection and retained underlying review/test evidence, including `gui_inventory_test.go:299`; those fault-injection tests were **not rerun**. Fresh package collision controls establish the actual new wrapper's nonzero propagation. No broader Go/archive campaign was run.

## Reviewer execution and selected tutorial results

Selections were retained in `R/test-enumeration.txt` before execution. Candidate tests and harnesses were executed unchanged. The primary artifact for all tutorial/browser checks was the owner's original hashed ZIP, not the reviewer rebuild.

| Current reviewer execution | Result and evidence |
|---|---|
| `node --test scripts/windows-alpha/package.test.mjs`, with `OBELISK_ALPHA_BUILD=T/build-complete`, `OBELISK_ALPHA_TEST=R/rehearsal` | **7 passed, 0 failed/skipped/cancelled**, 21,286.1018 ms; `package-tests.log`, `rehearsal/commands.json` |
| `node --test scripts/windows-alpha/boundary.test.mjs`, `OBELISK_ALPHA_INPUTS=R/rehearsal/rehearsal-inputs.json` | **1 passed, 0 failed/skipped/cancelled**, 471.2166 ms; `missing-catalog.log` |
| Existing browser rehearsal with that inputs file, installed Chrome and new `R/browser` output | **5 completed sessions**, named checks OFF 8, ON 8, foreign 8, refused 2, static 3; no forced browser/server cleanup; `browser.log`, `browser/summary.json` |
| Reviewer supplemental wrapper controls | Passed after the separately retained harness setup failure below; `supplemental-corrected.mjs`, log and events |
| Actual ConsoleHost typed stop | Reader, server and outer PowerShell exited 0 naturally; `console-context.json`, `console-results.json` |
| Separate offline reviewer build | Exit 0, byte-identical artifact; `reviewer-build.log`, build records |

The seven package tests cover archive/relocation identities; inert help/missing Node; missing/wrong components; generation and OFF/ON/reuse; exact queries and serving boundaries with stop/reopen; malformed scope/encoding and legacy exact-name/large-ID controls; and reader death versus deliberate static mode. These categories overlap tutorial and lifecycle observations; sessions, assertions and URLs are not added to the test count.

Two newly generated ten-file trees produced six successful snapshots: OFF, ON and a separately named OFF per workspace. OFF includes ten/excludes zero; ON includes eight/excludes two. Exact regular `.DS_Store` files remained on disk, the same-named directory was traversed, and near-name/sidecar entries remained included. Distinct equal-content occurrences retained separate records. Source identities matched before/after inventory and at final verification. Repeated writes to existing snapshots refused without replacement.

Original packaged viewers loaded, queried, stopped and reopened the same snapshots without regeneration. Catalog hashes and relevant directory state remained unchanged across viewing, refusal and shutdown controls; the supplemental output-directory sentinel was an intentional earlier test write. Exact punctuation names selected the matching inspector. A labeled foreign data-only fixture preserved actual LF versus literal backslash-n and native ID `9007199254740993`. Unscoped input displayed UNKNOWN; malformed canonical scope and malformed encoding did not produce complete-empty/static success. Missing selected input remained missing and failed explicitly, including through the actual PowerShell launcher.

Screenshots are retained under `R/browser/{off,on,foreign,refused,static}`: Library and Find/inspector frames for the three catalog cases, a scope-refusal frame and static Find. Viewport was 1440x1024 CSS pixels, scale 2, producing 2880x2048 PNGs. The reviewer visually inspected ON Find/inspector and the refusal screenshot. Browser checks also exercised skip-link keyboard focus and disclosure state preservation. This is packaged-runtime evidence, not a renewed Figma-fidelity claim or human GUI certification.

The server binds 127.0.0.1 with an explicit asset map and method/host/query checks. Tested routes refused manifest, launcher, binary, source/output/helper-style paths and POST. No package-adjacent manifest or launcher became a served file. Browser instrumentation observed only local application requests and no runtime exceptions. Inspected reader/server code consumes the selected snapshot and treats recorded source/media paths as data. This review did not use kernel-wide file/network tracing or claim that hashes alone prove absence of reads.

## Process ownership and shutdown

The seven-test group completed **ten** piped split-stop sessions through `Launch.ps1`, sending `st`, confirming the listener remained available, then `op\n`, while keeping stdin open through natural exit. Five browser sessions independently used the packaged Node launcher and stopped normally. These are overlapping lifecycle observations, not additional test totals.

The supplemental review used an explicitly labeled, external preload solely to record actual Node spawn/exit events; it did not rewrite package code. `R/process-trace.jsonl` records PowerShell parent IDs, launcher/server IDs, resolved executables, exact argument arrays and native exits. For example, launcher 7904 spawned packaged server 16064, which spawned the relocated package's reader 15036. Only that captured reader was intentionally terminated to exercise failure. Mode became failed and queries returned 503. The server then accepted split `st` / `op\r` / `\n` with stdin held open, waited for the failed reader and exited normally; this does **not** count as a naturally successful reader exit. A subsequent reopen used launcher 16936/server 14140/reader 13972, loaded the same catalog successfully and stopped normally. The trace contains the authoritative IDs for each event.

The missing-input, induced-reader-death and successful-reopen sessions all exercised split CRLF through the actual PowerShell launcher. Native failure is visible in mode/query state and the reader's recorded nonzero exit; the server's normal shutdown exit 0 is not called successful catalog adoption. An inventory directory collision separately traced the actual packaged producer 1180 and its exit 1.

Tool PTY session **28291** reported ConsoleHost, input/output redirection false and administrator token false. Typing `stop` followed by carriage return stopped URL `http://127.0.0.1:54033/`; reader **19156** exited 0, the packaged child exited 0, and outer PowerShell exited 0. No forced cleanup was needed. This reviewer ran one real-console typed-stop check; the author's Ctrl+C result remains author-only evidence, not a new reviewer pass or universal console claim.

Final task-scoped process enumeration found no remaining reviewer-owned Node, Chrome, native or PowerShell task processes. All **19 retained reviewer launcher/browser/console URLs** were no longer responding. The direct-module test listeners were also closed and checked within their tests; they are not invented additions to the final 19-URL list. Listener closure is supported separately by recorded process waits, not treated as proof of process termination by itself.

## Retained setup failure and historical evidence

The first reviewer supplemental attempt passed a Windows drive path directly to Node's ESM `--import`. Node refused it with `ERR_UNSUPPORTED_ESM_URL_SCHEME` before the package guard ran; the harness assertion therefore failed, exit 1. The only child exited normally. `R/supplemental.mjs`, `supplemental.log` and `supplemental-events.json` preserve this setup failure. A distinct `supplemental-corrected.mjs` uses `pathToFileURL`, with separate log/events, and completed the previously unexecuted controls. No candidate source/test or package was corrected or rebuilt for this retry. This was an ESM URL-format error, not a security-policy block.

The implementation report and retained author records describe seven author package passes, one author missing-input pass, five completed browser sessions with 29 named checks, ten split-stop sessions, two actual ConsoleHost paths, and eighteen closed URLs. They are **author evidence**, distinct from the executions above. Author console Ctrl+C waited for native/server exits 0 but outer interrupted PowerShell returned 1; typed stop returned all zeros. The author's earlier missing-embed build failure, PATHEXT harness failure/forced cleanup and static-readiness browser failure remain preserved and are not reclassified as passes. The author console summary identifies captured tool PTY output, not a raw terminal transcript or broad Windows Terminal qualification.

The accepted scope-closing recheck provides historical compatibility evidence: pre-correction reader SHA-256 `bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69` refuses populated scope; supported unscoped catalogs remain UNKNOWN. That older reader was not executed or rebuilt here. The shipped guide preserves corrected-reader pairing and forbids stripping scope or migration to manufacture compatibility.

## Preservation, limits and stopping point

`R/final-preservation.json` verifies all **20 candidate files**, **288 pre-existing tracked files**, all **480 author-manifested evidence files**, the original stage/ZIP, both main rehearsal extractions, all 92-source build identities and the three design inputs. This verification compares bytes without copying hundreds of historical artifacts. `R/final-processes.json` records no remaining task processes. Final closeout additionally verifies that this review report is the sole repository addition relative to the initial status, with branch/HEAD/index unchanged. No living record or ledger was edited.

The owner-reported negative scan does not classify the earlier alert. No new detection or security-policy block was reported by these executions; that is not antivirus approval or a malware-free claim. No full-system/security-history investigation, protection change, policy bypass, exclusion, quarantine restore, installation, upload or old publication-receipt helper execution occurred.

Remaining separate gates: clean/second-machine execution; an actually downloaded artifact under ordinary target policy; signing/reputation/public distribution; other PowerShell/browser/console combinations; actual execution under an unsupported installed Node runtime; original-alert classification; production catalogs/sources, scale, hostile races, ACLs, power loss, other platforms, archive recovery and physical media. Same-workstation relocation, bounded PATH and one console token observation do not qualify those areas. The local unsigned status and these accurately documented later gates are not defects in this bounded milestone.

**ONE next action:** owner acceptance and a separately authorized source/package checkpoint. No staging, commit, push, merge, tag, signing, publication, clean-machine setup or next implementation is performed by this review.
