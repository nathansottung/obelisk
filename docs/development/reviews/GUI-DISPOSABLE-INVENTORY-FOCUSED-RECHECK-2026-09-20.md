# Disposable inventory filename and scope focused recheck - 2026-09-20

**NEEDS_CHANGES**

| Controlling item | Reviewer disposition |
|---|---|
| F1/P2 - malformed encoded filenames become replacement-character names | **CLOSED** within the supported Unicode/disposable-preview contract |
| F2/P2 - newline-containing names lose characters in search | **CLOSED** through the explicit exact-name input contract |
| S1 - optional `.DS_Store` exclusion and durable scope | Filtering and normal scope/compatibility behavior verified; **NOT CLOSED**, due to S1-R1/P2 below |

S1 was missing requested functionality, not an original reproduced producer-safety defect. This recheck finds a concrete defect in its new scope validator. The bounded base inventory is not relabeled unsafe, and independently verified F1/F2 results are retained. No implementation correction was made.

## Candidate and reviewer provenance

This is a same-conversation Codex tool-assisted review, with source inspection, fresh Windows execution and actual headless Chrome input checks. It is not an independent agent, isolated-context review, human certification, owner acceptance or publication. The controlling authorization is submitted Prompt 15B, attachment `9d95356c-7792-4f79-85d2-8988d41b5c47/pasted-text.txt`, copied to external evidence as `submitted-prompt.txt`. Historical prompts were read as provenance, not executed as tasks.

Branch: `feat/gui-disposable-inventory`. Full HEAD and explicit complete inventory-patch parent: **`5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`**. The reviewed implementation is uncommitted working bytes, including untracked source/tests; HEAD alone is not this candidate. Index empty; no merge, rebase, cherry-pick, revert, sequencer or index-lock marker. No branch operation or remote lookup. No matching completed INVENTORY focused recheck existed at start.

Reviewer evidence root **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-focused-20260920-191824`.

Successful fixture/browser campaign **V** is `R/verified-run`. Author correction evidence **E** remains `C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-fix-20260920-163849`.

Read the actual [implementation report](GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md), [substantive review](GUI-DISPOSABLE-INVENTORY-REVIEW-2026-09-20.md), [correction follow-up](GUI-DISPOSABLE-INVENTORY-REVIEW-FOLLOWUP-2026-09-20.md), relevant current handoff/next-actions/status/coverage entries, preview README, native representation/callers and CONTRIBUTING schema/build rules. No applicable AGENTS.md was found in the repository or checked ancestor locations.

All **286** entries in E/final-current-identities.json matched the actual candidate, including the follow-up report. Its SHA-256 is `03bb49db99170d99c193ddc5b2a9adbb689b22fedad1c4dd80cca3ec39ef9ba9`. R/before.json retains that full path/size/hash inventory, SHA-256 **`3327470f9958928e8da88ad266bc636d2c3a825246d8f2a7624fbb2d4954dbd3`**. This identity inventory is neither a staging list nor an audit-coverage claim. This new report is excluded and hashed separately.

R/source contains verified buildable copies of those source/test/document bytes, excluding unnecessary Figma/PDF/ZIP/PNG binaries and `.git`. R/starting-state.json, full-inventory.patch, author-candidate-identities.json and author-correction-new.patch preserve status, complete tracked diff against the explicit parent, and correction identities/additions. Untracked source bytes are preserved directly, not inferred from tracked diff. Correction versus base is established by E/before.json, E/before-source and the recorded 16-modified/seven-added correction mapping; the retained base is not reconstructed from HEAD.

All artifacts listed by the correction, prior inventory reviewer and original inventory author manifests were verified: **681 / 896 / 260**, separately, with manifest identities in R/historical-verification.json. Required source/reproduction/screenshot/binary artifacts were available. These counts do not convert historical execution into fresh execution.

## S1-R1 / P2 - missing scope counts and conflicting aliases become reassuring defaults

Anchors: [gui_inventory_scope.go:43](../../../gui_inventory_scope.go#L43), [gui_inventory_scope.go:59](../../../gui_inventory_scope.go#L59), [gui_inventory_scope.go:63](../../../gui_inventory_scope.go#L63), [catalog-ui.mjs:39](../../../scripts/gui-preview/catalog-ui.mjs#L39).

The first validation pass checks uniqueness of raw key spellings and only the number of distinct keys, rather than requiring the exact eight declared keys. The subsequent Go JSON struct decoder matches field names case-insensitively even with DisallowUnknownFields. Consequently, differently cased aliases can occupy the eighth slot while an actual required field is absent. The missing integer receives zero; conflicting aliases are resolved by decode order before projection. The Node validator sees an already normalized, apparently complete scope and accepts it.

R/scope-probe.mjs starts with a freshly produced empty native snapshot and changes only its audit detail. This exact detail lacks `entries` and contains conflicting version aliases:

```json
{"Version":2,"version":1,"policy":"ignore-exact-ds-store","regularFiles":0,"includedFiles":0,"excludedFiles":0,"readBytes":0,"complete":true}
```

The fresh reader exits **0**, emits `ok:true`, and projects version 1, entries 0 and complete true. A second probe omits `excludedFiles` and supplies `"Complete":false,"complete":true`; it also exits 0 and projects excludedFiles 0/complete true. A canonical empty positive control succeeds; simply omitting entries without the extra alias refuses with exit 1. Thus acceptance is not an unrelated launch/schema failure or a fixture normalized before the native reader.

Exact inputs, hashes, argv, stdout/stderr and exits are in R/scope-reproduction.json and V/snapshots/reviewer-*.json. All four input hashes remain unchanged. The real browser accepts the first malformed case and displays **“0 entries visited”**, **“Completed within selected scope”**, and **“Truly empty source.”** V/output/browser-reviewer-scope/scope-library.png and browser-results.json retain that decisive presentation. Its three completed diagnostic checks mean reproduction of the defect, not successful strict-scope validation.

This violates the required absence-versus-observed-zero and ambiguous/unsupported-scope refusal contract. It does not establish source mutation or corruption of normally generated output. The necessary bounded correction is to enforce the exact required detail-key set and reject aliases/ambiguity before struct decoding, with regression cases for missing zero-valued counts and conflicting completion/version aliases through the real loader/projection. No migration or broad decoder repair is needed to address this finding. No fix is authorized or applied during this review.

## F1/P2 - encoding closure

[gui_catalog.go:59](../../../gui_catalog.go#L59) reaches validateGUIJSON before the unchanged shared native decoder. [gui_encoding.go](../../../gui_encoding.go) checks original UTF-8/JSON and active surrogate escapes, respecting escaped backslashes and quotes. Validation covers the complete bounded input before adoption. The original loss was legacy json.Unmarshal repairing malformed input; checking the decoded string would not distinguish that repair from a valid literal U+FFFD.

The producer validates path/name strings before marshaling. Windows additionally checks original native UTF-16 enumeration before os.ReadDir conversion; invalid scalar units refuse. Non-Windows name strings require valid UTF-8. Foreign names are data-only here; invalid Windows filesystem names were not claimed as created. Quiescent-tree and valid-arriving-CLI-argument limits remain explicit.

Fresh TestGUIEncodingOriginalBytes and TestGUIEncodingCompleteAdoption exercise raw FF, truncated UTF-8, UTF-8 surrogate bytes, isolated high/low escapes, invalid pairs, first/later-record refusal and legitimate replacement-character controls. Exact native queries retain valid controls. V/output/corrected-encoding.json replays the retained original FF and isolated-high-surrogate files through the fresh executable: both refuse; the valid literal replacement succeeds; input hashes remain unchanged. Verified retained red evidence establishes the corresponding pre-correction repaired successes without another old-source browser campaign.

The adapter's actual response path uses fatal TextDecoder decoding; scalar checks precede presentation/URL conversion. The affected Node tests cover first/later raw response fields, malformed percent-encoded HTTP input and valid scalar controls. The encoding-refused browser shows refusal without successful empty data or static fallback. Accents, non-Latin text, composed/decomposed spellings, emoji, U+FFFD, quotes, punctuation and backslashes retain their own IDs/hash/size in generated/foreign fixtures. Existing whole-load, advanced-section and no-replace controls also pass. Shared store.go decoding/schema and production startup remain unchanged.

## F2/P2 - actual browser input and evidence closure

The implemented contract is explicit **Exact name (JSON string)** with discoverable help and the inspector's **Use exact name** control. Ordinary search remains literal, case-insensitive substring search with its documented surrounding-whitespace trimming. Exact mode preserves case and every character and does not combine with the hash filter. This review does not impose different search semantics.

The decisive browser uses keyboard select-all, CDP Input.insertText and Enter in the real control. It observes actual fetch URLs/responses rather than replacing them or directly calling application search. It is simulated browser insertion, not native clipboard or manual-console testing. Fourteen foreign-data records distinguish LF, CR, CRLF, stripped spelling, literal backslash-n/r, leading/trailing controls, tabs/spaces, punctuation, Windows-looking text, U+FFFD, emoji and quote/backslash data with independent record/hash/size oracles.

V/output/browser-foreign/exact-query-observations.json records exact semantic parameters and native IDs. Actual URLs contain `%0A`, `%0D`, `%0D%0A` for the three control names and `%5Cn` for literal backslash-n; selected records 1, 2 and 3 remain distinct from decoys. The ordinary-input check emits `text=line%5Cnname.txt&hash=` and selects only its literal record. V/output/browser-foreign/exact-record-3.png visibly shows quoted CRLF entry, record 3 and its own checksum/28-byte evidence.

Malformed explicit input clears results and sends no repaired query; subsequent valid input recovers. Exact entry, selection and evidence survive disclosure modes, Back/Forward and keyboard skip activation with main focus. Existing query-failure protection, large exact IDs and static controls pass. Display escaping uses textContent and never changes authoritative path or ID. Source inspection confirms literal ordinary Windows/punctuation strings are not implicitly decoded; no foreign recorded path is opened.

## S1 filtering, durable scope and compatibility evidence retained

Actual CLI: `--gui-disposable-inventory [--ignore-ds-store[=true|false]] ABS_GENERATED_SOURCE ABS_NEW_CATALOG`. Default/explicit false include regular `.DS_Store`; bare/true enable exclusion. Flag precedes paths; invalid values, duplicate/misplaced flags and help return documented usage/exit 1. This is not a Settings preference.

The predicate is exactly `ops.ignoreDSStore && child.Name() == ".DS_Store"`, after path/link/type classification and regular-file bounds, before the read hook/os.Open/hash stage. Excluded entries remain in observations/rechecks and traversal/file limits. The executed read-stage observer proves those contents never reach the producer's read stage; independent reviewer hashing is separate. Native enabled-mode tests cover case variants in separate directories, exact-name symlink refusal, classification error, cancellation, entry/file bounds, changed excluded entries, existing and genuinely late output collisions. Existing junction and publication-point tests passed without skips. Source code preserves the no-replace link and truthful post-publication error ordering.

Fresh OFF/ON outputs use distinct absent names and an independently specified 28-file generated tree: 37 entries, 28 regular files; OFF includes 28/excludes 0/reads 851 bytes; ON includes 26/excludes exactly two/reads 782 bytes. Near/case names, other dotfiles/AppleDouble-style controls, .xmp/.aae and the child of a `.DS_Store` directory remain. Empty, all-excluded and ON-zero use separate small sources. Fixture entry/content and catalog hashes are verified after viewing. No producer source cleanup, source rename, permission change or global preference exists.

The native representation is the existing Audit{At,Action,Detail} history in store.go, populated directly in the new in-memory catalog with `GUI_DISPOSABLE_INVENTORY_V1` and a versioned detail. Prompt 15A explicitly allowed compatible provenance reuse and a narrow reader adaptation. There is no new native field, schema-version change, old-catalog rewrite, sidecar, unrelated-field sentinel or multi-file publication change. The preview-only populated-Audit acceptance boundary is intentional and within that authorization; **its incomplete strictness is S1-R1**, not an unresolved migration approval. Other advanced sections remain refused by the existing reflection/subset checks; arbitrary audit actions also refuse. Production consumers are not qualified by this decision.

| Actual reader/input | Fresh observed result |
|---|---|
| Corrected reader + new OFF/ON/empty/all-excluded/ON-zero | Supported scope validates and displays expected policy/counts |
| Corrected reader + older unscoped fixture | Records unchanged; historical scope UNKNOWN; `.DS_Store` remains searchable |
| Retained identified pre-correction reader + new OFF/ON | Exit 1, `ok:false`, `unsupported populated section: Audit`; no scope silently dropped |
| Same pre-correction reader + legacy supported fixture | Exit 0, valid records; positive launch/input control |

The fresh corrected Windows executable R/output/corrected-reader.exe has SHA-256 **`6816f61c26bc909f9123c738cab2657367f783ee5bf36fc82bd584a980e3e11b`**, built with `go build -o R/output/corrected-reader.exe .` from the verified R/source manifest. This is a new build, not the author's prepared corrected binary. Its path-dependent build identity is not expected to equal the author's executable hash.

The old control is the retained **pre-correction uncommitted inventory reader**, not the published parent: SHA-256 **`bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69`**. E/red.ps1 records `go build -o E/output/before-reader.exe .` in E/before-source; E/before.json identifies its 279-file checkpoint and E/output/before-build-exit.txt records success. Its bytes/provenance were verified and copied without overwriting E. R/execution-summary.json identifies both executables and the separate fault double. V/output/prior-reader-compatibility.json retains the matrix's raw old-reader results.

V/lifecycle.mjs reopens ON with its generated source temporarily unavailable and separately reopens foreign data, then restores the source. Scope survives fresh processes and does not depend on the CLI invocation, source or browser preference. Separate OFF/legacy launches show their own policy; no ON-state leak. Six unsupported operation/helper/path routes refuse. The actual catalog server accepts split `st` / `op\n`, exits 0 naturally with stdin held open, waits for reader exit 0 and closes its listener. Whole/split/CRLF/character stop and bounded fault cleanup remain covered by the selected existing Node controls. Browser shutdown is forced and waited, separately recorded; it is not natural-stop evidence.

## This review's executions and diagnostics

Installed tooling: Go 1.26.8 windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. Existing audit toolchain/module cache; GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0, task GOCACHE/TEMP/TMP and no GOTMPDIR. No installs, upgrades, global changes, remote CI or WSL.

R/native.ps1 enumerates then runs `^Test(GUIEncoding|GUIExact|GUIInventory|HashReaderBothRefusesPartialError|HashFileBothMatchesSHA256|ScanRecordsBlake3|ScanFolder_|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))` with timeout 120s, count 1 and JSON events. Its author-derived formatting command was changed to **read-only -l before execution**; no formatting edits ran. R/source still matches candidate bytes.

| Fresh execution layer | Result |
|---|---|
| Native selected tests | **24 top-level / 68 subtest passes**, zero failures/skips |
| Fresh Windows producer/reader + fault-double builds; vet | Exit 0 |
| Linux amd64 / Darwin arm64 cross-build and vet | Exit 0 for both; native runtimes NOT RUN |
| Read-only formatting and tracked whitespace | No findings; Git line-ending conversion advisories retained separately |
| Successful V Node groups: stop-command / preview / catalog / catalog-correction / inventory-correction | **5 / 7 / 4 / 8 / 4** passes, zero failures/skips |
| V producer and CLI | Five initial new snapshots; two explicit true/false new outputs; four deliberate usage refusals |
| V principal browser campaign | **11 completed sessions** |
| V source-unavailable/reopen controls | **Two completed sessions** |
| Supplemental scope probe | Four diagnostic cases, including two reproduced erroneous acceptances |
| Supplemental scope browser | One completed diagnostic reproduction; not a compliance pass |

V session check counts: foreign 25; OFF 38; ON 36; empty 3; all-excluded 3; ON-zero 11; legacy 38; static 55; exact-ID 12; query-failure 4; encoding-refused 2. Reopens: ON 36 and foreign 25. Scope diagnostic: 3. These counts are not summed into browser sessions or native tests. Exact argv, selected identities, per-suite logs/exits, request/response evidence, screenshots and waited process events are retained in the scripts and output directories.

**Reviewer harness failure retained:** the first `node fixtures.mjs ..` invocation passed relative source/output paths. The candidate correctly refused with `published:false`, processedFiles/readBytes zero and “explicit bounded absolute output required.” The shell continued into tests: initial four unaffected Node groups passed; the inventory-correction group failed because its snapshots did not exist. Seven dependent browser sessions failed before any completed check; four unaffected static/ID/query-failure/encoding-refusal sessions completed (55/12/4/2). All remain under R/output. The corrected campaign used explicit absolute paths and a new V directory; no failed output was deleted/reused. The repeated unaffected checks are recorded, not added together as new coverage. Overall there were 25 browser attempts: seven fixture-dependent failures, 17 completed compliance sessions including four repeated controls, and one diagnostic reproduction. No candidate fix occurred between attempts.

An initial unprivileged process inventory returned access denied; an authorized elevated read-only process query supplied lifecycle bookkeeping. This was a tooling permission diagnostic, not a candidate execution failure. The finding's positive diagnostic expectations are clearly distinguished from compliance expectations.

Visually inspected V/output/browser-reviewer-scope/scope-library.png and browser-foreign/exact-record-3.png. Catalog captures are 1440x1024 CSS at scale 2, yielding 2880x2048 PNGs. Only the static harness additionally exercises its narrow viewport; inherited JSON labels do not claim both viewports ran for every catalog session. No new design alignment or Figma fidelity approval is claimed.

Historical results remain separate: the base author's overlapping native selections and nine browsers; prior reviewer 17/47 plus supplemental 3/13 and 12 completed browsers with two distinct failures; correction author 24/68, initial Node 5/7/4/8/3 then affected 4/8/4, 16 completed corrected browsers plus one corrected initial failure, and reported Windows/Linux/Darwin builds/vet. Raw manifest verification preserves that provenance but does not relabel it this review's execution.

## Preservation and next gate

All 286 pre-existing candidate files remain byte-identical, including untracked source/tests, four living records, prior reports and the original untracked Figma/PDF plus ignored ZIP. Historical evidence and referenced PNG bytes remain unchanged under their verified manifests. R/source also remains byte-identical to its copied source entries. Generated source/catalog inputs are preserved; the deliberate unavailable-source rename was scoped to this review's generated fixture and restored. No production catalog, source/media path, credential or `.git` copy was used.

This report is the only new repository file. R/final-preservation.json, final-status.txt, final-processes.json, report-identity.json and evidence-identities.json retain final checks and a separate report hash. Evidence manifests exclude caches/temp/browser profiles and themselves. Branch, full HEAD, index and unrelated work remain unchanged; no staging, commit, push, merge or publication. Every task-owned test/browser/reader/server process completed or was stopped and waited; forced browser/fault cleanup is not counted as command-driven shutdown.

Production scale/catalogs, hostile-filesystem races, ACL enforcement, power loss, backup/recovery, Docker, native Linux/macOS execution, native clipboard/interactive consoles, external-tar Unicode containment, PR-04 and physical media remain outside this bounded result. No new platform/backend qualification was started.

**ONE next action:** separately authorize a bounded correction of **S1-R1/P2** (exact required scope keys and alias/ambiguity refusal), preserving F1/F2 closures and the established reader compatibility notice. Owner acceptance/publication remains gated on that correction and its focused recheck. Stop here; no source/test or living-record edits accompany this report.
