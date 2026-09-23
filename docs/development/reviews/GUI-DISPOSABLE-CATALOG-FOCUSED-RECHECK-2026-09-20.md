# Disposable catalog focused reviewer recheck - 2026-09-20

**GUI_DISPOSABLE_CATALOG_READY_FOR_OWNER_REVIEW**

| Finding | Disposition | Decisive result |
|---|---|---|
| R1: failed reader retains successful bootstrap/evidence | CLOSED | Invalid startup and terminal failure invalidate the server mode; failed browser operations clear and latch the affected view. Prior selection and late-response cases pass. |
| R2: native integers select another record's evidence | CLOSED | Exact strings preserve supported native IDs and int64 sizes before JavaScript parsing. Original reproduction, adjacent boundary IDs, maximum native ID and reversed ordering select their own evidence. |
| R3: split stop command ignored | CLOSED | Deterministic parser checks and actual static/catalog CLI runs accept split lines. Complete commands terminate naturally with stdin still open, and native readers exit 0. |

This is newly executed, same-conversation Codex reviewer checking of the corrected working candidate. It is not an independent-agent/context-isolated review, human certification, owner acceptance or publication. Only this new report was added in the repository; no correction was implemented during review. ONE next action: owner acceptance and separately authorized scoped catalog-preview publication.

## Candidate and retained evidence

Branch `feat/gui-catalog-readonly`; full HEAD and overall patch base `433cedac0a0f8a4e9dc67e3a1722b52c39c7cc6e`. Index empty. No merge, rebase, cherry-pick, revert, sequencer or index.lock marker. No branch/Git configuration change, remote lookup, staging, commit, push or merge.

New reviewer evidence root **Q**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-recheck-20260920-140643`.

Author correction root **E**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-fix-20260920-131810`.

Original substantive reviewer root **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-review-20260920-124607`.

The [implementation report](GUI-DISPOSABLE-CATALOG-IMPLEMENTATION-2026-09-20.md), [controlling review](GUI-DISPOSABLE-CATALOG-REVIEW-2026-09-20.md), [author follow-up](GUI-DISPOSABLE-CATALOG-REVIEW-FOLLOWUP-2026-09-20.md), current README and latest four living-record entries were read. The embedded earlier prompts were provenance only. No applicable AGENTS.md or previously completed catalog-focused closing recheck was found. The older static GUI focused report concerns a different milestone.

Before execution, all **270** working-file SHA-256/length identities matched E/final-current-identities.json, with no extra or missing working paths. This includes untracked source/tests/reports and the explicitly inventoried ignored design ZIP. The author follow-up is included in that manifest and also separately identified: 20,071 bytes, SHA-256 `3044bfa57a62252ac3ae23a823ed4392ff82490dc4f24ce7e7a114beda4687b4`. Q/before.json and candidate-before.json retain the complete/scoped manifests; Q/source contains hash-verified copies of all 270 files. Branch/HEAD are not substitutes for these uncommitted bytes.

Starting Git state contains 11 modified tracked files, 16 untracked candidate source/test/report files, and two pre-existing untracked design files. Q/status-before.txt lists every path. Q/tracked-vs-HEAD.patch and new-source-vs-HEAD.patch together describe the overall source/evidence patch against HEAD; the untracked PDF/Figma are inventoried, not included as source diffs. The published parent alone cannot reproduce the uncommitted catalog findings.

The correction relative to E/baseline is separately retained in Q/reviewed-correction.patch, alongside the copied E/correction.patch. All 262 baseline files still match E/before.json. The delta is exactly **12 changed, eight added, zero missing, 250 preserved**:

- Changed: gui_catalog.go, gui_catalog_test.go; scripts/gui-preview/{catalog-adapter.mjs, catalog-ui.mjs, server.mjs, catalog.test.mjs, catalog-browser-checks.mjs, README.md}; docs/development/{CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md, REVIEW_COVERAGE.csv}.
- Added: scripts/gui-preview/{catalog-protocol.mjs, stop-command.mjs, stop-command.test.mjs, catalog-correction.test.mjs, catalog-correction-browser-checks.mjs, testdata/reader-double.go, testdata/signal-launcher.mjs}; the author follow-up report.

These counts reconcile identities; they are not a staging list. main.go, store.go, app.mjs, static fixtures/HTML/CSS and unrelated source did not change in the correction or this recheck.

The retained red evidence is sound and was reused, not rerun as a historical campaign. E/baseline-repro.mjs imports the verified uncommitted baseline server. E/output/baseline-reproductions.json records R1 ok:true/503/reader exit 7, R2 bbbb selecting aaaa's /one/123 evidence, and R3 remaining alive after fragmented stop before a separately labeled whole-command cleanup. E/output/baseline-browser-r1 and baseline-browser-r2 retain actual defect DOM/screenshots; the latter shows record 9007199254740992 answering bbbb with aaaa. E/output/baseline-reader.exe and baseline-double.go/exe are retained, with commands and source linkage in E. The baseline double is the copied original reviewer mechanism. No compilation failure was substituted for a red runtime result. Fresh green fault injection uses the explicitly identified corrected-protocol double below.

## Fresh execution and source linkage

Installed tools: Go **1.26.8 windows/amd64**, Node **24.19.0**, Chrome **153.0.8010.48**. Q/native.ps1 retains the exact commands and process environment: installed audit go-mod/toolchain path, GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0, Q/output/go-cache and go-temp as GOCACHE/TEMP/TMP, GOTMPDIR unset. No download, installation, global change or platform switch.

Fresh build from the verified active checkout: `go build -o Q/output/review-reader.exe .`. SHA-256 **`f56514e8137c74ca5b0c771c46e4a42223537a2333b0829ae32e75f6dee71c2e`**, 14,273,024 bytes. Matching the retained author binary hash is a result of this new successful build, not reuse of that binary. Q/output/binary-identities.json records it.

The controlled fault executable is freshly compiled with `go build -o Q/output/reader-double.exe scripts/gui-preview/testdata/reader-double.go`, SHA-256 **`a482c1cc39ffb178fbbcc5c20e20d18d4d3d7aa4e73398ee9f72764d4f597aa2`**, 3,295,744 bytes. It reads only its prepared synthetic projection and emits documented faults. It is not the native catalog reader. All server/browser assets are the unchanged verified current candidate.

Q/inputs and inputs-regression are nonoverwriting copies of the author's prepared controls: **26 files plus one empty directory obstruction**. No fixture generator ran in this recheck. The original review unsafe-integers.json bytes are preserved. Independent reviewer parsing uses JSON source lexemes and BigInt, with literal decimal expectations, to audit the copied native numeric fixtures; it never derives expected IDs from rounded Numbers. Q/inputs-before.json, preservation-pre-report.json and final-preservation.json record copies, hashes, names and directory types.

| Fresh reviewer check | Result | Artifact under Q/output |
|---|---|---|
| Selected Go tests, enumerated first | 18 top-level passes; 25 subtest passes; one TestCatalogScale skip; zero failures | go-selected.txt, go-tests.jsonl, native-exits.json |
| Fresh reader/double builds; vet ./... | All exits 0 | build.txt, double-build.txt, vet.txt |
| Read-only gofmt -l and ordinary git diff --check | Exits 0; no formatting/whitespace findings | gofmt.txt, whitespace.txt |
| Real stop parser suite | 5 passes, no fail/skip | node-stop-command.txt |
| Existing static Node suite | 7 passes, no fail/skip | node-preview.txt |
| Existing catalog Node suite | 4 passes, no fail/skip | node-catalog.txt |
| Complete current correction Node suite | 8 passes, no fail/skip | node-catalog-correction.txt |
| Supplemental reviewer probe, corrected instrumentation revision | Three exact-fixture/raw-native audits, six parser boundary cases, three R1 response captures, eight helper/selection route refusals; exit 0 | reviewer-probes-v2.json/.txt, raw-v2-*.jsonl |

Go selection: `^Test(GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers)|OpenStore_|PersistObserver_|Catalog)`, executed with `-count=1 -json -run` and preceding `-list`. It selects 19 top-level names including the explicit opt-in scale skip. Native read/refusal/lifecycle, current/legacy/newer-schema compatibility and persistence-observer controls remain included. No full unrelated suite, scale override, race or cross-platform campaign.

Q/node.ps1 records `node --test --test-isolation=none scripts/gui-preview/<name>.test.mjs` for stop-command, preview, catalog and catalog-correction, with explicit reviewer input/reader/double environment variables. Q/output/node-selected.txt enumerates test names before execution; node-exits.json retains exits. Top-level tests are not combined with Go subtests or browser assertions.

Q/browser.ps1 records all exact Chrome harness invocations, fixtures, expectations and new evidence directories. Every session completed exit 0, errors [], server stopped, browser stopped and waited:

| Fresh browser session | Assertions passed |
|---|---:|
| failure-exit / failure-shape | 3 / 3 |
| failure-query / failure-invalid-id / failure-late | 4 / 4 / 5 |
| exact / exact-reversed / original review-integers | 12 / 12 / 4 |
| static | 55 |
| ALPHA recovery / BETA / ALPHA restart | 22 / 22 / 22 |
| valid empty | 3 |
| malformed / future / missing | 2 / 2 / 2 |

These are **16 separate browser sessions**, not repetitions borrowed from author results. Per-session browser-results.json, catalog-process.json, DOM observations, request logs and PNGs reside in Q/output/browser-<case>/. Browser requests remained on each selected loopback origin with no production API or runtime/console exception recorded. Catalog screenshots use 1440x1024 CSS pixels at scale 2 (2880x2048 PNG). Only the static branch also executes the 390x844 smoke check: the harness lists both possible sizes in every result, which is not evidence that every branch used both. No Figma comparison or new visual-fidelity acceptance was performed.

## R1: CLOSED

[catalog-adapter.mjs](../../../scripts/gui-preview/catalog-adapter.mjs:20) invalidates mode and pending work on detected terminal failure, validates complete newline-terminated envelopes, bounds accumulated bytes and receive time, and exposes current mode through a getter. [server.mjs](../../../scripts/gui-preview/server.mjs:36) generates mode.mjs per request. [catalog-ui.mjs](../../../scripts/gui-preview/catalog-ui.mjs:67) checks HTTP/envelope/IDs, uses request epochs, and clears the entire catalog view on an affected current failure; selection is cleared and failure latches across disclosure rerender.

The equivalent controlled bootstrap fault emits a complete valid corrected-protocol projection, then exits 7 after 100 ms. That initial projection may genuinely validate before exit; the test does not relabel it as invalid from inception. Once failure is detected, the mode has ok:false/no catalog and queries return 503. Fresh browser loading shows a failure/relaunch notice with no catalog identity, collections, results, inspector or static fallback. A shape-only ok:true envelope, partial output, timeout and unsafe numeric startup are refused rather than adopted as valid empty catalogs.

The query-failed and invalid-ID browser cases first obtain two valid controlled results, select aaaa and open its copy disclosure. The newly requested bbbb operation then fails. The query-failed double prints valid-looking JSON without the required terminating LF and exits 7: it cannot become a committed successful response. The browser clears previous identity, selection and evidence and requests relaunch. Q/output/reviewer-probes-v2.json independently captures prior mode/query, affected HTTP 503/body, failed mode and actual exit. The real freshly built reader also reproduces the native `%0A` hash termination: prior aaaa query succeeds, then HTTP 503, mode false, exit 1 with `invalid hash`.

The late-response browser control coordinates an observed delayed request and a newer busy/refused operation. After all observed responses settle, the failed view remains cleared; Expert rerender cannot restore it. A busy refusal does not falsely imply that a valid in-memory snapshot was never loaded or that the reader necessarily died. The affected browser view still reports its failed operation. No automatic polling or idle-page push notification is implemented or required: a new bootstrap/read observes terminal failure. Successful HTML-shell HTTP 200 alone is not a catalog-success label. Fresh real-native ALPHA, BETA and ALPHA relaunch sessions verify the supported recovery path.

Visually inspected representative screenshot: Q/output/browser-failure-query/failure-query.png. It shows the explicit previous-results-cleared/relaunch notice and no retained inspector. Other initial/invalid/late error screenshots and before/after DOM evidence are retained beside their runs. These observations and protocol assertions, rather than screenshot appearance alone, support closure.

## R2: CLOSED

The native types in [store.go](../../../store.go:107) use signed Go int IDs/references for the supported collections, folders, files, chunks, volumes and locations; File.SizeBytes is int64. Required row IDs must be positive and unique. LocationID zero remains the supported unassigned sentinel. Native relationship checks and joins use exact ints. On this Windows/amd64 execution, native max is 9223372036854775807.

[gui_catalog.go](../../../gui_catalog.go:235) emits `strconv.Itoa` IDs and `strconv.FormatInt` sizes before JSON crosses into Node. Native Search still returns int identities, converted directly to strings for query responses. Collection/folder/chunk membership, volume and location associations are resolved in Go; copy IDs include the exact native chunk ID plus bounded positional copy index. Persisted catalog bytes/schema and supported range are not rewritten, renumbered or restricted to JavaScript-safe IDs.

[catalog-protocol.mjs](../../../scripts/gui-preview/catalog-protocol.mjs:4) checks canonical decimal strings by length/lexical range, known membership and uniqueness; numeric, malformed, duplicate, unknown and out-of-range identities are refused. Both Node and browser use that contract. Browser state/Map keys/DOM labels retain strings. Query URLs serialize only text/hash, decoded as native strings; there is no numeric ID request parser or lossy conversion hidden in a request path. Static demo identities remain behind their existing separate mode boundary.

Fresh native raw output, Node transport and actual browser selection cover 1, **9007199254740991**, **9007199254740992**, **9007199254740993**, and **9223372036854775807**, plus the original two-record reviewer reproduction. Literal expected hashes a-e, source folders, sizes and related exact joins are independently checked against native JSON lexemes. Exact and reversed-file-order fixtures both select each record's own evidence; no-match clears selection rather than defaulting to a neighbor. Invalid zero/negative/native-overflow inputs are covered by Go; numeric/leading-zero/exponent/duplicate/unknown/out-of-range protocol IDs are explicitly refused by current Node tests, with invalid numeric selection failure also exercised in the browser.

Q/output/raw-v2-{exact,exact-reversed,review-integers}.jsonl retains unparsed fresh executable output and query IDs. Q/output/browser-review-integers/exact-inspector-9007199254740993.png was visually inspected: bbbb selects record 9007199254740993 with its /two source and exact 9007199254740993-byte size. Q/output/browser-exact and browser-exact-reversed retain maximum/adjacent-ID screenshots and every observed query/inspector association. No blanket large-ID refusal substitutes for correct association.

## R3: CLOSED

[stop-command.mjs](../../../scripts/gui-preview/stop-command.mjs:3) uses a fixed 4096-byte line buffer, LF delimiter, optional trimmed CR/whitespace, whole-overlong-line rejection and once-only callback. EOF accepts a final complete normalized stop; bare EOF/incomplete prefixes do not request shutdown. [server.mjs](../../../scripts/gui-preview/server.mjs:58) shares that parser across static/catalog launches, removes handlers, closes connections and waits for reader EOF, with a documented 500 ms termination fallback.

The five deterministic parser tests control actual parser.write chunk boundaries: whole, st/op, single characters, CR/LF split, blank/unknown/multiple/repeated lines, incomplete st, notstop/stoplater/case mismatch, EOF and close behavior. The reviewer probe additionally checks exactly 4095, 4096 and 4097 bytes with both LF and EOF; at-limit stop succeeds, over-limit command-looking suffix is rejected, and a subsequent genuine line succeeds. This is distinct from process writes, whose receiver chunk segmentation is not claimed to be deterministic.

The actual CLI integration runs five sequences in each mode, ten runs total. All exit 0 naturally, report `stdin stop` once, retain stdin open through exit and refuse a subsequent listener fetch. Incomplete-prefix/CR checks coordinate with live HTTP responses before the final chunk. Native catalog readers all exit 0 and are waited. Recorded elapsed times including startup are 147-403 ms; decisive split static/catalog runs are 164/403 ms. No eight-second fallback supplied EOF or cleanup to rescue those commands.

Final partial stop at EOF and native reader EOF exit 0. Separate bare-EOF cases observe the actual stdin end event, prove HTTP still works, then dispatch a simulated process SIGINT event through the existing real handler using the test wrapper; both modes exit 0. This is pipe/IPC testing, not interactive Windows console Ctrl+C certification. A pending-query shutdown control proves the slot busy, settles the affected connection and terminates/waits the intentionally delayed double after 513 ms with SIGTERM. That is the documented bounded fallback, not a graceful native stop. Existing static Node/browser harness cleanup also deliberately kills and waits its owned process; those forced cleanup outcomes are not counted as natural command success.

## Preservation, attribution and limits

All 270 pre-existing candidate files remain byte-identical. The sole repository addition is this report, giving 271 inventoried paths; no prior path is changed or missing. The four living records retain their author-addressed entries unchanged; this report supplies reviewer disposition without rewriting history. Q/final-current-identities.json and final-preservation.json reconcile the full path set and Git state. Q/report-identity.json separately hashes this report, avoiding a self-hash cycle. Q/evidence-identities.json inventories retained reviewer scripts, source, logs, binary identities and screenshots while excluding disposable caches/profiles/temp directories.

Hash preservation independently checked the 262-file retained baseline, original author evidence through R/author-before.json (5791 paths), original reviewer evidence manifest (313 paths), correction evidence manifest (527 paths), and seven extracted PDF/PNG/reference entries. No mismatch. Original Figma remains untracked, 219717 bytes, SHA-256 `69c5dd577d262c782ae657ac9538cf4c38530c6ed9af7f79beaeed87f92fddda`; PDF remains untracked, 3698553 bytes, SHA-256 `3c17b2b53335c2cf1e5d69bd616e4d6e3fa4613cf0dad0dfb21b0b786c8b7a86`; ignored ZIP remains 3888214 bytes, SHA-256 `72a1f3308e2c4421b62c8bf286f9fab7651393a1dbbbbe1520774feefb331aca`. No design conversion, upload, media or production-data access occurred.

All 26 copied input files retain their initial hashes, with exactly the same 27 directory entries, empty obstruction and absent missing.json. Source inspection confirms only selected-input Lstat/Open/Stat/bounded ReadAll/Close, with no source/media-path access, save or production startup. [main.go](../../../main.go:58) returns through the explicit reader command before defaults/App/OpenStore. The pure [native decoder wrapper](../../../store.go:1159) is unchanged by the correction. Fresh lifecycle tests assert the selected read and no persistence during search/projection. This combined evidence supports the bounded read-only claim; final hashes alone do not prove OS-wide I/O, transient-write absence or ACL enforcement.

Loopback binding, ephemeral ports, exact Host, methods/query limits, allowlisted assets, same-origin CSP and pending-response isolation remain intact. Existing isolation tests plus eight reviewer changed-helper/selection-route controls refuse code/evidence access and browser-selected input/executable parameters. No production registration, scan, repair, schema expansion, inventory access or live operation exists in this slice. Advanced populated sections remain refused; capacity, accessibility, parity and current verification remain unavailable. Fresh ALPHA/BETA controls include skip-link main focus/state preservation, real keyboard navigation, disclosures, no-match clearing and Back/Forward. This was not a full repeat of the original integration audit or Figma review.

Historical results remain separately attributed: original implementation final Go 17 top-level/20 subtests/one scale skip, Node static 7/catalog 4 plus separate launcher check, and its separately recorded browser sessions; original substantive reviewer Go 17/20/one skip, Node 7/4, browser/protocol/native-bound campaigns and three defect observations remain in their reports. Author correction Go 18/25/one skip; Node parser 5/static 7/catalog 4/correction 6 followed by two separately added one-test passes; author browser static 55, ALPHA/BETA/restart 22 each, empty 3, five refused inputs 2 each, failure 3/3/4/4/5 and exact 12/12/4 remain author evidence. The author did not run a final all-eight correction suite; this reviewer did. No overlapping historical/new counts are summed.

One supplemental reviewer instrumentation attempt failed: reviewer-probes.mjs expected ['1'] from the controlled double's aaaa request, although its documented control emits ['1','2']. It exited 1 after the earlier raw-ID/parser checks, not because of a candidate defect. The unchanged first script/log/exit/raw outputs are retained. reviewer-probes-v2.mjs corrects that expectation and writes separate output names; its complete run exits 0. No repository test/source was changed. Inspection diagnostics also included a nonexistent types.go/results.json lookup and sandbox-denied process enumeration; the actual store.go/browser-results.json were inspected and authorized process enumeration succeeded. These are not concealed runtime failures or approval-review rejections. Git line-ending advisory output did not modify files. All 20 correction whitespace checks emit no findings; their no-index exit 1 denotes expected differences, while ordinary diff --check exits 0.

All positively identified task readers/servers/browsers are stopped and waited; completed harness records and the final task-owned process query show no remaining task process. No broad name-based termination was used. Essential focused checks are complete. Remaining limits are the documented supported subset, Windows/amd64 and installed browser only, pipe/IPC signal coverage, no production scale/race/ACL/concurrent-replacement/platform/Docker/helper/hardware qualification, and no renewed visual acceptance. These limits do not reopen R1-R3 or authorize a production catalog trial.

**Stop for owner review. ONE next action: owner acceptance and separately authorized scoped catalog-preview publication.**
