# Disposable inventory S1-R1 targeted recheck - 2026-09-20

**GUI_DISPOSABLE_INVENTORY_READY_FOR_OWNER_REVIEW**

| Item | Disposition |
|---|---|
| S1-R1/P2 | **CLOSED** |
| F1/P2 - filename encoding | **PREVIOUSLY_CLOSED** |
| F2/P2 - search fidelity | **PREVIOUSLY_CLOSED** |
| S1 - optional `.DS_Store` exclusion and durable scope | **IMPLEMENTED_AND_VERIFIED** within the bounded disposable preview |

No remaining blocking defect was found in this targeted recheck. Aggregate S1 closure combines the prior verified filtering/provenance/compatibility evidence with this fresh check of the corrected scope interpretation boundary. It does not claim every earlier inventory experiment was rerun, human certification, production readiness or publication authorization.

## Reviewed identity and provenance

This is a same-conversation Codex source-and-execution review, not an independent agent or isolated model context. Submitted Prompt 15D is retained as R/submitted-prompt.txt from attachment `b0f1ad30-89a8-47a1-bb36-cd9ce2912a7f/pasted-text.txt`. Execution date is September 20 in America/New_York; some raw UTC timestamps fall on September 21.

Branch: **`feat/gui-disposable-inventory`**. Full HEAD and overall inventory-patch parent: **`5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`**. Index empty; no merge/rebase/cherry-pick/revert/sequencer/index-lock markers. No competing implementation process was observed. The actual candidate includes uncommitted modifications and untracked source/tests; HEAD alone is not the reviewed implementation.

Reviewer evidence root **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-scope-review-20260920-210104`.

Author correction evidence **A**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-scope-fix-20260920-194852`.

The [controlling finding](GUI-DISPOSABLE-INVENTORY-FOCUSED-RECHECK-2026-09-20.md), [S1-R1 author response](GUI-DISPOSABLE-INVENTORY-SCOPE-S1-R1-FOLLOWUP-2026-09-20.md), relevant current living records/README and native source/callers were inspected. Applicable repository/schema guidance and earlier inventory review/compatibility evidence were reused from this conversation where already read. No applicable AGENTS.md or already completed closing S1-R1 recheck was found. Historical prompts were provenance, not new tasks.

All **290** entries in A/final-current-identities.json matched, including new tests, records and the author report. That source manifest SHA-256 is **`f44e336a5d78b7bec63370837983f3d5ea3f9e91a43316e701b2b157f09d192c`**. The author report is 20,577 bytes, SHA-256 **`b3db65293266518ae18c3840e1a9fbdc4649af1ba16da8bf31361d0a0b352007`**. All **621** artifacts listed in A/evidence-identities.json matched; its SHA-256 is `5b1af1589bc1afd88355ac4345bba1f35150822e2896c9e33f1ebec5bd9cadce`.

R/before.json and author-verification.json retain exact path/size/hash identities. R/source is a verified buildable copy, including untracked implementation/tests, without `.git`, credentials, production state or unnecessary design archives. R/starting-state.json and full-inventory.patch record actual status and the complete tracked diff against the explicit parent. R/author-correction.patch and author-new-files.patch identify the bounded S1-R1 delta against retained pre-edit working bytes: two production Go files, documentation and focused tests. The full inventory diff was used to locate callers, not to restart the base review. No source/test/record changes were made during this task.

These identity counts are not staging lists, test totals or audited-file counts. This new closing report is the only additional repository file, excluded from the 290-entry candidate manifest and hashed separately in R/report-identity.json. No essential source, before-reproduction, executable or screenshot artifact was missing.

## Why S1-R1 closes

The original validator counted eight distinct key spellings before case-insensitive struct decoding. Case aliases could fill that count while an actual required member was missing, and decoding supplied zero or selected a conflicting value by order. The original two inputs omitted `entries` or `excludedFiles`; the reader and browser falsely accepted complete zero-count scope.

The corrected [gui_catalog.go:63](../../../gui_catalog.go#L63) calls `validateGUIInventoryScopeEncoding` after original-byte encoding validation and before `decodeCatalogJSON`. In [gui_inventory_scope.go](../../../gui_inventory_scope.go), `guiScopeMembers` validates decoded member names, membership, presence, uniqueness and non-null values before any struct interpretation can erase duplicates. The accepted scope paths are:

| Path | Exact decoded key policy |
|---|---|
| Catalog wrapper | Canonical `audit`, at most once |
| Supported native audit event | Required `at`, `action`, `detail`, exactly once each |
| Versioned detail JSON | Required `version`, `policy`, `entries`, `regularFiles`, `includedFiles`, `excludedFiles`, `readBytes`, `complete`, exactly once each |

Case variants are rejected, not normalized into accepted aliases. EqualFold is used only to recognize and refuse attempted variants of the audit wrapper; acceptance then requires exact canonical spelling. Filename, path, query and ID strings are not folded. Token-decoded canonical escapes are accepted, while escaped duplicates map to the same key and refuse. Duplicate tracking is local to each affected object; ordinary repeated field names in distinct file/collection objects and repeated text values are not global duplicates.

The raw wrapper check detects canonical duplicates and alias collisions before native decoding, including malformed-before-valid and valid-before-malformed orderings. Required inner nulls/missing/wrong types refuse; explicit zero remains valid. Existing native `inventoryScope` checks still enforce the version/action, timestamp relationship, policy/completion, bounded counts, included/excluded equations and read-byte/file relationships. No new denominator or schema restriction was invented in review.

Genuine native no-history forms—absent audit, audit:null and audit:[]—remain UNKNOWN. Those are empty history representations, not present scope objects with defaultable counts. A malformed event/detail, `Audit:[]`, duplicate wrappers ending in null/empty, or a missing required inner field cannot downgrade to legacy. Existing tests and fresh probes establish that distinction.

The producer still constructs canonical native Audit scope and passes its entire serialized catalog through `loadGUICatalog` before staging/publication ([gui_inventory.go:343](../../../gui_inventory.go#L343)). Thus the changed acceptance boundary is used by both reader adoption and producer validation. There is no alternate weak loader, displayed-label workaround, source rewrite or publication bypass. The unchanged no-replace link remains the publication point; source/output separation, cancellation, bounds and post-publication error ordering remain intact.

Native schema remains 8, with the existing Audit history/action/detail representation. The shared production decoder, migration stack, unrelated advanced-section rejection and persistence mechanisms are unchanged. The Node projection validator retains its exact scope keys/count checks, and the browser renders counts only after successful catalog adoption.

## Negative and positive execution evidence

The exact-before evidence is sufficient and was not automatically replayed. A/red.ps1 and A/output/red-native.json identify a fresh preserved pre-edit uncommitted build, SHA-256 `28cc08f25f42d7e6f6025dfefa6ee95c0cea6f9244dd0f9db0add1c2508f3cc7`, accepting both original inputs. Its retained red browser screenshot shows the false complete-empty disclosure. Original input SHA-256 values are `aa30c6b20eb9109c06518dac139c719f0f2dca9ab2f559231b7ffd52edebfeb9` and `7497babcf52eed1f7b79a7aa627e72706256036c6f0fafb5397c2d2c977e9bdb`. These bytes/probes/source identities were verified, not reconstructed from prose or HEAD.

Fresh R/output/scope-native.json records **23 cases: 19 explicit refusals and four valid controls** against the new reviewer executable. The original missing-count alias files now exit 1 with a scope error, `ok:false`, no catalog or IDs and unchanged bytes. Other cases cover required-field aliases, encoded canonical/alias/duplicate forms, conflicting completion members, event aliases and duplicate audit wrappers after otherwise valid nonempty records. Adversarial members remain raw JSON text; serializers do not deduplicate them before parsing.

The executed candidate tests [gui_inventory_scope_keys_test.go](../../../gui_inventory_scope_keys_test.go) exercise all required detail fields for missing/null/wrong-type/same/conflicting duplicate/case/escaped mechanisms, event members, wrapper collisions, explicit zero, legacy absence and actual read-only protocol refusal. Existing scope validation tests cover version/completion/count/unknown/extra-audit controls. Coverage is grounded in these mechanisms, not the number of generated subtests.

R/supplement.mjs adds **six reviewer-owned raw-input controls**, separate from candidate tests: a valid canonical audit before nonempty records; fully escaped canonical wrapper/event/detail keys; a case alias before records; valid-before-malformed and malformed-before-valid duplicate wrappers; and same-value duplicate wrappers. Valid counterparts retain all 28 file records and scoped counts; each invalid counterpart returns specifically an inventory audit scope error, not an unrelated missing dependency/schema/relationship error. All input hashes remain unchanged. A seventh supplemental operation creates a distinct explicit `--ignore-ds-store=false` output and checks its 28 records and identical scope values against the default-OFF oracle.

The actual Node reader/server route passes both scope-key tests: failed native scope cannot bootstrap catalog data, queries fail without IDs, and fresh valid -> invalid -> valid launches do not borrow prior counts or poison recovery. Browser sessions for both originals, a late duplicate wrapper and an encoded duplicate show explicit refusal/relaunch without results, selected evidence, scope counts, successful empty state or static-demo fallback. Native stderr/protocol establish the scope rejection cause; the unchanged adapter may surface its generic reader-exited/relaunch message.

Fresh generated positive outputs use explicit absolute task-owned source/output paths and separate absent output names. The 28-file oracle has 37 entries and 28 regular files: default/explicit OFF includes 28, excludes 0 and reads 851 bytes; ON includes 26, excludes exactly two and reads 782 bytes. Directory `.DS_Store` children, near/case names in separate directories, other dotfiles/metadata and .xmp/.aae controls remain. Separate empty, all-excluded and ON-zero sources retain distinct scope in actual browser views. Legacy catalogs retain UNKNOWN and searchable `.DS_Store` records.

The unchanged exclusion predicate is exact regular basename equality after classification and before content-open/hash. The selected exclusion/read-stage observer, excluded-boundary, publication and object-separation tests pass. Preservation includes generated entry/content comparisons and inspection of the actual read-only/open/publication paths; final hashes alone are not presented as proof of no transient writes or no producer reads.

R/lifecycle.mjs reopens ON with the task-generated source temporarily unavailable, restores that source, and reopens foreign recorded data separately. Scope persists without CLI/source/browser preference. Six forbidden operation/helper/path routes refuse. The actual viewer accepts split `st` / `op\n` with stdin held open, exits 0 naturally, waits native reader exit 0 and closes its listener. Browser processes are forced/stopped/waited separately; that cleanup is not natural-stop or interactive-console certification.

## Compatibility and closed findings carried forward

The S1-R1 correction tightens metadata interpretation without changing the reviewed native format or reader capability boundary. New populated-audit snapshots require the corrected preview reader; older supported unscoped snapshots remain UNKNOWN. There is no scope stripping, migration or broad production-consumer support requirement.

The prior reader's existing provenance is reused: pre-correction uncommitted inventory reader SHA-256 **`bc7e1fc73ec1121fa8c4523b8ae3b6917db91abf1760138089cd82433b374a69`**, not the published parent. R/output/prior-reader-compatibility.json freshly verifies its two new OFF/ON refusals (`unsupported populated section: Audit`, exit 1) and successful legacy control (exit 0). Corrected reader/projection/browser controls accept new scope and genuine legacy absence. No old-reader rebuild or broader compatibility investigation was required.

F1/P2 and F2/P2 remain previously reviewer-closed. Focused encoding regressions still reject malformed UTF-8/surrogates before repair and preserve legitimate U+FFFD/escaped text. The foreign-data browser control exercises actual quoted LF/CR/CRLF entry, literal backslash decoys, exact semantic URLs, IDs and selected hash/size evidence, plus disclosure/history/skip preservation. Ordinary literal search, exact large IDs, static mode and query-failure clearing pass. No foreign recorded path is opened. No new regression was observed and their historical closures were not reopened.

## This review's execution and provenance limits

Fresh reviewer producer/reader: **R/output/corrected-reader.exe**, SHA-256 **`f5d934b3649a1d5e3db1894dec3504acf44f955f9827fd05c63dc45f3357415d`**. Built using `go build -o R/output/corrected-reader.exe .` from verified R/source, with the full working-byte manifest above. The author's prepared corrected executable was not used as current execution proof. The separate fault-double SHA-256 is recorded in R/execution-summary.json.

Actual tooling: Go 1.26.8 windows/amd64, Node 24.19.0, Chrome 153.0.8010.48. Existing audit toolchain/module cache, GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0; reviewer-owned cache/temp with GOTMPDIR unset. No installs, global changes, downloads, WSL or remote CI.

| Fresh reviewer execution | Result |
|---|---|
| Enumerated Windows native selection | **14 top-level passes / 219 subtest passes**, zero failures/skips |
| Fresh Windows producer/reader and fault-double builds; vet | Exit 0 |
| Read-only gofmt and whitespace | No findings; line-ending advisories retained |
| Node stop-command / preview / catalog / catalog-correction / inventory-correction / inventory-scope-keys | **5 / 7 / 4 / 8 / 4 / 2** passes, zero failures/skips |
| Generated native snapshots | Five initial successful outputs plus one explicit-OFF supplemental output |
| Raw scope probe set | 23 cases: 19 refusals / four valid controls |
| Reviewer supplemental raw set | Six cases: four refusals / two valid controls |
| Real-browser campaign and reopen controls | **17 completed sessions**, no failed sessions |
| Cross-build/native foreign execution | Not run in this review; author Linux amd64/Darwin arm64 cross-build/vet evidence verified and retained as historical |

R/native.ps1 lists then executes `^Test(GUIEncoding|GUIExactNameProtocol|GUIInventory(Scope|Exclusion|ExcludedBoundaries|PublicationPoint|ObjectSeparation)|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))` with `-timeout 120s -count=1 -json`. This narrower selection is not the author's full 26-test selection. R/node.ps1 enumerates six named suites and uses `node --test --test-isolation=none`. Scripts, argv, test events, exits, independent fixture oracles, hashes, screenshots and process events are retained. Supplemental probes are external; no repository tests/goldens were changed.

Browser check counts: foreign 25; OFF 38; ON 36; empty 3; all-excluded 3; ON-zero 11; legacy 38; static 55; exact-ID 12; query-failure 4; encoding-refused 2; four malformed-scope sessions two checks each; ON reopen 36 and foreign reopen 25. Counts are not combined across sessions or with native tests. Requests and semantic query/ID/evidence observations are in per-session browser-results.json and exact-query-observations.json.

Visually inspected R/output/browser-refused-reviewer-missing-entries-case-alias/catalog-refused.png and browser-all-excluded/scope-library.png. They show explicit failure for malformed scope and a distinct one-excluded/zero-included successful scope for valid data. Other scope and exact-name screenshots are retained. Catalog viewport is 1440x1024 CSS at scale 2, producing 2880x2048 PNGs; only static additionally exercises its narrow smoke viewport. CDP insertion is browser automation, not native clipboard or human testing. No design-fidelity approval is implied.

No reviewer setup/test/build/browser failure or hidden retry occurred. Expected invalid-input nonzero exits are asserted refusals, not failed tests. Native/browsers completed once per recorded selection/case. Prior evidence is separate: base inventory review; F1/F2 closure and original S1-R1 NEEDS_CHANGES; identity-only reuse with no tests; author correction 26/242, Node 5/7/4/8/4 and its initial new two-test error-message failure followed by two passing targeted runs, 17 corrected browsers and one red diagnostic. Those historical failures and repetitions remain preserved, not erased or counted as fresh reviewer execution.

## Preservation and owner gate

All **290 pre-existing candidate paths** remain byte-identical, including untracked source/tests, four living records, all earlier reports, Figma/PDF and ignored ZIP inputs. Verified R/source bytes also remain unchanged. Historical author/reviewer artifacts and referenced PNG evidence remain preserved under their manifests. Generated sources, original/raw scope inputs and viewed snapshots retain their bytes and entry identities; the reviewer-owned unavailable-source rename was restored. The one new review report is separately hashed in external evidence.

R/final-preservation.json, final-status.txt, output/final-processes.json, final-current-identities.json, report-identity.json and evidence-identities.json retain closeout state. The external manifest excludes caches/temp/browser profiles and itself. Every task-owned reader/server/browser/test process completed or was stopped and waited; none remain. Branch, HEAD, empty index and unrelated work are unchanged. No staging, commit, push, merge, source correction, living-record edit, publication or production scan occurred.

This bounded closure does not qualify production catalogs/scale, hostile races, ACLs, power loss, backup/recovery, native Linux/macOS execution, interactive consoles, Docker, external-tar Unicode, PR-04 or physical media. Those existing limits are not new blockers for the reviewed disposable slice.

**ONE next action:** owner acceptance and separately authorized scoped inventory publication, including the reader-compatibility notice that new scoped snapshots require the corrected preview reader and older supported snapshots retain UNKNOWN scope. No further broad review, feature work or publication is started here.
