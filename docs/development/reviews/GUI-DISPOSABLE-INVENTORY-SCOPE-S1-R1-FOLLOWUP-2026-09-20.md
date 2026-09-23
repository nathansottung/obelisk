# Disposable inventory S1-R1 scope-key correction - 2026-09-20

**READY_FOR_GUI_INVENTORY_SCOPE_FOCUSED_RECHECK**

| Item | Disposition |
|---|---|
| S1-R1/P2 | **AUTHOR_ADDRESSED** |
| F1/P2 | **PREVIOUSLY_CLOSED** |
| F2/P2 | **PREVIOUSLY_CLOSED** |
| S1 overall | **AWAITING_REVIEWER_CLOSURE** |

This is one bounded implementation correction authorized by submitted Prompt 15C, not reviewer closure, owner acceptance or publication. The [controlling focused review](GUI-DISPOSABLE-INVENTORY-FOCUSED-RECHECK-2026-09-20.md) retains its NEEDS_CHANGES verdict and F1/F2 closures unchanged. S1 originally represented missing requested functionality; S1-R1 is the later reproduced strict-scope validation defect. No broad F1/F2 review or base inventory reimplementation occurred.

## Checkpoint, authorization and preserved bytes

Branch remains `feat/gui-disposable-inventory`; full HEAD and overall inventory-patch parent remain **`5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`**. Index empty, no active Git-operation markers or branch change. No competing implementation process was observed at setup. No staging, commit, push, merge, reset, stash/clean, remote lookup or production access.

New task evidence root **E**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-scope-fix-20260920-194852`.

Controlling reviewer evidence **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-focused-20260920-191824`.

The [original implementation](GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md), [substantive review](GUI-DISPOSABLE-INVENTORY-REVIEW-2026-09-20.md), [first correction follow-up](GUI-DISPOSABLE-INVENTORY-REVIEW-FOLLOWUP-2026-09-20.md), controlling focused report, applicable repository/schema guidance, living-record entries, README and actual source/callers were consulted. Previously read instructions/provenance were reused; no applicable AGENTS.md or later completed S1-R1 correction was found. Historical prompts were not replayed as tasks.

E/submitted-prompt.txt preserves attachment `45d5e5c1-e263-4d97-836b-23e72b02a585/pasted-text.txt`. E/setup.mjs verified the review's **286** recorded candidate identities plus the separately hashed focused report, yielding **287 pre-edit entries**, and all **700** retained reviewer evidence artifacts. The focused report SHA-256 remains `f7128af80b4c329733dc942bba86f897f0f78b52c34c03176a81a81f1dceb24a`. The preceding repeated Prompt 15B turn was an identity-only reuse, with no tests or changes; it is not a correction or additional review campaign.

E/before.json, starting-state.json, before.patch and baseline-verification.json preserve exact identities, tracked/untracked status, full tracked diff against the parent and reviewer-manifest identity. E/before-source retains buildable pre-edit working bytes including untracked tests/source; `.git`, private/production state and unnecessary Figma/PDF/ZIP/PNG binaries were not copied. This is the actual reviewed uncommitted implementation, not a checkout of HEAD. Original design binaries remain at their original paths and are covered by preservation identities.

## Root cause and correction

The old detail validator counted eight distinct raw key spellings, then decoded into a Go struct with case-insensitive field matching. An alias could occupy the eighth slot while a required count was missing; zero-value initialization supplied that count. Canonical-plus-alias collisions were resolved by member order. The reader projected that already-normalized scope; the protocol/browser could no longer detect the missing input and displayed complete/empty reassurance.

The same irreversible interpretation boundary also existed around the native `audit` wrapper and its event members: native struct decoding could fold case variants or discard duplicate wrappers before scope inspection. Correcting only the final detail key count or changing the displayed label would leave those paths ambiguous.

[gui_catalog.go](../../../gui_catalog.go) now calls `validateGUIInventoryScopeEncoding` after F1's original UTF-8/JSON-escape validation and **before** the shared native decoder. [gui_inventory_scope.go](../../../gui_inventory_scope.go) provides one exact decoded-member policy:

| Affected path | Accepted decoded member names |
|---|---|
| Native catalog scope history wrapper | `audit`, at most once |
| Single supported audit event | `at`, `action`, `detail`, each exactly once |
| Versioned JSON detail | `version`, `policy`, `entries`, `regularFiles`, `includedFiles`, `excludedFiles`, `readBytes`, `complete`, each exactly once |

`guiScopeMembers` reads JSON tokens and retains each value as RawMessage only after its decoded key is known to be canonical, unique and non-null. Valid escaped canonical keys such as `"\u0061udit"` and `"\u0065ntries"` are accepted. Encoded duplicates resolve to the same decoded name and refuse. Unsupported case variants refuse rather than enter an alias map. Unknown event/detail members also refuse. Matching is confined to the audit path; no filename, source path, search input, native ID or unrelated catalog key is normalized or case-folded.

The wrapper scanner checks only the relevant native audit member while preserving the existing policies for unrelated catalog members. It detects duplicate wrappers, including malformed/valid combinations in either order, before any dictionary or struct can discard one. Native event members receive the same canonical/required/duplicate/null checks. Existing native decoding and `inventoryScope` still enforce timestamp/action/types, detail size, version/policy/completion and all existing count/byte/relationship constraints. `inventoryScope` also uses the exact detail-member helper for typed in-memory validation. No new count equation or schema version was introduced.

**Legacy is absence of recorded scope, not malformed attempted scope.** The established native no-history forms—missing `audit`, `audit:null`, and `audit:[]`—remain UNKNOWN. A null history slice is not a scope event with null required counts. A present event/detail with null/missing/wrong-type fields, unsupported spelling or duplicates fails. `Audit:[]`, duplicate wrappers ending in empty/null, and a malformed scope followed by a valid wrapper cannot silently downgrade to legacy or become last-wins success.

The producer's existing complete `loadGUICatalog` validation occurs before source rechecks/staging/no-replace publication and therefore reaches the same new raw boundary. Valid producer output already uses canonical names and passes unchanged. No second loader, sidecar, multi-file protocol, traversal/filter change, global JSON replacement, Store.Log/save/startup call or rewrite of rejected catalogs was added. Shared store.go/schema/production backend code remains unchanged.

## Exact-before reproduction and corrected refusals

E/red.ps1 built E/output/before-reader.exe from E/before-source. Its SHA-256 is **`28cc08f25f42d7e6f6025dfefa6ee95c0cea6f9244dd0f9db0add1c2508f3cc7`**. The original reviewer input files were copied without reserialization from R/verified-run/snapshots:

- `reviewer-missing-entries-case-alias.json`: missing `entries`, conflicting `Version:2` and `version:1`.
- `reviewer-missing-excluded-case-alias.json`: missing `excludedFiles`, conflicting `Complete:false` and `complete:true`.

Both exact files exit 0 in the fresh pre-edit reader and project complete scope with invented zeros. E/output/red-native.json retains original hashes, argv, stdout/stderr and binary identity. E/output/red-browser/scope-library.png visibly reproduces “0 entries visited”, “Completed within selected scope” and “Truly empty source.” Its three completed diagnostic checks establish the defect, not compliant validation. The browser uses the preserved pre-edit implementation. No mutation experiment or build failure is substituted for this reproduction.

The corrected fresh executable refuses both original files with exit 1, `ok:false`, no catalog/IDs/partial bootstrap, and unchanged input hashes. E/output/scope-native.json includes **23 data-only raw fixture cases: 19 refusals and four valid controls**. It covers all eight detail aliases, escaped canonical/case/duplicate keys, conflicting completion order, event aliases/encoded duplicates and late duplicate wrappers after nonempty valid records. Exact duplicate-bearing JSON is assembled as member text; an object serializer is used only for ordinary envelopes or quoting the detail string, never to deduplicate adversarial members.

[gui_inventory_scope_keys_test.go](../../../gui_inventory_scope_keys_test.go) adds `TestGUIInventoryScopeKeys` and `TestGUIInventoryScopeRefusalProtocol`. They cover every required detail field's missing/null/wrong-type/case/duplicate/same/conflicting/encoded forms, canonical escaped keys, event members and wrapper collisions, input immutability and the actual native reader protocol. Valid zero and historical no-history controls succeed; existing nonzero/range/consistency tests remain passing. Repeated ordinary values and words such as audit/detail/entries in record text are not treated as key violations.

[inventory-scope-keys.test.mjs](../../../scripts/gui-preview/inventory-scope-keys.test.mjs) exercises the actual native-reader/server bootstrap and queries using the external raw fixtures. Invalid inputs produce enabled-but-failed catalog mode, no catalog/IDs and failing query responses. Fresh valid -> invalid -> valid launches do not reuse previous scope or poison the next valid load. Input SHA-256 remains unchanged and listeners/readers are closed and waited.

Four corrected real-browser refusal sessions cover both original inputs, a late duplicate audit wrapper and an encoded duplicate. E/output/browser-refused-reviewer-missing-entries-case-alias/catalog-refused.png shows explicit refusal/relaunch, replacing the false empty-scope screen. No successful empty results, selected evidence, old counts or synthetic-demo fallback appear. The unchanged adapter may surface its reader-exited/transport failure message instead of the native scope-specific reason; raw native diagnostics retain the precise refusal.

## Valid producer/viewer controls and compatibility

E/fixtures.mjs defines a small independent contents/hash/path oracle before invoking the real producer on new task-generated sources. OFF and ON publish to distinct absent paths outside the source. The 28-file corpus yields 37 entries/28 regular files: OFF includes 28/excludes 0/reads 851 bytes; ON includes 26/excludes exactly two/reads 782 bytes. Root/nested exact regular `.DS_Store` entries alone are omitted in ON. Near/case names in separate directories, other dotfiles, AppleDouble-style metadata, .xmp/.aae and the child sentinel inside a `.DS_Store` directory remain. Empty, all-excluded and ON-zero have separate generated sources and durable scope.

Source/tree and snapshot hashes are checked before/after; no source contents or permissions are changed by production code. The unchanged filter classifies before exclusion, bypasses content-open/hash for excluded regular files, counts visits/exclusions toward bounds and rechecks observations. Existing enabled no-replace, late-collision, cancellation, classification and resource-bound controls pass. Independent fixture hashing is not described as producer I/O observation or proof of no transient writes; code inspection and the existing read-stage observer supply that distinction.

The existing native Audit facility and reviewed preview-reader compatibility boundary remain unchanged. New scoped output requires the corrected preview. E/output/prior-reader-compatibility.json freshly confirms that the identified older pre-correction uncommitted reader (SHA-256 `bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69`) refuses OFF/ON with `unsupported populated section: Audit`, while its legacy positive control succeeds. Its verified retained provenance remains in the prior reports; it is not mislabeled the published parent or rebuilt here. Corrected readers show legacy UNKNOWN with original `.DS_Store` records searchable. Provenance was not stripped from new output to satisfy the older reader.

All valid scope categories run in actual browser sessions. Exact foreign-name input, selected hash/size/native ID, disclosure/history/skip behavior, large IDs, static mode and query-failure protection remain passing directly affected controls. F1/F2 remain previously reviewer-closed, not newly accepted by this author run. Foreign recorded names remain data and their paths are never opened.

E/lifecycle.mjs reopens ON with its generated source temporarily unavailable and separately reopens foreign data. The bounded fixture rename is restored in finally. Scope comes from the snapshot, not current CLI/source/browser state. Six forbidden operation/helper/path routes refuse. The actual viewer receives split `st` / `op\n` while stdin stays open, exits 0 naturally, waits reader exit 0 and closes its listener. Node's existing stop tests cover whole/split/CRLF/character parsing and bounded owned-adapter cleanup. Forced browser and intentional fault-reader cleanup remain separately labeled.

## Executions, diagnostics and limits

Installed tooling: Go 1.26.8 windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. Existing local audit toolchain/module cache, GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0; task-owned GOCACHE/TEMP/TMP; GOTMPDIR unset. No installs, upgrades, global setting changes, WSL or remote CI.

| This correction's execution | Result |
|---|---|
| Exact-before native/browser | Two reproduced erroneous acceptances; one completed diagnostic browser session |
| Enumerated Windows native selection | **26 top-level / 242 subtest passes**, zero failures/skips |
| Fresh Windows producer/reader + fault-double build; vet | Passed |
| Scoped gofmt; final read-only formatting/whitespace | No final findings |
| Linux amd64 and Darwin arm64 cross-build/vet | Passed; no foreign native runtime execution |
| Existing Node stop-command / preview / catalog / catalog-correction / inventory-correction | **5 / 7 / 4 / 8 / 4** passes |
| New scope-key Node group | Initial two-test failure on message expectation; two separately retained targeted runs of **two passing tests** after assertion correction |
| Raw fixture native protocol | 23 cases: 19 explicit refusals, four valid controls |
| Corrected browser campaign | **17 completed sessions**, zero failed sessions |

The Go selection is recorded in E/native.ps1 and output/native-selected.txt: `^Test(GUIEncoding|GUIExact|GUIInventory|HashReaderBothRefusesPartialError|HashFileBothMatchesSHA256|ScanRecordsBlake3|ScanFolder_|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))`, executed once with `-timeout 120s -count=1 -json`. New tests contribute within that selection; top-level and subtest counts are not combined. Only gui_catalog.go, gui_inventory_scope.go and the new Go test were formatted. Go implementation bytes did not change after the corrected build.

E/node.ps1 enumerates the six groups before execution. **Retained failure:** the new group initially asserted that mode.error must contain “scope”; the actual unchanged adapter can report “Catalog reader exited; relaunch to read a catalog.” Both new tests failed that expectation despite correctly refused data. The assertion was corrected first for that observed message, then for the existing reader/load terminal-message family so EOF/transport timing cannot make equivalent explicit failures fail the test. E/output/node-inventory-scope-keys.txt, node-scope-keys-final.txt and node-scope-keys-terminal-states.txt preserve the failure and both successful targeted runs. Each successful repeat has two tests; they are not added together. No source/UI/adapter behavior was changed to make the test pass.

Browser counts: foreign 25; OFF 38; ON 36; empty 3; all-excluded 3; ON-zero 11; legacy 38; static 55; exact-ID 12; query-failure 4; encoding-refused 2; four malformed-scope sessions two checks each; ON reopen 36 and foreign reopen 25. The separate red diagnostic has three checks. E/execution-summary.json, per-session browser-results.json/catalog-process.json, exact-query-observations.json, screenshots and command logs retain actual counts/requests/evidence/process outcomes.

Visually inspected the red scope screen and corrected original-input refusal screen. Captures use 1440x1024 CSS at scale 2 (2880x2048 PNG); only the static harness additionally uses its narrow smoke viewport. No new Figma fidelity or GUI redesign claim is made. Browser key entry uses CDP insertion, not native clipboard/manual human qualification.

Historical evidence stays separate: original author/base review; first correction's 24/68 and its retained initial browser failure; controlling review's 24 top-level/68 subtests, reviewer fixture failure history and 25 browser attempts as documented there. The latest identity-only reuse ran no tests. This correction's 26/242 and 17 corrected browsers are new execution, not inherited historical totals. No count of evidence files is treated as a test/audit total.

Production catalogs/scale, hostile races, ACLs, power loss, backup/recovery, native other-platform runtime, interactive console, Docker, external-tar Unicode containment, PR-04 and physical media remain outside qualification. No unrelated decoder, platform port or media work occurred.

## Runnable candidate and final identities

Fresh corrected executable: E/output/corrected-reader.exe, SHA-256 **`d27c07e2a0b08605690bc0eca16d6ad24dd95ca20bfdc9281e662d91d7fc4763`**. Built with `go build -o E/output/corrected-reader.exe .` from this corrected working candidate using E/native.ps1. The pre-edit executable is separately retained; neither old author nor reviewer binaries were overwritten. Cross-build and fault-double hashes are in E/execution-summary.json.

Actual retained viewing command from the repository:

```powershell
$task = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-scope-fix-20260920-194852'
$reader = "$task\output\corrected-reader.exe"
node scripts/gui-preview/server.mjs 0 --catalog "$task\snapshots\on.json" --adapter $reader
# Open the printed loopback URL. Type stop, press Enter, and wait.
```

Existing off.json, on-zero.json, empty.json, all-excluded.json and legacy.json are also retained inputs. The producing argv/exits are in output/producer-*.json. Do not replay a producer over those existing outputs. The following is only a **not-executed example**, with an output that must still be absent:

```powershell
$source = "$task\fixtures\- Generated café O'Brien &+%#"
& $reader --gui-disposable-inventory --ignore-ds-store $source "$task\snapshots\owner-on-next.json"
```

Against the exact 287-entry starting checkpoint, authorized changes are **seven modified paths**: gui_catalog.go, gui_inventory_scope.go, scripts/gui-preview/README.md, and the four living development records. **Three new paths**: gui_inventory_scope_keys_test.go, scripts/gui-preview/inventory-scope-keys.test.mjs and this report. The final inventory contains **290 entries**, with **280** pre-existing entries unchanged. These are identity counts, not staging lists or audited-file totals.

E/final-current-identities.json includes all final source/tests/records and this report; E/candidate-identities.json and candidate/ retain the ten final changed/new files. Together with the verified unchanged pre-edit bytes in before-source and E/before.json, these provide the exact corrected build context for a disposable recheck. E/correction.patch and new-files.patch distinguish this delta from the previously reviewed full inventory diff. Report hash is separate in E/report-identity.json; external evidence manifest includes the report/copy and need not hash itself. Caches/temp/browser profiles are excluded from archival evidence.

Final preservation checks retain historical reports and evidence, Figma/PDF/ZIP/PNG inputs, source/catalog bytes, branch/HEAD/index and unrelated work; authorized changed hashes are explicitly accounted for. All task-owned readers/servers/browsers/tests completed or were stopped and waited; none remain. No review, staging/publication, release or production integration follows automatically.

**ONE next action:** separately submit a targeted reviewer recheck of S1-R1/P2, the preserved reader compatibility boundary and directly affected controls. F1/P2 and F2/P2 stay previously closed; S1 overall awaits reviewer closure.
