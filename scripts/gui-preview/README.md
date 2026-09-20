# Isolated GUI preview

Library and the Smith Wedding comparison now use the supplied PDF references (pages 1 and 4) with a provisional teal/light treatment. Pages 2 and 3 remain unused visual alternatives, not disclosure modes. This is a synthetic design preview, not a final theme decision or production UI.

Requires preinstalled Node.js 24 (executed here with 24.19.0). No package install, Go backend, build step, configuration, catalog or device is needed.

From PowerShell:

```powershell
Set-Location 'C:\Users\nsott\source\repos\obelisk'
node scripts/gui-preview/server.mjs 0
```

Open the exact `http://127.0.0.1:<assigned-port>/` printed in that terminal. Port `0` asks Windows for an available port. To request a fixed port, replace `0` with e.g. `4317`; an occupied port fails without changing or stopping its existing listener. Only the exact loopback host is accepted. Do not launch the normal Go application for this preview.

Stop with **Ctrl+C** in that terminal and wait for the PowerShell prompt to return. The launcher also closes after 60 minutes. It writes no settings or browser storage. Closing the browser tab alone does not stop the server.

Library supports inventory search, storage selection, and Smith Wedding navigation via its name, Fix issues, or Compare copies. Return with the Library breadcrumb. Project search filters the supplied selected-file example; HDD-017 expands/collapses. Operational-looking controls produce explicit demo-only responses and perform no operations. Review wording retains the source ambiguity: six absent/differing items are called unresolved by that action, while the narrower matrix Unresolved count stays zero. The offline count of 200 is kept visible instead of reproducing the source clipping.

Find retains text search by name/path/hash/version/occurrence/medium, project/medium/availability filters, selection and occurrence details. The same original Library content inspector is available in the additional-preview-controls disclosure below the aligned tables. Use the fixture presentation selector for ready, partial-information, empty, held-loading and error cases; choose Ready to retry. Reset filters restores the default sample. These extra controls and Find's visual layout are not specified by the export.

Guided shows core evidence; Studio adds synthetic hash and inventory scope; Expert also exposes the synthetic medium identifier. All modes retain offline/unknown/verification warnings. Display selection lasts for this page session. It changes no guarantees or permissions. Back Up and Archive are workflow placeholders; Activity and Devices show static samples; operational Settings are deferred. No operational action is wired.

Checks:

```powershell
node --test --test-isolation=none scripts/gui-preview/preview.test.mjs
```

Optional rendering check with an already installed Chrome (no package installation). Supply a **new**, absolute evidence directory under AppData; the harness refuses to reuse an existing directory. It starts a loopback server and headless browser with a disposable profile, captures screenshots and a browser accessibility tree, then stops and waits. A 90-second deadline bounds the run. Example:

```powershell
node scripts/gui-preview/browser-check.mjs 'C:\Program Files\Google\Chrome\Application\chrome.exe' 'C:\Users\nsott\AppData\Local\ObeliskDev\gui-preview-owner-check-NEW-TIMESTAMP'
```

The preview lives outside Go's embedded UI tree. The server preloads exactly four assets, serves only GET/HEAD, refuses other methods and unknown paths, and has no filesystem browsing, production imports or proxy. Browser policy blocks network connections and form submissions; application/fixture modules have no I/O adapter. The browser-check harness is separate local validation tooling and is never served.

The current shell/Library/project references are readable and have been applied. Find, other workspaces, dialogs, additional disclosure layouts and responsive designs remain unspecified. Studio is initially selected as in all three Library references. Arial/system fonts and locally drawn SVG icons substitute for unprovided font/icon assets. Browser comparison uses 1440x1024 CSS pixels at scale 2 as a PDF-based assumption. Owner visual review and broader accessibility review remain outstanding. See the 2026-09-20 design-alignment report under docs/development/reviews for executed checks and screenshot locations.
