# PR-02 / OB-003 — independent review

**Outcome: NEEDS_CHANGES** — bounded. The OB-003 correction itself is **accepted**: the
destructive fallback is genuinely gone, the simpler implementation choice is the right one, and it
is supported by real evidence rather than assertion. What is required is small and confined to one
test assertion and one comment. Nothing about the helper needs reworking.

> **Independence limit, stated first.** This review was produced in the same session as the
> implementation, by the same author. It is therefore **not** an independent second party. To
> compensate, every reported fact was re-derived from the repository — blob ids rather than quoted
> hashes, a full suite run of my own, and a **disposable-copy control experiment** that
> deliberately tried to make the patch's tests fail. Findings F-1 and F-2 are the result of that
> adversarial pass, not of reading the author's report. A genuine second reviewer is still
> warranted before this is treated as reviewed.

---

## 1. Identity, verified not assumed

| | |
|---|---|
| Review base | `4cd867b2ddb26c945f7c74faee8ef32780743be7` |
| Branch | `fix/ob-003-safe-replacement` |
| HEAD | `4cd867b2ddb26c945f7c74faee8ef32780743be7` (no commits — matches "uncommitted") |
| Staging | empty |
| In-progress git operation | none |

Diffed against the base, **not** `main`. `main` is at `d97809b2`; diffing there would re-present
PR-01 as PR-02 work.

### Reviewed artifacts

| File | State | SHA-256 | Blob |
|---|---|---|---|
| `mirror.go` | modified | `51801765bab2d2d868c7aa802f8527fb8fa893c3be9864054a2e46a6ec56a211` | `c10a447dbcd4f4e9929ddf7e75eb664908a00ab0` |
| `appbackup.go` | modified | `731fd293bf359cd530a8923c85000371ec51c95377f27d62cb13c58aa2f33af6` | `c8e8dde51514aee015ea3000e661ed2f80568dc2` |
| `atomic_replace_test.go` | **new, untracked** | `b1077b11e4b5864c834c245bcee023b94489267e211ec9fefef32ac1c9e799a4` | `8a61f4e799b15e54d6303ec359dfb9be8840fc9c` |

The test file is untracked, so `git diff` omits it; it was read in full from disk. `git cat-file -e`
confirms it does not exist at the base, so it is genuinely new rather than a modified file.

### PR-01 is untouched — checked against the base blobs

| File | Working tree blob | Base blob | Identical |
|---|---|---|---|
| `store.go` | `bfce18705e877c09d00d6c318767682d9128cf70` | `bfce18705e877c09d00d6c318767682d9128cf70` | yes |
| `catalog_open_test.go` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` | yes |

Verified by object identity against the review base, not by comparing to a hash quoted in the
report. Neither path appears in the diff.

All four expected untracked doc trees are preserved. Nothing was fetched, pulled, switched, reset,
stashed, cleaned, restored, staged, committed or merged.

---

## 2. Findings, most severe first

### F-1 — REQUIRED. The patch's only new caller-side line has no test coverage. Proven.

**Anchor:** [`appbackup.go:396-403`](../../../appbackup.go) — the `_ = os.Remove(tmp)` added to
`RestoreAppBackup`'s `writeFile`. **Test anchor:**
`atomic_replace_test.go:239-258` `TestRestoreAppBackup_FailedPublishPreservesExistingFile`.

That test asserts the live `catalog.json` survives — which it would with or without the new line,
because survival is the *helper's* doing. It never inspects whether the staging file was removed.
The one assertion that would cover the new line is absent.

**Demonstrated by control experiment**, in a disposable copy of the repo (never the working
checkout). With the patched helper in place I removed only the new cleanup line:

| Probe | Result |
|---|---|
| patched `appbackup.go` (as submitted) | no `.tmp` left in the data dir — **PASS** |
| new cleanup line removed | `staging files left behind after a failed restore: [catalog.json.tmp]` — **FAIL** |
| **shipped suite** with the cleanup line removed | `ok ... 0.154s` — **exit 0, notices nothing** |

So the line is reached, it is effective, and its removal is invisible to the suite. That is
untested new production code in this patch — not a deferred broader guarantee.

**Failure scenario:** a later refactor of `writeFile` drops or reorders the cleanup. Every test
still passes. A failed restore then leaves `catalog.json.tmp` — and with `include_keys`,
`keystores/*.tmp` containing restored key material — beside the live files, with nothing reporting
it.

**Required fix:** one assertion in the existing test — after the failed restore, assert no `.tmp`
remains in the data dir. No new test function is needed.

### F-2 — REQUIRED. The new cleanup comment overstates what `os.Remove` achieves.

**Anchor:** `appbackup.go:397-399`:

> `// good; drop our staging copy rather than leaving restored keystore bytes`
> `// lying beside it under a .tmp name.`

Two inaccuracies, both material because the sentence is specifically about key material:

1. **The removal is unchecked.** `_ = os.Remove(tmp)` discards its error. If removal fails — the
   file is locked, the directory is read-only, the very conditions under which the rename just
   failed — the keystore bytes **are** left lying beside the destination, and nothing reports it.
   The comment states the outcome as achieved rather than attempted.
2. **Unlink is not erasure.** `os.Remove` unlinks; the bytes remain on the medium until
   overwritten. The sentence reads as though the key material is disposed of.

This matters more than typical comment drift because PR-01's F-2 was itself a comment that
justified behavior with a false premise. The patch is otherwise scrupulous about not overclaiming;
this line is out of step with it.

**Required fix:** reword to state the intent and its limits — a best-effort unlink of our own
staging copy, not a guarantee of removal and not secure erasure.

### F-3 — Optional. The new seam does not carry the package's own convention note.

**Anchor:** `mirror.go:315-319` (`var renameFile = os.Rename`); `atomic_replace_test.go:38-45`.

`renameFile` is package-level mutable state swapped by tests. The package already has two such
seams, and **both** document the rule that protects them:

- `durability_gate_test.go:9` — *"The build test shares the package-level failSave seam, so none call `t.Parallel()`."*
- `build_verify_test.go:9` — *"Tests here share the package-level build hooks, so none call `t.Parallel()`."*

`atomic_replace_test.go` carries no equivalent note. **No live defect:** I confirmed no test in the
package calls `t.Parallel()`, and `withRenameFailure` restores the previous value via `t.Cleanup`,
including per-subtest in the table case. But the convention exists precisely to keep it that way,
and this is the third seam to rely on it silently.

### F-4 — Observation, pre-existing, NOT this patch. Limits of the "distinct paths" assumption.

The helper's comment says tmp and final are "distinct paths in the same directory". That is true
**lexically**, and true for every current caller. It is not a structural guarantee:

- A source file literally named `X.mnemo_tmp` yields a destination path identical to the staging
  path used when publishing `X`. Nothing filters `mirrorTmpSuffix` from mirror or plan sets
  (`grep` shows the constant is used only to build staging names, never to skip inputs).
- Lexical construction says nothing about hostile aliases, symlink/junction destinations, or a
  mount that changes underneath the operation.

Pre-existing, requires an adversarially named source file, and belongs to OB-004's territory
(source/destination identity), not here. Recorded so the assumption is not later read as proof.

### F-5 — Observation. The handoff's "process interruption" test is not provided.

`02_FIRST_PULL_REQUESTS.md` PR-02 lists tests for "missing temporary file, full destination,
sharing/permission failure, cross-device attempt and **process interruption**". The first four are
covered (the middle ones by injection — see §4). Process interruption is not tested.

This is **consistent, not contradictory**: the patch explicitly declines to claim crash durability,
and the helper still performs no directory sync. But the handoff item is unmet and should remain
open rather than be considered discharged by PR-02.

### F-6 — Observation. One test does not discriminate the bug, as the author says.

`TestAtomicRename_FailureDoesNotConsumeTemporary` **passes against the pre-patch code** — I
verified this in the disposable copy. It pins the helper/caller division of labour over `tmp`; it
is not a regression test for OB-003. The report describes it exactly that way, so this is
confirmation of an accurate claim, not a discrepancy.

---

## 3. The helper and every caller

Read in full. `atomicRename` is now a single `renameFile` call; on error it wraps and returns; on
success it returns nil. There is no second rename, no removal of `final`, no copy path, and no
rollback. Repo-wide grep for `os.Remove(final|dest|destPath|tarPath)` finds **no** surviving
destructive publication fallback (`pipeline.go:1110` is a config-gated deletion of an intermediate
tar after its proofs complete — unrelated).

| Caller | Staging path | Same dir | Cleanup on failure | Can cleanup delete `final`? |
|---|---|---|---|---|
| `mirror.go:252` | `destPath + ".mnemo_tmp"` | yes | `os.Remove(tmp)`, `Failed++` | no |
| `mirror.go:308` `copyVerifyToDest` | `destPath + ".mnemo_tmp"` | yes | `os.Remove(tmp)`, error returned | no |
| `plans.go:764` | `dest + ".mnemo_tmp"` | yes | `os.Remove(tmp)`, `Unreadable++` | no |
| `appbackup.go:220` | `tarPath + ".tmp"` | yes | `os.Remove(tmp)`, error returned | no |
| `appbackup.go:396` | `dest + ".tmp"` | yes | `os.Remove(tmp)` **(new)**, error returned | no |

Five callers, matching the report. Every cleanup targets the staging path only; none can reach
`final`, subject to F-4's aliasing caveat. Normal replacement and first publication both remain
supported (§4, cases B and C).

**Error identity survives.** `fmt.Errorf("publish %s: %w", ...)` preserves `errors.Is`. I checked
the specific way wrapping could have broken callers: `os.IsNotExist` / `os.IsPermission` do **not**
unwrap, so a caller using them would silently start taking the wrong branch. Grep across
`mirror.go`, `plans.go` and `appbackup.go` finds **no** such check on the result, and no test
couples to the message text.

**On the new `appbackup.go` cleanup specifically**, as the brief asks: it removes only the staging
artifact that this operation created (`dest + ".tmp"`, written moments earlier by this same
closure); it cannot touch the previous destination or an unrelated file; and it does **not** mask
the publication failure — the original error from `atomicRename` is returned unchanged after the
removal attempt. The one inaccuracy is how the comment represents a failed removal (F-2), and its
effect is untested (F-1). It is correctly **not** entangled with the separately tracked
restored-keystore `0o644` permissions issue, which this patch rightly leaves alone.

---

## 4. Tests and reproduction

**Which tests exercise the real OS, and which use injection** — the distinction the brief asks for:

| Test | Rename path | What it proves |
|---|---|---|
| `MissingTempPreservesDestination` | **real `os.Rename`**, no injection | genuine OS failure; destination survives byte-identical |
| `PublishesWhenDestinationAbsent` | **real** | real first publication |
| `ReplacesExistingDestination` | **real** | real replacement over an existing file, on Windows |
| `InjectedFailuresPreserveDestination` (×4) | **injected** | handling only — *not* OS occurrence |
| `FailureDoesNotConsumeTemporary` | injected | contract pinning (see F-6) |
| `ExportAppBackup_FailedPublish…` | injected | real caller publication + cleanup |
| `RestoreAppBackup_FailedPublish…` | injected | real caller; **coverage gap at F-1** |

No success case depends on a fake returning nil — B and C both drive the real OS rename. That is
the right structure, and it is what makes the "no Windows API needed" conclusion credible.

The injection seam is sound: it replaces the primitive *inside* the real helper, so assertions run
against the production error path rather than a stub. The assertions would catch a destructive
retry — `mustBytes` fails the test if the destination cannot be read at all, which is exactly the
symptom the old code produced. Restoration is per-test via `t.Cleanup`, and the table case installs
inside each subtest, so no state leaks between tests (with F-3's caveat).

**Independent reproduction.** I copied the repo to a disposable directory, reverted `atomicRename`
to its pre-patch body there, and ran the new tests. The working checkout was never reverted.

```
--- FAIL: TestAtomicRename_MissingTempPreservesDestination
--- PASS: TestAtomicRename_PublishesWhenDestinationAbsent
--- PASS: TestAtomicRename_ReplacesExistingDestination
--- FAIL: TestAtomicRename_InjectedFailuresPreserveDestination  (all 4 subtests)
--- PASS: TestAtomicRename_FailureDoesNotConsumeTemporary
--- FAIL: TestExportAppBackup_FailedPublishPreservesPreviousBundle
--- FAIL: TestRestoreAppBackup_FailedPublishPreservesExistingFile
--- FAIL: TestAtomicRename_CommentDoesNotClaimDeleteFirst
```

This reproduces the author's pre-fix table exactly, including which two tests pass. The failure
message confirms the mode is destruction, not mutation: *"destination unreadable after the
operation … The system cannot find the file specified."* The tests genuinely discriminate the bug.

**Concrete untested behavior that matters to this patch:** F-1 only. I am not asking for tests to
raise counts; a real cross-device rename, a real locked-handle rename and a process-interruption
test would each need setup this patch deliberately does not claim.

---

## 5. Verification I ran myself

Windows host, existing toolchain, disposable fixtures and synthetic data only. No live catalog,
keystore, backup, NAS original, archival medium or hardware was touched. `GOTMPDIR` was **not**
set. No tools installed, no platform switch.

| Command | Exit | Result |
|---|---|---|
| targeted OB-003 tests | **0** | ok, 0.86 s |
| `-run 'Mirror\|Plan\|AppBackup\|Backup\|Incremental\|RenameCompat\|Copy'` | **0** | ok, 3.39 s |
| `go build ./...` | **0** | clean |
| `go vet ./...` | **0** | clean |
| `gofmt -l` on the three changed files | — | no output |
| `go test ./... -count=1 -v` (full, uncached) | **1** | **212 pass / 2 fail / 4 skip** |
| disposable-copy pre-patch reproduction | **1** | reproduces the author's table |
| disposable-copy cleanup control (F-1) | — | see F-1 |

**The full-suite result is independently reproduced, not reported-only.** 212 / 2 / 4 matches the
author's figure, and the failing identities and causes match by inspection of my own run:

```
tar_names_test.go:80:  cannot stage "tab\there.txt": ... syntax is incorrect.
tar_names_test.go:145: cannot stage "tab\there.txt": ... syntax is incorrect.
```

Those are `TestBuildRestore_HostileFilenamesRoundTrip` and `TestBuildFilelist_IsNulDelimited` —
**OBX-001**, the TAB-in-Win32-filename fixture defect, pre-existing and untouched here. Kept
separate from this patch's assessment. The 4 skips are the same environment-gated four. 212 − 204 =
the 8 new test functions; no previously passing test changed state.

**NOT TESTED, unchanged by this review:** Windows race testing (`-race` was not run and did not
become available); POSIX behavior of any kind, including permission semantics; real cross-device
rename; real sharing-violation rename; process interruption.

---

## 6. Answers to the report's five questions

**Q1 — Is wrapping the rename error acceptable? → Yes, keep it.** I specifically tested the way it
could bite: `os.IsNotExist`/`os.IsPermission` do not unwrap, and a caller using them would silently
change branch. No caller does, and no test couples to message text. `errors.Is` is preserved and
asserted. The message is mildly redundant (`publish X: rename tmp X: …`) since `LinkError` already
names both paths — cosmetic, not worth a change.

**Q2 — Is the one caller change in scope? → Yes, accept it.** It is one line inside the failure
branch the patch already had to touch, it addresses a leak of key material, and it does not mask
the original error. Keep it here rather than splitting it out — but finish it: F-1 (test) and F-2
(comment).

**Q3 — Should cross-device be detected explicitly? → No. Failing with the underlying error is
sufficient and is the better choice.** The roadmap's "platform-appropriate replacement primitive"
and "explicit cross-device handling" were proposed approaches, not requirements. No current caller
can produce `EXDEV` — every one stages inside the destination directory — so explicit detection
would add platform-specific error-code handling with no demonstrated requirement and no way to test
it honestly. Critically, the error is **not swallowed**: if a future caller ever stages across
filesystems, it surfaces with its real identity, which is precisely what the brief requires.

**Q4 — Is `TestAtomicRename_CommentDoesNotClaimDeleteFirst` appropriate? → Keep it, with one
reservation.** It is cheap and it guards the *reasoning* that produced the defect, which is
unusually valuable here. The reservation is that banning the literal substring `os.Remove(final)`
across the whole file is broader than the intent: a future legitimate use elsewhere in `mirror.go`
would fail it, and a grep cannot tell code from comment. Optional improvement: scope the check to
the `atomicRename` body. Not a blocker.

**Q5 — Does case C need POSIX evidence before the guarantee is claimed? → No, because the general
claim is not made.** The comment scopes it to "the host that runs the suite", which is the honest
form. The test is unconditional, so Linux CI will supply POSIX evidence automatically once it runs
— worth noting that CI is `ubuntu-latest` only (per OBX-001), so POSIX is the platform CI *will*
cover and Windows is the one that needs the local run this patch performed.

---

## 7. The simpler implementation choice — accepted

The roadmap proposed a Windows replacement primitive. The author instead established **by
execution on the actual Windows host** that `os.Rename` already replaces an existing destination,
and that the delete-retry path was never reached in ordinary success. I reproduced both facts.

That is the correct engineering call, and the correct order of operations: behavior verified first,
then code removed. Adding a `MoveFileEx` wrapper to match the historical plan would be unnecessary
platform-specific code with no demonstrated requirement. **Accepted.**

The comment's claim that Go's `os.Rename` issues `MoveFileEx` with `MOVEFILE_REPLACE_EXISTING` is
an accurate statement about the Go runtime, and — more importantly — the patch does not *rely* on
it: the behavior is pinned by an executing test, so the code stays correct even if that internal
detail changes.

---

## 8. Guarantee established, and what remains unproven

**Established, and I verified it independently:**

> When `atomicRename` fails, the file already at `final` is left byte-identical, and the error is
> returned with its underlying identity intact. No code path deletes `final` in response to a
> rename failure. When it succeeds, `final` holds exactly the staged bytes. This holds through two
> real callers: a previous app-backup bundle and a live `catalog.json` each survive a failed
> publication, and the caller reports failure rather than success.

**Correctly excluded by both the code comments and the report** — I checked each exclusion is
actually stated and not quietly contradicted elsewhere: power-loss durability; directory-sync
guarantees (still no fsync, unchanged); concurrent-writer safety; network-filesystem behavior; ACL
preservation. The report's separation of "injected errors prove handling, not OS occurrence" is
carried into the test file's own comments, not just the report. The helper's *name* is not used
anywhere to imply more than is proven.

**Remaining open, none of them regressions from this patch:** the missing directory sync (OB-003's
own recorded remainder); OBX-001; the restored-keystore `0o644` mode; source/destination identity
(OB-004, cf. F-4); process interruption (F-5).

---

## 9. Required to clear this review

Both are bounded and belong in one response, not separate cycles:

1. **F-1** — add an assertion to `TestRestoreAppBackup_FailedPublishPreservesExistingFile` that no
   `.tmp` staging file remains in the data dir after the failed restore.
2. **F-2** — reword the `appbackup.go:397-399` comment so it does not state removal as achieved,
   and does not read as secure erasure.

Optional, author's discretion: F-3 (seam convention note), Q4's narrowing of the grep test.

No change is required to `atomicRename` itself, to the caller map, or to the test structure. Once
F-1 and F-2 land, this is **READY_FOR_OWNER_REVIEW** in my assessment — subject to the independence
limit recorded at the top: a genuine second reviewer has still not seen this patch.

**Outcome: NEEDS_CHANGES.**
