# OBX-006 — Windows Unicode archive-build compatibility

**Outcome: `DESIGN_DECISION_REQUIRED`.**

**Date:** September 7, 2026
**Branch:** `fix/obx-006-windows-unicode-tar`
**Parent commit:** `c880c7d3afd7e61e367b8ee3aa64068af540c768` (OBX-001 fixture repair, published)
**Scope of change:** test-only. One new file, `tar_unicode_names_test.go`. **No production
code was changed**, because no supported, lossless, bounded correction exists at the
helper-invocation boundary for the required cases — §5 and §7 below give the evidence.

The regressions are added anyway, and they **fail**. That is deliberate: they make the
compatibility boundary visible, name it per case, and are what a future fix will be graded
against. Nothing was skipped, softened, or turned into a green empty package.

## 1. Checkpoint

Verified before any work: git root `C:/Users/Nathaniel/Documents/Software
Development/Mnemosyne/mnemo-go`, branch `fix/obx-001-windows-fixture`, HEAD
`c880c7d3afd7e61e367b8ee3aa64068af540c768`, nothing staged, no tracked modifications, no
rebase/merge/cherry-pick in progress. `fix/obx-006-windows-unicode-tar` did not exist and
was created from exactly that commit. No fetch, pull, reset, stash, clean, rebase, merge or
force-switch was run; no published branch was moved.

Untracked handoff and prompt material was present and is preserved untouched:
`docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/`,
`docs/OBELISK_POST_PR02_FIX_PROMPTS_2026-09-07/`,
`docs/development/PR01_REVIEW_ADDENDUM/`, `docs/development/UI_FEATURE_MAP-2026-09-06.md`.
The incorrectly extracted addendum folder was **not** repaired.

Prior published work, re-verified unchanged at the end of this task (local `==` remote):

| Branch | Commit |
|---|---|
| `main` | `d97809b2f730e09960632e2943e562531fa095a3` |
| PR-01 `fix/ob-001-catalog-open` | `4cd867b2ddb26c945f7c74faee8ef32780743be7` |
| PR-02 `fix/ob-003-safe-replacement` | `406ed2365b074398f4ef80094951753b961cf757` |
| OBX-001 `fix/obx-001-windows-fixture` | `c880c7d3afd7e61e367b8ee3aa64068af540c768` |

PR-01 touched `store.go`, `catalog_open_test.go` and five docs; PR-02 touched
`appbackup.go`, `mirror.go`, `atomic_replace_test.go` and six docs; OBX-001 touched
`tar_names_test.go` and three docs. None of those files is modified here.

## 2. Changed-file identities (vs `c880c7d3`)

| File | Status | Git blob |
|---|---|---|
| `tar_unicode_names_test.go` | **added** (397 lines) | `b31d8b59de7c7423697306106b4c4b8f53f18aa2` |
| `docs/development/reviews/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md` | added | this file |
| `docs/development/OB_STATUS.md` | appended (dated update) | see commit |
| `docs/development/NEXT_ACTIONS.md` | appended (dated update) | see commit |

Unchanged, confirmed by blob identity:
`tar_names_test.go` = `8d8789e346dc7bb2b73a7ac3ab356c8f33ebd487` (OBX-001 platform guard and
every existing assertion intact), `pipeline.go` = `f48509475bebe4a4321e82da3ebade6de1ba2949`,
`writer.go` = `dc04e845cdca4b216d55638aba930ac50ac0b992`.

## 3. Selected executable and environment

Pinned by `nativeTools` (`payload_naming_test.go:22-40`) and resolved identically by
`App.tool` (`pipeline.go:188-200`) from `cfg.Tools["tar"]`:

| | |
|---|---|
| Path | `C:\Windows\System32\tar.exe` |
| SHA-256 | `9B77D4C912F2EDAE8C241D0ECE1094D2AC068B084269CEAF85D7C7B085D2AE86` |
| Size | 92,176 bytes |
| FileVersion | `3.8.4 (WinBuild.160101.0800)` / ProductVersion `10.0.26100.7623` |
| `--version` | `bsdtar 3.8.4 - libarchive 3.8.4 zlib/1.2.13.1-motley liblzma/5.8.1 bz2lib/1.0.8 libzstd/1.5.7 cng/2.0 libb2/bundled` |
| OS | Windows 11 Pro, `10.0.26200` |
| Active ANSI code page (`HKLM\...\Nls\CodePage\ACP`) | **1252**; OEMCP 437 |
| Toolchain | `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, **`GOTMPDIR` unset** (real system temp) |

The binary carries **no discoverable embedded application manifest** — a byte scan for
`activeCodePage` and `<assembly` found neither — so nothing in the image overrides the
process ANSI code page to UTF-8.

Command shapes actually issued by the product (`pipeline.go:1033-1038`), working directory
inherited from the app, environment inherited unmodified (`run`, `pipeline.go:1217-1232`):

```
tar.exe --format=posix -cf <staging>/<name>/<name>.tar -C <staging>/<name> manifest-sha256.txt
tar.exe --format=posix -rf <staging>/<name>/<name>.tar -C <SrcRoot> --null -T <staging>/<name>/filelist.txt
```

The list (`pipeline.go:975-982`) is `RelPath + "\x00"` per file, written **as UTF-8, no BOM,
NUL-separated** — e.g. `café.txt` is `63 61 66 c3 a9 2e 74 78 74 00`.

Restore (`writer.go:687-694` plaintext, `writer.go:712-719` encrypted) puts member names on
**argv** after a literal `--`, never in a list file.

Only the code-page and locale facts above were collected. The full environment was not
dumped.

## 4. Reproduced failure

At the parent commit, unmodified, real temp, native exit code checked:

`go test -count=1 -run '^(TestBuildRestore_HostileFilenamesRoundTrip|TestBuildFilelist_IsNulDelimited)$' .` → **exit 1**

```text
tar_names_test.go:95:  BuildChunk with hostile filenames: tar.exe failed: exit status 1:
    tar.exe: : Couldn't visit directory: No such file or directory
tar_names_test.go:157: BuildChunk: tar.exe failed: exit status 1:
    tar.exe: : Couldn't visit directory: No such file or directory
```

Confirmed here, not taken from the OBX-001 report.

## 5. Diagnosed boundary

Probed with disposable synthetic fixtures in the session scratchpad (never in the
repository), each result checked with **Go's `archive/tar` reader**, never with the same
executable's listing output.

The five boundaries behave **differently**, which is why locating the failure mattered:

| | Boundary | Result |
|---|---|---|
| **A** | Go pathname and list-file bytes | **Lossless.** All six fixture names, including a supplementary-plane name and `日本dir/ascii.txt`, are created on NTFS and read back byte-identical. Go is not the problem. |
| **B** | Child-process argv and working directory | **Split.** A bare or ASCII-rooted *leaf* name survives argv intact — `café.txt`, `日本語.txt`, `𝕏supp.txt`, `star★.txt`, `asciidir/日本語.txt` and `a/b/日本語.txt` all archive correctly by name and bytes. But any path the helper resolves through the C-runtime ANSI APIs is converted in the process code page and loses anything outside CP1252: `-C` with a Unicode directory, `-f` with a Unicode archive path, and a **directory component** inside the tree (`日本dir/ascii.txt` → `Cannot stat: Invalid argument`). CP1252-representable Unicode passes everywhere (`-C srccafé`, `cafédir/ascii.txt` both fine). |
| **C** | Helper's decoding of names read through `-T` | **The primary failure.** The list is decoded in the process ANSI code page (CP1252), not UTF-8. Decisive pair: the same `café.txt`, same directory — UTF-8 list `63 61 66 c3 a9 …` → **exit 0 archiving `cafÃ©.txt`**; CP1252 list `63 61 66 e9 …` → exit 0 archiving `café.txt`. `日本語.txt` has no CP1252 form and fails outright. |
| **D** | Names written into archive headers | **Lossless.** `--format=posix` records whatever the helper actually opened as correct UTF-8; Go's reader recovers `café.txt`, `日本語.txt`, `𝕏supp.txt`, `star★.txt` exactly. The headers are not the defect. |
| **E** | Selected-member restoration | **Follows B**, because members go on argv. `-xf … -C out -- <member>` restores every Unicode *leaf* name correctly, byte-exact, with nothing extra. It fails only for a member beneath a non-CP1252 directory component (`日本dir/ascii.txt: Not found in archive`). |

**The mechanism.** UTF-8 list bytes are read back through the ANSI code page. `c3 a9` is not
`é` there; it is `Ã` + `©`. So the product asks for one file and the helper looks for
another. Where that other name happens to exist, the build **succeeds with the wrong file**.

**Upstream justification and its limit.** This is consistent with libarchive converting
multibyte path input to wide characters using the code page derived from the process locale
(`archive_string.c`'s current-code-page logic), while argv leaf names reach the wide
filesystem API intact. **That reading of upstream source does not establish this Microsoft
vendor build's behaviour** — the shipped binary was not built from inspected sources and
carries no manifest that would explain the split. Everything asserted above is asserted from
the executed probes, not from the source reading. Recorded as a limitation.

### 5.1 The wrong-file defect, and what currently contains it

`TestBuildChunk_WrongFileSelection_LookalikeNeighbour` puts `café.txt` and `cafÃ©.txt` side
by side with distinct bytes and selects only `café.txt`. Observed:

```text
stage verification: package contains "cafÃ©.txt", which is not a cataloged member
of this package (unexpected extra member)
```

So the build **fails safe** — but the guard is `verifyTarContents` (`pipeline.go:834-885`),
which runs only when `build_verify` is `full` or `contents` (`pipeline.go:1013`). The
default normalises to `full` (`integrity.go:50-58`), so the shipped default is protected.
**Under the `FAST` preset (`build_verify: none`) nothing compares members to the catalogue**,
and the wrong file would be written to a 30-year medium under the right file's name. That is
a pre-existing exposure this task surfaces; it is not introduced here and is not fixed here.

## 6. Candidate remedies tested, and why each was rejected

All executed against the pinned executable, with the archive read back by Go's reader.

| # | Candidate | Result |
|---|---|---|
| 1 | UTF-8 list (**current production**) | Wrong file or `Couldn't visit directory`. |
| 2 | UTF-8 list **with BOM** | Fails; the BOM is treated as part of the first name. |
| 3 | **UTF-16LE** list, with BOM, newline-separated | Fails; the reader is byte-oriented. |
| 4 | `-T -` (list on **stdin**) instead of a file | Fails identically — same reader, same code page. |
| 5 | **Absolute** UTF-8 paths in the list | Fails identically. |
| 6 | `--options hdrcharset=UTF-8` | **No effect on `-T` input.** It is an output-header option; the CP1252 misread is unchanged. |
| 7 | `LANG` / `LC_ALL` / `LC_CTYPE` set to UTF-8 forms on the **child only** (four combinations, inherited environment otherwise preserved) | **No effect.** `café.txt` still resolves to `cafÃ©.txt`; `日本語.txt` still fails. The Windows CRT takes its locale from the OS, not these variables — as the task anticipated. |
| 8 | `\\?\` extended-length prefix on `-C` / `-f` | **Worse.** Fails, and the helper reports the prefix mangled to `\?\`. |
| 9 | **CP1252-encoded list** | Works *only* for CP1252-representable names. `日本語` has no representation at all. Rejected as a fix: it is exactly the lossy re-encoding the task forbids, and a CP1252 probe is diagnostic evidence, not Unicode support. |
| 10 | **8.3 short paths** (`GetShortPathNameW`) for `-C` and `-f` | **Works**, and is lossless for package semantics — those paths are the working directory and the output file, and never appear in a member name. But it fixes only the root-path cases, not any filename, and 8.3 generation can be disabled per volume. Component of the alternatives in §8, not a fix on its own. |
| 11 | Names on **argv** instead of `-T` | Works for leaf names (§5 boundary B) — but see below. |

Nothing machine-wide was touched: no system locale or registry change, no "Beta: UTF-8"
option, no modification to `tar.exe` or its manifest, no tool installed or downloaded, no
shell interpolation, no source file renamed, transliterated or re-encoded, and no fallback
to recursively archiving the source instead of the planned membership.

**Why argv is not the bounded fix.** It is the only mechanism that carries a non-CP1252
*filename*, but (a) it still cannot carry a non-CP1252 **directory component**, which is a
required case and the common shape in a real tree; (b) a package's member list is unbounded
while a Windows command line is capped at 32,767 UTF-16 units, so it would have to be split
into repeated `-r` appends — a change in invocation count, error propagation and streaming
behaviour, and the very "unbounded command line / per-file invocation" shape the task rules
out. A partial move to argv would turn one demonstrated failure into a subtler one that
passes for leaf names and still loses Unicode directories.

**Conclusion.** There is no supported, lossless, bounded, invocation-local correction that
makes this helper archive the required names. §7 of the task therefore applies.

## 7. Regressions established (they fail; that is the record)

New file `tar_unicode_names_test.go`. Names are Go string literals, so the fixture bytes do
not depend on any shell's default text encoding. Fixture creation is checked explicitly:
each tree is walked back and compared byte-for-byte, and a filesystem that *normalises* a
name reports a fixture limitation rather than a pass — a branch that cannot fire on
Windows/NTFS, so it cannot hide this defect.

Subtest labels are ASCII on purpose. Go derives `t.TempDir()` from the test name, so a
Unicode subtest name silently put Unicode into the staging **and** source-root paths as well
as the filename; the first run did exactly that and misattributed three failures. Each case
now varies one thing.

Membership is checked with **Go's `archive/tar` reader**: the exact intended set is the
BagIt payload manifest `manifest-sha256.txt` (the documented first member) plus one member
per catalogued file, each content hash matching, **no duplicates and no unselected
neighbour**. A zero exit status is never accepted as evidence.

`go test -count=1 -v ./...`, uncached, native exit code checked:

| Test / subtest | Result | Cause |
|---|---|---|
| `TestBuildChunk_UnicodeFilenames_ExactMembers/ascii` (`ordinary.txt`) | **PASS** | — |
| `…/accented` (`café.txt`) | **FAIL** | boundary C: list misread; the file is not found (no decoy present) |
| `…/japanese` (`日本語.txt`) | **FAIL** | boundary C: no CP1252 form |
| `…/supplementary` (`𝕏supplementary.txt`, U+1D54F) | **FAIL** | boundary C |
| `…/bmp-symbol` (`star★.txt`) | **FAIL** | boundary C |
| `…/unicode-dir-component` (`日本dir/ascii.txt`) | **FAIL** | boundary C, and unreachable via B either |
| `TestBuildChunk_WrongFileSelection_LookalikeNeighbour` | **FAIL** | build archived `cafÃ©.txt`; stage verification rejected the package (§5.1) |
| `TestBuildChunk_UnicodeSourceRoot` (`source 日本★`) | **FAIL** | boundary B: `could not chdir to '…\source ???'` |
| `TestBuildChunk_UnicodeStagingDir` (`staging 日本★`) | **FAIL** | boundary B: `Failed to open '…\staging ???\UNI-STAGING.tar'` |
| `TestBuildRestore_UnicodeRoundTrip` | **FAIL** | never reaches restore — build fails first |

The round-trip test deliberately separates a successful build from a successful **complete**
round trip: whole-tree restore then one selected member at a time, comparing restored paths
and bytes and rejecting anything extra. It cannot report on restore yet, because the build
never produces a package. **Restore is therefore NOT PROVED through the product for Unicode
names.** The out-of-band probe (§5, boundary E) exercised the identical command shape
`RestoreChunk` builds and found leaf names restore byte-exact while a member under a
non-CP1252 directory is `Not found in archive` — but that is probe evidence about the
helper, not an end-to-end product result, and it is not claimed as one.

**No OBX-006 end-to-end compatibility is claimed.**

## 8. Design decision required

**The demonstrated compatibility boundary.** With `C:\Windows\System32\tar.exe`
(bsdtar 3.8.4) on an ACP-1252 host, the archive-build path can carry:

- any name the process code page can represent — ASCII and CP1252-representable Unicode; and
- via argv only, any *leaf* filename including non-CP1252 and supplementary-plane characters.

It **cannot** carry, by any invocation-local means tested: a non-CP1252 character in any
**directory component**, in the **source root**, or in the **staging/output path** (the last
two only via 8.3 short paths, where the volume still generates them), and it cannot carry
any non-CP1252 name at all through **`-T`**, which is the mechanism the product uses.

**Alternatives, smallest first.**

1. **Build the tar with Go's `archive/tar` writer on Windows only**, keeping the external
   helper everywhere else and for all reads. Removes boundaries B and C completely; header
   names (D) are already correct and would stay so. *Impact:* the build path stops being
   "external tar" on one platform — two code paths to keep honest; `--format=posix`
   equivalence, the manifest-first member order and streaming/bounded-memory behaviour must
   be reproduced and proven byte-comparable; `cfg.Tools["tar"]` would no longer select the
   builder on Windows. No new dependency, no new helper. Restore would still use the helper
   and would still fail for non-CP1252 directory components until item 2.
2. **Extend that to the restore path** (extract selected members with Go's reader), which is
   what actually closes the round trip and would also give OB-008 the extraction confinement
   it currently lacks — today confinement rests entirely on the external tar's defaults.
3. **Ship a known-good helper** (a libarchive or GNU tar build with UTF-8 path handling).
   *Impact:* packaging, provenance, signing, update and deployment burden on a 30-year
   archival tool; rejected here without a decision.
4. **Document the boundary and refuse early** — detect non-representable names at plan time
   and fail loudly instead of at `tar`. Honest, cheap, and *not a fix*; worth doing as a
   stopgap alongside 1 whichever way the decision goes.

**Recommended smallest next implementation scope:** item 1, Windows-only Go tar **writer**
behind the existing helper-selection boundary, with the tests in this patch as its
acceptance criteria and a byte-comparison against the helper's output for ASCII packages to
prove format equivalence. Item 2 follows only if 1 lands clean.

This is a decision, not a task to start unasked. **PR-03's P0 durability work stays queued
and is not displaced by it.**

## 9. Verification performed

Windows 11, `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, real system temp (**no `GOTMPDIR`
override**), disposable fixtures, native exit codes checked in PowerShell.

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **pass** (exit 0) |
| Vet | `go vet ./...` | **pass** (exit 0) |
| Format | `gofmt -l` over all tracked `*.go` plus the new file | **clean** |
| New Unicode regressions | `go test -count=1 -v -run '^TestBuildChunk_Unicode…\|…WrongFileSelection…\|…UnicodeRoundTrip$' .` | **exit 1**, per-case results in §7 |
| Existing hostile-name tests | `go test -count=1 -run '^(TestBuildRestore_HostileFilenamesRoundTrip\|TestBuildFilelist_IsNulDelimited)$' .` | **exit 1**, unchanged cause (BuildChunk) |
| Full suite, uncached | `go test -count=1 -v ./...` | **exit 1** — **212 top-level pass / 7 fail / 4 skip**, plus 15 passing and 5 failing subtests |
| Windows race | `go test -race …` | **NOT TESTED — unavailable** |

The 7 failures are the 2 pre-existing (`TestBuildRestore_HostileFilenamesRoundTrip`,
`TestBuildFilelist_IsNulDelimited`) plus the 5 added here. The 4 skips are unchanged:
`TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`, `TestVolumeHealth_SystemDisk`,
`TestTreeExpansionBudget`. The historical "212 pass / 2 fail / 4 skip" was never a target;
the pass count is unchanged because every new test is a new failure.

**Windows race testing remains NOT TESTED** — verified, not assumed: `go test -race` exits 2
with `-race requires cgo; enable cgo by setting CGO_ENABLED=1`, `CGO_ENABLED=0`, and no
`gcc` on PATH. Nothing was installed, no WSL or other platform was substituted, and the
`GOTMPDIR` override was not used. **No CI ran; no cross-platform result is claimed.** All
red/green comparison was done in disposable scratchpad copies; the working checkout was
never reverted.

## 10. Security diagnostics — observed and withheld

While probing malformed-for-the-code-page `-T` input, the helper's `Couldn't visit
directory:` diagnostics **printed text that was not present in the supplied list file**. It
tracked the decode failure, not the filename content.

This remains an **unconfirmed hypothesis**. No disclosure of sensitive data has been
demonstrated, no attempt was made to characterise or steer it, no real secret was probed or
placed anywhere for that purpose, and all fixtures were synthetic. The raw output is
**retained locally in the session scratchpad and deliberately excluded from this checkpoint
and from any published material**. No upstream or public security report is filed by this
task; whether one is warranted is a separate decision.

## 10a. Decision recorded 2026-09-07 (direction only — not implemented here)

The design decision requested in §8 was taken. Recorded here so the investigation and its
outcome stay together; the containment actually implemented is reported separately in
[OBX-006-CONTAINMENT-2026-09-07.md](OBX-006-CONTAINMENT-2026-09-07.md).

**A. PR-03 / OB-002 remains the next substantial production workstream.** Nothing below
displaces it.

**B. A streaming Go `archive/tar` writer — §8 option 1 — is the approved direction, and is
NOT built yet.** Its implementation, metadata contract and restore compatibility need their
own scope. Constraints it must satisfy when that scope opens:

- **Replace only Windows tar *construction*.** Not the whole build pipeline, not the read,
  verify, restore or par2 paths.
- **Share one explicit archive-member input contract** with the external-tar path, so both
  construct from the same declared membership rather than from two divergent notions of what
  is in the package.
- **Stream the selected files.** Preserve exact names and the exact expected membership;
  bounded memory; no whole-file buffering.
- **Preserve the existing logical layout** — the BagIt payload manifest stays the first
  member, and the on-medium package layout is unchanged.
- **Define metadata and entry-type support explicitly, and reject what is out of scope**
  rather than silently emitting whatever a library defaults to. Sparse files, hard and
  symbolic links, ACLs and NTFS alternate data streams are each a deliberate decision to
  support or refuse, decided in that scope and written down — not inherited by accident.
- **Check tar finalisation and underlying file completion separately.** A closed
  `tar.Writer` is not a durable file; both must be confirmed.
- **Keep content verification on during initial deployment**, independent of tier, until the
  writer has field evidence.
- **Preserve helper-selection transparency.** The operator must still be able to see which
  mechanism built a package; `cfg.Tools["tar"]` stopping being the builder on Windows is a
  visible change, not a silent one.
- **Validate external-reader interoperability** — GNU tar, bsdtar and Go's reader must all
  read the output, so a 30-year package is not readable only by this program.
- **Track Unicode archive/output-path restoration separately.** The writer fixes
  construction; boundary E (restore) is its own item and is not closed by it.

**C. Before PR-03, temporarily refuse Windows external-tar builds whose effective mode
disables content verification.** Implemented — see the containment report.

**D. The containment does not solve Unicode support and does not validate packages already
staged or written. OBX-006 remains open.**

## 11. What remains unproved

- That restoration works end to end for Unicode names **through the product** — the build
  never gets there. Probe evidence for the identical command shape is in §5/§7, and is not
  a product result.
- That every Windows `tar` behaves this way. Only the pinned `C:\Windows\System32\tar.exe`
  build was tested. GNU/MSYS tar was not tested (`nativeTools` avoids it for an unrelated
  reason: it reads a `C:\…` argument as an rsh host).
- Behaviour under any other active code page, including 65001, and under the machine-wide
  UTF-8 option — deliberately not changed.
- POSIX behaviour. Nothing here was executed on Linux or macOS; the new tests are
  platform-neutral by construction, and the non-Windows path is untouched, but that is an
  argument, not evidence.
- The upstream libarchive explanation for the argv/`-T` split (§5), for this vendor build.
- Long-path interaction with Unicode names, and 8.3 short-name availability on volumes where
  generation is disabled.
- OBX-001's `windows-latest` CI lane, still outstanding — **OBX-001 remains open**.
