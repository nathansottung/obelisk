# OBX-006 containment — fresh-session review

**Verdict: READY_FOR_OWNER_REVIEW**

**Date:** September 7, 2026
**Reviewer:** fresh-session AI review (Claude Opus 5, Claude Code). **Not a third-party human
audit** and not a security certification. Independent of the session that wrote the patch and
its reports; every claim below was re-derived from the source or re-executed here.
**Review only** — nothing was implemented, fixed, committed, pushed or merged; the native
writer was not started and PR-03 was not begun.

**Scope reviewed.** Whether the new guard correctly refuses **new** Windows external-tar builds
whose **normalized effective** configuration disables package-content verification. Not
reviewed as: a Unicode compatibility fix, a namespace-security proof, or any statement about
packages already staged or written.

---

## 1. The exact candidate

Verified at the start of the review, and re-verified unchanged at the end.

| | |
|---|---|
| Git root | `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go` |
| Branch | `fix/obx-006-windows-unicode-tar` |
| HEAD / review base | `c880c7d3afd7e61e367b8ee3aa64068af540c768` |
| Index | **empty** — nothing staged |
| In-progress Git operation | **none** (no rebase, merge, cherry-pick or bisect state) |

Reviewed against the stated base, **not** `main`, and including the untracked tests that an
ordinary `git diff` omits.

### File identities as reviewed

| File | State | SHA-256 (worktree) | Git blob |
|---|---|---|---|
| `pipeline.go` | modified | `85899040b654…0aa3d5` | `e321b3e32d58e7cc8a164dda8153d7383e3ad854` |
| `build_verify_windows_containment_test.go` | **untracked, new** | `588428978337…3b02d6b` | `0309a8087330bcff80cb1456234cd13c8063174f` |
| `build_verify_test.go` | modified | `d12acce608c6…08bb091` | `40fc06013b0111af505f2fbd11db98871866ddff` |
| `integrity_test.go` | modified | `0bd492054dfe…5367015` | `35b7df03daf219ea1f80a51cff770a64031e47da` |
| `integration_test.go` | modified | `74dd3c52b7ea…8de8937` | `23143240f00169e8f70b6ae1fa02c0bf34768d8a` |
| `tar_unicode_names_test.go` | **untracked, unchanged** | `07e916b994a5…9c6c4c68` | `b31d8b59de7c7423697306106b4c4b8f53f18aa2` |
| `…/OBX-006-CONTAINMENT-2026-09-07.md` | untracked | `b27a7d3b54ab…c4deb262` | — |
| `…/OBX-006-WINDOWS-UNICODE-IMPLEMENTATION-2026-09-07.md` | untracked | `803232e8edf1…4cd4485e5` | — |

`tar_unicode_names_test.go` matches the blob the containment report records as its
checkpoint (`b31d8b59…`) — the compatibility regressions are **byte-identical**, not softened.

**The patch matches its reported checkpoint.** No material divergence between what the
containment report describes and what is on disk.

**`pipeline.go` is the only production file modified.** PR-01 (docs only) and PR-02 / OB-003
(`appbackup.go`, `mirror.go`, `atomic_replace_test.go`) are **unchanged** against HEAD —
checked file by file across every `.go` file touched by commits `4cd867b`, `726eb73`, `406ed23`.
All prior reports and handoff material are preserved; this review adds one new file and
touches nothing else.

**No prior fresh-session review exists for these contents.** `docs/development/reviews/`
contains the author's own investigation and containment reports for OBX-006 and independent
reviews for PR-01/PR-02 only. This is not a duplicate.

No fetch, pull, switch, reset, stash, clean, restore, stage, commit, push or merge was run.

---

## 2. Does the guard enforce the intended effective-mode restriction?

**Yes.** Each lettered requirement, checked against the source and — where marked — executed.

### A. Decides on normalized effective configuration — **confirmed**

`BuildChunk` (`pipeline.go:996`) reads `iv := a.effectiveIntegrity(c.CollectionID)` and passes
that value to `assertWindowsTarBuildVerifiable`. `effectiveIntegrity` (`integrity.go:120-127`)
returns the collection's own `Integrity` override when one is set, else the global, and
**returns `.normalize()`d in both branches** — so `normBuildVerify` (`integrity.go:50-58`) has
already canonicalised legacy `"fast"` → `none` and blank/unknown → `full`. The guard then
applies `normBuildVerify` a second time before comparing; it never reads `iv.Preset`, and never
touches a raw config string. Legacy mapping is covered directly by
`TestContainment_ProductionPredicateFollowsGOOS`.

### B. A `full` global does not let a `none` archive override through — **confirmed, executed**

`TestContainment_RefusesWindowsBuildWhenVerificationDisabled/archive_override_on_a_verifying_global`
sets the global to `full`, applies a `FAST` per-archive override, asserts the fixture
precondition in both directions (global still `full`, effective now `none`), and the build is
refused. This is the case a preset-label check would pass. **Executed here: PASS.**

### C. A verifying override under a `FAST` global is **not** refused — **confirmed by construction and by my own probe**

Correct by construction (the guard reads only the effective tier), but **the shipped test set
does not cover this direction.** I verified it directly in a disposable copy of the tree with
the guard intact: global `build_verify: none`, per-archive `ARCHIVAL` override → effective
`full` → the build **succeeds**, reaches `STAGED`, attests `Contents: true`, and the staged tar
holds exactly the intended member set. Logged: `built at effective tier "full" under global
"none"`. See finding **F1** — a coverage gap, not a defect.

### D. Ordering — **confirmed, executed**

The guard call sits at `pipeline.go:996-999`, after the existing chunk-status and
`refusing to encrypt` pre-flights and **before** every irreversible step. Traced in source:

| Step | Location | Relative to guard |
|---|---|---|
| `a.tool("tar")` resolution | `pipeline.go:1000` | after |
| `os.MkdirAll(work)` — staging dir | `pipeline.go:1019` | after |
| `setStatus("BUILDING")` | `pipeline.go:1041` | after |
| member list written | `pipeline.go:1049` | after |
| `a.GenerateKey` | `pipeline.go:1146` | after |
| every `tar` invocation | `pipeline.go:1105`, `1108` | after |

`setStatus`/`fail` are not even in scope at the guard, so a refusal cannot write `FAILED` onto
the chunk — it stays `PLANNED`. Asserted by the tests and reproduced in my own probe (staging
directory empty after a refusal).

**`BuildChunk` is the only production path that constructs a new package.** Verified by
searching every `tar` create/append invocation and every `archive/tar` import in non-test code:
`pipeline.go:1105/1108` are the only archive-construction calls; `writer.go:661-717` and
`adopt.go:172-200` **read** an existing artifact (extract / `-tvf`); `appbackup.go` writes the
app's own catalog backup with Go's stdlib writer, not a media package. `pipeline.go:1249` is
the only place a build writes `STAGED`; `writer.go:278` and `store.go:1235` restore that status
on an already-built package.

### E. Preflight and attestation share one decision — **confirmed**

`iv` is hoisted to the top of `BuildChunk` and reused for `mode`, `bv.Preset`, `doContents` and
`doRoundtrip`; the previous `iv := a.effectiveIntegrity(...)` at the attestation site was
removed, not duplicated. Nothing re-reads config between the two points. **A label alone does
not imply checks ran**: `bv.Contents = true` is set only inside `if doContents` and only after
`verifyTarContents` returns nil (`pipeline.go:1126-1132`); `bv.DecryptRoundtrip` likewise.

### F. The refusal is actionable and cannot read as success — **confirmed, executed**

`BuildChunk` returns a non-nil error; the chunk stays `PLANNED`; no staging directory, no key,
no tar, no payload. Through the HTTP API the job is recorded **FAILED** with the refusal as its
label (`integrity_test.go` asserts exactly that via the new `jobFailure` helper, which treats a
COMPLETED job as the failure). The message names the three required things — the required
tiers, the concrete `café.txt` → `cafÃ©.txt` mechanism, and that enabling verification does
**not** fix Unicode — and `assertRefused` pins all three.

### G. No setting rewritten, no bypass, non-Windows unchanged — **confirmed, executed**

The guard is pure: it reads `iv` and returns an error. The shipped test asserts the saved
**global** setting survives a refusal. I additionally verified the **per-archive override**
survives byte-identically (`{Preset:FAST BuildVerify:none Par2Redundancy:5 RoutineVerifyLevel:C
VerifyDueMonths:24 ReadbackAfterWrite:true}` before and after) and is not cleared — see F1.
No bypass flag, no allowlist, no helper-version keying, no code-page workaround.
`TestContainment_NonWindowsPathUnaffected` pins that off the Windows path the `none` tier keeps
its documented behaviour: builds, no verification, amber warning intact.

### H. The platform decision is not reachable from configuration or API — **confirmed**

`buildUsesWindowsExternalTarHook` is an **unexported package-level `func() bool` variable** in
`package main`. It cannot be set from JSON config, an HTTP body, or any exported API — a
function value has no config representation, and the only writers in the whole tree are
`*_test.go` files. The only two environment variables production reads are
`OBELISK_AUTH_TOKEN` / `MNEMO_AUTH_TOKEN` (`main.go:96-98`), neither related. Unhooked, the
predicate is exactly `runtime.GOOS == "windows"`, pinned by
`TestContainment_ProductionPredicateFollowsGOOS`, which also fails loudly if any earlier test
leaks a non-nil hook.

---

## 3. Are the positive controls and ordering assertions valid?

**Yes — and I confirmed the negative controls are not vacuous by removing the guard.**

- **The tar-invocation observer has a real positive control.** `tarInvocationWatch` reuses
  `buildAfterTarHook`, which fires on the freshly written tar *between* the tar call and stage
  verification, so it can only fire if tar actually ran and produced an archive.
  `TestContainment_WatchDetectsTarInvocation` installs the **same** watch on a Contents build
  and requires it to fire. **Executed: PASS.** Without it, "tar never ran" would prove nothing.
- **Rejection comes from this guard, not from an unrelated error.** `assertRefused` matches the
  guard's own message text, not merely "some error". A missing-tool error cannot produce it —
  and could not arise anyway, since the guard precedes `a.tool("tar")`.
- **Ordering is asserted, not assumed**: tar not invoked, status still `PLANNED`, staging
  directory absent, `KeyRef` empty and neither keystore's key count changed.
- **The mismatch detector still works.** `TestContainment_VerifierStillRejectsMismatchedArchive`
  drives a real Contents build with the corruption seam and requires a `stage verification`
  failure and a `FAILED` package. **Executed: PASS.**
- **A supported ASCII fixture still builds at Contents and Full**, attests contents proven, and
  its exact member set is re-read with Go's own `archive/tar` reader. **Executed: PASS.**

### Control experiment — the regressions fail for the intended reason

In a **disposable copy** of the tree in the session scratchpad (the working checkout was never
reverted or modified), I replaced only the three-line guard call in `BuildChunk` with a no-op
and re-ran:

| Test | Guard present | Guard removed |
|---|---|---|
| `…RefusesWindowsBuildWhenVerificationDisabled/global_FAST_preset` | PASS | **FAIL** — "build must be REFUSED…" |
| `…/archive_override_on_a_verifying_global` | PASS | **FAIL** — same reason |
| `…RefusesBeforeKeyGeneration` | PASS | **FAIL** — same reason |
| `…WrongFileCannotCompleteUnverified` | PASS | **FAIL** — same reason |
| `TestBuildVerify_FastModeSkipsAndWarns` (Windows branch) | PASS | **FAIL** — "a fast build must be REFUSED…" |
| `…WatchDetectsTarInvocation` | PASS | PASS |
| `…VerifyingTiersStillBuild/{contents,full}` | PASS | PASS |
| `…NonWindowsPathUnaffected` | PASS | PASS |
| `…VerifierStillRejectsMismatchedArchive` | PASS | PASS |
| `…ProductionPredicateFollowsGOOS` | PASS | PASS |

Exactly the five refusal assertions fail, each for the intended reason, and nothing else moves.
The evidence is valid.

### The unsafe path, independently reproduced

The containment report's central claim is that at the `none` tier a **wrong** file reaches a
staged, catalogue-consistent package. I reproduced this in the guardless copy with a synthetic
disposable fixture (`café.txt` and its CP1252 look-alike side by side, only `café.txt`
selected). Result:

```text
status="STAGED"
warning="FAST build: stage-vs-source and decrypt round-trip verification were SKIPPED …"
staged member  "cafÃ©.txt"   sha=1c64e2dc8199d31b…
catalog wanted "café.txt"   sha=15a45d246fcab1ea…
```

The package reaches `STAGED`, eligible to be written, holding a **different file with different
bytes** than the catalogue records — and the only signal is the generic amber FAST warning,
which says nothing about a wrong file. With the guard in place that build is refused before tar
runs. Separately, at the default `full` tier the same fixture **is** caught in the real
checkout: `TestBuildChunk_WrongFileSelection_LookalikeNeighbour` fails with
`stage verification: package contains "cafÃ©.txt", which is not a cataloged member of this
package (unexpected extra member)`.

**This is the gap the patch closes, and it is real.**

---

## 4. Test-seam interference

**No actual interference found.** Traced rather than assumed:

- **No test in the package calls `t.Parallel()`** — zero occurrences tree-wide; the only two
  matches are comments in `build_verify_test.go` and `durability_gate_test.go` stating that
  these tests deliberately do not, because they share package-level build hooks. Go runs
  top-level tests in a package sequentially absent `t.Parallel()`, so the shipped subtests
  cannot overlap.
- **Restoration is real, not merely registered.** `onWindowsTarPath` saves the previous value
  and restores it in `t.Cleanup`; `tarInvocationWatch` does the same for `buildAfterTarHook`.
- **A leak detector exists and passes.** `TestContainment_ProductionPredicateFollowsGOOS`
  fails if the hook is non-nil when it runs, and asserts the unhooked predicate equals
  `runtime.GOOS`. It executed and passed on a real Windows host.
- **Asynchronous exposure is theoretical, pre-existing, and not triggered.** The HTTP
  integration tests run builds in `runJob` goroutines; both `job` and the new `jobFailure` block
  until the job reaches a terminal state, so a build goroutine does not normally outlive its
  test. Only a 90-second harness timeout could leave one running, and a stray goroutine reads
  the hook without being able to affect another test's assertions. This is the same property
  the existing `buildAfterTarHook` / `buildDecryptPassphraseHook` seams already have, and it is
  unverifiable here in any case (`-race` needs cgo; `CGO_ENABLED=0`, no gcc).

**A broad refactor is not warranted** — no concrete failure path was demonstrated.
Minor cosmetic note, not a finding worth acting on alone:
`TestContainment_VerifierStillRejectsMismatchedArchive` restores `buildAfterTarHook` to `nil`
rather than to its previous value, matching the pre-existing convention in `build_verify_test.go`.

---

## 5. Tests I executed vs. results reported only

Windows 11 Pro 26200, `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, real system temp (**no
`GOTMPDIR` override**), the existing pinned toolchain, disposable synthetic fixtures, no real
archival data touched. Nothing installed, no locale or machine setting changed, no platform
substituted.

### Executed by me

| Check | Command | Result |
|---|---|---|
| Containment suite | `go test -count=1 -v -run '^TestContainment_' .` | **PASS** — 8 top-level + 4 subtests, exit 0 |
| Affected FAST / verify tests | `go test -count=1 -v -run '^(TestBuildVerify_.*\|TestFastArchiveAttestsReducedIntegrity\|TestIntegrity.*)$' .` | **PASS** — including both intentionally changed tests |
| Preserved compatibility tests | `go test -count=1 -v -run '^(TestBuildChunk_.*\|TestBuildRestore_.*\|TestBuildFilelist_.*)$' .` | 7 fail, unchanged causes (§7) |
| Build | `go build ./...` | **PASS** (exit 0) |
| Vet | `go vet ./...` | **PASS** (exit 0) |
| Format | `gofmt -l` over the candidate production and test files | **clean** |
| **Full suite, uncached** | `go test -count=1 -v ./...` | **exit 1 — 220 pass / 7 fail / 4 skip**, plus 19 passing and 5 failing subtests |
| Control: guard removed (disposable copy) | as §3 | 5 intended failures, nothing else |
| Probe: verifying override under FAST global (disposable copy) | as §2C | **builds, attests, exact members** |
| Probe: refusal leaves per-archive override intact (disposable copy) | as §2G | **unchanged and not cleared** |

**The author's full-suite figure is independently reproduced, not merely relayed:** I obtained
**220 / 7 / 4** with **19 sub-pass / 5 sub-fail** — identical to the reported result, including
the same seven failing top-level tests. `gofmt -l` also lists two files, but both are inside
untracked `docs/**/source-materials/**/reproducers/` handoff material, outside this candidate
and outside the Go build; the candidate files themselves are clean.

The four skips are pre-existing and environmental — `TestCardCheck_UnlistableDirBlocksFormat`,
`TestCatalogScale`, `TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`. **No new skip was
introduced, and no skip conceals the containment or the compatibility failures.**

### Reported only — NOT independently reproduced

- **Windows race testing remains NOT TESTED.** `-race` requires cgo; `CGO_ENABLED=0` with no
  gcc available. Not run by the author, not run by me.
- **No CI result is established by this uncommitted patch.** No CI ran. The `windows-latest`
  lane outstanding from OBX-001 is still outstanding.
- The bsdtar 3.8.4 code-page diagnosis in the investigation report was not re-derived here
  beyond the wrong-file behaviour I reproduced above. Helper diagnostic bytes are deliberately
  omitted from this report; no upstream disclosure is claimed or implied.

---

## 6. Intentional Windows behaviour changes

Both are genuine, deliberate behaviour changes to a documented opt-out, correctly scoped and
correctly described:

1. **`TestBuildVerify_FastModeSkipsAndWarns`** — off the Windows external-tar path, unchanged
   in full, and still proves the FAST opt-out's semantics there. On that path it now asserts
   the refusal and that the package is not left `STAGED`. **Asserted, not skipped.** The
   corruption hook is still installed; the point is the build stops before it could fire.
2. **`TestFastArchiveAttestsReducedIntegrity`** — the override, its effective `FAST`/`none`
   configuration and the 5% par2 it plans are platform-independent and still asserted. Only the
   build's outcome changed: on Windows it must now be an explicit refusal, checked **by reason**
   through the real HTTP job. **Asserted, not skipped.**

`integration_test.go` gained only `jobFailure`, the mirror of the existing `job` helper. No
Unicode input was deleted and no identity check was weakened to make anything pass — confirmed
by diffing every modified test file.

---

## 7. Preserved compatibility failures

`tar_unicode_names_test.go` is byte-identical to the investigation checkpoint (`b31d8b59…`) and
retains all its compatibility assertions — including reading every produced archive with Go's
own `archive/tar` reader, so a zero exit status is never accepted as evidence. These tests run
under `newTestApp`, which sets no `build_verify` and therefore builds at the default `full`
tier; the containment does not touch them and cannot mask them.

The seven failures I reproduced, unchanged in cause:

```text
--- FAIL: TestBuildRestore_HostileFilenamesRoundTrip
--- FAIL: TestBuildFilelist_IsNulDelimited
--- FAIL: TestBuildChunk_UnicodeFilenames_ExactMembers
          (accented, japanese, supplementary, bmp-symbol, unicode-dir-component;
           the ascii subtest passes)
--- FAIL: TestBuildChunk_WrongFileSelection_LookalikeNeighbour
--- FAIL: TestBuildChunk_UnicodeSourceRoot
--- FAIL: TestBuildChunk_UnicodeStagingDir
--- FAIL: TestBuildRestore_UnicodeRoundTrip
```

These are the unresolved Unicode compatibility failures, **not** the old invalid-TAB fixture
failures repaired in `e62ee09`. `TestBuildChunk_WrongFileSelection_LookalikeNeighbour` still
fails at the Contents tier with the wrong-member detection quoted in §3 — containment removed
the route where the mismatch went **undetected**, not the mismatch itself.

---

## 8. Findings

One consolidated set. **No blocker.** No reachable bypass, no false success, no unintended
setting mutation, no incorrect platform behaviour, no invalid regression evidence and no new
material regression was found in the containment itself.

### F1 — coverage gap: no shipped test for a verifying override under a non-verifying global (minor, non-blocking)

The test set covers `none` under a `full` global (case B) but not its mirror: an effective
`Contents`/`Full` **archive override** under a global whose label is `FAST`/`none`. The guard
handles it correctly — I verified the build succeeds, attests `Contents: true` and stages the
exact member set — but that behaviour currently rests on construction plus my out-of-tree probe
rather than on a regression in the repository. A future edit that reintroduced a global-config
read would be caught by case B only if it read the global *instead of* the effective value; a
change that refused on *either* would slip through.

Also unpinned in-tree: a refusal leaves the **per-archive** override untouched (the shipped
assertion covers only the global config file). I verified both directly.

**Suggested, if the owner wants it closed in the same pass:** one subtest under
`TestContainment_RefusesWindowsBuildWhenVerificationDisabled`'s sibling
`TestContainment_VerifyingTiersStillBuild` — global `none`, archive override `ARCHIVAL`, assert
the build succeeds and `BuildVerified.Contents` is true — plus one added assertion that
`app.effectiveIntegrity(coll.ID)` is unchanged after the refusal in the existing override
subtest. Both are additive; neither changes production code.

### F2 — the refusal tests depend on toolchain availability they do not need (cosmetic)

Every containment test calls `nativeTools(t)`, which `t.Skip`s when `tar`, `gpg` or `par2` is
absent. The refusal path never resolves a tool, so on a machine without the toolchain the four
refusal tests would **skip** rather than run. Not a defect in this patch (it follows the
package's existing helper convention) and not observed here — all tools were present and every
test executed. Worth noting only because a skipped containment test is silent.

### Wording — accurate as written

I checked the two reports and both status entries for overstatement and found none needing
correction. In particular they correctly state that job bookkeeping still occurs on refusal
(a FAILED job and a log line are recorded — the claim is only that the unsafe **build
operation** is refused, not that no catalog activity occurs), that this patch does **not** fix
job durability, that packages built unverified before the guard are **not** retroactively made
safe, that `verifyTarContents` is not a namespace-security proof, and that Windows race testing
and CI are not established. The recorded suite figures match what I measured.

---

## 9. The precise guarantee accepted

> On the Windows external-tar construction path, **`BuildChunk` refuses to begin a new package
> build whose normalized effective build-verify tier is `none`** — deciding on
> `effectiveIntegrity` (per-archive override, else global, legacy `"fast"` already mapped),
> before tool resolution, staging-directory creation, the `BUILDING` status write, key
> generation and every `tar` invocation. The chunk remains `PLANNED`, no staging directory,
> key, tar or payload is produced, and no saved global or per-archive setting is altered. The
> tier that passed this preflight is the same value the later content verification and the
> recorded attestation use.

**And nothing more.** Explicitly outside the guarantee:

- **It does not fix Unicode filename support.** Seven compatibility tests still fail, by design.
- **It says nothing about packages already built.** Packages constructed unverified before this
  guard may exist, staged or already written to media. They are untouched, not upgraded, not
  marked verified, and **not retroactively safe**. Whether they remain eligible for write,
  rewrite or span operations requires a **separate assessment that has not been done**.
- **It is not a "no catalog activity" guarantee.** A refused build through the API still records
  a FAILED job and a log line. This patch does not address job durability.
- **It is not a namespace-security proof.** `verifyTarContents` compares members against this
  package's catalogue; it does not validate member names for `..`, absolute paths, links or
  aliasing, a `ChunkFileRef` with an empty `Hash` has only its presence checked, and restore
  confinement still rests entirely on the external tar's defaults — **OB-008**, **OBX-005**,
  **OB-009**, untouched.
- **It restricts only the path it checks**, asserts nothing about other platforms or about which
  tar binaries share the defect, and is temporary — superseded by the native writer under its
  own scope.

---

## 10. Verdict

**READY_FOR_OWNER_REVIEW.**

The containment does what it claims, on the value it claims, at the point it claims, with
evidence that survives independent execution and a guard-removal control. The acceptance
boundary is kept small: Unicode remains broken, the native writer is unbuilt, existing
artifacts are unreassessed, and namespace/restore/durability issues remain open — all recorded
here and in the author's reports, none silently treated as fixed. None of them is a reason to
block this containment.

**F1** is a test-coverage suggestion the owner may fold in or defer; it is not a defect and does
not gate acceptance. **F2** is cosmetic.

Consolidated here in one pass. **No further comment-only review loop is requested.**

---

## 11. Post-review state check

Re-verified after all checks, immediately before writing this file:

- Branch `fix/obx-006-windows-unicode-tar`; HEAD still `c880c7d3afd7e61e367b8ee3aa64068af540c768`.
- Index still **empty**; working-tree status identical to §1, entry for entry.
- All six candidate source/test files re-hashed to the **same SHA-256** recorded in §1.
- All experiments ran in disposable copies under the session scratchpad. The working checkout
  was never reverted, patched or restored.

This review added exactly one file — this report. Earlier reports are preserved and no issue
status was updated on the author's behalf. **OBX-006 remains open**; PR-03 / OB-002 was not
started.
