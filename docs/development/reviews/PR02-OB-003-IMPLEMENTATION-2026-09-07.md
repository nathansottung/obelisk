# PR-02 / OB-003 — safe destination replacement

**Implementation report, for one substantive independent review.**

Work branch `fix/ob-003-safe-replacement`, created from the published PR-01 checkpoint
`4cd867b2ddb26c945f7c74faee8ef32780743be7`. Nothing is staged, committed or pushed.

> **On the date in this filename.** The session began 2026-09-06 and the host clock rolled over to
> **2026-09-07** partway through, after the pre-fix reproduction and before this report was written.
> The report and the status entries carry 2026-09-07; the PR-01 documents dated 2026-09-06 are
> earlier work and are unmodified.

---

## 1. Identity

| | |
|---|---|
| Review base (diff against this, **not** `main`) | `4cd867b2ddb26c945f7c74faee8ef32780743be7` |
| Branch | `fix/ob-003-safe-replacement` |
| Branch HEAD (unchanged; no commits made) | `4cd867b2ddb26c945f7c74faee8ef32780743be7` |
| SHA-256 of the unified diff vs base | `45ab2bcbf7768cce82842f7345c9ac1fe4300980009fb42506fb121103281c29` |

`main` must not be used as the review base: it is at `d97809b2`, so a diff against it would
re-present all of PR-01 as if it were new work in this patch.

### Changed files

| File | Change | Lines | SHA-256 | Blob |
|---|---|---|---|---|
| `mirror.go` | modified | 478 | `51801765bab2d2d868c7aa802f8527fb8fa893c3be9864054a2e46a6ec56a211` | `c10a447dbcd4f4e9929ddf7e75eb664908a00ab0` |
| `appbackup.go` | modified | 523 | `731fd293bf359cd530a8923c85000371ec51c95377f27d62cb13c58aa2f33af6` | `c8e8dde51514aee015ea3000e661ed2f80568dc2` |
| `atomic_replace_test.go` | **new** | 276 | `b1077b11e4b5864c834c245bcee023b94489267e211ec9fefef32ac1c9e799a4` | `8a61f4e799b15e54d6303ec359dfb9be8840fc9c` |

`git diff --stat` vs base: `appbackup.go | 9 +++++++-`, `mirror.go | 44 ++++++++++++-------`,
2 files changed, 45 insertions(+), 8 deletions(-), plus the untracked new test file.

### PR-01 is untouched

| File | SHA-256 now | Matches published PR-01 |
|---|---|---|
| `store.go` | `66efd8c9d476ce6caa27ba46048ed8f7bd71691ddc0a12d13f4fbe37b9d6228e` | yes |
| `catalog_open_test.go` | `13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3` | yes |

Neither path appears in `git diff --name-only` against the base. Verified, not assumed.

---

## 2. Revalidation of OB-003

Re-read at the current tree rather than trusted from the baseline. The anchors in `OB_STATUS.md`
are **still exact**: the helper is at `mirror.go:318` and there are still **five** production
callers at the recorded lines. The defect stood verbatim as quoted:

```go
func atomicRename(tmp, final string) error {
	if err := os.Rename(tmp, final); err == nil {
		return nil
	}
	_ = os.Remove(final)          // any first failure, whatever the cause
	return os.Rename(tmp, final)  // and this can fail too
}
```

### Current caller map

Every caller stages `tmp` as the final path plus a suffix **in the same directory**, so temporary
and final are always distinct paths on the same filesystem. No current caller can produce a
cross-device rename.

| Anchor | Publishes | Temporary | Verification before publish | Cleanup on failure |
|---|---|---|---|---|
| `mirror.go:252` | each mirrored file | `destPath + ".mnemo_tmp"` | stream hash + read-back hash | `os.Remove(tmp)`, counts `Failed++` |
| `mirror.go:308` `copyVerifyToDest` | shared landing path (full + incremental mirrors) | `destPath + ".mnemo_tmp"` | stream hash + read-back hash | `os.Remove(tmp)`, error returned |
| `plans.go:764` | each file published by plan execution | `dest + ".mnemo_tmp"` | source re-verify vs snapshot + read-back | `os.Remove(tmp)`, counts `Unreadable++` |
| `appbackup.go:220` | the app-backup `.tar` bundle | `tarPath + ".tmp"` | tar written and closed, write errors checked | `os.Remove(tmp)`, error returned |
| `appbackup.go:396` | `catalog.json`, `jobs.json`, `formats.json`, `keystores/*` on restore | `dest + ".tmp"` | whole bundle verified before anything is written | **none — fixed here** |

Error propagation is real at every site: no caller discards the helper's error.

**Why the callers made the old bug worse rather than containing it.** Four of the five run
`_ = os.Remove(tmp)` when the helper fails. Under the old helper a double failure had already
deleted `final` and left the replacement at `tmp`; the caller's cleanup then removed that too, so
**both** the previous good file and its replacement were destroyed. The copy-then-verify
discipline these callers implement protects against bad *content*; it never protected against a
failed *publication*.

**The auto-export case is the sharpest.** `appbackup.go:469-479` reuses stable per-period bundle
names, so a failed rename there deleted the previous period's already-good backup.

---

## 3. Pre-fix reproduction

Run against the unfixed helper, in `t.TempDir()` disposable directories with synthetic bytes only.
No real catalog, keystore, backup, NAS original, archival drive, tape or optical medium was
touched at any point.

No temporary reversal was needed: the tests were written and executed **before** the fix, while
the destructive fallback was still in the working tree. (The one preparatory edit was adding the
`renameFile` seam and routing the existing buggy body through it, leaving the delete-and-retry
behavior intact — confirmed by the run below, which still shows the destination being destroyed.)

`go test ./... -run 'TestAtomicRename|TestExportAppBackup_FailedPublish|TestRestoreAppBackup_FailedPublish' -count=1` → **exit 1**:

```
--- FAIL: TestAtomicRename_MissingTempPreservesDestination
    destination unreadable after the operation (... destination.bin:
    The system cannot find the file specified.) — it must still be there
--- FAIL: TestAtomicRename_InjectedFailuresPreserveDestination/permission_denied
--- FAIL: TestAtomicRename_InjectedFailuresPreserveDestination/cross-device_link
--- FAIL: TestAtomicRename_InjectedFailuresPreserveDestination/sharing_violation
--- FAIL: TestAtomicRename_InjectedFailuresPreserveDestination/read-only_filesystem
--- FAIL: TestExportAppBackup_FailedPublishPreservesPreviousBundle
    the previous bundle must survive a failed export, but it is gone
--- FAIL: TestRestoreAppBackup_FailedPublishPreservesExistingFile
--- FAIL: TestAtomicRename_CommentDoesNotClaimDeleteFirst
```

The failure mode is the intended one, and it is stronger than "the bytes changed": the destination
file **no longer exists**. The loss reproduces at the helper *and* through two real callers.

**The two tests that passed pre-fix are the load-bearing evidence for the chosen fix.**
`TestAtomicRename_PublishesWhenDestinationAbsent` and
`TestAtomicRename_ReplacesExistingDestination` both passed against the unfixed code on this host —
meaning the very first `os.Rename` already replaced an existing destination, and the delete-retry
path was never reached in the ordinary success case.

Host: `windows/amd64`, Go `go1.26.4`, Microsoft Windows NT 10.0.26200.0.

---

## 4. Implementation choice

**The smallest correction is sufficient, and it was verified before being chosen.** The question
the brief posed — whether simply removing the fallback and returning the rename error is enough,
*including ordinary successful replacement on the real Windows host* — is answered **yes, by
execution**. `os.Rename` on this host replaces an existing file (Go issues `MoveFileEx` with
`MOVEFILE_REPLACE_EXISTING`), so the delete was never needed for the case its comment invoked.

The old comment asserted the opposite: *"on Windows it fails if the target exists, so remove
first."* That claim is false at this Go version on this host, and it is the reasoning that produced
the defect. It is corrected in the patch, and
`TestAtomicRename_CommentDoesNotClaimDeleteFirst` fails if either the claim or `os.Remove(final)`
returns to `mirror.go`.

No platform-specific API was added. Nothing in the patch does delete-then-rename,
delete-then-copy, copy-over-final, or rollback. Cross-filesystem publication fails with the
underlying error rather than silently degrading to a copy — a copy could not complete without
deleting or overwriting `final`, which is the loss window OB-003 is about.

```go
func atomicRename(tmp, final string) error {
	if err := renameFile(tmp, final); err != nil {
		// Wrapped for context, %w so callers can still inspect the cause.
		return fmt.Errorf("publish %s: %w", final, err)
	}
	return nil
}
```

`%w` keeps the cause inspectable; `TestAtomicRename_MissingTempPreservesDestination` asserts
`errors.Is(err, fs.ErrNotExist)` still holds through the wrapping, and the injection cases assert
`errors.Is` against each injected sentinel.

**Seam.** `var renameFile = os.Rename` — one package-level variable, production always holds
`os.Rename`. Chosen over a filesystem interface or a transaction journal, neither of which has a
demonstrated requirement here.

**Caller change (one, narrow).** `appbackup.go:396` — the app-state restore path — was the only
caller that discarded nothing on failure, leaving a `.tmp` beside the destination. Because that
helper publishes `keystores/*`, the leaked staging file could be **plaintext keystore bytes** left
under a `.tmp` name. It now removes its own staging file and returns the error. Deleting `tmp`
there is safe: the live destination is intact and the new bytes remain in the verified tar.

**Assumptions, now stated in the code:** `tmp` and `final` are distinct paths on the same
filesystem; `tmp` is a staging file this process owns; on failure `tmp` is left for the caller to
discard. All five callers satisfy these.

---

## 5. Results

All commands run on the host above with native exit codes checked.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | targeted OB-003 run, **pre-fix** | **1** | 8 failures — the reproduction in §3 |
| 2 | `go test ./... -run 'TestAtomicRename\|TestExportAppBackup_FailedPublish\|TestRestoreAppBackup_FailedPublish' -count=1 -v` | **0** | 12 pass (8 top-level, 4 subtests) |
| 3 | `go test ./... -run 'Mirror\|Plan\|AppBackup\|Backup\|Incremental\|RenameCompat\|Copy' -count=1` | **0** | ok, 2.6 s |
| 4 | `go build ./...` | **0** | clean |
| 5 | `go vet ./...` | **0** | clean |
| 6 | `gofmt -l mirror.go appbackup.go atomic_replace_test.go` | **0** | no output |
| 7 | `go test ./... -count=1 -v` (full, uncached) | **1** | **212 pass / 2 fail / 4 skip**, 74.0 s |

`GOTMPDIR` was **not** set. No tool was installed and no platform was switched.

### The full-suite count, compared by identity

The historical figure was 204 / 2 / 4; **204 is not a target**. 212 − 204 = 8, exactly the eight
new top-level test functions in `atomic_replace_test.go`. No previously passing test changed state.

The **two failures are the same two**, failing for the same documented cause:

```
--- FAIL: TestBuildRestore_HostileFilenamesRoundTrip
--- FAIL: TestBuildFilelist_IsNulDelimited
    tar_names_test.go:145: cannot stage "tab\there.txt": ...
    The filename, directory name, or volume label syntax is incorrect.
```

That is **OBX-001** — `tar_names_test.go` puts a TAB filename in the unconditional fixture list
while the `runtime.GOOS != "windows"` guard covers only newline names. TAB is illegal in Win32
filenames. It is a fixture defect in the OB-008 tar tests, unrelated to this patch, and explicitly
**not authorized** for repair here. It remains open.

The **four skips are the same four**: `TestCardCheck_UnlistableDirBlocksFormat`,
`TestCatalogScale`, `TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget` (the last three
gated on `OBELISK_PERF=1` / environment).

### Test inventory

| Case | Test | Covers |
|---|---|---|
| A | `TestAtomicRename_MissingTempPreservesDestination` | missing temporary, existing destination: error **and** byte-identical survival; no injection needed |
| B | `TestAtomicRename_PublishesWhenDestinationAbsent` | success with no destination |
| C | `TestAtomicRename_ReplacesExistingDestination` | success replacing an existing destination — **executed on the Windows host** |
| D | `TestAtomicRename_InjectedFailuresPreserveDestination` | 4 injected causes: permission, cross-device, sharing violation, read-only |
| D′ | `TestAtomicRename_FailureDoesNotConsumeTemporary` | pins the helper/caller division of labour over `tmp` |
| E | `TestExportAppBackup_FailedPublishPreservesPreviousBundle` | real caller: previous period's bundle survives; failure reported; staging tidied |
| E′ | `TestRestoreAppBackup_FailedPublishPreservesExistingFile` | real caller: live `catalog.json` survives a failed restore |
| — | `TestAtomicRename_CommentDoesNotClaimDeleteFirst` | the false comment and `os.Remove(final)` cannot return |

---

## 6. The exact guarantee achieved

> When `atomicRename` fails, the file already at `final` is left byte-identical to what it was
> before the call, and the error is returned rather than absorbed. No code path deletes `final` as
> a response to a rename failure. When it succeeds, `final` holds exactly the staged bytes.

Extended to callers: for the two `appbackup.go` callers this is demonstrated end-to-end — the
previous bundle and the live `catalog.json` each survive a failed publication, and the caller
reports the failure instead of claiming success.

### What is NOT established

Stated so the next reader does not over-read the helper's name:

- **Not crash durability.** No directory fsync, unlike `writeCatalog`. A power loss during the
  rename is untested and unclaimed.
- **Not universal atomicity.** The single-operation replace is a property of the local
  filesystems exercised here. Not established for network filesystems, SMB/NFS shares, or FUSE.
- **Not concurrency safety.** Nothing tests another process writing `final` at the same time.
- **Not ACL/metadata preservation** across the replace.
- **Injected errors prove handling, not occurrence.** The four sentinels in case D show what
  `atomicRename` does *with* such an error. They are **not** evidence that Windows produces
  `ERROR_NOT_SAME_DEVICE` or a sharing violation in those circumstances. A real cross-device or
  real locked-handle test was not attempted, and the sentinel names describe the simulated cause
  only. `mustBytes` in the test file and the `withRenameFailure` doc comment both say this.
- **POSIX permission behavior is still unexercised** — the local suite cannot run POSIX permission
  tests, as `OB_STATUS.md` already records for OB-003. Only case A and case C run against a real,
  uninjected OS rename.
- **Windows race testing remains NOT TESTED.** No `-race` run was performed; nothing here changes
  that status.

---

## 7. Scope

**In this patch:** the helper, one narrow caller cleanup, the regression tests, the corrected
comment, and documentation.

**Deliberately untouched:** `store.go` and `catalog_open_test.go` (PR-01, verified byte-identical);
OB-002 batching; OB-004 source identity; keystore and configuration fixes; OBX-001; UI, agents,
database; the frozen handoff; `PR01_REVIEW_ADDENDUM/` (the wrong bundle — not used as
authoritative and not repaired here).

### Related issues observed, recorded not fixed

1. **`atomicRename` still performs no directory sync**, unlike `writeCatalog` — noted in
   `OB_STATUS.md` under OB-003. Out of scope for a bounded correction; needs its own evidence.
2. **`appbackup.go:396` writes restored keystores at `0o644`** via `os.WriteFile` before the
   rename. That is the existing permission finding already recorded in `OB_STATUS.md`; this patch
   deletes the leaked `.tmp` but does **not** change the mode. Not authorized here.
3. **`plans.go:764` and `mirror.go:252` swallow the helper's error into a counter**
   (`Unreadable++` / `Failed++`) without surfacing which file failed or why. The error is not
   discarded silently at the helper level, but operator-visible detail is thin. Not a data-loss
   path; worth a separate look.

---

## 8. Questions for the independent reviewer

1. **Is wrapping the rename error acceptable?** `fmt.Errorf("publish %s: %w", final, err)` adds
   context and preserves `errors.Is`. It does change the message text callers surface. Should the
   bare error be returned instead?
2. **Is the one caller change in scope?** `appbackup.go:396` now removes its staging file on
   failure. It is a leak fix, not a loss fix — justified here because the leaked file can hold
   keystore bytes. Reasonable, or should it be split out?
3. **Should cross-device be detected explicitly?** `OB_STATUS.md`'s suggested fix says "handle
   cross-device explicitly rather than by retry." This patch handles it by *failing with the
   underlying error and never copying*. No current caller can produce it (all stage in the
   destination directory). Explicit detection would need platform-specific error codes with no
   demonstrated requirement — is failing-with-the-real-error sufficient?
4. **Is `TestAtomicRename_CommentDoesNotClaimDeleteFirst` appropriate?** It greps `mirror.go` for
   a banned substring. It pins the reasoning rather than behavior, and would fail if
   `os.Remove(final)` legitimately appeared elsewhere in the file.
5. **Does case C need to run on POSIX before the guarantee is claimed generally?** It currently
   runs wherever the suite runs; the evidence here is Windows-only.
