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
// Each keystore write keeps the previous generation (keys.json.prev) until the
// new file has been renamed into place, flushed and read back. A crash at any
// point leaves either the new file whole, or the old file whole beside a
// .prev copy of it; the next key change, and every passphrase lookup, reads the
// .prev copy too, so a key that was in a keystore before the crash is still
// readable after it. Keys are only ever added, so recovering merges the two
// generations and never chooses one over the other.

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

// recoverKeystore finishes a keystore write a crash interrupted. Called with the
// keystore lock held. A keystore with no .prev copy needs nothing. Otherwise the
// two generations are merged (keys are only added, so the union loses nothing)
// and the result published; a missing or unreadable keystore is rebuilt from its
// .prev copy. A .prev copy that cannot be read is left for the user and refuses.
func (a *App) recoverKeystore(path string) (recovered bool, err error) {
	prevPath := path + keystorePrevSuffix
	if _, err := os.Lstat(prevPath); os.IsNotExist(err) {
		return false, nil
	}
	prev, err := a.readExistingKeystore(prevPath, false)
	if err != nil {
		return false, fmt.Errorf("keystore %s has an earlier copy from an interrupted write that cannot be read; keep both files and restore by hand: %w", path, err)
	}
	stores := []*keystoreFile{prev}
	if cur, cerr := a.readExistingKeystore(path, false); cerr == nil {
		stores = append(stores, cur)
	}
	merged, err := mergeKeystores(stores)
	if err != nil {
		return false, fmt.Errorf("keystore %s and its earlier copy from an interrupted write disagree; keep both files and reconcile by hand: %w", path, err)
	}
	if err := a.publishKeystore(path, merged); err != nil {
		return false, err
	}
	return true, nil
}

// publishKeystore writes ks to path with the keystore lock held, keeping the
// previous generation until the new one is on disk and reads back as exactly
// ks. On a failed read-back the previous generation is put back. Cleanup is not
// deferred, so a crash (a test's panic) leaves the files as a crash would.
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

	// 1. Keep the previous generation. One left by an interrupted write is kept
	// as it is: recoverKeystore merged it into what is being written now.
	havePrev, madePrev := false, false
	if _, err := os.Lstat(prevPath); err == nil {
		havePrev = true
	} else if old, rerr := os.ReadFile(path); rerr == nil {
		tmp, err := writeStage(prevPath, old, 0o600)
		if err != nil {
			return fmt.Errorf("cannot keep the previous copy of keystore %s: %w", path, err)
		}
		if err := renameReplacing(tmp, prevPath); err != nil {
			_ = removeStage(tmp)
			return fmt.Errorf("cannot keep the previous copy of keystore %s: %w", path, err)
		}
		if err := syncDir(dir); err != nil {
			return fmt.Errorf("cannot flush keystore folder %s: %w", dir, err)
		}
		havePrev, madePrev = true, true
	} else if !os.IsNotExist(rerr) {
		return fmt.Errorf("cannot read keystore %s before changing it: %w", path, rerr)
	}
	noteKeystoreStep("prev-kept", path)

	// 2. The new generation, staged, flushed and renamed into place. If it never
	// takes the name, the keystore is unchanged and a copy made here is let go.
	dropMadePrev := func() {
		if madePrev {
			_ = os.Remove(prevPath)
		}
	}
	tmp, err := writeStage(path, b, 0o600)
	if err != nil {
		dropMadePrev()
		return err
	}
	noteKeystoreStep("staged", path)
	if err := renameReplacing(tmp, path); err != nil {
		_ = removeStage(tmp)
		dropMadePrev()
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
		if havePrev {
			if old, rerr := os.ReadFile(prevPath); rerr == nil {
				if t, werr := writeStage(path, old, 0o600); werr == nil {
					if renameReplacing(t, path) != nil {
						_ = removeStage(t)
					}
				}
			}
		}
		return err
	}
	noteKeystoreStep("read-back", path)

	// 4. Only now is the previous generation let go.
	if havePrev {
		if err := os.Remove(prevPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("keystore %s was written, but its previous copy could not be removed: %w", path, err)
		}
		_ = syncDir(dir)
	}
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
