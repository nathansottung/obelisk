# Disposable directory inventory implementation - 2026-09-20

**GUI_DISPOSABLE_INVENTORY_READY_FOR_REVIEW**

The new native command inventories an explicitly selected generated directory, validates a complete new native catalog, and publishes it without replacing an existing destination. The accepted read-only GUI browses those observed records without following their source paths. This is implemented and author-tested in the same Codex conversation; it is not substantive reviewer closure, owner acceptance, production approval or publication.

Branch: `feat/gui-disposable-inventory`. Exact parent and unchanged HEAD: `5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`. Its parent is catalog implementation `d254d7bc2893482aad949416544762d2b2c7ff4b`; the older catalog patch base `433cedac0a0f8a4e9dc67e3a1722b52c39c7cc6e` was not used as this task's starting point. The local chain, committed owner acceptance and existing publication receipt establish the checkpoint. No new remote lookup was required or performed.

Task evidence root **E**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-20260920-145714`.

Initial index and tracked worktree were clean; only the expected PDF/Figma were untracked. The branch did not already exist and was created once at the exact parent. No applicable AGENTS.md was found. Current handoff/status/next-action/coverage/README, the four catalog reports and publication receipt were consulted; previous prompts were provenance, not tasks to replay. E/submitted-prompt.txt retains this actual authorization. E/before.json inventories the actual 271 starting files, including the explicitly included ignored ZIP; that measured count is not a historical expectation or audit count. E/source-before retains relevant pre-edit source/records, and implementation-contract.md was written before edits.

## Native contract and implementation

[gui_inventory.go](../../../gui_inventory.go) implements `--gui-disposable-inventory ABS_SOURCE ABS_NEW_CATALOG`, dispatched by [main.go](../../../main.go) before production flags/default directories/configuration/App startup. There is no implicit source/output, browser producer endpoint, registration, existing-catalog update or background job.

The accepted schema is native version 8. The producer uses the existing `catalog`, `Collection`, `Folder` and `File` types. A sourced collection and folder describe the selected root and observation session. Each actual regular file gets a separate positive native ID, FolderID/CollectionID, preserved relative path spelling via filepath.ToSlash, observed byte size and mtime, actual observation FirstSeen, SHA-256 and catalog-only BLAKE3. Collection.CreatedAt records the observation session. NextID counters describe the new records, not renumbered existing state.

Folder.Path plus File.RelPath locate the source observation; a content hash does not identify a pathname or establish a count of physical backup copies. Equal bytes in distinct files retain separate records. Duplicate basenames in different directories retain distinct evidence. No native Volume/Location, Chunk, Copy, backup result, capacity, online status, parity or verification record is fabricated. The absent chunk-copy view truthfully makes no claim about physical backup existence. This is the native sourced-file representation, not a competing JavaScript schema or hard-coded catalog fixture generator.

The existing App.ScanFolder path was inspected in pipeline.go: it loads configuration, registers a folder, begins a Store batch, extracts media metadata, upserts files, flushes and logs; its parallel worker path collects partial problems and uses unrestricted whole-file hashing. That workflow is inappropriate for this producer's fail-closed/new-output contract. It was not called or refactored. The only reusable extraction is [hashing.go](../../../hashing.go)'s `hashReaderBoth(io.Reader)`: the existing hashFileBoth still opens/closes its file and delegates to exactly the same SHA-256/BLAKE3 core. The producer owns the explicit read-only file open, bounds, cancellation and before/after observations. Existing hashing/scanner regressions and a partial-read-error regression cover this shared change. The existing atomic hash-acceleration initializer only sets its in-memory default; no application initialization is introduced.

After traversal, the entire candidate is marshaled and passed through unchanged loadGUICatalog with an in-memory reader. This reuses all current size/presence/schema/relationship/subset validation. gui_catalog.go, store.go, config.go and pipeline.go remain unchanged. Advanced sections, retained versions and spanning retain their refusals. Native exact ID and int64-size string transport through projection/queries/Node/browser is unchanged.

## Source boundary, bounds and publication

The implemented developmental envelope is a quiescent ordinary local tree selected by the operator from freshly generated expendable test inputs. It is not a general arbitrary-path authorization service or a Nathan-specific sandbox. Actual executions use only this task's fixtures and separate output storage.

Source and output parent must already exist and be ordinary directories. Absolute path length is bounded; roots/defaults are not inferred and a source filesystem root is refused. Before traversal or staging, all existing components are checked with Lstat. Source/output-parent containment is checked using filepath.Rel and actual ancestor identities through os.SameFile in both directions. This supplements lexical comparison for aliases such as Windows short names; it does not claim adversarial rename confinement. Output-parent ancestors cannot lie inside source, nor source inside output parent. The output parent is rechecked before staging. No output-parent directory is created by the producer.

[gui_inventory_windows.go](../../../gui_inventory_windows.go) refuses nonordinary drive paths, UNC/device-prefix/stream/ambiguous trailing names and reserved DOS device names; GetDriveTypeW requires a fixed local drive. File attributes reject every reparse point, including junctions and symbolic links. Unknown attribute representation also refuses. [gui_inventory_other.go](../../../gui_inventory_other.go) retains absolute/non-link policy for other builds, but local mount/bind-alias semantics and other-platform runtime behavior were not qualified. The existing source/destination-boundary record was consulted; historical PR-04 remains unrecovered and is not reconstructed or marked integrated.

The producer reads directories with bounded ReadDir calls and inspects every encountered object; unsupported entries are errors, not omissions from an allegedly complete snapshot. It hashes only opened regular files from the selected generated source. File identity/mode/size/mtime are compared before/after reads, byte count must match the observation, and all source observations plus directory names are rechecked before publication. Observed inconsistency refuses the run. This is not an atomic filesystem snapshot, detection of every same-size/time-restored mutation, kernel-wide I/O tracing or defense against a malicious concurrently changing filesystem.

| Bound | Implemented value |
|---|---|
| Regular files / total entries | 64 / 128 |
| Directory depth / relative path | 8 / 512 bytes |
| Absolute source/output path | 4096 bytes |
| Observed file bytes / total observed content | 8 MiB / 32 MiB |
| Growth detection | At most one extra byte on an inconsistent file, then refusal |
| Cooperative run deadline | 30 seconds; checked between operations and bounded read chunks |
| Final catalog | Accepted reader's unchanged 4 MiB maximum and supported-subset limits |

The reader wrapper limits underlying requests to 64 KiB, and the shared hash core retains its existing buffer. Cancellation is cooperative; it does not forcibly interrupt a blocked kernel call. Counts report processed files/read bytes, never a fabricated total or percentage. Cancellation, read/stat/enumeration errors, limit, output failure or observed inconsistency before the publication point leaves no successful final snapshot. Legitimate empty input still validates and displays as a valid empty file inventory, distinct from failed loading.

Only after complete candidate construction/validation and source rechecks does the producer create its own unique `.inventory-*.tmp` in the separate output parent, write all bytes, Sync and Close. The same no-replace primitive used by config.go's successful initialization policy, **os.Link**, gives those completed bytes the final name only if it remains absent. This link is the precise successful-publication point. An early existence check is not the only collision protection. Existing or late-arriving destinations are never truncated, renamed away, merged or replaced. Output hard-link support is required here; there is no replacement fallback and no prerequisite imposed on all source/media backends.

Only the producer's own staging name is cleaned. A cleanup failure after successful linking returns an error with Published=true and the remaining staging path; it does not remove the final catalog. A status-output failure also reports whether publication occurred. No directory fsync/power-loss durability, cross-process hostile interference or atomic source snapshot is claimed. Test teardown of generated Go temporary fixtures is separate from producer rollback; the producer itself never deletes a successful final output.

## Execution, tools and independently generated inputs

Installed tools: Go 1.26.8 windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. All builds use the installed audit go-mod/toolchain, GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly and CGO_ENABLED=0. GOCACHE/TEMP/TMP point to this task's output/go-cache and output/go-temp; GOTMPDIR is unset. No installation, dependency/global setting change, WSL switch or external service was used.

Final paired binary: E/output/inventory-reader-bounded.exe, SHA-256 **`6f7c5abc89697a250c944d6b028da5e256a31aaf3873567792e75bdfae15b235`**. It is freshly built from the final source and supports both producer and accepted reader command. Earlier binaries inventory-reader.exe and inventory-reader-final.exe are separately retained with their actual build/test stages; neither was overwritten. reader-double.exe is the separately compiled existing fault emitter, not the native reader. Final identities and source copies link these distinctions explicitly.

E/setup-and-produce.mjs creates ALPHA and BETA sources independently from literal file contents, plus EMPTY and a generated sibling sentinel. Setup writes its oracle before producer execution; the producer receives only source/output argv and never reads that oracle. Both five-file trees include an empty file, equal-content files at different paths, `same.txt` and `nested space/same.txt` with different contents, and `nested space/雪.txt`. Setup records independent expected SHA-256, bytes, ID ordering/path and fixed test mtime. The producer derives its output from the actual directory at runtime. Native outputs are checked against that oracle; the browser checks the same independently prepared expectations.

Initial producer calls create snapshots/alpha.json, beta.json and empty.json. E/reopen-change.mjs temporarily renames only generated ALPHA to a checked task-owned sibling, proves its original path absent, opens the original catalog in a new real browser using the reader, and restores ALPHA in finally. The first snapshot's bytes remain unchanged. A separate genuine CLI attempt to publish again at that existing valid catalog exits 1 with Published=false, preserving it.

Only after the first viewing session, setup explicitly changes `ALPHA/nested space/same.txt` to the recorded new test bytes/mtime. E/authorized-source-change.json records old/new identities. The subsequent producer creates alpha2.json, without updating alpha.json. The final boundary refinement is then exercised with a new alpha-final.json from the final binary and a final browser pairing. The original catalog still has its first observation; these are separate snapshots, not in-place refresh.

Final retained native snapshot SHA-256 values:

| Snapshot | SHA-256 |
|---|---|
| alpha.json | c1161e52413d00e9ebbec768c828ce63bae1cc60f4545e80cbb929fe0d6f7f8a |
| beta.json | a27fec13a090cdff962844dbd4a12e8065de3566f259e89b91586bedf88fee25 |
| empty.json | 1764a1c2a2b357894979e30c5cb1a1a6dd5a57b6c601da885978d8a24bbab7d5 |
| alpha2.json | 123ea8f603b7ea23947056575e3d2f51a932c72841e075b407ddda14b313b4ca |
| alpha-final.json | fc9e5e87709c99cad9c177532e1a9d4222bc5994733fadde7cfd216b15f2b477 |

E/fixtures-before.json and fixtures-final.json have the same 17 directory/file entries. Sixteen are unchanged; the sole content difference is the explicitly logged ALPHA test-setup edit. Producer source-preservation assertions bracket the initial three runs, and later reader/new-snapshot checks preserve the first catalog. The sibling sentinel is unchanged. snapshots-final.json records the five catalogs and three independent oracles. Twenty-six copied accepted catalog/fault controls match their originals and remain unchanged. These final content/entry comparisons complement code-path/read-boundary tests; they do not prove absence of every transient write or treat incidental access-time changes as content writes.

## Actual author validation

| Execution stage | Result and artifact under E/output |
|---|---|
| Initial selected native/hash/scanner/reader tests | 14 top-level passes, 47 passing subtests, no failures/skips; go-selected.txt, go-tests-initial.jsonl |
| Explicit Windows device-name policy addition | 8 top-level passes, 28 passing subtests, no failures/skips; go-final-selected.txt, go-inventory-final.jsonl |
| Final object-ancestor boundary and hash partial-error checks | 10 top-level passes, 28 passing subtests, no failures/skips; go-boundary-selected.txt, go-boundary-final.jsonl |
| Builds/vet/read-only final gofmt | All executed exits 0; native-initial-exits.json, native-final-exits.json, native-bounded-exits.json; no final formatting findings |
| Existing Node parser/static/catalog/correction suites | 5 / 7 / 4 / 8 passes respectively, no failures/skips; node-selected.txt, node-*.txt, node-exits.json |
| Generated source producer/setup | Three initial successes, changed-source alpha2 success, final-pair success; one deliberate existing-output refusal; raw argv/status/exit logs retained |
| Generated viewer isolation and split stop | Six producer/helper/path-selection routes refused; native reader waited exit 0; actual split stop exits 0 naturally with stdin open, 248 ms including startup; reopen-change-isolation-stop.json |

The Go stages overlap and are not summed. The first selection was `^Test(GUIInventory|HashFileBothMatchesSHA256|ScanRecordsBlake3|ScanFolder_|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))`. Follow-up selections were `^TestGUIInventory` and `^Test(GUIInventory|HashReaderBothRefusesPartialError)`, each enumerated before `-count=1 -json -run`. Source refinements explain the targeted repeats. Initial accepted hashing/reader controls were not repeatedly claimed as fresh final-stage execution. No scale test was selected or override performed; the historical catalog scale skip remains historical.

New tests exercise actual observed identities/times/hashes, equal-byte separation, reader operation with recorded source absent, missing/wrong-kind source, source/output overlap and ancestor output, pre-existing output, file/entry/depth/path/byte limits, injected enumerate/stat/read permission failures, observed content/directory change, pre-run/late cancellation, nonexistent output parent, partial output write, unsupported link publication, late destination, cleanup before/after publication and status/progress failure. At-limit tests use reduced per-invocation test limits through the same enforcement code (5 files, 6 entries, depth 1, path 15, file bytes 6, total 21); no production-size qualification is inferred.

Native symlink creation and all three Windows junction cases (encountered entry, source root, output parent) executed and passed without privilege/global-permission changes. The targets are generated sibling sentinels only. Windows device/stream/UNC lexical tests do not open devices. Permission/read failures are deterministic injection, not a Windows ACL qualification. Post-publication cleanup tests prove the final catalog remains valid while the error reports Published=true; deliberately retained per-operation test staging is subsequently subject to ordinary Go temporary-fixture teardown, not archived as a production output. Terminal test events/logs are retained.

Nine actual browser sessions completed exit 0, with errors [], each server closed and browser stopped/waited:

| Session | Passed assertions |
|---|---:|
| generated ALPHA / BETA | 11 / 11 |
| ALPHA reopen with source unavailable | 11 |
| changed-source alpha2 / final binary pair | 11 / 11 |
| generated empty | 3 |
| accepted static mode | 55 |
| accepted prior-selection/query-failure propagation | 4 |
| accepted exact-large-ID evidence association | 12 |

The new test-only inventory-browser-checks.mjs consumes the external oracle through an explicit environment variable. It checks Library, Find, every file's own path/hash/size/record/source/observation, absence of invented storage/copies, no-match clearing and real skip-link activation preserving selection/query/history/time origin and main focus. The served UI, catalog adapter/protocol/server/stop parser remain unchanged. Existing Node correction checks separately cover failed bootstrap/protocol invalidation, exact unsafe/native-maximum IDs and reversed order, real reader termination/recovery, whole/split/character/CRLF stops in static/catalog modes, EOF and bounded delayed-reader cleanup.

Actual generated-catalog split stop wrote `st`, established HTTP still alive, then wrote `op\n` while keeping stdin open until exit. It reported stdin stop, server exit 0 and native reader exit 0; no forced cleanup caused success. Existing browser/static harness termination and intentionally delayed fault-reader cleanup remain forced, waited outcomes, not natural-stop or interactive-console certification. Pipe/IPC SIGINT evidence is not a manual PowerShell console test.

Representative inspected images: E/output/browser-beta/inventory-library.png and browser-final/inventory-record-3.png. The latter shows the changed nested file's own 55-byte observation and checksum; the former shows five distinct BETA records and no registered storage. Other generated record screenshots and per-session DOM/keyboard/request/process evidence are retained. Catalog screenshots are 1440x1024 CSS at scale 2 (2880x2048 PNG); only the static branch additionally uses its existing narrow smoke viewport. Existing layout/disclosures were reused; no unprovided Figma frame or additional visual-fidelity approval is claimed.

No runtime/setup/build suite failed and no hidden retry was used. Deterministic negative cases intentionally return failures as asserted. The first shell rg searches without an explicit directory yielded no useful source matches; explicit-file/directory inspection supplied the source evidence. Final packaging initially treated a null PowerShell missing-path result as a one-element array; explicit path enumeration confirmed zero missing paths and the exact expected scope, without a candidate correction or runtime rerun. These were inspection/bookkeeping diagnostics, not failed runtime tests. Historical integration/reviewer/correction/closing/publication evidence stays separate: in particular, the closing catalog Go 18/25/one scale skip, Node 5/7/4/8 and 16 browser sessions were not inherited as this task's execution. Publication ran no runtime tests.

## Runnable checkpoint and exact commands

From the repository root, view the completed final snapshot:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-20260920-145714'
$reader = "$task\output\inventory-reader-bounded.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\alpha-final.json" --adapter $reader
```

Open the exact loopback URL. Type `stop`, press Enter and wait for termination. BETA/empty/original ALPHA use their corresponding retained snapshot names. Static mode remains `node scripts/gui-preview/server.mjs 0`.

The final producer invocation executed was:

```powershell
& $reader --gui-disposable-inventory "$task\fixtures\ALPHA" "$task\snapshots\alpha-final.json"
```

That output now exists and an identical producer rerun must refuse. Any subsequent authorized observation must use an explicitly chosen absent output name; never delete/reuse retained snapshots merely to replay setup. Producer completion/Published=true precedes viewer launch. The final binary build command was `& $go build -o "$task\output\inventory-reader-bounded.exe" .` under the exact offline environment in native-bounded.ps1. Earlier initial/final scripts preserve their distinct source stages/binary names.

Executed fixture creation command: `node "$task\setup-and-produce.mjs" $task`. This is external setup provenance, not a reusable command over an existing directory: it uses exclusive creation and must not be replayed here. E/native.ps1, native-final.ps1, native-bounded.ps1, node.ps1, browser.ps1, setup-and-produce.mjs, reopen-change.mjs, final-pair.mjs and preserve.mjs retain the exact argv, environment, setup changes and result files. No generated source/catalog/binary/log/screenshot is inside the repository.

## Candidate identities, preservation and review gate

Changed existing paths: main.go, hashing.go, hashing_test.go, scripts/gui-preview/browser-check.mjs, scripts/gui-preview/README.md and the four living records CODEX_HANDOFF.md, NEXT_ACTIONS.md, OB_STATUS.md, REVIEW_COVERAGE.csv. New paths: gui_inventory.go, gui_inventory_windows.go, gui_inventory_other.go, gui_inventory_test.go, gui_inventory_windows_test.go, scripts/gui-preview/inventory-browser-checks.mjs and this report. This is nine changed, seven added, zero missing, with 262 of 271 original files preserved; final working inventory has 278 paths. These are identity reconciliation counts, not a staging list. The index is empty and HEAD unchanged; no commits, pushes, merges or automatic review.

E/final-current-identities.json covers all tracked/untracked candidate files and preserved design originals; candidate-identities.json and candidate/ retain the scoped final source/test/document bytes. This report is also hashed separately in report-identity.json to avoid self-hash ambiguity. E/final-preservation.json, status-final.txt and evidence-identities.json retain Git/operation/process state, differences, final binary hashes and external evidence identities. The evidence manifest excludes build caches/temp/browser profiles; those are not claimed as durable review records.

All historical repository reports and original Figma/PDF/ZIP are unchanged. Seven extracted design/reference entries match. External original review/correction/closing/publication manifests were independently checked: 313, 527, 489 and 74 files respectively, all matching; these overlapping sets are not added together or relabeled new test results. Prior original-author evidence remains in place; no tool wrote into it. The 26 copied accepted controls also match their originals. All task-owned readers/servers/browsers are stopped and waited; no task process remains at handoff.

ONE next action: **one bounded substantive review of this uncommitted candidate**, focused on actual-object source/output separation, non-following entry policy, observed completeness/limits/cancellation, no-replace publication and truthful post-publication errors, pure hashing extraction, generated record/evidence association and unchanged reader/isolation/lifecycle controls. No substantive review is automatically executed here.

Production catalogs, arbitrary sources, storage registration/inventory, media operations, parity/copy/restore engines, hostile concurrency, race/ACL/power-loss/interactive-console/platform/Docker/helper/hardware qualification and main integration remain unestablished. Generation-independent preservation/buffering, LTO-8 as first physical qualification target rather than a generation cap, explicit other-backend/generation qualification and separate Blu-ray workflow remain product planning constraints. Stop for review.
