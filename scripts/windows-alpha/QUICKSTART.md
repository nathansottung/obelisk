# Local Windows developer-alpha packaging candidate

This unsigned package is a developer rehearsal of generated-source inventory and a read-only viewer. It is not a released backup/archive product. The included full Obelisk executable has other commands; this tutorial launcher selects only the accepted inventory/reader modes and is not a security sandbox.

Prerequisites: Windows x64, **existing Node.js 24 x64** on PATH, Windows PowerShell, and an existing browser (Chrome is the rehearsal target). Go, Git, a compiler and the source checkout are not needed by testers. No runtime is downloaded or bundled. `package-manifest.json` records exact source, script and binary identities; hashes are not signatures or malware-free certification.

Unpack the ZIP into a **new ordinary fixed-local directory**, separate from your tutorial workspace. In PowerShell, change to that extracted directory. These commands are relative to the package:

```powershell
Get-Command node.exe -CommandType Application
node --version
.\Launch.ps1 check
.\Launch.ps1 help
```

If Node is missing or not version 24 x64, stop and ask the tester coordinator to resolve the prerequisite. If a security detection or permission/script-policy block occurs, stop and report it for investigation. Do not disable protections, add exclusions, allow a detection, restore quarantine, change execution policy or try alternate execution forms to get past it. Unsigned status does not make a warning harmless.

Choose a **new**, expendable workspace beneath your own `LOCALAPPDATA\ObeliskDev`. The accepted viewer requires that boundary. The immediate parent must exist; the generator may create the `ObeliskDev` root if absent. A prior workspace, even empty, is refused. Keep the directory quiescent and use a fixed local drive; no user archives, NAS, removable media or existing evidence folders.

```powershell
$workspace = Join-Path $env:LOCALAPPDATA 'ObeliskDev\Alpha tutorial cafe 01'
.\Launch.ps1 generate $workspace
.\Launch.ps1 inventory $workspace off.json
.\Launch.ps1 inventory $workspace on.json --ignore-ds-store
```

Generation, inventory and viewing are separate deliberate actions. The generator writes ten synthetic files only into the new workspace's `source` directory. The producer reads those selected sources and creates snapshots in the disjoint `catalogs` directory. Default OFF includes ten files; ON includes eight and excludes two exact regular `.DS_Store` files. The near name, sidecar and content below the same-named directory remain included. Equal bytes at two paths remain distinct records. Check both process exit and `published`; an error after publication may still say `published:true`. Do not delete a final catalog to pretend publication was rolled back.

```powershell
$off = Join-Path $workspace 'catalogs\off.json'
$on = Join-Path $workspace 'catalogs\on.json'
.\Launch.ps1 view $off
```

Open the printed `http://127.0.0.1:<port>/` URL. Library shows recorded collections and scope. Find searches recorded paths/hashes; select a result for the inspector. Try `café` and `same-`. For an exact name, enable **Exact name (JSON string)** and enter `"nested/O'Brien & +%# note.txt"`. Names/IDs are not normalized. The catalog is read but not modified, and recorded source/media paths are not opened. Historical recorded evidence is not current availability, a backup copy or verification.

In the launching terminal type **stop**, press Enter, and wait for both `Preview stopped ... catalog reader waited` and `Packaged child waited`. Confirm both exit codes are zero and that the former URL no longer responds. Ctrl+C uses the existing console signal path; interactive-console qualification remains a distinct owner check until recorded as passed. Closing a browser tab alone does not stop the viewer. Automatic shutdown remains 60 minutes.

```powershell
# After stopping the previous viewer:
.\Launch.ps1 view $on
# Type stop and wait, then reopen the retained snapshot without regeneration:
.\Launch.ps1 view $off
```

Repeating `inventory $workspace off.json` must refuse without replacing its bytes. For another observation, explicitly choose a new name such as `off-2.json`. Generation never merges or cleans a prior run. No catalog migration is performed.

Intentional static samples are separate: `.\Launch.ps1 static`. This action visibly uses synthetic demo records, not your catalog. Missing/invalid catalogs, adapters or assets must not fall back to static success. No browser filesystem/executable picker or producer endpoint exists; operational controls remain disabled/demo-only.

New scoped catalogs require the compatible corrected reader included in this package. The previously identified pre-correction reader refuses populated scope. Supported older unscoped catalogs remain UNKNOWN, never inferred OFF/zero. Pair outputs with the documented source/binary identities. Never strip/rewrite scope metadata for compatibility.

See SUPPORTED.md for exact limits and pending distribution gates, LICENSE and THIRD-PARTY-NOTICES.txt for included notices, and BUG-REPORT.md for optional redacted reporting. There is no telemetry or automatic upload.
