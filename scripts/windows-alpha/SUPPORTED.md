# Supported envelope and pending qualification

| Area | Candidate boundary |
|---|---|
| Target | One Windows amd64 build; same-workstation relocation is not a clean VM/second-machine result |
| Runtime | External Node 24 x64 and browser; `Launch.cmd` runs in the built-in Command Prompt; no PowerShell, execution-policy change, Go, Git or compiler required for use |
| Input | Small, quiescent, explicitly generated expendable sources on ordinary fixed local storage |
| Catalog | Existing preview subset of native schema 8; current records and supported copy relationships, strict original encoding and canonical inventory scope |
| Viewer | One or two fixed generated snapshots; Library/Find/A/B/All, exact-name representation, exact native IDs and two-sided comparison evidence; recorded source paths are text only |
| Comparison | Explicit reference and accepted single-root slash-relative alignment; five recorded-result classes, filters and details; no live verification, physical-copy proof or synchronization |
| Not provided | Production scan/registration, incremental updates, backup/archive operations, restore, device/media actions, installer, updates or public release |
| Still unqualified | Clean/second machine, broader interactive consoles, production/scale, hostile races, ACLs, power loss, unsupported storage/filesystems/names, other OS/architecture, Docker/CI and hardware |

Producer limits: 64 regular files (including excluded files), 128 entries, depth 8, 512-byte relative paths, 4096-byte absolute paths, 8 MiB per included file and 32 MiB total observed content. One growth-detection byte may be read before refusal. The 30-second cooperative deadline cannot interrupt a blocked kernel call. Unsupported entries, observed changes, cancellation or pre-publication errors prevent successful publication.

Source and output-parent directories must already exist and be disjoint lexically and by checked ancestor identity. Output must be absent. Windows path rules refuse UNC/device/stream/ambiguous paths, links/reparse points and special entries. The output filesystem must support the producer's no-replace hard-link publication. Staged content is validated, written, synced and closed before publication; a later cleanup/status failure retains truthful `published:true`. No replacement fallback or power-loss guarantee exists.

`.DS_Store` exclusion is OFF by default and opt-in per inventory. Only exact enumerated regular-file basenames match; directories of that name are traversed. Links/specials are refused before filtering. Near/case variants and sidecars remain included. Excluded entries count toward bounds and are rechecked but contents are not opened/hashed. Sources are unchanged. Durable scope distinguishes OFF, ON/counts, valid zero, all-excluded, empty and historical UNKNOWN; malformed required keys/counts cannot manufacture complete-empty success.

Viewer limits remain 4 MiB catalog input; 1000 files; 100 rows per ancillary table; 1000 potential copy occurrences; 4096-byte strings. Ordinary text/hash queries retain 256/64-byte native limits; exact-name queries retain 4096 decoded UTF-8 bytes, 24578 entry characters, a 16384-character URL cap and 32768-byte native line buffer. Legacy request bounds remain intact. No arbitrary advanced native sections, schema migration or normalization is added.

Two-input adoption is additionally bounded at 1000 aggregate files/copy occurrences,
16 MiB per reader response and 32 MiB for the pair. Comparison requires complete
enumeration from both adopted readers, one unambiguous root per input and unique
exact slash-relative keys. Case, Unicode sequences and supported controls remain
distinct; internal backslashes are literal. Unsupported/duplicate frames refuse;
no inferred basename/suffix mapping. Full typed SHA-256 and exact decimal sizes
supply evidence, with matching hash/unequal size classified inconclusive first.
The five classes partition recorded relative keys, not physical copies. Responses
are capped at 8 MiB, request URLs at 512 characters and reader deadlines at five
seconds. Failure/caps never become successful truncated or one-sided counts.

The new package requires its matching included reader and both comparison modules;
an earlier inventory-only executable/asset set is not interchangeable. The native
format remains schema 8, including strict scope compatibility and historical
UNKNOWN. No public release or replacement of the earlier frozen ZIP is implied.
Prompt20 remains deferred by the owner; local relocation is not second-machine
qualification. Pipe-based stop and any actual-console observations are recorded in
the package review records under `docs/development/reviews/` in the source
repository, not in this package; earlier console passes are not new evidence.

The packaged binary is launcher-only. It is built with `-tags guionly` and contains only `--gui-disposable-inventory` and `--gui-catalog-readonly`; any other first argument is refused with exit status 2. The executable reports its version (`0.9.2-dev-comparison.<commit>`, also in `package-manifest.json`) in each inventory result, in the viewer's reader line and footer, and in its refusal message. `main.go` (the legacy HTTP server, embedded web UI and backend routes) is not compiled in, and the remaining backend code is not linked. The executable links no code to listen on a port, make HTTP requests or start other programs; its build tests check the linked symbols against a full-build control. It is not a release escrow bundle, and the launcher is not a sandbox. Third-party helper executables are not bundled. Licensing notices cover linked dependencies and the Go runtime; public redistribution/signing/reputation review remains pending. Source/build hashes identify bytes, not safety certification.

Use `Launch.cmd` for every action. Do not use `bin/obelisk.exe` directly: it has no legacy server, and on its own it only prints its refusal message (TESTING.md checks this).

An owner-reported negative scan does not classify the earlier detection or certify this host/package. Protection must stay enabled. A new detection, quarantine or permission block must be investigated without retries in alternate forms. No security exclusion, policy change, administrator operation or automatic sample upload is part of this tutorial.

PR-04, external-tar Unicode containment, broader persistence/concurrency and other-platform qualification remain separate. LTO-8 is the first physical qualification target, not a generation limit; tape/ring buffer and Blu-ray are separate workstreams.
