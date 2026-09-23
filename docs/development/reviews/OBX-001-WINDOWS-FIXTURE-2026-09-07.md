# OBX-001 — Windows filename fixture repair (test-only)

**Date:** September 7, 2026
**Branch:** `fix/obx-001-windows-fixture`
**Parent commit:** `406ed2365b074398f4ef80094951753b961cf757` (PR-02 evidence commit, published)
**Scope:** test-only. One executable file changed: `tar_names_test.go`.
**Outcome:** the fixture defect is repaired and the two tests now reach product code —
**which immediately exposed a real, previously masked Windows failure.** That failure is
filed separately as **OBX-006** (§5) and is **not fixed here**.

## 1. What was wrong

`hostileNames()` listed `"tab\there.txt"` unconditionally, while the newline and long-path
fixtures sat inside the existing `runtime.GOOS != "windows"` guard. Windows filename rules
exclude TAB as well as newline, which the guard's comment did not say. So on Windows both
tests died in `makeHostileSource` at `os.WriteFile`, **before a single line of product code
ran**.

The two failures were therefore not evidence about the product at all. They were the
fixture failing to exist.

## 2. The change

`tar_names_test.go` only — the TAB name moved into the existing platform guard, plus the
inaccurate comment above `hostileNames` corrected. No production code, no other test, no
dependency, toolchain, CI or pictograph change. `gofmt` clean.

```text
 // hostileNames returns tree-relative names legal for the test platform and awkward for a
 // shell. Windows filename rules exclude TAB and newline, so those fixtures stay in the
 // non-Windows group, where the existing long-path fixture also remains. Unicode, leading
 // dashes, spaces and quotes are valid Windows names and stay unconditional — gated here
 // rather than left to fail on a box the developer is not looking at. Gating is not a
 // softer assertion: a name the platform will not create makes the fixture die in
 // os.WriteFile, so the product code under test never runs at all.
 func hostileNames() []string {
 		"spaces and 'quotes'.txt", // shell-quoting bait
-		"tab\there.txt",           // a control character that is not a newline
 	}
 	if runtime.GOOS != "windows" {
 		names = append(names,
 			"line\nbreak.txt",          // the -T list splitter
 			"sub/two\nlines\nhere.txt", // more than one split, in a subfolder
+			"tab\there.txt",            // a control character that is not a newline
```

The comment deliberately does **not** claim that Windows prohibits every ASCII control
character, nor that long paths are universally impossible there. Only TAB and newline were
tested, and the long-path fixture's own pre-existing comment is left as written.

File identity:

| File | Git blob |
|---|---|
| `tar_names_test.go` (parent) | `1e3ff776826d7e8d4c5c4da9f00e4468fb99bb60` |
| `tar_names_test.go` (this patch) | `8d8789e…` — see the commit for the recorded blob |

## 3. Fixture coverage kept

Retained on **Windows**: `ordinary.txt`, `--dashy.txt`, `sub/--also-dashy.txt`, `-x`,
`ünïcødé★ 日本語.txt`, `spaces and 'quotes'.txt`, plus the whole-package restore, the
selective `--dashy.txt` restore, and the "restored only what was asked for" assertion.
Retained on **POSIX**: both newline names, the long path, and now the TAB name.
Nothing was skipped, sanitized, or softened; no archive or restore assertion was weakened.

The Unicode name is a **valid** Windows filename and was deliberately left unguarded —
guarding it would have hidden OBX-006.

## 4. Before / after, executed

Windows 11, `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, real system temp
(**no `GOTMPDIR` override**), disposable fixtures, native exit codes checked.

**Before** — `go test -count=1 -v -run '^(TestBuildRestore_HostileFilenamesRoundTrip|TestBuildFilelist_IsNulDelimited)$' .` → exit 1:

```text
tar_names_test.go:80: cannot stage "tab\there.txt": open ...\005\tab	here.txt:
    The filename, directory name, or volume label syntax is incorrect.
tar_names_test.go:145: cannot stage "tab\there.txt": open ...\005\tab	here.txt:
    The filename, directory name, or volume label syntax is incorrect.
```

**After** — same command → still exit 1, but **the fixture now builds and the failure has
moved into product code**:

```text
tar_names_test.go:94:  BuildChunk with hostile filenames: tar.exe failed: exit status 1:
    tar.exe: : Couldn't visit directory: No such file or directory
tar_names_test.go:156: BuildChunk: tar.exe failed: exit status 1:
    tar.exe: : Couldn't visit directory: No such file or directory
```

That is the intended result of this task: the tests stopped lying about why they failed.

## 5. OBX-006 — Windows bsdtar filename-list handling fails for tested Unicode paths

**Not introduced by this change; unmasked by it.** Filed as its own issue, unfixed.

**Environment actually tested:** Windows 11, `C:\Windows\System32\tar.exe`,
**bsdtar 3.8.4 / libarchive 3.8.4**, active ANSI codepage **CP1252**, invoked as
`--format=posix -rf … -C <src> --null -T <list>` (`pipeline.go:1035`), with
`filelist.txt` written as **UTF-8** at `pipeline.go:976-981`.

**Filenames actually tested:** `ünïcødé★ 日本語.txt` and `café.txt` failing;
`ordinary.txt`, `--dashy.txt`, `sub/--also-dashy.txt`, `-x`, `spaces and 'quotes'.txt`
passing.

Isolated probes outside the repository, on disposable data:

| Probe | `-T` list contents | List encoding | Result |
|---|---|---|---|
| A | 5 ASCII hostile names, trailing NUL | ASCII | **exit 0** |
| B | same, no trailing NUL | ASCII | **exit 0** |
| C | the 6 names the Windows fixture now uses | UTF-8 | **exit 1** |
| D | `ünïcødé★ 日本語.txt` alone | UTF-8 | **exit 1** |
| 1 | `café.txt` alone | UTF-8 | **exit 1** |
| 2 | `café.txt` alone | CP1252 | **exit 0**, member archived |
| 3 | `ordinary.txt` alone | ASCII | **exit 0**, member archived |

Probes A/B clear the trailing NUL and the dash/space/subfolder shapes of suspicion.
Probes 1 vs 2 isolate the variable to the list's **encoding** alone.

**Explanation supported by the evidence:** this bsdtar build appears to decode the
`--null -T` list in the active ANSI codepage rather than UTF-8, so the UTF-8 bytes the
product writes do not resolve to the real path. Re-encoding the list to the ANSI codepage
is **not** on its own a sufficient repair: `日本語` has no CP1252 representation.

**Not established, and not claimed:** that every non-ASCII name fails; that every Windows
tar implementation behaves this way; behavior under other codepages including UTF-8
codepage 65001; GNU tar or MSYS tar behavior; POSIX behavior (not executed here); long-path
interaction; or which layer the correct fix belongs in.

**Security hypothesis, unconfirmed.** One probe's diagnostic error text contained path-like
bytes that did not come from the supplied input. That *may* indicate uninitialized-buffer
handling in the tool's list reader, but **no actual disclosure of sensitive data has been
demonstrated**, and no attempt was made to characterize it further. It is recorded as a
hypothesis only. **The raw diagnostic output is deliberately excluded from this checkpoint
and retained locally**, pending a decision on whether it warrants an upstream report.

**Impact as tested:** on this Windows host, a chunk containing the tested Unicode filenames
fails to build. Related to **OB-008** (the tar filename-handling work) — these are that
fix's regression tests, and this is the first time they have run against product code on
Windows.

## 6. Verification

Executed **before** the comment precision edit, on the same executable test logic:

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **pass** (exit 0) |
| Vet | `go vet ./...` | **pass** (exit 0) |
| Format | `gofmt -l .` | clean for all repository Go files |
| Full suite | `go test -count=1 -v ./...` | **212 pass / 2 fail / 4 skip** (exit 1) |
| Windows race | `go test -race …` | **NOT TESTED — unavailable** |

Executed **in the publication session** (the later edit is comment-only, so the suite was
not re-run): `gofmt -l tar_names_test.go` clean, `go vet ./...` pass, and diff review of
every staged path.

`gofmt -l` also names two files under the untracked
`docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/` and
`docs/development/PR01_REVIEW_ADDENDUM/` bundles. Those are frozen handoff material, not
repository packages, and were left untouched.

**The suite is not green, and its headline numbers are unchanged in a way that understates
the change.** The same two tests fail before and after, but the cause moved: before, fixture
creation failed and nothing was exercised; after, `BuildChunk` fails. No prediction is made
about what count a future OBX-006 fix will produce. The counts match the historically
reported figure because both count top-level tests; the run also contains 14 passing
subtests.

The 4 skips are unchanged: `TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`,
`TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`.

**Windows race testing remains NOT TESTED.** Verified, not assumed: `go test -race` exits 2
with `-race requires cgo; enable cgo by setting CGO_ENABLED=1`; `CGO_ENABLED=0` and no
`gcc` is on PATH. No tooling was installed. **No CI ran and no cross-platform validation is
implied.**

## 7. What this closes, and what it does not

**Repaired:** the OBX-001 **local Windows fixture** sub-scope. The TAB case is no longer
attempted where the platform will not create it, and both tests now exercise product code
on Windows.

**Still open, explicitly not claimed:** the **`windows-latest` CI lane** for OBX-001 — no CI
has run and none is configured by this change, so **OBX-001 as a whole is not complete**.
**OBX-006 is new and open.** Windows race support, macOS support, and optical/LTO support
are unestablished.

**Proposed next work:** a separate, bounded **Windows Unicode archive-build compatibility
repair** for OBX-006, with **PR-03 queued immediately after** it. Neither is started.
