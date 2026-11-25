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

// A keystore whose drive is unplugged is placed by the drive it was last seen
// on: being put away is not a reason to refuse every write, and a write to the
// drive it lives on still refuses. A keystore never seen online stays unknown,
// and says to reconnect it rather than to move it.
func TestKeyPlacement_UnpluggedKeystoreUsesLastSeenDrive(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	root := t.TempDir()
	ks1, ks2 := filepath.Join(root, "k1", "keys.json"), filepath.Join(root, "k2", "keys.json")
	other, onK2 := filepath.Join(root, "d"), filepath.Join(root, "k2dest")
	devs := map[string]string{
		filepath.Join(root, "k1"):     "disk-1",
		filepath.Join(root, "k2"):     "disk-2",
		filepath.Join(root, "d"):      "disk-4",
		filepath.Join(root, "k2dest"): "disk-2",
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

	if _, err := app.checkKeyPlacement(enc, other, keys, false); err != nil {
		t.Fatalf("both keystores present: %v", err)
	}
	unplugged[filepath.Join(root, "k2")] = true
	if _, err := app.checkKeyPlacement(enc, other, keys, false); err != nil {
		t.Fatalf("keystore 2 put away, writing to another drive: %v", err)
	}
	_, err := app.checkKeyPlacement(enc, onK2, keys, false)
	var kd *KeyDriveError
	if !errors.As(err, &kd) || len(kd.Same) != 1 || kd.Same[0] != ks2 {
		t.Fatalf("a write to the put-away keystore's drive: %v", err)
	}
	// What was last seen outlives this run.
	fresh := &App{DataDir: app.DataDir, Store: app.Store}
	if _, err := fresh.checkKeyPlacement(enc, other, keys, false); err != nil {
		t.Fatalf("after a restart: %v", err)
	}

	ks3 := filepath.Join(root, "k3", "keys.json")
	devs[filepath.Join(root, "k3")] = "disk-5"
	unplugged[filepath.Join(root, "k3")] = true
	_, err = app.checkKeyPlacement(enc, other, []string{ks1, ks3}, false)
	if !errors.As(err, &kd) || len(kd.Unseen) != 1 || kd.Unseen[0] != ks3 || len(kd.Unknown) != 0 {
		t.Fatalf("a keystore never seen: %v", err)
	}
	if !strings.Contains(err.Error(), "reconnect") {
		t.Errorf("the refusal does not say to reconnect the keystore: %v", err)
	}
	n, err := app.checkKeyPlacement(enc, other, []string{ks1, ks3}, true)
	if err != nil || !strings.Contains(strings.ToLower(n), "reconnect") || strings.Contains(n, "Move the keystore") {
		t.Fatalf("the override notice for an unseen keystore: %q %v", n, err)
	}
}
