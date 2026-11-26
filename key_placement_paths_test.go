//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// postAPI sends one API request to app and returns the recorder.
func postAPI(t *testing.T, app *App, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	api(mux, app)
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", path, bytes.NewReader(b)))
	return rec
}

func wantKeyDrive409(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"key_drive":true`) {
		t.Fatalf("%s: want a key-drive refusal, got %d %s", what, rec.Code, rec.Body.String())
	}
}

// wantPrepareKeyDrive prepares a gated op and wants it refused as a key drive,
// answered over HTTP with key_drive set so the page can offer the override.
func wantPrepareKeyDrive(t *testing.T, app *App, op string, params map[string]any) {
	t.Helper()
	_, err := app.PrepareDestructive(testSession, op, params)
	var kd *KeyDriveError
	if !errors.As(err, &kd) {
		t.Fatalf("%s prepare: want a key-drive refusal, got %v", op, err)
	}
	rec := httptest.NewRecorder()
	destructiveHTTPError(rec, err)
	wantKeyDrive409(t, op+" prepare over HTTP", rec)
	allowed := map[string]any{"allow_key_drive": true}
	for k, v := range params {
		allowed[k] = v
	}
	if _, err := app.PrepareDestructive(testSession, op, allowed); err != nil {
		t.Fatalf("%s prepare with the override: %v", op, err)
	}
}

// A refill of staging copies the encrypted payload into staging, so staging on
// a keystore's drive refuses there as it does for a build.
func TestKeyPlacement_RefillStagingOnKeystoreDrive(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "ENC-REFILL")
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := app.LoadConfig()
	stubDevices(t, map[string]string{
		staging:                            "disk-1",
		filepath.Dir(cfg.KeystorePaths[0]): "disk-1",
		filepath.Dir(cfg.KeystorePaths[1]): "disk-2",
		medium:                             "disk-3",
	})
	cp := currentCopyAt(t, c, filepath.Join(medium, c.Name))
	_, err := app.RefillStaging(c.ID, cp.VolumeID, "", noProg)
	var kd *KeyDriveError
	if !errors.As(err, &kd) {
		t.Fatalf("refill into staging on the keystore's drive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, c.Name)); err == nil {
		t.Fatal("the refused refill wrote staging")
	}
	wantKeyDrive409(t, "API refill", postAPI(t, app, fmt.Sprintf("/api/chunks/%d/refill", c.ID), map[string]any{"volume_id": cp.VolumeID}))

	res, err := app.RefillStaging(c.ID, cp.VolumeID, "", noProg, PackageWriteOptions{AllowKeyDrive: true})
	if err != nil {
		t.Fatalf("overridden refill: %v", err)
	}
	if n, _ := res["key_drive_notice"].(string); !strings.HasPrefix(n, "ACTION NEEDED") {
		t.Fatalf("overridden refill gave no notice: %v", res)
	}
}

// A write whose target already holds files is asked about the key drive first,
// before the write-over confirmation, and the write-over's own confirmation
// refuses (and offers the override) too, so the user never confirms an
// overwrite only to see the job fail.
func TestKeyPlacement_WriteOverOnKeystoreDrive(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "ENC-OVER", false)
	cfg, _ := app.LoadConfig()
	keyDir := filepath.Dir(cfg.KeystorePaths[0])
	if err := os.MkdirAll(filepath.Join(keyDir, c.Name), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(keyDir, c.Name, "earlier.txt"), "an earlier partial write")

	wantKeyDrive409(t, "API write", postAPI(t, app, fmt.Sprintf("/api/chunks/%d/write", c.ID), map[string]any{"dest_dir": keyDir}))
	wantPrepareKeyDrive(t, app, "write-over", map[string]any{"id": float64(c.ID), "dest_dir": keyDir})
}

func TestKeyPlacement_SpanWriteOverOnKeystoreDrive(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "ENC-SPANOVER", true)
	cfg, _ := app.LoadConfig()
	keyDir := filepath.Dir(cfg.KeystorePaths[0])
	if err := os.MkdirAll(filepath.Join(keyDir, c.Name), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(keyDir, c.Name, "earlier.txt"), "an earlier partial write")

	wantKeyDrive409(t, "API span-write", postAPI(t, app, fmt.Sprintf("/api/chunks/%d/span-write", c.ID), map[string]any{"dest_dir": keyDir}))
	wantPrepareKeyDrive(t, app, "span-write-over", map[string]any{"id": float64(c.ID), "dest_dir": keyDir})
}

// Re-writing a copy writes the encrypted package to the copy's drive again.
func TestKeyPlacement_RewriteCopyOnKeystoreDrive(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "ENC-REWRITE")
	cfg, _ := app.LoadConfig()
	stubDevices(t, map[string]string{
		staging:                            "disk-3",
		filepath.Dir(cfg.KeystorePaths[0]): "disk-1",
		filepath.Dir(cfg.KeystorePaths[1]): "disk-2",
		medium:                             "disk-1",
	})
	cp := currentCopyAt(t, c, filepath.Join(medium, c.Name))
	wantPrepareKeyDrive(t, app, "rewrite-copy", map[string]any{"id": float64(c.ID), "volume_id": float64(cp.VolumeID)})
}

// A keystore whose drive is not connected cannot be placed. Its drive number
// is no guide: disk numbers are handed out as drives are plugged in, so the
// put-away keystore's drive can come back under another number (and another
// letter) and be the very destination. So it refuses, with nothing remembered
// from an earlier sighting, and says to reconnect it rather than to move it.
func TestKeyPlacement_DisconnectedKeystoreRefusesAndSaysReconnect(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	root := t.TempDir()
	ks1, ks2 := filepath.Join(root, "k1", "keys.json"), filepath.Join(root, "k2", "keys.json")
	back := filepath.Join(root, "f") // keystore 2's drive, plugged in again elsewhere
	devs := map[string]string{
		filepath.Join(root, "k1"): "disk-1",
		filepath.Join(root, "k2"): "disk-2",
		back:                      "disk-3",
	}
	unplugged := map[string]bool{}
	prev := keystoreDeviceOf
	keystoreDeviceOf = func(p string) (string, error) {
		abs, _ := filepath.Abs(p)
		best, dev := "", ""
		for prefix, d := range devs {
			if within(prefix, abs) && len(prefix) > len(best) {
				best, dev = prefix, d
			}
		}
		if unplugged[best] {
			return "", errNoExistingParent
		}
		if dev == "" {
			return "", errKeystoreDiskUnknown
		}
		return dev, nil
	}
	t.Cleanup(func() { keystoreDeviceOf = prev })
	enc := &Chunk{Name: "ENC-AWAY", Encrypted: true}
	keys := []string{ks1, ks2}
	writeKeystoreStub(t, ks1)
	writeKeystoreStub(t, ks2)

	// Seen connected first, then put away and plugged in again as disk-3.
	if _, err := app.checkKeyPlacement(enc, back, keys, false); err != nil {
		t.Fatalf("both keystores present, another drive: %v", err)
	}
	unplugged[filepath.Join(root, "k2")] = true
	_, err := app.checkKeyPlacement(enc, back, keys, false)
	var kd *KeyDriveError
	if !errors.As(err, &kd) || len(kd.Offline) != 1 || kd.Offline[0] != ks2 || len(kd.Unknown) != 0 {
		t.Fatalf("a write while keystore 2 is not connected: %v", err)
	}
	if !strings.Contains(err.Error(), "reconnect") {
		t.Errorf("the refusal does not say to reconnect the keystore: %v", err)
	}
	n, err := app.checkKeyPlacement(enc, back, keys, true)
	if err != nil || !strings.Contains(strings.ToLower(n), "reconnect") || strings.Contains(n, "Move the keystore") {
		t.Fatalf("the override notice for a disconnected keystore: %q %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(app.DataDir, "keystore-drives.json")); err == nil {
		t.Fatal("a keystore's drive number was remembered")
	}
}

// A keystore whose file is not there is not placed by the folder above it: the
// drive now at its letter may be another drive, while the keystore's own drive
// is plugged in elsewhere and is the destination. It refuses as disconnected.
func TestKeyPlacement_MissingKeystoreFileIsNotPlacedByItsFolder(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	root := t.TempDir()
	ks1, ks2 := filepath.Join(root, "k1", "keys.json"), filepath.Join(root, "k2", "keys.json")
	for _, p := range []string{ks1, filepath.Dir(ks2)} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, ks1, `{"keys":[]}`)
	if err := os.MkdirAll(filepath.Dir(ks2), 0o755); err != nil {
		t.Fatal(err)
	}
	// k2 is now another drive at the keystore's letter; f is the keystore's drive.
	back := filepath.Join(root, "f")
	stubDevices(t, map[string]string{
		filepath.Join(root, "k1"): "disk-1",
		filepath.Join(root, "k2"): "disk-7",
		back:                      "disk-3",
	})
	_, err := app.checkKeyPlacement(&Chunk{Name: "ENC-GONE", Encrypted: true}, back, []string{ks1, ks2}, false)
	var kd *KeyDriveError
	if !errors.As(err, &kd) || len(kd.Offline) != 1 || kd.Offline[0] != ks2 {
		t.Fatalf("a keystore whose file is missing: %v", err)
	}
}
