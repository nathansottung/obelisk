package main

// destauth.go — destination authority.
//
// The invariant this file exists to keep: Obelisk NEVER writes into registered
// source data. Until now that was one boolean — AssertOutsideSources — asked once
// at function entry about a PATHNAME STRING, after which every leaf write rebuilt
// an absolute path and reopened it by name. Three different questions were riding
// on that single answer:
//
//  1. Is the destination object registered source data, inside it, or around it?
//  2. Does every leaf operation land beneath the destination we decided about?
//  3. Is this really the medium the operator selected?
//
// This file separates (1) and (2) and deliberately does NOT answer (3).
//
//   - AUTHORIZATION — Store.AuthorizeDestination opens the destination, derives
//     its canonical name FROM THE OPEN HANDLE, decides against a snapshot of the
//     registered sources, and returns the handle it decided about. The pathname
//     is never re-resolved afterwards. Deciding on the handle is what makes an
//     alias (junction, 8.3 short name, extended-length spelling) get the same
//     verdict as the real path: they all canonicalise to the same object.
//
//   - CONFINEMENT — every subsequent operation is relative to that retained
//     handle, via os.Root or the platform descriptor-relative publication API.
//     No absolute pathname is reopened after the decision, including staging
//     paths, read-backs, sidecars and failure cleanup. A stored canonical
//     pathname is NOT a capability; the open handle is.
//
// MEDIA IDENTITY IS NOT ESTABLISHED HERE. A retained handle proves the writes go
// to ONE SPECIFIC DIRECTORY OBJECT. It does not prove that object is the volume,
// session or removable medium the operator chose.
//
// SCOPE. Only MirrorToVolume and mirror-mode BackupChanges (with their shared leaf
// copy and inventory sidecar) use this boundary so far. Every other writer still
// uses AssertOutsideSources.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// errRelEscape marks a destination-relative path that is not a plain, confined
// relative path. It is returned BEFORE the path reaches the os.Root, so a bad
// catalog RelPath, folder label or staged entry name is rejected at one place
// rather than being handed to the filesystem to adjudicate.
var errRelEscape = errors.New("destination-relative path is not confined")

// destRel validates a destination-relative path. It must be a non-empty, slash-
// separated, relative path with no empty, "." or ".." element, no leading slash
// and no drive letter or UNC volume. Catalog RelPath values, mirrorFolderLabels
// output and sidecar component names all pass through here. os.Root would refuse
// an escaping path anyway; validating first gives one clear application error.
func destRel(rel string) (string, error) {
	r := rel
	if r == "" || strings.TrimSpace(r) == "" {
		return "", fmt.Errorf("%w: empty path", errRelEscape)
	}
	r = filepath.ToSlash(r)
	// Catalog names are not user-entered root configuration. Never trim them.
	// Drive qualification is rejected on every host, including Unix.
	if len(r) >= 2 && ((r[0] >= 'A' && r[0] <= 'Z') || (r[0] >= 'a' && r[0] <= 'z')) && r[1] == ':' {
		return "", fmt.Errorf("%w: %q is drive-qualified", errRelEscape, rel)
	}
	if err := platformDestName(r); err != nil {
		return "", fmt.Errorf("%w: %q: %w", errRelEscape, rel, err)
	}
	if v := filepath.VolumeName(filepath.FromSlash(r)); v != "" {
		return "", fmt.Errorf("%w: %q names volume %q", errRelEscape, rel, v)
	}
	if strings.HasPrefix(r, "/") {
		return "", fmt.Errorf("%w: %q is rooted", errRelEscape, rel)
	}
	for _, el := range strings.Split(r, "/") {
		switch el {
		case "":
			return "", fmt.Errorf("%w: %q has an empty path element", errRelEscape, rel)
		case ".", "..":
			return "", fmt.Errorf("%w: %q contains a %q element", errRelEscape, rel, el)
		}
	}
	return r, nil
}

// ---- volume identity ----------------------------------------------------------

// pathVolumeID is the recorded identity of the volume holding path, or "" when it
// cannot be read (the path is absent, unreadable, or the platform records none).
func pathVolumeID(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	id, err := volumeIDOf(f)
	if err != nil {
		return ""
	}
	return id
}

// ---- the registered-source snapshot -----------------------------------------

// sourceRef is one registered source root as it was resolved at authorization
// time. canonical is handle-derived whenever the root exists on disk, so an alias
// of a source compares equal to the source itself.
//
// A source whose drive is not attached has no canonical name. It is kept as
// offline, with the volume identity it was recorded on (if any) and the absolute
// path it was registered at, and decided by decideOffline instead.
type sourceRef struct {
	registered string // exactly as registered in the catalog
	canonical  string // opened root, or opened ancestor plus the absent descendant
	fromHandle bool
	offline    bool   // the source's volume is not where the source was registered
	volumeID   string // recorded volume identity, for offline sources
	abs        string // absolute registered path, for offline sources
}

// sourceSnapshot is the set of registered sources an authorization decided
// against, captured ONCE.
//
// SYNCHRONISATION: Store.sourceFolders takes the general Store mutex and releases
// it before returning. The mutex is NOT held for the duration of the job — a
// mirror can run for hours and needs the same mutex to record its results.
//
// RESIDUAL: a source registered AFTER authorization, or protected data moved INTO
// the authorized tree afterwards, is not covered by this snapshot. The retained
// handle still confines every write to the directory that was authorized; it does
// not re-decide whether that directory has since become protected.
type sourceSnapshot struct {
	at   time.Time
	srcs []sourceRef
}

// sourceSnapshot resolves every registered source root to a canonical name,
// preferring the identity of the OPEN OBJECT over the spelling of its name.
//
// A root that cannot be opened for a reason other than "does not exist" is a
// refusal: authorizing a destination while a source location is unresolved would
// be authorizing on a guess. A root that does not exist is one of three things:
//
//   - its folder was removed but its volume is still there: resolve the existing
//     ancestor and protect the path the source would occupy;
//   - its volume is not attached (no ancestor exists, or a different volume now
//     answers at that path): it cannot hold the destination, so it is offline and
//     only blocks a destination on the volume it was recorded on;
//   - its volume identity was never recorded and nothing answers at its path: it
//     is offline and blocks only a destination whose path could be its path.
func (s *Store) sourceSnapshot() (*sourceSnapshot, error) {
	folders := s.sourceFolders() // takes and RELEASES the Store mutex
	snap := &sourceSnapshot{at: time.Now().UTC()}
	for _, folder := range folders {
		r := folder.Path
		abs, err := filepath.Abs(r)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve registered source %s: %w", r, err)
		}
		f, oerr := os.Open(abs)
		if oerr == nil {
			c, cerr := handleCanonicalPath(f)
			vid, verr := volumeIDOf(f)
			f.Close()
			if cerr != nil {
				return nil, fmt.Errorf("cannot establish the real location of registered source root %s: %w — refusing to authorize a destination while a protected source is unresolved", r, cerr)
			}
			snap.srcs = append(snap.srcs, sourceRef{registered: r, canonical: c, fromHandle: true})
			switch {
			case verr != nil || vid == "":
			case folder.VolumeID == "":
				s.recordFolderVolume(folder.ID, vid)
			case vid != folder.VolumeID:
				// The path now answers on a different drive than the one the source
				// was recorded on. That drive may be attached elsewhere, still holding
				// the source: protect it too.
				snap.srcs = append(snap.srcs, sourceRef{registered: r, offline: true, volumeID: folder.VolumeID, abs: abs})
			}
			continue
		}
		if !errors.Is(oerr, fs.ErrNotExist) {
			return nil, fmt.Errorf("cannot open registered source root %s to establish its real location: %w — refusing to authorize a destination on a guess", r, oerr)
		}
		offline := sourceRef{registered: r, offline: true, volumeID: folder.VolumeID, abs: abs}
		ancestor, missing, err := deepestExistingDir(abs)
		if errors.Is(err, errNoExistingParent) {
			snap.srcs = append(snap.srcs, offline)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("registered source %s is unresolved: %w; restore access to its parent or repair the alias before retrying", r, err)
		}
		if folder.VolumeID != "" {
			if vid := pathVolumeID(ancestor); vid != "" && vid != folder.VolumeID {
				snap.srcs = append(snap.srcs, offline)
				continue
			}
		}
		parent, err := os.OpenRoot(ancestor)
		if err != nil {
			return nil, fmt.Errorf("cannot open ancestor of registered source %s: %w", r, err)
		}
		canon, err := rootCanonicalPath(parent)
		parent.Close()
		if err != nil {
			return nil, fmt.Errorf("cannot locate ancestor of registered source %s: %w", r, err)
		}
		canon = filepath.Join(append([]string{canon}, missing...)...)
		snap.srcs = append(snap.srcs, sourceRef{registered: r, canonical: canon})
	}
	return snap, nil
}

// decide answers the source-overlap question in BOTH directions plus equality,
// over canonical names, at path-segment boundaries.
//
//	equal    — the destination IS a registered source
//	inside   — the destination is beneath a registered source
//	ancestor — the destination CONTAINS a registered source
//
// The ancestor direction is the one AssertOutsideSources omits, and it needs no
// links at all to republish protected data over itself: mirror two registered
// folders to their common parent and the per-folder subtree labels reconstruct the
// source paths exactly. Offline sources are skipped here; see decideOffline.
func (snap *sourceSnapshot) decide(destCanonical, display string) error {
	d := normPath(destCanonical)
	if d == "" {
		return fmt.Errorf("refusing: could not establish the real location of destination %s", display)
	}
	for _, s := range snap.srcs {
		if s.offline {
			continue
		}
		if err := overlapError(d, normPath(s.canonical), display, destCanonical, s.registered); err != nil {
			return err
		}
	}
	return nil
}

// decideOffline applies the offline sources to a destination on the volume named
// destVolumeID (empty when that is unknown), whose path as requested is requested
// and as resolved is canonical.
//
// A source recorded on a volume blocks only a destination on that same volume: the
// volume is attached somewhere other than where the source was registered, and
// the destination may be inside it. When either identity is unknown the decision
// falls back to paths: refuse when the destination's path could be the source's.
func (snap *sourceSnapshot) decideOffline(destVolumeID, requested, canonical, display string) error {
	for _, s := range snap.srcs {
		if !s.offline {
			continue
		}
		if s.volumeID != "" && destVolumeID != "" {
			if s.volumeID == destVolumeID {
				return fmt.Errorf("refusing: the drive that holds registered source %s is attached at a different location, and %s is on that drive. Reattach it where the source was registered, or choose a destination on another drive", s.registered, display)
			}
			continue
		}
		src := normPath(s.abs)
		for _, p := range []string{requested, canonical} {
			if err := overlapError(normPath(p), src, display, p, s.registered+" (its drive is not attached)"); err != nil {
				return err
			}
		}
		if s.volumeID == "" {
			if err := sameFolderOtherLetter(s, canonical, display); err != nil {
				return err
			}
		}
	}
	return nil
}

// sameFolderOtherLetter covers a source registered before volume identities were
// recorded whose drive came back under a different letter. If the destination's
// drive holds a folder at the source's place (its path without the drive letter),
// that folder may be the source itself, and the destination must not be it,
// inside it, or around it.
//
// A source registered as a whole drive (E:\) has no place to look for: every
// drive has a root. It is left to the path rule rather than refusing every
// destination on every drive. The candidate folder is compared by the location
// read from its handle, so a short (8.3) or differently cased spelling of the
// source's path still matches.
func sameFolderOtherLetter(s sourceRef, canonical, display string) error {
	srcVol := filepath.VolumeName(s.abs)
	destVol := filepath.VolumeName(canonical)
	if !isDriveLetter(srcVol) || !isDriveLetter(destVol) || strings.EqualFold(srcVol, destVol) {
		return nil // only a drive letter can change; a UNC share is not mapped onto one
	}
	rel := s.abs[len(srcVol):]
	if strings.Trim(rel, `\/`) == "" {
		return nil
	}
	f, err := os.Open(destVol + rel)
	if err != nil {
		return nil
	}
	candidate, cerr := handleCanonicalPath(f)
	st, serr := f.Stat()
	f.Close()
	if cerr != nil || serr != nil || !st.IsDir() {
		return nil
	}
	if err := overlapError(normPath(canonical), normPath(candidate), display, canonical, candidate); err != nil {
		return fmt.Errorf("refusing: registered source %s is not attached under its letter, and %s is at the same place on drive %s, so it may be the same drive under a new letter. %w", s.registered, candidate, destVol, err)
	}
	return nil
}

// overlapError reports whether destination d and source r (both normPath keys)
// are the same folder or one contains the other.
func overlapError(d, r, display, destName, registered string) error {
	if r == "" {
		return fmt.Errorf("refusing: registered source %s has an unresolved location", registered)
	}
	switch {
	case d == r:
		return fmt.Errorf("refusing: %s is registered source root %s (both name the same folder: %s); Obelisk never writes into source data", display, registered, destName)
	case canonicalWithin(r, d):
		return fmt.Errorf("refusing: %s is inside source root %s (it resolves to %s); Obelisk never writes into source data", display, registered, destName)
	case canonicalWithin(d, r):
		return fmt.Errorf("refusing: %s (it resolves to %s) CONTAINS registered source root %s. Writing here would copy protected originals back over themselves. Pick an output folder that holds no registered source — a dedicated subfolder, or a different drive", display, destName, registered)
	}
	return nil
}

// filepath.Rel understands filesystem roots and volumes; a separator appended
// to a root spelling does not. Inputs are comparison keys, never stored names.
func canonicalWithin(parent, child string) bool {
	rel, err := filepath.Rel(filepath.FromSlash(parent), filepath.FromSlash(child))
	return err == nil && rel != ".." && !filepath.IsAbs(rel) &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ---- the destination filesystem ------------------------------------------------

// destFS is what the platform policy learned about the destination's filesystem:
// its name, the largest file it can hold (0 when there is no practical limit),
// the checks that could not run there, the findings the user must decide about,
// and each folder level's permissions from the destination up to the volume root
// (for the choice fingerprint).
type destFS struct {
	name       string
	maxFile    int64
	notChecked []string
	findings   []destFinding
	chain      []destLevel
}

// destLevel is one folder on the way from a destination up to its volume root, as
// the fingerprint of a recorded choice sees it.
type destLevel struct {
	path  string
	fs    string
	perms string // owner and permissions as the platform reports them
}

// Checks that cannot run on FAT and exFAT, as listed in notices and results.
const (
	destCheckPermissions = "folder permissions"
	destCheckHardLinks   = "hard links"
	destCheckFileIDs     = "stable file IDs"
)

// Kinds of destination finding.
const (
	// Other accounts on this PC can change the folder, through a group Windows
	// grants by default (Authenticated Users or Users, inherited).
	destFindingSharedGroup = "shared-group"
	// The folder is a drive root with Windows' default "create folders" grant.
	destFindingDriveRoot = "drive-root"
	// The filesystem cannot record permissions, hard links or stable file IDs.
	destFindingUnchecked = "unchecked"
)

// destFinding is one thing about a destination that is acceptable only once the
// user has chosen what to do about it.
type destFinding struct {
	Kind      string `json:"kind"`
	Folder    string `json:"folder"`
	Group     string `json:"group,omitempty"`
	SID       string `json:"sid,omitempty"`
	Inherited bool   `json:"inherited,omitempty"`
	// Permissions: the group can also change who may use the folder (change its
	// permissions or take ownership), not only its contents.
	Permissions bool   `json:"can_change_permissions,omitempty"`
	FS          string `json:"filesystem,omitempty"`
	MaxFile     int64  `json:"max_file,omitempty"`
}

// appendFinding adds f unless an equal finding is already listed.
func appendFinding(list []destFinding, f destFinding) []destFinding {
	for _, g := range list {
		if g == f {
			return list
		}
	}
	return append(list, f)
}

// ---- the authorized destination ---------------------------------------------

// DestRoot is a destination directory that has been authorized, together with the
// open handle the decision was made about. Every write goes through this handle.
//
// WHAT IT GUARANTEES: every operation lands beneath the directory object that was
// authorized — not beneath whatever its pathname resolves to later. A junction,
// symlink or reparse point BELOW the root cannot redirect a leaf operation out.
//
// WHAT IT DOES NOT GUARANTEE: that the object is the intended medium (see the
// file header); confinement of EXTERNAL HELPER PROCESSES, which receive pathname
// operands and are not constrained by a handle this process holds; or anything
// under privileged topology changes (bind mounts, device or drive-letter
// substitution), which are outside the threat model.
type DestRoot struct {
	root      *os.Root
	canonical string // handle-derived at authorization; never re-resolved
	display   string // the operator's spelling, for messages and catalog records
	snap      *sourceSnapshot
	fs        destFS
	accepted  map[string]bool // findings the destination was accepted with (see findingKey)
	token     string          // collision avoidance; object ownership is retained separately
	release   func() error    // cooperative lease retained through the caller's catalog credit
	volume    uint64
	seq       atomic.Uint64
	mu        sync.Mutex
	stages    map[string]*destStage
	closeOnce sync.Once
	closeErr  error
}

// AuthorizeDestination resolves path to a real directory, opens it, decides
// against the registered sources using the identity of THE OPENED OBJECT, and
// returns the handle that was decided about. The caller must Close it.
//
// Order is the whole point. Open first, then name: the canonical name comes from
// the open handle, so the verdict is about the object being held rather than about
// a string that may name something else by the time it is used.
//
// A destination with findings (other accounts can change it, or its filesystem
// cannot record permissions) is refused with a *DestinationChoiceError until the
// user has made a choice that still matches the folder (see destchoice.go). That
// decision is taken before anything is created, so nothing is written first.
//
// BOOTSTRAP creates one absent component at a time. A single-component Mkdir
// does not traverse a terminal link and an existing entry is a refusal. Before
// any next creation the newly opened parent's actual location is authorized.
func (s *Store) AuthorizeDestination(path string) (*DestRoot, error) {
	in, err := s.inspectDestination(path)
	if err != nil {
		return nil, err
	}
	if len(in.findings) > 0 && in.choice == nil {
		in.anchor.Close()
		return nil, &DestinationChoiceError{Check: s.checkFromInspection(in)}
	}
	display, snap, ancCanon := in.display, in.snap, in.anchorCanon

	root := in.anchor
	var created []uint64
	for _, component := range in.missing {
		if _, err := destRel(component); err != nil {
			root.Close()
			return nil, err
		}
		// Only the first parent may contain sources: creation of this single
		// absent, disjoint child is authorized, not its sibling subtrees.
		if err := snap.decide(filepath.Join(ancCanon, component), display); err != nil {
			root.Close()
			return nil, err
		}
		if _, err := checkDestNamespace(root); err != nil {
			root.Close()
			return nil, err
		}
		if err := bootstrapMkdir(root, component); err != nil {
			root.Close()
			return nil, fmt.Errorf("cannot safely create destination component %q: %w; inspect the destination and retry", component, err)
		}
		sub, serr := root.OpenRoot(component)
		root.Close()
		if serr != nil {
			return nil, fmt.Errorf("cannot open destination component %q: %w", component, serr)
		}
		root = sub
		if id, err := rootIdentity(root); err == nil {
			created = append(created, id.idx)
		}
		ancCanon, err = rootCanonicalPath(root)
		if err == nil {
			err = snap.decide(ancCanon, display)
		}
		if err != nil {
			root.Close()
			return nil, err
		}
	}
	if in.choice != nil && len(created) > 0 {
		s.recordCreatedUnderChoice(in.choice, created)
	}

	// Re-derive the canonical name from the handle actually retained, and take the
	// verdict again on it. This closes the gap between "the name we intended" and
	// "the object we are holding".
	canon, err := rootCanonicalPath(root)
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("cannot establish the real location of %s: %w — refusing rather than writing on a guess", display, err)
	}
	if err := snap.decide(canon, display); err != nil {
		root.Close()
		return nil, err
	}
	if err := snap.decideOffline(rootVolumeID(root), in.abs, canon, display); err != nil {
		root.Close()
		return nil, err
	}

	destfs, err := checkDestNamespace(root)
	if err != nil {
		root.Close()
		return nil, err
	}
	release, err := acquireDestLease(root)
	if err != nil {
		root.Close()
		return nil, err
	}
	id, err := rootIdentity(root)
	if err != nil {
		release()
		root.Close()
		return nil, err
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		release()
		root.Close()
		return nil, fmt.Errorf("cannot generate a staging token: %w", err)
	}
	// Only findings on the destination itself, or what folders created inside it
	// inherit, may cover folders inside it. A grant accepted on a folder above the
	// destination says nothing about a shared folder later moved into it.
	accepted := map[string]bool{}
	for _, f := range in.findings {
		if atOrBelow(f.Folder, in.intended) {
			accepted[findingKey(f)] = true
		}
	}
	for _, f := range destfs.findings {
		if atOrBelow(f.Folder, canon) {
			accepted[findingKey(f)] = true
		}
	}
	return &DestRoot{root: root, canonical: canon, display: display, snap: snap, fs: destfs, accepted: accepted,
		token: hex.EncodeToString(tok), release: release, volume: id.vol}, nil
}

// findingKey groups findings the same decision covers: a grant of the same
// strength to the same group, wherever below the destination it appears, or the
// destination's filesystem. A grant that can also change permissions is a
// different, stronger finding than one that can only change files.
func findingKey(f destFinding) string {
	if f.Kind == destFindingUnchecked {
		return f.Kind
	}
	if f.Kind == destFindingDriveRoot { // others may only create folders: weaker than Modify
		return "root:" + f.SID
	}
	if f.Permissions {
		return "sid:" + f.SID + ":permissions"
	}
	return "sid:" + f.SID
}

// isDriveLetter reports whether vol is a drive-letter volume name such as "E:".
func isDriveLetter(vol string) bool { return len(vol) == 2 && vol[1] == ':' }

// atOrBelow reports whether folder is root or inside it.
func atOrBelow(folder, root string) bool {
	f, r := normPath(folder), normPath(root)
	return f == r || canonicalWithin(r, f)
}

// destInspection is everything decided about a destination before anything is
// created: the deepest existing folder (held open), the components still to
// create, the findings for the folder that will be written into, and the recorded
// choice that covers them, if any.
type destInspection struct {
	display     string
	abs         string
	snap        *sourceSnapshot
	anchor      *os.Root // deepest existing folder; caller closes
	anchorCanon string
	anchorID    fileIdent
	missing     []string
	intended    string
	fs          destFS
	findings    []destFinding
	fingerprint string
	choice      *DestinationChoice // a valid choice covering the findings, or nil
	stale       *DestinationChoice // a choice for this folder that no longer matches
}

// inspectDestination runs every check AuthorizeDestination makes before creating
// anything, and writes nothing: the source overlap decision, the platform policy
// on the deepest existing folder and its ancestors, and what a folder created
// there would inherit. A refusal is returned as an error.
func (s *Store) inspectDestination(path string) (*destInspection, error) {
	display := strings.TrimSpace(path)
	if display == "" {
		return nil, fmt.Errorf("a destination folder is required")
	}
	abs, err := filepath.Abs(display)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve destination %s: %w", display, err)
	}
	abs = filepath.Clean(abs)

	snap, err := s.sourceSnapshot()
	if err != nil {
		return nil, err
	}
	existing, missing, err := deepestExistingDir(abs)
	if err != nil {
		return nil, err
	}
	anc, err := os.OpenRoot(existing)
	if err != nil {
		return nil, fmt.Errorf("cannot open destination folder %s: %w", existing, err)
	}
	ancCanon, err := rootCanonicalPath(anc)
	if err != nil {
		anc.Close()
		return nil, fmt.Errorf("cannot establish the real location of %s: %w — refusing rather than writing on a guess", existing, err)
	}
	// Preflight the intended path; this is not permission to traverse links in
	// the missing suffix. The component creation loop enforces that rule.
	intended := ancCanon
	if len(missing) > 0 {
		intended = filepath.Join(append([]string{ancCanon}, missing...)...)
	}
	fail := func(err error) (*destInspection, error) {
		anc.Close()
		return nil, err
	}
	if err := snap.decide(intended, display); err != nil {
		return fail(err)
	}
	if err := snap.decideOffline(rootVolumeID(anc), abs, intended, display); err != nil {
		return fail(err)
	}
	nsfs, err := checkDestNamespace(anc)
	if err != nil {
		return fail(err)
	}
	findings := nsfs.findings
	if len(missing) == 0 {
		// Folders Obelisk creates inside an existing destination inherit from it;
		// a grant they would inherit (even one that is inherit-only on the folder
		// itself) is a finding about this folder.
		pred, err := predictDestChild(anc, ancCanon, 1)
		if err != nil {
			return fail(err)
		}
		for _, f := range pred {
			findings = appendFinding(findings, f)
		}
	} else {
		// The folder written into does not exist yet: its filesystem findings are
		// the anchor's, named for the folder that will be created, plus whatever it
		// would inherit.
		for i := range findings {
			if findings[i].Kind == destFindingUnchecked {
				findings[i].Folder = intended
			}
		}
		first := filepath.Join(ancCanon, missing[0])
		pred, err := predictDestChild(anc, first, len(missing))
		if err != nil {
			return fail(err)
		}
		for _, f := range pred {
			// Every folder created down to the destination inherits the same grant;
			// name it for the destination the user chose.
			f.Folder = intended
			findings = appendFinding(findings, f)
		}
		if len(missing) > 1 {
			// A no-propagate grant reaches only the first new folder. It is still
			// something other accounts can change, so it is shown, named for that
			// folder; being above the destination, it covers nothing inside it.
			firstOnly, err := predictDestChild(anc, first, 1)
			if err != nil {
				return fail(err)
			}
			for _, f := range firstOnly {
				findings = appendFinding(findings, f)
			}
		}
	}
	id, err := rootIdentity(anc)
	if err != nil {
		return fail(err)
	}
	in := &destInspection{display: display, abs: abs, snap: snap, anchor: anc, anchorCanon: ancCanon, anchorID: id,
		missing: missing, intended: intended, fs: nsfs, findings: findings, fingerprint: destFingerprint(nsfs.chain)}
	if len(findings) > 0 {
		in.choice, in.stale = s.coveringChoice(in)
	}
	return in, nil
}

// rootIdentity is the identity of the directory r holds, read from a handle.
func rootIdentity(r *os.Root) (fileIdent, error) {
	f, err := r.Open(".")
	if err != nil {
		return fileIdent{}, err
	}
	defer f.Close()
	return handleIdentity(f)
}

// rootVolumeID is the volume identity of the directory r holds, or "".
func rootVolumeID(r *os.Root) string {
	f, err := r.Open(".")
	if err != nil {
		return ""
	}
	defer f.Close()
	id, err := volumeIDOf(f)
	if err != nil {
		return ""
	}
	return id
}

// Test seam immediately before the actual single-component creation.
var bootstrapMkdir = func(r *os.Root, component string) error { return r.Mkdir(component, 0o755) }

// errNoExistingParent means nothing exists anywhere along a path, not even its
// volume root — for a drive letter, the drive is not attached.
var errNoExistingParent = errors.New("no existing parent folder")

// deepestExistingDir splits abs into the deepest existing directory and the
// components below it that do not exist yet.
//
// This walk uses pathname lookups, and that is fine: it only chooses WHERE TO
// START. Nothing is authorized by it — the ancestor it picks is then opened and
// judged on its handle-derived identity, and the leaf is judged again on its own.
func deepestExistingDir(abs string) (string, []string, error) {
	var missing []string
	p := abs
	for {
		st, err := os.Lstat(p)
		if err == nil {
			// Lstat distinguishes an existing dangling entry from absence. Stat
			// then follows both Unix symlinks and Windows junction/reparse forms
			// (which need not report ModeSymlink).
			st, err = os.Stat(p)
			if err != nil {
				return "", nil, fmt.Errorf("cannot resolve existing entry %s: %w", p, err)
			}
		}
		if err == nil {
			if !st.IsDir() {
				return "", nil, fmt.Errorf("destination %s exists and is not a folder", p)
			}
			return p, missing, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", nil, fmt.Errorf("cannot inspect %s: %w", p, err)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", nil, fmt.Errorf("%w for %s", errNoExistingParent, abs)
		}
		base := filepath.Base(p)
		if base == "" || base == "." || base == ".." {
			return "", nil, fmt.Errorf("cannot create destination %s: unusable path component %q", abs, base)
		}
		missing = append([]string{base}, missing...)
		p = parent
	}
}

// rootCanonicalPath derives an os.Root's canonical location from the handle it
// holds, by opening "." within it and asking the platform where that object
// actually lives. Root.Name() is the string the root was opened WITH, which is
// exactly what must not be trusted here.
func rootCanonicalPath(r *os.Root) (string, error) {
	f, err := r.Open(".")
	if err != nil {
		return "", err
	}
	defer f.Close()
	return handleCanonicalPath(f)
}

// Name is the operator's spelling of the destination — for messages, log lines,
// the Performance strip and catalog records. It is NOT a capability and is never
// reopened for a mutation.
func (d *DestRoot) Name() string { return d.display }

// Canonical is the handle-derived location the authorization decided about.
// Also not a capability: read-only queries (free space) may use it; writes go
// through the handle.
func (d *DestRoot) Canonical() string { return d.canonical }

// Filesystem is the destination filesystem's name as the platform reports it.
func (d *DestRoot) Filesystem() string { return d.fs.name }

// MaxFileSize is the largest file the destination filesystem can hold, or 0 when
// it has no practical limit.
func (d *DestRoot) MaxFileSize() int64 { return d.fs.maxFile }

// Notes lists, for the job result, every warning the destination was accepted
// with and every check that could not run on its filesystem.
func (d *DestRoot) Notes() []string {
	var out []string
	for _, f := range d.fs.findings {
		switch f.Kind {
		case destFindingSharedGroup:
			out = append(out, fmt.Sprintf("%s can change %s (accepted by your choice)", f.Group, f.Folder))
		case destFindingDriveRoot:
			out = append(out, fmt.Sprintf("%s can create folders in the drive root %s (accepted by your choice)", f.Group, f.Folder))
		}
	}
	for _, c := range d.fs.notChecked {
		out = append(out, "not checked on this drive: "+c)
	}
	return out
}

// CheckFileSizes refuses, before anything is written, a job whose largest file
// will not fit on the destination filesystem.
func (d *DestRoot) CheckFileSizes(sizes []int64) error {
	if d.fs.maxFile <= 0 {
		return nil
	}
	n, largest := 0, int64(0)
	for _, s := range sizes {
		if s > d.fs.maxFile {
			n++
			largest = max(largest, s)
		}
	}
	if n == 0 {
		return nil
	}
	return fmt.Errorf("%d file(s) are larger than %s can hold (largest %.2f GB; the limit is 4 GB per file). Use an exFAT or NTFS drive for this destination", n, d.fs.name, float64(largest)/1e9)
}

// Close follows completion of the owning job (including its workers and catalog
// updates). Borrowed staging files must not outlive it. Independent jobs obtain
// independent DestRoots and are coordinated by the platform lease.
func (d *DestRoot) Close() error {
	if d == nil || d.root == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.closeErr = errors.Join(d.discardAll(), d.root.Close(), d.release())
	})
	return d.closeErr
}

// MkdirAll creates rel (and its parents) beneath the authorized root. An empty
// rel or "." is the root itself, which already exists.
func (d *DestRoot) MkdirAll(rel string) error {
	r := filepath.ToSlash(rel)
	if r == "" || r == "." {
		return nil
	}
	v, err := destRel(r)
	if err != nil {
		return err
	}
	parent := d.root
	defer func() {
		if parent != d.root {
			parent.Close()
		}
	}()
	for _, component := range strings.Split(v, "/") {
		if err := d.checkParent(parent); err != nil {
			return err
		}
		if err := parent.Mkdir(component, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		sub, err := parent.OpenRoot(component)
		if err != nil {
			return err
		}
		if parent != d.root {
			parent.Close()
		}
		parent = sub
	}
	return d.checkParent(parent)
}

// checkParent re-checks a directory below the root at the moment it is used: its
// namespace must still pass the platform policy and it must be on the same volume
// as the authorized root, so a mount point below the root cannot redirect writes.
func (d *DestRoot) checkParent(parent *os.Root) error {
	ns, err := checkDestNamespace(parent)
	if err != nil {
		return err
	}
	// A folder inside the destination may not let anyone change it that the
	// destination itself was not accepted with — for example a shared folder moved
	// into a private destination, or one left shared by Make private.
	for _, f := range ns.findings {
		// Folders above the destination were judged when it was authorized.
		if !atOrBelow(f.Folder, d.canonical) {
			continue
		}
		if !d.accepted[findingKey(f)] {
			who := f.Group
			if f.Kind == destFindingUnchecked {
				who = "any account (" + f.FS + " has no permissions)"
			}
			return fmt.Errorf("refusing: %s inside the destination lets %s change it, and %s was not accepted for that. Obelisk won't write there; check that folder, make it private, or choose another destination", f.Folder, who, d.display)
		}
	}
	f, err := parent.Open(".")
	if err != nil {
		return err
	}
	id, err := handleIdentity(f)
	f.Close()
	if err != nil {
		return err
	}
	if id.vol != d.volume {
		return fmt.Errorf("unsupported publication namespace: nested filesystem differs from the leased destination volume")
	}
	return nil
}

// Open opens rel for reading, relative to the authorized root.
func (d *DestRoot) Open(rel string) (*os.File, error) {
	v, err := destRel(rel)
	if err != nil {
		return nil, err
	}
	return d.root.Open(v)
}

// Stat stats rel relative to the authorized root.
func (d *DestRoot) Stat(rel string) (fs.FileInfo, error) {
	v, err := destRel(rel)
	if err != nil {
		return nil, err
	}
	return d.root.Stat(v)
}

// HashRel is the copy-then-verify read-back: SHA-256 of rel, read back through the
// authorized handle rather than reopened by absolute name.
func (d *DestRoot) HashRel(rel string) (string, error) {
	if stage := d.stage(rel); stage != nil {
		if _, err := stage.file.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		return hashReader(stage.file)
	}
	f, err := d.Open(rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(f)
}

// SampleHashRel is the level-C fingerprint of rel, read through the authorized
// handle. Same fingerprint as sampleHashHex — same function, different opener.
func (d *DestRoot) SampleHashRel(rel string) (string, error) {
	f, err := d.Open(rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return sampleHashOpen(f)
}

// Test observation after the OS has reported a conflicting cooperative lease.
var destLeaseContended = func() {}
