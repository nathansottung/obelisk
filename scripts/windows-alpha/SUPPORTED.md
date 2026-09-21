# Supported envelope and pending qualification

| Area | Candidate boundary |
|---|---|
| Target | One Windows amd64 build; same-workstation relocation is not a clean VM/second-machine result |
| Runtime | External Node 24 x64, Windows PowerShell and browser; no Go/Git/compiler required for use |
| Input | Small, quiescent, explicitly generated expendable sources on ordinary fixed local storage |
| Catalog | Existing preview subset of native schema 8; current records and supported copy relationships, strict original encoding and canonical inventory scope |
| Viewer | Read-only catalog adoption, Library/Find/inspector, exact-name representation and native ID precision; recorded source paths are text only |
| Not provided | Production scan/registration, incremental updates, backup/archive operations, restore, device/media actions, installer, updates or public release |
| Still unqualified | Clean/second machine, broader interactive consoles, production/scale, hostile races, ACLs, power loss, unsupported storage/filesystems/names, other OS/architecture, Docker/CI and hardware |

Producer limits: 64 regular files (including excluded files), 128 entries, depth 8, 512-byte relative paths, 4096-byte absolute paths, 8 MiB per included file and 32 MiB total observed content. One growth-detection byte may be read before refusal. The 30-second cooperative deadline cannot interrupt a blocked kernel call. Unsupported entries, observed changes, cancellation or pre-publication errors prevent successful publication.

Source and output-parent directories must already exist and be disjoint lexically and by checked ancestor identity. Output must be absent. Windows path rules refuse UNC/device/stream/ambiguous paths, links/reparse points and special entries. The output filesystem must support the producer's no-replace hard-link publication. Staged content is validated, written, synced and closed before publication; a later cleanup/status failure retains truthful `published:true`. No replacement fallback or power-loss guarantee exists.

`.DS_Store` exclusion is OFF by default and opt-in per inventory. Only exact enumerated regular-file basenames match; directories of that name are traversed. Links/specials are refused before filtering. Near/case variants and sidecars remain included. Excluded entries count toward bounds and are rechecked but contents are not opened/hashed. Sources are unchanged. Durable scope distinguishes OFF, ON/counts, valid zero, all-excluded, empty and historical UNKNOWN; malformed required keys/counts cannot manufacture complete-empty success.

Viewer limits remain 4 MiB catalog input; 1000 files; 100 rows per ancillary table; 1000 potential copy occurrences; 4096-byte strings. Ordinary text/hash queries retain 256/64-byte native limits; exact-name queries retain 4096 decoded UTF-8 bytes, 24578 entry characters, a 16384-character URL cap and 32768-byte native line buffer. Legacy request bounds remain intact. No arbitrary advanced native sections, schema migration or normalization is added.

The full binary embeds the existing production UI and development escrow placeholder because the accepted Go source does. Neither is activated by the launcher; it is not a minimal-command sandbox or a release escrow bundle. Third-party helper executables are not bundled. Licensing notices cover linked dependencies and the Go runtime; public redistribution/signing/reputation review remains pending. Source/build hashes identify bytes, not safety certification.

An owner-reported negative scan does not classify the earlier detection or certify this host/package. Protection must stay enabled. A new detection, quarantine or permission block must be investigated without retries in alternate forms. No security exclusion, policy change, administrator operation or automatic sample upload is part of this tutorial.

PR-04, external-tar Unicode containment, broader persistence/concurrency and other-platform qualification remain separate. LTO-8 is the first physical qualification target, not a generation limit; tape/ring buffer and Blu-ray are separate workstreams.
