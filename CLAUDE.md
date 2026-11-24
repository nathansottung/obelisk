# Working rules for Claude sessions

These rules apply to every session in this repository.

## Files and drives

- Work only on the C: drive and in temp folders (test temp: `%USERPROFILE%\obelisk-test-tmp`;
  set `TMP` and `TEMP` to it for test runs). The write guard refuses folders that other
  users can change, which a folder made at the drive root is.
  Never touch a NAS, network share, mapped drive or external drive unless the
  owner says so.
- Never delete user data.
- Never `git checkout`, `git restore`, `git reset` or `git stash` files that have
  uncommitted changes. Commit work in progress first.

## Commits and PRs

- Write commit and PR messages in generic terms. Leave out private review IDs.

## Tests

- Write tests that fail first, then make them pass.
- Run the full suite on both catalog engines (the default, the catalog database,
  and `OBELISK_CATALOG_ENGINE=json`), plus the known-failure gate:
  `go test -json ./... > out.jsonl` then `node scripts/ci/go-known-failures.mjs out.jsonl`,
  run from the repo folder.
- Frozen fixtures of old catalog and app-backup formats (`testdata/frozen`) are never
  edited or rebuilt. A schema change adds a new frozen fixture made by the old code.
  Tests must not fake an old format by stripping fields from a new one.

## Flow for each change

1. Open a PR and wait for CI.
2. Have the change reviewed fresh.
3. Fix every High and Medium, and re-check each fix.
4. When CI is green, merge with a merge commit.
5. Post the Lows to issue #6.
6. Update `docs/STATUS.md`.
7. Update `docs/KNOWN_ISSUES.md` when a fix merges.

## Design

- Follow `docs/ROADMAP.md`.
- Keep the restore doctrine: what goes on the media must always be restorable by
  hand with `par2`, `gpg` and `tar`. `docs/CONTRIBUTING.md` lists it with the
  other house rules.
