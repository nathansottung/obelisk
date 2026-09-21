# Disposable inventory review follow-up - 2026-09-20

**READY_FOR_GUI_INVENTORY_FOCUSED_RECHECK**

| Controlling item | Author disposition |
|---|---|
| F1, P2: malformed encoded filenames become replacement-character names | Filename encoding: **AUTHOR_ADDRESSED** |
| F2, P2: newline-containing recorded names cannot be searched literally | Search fidelity: **AUTHOR_ADDRESSED** |
| S1: missing optional `.DS_Store` exclusion and durable scope | `.DS_Store` and durable scope: **AUTHOR_IMPLEMENTED** |

These are author dispositions following one separately authorized correction pass, not targeted reviewer closure, owner acceptance or publication. S1 is newly completed requested work, not a reproduced producer safety failure. The bounded base workflow was not relabeled unsafe. ONE next action is a targeted recheck of these items and their directly affected contracts.

## Identity, authorization and preserved baseline

Branch `feat/gui-disposable-inventory`; full HEAD and complete inventory-patch parent remain `5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`. Index empty; no branch creation/switch, active Git operation, remote lookup, staging, commit, push or merge.

Correction evidence root **E**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-fix-20260920-163849`.

Controlling [substantive review](GUI-DISPOSABLE-INVENTORY-REVIEW-2026-09-20.md) and [original author report](GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md) remain unchanged. Current handoff/status/next-actions/coverage/README, relevant native types/callers and retained provenance were inspected. No applicable AGENTS.md or completed correction for these bytes was found. E/submitted-prompt.txt retains actual Prompt 15A, which now sequences the missing addendum work after review. No historical prompt was replayed as another task.

The review's actual final working-byte manifest matched all **279** current entries, including its added report. All **896** artifacts listed in its external evidence manifest matched. Counts are identity inventories, not staging lists or substantive coverage claims. E/before.json, starting-state.json, before.patch and baseline-verification.json retain exact path/hash/Git evidence. E/before-source preserves the reviewed uncommitted source sufficient to build/run it, including untracked implementation/tests; `.git`, credentials, production state and the three design binaries were not copied. Original design bytes remained in place and were hashed. A checkout of HEAD alone was not substituted for the red baseline.

Fresh before-copy executable: E/output/before-reader.exe, SHA-256 `bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69`. Fresh corrected Windows executable: E/output/corrected-reader.exe, SHA-256 **`4151ef719a7da830d630ce1158d5d045300970cac81e6ba3bd4fd9c625a1d5eb`**. Both are separately retained. The corrected production Go bytes were unchanged after this build; later refinements were preview JavaScript/CSS/tests/documentation. Cross-build and fault-emitter identities are in E/execution-summary-final.json and binary-identities.json.

## Exact before replay and root causes

Before editing the candidate, E/red.ps1 built the verified uncommitted copy and ran the retained foreign-name browser reproduction into a new directory. E/red-encoding.mjs passed the original malformed byte/escape inputs to that fresh native reader. Invalid raw UTF-8 and an isolated high-surrogate escape both exited 0 and projected `x\ufffd.txt`; the valid literal U+FFFD control also exited 0. The browser reproduction completed four earlier checks and then failed on the newline name. These are successful reproductions of failures, not compliant before-fix behavior.

F1 first loses information in the unchanged shared decoder's legacy `json.Unmarshal`, reached by the preview loader without original-byte validation. A check on the already-decoded string cannot distinguish repaired input from a legitimate U+FFFD. Producer serialization has a corresponding pre-encoding obligation for names observed as Go strings. Windows enumeration has an earlier UTF-16-to-Go conversion boundary as well. At the Node response boundary, nonfatal Buffer-to-UTF-8 decoding could likewise repair malformed bytes. JavaScript JSON.parse itself retains isolated UTF-16 units, so scalar validation must precede later URL/text encoding or display rather than incorrectly assuming that parse repaired them.

F2 is a single-line HTML search control combined with a generic trimmed substring search. Direct `input.value` assignment strips LF. A later instrumented replay using the unchanged before-copy and actual CDP text insertion observed a different input transformation: `line\nname.txt` became `line name.txt`; the emitted URL contained `text=line+name.txt`, the real response was `ids:[]`, and previous selection cleared. E/output/red-observed-input-v2/red-input-query-response.json records input, query, response and before/after evidence. The first observer assumed insertion would equal direct assignment and failed; its script/output are preserved. Neither behavior is used as a reason to alter stored filenames.

The missing flag/scope was established from the actual parser, traversal, native representation and documentation. No invented pre-fix failing exclusion test was used. E/correction-contract.txt records the bounded encoding, exact-entry and native-audit compatibility decision before source edits.

## F1: validation before irreversible conversion

[gui_encoding.go](../../../gui_encoding.go) validates original UTF-8 and JSON syntax and walks active string escapes before native decoding. It rejects isolated high/low surrogates and malformed pairs while respecting escaped backslashes/quotes. Literal `\\uD800` text is not an active surrogate escape; legitimate U+FFFD, accents/non-Latin characters and valid supplementary pairs remain supported. The whole bounded catalog is checked before adoption, including malformed later records. The shared production `decodeCatalogJSON` and its ordinary application callers remain unchanged.

[gui_inventory.go](../../../gui_inventory.go) checks source/output path strings and enumerated names for valid UTF-8 before their catalog serialization. [gui_inventory_windows.go](../../../gui_inventory_windows.go) additionally performs a bounded native FindFirstFile/FindNextFile UTF-16 name check before os.ReadDir converts names. Valid surrogate pairs and U+FFFD pass; isolated units refuse. The tree still must remain quiescent between this pass and observation; this does not claim hostile-race confinement. Non-Windows names are validated as original Go string bytes before encoding, without defining a new arbitrary-byte filename scheme. Caller/terminal argument conversion outside the process cannot be reversed; CLI arguments must arrive as valid Unicode.

The preview adapter uses fatal UTF-8 response decoding. Its existing complete-line, total-byte, timeout, validation and failed-mode behavior stays intact. Protocol fields used for filenames/evidence must contain well-formed Unicode before successful display. Exact-entry parsing rejects isolated UTF-16 units before TextEncoder/URL conversion. The server rejects malformed percent-encoded UTF-8 before URLSearchParams can repair it. Invalid later records do not become a successful partial, empty or static snapshot.

New Go tests exercise explicit malformed byte arrays, truncated encodings, isolated high/low escapes and order variants, plus valid replacement/pairs/quotes/backslashes and literal escape-looking controls. Whole-loader tests replace first/later native file names in original encoded bytes. New Node tests exercise raw response bytes and first/later projected records through the actual decoder helper and strict validator. Direct corrected-reader replays now refuse both original malformed inputs with encoding errors; the valid literal control still succeeds and rejected files retain their exact bytes. These are data/protocol tests; invalid Windows filenames were not claimed as natively created. Native UTF-16 rejection is covered by pure unit controls and bounded live enumeration of valid generated names.

## F2: explicit, reversible exact-name entry

The preview now offers **Exact name (JSON string)** alongside the unchanged ordinary literal substring search. It uses the existing single-line control with an explicit quoted representation, rather than relying on raw CR/LF retention or textarea normalization. For example, actual CRLF is entered as `"line\r\nname.txt"`; literal backslash-n is entered as `"line\\nname.txt"`. Leading/trailing spaces and tabs are preserved in the explicit representation. The inspector's **Use exact name** button fills it from the selected authoritative record.

[catalog-names.mjs](../../../scripts/gui-preview/catalog-names.mjs) handles this preview-local representation. No implicit unescaping of ordinary Windows paths or global query language was added. Ordinary search retains its prior case-insensitive/trimmed substring semantics. Explicit exact mode performs case-sensitive, character-for-character equality and does not combine with the separate hash filter. Invalid entry produces a useful alert, clears results and sends no repaired query. A valid next entry can recover from an entry error; a failed catalog load remains latched and requires relaunch.

The exact parameter has a 4096-UTF-8-byte semantic bound. Its quoted input has a 24578-character bound; exact HTTP URLs are capped at 16384 characters and the native scanner at 32768 bytes to accommodate escaped controls. Existing ordinary text/hash limits remain 256/64, ordinary URLs 2048 and ordinary native lines 4096. Catalog size/row/string limits are unchanged. Exact mode bypasses production Search's trimming through a small preview-only equality branch; native IDs remain separate exact decimal strings. Retired-collection exclusion remains consistent.

Control-containing/edge-whitespace paths display as `JSON name: "..."` using text nodes, keeping authoritative values separate from visible escapes. Quotes, percent/plus/hash punctuation and backslashes are inert. Exact state and selected evidence survive disclosures, Back/Forward and keyboard skip activation; Enter submits the real input. A fresh launch reopens the persisted data with session-local query state reset normally. The only styling change is the new checkbox/help/error treatment; no Figma state or layout redesign was invented.

The 14-record foreign-data fixture has actual LF, CR, CRLF, leading/trailing controls, tabs/spaces, stripped-name and literal-backslash decoys, quotes, a Windows-looking path, U+FFFD and emoji with distinct expected sizes/hashes. Browser tests use keyboard select-all, CDP Input.insertText and Enter in the real input; an observer clones actual fetch responses without substituting them. Assertions match the exact semantic URL parameter, native response ID and selected path/hash/size. Ordinary backslash-n also passes through the real ordinary input and selects only its literal record. This is browser insertion simulation, not native clipboard or interactive-console qualification.

## S1: exact optional filtering and durable native scope

Actual CLI syntax:

```text
--gui-disposable-inventory [--ignore-ds-store[=true|false]] ABS_GENERATED_SOURCE ABS_NEW_CATALOG
```

The optional flag precedes both paths. Bare flag and `=true` enable it; absence and `=false` disable it. Invalid values, duplicate/misplaced flags or wrong argument counts refuse with usage. `--gui-disposable-inventory --help` currently returns usage as an error, exit 1. No global preference, browser scan endpoint or Settings workflow was introduced.

Filtering happens only after existing path/type/link checks establish a regular file, using exact case-sensitive enumerated basename `.DS_Store`. Directories of that name are traversed. Links/reparse/special entries cannot bypass refusal. Excluded regular files count toward the same 64-file and 128-entry bounds, are recorded as observations and rechecked before publication, but never reach the content-open/hash stage. Enumeration/stat failures, observed changes, limits and cancellation remain failures. No source cleanup, rename, permission change, metadata stripping or general ignore engine exists.

The compatible native facility is the existing `Audit{At,Action,Detail}` history and catalog `audit` array, normally used for timestamped action details. Its persisted field spellings are `at`, `action`, `detail`. The producer constructs one audit event directly as part of its in-memory new catalog; it does not call Store.Log, save, jobs or application startup. The action `GUI_DISPOSABLE_INVENTORY_V1` identifies a versioned detail contract, not an unrelated native field repurposed as a sidecar.

Detail contains exactly these eight required fields: version=1, effective policy (`include-all` or `ignore-exact-ds-store`), entries, regularFiles, includedFiles, excludedFiles, readBytes and complete=true. No excluded-path list is stored. [gui_inventory_scope.go](../../../gui_inventory_scope.go) validates the bounded event, timestamp/collection relationship, field presence/uniqueness/types, policy/version/completion and count/byte relationships. Only this exact audit event is admitted by the preview; arbitrary Audit or other previously refused advanced sections remain refused. No catalog schema change, migration, sentinel records or multi-file publication protocol was added.

The new view distinguishes explicit OFF, explicit ON including zero observed exclusions, genuinely empty sources, all-excluded nonempty sources and historical UNKNOWN scope. Existing old catalog records remain visible irrespective of today's producer setting. Scope labels describe recorded successful traversal within selected scope, not whole-source completeness, missing files, current availability, parity or verification. Producer errors do not publish a complete scope event.

**Compatibility decision:** the freshly built before-copy reader refuses both new OFF and ON snapshots because their audit section is populated; it successfully reads the legacy unfiltered fixture. E/output/prior-reader-compatibility.json retains exact exits/stdout. Corrected readers accept legacy supported catalogs with UNKNOWN scope. No claim is made that older readers can safely discard scope or that generic production readers have been qualified. Pair new scoped snapshots with the corrected preview. This is the narrow projection/validation expansion for an existing native provenance facility authorized by Prompt 15A, not an unversioned native-format workaround.

## Fixtures, execution and retained diagnostics

Installed tooling: Go 1.26.8 windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. Go uses the existing audit toolchain/module cache, GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0 and E/output/go-cache and go-temp as GOCACHE/TEMP/TMP; GOTMPDIR is unset. No installation, upgrade, global settings, WSL or remote CI. Cross-builds do not establish native execution.

E/fixtures.mjs independently defines the generated source contents and oracle before production, then invokes the real native producer with only source/output arguments. The same 28-file tree yields separate NEW off.json/on.json: 37 entries, 28 regular files; OFF includes 28/excludes 0/reads 851 bytes; ON includes 26/excludes exactly 2/reads 782 bytes. Root/nested exact files, near names/case variants in distinct numbered or named directories, other dotfiles, __MACOSX, Spotlight/Trashes, Thumbs.db/desktop.ini, .xmp/.aae, a `.DS_Store` directory sentinel, empty/equal-content files, same basenames, punctuation, accents, composed/decomposed names and non-Latin/emoji/replacement-character controls are retained. No case-only fixture overwrite was attempted.

Separate empty, all-excluded and ON-zero sources yield distinct scope. Foreign and legacy fixtures are native-shaped data-only copies with independently specified expected names/evidence; foreign paths are never followed. Legacy audit absence is explicit test setup, not a production rewrite. Sources and surrounding entries are hashed before viewing and verified at closeout. No source content change is made by this correction's producer/setup after fixture creation; the only temporary source rename is the controlled unavailable-source test, restored in finally.

| Actual correction execution | Result |
|---|---|
| Enumerated Windows native selection | **24 top-level passes / 68 passing subtests**, 0 failures/skips |
| Windows producer/reader and existing fault-double builds; vet; final format/whitespace | Passed; no final format/whitespace findings |
| Linux amd64 and Darwin arm64 cross-build/vet | Both passed; native runtimes NOT RUN |
| Initial Node stop-command / preview / catalog / catalog-correction / new inventory-correction | **5 / 7 / 4 / 8 / 3** passes |
| After exposing strict response decoder for original-byte tests: affected catalog / catalog-correction / inventory-correction | **4 / 8 / 4** passes, separate overlapping selection |
| Initial real producer fixtures | Five successful new outputs, independent content/scope checks |
| CLI explicit true/false and usage controls | Two verified new outputs in v2 plus four usage refusals; one earlier OFF output retained after fixture-side diagnostic |
| Corrected browser attempts | **17 attempts: 16 completed successfully; 1 initial failure retained and corrected** |

E/native.ps1 enumerates `^Test(GUIEncoding|GUIExact|GUIInventory|HashReaderBothRefusesPartialError|HashFileBothMatchesSHA256|ScanRecordsBlake3|ScanFolder_|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))`, then executes `go test -timeout 120s -count=1 -json -run ... .`, fresh builds and vet. E/cross.ps1 records process-selected target builds/vet. E/node.ps1 and node-final-selected.txt record exact Node groups and `node --test --test-isolation=none` calls. Original-byte, exact-name, scope, excluded-file read observer, no-replace/cancel/bounds and malformed-scope regressions are in the new candidate tests. Existing base failure/change/publication/hash/reader tests ran in the same bounded selection. No native suite was rerun just for an identity checkpoint.

Native exclusion tests prove the content-read stage is never reached for excluded files, while source/type/stat errors still refuse. They exercise an exact-name native symlink, generic native junction controls, entry/regular-file limits, cancellation immediately before publication, excluded-file changes, existing/late destinations and scope unknown/malformed/duplicate/version/count/completion refusal. Prior write/short-write/link/post-publication cleanup and status controls remain passing. This is deterministic injection plus source inspection, not native ACL or power-loss qualification. Test-framework teardown of its own temporary probes is distinct from producer cleanup; the producer still removes only its own staging name and never rolls back a published final catalog.

Corrected browser stages under E/output:

| Session | Completed checks |
|---|---:|
| off / on / legacy | 38 / 36 / 38 |
| empty / all-excluded / on-zero | 3 / 3 / 11 |
| initial foreign exact-entry run | 24 |
| static / exact large IDs / encoding-refused load | 55 / 12 / 2 |
| initial query-failure control | Failed after 1 check; not a completed pass |
| query-failure-final / foreign-final | 4 / 24 |
| on-reopen / foreign-reopen | 36 / 24 |
| ordinary-literal / final-input | 25 / 25 |

The initial query-failure control encountered the newly created empty role=alert before the failed request had occurred. The new entry error now has alert semantics only when it contains an error. The actual failure-propagation and foreign-entry sequences were rerun in new directories and passed. Later input-only CSS aligns the checkbox with its label and keeps help compact; final-input captures the final UI. These changes and selective repeats are distinct from the initial stage; partial assertions are not passed sessions.

Before-copy browser evidence is separate: original red expectation failed after four checks; first insertion observer failed due to assuming direct-assignment semantics; corrected observer completed three diagnostic checks documenting actual input/query mismatch. E/red-encoding.mjs has two malformed-name red cases plus the valid-literal control. Passing diagnostic predicates mean reproduction, not compliance. No red counts are added to corrected results.

Other retained diagnostics: the first CLI option verifier used the wrong JavaScript property spelling for native `detail` after the producer had successfully created explicit-off.json. That output was preserved. The v2 verifier uses the actual native field and distinct NEW output names; it did not delete/reuse the existing output or relabel it unpublished. One attempt to save computed counts lacked the external-directory sandbox override and was denied; the same already-completed event log was later parsed and saved with the authorized override, without rerunning tests. Initial command output remains represented in the diagnostic ledger; these were evidence-tooling issues, not producer test failures.

E/lifecycle.mjs reopens ON with its generated source temporarily unavailable, separately restarts the foreign-data view, preserves selected snapshot bytes, restores the source, verifies six forbidden inventory/helper/path routes, then sends `st` and `op\n` to the actual viewer while stdin remains open. The viewer exits 0 naturally, waits reader exit 0 and closes its listener, with no forced cleanup causing that success. Browser harness termination remains forced and waited, not natural-stop or interactive-console evidence. Every owned process is stopped/waited at handoff.

Actual query/input/ID/hash/size observations and screenshots are retained in browser directories. Visually inspected examples include browser-on/scope-library.png and browser-final-input/exact-record-3.png, showing durable ON counts and exact CRLF entry with its own evidence. Catalog viewport is 1440x1024 CSS at scale 2 (2880x2048 PNG); static additionally runs its narrow smoke viewport. The inherited JSON's two viewport labels do not imply both ran for catalog sessions. No unsupported Figma fidelity claim is made.

Historical author/reviewer results remain historical: original final inventory 10/28 and earlier overlapping runs; substantive reviewer 17/47, supplemental 3/13, Node 5/7/4/8 and 12 completed browser sessions plus its two distinct failures. They are not new executions here. External-tar Unicode containment, PR-04, production catalogs, scale, hostile races, ACLs, power loss, native Linux/macOS runtime, native clipboard/interactive console and hardware remain unqualified.

## Runnable checkpoint and final identities

From the repository, browse an actual completed scoped snapshot:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-fix-20260920-163849'
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\on.json" --adapter $reader
# Open the printed loopback URL. Type stop, press Enter, and wait.
```

Examples for separately choosing NEW output names, not commands already executed with these filenames:

```powershell
$source = "$task\fixtures\- Generated café O'Brien &+%#"
& $reader --gui-disposable-inventory --ignore-ds-store=false $source "$task\snapshots\owner-off-next.json"
& $reader --gui-disposable-inventory --ignore-ds-store $source "$task\snapshots\owner-on-next.json"
```

Both names must still be absent; never replace retained outputs to replay setup. Producer completion and its published state precede viewing. A nonzero error after the no-replace link still acknowledges published=true; the final catalog is not removed. Exact executed argv/status and independent setup oracles remain in E/fixtures.mjs, producer-*.json, cli-option-cases.json, red/green encoding records and the command ledger.

Compared with the verified **279-file reviewed checkpoint**, authorized corrections modify **16 pre-existing paths**: gui_catalog.go; gui_inventory.go; gui_inventory_other.go; gui_inventory_windows.go; gui_inventory_windows_test.go; scripts/gui-preview/{README.md,browser-check.mjs,catalog-adapter.mjs,catalog-protocol.mjs,catalog-ui.mjs,server.mjs,style.css}; and the four living records. **Seven paths are added**: gui_encoding.go, gui_inventory_scope.go, gui_inventory_correction_test.go, scripts/gui-preview/catalog-names.mjs, inventory-correction.test.mjs, inventory-correction-browser-checks.mjs, and this report. The exact path lists, complete final working-byte manifest including tests/report, scoped candidate copies, diffs and separate report hash are retained in E/correction-paths.json, final-current-identities.json, candidate-identities.json, candidate/, final-preservation.json and report-identity.json. The final measured repository inventory has **286 entries**, with 263 pre-existing entries unchanged; those are preservation counts, not an audited-file total.

All unrelated source, shared production decoder/schema/backend code, original implementation/review reports, historical author/reviewer evidence, original Figma/PDF/ZIP references, HEAD and index are preserved. Authorized changed hashes are explicitly distinguished from preserved hashes; no claim is made that correction files remained identical. Generated catalogs, binaries, fixtures, screenshots and raw logs remain outside the repository. The evidence manifest excludes caches/temp/browser profiles and itself, not retained review artifacts.

ONE next gate: targeted reviewer recheck of F1/P2 original-byte rejection and valid controls; F2/P2 actual explicit input/query/selection semantics and failed-load protection; S1 classification/no-read/count/boundary behavior and strict versioned native Audit scope including old-reader refusal/legacy UNKNOWN; and directly affected publication, exact-ID, skip/disclosure/history, static, isolation and natural split-stop controls. Do not repeat the full substantive review or predeclare closure. No automatic review, publication, registration, comparison/parity, copy/restore, backend tar repair, OS port or media task follows.

Generation-independent preservation/buffering remains the direction. LTO-8 is the first physical qualification target, not a generation cap; other backends/generations require explicit qualification and Blu-ray remains separate. This candidate is developmental and disposable-only, not production or main-integration approval.
