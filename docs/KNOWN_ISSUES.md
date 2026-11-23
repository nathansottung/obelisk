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
- Encrypted media no longer receive `filelist.txt` or a plain tar. Private media also keep every plain file list off the medium, and restore notes name only files on it ([#31](https://github.com/nathansottung/obelisk/pull/31)). Encrypted media with private media off still show file names in their manifest. Media already written are not changed. See below.

## Check media written before PR #31

Media written before [#31](https://github.com/nathansottung/obelisk/pull/31) may hold plain file names or a plain tar, even if the package was encrypted or private media was on. A plain tar exposes file contents as well as names. The fix cannot change discs or tapes already written.

These checks only read the medium. Mount a tape through LTFS read-only. Replace `E:\` or `/path/to/medium` with the medium.

On Windows, in PowerShell:

```powershell
Get-ChildItem -LiteralPath E:\ -Recurse -File -Force | Where-Object { $_.Name -match '(\.manifest\.json|\.manifest\.hidden|^catalog_snapshot\.json|^INVENTORY\.md|^manifest-sha256\.txt|^tagmanifest-sha256\.txt|^bag-info\.txt|^filelist\.txt|\.tar)$' } | Select-Object -ExpandProperty FullName
```

On Linux or macOS:

```bash
find /path/to/medium -type f \( -name '*.manifest.json' -o -name '*.manifest.hidden' -o -name catalog_snapshot.json -o -name INVENTORY.md -o -name manifest-sha256.txt -o -name tagmanifest-sha256.txt -o -name bag-info.txt -o -name filelist.txt -o -name '*.tar' \) -print
```

Read the results for each package folder:

| Found | Meaning |
|---|---|
| `NAME.tar` beside `NAME.tar.gpg` | **Plain copy of the contents.** The encryption does not protect this medium. |
| `filelist.txt` | Plain file names. |
| `.NAME.manifest.hidden` | Plain file names, left by an older burn. |
| `catalog_snapshot.json` or `INVENTORY.md` in `OBELISK_SEAL`, `OBELISK_DOCK`, or an older `MNEMOSYNE_SEAL` or `MNEMOSYNE_DOCK` folder | Plain file names for every package on that medium, including private ones. |
| `NAME.manifest.json`, `manifest-sha256.txt`, `tagmanifest-sha256.txt` or `bag-info.txt` beside `NAME.manifest.json.gpg` | Plain file names on a private medium. |
| `NAME.manifest.json` or the BagIt files with no `.gpg` manifest | Normal for a package that was not private. Its file names are readable. |
| `NAME.tar` with no `NAME.tar.gpg` | Normal for an unencrypted package. |

An older build may also have named an unencrypted payload `NAME.tar.gpg`. If `tar -tf NAME.tar.gpg` lists files, that payload is a plain tar.

If a medium exposes something it should not, keep it physically secure. It may be your only copy, so do not erase or destroy it yet. Files on write-once media cannot be removed, and Obelisk cannot rebuild a package that is already written. Make a clean copy by hand instead:

1. Confirm the payload is really encrypted. Run `tar -tf NAME.tar.gpg`, or `tar -tf NAME.seg001` for a spanned package. **If it lists any file, stop.** That payload is plain, so a copy of it would be too. It needs a new encrypted package made from the original files.
2. Copy only these files to a new medium: `NAME.tar.gpg` (or every tape's `NAME.segNNN`), every `NAME.tar.gpg*.par2` file (on a spanned set, these are on the last tape), and `RESTORE.txt`.
3. Copy the manifest. For a private package, copy `NAME.manifest.json.gpg`. For a package that was not private, `NAME.manifest.json` shows file names but holds the per-file hashes. Keep it unless the names must be hidden. Without it, the list is still inside the encrypted tar.
4. On the new medium, run `par2 verify NAME.tar.gpg.par2`. For a spanned package, first rejoin the segments in a temporary folder, as `RESTORE.txt` shows. On Windows, run its `copy /b` line in `cmd.exe`, not PowerShell.
5. Test the passphrase and the contents in `cmd.exe` or a Unix shell, without writing the plain tar to disk: `gpg -d NAME.tar.gpg | tar -tf -`
6. Label the new medium. Obelisk's catalog does not record this copy on its own.

Decide what to do with the old medium only after the new one verifies.

Older private media can also have a `RESTORE.txt` that names `NAME.manifest.json` when only `NAME.manifest.json.gpg` is there. Read the list with `gpg -d NAME.manifest.json.gpg`. Older spanned packages can show a garbled last segment name in `RESTORE.txt`. The rejoin commands in that file are still correct.

## Being fixed

- Key handling and key backups
- Interrupted moves, restores, and upgrades
- Restore checks
- Copy-verification credit and what the screens claim
- Recovery instructions
- Release signing
