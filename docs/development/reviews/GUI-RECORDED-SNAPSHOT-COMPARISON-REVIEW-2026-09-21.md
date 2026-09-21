# Recorded snapshot comparison substantive review — 2026-09-21

**GUI_RECORDED_SNAPSHOT_COMPARISON_READY_FOR_OWNER_REVIEW**

No material blocker was found in the selected generated-data, two-fixed-snapshot
contract. The explicit reference/root acceptance, exact-key classification,
complete recorded-set requirement, two-sided attribution and read-only boundary
are supported by source inspection and fresh execution below. No implementation
fix was made. This is bounded review readiness, not owner acceptance or publication.

This is a same-session AI-assisted reviewer pass under separately submitted
Prompt26, with author context available. It is not an independent/context-isolated
agent review, human certification or screen-reader execution. Prompt26 is a
conversation label, not a new repository issue identifier.

## Candidate and provenance

Branch: `feat/gui-recorded-snapshot-comparison`.
Full HEAD and complete uncommitted patch parent:
`4abbfa566470b9f25e59c3eb190c801b622b613c`.
The index was empty and Git operation markers absent. No branch change or remote
operation occurred. The comparison implementation is in working files, not HEAD.

Author root (**A**):
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-recorded-comparison-20260921-153523`.
Controlling report: [author implementation](GUI-RECORDED-SNAPSHOT-COMPARISON-IMPLEMENTATION-2026-09-21.md).
All **320 entries** in A/candidate-identities.json matched raw working bytes.
Manifest SHA-256:
`b3b6e59baef437b84b25803a7ee60e826362f6f902cf7e359964e30250eb7df2`.
This is an identity inventory, not a claim that 320 files received substantive audit.

Reviewer root (**R**):
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-recorded-comparison-review-20260921-162955`.
`checkpoint.json`, `starting-identities.json`, `tracked-delta.patch` and `source/`
preserve the actual starting candidate, full tracked delta and new-file bytes.
`submitted-prompt.txt` retains this authorization. No matching completed substantive
comparison review existed; the author report and repeated identity-only turn
were not treated as a review. `design-identities.json` separately accounts for
untracked `Untitled.pdf`, `docs/Obelisk.fig`, and the ignored readable-design ZIP.

The complete candidate delta reviewed against the parent comprises:

| Category | Paths |
| --- | --- |
| Modified runtime | `gui_catalog.go`; `scripts/gui-preview/catalog-adapter.mjs`, `catalog-ui.mjs`, `server.mjs`, `style.css` |
| New runtime | `scripts/gui-preview/recorded-comparison.mjs`, `comparison-ui.mjs` |
| Modified test routing | `scripts/gui-preview/multi-snapshot-browser-checks.mjs` |
| New tests/double | `gui_comparison_test.go`; `scripts/gui-preview/recorded-comparison.test.mjs`, `comparison-browser-checks.mjs`, `testdata/comparison-reader-double.go` |
| Candidate documentation | preview README; CODEX_HANDOFF, NEXT_ACTIONS, OB_STATUS, REVIEW_COVERAGE; author implementation report |

There are 11 modified tracked paths and seven new candidate paths, excluding the
two pre-existing untracked design inputs. This review adds only this report.
The source trace also covered the shared protocol/name validation, browser
harness, native read/validation/projection, inventory scope and producer hashing.
Current living entries, the retained Prompt25/contract, prior multi-snapshot
review/label closure/acceptance, and external scope-reconciliation mapping were
consulted. Historical instructions were not executed. No applicable AGENTS.md
was located. No repository feature matrix exists; its external mapping and
existing coverage records were reused without adding another matrix.

## Contract dispositions

### A. Explicit reference, frame and exact pairing — supported

`comparison-ui.mjs:6` creates a fresh view with no selected reference and a
disabled action until a reference is chosen. Full A/B labels, roots, recorded
times and per-source scope cards precede the explicit alignment-acceptance action.
The label/date/launch order is never authority. Swapping reference clears the
previous result and detail. Both directions and reversed launches were exercised.

`gui_catalog.go` projection supplies optional preview-only `comparisonFrame`
for exactly one recorded Folder and Collection. Native relationships are checked
before projection. `recorded-comparison.mjs:6` requires `slash-relative-v1`, a
nonempty root, consistent record count and exact file/root association. Slash
alone separates components; internal backslashes remain literal. Case, Unicode
sequences, supported controls and punctuation are not normalized. Leading
slash/backslash, drive prefixes, NUL, empty components and dot/dot-dot components
refuse comparison. Root text is not resolved with the host filesystem.

Map joins use exact keys and deterministic UTF-16 ordering, not IDs, basenames,
hashes or display strings. Duplicate keys are refused before union/counting.
Reviewer native probes reverse duplicate row order and place the ambiguous input
on either side; all four refuse with 422 while browsing succeeds. Native missing
root/absolute key and protocol-only missing frame, foreign convention and wrong
root association refuse. The latter are explicitly projection-boundary probes,
not claims those faulty projections came from the native reader. Deliberate
single-root support is an accepted boundary, not a request for a mapping editor.

### B. Evidence, exact values and count invariants — supported

`recorded-comparison.mjs:20` checks contradictory matching full SHA-256 with
unequal exact sizes before assigning difference/agreement. Known unequal size
or unequal compatible full hash gives difference; equal metadata without
supported full content evidence is inconclusive. Equal full typed SHA-256 and
exact size gives recorded agreement. Native `File.Hash` is full-file SHA-256
content evidence: `gui_inventory.go` hashes the bounded full file through
`hashReaderBoth` and records its exact size; `hashing.go` uses SHA-256 streaming.
Display abbreviations, partial-file hashes and artifact digests do not classify
content. Nonempty unsupported native hashes are refused by native validation.

Projection serializes int/native IDs and int64 sizes to decimal strings before
JavaScript parsing. Protocol validation bounds them; classification uses BigInt.
Missing/null native size is refused, rather than silently observed as zero.
Zero-size records remain supported. Unsupported algorithms/invalid numerical
evidence are probed only at their explicitly identified pure/protocol boundary.
An incompatible digest does not suppress a valid exact-size difference.

R/reviewer-expectations.json contains an independent native table, not output
computed by the comparer: six paired paths give agreement 1, difference 3,
inconclusive 2; distinct `first/same` and `second/same` paths add one one-sided
row each despite equal content. Each side has seven rows; union eight. Native
IDs deliberately differ between paired sides. Fresh server checks in both
launch/reference directions verify each expected class and all four invariants:
paired sum, each side equals paired plus its own one-sided count, and union
equals paired plus both one-sided counts. Every union key is unique.

The candidate's separately executed 20-key fixture adds exact large values,
case/Unicode, CR/LF/CRLF, literal backslash-n, hostile text and longer paths.
Browser detail independently asserts IDs 9007199254740993/9007199254740994 and
sizes 9007199254740992/9007199254740993, including reversed attribution. These
are synthetic supported records, not huge-file or legal-Windows-name observations.
Filtered visible count is explicitly separate from full union/side/class totals.

### C. Completeness, bounds, failure and stale work — supported

The native enumeration branch in `gui_catalog.go:327` walks all adopted Files,
including retired collections; it does not reuse Search or a current filter.
`catalog-adapter.mjs` validates known unique IDs, exact count, artifact digest
and complete marker against the adopted set. Equal cardinality plus known unique
membership proves enumeration of that stored set. Both required readers must
succeed. Completeness does not claim historical coverage of all source files.

Existing bounds remain: 4 MiB native input; 100 rows per ancillary table; bounded
1000-row slices/4096-byte text; aggregate 1000 files/1000 copy occurrences;
16 MiB per reader frame; 32 MiB adopted pair; five-second reader response deadline.
Comparison adds 512-character request URLs and an 8 MiB serialized response cap.
Joining/sorting and response construction are bounded by adopted inputs. The
8 MiB check occurs after serialization of the already bounded result; it is not
a streaming allocation cap. One aggregate native request runs at a time; busy
requests are refused rather than queued. UI cancellation invalidates use of
responses; finite native work completes or times out rather than being instantly
aborted by a filter change. No broader throughput/scale qualification follows.

Reviewer positive boundary control accepts two 500-record native inputs and
returns all 500 paired differences; two 501-record inputs refuse adoption.
Existing tests freshly exercise 8 MiB refusal through a labeled protocol double
whose startup projections fit reader/pair bounds. This does not claim an
oversized native catalog passed the native 4 MiB gate.

Server selectors allow only the fixed handles and request identity, with no
browser-supplied input/adapter path. Invalid selectors return 400; unsupported
methods 405; comparison-frame/response-cap refusal 422; reader/enumeration
unavailability 503. Faults do not become successful-empty/one-sided results.
Partial, truncated, unknown-ID, exit and timeout controls passed; reviewer-added
first-reader partial/exit/timeout controls complement the second-reader cases.
The timeout was observed at about 5025ms, followed by waited termination. A
separate clean restart succeeds. Native malformed input refuses startup.

`comparison-ui.mjs` binds results to reference/counterpart/request, verifies the
whole deterministic result against adopted evidence, and looks up each side by
its own exact ID. Epoch and connected-panel checks discard old successes/errors.
Fresh external reviewer browser probes hold actual server responses, then change
filter, reference, navigation or selection via a newer completed result. Both
late success and injected late error are tested for each transition. No old
callback overwrites newer evidence or clears its selection. A current simulated
503 clears previous successful rows/details. Mismatched result IDs are refused.

### D. Scope, time and wording — supported

Shared strict scope-key validation remains unchanged and freshly exercised by
the affected native/Node controls. OFF/ON/UNKNOWN, explicit zero/positive exclusions,
valid empty/all-excluded inputs, and missing/known offset timestamps stay attached
to their own source. Scope differences qualify coverage without blocking valid
paired content evidence. Inventory observation time is projected from validated
Audit evidence; first-seen/load/current time is not substituted for missing history.

The regular-file `.DS_Store` one-sided row against exact exclusion policy is
qualified as outside that policy, without inventing excluded paths, unwanted
copies or physical deletion. Comparison operates on native file rows, not directory
entries; exact slash basename matching does not include near names. The scope
detail screenshot shows the absent counterpart without an invented ID and its
own recorded timestamp/policy. Unknown remains unknown.

Rendered notice, summary, row labels and detail consistently identify recorded
evidence. Empty/all-excluded display zero keys, not 100% verification. No Fix,
Sync, Register, Delete, Restore, live Verify or newest-wins action is enabled.
Artifact hashes and A/B names are not independent-copy claims. This functional
view requires no additional Figma frame or final-theme approval for bounded review.

### E. Two-sided identity and compatibility — supported

Same/long/special basenames remain visibly qualified by Snapshot A/B and bound
to session handles. Matched IDs may overlap or differ; inspector values match
independently known input evidence. Missing counterparts have no fabricated
record. Reversed direction and launch, explicit reference selection, class/literal/
exact-name filters, no-results clearing, keyboard/skip focus, Back/Forward and
accessibility names passed. Source paths/text-shaped markup remain inert text.

Directly affected static, single-catalog, native two-snapshot A/B/All, large-ID
and control-name scenarios passed. The existing top-level Snapshot view selector
does not select a comparison reference; comparison still requires its own explicit
action. No persistent registration/history or durable identity was introduced.

### F. Read-only boundary and lifecycle — supported within stated limits

Source inspection traces native catalog adoption to read-only `os.Open`, bounded
decoding and a read-only Store, and enumeration to in-memory records. Comparison
logic has no filesystem import or operational inventory/repair/registration call.
Recorded roots/paths are text only. Static allowlisted assets, loopback binding,
Host checking and finite request selectors remain intact. Tests exercise arbitrary
selector/method rejection; no private-path probe was used.

Fresh R-owned sentinel files were exclusively locked before native startup;
both catalogs were exclusively locked after adoption. An actual comparison
succeeded while locks remained held, and the catalog-directory watcher observed
zero mutation events. `read-only-stop-result.json`, `locked-comparison.json` and
`locked-reader-exits.json` retain evidence. This supplements source tracing and
hashes; it is not host-wide syscall tracing, ACL/race/power-loss qualification.
Producer setup reads new authorized source fixtures separately from comparison.

Decisive lifecycle evidence in R/split-stop.json: server PID 5664 and readers
11884/9084 all exited 0; `st` followed by `op\n`, stdin held open, listener closed,
forced=false. Every browser/server/reader harness awaits owned processes and
retains exit results. Browser force-stop/wait is harness teardown, not a natural
stop claim. Deliberate reader faults include exit 7 and SIGTERM for malformed
responses/timeouts; these are successful refusal tests, not normal-reader exits.
No owner process or workspace was attached to, inspected or stopped.

## Fresh reviewer execution

The author-reported 28 native top-level/242 subtests, 38 Node tests and 14 browser
sessions remain historical author evidence. The following are new reviewer runs;
overlapping selections and assertion groups are not added into unique test totals.

Environment: installed `C:\Program Files\nodejs\node.exe`, **v24.19.0**;
Chrome `C:\Program Files\Google\Chrome\Application\chrome.exe`,
**153.0.8010.48**; explicit retained Go **1.26.8 windows/amd64** at
`C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin\go.exe`.
Go selection uses GOTOOLCHAIN=local, GOPROXY/GOSUMDB=off, CGO_ENABLED=0,
GOWORK=off, retained GOMODCACHE and fresh R cache/temp; no GOTMPDIR override,
installation, download or global setting change.

Fresh reader build from the verified checkout:
`go build -mod=readonly -o R/output/reader.exe .`, exit 0.
SHA-256: `c08dd1d0b497224754cee68f4cb0b5faf1e7e1ce5b0d6e03f3b77aa00ecd8653`.
It happens to match the author's binary hash; it was freshly built, not copied.
`run-native.ps1`, `native-exits.json` and build logs retain the execution.
Explicit protocol doubles were also freshly built; identities are separate in
`binary-identities.json`. No native source was added or mutated for this review.

| Execution | Fresh outcome and records |
| --- | --- |
| Native selection | `go test -mod=readonly -list ^TestGUI .`, then `go test -mod=readonly -count=1 -json -run ^TestGUI -timeout 4m .`: 28 top-level pass, 242 subtests pass, zero fail/skip. `native-selection.txt`, `native-tests.jsonl`. |
| Build/vet/format | Reader and doubles build exit 0; `go vet -mod=readonly .` exit 0; read-only `gofmt -l` on changed Go files empty; `git diff --check` exit 0. The unrelated archive suite was not run. |
| Node | Enumerated six selected test files before execution: stop-command, preview, catalog, catalog-correction, multi-snapshot, recorded-comparison. `node --test` passes 38 tests, zero suites/fail/cancel/skip/todo. `run-node.ps1`, `node-selection.txt`, `node-tests.log`. |
| Independent probes | `node R/reviewer-probes.mjs R`: 16 probe groups pass, separately counted from Node tests. `reviewer-expectations.json`, per-direction results and `reviewer-probes-results.json` retain cases/refusals/cleanup. |
| First-reader faults | `node R/first-reader-faults.mjs R`: three fault controls and one clean restart pass. `first-reader-faults.json` retains responses, elapsed times, PIDs/exits/listener closure. |
| Browser | 15 successful sessions, detailed below; no failed sessions/retries. Real browser/native reader controls plus explicitly labeled protocol/client fault seams. |
| Read-only/stop | `run-read-only-stop.ps1`: exit 0, zero watcher events; native comparison under exclusive locks and stdin-open split stop pass. |

Browser session/check-entry counts: forward 22, reverse 22, long labels 22,
reopen 22, scope 4, empty 4, refused 3, reader-failure 3, existing native 17,
large 16, controls 9, single 22, static 55, reviewer-late 13, reviewer-scope 3.
These are named harness checks, not an exhaustive count of internal assertions.
Exact arguments and exits are in `run-browsers.ps1`, `browser-exits.json`,
`reviewer-browser-exit.json`, `reviewer-scope-exit.json`, with each session's
`browser-results.json` and `catalog-process.json`. No runtime test or browser
session failed or required retry. Expected refusals/timeouts remain in logs.
The final evidence-summary PowerShell helper stalled after writing binary/input
identities and was interrupted (exit 1). Its original script is retained; a v2
uses plain version text and shallower JSON serialization. Preservation was then
rechecked with that helper; no runtime tests were replayed. An attempted read of
the not-yet-written final manifest also returned file-not-found. These are closing
evidence-helper/diagnostic failures, not candidate failures or passing tests.

`setup-comparison.mjs` generates fresh native fixture projections and two new
producer observations; `producer-0.json`/`producer-1.json` preserve exact arguments.
Controlled native cases, protocol simulations, pure cases and client delay/error
seams are distinguished above. Reviewer-only harnesses live in R and import the
unchanged candidate runtime. Copied author harness scripts were reused as scripts,
not as earlier passing results. No author catalog or output was overwritten.

## Screenshots and preservation

Comparison screenshots use 1440x1024 CSS pixels, scale 2; only the static control
also exercises 390x844. Shared harness metadata lists both sizes; that does not
mean every comparison screenshot was captured at both. Representative artifacts:

- `browser-compare-forward/comparison-results.png` and `comparison-large-detail.png`.
- `browser-reviewer-late/reviewer-swapped-reference-detail.png` and `reviewer-swapped-summary.png`.
- `browser-reviewer-scope/reviewer-scope-qualified-detail.png`.
- `browser-compare-empty/compare-empty.png`.
- `browser-compare-refused/comparison-refused.png`.
- `browser-compare-failure/comparison-reader-failed.png` and `browser-reviewer-late/reviewer-current-failure.png`.

Swapped exact evidence, scope summary/detail, empty/all-excluded and duplicate
refusal screenshots were visually inspected. Long labels wrap with their A/B
qualifiers. Source text, scope and unknown time remain legible and attributed.
No additional visual-fidelity, responsive-theme or Figma-frame claim is made.

`final-preservation.json` rechecks all 320 candidate entries and retained source
bytes, designs, author input identities, final HEAD/index/operation state and
package hash. `execution-summary.json`, `binary-identities.json` and
`reviewer-input-identities.json` retain execution/build/probe identities; further
generated fixture hashes are in `input-identities.json`. Historical reports,
candidate source/tests and living records remain unchanged. Only this new report
is added to the repository; raw evidence stays under R. Source-input before/after
assertions and the watcher/lock probe supplement final hashes. Owner workspaces
are preserved by exclusion, not a broad scan of their contents.

Frozen ZIP remains
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225\build-complete\0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`,
SHA-256 `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`.
It contains neither multi-snapshot nor comparison support. No package refresh,
public binary distribution, source publication, staging, commit, push, merge,
security-setting change/detection or media operation occurred.

Remaining boundaries: supported single-root generated snapshots, exact supported
path representation and SHA-256 content evidence; no live verification, durable
storage identity, physical-copy proof, arbitrary root mapping, schema migration,
registry/history or production recovery. Windows synthetic execution does not
qualify Linux/macOS native behavior, ACL/races, concurrency/power loss, scale,
hardware/media or a second machine. Prompt20 remains **DEFERRED_BY_OWNER**.
Generation-independent preservation/buffering, LTO-8 as first physical target
rather than generation limit, explicit other-backend qualification and separate
Blu-ray workflow remain planning directions.

**ONE next action:** separately authorized owner acceptance and scoped source
publication of this reviewed candidate. Stop here; no automatic follow-on work.
