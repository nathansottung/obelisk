# Windows comparison package acceptance and source checkpoint — 2026-09-21

The owner separately submitted Prompt 30 and accepted the reviewed LOCAL UNSIGNED
Windows x64 generated-data package at 2026-09-21T20:00:53.1389919-04:00. This is owner acceptance
of the bounded package, not a claim of owner dogfood, independent human review,
security certification, production readiness or public binary distribution.

## Source and unchanged artifact identities

Branch: feat/windows-comparison-alpha-package.
PACKAGE_PATCH_BASE_SHA: e3d9bef998a20dff78dc67463dfb8f848aad76ce
PACKAGE_IMPLEMENTATION_SHA: be6f60f178c793835cdbad82c9f774f61fd4071d
Implementation tree: b15b4534f7c34bfa3c764a628ae7d4825f04d1f1
Implementation message: Add reviewed Windows multi-snapshot comparison packaging

The separate evidence commit contains this acceptance record, the byte-preserved
historical implementation/review reports and non-packaged living-record closeout.
Its own SHA and publication result belong in the external receipt, not this commit.

Accepted comparison archive (unchanged pre-existing bytes):
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-alpha-20260921-175648\build\0.9.0-dev-comparison-package.e3d9bef998a2-windows-amd64-local.zip`
SHA-256: `33d1d9e5087c35aca23606957777f3d13ff8267de798c9848c79ada4697f00d2`.

Older inventory archive remains separately accepted and unchanged:
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-inventory-alpha-20260920-232225\build-complete\0.9.0-dev-inventory-package.bfbce891df78-windows-amd64-local.zip`
SHA-256: `f25125c46acca5635b2297305d849888fbded129281708875a64289c4b22c5de`.

Original reviewed candidate manifest:
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-alpha-20260921-175648\candidate-identities.json`
SHA-256: `44f01813dc8074428c816b04194da996c1b9612dba75d90181e0cd46755a31fe`.
Original packaged manifest: author directory `build/stage/package-manifest.json`,
SHA-256: `9ab8ac95a56cc16686b778070667dbd180a58a5062e21e6699e0c7fb7196c28c`.
Native build-source manifest SHA-256:
`18d3943c151038f28d554ee103aba1425ecdedc72c9bdd732a54e66d7d20a3b6`.
The reviewed source manifest, package manifest and ZIP identify different things.
Neither raw working-byte SHA-256 nor Git blob IDs substitute for the other.

The ZIP used committed runtime inputs at e3d9bef998a20dff78dc67463dfb8f848aad76ce plus the then-uncommitted
reviewed packaging inputs. This task commits those inputs and associates the
already-built ZIP with that source checkpoint. The ZIP was NOT rebuilt from the
new implementation/evidence commits. No name, member, manifest, timestamp,
signature or embedded version was changed. The reviewer control build's exact
byte equality is scoped to its recorded environment, not universal reproducibility
or safety certification. Existing attributes govern Git CRLF/LF normalization;
per-input raw hashes and actual committed blobs are mapped in the external receipt
evidence without editing the original manifests or normalizing working files.

## Accepted behavior and retained qualifications

Explicit generation creates new ALPHA/BETA expendable sources. Separate native
inventories write new OFF/ON catalogs; .DS_Store remains default-OFF and narrowly
matches regular-file basenames. Retained catalogs open singly or as two fixed
inputs, with A/B/All Find/Library, deliberate reference and supported root
alignment, two-sided recorded evidence, stop/wait and reopen without regeneration.
Native IDs, supported exact names, durable OFF/ON/UNKNOWN scope and strict scope
keys are preserved. New scoped catalogs require the matching corrected reader;
the documented older reader refuses them. No stripping metadata, migration or
renumbering is accepted. Session labels are not durable storage identities or
independent-copy proof. Recorded differences/absence do not establish current
source state, physical loss, synchronization, recent verification or backup health.

Runtime prerequisites remain Windows x64, Node 24 x64, PowerShell 5.1 and a browser.
The packaged tutorial needs no Go/Git/checkout; the full native binary is not
confined by a security sandbox. No production scanning, durable registry/history,
live verification, backup/restore qualification, media operation or expanded
schema/limits is accepted. No binary upload, release/tag/signing or main merge.

Prompt20 remains **DEFERRED_BY_OWNER**; second/clean-machine qualification is
pending, not failed. Broader platforms, consoles, downloaded-file/security-policy
handling and public signing/distribution remain separate. Historical alert
classification is unresolved: a reported negative scan did not classify it.
Protections remain enabled; this checkpoint is not a security investigation.

## Evidence attribution and checkpoint checks

Historical AUTHOR Prompt28: 10 package tests, 1 boundary test, nine browser
sessions, split/real-console stop, comparison with 22 source locks, induced reader
failure and reopen. Two supplemental harness failures were corrected and retained.
Historical REVIEWER Prompt29: 10 package tests, 1 boundary test, nine browser
sessions, verified exclusive locks, failure/reopen and real-console controls;
separate offline rebuild byte-identical. The optional empty-EOF probe had an
incorrect harness expectation and required owned-process cleanup. That diagnostic
remains recorded; the supported EOF-with-stop-token control passed. These were
same-conversation AI-assisted passes with author context, not independent humans.
No counts are combined into inflated unique coverage or claimed as fresh tests.

This CHECKPOINT performs identity/hash, cumulative-diff, explicit staged-path/blob,
attribute-normalization, whitespace/scope, commit-parent/tree and remote checks.
No package launch, browser session, inventory run or build was performed. No active
local hooks were configured; ordinary existing push CI is separate from verified
source publication and is not presumed passed. Release automation is v-tag-only;
this sole branch push does not request tags or release assets.

Implementation allowlist: ten reviewed scripts/windows-alpha paths plus the GUI
README (11 paths). Evidence allowlist: RELEASE_CHECKLIST.md; the four living
records; historical implementation and review reports; this acceptance record
(8 paths). Raw artifacts, screenshots/logs, generated inputs, caches, designs,
credentials and unrelated files are excluded. The original 17-path author set and
added review report are reconciled; package inputs are not edited for acceptance.

Historical reports:
[implementation](WINDOWS-COMPARISON-ALPHA-PACKAGE-IMPLEMENTATION-2026-09-21.md) and
[review](WINDOWS-COMPARISON-ALPHA-PACKAGE-REVIEW-2026-09-21.md).

## Publication receipt and next activity

Authorized sole destination: https://github.com/nathansottung/obelisk.git,
refs/heads/feat/windows-comparison-alpha-package, normal non-force source push.
Live lookup before publication succeeded and found this branch absent. Actual
post-push local/tracking/live SHAs and the final evidence commit are recorded only
after verification in:
`C:\Users\nsott\AppData\Local\ObeliskDev\windows-comparison-publish-20260921-200052\publication-receipt.md`.
This record does not pre-claim a push result. The external receipt controls the
verified next-task base; neither ZIP is uploaded or replaced.

One suggested next activity after verified publication: owner dogfood of the
accepted new package with fresh generated workspaces, if not already done. No
owner session is claimed or started; no second-machine target or automatic next
feature is requested.
