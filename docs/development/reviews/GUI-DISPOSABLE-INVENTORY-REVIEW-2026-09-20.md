# Disposable directory inventory substantive review - 2026-09-20

**NEEDS_CHANGES**

The bounded generated-directory -> new native catalog -> read-only viewer workflow passed the executed base checks. The combined requested milestone is not accepted: filename fidelity has two reproduced boundary limitations, and the optional exact `.DS_Store` exclusion with durable scope disclosure is not implemented. Missing addendum scope is not labeled a reproduced producer safety failure.

This is one substantive, same-conversation Codex review of the existing uncommitted candidate. It is not independent-agent/context-isolated review, human certification, owner acceptance or publication. No correction, candidate-test edit, living-record update or branch change was made. Only this report was added to the repository.

## Reviewed identity and provenance

Branch: `feat/gui-disposable-inventory`. Full HEAD and complete patch base: `5bc3a58d7ea9ad8f6a963859e0b3e52be06b122b`. The implementation is in working bytes, including untracked additions, not HEAD. Index empty; no merge/rebase/cherry-pick/revert/sequencer/index-lock markers. No remote check was needed or performed.

Reviewer evidence root **R**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-review-20260920-160314`.

Author evidence root **A**:
`C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-20260920-145714`.

The [author implementation report](GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md), current handoff/status/next-actions/coverage/README, actual command/parser/source/tests, retained author authorization and manifests were inspected. The accepted catalog reports were consulted for affected reader, exact-ID, failure-state and stop contracts. No applicable AGENTS.md or matching completed inventory review was found. Historical PR-04 and Windows external-tar Unicode containment remain outstanding; neither was reconstructed or cleared by this review.

All 278 entries in A/final-current-identities.json matched current working bytes, including the new implementation/tests and preserved design inputs. All 260 artifacts listed in A/evidence-identities.json matched. R/author-verification.json records the author manifest identity. An evidence-artifact manifest was not substituted for the source manifest. R/before.json and R/source/ retain the complete starting inventory and bytes; R/candidate-identities.json retains the exact 16-path candidate manifest. R/tracked.patch is against the explicit parent; R/author-new.patch and the verified new-file bytes cover additions. R/candidate-copy is a disposable copy used for supplemental probes; its reviewer helpers are explicitly separate from candidate files.

The scoped candidate is exactly nine modified paths and seven additions:

| Kind | Paths |
|---|---|
| Modified implementation/tests | main.go; hashing.go; hashing_test.go; scripts/gui-preview/browser-check.mjs |
| Modified documentation | scripts/gui-preview/README.md; docs/development/CODEX_HANDOFF.md; NEXT_ACTIONS.md; OB_STATUS.md; REVIEW_COVERAGE.csv |
| Added implementation/tests | gui_inventory.go; gui_inventory_windows.go; gui_inventory_other.go; gui_inventory_test.go; gui_inventory_windows_test.go; scripts/gui-preview/inventory-browser-checks.mjs |
| Added author report | docs/development/reviews/GUI-DISPOSABLE-INVENTORY-IMPLEMENTATION-2026-09-20.md |

No later addendum candidate or report was present. A/submitted-prompt.txt establishes the base inventory authorization; it does not contain the requested optional exclusion. Available source, documentation and retained provenance do not establish receipt or execution of Prompt 14A, and no explicit owner deferral was found. This review's submitted Prompt 15 explicitly supplies B/C as requested requirements. It therefore establishes their review scope now, without retroactively claiming that an unsubmitted addendum was executed or ignored.

## Separate requirement dispositions

| Requirement | Disposition |
|---|---|
| A. Bounded actual inventory into a validated NEW catalog, then read-only browsing | Implemented and verified within the declared small, quiescent, generated local Windows envelope. No new material base-publication/source-write defect reproduced. This does not independently accept the combined milestone. |
| B. Supported filename fidelity, explicit unsupported behavior and platform limits | Partially implemented. Actual Windows Unicode/punctuation names and opaque foreign backslashes round-trip, search and select correctly in the exercised cases. Malformed encoded names are accepted after replacement (F1); accepted newline-containing recorded names cannot be searched literally through the UI (F2). Cross-platform runtime qualification is NOT RUN. |
| C. Optional default-off exact-regular-basename `.DS_Store` exclusion with durable visible scope | Not implemented. Default inclusion verified. No enabled mode, exclusion counts/reasons, persisted policy/version or reopened scope disclosure exists. This is missing requested scope (S1), not a failed test of an existing option. |

## Reproduced findings and missing work

### F1 - P2: malformed encoded paths become different accepted filenames

Anchors: [gui_catalog.go:60](../../../gui_catalog.go#L60), [store.go:1159](../../../store.go#L1159), [gui_catalog.go:298](../../../gui_catalog.go#L298); producer serialization/validation at [gui_inventory.go:309](../../../gui_inventory.go#L309).

The preview loader delegates to `decodeCatalogJSON`, which is `json.Unmarshal`, without a pre-loss encoding check. A reviewer-owned valid native snapshot was changed only at the first file's `rel_path`: one probe placed byte FF inside the JSON string `x<FF>.txt`; another used the unpaired JSON escape `x\ud800.txt`. Both fresh native-reader processes exited 0, returned successful projections with `x\ufffd.txt`, and returned ID `1` when searching the replacement spelling. A third positive control containing the legitimate literal U+FFFD also loaded and searched correctly. The malformed originals and literal replacement become indistinguishable at the projected filename boundary. Input catalog bytes remained unchanged; the loss occurs in the accepted in-memory representation.

Reproduction: R/cli-and-summary.mjs, R/output/encoding-cli.json and the three separately retained R/snapshots/{invalid-utf8,lone-surrogate,literal-replacement}.json inputs. Supplemental `TestReviewerEncodingBoundary` records the same behavior with actual loader/reader calls. Its passing test status means the diagnostic reproduced the specified behavior, not that malformed-name acceptance satisfies the requirement.

This decoder is unchanged from the published parent. R/output/parent-reader-comparison.json records matching reader/server/protocol/UI source semantics, accounting explicitly for normal Git working-tree CRLF representation without rewriting anything. This is an inherited compatibility gap exposed by the expanded filename requirement, not a demonstrated new Windows enumeration regression. No invalid Windows filesystem name was created, and no Linux/macOS byte-name runtime claim follows. The producer also validates after JSON marshaling; that ordering alone cannot prove preservation of a Go string that encoding already repaired.

Smallest complete correction: define the supported encoded-name contract and validate before irreversible replacement at the applicable preview input, producer serialization and protocol boundaries. Refuse unsupported input explicitly while preserving valid literal U+FFFD. Cover malformed UTF-8, unpaired escapes and valid Unicode round trips without weakening native subset validation or undertaking a general production decoder repair. A post-decoding `utf8.ValidString` check alone would be insufficient.

### F2 - P2: an accepted newline-containing recorded name cannot be searched literally in Find

Anchors: [catalog-ui.mjs:47](../../../scripts/gui-preview/catalog-ui.mjs#L47) and [catalog-ui.mjs:48](../../../scripts/gui-preview/catalog-ui.mjs#L48).

A data-only foreign snapshot retained the native schema and distinct records, with one path `line\nname.txt`. The reader/protocol accept it. The real browser's single-line `type=search` input changes that full value to `linename.txt`, so the literal full-name search does not find the record. R/output/browser-foreign is the failed expectation: four earlier checks completed, then the condition for the newline name timed out; this session is not a pass.

R/output/browser-foreign-diagnostic-v2/data-boundary-observations.json captures requested versus actual input. That completed diagnostic separately proves the exact control-containing path survives native HTTP query and DOM text, and the correct record/hash/size remains selectable from unfiltered results. Literal backslashes and hostile-looking `<img src=x onerror=alert(1)>.txt` remain inert text, not local paths or executable markup. Newline/tab glyphs visually collapse as whitespace in ordinary rendering, so DOM equality alone is not an unambiguous human-readable representation. These are data-only foreign names, not impossible local Windows fixture claims.

Smallest complete correction: explicitly support control-containing recorded components with a reversible display/search representation, or document and enforce a compatible unsupported-input refusal before successful presentation. Keep authoritative spelling and IDs separate from display/search keys. Preserve substring/case-insensitive search without merging records. Do not silently remove characters or treat an unqualified foreign path as a host filesystem path. This finding does not require a GUI redesign or native Mac/Linux installation.

### S1 - missing requested scope: optional exclusion and durable inventory scope

Anchor: [gui_inventory.go:415](../../../gui_inventory.go#L415). `runGUIInventory` accepts exactly two positional arguments; there is no exclusion parser/option. Traversal hashes every supported regular file. Native Collection/Folder/File output and the preview projection contain no effective exclusion policy/version or observed excluded counts. There is no enabled-mode runtime behavior to claim or test.

R/names.mjs created ordinary root/nested `.DS_Store` files. Both appear in the default snapshot, along with `.DS_Store.bak`, `photo.DS_Store`, `._.DS_Store`, `__MACOSX`, `.Spotlight-V100`, `.Trashes`, Thumbs.db, desktop.ini and photo .xmp/.aae examples. A directory named `.DS_Store` retains its child sentinel. Default behavior is correct inclusion, but it does not implement the requested option.

One correction pass must address the entire requested option: default off; enabled exclusion of only an enumerated regular basename exactly `.DS_Store` at any depth; no case folding or platform gating; type/link refusal before filtering; excluded entries still bounded; source contents untouched; honest observed counts/reasons; OFF/ON outputs always NEW; no-replace/cancel controls; only-excluded versus truly empty; durable effective policy/version/scope visible after reopen; older policy-unknown catalogs left truthful and their records visible. Case variants that cannot coexist locally need separate fixtures or pure matching tests.

Durable provenance needs an explicit native compatibility decision: the current producer/reader projection has no field for that policy. This review does not prescribe a schema migration, invent a native field, relax advanced-section refusals or accept an external developer log as historical scope. Establish whether a bounded additive native contract is appropriate and obtain any required scope authorization before implementing broader compatibility work. No such dependency has been resolved here.

## Base source, native and publication assessment

`main` dispatches `--gui-disposable-inventory` before ordinary flags/default paths/configuration/application initialization. The only package initializer found is the existing in-memory hash-acceleration default. The producer does not call App, OpenStore, ScanFolder, registration, media metadata extraction, jobs or services. The extracted `hashReaderBoth` retains the previous SHA-256/BLAKE3 streaming core; producer code owns read-only opens, size/identity observations, bounded reads and close handling. Directly affected shared-hash/scanner tests passed.

Output uses authoritative schema-8 catalog/Collection/Folder/File types. Each path receives its own positive file record ID; equal content does not collapse records, and duplicate basenames retain distinct hashes/selection. Folder.Path and RelPath locate the observation as recorded strings; no Volume/Location/chunk/copy, capacity, availability, parity, successful backup or physical-copy count is fabricated. Native IDs remain exact decimal strings at the browser protocol boundary. Observed size/mtime and real SHA-256 match independent fixture values; FirstSeen/CreatedAt describe the run. BLAKE3 remains catalog-only. The accepted reader's 4 MiB, row/string/relationship and advanced-section refusal limits are unchanged.

The selected source and output parent must exist as ordinary disjoint directories. Absolute-path policy, component Lstat, lexical Rel containment and actual ancestor SameFile checks precede enumeration/staging. Windows fixed-drive and reparse rejection are explicit; generated native symlink and all three junction tests executed without skips or privilege changes. Native special-device/UNC opening was not performed. The non-Windows stub does not establish local mount/bind-alias or raw-byte-name qualification.

Limits remain 64 files, 128 total entries, depth 8, 512-byte relative and 4096-byte absolute paths, 8 MiB/file and 32 MiB observed total. The stream can read one extra growth-detection byte before refusal; requests are capped at 64 KiB. The 30-second deadline is cooperative, not a hard interruption of a blocked kernel call. Enumeration/read/stat failures and observed identity/size/mtime/entry changes refuse publication rather than yielding a successful partial/empty catalog. At-limit and exceeded-limit tests use the actual enforcement code with modest per-invocation bounds; no scale claim is made.

After complete construction, reader validation and source rechecks, CreateTemp writes only in separate output storage; Write, short-write, Sync and Close errors precede publication. `os.Link(stageName, output)` is the actual no-replace publication operation and successful-publication point. The real link operation refused a deterministically late file and a late directory. Existing valid/damaged files, directories, symbolic links and dangling links survived the exercised refusals. No delete/replace fallback exists. Only owned staging is removed. After successful link, cleanup/status errors acknowledge Published=true and preserve the final catalog.

Candidate tests cover injected read/stat/enumeration errors, progress/status failures, pre/late cancellation, source changes, partial write, link failure and pre/post-publication cleanup failure. Reviewer supplements add output-kind cases, a stage refusal, explicit write error and a closed staging descriptor causing real Sync failure. That last probe is descriptor fault injection, not physical flush failure; separate close-only failure injection and native ACL failure are NOT RUN. Hard-link capability is required for this output publication method only. No directory-sync/power-loss guarantee, atomic source snapshot, hostile concurrent mutation confinement or universal backend prerequisite is inferred.

## Reviewer executions and evidence

Installed tools/process configuration: Go 1.26.8 windows/amd64 from the existing audit toolchain/module cache; Node 24.19.0; Chrome 153.0.8010.48. Go used GOTOOLCHAIN=local, GOPROXY=off, GOFLAGS=-mod=readonly, CGO_ENABLED=0 and reviewer-owned GOCACHE/TEMP/TMP. GOTMPDIR was unset. No installation, dependency upgrade, global setting change, WSL, remote CI or network workaround occurred.

| Reviewer execution | Actual result |
|---|---|
| Existing candidate/native/hash/scanner/reader selection | 17 top-level passes, 47 passing subtests, 0 failures, 0 skips |
| Supplemental native probes in disposable copy | 3 top-level passes, 13 passing subtests, 0 failures, 0 skips; includes positive reproduction diagnostics, not compliance passes for F1 |
| Fresh producer/reader and existing fault-double builds; vet | Exits 0 |
| Read-only gofmt and whitespace | No formatting/whitespace findings; Git conversion advisories retained separately from findings |
| Node stop-command / preview / catalog / catalog-correction groups | 5 / 7 / 4 / 8 passes, no failures/cancellations/skips |
| Direct CLI refusal cases | 8 deliberate refusals, each exit 1; absent/excess arguments, relative paths, missing/wrong-kind source, overlap |
| Direct encoding probes | 3 reader exits 0: two reproduced malformed-name replacements and one valid-literal positive control |

`R/native.ps1` enumerates and executes `^Test(GUIInventory|HashReaderBothRefusesPartialError|HashFileBothMatchesSHA256|ScanRecordsBlake3|ScanFolder_|GUICatalog(NativeDecodeAndSearch|RefusedLoads|ReadLifecycle|ExactIntegers))` with `go test -timeout 120s -count=1 -json -run ... .`, builds R/output/inventory-reader.exe and reader-double.exe, runs vet and read-only formatting. `R/supplemental.ps1` enumerates `^TestReviewer` then runs it with timeout 90s in the copy. `R/node.ps1` records the four actual suite names and uses `node --test --test-isolation=none`. Exact scripts, selected-test lists, terminal events and exits are retained; source inspection is not mislabeled execution.

`R/setup-and-produce.mjs`, adapted by location from retained author setup, creates NEW ALPHA/BETA/EMPTY and a sibling sentinel. It specifies expected contents/hashes independently before invoking the producer; neither producer nor viewer uses the setup oracle as inventory input. Both five-file sources contain an empty file, nested spaces/Unicode, equal-content distinct paths, and same basenames with different contents. The original author fixtures are not scanned or altered. Twenty-six accepted control inputs were copied into R before Node/browser use.

`R/names.mjs` supplies a separate reviewer-designed 26-file spelling corpus and records exact enumeration. Accents, non-Latin text, emoji, apostrophes, brackets/parentheses, &, +, %, #, leading hyphens, spaces and nested paths pass persisted native/hash/size/mtime and real browser search/selection checks. Source and output arguments themselves exercise punctuation, with Unicode also in the source root. Exclusive creation established that composed/decomposed accented names coexist and remain separate records; the attempted case-only pair returned EEXIST, leaving the first bytes intact. No filesystem settings were changed to force a case-sensitive pair. Valid literal U+FFFD is preserved. Windows-impossible control/markup/backslash components were tested only as foreign recorded data, never claimed as native filesystem creation.

Fourteen real browser sessions were attempted, with the following distinct results:

| Session | Result / completed checks |
|---|---|
| ALPHA / BETA | PASS, 11 / 11 |
| Empty / static | PASS, 3 / 55 |
| Query failure / exact large IDs | PASS, 4 / 12 |
| ALPHA source unavailable / changed-source NEW alpha2 | PASS, 11 / 11 |
| Names / names reopen with recorded source unavailable | PASS, 32 / 32 |
| Catalog disclosures and Back/Forward | PASS, 6 |
| Foreign full-name search | FAILED expectation at newline name after 4 checks; F2 |
| First foreign diagnostic | INCOMPLETE after 10 checks: reviewer JavaScript string-escaping error, not a candidate defect |
| Corrected foreign diagnostic v2, new evidence directory | PASS, 13; records exact HTTP/DOM behavior, newline input sanitization and inert markup |

Thus 12 sessions completed successfully, one reproduced candidate expectation failed, and one reviewer diagnostic failed. Sessions are not assertions or Node tests; partial checks are not counted as completed passes. All sessions retained results and stopped/waited browser and catalog processes. Browser termination by the inherited harness is forced and waited, not a natural console-stop qualification.

The failed first diagnostic and original helper remain preserved; only a new external v2 helper fixed string construction. An initial Node summary mistakenly read PowerShell UTF-16 Go logs as UTF-8 and reported zero parsed events. R/output/native-counts-verified.json parses the actual encoding and records 17/47 and 3/13 from terminal events. That bookkeeping correction did not rerun native tests or change candidate bytes. Earlier unsuccessful rg glob/ancestor lookups were followed by explicit paths; they are not runtime results.

The generated-record checks cover Library/Find, each selected record's own source/path/bytes/hash/native ID, no-match clearing and real keyboard skip activation without resetting state. Supplementary catalog checks exercise Guided/Studio/Expert rerendering plus Back/Forward with the same selection/evidence and no reload. Accepted Node controls cover failed bootstrap/query/protocol behavior, unsafe/native-maximum IDs, loopback/asset/API boundaries and whole/split stop parsing with stdin held open. No silent static fallback was observed. The existing catalog-only notice remains truthful for generated test snapshots; it does not disclose a nonexistent exclusion policy.

`R/reopen-change.mjs` makes only reviewer ALPHA temporarily unavailable, opens its persisted catalog in a fresh browser, restores the source, refuses an existing valid output, then explicitly changes one setup file between completed runs and creates a different NEW alpha2 snapshot. It preserves alpha.json. `R/names-reopen.mjs` separately makes only the spelling-corpus source unavailable, opens the names snapshot in a fresh browser, preserves its bytes and restores the source. These controlled unavailable sources and code inspection support the no-source-reopen boundary; final hashes alone are not claimed to prove no reads/transient writes.

Six additional routes reject inventory/source/output/helper access. A real generated-catalog CLI session accepted `st` then `op\n`, with HTTP still available after the prefix and stdin open through natural exit: server exit 0, reader exit 0, listener closed, 208 ms, no forced cleanup. Exact evidence is R/output/reopen-change-isolation-stop.json. Accepted fault controls separately exercise forced reader cleanup. Interactive-console Ctrl+C, native ACL, kernel-wide I/O monitoring, other-platform runtime, race, Docker, scale, power-loss and media tests remain NOT RUN.

Screenshots and result JSON are under R/output/browser-*. Catalog viewport is 1440x1024 CSS at scale 2 (2880x2048 PNG); static additionally runs its narrow smoke viewport. The inherited JSON lists both viewport descriptions even for catalog-only sessions; this is not proof both ran. Visually inspected examples: browser-names/inventory-record-8.png and browser-foreign-diagnostic-v2/data-record-2.png, showing literal punctuation and inert markup with selected evidence. No new layout or Figma-fidelity claim is made.

Historical catalog recheck 18/25 with one scale skip, historical 16 sessions, author final inventory 10/28 and overlapping earlier runs, author Node 5/7/4/8 and nine sessions remain historical. They are not added to reviewer counts. No Prompt 14A execution was discovered or invented.

## Checkpoint, preservation and one follow-up

Fresh reviewer binary identity, complete reviewed working-byte identities, final fixture/catalog/control hashes, source-change accounting, author evidence revalidation and Git/process state are retained in R/final-preservation.json, binary-identities.json, final-current-identities.json and evidence-identities.json. The report has a separate identity to avoid a self-hash cycle. Build caches, temporary test directories and browser profiles are excluded from the durable evidence manifest, not presented as reviewed artifacts.

Final reconciliation preserves all 278 pre-existing repository entries byte-for-byte, including candidate tests, living records, historical reports, original Figma/PDF and ignored ZIP. This report is the sole new repository path (279 total measured entries); HEAD, branch and empty index remain unchanged. Candidate-copy supplemental helpers and all generated inputs/outputs/screenshots/logs remain outside the repository. Source setup's one intentional ALPHA content change and temporary unavailable-source renames are distinct from producer behavior. No task-owned listener/reader/browser remains at handoff; processes were stopped and waited. No historical evidence, catalog, design or production source was used as a destructive probe.

From the repository root, the retained base-review snapshot can be browsed with:

```powershell
$review = 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-inventory-review-20260920-160314'
node scripts/gui-preview/server.mjs 0 --catalog "$review\snapshots\alpha2.json" --adapter "$review\output\inventory-reader.exe"
# Open the printed loopback URL. Type stop, press Enter, and wait.
```

Do not rerun exclusive fixture-generation/probe scripts in this evidence directory or reuse an existing output name. The report and evidence preserve reproductions for a new separately authorized correction/recheck directory.

ONE proposed next action: authorize one complete correction pass addressing F1, F2 and S1 together, followed by a targeted reviewer recheck of those changes and directly affected base/read-only/lifecycle controls. Missing native platform availability is a qualification limit, not by itself a Windows defect. Durable-scope compatibility must be made explicit rather than silently implemented as an unrelated migration. No correction is performed or authorized by this review. After eventual acceptance, publication still requires separate destination-specific authorization.

Generation-independent preservation/buffering remains the direction; LTO-8 is the first physical qualification target, not a generation limit; other backends/generations require explicit qualification and Blu-ray remains separate. This review establishes no production registration/inventory, storage/media operation, parity, copy/restore, tar repair, main integration or release approval.
