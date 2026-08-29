# Contributing

Thanks for helping keep a decades-scale archival tool boring and trustworthy.
The bar here is *longevity and clarity*, not cleverness.

## Build & check

Requires **Go 1.22+**. No other toolchain, no code generation, no `make`.

```bash
go build ./...        # compiles the binary (UI is embedded via //go:embed)
go vet ./...          # catches the usual mistakes
gofmt -l .            # must print nothing — run `gofmt -w .` to fix
```

**Pictograph check** (the emoji policy — only ✓ and ✗ are allowed anywhere):

```bash
# Prints any forbidden pictograph in the UI, docs, README, or Go source.
# Must output nothing. This is an ALLOW-LIST, not a block-list: anything that
# is not plain ASCII, permitted typography, a math sign, or an accented letter
# is flagged — so a new emoji from any Unicode block is caught, not just the
# ones we thought of. The only permitted pictographs are ✓ (U+2713, success)
# and ✗ (U+2717, failure). Permitted typography: — – … · × ≥ ≤ ≈ ≠ − ° ′ ″
# superscripts, curly quotes, non-breaking space, and the arrows → ← • ‹ › » «.
python - <<'PY'
import glob
ALLOWED = set('✓✗—–…·×≥≤≈≠−°²³⁷′″©→←•‹›»«“”‘’')
def bad(ch):
    o = ord(ch)
    if o < 128 or ch in ALLOWED or o in (0x00A0,0x2011): return False
    if 0x00C0 <= o <= 0x017F: return False   # accented Latin letters
    return True
files = (['ui/index.html','README.md','RELEASE_CHECKLIST.md']
         + glob.glob('docs/**/*.md', recursive=True)
         + [f for f in glob.glob('*.go') if not f.endswith('_test.go')])
for f in files:
    for n,line in enumerate(open(f,encoding='utf-8'),1):
        hit=[c for c in line if bad(c)]
        if hit: print(f"{f}:{n}: {' '.join('U+%04X'%ord(c) for c in dict.fromkeys(hit))}  {line.strip()[:70]}")
PY
```

Cross-compile the release targets (all pure Go, `CGO_ENABLED=0`):

```bash
GOOS=windows GOARCH=amd64 go build -o obelisk.exe .
GOOS=linux   GOARCH=amd64 go build -o obelisk-linux-amd64 .
GOOS=darwin  GOARCH=arm64 go build -o obelisk-macos-arm64 .
```

**Every PR must build, vet, and gofmt cleanly on all three OSes.** Platform
code is isolated behind build tags (`*_windows.go` / `*_unix.go`), so keep
platform-specific calls there.

## Code style

Match the surrounding code — it is intentional, not accidental:

- **Flat files, one concern each.** New feature areas get their own top-level
  file (see [`ARCHITECTURE.md`](ARCHITECTURE.md)), all `package main`. Don't
  introduce package trees or internal frameworks.
- **Standard library first.** The only third-party dependency is QR-code
  generation. Adding a dependency needs a strong, stated reason in the PR;
  anything that pulls in CGO is a hard no (the whole point is a static binary).
- **The `Store` is the only thing that touches `catalog.json`.** Persistence,
  migrations, and reboot recovery live in `store.go`. Other files call `Store`
  methods; they never marshal the catalog themselves.
- **Never break the restore doctrine.** The on-media artifacts must always be
  restorable by hand with `par2` / `gpg` / `tar`. par2 is computed over the
  payload; no compression layer, ever. If a change touches build/write/restore,
  it must keep `RESTORE.txt` accurate and end-to-end true.
- **Persisted JSON is a contract.** Don't rename existing `json:"..."` tags or
  API routes — add fields/aliases and keep old catalogs loading. Match the
  existing comment density and naming.
- **UI is one vanilla-JS file** (`ui/index.html`), no build step. Keep it that
  way; changes ship by editing the file.
- **Show the artifact.** Every action that produces an artifact — a printable
  label, a Recovery Kit, an exported report, a drift table — MUST end by
  *showing or opening that artifact*, never a bare "job completed" with nothing
  to click. Open it in a new tab (label/report), surface a result panel with its
  path + contents (kit), or capture the background job's `Result` and expose it
  on the Jobs row (`jobArtifact`). A user should never have to go hunting on disk
  for what an action just made. This was v1's most-reported UX failure — a job
  would finish and simply show a percentage, leaving the operator unsure what
  happened or where the output went.
- **Every path input gets the folder picker.** Any field where the operator
  types a filesystem path (scan source, staging, write/restore/kit destination,
  adopt mount, …) must offer the **Browse…** picker (`browseBtn('id')`) next to
  it. Typing a raw path by hand — v1's only option — is error-prone; the picker
  (backed by the read-only `GET /api/browse`) is the default, while the field
  stays editable so a not-yet-existing folder can still be typed.
- **Protection status is ALWAYS colour + shape + text label together — never
  colour alone.** The six statuses (`UNASSIGNED`, `NOT_BACKED_UP`, `PARTIAL`,
  `COMPLETE`, `OVER_COMPLETE`, `OUT_OF_POLICY`) each pair a colour with a small
  CSS-drawn dot (the shape) *and* a text label everywhere they appear (dashboard
  counts, workbench tree dots, drift report, search results). Colour alone fails
  for colour-blind users and in grayscale print — so it is not allowed. The
  "shape" is drawn in CSS, never an OS emoji (see the pictograph policy below).
  Use the `protBadge()` helper; do not hand-roll a coloured dot. The palette is
  fixed: `ok #2E5E4E`, `warn #9A6B1F`, `bad #A03123`, `idle #8A938C`, plus blue
  `#1E3D8F` (over-complete) and purple `#6B2D86` (out-of-policy). **Introduce no
  other status colours anywhere.**
- **Pictographs & emoji: only a checkmark (✓) and a cross (✗) are permitted,
  anywhere in the product** — UI, tooltips, help lines, toasts, job labels, empty
  states, docs, README, and handbook. `✓` means success/verified; `✗` means
  failure. No other emoji or decorative glyph — no folder, printer, writing-hand,
  book, package, or magnet emoji; no coloured squares or circles; no arrows used
  as icons. OS emoji render differently on every platform and cheapen the
  professional register, so they are banned. The permitted pair is written as
  plain text glyphs (or inline SVG); everything else that used to be an emoji
  becomes either colour + text, a CSS-drawn shape, or just words. **Warnings**
  follow the colour + shape + text rule: amber (`--warn`) plus the word
  **"Warning:"** — never a warning-sign emoji. Enforcement: `grep` the UI and
  docs for pictographs and confirm only `✓`/`✗` remain (see the check in
  `## Build & check`). Plain typography — em dash, middle dot, ellipsis, the
  arrows, and math signs — is not a pictograph and is fine.
- **Line endings are normalized by `.gitattributes`.** The repo root carries a
  `.gitattributes` with `* text=auto` plus explicit `eol=lf` for `*.go`, `*.md`,
  and `*.html`, and `eol=crlf` for `*.bat` (so `cmd.exe` parses batch files
  correctly). Don't commit CRLF into source or docs; if a clone shows spurious
  whole-file diffs, run `git add --renormalize .` once.
- **Navigation stays lazy: one level at a time, with a payload cap.** The treemap
  and the Archives folder tree both aggregate a single level server-side (immediate
  children + worst-status/byte rollups) and never build or ship the whole tree. A
  wide level is capped (`folderTreeMaxLimit`, 200 rows) and paged, so the JSON for
  one expansion stays under **50 KB** even at 200k+ files. `TestTreeExpansionBudget`
  asserts that cap plus a **100 ms** server-compute budget on the widest folder, and
  it runs in CI on every push (`OBELISK_PERF=1`). Client **render** time can't be
  measured from Go, so the payload cap is its enforcement proxy: under 50 KB of
  virtualized rows, render is arithmetic (target < 150 ms). If you add a level-at-a-
  time view, reuse `levelAggregate` and keep it inside this budget.

## Writing standard (George Orwell's six rules)

All functional, user-facing prose follows the six rules from George Orwell's
*"Politics and the English Language"* (1946), quoted here in full:

> 1. Never use a metaphor, simile, or other figure of speech which you are used to
>    seeing in print.
> 2. Never use a long word where a short one will do.
> 3. If it is possible to cut a word out, always cut it out.
> 4. Never use the passive where you can use the active.
> 5. Never use a foreign phrase, a scientific word, or a jargon word if you can
>    think of an everyday English equivalent.
> 6. Break any of these rules sooner than say anything outright barbarous.

Two project amendments:

- **(a) Rule 5, clarified.** A *necessary* technical term (hash, parity, mount,
  SMART, par2) is allowed — with a plain gloss at its first use on each screen, per
  the [plain-language standard](#plain-language-standard-all-ui-copy) below.
  *Invented or borrowed* jargon is not: prefer "compare" over "reconcile" and
  "inventory" / "read in" over "ingest" in prose, keeping the technical term only in
  code and in that first-use gloss.
- **(b) Scope.** These rules govern **functional** text: labels, help lines, error
  messages, dialogs, the handbook, and README instructions. **Brand** lines — the
  tagline and the About-page epigraph — sit under rule 6 and may keep a deliberate
  flourish. Developer docs (this file, `ARCHITECTURE.md`) are out of scope.

**Error messages get special attention.** In active voice, each states — in this
order, three sentences maximum — **what happened, what it means, and what to do
next**. *"Read-back check failed for FILE: the copy on the drive does not match the
source. The drive may be failing, or the file changed while copying. Copy it again;
if it keeps failing, check the drive's health."* Not *"read-back verification
failed."*

**The lint.** `scripts/prose-lint` greps the UI and docs for a banned list — the
filler, worn figures, and passive patterns above — and runs in CI as a non-blocking
warning. The list lives in one editable file, `scripts/banned-phrases.txt`, so the
standard grows there, not in the script. Jargon like "reconcile"/"ingest" is swept
by hand rather than linted, because those words are also stable code and API names
that must not change.

### Appendix: twenty teaching examples (before → after)

The first sweep against this standard produced these rewrites. Keep them as worked
examples of what each rule looks like in practice.

| Rule | Before | After |
|---|---|---|
| 4 — active over passive | This folder doesn't exist yet — it **will be created** on first use. | This folder doesn't exist yet — **the app creates it** on first use. |
| 4 — active over passive | …as you inventoried drives, matches **are recorded** as verified copies. | …as you inventoried drives, **Obelisk records the matches** as verified copies. |
| 5 — jargon → everyday | Pick the Archive(s) to **reconcile** against | Pick the Archive(s) to **compare** against |
| 5 — jargon → everyday | scan, build, write, verify, **reconcile**, recovery kit | scan, build, write, verify, **compare**, recovery kit |
| 5 — jargon → everyday | these drives should be **reconciled** against | these drives should be **compared** against |
| 5 — jargon → everyday | **Reconciling** against: … | **Comparing** against: … |
| 5 — jargon → everyday | the tool **reconciles** a hand-emptied `_deleted` gracefully | the tool **handles** a hand-emptied `_deleted` gracefully |
| 5 — jargon → everyday | the next scan **reconciles** gracefully (README) | the next scan **copes** gracefully |
| 5 — jargon → everyday | Dock — **ingest** a stack of legacy drives | Dock — **inventory** a stack of legacy drives |
| 5 — jargon → everyday | Click **Ingest** and the app does the rest | Click **Inventory** and the app does the rest |
| 5 — jargon → everyday | …or **ingest** a specific mounted path / folder | …or **inventory** a specific mounted path / folder |
| 5 — jargon → everyday | **Ingest path** (button) | **Inventory path** |
| 5 — jargon → everyday | **Ingest this drive** (button) | **Inventory this drive** |
| 5 — jargon → everyday | No drives **ingested** yet. | No drives **inventoried** yet. |
| 5 — jargon → everyday | **Ingesting** X — hashing & matching… (toast) | **Inventorying** X — hashing & matching… |
| 5 — jargon → everyday | Step 2: Plug in a drive and **ingest** it (handbook) | Step 2: Plug in a drive and **inventory** it |
| 5 — jargon → everyday | it offers Re-verify… instead of **Ingest** (handbook) | it offers Re-verify… instead of **Inventory** |
| 5 — jargon → everyday | ### Dock — **ingest** a stack of legacy drives (README) | ### Dock — **inventory** a stack of legacy drives |
| 5 — jargon → everyday | job fallback label: `ingest failed` | `inventory failed` |
| errors (what/why/next) | read-back **verification failed** for FILE | **Read-back check failed for FILE: the copy on the drive does not match the source. The drive may be failing, or the file changed while copying. Copy it again; if it keeps failing, check the drive's health.** |

## Plain-language standard (all UI copy)

Every label, setting, button, message, view header, and empty state must read at
about an **8th-grade level**. A careful non-technical person — a photographer,
musician, filmmaker, or family archivist — should understand what a control does
without a manual. This is a hard requirement, audited on every UI change.

The rules:

- **Say what it does, in plain words, from the user's point of view.** Name the
  benefit or the action, not the mechanism. *"File types to not worry about"*, not
  *"Drift informational extensions (muted in reconcile, excluded from alarm
  totals)."*
- **Every setting is a short label plus one plain help sentence beneath it.** The
  label is scannable; the one-liner (rendered with the `help` class) says what it
  does and when you'd change it. No setting ships without its help line — this is
  the specific thing the audit checks.
- **Jargon is allowed only with an inline plain gloss the first time it appears on
  a screen.** Necessary terms (SMART, par2, EXIF, keystore, escrow, finalize/seal)
  get a parenthetical the first time they show up in a view — e.g. *"repair data
  (par2 = the free tool that makes it)"*, *"the drive's self-reported health
  (SMART)"*. After the first gloss on that screen, the bare term is fine.
- **Prefer short words and short sentences.** Cut hedging and cleverness. One idea
  per sentence. Numbers and units belong in the help line, not the label.
- **Every left-nav item carries a one-line plain job description** as a hover
  `title` (and a tap-hold handler on narrow screens). Example: Inventory drives →
  *"Plug in a drive, get a complete record of what's on it."*
- **Don't dumb down the behavior — only the words.** Precision stays; the sentence
  that carries it just gets readable. When a control is genuinely advanced, say so
  plainly (*"Leave blank unless you know you need it"*) rather than hiding it.

When you add or change any user-facing string, re-read it as someone who has never
seen the app. If it needs a mental translation step, rewrite it.

## Schema versioning (the forward-compatibility guarantee)

Every persisted file (`catalog.json`, keystores, package manifests, dock inventory
sidecars) carries a `schema_version`. `store.go` defines `currentSchemaVersion` and
enforces one contract on load:

- **`version == current`** → proceed.
- **`version < current`** → run the ordered, idempotent `schemaMigrations` (one
  function per step), after writing a `catalog.json.pre-schema-vN-<timestamp>`
  backup of the exact old bytes.
- **`version > current`** → **refuse to write** (a newer app created it; silently
  dropping fields we don't understand would be data loss). Read-only viewing is
  allowed; the reason is surfaced in the startup log and `GET /api/health`.

These are **hard rules** — a broken one can silently corrupt a decade-old archive:

1. **Persisted fields are append-only.** Never rename a `json:"..."` tag, never
   repurpose a field, never change what an existing field *means*. Add a new field
   instead. (A rename is a remove + an add; see rule 3.)
2. **Every new field must tolerate being absent.** Old files won't have it, so its
   Go zero value must be a correct, safe default — and that meaning must be
   documented in a comment at the struct field. If zero-value isn't safe, backfill
   it in a migration, not with special-casing scattered across the code.
3. **Removing or re-meaning a field requires a migration and a major schema bump.**
   Append a `schemaMigration{To: N, Fn: …}` (idempotent), bump
   `currentSchemaVersion` to `N`, and never edit an existing migration — old files
   still replay it.
4. **Only `store.go` migrates.** Migrations are ordered, idempotent, and registered
   in one place. Don't scatter ad-hoc "if field missing…" fixups through the code.
5. **Add a fixture test for any schema change.** `schema_test.go` loads a checked-in
   `testdata/catalog_schema*.json`, saves, reloads, and asserts nothing is lost; a
   new schema version gets its own fixture and a migration round-trip.

## Writing the User Handbook (`docs/handbook/`)

The handbook is the **novice-first, task-based** user guide — written for a
careful photographer or family archivist, **not** a developer. It is separate
from these maintainer docs. When you touch a user-facing flow, update the
matching guide. These rules are **enforced**; a handbook PR that breaks them
gets sent back:

- **Plain language, ~8th-grade reading level.** Short sentences. No jargon for
  its own sake. Never assume the reader knows what a *hash*, a *mount*, or a
  *terminal* is — if you use such a word, define it right there.
- **Define every technical term at first use** (in parentheses) **and** in
  `glossary.md`.
- **Every step states its expected result** — "You should now see…". A step the
  reader can't confirm is a step they can't trust.
- **Every guide ends with two sections:** `## How to know it worked` and
  `## If something went wrong`. (The glossary and troubleshooting pages are
  exempt.)
- **Screenshots are referenced as placeholders** `../img/<guide>-<step>.png`
  (handbook pages live in `docs/handbook/`, images in `docs/img/`), and each
  guide ends with a `## Screenshots to capture` checklist naming every
  placeholder. Don't block a guide on missing images — ship the words, list the
  shots.
- **Ground every step in a real, current UI label.** Before writing "click X",
  confirm X exists and is spelled that way in `ui/index.html`. **Where the docs
  and the UI disagree, flag it in the PR description — do not paper over it** by
  quietly describing what "should" be there. A `<!-- VERIFY: … -->` comment in
  the draft is fine for handoff, but it must be resolved before merge.
- **Reinforce the promise** where relevant: the app never touches your
  originals, never deletes anything, never phones home.

## Smoke test (manual, end-to-end)

There is no CI harness yet; the trustworthy check is to **drive a real archive
through the pipeline against a scratch data dir** and confirm the bytes come
back. This is the flow to run before opening a PR that touches the pipeline:

```bash
# 0. build and start against a throwaway data dir + port
go build -o /tmp/mn . && /tmp/mn -port 7799 -data /tmp/mn-data &

# 1. point staging + tools at real binaries (Preflight in the UI shows paths)
curl -s -X PUT http://127.0.0.1:7799/api/config \
  -d '{"staging_dir":"/tmp/mn-stage","tools":{"tar":"tar","par2":"par2"}}'

# 2. create an archive, scan a small folder into it
curl -s -X POST http://127.0.0.1:7799/api/archives -d '{"name":"Smoke"}'
curl -s -X POST http://127.0.0.1:7799/api/collections/1/scan -d '{"path":"/some/small/folder"}'

# 3. plan (plaintext to skip keystores), build package 1, write to a folder
curl -s -X POST http://127.0.0.1:7799/api/plan \
  -d '{"collection_id":1,"media_kind":"HDD","target_gb":1,"encrypted":false}'
curl -s -X POST http://127.0.0.1:7799/api/packages/1/build
curl -s -X POST http://127.0.0.1:7799/api/chunks/1/write -d '{"dest_dir":"/tmp/mn-medium"}'

# 4. restore it to a scratch dir and diff against the source
curl -s -X POST http://127.0.0.1:7799/api/chunks/1/restore \
  -d '{"source_dir":"/tmp/mn-medium/Smoke-C0001","output_dir":"/tmp/mn-restore"}'
diff -r /some/small/folder /tmp/mn-restore/...   # must be identical
```

Watch `GET /api/jobs` for each async step to reach `COMPLETED`. The pass
criterion is simple and non-negotiable: **the restored files are byte-identical
to the source.** For encryption/spanning/privacy changes, also exercise those
paths (register two keystores; use a small `target_gb` to force ≥3 segments;
toggle `private_media` and confirm no plaintext `manifest.json` on the medium).

You can also verify the doctrine holds *without Obelisk* — that's the real
test: `par2 verify` → `gpg -d` → `tar -xf` by hand on the written folder, per
[`RESTORE_RUNBOOK.md`](RESTORE_RUNBOOK.md).

## Cutting a release

Releases are fully automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml):
pushing a `v*` tag cross-compiles all targets (pure Go, `CGO_ENABLED=0`), zips
each with the docs, writes `SHA-256SUMS.txt`, and publishes a GitHub Release.
The version is baked into the binary via `-ldflags "-X main.appVersion=<tag>"`.

```bash
# from a clean main that builds + vets + tests green:
git tag v0.9.0
git push --tags
```

That's the whole process — watch the **Release** workflow in the Actions tab;
the binaries and checksums appear on the Releases page when it finishes. (Every
push/PR also runs the **CI** workflow: vet + build + `go test ./...`.)

## PR expectations

- **One focused change per PR**, with a description of *why*, not just *what*.
- **Builds + vet + gofmt clean** on windows/linux/darwin.
- **Ran the smoke test** (or the relevant subset) — say so, and paste the
  restore-matches-source result.
- **No behavior change hidden in a "docs" or "refactor" PR.** If you rename a
  user-visible string, keep JSON tags and routes stable.
- **Update the docs you touched:** `README.md`, this file, `ARCHITECTURE.md`,
  and — if the on-media layout changes — `RESTORE_RUNBOOK.md`, which is the
  authoritative 30-year document and must stay hand-followable.
