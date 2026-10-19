# PR-04 / OB-004 — slice A: pinned toolchain and the first integrated destination-authority boundary

**Executed 7 September 2026.** First implementation slice of PR-04, following the approved
decisions in
[PR04-OB-004-BOUNDARY-DECISION-2026-09-07.md](PR04-OB-004-BOUNDARY-DECISION-2026-09-07.md),
which is preserved unchanged.

This slice converts **one complete write path** — full mirror, its shared copy-verify-publish leaf
primitive, its inventory sidecar, and the incremental "back up changes" caller with all of its
destination effects — to a handle-based destination authority, and pins the compiler that makes
that possible. **It does not fix OB-004.** Thirteen other mutation callers remain on the old
pathname check and are listed by name in §7.

**Result: READY_FOR_REVIEW for slice A.** Nothing is staged, committed, pushed or published.

---

## 1. Exact source base

| Item | Value |
|---|---|
| Repository | `C:\Users\Nathaniel\Documents\Software Development\Mnemosyne\mnemo-go` |
| Branch | `fix/ob-004-source-boundaries` (unchanged; never switched, rebased, or moved) |
| HEAD / PR-04 base | `0dc7d5399c6889e015aebc9ba02e694a909d6e9c` |
| Git state at entry | No staged changes; one tracked modification (`docs/development/NEXT_ACTIONS.md`, the prerequisite's note); five untracked paths (the four handoff/prompt folders plus the prerequisite report). **No in-progress rebase, merge, cherry-pick or bisect.** |
| Host | Windows 11 Pro 10.0.26200, x64 |
| Project instruction files | None. No `CLAUDE.md`, no `AGENTS.md` |

The checkpoint matched the expected state exactly. The prerequisite report, its `NEXT_ACTIONS.md`
note and all untracked handoff material are preserved untouched; the incorrectly extracted
`docs/development/PR01_REVIEW_ADDENDUM/` folder was again left alone.

No fetch, pull, switch, stash, clean, reset, rebase, merge, stage, commit, push or ref move of any
kind was performed.

---

## 2. Compiler acquisition and selection

| | Value |
|---|---|
| **Original bundled compiler** | `go1.26.4 windows/amd64`, `GOROOT=C:\Program Files\Go`, `GOTOOLCHAIN=auto` |
| **Selected compiler** | `go1.26.8 windows/amd64` |
| **Selected GOROOT** | `C:\Users\Nathaniel\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.8.windows-amd64` |
| **How acquired** | Go's own verified toolchain mechanism: `GOTOOLCHAIN=go1.26.8 go version`, which downloads the toolchain **as a module** into the user-owned module cache |
| **How verified** | Module-path verification against the public checksum database. `GOSUMDB=sum.golang.org`; `GONOSUMDB`, `GOPRIVATE`, `GOFLAGS`, `GOINSECURE` and `GONOSUMCHECK` all **empty** — no verification was disabled and no third-party build was used. Recorded ziphash: `h1:V1/NuqIEUQV90GemGKR4m+57HDDasNnhxYAnodtku/E=` |
| **Environment changes** | **Process-scoped only.** `GOTOOLCHAIN=go1.26.8` was passed per command. No MSI was run, the system Go installation was not overwritten, global `PATH` was not edited, and **`go env -w` was never used** — the user's `GOENV` file is still absent/empty |

**Restoration state at exit.** Nothing needs undoing: no persistent environment change was made.
Outside this module, `go version` still reports the bundled `go1.26.4` and `GOROOT` is still
`C:\Program Files\Go`. **Inside** the module, plain `go` now selects `go1.26.8` on its own,
because `GOTOOLCHAIN=auto` honours the `go 1.26.8` directive in `go.mod` — that is the intended
pinning mechanism, not a leftover override.

`go1.26.8` is past the GO-2026-4970 / CVE-2026-39822 fix (`1.25.12`, `1.26.5`), so unlike the
prerequisite's `go1.26.4` probes this is **not** an affected compiler.

---

## 3. Build configuration changes

| File | Change |
|---|---|
| `go.mod` | `go 1.22.2` → **`go 1.26.8`**. **No `toolchain` directive added** — a second version number that merely duplicates the `go` directive makes selection harder to read, not easier. Module path and every dependency version are **unchanged**; `go.sum` is **byte-identical** |
| `Dockerfile:13` | `golang:1.22-alpine` → **`golang:1.26.8-alpine`**, pinned by the multi-arch index digest `sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628` |
| `.github/workflows/ci.yml:15` | `go-version: stable` → **`"1.26.8"`**, with a comment stating why a floor is not the same as "stable" |
| `.github/workflows/release.yml:23` | same pin |
| `.github/workflows/ci.yml` | **new `windows-boundary` job** (§8) |
| `docs/CONTRIBUTING.md` | Build requirement raised to **Go 1.26.8 or later**, with the reason and the four places that must stay in step |
| `README.md` | Badge and the build note raised to **Go 1.26.8+** |

**The Docker tag was verified, not assumed.** `golang:1.26.8-alpine` was queried on Docker Hub and
confirmed to exist and to publish `linux/amd64` and `linux/arm64` — the two platforms
`release.yml` builds the image for. The digest above is the resolved index digest returned by the
registry on 2026-09-07; **no digest was invented.**

**Nothing else about the container changed.** The runtime image (`alpine:3.20`), the installed
restore tools, `VOLUME`, `EXPOSE`, `ENTRYPOINT`, `CMD`, the healthcheck, mounts, privileges,
networking and the deployment architecture are all untouched.

**A changed Dockerfile is not a Docker build.** `docker` is **not installed on this host**
(`docker version` fails). The image was **NOT BUILT and NOT RUN**. What is established is that the
tag and its two required architectures exist in the registry; that the Go sources compile for
`linux/amd64` and `linux/arm64` (§8); and nothing more.

**No dependency was added.** The Windows canonical-path and identity calls use the standard
`syscall` package plus one `kernel32` entry point obtained through `syscall.NewLazyDLL` — no
`golang.org/x/sys`, no `filepath-securejoin`. `go.sum` is unchanged, and the
"one static binary, no CGO" bargain in `docs/ARCHITECTURE.md` is intact.

### Documented build requirement

Supported build floor: **Go 1.26.8**. The reason is recorded in `docs/CONTRIBUTING.md`:
`os.Root.Rename` / `MkdirAll` / `Chtimes` arrive in **1.25**, and **1.25.12 / 1.26.5** carry the
GO-2026-4970 fix. **1.25.12 and 1.26.5 are explicitly NOT described as the preferred build
versions** — they are the patch floor; 1.26.8 is what this project validates and pins.

---

## 4. Toolchain-only baseline versus the application fix

Three runs, all with `go1.26.8`, all uncached (`-count=1`), on disposable `git archive HEAD`
exports of `0dc7d53` so the production tree was never disturbed:

| Run | Source | `go.mod` | Result |
|---|---|---|---|
| **B1** — compiler only | pristine `0dc7d53` | `go 1.22.2` | **7 failures**, all OBX-006 Unicode |
| **B2** — compiler + directive | pristine `0dc7d53` | `go 1.26.8` | **240 pass / 7 fail / 4 skip** |
| **FIX** — working tree | slice A | `go 1.26.8` | **259 pass / 7 fail / 6 skip** |

**The toolchain change altered nothing.** B1 and B2 produce the *identical* failure set, and B2
reproduces the historical **240 / 7 / 4** figure exactly. That figure is used here only as a
comparison point between two runs I executed, not as a target.

**The 7 failures are the same 7 in all three runs** and are the known open OBX-006 Windows
Unicode work, untouched by this slice:
`TestBuildRestore_HostileFilenamesRoundTrip`, `TestBuildFilelist_IsNulDelimited`,
`TestBuildChunk_UnicodeFilenames_ExactMembers` (5 subtests: accented, japanese, supplementary,
bmp-symbol, unicode-dir-component), `TestBuildChunk_WrongFileSelection_LookalikeNeighbour`,
`TestBuildChunk_UnicodeSourceRoot`, `TestBuildChunk_UnicodeStagingDir`,
`TestBuildRestore_UnicodeRoundTrip`. **None of the seven was weakened, skipped or relaxed.**

**The deltas are all additions:** +19 new passing tests, and +2 **loud** skips
(`TestDestAuth_SymlinkAliasOfSourceRefused`, `TestDestAuth_NestedSymlinkLeafRefused`) that report
*"SKIP (NOT TESTED): symbolic links cannot be created on this host: A required privilege is not
held by the client."* The 4 pre-existing skips are unchanged.

---

## 5. The authorization and root-relative operation contract

New file `destauth.go`, with per-platform helpers in `destauth_windows.go`, `destauth_linux.go`
and `destauth_other.go`. No second general path-traversal engine was written: confinement is
`os.Root`, and this code supplies only the decision and the platform identity calls Go does not
export.

### A. Authorization of the destination object — `Store.AuthorizeDestination` (`destauth.go:236`)

1. **Open first, then name.** The destination is opened, and its canonical location is derived
   **from that open handle** — `GetFinalPathNameByHandle` on Windows (`destauth_windows.go:37`),
   `/proc/self/fd/<n>` on Linux, and on macOS/BSD `EvalSymlinks` **proved against the descriptor's
   own fstat identity** (§10). The extended-length prefix is stripped
   (`destauth_windows.go:63`) so a handle-derived name and a `filepath.Abs` name share one
   spelling. There is no string check followed by opening a possibly different object.
2. **Registered sources are canonicalised the same way.** `Store.sourceSnapshot`
   (`destauth.go:136`) opens each registered source root and takes its canonical name from the
   handle too — which is what makes an alias of a source compare *equal* to the source. A root
   that cannot be opened for any reason other than "does not exist" is a **refusal**, not a pass:
   authorizing while a protected location is unresolved would be authorizing on a guess.
3. **Three separate answers, not one boolean** — `sourceSnapshot.decide` (`destauth.go:175`),
   compared at path-segment boundaries:
   - **equal** — the destination *is* a registered source → refused;
   - **inside** — the destination is beneath one → refused;
   - **ancestor** — the destination *contains* one → refused, with the message naming the specific
     source and telling the operator to pick a narrower output folder. **The refusal is
     deliberately conservative: it does not try to prove some subtree of a broad parent is safe,
     and it does not silently authorize the parent.**
4. **Bootstrap is explicit and confined.** A destination that does not exist yet is created
   through its nearest existing ancestor: the ancestor is opened, the verdict is taken on the
   *intended* canonical name, the missing components are created with `Root.MkdirAll` **relative
   to the authorized ancestor**, and the leaf is opened with `Root.OpenRoot`. **No unrestricted
   `os.MkdirAll` runs before the decision** — the two that used to (`mirror.go`, `incremental.go`)
   are gone.
5. **The verdict is taken again on the handle finally retained** (`destauth.go:302`), closing the
   gap between the name intended and the object held.
6. **The handle is returned and never re-derived.** `DestRoot.Canonical()` exists for read-only
   queries (free space) and messages. It is not a capability and is never reopened for a mutation.

### B. Operations within the retained authorized directory

`DestRoot` methods, every one root-relative: `MkdirAll`, `stagingFor`, `Publish`, `Discard`,
`Open`, `Stat`, `HashRel`, `SampleHashRel`, `WriteFileAtomic`.

- **Relative paths are validated once, at one place** — `destRel` (`destauth.go:76`) rejects
  empty, rooted, drive- or UNC-qualified paths and any empty, `.` or `..` element. Catalog
  `RelPath`, `mirrorFolderLabels` output and sidecar component names all pass through it.
- **No absolute pathname is reopened after authorization** — including the read-back, the level-C
  sample, the sidecars and every cleanup. `os.Remove(tmp)` became `DestRoot.Discard(relTmp)`.
- **Staging is uniquely owned and never follows an existing leaf.** `stagingFor`
  (`destauth.go:407`) builds `.<per-authorization random token>-<counter>.mnemo_tmp` in the target
  directory and opens it `O_CREATE|O_EXCL`. A pre-existing file with the staging suffix is an
  **error, never a truncation** — the previous `os.Create(destPath + ".mnemo_tmp")` truncated
  whatever that name resolved to.
- **PR-02's guarantee is carried through verbatim.** `DestRoot.Publish` (`destauth.go:443`) wraps
  `Root.Rename` and **never removes the final name**; a failed publish leaves the existing file
  byte-identical and returns the error. Verified by execution on `go1.26.8` (§6 probe `[2]`,
  `[3]`) and by regression (§8).
- **A rename publish was not converted into a truncating write.** Every converted writer stages
  and publishes; none merely switched to `Root.Create`, which would still truncate a hard-linked
  leaf.
- **No chmod or timestamp change is applied to an existing destination because its contents
  match.** The converted path does not touch destination metadata at all; the only `Chmod` is on
  a staging file this operation created, before it is published.

### Source-registration snapshot and synchronisation

`SourceRoots()` takes the general Store mutex and **releases it before returning**. The snapshot
is taken once, at authorization; **the Store mutex is not held for the duration of a job** that
also needs it to record results.

**Residual, stated rather than hidden:** a source registered *after* authorization, or protected
data moved *into* the authorized tree afterwards, is not covered by the snapshot. The retained
handle still confines every write to the directory that was authorized; it does not re-decide
whether that directory has since become protected. Re-authorizing at batch boundaries would
narrow this and is not done in slice A.

**No conflicting-registration or ownership dependency was found that makes the stated guarantee
impossible** — and the guarantee stated in §11 is deliberately narrower than "OB-004 is fixed".

---

## 6. Platform behaviour, measured on the selected compiler

Probe in `…/scratchpad/rootprobe268`, run with `go1.26.8` on this Windows host. Raw output
retained (`rootprobe268-out.txt`, `rootprobe268-out2.txt`).

```
[2]  Root.Rename over an existing destination: err=<nil>, final="NEW"     ← PR-02 replace behaviour holds
[3]  Root.Rename with missing staging: err=..., final still "NEW"          ← failed publish preserves the destination
[4]  O_CREATE|O_EXCL on a pre-existing staging leaf: "file exists", content preserved
[5]  create through a nested junction:  "path escapes from parent"
[5b] MkdirAll through a nested junction: "path escapes from parent"
[5c] Rename INTO a nested junction:      "path escapes from parent"
[5d] Stat through a nested junction:     "path escapes from parent"
[6]  absolute path / "..\" / "NUL" / "sub/../../prot/y.txt": all refused
[7]  renaming the authorized root while its handle is open: REFUSED by the OS (sharing violation)

[A]  Root.Open(".") yields a handle → the root's own canonical location is derivable
[C]  GetFinalPathNameByHandle(directory junction) = the junction's TARGET
[D]  GetFinalPathNameByHandle("C:\Users\NATHAN~1") = "C:\Users\Nathaniel"   ← 8.3 short names resolve
[E]  bootstrap: ancestor MkdirAll + OpenRoot works, and the child root outlives the ancestor's close
[F]  stage + Root.Rename over a hard-linked leaf: protected original still "ORIGINAL SOURCE BYTES"
```

`[7]` is the honest limit and is discussed in §10.

---

## 7. Integrated callers, and the ones still on the old boundary

### Converted in slice A — the complete path, end to end

| Caller | Every destination effect now root-relative |
|---|---|
| `MirrorToVolume` (`mirror.go:142`) | authorization + bootstrap; per-file parent creation; staging; payload write; read-back; publication; level-C sample; failure cleanup; sidecar |
| `copyVerifyToRoot` (`mirror.go:325`) | the shared leaf primitive — replaces `copyVerifyToDest`; stage → read back → publish → discard, all through the handle |
| `writeVolumeInventoryTo` (`mirror.go:433`) | both sidecar files, staged and published like payload, through the same handle |
| `BackupChanges`, mirror mode (`incremental.go:257`) | authorization + bootstrap; the shared leaf primitive; **and its own sidecar**; free-space preflight moved to the authorized canonical name |

`BackupChanges` is **not** claimed as covered merely because it shares a helper. Its destination
effects were traced individually: payload (shared helper), the `OBELISK_DOCK` sidecar (its own
call, now `writeVolumeInventoryTo`), the free-space preflight, and its remaining effects —
`mergeMirrorRefs` and `recordBackupSession` — which write only to the catalog, not to the
destination.

Supporting changes, deliberately small:

- `mirrorCopyStream` (`mirror.go:82`) is the copy engine, taking an already-open destination file.
  `mirrorCopyFile` (`mirror.go:66`) is now a thin wrapper over it and is retained **unchanged in
  behaviour** for its one unconverted caller, `plans.go:744`.
- `hashReader` (`pipeline.go`) and `sampleHashOpen` (`verify_levels.go`) let a read-back through
  the authorized handle use the *same* implementation as the by-pathname versions.
- `AssertOutsideSources` (`store.go:1903`) is **unchanged in behaviour** and **redocumented** as a
  preflight message-quality check, naming the four things it cannot decide and pointing to the
  real boundary. The ancestry rule was deliberately **not** added to it: changing the verdict for
  thirteen callers this slice does not test would be exactly the untested widening this slice is
  meant to avoid.

### Still on the old boundary after slice A — NOT FIXED

`plans.go:682` (`ExecutePlanFromDrive`), `writer.go:303` (`WriteChunk`), `writer.go:632`
(`RestoreChunk`), `span.go:134` (`SpanWriteNext`), `pipeline.go:165`, `pipeline.go:169`,
`pipeline.go:174` (`SaveConfig`), `pipeline.go:1027` (`BuildChunk`), `appbackup.go:257`
(`ExportAppBackup`), `appbackup.go:520` (auto-backup), `bagit.go:238` (`exportBagForChunks`),
`escrow.go:344` (`WriteEscrowBundle`), `recoverykit.go:38` (`BuildRecoveryKit`), `finalize.go:169`
and `finalize.go:249` (`FinalizeVolume` / `writeFinalizeSidecar`), `dock.go:281` (`mirrorAdopt`,
read-only, correctly a policy refusal), `quarantine.go:140` (`QuarantineTerritory`).

Every hazard reproduced in the prerequisite still applies to those callers. The truncating-writer
hard-link hazard in particular is **still live** for `copyFile` (`writer.go:761`) and its callers,
and is demonstrated again below.

**Not expanded into, as instructed:** general `copyFile` migration; restore/extraction paths;
application-state generation; keystore operations; tape or optical lifecycle; a native Unicode tar
writer; the OB-007 ownership/process-lock project; any UI change.

---

## 8. Evidence

### Red reproduction — pristine source, same compiler

`…/scratchpad/base-b2/zz_ob004_red_test.go` (retained, with its output in
`…/scratchpad/red-reproduction.txt`) runs against a disposable `git archive HEAD` export of
`0dc7d53` compiled with **the same `go1.26.8`**, so the only difference between red and green is
the application change. All six reproduce:

```
RED1 GUARD: AssertOutsideSources(ancestor) = ALLOWED
RED1 REPRODUCED: registered source original was REPUBLISHED (different object) by a mirror to its parent
RED2 GUARD: real path REFUSED, alias ALLOWED
RED2 REPRODUCED: 6 new path(s) written INSIDE a registered source through a destination alias (sidecar included)
RED3 REPRODUCED: a leaf write escaped an authorized destination root into a registered source
RED4 GUARD: AssertOutsideSources(extended-length spelling of a source) = ALLOWED
RED4 REPRODUCED: 4 new path(s) written INSIDE a registered source via the `\\?\` spelling
RED5 short spelling: ...\TESTRE~1\003\A-VERY~1
RED5 GUARD: AssertOutsideSources(8.3 spelling inside a source) = ALLOWED
RED5 REPRODUCED: 4 new path(s) written INSIDE a registered source via the 8.3 spelling
RED6 REPRODUCED: a truncating leaf writer overwrote a registered source original through a pre-existing hard link
```

**RED5 is a new finding.** The prerequisite recorded Windows 8.3 short-name aliasing as
**NOT TESTED**; it is now reproduced — the old guard allowed the short spelling and four files
landed inside a registered source. RED6 exercises `copyFile` (`writer.go:761`) and therefore
**still reproduces against the current branch**, because that writer is unconverted (§7).

### Green regressions — durable, in the repository

`destauth_test.go` (portable) and `destauth_windows_test.go` (Windows link and naming mechanisms).
All drive real production workflows — `MirrorToVolume` and `BackupChanges` — not a predicate.

| Test | Maps to | Result |
|---|---|---|
| `TestDestAuth_OrdinarySafeDestination` | §3.5 positive control | PASS |
| `TestDestAuth_AncestorDestinationRefused` | §3.1 | PASS |
| `TestDestAuth_AncestorDestinationRefusedForBackupChanges` | §3.1, incremental caller | PASS |
| `TestDestAuth_JunctionAliasOfSourceRefused` | §3.2 | PASS |
| `TestDestAuth_NestedJunctionLeafRefused` | §3.3 | PASS |
| `TestDestAuth_SidecarThroughNestedJunctionRefused` | §3.2/§3.3, sidecar authority | PASS |
| `TestDestAuth_RootSubstitutedAfterAuthorization` | §3.4, deterministic barrier | PASS |
| `TestDestAuth_HardLinkedDestinationLeaf` | §3.6 | PASS |
| `TestDestAuth_ExtendedLengthPrefixRefused` | §3.7 | PASS |
| `TestDestAuth_ShortNameSpellingRefused` | §3.9 (was NOT TESTED) | PASS |
| `TestDestAuth_PublishFailurePreservesPriorDestination` | PR-02 guarantee, failure completion | PASS |
| `TestDestAuth_PublishFailureReportsTheCause` | the job reports the real failure | PASS |
| `TestDestAuth_SidecarAndCleanupObeyAuthority` | sidecar + cleanup share the payload's authority | PASS |
| `TestDestAuth_StagingIsUniquelyOwned` | unique staging, no truncation of an existing leaf | PASS |
| `TestDestAuth_BackupChangesLandsThroughAuthorizedRoot` | incremental path + bootstrap | PASS |
| `TestDestAuth_EqualAndInsideSourceStillRefused` | the old directions do not regress | PASS |
| `TestDestAuth_RelPathValidation`, `_FileAsDestinationRefused`, `_CanonicalComesFromTheHandle` | contract | PASS |
| `TestDestAuth_SymlinkAliasOfSourceRefused` | symlink alias — a **different mechanism** | **SKIP (NOT TESTED)**, privilege not held |
| `TestDestAuth_NestedSymlinkLeafRefused` | nested symlink | **SKIP (NOT TESTED)**, privilege not held |

Assertions are not "the bytes are the same". Each protected-data claim checks **object identity
captured before the mutation from an open handle** (`handleIdentity` —
`GetFileInformationByHandle` on Windows, `fstat` on Unix), **directory membership** via a full
walk, **contents**, and the absence of any staging leftover. This is the §3.8 trap avoided
deliberately: `os.Stat` fills its identity fields lazily on Windows by reopening the name, so a
late `os.SameFile` can report "same file" about a different object.

### Existing suites

All with `go1.26.8`, uncached:

| Suite | Result |
|---|---|
| `TestAtomicRename_*` (PR-02) | **6/6 PASS** |
| `TestMirror_*` | **4/4 PASS** |
| `TestIncremental_*` | **3/3 PASS** |
| `TestDestAuth_*` | **19 PASS, 2 loud SKIP** |
| PR-01 startup / schema / recovery-save (`TestOpenStore_*`, `TestCatalog*`, `TestPersistObserver_*`, `TestWriteCatalog_*`, `TestAppendVerifyEventErr_*`, `TestBuildChunk_TerminalStageWriteIsDurabilityGate`) | **17/17 PASS** |
| PR-03 completion + job UI (`TestDurableCompletion_*`, `TestJobsUI_*`) | **20/20 PASS** |
| OBX-006 containment (`TestContainment_*`) | **8/8 PASS** |
| `go build ./...` | pass |
| `go vet ./...` | pass |
| `gofmt -l .` | clean for every Go file in the module (the two files it lists are pre-existing untracked handoff material outside the build, deliberately untouched) |
| Full uncached suite (`go test -count=1 -v ./...`) | **259 pass / 7 fail / 6 skip**; the 7 are the unchanged OBX-006 Unicode failures (§4) |

### Cross-compilation — compilation only, NOT execution

`CGO_ENABLED=0 go build` succeeds for `linux/amd64`, `linux/arm64`, `darwin/amd64`,
`darwin/arm64`, `windows/arm64`. **This proves the new platform files compile. It is not evidence
that the boundary behaves correctly on those platforms**, and it is not a substitute for running
the tests there.

### The new Windows CI job

The repository had **no Windows job at all**. `ci.yml` now has `windows-boundary`:
`windows-latest`, `go-version: "1.26.8"`, `go build`, `go vet`, then two enumerated test steps —
`-run "^TestDestAuth_"` and `-run "^(TestMirror_|TestIncremental_|TestAtomicRename_)"`. Nothing is
`continue-on-error` and no test is suppressed; the existing Linux job keeps every check it had,
**including the `-race` lane**.

**Preparing workflow YAML is not a CI run. No CI executed.** Neither job has been triggered; the
YAML is unexecuted configuration.

---

## 9. What was NOT established

- **No CI run.** The Windows job is written, not executed.
- **Docker: NOT BUILT.** Docker is absent from this host. Tag and architecture existence were
  verified against the registry; the build was not performed.
- **Windows `-race`: NOT TESTED.** The race detector needs cgo; `gcc` is **absent** here and
  `CGO_ENABLED=1 go test -race` fails to build. No C toolchain was installed to manufacture the
  evidence.
- **Linux and macOS: NOT TESTED.** Compiled, never executed. `destauth_linux.go` and
  `destauth_other.go` carry that statement in their own headers.
- **Symbolic links: NOT TESTED.** This host refuses symlink creation without elevation. The two
  symlink tests skip loudly with that reason. Junctions and hard links were the mechanisms
  actually exercised, and they are **different mechanisms**, not stand-ins.
- **Root substitution after authorization: the substitution branch was NOT EXERCISED.** On this
  host the OS **refused the rename outright** while the handle was open — a Windows sharing-mode
  side effect, **not** a portable guarantee and not something the design relies on. The test
  records which mechanism fired and asserts the property that matters (nothing reached the
  protected source) either way. Go documents that a `Root` follows the original directory when it
  is moved; **that behaviour remains unverified by me.**
- **Media identity: NOT ESTABLISHED, and deliberately not claimed.** A retained handle proves the
  writes go to one specific directory object. It does **not** prove that object is the volume,
  session or removable medium the operator selected. `MirrorToVolume` still takes `destDir` and
  `volumeID` as unrelated parameters. Path spelling, filesystem identity and medium identity are
  three different answers, and **medium identity is not inferred from a drive letter or a label.**
- **External helper processes are NOT confined.** `tar`, `par2`, `gpg`, `dvdisaster`, `growisofs`
  and the operator-configured burn command receive pathname operands and run as separate
  processes. **A `Root` handle held by this process confines nothing about a child process.** No
  helper-executed mutation is inside slice A's accepted scope.
- **Privileged topology changes are out of the threat model** — bind mounts, mount substitution,
  drive-letter reassignment, device swap. Go's repaired `Root` API is **not** claimed to prevent
  any of them; that is outside its documented contract. Read-only mounts and filesystem
  permissions on source volumes remain the defence in depth.
- **macOS case-folding remains a gap.** `normPath` (`store.go:32`) folds case only on Windows, so
  two spellings of one path on a case-insensitive macOS volume can still compare unequal.
  CODE_OBSERVED, unexercised, unchanged by this slice.
- **`plan.Mapping` relative-path validation** (`plans.go:740`) remains CODE_OBSERVED and
  unconverted.

---

## 10. macOS/BSD tier — the difference, stated plainly

Windows and Linux answer "where does the object behind this descriptor live?" directly
(`GetFinalPathNameByHandle`, `/proc/self/fd`). macOS answers it with `fcntl(F_GETPATH)`, which the
standard library does not export and which cannot be reached without adding `golang.org/x/sys` — a
dependency this codebase declines.

`destauth_other.go` therefore resolves the location by name with `EvalSymlinks` and then **proves
the descriptor held is that object**, by comparing identities taken from the open descriptor and
from the resolved name; a disagreement is a refusal. The name is never used again afterwards.

This closes resolve-then-swap. It is a **weaker tier than the other two** and is labelled as such
in the source. It is also **NOT TESTED** — no macOS or BSD host was available.

---

## 11. The exact guarantee achieved

**For `MirrorToVolume`, `copyVerifyToRoot`, `writeVolumeInventoryTo` and `BackupChanges` in mirror
mode, and for no other caller:**

> The destination is authorized once, on the identity of the object actually opened, against a
> snapshot of the registered source roots canonicalised the same way; it is refused if it is, is
> inside, or contains a registered source, under any spelling that resolves to the same object on
> this platform, and refused if that identity cannot be established. Every subsequent destination
> mutation — parent creation, staging, payload bytes, read-back, publication, the level-C sample,
> both sidecar files and all failure cleanup — is performed relative to the retained handle and
> cannot address anything outside it. Staging files are uniquely owned and are created without
> following an existing leaf. Publication never removes the destination it is replacing, and never
> becomes a truncating write, so a pre-existing hard link to a protected original survives intact.

**What that is not:** it is not proof the destination is the intended medium; it is not confinement
of external helper processes; it is not protection against privileged topology change; it does not
cover sources registered after authorization; and it is **not** OB-004 fixed — thirteen mutation
callers listed in §7 still use the old pathname check, and the truncating-writer hard-link hazard
is still live for them.

---

## 12. Files changed

```
 .github/workflows/ci.yml         |  47 +++++-   pinned compiler + new windows-boundary job
 .github/workflows/release.yml    |   2 +-       pinned compiler
 Dockerfile                       |  11 ++-      builder image 1.22-alpine -> 1.26.8-alpine@digest
 README.md                        |   6 +-       build requirement
 docs/CONTRIBUTING.md             |  11 ++-      build requirement + why
 docs/development/NEXT_ACTIONS.md |  +           dated status note appended (prerequisite's note preserved)
 go.mod                           |   2 +-       go 1.22.2 -> go 1.26.8   (go.sum UNCHANGED)
 incremental.go                   |  25 +--      BackupChanges converted
 mirror.go                        | 174 ++++--   mirror path converted
 pipeline.go                      |   9 +-       hashReader extracted
 store.go                         |  31 ++-      AssertOutsideSources redocumented (behaviour unchanged)
 verify_levels.go                 |   8 +        sampleHashOpen extracted
 destauth.go                      |  new         the boundary
 destauth_windows.go              |  new         handle-derived location + identity
 destauth_linux.go                |  new         handle-derived location + identity (NOT TESTED)
 destauth_other.go                |  new         weaker tier, identity-verified (NOT TESTED)
 destauth_test.go                 |  new         portable regressions
 destauth_windows_test.go         |  new         Windows link/naming regressions
```

**Not staged, not committed, not pushed. No branch ref moved. No release published. No other issue
started.**

Retained evidence outside the repository, in this session's scratch space:
`base-b1/`, `base-b2/` (disposable `git archive` exports with the red reproduction source),
`baseline-b1.txt`, `baseline-b2.txt`, `baseline-b2-verbose.txt`, `fix-full-suite.txt`,
`destauth-run1.txt`, `red-reproduction.txt`, `rootprobe268/`, `rootprobe268-out.txt`,
`rootprobe268-out2.txt`.

---

## 13. Status

**READY_FOR_REVIEW — PR-04 slice A: pinned toolchain and the first integrated
destination-authority boundary.**

**Not** "OB-004 fixed."
