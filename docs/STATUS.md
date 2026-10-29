# Status

Work done and open, item by item. Phases match [ROADMAP.md](ROADMAP.md). Known defects are
tracked in the defect register, [development/OB_STATUS.md](development/OB_STATUS.md).

| Item | Phase | Status | PR or issue | Updated |
|---|---|---|---|---|
| Correct the source-safety, verification and platform claims in the docs | Safety | Merged | [#2](https://github.com/nathansottung/obelisk/pull/2) | 2026-09-23 |
| API hardening, launcher-only Windows package, CI release guard | Platform | Merged | [#3](https://github.com/nathansottung/obelisk/pull/3) | 2026-09-23 |
| Retry Windows renames blocked by a brief lock | Safety | Merged | [#4](https://github.com/nathansottung/obelisk/pull/4) | 2026-10-28 |
| Slice A: guard mirror and drive-backup destinations by the folder actually opened | Safety | Merged | [#5](https://github.com/nathansottung/obelisk/pull/5) | 2026-10-28 |
| Destination checks follow-ups: stop-message wording, stale inventory after a stop, test precision, small-file speed, and the config-trust Lows | Safety | Open | [#6](https://github.com/nathansottung/obelisk/issues/6) | 2026-10-29 |
| Retry catalog and jobs reads blocked by a brief Windows lock | Safety | Merged | [#7](https://github.com/nathansottung/obelisk/pull/7) | 2026-10-28 |
| Slice B: guard plan, quarantine, restore, repair, escrow and staging writes | Safety | Merged | [#8](https://github.com/nathansottung/obelisk/pull/8) | 2026-10-28 |
| Delete gate: prepare, confirm with a typed word, re-count before running | Safety | Merged | [#9](https://github.com/nathansottung/obelisk/pull/9) | 2026-10-28 |
| Config trust: burn command without a shell, validated tools and keystores, owner-only secrets | Safety | Merged | [#10](https://github.com/nathansottung/obelisk/pull/10) | 2026-10-29 |
| Slice C: guard the remaining writers | Safety | Merged | [#12](https://github.com/nathansottung/obelisk/pull/12) | 2026-10-29 |
| Cross-drive, part 1: record what each drive scan could not read, and keep a history of scans | Coverage | Merged | [#13](https://github.com/nathansottung/obelisk/pull/13) | 2026-10-29 |
| Cross-drive, part 2: compare drives against a reference (reports only), shared file classifier, scan history in app backups | Coverage | In review | [#14](https://github.com/nathansottung/obelisk/pull/14) | 2026-10-29 |
