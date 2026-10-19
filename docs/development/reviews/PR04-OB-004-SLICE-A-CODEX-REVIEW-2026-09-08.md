# PR-04 / OB-004 slice A — independent Codex review, 2026-09-08

**Outcome: NEEDS_CHANGES.** The existing Windows selection passes, but additional
disposable probes expose source-authorization failures and data-loss regressions.
F1–F7 below concern the implemented slice or behavior changed by its extraction;
accepting slice A does **not** require completing OB-004 or starting slice B.

## Checkout and preservation

| Item | Independently observed |
|---|---|
| Project root | `C:\Users\Nathaniel\Documents\Software Development\Mnemosyne\mnemo-go` |
| Branch | `fix/ob-004-source-boundaries` |
| HEAD / comparison base | `0dc7d5399c6889e015aebc9ba02e694a909d6e9c` |
| Index | No staged changes; no unmerged entries |
| Tracked patch | 12 modified files; 320 insertions / 88 deletions against the recorded base |
| Untracked at review entry | 281 files: 6 slice-A Go files and 275 documentation/bundle files |
| Instructions | No `AGENTS.md` or `CLAUDE.md` found, including imported directories and checked ancestor directories; `.claude/settings.local.json` is an old permission allowlist, not authorization to execute its commands |
| Handoff | No pre-existing `docs/development/CODEX_HANDOFF.md`; created alongside this report |

The root, branch, HEAD and implementation inventory match the reported checkpoint.
The current `OB_STATUS.md` still describes OB-004 at the old baseline; the latest
PR-04 progress is appended to `NEXT_ACTIONS.md` and recorded in the two September 7
PR-04 reports. Those earlier reports remain historical evidence, including their
superseded compiler-decision prompt. They were not treated as active instructions.

Reviewed the complete tracked diff against the base, all six new Go files (including
both test files), and the relevant surrounding callers and Go 1.26.8 library code.
No production, test, build, dependency or prior documentation file was changed.
SHA-256 comparison of all 489 entry files found zero changes before writing these
two review/handoff documents. Final preservation verification is recorded in the
handoff. Ignored existing binaries and local settings were not written.

### Existing uncommitted inventory

Modified tracked files, all unstaged:

```text
.github/workflows/ci.yml
.github/workflows/release.yml
Dockerfile
README.md
docs/CONTRIBUTING.md
docs/development/NEXT_ACTIONS.md
go.mod
incremental.go
mirror.go
pipeline.go
store.go
verify_levels.go
```

Untracked slice-A source/tests, read in full:

```text
destauth.go
destauth_linux.go
destauth_other.go
destauth_windows.go
destauth_test.go
destauth_windows_test.go
```

Other pre-existing untracked material, preserved:

| Path | Files |
|---|---:|
| `docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/` | 132 |
| `docs/OBELISK_POST_PR02_FIX_PROMPTS_2026-09-07/` | 8 |
| `docs/development/PR01_REVIEW_ADDENDUM/` | 132 |
| `docs/development/UI_FEATURE_MAP-2026-09-06.md` | 1 |
| `docs/development/reviews/PR04-OB-004-BOUNDARY-DECISION-2026-09-07.md` | 1 |
| `docs/development/reviews/PR04-OB-004-SLICE-A-IMPLEMENTATION-2026-09-07.md` | 1 |

This review adds only this file and `docs/development/CODEX_HANDOFF.md`.

## Findings requiring changes

### F1 — P1: filesystem roots bypass both overlap directions

**Anchor:** `destauth.go:188–191`, `sourceSnapshot.decide`.

`normPath` retains the trailing separator for `C:\` and `/`. Appending another
`/` makes the prefix `c://` or `//`, which does not match an ordinary descendant.
Consequently, a destination at a volume root can contain registered sources yet
be authorized; a registered whole-volume source also fails to protect descendants.
The new boundary inherits this edge case from the old prefix policy instead of
enforcing its stated equal/inside/ancestor contract.

**Executed:** `TestReview_FilesystemRootOverlapRefused` called the real
`AuthorizeDestination`. It authorized `C:\` despite a registered source in a
temporary directory, and authorized an existing temporary destination after `C:\`
was registered as a source. Both refusal assertions failed. The volume-root check
was authorization-only: no payload, sidecar or directory was written at `C:\`.
The analogous `/` case is source-observed, not Unix execution.

**Required:** make segment-boundary containment handle filesystem roots explicitly;
cover both directions with positive sibling controls. Whole-volume source exclusion
is source protection, not the deferred removable-media identity project.

### F2 — P1: validation silently merges distinct filenames

**Anchor:** `destauth.go:77`, also `DestRoot.MkdirAll` at `:380`.

`strings.TrimSpace(rel)` changes a catalog filename before publication. Leading
spaces are valid even on Windows. A single-folder source containing `" a.txt"`
and `"a.txt"` sends both payloads to `a.txt`; the catalog still records their
original names and hashes. Trimming directory paths also causes problems for valid
space-ending components. Validation must not silently rename archival content.

**Executed:** `TestReview_WhitespaceNamesRemainDistinct`, separately through full
mirror and incremental mirror. Both reported two successful files and zero failures,
but `" a.txt"` was missing. Full mirror reported **100% coverage** for the two
files. The same two workflow probes passed against pristine `0dc7d53`, with the
same Go 1.26.8 compiler. This is a new data-loss regression.

**Required:** preserve exact relative-name bytes during validation and parent
creation. If a name cannot be represented safely, refuse it explicitly without
recording a verified copy; do not normalize it onto another valid filename.

### F3 — P1: an absent registered source loses its existing ancestor's identity

**Anchor:** `destauth.go:157`, `Store.sourceSnapshot`.

On `ErrNotExist`, a registered source is stored only as `filepath.Clean(abs)`.
Its missing leaf may have an existing junction/symlink or short-name ancestor.
The destination uses a canonical ancestor but the protected source does not, so
two spellings of the intended same location compare unrelated. The comment that
an absent root cannot be aliased overlooks aliases in its existing ancestors.

**Executed:** `TestReview_MissingSourceUnderAliasRefused` created a junction
`alias -> real`, registered `alias/missing-source` while the leaf was absent, then
authorized `real/missing-source`. Authorization returned success **and created the
registered source directory**. No source registration changed during the operation.

**Required:** establish the intended location of an absent source through its
existing ancestors, or refuse when that location cannot be established. Do not
fall back to an incomparable spelling and allow creation.

### F4 — P1: bootstrap can mutate a source before the final refusal

**Anchor:** `destauth.go:264–290`, especially `anc.MkdirAll` at `:280`.

The code authorizes an *intended child name*, then creates through a broader
ancestor handle that can itself contain protected sources. `deepestExistingDir`
uses `os.Stat`, so a dangling link is treated as a missing component. Appending
that component to `ancCanon` does not prove that it is absent from the namespace.

**Source-traced Unix example, NOT EXECUTED:** `/fixture/protected` is a registered,
existing source; `/fixture/alias` is a relative symlink to `protected/missing`;
the requested destination is `/fixture/alias/child`. The deepest existing directory
is `/fixture`. The intended-name check allows `/fixture/alias/child`.
`Root.MkdirAll("alias/child")` follows the in-root relative link and can create
`protected/missing/child`. The final handle-derived check refuses it only **after
the protected tree has changed**. No race, bind mount or new registration is needed.

This trace follows the selected compiler's `os/root_openat.go` implementation:
`rootMkdirAll` creates missing components, and `doInRoot` substitutes relative
symlink targets. Root confinement is confinement to `/fixture`, not authorization
to mutate every subtree there. The [Go Root contract](https://pkg.go.dev/os#Root)
permits relative symlinks that remain within a root.

**Required:** make missing-root creation enforce source authorization before each
creation through the actual parent objects. Detect dangling links and safely handle
component substitution. A final check alone cannot undo forbidden writes. Add a
Unix regression with protected directory membership checked after refusal.

### F5 — P1: staging-name uniqueness does not retain staging-object ownership

**Anchors:** `destauth.go:443–470`, `:544–552`; `mirror.go:337–353`.

`O_EXCL` protects the initial create, and the random name avoids ordinary concurrent
job collisions. After the file closes, however, read-back, publication and cleanup
address the name again. They carry no retained file identity or protected staging
namespace. Most directly, the new sidecar path closes `f` before
`d.root.Chmod(tmpRel, perm)`: a replacement hard link at that name would receive
the metadata change, including when the shared object is a protected original.
Confining a name to the destination does not make a hard-linked object private.

**Executed bounded interleaving:** `TestReview_PublicationRetainsVerifiedStagingIdentity`
used the existing `rootRename` fault seam to rename the verified staging file aside
and create different bytes at its name immediately before the actual `Root.Rename`.
The real `MirrorToVolume` then reported one verified copy, zero failures and 100%
coverage while the published file contained `UNVERIFIED REPLACEMENT`. This is an
ordered filesystem substitution, not a timing-based race-detector run. It models a
writer able to replace entries inside the destination; it does not alter the
catalog or register/move a source root during the job.

The **chmod/hard-link and cleanup variants are source-observed**, not independently
executed races. `Discard` removes whichever object now has the name; the published
claim that it removes only the file this operation created is too strong.
Post-verification name substitution also existed in the old pathname design;
this finding is an unclosed staging-ownership claim in the new boundary, with a
new name-based metadata operation, not a claim that all of this hazard is new.

**Required:** apply metadata through the owned open file before closing it, and
retain or isolate staging authority through verification/publication/cleanup so
an entry substituted by another writer is not trusted as this operation's file.
Add ordered substitution checks. This is a bounded slice-A staging requirement,
not a demand for repository-wide OB-007 process locking.

### F6 — P1: extraction changes the unconverted wrapper's source-open failure

**Anchor:** `mirror.go:66–71`, reached by `plans.go:744`.

`mirrorCopyFile` now calls `os.Create(dst)` before `mirrorCopyStream` opens the
source. At the base, it opened the source first. A missing/unreadable source now
truncates an existing destination staging file before returning the source error.
If that staging name is a hard link to an original, the original loses its bytes
even though no source could be opened. This contradicts the implementation report's
claim that the unconverted wrapper is unchanged in behavior.

**Executed:** `TestReview_UnconvertedSourceOpenFailurePreservesHardlink` used a
missing source and a synthetic `.mnemo_tmp` hard link. Current code reduced the
original from `ORIGINAL` to an empty file. The identical probe passed against
pristine `0dc7d53` on Go 1.26.8.

**Required:** restore the prior source-open-before-destination-create failure
ordering in the retained wrapper. This does not require converting
`ExecutePlanFromDrive` or fixing its pre-existing successful-copy hard-link hazard.

### F7 — P2: the new portable validation tests conflict with Unix implementation

**Anchors:** `destauth.go:81–84`; `destauth_test.go:447`, `:598–602`.

`filepath.VolumeName` is host-specific. In the selected compiler's
`internal/filepathlite/path_unix.go:41–42`, `volumeNameLen` returns zero. On Unix,
`C:/x` therefore passes this validator, and `C:\x` is an ordinary single filename.
The portable tests unconditionally require rejection of both; the sidecar test
also requires `WriteFileAtomic("C:/absolute.md", ...)` to fail, although it can
create a local `C:` directory and file on Unix.

**Evidence: source inspection, NOT a Linux CI run.** These assertions are expected
to fail in the existing Linux `go test ./...` job on an ordinary writable fixture.
The Windows selection and successful cross-compilation cannot exercise them.

**Required:** implement the advertised host-independent drive-qualified-name
rejection, preserving the portable contract and its tests, then execute them on
a Unix host. Do not claim the new CI gate passed from YAML inspection or compilation.

## Scope and other observations

The converted callers are `MirrorToVolume`, `copyVerifyToRoot`,
`writeVolumeInventoryTo`, and **mirror-mode** `BackupChanges`, including its own
inventory call. Their destination parent creation, staging, read-back, sample,
rename and cleanup use the retained `os.Root`. Free-space queries use the canonical
name read-only; catalog/session writes are separate application-state effects.
The `hashReader` and `sampleHashOpen` extractions preserve their algorithms.

Existing guarded paths remain at the following current anchors. These are **not
required to migrate for slice A**; F6 only repairs a regression in their shared
wrapper.

```text
plans.go:682                 ExecutePlanFromDrive
writer.go:303,632            WriteChunk / RestoreChunk
span.go:134                  SpanWriteNext
pipeline.go:165,169,174      SaveConfig validation
pipeline.go:1027             BuildChunk
appbackup.go:257,520         ExportAppBackup / auto-backup
bagit.go:238                 exportBagForChunks
escrow.go:344                WriteEscrowBundle
recoverykit.go:38            BuildRecoveryKit
finalize.go:169,249          FinalizeVolume / writeFinalizeSidecar
dock.go:281                  mirrorAdopt (read-only policy check)
quarantine.go:140            QuarantineTerritory
```

This is 17 guard invocations in 15 caller functions, including the config validator
and read-only adopt policy; separating those two leaves the thirteen reported
mutation callers. `copyFile`'s old hard-link truncation remains deferred.

Other limits and observations:

- **Publication:** the direct `Root.Rename` has no delete-destination fallback.
  Ordinary replacement and injected publication-failure tests pass, preserving
  PR-02's prior-file guarantee for those exercised cases. This does not settle F5.
- **Sidecars:** both files are now staged through the same root. Failure is still
  ignored by both workflow callers, as before; a partial/missing inventory is not
  surfaced as a workflow error. No new sidecar durability guarantee is established.
- **Error classification:** `mirror.go:342` wraps every streaming error with
  `errCopySource`, including destination write/sync/close errors, and formats the
  cause with `%v`. Thus the comments promising source/destination differentiation
  overstate the implementation. Full mirror already grouped copy-engine errors
  as skipped before this slice; this is recorded separately from the blockers.
- **Metadata:** no matching-final-file chmod or timestamp update was added.
  Sidecar staging chmod is the new metadata operation reviewed in F5. Tests of
  byte equality/object identity do not by themselves prove metadata preservation.
- **Platforms:** Windows obtains canonical names from handles; Linux uses
  `/proc/self/fd` and refuses unavailable resolution. The macOS/BSD fallback
  resolves `f.Name()` and compares descriptor identities, rather than obtaining
  a canonical path directly from the kernel. It remains a weaker, unexecuted
  path. Case-insensitive macOS spelling, moved source registrations, privileged
  topology changes, media identity and helper-process confinement remain the
  recorded deferred limitations. They are not newly required closure work here.
- **Tests:** the new tests provide useful real-workflow evidence, but several
  exercise primitives directly. Their comments saying every test drives a full
  workflow or checks every preservation property are broader than the assertions.

## Compiler, Docker and CI review

Build, test and vet invocations used process-local `GOTOOLCHAIN=go1.26.8` and
`GOPROXY=off`. `go version` and `go env` reported:

```text
go version go1.26.8 windows/amd64
GOROOT=C:\Users\Nathaniel\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64
GOTOOLCHAIN=go1.26.8
GOVERSION=go1.26.8
GOOS=windows
GOARCH=amd64
CGO_ENABLED=0
```

The cached toolchain `VERSION` also says `go1.26.8` (build timestamp
`2026-08-28T16:20:06Z`). Its `bin/go.exe` SHA-256 is
`21761ECEB9302062C9623FB699F332C8C7FE000F15F70EFE8DA01A2CFBBC16B9`.
An explicit `GOTOOLCHAIN=local` invocation of `C:\Program Files\Go\bin\go.exe`
still reports **go1.26.4**. The system installation was not assumed to have changed.
No tools were downloaded/installed, no `go env -w` or global PATH/settings edits
were performed, and no WSL was used. Original toolchain acquisition/checksum-database
verification remains author-reported; this review verified the executable selected.

The four build files consistently select 1.26.8. `go.mod` raises the minimum rather
than adding a second `toolchain` directive. Dependencies are unchanged and `go.sum`
has no diff. README and contributor build requirements agree. The approved version
is beyond the 1.26.5 fix threshold documented by the
[Go vulnerability record](https://pkg.go.dev/vuln/GO-2026-4970).

Docker uses `golang:1.26.8-alpine` with the recorded index digest
`sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628`.
The runtime stage and deployment settings are unchanged. **No Docker build/run or
independent registry/digest verification was performed in this review.** Registry
existence and architecture availability are the author's September 7 evidence.

The Linux CI job retains its full suite and race step. The new Windows job builds,
vets and selects destination-authority plus mirror/incremental/atomic-rename tests.
Those commands can legitimately skip unsupported link mechanisms. Release compiler
selection is pinned consistently. **No CI or release job was triggered or observed
running.** F7 is a source-observed expected Linux test failure, not a CI result.

## Independent executions

These results are separate from the author's **259 pass / 7 fail / 6 skip**.
The full suite was **not rerun**; the seven reported OBX-006 Windows Unicode
failures were not independently re-executed or reclassified.

| Check | Actual result |
|---|---|
| Initial targeted test command in sandbox | Setup failed before tests: access denied to existing `AppData\Local\go-build` cache entry |
| Same targeted command through approved sandbox escalation | Exit 0: **32 top-level PASS / 2 SKIP**; no failures |
| Current-snapshot reviewer probes | Exit 1: **5 top-level FAIL**, including both whitespace workflow subtests; expected refusal/preservation assertions reproduced F1/F2/F3/F5/F6 |
| Pristine-base comparison probes, same compiler | Exit 0: **2 top-level PASS**, including both whitespace workflow subtests; supports new-regression classification for F2/F6 |
| Windows amd64 `go build -o <scratch>/obelisk-review.exe ./...` and `go vet ./...` | Both exit 0 |
| `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o <scratch>/obelisk-linux-amd64 .` and `go vet ./...` | Both exit 0; compilation/static checking only |
| `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o <scratch>/obelisk-darwin-arm64 .` and `go vet ./...` | Both exit 0; compilation/static checking only |
| Selected toolchain `gofmt -l` on the 11 changed/new Go files | No output; no formatting writes |
| `git diff --check` | Exit 0; Git emitted line-ending warnings, not whitespace defects |
| SHA-256 preservation comparison | 489 original tracked/untracked files unchanged |

Exact existing-suite selection, run from the project root:

```powershell
$env:GOTOOLCHAIN='go1.26.8'
$env:GOPROXY='off'
go test -count=1 -run '^(TestDestAuth_|TestMirror_|TestIncremental_|TestAtomicRename_)' -v ./...
```

Breakdown: `TestDestAuth_*` **19 pass / 2 skip**, `TestMirror_*` **4 pass**,
`TestIncremental_*` **3 pass**, `TestAtomicRename_*` **6 pass**. Counts exclude
nested subtests. Junction, hard-link, extended-prefix and short-name cases executed.
The two symbolic-link cases explicitly skipped with Windows privilege errors.
The root-substitution test passed because Windows **refused the directory rename**
while the root handle was open: its successful-substitution branch did not execute.
An approved tool sandbox escalation does not imply administrator privileges or a
change to the host's symlink capability.

Exact reviewer selection in each disposable source directory:

```powershell
$env:GOTOOLCHAIN='go1.26.8'
$env:GOPROXY='off'
go test -count=1 -run '^TestReview_' -v .
```

The current snapshot contains byte copies of the 208 tracked working-tree files
plus the six new slice-A Go files. The base snapshot was exported with read-only
`git archive` from the explicit base SHA. Only reviewer harness files were added
to these disposable copies; no project test was edited. All mutation fixtures are
synthetic `t.TempDir()` trees, never production originals, catalogs or media.

Retained scratch evidence (outside the checkout; temporary storage is not durable):

```text
C:\Users\NATHAN~1\AppData\Local\Temp\obelisk-pr04-review-20260908-384e75ae13b74e12b9b5db3ffa9db3f1\
  entry-manifest.json
  current\zz_review_probe_test.go
  base\zz_review_probe_test.go
  review-probes.txt
  baseline-probes.txt
```

| Evidence file | SHA-256 |
|---|---|
| `current/zz_review_probe_test.go` | `84EE93CA1ABE61B6AB1B65EA7970996478E29EC69CE20AC65FCDCFF7F93C5770` |
| `base/zz_review_probe_test.go` | `456EE94D0275BEDFBDB9A2661B713AF85854E3131EB7CB21A6C4773588482A1F` |
| `review-probes.txt` | `1BF78D07F25368BC9C4B2F521C2C0AD6C284FCAA2703CE99A12C2C7C18F8D0C6` |
| `baseline-probes.txt` | `A7B8F6A076B9F585B2BC8A8F6424708F3502071CCC5E26C3D2A14D4A302E7C07` |

Windows race detection, Unix execution, Unix bootstrap reproduction, successful
post-authorization root substitution, symlink behavior, Docker execution, CI,
other architectures and real media are **NOT TESTED in this review**. The initial
cache denial was a tool restriction, not a product defect. Subsequent executed
probe failures are application behavior in disposable fixtures.

## Reviewed patch identity

SHA-256 of current working-file bytes, independent of Git line-ending conversion:

```text
.github/workflows/ci.yml AB2E2861381F9C70CAF6B9B5203427AAE852218AF8859AD7C0E44A33D0DBDA42
.github/workflows/release.yml 4DADF5B2ABEB30EADD79E8173DC2839CCC7759024E8D6880F462CE2D756BD9A8
Dockerfile E90032F0C6DCD54AC2EE33E253FB27D5EC4BA26A5E43F2776EA180F599A3AB19
go.mod 035D8695F60419AE26E13BEFB4F4FBF3D706BC5DB7B6542FFF907D9F2253DB1A
go.sum 9D169AD00514AD13D9A10E1B1AC4723171C020A23B792C76721D97CCD47A1993
incremental.go 4C7075002DED5833558B4A396C7D86189CC8A26F0C4AB146C46AA5590658A16D
mirror.go FD331C82941366CFB2BD7754DE2D69A4D86BC4639C55C8DF2AA82BE59DB2B547
pipeline.go A1F1727FB958627A00BD8084E7BC62706444F1EDB246E3B80CCD932D727BFBB0
store.go 0E62A59D4B513D31718CB936484752B375574F3CBC28869AA5B411BB2FCD4FD6
verify_levels.go 4EBB572C7FB10B9A441D9607D8D8DDC5B22CC6E220C234FEBB85DDC899CA4363
destauth.go F38A741CB7EE08EFEF1270908FED3F652401048882BBE880F17F5798BC4C4745
destauth_linux.go 13B114E708B6A4381C4AF68787E2D8D813F85FF34DC4897362170147C00B4F00
destauth_other.go E3AA48F3609E0EBF3190E865ACA691FF641030D3F6BCE6D5875E79ADBFC38CA5
destauth_test.go 72F739F232BBFBD0DD26295B7DB803B4780D0F4F263CF8A53D582D6A771FB413
destauth_windows.go D7B18BF46F597788690A76B78666AB949978CAFCD3EF001C05807069E58D73C8
destauth_windows_test.go C8A7BEF05124AAFEC911DC1DEDC648AC89B4C300A652B85A55EF1D2DCE380783
```

## Exact next action

Owner review of **F1–F7**, followed by authorization of a bounded slice-A repair on
this existing branch and base. Repair the listed authorization, bootstrap,
name-preservation, staging-ownership, wrapper-order and portable-validation issues;
add their regressions and rerun the targeted selection, with real Unix execution
for F4/F7 when available. Preserve the thirteen deferred mutation callers and the
recorded limitations. **Do not start slice B or publish this candidate as ready.**

This review turn authorizes no production edits. No fetch, pull, branch switch,
reset, stash, clean, stage, commit, push or merge was performed. Prior reports and
imported handoff history remain intact.
