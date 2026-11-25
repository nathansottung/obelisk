package main

// keystore_lock.go — one key change at a time, and no key lost to a crash.
//
// Every change to the keystores (a generated key, a sync) runs as one locked
// operation: read every keystore, check them, merge, write each one, and read
// each one back. The lock is the operating system's, taken on a lock file beside
// each keystore (keys.json.lock), so it holds against a second Obelisk process
// and against a second catalog configured with the same keystore files, not only
// against another job in this process. The keystore file itself is never locked:
// a byte-range lock on Windows would stop readers and the rename that publishes
// it. The lock files stay in place, for the reason given in dataowner.go.
//
// Each keystore write first keeps the previous generation beside it
// (keys.json.prev), then stages, flushes, renames and reads back the new one.
// A crash at any point leaves the new file whole, or the old file whole, and
// the previous generation whole beside it. Every passphrase lookup reads the
// previous generation too, and every key change first merges it back in, so a
// key that was in a keystore before the crash is still readable after it. Keys
// are only ever added, so recovering is a union and never chooses one
// generation over the other. Nothing here removes a file.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	keystoreLockSuffix = ".lock"
	keystorePrevSuffix = ".prev"
)

// keystoreLockWait bounds the wait for another holder of a keystore lock.
var keystoreLockWait = 30 * time.Second

// keystoreProcessMu orders key changes within this process before the
// operating-system locks are tried, so local callers queue rather than poll.
var keystoreProcessMu sync.Mutex

// takeKeystoreLock takes the operating-system lock on a keystore's lock file; a
// test seam.
var takeKeystoreLock = lockFileExclusive

// keystoreWriteStep, when set, is told each step of a keystore write as it
// completes. Tests use it to stop the write at that point, as a crash would.
var keystoreWriteStep func(step, path string)

func noteKeystoreStep(step, path string) {
	if keystoreWriteStep != nil {
		keystoreWriteStep(step, path)
	}
}

// errKeystoreBusy: another process or catalog held a keystore lock past the wait.
var errKeystoreBusy = errors.New("another Obelisk process is changing these keystores")

// lockKeystores takes the lock for a key change over paths. It takes them in one
// order (by lower-case absolute path) so two holders of overlapping sets never
// wait on each other in a circle. makeFolders creates a missing keystore folder
// (first-use key generation); otherwise a missing folder is an unreachable
// keystore and refuses.
func lockKeystores(paths []string, makeFolders bool) (release func(), err error) {
	type entry struct{ key, lock string }
	var entries []entry
	seen := map[string]bool{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("keystore %s: %w", p, err)
		}
		key := strings.ToLower(filepath.Clean(abs))
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry{key, abs + keystoreLockSuffix})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })

	keystoreProcessMu.Lock()
	var held []*os.File
	release = func() {
		for i := len(held) - 1; i >= 0; i-- {
			_ = unlockFile(held[i])
			_ = held[i].Close()
		}
		held = nil
		keystoreProcessMu.Unlock()
	}
	for _, e := range entries {
		if makeFolders {
			if err := os.MkdirAll(filepath.Dir(e.lock), 0o755); err != nil {
				release()
				return nil, fmt.Errorf("cannot create the keystore folder %s: %w", filepath.Dir(e.lock), err)
			}
		}
		f, err := os.OpenFile(e.lock, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			release()
			return nil, fmt.Errorf("cannot open the keystore lock %s; is its drive connected? %w", e.lock, err)
		}
		deadline := time.Now().Add(keystoreLockWait)
		for {
			err = takeKeystoreLock(f)
			if err == nil {
				break
			}
			if !lockHeldElsewhere(err) || time.Now().After(deadline) {
				f.Close()
				release()
				if lockHeldElsewhere(err) {
					return nil, fmt.Errorf("%w (waited %s for %s); try again when it has finished", errKeystoreBusy, keystoreLockWait, e.lock)
				}
				return nil, fmt.Errorf("cannot lock %s, so the keystores were not changed: %w", e.lock, err)
			}
			time.Sleep(25 * time.Millisecond)
		}
		held = append(held, f)
	}
	return release, nil
}

// recoverKeystore merges a keystore's previous generation (keys.json.prev) into
// it, with the keystore lock held. Keys are only ever added, so the previous
// generation normally holds nothing the keystore lacks and this changes nothing.
// When a crash or damage left the keystore behind it, or unreadable, the union is
// published: recovery never chooses one generation over the other. A previous
// generation that cannot be read, or that disagrees, refuses and is left for the
// user.
func (a *App) recoverKeystore(path string) (recovered bool, err error) {
	prevPath := path + keystorePrevSuffix
	if _, err := os.Lstat(prevPath); os.IsNotExist(err) {
		return false, nil
	}
	prev, err := a.readExistingKeystore(prevPath, false)
	if err != nil {
		return false, fmt.Errorf("the previous copy of keystore %s cannot be read; keep both files and restore by hand: %w", path, err)
	}
	cur, cerr := a.readExistingKeystore(path, false)
	if cerr == nil && keystoreCovers(cur, prev) {
		return false, nil // nothing to recover
	}
	stores := []*keystoreFile{prev}
	if cerr == nil {
		stores = append(stores, cur)
	}
	merged, err := mergeKeystores(stores)
	if err != nil {
		return false, fmt.Errorf("keystore %s and its previous copy disagree; keep both files and reconcile by hand: %w", path, err)
	}
	if err := a.publishKeystore(path, merged); err != nil {
		return false, err
	}
	return true, nil
}

// keystoreCovers reports whether cur holds every key of prev, with its secret.
func keystoreCovers(cur, prev *keystoreFile) bool {
	have := map[string]any{}
	for _, k := range cur.Keys {
		have[k["key_ref"].(string)] = k["passphrase"]
	}
	for _, k := range prev.Keys {
		if v, ok := have[k["key_ref"].(string)]; !ok || v != k["passphrase"] {
			return false
		}
	}
	return true
}

// publishKeystore writes ks to path with the keystore lock held. The previous
// generation is kept beside it (keys.json.prev) before the new one is written,
// and stays there: the new file is staged, flushed, renamed into place and read
// back as exactly ks. The previous copy is refreshed only from a keystore that
// reads whole and holds every key it already has, so it never loses a key; it is
// never removed. On a failed read-back the generation kept by this write is put
// back. Cleanup is not deferred, so a crash (a test's panic) leaves the files as
// a crash would.
func (a *App) publishKeystore(path string, ks *keystoreFile) error {
	if a.keystoreMutationObserver != nil {
		a.keystoreMutationObserver(path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ks.Marker, ks.LegacyMarker, ks.SchemaVersion = 1, 0, currentSchemaVersion
	b, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		return fmt.Errorf("keystore %s: cannot encode its keys", path)
	}
	dir := filepath.Dir(path)
	prevPath := path + keystorePrevSuffix

	// 1. Keep the previous generation.
	keptNow := false
	old, rerr := os.ReadFile(path)
	switch {
	case rerr == nil:
		cur, cerr := a.readExistingKeystore(path, false)
		refresh := cerr == nil
		if refresh {
			if _, serr := os.Lstat(prevPath); serr == nil {
				prev, perr := a.readExistingKeystore(prevPath, false)
				refresh = perr == nil && keystoreCovers(cur, prev)
			}
		}
		if refresh {
			tmp, err := writeStage(prevPath, old, 0o600)
			if err != nil {
				return fmt.Errorf("cannot keep the previous copy of keystore %s: %w", path, err)
			}
			if err := renameReplacing(tmp, prevPath); err != nil {
				_ = removeStage(tmp)
				return fmt.Errorf("cannot keep the previous copy of keystore %s: %w", path, err)
			}
			if err := flushFile(prevPath); err != nil {
				return fmt.Errorf("the previous copy of keystore %s may not have reached the disk: %w", path, err)
			}
			if err := syncDir(dir); err != nil {
				return fmt.Errorf("cannot flush keystore folder %s: %w", dir, err)
			}
			keptNow = true
		}
	case !os.IsNotExist(rerr):
		return fmt.Errorf("cannot read keystore %s before changing it: %w", path, rerr)
	}
	noteKeystoreStep("prev-kept", path)

	// 2. The new generation, staged, flushed and renamed into place.
	tmp, err := writeStage(path, b, 0o600)
	if err != nil {
		return err
	}
	noteKeystoreStep("staged", path)
	if err := renameReplacing(tmp, path); err != nil {
		_ = removeStage(tmp)
		return err
	}
	if err := flushFile(path); err != nil {
		return fmt.Errorf("keystore %s may not have reached the disk: %w", path, err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("cannot flush keystore folder %s: %w", dir, err)
	}
	noteKeystoreStep("renamed", path)

	// 3. Read it back: it must hold exactly what was written.
	if err := a.keystoreReadsBack(path, ks); err != nil {
		if keptNow {
			if t, werr := writeStage(path, old, 0o600); werr == nil {
				if renameReplacing(t, path) != nil {
					_ = removeStage(t)
				}
			}
		}
		return err
	}
	noteKeystoreStep("read-back", path)
	noteKeystoreStep("done", path)
	return nil
}

// keystoreReadsBack reads path with the strict reader and checks it holds
// exactly the keys of want.
func (a *App) keystoreReadsBack(path string, want *keystoreFile) error {
	got, err := a.readExistingKeystore(path, false)
	if err != nil {
		return fmt.Errorf("keystore %s did not read back after writing: %w", path, err)
	}
	if !sameKeyRecords(got.Keys, want.Keys) {
		return fmt.Errorf("keystore %s did not read back as written", path)
	}
	return nil
}

// sameKeyRecords compares key records by their JSON form, since a decoded
// record carries json.Number where the written one may hold an int.
func sameKeyRecords(a, b []map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return reflect.DeepEqual(a, b)
	}
	var na, nb any
	if json.Unmarshal(ja, &na) != nil || json.Unmarshal(jb, &nb) != nil {
		return bytes.Equal(ja, jb)
	}
	return reflect.DeepEqual(na, nb)
}

// flushFile asks the system to write path's data and metadata to the disk.
func flushFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// keyRefs lists the key references in ks.
func keyRefs(ks *keystoreFile) map[string]bool {
	out := map[string]bool{}
	for _, k := range ks.Keys {
		if ref, ok := k["key_ref"].(string); ok {
			out[ref] = true
		}
	}
	return out
}

// settledKeystoreCheck runs check, a read-only check of the keystores at paths
// made without their lock. One that fails may have seen another key change
// half done (one keystore written, the next not yet), so where every lock file
// is already there, and taking the lock therefore changes nothing on disk, it
// runs again under the lock and that answer stands. A change half done always
// has its lock files: they are made before its first write.
func settledKeystoreCheck(paths []string, check func() error) error {
	err := check()
	if err == nil || !keystoreLocksPresent(paths) {
		return err
	}
	unlock, lerr := lockKeystores(paths, false)
	if lerr != nil {
		return lerr
	}
	defer unlock()
	return check()
}

// settledKeystoreStatus is keystoreStatus checked as settledKeystoreCheck does.
func (a *App) settledKeystoreStatus(paths []string, allowMissing bool) map[string]any {
	var st map[string]any
	_ = settledKeystoreCheck(paths, func() error {
		st = a.keystoreStatus(paths, allowMissing)
		if !st["ok"].(bool) {
			return errors.New("not ok")
		}
		return nil
	})
	return st
}

// keystoreLocksPresent reports whether every keystore at paths has its lock file.
func keystoreLocksPresent(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		fi, err := os.Lstat(p + keystoreLockSuffix)
		if err != nil || !fi.Mode().IsRegular() {
			return false
		}
	}
	return true
}
