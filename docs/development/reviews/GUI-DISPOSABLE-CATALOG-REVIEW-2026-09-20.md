# Disposable catalog substantive review - 2026-09-20

**NEEDS_CHANGES**

Three findings require one bounded correction pass before a targeted recheck. The normal synthetic workflow and directly affected compatibility checks pass, but failure adoption, native integer identity, and stdin command framing are not correct across the reviewed boundaries. No fixes were implemented.

This is same-session Codex source inspection and newly executed reviewer checks following the implementation in this conversation. It is not an independent-agent review, human certification, owner acceptance, or publication. The submitted Prompt 12 is retained as `review-prompt.txt`; Prompt 11 was read as provenance, not executed again. No applicable AGENTS.md was found in the repository or its ancestor directories. No matching completed catalog review existed when this review began.

## Candidate and evidence linkage

Branch: `feat/gui-catalog-readonly`. Full HEAD and complete patch base: `433cedac0a0f8a4e9dc67e3a1722b52c39c7cc6e`. No branch switch, Git operation, or network lookup. Index empty; no merge/rebase/cherry-pick/revert/sequencer operation or index.lock found.

Reviewer evidence root **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-review-20260920-124607`.

Author evidence root **A**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-20260920-120412`.

All 21 paths in A/candidate-identities.json matched the working bytes before execution, including the new/untracked Go and JavaScript files, unchanged runtime assets, and implementation report. R/candidate-before.json records their exact SHA-256/length identities; R/source retains those bytes. The implementation report is included, not excluded from that manifest: SHA-256 `4148115b01ed2a265e35620d0f6fea51707412dc5007a7ce19f542aa9aef8a1e`. No material candidate drift was found.

The complete patch against the specified base comprises these paths:

| Existing modified paths (11) | New candidate paths (7) |
|---|---|
| main.go; store.go | gui_catalog.go; gui_catalog_test.go |
| scripts/gui-preview/app.mjs; server.mjs; browser-check.mjs; preview.test.mjs; README.md | scripts/gui-preview/catalog-adapter.mjs; catalog-ui.mjs; catalog-browser-checks.mjs; catalog.test.mjs |
| docs/development/CODEX_HANDOFF.md; NEXT_ACTIONS.md; OB_STATUS.md; REVIEW_COVERAGE.csv | docs/development/reviews/GUI-DISPOSABLE-CATALOG-IMPLEMENTATION-2026-09-20.md |

These counts are reconciliation, not a staging allowlist. R/tracked-diff.patch and R/new-path-diffs.patch cover the complete patch, including untracked additions; R/status-before.txt records all untracked paths. Untitled.pdf and docs/Obelisk.fig are unrelated pre-existing untracked design inputs. The original design ZIP remains ignored under the existing rule. None was treated as a clean-worktree prerequisite.

The four current living-record additions consistently identify an author candidate awaiting substantive review. Their older pending/publication statements are historical. They were read and preserved, as were the implementation report and README.

## Findings requiring correction

### R1 - P2: reader failure does not invalidate the successful bootstrap

At [catalog-adapter.mjs:15](../../../scripts/gui-preview/catalog-adapter.mjs:15), process failure sets `dead` and rejects a pending request, but leaves the adopted mode successful. [server.mjs:18](../../../scripts/gui-preview/server.mjs:18) serializes that mode once into a permanent asset. [catalog-ui.mjs:28](../../../scripts/gui-preview/catalog-ui.mjs:28) relies on that success flag to display catalog identity/collections/storage. Query failure clears result selection but does not invalidate those other success representations.

Reproduction: the reviewer-owned protocol-double.exe prints a correctly shaped success envelope, then exits 7 after 100 ms. After waiting 250 ms, the actual server still serves `ok:true` from /mode.mjs, while /catalog-query returns 503 and the recorded child exit is 7. A fresh real browser shows the collection `CONTROLLED FAILED ADAPTER`, a successful catalog identity, and a query-unavailable message, with no failed-load alert or explicit stale/previous-snapshot labeling. See R/output/protocol-probes.json and R/output/browser-failed-exit-repro/{observed.json,failed-reader-success-bootstrap.png,catalog-process.json}.

This is also corroborated with the fresh real native reader: `/catalog-query?hash=%0A` passes the server's length checks and causes [gui_catalog.go:302](../../../gui_catalog.go:302) to return `invalid hash` and exit 1. The query becomes 503, but mode.mjs remains `ok:true` with ALPHA data. R/output/real-reader-failure.json retains the actual query, mode and native exit. This is a synthetic protocol input, not a production-data trial.

Related envelope probe: a double returning only `{"ok":true}` is also adopted as success despite lacking a catalog. The browser renderer dereferences data.digest without validation. This shape case was observed at the real server boundary; the resulting browser exception is source-inferred, not claimed as a separately executed screenshot test. Partial JSON, malformed JSON and no-output timeout were correctly refused.

Expected: invalid startup output or a failed reader session must not continue to be presented as a successful current catalog. The smallest complete correction is to validate the narrow response envelope, propagate terminal reader failure into authoritative server mode and browser state, and either clear all success-only catalog panels or explicitly label a retained snapshot stale/unavailable under a documented contract. A startup delay alone cannot handle later failures. Include failed-exit-after-success, malformed success envelope, real reader termination and browser state regressions. Do not duplicate the native schema in JavaScript or broaden the reader.

### R2 - P2: accepted native integers lose identity and precision in JavaScript

[gui_catalog.go:149](../../../gui_catalog.go:149) checks positive/unique native int IDs, with no JavaScript-safe bound. [gui_catalog.go:271](../../../gui_catalog.go:271) and the search response serialize IDs and int64 sizes as JSON numbers. JSON.parse in the Node bridge rounds them before forwarding; [catalog-ui.mjs:69](../../../scripts/gui-preview/catalog-ui.mjs:69) then uses strict numeric equality and Array.find to choose evidence.

R/probe inputs with spaces/unsafe-integers.json is a small schema-8 derivative of ALPHA with no chunks and two distinct file IDs: 9007199254740992 and 9007199254740993. Their hashes remain a*64 and b*64; folders remain /one and /two. The second size is 9007199254740993. The actual native reader accepts it and emits the exact second ID for `{"hash":"bbbb"}`; see R/output/native-unsafe-integers.jsonl, whose raw bytes are retained separately from parsed summaries.

The Node/browser projection instead contains the same ID, 9007199254740992, for both files and rounds the second size. The hash query selects the first file's aaaa checksum, /one source folder and 123-byte evidence. This is not merely cosmetic formatting. R/output/integer-projection.json and R/output/browser-integer-repro/{observed.json,wrong-record-selected.png} reproduce the incorrect association through the unchanged server/UI and fresh native executable.

Expected: distinct accepted native records and recorded sizes stay exact, or the input is explicitly refused before any projection is adopted. The smallest bounded correction can reject projected native IDs/references/sizes outside the JavaScript safe-integer range and document that subset; alternatively use exact strings consistently through projection, queries and matching. No native persisted-format migration is needed. Test adjacent unsafe IDs, exact boundary values, oversized byte counts, and a bbbb query selecting its own evidence or a complete load refusal.

### R3 - P2: the documented stop command is parsed as a pipe chunk

[server.mjs:66](../../../scripts/gui-preview/server.mjs:66) compares each `data` event's trimmed contents with `stop`, without buffering a command line. Stream chunk boundaries need not coincide with commands.

R/lifecycle-isolation.mjs launches the actual catalog CLI with input and executable paths containing spaces, writes `st`, waits 100 ms, writes `op\n`, and waits 300 ms. The launcher remains alive. A subsequent whole `stop\n` plus EOF exits 0 and closes the listener. R/output/lifecycle-isolation.json records both attempts, child PIDs/exits and cleanup. The initial fragmented command was not a successful stop. This does not require an owner-selected malicious executable or same-user interference.

Expected: the documented stop line works regardless of pipe delivery boundaries. Add bounded line buffering (including a clear EOF policy for pending input) and tests for whole, split and multiple delivered lines. Preserve the existing static and catalog launch contracts. Bare EOF currently does not request shutdown; the separate static-EOF probe remained alive and was forcibly stopped/waited by its known PID. Bare EOF alone is not a separate blocker because the README instructs callers to send stop before closing stdin.

## Source-anchored implemented contract and dispositions

Native representation is `catalog` in [store.go:1026](../../../store.go:1026), JSON schema_version 8. [main.go:58](../../../main.go:58) handles the explicit first argument --gui-catalog-readonly and returns before defaultDataDir, production flags/configuration, App/OpenStore, HTTP routes, jobs or workers. No package init function was found in the repository. The only shared decoder change is the exact json.Unmarshal wrapper at [store.go:1159](../../../store.go:1159); ordinary openStore classification, initialization, migration, recovery/save and jobs loading remain unchanged. Directly affected compatibility checks pass.

The native input boundary at [gui_catalog.go:27](../../../gui_catalog.go:27) uses Lstat, refuses nonregular/link input, opens with os.Open (read-only), checks the opened file, reads at most 4 MiB + 1, and closes it. It never creates a directory, lock, sidecar, backup or replacement. loadGUICatalog discards any bytes accompanying an I/O error, shares native decoding, checks schema and explicit file sizes, validates before constructing a private read-only Store, and installs a refusing failSave guard. Fixture generation is a separate test-only setup entry point; no browser/reader call reaches it. OpenStore is deliberately not called: inspection confirms its MkdirAll, migration, seeding/recovery/save and jobs side effects are inappropriate here.

Accepted populated tables are collections, folders, current files, nonspanned chunks/copies, volumes and locations, plus schema_version and next_id. Null/omitted/empty top-level slices represent no rows. Nonempty keys, burn_queues, drift, dock_sessions, volume_snapshots, events, templates, conflicts_review, plans, tape_checks, profiles, assignments, protection, audit, quarantine and backup_sessions are refused. Populated file versions, nonzero event_id, spanned chunks or populated segments are refused too. IDs must be positive/unique per table; pointer rows nonnull; required relationships valid; sizes explicit/nonnegative; nonempty file hashes must be SHA-256 hex with algorithm sha256. Historical chunk memberships not matching current file/hash/collection are not claimed as current-content copies. Unknown JSON members/duplicate JSON keys retain native decoder behavior, not a new strict-JSON policy.

Limits are enforced before adoption: 4 MiB catalog bytes; 1000 file rows/individual slices; 100 rows per ancillary table; 1000 potential membership-copy combinations; reflected string values 4096 bytes. next_id is ignored by projection and its map keys are not traversed by the string-limit helper: do not interpret the string check as a universal JSON-member limit. The total input byte bound still applies. Native stdin scanning has a 4096-byte token buffer; search accepts text up to 256 UTF-8 bytes and hash up to 64 bytes. Server URL/query limits and unsupported keys/duplicates are additional restrictions. At/over boundaries were exercised without overriding the guarded scale skip.

runGUICatalog emits one JSON-line startup projection and then query responses. It calls existing [Store.Search](../../../store.go:2145), which uses trimmed case-insensitive relative-path substring and SHA-256 prefix filters combined as AND, excludes retired collections in default searches, and returns at most 1000 matches. Search, locationsMapLocked, chunkPhysCopies, fileProtectionLocked, resolveProfileLocked, payloadName and VerifiedCopyCount were traced as in-memory computations. Only IDs from Search are exposed; computed protection/parity totals are not treated as recorded facts. Paths may be lexically joined/normalized but are not statted or opened.

Node selects absolute catalog/executable arguments at launch and checks the input parent resolves beneath ObeliskDev. spawn receives an argument array without a shell. No browser query or catalog value selects an executable/input. Five-second receive timers refuse and terminate a stalled reader. stdout lines are limited to 16 Mi characters after readline has assembled them, not by a streaming byte cap; stderr retains 4096 characters. No hostile-executable memory-containment claim is supported or required here. Envelope/terminal failure propagation has R1; native-to-JavaScript numeric projection has R2. Busy requests are rejected before writing into another pending slot; existing concurrent-query controls pass.

The server binds 127.0.0.1 with port 0 supported, checks the exact Host, and serves only /, app.mjs, fixtures.mjs, style.css, catalog-ui.mjs and generated mode.mjs. The only dynamic catalog query accepts bounded text/hash. Raw input/executable/source/evidence/design routes are absent. Static CSP blocks connections; catalog CSP permits same-origin requests only. Existing method/Host isolation plus reviewer raw encoded traversal and executable/input-selection probes passed. None read a real protected file.

Supported display fields are collection ID/name/retired; volume label/kind/recorded location; current file ID/collection/relative path/source-folder text/bytes/hash algorithm/hash/first-seen; matching chunk name, copy identifier/path/volume/location/superseded and recorded verification pointers/time. Missing supported values and zero observation times are unavailable. Session load time and catalog digest are distinct from stored observations. No current capacity/accessibility/parity, automatic original/reference, deduplication, registration, source verification or media action is introduced. Static demo remains separate; other catalog workspaces explicitly lack projections. Find and catalog layouts are functional adaptations, not new Figma fidelity claims.

Library/Find snapshot selection, native search, no-results clearing, equal names in distinct folders, inspector/copy disclosures, skip-link main focus, Tab/Shift+Tab and Back/Forward pass for ordinary ALPHA/BETA inputs. Markup and a synthetic file-URL string remain literal text; browser request logs contain only the selected loopback origin and no production API or page runtime/console error in completed runs. The expected notice matches the bounded native read path: "Disposable catalog mode — reading a synthetic catalog snapshot. No source/media files are opened. The selected catalog is not modified."

Snapshot bytes are read once per launch; queries use memory. There is no reload/picker. Shutdown closes server connections, calls reader.close and waits. reader.close ends stdin and immediately kills a still-live reader; observed SIGTERM exits must not be described as natural native EOF completion. The whole-command launcher stop itself exits 0 and its listener closes. The command-framing limitation is R3. Existing browser/static harnesses also use positively identified forced termination in their cleanup; those are stopped/waited controls, not manual Ctrl+C certification.

## Reviewer execution and artifacts

Tools actually used: installed Go 1.26.8 Windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. The README's installed toolchain/module cache was used directly, with GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly, CGO_ENABLED=0; reviewer-owned GOCACHE/GOTMPDIR/TEMP/TMP. No downloads, installation, global settings or platform switch. The fresh reader built from the matching checkout has SHA-256 `39db3ced3b24f6df312cba7eaa47288f7746f0fc31176c9b525c8c11016db978`. It is R/output/obelisk-gui-reader.exe. The spaced-path copy is byte-identical. The separate protocol-double.go/exe is clearly reviewer instrumentation, not the candidate binary.

The six author fixture files were copied without overwriting into R/inputs, with an empty directory.json obstruction and missing.json absent. Their native bytes/provenance were checked against guiFixture and the author records. No regeneration was necessary. Reviewer-only derivatives and small at/over cases were prepared separately in R/probe inputs with spaces before any reader sessions; exact setup/case expectations and pre/post hashes are retained. URL-like text used a separately prepared R/url-inputs/url.json. No real source, share, drive, keyring or production catalog was used.

| Newly executed reviewer check | Result / artifact under R/output |
|---|---|
| Selected Go tests, enumerated before execution | 17 top-level passes; 20 subtest passes; 1 TestCatalogScale skip; 0 failures. go-selected.txt, go-tests.jsonl, native-exits.json |
| Fresh build, vet ./..., read-only gofmt -l | All exits 0; no formatting paths. build.txt, vet.txt, gofmt.txt |
| Existing static Node suite | 7 passes, no fail/skip. node-static.txt |
| Existing catalog Node suite | 4 passes, no fail/skip. node-catalog.txt |
| Static browser | 55 assertions pass. browser-static/ |
| ALPHA, BETA, fresh ALPHA restart | 22 assertions pass per separately launched session. browser-alpha/, browser-beta/, browser-restart/ |
| Valid empty browser | 3 assertions pass. browser-empty/ |
| Malformed, zero, missing, future, directory browser | 2 assertions pass per session. browser-malformed/, browser-zero/, browser-missing/, browser-future/, browser-directory/ |
| Reviewer native subset/bound controls | 57 declared accept/refuse cases match expectations; one unsafe-integer defect observation; five additional query-bound/termination observations. native-probes.json and raw native-*.jsonl |
| Reviewer protocol doubles | Six sessions: partial/malformed/timeout refuse; successful control queries; failed-exit and missing-catalog-envelope expose R1. protocol-probes.json |
| Reviewer browser defect probes | Two checks each (defect observation plus request/error isolation), confirming R1/R2, not acceptance passes. browser-failed-exit-repro/, browser-integer-repro/ |
| Reviewer URL-like string browser | 2 checks pass; literal URL/markup text and page isolation. browser-url/ |
| Reviewer actual-reader terminal failure | Exit 1, query 503, stale successful mode corroborates R1. real-reader-failure.json |
| Reviewer launcher/isolation | Whole stop + EOF works with spaced paths; split stop exposes R3; bare static EOF requires identified forced cleanup. Eight raw traversal/selection/UTF-8-limit probes return expected refusal codes. lifecycle-isolation.json |
| Whitespace | git diff --check exit 0. whitespace.txt |

Go selection was `^Test(GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle)|OpenStore_|PersistObserver_|Catalog)`, run with -count=1 -json. The selected names, rather than only totals, are in go-selected.txt. This includes native read-error/empty/invalid/current/newer-schema, initialization and recovery-save controls and current/legacy migration round trips. No full unrelated Go suite, platform campaign or scale qualification was run. Reviewer fixtures were not generated by replaying TestGUICatalogGenerateFixtures.

Commands and environment are retained in R/commands.md. Reviewer additions are R/review-probes.mjs, lifecycle-isolation.mjs, real-failure-and-url-setup.mjs, output/protocol-double.go, review-browser.mjs/review-browser-checks.mjs and url-browser.mjs/url-browser-checks.mjs. The external browser harness derives from the byte-verified candidate harness, changing its import locations and substituting explicitly named reviewer assertions; it still drives the unchanged candidate server/UI. It is not described as an exact unmodified suite replay. Native probing stores raw stdout before JSON parsing so the numeric defect cannot be hidden by the observation tool's own rounding.

Screenshots captured at 1440 x 1024 CSS pixels, scale 2 (2880 x 2048 PNG). Static harness additionally captures 390 x 844, scale 1. Its shared results metadata lists that mobile size in catalog runs too, but catalog runs actually stayed at desktop size. Library/Find/inspector, empty/refused and defect screenshots were captured; representative ALPHA Library, BETA Find, empty, malformed and both defect images were visually inspected. Long Library inspector content extends below the viewport; full selected text is retained in the DOM observations where needed. No renewed pixel-perfect reference review or unprovided responsive design is claimed.

The only interrupted reviewer preparation was an initial diff-save command: PowerShell treated Git's existing line-ending warning as a terminating error under ErrorActionPreference=Stop. Candidate/source/fixture copies had completed. The diff was saved subsequently with normal native stderr handling; no runtime tests were started by the interrupted command. Missing AGENTS.md and two overly broad rg glob diagnostics were inspection attempts, not test failures. git diff --no-index returns 1 for expected new-file differences; that is not a failing validation. No runtime retry or source fix was hidden. Author/historical test totals and their earlier failures remain author evidence; none was relabeled as reviewer execution or combined with overlapping reviewer runs.

## Read-only proof, preservation and remaining action

Read-only disposition is supported for the bounded synthetic workflow: the inspected reader has only the explicit selected-file read operations; all stored paths stay in memory/text. Newly executed native seams assert exactly one selected read, discard bytes with not-found/permission/arbitrary I/O errors, and observe zero persistence attempts across query/projection. Real native lifecycle and Node sessions compare protected input names/bytes after success, queries, restart, refused loads and shutdown. Reader stdout/stderr/process evidence is separate from fixture-setup writes. No source/media I/O path was found or browser request to a recorded path observed. This is not OS-wide attempted-I/O monitoring, transient-write tracing, ACL enforcement, hostile-executable containment, power-loss or concurrent-replacement qualification. Last-access metadata is not treated as a content write.

R/repo-before.json covers all 261 pre-existing repository paths, including the candidate, unrelated files, historical reports and original Figma/PDF/ZIP. R/author-before.json covers all 5791 files in the author evidence tree, including retained source, binary, inputs, logs, screenshots and caches. The preservation checks found every file byte-identical; no pre-existing path was deleted or changed. Original extracted reference hashes are verified separately against their retained manifest. R/final-preservation.json records final branch/HEAD/index, additions, input/reference checks and task-process status; R/evidence-identities.json links logs, scripts, screenshots and executables. Only this new report is added to the repository. Existing tests/source/living records/author report and ignore rules are untouched.

All reviewer-owned readers, servers and browsers are stopped and waited; no matching task runtime process remains and probed listeners are closed. Intentional forced cleanup is explicitly recorded above and is not called graceful termination. No staging, commit, push, merge, publication, production-data trial, inventory/registration, media work or unrelated workstream occurred.

**ONE next action:** a separately authorized bounded correction pass for R1-R3, preserving this report/evidence, followed by the targeted recheck identified above. Owner acceptance/publication is not yet supported. Unrestricted native catalogs, live inventory, scale, ACLs, concurrent writers, other-platform/Docker execution, recovery and hardware remain unqualified; unrelated persistence/keystore, PR-04, tar/Unicode, tape/ring-buffer and Blu-ray work stays separate.
