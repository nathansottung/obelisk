//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func keyRec(ref, pass string) map[string]any {
	return map[string]any{"key_ref": ref, "algorithm": "GPG-AES256", "passphrase": pass,
		"created_at": "2026-11-01T00:00:00Z", "note": ref}
}

// An app-state restore writes keystores as a key change: under the keystore lock
// (so a change in progress elsewhere is waited for, and a restore that cannot
// get the lock changes nothing), merged into the keystore already there, so a
// key newer than the backup is kept.
func TestAppRestore_KeystoresTakeTheLockAndKeepNewerKeys(t *testing.T) {
	src := abTestApp(t)
	abPopulate(t, src)
	ks := mustConfig(t, src).KeystorePaths[0]
	if err := writeStore(ks, &keystoreFile{Marker: 1, Keys: []map[string]any{keyRec("K-BACKUP", "pass-in-backup")}}); err != nil {
		t.Fatal(err)
	}
	res, err := src.ExportAppBackup(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}

	dst := abTestApp(t)
	dest := filepath.Join(dst.DataDir, "keystores", filepath.Base(ks))
	if err := writeStore(dest, &keystoreFile{Marker: 1, Keys: []map[string]any{keyRec("K-NEWER", "pass-only-here")}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(dest)

	// Another holder has the lock: the restore waits, then stops having changed
	// nothing.
	lf, err := os.OpenFile(dest+keystoreLockSuffix, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := takeKeystoreLock(lf); err != nil {
		t.Fatal(err)
	}
	orig := keystoreLockWait
	keystoreLockWait = 300 * time.Millisecond
	t.Cleanup(func() { keystoreLockWait = orig })
	_, rerr := dst.RestoreAppBackup(res.TarPath)
	if rerr == nil || !strings.Contains(rerr.Error(), "before changing anything") {
		t.Fatalf("a restore that could not take the keystore lock must stop before changing anything: %v", rerr)
	}
	if after, _ := os.ReadFile(dest); string(after) != string(before) {
		t.Fatal("the keystore was written while another holder had its lock")
	}
	if n := len(dst.Store.Chunks(0)); n != 0 {
		t.Fatalf("the refused restore changed the catalog: %d packages", n)
	}
	_ = unlockFile(lf)
	_ = lf.Close()

	if _, err := dst.RestoreAppBackup(res.TarPath); err != nil {
		t.Fatal(err)
	}
	got, err := dst.readExistingKeystore(dest, false)
	if err != nil {
		t.Fatal(err)
	}
	refs := keyRefs(got)
	if !refs["K-BACKUP"] || !refs["K-NEWER"] {
		t.Fatalf("the restored keystore must hold the backup's key and the newer one: %v", refs)
	}
	if _, err := os.Stat(dest + keystorePrevSuffix); err != nil {
		t.Fatalf("the restore must keep the previous generation like every key change: %v", err)
	}
}

// A backup keystore that cannot be read is never written over a keystore that
// can: the restore stops before changing anything, and the live keys stay.
func TestAppRestore_UnreadableBackupKeystoreNeverReplacesAReadableOne(t *testing.T) {
	a := abTestApp(t)
	dest := filepath.Join(a.DataDir, "keystores", "jobs.json")
	if err := writeStore(dest, &keystoreFile{Marker: 1, Keys: []map[string]any{keyRec("K-LIVE", "pass-live")}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(dest)
	bundle := orderedJobRestoreBundle(t, []rawMember{{"keystores/jobs.json", []byte(`{"keys":[{"oops":1}]}`)}}, false)
	_, err := a.RestoreAppBackup(bundle)
	if err == nil || !strings.Contains(err.Error(), "before changing anything") {
		t.Fatalf("want a refusal before any change, got %v", err)
	}
	if after, _ := os.ReadFile(dest); string(after) != string(before) {
		t.Fatal("an unreadable backup keystore replaced a readable one")
	}
}
