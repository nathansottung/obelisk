package main

// keys_ready.go — a key is not ready until it is safe to depend on.
//
// The rule: a key is ready only when it is in every keystore and the user has
// made an offline copy of it (a printout, a plain-text file on a drive they
// choose, or a password-manager note) and proved that copy by typing or pasting
// it back. Nothing is encrypted with a key that is not ready. Lose every copy of
// a key and every package made with it can never be opened.
//
// One shared key serves every encrypted build: the newest shared key that is
// ready is the current key. Making a new one (rotation) is always the user's
// choice, and until the new key is proven builds keep using the old one. A
// package meant to be handed to someone else can have a separate key of its own,
// which needs its own offline copy and proof before that package is built. No
// key is ever removed: each package keeps opening with the key it was made with.
//
// The secret leaves the server only in the reply to a deliberate reveal, and
// never goes into a URL, the audit log, a job record or the catalog.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	keyPurposeShared  = "shared"
	keyPurposePackage = "package"

	keyStateReady     = "ready"
	keyStateNotReady  = "not ready"
	keyStateNotProven = "not proven" // a key made before key readiness
	keyStateProven    = "proven"     // such a key, proven since
)

// keyCopyKinds are the offline copies a key can be proven from.
var keyCopyKinds = map[string]string{
	"printout":         "printout",
	"file":             "plain-text file",
	"password-manager": "password-manager note",
}

// KeyState is what the Keys page shows for one key. It holds no secret.
type KeyState struct {
	Ref         string     `json:"key_ref"`
	Name        string     `json:"name"`
	Note        string     `json:"note,omitempty"`
	Purpose     string     `json:"purpose"` // shared, package, or "" for a key from before
	PackageID   int        `json:"package_id,omitempty"`
	PackageName string     `json:"package_name,omitempty"`
	Fingerprint string     `json:"fingerprint"`
	CreatedAt   time.Time  `json:"created_at"`
	State       string     `json:"state"`
	Step        string     `json:"step,omitempty"` // what to do next; empty when nothing is left
	InKeystores bool       `json:"in_keystores"`
	Current     bool       `json:"current,omitempty"`
	Packages    int        `json:"packages"`
	ProvenAt    *time.Time `json:"proven_at,omitempty"`
	Proofs      []KeyProof `json:"proofs,omitempty"`
}

// KeyNotReadyError refuses an encrypted build whose key is not ready, and names
// the step to finish.
type KeyNotReadyError struct {
	Package string
	Ref     string // the key, when there is one
	Step    string
}

func (e *KeyNotReadyError) Error() string {
	return fmt.Sprintf("refusing to encrypt package %s: no key is ready for it. %s", e.Package, e.Step)
}

// keyDisplayName is the name the user reads for a key.
func keyDisplayName(m *KeyMeta) string {
	if m == nil {
		return ""
	}
	if m.Name != "" {
		return m.Name
	}
	if m.Note != "" {
		return m.Note
	}
	return m.Ref
}

// keyLabel names a key for RESTORE.txt and messages: its reference, name and
// creation date.
func (a *App) keyLabel(ref string) string {
	m := a.Store.KeyMetaCopy(ref)
	if m == nil {
		return ref
	}
	return fmt.Sprintf("%s (%q, created %s)", ref, keyDisplayName(m), m.CreatedAt.UTC().Format("2006-01-02"))
}

// keystoreRefSets reads every keystore once and reports which keys each holds.
// A keystore that cannot be read is named in unreadable.
func (a *App) keystoreRefSets(paths []string) (sets []map[string]bool, unreadable []string) {
	for _, p := range paths {
		ks, err := a.readExistingKeystore(p, false)
		if err != nil {
			unreadable = append(unreadable, p)
			continue
		}
		sets = append(sets, keyRefs(ks))
	}
	return sets, unreadable
}

// KeyStates is every key with its readiness, newest first.
func (a *App) KeyStates() []KeyState {
	cfg, _ := a.LoadConfig()
	paths := cfg.KeystorePaths
	sets, unreadable := a.keystoreRefSets(paths)
	counts := a.Store.KeyPackageCounts()
	metas := a.Store.KeyMetas()
	out := make([]KeyState, 0, len(metas))
	// Newest first: later in the catalog is newer, so keys made in the same
	// instant still sort by the order they were made.
	for i := len(metas) - 1; i >= 0; i-- {
		mp := metas[i]
		m := a.Store.KeyMetaCopy(mp.Ref)
		if m == nil {
			continue
		}
		st := KeyState{Ref: m.Ref, Name: keyDisplayName(m), Note: m.Note, Purpose: m.Purpose,
			PackageID: m.PackageID, Fingerprint: m.Fingerprint, CreatedAt: m.CreatedAt,
			Packages: counts[m.Ref], ProvenAt: m.ProvenAt, Proofs: m.Proofs}
		if m.Purpose == keyPurposePackage {
			if c := a.Store.Chunk(m.PackageID); c != nil {
				st.PackageName = c.Name
			}
		}
		st.InKeystores = len(paths) >= MinKeystores && len(unreadable) == 0 && len(sets) == len(paths)
		for _, s := range sets {
			st.InKeystores = st.InKeystores && s[m.Ref]
		}
		st.State, st.Step = keyReadiness(m, st.InKeystores, unreadable)
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	for i := range out {
		if out[i].Purpose == keyPurposeShared && out[i].State == keyStateReady {
			out[i].Current = true // the newest ready shared key
			break
		}
	}
	return out
}

// keyReadiness decides a key's state and the next step for it.
func keyReadiness(m *KeyMeta, inKeystores bool, unreadable []string) (state, step string) {
	name := keyDisplayName(m)
	if m.Purpose == "" {
		// Made by a build before key readiness. It still restores; it is never
		// used for a new build.
		if m.ProvenAt != nil {
			return keyStateProven, ""
		}
		return keyStateNotProven, fmt.Sprintf("Make an offline copy of key %s and prove it (Keys → %s → Make an offline copy). Restores do not wait for this.", name, name)
	}
	if !inKeystores {
		if len(unreadable) > 0 {
			return keyStateNotReady, fmt.Sprintf("Key %s cannot be checked in every keystore: reconnect keystore %s, then run key sync.", name, strings.Join(unreadable, ", "))
		}
		return keyStateNotReady, fmt.Sprintf("Key %s is not in every keystore: reconnect both keystores and run key sync (Keys → Sync keystores).", name)
	}
	if m.ProvenAt == nil {
		return keyStateNotReady, fmt.Sprintf("Make an offline copy of key %s and prove it by typing it back (Keys → %s → Make an offline copy).", name, name)
	}
	return keyStateReady, ""
}

// separateKeyFor is the separate key made for package id, or nil.
func (a *App) separateKeyFor(id int) *KeyMeta {
	var best *KeyMeta
	for _, m := range a.Store.KeyMetas() {
		if m.Purpose == keyPurposePackage && m.PackageID == id && (best == nil || m.CreatedAt.After(best.CreatedAt)) {
			best = m
		}
	}
	if best == nil {
		return nil
	}
	return a.Store.KeyMetaCopy(best.Ref)
}

// keyForBuild picks the key an encrypted build of c uses: the package's own
// separate key when it has one, otherwise the current shared key. Either must be
// ready, or the build refuses and names the step to finish.
func (a *App) keyForBuild(c *Chunk) (ref, pass, fpr string, err error) {
	states := a.KeyStates()
	pick := func(ok func(KeyState) bool) *KeyState {
		for i := range states {
			if ok(states[i]) {
				return &states[i]
			}
		}
		return nil
	}
	var st *KeyState
	if sep := a.separateKeyFor(c.ID); sep != nil {
		st = pick(func(s KeyState) bool { return s.Ref == sep.Ref })
		if st == nil || st.State != keyStateReady {
			step := "Finish its separate key on the Keys page."
			if st != nil {
				step = st.Step
			}
			return "", "", "", &KeyNotReadyError{Package: c.Name, Ref: sep.Ref, Step: step}
		}
	} else {
		st = pick(func(s KeyState) bool { return s.Current })
		if st == nil {
			step := "Create a key on the Keys page (Keys → Create key), make an offline copy and prove it."
			if newest := pick(func(s KeyState) bool { return s.Purpose == keyPurposeShared }); newest != nil {
				step = newest.Step
				return "", "", "", &KeyNotReadyError{Package: c.Name, Ref: newest.Ref, Step: step}
			}
			return "", "", "", &KeyNotReadyError{Package: c.Name, Step: step}
		}
	}
	pass, err = a.Passphrase(st.Ref)
	if err != nil {
		return "", "", "", &KeyNotReadyError{Package: c.Name, Ref: st.Ref, Step: "Its key cannot be read from the keystores: reconnect them and run key sync."}
	}
	if !strings.EqualFold(passFingerprint(pass), st.Fingerprint) {
		return "", "", "", fmt.Errorf("refusing to encrypt package %s: key %s in the keystores does not match its recorded fingerprint; reconcile the keystores first", c.Name, st.Ref)
	}
	return st.Ref, pass, st.Fingerprint, nil
}

// keyBuildGate is keyForBuild without the secret, for the build route to refuse
// before a job starts.
func (a *App) keyBuildGate(c *Chunk) error {
	if c == nil || !c.Encrypted || c.Adopted {
		return nil
	}
	_, _, _, err := a.keyForBuild(c)
	return err
}

// CreateKey makes a new key, shared or for one package, and saves it to both
// keystores. It returns the key's state, never its secret. The key is not ready
// until an offline copy of it is proven.
func (a *App) CreateKey(name, purpose string, packageID int) (KeyState, error) {
	cfg, err := a.LoadConfig()
	if err != nil {
		return KeyState{}, err
	}
	if len(cfg.KeystorePaths) < MinKeystores {
		return KeyState{}, fmt.Errorf("set %d keystore locations on different drives first (Keys → Create key asks for them)", MinKeystores)
	}
	if err := checkKeystorePaths(cfg.KeystorePaths); err != nil {
		return KeyState{}, err
	}
	switch purpose {
	case "", keyPurposeShared:
		purpose, packageID = keyPurposeShared, 0
	case keyPurposePackage:
		c := a.Store.Chunk(packageID)
		if c == nil {
			return KeyState{}, fmt.Errorf("package %d not found", packageID)
		}
		if c.Adopted {
			return KeyState{}, fmt.Errorf("package %s was adopted as found; Obelisk never encrypts an adopted backup", c.Name)
		}
		if !c.Encrypted {
			return KeyState{}, fmt.Errorf("package %s is not encrypted, so it has no key", c.Name)
		}
		if a.separateKeyFor(packageID) != nil {
			return KeyState{}, fmt.Errorf("package %s already has its own key; a key is never removed or replaced", c.Name)
		}
		if name == "" {
			name = "Key for package " + c.Name
		}
	default:
		return KeyState{}, fmt.Errorf("unknown key purpose %q", purpose)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		n := 1
		for _, m := range a.Store.KeyMetas() {
			if m.Purpose == keyPurposeShared {
				n++
			}
		}
		name = fmt.Sprintf("Shared key %d", n)
	}
	if len(name) > 120 || strings.ContainsAny(name, "\r\n") {
		return KeyState{}, fmt.Errorf("a key name is one line of at most 120 characters")
	}
	ref, _, _, err := a.GenerateKey(name)
	if err != nil {
		return KeyState{}, err
	}
	if err := a.Store.UpdateKeyMeta(ref, func(m *KeyMeta) {
		m.Name, m.Purpose, m.PackageID = name, purpose, packageID
	}); err != nil {
		return KeyState{}, fmt.Errorf("key %s was saved to the keystores, but its record was not saved: %w", ref, err)
	}
	a.Store.Log("keys", fmt.Sprintf("created key %s (%s); not ready until an offline copy is proven", ref, name))
	for _, st := range a.KeyStates() {
		if st.Ref == ref {
			return st, nil
		}
	}
	return KeyState{}, fmt.Errorf("key %s was created but cannot be found", ref)
}

// keySecret reads key ref's secret and checks it against the recorded
// fingerprint.
func (a *App) keySecret(ref string) (*KeyMeta, string, error) {
	m := a.Store.KeyMetaCopy(ref)
	if m == nil {
		return nil, "", fmt.Errorf("key %s not found", ref)
	}
	pass, err := a.Passphrase(ref)
	if err != nil {
		return nil, "", fmt.Errorf("key %s cannot be read from the keystores: %w", ref, err)
	}
	if !strings.EqualFold(passFingerprint(pass), m.Fingerprint) {
		return nil, "", fmt.Errorf("key %s in the keystores does not match its recorded fingerprint", ref)
	}
	return m, pass, nil
}

// keyOpensText says what a key opens, for the copies the user keeps.
func (a *App) keyOpensText(m *KeyMeta) string {
	switch m.Purpose {
	case keyPurposePackage:
		name := fmt.Sprintf("package %d", m.PackageID)
		if c := a.Store.Chunk(m.PackageID); c != nil {
			name = c.Name
		}
		return fmt.Sprintf("only package %s, the one it was made for", name)
	case keyPurposeShared:
		return fmt.Sprintf("every Obelisk package encrypted with key %s. The RESTORE.txt beside each package names its key", m.Ref)
	}
	return fmt.Sprintf("the Obelisk packages encrypted with key %s. The RESTORE.txt beside each package names its key", m.Ref)
}

// passwordManagerBlock is a plain-text note to paste into a password manager's
// secure note: the key, its fingerprint, what it opens, and how to restore by
// hand. Plain "Label: value" lines, no formatting, so any password manager keeps
// it as typed.
func (a *App) passwordManagerBlock(m *KeyMeta, pass string) string {
	var b strings.Builder
	b.WriteString("Obelisk encryption key\n")
	b.WriteString("Name: " + keyDisplayName(m) + "\n")
	b.WriteString("Key ref: " + m.Ref + "\n")
	b.WriteString("Created: " + m.CreatedAt.UTC().Format("2006-01-02") + "\n")
	b.WriteString("Opens: " + a.keyOpensText(m) + "\n")
	b.WriteString("Passphrase: " + pass + "\n")
	b.WriteString("SHA-256 fingerprint: " + m.Fingerprint + "\n")
	b.WriteString("\n")
	b.WriteString("Restore by hand with par2, gpg and tar. NAME is the package name in its RESTORE.txt.\n")
	b.WriteString("gpg asks for the passphrase. Never type it on a command line.\n")
	b.WriteString("  par2 verify NAME.tar.gpg.par2\n")
	b.WriteString("  gpg -d -o NAME.tar NAME.tar.gpg\n")
	b.WriteString("  tar -xf NAME.tar\n")
	b.WriteString("\n")
	b.WriteString("Anyone who reads this note can open these backups. Lose every copy of this key and they can never be opened.\n")
	return b.String()
}

// RevealKey returns key ref's secret, laid out for the offline copy the user
// chose to make. It is called only from the Keys page after a deliberate click,
// in a request body, never a URL. The audit log records that it was shown, not
// what it is.
func (a *App) RevealKey(ref, kind string) (map[string]any, error) {
	what, ok := keyCopyKinds[kind]
	if !ok || kind == "file" {
		return nil, fmt.Errorf("choose a printout or a password-manager note; a plain-text file is saved by Obelisk to where you choose")
	}
	m, pass, err := a.keySecret(ref)
	if err != nil {
		return nil, err
	}
	lines := keyLines(pass)
	rows := make([]map[string]string, 0, len(lines))
	for _, ln := range lines {
		rows = append(rows, map[string]string{"label": sheetLineLabel(ln.No, len(lines)), "groups": ln.Groups, "code": ln.Code})
	}
	out := map[string]any{"key_ref": m.Ref, "name": keyDisplayName(m), "fingerprint": m.Fingerprint,
		"created_at": m.CreatedAt, "opens": a.keyOpensText(m), "lines": rows, "length": len(pass)}
	switch kind {
	case "printout":
		out["sheet"] = keySheet(m.Ref, keyDisplayName(m), m.Fingerprint, pass)
	case "password-manager":
		out["block"] = a.passwordManagerBlock(m, pass)
	}
	a.Store.Log("keys", fmt.Sprintf("key %s (%s) was shown to make a %s", m.Ref, keyDisplayName(m), what))
	return out, nil
}

// keyFileRefusal checks where a plain-text copy of key ref may be saved. Never
// in the data folder, never on a drive that holds a backup (staging or any
// package copy), and never on a drive Obelisk cannot identify.
func (a *App) keyFileRefusal(ref, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("give the whole path of the file to create, for example E:\\keys\\obelisk-key.txt")
	}
	cmp := func(p string) string {
		p = filepath.Clean(p)
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	if a.DataDir != "" && (cmp(path) == cmp(a.DataDir) || canonicalWithin(cmp(a.DataDir), cmp(path))) {
		return fmt.Errorf("refusing to save a key in Obelisk's data folder %s: a copy there is lost with the catalog. Choose a drive you keep apart", a.DataDir)
	}
	dir := filepath.Dir(path)
	disks, err := keyPlacementDevice(dir)
	if err != nil {
		return fmt.Errorf("refusing to save the key to %s: Obelisk cannot tell which drive it is on, so it cannot show the key is kept apart from your backups", dir)
	}
	cfg, _ := a.LoadConfig()
	type place struct{ what, path string }
	var places []place
	if cfg.StagingDir != "" {
		places = append(places, place{"the staging folder, where packages are built", cfg.StagingDir})
	}
	for _, c := range a.Store.Chunks(0) {
		for i := range c.Copies {
			cp := c.Copies[i]
			if cp.Path == "" {
				continue
			}
			what := "a backup destination (a copy of package " + c.Name + ")"
			if c.valuesOf(&cp).KeyRef == ref {
				what = "a drive holding package " + c.Name + ", which this key opens"
			}
			places = append(places, place{what, cp.Path})
		}
	}
	for _, pl := range places {
		d, derr := keyPlacementDevice(pl.path)
		if errors.Is(derr, errNoExistingParent) {
			continue // that drive is not connected, so it is not this one
		}
		if derr != nil {
			return fmt.Errorf("refusing to save the key: Obelisk cannot tell which drive %s is on, so it cannot show %s is on another drive", pl.path, dir)
		}
		if sharesDisk(disks, d) {
			return fmt.Errorf("refusing to save the key to %s: that drive also holds %s. A key may sit on a drive only if nothing on it is encrypted with it; choose a drive that holds no backups", dir, pl.what)
		}
	}
	return nil
}

// SaveKeyFile writes a plain-text copy of key ref to path, a new file the user
// chose. It is never written anywhere else, and never on its own: only this call,
// from the Keys page, saves one. An existing file is never replaced.
func (a *App) SaveKeyFile(ref, path string) (string, error) {
	if err := a.keyFileRefusal(ref, path); err != nil {
		return "", err
	}
	m, pass, err := a.keySecret(ref)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("refusing to replace %s: choose a new file name", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	dir, name := filepath.Dir(path), filepath.Base(path)
	of, err := a.openOutFolder(dir)
	if err != nil {
		return "", err
	}
	defer of.Close()
	body := []byte(keySheet(m.Ref, keyDisplayName(m), m.Fingerprint, pass) + "\n" + a.passwordManagerBlock(m, pass))
	tmp, f, err := of.Create(name)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(body); err != nil {
		of.Discard(tmp)
		return "", err
	}
	sum := sha256.Sum256(body)
	if err := of.Commit(tmp, name, 0o600, hex.EncodeToString(sum[:])); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("refusing to replace %s: choose a new file name", path)
		}
		return "", err
	}
	a.Store.Log("keys", fmt.Sprintf("a plain-text copy of key %s (%s) was saved to %s", m.Ref, keyDisplayName(m), path))
	return path, nil
}

// KeyProofResult reports a proof without the secret.
type KeyProofResult struct {
	OK           bool     `json:"ok"`
	KeyRef       string   `json:"key_ref"`
	Lines        int      `json:"lines,omitempty"`
	MissingLines []int    `json:"missing_lines,omitempty"`
	BadLines     []int    `json:"bad_lines,omitempty"`
	Notes        []string `json:"notes,omitempty"`
	State        string   `json:"state,omitempty"`
}

// ProveKey checks one offline copy of key ref: a sheet typed back line by line,
// a password-manager note pasted back, or the passphrase typed whole. The whole
// key is compared by SHA-256; for a sheet, a missing or changed line is named.
// Only a match proves the key.
func (a *App) ProveKey(ref, text, kind string) (KeyProofResult, error) {
	res := KeyProofResult{KeyRef: ref}
	what, ok := keyCopyKinds[kind]
	if !ok {
		return res, fmt.Errorf("say which copy this is: a printout, a plain-text file or a password-manager note")
	}
	m, pass, err := a.keySecret(ref)
	if err != nil {
		return res, err
	}
	typed := ""
	if keySheetLineRe.MatchString(firstSheetLine(text)) {
		got, sheet := parseKeySheet(text)
		res.Lines, res.MissingLines, res.BadLines = sheet.Lines, sheet.MissingLines, sheet.BadLines
		res.Notes = append(res.Notes, sheet.Notes...)
		// Compare each line with the key's own, so a line left off the end, or
		// one changed so that its check code still passes, is named too.
		want := keyLines(pass)
		have := sheetPayloads(text)
		for _, ln := range want {
			p, ok := have[ln.No]
			switch {
			case !ok && !containsInt(res.MissingLines, ln.No):
				res.MissingLines = append(res.MissingLines, ln.No)
			case ok && p != ln.Chars && !containsInt(res.BadLines, ln.No):
				res.BadLines = append(res.BadLines, ln.No)
			}
		}
		for no := range have {
			if no > len(want) && !containsInt(res.BadLines, no) {
				res.BadLines = append(res.BadLines, no)
			}
		}
		sort.Ints(res.MissingLines)
		sort.Ints(res.BadLines)
		typed = got
	} else if v, ok := noteField(text, "passphrase"); ok {
		typed = v
	} else {
		typed = strings.TrimSpace(text)
	}
	match := typed != "" && strings.EqualFold(passFingerprint(typed), m.Fingerprint)
	res.OK = match && len(res.MissingLines) == 0 && len(res.BadLines) == 0
	if !res.OK {
		res.Notes = nil
		if len(res.MissingLines) > 0 {
			res.Notes = append(res.Notes, "missing line(s): "+lineNames(res.MissingLines))
		}
		if len(res.BadLines) > 0 {
			res.Notes = append(res.Notes, "changed line(s): "+lineNames(res.BadLines))
		}
		if len(res.Notes) == 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("what you entered does not match key %s (its SHA-256 differs). Check the copy and try again", m.Ref))
		}
		res.State = a.keyStateOf(ref)
		return res, nil
	}
	now := time.Now().UTC()
	if err := a.Store.UpdateKeyMeta(ref, func(k *KeyMeta) {
		if k.ProvenAt == nil {
			k.ProvenAt = &now
		}
		k.Proofs = append(k.Proofs, KeyProof{Kind: kind, At: now})
	}); err != nil {
		return res, fmt.Errorf("the copy matched, but the proof was not saved: %w", err)
	}
	a.Store.Log("keys", fmt.Sprintf("key %s (%s) proven from its %s", m.Ref, keyDisplayName(m), what))
	res.State = a.keyStateOf(ref)
	return res, nil
}

func (a *App) keyStateOf(ref string) string {
	for _, st := range a.KeyStates() {
		if st.Ref == ref {
			return st.State
		}
	}
	return ""
}

// firstSheetLine is the first line of text that looks like a sheet line, or "".
func firstSheetLine(text string) string {
	for _, ln := range strings.Split(text, "\n") {
		if keySheetLineRe.MatchString(ln) {
			return ln
		}
	}
	return ""
}

// sheetPayloads is each typed sheet line's characters, by line number.
func sheetPayloads(text string) map[int]string {
	out := map[int]string{}
	for _, raw := range strings.Split(text, "\n") {
		m := keySheetLineRe.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		no, _ := strconv.Atoi(m[1])
		if _, dup := out[no]; dup {
			continue
		}
		out[no] = strings.ReplaceAll(strings.ReplaceAll(m[3], " ", ""), "\t", "")
	}
	return out
}

// noteField reads "Label: value" from a pasted note, ignoring case.
func noteField(text, label string) (string, bool) {
	for _, ln := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), label) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}
