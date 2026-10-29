# Roadmap

What Obelisk is working towards, grouped by theme. Each item says what it is for, what it waits
on, and how we will know it is done.

- Shipped and in-progress work, item by item: [STATUS.md](STATUS.md).
- Known defects and their state: the defect register,
  [development/OB_STATUS.md](development/OB_STATUS.md).

The themes run roughly in order. Safety comes first, because nothing else is worth building on an
archive that can damage its sources or say "verified" when it isn't.

---

## Safety

### Guard the remaining writers (Slice C)
- **Goal:** every place Obelisk writes is checked against the sources by real location, as
  mirror, plan, restore, quarantine and escrow writes already are. The remaining writers are
  BagIt export, the recovery kit, the app-backup export folder, package write and span
  destinations, dock and finalize sidecars, and the staging folder at build time.
- **Depends on:** the destination guard (Slice A) and the source check (Slice B), both merged.
- **Done when:** no writer uses the spelling-only source check, and each converted writer has a
  test that writes through a folder link, short name or other spelling of a source and is
  refused.

### Truthful "verified"
- **Goal:** "verified" means the bytes on the medium were read back and matched, and the record
  says so. A failed or unrecorded check never shows as verified.
- **Depends on:** Slice C, so every write goes through the same verified publish step.
- **Done when:** every status that says "verified" names the check behind it and when it ran,
  and tests show that a failed flush, a failed read-back or a missing record leaves the item
  unverified.

### Portability
- **Goal:** a package can be restored on another machine, years later, with stock tools.
  - Tar keeps non-ASCII (Unicode) file names exactly, on every platform.
  - The gpg used to encrypt is a pinned, recorded version, and decrypt is tested with other
    OpenPGP tools.
  - A written runbook restores a package by hand, without Obelisk.
- **Depends on:** nothing.
- **Done when:** the Unicode file-name tests pass on Windows, Linux and macOS with no
  known-failure exceptions; decrypt tests pass with at least two other OpenPGP implementations;
  and someone who has not seen the code restores a package by following
  [RESTORE_RUNBOOK.md](RESTORE_RUNBOOK.md) alone.

---

## Scale

### 3 million files and 115 TB
- **Goal:** the catalog, scan, package and verify steps stay usable on an archive of 3 million
  files and 115 TB.
- **Depends on:** the resumable first scan.
- **Done when:** a test archive of that size is cataloged, packaged and verified with memory and
  time recorded, and the UI stays responsive throughout.

### Resumable first scan
- **Goal:** a first scan that stops (crash, reboot, unplugged drive) carries on where it
  stopped instead of starting again.
- **Depends on:** nothing.
- **Done when:** a scan killed at a random point and restarted finishes with the same catalog
  as an uninterrupted scan, without re-hashing finished files.

### Rolling re-checks
- **Goal:** stored copies are re-checked a slice at a time on a schedule, so every copy is
  re-read within a set period without one long job.
- **Depends on:** truthful "verified".
- **Done when:** a schedule covers every copy within its period, each re-check is recorded per
  copy, and a damaged copy is reported at its next turn.

---

## Platform

### Version command
- **Goal:** `obelisk version` prints the version, commit and build date, and the same appears in
  the UI and in every package's metadata.
- **Depends on:** nothing.
- **Done when:** the command, the UI and a new package all report the same version and commit.

### Mac milestone
- **Goal:** macOS is a supported platform, not only a build target.
- **Depends on:** a destination guard for macOS (today macOS restores only into a new folder),
  and the portability tests.
- **Done when:** the full test suite passes on macOS in CI, and backup, verify and restore work
  on a real Mac.

### Docker
- **Goal:** the container image is a tested, documented way to run Obelisk next to a NAS.
- **Depends on:** the version command.
- **Done when:** a release publishes a versioned image, CI tests it, and sources mounted `:ro`
  are shown to stay unchanged.

### TrueNAS SCALE app
- **Goal:** Obelisk runs as a TrueNAS SCALE app with the dataset mounted read-only. It uses
  `zfs diff` to find what changed since the last snapshot, and pool scrub results as a health
  signal.
- **Depends on:** Docker.
- **Done when:** the app installs from a catalog entry, the dataset is mounted read-only, a
  rescan reads only what `zfs diff` reports, and scrub results show on the dashboard.

---

## Coverage

### One coverage map
- **Goal:** one map of the whole archive, with toggles for file type, copy count, drives seen
  on, offsite, and seen before.
- **Depends on:** nothing.
- **Done when:** each toggle changes the map, and the counts match the catalog.

### Fast scan
- **Goal:** a first scan of a large drive takes minutes, not hours. When run as administrator
  on NTFS, it reads the file table directly; otherwise it walks folders in parallel.
- **Depends on:** the resumable first scan.
- **Done when:** both methods produce the same catalog on the same drive, and the timings are
  recorded.

### Adopt any backup
- **Goal:** an existing backup made by any tool (rsync, Explorer copy, others) is recognised by
  hash and counted as a copy. It is only read, and each file is verified.
- **Depends on:** truthful "verified".
- **Done when:** a folder copied with rsync and one copied with Explorer are both adopted, the
  adopted files count as copies, and nothing on the adopted drive changes.

### Encrypted copies count
- **Goal:** an encrypted copy counts towards a file's copies once it has been checked.
- **Depends on:** truthful "verified".
- **Done when:** a file whose only other copy is encrypted shows the right copy count, and that
  copy drops out of the count when its check fails.

### Copy missing
- **Goal:** copy the files a drive lacks onto it.
  - One-way only, and hash-matched.
  - Each copy is verified.
  - Never overwrites or deletes.
  - A name clash is flagged, with an option to copy alongside under a new name.
- **Depends on:** Slice C, and adopt any backup.
- **Done when:** tests show no file is overwritten or deleted, every clash is flagged, and every
  copied file is verified.

### Fill gaps by building packages
- **Goal:** files with too few copies are gathered into new packages, ready to write.
- **Depends on:** the coverage map, and copy missing.
- **Done when:** after the packages are written and verified, every file meets its profile's
  copy count.

---

## Recovery

### A map of the whole NAS on every drive
- **Goal:** every drive carries an encrypted, compressed map of the whole NAS (paths, sizes and
  SHA-256 hashes), refreshed on each rescan.
- **Depends on:** the portability work, so the map can be read with stock tools.
- **Done when:** each drive holds a current map that decrypts and reads with stock tools.

### Drives found → what's missing
- **Goal:** from whatever drives are found, list what is missing, with a count of recovered and
  lost files.
- **Depends on:** the map on every drive.
- **Done when:** with 3 drives of a 5-drive set, Obelisk lists exactly what is missing.

### Orphans sorted
- **Goal:** files found on a drive that are no longer in the NAS are sorted as unique, an older
  version, or a duplicate.
- **Depends on:** the map on every drive.
- **Done when:** a test set with each kind sorts every file correctly.

### Change and loss alerts
- **Goal:** alert when files change unexpectedly, or when a file's copies fall below its
  profile.
- **Depends on:** rolling re-checks.
- **Done when:** each kind of alert is raised in a test and reaches the configured channel.

---

## Insight

- **Reference sets:** mark a set of files as the one to keep and compare others against.
- **Dashboards with CSV/JSON export:** every view can be exported.
- **Media health:** SMART, disc and tape health in one place.
- **3-2-1 check:** says which files meet three copies, on two kinds of media, with one offsite.
- **Duplicate report:** lists duplicate files and how much space they use.
- **Format watch:** flags formats at risk of becoming hard to open.
- **Favorites:** taken from Lightroom ratings, flags and XMP stars.

- **Depends on:** the coverage map.
- **Done when:** each view matches the catalog in tests and exports to CSV and JSON.

---

## Proof

In order. Each step starts only when the one before it is done.

1. **Dogfood on copies:** run Obelisk on copies of real data, never the originals.
2. **Blu-ray:** write-once media, written, verified and restored.
3. **LTO restore drill:** restore from tape onto a different machine.
4. **Originals and announce:** run on the original data, then announce.

- **Depends on:** Safety and Recovery done.
- **Done when:** each step's run is written up with what was restored and checked.

---

## Not planned

- **Two-way sync.** Obelisk copies one way. It never changes or deletes files on a source.

## Parked

Not ruled out, but not being worked on:
- cloud destinations
- TL2000 tape library support
- succession tools (handing an archive on to someone else)
