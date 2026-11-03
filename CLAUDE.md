# Working rules for Claude sessions

These rules apply to every session in this repository.

## Files and drives

- Work only on the C: drive and in temp folders (test temp: `C:\obelisk-test-tmp`).
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

## Flow for each change

1. Open a PR and wait for CI.
2. Have the change reviewed fresh.
3. Fix every High and Medium, and re-check each fix.
4. When CI is green, merge with a merge commit.
5. Post the Lows to issue #6.
6. Update `docs/STATUS.md`.

## Design

- Follow `docs/ROADMAP.md`.
- Keep the restore doctrine: what goes on the media must always be restorable by
  hand with `par2`, `gpg` and `tar`. `docs/CONTRIBUTING.md` lists it with the
  other house rules.
