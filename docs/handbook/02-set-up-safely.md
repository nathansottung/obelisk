# Set up safely

This guide gets Obelisk ready before you make your first backup. Take your time. None of these steps touch your original photos or files. Obelisk only ever reads your files, never changes or deletes them, and never sends anything over the internet.

Before you start, make sure Obelisk is running and open in your web browser at http://127.0.0.1:7821. On the left you will see tabs: Home, Vault, Protection, Integrity, Packages, Burn, Volumes, Dock, Keys, Jobs, and Settings. At the bottom-left there is a small status "lamp" (a little indicator that tells you if the app is ready).

---

## Part 1: Choose a staging folder

A "staging folder" (a big scratch work folder) is a temporary workspace on your computer. When Obelisk builds a "package" (one media-sized sealed unit of your files), it does the work in the staging folder first, then writes the finished result to your drive.

Two good-to-know facts:

- The staging folder should be **big and fast**. Fast means an internal drive (like your computer's main disk) or a fast external SSD, not a slow USB stick. This makes building quicker.
- The staging folder is **reused for each package, one at a time**. So it only needs enough room for your **single biggest package**, not your whole archive. For example, if your biggest package is sized for a 100 GB disc, you need a little more than 100 GB free, not room for everything you own.

Here is how to set it:

1. Click the **Settings** tab on the left.
   You should now see the settings page with boxes for folder paths.

2. Under **Pipeline**, find the box labeled **"Staging folder (big + fast; the NAS itself is ideal)."** Click the **Browse…** button next to it.
   A folder picker window opens.

3. Pick a folder on a big, fast drive with plenty of free space. If you are not sure, make a new folder named something like `Obelisk-Staging` on your main drive and choose that.
   The folder's path now appears in the box.

4. Click **Save settings** at the bottom of that section.
   You should now see a "Settings saved" message, and the staging folder path stays filled in.

![The Settings page with the staging folder path filled in](../img/02-staging-folder.png)

That's it for staging. If you never plan to encrypt (scramble) your packages, you can skip the rest of this guide and move on to your first backup.

---

## Part 2: Decide about encryption (optional)

"Encryption" means scrambling your packages so only someone with the secret passphrase (a long password) can read them. This is useful if a drive might be lost or stolen. It is completely **optional**. If you want to keep things simple, you can skip encryption, and you will not need any of the keystore steps below.

If you do want encryption, read on carefully. There is one rule that protects you from a heartbreaking mistake.

### The two-keystore rule, in plain terms

A "keystore" (a small file that holds the secret passphrases for encrypted packages) is what unlocks your encrypted packages later. If you lose the only keystore, your encrypted data is **gone forever** — no one, including you, can recover it.

To protect you from that, Obelisk **refuses to build encrypted packages until you have at least TWO keystores registered**. Two keystores means two separate copies of the secret, so losing one is not a disaster. They must be on **two different devices** — for example, one on your computer and one on a USB stick you store somewhere safe.

Important: Obelisk never stores the passphrase itself in its catalog. It only keeps a "fingerprint" (a short code that can confirm a passphrase is correct but cannot reveal it). So you are the only keeper of the real secret. That is exactly why two copies matter.

### Create your key

Before your first encrypted package, make the key that will lock it. Obelisk will not encrypt anything with a key until the key is **ready**: it is in both keystores, and you have made an offline copy of it and proved that copy by typing it back.

1. Click the **Keys** tab, then **Create key…**.
   If you have not set two keystores yet, the dialog asks for them first: give two keystore files on **two different drives** (for example `D:\obelisk-keys\keys.json` and `E:\obelisk-keys\keys.json`). Obelisk refuses two files on one drive.

2. Give the key a name you will recognise later, such as "Family archive key", and click **Create key**.
   The key is made and saved to both keystores. It shows as **not ready**.

3. Obelisk goes straight on to **Make an offline copy**. Make at least one of these, and as many as you like:
   - **Print it.** Click **Show the key to print**. The key appears as short numbered lines (`L01/03`, `L02/03`, …), each with a check code, and the full SHA-256 fingerprint. Click **Print…**.
   - **Save a plain-text file.** Type the whole path of a new file on a drive you keep apart, and click **Save the file**. Obelisk refuses its own data folder, a drive that holds your backups, and a drive that holds a package this key opens. It never saves a key file on its own.
   - **Copy it into a password manager.** Click **Show the note**, then **Copy the note**, and paste it into a secure note. The note holds the key, its fingerprint, what it opens, and how to restore by hand. Obelisk clears the clipboard 30 seconds later and says so.

4. Click **I made a copy: prove it →**. Type the key back from your printout line by line, or paste back your text file or note, and click **Prove it**.
   Obelisk compares the whole key by its SHA-256 fingerprint. If a line is missing or changed, it names that line. When it matches, the key shows **ready**, and encrypted builds can run.

**Why this matters:** lose every copy of a key and every package made with it can never be opened, by you or anyone. Keep the copies in **separate places**: a printout in one place, a file or a password-manager note in another, and the two keystores on their own drives.

The key is shown only inside the offline-copy dialog, and only after you click to show it. It never goes into a web address, Obelisk's log, a job record or an app-state backup.

### One key for every package

Every encrypted package uses your **current key**: the newest key that is ready. **One key opens every package made with it.** To limit that, make a new key with **Make a new key…** on the Keys tab. Once you prove the new key, new packages use it. Until then they keep using the old one. No key is ever removed, so every package keeps opening with the key it was made with. The Keys tab shows each key's name, the date it was made, and how many packages it opens, and each package's `RESTORE.txt` names its key.

**A separate key for one package.** If a package is meant to be handed to someone else, click **Separate key…** on that package before you build it. That key opens only that package, so you can hand over the package and its key without handing over your other backups. It needs its own offline copy and proof before the package is built. This option is off unless you choose it.

**Keys made before this change** show **not proven**. Make an offline copy of each and prove it from the Keys tab. Restores do not wait for this, and no new package uses those keys.

**Adopted backups** (drives or discs you brought into the catalog as they were) are shown as **not encrypted, adopted as found** when they were not encrypted. Obelisk never encrypts data in place, so they have no encrypt control. To get an encrypted copy, plan and build a new package from the source.

![The Keys tab showing a ready key](../img/02-two-keystores.png)

---

## Part 3: More copies of your keys (optional but wise)

You can make another offline copy of a key at any time: on the **Keys** tab, click **Make an offline copy…** beside it, then **Prove a copy…**.

You can also export a **Recovery Kit** for safekeeping: on the **Keys** tab, click **Export Recovery Kit…**, choose a folder, and store the result somewhere safe. The kit holds every key, printed one per page, so store it as carefully as the keys themselves. (The [Recovery Kit guide](09-the-recovery-kit.md) explains what's inside.)

---

## How to know it worked

- The **Settings** page shows your staging folder path saved.
- If you chose encryption: the **Keys** tab lists both keystores as reachable, and under **Your key** it says which key encrypted builds use. That key shows **ready**.
- You hold at least one offline copy of that key, proven, and you have decided where to keep it, apart from your backups.

## If something went wrong

- **The Keys tab says no key is ready.** Finish the step it names: create a key, or make an offline copy of it and prove it.
- **A build says no key is ready.** Click **Create key →** (or **Finish the key →**) in the message. It takes you to the step to finish.
- **The proof names a line.** That line on your copy is missing or mistyped. Fix it and click **Prove it** again.
- **A keystore shows unreachable.** Its file could not be found: plug its drive in and reload.
- **Browse… does not open a picker.** Make sure Obelisk is still running and refresh the page in your browser (press F5).
- **Not enough space when building later.** Your staging folder's drive may be too full. Free up space, or choose a staging folder on a drive with more room. Remember it only needs room for your single biggest package.
- Need a reminder on any screen? Click the small **i** help button on that screen.

## Screenshots to capture

- `../img/02-staging-folder.png` — The Settings page with the staging folder path filled in.
- `../img/02-two-keystores.png` — The Keys tab showing two reachable keystores and a ready key.
