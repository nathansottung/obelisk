# Known issues

## Status

Obelisk is pre-release. A full review in November 2026 found data-safety
problems. Do not trust it with your only copy of anything until a beta is
published.

## Fixed on main

- Card Check no longer says a card is safe to format; it reports the number of verified copies and when they were last checked ([#25](https://github.com/nathansottung/obelisk/pull/25)).
- One Obelisk process now owns a data folder; a second is refused ([#26](https://github.com/nathansottung/obelisk/pull/26)).
- The local web interface now requires a signed-in session on every start ([#29](https://github.com/nathansottung/obelisk/pull/29)).

## Being fixed

- Key handling and key backups
- Interrupted moves, restores, and upgrades
- Restore checks
- Copy-verification credit and what the screens claim
- Recovery instructions
- Release signing
