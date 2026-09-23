# PR-02 / OB-003 — review follow-up: F-1 and F-2 applied

**This is the author's response to a same-session adversarial review, not independent approval.**
The review it answers —
[PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md](PR02-OB-003-INDEPENDENT-REVIEW-2026-09-07.md) —
records that limitation in its own opening paragraph, and it is preserved unmodified and unrenamed
so its provenance stays visible. Both documents were produced by the same session that wrote the
patch. A genuine second reviewer has still not seen this work.

Branch `fix/ob-003-safe-replacement`, HEAD `4cd867b2f730…` unchanged (nothing committed). PR-02
continues to be measured against the review base `4cd867b2ddb26c945f7c74faee8ef32780743be7`, never
`main`.

---

## 1. Artifact identities

| File | At session start | Now | Change |
|---|---|---|---|
| `appbackup.go` | `c8e8dde51514aee015ea3000e661ed2f80568dc2` | `f007d878b86ef83b8ce40c118cfb81d9d76f4060` | **comment only** (F-2) |
| `atomic_replace_test.go` | `8a61f4e799b15e54d6303ec359dfb9be8840fc9c` | `097a61e7f1aee356717aebee735c0547c2bae69c` | one test extended (F-1) |
| `mirror.go` | `c10a447dbcd4f4e9929ddf7e75eb664908a00ab0` | `c10a447dbcd4f4e9929ddf7e75eb664908a00ab0` | **unchanged** |
| `store.go` (PR-01) | `bfce18705e877c09d00d6c318767682d9128cf70` | `bfce18705e877c09d00d6c318767682d9128cf70` | **unchanged** |
| `catalog_open_test.go` (PR-01) | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` | **unchanged** |

SHA-256 now: `appbackup.go` `c088995ee6dba916c1b3f6657f84e9a7d8b435d187fc0643fa3fb8b060df27d9`;
`atomic_replace_test.go` `b46875d17a29c716ec5308f236feae47471d1755c5ce124dc3d0728cee1f0bf5`;
`mirror.go` `51801765bab2d2d868c7aa802f8527fb8fa893c3be9864054a2e46a6ec56a211`.

The replacement helper was **not** touched, and no platform-specific API was introduced.

---

## 2. F-2 — the comment, as actually saved

Diffed against `appbackup.go` as it stood at the start of this session:

```diff
@@ -395,8 +395,10 @@ func (a *App) RestoreAppBackup(tarPath string) (RestoreResult, error) {
 		if err := atomicRename(tmp, dest); err != nil {
 			// atomicRename leaves the existing dest untouched, so the live file is still
-			// good; drop our staging copy rather than leaving restored keystore bytes
-			// lying beside it under a .tmp name.
+			// good. Best-effort cleanup of this operation's staging file: the removal can
+			// fail too (its error is deliberately discarded here), and unlinking a path is
+			// not secure erasure of the bytes behind it. Either way the original
+			// publication error is what gets returned.
 			_ = os.Remove(tmp)
 			return err
 		}
```

**This is the whole production delta for this task — two comment lines removed, four added.** The
executable cleanup and publication behavior are byte-for-byte what the review examined.

The replacement no longer claims that staging bytes are always removed, no longer implies that a
cleanup failure is reported (it says the opposite, explicitly), does not present unlinking as
erasure, and says nothing about restored-key permissions. The accurate first sentence — that
`atomicRename` leaves `dest` untouched — was kept rather than restated.

---

## 3. F-1 — the test change

Extended the existing `TestRestoreAppBackup_FailedPublishPreservesExistingFile`. No new test
function, no new framework, no global hook. The pre-existing assertion that the previous
destination survives byte-identical is retained unchanged.

```diff
-	withRenameFailure(t, fs.ErrPermission)
+	// Fail every publication, recording which staging files the restore had actually
+	// created at the moment each rename failed. Without that record, the "it is gone
+	// afterwards" assertion below could pass vacuously — a file never created is also
+	// a file not found.
+	stagedAtFailure := map[string]bool{}
+	prevRename := renameFile
+	renameFile = func(tmp, final string) error {
+		stagedAtFailure[tmp] = fileExists(tmp)
+		return fs.ErrPermission
+	}
+	t.Cleanup(func() { renameFile = prevRename })
 
 	if _, err := app.RestoreAppBackup(tarPath); err == nil {
 		t.Fatal("a restore that cannot publish must report the failure")
 	}
 	mustBytes(t, catalog, sentinel, "failed app-state restore")
+
+	catalogTmp := catalog + ".tmp"
+	if !stagedAtFailure[catalogTmp] {
+		t.Fatalf("fixture proves nothing: %s was never created before publication failed (observed: %v)",
+			filepath.Base(catalogTmp), stagedAtFailure)
+	}
+	// Require a positive not-found. A permission or other stat error must fail the test
+	// rather than be read as successful cleanup.
+	if _, err := os.Lstat(catalogTmp); err == nil {
+		t.Errorf("%s must be removed after a failed publication, but it is still there", filepath.Base(catalogTmp))
+	} else if !errors.Is(err, fs.ErrNotExist) {
+		t.Errorf("could not establish that %s is gone: %v", filepath.Base(catalogTmp), err)
+	}
```

(The full-context comment block naming the catalog-only scope is in the source; trimmed here.)
No new imports were needed — `errors`, `io/fs`, `os` and `path/filepath` were already in use.

### Evidence the staging file existed before publication failed

The review's own probe risk was that an "it is gone" assertion could pass because the file was
never created. That is closed structurally rather than by assumption: the injected rename records
`fileExists(tmp)` **at the moment the publication fails**, inside the real production call path.
The test then *fails hard* (`t.Fatalf`, "fixture proves nothing") if that record does not show the
catalog staging file as present. A vacuous pass is therefore not reachable — the fixture must
prove the file existed before the absence assertion is allowed to mean anything.

The control run in §4 confirms this positively: with cleanup disconnected the test fails on the
**removal** assertion, not on the non-vacuity guard, which is only possible if the staging file was
genuinely created.

### Positive not-found, not merely "no error"

Cleanup success is established with `os.Lstat` plus `errors.Is(err, fs.ErrNotExist)`. A permission
error, an I/O error or any other stat failure takes the `else if` branch and **fails the test**
rather than being read as successful cleanup. Real filesystem, disposable `t.TempDir()`, synthetic
bytes; no fake deletion result.

### Scope of what this actually covers — stated precisely

`gatherMembers` builds a deterministic slice with `catalog.json` first, `config.json` is deferred
to the end of the restore, and the restore returns at its first failed member. With
`includeKeys=false` the bundle contains **no keystore member at all**. So the file exercised here
is the **catalog staging file**, `catalog.json.tmp`.

**This is not a keystore restoration or key-security test.** Keystore staging runs through the same
`writeFile` closure, but that path is not exercised by this fixture, and no claim is made that it
is. The test comment says so at the assertion itself.

---

## 4. The cleanup-line disconnection experiment

Run in an isolated disposable copy of the repository (`scratchpad\f12-probe`, `.git` excluded).
**The working checkout was never modified or reverted**; the production line is present in it
throughout.

| Probe state | Command | Exit | Result |
|---|---|---|---|
| cleanup line **intact** (as submitted) | `go test . -run TestRestoreAppBackup_FailedPublishPreservesExistingFile -count=1` | **0** | ok, 1.30 s |
| cleanup line **removed** (`return atomicRename(tmp, dest)`) | same | **1** | `atomic_replace_test.go:282: catalog.json.tmp must be removed after a failed publication, but it is still there` |

The assertion fails for the intended reason and identifies the exact staging pathname. Compare with
the state before this task, where the review demonstrated the same disconnection produced
`ok ... exit 0` — the suite noticed nothing. That gap is now closed for the exercised case.

---

## 5. Verification

Windows host, existing toolchain, disposable fixtures and synthetic data only. `GOTMPDIR` **not**
set. No tools installed, no platform switch.

| Command | Exit | Result |
|---|---|---|
| `go test ./... -run 'TestRestoreAppBackup_FailedPublishPreservesExistingFile' -count=1 -v` | **0** | PASS |
| `go test ./... -run 'TestAtomicRename\|TestExportAppBackup\|TestRestoreAppBackup\|AppBackup\|Backup\|Mirror\|Plan\|RenameCompat' -count=1` | **0** | ok, 1.74 s |
| `go build ./...` | **0** | clean |
| `go vet ./...` | **0** | clean |
| `gofmt -l appbackup.go mirror.go atomic_replace_test.go` | — | no output |
| `go test ./... -count=1 -v` (full, uncached) | **1** | **212 pass / 2 fail / 4 skip** |

The pass count is **unchanged at 212** because F-1 extended an existing test rather than adding
one — the expected outcome, not a target that was aimed at. These counts describe this run only.

The two failures are `TestBuildRestore_HostileFilenamesRoundTrip` and
`TestBuildFilelist_IsNulDelimited`, failing at `tar_names_test.go:80` and `:145` with
`cannot stage "tab\there.txt": ... The filename, directory name, or volume label syntax is
incorrect.` That is **OBX-001**, the pre-existing Windows TAB fixture defect — kept separate from
this patch and untouched. The four skips are the same environment-gated four.

**NOT TESTED:** Windows race testing (no `-race` run was performed; the status is unchanged). No
claim is made that Linux CI has validated this patch — it is uncommitted and has never reached CI.

---

## 6. What is and is not now established

- **Successful staging cleanup is now checked in the exercised case** — a failed app-state restore
  of the catalog member leaves no `catalog.json.tmp` behind, verified by positive not-found.
- **Cleanup can still fail, and that error is still discarded.** `_ = os.Remove(tmp)` is unchanged
  by design; the comment now states this instead of implying the opposite. Nothing reports a failed
  cleanup to the operator.
- **Removal is not secure erasure.** `os.Remove` unlinks; the bytes remain on the medium until
  overwritten. No test asserts otherwise and the comment no longer suggests otherwise.
- **Keystore staging is not covered** by this fixture (§3), only the catalog staging file.
- Unchanged and still separate: restored-keystore `0o644` permissions, crash durability and the
  missing directory sync, source/destination aliasing (OB-004 territory, incl. the `.mnemo_tmp`
  aliasing note), OBX-001, and general cleanup-error reporting.

### Follow-up observation, recorded not fixed

**Cleanup-failure observability.** When `os.Remove(tmp)` fails after a failed publication, the
staging file survives and nothing records it — no log line, no counter, no operator-visible signal.
This belongs to the same discarded-error family already tracked as **OBX-004**
(`LoadConfig`/`SaveConfig`/`loadJobs` discarding read and unmarshal errors); this is a further
instance at `appbackup.go:401`, in the restore staging path. Recorded here as an observation
against the existing OBX-004 family — **no new ID is minted and the backlog is not renumbered**,
and it is deliberately not fixed in this bounded patch.

---

## 7. Status

Both requested corrections are applied and checked in this session:

- **F-1 APPLIED** — assertion added, non-vacuity guaranteed by construction, control experiment
  confirms it fails when the production line is disconnected.
- **F-2 APPLIED** — comment saved in `appbackup.go`; production delta is comment-only.

Optional items from the review were **not** taken and remain open at the author's discretion: F-3
(the seam's `t.Parallel()` convention note) and Q4's narrowing of the comment-grep test.

Nothing was committed, pushed or merged; PR-03 is not started. **READY_FOR_FRESH_REVIEW** — by a
reviewer who has not seen this patch before.
