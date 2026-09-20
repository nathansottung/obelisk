# Disposable catalog R1-R3 author follow-up - 2026-09-20

READY_FOR_GUI_CATALOG_FOCUSED_RECHECK

- R1: AUTHOR_ADDRESSED
- R2: AUTHOR_ADDRESSED
- R3: AUTHOR_ADDRESSED

This is the authorized correction and author validation in the same Codex conversation. It is not reviewer closure, owner acceptance, publication, or a second substantive review. The controlling [NEEDS_CHANGES review](GUI-DISPOSABLE-CATALOG-REVIEW-2026-09-20.md) and [original implementation report](GUI-DISPOSABLE-CATALOG-IMPLEMENTATION-2026-09-20.md) remain unchanged. ONE next action: targeted reviewer recheck of R1-R3 and directly affected controls.

## Candidate, provenance and pre-fix replay

Branch remains `feat/gui-catalog-readonly`; full HEAD/complete integration patch base remains `433cedac0a0f8a4e9dc67e3a1722b52c39c7cc6e`. Index empty. No merge/rebase/cherry-pick/revert/sequencer operation or index.lock found. No branch creation/switch, Git setup, remote lookup, staging, commit, push or merge.

Task evidence root **E**: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-fix-20260920-131810`.

Before editing, the 261 working paths from the review's repo-before.json matched, and the separately added review report matched SHA-256 `ac4d75037745d1cf733571fabd0834297efb79ea9f343b5c249d67a1f9d3dc50`. All 262 starting files were copied and hash-verified into E/baseline, including new/untracked source/tests and excluded design inputs. E/before.json, status-before.txt, operations.json and before.patch retain the starting identities/status/diff. No existing matching correction was found; no applicable AGENTS.md was found. Current reports/README/living records and retained prompt history were consulted; embedded historical tasks were not replayed. The actual new submitted instruction is E/submitted-prompt.txt.

The baseline build ran from the retained uncommitted source copy, not from the published static-GUI parent. E/output/baseline-reader.exe SHA-256: `a3bccac680e65b8bf37a1038a00eca77f256d7f759c6d55aa8535afdfde36c23`. The reviewer's original protocol-double.go was copied and freshly built separately. Author/reviewer inputs/evidence were never overwritten.

E/baseline-repro.mjs reproduced all three findings before correction: valid-looking bootstrap then exit 7 left mode.ok true and query 503; the bbbb query selected aaaa evidence after large IDs rounded; `st` followed 100 ms later by `op\n` was ignored after 300 ms, and a subsequent whole stop command cleaned up the owned launcher. Exact observations/exits are in output/baseline-reproductions.json. The historical browser assertions were copied into an external harness, changing only the server import to E/baseline. Fresh baseline-browser-r1 and baseline-browser-r2 runs each confirmed the original defect and captured screenshots with the freshly built baseline binaries. These are new baseline observations, separate from the completed review's evidence and from corrected acceptance assertions.

## R1 - detected failure, adoption and recovery

[catalog-adapter.mjs](../../../scripts/gui-preview/catalog-adapter.mjs) now validates the narrow response envelope before adoption and exposes a current mode getter. Terminal process/transport/protocol failure clears successful catalog mode and settles pending work. Complete LF-terminated JSON responses commit operations; a valid-looking partial response followed by EOF/failed exit is refused. Accumulated stdout is capped at 16 MiB before concatenation/parsing; the existing five-second response timeout and bounded stderr retention remain. Unsolicited/invalid responses cannot later revive a failed session.

[catalog-protocol.mjs](../../../scripts/gui-preview/catalog-protocol.mjs) validates the display-response shape, exact identities, bounds and query membership. It is not another persisted-catalog parser or a replacement for native schema validation. [server.mjs](../../../scripts/gui-preview/server.mjs) generates mode.mjs from current reader state on each bootstrap request, rather than caching a once-successful body. Static HTML may still return 200; that is not catalog success. Invalid protocol/terminal query failures return 503; explicit bounded query refusals return a failed operation response. Busy requests remain refused without writing into another request's pending slot.

[catalog-ui.mjs](../../../scripts/gui-preview/catalog-ui.mjs) validates received identities and latches a failed read view. A failed query clears catalog identity, collections/storage panels, results and inspector/disclosures, and shows an explicit relaunch instruction. Request epochs prevent late/superseded responses from restoring cleared evidence; disclosure rerenders preserve the failed state. No old evidence is retained as the failed selection's result. Earlier validated responses are not retroactively claimed never to have existed. No picker, live reload, health polling or retry framework was added. An idle page is not continuously notified of external reader failure; the next read/bootstrap observes it. The supported recovery is fresh relaunch/reopen, demonstrated after the fault sessions with ALPHA, BETA and restarted ALPHA.

New server/browser regressions distinguish malformed initial envelopes, valid bootstrap followed by exit 7, genuine prior query/selection followed by an unterminated valid-looking query response and exit 7, malformed/numeric/unknown query IDs, and late completion behind a failed busy request. The real native `%0A` hash failure still exits 1 and now invalidates mode. Synthetic fault emitters are not described as native-runtime failures. Positive real-native controls prove that failure handling does not simply reject every reader.

## R2 - exact identity and evidence association

The supported native ID fields are signed Go `int`: collection/folder/file/chunk/volume/location IDs and their relationship references. Valid rows remain positive and unique per table. Existing absent optional relationships use zero where supported; invalid row zero/negative IDs and missing references remain refused. File size is nonnegative signed int64. No native catalog schema, range, relationship rule, persisted bytes or record ordering was changed.

[gui_catalog.go](../../../gui_catalog.go) uses strconv.Itoa directly on exact native IDs and native Search file_id values, before JSON encoding. Collection/volume/file IDs and returned search IDs are canonical decimal strings. idMax advertises the producer's native signed-int maximum (9223372036854775807 on the executed Windows/amd64 build; the code also describes a 32-bit producer's range without claiming a 32-bit runtime test). Chunk-copy identifiers continue to embed exact native chunk IDs plus a bounded positional copy index. All folder/collection/chunk-file/copy-volume/location joins occur in exact native int maps before display projection; omitted identity fields were not invented merely to change the protocol.

Recorded byte sizes use strconv.FormatInt, preserving int64 values, including zero. This directly related evidence correction avoids silently rounding recorded sizes. Other numeric fields were not globally rewritten. Native catalog files still store their original numeric fields and schema_version 8; there is no renumbering, migration, rewriting or blanket refusal of otherwise supported large IDs.

Node and browser treat IDs as opaque strings. The response validator rejects numeric values (even small numeric IDs), noncanonical decimal syntax, zero/negative row IDs, overflow, duplicate and unknown query IDs. It compares canonical strings by length/lexical order; no Number/parseInt or floating-point intermediate is used for identities. Browser lookup uses a Map keyed by exact strings after validation; no first-record, basename, index or default fallback remains. Search requests still contain only bounded text/hash; no browser ID selector or native ID-request parser exists in this slice. Static-demo IDs remain behind the existing mode boundary.

TestGUICatalogExactIntegers and the explicit TestGUICatalogCorrectionFixtures setup construct exact native values using strconv.Atoi, never JavaScript Number literals. Fixtures cover 1, 9007199254740991, 9007199254740992, 9007199254740993 and native maximum, with distinct hashes, folders, collections, volumes, locations and matching copies. The raw review reproduction is copied unchanged too. Native tests verify zero/negative/overflow refusal, exact output/search IDs and copy/location joins. Node/browser tests select every record's own checksum/source/size, reverse native file ordering, check no-match behavior and reject invalid protocol IDs. Screenshots show the review's bbbb record with /two and exact size 9007199254740993, and native maximum with its own eeee checksum, folder and copy identifier.

## R3 - bounded lines, EOF and shutdown

[stop-command.mjs](../../../scripts/gui-preview/stop-command.mjs) is a bounded byte accumulator: LF terminates a UTF-8 command, optional CR and surrounding whitespace are trimmed, and the exact case-sensitive word stop is accepted. Blank/unknown lines are ignored. A 4096-byte line is allowed; an overlong line and every suffix are discarded until LF. A final complete normalized stop at EOF is accepted; bare EOF and incomplete prefixes do not shut down the live listener. Memory does not grow with an unterminated command. Accepted stop fires once; closed parsers discard pending/future input.

The shared launcher routes stop/SIGINT/SIGTERM/timeout through its idempotent shutdown path, removes its stdin/signal listeners, closes connections and waits for its reader. Reader.close is idempotent, rejects pending work, ends stdin, waits for normal EOF exit, and uses a 500 ms termination fallback only if needed. Native successful sessions exited 0 in corrected stop tests; the deliberately delayed fault emitter required the recorded bounded SIGTERM fallback. Those are distinct outcomes.

Five parser tests explicitly control chunk boundaries, including whole/split/one-character input, stop\r then \n, multiple/unknown/repeated lines, incomplete prefixes, notstop/stoplater, EOF, overlong rejection and parser cleanup. The real spawned static and catalog CLIs each exercise five command sequences with stdin kept open through exit. Incomplete prefixes are coordinated with a successful live HTTP request; writer calls alone are not claimed to prove receiver chunk boundaries. All ten sessions exit 0 once, close their listeners and wait for owned readers. A final partial EOF-stop test and a real native EOF control also pass.

A separate focused pipe/IPC test observes actual stdin EOF, verifies both static/catalog listeners remain live, then dispatches a process SIGINT event into the real handler and checks exit/listener/reader cleanup. Its test-only signal-launcher.mjs is not served or used in normal launches. This is not an interactive Windows console Ctrl+C test. A further focused pending-request test observes a busy slot, closes the server, confirms the request settles, and records bounded termination of the delayed owned emitter in 513 ms. No broad process-name termination was used.

## Executed checks and retained artifacts

Installed tools: Go 1.26.8 Windows/amd64, Node 24.19.0 and Chrome 153.0.8010.48. The existing module/toolchain cache was reused offline with GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly and CGO_ENABLED=0. GOCACHE and TEMP/TMP use existing task output directories; GOTMPDIR was unset, not redirected to a problematic path. No downloads, installations, dependency/global-setting changes or platform switch.

Fresh corrected binary E/output/corrected-reader.exe SHA-256: `f56514e8137c74ca5b0c771c46e4a42223537a2333b0829ae32e75f6dee71c2e`. Test-only reader-double.exe SHA-256: `a482c1cc39ffb178fbbcc5c20e20d18d4d3d7aa4e73398ee9f72764d4f597aa2`. Both were built after formatting from this candidate; no historical binary was overwritten. Final source/binary hashes and copies are retained together.

| This correction's execution | Actual result / E/output artifact |
|---|---|
| Pre-fix build and exact R1-R3 replay | Builds exit 0; all three defects reproduced. baseline-build-exits.json, baseline-reproductions.json |
| Pre-fix R1/R2 browser replay | Two separate completed defect-observation runs, 2 checks each; baseline-browser-r1/, baseline-browser-r2/ |
| Corrected selected Go | 18 top-level passes, 25 subtest passes, 1 TestCatalogScale skip, no failures; go-selected.txt, go-corrected.jsonl |
| Separate exact fixture setup | 1 top-level pass; fixture-setup.jsonl. Writes completed before corrected reader/browser sessions |
| Corrected build, double build, vet ./..., formatting | All exits 0; no remaining gofmt paths; native-exits.json |
| Node parser / existing static / existing catalog | 5 / 7 / 4 passes respectively, no fail/skip; node-stop-command.txt, node-preview.txt, node-catalog.txt |
| Initial correction Node suite | 6 top-level passes, no fail/skip; node-catalog-correction.txt |
| Subsequently added EOF/signal test only | 1 pass; node-eof-signal.txt. Not a rerun of the six tests |
| Subsequently added pending-shutdown test only | 1 pass; node-pending-shutdown.txt. Not a rerun of the six tests |
| Corrected failure/late-response browser cases | Exit 3; shape 3; query 4; invalid-ID 4; late-response 5 assertions, separately recorded |
| Corrected exact-ID browser cases | Exact 12; reversed 12; original review reproduction 4 assertions, separately recorded in browser-summary.json |
| Existing static browser | 55 assertions; browser-static/ |
| ALPHA recovery / BETA / fresh ALPHA restart | 22 assertions each; browser-alpha-recovery/, browser-beta/, browser-alpha-restart/ |
| Empty / malformed, zero, missing, future, directory | 3 for valid empty; 2 per refused session; separate browser directories |
| Whitespace and preservation | Final checks and hashes in final-preservation.json/evidence-identities.json |

Go selection, enumerated before running, was `^Test(GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers)|OpenStore_|PersistObserver_|Catalog)`, executed with -count=1 -json. Directly affected native decode/refusal/current/legacy/newer-schema compatibility controls remain included. No full unrelated Go/helper/scale campaign. The fixture generator is an explicit setup test, not a reader entry point. Node selected names were retained before the suites; later focused names are explicit in their commands/logs.

All corrected browser runs use the unchanged native/browser boundary and actual served assets. New assertions are in catalog-correction-browser-checks.mjs, reached through the existing installed-Chrome/CDP harness. Failure cases cover exit, shape, query, invalid-ID and late-response paths; exact, exact-reversed and review-integers cover identity/evidence association. Runtime requests remain on each fresh loopback origin; no external/production API request or page runtime/console error was recorded in completed runs. Existing static/catalog keyboard and Back/Forward checks passed. Representative corrected failure and large-ID inspector screenshots were visually inspected at 1440 x 1024 CSS/scale 2 (2880 x 2048 PNG); no renewed Figma/theme or responsive-layout acceptance is claimed.

Original author results (17 Go top-level/20 subtests/one scale skip; Node 7/4 and separate launcher control) and completed review results remain historical evidence. They are not relabeled or summed with this correction. This pass had no failed runtime suite, compilation/setup failure or hidden retry. One apply_patch attempt was rejected because it specified delete/add operations on the same file; no edit was applied by that attempt and the patch was resubmitted as an update. Inspection-only rg glob diagnostics are not test failures. Baseline defects, intentional emitted errors, timeout/termination controls and passing corrected assertions remain separately identified.

## Changed paths and preservation

Relative to the verified 262-file pre-correction checkpoint, twelve existing paths change: gui_catalog.go, gui_catalog_test.go; scripts/gui-preview/catalog-adapter.mjs, catalog-ui.mjs, server.mjs, catalog.test.mjs, catalog-browser-checks.mjs and README.md; docs/development/CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md and REVIEW_COVERAGE.csv. Seven source/test files are added: catalog-protocol.mjs, stop-command.mjs, stop-command.test.mjs, catalog-correction.test.mjs, catalog-correction-browser-checks.mjs, testdata/reader-double.go and testdata/signal-launcher.mjs under scripts/gui-preview. This follow-up report is the eighth new path. Counts are reconciled by path in the final manifest, not used as a staging allowlist.

The original implementation/review reports, all other baseline files, main.go/store.go shared production entry/decoder bytes, static fixtures/HTML/CSS/app and historical safety/GUI records are preserved. Original Figma/PDF/ZIP and extracted PNG/reference identities are checked against retained hashes. The four existing living records receive author-response/targeted-recheck entries; no competing master ledger was introduced. The complete final working path set, including untracked additions and this report, is hashed in final-current-identities.json. candidate-identities.json and final-candidate retain the scoped source/tests/runtime assets/records; this report's hash is also recorded separately in report-identity.json to avoid a self-hash cycle.

Inputs are copied or generated separately under E/inputs and E/inputs-regression; builds, caches, browser profiles, logs and screenshots are under output. inputs-before-corrected.json and final-preservation.json compare bytes and directory entries across sessions. Read-only source/caller inspection remains applicable: the loader uses only the selected catalog's Lstat/Open/Stat/bounded read/Close, never source/media paths. Newly executed native boundary tests still observe a single selected read, discard bytes accompanying errors and observe zero persistence attempts during query/projection. This is not OS-wide I/O monitoring, transient-write tracing, ACL enforcement or concurrent-writer qualification. No real inventory, keys, application defaults, media data or backend repair was accessed.

## Paired launch and next gate

From the repository root, use the retained corrected pair:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-fix-20260920-131810'
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\inputs\alpha.json" --adapter $reader
```

Open the newly printed loopback URL. Use beta.json or empty.json for the corresponding copied control. Exact fixtures use inputs-regression/exact.json or exact-reversed.json with the same corrected native binary. Type stop and press Enter; wait for the stop message and prompt. Static launch remains `node scripts/gui-preview/server.mjs 0`. Do not pair the old numeric-ID adapter with the corrected client.

Exact executed environment/build/test/fixture/browser commands are retained in E/commands.md; the README contains current rebuild/setup instructions that use new output names. Correction fixture setup is `OBELISK_GUI_CORRECTION_FIXTURES=<new absolute directory>` followed by `go test -count=1 -run '^TestGUICatalogCorrectionFixtures$' .`, using the documented installed offline Go environment, then removal of that setup variable. Never rerun setup over retained inputs or overwrite evidence binaries.

All task-owned readers/servers/browsers are stopped and waited; intentional synthetic termination fallbacks and baseline cleanup remain explicitly recorded. Branch/HEAD/index and preservation are checked at handoff. No automatic reviewer execution or publication follows this author response. **ONE next action: targeted reviewer recheck of R1-R3 and directly affected controls.** Unrestricted/advanced native catalogs, live inventory, scale, ACLs, other platforms/Docker, recovery and hardware remain unqualified; unrelated workstreams stay separate.
