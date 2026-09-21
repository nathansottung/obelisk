# Recorded snapshot comparison — author implementation, 2026-09-21

Status: **GUI_RECORDED_SNAPSHOT_COMPARISON_READY_FOR_REVIEW**. This is an
uncommitted author candidate, not a substantive review or owner acceptance.
Submitted Prompt25 is a conversation label, not an issued repository change ID.

## Provenance and identities

Branch: `feat/gui-recorded-snapshot-comparison`.
Parent and unchanged HEAD: `4abbfa566470b9f25e59c3eb190c801b622b613c`.
The preceding published implementation is
`265f93f334af8c529616caba00ec2bde1af89418`. The local starting branch was
`feat/gui-multi-snapshot-readonly`; tracked files/index were clean and no Git
operation was active. The new task branch was created at that parent. No remote
lookup or publication replay was performed.

Evidence root, abbreviated **R** below:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-recorded-comparison-20260921-153523`.
`submitted-prompt.txt` retains the complete authorization. `starting-checkpoint.json`,
`starting-identities.json` and `starting-source/` retain the starting identity and
actual tracked source bytes. The existing publication receipt under
`gui-multi-snapshot-publish-20260921-151311/publication-receipt.md` was reused.
`comparison-contract.txt` was recorded before implementation execution.

Full final raw SHA-256/size identities are in `candidate-identities.json` and
`binary-identities.json`; `candidate-source/` retains the exact source/document
bytes, including this report. `preservation.json` compares starting identities
and records final HEAD/index/operation markers and the unchanged package hash.
`input-identities.json` identifies generated test inputs. These external manifests
avoid a self-referential report checksum. The binaries were built from this task's
native source; final `gui_catalog.go`, native tests and double source are unchanged
since their respective successful builds/checks. Source copies include build inputs.

Modified runtime paths: `gui_catalog.go`, `scripts/gui-preview/catalog-adapter.mjs`,
`catalog-ui.mjs`, `server.mjs`, and `style.css`. New runtime paths:
`scripts/gui-preview/recorded-comparison.mjs` and `comparison-ui.mjs`.
Tests: new `gui_comparison_test.go`, `scripts/gui-preview/recorded-comparison.test.mjs`,
`comparison-browser-checks.mjs`, `testdata/comparison-reader-double.go`; changed
`multi-snapshot-browser-checks.mjs`. Documentation: preview README, existing
CODEX_HANDOFF/NEXT_ACTIONS/OB_STATUS/REVIEW_COVERAGE, and this single new report.

## Supported contract and implementation

The existing fixed maximum of two startup snapshots is unchanged. Comparison is
available only in the two-input session. Each source's complete A/B label, root,
recorded observation time and scope is shown before the explicit reference
selection and **Accept root alignment and compare recorded snapshots** action.
Argument order and dates never select an authoritative reference. Root text is
display evidence, never a host path to resolve or open.

The native read-only projection adds optional `comparisonFrame` metadata only
for one recorded Folder and one Collection with a nonempty root. It is preview
protocol metadata, not a native persisted format migration. Every projected
file must associate with that root. The declared convention is `slash-relative-v1`:
slash separates segments; internal backslashes are literal. Nonempty valid
Unicode keys preserve case, Unicode sequences, supported controls and punctuation.
Leading slash/backslash, drive prefixes, NUL, empty slash segments and `.`/`..`
segments are refused. No host normalization, suffix matching, basename join,
ID join, content deduplication or moved-file inference occurs. Duplicate keys
refuse the entire comparison. Unsupported/multi-root inputs remain browsable.

Each action invokes a new `{enumerate:true}` handshake on both adopted readers.
Native enumeration walks all validated stored Files, including retired
collections, independently of Search. The adapter requires a complete marker,
matching artifact digest/count and exactly all known unique IDs. Only after both
readers succeed does comparison use the adopted immutable projections. Completeness
means all stored records, not historical coverage of every source file.

Map joins produce the exact relative-key union, ordered by JavaScript UTF-16
string ordering, without locale folding. IDs and bytes remain decimal strings;
sizes use bounded exact int64 evidence and BigInt comparison. The five disjoint
classes and precedence are:

1. A missing side is only recorded in reference/counterpart; no invented ID.
2. Matching full comparable SHA-256 with known unequal exact sizes is inconsistent,
   hence inconclusive, before any difference/agreement rule.
3. Known unequal exact sizes or unequal full compatible SHA-256 establish recorded
   difference, with the reason displayed.
4. Full typed SHA-256 and exact sizes agreeing establish recorded checksum agreement.
   Hex case is immaterial; shortened/untyped digests are not used.
5. Otherwise the pair is inconclusive; equal timestamps/size alone do not suffice.

The UI displays union counts and each side's recorded-row total; paired and two
one-sided classes partition the union. Empty/all-excluded sets show zero keys,
never a health percentage. Full digest type/value, exact native identity/bytes,
original path, root, snapshot digest/handle/label, recorded time, scope and
first-seen evidence appear per side. Historical observation time remains unknown
when absent; load time and first-seen are not substitutes.

OFF/ON/UNKNOWN and validated completion/exclusion statements remain source-specific.
Scope differences qualify coverage, not paired content evidence. A one-sided
regular `.DS_Store` key against the exact exclusion policy can be described as
outside that policy. It does not invent individual excluded paths from counts,
apply the rule to near names/directories, or claim deletion/lost backup.

`GET /catalog-compare` accepts exactly one reference, counterpart and request
identity, all bound to the fixed startup handles; no path argument exists. The
response carries direction/request and exact per-side IDs. Browser validation
recomputes the deterministic expected envelope from validated adopted evidence
and refuses mismatch. Details select only those exact IDs. Reference/navigation
changes clear results; filter changes clear selection and invalidate pending work.
Epoch checks discard late successes/errors. Completed results filter locally by
literal case-sensitive path, exact JSON string or class. No persistent results,
export feature, registry, action on source files, or static-result fallback exists.

Malformed request selectors return 400 (unsupported HTTP methods 405); unsupported
frames/duplicate keys/oversize comparison return 422 while browsing survives.
Reader/enumeration failure returns 503 and latches/closes the pair; relaunch is
required. No surviving input is treated as an empty counterpart. UI errors clear
old counts/details. The common adapter permits one aggregate query/enumeration
at a time; busy requests fail instead of creating a work queue.

Existing native 4 MiB inputs, finite section/text limits, aggregate 1000 files and
1000 copy occurrences, 16 MiB reader frames, 32 MiB adopted pair and 5-second reader
deadlines remain unchanged. Comparison adds a 512-character request URL limit
and 8 MiB serialized response limit with refusal, not truncation. Exact paths
remain bounded by the existing 4096-byte name contract. Joining is bounded by
adopted records, with O(n log n) deterministic sorting; cancellation invalidates
browser use, while already-started finite reader work ends or times out. Existing
500ms forced termination fallback is retained and distinguished from natural stop.

Existing `snapshot.go` drive/folder routines were inspected: content-hash overlap,
similarity thresholds and selected peers/healthy-pair wording do not implement
this exact-path, explicitly selected five-way contract. They were not wired in or
modified. Native schema, operational backend workflows and dependencies are unchanged.

## Fresh validation and reproducibility

All execution below occurred for this author task. Prior multi-snapshot author,
reviewer and publication results are historical context, not added to these totals.
No execution test failed or needed retry. One extra browser run followed a small
control-spacing adjustment and addition of a class-filter assertion; it is counted
separately. Initial shell discovery included a missing root AGENTS.md lookup;
that read failure is not a passing test or an application failure.

Exact executable paths, environment variables, arguments and outputs are retained
in `run-native.ps1`, `native-exits.json`, `run-node.ps1`, `node-exit.json`,
`run-browsers.ps1`, `browser-exits.json`, `run-final-browser.ps1`,
`browser-final-exit.json`, `setup-comparison.mjs`, `producer-0.json`,
`producer-1.json`, `run-read-only-stop.ps1`, and `read-only-stop.mjs`.

Native selection used the installed Go 1.26.8 executable under the retained audit
toolchain, GOTOOLCHAIN=local, GOPROXY/GOSUMDB=off, CGO_ENABLED=0, GOWORK=off,
the retained module cache and fresh R cache/temp. Commands were
`build -mod=readonly -o R/output/reader.exe .`, build of each explicit reader double,
`test -mod=readonly -list ^TestGUI .`,
`test -mod=readonly -count=1 -json -run ^TestGUI -timeout 4m .`, and
`vet -mod=readonly .`. Builds/vet exit 0; changed Go files are gofmt-clean.
`native-tests.jsonl`: **28 top-level pass, 242 subtests pass, zero fail/skip**.
The unrelated archive suite was not rerun.

`node --test` selected the six files recorded in `run-node.ps1`: stop-command,
preview, catalog, catalog-correction, multi-snapshot, recorded-comparison.
**38 tests pass, zero suites/fail/cancel/skip/todo**. Native/protocol fault doubles
cover partial/truncated/unknown enumeration IDs, reader exit/timeout, request
selectors and refusal limits. The 1002-file adoption cap is exercised. The actual
server 8 MiB comparison cap uses two explicit protocol-double projections within
reader/pair bounds; it does not claim those oversized synthetic inputs pass the
native 4 MiB file gate. Invalid/incompatible SHA-512 evidence is tested at the
pure comparison boundary only; the native format still supports SHA-256.

Controlled native-validated `comparison-inputs/a.json` and `b.json` each contain
16 records, independently expected union 20: agreement 7, difference 3,
inconclusive 2, reference-only 4, counterpart-only 4. Independent cases are retained
in `independent-expected-cases.json`, literal assertions and browser expectations.
They cover equal-content distinct paths, same basenames in different directories,
case/Unicode distinctions, LF/CR/CRLF versus literal backslash-n, hostile markup,
long labels, contradictions, missing evidence and overlapping/large IDs.
IDs 9007199254740993/9007199254740994 and bytes
9007199254740992/9007199254740993 are independently asserted in the real browser.
These foreign-name/large-number fixtures are synthetic, not legal-Windows-source
claims. Fresh accepted producer runs generated `off-0.json`/`off-1.json` from
new R-owned sentinel sources; fresh native fixture execution also supplies scope
controls. OFF/OFF, OFF/ON, ON zero/positive, UNKNOWN, valid empty/all-excluded,
recorded time and reopened controls are exercised.

Installed Chrome **153.0.8010.48** ran **14 successful sessions**, separately from
test counts. Initial sessions/check entries: compare-forward 21, reverse 21,
long same-basename 21, reopen 21, scope 4, empty 4, refused 3, reader-failure 3,
existing native 17, large 16, controls 9, single 22, static 55. Final long-label
session after CSS/class-filter changes: **22 checks**. These are named harness
checks, not a claim to count every internal assertion. Browser evidence includes
both-sided selection, reversed direction/launch order, deterministic held old
success and injected late error after a newer reference/result/selection, mismatched
response ID refusal, safe text, exact filters, keyboard/history/skip focus and AX
combobox naming. Delayed/fault injection is explicitly a test seam; actual server
requests and native readers are used for the successful evidence cases.

All browser artifacts are under R/browser-*/ with browser-results.json and
catalog-process.json. Representative screenshots: `browser-final-long/comparison-before.png`,
`comparison-results.png`, `comparison-large-detail.png`, `comparison-invalid-response.png`;
`browser-compare-scope/compare-scope.png`, `browser-compare-empty/compare-empty.png`,
`browser-compare-refused/comparison-refused.png`, and
`browser-compare-failure/comparison-reader-failed.png`. Comparison uses 1440x1024
CSS pixels at scale 2; the static control also exercises 390x844. The final long-label
before screenshot and initial results were visually inspected. Full labels wrap;
final controls stack with spacing. This is a functional view using the established
shell, not a recovered Figma frame or claim of supplied comparison-design fidelity.

## Read-only and lifecycle evidence

Code-path inspection finds source/root paths used as text; enumeration reads the
already adopted in-memory native records. `read-only-stop-result.json` records
exclusive FileShare.None locks on both new sentinel source files before native
startup, then both adopted catalogs before actual comparison. Comparison succeeds
while locked (`locked-comparison.json`); a watcher observes zero input mutation
events. This observable boundary supplements byte preservation and is not a
host-wide syscall trace. Producer fixture creation is the separately authorized
task-owned source-read path, not comparison I/O.

`split-stop.json`: server PID 17188, reader PIDs 13904/7860, all exit 0; split
`st` then `op\n` with stdin held open, listener verified closed, forced=false.
Per-session logs retain browser/readers' waited exits. Deliberate fault cleanup
and harness teardown are not represented as successful natural stops. All owned
services used ephemeral loopback ports; no owner process or workspace was used.

Launch from the repository:

```powershell
$r = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-recorded-comparison-20260921-153523'
node scripts/gui-preview/server.mjs 0 --catalog "$r\comparison-inputs\a.json" --catalog "$r\comparison-inputs\b.json" --adapter "$r\output\reader.exe"
# Open the printed URL; select Compare recorded snapshots and a reference.
# Type stop followed by Enter, and wait for the reported server/reader exits.
```

## Preservation and remaining scope

Historical reports and original design inputs remain byte-preserved. Original
`Untitled.pdf` and `docs/Obelisk.fig` remain untracked; the existing design ZIP
remains ignored. Only the listed candidate paths/living records/new report change.
Index remains empty; no staging, commit, push, merge, tag or package refresh occurred.
Runtime/build evidence stays under R. No installations, network dependency fetch,
security changes/detections, private-data probes or media operations occurred.

Frozen ZIP:
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225\build-complete\0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`.
Unchanged SHA-256: `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`.
It contains neither this feature nor the preceding multi-snapshot development;
new readers are not replacements for it.

The existing external scope-reconciliation matrix was reused; its comparison
direction is now implemented only in this bounded generated-data preview. The
historically referenced repository feature-matrix file is absent. No competing
matrix was invented; the existing coverage ledger and living feature/status
records carry this update. One substantive reviewer pass is next.

Multi-root mapping, unsupported path frames, arbitrary algorithms, dynamic inputs,
durable storage identity, registration/history and exports remain out of scope.
No live verification, physical-copy proof, production scanning/recovery, native
schema migration or public package is claimed. Prompt20 remains **DEFERRED_BY_OWNER**;
no second-machine, Linux/macOS native, ACL, power-loss, concurrency or hardware
qualification is inferred. Generation-independent preservation/buffering, LTO-8
as first physical target rather than generation limit, explicit other-backend
qualification and a separate Blu-ray workflow remain planning directions.
