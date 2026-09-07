# OBX-006 — temporary containment for unverified Windows external-tar builds

**Date:** September 7, 2026
**Branch:** `fix/obx-006-windows-unicode-tar`
**Parent commit:** `c880c7d3afd7e61e367b8ee3aa64068af540c768`
**Scope:** one small production guard plus its tests. **The native tar writer is NOT
implemented here**, and PR-03 is not started.

**This is containment, not a compatibility fix.** Unicode filenames still fail on this path.
The diagnosis that produced this decision is preserved unchanged in
[OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md](OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md),
whose §10a now records the decision itself.

## 1. Checkpoint

Verified before editing: branch `fix/obx-006-windows-unicode-tar`, HEAD
`c880c7d3afd7e61e367b8ee3aa64068af540c768`, nothing staged, no in-progress git operation,
and exactly the expected uncommitted work present. No fetch, pull, switch, stash, clean,
reset, rebase, stage, commit, push or merge was run; no previous fix branch was touched.

Identities of the prior work as found (all preserved; the investigation report gained only
the §10a decision section):

| File | Blob as found |
|---|---|
| `tar_unicode_names_test.go` | `b31d8b59de7c7423697306106b4c4b8f53f18aa2` (**unchanged**) |
| `…/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md` | `ae36269ffb583a76d5a61bcd7a02a7b14305c48f` |
| `docs/development/OB_STATUS.md` | `787493f87dc52f6e332c8c7e371c7096616c4585` |
| `docs/development/NEXT_ACTIONS.md` | `0dc0b234d25ea2d41a622f31c02ab132460d9315` |

## 2. Changed files (vs `c880c7d3`)

| File | Change | Blob |
|---|---|---|
| `pipeline.go` | **production guard** (+74/−1) | `f4850947…` → `e321b3e32d58e7cc8a164dda8153d7383e3ad854` |
| `build_verify_windows_containment_test.go` | added, containment regressions | `0309a8087330bcff80cb1456234cd13c8063174f` |
| `build_verify_test.go` | Windows expectation of the FAST opt-out | `e436b952…` → `40fc06013b0111af505f2fbd11db98871866ddff` |
| `integrity_test.go` | Windows expectation of the FAST archive build | `d8cad743…` → `35b7df03daf219ea1f80a51cff770a64031e47da` |
| `integration_test.go` | `jobFailure` harness helper | `1aa5e58d…` → `23143240f00169e8f70b6ae1fa02c0bf34768d8a` |
| `tar_unicode_names_test.go` | **untouched** | `b31d8b59…` |

## 3. The unsafe path, revalidated

Traced in code before changing anything:

- `BuildChunk` reads the tier from `a.effectiveIntegrity(c.CollectionID)`
  (`integrity.go:120-127`), which returns the archive's own override if set, else the global
  — **already `.normalize()`d**, so `normBuildVerify` has canonicalised legacy `"fast"` to
  `"none"` and blank to `"full"` (`integrity.go:50-58`).
- `doContents := mode == BuildVerifyFull || mode == BuildVerifyContents` gates
  `verifyTarContents` (`pipeline.go:834-885`). At `none` it is skipped, and no other step
  compares the archive's members against the catalog.
- The reproduced wrong-file case reaches a staged payload that way: tar exits 0 having
  archived `cafÃ©.txt` for a requested `café.txt`; `hashFileHex(tarPath)` then records the
  hash **of that wrong archive** into `c.TarHash`, so every downstream check is
  self-consistent. The package is stamped `STAGED` with the amber warning and is eligible to
  be written.

**Four different guarantees, none substituting for another:**

| Guarantee | Where | Catches the wrong file? |
|---|---|---|
| Helper success (tar exit 0) | `run`, `pipeline.go:1217` | **No** — the demonstrated failure exits 0 |
| Package membership/content | `verifyTarContents`, gated by `doContents` | **Yes** — this is the only one |
| Encryption round-trip | `decryptRoundtripHash`, gated by `doRoundtrip` | **No** — proves ciphertext ↔ tar, not tar ↔ source |
| Media write / read-back | `writer.go:397-400`, always on | **No** — proves the medium matches the staged bytes |

Confirmed by execution, not by reading: at the default `full` tier the mismatch **is**
caught — `TestBuildChunk_WrongFileSelection_LookalikeNeighbour` fails with
`stage verification: package contains "cafÃ©.txt", which is not a cataloged member of this
package (unexpected extra member)`. At `none` there is nothing to catch it. That is the gap
this patch closes.

## 4. The guard

`assertWindowsTarBuildVerifiable(iv Integrity)` in `pipeline.go`, called from `BuildChunk`
immediately after the existing `refusing to encrypt` pre-flight and **before** `a.tool("tar")`,
`os.MkdirAll(work)`, the `BUILDING` status write, the member list, `a.GenerateKey` and every
`tar` invocation.

- **Decides on the normalised effective configuration.** `iv` is hoisted to the top of
  `BuildChunk` and reused for the attestation later, so the guard and the recorded tier
  cannot disagree. It reads neither the preset label nor the raw config string; legacy
  `"fast"` is refused on the same terms because `normBuildVerify` has already mapped it.
- **Returns through the existing pre-flight failure path** — the same bare-error shape as
  `refusing to encrypt` — so it surfaces on the job with its reason. The job runner records
  a FAILED job and a log line; **the claim is only that the unsafe build operation is
  refused**, not that no catalog, log or directory activity of any kind occurs. Measured:
  the package stays `PLANNED`, its staging directory is not created, no key is generated and
  no tar runs.
- **Message states all three required things:** Windows external-tar builds temporarily
  require Contents or Full; the helper can misinterpret filenames (with the concrete
  `café.txt` → `cafÃ©.txt` case); and enabling verification does **not** fix Unicode support.
- **Changes no saved setting.** A refused build leaves global and per-archive configuration
  exactly as the operator set it — asserted in the tests.
- **Deliberately narrow.** It restricts the current Windows external-tar path. It makes no
  claim that every Windows tar shares the defect, is not keyed off the helper's version
  string or any allowlist, offers no bypass flag, and applies no code-page workaround.
- Non-Windows behaviour is unchanged, and `Contents`/`Full` semantics are untouched. The
  preset system is not redesigned.

The platform predicate sits behind `buildUsesWindowsExternalTarHook`, following the existing
`buildAfterTarHook` / `buildDecryptPassphraseHook` convention: nil in production — the only
value a shipped binary has — and wired solely from `*_test.go`, so the containment can be
exercised on every platform. It exists to let a test *assert* the guard, not to disable it.

## 5. Containment regressions — all pass

`go test -count=1 -v -run '^TestContainment_' .` → **exit 0**

| Test | Proves |
|---|---|
| `…RefusesWindowsBuildWhenVerificationDisabled/global_FAST_preset` | refused; tar never ran; status stayed `PLANNED`; staging dir not created; saved global setting unchanged |
| `…/archive_override_on_a_verifying_global` | **the decision follows effective configuration** — global is `full`, the archive override makes it `none`, and it is still refused. A guard reading the global label would pass this |
| `…RefusesBeforeKeyGeneration` | encrypted package at `none`: refused with no `KeyRef` recorded and no key appended to either keystore |
| `…WatchDetectsTarInvocation` | **positive control**: the same watch, on a Contents build, *does* fire — so "tar never ran" above is not vacuous |
| `…VerifyingTiersStillBuild/contents`, `/full` | ordinary supported ASCII content still builds, still attests contents proven, exact member set verified with Go's `archive/tar` reader |
| `…NonWindowsPathUnaffected` | off the Windows path the `none` tier keeps its documented behaviour — builds, no verification, amber warning intact |
| `…WrongFileCannotCompleteUnverified` | `café.txt` + `cafÃ©.txt` on disk, only `café.txt` selected: the unverified route is refused — no tar, nothing staged, no package to write |
| `…VerifierStillRejectsMismatchedArchive` | the check the containment depends on still works: a corrupted member is rejected at `contents`, package `FAILED` |
| `…ProductionPredicateFollowsGOOS` | unhooked, the predicate is the real `runtime.GOOS`; verifying tiers are never refused on any platform; legacy `"fast"` is treated as `none` |

Fixtures are disposable and synthetic; every build runs the real `BuildChunk`.

## 6. Intentional behaviour change to existing tests

Two tests asserted that an unverified Windows build **succeeds**. Only their Windows-specific
expectation was changed; neither was skipped, and their other-platform coverage is intact.

- **`TestBuildVerify_FastModeSkipsAndWarns`** — off Windows, unchanged in full. On the
  Windows external-tar path it now asserts the refusal and that the package is not `STAGED`.
  The corrupted-tar hook is still installed, and the point is that the build is stopped
  before it could fire.
- **`TestFastArchiveAttestsReducedIntegrity`** — the override, its effective `FAST/none`
  configuration and the 5% par2 it plans are platform-independent and still asserted. Only
  the build's outcome changes: on Windows it must be an explicit refusal, checked by reason
  through the HTTP job, with the package not left `STAGED`.

`integration_test.go` gained `jobFailure`, the mirror of the existing `job` helper: it waits
for a FAILED job and returns its label, and treats a COMPLETED job as the failure.

No Unicode input was deleted and no identity check was weakened to make anything pass.

## 7. Preserved compatibility failures

`tar_unicode_names_test.go` is **byte-identical** to the investigation checkpoint
(`b31d8b59…`). Its seven failures — five subtests plus the two pre-existing hostile-name
tests — are unchanged in cause and are meant to remain visible:

```text
--- FAIL: TestBuildRestore_HostileFilenamesRoundTrip
--- FAIL: TestBuildFilelist_IsNulDelimited
--- FAIL: TestBuildChunk_UnicodeFilenames_ExactMembers  (accented, japanese,
          supplementary, bmp-symbol, unicode-dir-component)
--- FAIL: TestBuildChunk_WrongFileSelection_LookalikeNeighbour
--- FAIL: TestBuildChunk_UnicodeSourceRoot
--- FAIL: TestBuildChunk_UnicodeStagingDir
--- FAIL: TestBuildRestore_UnicodeRoundTrip
```

`TestBuildChunk_WrongFileSelection_LookalikeNeighbour` still fails at the Contents tier —
containment removed the route where the mismatch went **undetected**, not the mismatch.

## 8. Verification

Windows 11, `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, real system temp (**no `GOTMPDIR`
override**), existing toolchain, disposable data, native exit codes checked in PowerShell.

| Check | Command | Result |
|---|---|---|
| Containment | `go test -count=1 -v -run '^TestContainment_' .` | **exit 0** — 8 top-level, 4 subtests, all pass |
| Updated FAST tests | `go test -count=1 -v -run '^(TestBuildVerify_.*\|TestFastArchiveAttestsReducedIntegrity)$' .` | **exit 0** — all pass |
| Build | `go build ./...` | **pass** (exit 0) |
| Vet | `go vet ./...` | **pass** (exit 0) |
| Format | `gofmt -l` over all tracked `*.go` plus both new files | **clean** |
| Full suite, uncached | `go test -count=1 -v ./...` | **exit 1** — **220 top-level pass / 7 fail / 4 skip**, plus 19 passing and 5 failing subtests |
| Windows race | `go test -race …` | **NOT TESTED — unavailable** |

Against the investigation checkpoint's 212/7/4: **+8 passes** (the new containment tests),
**the same 7 failures** (§7), **no new skips**. That earlier count was a record, not a
target.

**Windows race testing remains NOT TESTED** — `-race` requires cgo, `CGO_ENABLED=0`, no
`gcc`. Nothing was installed, no machine-wide locale changed, no platform substituted, no
`GOTMPDIR` override used. **No CI ran; no CI or cross-platform result is implied.** No
control experiment touched the working checkout.

Anomalous helper diagnostics remain synthetic-only and retained locally, outside this
repository. Nothing is published here, and no disclosure is claimed.

## 9. Remaining exposure — explicitly not closed

**This restricts NEW builds through the checked path only.**

- **Packages built unverified before this guard may exist**, staged or already written. They
  have not been erased, altered, upgraded or marked verified, and this guard does **not**
  retroactively make them safe. Their eligibility for later write, rewrite or span
  operations **requires a separate assessment** that has not been done.
- **Unicode support is not fixed.** The build path still fails for the tested names with
  verification enabled.
- **`verifyTarContents` is not a namespace-security proof.** It compares members against the
  catalog for this package. It does not validate member names for `..`, absolute paths,
  links or aliasing; a `ChunkFileRef` with an empty `Hash` has only its presence checked; it
  says nothing about source aliasing, and extraction confinement on restore still rests
  entirely on the external tar's defaults. Those remain **OB-008**, **OBX-005** and
  **OB-009**, separately tracked and untouched.
- The containment is temporary. It is superseded by the native writer when that lands under
  its own scope, not by this task.

**OBX-006 remains open.** PR-03 / OB-002 remains the next substantial workstream.
