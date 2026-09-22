# Tester checklist

This package is an unsigned developer alpha for **generated tutorial data only**.
QUICKSTART.md has the full commands; this page says what to run, what to report
and what never to use.

## What to run

Run everything from **Command Prompt** in the extracted package directory, with
workspaces beneath `%LOCALAPPDATA%\ObeliskDev`.

1. Unblock the downloaded ZIP (Properties → Unblock) **before** extracting, then
   extract into a new empty directory.
2. `Launch.cmd check`: expect a JSON line with `filesVerified` and exit 0.
3. Single snapshot: `generate`, `inventory … off.json`, `inventory … on.json
   --ignore-ds-store`, then `view` each catalog. Stop each viewer by typing
   `stop` and Enter.
4. Two snapshots: `generate-pair`, three inventories, then `view A B`, `view B A`
   and `view A B-on`. Compare the recorded counts with the expectations in
   QUICKSTART.md (9/1/0/1/1 for OFF/OFF; 7/1/0/3/1 for ALPHA OFF against BETA ON).
5. Reopen: stop, then `view` the same catalogs again without regenerating.
6. Refusals: repeat an existing `inventory` name, give `view` a relative path,
   and start `bin\obelisk.exe` directly with no arguments. Each must refuse. The
   executable prints `Obelisk <version>: this build contains only …` and exits 2.

## What to report

For each run, BUG-REPORT.md lists the fields. Always include:

- the `Obelisk developer alpha <version>` line the launcher prints, and the
  version in the viewer footer (they must match);
- the ZIP SHA-256, Windows build, Node and browser versions;
- the exact command, exit code and `published` value;
- whether the viewer stopped and its URL stopped responding;
- any Windows dialog, warning, Defender detection or permission block, word for
  word, with the time. Do not click past it.

Mismatched versions, a count that differs from QUICKSTART.md, a viewer that does
not stop, or any file changed outside the chosen workspace are all worth
reporting even if nothing crashed.

## What never to use

- **No real data.** Use only the generated tutorial workspaces. Do not point
  `inventory` or `view` at photos, documents, archives, backups, NAS shares,
  removable drives or any catalog you care about.
- **No production or private catalogs**, including ones made by other Obelisk
  builds.
- **No workarounds.** Do not disable Defender or SmartScreen, add exclusions,
  restore quarantined files, change security settings or choose "Run" on a
  security warning.
- **No redistribution.** Do not forward the ZIP, post it or upload it anywhere.

This package does not back up, restore, copy or verify anything. Recorded
agreement between snapshots is catalog evidence only, not proof that files exist
or are safe.
