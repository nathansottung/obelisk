package main

// destchoice.go — the user's decision about a destination with findings.
//
// Some destinations are acceptable only once the user has chosen what to do: a
// folder other accounts on this PC can change through a group Windows grants by
// default, a drive root, or a drive whose filesystem (exFAT, FAT32) cannot record
// permissions at all. Obelisk refuses to write to such a folder until the user
// picks one of three choices, none of them preselected:
//
//   - Make this folder private: Obelisk removes the other groups from that one
//     folder's permissions, after showing the exact change and a second confirm.
//     Only the folder's own permissions change; nothing inside it is touched.
//   - Use it anyway: the user types "Continue". The choice is recorded against the
//     folder's volume and identity, with a fingerprint of the permissions from the
//     folder up to its drive root and the date. Any later change to those
//     permissions, or to the filesystem, makes Obelisk ask again.
//   - Pick another folder.
//
// All of this is enforced here, server-side: AuthorizeDestination refuses with a
// *DestinationChoiceError until a matching choice exists.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// destUseAnywayWord is what the user types to use a folder anyway.
const destUseAnywayWord = "Continue"

// DestinationChoice is a recorded "Use it anyway" decision.
type DestinationChoice struct {
	Volume      uint64    `json:"volume"`      // volume serial (Windows) or device (Linux)
	Folder      uint64    `json:"folder"`      // the folder's file ID on that volume
	Path        string    `json:"path"`        // where the folder was when the choice was made
	Fingerprint string    `json:"fingerprint"` // permissions and filesystem, folder up to drive root
	Choice      string    `json:"choice"`
	ChosenAt    time.Time `json:"chosen_at"`
	Findings    []string  `json:"findings,omitempty"` // what the user was told, for the record
	Created     []uint64  `json:"created,omitempty"`  // folders Obelisk then created inside it
}

// DestinationCheck is the answer to "can Obelisk write here?" for the UI and API.
type DestinationCheck struct {
	Path        string             `json:"path"`
	Folder      string             `json:"folder,omitempty"` // the folder written into
	Status      string             `json:"status"`           // pass | choose | chosen | refuse
	Reason      string             `json:"reason,omitempty"` // for refuse
	Findings    []destFinding      `json:"findings,omitempty"`
	Fingerprint string             `json:"fingerprint,omitempty"`
	Notice      *DestinationNotice `json:"notice,omitempty"`
	ChosenAt    *time.Time         `json:"chosen_at,omitempty"`
}

// Destination check statuses.
const (
	destStatusPass   = "pass"   // nothing to decide
	destStatusChoose = "choose" // findings, and no matching choice: ask the user
	destStatusChosen = "chosen" // findings covered by a recorded choice
	destStatusRefuse = "refuse"
)

// Destination choices offered in a notice.
const (
	destChoiceMakePrivate = "make_private"
	destChoiceUseAnyway   = "use_anyway"
	destChoicePickAnother = "pick_another"
)

// DestinationNotice is the plain-words explanation shown before a choice.
type DestinationNotice struct {
	Title   string              `json:"title"`
	Lines   []string            `json:"lines"`   // each begins with ✗ or ✓
	Choices []DestinationOption `json:"choices"` // in order; none preselected
	Text    string              `json:"text"`    // the whole notice as plain text
}

// DestinationOption is one choice in a notice.
type DestinationOption struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// DestinationChoiceError refuses a write until the user has chosen. It carries
// the notice so a caller can show it.
type DestinationChoiceError struct {
	Check *DestinationCheck
}

func (e *DestinationChoiceError) Error() string {
	return fmt.Sprintf("choose what to do about %s before Obelisk writes there: %s", e.Check.Folder, e.Check.Notice.Title)
}

// destFingerprint condenses the permissions and filesystem of a folder and every
// folder above it. Any change to any of them changes the fingerprint.
func destFingerprint(chain []destLevel) string {
	h := sha256.New()
	for _, l := range chain {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", normPath(l.path), l.fs, l.perms)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CheckDestination reports, without writing anything, whether Obelisk may write
// to path and what the user must decide first.
func (s *Store) CheckDestination(path string) *DestinationCheck {
	in, err := s.inspectDestination(path)
	if err != nil {
		return &DestinationCheck{Path: path, Status: destStatusRefuse, Reason: err.Error()}
	}
	defer in.anchor.Close()
	return s.checkFromInspection(in)
}

func (s *Store) checkFromInspection(in *destInspection) *DestinationCheck {
	c := &DestinationCheck{Path: in.display, Folder: in.intended, Findings: in.findings, Fingerprint: in.fingerprint}
	switch {
	case len(in.findings) == 0:
		c.Status = destStatusPass
	case in.choice != nil:
		c.Status = destStatusChosen
		t := in.choice.ChosenAt
		c.ChosenAt = &t
	default:
		c.Status = destStatusChoose
		c.Notice = destNotice(in.intended, in.findings, in.stale, destCanMakePrivate(in))
	}
	return c
}

// destCanMakePrivate reports whether "Make this folder private" can resolve every
// finding: the folder exists, keeps permissions, is not a drive root, and the only
// findings are groups Windows grants on that folder itself.
func destCanMakePrivate(in *destInspection) bool {
	if !destMakePrivateSupported || len(in.missing) > 0 || len(in.findings) == 0 {
		return false
	}
	for _, f := range in.findings {
		if f.Kind != destFindingSharedGroup || normPath(f.Folder) != normPath(in.anchorCanon) {
			return false
		}
	}
	return true
}

// coveringChoice finds a recorded choice that still covers the inspected
// destination: one made for this very folder with the same fingerprint, or one
// made for a folder Obelisk then created this folder inside, whose own folder is
// unchanged. A choice for this folder whose fingerprint no longer matches is
// returned as stale, so the notice can say why Obelisk is asking again.
func (s *Store) coveringChoice(in *destInspection) (valid, stale *DestinationChoice) {
	s.mu.Lock()
	choices := append([]*DestinationChoice(nil), s.c.DestinationChoices...)
	s.mu.Unlock()
	for _, c := range choices {
		if c.Volume != in.anchorID.vol {
			continue
		}
		if c.Folder == in.anchorID.idx {
			if c.Fingerprint == in.fingerprint {
				return c, nil
			}
			stale = c
			continue
		}
		for _, id := range c.Created {
			if id == in.anchorID.idx && choiceFolderUnchanged(c) {
				return c, nil
			}
		}
	}
	return nil, stale
}

// choiceFolderUnchanged reopens the folder a choice was made for and confirms it
// is the same folder with the same permissions and filesystem.
func choiceFolderUnchanged(c *DestinationChoice) bool {
	r, err := os.OpenRoot(c.Path)
	if err != nil {
		return false
	}
	defer r.Close()
	id, err := rootIdentity(r)
	if err != nil || id.vol != c.Volume || id.idx != c.Folder {
		return false
	}
	fs, err := checkDestNamespace(r)
	return err == nil && destFingerprint(fs.chain) == c.Fingerprint
}

// recordCreatedUnderChoice notes the folders Obelisk created inside a folder the
// user chose to use anyway, so later runs into them are covered by that choice.
func (s *Store) recordCreatedUnderChoice(c *DestinationChoice, created []uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.c.DestinationChoices {
		if rec.Volume == c.Volume && rec.Folder == c.Folder {
			rec.Created = append(rec.Created, created...)
			_ = s.save()
			return
		}
	}
}

// ChooseUseAnyway records the user's "Use it anyway" for path. typed must be
// exactly "Continue", and fingerprint must be the one the user was shown: if the
// folder changed since, the user must look again.
func (s *Store) ChooseUseAnyway(path, fingerprint, typed string) (*DestinationCheck, error) {
	if strings.TrimSpace(typed) != destUseAnywayWord {
		return nil, fmt.Errorf("type %s to use this folder anyway", destUseAnywayWord)
	}
	in, err := s.inspectDestination(path)
	if err != nil {
		return nil, err
	}
	defer in.anchor.Close()
	if len(in.findings) == 0 {
		return s.checkFromInspection(in), nil
	}
	if fingerprint != in.fingerprint {
		return nil, fmt.Errorf("the permissions of %s changed since you looked; check the folder again", in.intended)
	}
	rec := &DestinationChoice{Volume: in.anchorID.vol, Folder: in.anchorID.idx, Path: in.anchorCanon,
		Fingerprint: in.fingerprint, Choice: destChoiceUseAnyway, ChosenAt: time.Now().UTC()}
	for _, f := range in.findings {
		rec.Findings = append(rec.Findings, destFindingSummary(f))
	}
	s.mu.Lock()
	kept := s.c.DestinationChoices[:0]
	for _, c := range s.c.DestinationChoices {
		if c.Volume != rec.Volume || c.Folder != rec.Folder {
			kept = append(kept, c)
		}
	}
	s.c.DestinationChoices = append(kept, rec)
	err = s.save()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	s.Log("destination", fmt.Sprintf("use anyway: %s (%s)", in.intended, strings.Join(rec.Findings, "; ")))
	in.choice = rec
	return s.checkFromInspection(in), nil
}

func destFindingSummary(f destFinding) string {
	switch f.Kind {
	case destFindingSharedGroup:
		return f.Group + " can change " + f.Folder
	case destFindingDriveRoot:
		return f.Group + " can create folders in the drive root " + f.Folder
	case destFindingUnchecked:
		return f.Folder + " is on " + f.FS + ": permissions, hard links and file IDs not checked"
	}
	return f.Kind + " " + f.Folder
}

// ---- Make this folder private ------------------------------------------------

// DestinationPrivatePlan is the exact permission change "Make this folder
// private" will make, shown before it is applied.
type DestinationPrivatePlan struct {
	Folder  string   `json:"folder"`
	Before  []string `json:"before"`
	After   []string `json:"after"`
	Removed []string `json:"removed"`
	Token   string   `json:"token"`
	// The exact security descriptors, for anyone who wants to compare them.
	BeforeSDDL string `json:"before_sddl"`
	AfterSDDL  string `json:"after_sddl"`
}

// destPrivateChange is a platform-computed permission change for one folder.
type destPrivateChange struct {
	id                      fileIdent
	before, after           string
	beforeLines, afterLines []string
	removedLines            []string
}

type pendingPrivate struct {
	path    string
	change  *destPrivateChange
	expires time.Time
}

var (
	privateMu      sync.Mutex
	privatePending = map[string]*pendingPrivate{}
)

// destPrivateTTL is how long a shown permission change may be applied.
const destPrivateTTL = 10 * time.Minute

// PreviewMakePrivate works out the exact permission change that would make path
// private and returns it with a one-use token. Nothing is changed.
func (s *Store) PreviewMakePrivate(path string) (*DestinationPrivatePlan, error) {
	in, err := s.inspectDestination(path)
	if err != nil {
		return nil, err
	}
	defer in.anchor.Close()
	if !destCanMakePrivate(in) {
		return nil, fmt.Errorf("Obelisk can only make an existing NTFS folder private when the other groups are granted on that folder itself; choose Use it anyway or pick another folder")
	}
	ch, err := planPrivateFolder(in.anchorCanon)
	if err != nil {
		return nil, err
	}
	if ch.id != in.anchorID {
		return nil, fmt.Errorf("%s changed while Obelisk was looking at it; check the folder again", in.anchorCanon)
	}
	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(tok)
	privateMu.Lock()
	for k, p := range privatePending {
		if time.Now().After(p.expires) {
			delete(privatePending, k)
		}
	}
	privatePending[token] = &pendingPrivate{path: in.anchorCanon, change: ch, expires: time.Now().Add(destPrivateTTL)}
	privateMu.Unlock()
	return &DestinationPrivatePlan{Folder: in.anchorCanon, Before: ch.beforeLines, After: ch.afterLines,
		Removed: ch.removedLines, Token: token, BeforeSDDL: ch.before, AfterSDDL: ch.after}, nil
}

// ApplyMakePrivate applies the change a preview showed. confirmed is the user's
// second confirmation. The token is used once; the folder must still be the same
// folder with the same permissions as when the change was shown. Only that
// folder's permissions change; nothing inside it is touched.
func (s *Store) ApplyMakePrivate(path, token string, confirmed bool) (*DestinationCheck, error) {
	if !confirmed {
		return nil, fmt.Errorf("confirm the permission change before Obelisk makes it")
	}
	privateMu.Lock()
	p := privatePending[token]
	delete(privatePending, token)
	privateMu.Unlock()
	if p == nil || time.Now().After(p.expires) {
		return nil, fmt.Errorf("that permission change has expired or was already used; preview it again")
	}
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || normPath(abs) != normPath(p.path) {
		return nil, fmt.Errorf("that permission change was shown for %s, not %s", p.path, path)
	}
	if err := applyPrivateFolder(p.path, p.change); err != nil {
		return nil, err
	}
	s.Log("destination", "made private: "+p.path+" (removed "+strings.Join(p.change.removedLines, "; ")+")")
	return s.CheckDestination(path), nil
}

// ---- the notice ---------------------------------------------------------------

// destNotice writes the plain-words notice for a destination's findings.
func destNotice(folder string, findings []destFinding, stale *DestinationChoice, canMakePrivate bool) *DestinationNotice {
	n := &DestinationNotice{}
	var groups []string
	var root, unchecked *destFinding
	groupFolder := folder
	for i := range findings {
		f := &findings[i]
		switch f.Kind {
		case destFindingSharedGroup:
			if !destContains(groups, f.Group) {
				groups = append(groups, f.Group)
			}
			groupFolder = f.Folder
		case destFindingDriveRoot:
			root = f
		case destFindingUnchecked:
			unchecked = f
		}
	}
	if stale != nil {
		n.Lines = append(n.Lines, fmt.Sprintf("✗ You chose to use this folder on %s, but its permissions or its drive have changed since, so Obelisk is asking again.", stale.ChosenAt.Format("2 January 2006")))
	}
	switch {
	case len(groups) > 0:
		n.Title = "Other accounts on this PC can change this folder"
		n.Lines = append(n.Lines,
			fmt.Sprintf("✗ What we found: %s lets %s add, change and delete files in it. %s Windows sets this up for folders made directly on a drive.", groupFolder, joinAnd(groups), groupMeaning(groups)),
			"✗ Why it matters: Another account on this PC could change or delete your backup copies, and you might not find out until you need them.")
		if groupFolder != folder {
			n.Lines = append(n.Lines, fmt.Sprintf("✗ Obelisk will write into %s, inside that folder, so the same applies there.", folder))
		}
	case root != nil:
		n.Title = "This is the top folder of a drive"
		n.Lines = append(n.Lines,
			fmt.Sprintf("✗ What we found: %s is the top folder of its drive, and Windows lets %s (anyone who signs in to this PC) create folders there.", root.Folder, root.Group),
			"✗ Why it matters: Another account on this PC could create a folder with the name Obelisk is about to use. Obelisk will not write into a folder another account made, so your backup could stop part-way.")
	case unchecked != nil:
		n.Title = "This drive can't record who may change files"
	}
	if unchecked != nil {
		n.Lines = append(n.Lines, fmt.Sprintf("✗ What we found: %s is on a drive formatted as %s. %s has no folder permissions, no hard links and no stable file IDs, so Obelisk could not run these checks:", unchecked.Folder, unchecked.FS, unchecked.FS),
			"✗ Who can change the folder: any account, on any PC this drive is plugged into, could change or delete your backup copies.",
			"✗ Whether a file is the one Obelisk checked a moment ago: a file swapped during a copy might go unnoticed. Obelisk still reads every copy back and compares it with the original.",
			"✓ Hard links: "+unchecked.FS+" can't make them, so a backup copy can't share its contents with another file. You lose nothing here.")
		if unchecked.MaxFile > 0 {
			n.Lines = append(n.Lines, "✗ File size: "+unchecked.FS+" can't hold a file larger than 4 GB. Obelisk refuses such a job before it copies anything.")
		}
	}
	if unchecked != nil && len(groups) == 0 && root == nil {
		n.Lines = append(n.Lines, "✓ Who it's fine for: If this drive stays with you and only your own PCs use it, this is usually fine.")
	} else {
		n.Lines = append(n.Lines, "✓ Who it's fine for: If you're the only person who signs in to this PC, this is usually fine.")
	}

	if canMakePrivate {
		n.Choices = append(n.Choices, DestinationOption{ID: destChoiceMakePrivate, Label: "Make this folder private",
			Detail: fmt.Sprintf("Obelisk removes %s from the permissions of %s, so only you, Administrators and Windows itself can change it. You'll see the exact change and confirm it first. Files already in the folder keep the permissions they have.", joinAnd(groups), groupFolder)})
	}
	n.Choices = append(n.Choices,
		DestinationOption{ID: destChoiceUseAnyway, Label: "Use it anyway",
			Detail: fmt.Sprintf("Type %s to confirm. Obelisk records your choice for this folder, with today's date, and asks again if its permissions or its drive change.", destUseAnywayWord)},
		DestinationOption{ID: destChoicePickAnother, Label: "Pick another folder",
			Detail: "Nothing is written here. Choose a different folder, such as one inside your user folder or on another drive."})

	var b strings.Builder
	b.WriteString(n.Title + "\n\n")
	for _, l := range n.Lines {
		b.WriteString(l + "\n")
	}
	b.WriteString("\nChoose one:\n")
	for _, c := range n.Choices {
		b.WriteString("  ( ) " + c.Label + ": " + c.Detail + "\n")
	}
	n.Text = strings.TrimRight(b.String(), "\n")
	return n
}

func groupMeaning(groups []string) string {
	var parts []string
	for _, g := range groups {
		switch g {
		case "Authenticated Users":
			parts = append(parts, "Authenticated Users means anyone who signs in to this PC.")
		case "Users":
			parts = append(parts, "Users means every standard account on this PC.")
		}
	}
	return strings.Join(parts, " ")
}

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func destContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
