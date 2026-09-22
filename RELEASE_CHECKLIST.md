# Release checklist — v0.9.0

Prepared 2026-07-07. This confirms the three release-prep workstreams. Tick the two
push-time items (CI, tag) after pushing; everything else is verified in-repo.

## 1. Version audit — one source of truth

- [x] **Single source: `appVersion`** ([main.go](main.go)) is the only version
  string; the in-repo default is **`0.9.0-dev`** (marks any non-release build).
- [x] **Injected at build time from the git tag** via
  `-ldflags "-X main.appVersion=<tag>"` — in
  [`.github/workflows/release.yml`](.github/workflows/release.yml) (`${TAG}`) and the
  [`Dockerfile`](Dockerfile) (`VERSION` arg). The container image also derives tags
  and OCI labels from the tag via `docker/metadata-action`.
- [x] **Everything else derives from `appVersion`:** startup banner, `GET /api/health`,
  About/escrow status (`GET /api/escrow`), BagIt `Bag-Software-Agent`, package
  manifests (`obelisk_version`), dock inventory sidecars, and the Recovery Kit all
  read `appVersion` — no independent version literal anywhere.
- [x] **No hardcoded `2.0`/`2.1`/`v1`/`v2` app-version strings remain.** Fixed:
  `main.go` default + comment, `escrow.go` comment, `README.md` (Docker pin →
  Releases page), `release.yml` + `CONTRIBUTING.md` tag examples (`v0.9.0`), handbook
  install banner (`Obelisk <version>`).
- [x] **README stops claiming version numbers in prose** — it points to
  **the latest release** (badges + Releases page + "pin a release tag from the
  Releases page").
- [x] **`grep -E '2\.0\.0|2\.1\.0|v1\.|v2\.'` audited.** Remaining matches are **not**
  app versions and are intentionally left:
  - `escrow_manifest.json` — third-party **license** ids (`GPL-2.0`, `LGPL-2.1`,
    `CDDL-1.0`) and dependency URLs (`openjpeg v2.5.2`).
  - `RESTORE_RUNBOOK.md`, `recoverykit.go`, `README.md` — **Parchive/PAR2 2.0** (the
    par2 file-format spec, not our version).
  - `integration_test.go`, `versions.go`, `README.md` — **"restore v1 / version 2"**
    refer to per-file *content versions*, not the app.
  - `.vscode/launch.json` — VSCode launch-file **schema** `0.2.0`.
  - `auth_test.go` (`0.0.0.0:7821`), `space.go` (`mult = 2.0`) — not versions.

## 2. Schema versioning — the forward-compatibility guarantee

- [x] **`schema_version` on every persisted file.** `catalog.json` root (first field),
  keystores (`schema_version` beside `obelisk_keystore`), package manifests
  (`schema_version` + `obelisk_version`), and dock inventory sidecars. Current
  version: **`schema_version: 1`** (`currentSchemaVersion` in [store.go](store.go)).
- [x] **Load contract enforced** ([store.go](store.go) `OpenStore` / `writeCatalog`):
  - `== current` → proceed;
  - `< current` → **back up exact bytes** (`catalog.json.pre-schema-vN-<ts>`) then run
    the ordered, idempotent **`schemaMigrations`** registry (one func per step);
  - `> current` → **refuse to write** with a clear message ("created by a newer
    version… Upgrade the app… refusing to save so newer fields aren't silently
    dropped"); read-only viewing allowed, surfaced in the startup log and
    `GET /api/health` (`read_only` / `read_only_reason`).
- [x] **CONTRIBUTING hard rules** ([docs/CONTRIBUTING.md](docs/CONTRIBUTING.md),
  "Schema versioning"): persisted fields are **append-only** (never rename, repurpose,
  or re-mean); **removal requires a migration + major bump**; **every new field must
  tolerate being absent** (zero-value semantics documented at the struct); only
  `store.go` migrates; every schema change adds a fixture test.
- [x] **Round-trip fixture test** ([schema_test.go](schema_test.go) +
  [testdata/catalog_schema1.json](testdata/catalog_schema1.json)): loads a checked-in
  schema-1 catalog, saves, reloads, and asserts nothing is lost (byte-identical
  persistence + deep spot-checks). Plus a **legacy → v1 migration + backup** test and a
  **newer-schema read-only refusal** test.

## 3. CI, links, and tests

- [x] **`go build ./...`** — clean.
- [x] **`go vet ./...`** — clean.
- [x] **`go test ./...`** — all packages pass locally (with `gpg`/`par2` present, same
  as CI installs).
- [x] **`gofmt -l`** — clean.
- [x] **README links point to `nathansottung/obelisk`** — all 8 repo references (CI
  badge, Release badge, Releases page, GHCR image ×2, etc.) updated; the only
  `github.com/*/obelisk` link is `nathansottung/obelisk`. Third-party links
  (LTFS, par2cmdline-turbo, stenc) are unrelated and correct.
- [x] **Release-download repo fixed** — `escrowRepo` ([escrow.go](escrow.go)) now
  `nathansottung/obelisk`, matching where `release.yml` publishes, so Escrow-Bundle
  binary fetches resolve instead of 404ing.
- [x] **No `microsoft`/wrong-org references** remain in code, docs, or workflows.
- [x] **CI workflow is valid** ([.github/workflows/ci.yml](.github/workflows/ci.yml)):
  standard `actions/checkout@v4` + `actions/setup-go@v5`, installs `gnupg`/`par2`, runs
  `go vet` → `go build` → `go test ./... -v`. The local equivalent is green above.
- [ ] **CI is green on the pushed branch** — confirm the **CI** workflow run passes on
  GitHub after pushing (cannot be triggered from here).
- [ ] **Tag the release** — from a clean `main`: `git tag v0.9.0 && git push --tags`
  (drives `release.yml`: cross-compiled binaries, checksums, GitHub Release, GHCR image).

- [x] **Go module path matches the repo** — `go.mod` is now
  `github.com/nathansottung/obelisk` (no internal imports referenced it, so the
  rename is inert to the build; verified with `go build`/`go vet`/`go test`).


## 2026-09-20 - local inventory developer-alpha packaging checkpoint

This scoped entry supersedes no historical release/schema/test claims above. Native inventory preview remains schema 8; this is an unsigned generated-source PACKAGING CANDIDATE, not a v0.9.0 release or backup/archive product release candidate. Branch feat/windows-inventory-alpha-package, HEAD bfbce891df78d529c6be2d2912dc8443597c007e, uncommitted/unstaged.

- [x] One local Windows amd64 package built from accepted committed runtime; external Node 24 x64/browser prerequisites declared.
- [x] Exact 20-entry ZIP/manifests, source versus uncommitted script identities, linked dependency notices and exclusions recorded.
- [x] Two same-workstation extractions/relocations, seven corrected package tests plus one separate missing-input test, five completed browser sessions, split stop/reopen and two ConsoleHost paths recorded with retained setup failures.
- [ ] Substantive package review and owner acceptance.
- [ ] Clean Windows VM/second-machine and broader terminal testing.
- [ ] Original security-alert classification; public signing/reputation/SmartScreen, distribution/privacy/license review and separate publication authorization.
- [ ] Production/scale/other-platform/media/backup/recovery qualification; not implied by packaging.

See [the package implementation report](docs/development/reviews/WINDOWS-INVENTORY-ALPHA-PACKAGE-IMPLEMENTATION-2026-09-20.md). No tags, CI/release workflow, uploads, installs or security controls were changed. ONE next action: substantive package review.


## 2026-09-21 - accepted local package; source checkpoint only

- [x] Owner accepted the bounded local unsigned Windows package after the substantive review.
- [x] Reviewed packaging source committed as 082ae8375130bdb9943d31d7432c87a3c53fbbb8; original ZIP unchanged (SHA-256 f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de).
- [x] Historical author/reviewer validation preserved separately; no new build/runtime tests for this checkpoint.
- [ ] Live source publication: final evidence SHA/result recorded in the external receipt after verification.
- [ ] Clean/second-machine and downloaded-file policy qualification; broader console/runtime/browser/platform coverage.
- [ ] Original-alert classification and separate public signing/reputation/distribution authorization. No binary upload/release/tag is authorized here.

See [owner acceptance and frozen artifact identities](docs/development/reviews/WINDOWS-INVENTORY-ALPHA-PACKAGE-ACCEPTANCE-2026-09-21.md). ONE proposed next action after source publication: separately authorize rehearsal of this exact ZIP on a clean/second Windows machine.


## New local comparison package candidate — 2026-09-21

- [x] Fresh accepted-runtime build; new distinct 22-entry unsigned ZIP.
- [x] Two extracted relocations, native generated ALPHA/BETA OFF/ON, browser comparison, pipe split-stop and real console typed stop checked.
- [x] Accepted runtime and original frozen inventory ZIP preserved.
- [ ] Substantive package-delta review and separate owner acceptance.
- [ ] Second-machine/downloaded-file qualification: Prompt20 remains DEFERRED_BY_OWNER.
- [ ] Public signing/distribution and unresolved prior alert classification remain separate gates.

The new candidate is uncommitted on feat/windows-comparison-alpha-package at e3d9bef998a20dff78dc67463dfb8f848aad76ce. See [new implementation report](docs/development/reviews/WINDOWS-COMPARISON-ALPHA-PACKAGE-IMPLEMENTATION-2026-09-21.md) for exact identities, scoped results and retained harness failures. Earlier package acceptance does not cover this archive.


## Comparison package owner acceptance — 2026-09-21

- [x] Bounded substantive review and separately submitted owner acceptance.
- [x] Reviewed implementation committed as be6f60f178c793835cdbad82c9f774f61fd4071d; original comparison ZIP unchanged at 33d1d9e5087c35aca23606957777f3d13ff8267de798c9848c79ada4697f00d2.
- [x] Older frozen inventory ZIP preserved separately; no rebuild or runtime test for this checkpoint.
- [ ] Source publication: actual evidence SHA and live verification recorded in the external receipt, without self-referential commit text.
- [ ] Second/clean-machine and downloaded-file qualification; Prompt20 remains DEFERRED_BY_OWNER.
- [ ] Historical alert classification, broader runtime/platform/console scope and separately authorized public signing/distribution.

See [comparison package acceptance](docs/development/reviews/WINDOWS-COMPARISON-ALPHA-PACKAGE-ACCEPTANCE-2026-09-21.md). Package-input docs and historical reports retain their reviewed bytes; this later non-packaged entry records acceptance.
