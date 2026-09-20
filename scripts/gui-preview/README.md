# Isolated GUI preview

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
