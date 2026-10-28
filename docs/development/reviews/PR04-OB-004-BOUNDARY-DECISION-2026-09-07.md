# PR-04 / OB-004 — source and destination filesystem authority: prerequisite decision package

**Executed 7 September 2026.** Bounded prerequisite pass. **No production source, test, UI,
dependency, toolchain or CI file was changed.** This document is the only deliverable, plus the
narrow status note appended to `NEXT_ACTIONS.md`.

Original issue IDs and PR numbers are preserved: this is a prerequisite slice **within PR-04**,
covering **OB-004** and the **OB-007 prerequisite** portion named by
`02_FIRST_PULL_REQUESTS.md`. It is not a new epic and nothing is renumbered.

---

## 1. Source base and environment

| Item | Value |
|---|---|
| Repository | `C:\Users\Nathaniel\Documents\Software Development\Mnemosyne\mnemo-go` |
| Branch at entry | `fix/ob-002-durable-completion` |
| HEAD at entry (PR-04 base) | `0dc7d5399c6889e015aebc9ba02e694a909d6e9c` |
| New work branch | `fix/ob-004-source-boundaries`, created **from the explicit base SHA**, not from `main` and not from PR-03's parent |
| Index / tracked tree at entry | Clean. No staged changes, no tracked modifications, no in-progress rebase/merge/cherry-pick/bisect |
| Untracked at entry (expected, untouched) | `docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/`, `docs/OBELISK_POST_PR02_FIX_PROMPTS_2026-09-07/`, `docs/development/PR01_REVIEW_ADDENDUM/`, `docs/development/UI_FEATURE_MAP-2026-09-06.md` |
| Project instruction files | None found — no `CLAUDE.md`, no `AGENTS.md`. `.claude/settings.local.json` contains only a Bash permission allowlist |
| Host | Windows 11 Pro 10.0.26200, x64 |
| Local compiler | `go1.26.4 windows/amd64`, `GOROOT=C:\Program Files\Go`, `GOTOOLCHAIN=auto` |
| `go.mod` | `go 1.22.2`. **No `toolchain` directive.** |

The incorrectly extracted `docs/development/PR01_REVIEW_ADDENDUM/` folder was **not repaired** and
not touched, as instructed.

### Anchors are revalidated, not inherited

`OB_STATUS.md` anchors OB-004 at `store.go:1724-1753` / `store.go:26-39` / `store.go:1712`. Those
line numbers are stale after PR-01/02/03. At `0dc7d53` the current anchors are:

| Symbol | Baseline anchor | **Current anchor at `0dc7d53`** |
|---|---|---|
| `normPath` | `store.go:26-39` | **`store.go:32-42`** |
| `SourceRoots` | `store.go:1712` | **`store.go:1866-1876`** |
| `AssertOutsideSources` | `store.go:1724-1753` | **`store.go:1884-1907`** |

The **decision line is unchanged in substance**: `store.go:1901`,
`np == rp || strings.HasPrefix(np, rp+"/")`, over `normPath`-cleaned strings, with
`filepath.Abs` as the only resolution.

The baseline's "20 call sites" hypothesis **is revalidated and holds**: exactly **20 production
call sites** at this SHA. (`datamap.go:130` and `dock.go:16` are a documentation string and a
comment; `datamap_test.go:118` is a test.)

---

## 2. The write-authority boundary as it actually is

### 2.1 Three distinct properties, currently collapsed into one boolean

`AssertOutsideSources` answers exactly one question — *"is this pathname string at or beneath a
registered source pathname string?"* — and every caller treats the answer as "safe to write."
Three separate properties are being conflated:

| Property | What it should mean | Status at `0dc7d53` |
|---|---|---|
| **Source-root overlap** | The object being written is not, and is not inside, protected source data | Tested **lexically, one direction only**, on strings |
| **Destination containment** | Every leaf operation lands *beneath the authorized destination* and nowhere else | **Not tested at all.** Leaf paths are rebuilt with `filepath.Join` and reopened by absolute name |
| **Media identity** | The destination really is the removable medium / volume / session the operator selected | **Not tested at all.** `MirrorToVolume(collectionID, folderIDs, destDir, volumeID, …)` takes `destDir` and `volumeID` as unrelated parameters and never cross-checks them. `resolveDeviceIdentity` exists (`deviceid.go:35`) but is used only for dock ingest (`dock.go:117,172`), home mounts (`home.go:531`) and the plan **source** drive (`plans.go:692`) — never to bind a write destination |

The guard is also **one-directional**. It rejects a destination *inside* a source. It does not
reject a destination that *contains* one. Section 3.1 turns that into a reproduction with no
links involved at all.

### 2.2 Guard → mutation caller matrix

All 20 guard calls are **preflight at function entry**. The mutation column lists what is reached
afterwards, and the "escape surface" column names what the entry check cannot see.

| # | Guard call | Protected input | Destination root selection | Relative-path origin | Actual mutations reached | Escape surface |
|---|---|---|---|---|---|---|
| 1 | `mirror.go:132` `MirrorToVolume` | `destDir` | Caller-supplied string | Catalog `File.RelPath`, prefixed by `mirrorFolderLabels` base name when >1 folder (`mirror.go:184-185`) | `os.MkdirAll` ×2 (`187`, `219`), `mirrorCopyFile`→`os.Create` (`65`), `os.Remove` ×3 (`240,248,253`), `atomicRename`→`renameFile` (`348`) | Per-file `destPath` rebuilt in-loop and reopened by absolute name; never re-checked |
| 2 | `mirror.go:385` `writeVolumeInventory` | `destDir` | Same string, re-passed | Fixed `OBELISK_DOCK/…` | `os.MkdirAll` (`389`), `os.WriteFile` ×2 (`444`, `474`) | Sidecar follows any alias the destination has |
| 3 | `incremental.go:300` `BackupChanges` | `destDir` | Caller-supplied | Catalog `RelPath` | `os.MkdirAll` (`303`), then **shared** `copyVerifyToDest` (`mirror.go:292`) | Same as #1; shares the leaf primitive |
| 4 | `plans.go:682` `ExecutePlanFromDrive` | `plan.DestinationRoot` | Persisted plan field | `plan.Mapping[sf.Hash]` — a **persisted mapping value**, joined at `plans.go:740` | `os.MkdirAll` ×2 (`704`, `739`), `mirrorCopyFile`, `os.Remove` ×4 (`746,753,760,765`), `atomicRename` | Mapping values are catalog-persisted and never re-validated against the destination root |
| 5 | `writer.go:303` `WriteChunk` | `destDir` | Caller-supplied | `c.Name`, then staged entry names | `os.MkdirAll` (`315`), `ringCopy`→`os.Create` (`70`), **`copyFile`→`os.Create` (`390`→`767`)** | `copyFile` **truncates in place** — see §3.6 |
| 6 | `writer.go:632` `RestoreChunk` | `outputDir` | Caller-supplied | tar member names (external `tar`) | `os.MkdirAll` (`635`), `rejoinSegments`→`os.Create` (`span.go:344`), **external `tar`/`par2`/`gpg`** | External helpers are **not confined** by any Go-side handle (§6.3). Member confinement is OB-008, not this issue |
| 7 | `span.go:134` `SpanWriteNext` | `destDir` | Caller-supplied | `c.Name` + segment index | `os.MkdirAll` (`158`), `ringCopy`→`os.Create`, `copyFile` (`274`, `303`) | Truncating writers |
| 8 | `pipeline.go:165,169,174` `SaveConfig` | `StagingDir`, each `KeystorePaths` entry, `AutoExportDir` | Config fields | — | `os.WriteFile` (`179`) writes the config itself | **Validation-time only.** A later alias/rename makes the stored value stale |
| 9 | `pipeline.go:1020` `BuildChunk` | `cfg.StagingDir` | Config field, **re-checked at build time** | `c.Name` | `os.MkdirAll` (`1024`), `os.WriteFile` (`1053`, `1096`), `os.Remove` (`1184`), **external `tar`/`gpg`/`par2`** | The re-check is the best pattern in the codebase today — and is still a string test |
| 10 | `appbackup.go:257` `ExportAppBackup` | `destDir` | Caller-supplied | dated base name | `exportAppBackupTo`: `os.MkdirAll` (`183`), `os.Create` (`187`), `os.Remove` ×2 (`217,221`), `os.WriteFile` (`231`) | — |
| 11 | `appbackup.go:520` auto-backup | `dir` | Config-derived | dated name | same as #10 | Runs unattended on a schedule |
| 12 | `bagit.go:238` `exportBagForChunks` | `outputDir` | Caller-supplied | Catalog `RelPath` via `addFile` (`bagit.go:256-260`) | `os.MkdirAll` ×2, `os.WriteFile` ×4 | Truncating writer at the leaf |
| 13 | `escrow.go:344` `WriteEscrowBundle` | `destDir` | Caller-supplied | fixed component names | `os.MkdirAll` ×2 (`348`, `364`), `os.WriteFile` ×2, `placeEscrowFile`→`os.WriteFile`/`os.Create` (`419`, `430`) | Truncating writer; **secret-bearing** output |
| 14 | `recoverykit.go:38` `BuildRecoveryKit` | `outputDir` | Caller-supplied | fixed names + `k.Ref` | `os.MkdirAll` (`43`), `os.WriteFile` ×11 (`57`–`168`) | **Secret-bearing** (key cards, QR PNGs). Overlaps OB-006/OB-011 |
| 15 | `finalize.go:169` `FinalizeVolume` | `mountPath` | Caller-supplied | — | via #16 | — |
| 16 | `finalize.go:249` `writeFinalizeSidecar` | `mountPath` | Same string, re-passed | fixed `…/FINALIZATION.json` etc. | `os.MkdirAll` (`253`), `os.WriteFile` ×3 (`295`, `302`, `345`) | Seal sidecar follows any alias |
| 17 | `dock.go:281` `mirrorAdopt` | `mountPath` | Caller-supplied | — | **None** — read-only adopt | Correctly used as a *policy* refusal, not a write gate |
| 18 | `quarantine.go:140` `QuarantineTerritory` | `absPath` | — | — | Gates `Quarantine`: `os.MkdirAll` (`468`), `os.Rename` (`471`); `UnQuarantine` (`514`, `517`); `pruneEmptyDirs` `os.Remove` (`710`) | The **only per-path** caller. It gates a *move source*, not a write destination |

**Unguarded mutation sites** reached without any `AssertOutsideSources` on the path they touch:
`burner.go:175-176` (renames the plaintext manifest out of and back into `StagingDir`, then hands
`c.StagedDir` to an operator-configured shell command via `runShell`), `store.go:1170/1383/1439-1461/1555-1561/3503-3518`
(data-directory writes), `migrate.go:81-86`, `removal.go:38-53`, `dvdisaster.go:140`,
`escrow.go:458-556` (fetch cache), `pipeline.go:358-367/1405/1424/1478`.

**Failure cleanup.** Every cleanup is `os.Remove(tmp)` by absolute pathname
(`mirror.go:240,248,253,299,305,309`, `plans.go:746,753,760,765`, `pipeline.go:1184`,
`appbackup.go:217,221,402`, `escrow.go:547,553`). Cleanup therefore travels the **same unchecked
absolute pathname** as the write, and reaches whatever that name resolves to at cleanup time.

**PR-02's guarantee, which must survive.** `atomicRename` (`mirror.go:348-354`) never removes
`final`; a failed publication leaves the existing file byte-identical and returns the error. Any
replacement primitive must keep exactly this property.

---

## 3. Reproductions

All fixtures are synthetic files under `t.TempDir()`. Probe code and raw output live **outside the
repository** in this session's scratch space and were never added to the working tree:

- `…/scratchpad/probe-src/` — a **disposable `git archive HEAD` export** of `0dc7d53`, so the
  probes drive **real production code**, not a re-implementation. Probes:
  `zz_ob004_probe_test.go`, `zz_ob004_probe2_test.go`.
- `…/scratchpad/probe-run-1.txt`, `probe-run-2.txt`, `rootprobe-out.txt` — raw output, retained.
- `…/scratchpad/rootprobe/`, `…/scratchpad/apiprobe/` — toolchain probes (§5).

Nothing outside those disposable directories was written. No storage was mounted or unmounted, no
permissions or elevation were changed, and no production catalog, key, NAS original, backup, tape
or optical medium was touched.

### Link-mechanism capability on this host — stated precisely

| Mechanism | Available? | Used in |
|---|---|---|
| **Directory junction** (`mklink /J`, reparse point) | **Yes** | P2, P3, P5, root probe |
| **Hard link** (`mklink /H`) | **Yes** | P1B witness, P7 |
| **Directory symbolic link** (`mklink /D`) | **No** — *"You do not have sufficient privilege"* | — |
| **File symbolic link** (`mklink`) | **No** — same refusal | — |

Symbolic links are **NOT TESTED**, not "safe". Junctions, symlinks and hard links are distinct
mechanisms with distinct resolution rules, and only the two marked *Yes* were exercised.

### 3.1 P1B — destination is an **ancestor** of registered sources. No links at all.

Two registered source folders `…/Alpha` and `…/Bravo` under a common parent. Mirror both to the
parent.

```
GUARD VERDICT: AssertOutsideSources("…\002") = ALLOWED
  ... while the SAME guard refuses each registered source it contains:
      …\002\Alpha -> true
      …\002\Bravo -> true
MIRROR RESULT: mirrored=2 failed=0
REPRODUCED (publish): registered source leaf …\002\Alpha\a.txt is a DIFFERENT filesystem object after the run
REPRODUCED (hard link): the pre-existing hard link to the source original is now ORPHANED from the source name
```

Because `multi := len(usedFolders) > 1` (`mirror.go:185`) and `mirrorFolderLabels` uses the source
folder's **base name**, `filepath.Join(destDir, label + "/" + RelPath)` **reconstructs the source
path exactly**. The registered original is republished by rename. This needs **no symlink, no
junction, and no race** — it is pure lexical one-directionality in the guard, and it is the
finding I would fix first.

### 3.2 P2 — destination **alias** reaching a registered source (junction)

`dest-alias` is a junction to `ProtectedB`, a source root registered in another archive.

```
CONTROL: AssertOutsideSources("…\ProtectedB") = REFUSED (refusing: … is inside source root …)
GUARD VERDICT: AssertOutsideSources("…\dest-alias") = ALLOWED — junction to the SAME directory the guard just refused
protected source … BEFORE: [keepme.txt]
protected source … AFTER:  [OBELISK_DOCK/INVENTORY.md OBELISK_DOCK/catalog_snapshot.json a.txt keepme.txt sub/deep.txt]
REPRODUCED: 4 new file(s) written INSIDE registered source … through a destination alias
```

The guard refuses a path and then permits a second name for the identical directory in the same
run. The `OBELISK_DOCK` sidecar (caller #2) followed the alias too.

### 3.3 P3 — **nested** link redirects a leaf operation

`safe-dest` is a genuinely safe directory; `safe-dest\sub` is a junction to `ProtectedB`. The
catalog holds rel path `sub/inner.txt`.

```
GUARD VERDICT: AssertOutsideSources("…\safe-dest") = ALLOWED — and it is genuinely a safe directory; the escape is one level below it
protected source … BEFORE: [keepme.txt]
protected source … AFTER:  [inner.txt keepme.txt]
REPRODUCED: leaf write escaped an authorized destination root into registered source …
```

This is the case an entry-point check **cannot** answer, however the destination string is
resolved: the authorized root is correct and the escape is below it.

### 3.4 P5 — root **substituted after** authorization (deterministic barrier, no sleeps)

The progress callback is the barrier. After `01.txt` is published, the destination directory is
renamed away and a junction to `ProtectedB` is put in its place.

```
GUARD VERDICT (at authorization): AssertOutsideSources("…\dest") = ALLOWED, and it was TRUE at that instant
BARRIER: after publishing 01.txt, "…\dest" was renamed away and replaced by a junction to registered source "…\ProtectedB"
MIRROR RESULT: mirrored=4 failed=0 swapped=true
protected source … AFTER: [02.txt 03.txt 04.txt OBELISK_DOCK/INVENTORY.md OBELISK_DOCK/catalog_snapshot.json keepme.txt]
REPRODUCED: 5 file(s) written into registered source … AFTER a check that was correct when it ran
```

No timing assumption: the swap is ordered by the callback, not by a sleep. This is why resolving
the string better is not the fix.

### 3.5 P4 — positive control

```
POSITIVE CONTROL OK: mirrored=2 into …\ordinary-dest; source object identity and tree unchanged
```

An ordinary destination copies cleanly, the source leaf is the same object afterwards, and the
source tree is unchanged. Any proposed fix must keep this green.

### 3.6 P7 — hard-linked existing destination leaf: the two leaf primitives differ

A legitimate destination whose leaf is already a hard link to a registered source original.

```
rename-based publish (copyVerifyToDest): source original now = "ORIGINAL SOURCE BYTES\n"
SAFE: an atomic rename replaced the destination NAME; the shared object was untouched
truncate-based publish (copyFile, writer.go:761): source original now = "NEW DESTINATION BYTES\n"
REPRODUCED: writing a legitimate destination leaf OVERWROTE a registered source original through a pre-existing hard link
```

**This is a real behavioural split, not a theoretical one.** PR-02's stage-and-rename discipline
already protects the mirror path against hard-linked leaves. The `os.Create`/`O_TRUNC` writers do
not: `copyFile` (`writer.go:761`, called by `WriteChunk` at `writer.go:390`, `span.go:274,303`,
`bagit.go:297`, `burner.go:298`, `migrate.go:86`), `placeEscrowFile` (`escrow.go:430`), and every
bare `os.WriteFile` leaf. A `Root`-relative rewrite must not quietly convert a rename publish into
a truncating one.

### 3.7 P6 / P9 — pathname spellings

| Spelling | Guard verdict | OS resolves to protected bytes? |
|---|---|---|
| source root | REFUSED | — |
| child of source | REFUSED | — |
| `..` round trip | REFUSED | — |
| forward-slash spelling | REFUSED | — |
| trailing dot `Protected.` | REFUSED | yes — but `filepath.Abs` strips the dot, so the guard sees the canonical name and correctly refuses |
| trailing space `Protected ` | REFUSED | no (`Abs` strips it; the OS does not resolve the spaced form) |
| **`\\?\C:\…\Protected`** | **ALLOWED** | **yes** |
| unrelated sibling | ALLOWED | n/a (correct) |

`filepath.Abs` leaves the extended-length prefix in place, so `normPath` yields
`//?/c:/…/protected`, which matches no source root — while the OS resolves it to the protected
file. **Confirmed guard bypass by spelling alone.** Windows 8.3 short names were not separately
exercised; the temp path itself contains `NATHAN~1`, which is consistent throughout and so does
not test the mixed short/long case. **NOT TESTED.**

### 3.8 P8 — a method finding that constrains the design

```
os.SameFile(statTakenBefore_lazy , statAfter) = true
os.SameFile(statTakenBefore_eager, statAfter) = false
CONFIRMED: the lazily-stat'd handle silently re-resolved the NAME and reported 'same file' for a DIFFERENT object
```

On Windows, `os.Stat` loads the file-identity fields **lazily, by re-opening the name**. A
`FileInfo` captured before a mutation and compared afterwards can therefore report "same file"
about a *different* object. Any design of the form *stat → act → `os.SameFile`* is unsound on
this platform unless the identity is forced before the mutation. This is exactly the trap the
prompt warns about, and it is now demonstrated rather than assumed.

### 3.9 Assessed but NOT reproduced — stated as gaps

| Case | Status |
|---|---|
| Symlink (file or directory) alias | **NOT TESTED** — cannot create symlinks on this host without elevation or Developer Mode |
| Destination *becoming unavailable* mid-run (as opposed to substituted) | **NOT TESTED** |
| Mount/bind substitution of a real volume | **NOT TESTED** and deliberately out of bounds — no storage was mounted |
| Windows 8.3 short-name aliasing of a source root | **NOT TESTED** |
| Relative-path escape via `plan.Mapping` values (`plans.go:740`) | **CODE_OBSERVED only** — persisted mapping values are joined without validation; not exercised |
| macOS case-insensitive volumes (`normPath` folds case only when `GOOS == "windows"`, `store.go:38`) | **CODE_OBSERVED only**, no macOS host |

---

## 4. Proposed contract

Two responsibilities, kept separate, in one small shared boundary used by the actual writers —
not duplicated checks at 20 call sites.

### A. Authorize the destination **object**, once, per operation

```go
// destauth.go (new)
type DestRoot struct { /* holds an *os.Root plus the canonical name it was authorized under */ }

// AuthorizeDestination resolves path to a real directory, opens a handle on it, derives the
// canonical name FROM THAT HANDLE, decides against the registered sources, and returns the
// handle that was decided about. The name is never re-resolved afterwards.
func (s *Store) AuthorizeDestination(path string, want MediaExpectation) (*DestRoot, error)
```

Order matters, and it is the whole point:

1. **Open first, then name.** Open the handle, then obtain the canonical pathname *from the open
   handle* (`GetFinalPathNameByHandle` on Windows; `/proc/self/fd` on Linux; `F_GETPATH` on
   macOS). This is what makes the decision be *about the object you are holding*, and it is why
   `EvalSymlinks`-then-write is not equivalent.
2. **Decide three things separately**, never as one boolean:
   - **overlap** — the authorized object is not, and is not beneath, any registered source
     (handle-derived canonical names on both sides);
   - **ancestry** — the authorized object is not an **ancestor** of any registered source. *This
     is the missing direction that §3.1 exploits, and it is a one-line policy addition, not a
     platform problem;*
   - **media identity** — the object matches `want` (the volume/session the operator chose).
     Where identity cannot be resolved, that is an **explicit refusal with a named recovery
     action**, never a silent pass. Unknown identity must not become authorization.
3. **Return the handle.** Everything downstream is relative to it.

### B. Perform every leaf operation **relative to that handle**

```go
func (d *DestRoot) MkdirAll(rel string) error
func (d *DestRoot) CreateStaging(rel string) (*os.File, error)
func (d *DestRoot) Publish(relTmp, relFinal string) error // MUST NOT remove relFinal on failure
func (d *DestRoot) Discard(rel string) error              // failure cleanup, root-relative
func (d *DestRoot) Stat(rel string) (fs.FileInfo, error)
func (d *DestRoot) Chtimes(rel string, at, mt time.Time) error
```

Rules:

- `rel` is validated as a relative, `..`-free, non-rooted, non-drive-qualified slash path
  **before** it reaches the root. Catalog `RelPath`, `mirrorFolderLabels` output, `plan.Mapping`
  values and staged entry names all become `rel` inputs and are all validated at this one place.
- **No absolute pathname is ever reopened after authorization** — including temporary paths,
  cleanup paths and metadata operations. `os.Remove(tmp)` becomes `d.Discard(relTmp)`.
- `Publish` wraps `Root.Rename` and preserves **PR-02's guarantee verbatim**: it never removes
  the destination; a failed publish leaves the existing file byte-identical and returns the
  underlying error. Cross-filesystem publication stays unsupported and stays an error, not a
  silent copy.
- **Do not convert a rename publish into a truncating one.** §3.6 shows stage-and-rename already
  protects hard-linked leaves; `Root.Create` on an existing hard-linked leaf would still truncate
  the shared object. Truncating writers must be migrated *to* `CreateStaging` + `Publish`, not
  merely to `Root.Create`.

### How each named hazard is handled or explicitly bounded

| Hazard | Handling |
|---|---|
| Nested link below the authorized root (§3.3) | **Closed** by root-relative operations — verified refused on this host (§5.2 [1]) |
| Destination alias of a source (§3.2) | **Closed** by deciding on the handle-derived canonical name, not the supplied string |
| Destination is an ancestor of a source (§3.1) | **Closed** by the explicit ancestry rule — the direction the current guard omits |
| Root renamed/substituted after authorization (§3.4) | **Bounded, not absolute.** A retained handle binds operations to the original object where the platform tracks it. On this host the OS additionally refused to rename the directory while the handle was open (§5.2 [7]) — that is a Windows sharing-mode side effect, **not** a guarantee to rely on |
| Existing hard-linked destination leaf (§3.6) | **Bounded** — stage-and-rename preserves the shared object. A caller that must overwrite in place still reaches it; those callers must be converted, and until they are, they remain listed as unconverted |
| Registered sources change during a long job | **Explicitly bounded.** `SourceRoots()` is read at authorization. A source registered *afterwards*, or protected data *moved into* the authorized tree, is not covered by the handle. Re-authorize at batch boundaries; state the residual |
| Media identity | Refuse on unknown identity; recovery is an explicit operator re-selection or re-dock, never an implicit pass |
| Arbitrary privileged topology change (bind mounts, drive letter reassignment, device substitution) | **Not claimed.** Out of the threat model. OS/container read-only mounts and filesystem permissions are the defence in depth here, and the application still validates its own writes |

**A retained handle proves you are writing to *one specific object*. It does not prove that object
is the intended removable medium** — that is the separate media-identity responsibility above, and
it must not be smuggled in as a side effect of holding a `Root`.

---

## 5. The compiler decision — and it is the blocker

### 5.1 Exact API availability, from the primary source

Read from the installed toolchain's own API manifests, `C:\Program Files\Go\api\go1.2{4,5,6}.txt`:

| Go version | `os.Root` surface introduced |
|---|---|
| **1.24** | `os.OpenRoot`, `os.OpenInRoot`, type `Root`; methods `Close`, `Create`, `FS`, `Lstat`, `Mkdir`, `Name`, `Open`, `OpenFile`, `OpenRoot`, `Remove`, `Stat` |
| **1.25** | `Chmod`, `Chown`, `Chtimes`, `Lchown`, `Link`, `MkdirAll`, `ReadFile`, `Readlink`, `RemoveAll`, **`Rename`**, `Symlink`, `WriteFile` |
| **1.26** | *(no new `os.Root` methods)* |

**The baseline's "os.Root requires Go 1.24" note is not sufficient for this design.** The contract
in §4 needs `Root.Rename` (publish), `Root.MkdirAll` (parent creation) and `Root.Chtimes`
(metadata) — **all three are Go 1.25**. The API floor is **Go 1.25**, not 1.24.

### 5.2 Behaviour probe — scope and limits stated up front

Isolated probe, `…/scratchpad/rootprobe`, run with the **locally installed `go1.26.4
windows/amd64`**. It says what this toolchain does on this Windows host. It is **not** evidence
about Unix builds, about other Go versions, or about privileged topology changes.

```
[1] Root.Create("sub/escaped.txt") REFUSED: openat sub/escaped.txt: path escapes from parent   ← closes §3.3
[2] positive control: in-root create OK
[3] Root.Create(`..\escaped2.txt`) REFUSED: path escapes from parent
[4] Root.Create(<absolute protected path>) REFUSED: path escapes from parent
[5] Root.Create("NUL") REFUSED: path escapes from parent                                        ← reserved device names
[6] Root.Rename in-root OK (staging -> published)                                               ← Go 1.25 API present
[7] NOT TESTED: could not rename the root directory: … being used by another process
```

`[7]` is honest and not a pass: holding the handle made Windows refuse the rename outright, so the
substitution case of §3.4 **could not be exercised through `os.Root` here**. Go documents that a
`Root` follows the original directory when it is moved; that behaviour is **unverified by me**.

### 5.3 GO-2026-4970 — and the local toolchain is an affected version

**GO-2026-4970 / CVE-2026-39822**, `os` standard library. Improper symlink handling on **Unix**:
when a path ends in `/` and the final component is a symbolic link, the operation opens the target
**outside the root**. Affected symbols: `OpenInRoot`, `Root.Create`, `Root.Open`, `Root.OpenFile`,
`Root.OpenRoot`, `Root.ReadFile`, `Root.WriteFile`.

| | Version |
|---|---|
| Affected | before **1.25.12**; **1.26.0 – 1.26.4**; 1.27.0-0 – 1.27.0-rc.1 |
| Fixed | **1.25.12**, **1.26.5**, 1.27.0-rc.2 |

**The local probe compiler, `go1.26.4`, is inside the affected range.** My §5.2 results are a
Windows API probe on an affected toolchain and must not be read as proof of a fixed cross-platform
security boundary. Obelisk ships Linux and macOS binaries (`README.md:106`), so this directly
constrains the supported build.

### 5.4 Three different version numbers, kept separate

| | Version | Why |
|---|---|---|
| **Minimum API availability** | **1.25** | `Root.Rename` / `MkdirAll` / `Chtimes` land in 1.25 |
| **Patched compiler for supported builds** | **≥ 1.25.12**, or **≥ 1.26.5** on the 1.26 line | GO-2026-4970 fix; required for the Unix builds we ship |
| **Toolchain used for local probes** | **1.26.4** | Affected. Windows-only observations. Not a security guarantee |

### 5.5 What the current build files do, measured

- **`go.mod` says `go 1.22.2`, with no `toolchain` directive.**
- Probed in `…/scratchpad/apiprobe` with `GOTOOLCHAIN=local`: with `go 1.22.2` in `go.mod`,
  `go build` of a program calling `os.OpenRoot` **succeeded (exit 0) and the binary ran**, while
  `go vet` on the same module reported
  `os.OpenRoot requires go1.24 or later (file is go1.22)`. **The build does not enforce the floor
  but vet does** — adopting `os.Root` without raising the directive leaves the module in a state
  where the compiler is silent and the vet gate fails. The `go` directive must be raised.
- **`Dockerfile:13` pins `FROM golang:1.22-alpine`.** That image **cannot compile `os.Root` at
  all**. This is the concrete, unavoidable cost of the recommendation.
- **`.github/workflows/ci.yml:15` and `release.yml:23` use `go-version: stable`.** Today "stable"
  is already past the floor, but *"stable"* is not a floor — it silently tracks whatever is
  current, in both directions, and pins nothing. It must become an explicit minimum.
- Supported platforms, `README.md:106`: Windows (x64, ARM), Linux (x64, ARM), macOS (Intel, Apple
  Silicon). Release artifacts confirm `darwin-arm64`, `darwin-amd64`.

Nothing above was changed. No compiler was installed, no toolchain downloaded, no `go.mod`,
`go.sum`, CI or Docker file edited, and no dependency introduced.

### 5.6 Recommendation

**Adopt `os.Root`, and raise the build floor to Go 1.25.12 (or ≥ 1.26.5).**

*Compatibility cost, stated plainly:* `go.mod` `go 1.22.2` → `go 1.25` (with an explicit
`toolchain` line); `Dockerfile` `golang:1.22-alpine` → a patched 1.25.12+/1.26.5+ image; CI/release
`go-version: stable` → an explicit pinned minimum. Anyone building Obelisk from source needs a
2025-or-later Go. For a tool whose entire promise is *never write into source data*, on a codebase
that already declines dependencies to keep the one-static-binary bargain, that is the right trade.

*The alternative, briefly.* Keeping the 1.22 baseline means either a dependency
(`github.com/cyphar/filepath-securejoin`, or `golang.org/x/sys` for `openat2`) or a hand-written
per-platform layer (`RESOLVE_BENEATH` on Linux; handle-relative opens on Windows; neither covers
macOS well). The dependency route contradicts the documented dependency bargain in `go.mod` and
`docs/ARCHITECTURE.md` and still would not give a maintained, audited boundary on all three
platforms. The hand-rolled route means owning security-critical platform code that upstream Go
already maintains and patches — including patching advisories like GO-2026-4970 for us. Neither is
worth it to avoid a compiler bump.

**This is the one decision required to proceed.** It is an owner call because it changes the
supported build environment, and it blocks implementation: I will not substitute a "temporary"
string check and present it as the fix.

---

## 6. Limitations that must ship with any claim

### 6.1 Threat and platform model actually supported

Supported: an operator misconfiguration or a hostile *pathname* — an alias, a nested link, an
ancestor destination, a rename between check and use — under the process's own privileges.

**Not** supported and not claimed: privileged topology changes (bind mounts, mount substitution,
drive-letter reassignment, device swap), an attacker with write access to the data directory or
the catalog, or anything that requires trusting a foreign medium's manifest.

OS-level read-only mounts and filesystem permissions on the source volumes are **defence in
depth** and are the correct answer to the unsupported cases. The application still validates its
own writes; neither replaces the other.

### 6.2 Platform coverage of the evidence

Windows 11 only. Symlinks NOT TESTED. macOS case-folding NOT TESTED (`normPath` folds case only on
Windows — `store.go:38` — so a case-insensitive macOS volume is unprotected by the current guard,
observed in source, unexercised). Linux NOT TESTED. **Windows `-race` remains unestablished**
(needs cgo) and **no CI ran**.

### 6.3 External helpers are a separate boundary

`tar`, `par2`, `gpg`, `dvdisaster`, the operator-configured burn command (`burner.go:182`
`runShell`) and `growisofs` receive **path operands** and run as separate processes:
`adopt.go:177,194,200`, `pipeline.go:954,1292`, `privacy.go:77`, `writer.go:708,717`,
`dvdisaster.go:138`, `escrow.go:189`, `span.go`/`stenc.go`/`tape.go`.

**A Root handle held by the parent Go process confines nothing about a child process.** Handing a
helper a directory path is handing it the whole pathname namespace. Confining those requires a
different mechanism (a working directory the helper cannot leave plus argument discipline, or OS
sandboxing) and is recorded here as an out-of-scope boundary. Tar **member** confinement is
OB-008/PR-08, not OB-004.

---

## 7. Bounded first production patch

Prefer one **integrated** production path over a large unused abstraction — and do **not** claim
OB-004 is fixed while other writers still use the old boundary.

**Slice 1 (proposed, not implemented here).**

| File | Change |
|---|---|
| `destauth.go` *(new)* | `DestRoot`, `Store.AuthorizeDestination`, rel-path validation, `MkdirAll` / `CreateStaging` / `Publish` / `Discard` / `Stat`. `Publish` preserves PR-02's no-delete guarantee |
| `store.go` | Add the **ancestry** direction to the source-boundary policy and expose it to `AuthorizeDestination`. `AssertOutsideSources` stays, still called by the 19 unconverted callers, and is **explicitly redocumented as a preflight message-quality check, not the security boundary** |
| `mirror.go` | `MirrorToVolume` (`:132`) authorizes once and drives `copyVerifyToDest` root-relative; `writeVolumeInventory` (`:385`) writes its sidecar through the same `DestRoot`; cleanup at `:240,248,253` becomes `Discard` |
| `incremental.go` | `BackupChanges` (`:300`) — free, because it shares `copyVerifyToDest` |

That converts **one complete, real write path end to end** (staging, publish, sidecar, cleanup)
plus the caller that shares its leaf primitive. Everything else keeps today's behaviour and is
listed, by name, as unconverted.

**Explicitly still on the old boundary after Slice 1**, and to be said so in the PR description:
`plans.go:682`, `writer.go:303`, `writer.go:632`, `span.go:134`, `pipeline.go:165/169/174/1020`,
`appbackup.go:257/520`, `bagit.go:238`, `escrow.go:344`, `recoverykit.go:38`,
`finalize.go:169/249`, `dock.go:281`, `quarantine.go:140`.

---

## 8. Regression acceptance criteria

Each maps to a reproduction above. Every one must assert the registered source is unchanged —
**by object identity forced before the mutation** (§3.8), not by a late `os.SameFile`.

| Test | Asserts | From |
|---|---|---|
| `TestDestAuth_AncestorDestinationRefused` | A destination containing a registered source is **refused**; no `.mnemo_tmp` ever appears inside a source | §3.1 |
| `TestDestAuth_JunctionAliasOfSourceRefused` | A junction aliasing a source root is refused, with the same verdict as the real path | §3.2 |
| `TestDestAuth_NestedLinkLeafRefused` | Root authorized; leaf through a nested junction refused; protected tree unchanged | §3.3 |
| `TestDestAuth_RootSubstitutedAfterAuthorization` | Deterministic barrier (callback, **no sleep**); post-swap writes do not reach the protected source | §3.4 |
| `TestDestAuth_OrdinarySafeDestination` | **Positive control** — copies cleanly, source object identity and tree unchanged | §3.5 |
| `TestDestAuth_HardLinkedDestinationLeaf` | Stage-and-rename leaves the shared object intact; a converted writer does not regress to truncation | §3.6 |
| `TestDestAuth_ExtendedLengthPrefixRefused` | `\\?\`-prefixed source spelling is refused | §3.7 |
| `TestAtomicRename_*` (existing) | **Unchanged and still green** — PR-02's guarantee survives | PR-02 |
| Skipped-with-reason | Symlink cases must **skip loudly as NOT TESTED** where links cannot be created — never silently pass | §3 capability table |

---

## 9. Remaining PR-04 slices, and non-scope

**Remaining PR-04 slices:** convert the writers listed in §7; media-identity binding for
`MirrorToVolume` / `SpanWriteNext` / `WriteChunk` / `FinalizeVolume`; `plan.Mapping` rel-path
validation (`plans.go:740`); macOS case-folding in the boundary; the OB-007 prerequisite —
routing these mutations through the store-owned authority rather than 20 independent entry points.

**Explicit non-scope, unchanged by this pass:** a new extraction engine; a general archive audit;
tar member validation and extraction confinement (OB-008/PR-08); the full OB-007 ownership rewrite
and process locking (PR-06); the native Windows tar writer; the containment review's optional
F1/F2 tests; OB-009/OB-010 recovery work; any UI change.

**OB-006 pull-forward option.** The historical roadmap permits pulling OB-006 keystore protection
forward if PR-04 becomes a larger blocked workstream. PR-04 **is** blocked, on §5.6, and OB-006 is
a P0 whose work does not depend on that decision. Noting the option as the roadmap allows — **not
switching tasks, not renumbering, and not starting it.** That is the owner's call.

---

## 10. Checks executed in this session — reported separately

**Kept apart from the prior 240 pass / 7 fail / 4 skip evidence, which was NOT rerun and is
NOT claimed here.** Production files are unchanged, so no full-suite run was required.

| Check | Where | Result |
|---|---|---|
| `go build ./...` on a disposable `git archive HEAD` export | scratch | pass |
| `go vet .` on that export | scratch | pass |
| OB-004 probes P1–P9 | scratch export of `0dc7d53` | reproductions as recorded in §3; `TestProbeP1` failed on its own inverted assertion (the §3.8 lazy-identity trap) and was superseded by `TestProbeP1B` |
| `os.Root` behaviour probe | `…/scratchpad/rootprobe`, `go1.26.4` | §5.2, `[7]` NOT TESTED |
| `os.Root` API-gating probe | `…/scratchpad/apiprobe`, `GOTOOLCHAIN=local` | §5.5 |

The **seven known Windows Unicode compatibility failures stand** (OBX-006 remains open). **Windows
race testing and CI remain unestablished** — neither was executed here.

---

## 11. Preservation check

- Branch `fix/ob-004-source-boundaries` created at `0dc7d5399c6889e015aebc9ba02e694a909d6e9c`.
- `git status` at exit shows **no tracked modification, nothing staged**; the four expected
  untracked handoff/prompt paths are untouched; the only addition is this report.
- No production source, test, UI, dependency, toolchain or CI file was modified.
- No commit, push, merge, fetch, pull, reset, stash, clean, rebase, or ref change of any kind.
- Historical reports and frozen handoff documents are unmodified.

---

## 12. The single decision required

**Approve raising the supported build floor to Go 1.25.12 (or ≥ 1.26.5), including replacing
`Dockerfile`'s `golang:1.22-alpine` and pinning CI's `go-version: stable` to an explicit
minimum** — or direct the narrow-platform/dependency alternative in §5.6 instead.

Implementation of PR-04 waits on that answer.
