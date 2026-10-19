# PR-04 slice-A F5 trust decision and implementation addendum - 2026-09-08

**READY_FOR_UNIX_VALIDATION.** The owner approved the trusted effective-filesystem-
principal model. The bounded implementation and Windows validation are complete;
Linux runtime acceptance is **NOT TESTED**. This is not an OB-004 closure or a
claim that macOS/BSD can currently perform the converted writes.

This addendum supersedes the decision blocker in the [prior response](PR04-OB-004-SLICE-A-REVIEW-RESPONSE-2026-09-08.md).
That response, the [original review](PR04-OB-004-SLICE-A-CODEX-REVIEW-2026-09-08.md),
the September 7 reports, and all previous F1-F7 tests remain byte-for-byte intact.

## Candidate and preservation

- Branch: `fix/ob-004-source-boundaries`.
- HEAD / PR-04 base: `0dc7d5399c6889e015aebc9ba02e694a909d6e9c`.
- Uncommitted working tree; empty index; no in-progress Git operation.
- Entry candidate: 497 tracked/untracked files copied and hashed before editing.
  Entry-manifest SHA-256:
  `4979e6c36155ba87105bf298e8f955849e689313b5e862df61af640193436fda`.
- Evidence directory:
  `C:\Users\NATHAN~1\AppData\Local\Temp\obelisk-pr04-trust-20260908-b172c90c16f240b4851948da7ab3f608`.
  `before/` and `entry-manifest.json` identify the incoming candidate;
  `candidate-manifest.json` identifies every final candidate file by path, size
  and SHA-256. `candidate.zip` contains those exact files under `candidate/` and
  the manifest at its top level. The final delivery reports their hashes.

Only destination authority, staging, its tests, one verification call in
`mirror.go`, the Linux validation script/CI step, and narrow reporting changed in
this follow-up. Compiler/dependency pins and all other incoming repairs remain.
No applicable AGENTS.md/CLAUDE.md was found. No Git mutation or publication ran.

## Approved boundary and the actual remaining attack

Processes with Obelisk's own effective filesystem authority are trusted not to
tamper with its private, operation-owned staging/publication area. Malicious
same-principal processes and privileged host administrators are excluded;
stronger isolation requires separately designed deployment work. This grants no
exception for ordinary application concurrency, source overlap, hostile names,
untrusted filesystem entries, hard links, cleanup or incorrect backup credit.

The original F5 defect reopened a closed staging name after exclusive creation
and later published by that mutable name. Retaining and verifying the original
file, applying metadata to it, and using Windows file-handle rename/disposition
repair that implementation defect. Windows holds a non-sharing file handle.

The remaining Unix attack is more specific than sharing a UID: the actor must
enter the operation's private `.TOKEN-SEQUENCE.obelisk-stage` directory, rename
`payload.mnemo_tmp` away, then create or move a substitute into that same private
entry after the final content/identity checks and before `renameat` uses it. The
characterization hook deliberately obtains the retained private root and does
exactly this. Mutating that operation-owned private entry crosses the approved
non-tampering boundary. Knowing the visible directory name alone is insufficient
under the admitted permissions; grants allowing other principals to replace the
publication parent or an ancestor are separately refused.

The previous ordered substitution, metadata/hard-link, and container-substitution
tests are unchanged. They show the behavior at their particular barriers, not
isolation. `TestDestAuth_F5SamePrincipalLateSubstitutionCharacterization` preserves
the late attack explicitly: its expected observation is successful hostile
publication. A passing characterization assertion is **not a security PASS**.
It has been compiled, **not executed on Linux here**. The original red probes
remain in the prior response's separate evidence directory.

F5 therefore combines **implementation repairs**, an **accepted threat-model
decision**, and **characterized behavior that is not defended against**.

## Enforced prerequisites

Windows admits local fixed/removable NTFS with persistent ACLs. Actual opened
directory DACLs and owners are checked through the publication parent and its
ancestors. Mutation grants to other principals, NULL DACLs, unknown ACE forms,
untrusted owners, impersonating threads, remote drives/shares, and unestablished
filesystem semantics are refused. Deny ACEs are not used to subtract suspicious
allow grants; refusal is intentionally conservative. Trusted owners/grantees are
the process user, SYSTEM, Administrators and privileged TrustedInstaller.

New Windows stage files receive a protected owner-only DACL, with exclusive
creation and a retained read/write/delete handle. Existing destination ACLs are
never changed to make them pass. Namespace inspection's absolute ancestor opens
are read-only; writes and publication remain relative to retained roots/handles.
[Microsoft documents handle-based security inspection](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-getsecurityinfo)
and [file security and rename behavior](https://learn.microsoft.com/en-us/windows/win32/fileio/file-security-and-access-rights).

Linux admits local ext2/3/4, XFS, Btrfs and tmpfs, checked with `fstatfs` on the
opened namespace. Every actual ancestor is walked by descriptor. Owners must
be the effective UID or root; publication parents cannot grant group/other
write. A writable ancestor is admitted only with sticky-directory protection
and trusted ancestor/child ownership, as for an ordinary root-owned `/tmp`.
On these filesystems Linux POSIX ACL named-user/group write grants are bounded
by the group-class mask checked here; this is not an inference from modes on an
arbitrary share. See the [Linux ACL access rules and mode correspondence](https://man7.org/linux/man-pages/man5/acl.5.html).

Linux private containers are exclusively created at 0700, never reused. Their
payloads start at 0600. Publication checks the owned file's UID, single link and
absence of group/other write. Windows and Linux refuse a nested publication
filesystem different from the leased destination volume. Neither implementation
recursively chmods/chowns existing data.

Unsupported: Windows ReFS/FAT/exFAT and unverified remote/share semantics;
Linux CIFS/NFS/FUSE/overlay and other unlisted filesystems anywhere in the checked
ancestor chain; untrusted writable publication namespaces; unknown ACL semantics.
**macOS/BSD converted writes now explicitly refuse** because native ACL/ancestor
validation is not implemented. Their ACLs cannot safely inherit the Linux mask
argument. This is a concrete platform prerequisite, not a passing runtime gate.
Other mutation callers were not converted or disabled.

## Publication, cleanup and cooperating jobs

The source-supplied digest is retained on the owned stage. Content disagreement,
an attempt to change that expectation, or an observed identity disagreement
refuses publication. Rechecking before publication does not isolate a writer
able to alter the file afterward. Payload and sidecar metadata use the retained
file; no delete-final fallback exists. Failed rename preserves the prior good
destination; a post-rename close failure withholds credit without deleting it.
Unknown staging replacements are not removed. Empty Unix private containers and
unknown entries remain for inspection, without recursive pathname cleanup.

Every converted job retains a cooperative lease until `DestRoot.Close`, after
its workers, verification, sidecar and catalog work. Windows serializes by local
volume across processes/sessions using a named mutex acquired/released on one
dedicated OS thread. Linux serializes converted jobs in the host mount namespace
using `flock` on the root inode, including disjoint volumes and bind aliases.
These intentionally conservative scopes prevent overlapping-root collisions;
a waiter refuses specifically after 30 seconds. Kernel close/process exit releases
the lease. See [Windows mutex ownership](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-releasemutex)
and [Linux flock semantics](https://man7.org/linux/man-pages/man2/flock.2.html).

The lease is application coordination, not isolation or media identity. Existing
unconverted mutation callers remain outside this boundary. Independent stages
within a job use 128-bit operation tokens, sequence numbers and exclusive creation;
they never adopt an arbitrary existing file/container. `Close` follows worker
completion, as in the converted workflows; borrowed staging files do not outlive
the owning job. A later authorized job can replace a completed backup; this does
not establish permanent copy availability or bind catalog volume IDs to media.

## F1-F7 disposition

| Finding | Current disposition |
| --- | --- |
| F1 root overlap | Root-aware overlap repair preserved; Windows positive/negative regressions pass; Linux runtime open. |
| F2 whitespace identity/credit | Exact-name repair preserved; Windows mirror/incremental regressions pass; Unix trailing-space case runtime open. |
| F3 absent source under alias | Canonical existing-ancestor repair preserved; Windows junction/missing-components checks pass; Linux symlink case runtime open. |
| F4 Unix bootstrap write | Per-component authorization/refusal preserved; Linux dangling/substituted bootstrap tests compiled, NOT TESTED at runtime. |
| F5 ownership/publication | Implementation + approved scope decision as above; Windows checks pass; Linux contract and characterization execution open; macOS/BSD explicitly unsupported. |
| F6 legacy source-open ordering | Repair and stronger hard-link preservation test unchanged; Windows PASS; Linux runtime included in the gate. |
| F7 portable drive-qualified names | Host-independent rejection/test alignment preserved; Windows PASS; Linux runtime open. |

## Actual evidence and limits

Host: Windows NT 10.0.26200.0, `go version go1.26.8 windows/amd64`, NTFS verified by
the production handle-based policy on the fixtures. Compiler selection was
process-scoped with `GOPROXY=off`. Existing x/sys v0.44.0 remains unchanged.

- Current Windows targeted command:
  `go test -run '^(TestDestAuth_|TestMirror_|TestIncremental_|TestAtomicRename_)' -count=1 -json ./...`.
  **47 top-level passes / 2 skips**, including the no-op parent invocation of the
  subprocess helper (the cooperating-process test also executes it as a child).
  Final-candidate log: `windows-final-candidate.jsonl` in the evidence directory.
- Skips: `TestDestAuth_SymlinkAliasOfSourceRefused` and
  `TestDestAuth_NestedSymlinkLeafRefused`; Windows could not create the symlinks.
  Junction tests are separate and passed.
- New Windows tests cover protected stage ownership, actual untrusted publication
  and ancestor ACL grants, unsupported security descriptors, expected-content
  mismatch/prior preservation, eight independent stages, and OS-confirmed
  contention between jobs and separate processes. Permission-configuration tests
  do **not** claim execution as another user. No new account or impersonation ran.
- Windows build/vet PASS. Linux amd64, macOS arm64, FreeBSD amd64 production
  build, test compilation and vet PASS; module verification PASS. Formatting,
  whitespace and Git Bash syntax-only validation of the Linux script PASS.
- The first default-temp run was correctly refused: sandbox principals have
  mutation ACL grants in that ancestor chain. A newly created owner-only fixture
  directory under the user profile supplied the positive controls. No existing
  permissions changed. Early compile mistakes and a test-fixture ACL inheritance
  error were corrected before the recorded final run; earlier logs are retained.
- Go cache sandbox access initially failed before compilation; tool-level cache
  access was approved. This is not administrator elevation or a safety PASS.
- Historical results remain **39 targeted passes / 2 skips** and **266 full-suite
  passes / 7 known Unicode failures / 6 skips**. The full suite was not rerun to
  duplicate those figures. No claim is made that the Unicode failures changed.
- Linux/macOS/BSD runtime, Windows race, CI and Docker execution: **NOT TESTED**.
  Docker/podman are absent; no authorized disposable Unix runtime was supplied.
  WSL/SSH executable presence is not a configured or authorized test environment;
  no distro, remote host, production storage or real NAS was selected.

## One next action: execute the exact candidate's Linux gate

The [validation script](../../../scripts/validate-pr04-slice-a-linux.sh) enumerates
all **46 exact safety test names before constructing its selector**, checks the
runtime test inventory against that enumeration, and refuses a gate with skips.
It covers staging/ownership, bootstrap F4, portable-name F7, missing source aliases,
symlinks, hard links, source/prior preservation, cleanup, mismatches and cooperating
jobs/processes. The late same-principal characterization is a separate invocation
explicitly labeled as an attack that is not defended against.

Required environment: an already authorized disposable Linux host in one host
mount namespace, existing Go 1.26.8, `/proc`, and private fixture storage whose
entire ancestor chain satisfies the admitted local-filesystem policy. Run as the
ordinary test user; do not create accounts, install tools, mount storage or elevate.
From the transferred `candidate/` directory, run:

```bash
export TMPDIR="$(mktemp -d)"
bash scripts/validate-pr04-slice-a-linux.sh
```

The script records actual compiler, OS, user, filesystem and fixture modes when it
runs. If a C compiler is already available and authorized, set `PR04_RACE=1` for
the additional race check. The minimal existing Linux CI step now calls this
script with its existing compiler pin and available C toolchain. That workflow
has **not** run.

Deliver `candidate.zip` and verify its reported SHA-256 and per-file manifest in
the disposable environment before executing. It includes untracked source/tests;
HEAD alone does not identify or deliver this patch. No commit, push, upload or
workflow dispatch has occurred, and GitHub does not contain this uncommitted
candidate by implication. Preserve runtime logs for the subsequent focused recheck.
A Linux pass cannot validate macOS/BSD ACL code that is still absent. Do not begin
slice B or mark the thirteen deferred mutation callers/all OB-004 resolved.
