# Manual check: a drive mounted in a folder

This check confirms that Obelisk reads a drive's disk and partition from the volume
that holds the path, on real hardware. Automated tests cannot mount a second disk.

The check reads only. It writes nothing to the drive. Use a spare USB drive anyway.

## What you need

- A Windows PC with Go and this repository.
- A USB drive with an NTFS, exFAT, or FAT32 partition. It must not be your system disk.
- An empty folder on an NTFS drive for the mount, such as
  `C:\Users\<you>\obelisk-test-tmp\usb-mount`.

## Steps

1. Plug in the USB drive and open **Disk Management** (`diskmgmt.msc`).
2. Right-click the USB partition and choose **Change Drive Letter and Paths**.
3. Choose **Add**, then **Mount in the following empty NTFS folder**, and pick the
   empty folder.
4. In PowerShell, from the repository folder, run:

   ```powershell
   $env:OBELISK_MANUAL_MOUNT = "C:\Users\<you>\obelisk-test-tmp\usb-mount"
   go test -count=1 -v -run '^TestManual_FolderMountedDrive$' .
   Remove-Item Env:OBELISK_MANUAL_MOUNT
   ```

5. Check a link that crosses to that drive. Make a junction elsewhere on `C:` that
   points at the mount, and run the check through it:

   ```powershell
   cmd /c mklink /J "C:\Users\<you>\obelisk-test-tmp\usb-link" "C:\Users\<you>\obelisk-test-tmp\usb-mount"
   $env:OBELISK_MANUAL_MOUNT = "C:\Users\<you>\obelisk-test-tmp\usb-mount"
   $env:OBELISK_MANUAL_LINK = "C:\Users\<you>\obelisk-test-tmp\usb-link"
   go test -count=1 -v -run '^TestManual_FolderMountedDrive$' .
   Remove-Item Env:OBELISK_MANUAL_MOUNT, Env:OBELISK_MANUAL_LINK
   cmd /c rmdir "C:\Users\<you>\obelisk-test-tmp\usb-link"
   ```

   `rmdir` removes only the junction. It does not touch the drive.
6. To undo the mount, return to **Change Drive Letter and Paths**, select the folder
   path, and choose **Remove**. This removes the mount only. The data stays on the
   drive.

## What you should see

The test prints two lines and passes:

```text
Windows:  disk 2, serial "ABC123", size 500107862016, partition mbr:0a1b2c3d-01
Obelisk:  disk 2, serial "ABC123", size 500107862016, partition mbr:0a1b2c3d-01
```

A GPT drive shows `gpt:` and its partition GUID on both lines. An MBR drive, which
most USB sticks are, shows `mbr:`, its disk signature, and the partition number.

The disk number, serial, size, and partition must agree. The disk must not be the
one that holds `C:`.

## If it fails

- **"no single partition is mounted"**: the folder path does not match the mount.
  Check it in Disk Management.
- **"is on the system disk"**: the folder holds a partition of the system disk. Mount
  a different drive.
- **An empty serial on both lines** is normal for some USB bridges.
- **"Windows reported no partition ID"**: the disk has no signature or an unknown
  partition style, so there is nothing to compare. The check fails rather than pass
  without comparing.
- **An MBR number that differs by a little**: Windows skips empty partition slots and
  counts logical partitions differently. If the signature matches and the drive has an
  empty slot or logical partitions, report the output so it can be checked by hand.
- **Any other mismatch** is a bug. Keep the output and report it.
- **Access denied from `Get-Partition`**: run PowerShell as administrator.
