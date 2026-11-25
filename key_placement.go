package main

// key_placement.go — a key never sits beside the data it unlocks.
//
// The rule: a key may sit on a drive only if nothing on that drive is encrypted
// with it. Every keystore holds every key, so an encrypted package may not be
// written (to staging, to a volume, or as a tape segment) to a drive that holds
// any keystore. Drives are compared by the same identity keystore_rules.go uses
// to keep the keystores apart (the physical disks under each volume, so two
// partitions of one disk are one drive). A drive that cannot be identified, the
// destination's or a keystore's, is not proof of a separate drive and refuses
// too. The user may override for one write; the write then carries a notice
// saying what was done and what to do about it.
//
// A keystore kept on a drive that is put away (unplugged) is placed by the
// drive it was last seen on, which Obelisk remembers in keystore-drives.json in
// its data folder: the drive's identity is its physical disks, which do not
// change while it is away. One never seen connected cannot be placed, refuses,
// and says to reconnect it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
)

// PackageWriteOptions are the user's choices for one package write.
type PackageWriteOptions struct {
	// AllowKeyDrive writes an encrypted package even where its key would sit on
	// the same drive (or a drive that cannot be told apart), after the user has
	// been told and chose to.
	AllowKeyDrive bool
}

func packageWriteOptions(opts []PackageWriteOptions) PackageWriteOptions {
	var out PackageWriteOptions
	for _, o := range opts {
		out.AllowKeyDrive = out.AllowKeyDrive || o.AllowKeyDrive
	}
	return out
}

// KeyDriveError refuses an encrypted package write whose destination shares a
// drive with a keystore, or whose drive (or a keystore's) cannot be identified.
type KeyDriveError struct {
	Package string
	Dest    string
	Same    []string // keystores on the destination's drive
	Unknown []string // the destination or keystores whose drive could not be identified
	Unseen  []string // keystores not connected now and never seen connected
}

func (e *KeyDriveError) Error() string {
	var why []string
	if len(e.Same) > 0 {
		why = append(why, fmt.Sprintf("%s is on the same drive as keystore %s", e.Dest, strings.Join(e.Same, ", ")))
	}
	if len(e.Unknown) > 0 {
		why = append(why, fmt.Sprintf("Obelisk cannot tell which drive %s is on, so it cannot show the key is kept apart from the package", strings.Join(e.Unknown, ", ")))
	}
	if len(e.Unseen) > 0 {
		why = append(why, fmt.Sprintf("keystore %s is not connected and Obelisk has never seen which drive it is on; reconnect it once so Obelisk can check", strings.Join(e.Unseen, ", ")))
	}
	return fmt.Sprintf("refusing to write encrypted package %s: %s. A key may sit on a drive only if nothing on that drive is encrypted with it. Choose a destination on another drive, move the keystore, or reconnect it; to write here anyway, confirm the key-drive override (allow_key_drive)", e.Package, strings.Join(why, "; "))
}

// Notice is what an overridden write reports and logs.
func (e *KeyDriveError) Notice() string {
	var what, todo []string
	if len(e.Same) > 0 {
		what = append(what, fmt.Sprintf("on the same drive as keystore %s: anyone who has this drive has the key to the package, and losing the drive loses both", strings.Join(e.Same, ", ")))
	}
	if len(e.Unknown) > 0 {
		what = append(what, fmt.Sprintf("where Obelisk could not tell the drive of %s apart from the keystores", strings.Join(e.Unknown, ", ")))
	}
	if len(e.Same) > 0 || len(e.Unknown) > 0 {
		todo = append(todo, "Move the keystore to a drive that holds no package encrypted with its keys, then run key sync.")
	}
	if len(e.Unseen) > 0 {
		what = append(what, fmt.Sprintf("while keystore %s was not connected and its drive had never been seen", strings.Join(e.Unseen, ", ")))
		todo = append(todo, fmt.Sprintf("Reconnect keystore %s and check that its drive holds no package encrypted with its keys.", strings.Join(e.Unseen, ", ")))
	}
	return fmt.Sprintf("ACTION NEEDED: encrypted package %s was written to %s by your override, %s. %s", e.Package, e.Dest, strings.Join(what, "; "), strings.Join(todo, " "))
}

// checkKeyPlacement decides whether encrypted package c may be written to dest.
// A plaintext package has no key and is not checked. With allow, a write that
// would be refused goes ahead and the notice to show the user is returned.
func (a *App) checkKeyPlacement(c *Chunk, dest string, keystores []string, allow bool) (notice string, err error) {
	if c == nil || !c.Encrypted {
		return "", nil
	}
	kd := &KeyDriveError{Package: c.Name, Dest: dest}
	destDisks, derr := keyPlacementDevice(dest)
	if derr != nil {
		kd.Unknown = append(kd.Unknown, dest)
	}
	seen := map[string]string{}
	for _, p := range keystores {
		disks, kerr := keyPlacementDevice(p)
		switch {
		case kerr == nil:
			seen[p] = strings.Join(disks, deviceSeparator)
		case errors.Is(kerr, errNoExistingParent):
			// Not connected: placed by the drive it was last seen on.
			last, ok := a.lastSeenKeystoreDrive(p)
			if !ok {
				kd.Unseen = append(kd.Unseen, p)
				continue
			}
			disks = strings.Split(last, deviceSeparator)
		default:
			kd.Unknown = append(kd.Unknown, p)
			continue
		}
		if derr == nil && sharesDisk(destDisks, disks) {
			kd.Same = append(kd.Same, p)
		}
	}
	a.rememberKeystoreDrives(seen)
	if len(kd.Same) == 0 && len(kd.Unknown) == 0 && len(kd.Unseen) == 0 {
		return "", nil
	}
	if !allow {
		return "", kd
	}
	notice = kd.Notice()
	a.Store.Log("keys", notice)
	return notice, nil
}

// keyPlacementDevice is the drive identity of path by keystore_rules.go's rule,
// split into its disks. A path with nothing of it present cannot be identified.
func keyPlacementDevice(path string) ([]string, error) {
	dev, err := keystoreDeviceOf(path)
	if err != nil {
		return nil, err
	}
	if dev == "" {
		return nil, errors.New("no drive identity")
	}
	return strings.Split(dev, deviceSeparator), nil
}

// keystoreDrivesMu serialises reads and saves of keystore-drives.json.
var keystoreDrivesMu sync.Mutex

func (a *App) keystoreDrivesPath() string {
	if a.DataDir == "" {
		return ""
	}
	return filepath.Join(a.DataDir, "keystore-drives.json")
}

func keystoreDriveKey(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	return strings.ToLower(filepath.Clean(abs))
}

// readKeystoreDrives is the drive each keystore was last seen on, by path.
func (a *App) readKeystoreDrives() map[string]string {
	out := map[string]string{}
	if path := a.keystoreDrivesPath(); path != "" {
		if b, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(b, &out)
		}
	}
	return out
}

// lastSeenKeystoreDrive is the drive identity keystore p was last seen on.
func (a *App) lastSeenKeystoreDrive(p string) (string, bool) {
	keystoreDrivesMu.Lock()
	defer keystoreDrivesMu.Unlock()
	dev, ok := a.readKeystoreDrives()[keystoreDriveKey(p)]
	return dev, ok && dev != ""
}

// rememberKeystoreDrives records the drives the keystores in seen are on now.
// It is best effort: a record that cannot be saved only means a keystore put
// away before its drive was recorded refuses as unseen, which is the safe side.
func (a *App) rememberKeystoreDrives(seen map[string]string) {
	path := a.keystoreDrivesPath()
	if path == "" || len(seen) == 0 {
		return
	}
	keystoreDrivesMu.Lock()
	defer keystoreDrivesMu.Unlock()
	cur := a.readKeystoreDrives()
	next := make(map[string]string, len(cur)+len(seen))
	for k, v := range cur {
		next[k] = v
	}
	for p, dev := range seen {
		next[keystoreDriveKey(p)] = dev
	}
	if reflect.DeepEqual(cur, next) {
		return
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return
	}
	tmp, err := writeStage(path, b, 0o600)
	if err != nil {
		return
	}
	if err := renameReplacing(tmp, path); err != nil {
		_ = removeStage(tmp)
	}
}

func sharesDisk(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}
