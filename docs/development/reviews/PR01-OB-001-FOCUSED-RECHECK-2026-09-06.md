# PR-01 / OB-001 — focused recheck

**Outcome: NEEDS_CHANGES**

One item, and only one: **the F-2 production comment correction has not been applied.**
`store.go` on disk is byte-identical to the version the independent review examined. The
"explicitly authorized comment-only correction in `store.go`" described in the recheck brief is
**not present in the working tree**, and no F-2 closeout report exists under
`docs/development/reviews/`.

F-1, F-3, F-4 and F-5 are **CLOSED**, each verified by running the code rather than by reading the
follow-up's account of it. No new blocker was found in them, no new design workstream is proposed,
and no production-logic change is requested.

Review only. Nothing was edited, staged, committed, pushed or merged. All probing ran in
throwaway package copies under the session scratchpad.

---

## 1. Material difference from the documented follow-up — read this first

The brief describes a review target of "tests/documentation, followed by an explicitly authorized
comment-only correction in `store.go`". **The second half of that has not happened.**

| Expected per the brief | Found on disk |
|---|---|
| `store.go` carries a corrected F-2 comment | `store.go` SHA-256 `8baa0842…24bc56` — **identical** to the pre-follow-up, independently reviewed file |
| An F-2 closeout report exists | `docs/development/reviews/` holds three files; there is no closeout report |

The wrong sentence is still at `store.go:1086-1088`, quoted verbatim from the current file:

```go
	// writeCatalog, dailyBackup (create and prune) and backupBeforeMigrate; it does NOT
	// cover the jobs.json sidecar (saveJobs), which OpenStore only ever reads.
```

A search of the whole file for the corrected wording (`loadJobs DOES write`,
`only after the read classification`) returns nothing.

This is **consistent with the follow-up report**, which states plainly in its section 1 that the
edit was not made because the follow-up task forbade touching `store.go`, and which supplies the
replacement text. So the follow-up did not overclaim — the brief's premise is simply ahead of the
tree. Reported here rather than treated as the same target, and **not fixed**, because this pass is
review-only.

---

## 2. Reviewed artifact identity

| Property | Value |
|---|---|
| Git root | `C:/Users/Nathaniel/Documents/Software Development/Mnemosyne/mnemo-go` |
| Branch | `fix/ob-001-catalog-open` |
| `HEAD` | `d97809b2f730e09960632e2943e562531fa095a3` — nothing committed |
| Staged content | none |
| Tracked modifications | ` M store.go` only |
| In-progress git operation | none (`MERGE_HEAD`, `REBASE_HEAD`, `rebase-merge`, `rebase-apply`, `CHERRY_PICK_HEAD`, `BISECT_LOG`, `REVERT_HEAD`, `SEQUENCER` all absent) |

Full hashes, not truncations (F-5 asks for exactly this):

| File | Lines | SHA-256 | git blob |
|---|---|---|---|
| `store.go` (working tree) | 3527 | `8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56` | `04f0b5051a83a43aa5d9a8c1611884497e484808` |
| `catalog_open_test.go` (**untracked**, explicitly included) | 747 | `13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` |
| `store.go` at base `HEAD:store.go` | — | — | `00d47561962b4c59f3c4c9fbc8245da726675ec0` |

`catalog_open_test.go` is untracked and absent from `git diff`; it was read directly from disk in
full. `git diff --stat` still reports `store.go | 83 +++---, 1 file changed, 80 insertions(+), 3
deletions(-)` — unchanged from the original review.

### Is executable production code unchanged?

**Yes, and more strongly than asked.** The `store.go` delta since the independent review is not
merely comment-only — it is **empty**. Byte-identical content, identical git blob id. There is no
executable change to inspect because there is no change at all. The production decision logic
reviewed and executed in the independent review is exactly what is on disk now.

### Review chain read

All four local files under `docs/development/reviews/` and the two living status documents:

- `PR01-OB-001-2026-09-06.md` (implementation report, 393 lines) — its append-only correction block
  is at section 8, line 346, and the original erroneous "431 lines" row at line 10 is **intact
  above it**, confirming the correction was appended rather than a rewrite.
- `PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md` — unchanged, SHA-256
  `1b5cc6042dce58acf015debb18f96bfaf6139d3a3c9fc1a80a2f5fe25beab33b`.
- `PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md`.
- `OB_STATUS.md`, `NEXT_ACTIONS.md`, `BASELINE-2026-09-06.md` (untouched), `docs/CONTRIBUTING.md`.

There is **no F-2 closeout document**. Nothing was invented to fill that gap.

---

## 3. Item-by-item verdict

| Item | Verdict |
|---|---|
| F-1 zero-length history | **CLOSED** |
| F-2 jobs-write reachability, production comment | **STILL_OPEN** |
| F-3 backup-prune positive control | **CLOSED** |
| F-4 fixture-state comparison | **CLOSED** |
| F-5 artifact identity in reports | **CLOSED** |

### F-1 — CLOSED

Both offending comments are gone from `catalog_open_test.go`. The header now carries a block titled
"What the zero-length case actually did, corrected (independent review, F-1)" stating that a
zero-length catalog "did NOT take that path at baseline", that `json.Unmarshal` rejected it before
`existed` was consulted, and that `existed = len(b) > 0` "was a LATENT HAZARD, never a live defect".
The doc comment on `TestOpenStore_ZeroLengthCatalogIsDamagedNotNew` now says the test "pins a
MESSAGE and the point at which the refusal happens — it does not pin a repaired overwrite, because
there was none."

The distinction the brief asks for is drawn explicitly: existing rejection behavior versus the
clearer diagnostic. The comment also records what the test is still worth — naming the
`.bak-YYYYMMDD` sidecar, and pinning `existed = len(b) > 0` shut against a refactor that moves the
parse — so "only a message" is not misread as "pointless".

Regression coverage is intact: the test still asserts refusal, no usable `Store`, zero attempted
writes, and fixture preservation. It passes.

### F-2 — STILL_OPEN

The correction is present in `catalog_open_test.go` (a full six-line reachability trace with
anchors), in the PR-01 report's correction 2, and in the `OB_STATUS.md` dated block. It is **absent
from `store.go`**, which is the one place the finding named.

The underlying analysis was re-verified against current source and is correct:

| Anchor | Fact |
|---|---|
| `store.go:3370` | `loadJobs` |
| `store.go:3400-3401` | `if changed { s.saveJobs() }` — fires when a `RUNNING` job is reconciled to `INTERRUPTED` |
| `store.go:3350`, `3362-3363` | `saveJobs` writes `jobs.json.tmp` and renames it |
| `store.go:1283`, `1284` | `s.loadJobs()` immediately before `return s, nil` |
| `store.go:1164`, `1172`, `1183` | all three refusals return earlier |

So a successful open after an unclean shutdown **does** write the sidecar, and the exclusion holds
only by control flow — never by "OpenStore only ever reads" it. **This is a comment defect, not a
behavioral one.** No test fails because of it; nothing an operator sees changes. The remedy is the
four-line replacement already drafted in the follow-up report's section 1.

### F-3 — CLOSED

The control reaches the real retention loop and is not vacuous. Verified by running it, and by
re-running the disconnection experiment first-hand.

**It reaches the production path.** `TestPersistObserver_SeesBackupPrune` seeds a real catalog,
writes 15 synthetic `catalog.json.bak-2020MMDD` sidecars, arms the spy, then opens a **second**
`Store` and calls `AddCollection`, which flows `save` to `writeCatalog` to `dailyBackup` to the
retention loop at `store.go:1483-1486`. The observer is installed only through `openStore`; the
test never invokes `notePersist` or the observer function directly.

The second-`Store` device is necessary and correct: `dailyBackup` returns early while `s.lastBak`
already equals today (`store.go:1470-1473`), so pruning fires only on a `Store`'s first write of the
day, and the fixture store has taken that slot. Determinism comes from `dailyBackup` sorting its
glob by **name** (`store.go:1481`), so far-past 2020 dates fix the outcome. No sleep, no clock
manipulation, no mtime dependence, no retention-constant change.

**Expectations are not vacuous, and not derived solely from the observer.** The load-bearing
assertions are independent disk checks:

- `len(pruned) == 0` is a `t.Fatalf`, so an unwired observer fails rather than reporting a clean zero;
- every reported prune path must be gone from disk (`os.Stat` must yield `fs.ErrNotExist`);
- exactly 14 sidecars must remain, counted by globbing the directory;
- `bak-20200101` and `bak-20200102` must be absent from disk **and** present in the observer's
  report — checked in both directions;
- `bak-20200115` must survive.

One term is observer-derived: `wantPrunes := len(before) + len(created) - 14` uses the observed
`backup-create` count. I checked whether that could mask a defect and it **fails safe** — if the
observer under-reported a create, `wantPrunes` would come out one too small and the equality
assertion would fail, not silently pass. `len(before)` comes from disk and `14` is the policy
constant, so the arithmetic is anchored outside the thing under test. Not a blocker; noted as an
optional tidy in section 6.

**The disconnection experiment, re-run first-hand** in a fresh disposable copy carrying the current
test file (hash `13bc5e2e…87c9a3` confirmed inside the copy), with the single line
`s.notePersist("backup-prune", matches[0])` deleted from `store.go` and nothing else changed:

```
catalog_open_test.go:443: the observer recorded NO backup-prune for a write that must
prune 16 sidecars down to 14; recorded [catalog-write catalog.json]
--- FAIL: TestPersistObserver_SeesBackupPrune (0.03s)                    exit=1
```

The failure names the right thing, and the message is itself evidence that the production path ran:
16 sidecars were present, and `catalog-write` was still observed, so only the prune notification
went missing. This is my own run, not the follow-up's reported output.

### F-4 — CLOSED

**Namespace, types and contents.** `snapshotFixture` walks the entire fixture directory with
`filepath.WalkDir` and records, per entry keyed by slash-separated relative path: `kind` (`file`,
`dir`, `symlink`, `irregular`), `size`, and SHA-256 of the exact bytes for regular files, or the raw
link target for links. `catalog.json.tmp`, `jobs.json`, `.pre-schema-*` and new subdirectories are
covered by construction rather than by enumeration of expected patterns.

**Explicit failure on inspection errors.** Every `WalkDir` error, `filepath.Rel` error,
`os.Readlink` error and `os.ReadFile` error is wrapped and surfaced through a single `t.Fatalf`.
Irregular entries (device, socket, pipe) are recorded by kind rather than skipped, so their
appearance or disappearance still fails the comparison. `snapshotFixture` additionally fails if
`catalog.json` is missing, so an empty snapshot cannot trivially match.

**Does not follow links out of the fixture — verified empirically, with one gap.** POSIX symlink
creation needs elevation on this machine, so that exact branch could not be exercised
(`A required privilege is not held by the client` — recorded as a skip, not a pass). A Windows
directory **junction**, which needs no elevation, exercises the same reparse-point branch and was
tested:

```
mklink: Junction created for ...\001\escape <<===>> ...\002
recorded 'escape' as kind="symlink" target="...\002"
traversal stayed inside the fixture; 3 entries recorded
```

The link is recorded by target and not walked through; the file planted outside the fixture never
appeared in the snapshot.

**It complements the observer rather than replacing it — demonstrated, not assumed.** The brief
asks for this specifically, so I tested both directions on the same injected source.

*A/B probe, re-run first-hand.* Two unobserved stray writes were injected into the refuse branch of
`openStore` (`catalog.json.tmp` and `jobs.json`). The fixture comparison catches both:

```
catalog_open_test.go:547: refused startup (permission denied): catalog.json.tmp was CREATED
  (file 5 bytes sha256:e224ddc6b55af8b2) — the refused open wrote something
catalog_open_test.go:547: refused startup (permission denied): jobs.json was CREATED
  (file 2 bytes sha256:44136fa355b3678a) — the refused open wrote something
--- FAIL: TestOpenStore_UnreadableCatalogFailsClosed/permission_denied            exit=1
```

*Complementarity probe, added by me.* Against that same injected source, what did the observer
alone say?

```
observer recorded 0 attempted authority write(s): []
  ...yet catalog.json.tmp WAS created on disk
  ...yet jobs.json WAS created on disk
CONFIRMED: observer says zero attempts while real files appeared
```

The observer is structurally blind to uninstrumented paths; the fixture comparison is the only
thing that catches them. Conversely the disconnection experiment in F-3 shows the observer carries
the "attempted" half that file state cannot express. Both are asserted in every rejected-read test,
and neither is asserted alone. That is complementarity, evidenced.

Both A/B probes' evidence is available and was reproduced in this pass; nothing rests on the
follow-up's reported output.

### F-5 — CLOSED

The reports distinguish the artifacts correctly, and I verified the figures against the files
rather than accepting them:

- The PR-01 implementation report's original "431 lines" row survives at line 10, with an
  **append-only** correction block at section 8 identifying 431 as wrong and the reviewed artifact
  as 509 lines, SHA-256 `69972b5d…7d200c`. Nothing above the block was edited.
- The follow-up report's identity table lists `store.go` before and after with the same full hash
  `8baa0842…24bc56` (byte-identical), and `catalog_open_test.go` as 509 lines
  `69972b5d…7d200c` before, 747 lines `13bc5e2e…87c9a3` after. **Both match the files on disk**,
  checked with `sha256sum`, `git hash-object` and `wc -l` in section 2 above.
- The follow-up states explicitly that "509 is now historical too" and that "neither number is a
  target". The independent review's conclusions are correctly left as historical evidence about the
  509-line artifact; nothing rewrites it as though it had examined the 747-line file.

Full hashes are recorded in section 2 rather than truncations, per the brief.

---

## 4. The bounded guarantee accepted

**Accepted, unchanged:**

> After a **rejected** catalog read, `OpenStore` does not proceed into catalog or backup mutation.
> No usable `Store` is returned, the error preserves the underlying cause, and the fixture
> directory is byte-identical.

**Explicitly not accepted, and not claimed anywhere in the current artifacts:**

> `OpenStore` never performs any filesystem mutation.

`os.MkdirAll(dataDir, 0o755)` at `store.go:1133` runs **before** the read and does mutate the
filesystem. The test file states this boundary in its own header ("The guarantee is NOT 'no
filesystem write anywhere during `OpenStore`'"), and the follow-up and independent review both
state it. No document overreaches.

**No concrete reachable gap was found.** Re-checking the six mutating call sites in `store.go`
against a rejected open: `MkdirAll` (`1133`) precedes the read and is outside the claim by design;
`catalog-write` (`1409`, `1424`), `backup-create` (`1478`), `backup-prune` (`1484`) and
`pre-schema-backup` (`1346`) are all downstream of the refusal returns at `1164`/`1172`/`1183` and
all instrumented; `saveJobs` (`3362-3363`) is unreachable because `loadJobs` (`1283`) sits after
those returns. **No general filesystem abstraction is required to close this review**, and none is
recommended.

---

## 5. Checks performed

Disposable `t.TempDir()` fixtures and synthetic data only. No production catalog, real keystore,
NAS original, backup destination, tape or optical medium touched. **No `GOTMPDIR` override** — the
normal test environment was preserved, confirmed before running. No tool installed, no global
setting altered, no platform switch.

Environment: `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, `gcc` absent, module `go 1.22.2`.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `go build ./...` | **0** | PASS |
| 2 | `go vet ./...` | **0** | PASS |
| 3 | `gofmt -l $(git ls-files '*.go') catalog_open_test.go` | **0** | PASS, nothing flagged |
| 4 | `go test -count=1 -run 'TestOpenStore_\|TestPersistObserver_\|TestCatalog\|TestWriteCatalog_' -v .` | **0** | PASS — 15 top-level, 6 subtests, 1 skip |
| 5 | `go test ./... -count=1 -v` (full, uncached) | **1** | **204 pass / 2 fail / 4 skip**, 75.3 s |
| 6 | Disconnection experiment, disposable copy (F-3) | **1** | Control fails for the intended reason |
| 7 | A/B stray-write probe, disposable copy (F-4) | **1** | New comparison catches both strays |
| 8 | Complementarity probe, disposable copy (F-4) | **0** | Observer reports zero while files appeared |
| 9 | Junction traversal probe, disposable copy (F-4) | **0** | Link recorded, not followed |
| 10 | POSIX symlink traversal probe (F-4) | **0 (SKIP)** | **Not exercised** — symlink creation needs elevation |
| — | `go test -race` | — | **NOT TESTED**, unchanged. `CGO_ENABLED=0`, no `gcc`. Not a pass |

### Run 4 — targeted, all passing

```
--- PASS: TestPersistObserver_SeesRealMutations
--- PASS: TestPersistObserver_SeesBackupPrune
--- PASS: TestOpenStore_UnreadableCatalogFailsClosed
    --- PASS: .../permission_denied  .../io_error  .../partial_bytes_with_an_error
--- PASS: TestOpenStore_ZeroLengthCatalogIsDamagedNotNew
--- PASS: TestOpenStore_InvalidJSONStillFailsWithDamagedMessage
--- PASS: TestOpenStore_NewerSchemaStillOpensReadOnly
--- PASS: TestOpenStore_FreshDirectoryStillInitializes
--- PASS: TestOpenStore_NotExistShapesAllInitialize
    --- PASS: .../bare_sentinel  .../PathError  .../wrapped
--- PASS: TestOpenStore_ExistingCatalogReopensIntact
--- PASS: TestWriteCatalog_SaveFailurePropagates          [unchanged compatibility]
--- PASS: TestOpenStore_RecoverySaveFailureIsNonFatal     [unchanged compatibility]
--- PASS: TestCatalogCurrentRoundTrip                     [unchanged compatibility]
--- PASS: TestCatalogMigrateV1ToV2                        [unchanged compatibility]
--- PASS: TestCatalogLegacyMigratesAndBacksUp             [unchanged compatibility]
--- PASS: TestCatalogNewerSchemaIsReadOnly                [unchanged compatibility]
--- SKIP: TestCatalogScale                                [OBELISK_SCALE gate]
```

The recovery-save policy test passes unmodified, as required.

### Run 5 — full suite, reported separately as my own execution

The brief notes a full rerun is not required for a comment change. I ran one anyway, and this is my
own result, not a restatement of the previously reported figure:

**204 pass / 2 fail / 4 skip.** It happens to match the previously reported 204/2/4, which is
expected — `store.go` and `catalog_open_test.go` are both unchanged since that run.

**The suite is not green and is not described as green.** The two failures are the OBX-001 Windows
TAB fixtures, same identities and same cause, unrelated to this patch:

```
tar_names_test.go:80:  TestBuildRestore_HostileFilenamesRoundTrip
tar_names_test.go:145: TestBuildFilelist_IsNulDelimited
  cannot stage "tab\there.txt": ... The filename, directory name, or volume label syntax is incorrect.
```

The four skips are `TestCardCheck_UnlistableDirBlocksFormat`, `TestCatalogScale`,
`TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`, on their usual gates.

**204 is not frozen as an acceptance count.** It describes this run; adding or removing a test moves
it. Compare failure identities and causes.

---

## 6. Blockers versus optional suggestions

### Blocker (one)

**B-1. Apply the F-2 comment correction to `store.go:1086-1088`.** The replacement text is already
drafted in the follow-up report's section 1 and needs no further design. This is a comment-only
edit with no executable change; once applied, F-2 closes and this recheck's verdict becomes
`READY_FOR_OWNER_REVIEW` on the same evidence.

### Optional, not blocking

- **O-1.** In `TestPersistObserver_SeesBackupPrune`, `wantPrunes` takes its `created` term from the
  observer under test. It fails safe (section 3, F-3), so this is cosmetic: deriving the create
  count from a disk re-glob instead would make every term in the arithmetic independent.
- **O-2.** The POSIX symlink branch of `snapshotFixture` is unexercised on this machine (elevation
  required). The Windows junction probe covers the same reparse-point branch, so this is a coverage
  note, not a defect. It would be exercised for free by any Linux or macOS CI run.
- **O-3.** The independent review file uses the section sign (U+00A7), which the project's
  pictograph allow-list in `docs/CONTRIBUTING.md` does not permit. Left unfixed on purpose: that
  file is preserved as historical evidence. Worth knowing that the repository check also already
  flags a pre-existing `ui/index.html` hit (U+2715), so neither is introduced by PR-01.

No new design workstream is proposed.

---

## 7. Still open after this recheck

- **B-1, the `store.go` comment.** Documentation defect only.
- **The initialization-identity residual, open under OB-001.** `fs.ErrNotExist` does not establish
  intentional first use, and `os.MkdirAll(dataDir)` at `store.go:1133` precedes the read: opening
  an absent, unmounted-like data directory creates the tree and writes a fresh catalog plus a daily
  backup, with no error. Independently reproduced in the earlier review. **OB-001 must not be
  marked resolved as a whole** — only the read-failure branch is delivered. Storage identity is a
  separate design decision and is deliberately not started here.
- **Windows and race limitations.** All evidence is windows/amd64. `-race` is **NOT TESTED**
  because `CGO_ENABLED=0` and no C compiler is present; that is a gap, not a pass. The two OBX-001
  TAB failures remain, untouched by this patch and correctly not bundled into it. No Linux or macOS
  evidence for this path.
- **OBX-001, OBX-004, OB-002, OB-003** — untouched and open.

---

## 8. Post-review state verification

Re-checked after all analysis and test execution:

| Property | Value | Unchanged |
|---|---|---|
| Branch | `fix/ob-001-catalog-open` | yes |
| `HEAD` | `d97809b2f730e09960632e2943e562531fa095a3` | yes |
| Staged content | none | yes |
| `store.go` SHA-256 | `8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56` | yes |
| `catalog_open_test.go` SHA-256 | `13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3` | yes |
| Working-tree status | ` M store.go`, `?? catalog_open_test.go` | yes |
| `git diff --stat` | `store.go | 83 +++---, 80 insertions(+), 3 deletions(-)` | yes |

No pull, fetch, branch switch, reset, stash, clean, restore, stage, commit, push or merge.
No production source or test file was edited. The implementation report, the independent review,
`BASELINE-2026-09-06.md`, `OB_STATUS.md`, `NEXT_ACTIONS.md`, the frozen handoff and
`docs/development/PR01_REVIEW_ADDENDUM/` were all left exactly as found. No issue status changed.
This file is the only addition.

`READY_FOR_OWNER_REVIEW` is withheld solely for B-1. This verdict is not permission to commit or
merge and is not a production-readiness claim. PR-02 was not started.
