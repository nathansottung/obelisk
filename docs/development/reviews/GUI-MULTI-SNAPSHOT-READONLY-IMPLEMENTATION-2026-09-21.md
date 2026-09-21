# Two generated snapshots in one read-only session — 2026-09-21

GUI_MULTI_SNAPSHOT_READY_FOR_REVIEW

Author implementation and actual Windows execution, not independent reviewer
acceptance, owner milestone acceptance, publication or production qualification.
The owner submitted Prompt22 to select this milestone; its number is a conversation
label, not a repository issue ID. No subagent or independent context was used.

## Identity, authority and source decision

- Repository: `C:\Users\nsott\source\repos\obelisk`
- Parent and unchanged HEAD: `a099ddc7530d81a9f3206e426b81def5172b16ec`
- Starting branch: `feat/windows-inventory-alpha-package`
- New branch: `feat/gui-multi-snapshot-readonly`, created at that exact parent.
- Accepted packaging implementation: `082ae8375130bdb9943d31d7432c87a3c53fbbb8`.
- Evidence directory **E**: `C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-20260921-130125`.
- Submitted instruction: `E\submitted-prompt.txt`; chosen pre-implementation
  contract: `E\implementation-contract.txt`; starting checkpoint and raw hashes:
  `initial-context.json`, `initial-identities.json`.
- Final exact working source and new-file identities: `candidate-identities.json`;
  delta/preservation: `final-preservation.json`; separate binaries/ZIP/planning
  records: `artifact-identities.json`. These identify uncommitted working bytes,
  not a new implementation commit.

Local checkpoint matched the expected parent, index empty and no active operation;
only `Untitled.pdf` and `docs/Obelisk.fig` were untracked. Applicable instruction
search found no AGENTS.md. The existing scope report/matrix/proposed updates under
`scope-reconciliation-20260921-113337` were reused, not rediscovered or changed.
Relevant report-local rows: SR-01..08 (bounded baseline/selection), SR-10 (namespace
and federated search), SR-13 (limits), SR-15 (scope/age), SR-27 (GUI), SR-30 (deferral).
Only option A's transient selection was implemented. Persistent recents and options
B/C remain proposals. No extant standalone feature matrix was located; existing
handoff/status/next-actions/coverage records are updated without creating a master.

Source publication evidence from the existing receipt/resume record was reused;
no remote lookup or publication helper was replayed. Prompt20 remains
**DEFERRED_BY_OWNER**; second/clean-machine qualification is **PENDING**.

## Implementation and exact contract

The development launcher is `scripts/gui-preview/server.mjs`, not the frozen
package launcher. Syntax adds a second `--catalog` before the existing `--adapter`:

```powershell
$multi = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-multi-snapshot-20260921-130125'
node scripts/gui-preview/server.mjs 0 --catalog "$multi\multi-inputs-v3\a.json" --catalog "$multi\multi-inputs-v3\b.json" --adapter "$multi\output\reader.exe"
# Send stop followed by Enter; keep stdin open until the complete command acts.
# Wait for both reader exit records and the closed listener.
```

Static (`node scripts/gui-preview/server.mjs 0`) and one-catalog syntax remain
valid. A third input or malformed syntax refuses. Both inputs are fixed for the
process lifetime and must meet the existing selected-path/native validation rules.
No browser path/executable selection, upload, dynamic picker, auto-discovery,
registry, local-storage history, source rescan or schema migration was introduced.

`catalog-adapter.mjs:startCatalogSession` composes two existing native readers.
Startup fully validates both before exposing a successful pair. Duplicate byte
digests refuse, including the same filename twice and separately named copies.
Different digests preserve their records even when IDs, names and content overlap.
The wrapper is not an OS sandbox or proof that arbitrary selected inputs are
synthetic; the generated/quiescent input provenance remains an authorization limit.

Each startup projection receives a random 128-bit process-local handle bound to its
actual validated catalog/digest. The server checks `snapshot=all` or a known handle
against the fixed allowlist. Unknown, duplicated or absent handle parameters fail;
there is no first-input fallback. Display labels/basenames are inert text, not
authority. Digest labels identify catalog bytes, not physical copies or a durable
medium identity. Handles may change after restart.

Query results are grouped as `{snapshot, ids}`; browser-selected identities are
objects with separate `{snapshot, id}` fields. Exact native decimal strings are
preserved throughout. Selection resolves only inside that handle's already
validated projection, using IDs returned by the corresponding real native query;
there is no extra filesystem/record-open endpoint. Validation rejects unknown IDs,
numeric coercion, duplicate groups/IDs, missing groups and cross-filter responses.
`catalog-protocol.mjs:validateSnapshots/validateMatches` enforce the same browser
and server contract. No ambiguous delimiter or ID renumbering is used.

Library lists both snapshots with individual identity/provenance and selection
buttons. Find uses a named semantic select for All/A/B. Every result and inspector
identifies its snapshot; switching filters clears selection. Query text edits
invalidate old requests immediately, including the debounce interval; navigation
and responses use epochs to prevent stale UI replacement. Existing exact-name
parsing preserves LF, CR, CRLF, literal escape-looking text, whitespace and Unicode.
All rendering uses text/attributes, not catalog HTML execution. Source paths stay
text and are never followed by the viewer.

The only Go implementation change is `gui_catalog.go:projection`: optional
read-only `recordedAt` from the already validated `Audit[0].At`. This was the
source-level dependency missing from the old response. It neither modifies the
native catalog nor broadens accepted audit sections. Timestamp offset survives
projection. Supported historical input without an inventory event shows unknown;
file first-seen timestamps are not promoted to whole-inventory times. Session
load time is explicitly separate and cannot refresh recorded evidence age.

Scope stays per input: OFF, ON/counts, known zero, genuine empty, all-excluded and
UNKNOWN remain distinct. Today's exclusion option is not applied as a display
filter. All totals count returned recorded entries, not unique bytes, independent
copies, complete source coverage or combined exclusion counts. Current availability,
capacity, parity and verification remain unavailable. No comparison engine exists
in this slice. The native search limit is 1000 and the loaded record cap is 1000,
so no supported query silently truncates; retired collections retain existing rules.

Per-input caps remain: 4MiB native input, 1000 files, 100 ancillary rows per table,
1000 potential occurrences, 4096-byte strings and accepted query/encoding bounds.
Aggregate caps: 1000 records and 1000 projected copy occurrences, at most two
readers, one outstanding aggregate query, 16MiB per reader response and 32MiB
serialized session projection. Concurrent work is refused rather than queued.
Existing 5s active-response deadlines and 500ms close fallback remain. Native
query results are finite ID lists. Startup only reads the selected catalog files,
necessary program assets and path-validation metadata, not recorded source paths.

All success requires successful valid results for every requested input. A reader
failure clears/latches the session/view and closes owned readers; invalid startup
shows an explicit pair refusal, never a complete one-input or static fallback.
Changing route cannot revive stale successful data. Server loopback/Host/method/
asset allowlists, 60-minute automatic stop and split-line stop parser remain.
Normal close waits native readers; fallback kills are identified as such in logs.

## Changed and new paths

Runtime changes: `gui_catalog.go`; `scripts/gui-preview/catalog-adapter.mjs`,
`catalog-protocol.mjs`, `catalog-ui.mjs`, `server.mjs`.

Validation changes: `scripts/gui-preview/browser-check.mjs`; new
`gui_multi_snapshot_test.go`, `scripts/gui-preview/multi-snapshot.test.mjs`,
`scripts/gui-preview/multi-snapshot-browser-checks.mjs`.

Documentation: preview README; CODEX_HANDOFF, NEXT_ACTIONS, OB_STATUS,
REVIEW_COVERAGE.csv; this distinct implementation report. No production operation,
package allowlist/launcher, native persistence format, dependency, framework or
historical report was edited. New runtime composition stays in existing packaged
module paths; the accepted package bytes remain frozen regardless of source changes.

## Actual fixture and execution evidence

Selected toolchain was explicitly the retained Go1.26.8 Windows amd64 executable,
confirmed in `toolchain.txt`, not PATH Go1.27. Offline process settings:

```powershell
$g = 'C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64\bin'
$env:GOTOOLCHAIN='local'; $env:GOPROXY='off'; $env:GOSUMDB='off'
$env:CGO_ENABLED='0'; $env:GOWORK='off'
$env:GOMODCACHE='C:\Users\nsott\AppData\Local\ObeliskDev\audit-2026-09-19\go-mod'
$env:GOCACHE="$multi\go-cache"; $env:TEMP="$multi\tmp"; $env:TMP=$env:TEMP
& "$g\go.exe" build -mod=readonly -o "$multi\output\reader.exe" .
& "$g\go.exe" build -mod=readonly -o "$multi\output\double.exe" scripts/gui-preview/testdata/reader-double.go
& "$g\go.exe" test -mod=readonly -list '^TestGUI' .
& "$g\go.exe" vet -mod=readonly .
& "$g\gofmt.exe" -l gui_catalog.go gui_multi_snapshot_test.go
```

Both builds and vet exited0; format and final whitespace checks were clean.
The new reader SHA-256 is
`e8562bda92252ffb774edf079765cf4c0dda860ed0f516feafb6633dac0d2bf0`.
The labeled protocol fault emitter is
`a482c1cc39ffb178fbbcc5c20e20d18d4d3d7aa4e73398ee9f72764d4f597aa2`;
it is not a real catalog reader or production executable.

Native fixture generation uses existing `guiFixture`, `guiExactFixture`,
`scopeKeyCases` and `produceGUIInventory`, through explicit setup tests. The new
setup creates new synthetic source directories disjoint from output directories;
no owner data is enumerated. The actual commands were:

1. `go test -mod=readonly -count=1 -json -run '^TestGUI' -timeout 4m .` with
   `OBELISK_GUI_FIXTURE_OUTPUT=E\catalog-inputs`,
   `OBELISK_GUI_CORRECTION_FIXTURES=E\correction-inputs`,
   `OBELISK_GUI_MULTI_FIXTURES=E\multi-inputs`. Retained as `native-tests.jsonl`:
   25 top-level passes / 242 subtest passes, one new fixture-setup failure,
   zero skips. Output-parent overlap was correctly refused by the native producer.
2. After correcting that setup, `go test -mod=readonly -count=1 -json -run
   '^TestGUIMultiSnapshot' -timeout 2m .` with new `E\multi-inputs-v2`:
   recorded-time test passed; fixture setup failed because a borrowed case label
   contained a path separator. Retained as `native-multi-corrected.jsonl`.
3. After assigning bounded numeric fixture filenames, the same focused command
   with new `E\multi-inputs-v3` passed both top-level tests, zero failures/skips;
   `native-multi-final.jsonl`. These are the retained browser/Node inputs.
4. Final full GUI selection, same command as step1 with new
   `final-native-catalogs`, `final-native-correction`, `final-native-multi` output
   variables: **26 top-level passes / 242 subtest passes / zero failures/skips**,
   `native-final-all.jsonl`. These overlapping runs are not added together.

`a.json` and `b.json` are unaltered finite native producer outputs (OFF/ON), with
overlapping IDs, equal-content records and differing `shared.txt` evidence.
`b-aged.json` is an explicitly derived native-format fixture with matching existing
inventory-event/collection time `2024-02-03T04:05:06-05:00`. `unknown.json` removes
audit only in a separately labeled fixture to model historical absence, never to
repair compatibility. `zero`, `empty`, `excluded` are native producer outputs.
Large-ID and LF/CR/CRLF/literal-escape catalogs are supported native-format fixtures,
not claims those names were created as Windows filenames. Their exact integers
originate in Go, not rounded JavaScript numeric constants. Native malformed-scope
fixtures preserve raw duplicate/case-variant members. `limit-a/b` are valid
individual catalogs exceeding the combined cap. `multi-faults` contains copies
of fresh protocol fixtures with a deliberately distinct emitted digest; only the
fault-emitter tests use them. This is simulation, not altered production evidence.

Node selection (actual Node24 tooling already available; no installs):

```powershell
$env:OBELISK_GUI_TEST_INPUTS="$multi\catalog-inputs"
$env:OBELISK_GUI_CORRECTION_INPUTS="$multi\correction-inputs"
$env:OBELISK_GUI_TEST_ADAPTER="$multi\output\reader.exe"
$env:OBELISK_GUI_TEST_DOUBLE="$multi\output\double.exe"
$env:OBELISK_GUI_MULTI_FIXTURES="$multi\multi-inputs-v3"
$env:OBELISK_GUI_MULTI_FAULTS="$multi\multi-faults"
node --test scripts/gui-preview/stop-command.test.mjs scripts/gui-preview/preview.test.mjs scripts/gui-preview/catalog.test.mjs scripts/gui-preview/catalog-correction.test.mjs scripts/gui-preview/multi-snapshot.test.mjs
node --test scripts/gui-preview/multi-snapshot.test.mjs
```

Initial selected **31/31 pass**, zero failure/skip (`node-tests.log`): stop5,
preview7, catalog4, catalog-correction8, multi7. Final multi **7/7 pass**,
zero failure/skip (`node-multi-final.log`) after explicit duplicate/limit error
wording. Not additive. Coverage includes native pair/reversal/reopen, exact IDs
and names, scope/age combinations, duplicate artifacts, invalid second/missing/
unsupported/malformed scope inputs, real aggregate-limit refusal, handle/record
validation, faulted All responses, owned shutdown and no input-directory changes.

The pair CLI test sends `st`, verifies listener still responds, then `op\n`, with
stdin kept open until exit. Both real readers exit0, server exits0, listener
closure is verified separately; full argv/PID/stop output is in test diagnostics.
Existing delayed fault tests explicitly distinguish forced termination from native
EOF success. Existing static CLI smoke uses a forced validation stop; this does
not qualify interactive console Ctrl+C or second-machine execution.

## Real browser evidence and visual boundary

Installed Chrome reported `Chrome/153.0.8010.48` through CDP. Harness command:

```powershell
node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' "$multi\browser-native-pair" "$multi\multi-inputs-v3\a.json" "$multi\output\reader.exe" multi-native "$multi\multi-inputs-v3\b.json"
```

This is the actually executed unaltered native pair. The same harness executed
the following pairings in fresh evidence/profile directories. Never rerun into
an existing evidence directory; the harness refuses reuse.

| Evidence directory under E | Inputs / expected argument | Actual checks |
|---|---|---:|
| browser-native-pair | a / b; multi-native | 17 |
| browser-pair-final | a / b-aged; multi-core | 17 |
| browser-large-final | large-a / large-b; multi-large | 16 |
| browser-scopes-final | unknown / zero; multi-scopes | 4 |
| browser-empty-final | empty / excluded; multi-empty | 4 |
| browser-controls-final | controls / unknown; multi-controls | 9 |
| browser-reverse-final | b-aged / a; multi-core | 17 |
| browser-reopen-final | a / b-aged; multi-core | 17 |
| browser-duplicate-final | a / a-copy; multi-refused | 2 |
| browser-invalid | a / scope-4; multi-refused | 2 |
| browser-runtime-failure | correction-inputs/late / multi-faults/query-failed; multi-failure, double.exe | 4 |
| browser-single-final | catalog-inputs/alpha; ALPHA, no second path | 22 |
| browser-static-final | no catalog/adapter/expected arguments | 55 |

All completed successfully. `browser-session-summary.json` retains all **21**
completed browser sessions, including earlier presentation passes; they are not
21 independent reviews and their assertion counts are not added into a coverage
total. Earlier `browser-pair-1`, large/scopes/empty/controls/duplicate/invalid/
reverse/reopen results remain, not relabeled as final UI executions. `browser-invalid`
is reused refusal-path evidence; subsequent changes affected success cards and
more specific error wording, not its refusal behavior.

The browser validated actual A/B/All inspectors, adjacent large IDs and int64
maximum, source-specific bytes, exact controls, accessible selection, Tab/Shift+Tab,
skip-link focus without reset, Back/Forward, disclosure modes, source details,
absent-record clearing, inert labels and runtime failure latching. A labeled client
fetch-delivery seam delays a completed real native response; switching filters
then releasing it cannot overwrite the current inspector. No fake arrays replace
the normal native workflow. Page requests and runtime errors are retained; no
external/API page requests or runtime/console errors were observed.

Each directory includes `browser-results.json`, `catalog-process.json` and PNGs.
Key screenshots: `pair-library.png`, `shared-inspector-1.png`,
`shared-inspector-2.png`, `pair-find-final.png` in browser-native-pair/pair-final;
`large-inspector-1/2.png`, `literal-name-inspector.png`, `empty-and-excluded.png`,
`runtime-reader-failed.png` in their matching directories. Actual pair screenshots
were inspected at 1440x1024 CSS / scale2. Expanded Find provenance initially pushed
results below the viewport; final semantic disclosures keep key scope/time visible.
This preserves the shell direction, not a claim of new Figma fidelity. No Figma
decode/export/image work or unrelated redesign occurred. Harness viewport metadata
also lists its existing narrow mode, but pair runs executed only the desktop size;
the static control separately exercised 390x844. Do not infer pair narrow-screen
qualification from the shared metadata array.

Browser processes are deliberately terminated and waited by the harness after
checks, as recorded; servers close and wait their readers. Normal native cases
close gracefully; protocol fault cases retain the actual exit/signal. No owner
process was enumerated, stopped, reused or attached. Ports were assigned on new
loopback listeners and profiles were new task-owned directories.

## Preservation, exclusions and next action

Before/after native/Node controls compare selected input bytes and directory
entries. Final input/source manifests identify all generated evidence inputs.
The final repository manifest/delta verifies unrelated source and every historical
report/design input against starting raw hashes. Package extracts and owner
workspaces were never opened or modified. The original ZIP was narrowly rehashed
and still equals `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`;
no rebuild, repackage, rename, signing or package manifest edit occurred. Original
source-manifest identity remains `60ea696a91f4c467b543b381a93e6294813ed5030e557386722e55b1de6713a0`.

HEAD is unchanged, index empty, no commit/push/merge/tag/upload. New repository
paths are this authorized uncommitted candidate only; design files stay untracked.
Existing config/job, GUI, catalog, filename/search and scope-key closures remain
closed within their original scopes. This author run does not re-accept them.

Explicitly excluded: persistent registry/history, native migration, operational
comparison, current source verification, production inventory/recovery, scale,
ACL/power-loss/race/hostile storage guarantees, hardware/media and second-machine
qualification, public package/distribution and framework migration. Original
security alert classification remains unestablished; no new detection was reported
by these executions and no security settings changed. PR-04 and external-tar
Unicode remain separate. Generation-independent preservation/ring-buffer intent,
LTO-8 as first physical target rather than generation limit, other-backend
qualification and separate Blu-ray planning remain untouched.

ONE next action: substantive review of this exact uncommitted candidate and
retained author evidence. No automatic implementation follow-on, publication,
second-machine request or replacement of the frozen dogfood artifact.
