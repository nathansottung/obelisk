//go:build !guionly

package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// withRealKeystoreDevices runs the rest of the test with the real device lookup
// (tests otherwise treat each folder as its own device; see testmain_test.go).
func withRealKeystoreDevices(t *testing.T) {
	t.Helper()
	prev := keystoreDeviceOf
	keystoreDeviceOf = realKeystoreDevice
	t.Cleanup(func() { keystoreDeviceOf = prev })
}

// Two keystores must be two files on two devices: the same path twice, a
// letter-case variant of one, or two on one device are refused when saved and
// are not a valid set where keystores are used.
func TestKeystores_RefuseDuplicatesAliasesAndOneDevice(t *testing.T) {
	withRealKeystoreDevices(t)
	app := dockApp(t)
	a := filepath.Join(t.TempDir(), "keys.json")
	b := filepath.Join(t.TempDir(), "keys.json") // another folder, same device (C:)
	for name, paths := range map[string][]string{
		"the same path twice": {a, a},
		"a case variant":      {a, strings.ToUpper(a[:1]) + strings.ToUpper(a[1:])},
		"one device":          {a, b},
	} {
		if _, err := app.SaveConfig(map[string]any{"keystore_paths": paths}); err == nil {
			t.Errorf("%s: accepted keystore_paths %v", name, paths)
		}
		if st := app.keystoreStatus(paths, true); st["ok"] == true {
			t.Errorf("%s: %v reported as a valid keystore set", name, paths)
		}
	}
}

// A restore does not point all its keystores at the one data folder: when the
// restored set would share a device, the current keystore paths are kept and
// the restore says where the restored files are.
func TestRestoreAppBackup_DoesNotCollapseKeystores(t *testing.T) {
	app, dataDir := compatApp(t)
	k1 := filepath.Join(t.TempDir(), "k1.json")
	k2 := filepath.Join(t.TempDir(), "k2.json")
	writeKeysForTest(t, k1, "K1")
	writeKeysForTest(t, k2, "K1")
	cfg := mustConfig(t, app)
	cfg.KeystorePaths = []string{k1, k2}
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(t.TempDir(), "obelisk-appbackup.tar")
	if _, err := app.exportAppBackupTo(tarPath, true); err != nil {
		t.Fatal(err)
	}
	res, err := app.RestoreAppBackup(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	got := mustConfig(t, app).KeystorePaths
	inData := 0
	for _, p := range got {
		if strings.HasPrefix(filepath.Clean(p), filepath.Join(dataDir, "keystores")) {
			inData++
		}
	}
	if inData > 1 {
		t.Fatalf("the restore pointed %d keystores at the one data folder: %v", inData, got)
	}
	if res.KeystoreNote == "" {
		t.Error("the restore did not say where the restored keystores are")
	}
}

// A keystore on a device that is not plugged in cannot be placed on a device.
// That is no reason to refuse the set: saving settings still works, and the
// keystore status says the keystore is unreachable rather than naming the
// device rule.
func TestKeystores_UnpluggedKeystoreDoesNotBlockSaves(t *testing.T) {
	prev := keystoreDeviceOf
	keystoreDeviceOf = func(p string) (string, error) {
		if strings.Contains(p, "unplugged") {
			return "", errUnpluggedForTest
		}
		return prev(p)
	}
	t.Cleanup(func() { keystoreDeviceOf = prev })
	app := dockApp(t)
	a := filepath.Join(t.TempDir(), "keys.json")
	u := filepath.Join(t.TempDir(), "unplugged", "keys.json")
	paths := []any{a, u}
	if _, err := app.SaveConfig(map[string]any{"keystore_paths": paths}); err != nil {
		t.Fatalf("keystore paths with one unplugged: %v", err)
	}
	if _, err := app.SaveConfig(map[string]any{"keystore_paths": paths, "par2_extra_args": "-t2"}); err != nil {
		t.Fatalf("a settings save with one keystore unplugged: %v", err)
	}
	st := app.keystoreStatus([]string{a, u}, false)
	if reason, _ := st["reason"].(string); strings.Contains(reason, "device") || st["ok"] == true {
		t.Fatalf("status of a set with an unplugged keystore: %v", st)
	}
}

var errUnpluggedForTest = errors.New("no existing parent folder")
