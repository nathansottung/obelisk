# Local Windows comparison developer-alpha packaging candidate

This unsigned package is a local candidate for generated-source inventory, one/two-snapshot browsing and recorded comparison. It supersedes the accepted comparison package (ZIP SHA-256 `33d1d9e5…`) for the same tutorial content, and it is separate from the earlier accepted inventory-only ZIP (`f25125c4…`). Both earlier ZIPs stay frozen and unchanged. This package awaits its own package review. It is not a released backup/archive product.

The included Obelisk executable is launcher-only. It contains only the inventory and read-only viewer modes and refuses every other command. The executable has no HTTP server, web UI or backup/archive routes. The `view` action runs the package's own Node.js viewer, which listens on 127.0.0.1 only and asks the executable for catalog data. The launcher is not a security sandbox.

Prerequisites: Windows x64, **existing Node.js 24 x64** on PATH, and an existing browser (Chrome is the rehearsal target). `Launch.cmd` runs in the built-in Command Prompt. PowerShell, Go, Git, a compiler and the source checkout are not needed by testers. No runtime is downloaded or bundled. `package-manifest.json` records exact source, script and binary identities; hashes are not signatures or malware-free certification. See TESTING.md for what to run, what to report and what never to use.

## Before extracting: unblock the downloaded ZIP

Windows marks downloaded files as coming from the internet and copies that mark to every extracted file. Remove it once, from the ZIP, **before** extracting:

1. Right-click the ZIP and choose **Properties**.
2. On the **General** tab, tick **Unblock** next to "This file came from another computer…", then choose **OK**. If there is no Unblock checkbox, the ZIP is not marked; continue.
3. Extract the ZIP into a **new ordinary fixed-local directory**, separate from your tutorial workspace.

What Windows may still show:

- **Skipped Unblock and double-clicked `Launch.cmd`:** an "Open File - Security Warning" dialog saying the publisher could not be verified (Unknown Publisher, type Windows Command Script). Choose **Cancel**, unblock the ZIP, and extract it again into a new directory. Do not choose Run to get past it.
- **Running `Launch.cmd` from Command Prompt:** no Windows prompt is expected, whether or not the ZIP was unblocked. Microsoft Defender may still scan the unsigned executable on first use, which can briefly delay the first command.
- **A Defender detection, quarantine, SmartScreen "Windows protected your PC" screen or permission block:** stop and report it (see below).

Loopback-only listening is not expected to produce a firewall prompt. If one appears, choose Cancel and report it.

## Check the prerequisites

Open **Command Prompt** (Start menu → type `cmd`) and change to the extracted directory, for example `cd /d "%USERPROFILE%\Downloads\obelisk-package"`. These commands are relative to the package:

```bat
where node
node --version
Launch.cmd check
Launch.cmd help
```

Every action except `help` first prints a line naming the package version, for example `Obelisk developer alpha 0.9.2-dev-comparison.<commit>`. Include that line in any report.

If Node is missing or not version 24 x64, stop and ask the tester coordinator to resolve the prerequisite. If a security detection or permission block occurs, stop and report it for investigation. Do not disable protections, add exclusions, allow a detection, restore quarantine or try alternate execution forms to get past it. Unsigned status does not make a warning harmless.

## Single-snapshot walkthrough

Choose a **new**, expendable workspace beneath your own `%LOCALAPPDATA%\ObeliskDev`. The accepted viewer requires that boundary. The immediate parent must exist; the generator may create the `ObeliskDev` root if absent. A prior workspace, even empty, is refused. Keep the directory quiescent and use a fixed local drive; no user archives, NAS, removable media or existing evidence folders.

```bat
set "workspace=%LOCALAPPDATA%\ObeliskDev\Alpha tutorial cafe 01"
Launch.cmd generate "%workspace%"
Launch.cmd inventory "%workspace%" off.json
Launch.cmd inventory "%workspace%" on.json --ignore-ds-store
```

Keep the quotation marks around paths. Command Prompt expands `%NAME%` inside them; that is how `%LOCALAPPDATA%` and `%workspace%` work.

Generation, inventory and viewing are separate deliberate actions. The generator writes ten synthetic files only into the new workspace's `source` directory. The producer reads those selected sources and creates snapshots in the disjoint `catalogs` directory. Default OFF includes ten files; ON includes eight and excludes two exact regular `.DS_Store` files. The near name, sidecar and content below the same-named directory remain included. Equal bytes at two paths remain distinct records. Each inventory result line reports `version` (the executable's version), `published` and counts. Check both process exit and `published`; an error after publication may still say `published:true`. Do not delete a final catalog to pretend publication was rolled back.

```bat
Launch.cmd view "%workspace%\catalogs\off.json"
```

The viewer prints `Native catalog reader version: …`, then a `http://127.0.0.1:<port>/` URL. Open that URL. The page footer also shows the reader version. Library shows recorded collections and scope. Find searches recorded paths/hashes; select a result for the inspector. Try `café` and `same-`. For an exact name, enable **Exact name (JSON string)** and enter `"nested/O'Brien & +%# note.txt"`. Names/IDs are not normalized. The catalog is read but not modified, and recorded source/media paths are not opened. Historical recorded evidence is not current availability, a backup copy or verification.

In the launching window type **stop**, press Enter, and wait for both `Preview stopped ... catalog reader waited` and `Packaged child waited`. Confirm both exit codes are zero and that the former URL no longer responds. Ctrl+C uses the existing console signal path. Command Prompt may then ask `Terminate batch job (Y/N)?`; the viewer has already stopped, so either answer is fine. Interactive-console qualification remains a distinct owner check until recorded as passed. Closing a browser tab alone does not stop the viewer. Automatic shutdown remains 60 minutes.

```bat
rem After stopping the previous viewer:
Launch.cmd view "%workspace%\catalogs\on.json"
rem Type stop and wait, then reopen the retained snapshot without regeneration:
Launch.cmd view "%workspace%\catalogs\off.json"
```

Repeating `inventory "%workspace%" off.json` must refuse without replacing its bytes. For another observation, explicitly choose a new name such as `off-2.json`. Generation never merges or cleans a prior run. No catalog migration is performed.

Intentional static samples are separate: `Launch.cmd static`. This action visibly uses synthetic demo records, not your catalog. Missing/invalid catalogs, adapters or assets must not fall back to static success. No browser filesystem/executable picker or producer endpoint exists; operational controls remain disabled/demo-only.

New scoped catalogs require the compatible corrected reader included in this package. The previously identified pre-correction reader refuses populated scope. Supported older unscoped catalogs remain UNKNOWN, never inferred OFF/zero. Pair outputs with the documented source/binary identities. Never strip/rewrite scope metadata for compatibility.

See SUPPORTED.md for exact limits and pending distribution gates, TESTING.md for the tester checklist, LICENSE and THIRD-PARTY-NOTICES.txt for included notices, and BUG-REPORT.md for optional redacted reporting. There is no telemetry or automatic upload.

## Two-snapshot ALPHA/BETA walkthrough

After the prerequisite/check steps above, run these commands from the **new
extracted package**. Choose a different unused workspace name for each rehearsal.
Keep this ordinary local workspace separate from the package. Generation creates
two small expendable source trees; it does not scan or create catalogs.

```bat
set "pair=%LOCALAPPDATA%\ObeliskDev\Comparison tutorial 01"
Launch.cmd generate-pair "%pair%"
Launch.cmd inventory "%pair%\ALPHA" off.json
Launch.cmd inventory "%pair%\BETA" off.json
Launch.cmd inventory "%pair%\BETA" on.json --ignore-ds-store
set "a=%pair%\ALPHA\catalogs\off.json"
set "b=%pair%\BETA\catalogs\off.json"
set "bOn=%pair%\BETA\catalogs\on.json"
Launch.cmd view "%a%"
rem Type stop and Enter; wait for Preview stopped and Packaged child waited.
Launch.cmd view "%a%" "%b%"
```

Each OFF snapshot is an actual native inventory of 11 generated files. BETA ON
contains nine records and excludes two regular `.DS_Store` files. Near name
`.DS_Store.bak`, the sidecar and `directory/.DS_Store/keep.txt` remain included.
All excluded files stay on disk. Existing workspaces/catalogs refuse replacement.

In the two-input session, Library/Find provide **Snapshot view** A/B/All. Both
inputs deliberately share basename `off.json`; the A/B qualifiers distinguish
their session identities. Open **Compare recorded snapshots**, explicitly choose
ALPHA's Snapshot A as reference, inspect both recorded roots/scopes/times, then
click **Accept root alignment and compare recorded snapshots**. Alignment is a
chosen relative-root frame, not proof of physical equivalence or independent copies.

Independent OFF/OFF expectation: 12 exact keys in the union, 11 recorded on each
side; **9 recorded checksum agreements, 1 difference, 0 inconclusive, 1 only in
reference, 1 only in counterpart**. The difference is `nested/O'Brien & +%# note.txt`;
one-sided paths are `ALPHA-only.txt` and `BETA-only.txt`. Equal bytes at different
paths remain separate. Try the class filter, literal `café` filter and exact JSON
path `"nested/O'Brien & +%# note.txt"`; select a row to inspect both sides.

Agreement concerns full recorded checksums, not a current read or backup-health
verdict. Only-recorded means absence from that complete recorded set, not physical
deletion. The ordinary producer supplies full hashes, so this tutorial has no
inconclusive pair; supported missing/conflicting evidence is not fabricated here.

```bat
rem Stop and wait between each session; these commands reuse retained snapshots.
Launch.cmd view "%b%"
Launch.cmd view "%b%" "%a%"
rem BETA is now Snapshot A. Choose either reference deliberately in the GUI.
Launch.cmd view "%a%" "%bOn%"
rem With ALPHA as reference: agreement 7, difference 1, reference-only 3,
rem counterpart-only 1; union 12. The .DS_Store rows retain scope qualification.
Launch.cmd view "%a%" "%b%"
```

Argument order assigns A/B for that session, never an authoritative original.
Changing reference reverses side attribution and one-sided classes. No reference
or alignment is accepted automatically. Unknown historical scope/time remain
unknown, never inferred from load time. Duplicate identical artifacts, unsupported
frames, malformed inputs and either reader failure are refused without a partial
successful comparison. For reader failure, stop/wait and relaunch valid inputs;
for ambiguous comparison frames, ordinary browsing may remain available.

`view` accepts exactly one or two existing absolute catalog paths. Missing values,
third inputs, invalid option combinations and missing package components fail
explicitly. Identical input paths refuse before startup; identical-byte artifacts
under different names are refused by the accepted runtime. No-argument help is
inert; static samples require the explicit `static` action. The viewer's generated
data boundary remains beneath LOCALAPPDATA/ObeliskDev. Do not supply private or
production catalogs/sources. The wrapper is convenience, not a security sandbox.
