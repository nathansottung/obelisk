# Known issues

## Status

Obelisk is pre-release. A full review in November 2026 found data-safety
problems. Do not trust it with your only copy of anything until a beta is
published.

Sign-in keeps out other accounts on the PC and web pages. It does not keep out
programs running under your own account, or an administrator.

Download links open with a normal click or a middle-click. "Save link as" does
not work.

## Fixed on main

- Card Check no longer says a card is safe to format; it reports the number of verified copies and when they were last checked ([#25](https://github.com/nathansottung/obelisk/pull/25)).
- One Obelisk process now owns a data folder; a second is refused ([#26](https://github.com/nathansottung/obelisk/pull/26)).
- The local web interface now requires a signed-in session on every start ([#29](https://github.com/nathansottung/obelisk/pull/29)).
- Private and encrypted media no longer receive a plain file list or plain tar, and restore notes name only files on the medium ([#31](https://github.com/nathansottung/obelisk/pull/31)). Media already written are not changed. See below.

## Check media written before PR #31

Media written before [#31](https://github.com/nathansottung/obelisk/pull/31) may hold plain file names or a plain tar, even if the package was encrypted or private media was on. A plain tar exposes file contents as well as names. The fix cannot change discs or tapes already written.

These checks only read the medium. Mount a tape through LTFS read-only. Replace `E:\` or `/path/to/medium` with the medium.

On Windows, in PowerShell:

```powershell
Get-ChildItem -LiteralPath E:\ -Recurse -File | Where-Object { $_.Name -match '(\.manifest\.json|^manifest-sha256\.txt|^tagmanifest-sha256\.txt|^bag-info\.txt|^filelist\.txt|\.tar)$' } | Select-Object -ExpandProperty FullName
```

On Linux or macOS:

```bash
find /path/to/medium -type f \( -name '*.manifest.json' -o -name manifest-sha256.txt -o -name tagmanifest-sha256.txt -o -name bag-info.txt -o -name filelist.txt -o -name '*.tar' \) -print
```

Read the results for each package folder:

| Found | Meaning |
|---|---|
| `NAME.tar` beside `NAME.tar.gpg` | **Plain copy of the contents.** The encryption does not protect this medium. |
| `filelist.txt` | Plain file names. |
| `NAME.manifest.json`, `manifest-sha256.txt`, `tagmanifest-sha256.txt` or `bag-info.txt` beside `NAME.manifest.json.gpg` | Plain file names on a private medium. |
| `NAME.manifest.json` with no `.gpg` manifest | Normal for a package that was not private. Its file names are readable. |
| `NAME.tar` with no `NAME.tar.gpg` | Normal for an unencrypted package. |

An older build may also have named an unencrypted payload `NAME.tar.gpg`. If `tar -tf NAME.tar.gpg` lists files, that payload is a plain tar.

If a medium exposes something it should not, keep it physically secure. Rebuild the package with a current build, write and verify a new medium, and only then decide what to do with the old one. Files on write-once media cannot be removed.

Older private media can also have a `RESTORE.txt` that names `NAME.manifest.json` when only `NAME.manifest.json.gpg` is there. Read the list with `gpg -d NAME.manifest.json.gpg`. Older spanned packages can show a garbled last segment name in `RESTORE.txt`. The rejoin commands in that file are still correct.

## Being fixed

- Key handling and key backups
- Interrupted moves, restores, and upgrades
- Restore checks
- Copy-verification credit and what the screens claim
- Recovery instructions
- Release signing
