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
- Obelisk no longer removes or overwrites anything you own unless you type **Delete** or **Overwrite**. It removes on its own only files it made, inside its own folders ([#40](https://github.com/nathansottung/obelisk/pull/40)). See below for what changes in use.

## Removal and overwrite since PR #40

- **Leftovers are listed, not swept.** Files an earlier or interrupted run left in Obelisk's folders now stay until you remove them. Settings lists each under **Kept automatically, and left behind**, with its size and age.
- **Kept mirror versions use space.** When a mirror would replace a different file, it first keeps the old one under `_obelisk-previous/<time of the run>/` on that drive. These are never removed on their own and never count as copies. Remove them with **Remove kept versions…**.
- **Writing over an existing copy asks first.** If the package's folder on the destination already holds something, the write stops and says what it is, then offers **Verify it** or **Overwrite…**.
- **Exports get new names.** Each Recovery Kit, escrow bundle, BagIt export and app backup goes in a new folder named by when it was made, with an `EXPORT-INFO.txt`. Older ones are labelled superseded on the export screen and are never changed.
- **Escrow tool caches from before the upgrade.** A cached tool file whose hash no longer matches its pin cannot be refreshed, because Obelisk has no record of downloading it. Remove the file yourself to let it download again.
- **A leftover can be offered while a job is using it.** The list has no age floor yet, so a build or stage that is running now may appear. Do not remove anything from the list while a job runs.

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
| `catalog_snapshot.json` or `INVENTORY.md` in `OBELISK_SEAL`, `OBELISK_DOCK`, or an older `MNEMOSYNE_SEAL` or `MNEMOSYNE_DOCK` folder | Open the file. If a private package lists file names, they are exposed. A package showing `encrypted_file_list`, or "File names are encrypted", is clean. |
| `NAME.manifest.json`, `manifest-sha256.txt`, `tagmanifest-sha256.txt` or `bag-info.txt` beside `NAME.manifest.json.gpg` | Plain file names on a private medium. |
| `NAME.manifest.json` or the BagIt files with no `.gpg` manifest | Normal for a package that was not private. Its file names are readable. |
| `NAME.tar` with no `NAME.tar.gpg` | Normal for an unencrypted package. |

An older build may also have named an unencrypted payload `NAME.tar.gpg`. If `tar -tf NAME.tar.gpg` lists files, that payload is a plain tar.

If a medium exposes something it should not, keep it physically secure. It may be your only copy, so do not erase or destroy it yet. Files on write-once media cannot be removed, and Obelisk cannot rebuild a package that is already written. Make a clean copy by hand instead:

1. Confirm the payload is really encrypted. Run `tar -tf NAME.tar.gpg`, or `tar -tf NAME.seg001` for a spanned package. **If it lists any file, stop.** That payload is plain, so a copy of it would be too. It needs a new encrypted package made from the original files.
2. Copy only these files to a new medium: `NAME.tar.gpg` (or every tape's `NAME.segNNN`), every `NAME.tar.gpg*.par2` file (on a spanned set, these are on the last tape), and `RESTORE.txt`. A spanned package may need a new set of media. Put `RESTORE.txt` and the manifest on each one.
3. Copy the manifest. For a private package, copy `NAME.manifest.json.gpg`. For a package that was not private, `NAME.manifest.json` shows file names but holds the per-file hashes. Keep it unless the names must be hidden. Without it, the list is still inside the encrypted tar.
4. On the new medium, run `par2 verify NAME.tar.gpg.par2`. For a spanned package, first rejoin the segments in a temporary folder, as `RESTORE.txt` shows. On Windows, run its `copy /b` line in `cmd.exe`, not PowerShell.
5. Test the passphrase and the contents in `cmd.exe` or a Unix shell, without writing the plain tar to disk: `gpg -d NAME.tar.gpg | tar -tf -`
6. Label the new medium. Obelisk's catalog does not record this copy on its own.

Decide what to do with the old medium only after the new one verifies.

Older private media can also have a `RESTORE.txt` that names `NAME.manifest.json` when only `NAME.manifest.json.gpg` is there. Read the list with `gpg -d NAME.manifest.json.gpg`. Older spanned packages can show a garbled last segment name in `RESTORE.txt`. The rejoin commands in that file are still correct.

## Rebuilding a package that already has copies

Each copy now records its own key and hashes. A rebuild gives the package new ones and changes no existing copy, so older copies keep verifying and restoring in the app, and Obelisk no longer warns before a rebuild.

When a catalog is upgraded, each existing copy inherits its package's current key and hash. **One limit applies to a copy that an earlier version retired after an accepted rebuild.** It stays listed as superseded, and the key and hash it inherits may not be the ones it was written with. Restore such a copy by hand with `par2`, `gpg` and `tar`, using the key named in the `RESTORE.txt` on its medium. The old key is still in the keystore.

If staging is damaged or gone, **Refill staging** on a verified copy brings the package back from that copy after checking it against the copy's own hash. A copy that fails a check but can be repaired by its `par2` set shows as repairable.

## Copy ID file on new media

Each new write now puts a small `COPY-ID.txt` beside the package. It holds a format version and a random ID, and nothing else. It is not needed for a restore by hand. Obelisk uses it to tell copies apart, so a moved drive, a second copy of the same build and a retired copy are no longer confused.

**After the upgrade, every copy written before this change shows "not identified, verify to confirm" and no longer counts as protecting its package.** Its last-verified date is kept. Run a full check (Verify) of each medium to give its copies their credit back. Obelisk never writes an ID onto an existing medium. A package built before this change keeps its older `RESTORE.txt`, without the line about the file, until it is rebuilt.

**A spanned set written before this change cannot get its credit back by a check**, because Obelisk has no check for a whole spanned set. It stays "not identified" until the set is written again. It still restores, in the app and by hand. A check for a whole spanned set is tracked in [#38](https://github.com/nathansottung/obelisk/issues/38) and is required before tape is a qualified target.

If two media hold the same ID, for example after a drive was cloned, neither counts until you choose which one is the copy. Write a new copy to the other medium to give it its own ID.

## Being fixed

- Key handling and key backups
- Interrupted moves, restores, and upgrades
- Restore checks
- Copy-verification credit and what the screens claim
- Recovery instructions
- Release signing
