# PR-01 / OB-001 — response to the independent review

**Status: READY_FOR_FOCUSED_RECHECK**, with one boundary carve-out named in full below.

**This is the author-side response to the review, not independent approval of it.** The same
session produced the independent review and these changes; nothing here should be read as a second
opinion. A focused recheck by someone else is still required.

| | |
|---|---|
| Base SHA | `d97809b2f730e09960632e2943e562531fa095a3` |
| Current HEAD | `d97809b2f730e09960632e2943e562531fa095a3` — unchanged, nothing committed |
| Branch | `fix/ob-001-catalog-open` |
| Review answered | [PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md](PR01-OB-001-INDEPENDENT-REVIEW-2026-09-06.md), findings F-1 through F-5 |
| Scope | Tests and documentation only. **`store.go` is byte-identical to its pre-session hash.** |
| PR-02 | Not started |

---

## 1. The one carve-out, stated first

**F-2 asked for a correction to a comment inside `store.go:1086-1088`. That edit was not made,
because this task explicitly forbids touching `store.go`.** The wrong sentence is still in the
production source:

```go
// writeCatalog, dailyBackup (create and prune) and backupBeforeMigrate; it does NOT
// cover the jobs.json sidecar (saveJobs), which OpenStore only ever reads.
```

`OpenStore` does not only read the sidecar (section 4). The correction has been made everywhere
this task permits — `catalog_open_test.go`, the PR-01 implementation report, `OB_STATUS.md` — but a
reader of `store.go` alone still meets the wrong reason.

**The exact replacement text, for whoever next has a legitimate reason to touch `store.go`:**

```go
// writeCatalog, dailyBackup (create and prune) and backupBeforeMigrate; it does NOT
// cover the jobs.json sidecar. loadJobs DOES write it via saveJobs when it reconciles a
// RUNNING job, but only after the read classification has already succeeded — every
// refusal returns before loadJobs runs — so no jobs write is reachable on a rejected open.
```

This is deliberate compliance with the stated boundary, not an overlooked item. It is the single
thing a focused recheck should confirm is still outstanding.

---

## 2. Checkpoint verification

Verified before editing anything, and again afterwards.

| Property | Before | After | Note |
|---|---|---|---|
| Branch | `fix/ob-001-catalog-open` | `fix/ob-001-catalog-open` | unchanged |
| `HEAD` | `d97809b2…` | `d97809b2…` | unchanged, nothing committed |
| Staged content | none | none | unchanged |
| In-progress git operation | none | none | `MERGE_HEAD`, `REBASE_HEAD`, `rebase-merge`, `rebase-apply`, `CHERRY_PICK_HEAD`, `BISECT_LOG`, `REVERT_HEAD`, `SEQUENCER` all absent |
| Tracked modifications | ` M store.go` | ` M store.go` | unchanged |

The implementation on disk matched the reviewed patch exactly: `store.go` at
`8baa0842…24bc56` and `catalog_open_test.go` at `69972b5d…7d200c`, 509 lines. **No material
difference from the reviewed checkpoint**, so the review's findings applied as written.

No pull, fetch, branch switch, reset, stash, clean, restore, stage, commit, push or merge was
performed. `docs/development/PR01_REVIEW_ADDENDUM/` still holds the wrong bundle and was left
alone; that housekeeping is separate.

### Artifact identity (F-5)

| File | Lines | SHA-256 | git blob |
|---|---|---|---|
| `store.go` before | 3527 | `8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56` | `04f0b5051a83a43aa5d9a8c1611884497e484808` |
| **`store.go` after** | **3527** | **`8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56`** | **`04f0b5051a83a43aa5d9a8c1611884497e484808`** |
| `catalog_open_test.go` before (reviewed artifact) | 509 | `69972b5d7f1e00cd84b31ad2abbccdccc3a23e98fd51f5b840c35a4f207d200c` | `f873ef27ad30bc38fc55f36ebe81234a4e10752a` |
| `catalog_open_test.go` after | 747 | `13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3` | `55fa688d1dd0b37e956cbdb9ec22ab232df3c6b8` |

**`store.go` is byte-identical.** The production fix was not touched.

On the 431 figure: the PR-01 implementation report claimed 431 lines for a file that was 509. That
is corrected in an append-only block at the end of that report, which leaves its original text
intact. **509 is now historical too** — it is the artifact the independent review examined, and it
stays recorded as such. The current figure is 747 and will move again; neither number is a target.

---

## 3. F-1 — zero-length history

**Change:** [catalog_open_test.go](../../../catalog_open_test.go) header note (new block, "What the
zero-length case actually did, corrected") and the doc comment on
`TestOpenStore_ZeroLengthCatalogIsDamagedNotNew`.

The two comments that asserted the disproved claim are gone. What they said, and what replaced it:

| Was | Now |
|---|---|
| "A zero-length catalog.json (the torn-write artifact) took the same path, because the old test was `len(b) > 0`." | "A zero-length catalog.json did NOT take that path at baseline… `existed = len(b) > 0` was a LATENT HAZARD, never a live defect." |
| "…used to satisfy `len(b) > 0 == false` and take the brand-new path, **which skips the schema gate and the pre-migration backup and then overwrites the file**." | "…this test pins a MESSAGE and the point at which the refusal happens — it does not pin a repaired overwrite, because there was none." |

The regression coverage is unchanged: the test still runs, still asserts refusal, still asserts a
usable Store is not returned, still asserts zero attempted writes, and still asserts the fixture is
preserved. Only the claim about history changed.

The comment also now records what the test is genuinely worth, so nobody reads "it only pins a
message" as "it is pointless": rejecting before `json.Unmarshal` names the `.bak-YYYYMMDD` sidecar
to recover from, and it pins `existed = len(b) > 0` shut so a later refactor that moves the parse
cannot reintroduce the latent hazard.

**Evidence:** the independent review executed pristine baseline blob `00d47561` and observed
`err=catalog.json is damaged: unexpected end of JSON input`, the file left at 0 bytes, and no
backup written. That evidence stands unchanged; only the description of it in the test file was
wrong, and now is not. No production behavior changed.

---

## 4. F-2 — jobs-write reachability

**Change:** the header block "The observer's scope, stated without ambiguous arithmetic" in
`catalog_open_test.go`; correction 2 in the PR-01 report; the dated update in `OB_STATUS.md`. **Not
in `store.go` — see section 1.**

### The actual ordering, traced with current anchors

| Step | Anchor | What happens |
|---|---|---|
| 1 | `store.go:1133` | `os.MkdirAll(dataDir, 0o755)` — mutates, before any read |
| 2 | `store.go:1157` | `b, readErr := readFile(s.path)` |
| 3 | `store.go:1164` | refusal returns: zero-length |
| 4 | `store.go:1172` | refusal returns: invalid JSON |
| 5 | `store.go:1183` | refusal returns: unreadable (the OB-001 branch) |
| 6 | `store.go:1283` | `s.loadJobs()` — reached only if none of 3-5 returned |
| 7 | `store.go:1284` | `return s, nil` |

And inside the sidecar path:

| Anchor | What happens |
|---|---|
| `store.go:3370` | `func (s *Store) loadJobs()` |
| `store.go:3400-3401` | `if changed { s.saveJobs() }` — fires when a job left `RUNNING` is reconciled to `INTERRUPTED` |
| `store.go:3350` | `func (s *Store) saveJobs()` |
| `store.go:3362-3363` | `os.WriteFile(tmp…)` then `os.Rename(tmp, s.jobs.path)` |

**So a successful startup after an unclean shutdown does write `jobs.json`, uninstrumented.** The
old justification ("`OpenStore` only ever reads the sidecar") was false. The correct one is
control flow: `loadJobs` at step 6 sits after every refusal at steps 3-5, so a **rejected** open
never reaches it, and no jobs write is reachable on the path the "zero attempted writes" claim
covers. The conclusion survives; the reason is now the true one.

`loadJobs`, `saveJobs` and config loading are untouched. That fail-open family remains OBX-004 and
is out of scope here.

### Call sites versus operation groups (the ambiguous arithmetic)

The review flagged that "four observed paths" and "nine mutation sites" were being compared without
saying they count different things. Restricting to `store.go`, stated explicitly in the test header:

**Six filesystem call sites that mutate state, grouping into four observed operations plus two
unobserved:**

| Operation reported | Call site(s) | notePersist at |
|---|---|---|
| `catalog-write` | `os.OpenFile` of `catalog.json.tmp` (`1409`) **and** `os.Rename` onto `catalog.json` (`1424`) — **two call sites, one operation** | `1372`, first statement of `writeCatalog`, ahead of the `failSave` seam and the read-only gate |
| `backup-create` | `os.WriteFile` (`1478`) | `1477` |
| `backup-prune` | `os.Remove` (`1484`) | `1483` |
| `pre-schema-backup` | `os.WriteFile` (`1346`) | `1345` |
| *(not observed)* | `os.MkdirAll(dataDir)` (`1133`) | — |
| *(not observed)* | `saveJobs`: `os.WriteFile` (`3362`) + `os.Rename` (`3363`) | — |

The earlier "nine sites" figure counted `os.MkdirAll`, the two `catalog-write` sites, the two
`saveJobs` sites, and the three single-site backup operations, plus `os.OpenFile`/`os.Rename`
listed separately — an inconsistent mix. The table above is the one to use: **four observed
operations, six mutating call sites, two unobserved paths.**

---

## 5. F-3 — the backup-prune positive control

**Change:** new test `TestPersistObserver_SeesBackupPrune` in `catalog_open_test.go`, plus a
corrected doc comment on `TestPersistObserver_SeesRealMutations` saying it covers three of four
groups and pointing at the new one.

### How it satisfies each constraint

| Constraint | How |
|---|---|
| Disposable synthetic fixtures | `t.TempDir()` via `seedRealCatalog`, plus 15 synthetic `catalog.json.bak-2020MMDD` files with distinct contents |
| Triggers the **actual production** pruning path | `st.AddCollection(…)` → `save()` → `writeCatalog` → `dailyBackup` → the retention loop at `store.go:1483-1486`. Nothing is called directly |
| Does not call the observer directly | The spy is installed only through `openStore`; the test never invokes `notePersist` or the observer function |
| Demonstrates the observer records the attempt | Asserts at least one `backup-prune` op, and that the count equals the arithmetic below |
| Confirms the pruning **actually occurred** | Every reported file must be gone from disk (`os.Stat` must return `fs.ErrNotExist`); exactly 14 sidecars must remain; the two oldest must be absent and the newest synthetic present |
| No retention-policy, clock or production change | The 14 constant, `time.Now` and `store.go` are all untouched |
| No sleeps, no midnight or mtime dependence | `dailyBackup` sorts its glob by **name** (`store.go:1481`), so far-past 2020 dates fix the outcome. Nothing reads mtime; nothing sleeps |

### The one non-obvious mechanic

Pruning runs only on a `Store`'s **first** write of the day — `dailyBackup` returns early while
`s.lastBak` already equals today (`store.go:1470-1473`). The fixture store has taken that slot, so
the test opens a **second** store over the same directory, which is a fresh `Store` with an empty
`lastBak`, and writes through that. This uses the existing seam and needed no new hook.

### Midnight robustness

The expected prune count is derived from the observed before-state rather than hard-coded:

```go
wantPrunes := len(before) + len(created) - 14
```

If the date rolls over between fixture and mutation, `dailyBackup` creates one extra sidecar; the
observed `backup-create` count feeds straight into the arithmetic and the assertion still holds.

### Result, and the disconnection experiment

```
go test -count=1 -run 'TestPersistObserver_SeesBackupPrune' -v .        exit=0
--- PASS: TestPersistObserver_SeesBackupPrune (0.02s)
```

The review asked for evidence that removing the notification makes the control fail for the
intended reason. In an isolated disposable copy of the package with the single line
`s.notePersist("backup-prune", matches[0])` deleted from `store.go` and nothing else changed:

```
catalog_open_test.go:443: the observer recorded NO backup-prune for a write that must
prune 16 sidecars down to 14; recorded [catalog-write catalog.json]
--- FAIL: TestPersistObserver_SeesBackupPrune (0.03s)          exit=1
```

The failure names the right thing, and the message confirms the production path really ran: 16
sidecars were present and the catalog write was still observed. The owner's working tree was not
reverted or modified for this.

---

## 6. F-4 — fixture-state comparison

**Change:** `catalogState` / `snapshotCatalogState` / `requireUnchanged` / `sortedKeys` /
`equalStrings` replaced by `fixtureEntry` / `fixtureState` / `snapshotFixture` / `requireUnchanged`
/ `sortedFixtureKeys`.

### What the comparison now covers

The old snapshot globbed `catalog.json`, `catalog.json.bak-*` and `catalog.json.pre-schema-*`.
The new one enumerates **every entry in the disposable fixture directory** and records, per entry:

- **name** — slash-separated path relative to the fixture directory, so subdirectories are covered;
- **type** — `file`, `dir`, `symlink`, or `irregular` (device, socket, pipe), so a file swapped for
  a directory or a link is caught rather than showing up as a read error;
- **bytes** — SHA-256 of the exact content for regular files, and the raw link target for symlinks.

Temporary and migration artifacts (`catalog.json.tmp`, `jobs.json`, `jobs.json.tmp`,
`.pre-schema-*`) are inside the claimed scope and are now covered **by construction** rather than
by having guessed the write paths correctly.

### Safety and strictness

- **Snapshot after fixture setup.** Unchanged from before: `seedRealCatalog` persists the fixture,
  including its legitimate daily backup, and only then is the snapshot taken and the spy armed.
- **Explicit failure, never silent omission.** Every enumeration error, `Rel` error, `Readlink`
  error and `ReadFile` error is a `t.Fatalf`. An artifact that cannot be inspected is reported, not
  dropped from the comparison. `snapshotFixture` additionally fails if `catalog.json` is missing,
  so an empty snapshot cannot trivially "match".
- **Stays inside the fixture.** `filepath.WalkDir` does not follow symlinks. A link planted in the
  fixture is recorded by its target string and never walked through, so traversal cannot reach
  production storage.
- Added and removed entries are reported separately and by name, so a diff says which artifact
  appeared rather than only that a count changed.

### The claim boundary is unchanged and restated in the source

The header block now says, in the test file itself:

- observer evidence establishes **attempted** operations, within the four groups;
- fixture-state comparison establishes **preservation** of observed fixture state;
- neither an absent temp file nor matching final bytes alone proves no write was attempted, which
  is why both are asserted and neither alone;
- the guarantee is **not** "no filesystem write anywhere during `OpenStore`" —
  `os.MkdirAll(dataDir)` at `store.go:1133` runs **before** the read and does mutate the
  filesystem, and is outside this bounded claim by design.

### Evidence that the change has teeth

An A/B run in an isolated disposable copy. Two unobserved stray writes were injected into the
refuse branch of `openStore` (`catalog.json.tmp` and `jobs.json`), and both the old and new
comparisons were run against **that same modified source**:

New comparison — catches both:

```
catalog_open_test.go:547: refused startup (permission denied): catalog.json.tmp was CREATED
  (file 5 bytes sha256:e224ddc6b55af8b2) — the refused open wrote something
catalog_open_test.go:547: refused startup (permission denied): jobs.json was CREATED
  (file 2 bytes sha256:44136fa355b3678a) — the refused open wrote something
--- FAIL: TestOpenStore_UnreadableCatalogFailsClosed/permission_denied      exit=1
```

Old glob-based logic, replicated exactly and run against the same source — passes anyway:

```
OLD glob snapshot says unchanged = true
  ...but catalog.json.tmp IS present on disk
  ...but jobs.json IS present on disk
CONFIRMED: the pre-F-4 snapshot passes while stray artifacts exist
--- PASS: TestProbe_OldGlobSnapshotMissesStrays                             exit=0
```

Neither the observer nor the old snapshot would have caught those; the new comparison does. Both
experiments ran in throwaway copies; the working tree was not modified.

`TestOpenStore_ExistingCatalogReopensIntact` was also upgraded from a `catalog.json`-bytes check to
the whole-directory comparison, so "reopening does not rewrite" now means the entire data directory,
not one file.

---

## 7. Tests run

Disposable `t.TempDir()` fixtures and synthetic data only. No production catalog, real keystore,
NAS original, backup destination, tape or optical medium was touched. No tool installed, no Go
baseline change, no platform switch, and **no `GOTMPDIR` override** — the normal test environment
was preserved.

Environment: `go1.26.4 windows/amd64`, `CGO_ENABLED=0`, `gcc` not present, module `go 1.22.2`.

| # | Command | Exit | Result |
|---|---|---|---|
| 1 | `go build ./...` | **0** | PASS |
| 2 | `go vet ./...` | **0** | PASS |
| 3 | `gofmt -l $(git ls-files '*.go') catalog_open_test.go` | **0** | PASS, nothing flagged |
| 4 | `go test -count=1 -run 'TestOpenStore_\|TestPersistObserver_' -v .` | **0** | PASS, all 9 top-level + 6 subtests |
| 5 | `go test ./... -count=1 -v` (full, uncached) | **1** | **204 pass / 2 fail / 4 skip**, 74.3 s |
| 6 | Disconnection experiment, disposable copy: `backup-prune` notification removed | **1** | New control FAILS for the intended reason (section 5) |
| 7 | A/B experiment, disposable copy: stray writes injected on the refuse branch | **1** new / **0** old | New comparison catches, old misses (section 6) |
| — | `go test -race` | — | **NOT TESTED**, unchanged. `CGO_ENABLED=0`, no `gcc`. Not a pass |

### Run 4 — targeted

```
--- PASS: TestPersistObserver_SeesRealMutations
--- PASS: TestPersistObserver_SeesBackupPrune                 [new, F-3]
--- PASS: TestOpenStore_UnreadableCatalogFailsClosed
    --- PASS: .../permission_denied  .../io_error  .../partial_bytes_with_an_error
--- PASS: TestOpenStore_ZeroLengthCatalogIsDamagedNotNew
--- PASS: TestOpenStore_InvalidJSONStillFailsWithDamagedMessage
--- PASS: TestOpenStore_NewerSchemaStillOpensReadOnly
--- PASS: TestOpenStore_FreshDirectoryStillInitializes
--- PASS: TestOpenStore_NotExistShapesAllInitialize
    --- PASS: .../bare_sentinel  .../PathError  .../wrapped
--- PASS: TestOpenStore_ExistingCatalogReopensIntact
--- PASS: TestOpenStore_RecoverySaveFailureIsNonFatal          [pre-existing, unmodified]
```

Compatibility tests that must not move, all passing unmodified: `TestCatalogCurrentRoundTrip`,
`TestCatalogMigrateV1ToV2`, `TestCatalogLegacyMigratesAndBacksUp`, `TestCatalogNewerSchemaIsReadOnly`,
`TestWriteCatalog_SaveFailurePropagates`, `TestOpenStore_RecoverySaveFailureIsNonFatal`.

### Run 5 — full suite, by identity

**The suite is not green and is not described as green.** 204 pass / 2 fail / 4 skip.

The two failures are the same identities and the same cause as baseline and as the previous run —
OBX-001, the Windows-invalid TAB fixture, untouched by this work and deliberately not bundled in:

```
tar_names_test.go:80:  TestBuildRestore_HostileFilenamesRoundTrip
tar_names_test.go:145: TestBuildFilelist_IsNulDelimited
  cannot stage "tab\there.txt": ... The filename, directory name, or volume label syntax is incorrect.
```

The four skips are the same four on the same gates: `TestCardCheck_UnlistableDirBlocksFormat`,
`TestCatalogScale`, `TestVolumeHealth_SystemDisk`, `TestTreeExpansionBudget`.

**On the count.** 203 → 204 is exactly the one new top-level test function,
`TestPersistObserver_SeesBackupPrune`. 204 is a description of this run, not a target: adding or
removing a test moves it, and identities and causes are what to compare.

### The contaminated run, retained as historical evidence

The independent review's first full-suite attempt set `GOTMPDIR` into the scratchpad and reported
**7 failures**. Five were an artifact of that override — `par2.exe` rejects the relocated temp path
("Ignoring out of basepath source file") — and the clean rerun gave 203/2/4. That record stays in
the independent review with its explanation and rerun. **Those five are environment-caused and are
not production regressions.** No `GOTMPDIR` override was used in this session's runs.

### Remaining limitations

- **windows/amd64 only.** No Linux or macOS evidence for this path.
- **Race: NOT TESTED**, unchanged, for the recorded reason. Not a pass.
- No hardware, tape, optical or Docker validation, and none is warranted here.
- The observer still bounds the "no writes attempted" evidence to its four operation groups. The
  whole-directory comparison now backs it independently at the file level, but `os.MkdirAll`
  remains outside the claim by design.
- **The `store.go:1086-1088` comment still carries the wrong reason** (section 1).

---

## 8. Files changed

| File | Change | Allowed by the brief |
|---|---|---|
| `catalog_open_test.go` | 509 → 747 lines: corrected header and test comments (F-1, F-2), whole-directory fixture comparison (F-4), new `backup-prune` control (F-3) | yes — test file |
| `docs/development/reviews/PR01-OB-001-2026-09-06.md` | append-only correction block, section 8; nothing above edited | yes — current PR-01 documentation |
| `docs/development/OB_STATUS.md` | one dated update block under OB-001 | yes — dated correction in a living status document |
| `docs/development/NEXT_ACTIONS.md` | one dated status block | yes — dated correction in a living next-actions document |
| `docs/development/reviews/PR01-OB-001-REVIEW-FOLLOWUP-2026-09-06.md` | this report, new | yes |

**Not touched:** `store.go` (byte-identical), any other production source, `go.mod`/`go.sum`, the
toolchain, unrelated tests including the Windows TAB fixture, UI, the frozen handoff
`docs/OBELISK_IMPLEMENTATION_HANDOFF_2026-09-06/`, `BASELINE-2026-09-06.md`, and the independent
review itself. No issue status was changed. `docs/development/PR01_REVIEW_ADDENDUM/` was left as
found.

---

## 9. Scope

### Fixed by PR-01, unchanged by this follow-up

A catalog read error that is not a positively identified not-found refuses startup: no usable
`Store`, a wrapped error preserving the cause, and no seeding, recovery save, catalog replacement,
daily-backup creation or backup pruning. Partial bytes returned with an error are discarded. A
zero-length catalog is rejected earlier with an actionable message — a **diagnostics** improvement,
since baseline already failed closed. Invalid-JSON, newer-schema read-only, fresh-directory
initialization and recovery-save-non-fatal behavior are all preserved and re-verified.

### Still open

- **The initialization-identity residual, open under OB-001.** `fs.ErrNotExist` does not establish
  intentional first use, and `os.MkdirAll(dataDir)` at `store.go:1133` precedes the read: opening
  an absent, unmounted-like data directory creates the tree and writes a fresh catalog plus a daily
  backup, with no error. Independently reproduced. **OB-001 must not be marked resolved as a
  whole** — only the read-failure branch is delivered. Storage identity is a separate design
  decision and was not started.
- The `store.go` comment in section 1.
- Non-Windows evidence; race testing; OBX-001, OBX-004, OB-002, OB-003 — all untouched and open.

---

## 10. Outcome

**READY_FOR_FOCUSED_RECHECK**, on these terms:

- F-1, F-3, F-4 and F-5 are addressed in full, each with executed evidence rather than assertion.
- F-2 is addressed in every artifact this task permits, with the corrected control-flow trace and
  current anchors. **Its `store.go` comment remains uncorrected because the brief forbids editing
  `store.go`** — the replacement text is supplied in section 1 for a later change.
- This is not independent approval. The same session wrote the review and this response; a recheck
  by someone else is still needed, and this status is not permission to commit, merge, or a
  production-readiness claim.

A recheck should focus on: the `backup-prune` control's use of a second `Store` to reach the
retention loop (section 5); the strictness and traversal safety of `snapshotFixture` (section 6);
and whether the `store.go` comment carve-out should be closed now or folded into the next change
that legitimately touches that file.

No commits, pushes, merges or PR-02 work.

---

## 11. Addendum — F-2 applied in `store.go` (2026-09-06)

The comment carve-out noted in sections 9 and 10 is now closed. A follow-up task explicitly
authorized editing that comment, superseding the earlier prohibition on touching `store.go`.

**What changed.** The `persistObserver` field comment (`store.go:1082-1090`) previously ended:

    // ...it does NOT
    // cover the jobs.json sidecar (saveJobs), which OpenStore only ever reads.

"which OpenStore only ever reads" was inaccurate: `loadJobs` calls `saveJobs` at `store.go:3401`
when it flips a job left `RUNNING` to `INTERRUPTED`. The corrected text states the real reason a
refused startup still writes nothing to the sidecar — the rejected catalog read returns before
`loadJobs` is reached at `store.go:1283` — and records that the `os.MkdirAll(dataDir)` at
`store.go:1133` precedes the read and is likewise outside the observer's scope:

    // ...Covers
    // writeCatalog, dailyBackup (create and prune) and backupBeforeMigrate. Jobs-sidecar
    // writes are outside this observer's scope: loadJobs can call saveJobs during startup
    // reconciliation, but a rejected catalog read returns before loadJobs is reached. The
    // dataDir MkdirAll precedes the read and is also outside this observer's scope.

**Scope of the edit.** Comment text only — one hunk, two lines removed and four added. No
executable production code, tests, interfaces, dependencies or unrelated formatting were touched.
`catalog_open_test.go` is byte-identical to its pre-edit state. `gofmt -l` reports no files needing
formatting. The full suite was not rerun; only the read-only formatting check was executed.

    store.go sha256 before: 8baa084202ede508114245d927815bfcdee601bcf028beecd093fa860324bc56
    store.go sha256 after:  66efd8c9d476ce6caa27ba46048ed8f7bd71691ddc0a12d13f4fbe37b9d6228e
    catalog_open_test.go:   13bc5e2e887313f062b5f4ca9798f113520094c5ec0dae0c5c874c813487c9a3 (unchanged)

**What this does not change.** The independent reviews and the BASELINE document are untouched. The
initialization-identity residual in section 9 stays open: **OB-001 is still not resolved as a
whole.** Sections 1-10 above stand as written; this addendum only records that F-2's `store.go`
half is now applied. The work remains uncommitted — no staging, commits, pushes or PR-02.
