# Isolated GUI preview

## Two generated snapshots — uncommitted author candidate, 2026-09-21

The development server now accepts exactly two fixed startup inputs. This does
not update the frozen Windows package. From the repository in PowerShell:

```powershell
$multi = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-20260921-130125'
node scripts/gui-preview/server.mjs 0 --catalog "$multi\multi-inputs-v3\a.json" --catalog "$multi\multi-inputs-v3\b.json" --adapter "$multi\output\reader.exe"
# Type stop and press Enter; wait for both reader exits and listener shutdown.
```

Both example inputs are unaltered outputs of the native finite producer on new
synthetic sources. The separate `b-aged.json` is a labeled synthetic historical
timestamp fixture, not the original producer output. The binary is this new
candidate's reader, not the packaged dogfood executable.

Library presents two selectable entries. Find's **Snapshot view** selects All
loaded snapshots or either input. Each result and inspector carries its source;
native IDs remain exact strings in separate snapshot namespaces. Find's source
cards disclose scope and artifact details without hiding policy/recorded time.
New controls are functional choices, not supplied Figma frames.

Source labels now begin with **Snapshot A** or **Snapshot B**, followed by the
original catalog basename. These qualifiers are assigned to startup handles
once per session and remain consistent in Library, Find, scope summaries,
counts and the inspector. Reversing inputs on a new launch reverses the A/B
assignment; these are display labels, not durable catalog/device identities.
Long basenames wrap in result/source content, keeping the qualifier first.

The server issues random process-local handles, validates them against the
startup allowlist, and binds results as separate snapshot/record fields. Catalog
digests identify artifacts, not independent physical copies. Identical-byte
inputs (including renamed copies) are refused; overlapping records in different
artifacts are retained. No persistent recents, merge, registration or comparison.

Both readers must validate before a successful session. All queries require both
results; failure clears/latches the view without demo or one-input fallback.
There are at most two readers, one outstanding aggregate query, existing 5-second
reader response deadlines and stop/EOF with a 500ms forced-termination fallback.
Per-input native limits are unchanged. Aggregate adoption is capped at 1000 file
records and 1000 copy occurrences; each reader response is capped at 16MiB and
the serialized pair projection at 32MiB. There is no truncation at supported
scale: native queries limit at 1000 against at most 1000 loaded records. Counts
are returned recorded entries, not merged scope, unique content or verified copies.

`recordedAt` projects the existing validated inventory Audit event timestamp
(including its offset); absent historical event time stays unknown. Load time
remains separate. OFF/ON/UNKNOWN and each snapshot's exclusion counts remain
independent. Source/media paths remain text. Browser filter/query changes discard
old selection and late responses; no browser file picker or arbitrary path route.

Original launches remain supported:

```powershell
node scripts/gui-preview/server.mjs 0
node scripts/gui-preview/server.mjs 0 --catalog "$multi\multi-inputs-v3\a.json" --adapter "$multi\output\reader.exe"
```

See [the implementation and author validation report](../../docs/development/reviews/GUI-MULTI-SNAPSHOT-READONLY-IMPLEMENTATION-2026-09-21.md).
The [focused recheck](../../docs/development/reviews/GUI-MULTI-SNAPSHOT-READONLY-FOCUSED-RECHECK-2026-09-21.md)
closed R1's same-basename ambiguity. The owner accepted this bounded source
milestone on 2026-09-21; the [acceptance record](../../docs/development/reviews/GUI-MULTI-SNAPSHOT-SOURCE-ACCEPTANCE-2026-09-21.md)
separates source publication from the unchanged frozen package. Prompt20 remains
DEFERRED_BY_OWNER, with second-machine qualification pending.

## Earlier accepted single/static preview and historical evidence

Library and the Smith Wedding comparison now use the supplied PDF references (pages 1 and 4) with a provisional teal/light treatment. Pages 2 and 3 remain unused visual alternatives, not disclosure modes. This is a synthetic design preview, not a final theme decision or production UI.

Requires preinstalled Node.js 24 (executed here with 24.19.0). No package install, Go backend, build step, configuration, catalog or device is needed.

From PowerShell:

```powershell
Set-Location 'C:\Users\nsott\source\repos\obelisk'
node scripts/gui-preview/server.mjs 0
```

Open the exact `http://127.0.0.1:<assigned-port>/` printed in that terminal. Port `0` asks Windows for an available port. To request a fixed port, replace `0` with e.g. `4317`; an occupied port fails without changing or stopping its existing listener. Only the exact loopback host is accepted. Do not launch the normal Go application for this preview.

Stop with **Ctrl+C** in that terminal and wait for the PowerShell prompt to return. The launcher also closes after 60 minutes. It writes no settings or browser storage. Closing the browser tab alone does not stop the server.

Library supports inventory search, storage selection, and Smith Wedding navigation via its name, Fix issues, or Compare copies. Return with the Library breadcrumb. Project search filters the supplied selected-file example; HDD-017 expands/collapses. Operational-looking controls produce explicit demo-only responses and perform no operations. Review wording retains the source ambiguity: six absent/differing items are called unresolved by that action, while the narrower matrix Unresolved count stays zero. The offline count of 200 is kept visible instead of reproducing the source clipping.

Find retains text search by name/path/hash/version/occurrence/medium, project/medium/availability filters, selection and occurrence details. The same original Library content inspector is available in the additional-preview-controls disclosure below the aligned tables. Use the fixture presentation selector for ready, partial-information, empty, held-loading and error cases; choose Ready to retry. Reset filters restores the default sample. These extra controls and Find's visual layout are not specified by the export.

Guided shows core evidence; Studio adds synthetic hash and inventory scope; Expert also exposes the synthetic medium identifier. All modes retain offline/unknown/verification warnings. Display selection lasts for this page session. It changes no guarantees or permissions. Back Up and Archive are workflow placeholders; Activity and Devices show static samples; operational Settings are deferred. No operational action is wired.

Checks:

```powershell
node --test --test-isolation=none scripts/gui-preview/preview.test.mjs
```

Optional rendering check with an already installed Chrome (no package installation). Supply a **new**, absolute evidence directory under AppData; the harness refuses to reuse an existing directory. It starts a loopback server and headless browser with a disposable profile, captures screenshots and a browser accessibility tree, then stops and waits. A 90-second deadline bounds the run. Example:

```powershell
node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-preview-owner-check-NEW-TIMESTAMP'
```

The preview lives outside Go's embedded UI tree. The server serves an explicit asset allowlist (HTML, application, fixtures, stylesheet, catalog view and mode bootstrap), refuses other methods and unknown paths, and has no filesystem browsing or production API proxy. Static mode blocks browser connections; catalog mode permits its same-origin bounded query endpoint. Test harnesses and the process bridge are never served.

The current shell/Library/project references are readable and have been applied. Find, other workspaces, dialogs, additional disclosure layouts and responsive designs remain unspecified. Studio is initially selected as in all three Library references. Arial/system fonts and locally drawn SVG icons substitute for unprovided font/icon assets. Browser comparison uses 1440x1024 CSS pixels at scale 2 as a PDF-based assumption. Owner visual review and broader accessibility review remain outstanding. See the 2026-09-20 design-alignment report under docs/development/reviews for executed checks and screenshot locations.

## Disposable native catalog mode

The static launch above remains supported. The owner accepted its bounded visual milestone in the published GUI evidence commit; the historical implementation reports retain their original dispositions. Catalog mode is an uncommitted candidate with R1-R3 author corrections awaiting targeted reviewer recheck.

Use only task-owned synthetic inputs. The correction task copied fixtures into `C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-fix-20260920-131810\inputs`, separately from its `output` directory, and prepared exact-ID/fault controls in `inputs-regression`. Original author/reviewer evidence remains unchanged. Native schema 8 is required; populated advanced sections (including profiles, assignments, events, snapshots and keys), retained file versions and spanned chunks are refused in this initial slice. Supported: collections, folders, current files, nonspanned chunks/copies, volumes and locations. File IDs are native logical-record identities, not deduplicated content IDs. A copy is shown for a current file only when chunk membership records the same file ID and hash.

Reader limits: 4 MiB input; 1000 files; 100 rows per collection/folder/chunk/volume/location table; 1000 potential membership-copy combinations; 4096-byte strings; native file size must be explicit. Queries reuse native `Store.Search`: relative-path substring (256 UTF-8 bytes) and SHA-256 prefix (64 bytes), combined as AND, up to 1000 results. Retired collections are excluded from the native default search. A busy reader refuses a concurrent request with 503 instead of mixing responses. Queries do not read the input again or search source folders. No automatic reload or browser input picker exists; stop/relaunch to select another file.

Build and fixture setup, from repository root, using the already installed toolchain/cache (no downloads):

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-catalog-fix-20260920-131810'
$env:GOMODCACHE = 'C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod'
$go = "$env:GOMODCACHE\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\go.exe"
$env:GOTOOLCHAIN = 'local'
$env:GOPROXY = 'off'
$env:GOFLAGS = '-mod=readonly'
$env:CGO_ENABLED = '0'
$env:GOCACHE = "$task\output\go-cache"
$env:TEMP = "$task\output\go-temp"
$env:TMP = $env:TEMP
Remove-Item Env:GOTMPDIR -ErrorAction SilentlyContinue
# The retained corrected-reader.exe is already built. Use a new name for a rebuild:
$reader = "$task\output\reader-$(Get-Date -Format yyyyMMdd-HHmmss).exe"
if (Test-Path -LiteralPath $reader) { throw 'Choose a new output name' }
& $go build -o $reader .
# Existing copied fixtures are ready. For NEW fixtures use a NEW nonexistent directory:
$env:OBELISK_GUI_FIXTURE_OUTPUT = "$task\inputs-new"
& $go test -count=1 -run '^TestGUICatalogGenerateFixtures$' .
Remove-Item Env:OBELISK_GUI_FIXTURE_OUTPUT
```

Fixture setup intentionally writes native catalog JSON using native Go types. Never rerun it against an existing input directory. It creates ALPHA/BETA/valid-empty and malformed/zero/future/directory controls; missing.json is deliberately absent. It creates no config or keys. Build/setup complete before reading.

Launch against the existing prepared fixture:

```powershell
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\inputs\alpha.json" --adapter $reader
```

Open the newly printed loopback URL. Use beta.json to see different persisted records, or empty.json for a valid empty representation. Input and adapter arguments must be absolute; the input parent must resolve beneath ObeliskDev. The file itself must be regular and not a link. This is operator-selected disposable storage, not permission to read live catalogs. The direct Go command is `corrected-reader.exe --gui-catalog-readonly <file>`; it runs before production startup and exchanges projection/search JSON over stdio, with no HTTP backend.

Expected notice: **Disposable catalog mode — reading a synthetic catalog snapshot. No source/media files are opened. The selected catalog is not modified.** Library shows recorded collection/storage labels; Find queries recorded relative paths and hashes; selection displays native record/source-folder/size/hash/first-seen and matching chunk-copy evidence in the inspector. Source paths remain text. Capacities, accessibility, parity and verification performed now are unavailable. Failed load never falls back to static samples; failed queries clear results. Session load time and input digest are separate from recorded observation times.

Stop with Ctrl+C and wait for the terminal prompt, or type `stop` and press Enter. Automated callers can send `stop\n` while keeping stdin open and wait for exit 0. Commands are case-sensitive after trimming surrounding whitespace, LF-delimited with optional CR. Split/one-character chunks work; blank/unknown lines are ignored. A final partial `stop` at EOF is accepted; bare EOF or an incomplete prefix does not stop the listener. Lines over 4096 bytes are discarded through the next LF, including any command-looking suffix. Shutdown is idempotent, removes its stdin/signal listeners, closes connections, ends reader stdin, and waits. A reader that does not exit within 500 ms is terminated and waited; normal native runs exited 0. Windows process.kill is abrupt and is not an interactive Ctrl+C test. The stop message records the reason and reader exit. No live operations are enabled.

Checks (fixture setup must have completed first):

```powershell
$env:OBELISK_GUI_TEST_INPUTS = "$task\inputs"
$env:OBELISK_GUI_TEST_ADAPTER = $reader
node --test --test-isolation=none scripts/gui-preview/catalog.test.mjs
node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' "$task\output\browser-NEW" "$task\inputs\alpha.json" $reader ALPHA
```

The browser runner also accepts BETA, empty or refused as the final expectation. Evidence directories must be new. Static browser syntax remains unchanged. This is not production integration, an ACL/concurrency qualification, cross-platform or media testing. See the disposable-catalog implementation report for actual executions, failures/retries, limits and exact candidate identities.

### Corrected protocol and focused recheck

Pair the current server/browser with the corrected reader. Its preview projection emits collection/volume/file IDs and search IDs as canonical positive decimal strings; the native signed-int maximum is advertised as `idMax`. Native relationships still join exact Go ints. Copy identifiers embed the exact native chunk ID plus a bounded positional copy index. Recorded int64 byte sizes are decimal strings, including zero. No persisted schema change or large-ID rejection/renumbering occurs. The Node/browser response validator rejects numeric, malformed, duplicate, unknown and out-of-range IDs without coercion or fallback. Search requests still contain only text/hash, not an ID selector. The old numeric-ID reader is not compatible with this corrected protocol.

Startup adopts only a validated, newline-terminated response envelope. Detected process/transport/protocol failure invalidates the server mode; mode.mjs is generated from current reader state. A failed browser query clears the entire catalog view, identity and selection, and latches an explicit relaunch instruction; a late response or disclosure rerender cannot restore old evidence. A valid completed snapshot is not retroactively described as never having existed. No data is retained in the failed view. Relaunch/reopen is the existing recovery path; no new picker, retry framework or background health polling was added. Idle pages are not continuously notified of external failures: the next read/bootstrap observes failure. HTML-shell HTTP 200 is not catalog success.

Correction fixture setup uses exact native types and integer formatting:

```powershell
# Optional NEW setup only; never overwrite inputs-regression or historical inputs.
$env:OBELISK_GUI_CORRECTION_FIXTURES = "$task\inputs-regression-NEW"
& $go test -count=1 -run '^TestGUICatalogCorrectionFixtures$' .
Remove-Item Env:OBELISK_GUI_CORRECTION_FIXTURES
# Use existing prepared controls and retained test-only emitter for this checkpoint:
$env:OBELISK_GUI_CORRECTION_INPUTS = "$task\inputs-regression"
$env:OBELISK_GUI_TEST_DOUBLE = "$task\output\reader-double.exe"
node --test --test-isolation=none scripts/gui-preview/stop-command.test.mjs
node --test --test-isolation=none scripts/gui-preview/catalog-correction.test.mjs
```

The double is explicitly compiled from `testdata/reader-double.go` using `go build -o <new-double-output.exe> scripts/gui-preview/testdata/reader-double.go`; it is not the native adapter and reads only its prepared projection for fault tests. Tests distinguish real reader exits from intentional faults. Browser expectations also include exact, exact-reversed, review-integers, failure-exit, failure-shape, failure-query, failure-invalid-id and failure-late with their corresponding prepared inputs. See the 2026-09-20 follow-up report for actual runs, baseline reproductions and the pending targeted-recheck gate. Author-addressed does not mean reviewer closure.

## Disposable directory inventory candidate

The catalog milestone above was accepted and published at `5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`; its older pending-review wording is historical. The new `feat/gui-disposable-inventory` candidate is uncommitted and awaits one substantive review. It adds a local producer, not a browser scanning workflow.

The producer **reads the explicitly selected generated test files**, computes real SHA-256 and catalog-only BLAKE3, and publishes a new native schema-8 catalog. The viewer then reads only that catalog; it does not reopen recorded source paths. Separate file records preserve distinct paths, including equal bytes and duplicate basenames. Folder.Path plus File.RelPath locate each observation; a content hash is not a pathname or a copy count. No storage registration, Volume/Location or backup-package/copy record is fabricated. CreatedAt/FirstSeen describe this observation session; mtime and byte size are observed from the file. Current availability, capacity, parity, backup success and verification remain unavailable.

Use **only newly generated expendable directories**, with separate source and output-parent trees. Both must already exist, use bounded absolute paths, and be disjoint by lexical containment and actual ancestor identities. The output filename must remain absent. Do not select the repository, historical evidence, real archives or media. The portable command has no implicit roots or Nathan-specific security boundary; this is a developmental operator contract, not a sandbox against arbitrary authorized-user paths or malicious filesystem changes.

Executed checkpoint (from repository root):

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-20260920-145714'
$reader = "$task\output\inventory-reader-bounded.exe"
# A completed, retained snapshot is ready for read-only viewing:
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\alpha-final.json" --adapter $reader
# Open the printed loopback URL. Type stop, press Enter, and wait for termination.
```

The finite producer invocation used for that snapshot was:

```powershell
& $reader --gui-disposable-inventory "$task\fixtures\ALPHA" "$task\snapshots\alpha-final.json"
```

That output now exists: rerunning that exact producer command **must refuse**, preserving the snapshot. To make another observation, explicitly choose a new absent output name in the existing separate snapshots directory. Wait for producer exit and `published:true` before launching the viewer. A nonzero exit can still report `published:true` if cleanup/status reporting failed after publication; inspect the named catalog/staging path and do not delete the final file to simulate rollback.

Limits: 64 files, 128 total entries, depth 8, 512-byte relative paths, 4096-byte absolute paths, 8 MiB observed bytes per file and 32 MiB total. One additional byte may be read to detect growth, then the run refuses. All files must be regular; every encountered entry and existing path ancestor is checked. Windows supports ordinary fixed local drive paths and refuses UNC/device/stream/ambiguous names and all reparse attributes, including junctions. Other-platform compilation policy exists but is not runtime-qualified. Do not infer unrestricted source/backend support.

The directory must remain quiescent. The producer detects observed identity/mode/size/mtime changes and directory-entry differences, but it is not an atomic filesystem snapshot or race-resistant confinement. A 30-second cooperative deadline/cancellation is checked between filesystem operations and bounded reads; it cannot forcibly interrupt a blocked kernel filesystem call. Progress gives processed file/read-byte counts, not an invented percentage. Error, unsupported entry, limit or cancellation before publication leaves no successful final snapshot.

After full construction and validation by the unchanged accepted native reader, the producer writes/syncs/closes a unique temporary in the output parent. `os.Link` publishes it only if the final name is still absent. A late destination survives. Hard-link support is an output-filesystem prerequisite for this producer only; no replacement fallback or power-loss durability is claimed. Only the operation's own staging name is removed. Cleanup failures identify remaining staging and whether publication occurred. Source trees receive no producer writes, markers or sidecars.

Builds use the installed offline Go environment above, with this task's output/go-cache and output/go-temp as GOCACHE/TEMP/TMP and GOTMPDIR unset. Use a new binary output name for any later rebuild; never overwrite retained evidence. Actual setup/build/test argv, environment and outcomes are in the task scripts and implementation report. `setup-and-produce.mjs` is retained execution provenance and refuses reuse of its generated directories; do not replay it over this checkpoint. The generated ALPHA tree has one explicitly recorded post-first-snapshot test change; alpha.json remains the original snapshot, alpha2.json and alpha-final.json are separate later observations.

New source tests are `TestGUIInventory*` and `TestHashReaderBothRefusesPartialError`. Existing affected hashing/scanner and reader/Node/browser controls remain separate. Test-only `inventory-browser-checks.mjs` reads an independently prepared oracle through `OBELISK_GUI_INVENTORY_ORACLE`; neither the producer nor served application reads that oracle. It is never served. No new design frame or layout is claimed. See [the inventory implementation report](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md) for exact executions, screenshots, identities and review scope.

### Filename corrections and recorded exclusion scope - 2026-09-20

The [substantive review](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-REVIEW-2026-09-20.md) found F1 (P2, malformed encoded names) and F2 (P2, newline search), and recorded missing requested functionality S1. The correction candidate addresses F1/F2 and implements S1; it awaits a targeted reviewer recheck. This supersedes earlier pending-substantive-review and unchanged-reader wording for this candidate. It is not reviewer closure or publication.

Names support valid Unicode scalar values, retaining enumerated spelling without normalization. Preview catalog JSON is checked in its original UTF-8 bytes and active JSON escapes before the shared native decoder can replace malformed input. Invalid UTF-8 and isolated surrogate escapes refuse the entire snapshot; valid U+FFFD, supplementary pairs and literal backslash-u text remain valid. Windows enumeration additionally checks raw UTF-16 names before conversion; non-Windows raw Go names must be valid UTF-8. This is not an arbitrary-byte filename format, a production decoder change, or other-platform runtime qualification. CLI arguments must themselves arrive as valid Unicode; the process cannot reconstruct characters already changed by a caller or terminal.

Ordinary **Recorded path contains** keeps its existing literal, case-insensitive substring behavior (including the existing surrounding-whitespace trimming); it never interprets backslash escapes. To enter a whole recorded name exactly, enable **Exact name (JSON string)** and enter one quoted JSON string. Examples:

| Intended name data | Exact entry |
|---|---|
| Actual LF between `line` and `name.txt` | `"line\nname.txt"` |
| Actual CR / CRLF | `"line\rname.txt"` / `"line\r\nname.txt"` |
| Literal backslash followed by n | `"line\\nname.txt"` |
| Literal Windows-style path | `"C:\\folder\\new.txt"` |
| Leading/trailing spaces or tab | `" name "` / `"\tname\t"` |

The inspector's **Use exact name** button fills this explicit mode for the selected record. Control-containing or edge-whitespace names display as `JSON name: "..."`; authoritative records and native IDs stay unchanged. Invalid exact entry shows an error, clears results and sends no repaired query. Enter submits immediately. Exact mode matches case and every character and uses no SHA-256 filter; switching back restores ordinary search. Disclosure changes, Back/Forward and skip links preserve the active entry/selection. A fresh launch rereads the catalog; query state remains session-local. Browser checks use CDP text insertion, not native OS clipboard qualification.

The exact query is a bounded additive preview operation: at most 4096 UTF-8 bytes after decoding the JSON string, at most 24578 input characters, a 16384-character URL cap and a 32768-byte native request-line buffer. Ordinary text/hash limits remain 256/64 and ordinary URL/native-line limits remain 2048/4096. The catalog's 4 MiB and existing row/string/subset limits are not raised. The only new served asset is the fixed `catalog-names.mjs`; there is no producer endpoint or arbitrary path/executable selection.

**Ignore macOS Finder metadata (.DS_Store)** is a producer option, default OFF. Exclude these files from this inventory; leave the source files unchanged. Actual syntax:

```text
--gui-disposable-inventory [--ignore-ds-store[=true|false]] ABS_GENERATED_SOURCE ABS_NEW_CATALOG
```

The flag precedes both paths. Bare flag and `=true` enable it; absence or `=false` disable it. Unknown values, repetition, misplaced flags and incorrect argument counts refuse with usage. `--gui-disposable-inventory --help` currently prints usage as an error and exits 1. There is no global preference.

Only a classified regular file whose enumerated basename is exactly `.DS_Store` is excluded, at any depth and on any build. Links/reparse/special entries are refused before filtering; a directory of that name is traversed. Near names, case variants, other dotfiles, `._*`, `__MACOSX`, `.Spotlight-V100`, `.Trashes`, Thumbs.db, desktop.ini and .xmp/.aae remain outside this rule. Excluded files count toward the existing 64-regular-file and 128-entry bounds and are rechecked, but their contents are not opened or hashed. Errors/cancellation still prevent publication.

Each new snapshot records its effective policy and observed entry/regular/included/excluded/read-byte counts in ONE existing native `audit` event: action `GUI_DISPOSABLE_INVENTORY_V1`, timestamp equal to the collection observation time, bounded JSON `detail` with version 1 and complete=true. The versioned event uses native Audit history, not an invented catalog field, sentinel record, sidecar or migration. The preview accepts only that exact audited scope shape, with required fields, strict counts/relationships and no other populated advanced section. Absent audit means historical policy/counts UNKNOWN. Empty and all-excluded scopes are distinct after reopen. Scope is recorded history, not live or whole-source verification.

**Compatibility:** the pre-correction preview refuses new populated-audit snapshots. It does not silently drop their scope, and must not be advertised as compatible with these new outputs. The corrected reader continues to accept older supported snapshots with UNKNOWN scope and displays their existing `.DS_Store` records. Pair new output with the corrected reader. Broader production readers and native audit consumers have not been qualified by these preview checks.

Current retained output and viewer command, from the repository root:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-fix-20260920-163849'
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\on.json" --adapter $reader
# Open the printed loopback URL. Type stop, press Enter, and wait.
```

`off.json`, `empty.json`, `all-excluded.json`, `on-zero.json`, `legacy.json` and `foreign.json` are also retained actual inputs. Producer examples below create NEW outputs and were not executed with these names; the filenames must still be absent. The source is the generated correction fixture, not a production folder:

```powershell
$source = "$task\fixtures\- Generated café O'Brien &+%#"
& $reader --gui-disposable-inventory --ignore-ds-store=false $source "$task\snapshots\owner-off-next.json"
& $reader --gui-disposable-inventory --ignore-ds-store $source "$task\snapshots\owner-on-next.json"
```

Wait for producer completion and inspect both exit status and `published` before viewing. Existing and late-arriving outputs remain protected by the existing no-replace publication operation. No source cleanup or output replacement is authorized. See the [correction follow-up](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-REVIEW-FOLLOWUP-2026-09-20.md) for exact identities, execution stages, screenshots, retained diagnostics and the one targeted-review gate.

### Scope-key correction - 2026-09-20

The [focused review](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-FOCUSED-RECHECK-2026-09-20.md) closed F1/P2 and F2/P2 but found S1-R1/P2: case-variant audit-detail keys could conceal missing counts and produce a false complete/empty disclosure. The bounded correction is author-addressed and awaits a targeted recheck; S1 is not yet reviewer-closed.

The preview now checks the original catalog's decoded member names before native struct decoding. Only canonical `audit`, event `at`/`action`/`detail`, and detail `version`/`policy`/`entries`/`regularFiles`/`includedFiles`/`excludedFiles`/`readBytes`/`complete` are accepted on this scope path. Each required member must appear exactly once and be non-null; existing type/value/count relationships still apply. Case variants, duplicate wrappers/members, encoded duplicates and conflicting aliases refuse the whole snapshot. Valid JSON escapes spelling a canonical name are accepted. Filename, path, search and ID spelling is not normalized.

Genuine older no-history forms (missing `audit`, `audit:null`, `audit:[]`) still mean UNKNOWN scope. They are native empty-history representations, not present scope objects with missing counts. A present malformed event/detail or a case-variant/duplicate audit wrapper cannot downgrade to UNKNOWN. The raw check runs before reader adoption and through the producer's existing complete validation before no-replace publication. There is no schema migration, global decoder replacement or new publication protocol. The previous reader still refuses populated audit; pair scoped output with the corrected reader.

Actual newly built binary and retained generated snapshot:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-scope-fix-20260920-194852'
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\on.json" --adapter $reader
# Open the printed loopback URL. Type stop, press Enter, and wait for termination.
```

The OFF, ON-zero, empty, all-excluded and legacy snapshots are also retained in that directory. Do not rerun a producer over an existing output. See [the S1-R1 follow-up](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-SCOPE-S1-R1-FOLLOWUP-2026-09-20.md) for exact source/binary identities, raw adversarial fixtures, tests, screenshots and the next review gate.


## 2026-09-20 - owner acceptance of reviewed disposable inventory

Publication closeout documentation: owner accepted the bounded Windows milestone at 2026-09-20 21:21:07 America/New_York (2026-09-21T01:21:07.268Z). Corrected implementation: 8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c. F1/F2 remain previously closed; S1-R1 is closed and S1 is implemented and verified by the [closing recheck](../../docs/development/reviews/GUI-DISPOSABLE-INVENTORY-SCOPE-S1-R1-FOCUSED-RECHECK-2026-09-20.md). Earlier candidate/pending wording above is historical. This notice is added in the separate documentation commit; it does not change any command, launcher, schema or runtime behavior.

Newly scoped catalogs require the corrected compatible inventory/preview reader from implementation 8eb178bcb6f8f7367d2cb9aa75d2f4059d92a85c. The previously identified pre-correction uncommitted reader (SHA-256 bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69) refuses populated Audit scope; this documented refusal is not corruption or universal backward compatibility. Supported older unscoped catalogs remain readable with scope UNKNOWN, never inferred OFF/zero. Pair outputs and readers by their documented source/build identities; an earlier prepared executable is not automatically suitable. Do not remove or rewrite scope metadata to make an older reader accept a catalog. No automatic catalog migration is performed.

Use the retained corrected reader and output identities in the closing report, or build this exact implementation with the already documented go build -o <new-reader-output.exe> . command and isolated offline build environment. The finite producer syntax and new-output prerequisites above remain unchanged. Never overwrite retained binaries or snapshots. No executable was rebuilt during this publication. The final evidence tip and live remote verification are recorded in C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-publish-20260920-212106\publication-receipt.md.


## Local Windows developer-alpha package candidate - 2026-09-20

A local unsigned packaging candidate now wraps this unchanged preview and finite inventory producer. It is uncommitted on feat/windows-inventory-alpha-package at bfbce891df78d529c6be2d2912dc8443597c007e and awaits substantive package review; it is not a public release. See the package [quickstart](../windows-alpha/QUICKSTART.md), [supported envelope](../windows-alpha/SUPPORTED.md), and [implementation/qualification report](../../docs/development/reviews/WINDOWS-INVENTORY-ALPHA-PACKAGE-IMPLEMENTATION-2026-09-20.md). Testers use package-relative commands with existing Node 24 x64, Windows PowerShell and a browser, without Go/Git or this checkout. The accepted ObeliskDev catalog boundary and corrected-reader/UNKNOWN compatibility notice remain in force. No production scan, installation or security bypass is authorized.
