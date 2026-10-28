# PR-04 slice A — bounded F1–F7 review response, 2026-09-08

**Status: BLOCKED on the F5 Unix ownership/threat-model decision.** Windows repairs
and regressions are implemented and exercised. Unix production repairs and tests
are prepared and cross-compiled, but have not run on Unix. This is not independent
approval, not closure of OB-004, and not authorization to publish or start slice B.

Controlling findings retain their original numbering from
[the preserved Codex review](PR04-OB-004-SLICE-A-CODEX-REVIEW-2026-09-08.md).
The September 7 implementation and prerequisite reports remain unchanged history.

## Candidate and preservation

- Branch: `fix/ob-004-source-boundaries`.
- HEAD and overall PR-04 base: `0dc7d5399c6889e015aebc9ba02e694a909d6e9c`.
- Entry: 12 tracked modifications, six untracked slice-A Go source/test files,
  additional untracked documentation, empty index, no unmerged entries or ongoing
  Git operation. No applicable `AGENTS.md` or `CLAUDE.md` was found in the checkout
  or checked ancestors. Read the handoff, implementation report and full review.
- Before editing, recorded SHA-256 and byte length of **491** tracked/untracked
  entry files and copied their exact bytes outside the repository. The source/test
  entry hashes match the review's listed identities. All 491 saved originals were
  rehashed afterward with **zero mismatches**. Auxiliary probe files were added
  alongside the saved snapshot; no saved original was replaced.
- No fetch, pull, branch switch, reset, stash, clean, stage, commit, push, merge,
  ref rewrite or prior-report rewrite. The index and HEAD remain unchanged.
- Final preservation check: **481 of 491** entry files remain byte-identical in
  the checkout; the ten changed originals are exactly the authorized source,
  dependency, architecture and current-handoff/status files. All 491 saved
  pre-edit copies still match. The old NEXT_ACTIONS bytes remain an exact prefix
  before the appended repair note. See `final-preservation.json`.

Evidence root (temporary storage, outside the checkout):

```text
C:\Users\NATHAN~1\AppData\Local\Temp\obelisk-pr04-repair-20260908-7ebc402cac2e4fb399084b300514a718\
  entry-manifest.json             identities of all 491 entry files
  pre-repair\                    exact entry copies, plus auxiliary test harnesses
  base.zip, base\                git archive of the explicit PR-04 base
  repair-identities.json          before/after source, dependency and test hashes
  pre-repair-list.txt             executed probe enumeration
  pre-repair-probes.txt           five original failing probes plus the stronger F6 test
  base-f6-list.txt, base-f6.txt    identical F6 test against the base
  targeted-corrected.txt          39 pass / 2 skip
  earlier-safety-list.txt         earlier safety-test enumeration
  earlier-safety.txt              45 pass / 1 opt-in scale skip
  sidecar-production-final.txt    production WriteFileAtomic metadata/failure test
  full-test-list.txt              279 enumerated top-level tests
  full-suite.txt                  fresh uncached suite, 266 pass / 7 fail / 6 skip
```

The original reviewer harness was copied from its retained scratch directory and
executed unchanged against the saved pre-repair slice. It reproduces **five**
top-level failures, F1/F2/F3/F5/F6; both F2 workflow subtests fail. **F4 and F7
remain source-derived Unix defects, not pre-repair runtime reproductions.**

## F1–F7 implementation and regression map

| Finding | Production repair and preserved contract | Regression and actual result |
|---|---|---|
| **F1 — filesystem-root overlap** | `destauth.go`: `sourceSnapshot.decide` uses `canonicalWithin`, based on `filepath.Rel` over normalized comparison keys. Equality, inside and containing relationships handle volume roots without doubling separators. Stored paths are not comparison keys. | `TestDestAuth_F1FilesystemRootOverlap` **PASS Windows**. Real root authorization is read-only: it never requests a write at a system/volume root. A whole-volume source refuses equality, existing descendants and missing descendants. Disposable safe siblings and shared-prefix siblings succeed. Sentinel bytes, pre-mutation identity and membership survive. Unix root behavior is prepared, unexecuted. |
| **F2 — filename collision/false coverage** | `destRel` and `DestRoot.MkdirAll` preserve exact names instead of trimming them. The user-entered destination-root string retains its separate input cleanup. Windows trailing-space/trailing-dot components, alternate data streams and reserved/nonlocal names are explicitly refused rather than silently mapped. Whitespace-only names remain an explicit unsupported case. No application-wide normalization change. | `TestDestAuth_F2ExactWhitespaceNames` **PASS**, both full and incremental mirror. `" a.txt"` and `"a.txt"` have different contents and destination identities; actual directory-entry names, both source identities, both stored contents, two catalog references with correct hashes/sizes, and 2/2 coverage agree. The pre-repair reviewer probe reports two successes/100% coverage despite a missing leading-space file. `TestDestAuth_F2UnixSpaceEndingDirectory` is compiled, **NOT EXECUTED**. |
| **F3 — absent source loses alias ancestry** | `Store.sourceSnapshot` canonicalizes the deepest existing ancestor by its open handle and appends **all** missing descendant components. `deepestExistingDir` uses `Lstat` to distinguish a present dangling entry from an absent component, then `Stat` to resolve existing Unix links and Windows junctions. Unresolved authority refuses with an instruction to restore parent access/repair the alias, not remove registration. | `TestDestAuth_F3MissingSourceUnderJunction` **PASS**. A registered absent two-component descendant protects equality, its ancestor and its descendants through the real spelling; none is created. A safe sibling succeeds without unregistering the source. Original sentinel bytes/identity and membership are preserved. The original one-component reviewer probe reproduces the pre-repair creation. Unix symlink counterpart is compiled, **NOT EXECUTED**. |
| **F4 — unsafe Unix bootstrap** | `AuthorizeDestination` no longer calls broad-ancestor `MkdirAll` on the missing suffix. It performs a single-component `Root.Mkdir`, treats an existing entry as refusal, opens the child, and authorizes its actual location before any next creation. A dangling link is refused before creation. The broad first ancestor authorizes only the absent disjoint child, not mutations through sibling subtrees. No ordinary absolute-path mutation follows `EvalSymlinks`. | `TestDestAuth_F4DanglingBootstrap` and `TestDestAuth_F4BootstrapComponentSubstitution` compile for Linux/macOS/FreeBSD, **NOT EXECUTED**. Assertions preserve protected membership, sentinel bytes and pre-mutation identity after refusal; deterministic barriers introduce a link immediately before or immediately after the first mkdir. The ordinary missing sibling remains a positive control. Source inspection establishes the pre-repair defect; cross-builds do not turn it into runtime evidence. |
| **F5 — staging substitution, metadata and cleanup** | `DestRoot` retains stage objects; stream/read-back and sidecar chmod use their open files. `deststage_windows.go` creates an exclusive read/write/delete handle, renames that handle into a held parent, and deletes failed staging by handle disposition. `deststage_unix.go` uses a held mode-0700 private directory plus `renameat`; see the **unsatisfied same-UID requirement below**. No new digest is accepted from substituted bytes. | `TestDestAuth_F5PublicationSubstitution` **PASS Windows**, publish and injected-failure branches: the exact pre-publication replacement attempt receives sharing violation 32. It proves **platform refusal of substitution**, not a successful swap. Success publishes verified bytes with coverage; failure preserves prior bytes/identity and gives no catalog credit. `TestDestAuth_F5SidecarMetadataAndCleanup` **PASS**, using real `WriteFileAtomic`, a barrier before chmod, and failure after metadata. The attempted hard-link substitution was refused; original mode/bytes/identity survive. Unix tests of a replaced public container and an intruded private entry compile but are **NOT EXECUTED**. |
| **F6 — unconverted wrapper truncation** | `mirrorCopyFile` opens the source before `os.Create(dst)` again. `mirrorCopyStream` takes open files and leaves their lifetime to its caller; destination write/sync errors are no longer mislabeled as source reads. The retained wrapper still uses its old truncating destination writer after source-open success. | `TestMirror_F6SourceOpenFailurePreservesHardlink`: **base PASS → pre-repair FAIL → repaired PASS**, same compiler and same test logic. Both original and staging-link identities are captured from handles before mutation; both names must retain `ORIGINAL` and their identity after the missing-source error. The pre-repair run empties both names. No legacy writer migration. |
| **F7 — portable validation/test mismatch** | `destRel` explicitly rejects an ASCII drive letter followed by `:` on every host. The validator no longer depends solely on the host-specific `filepath.VolumeName`. Existing valid refusal assertions remain. | `TestDestAuth_F7PortableDriveNames`, existing `TestDestAuth_RelPathValidation`, and the sidecar drive-name assertion **PASS Windows**. `C:/x`, `C:\x`, `c:relative` and UNC qualification are refused; no local `C:` tree is created. The same tests compile on Unix; **Unix runtime gate remains outstanding**. |

All new mutation fixtures are disposable `t.TempDir` trees outside the checkout.
Original identity is captured before mutations from open handles, not lazy Windows
pathname-derived `FileInfo` identity. Deterministic callbacks are used, not sleeps.
The existing full suite includes its read-only system-disk identity query; no real
original, application catalog, key, backup contents or archival medium was opened.

## Current publication guarantee and the unsatisfied F5 decision

The lifecycle is now: exclusive creation → write/hash source bytes → metadata on
the same file when needed → sync → seek/read-back through the retained file →
platform publication → close → catalog credit. Only the original streamed digest
is eligible for payload credit. No earlier staging pathname is reopened for its
verification. A failed rename never triggers a delete-destination fallback. A
close error **after** successful rename can withhold credit while leaving the new
published file present; it is not described as preservation of the prior version.

**Windows:** `NtCreateFile(FILE_CREATE)` receives read/write/delete access with
no write/delete sharing (currently no sharing at all), relative to the parent
opened through `os.Root`. `NtSetInformationFile(FileRenameInformation)` names the
new single leaf relative to that parent handle and acts on the original open file.
Failure cleanup uses `FileDispositionInformation` on the same handle. Sidecar
mode changes use `File.Chmod` before closing. The Windows tests exercised ordinary
replacement, nested parents, destination hard links, failure preservation, and
sharing-mode refusal of the reviewer substitution. Filesystem support errors are
refusals; there is no fallback to pathname publication or truncation. The native
handle/name relationship is described by Microsoft's
[rename structure](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info).

**Linux:** destination/source location still comes from `/proc/self/fd`, identity
from `fstat`; unavailable or removed-object resolution refuses. Bootstrap uses
held `os.Root` parents and single-component mkdir as above. Publication uses two
retained directory descriptors and a fixed private source entry.

**macOS/BSD:** location retains the prior weaker `EvalSymlinks` plus open-descriptor
identity comparison; no native `F_GETPATH` claim is made. Mutations use held
parents, not a checked absolute pathname. The staging rename is the same
descriptor-relative Unix mechanism. Linux `/proc` behavior, macOS/BSD behavior,
and Windows junction results are separate evidence categories.

**Unix F5 is not fully satisfied under a hostile same-account writer.** A mode-0700
private directory prevents other accounts with access to the public destination
from changing its interior. A held directory plus `renameat` can survive a public
directory-name substitution; [POSIX specifies descriptor-relative operands](https://pubs.opengroup.org/onlinepubs/9799919799/functions/rename.html).
But a same-UID process can still alter the private entry between the retained-file
identity check and rename/cleanup, or directly change its bytes. The new identity
check catches the deterministic intrusion; **it does not close that race** and is
not presented as doing so. The private namespace's creation/open also assumes no
hostile same-UID process. Windows file-sharing enforcement is not a Unix primitive.

**Decision required:** explicitly accept trusted same-UID processes for the Unix
private-staging contract, or require a stronger platform/process isolation design
that protects the staging namespace from same-UID adversaries. This repair does
not silently approve the narrower assumption or start a general ownership project.
Pending that decision, the candidate remains **BLOCKED**, even with passing
Windows results. Arbitrary actors able to modify the same objects, privileged
topology changes and post-publication edits are not universally protected.

Unix cleanup deliberately retains empty `.obelisk-stage` container directories:
deleting their public names could delete a substituted unrelated directory. Failed
private-entry ownership checks retain unknown entries for inspection. This behavior
is visible in code and is another cost to consider with the Unix decision. Ordinary
safe Unix destinations remain implemented; the platform is not disabled to pass tests.

Unchanged limits: authorization uses one source-registration snapshot; later
registration or relocation of protected trees is outside it. Case-insensitive
macOS spellings remain a recorded gap. No medium/session identity, external-helper
confinement, privileged mount/device protection or crash-directory durability is
claimed. Sidecar failure remains best-effort in both workflows as before.

## Narrow dependency and code scope

`golang.org/x/sys v0.44.0` is the one added module, specifically for F5's native
Windows handle operations and portable Unix `renameat` (including macOS/BSD
bindings absent from the standard `syscall` surface). Its source and checksum
record were already cached. The offline `go mod download` lookup failed; the two
checksums were taken from the existing checksum-database record, and subsequent
`-mod=readonly` builds/tests used that cached module. No dependency version was
invented or checksum verification disabled. No tools were installed.

The **Go 1.26.8 directive and all approved compiler/Docker/workflow pins are
unchanged**. No machine settings, `go env -w`, global PATH, WSL or administrator
elevation. Tool-level cache access was approved after sandbox failures before
test execution; those failures are not classified as safety passes.

Changed existing source/tests: `destauth.go`, `destauth_other.go` (comments),
`destauth_test.go`, `destauth_windows_test.go`, `mirror.go`. Added:
`deststage_windows.go`, `deststage_unix.go`, `destauth_repair_test.go`,
`destauth_unix_test.go`, `mirror_source_order_test.go`. `go.mod`/`go.sum` record the
one dependency. `docs/ARCHITECTURE.md` records its purpose; this report, handoff
and the narrow NEXT_ACTIONS addendum describe the actual guarantee and blocker.
Earlier slice edits in `incremental.go`, `pipeline.go`, `store.go`,
`verify_levels.go`, build files and prior reports are preserved byte-for-byte.

## Repair source identities

SHA-256 of working-file bytes (independent of Git line-ending conversion). The
complete entry inventory and pre-edit bytes remain in the evidence root.

| File | Pre-repair SHA-256 | Repaired SHA-256 |
|---|---|---|
| `destauth.go` | `F38A741CB7EE08EFEF1270908FED3F652401048882BBE880F17F5798BC4C4745` | `28727C3253117A3F8E682A4D273FC682E7D9607C4A1A5A051C9A6A28C86E9080` |
| `destauth_other.go` | `E3AA48F3609E0EBF3190E865ACA691FF641030D3F6BCE6D5875E79ADBFC38CA5` | `F21BE23222AD0AA1C3D1057308B7081695C3DEFAA126214CABE6AD0439AADF29` |
| `destauth_test.go` | `72F739F232BBFBD0DD26295B7DB803B4780D0F4F263CF8A53D582D6A771FB413` | `76D4B6C6932F3EBF2FF6881FA8045D9AE0E3A18759F40F9732188C33898E3E3B` |
| `destauth_windows_test.go` | `C8A7BEF05124AAFEC911DC1DEDC648AC89B4C300A652B85A55EF1D2DCE380783` | `FBF8E9EB0CF366BF054686A8E572C3FC5325F1E719D0720DFA3400966018436F` |
| `mirror.go` | `FD331C82941366CFB2BD7754DE2D69A4D86BC4639C55C8DF2AA82BE59DB2B547` | `C6FADF3A5B9D3B6C02965B97949182085CA15C467583334A120C701F6F6D9BE2` |
| `go.mod` | `035D8695F60419AE26E13BEFB4F4FBF3D706BC5DB7B6542FFF907D9F2253DB1A` | `B7A733BBE3803FFE7489CDDE694CCBE806F06F076ED149DF71062C7903749E6E` |
| `go.sum` | `9D169AD00514AD13D9A10E1B1AC4723171C020A23B792C76721D97CCD47A1993` | `D43968AF1F5612470AC9998069F1C879DBFF6C1B307946B90089C225896381C6` |
| `deststage_windows.go` | new file | `FDBEA58C896A3E8D7AF048E84B065546DEFA6B76A10C1EDABD17D1C689EEEFD6` |
| `deststage_unix.go` | new file | `4206D9F22CDD673BAA1229172B9C8370F252F3473DDFD487078CD19311CEDFA3` |
| `destauth_repair_test.go` | new file | `1C7D19AD801001FD43A0D3C278B7D3B8DA519D6060E040D6D8B1CE2CFF082078` |
| `destauth_unix_test.go` | new file | `BA89CD0B40DE9B9386EA83BFE5543AC0AB8E547C119D317C73C83C8EE3A40850` |
| `mirror_source_order_test.go` | new file | `B846D7055CF3A556CCDBBF8D7EEF0B57B92DB88ABB51725AF2EEBE1056D7140C` |

## Deferred callers and F6 boundary

The thirteen mutation callers still on the old pathname guard are:

1. `ExecutePlanFromDrive` (`plans.go`)
2. `WriteChunk` (`writer.go`)
3. `RestoreChunk` (`writer.go`)
4. `SpanWriteNext` (`span.go`)
5. `BuildChunk` (`pipeline.go`)
6. `ExportAppBackup` (`appbackup.go`)
7. `maybeAutoExport` (`appbackup.go`)
8. `exportBagForChunks` (`bagit.go`)
9. `WriteEscrowBundle` (`escrow.go`)
10. `BuildRecoveryKit` (`recoverykit.go`)
11. `FinalizeVolume` (`finalize.go`)
12. `writeFinalizeSidecar` (`finalize.go`)
13. `QuarantineTerritory` (`quarantine.go`)

`SaveConfig` validation and read-only `mirrorAdopt` are the other two functions in
the review's 17 guard invocations/15-function inventory; neither is counted as a
mutation caller here. Their authority migration remains deferred.

F6 repairs the **new source-open failure ordering**, not their pre-existing
successful-copy hard-link hazard. `ExecutePlanFromDrive` still makes parents by
pathname and removes its deterministic temporary name on an error; that cleanup
predates the slice and may unlink that staging name. The direct-wrapper F6 test
claims preservation of both names; it does not falsely extend that claim to plan
cleanup. The protected original's content/identity is no longer truncated merely
because the wrapper cannot open the source. The wrapper has no metadata mutation;
streaming now borrows file handles and the wrapper explicitly closes its output.
`copyFile` and the other old writers were not converted. PR-02's six regressions
still pass, including its legal platform fixtures; Unicode tests were not edited.

## Executed validation and outstanding platform gates

Every Go command selected `GOTOOLCHAIN=go1.26.8`, `GOPROXY=off`. Current compiler:
`go1.26.8 windows/amd64`, `CGO_ENABLED=0`; executable SHA-256:
`21761ECEB9302062C9623FB699F332C8C7FE000F15F70EFE8DA01A2CFBBC16B9`.

Tests were enumerated before execution, rather than accepting a zero-match exit.

```powershell
# Full name lists/results are retained under the evidence root.
go test -mod=readonly -list '^(TestDestAuth_|TestMirror_|TestIncremental_|TestAtomicRename_)' .
go test -mod=readonly -count=1 -run '^(TestDestAuth_|TestMirror_|TestIncremental_|TestAtomicRename_)' -v .
# Earlier safety selection, likewise enumerated before running:
go test -mod=readonly -count=1 -run '^(TestOpenStore_|TestCatalog|TestPersistObserver_|TestWriteCatalog_|TestAppendVerifyEventErr_|TestBuildChunk_TerminalStageWriteIsDurabilityGate|TestDurableCompletion_|TestJobsUI_|TestContainment_)' -v .
go test -mod=readonly -list '^Test' ./...
go test -mod=readonly -count=1 -v ./...
go build -mod=readonly -o <scratch>/obelisk-repaired-windows.exe ./...
go vet -mod=readonly ./...
```

| Actual execution | Result |
|---|---|
| Original pre-repair reviewer probes | **5 FAIL** as expected, F1/F2/F3/F5/F6 |
| Additional identical F6 test | **base PASS / pre-repair FAIL / repaired PASS** |
| Repaired boundary + mirror/incremental + PR-02 | **39 PASS / 2 SKIP**, no failures |
| Earlier catalog/PR-01, completion/PR-03, UI and OBX-006 containment | **45 PASS / 1 opt-in scale SKIP**, no failures |
| Final production-sidecar metadata/cleanup regression | **PASS** |
| Fresh full Windows suite, 279 top-level tests, 108.223 s | **266 PASS / 7 FAIL / 6 SKIP**, exit 1 |
| Windows build and vet; `go mod verify` | **PASS**, all cached modules verified |
| Selected Go 1.26.8 gofmt on changed/new Go files; `git diff --check` | **Clean** |
| Linux amd64, macOS arm64, FreeBSD amd64 build, test-binary compile and vet | **PASS — compilation/static checks only** |

The first repaired Windows selection exposed a junction positive-control failure:
Go's junction `Lstat` result need not carry `ModeSymlink`. Resolving every existing
entry after `Lstat` corrected it. The rerun and final full suite pass those cases;
the first failed run is retained as `targeted-first.txt`.

The fresh full-suite result is distinct from the old **259/7/6** report. The seven
failures remain exactly:

```text
TestBuildRestore_HostileFilenamesRoundTrip
TestBuildFilelist_IsNulDelimited
TestBuildChunk_UnicodeFilenames_ExactMembers
TestBuildChunk_WrongFileSelection_LookalikeNeighbour
TestBuildChunk_UnicodeSourceRoot
TestBuildChunk_UnicodeStagingDir
TestBuildRestore_UnicodeRoundTrip
```

Six skips: `TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`,
`TestDestAuth_SymlinkAliasOfSourceRefused`, `TestDestAuth_NestedSymlinkLeafRefused`,
`TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`. The two symbolic-link
tests explicitly report missing Windows privilege. Junctions, hard links, extended
prefixes and short names actually executed. Root-directory substitution was
attempted but Windows refused the rename; its successful-substitution branch
remains untested. The F5 hard-link replacement was also **attempted but refused**;
it is not reported as an exercised metadata attack against a substituted object.

**No Unix runtime, CI, Docker or Windows race check executed.** No available Unix
runtime was selected and no WSL/tool installation was attempted. Cross-built test
binaries are artifacts for the intended runtime, not passing tests. Existing CI
configuration remains unexecuted and unchanged by this repair. After the Unix F5
decision, run the enumerated boundary selection and full suite on Linux and
macOS/BSD with Go 1.26.8, including the F3 symlink, F4 bootstrap, F5 private-container
and F7 validation cases. Keep Windows race and unsupported link mechanisms visible.

**Stop state: BLOCKED on F5's Unix security assumption/isolation decision; repaired
Windows candidate and reproducible evidence retained for focused review. Nothing
staged, committed or published.**
