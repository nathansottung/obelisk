//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubDevices makes keystoreDeviceOf answer from devs, by path prefix: the
// longest prefix listed names the device; "" in the map's value, or a path
// matching no prefix, is an unknown device.
func stubDevices(t *testing.T, devs map[string]string) {
	t.Helper()
	prev := keystoreDeviceOf
	keystoreDeviceOf = func(p string) (string, error) {
		abs, _ := filepath.Abs(p)
		best, dev := "", ""
		for prefix, d := range devs {
			if within(prefix, abs) && len(prefix) > len(best) {
				best, dev = prefix, d
			}
		}
		if dev == "" {
			return "", errKeystoreDiskUnknown
		}
		return dev, nil
	}
	t.Cleanup(func() { keystoreDeviceOf = prev })
}

// writeKeystoreStub puts a keystore file at p, for placement checks, which
// place a keystore only by its own file.
func writeKeystoreStub(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p, `{"keys":[]}`)
}

func within(prefix, p string) bool {
	rel, err := filepath.Rel(prefix, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func TestKeyPlacement_Rule(t *testing.T) {
	root := t.TempDir()
	ks1, ks2 := filepath.Join(root, "k1", "keys.json"), filepath.Join(root, "k2", "keys.json")
	dest, other, unknown := filepath.Join(root, "k1", "packages"), filepath.Join(root, "d"), filepath.Join(root, "u")
	stubDevices(t, map[string]string{
		filepath.Join(root, "k1"): "disk-1",
		filepath.Join(root, "k2"): "disk-2" + deviceSeparator + "disk-3", // a volume over two disks
		filepath.Join(root, "d"):  "disk-4",
		filepath.Join(root, "s"):  "disk-3",
	})
	st, err := openTestStore(t, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{Store: st}
	enc := &Chunk{Name: "ENC-1", Encrypted: true}
	unknownKS := filepath.Join(root, "u", "keys.json")
	for _, p := range []string{ks1, ks2, unknownKS} {
		writeKeystoreStub(t, p)
	}
	keys := []string{ks1, ks2}

	t.Run("plaintext is not checked", func(t *testing.T) {
		if n, err := app.checkKeyPlacement(&Chunk{Name: "PLAIN"}, dest, keys, false); err != nil || n != "" {
			t.Fatalf("%q %v", n, err)
		}
	})
	t.Run("another drive is allowed", func(t *testing.T) {
		if n, err := app.checkKeyPlacement(enc, other, keys, false); err != nil || n != "" {
			t.Fatalf("%q %v", n, err)
		}
	})
	for name, d := range map[string]string{"same drive": dest, "shares one disk": filepath.Join(root, "s")} {
		t.Run(name+" refuses", func(t *testing.T) {
			_, err := app.checkKeyPlacement(enc, d, keys, false)
			var kd *KeyDriveError
			if !errors.As(err, &kd) || len(kd.Same) != 1 {
				t.Fatalf("not refused as a key drive: %v", err)
			}
			if !strings.Contains(err.Error(), "allow_key_drive") || !strings.Contains(err.Error(), "same drive") {
				t.Errorf("refusal does not say what to do: %v", err)
			}
		})
	}
	t.Run("unknown destination refuses", func(t *testing.T) {
		_, err := app.checkKeyPlacement(enc, unknown, keys, false)
		var kd *KeyDriveError
		if !errors.As(err, &kd) || len(kd.Unknown) != 1 || kd.Unknown[0] != unknown {
			t.Fatalf("unknown destination not refused: %v", err)
		}
	})
	t.Run("unknown keystore refuses", func(t *testing.T) {
		_, err := app.checkKeyPlacement(enc, other, []string{ks1, unknownKS}, false)
		var kd *KeyDriveError
		if !errors.As(err, &kd) || len(kd.Unknown) != 1 {
			t.Fatalf("unknown keystore not refused: %v", err)
		}
	})
	t.Run("override gives a notice to act on", func(t *testing.T) {
		n, err := app.checkKeyPlacement(enc, dest, keys, true)
		if err != nil || !strings.HasPrefix(n, "ACTION NEEDED") || !strings.Contains(n, ks1) {
			t.Fatalf("%q %v", n, err)
		}
		if a := app.Store.LastAudit(); a == nil || a.Detail != n {
			t.Fatal("the override was not logged")
		}
	})
}

// treeSnapshot lists every file under root with its size.
func treeSnapshot(t *testing.T, root string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, e := d.Info(); e == nil {
				out[p] = fi.Size()
			}
		}
		return nil
	})
	return out
}

// An encrypted package is not written to the drive that holds a keystore, by
// the API or by the write itself; the override writes it and says what to do.
func TestKeyPlacement_EncryptedWriteToKeystoreDrive(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	cfg, _ := app.LoadConfig()
	keyDir := filepath.Dir(cfg.KeystorePaths[0])
	src, refs := makeSource(t)
	c := app.Store.AddChunk(Chunk{
		Name: "ENC-KEYDRIVE", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: true,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("build: %v", err)
	}
	_ = staging

	// The keystore's own drive, seen through the test's folder-as-device rule.
	before := treeSnapshot(t, keyDir)
	_, err := app.WriteChunk(c.ID, keyDir, 0, 0, 0, 0, noProg)
	var kd *KeyDriveError
	if !errors.As(err, &kd) {
		t.Fatalf("write to the keystore's drive: %v", err)
	}
	mux := http.NewServeMux()
	api(mux, app)
	body, _ := json.Marshal(map[string]any{"dest_dir": keyDir})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", fmt.Sprintf("/api/chunks/%d/write", c.ID), bytes.NewReader(body)))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"key_drive":true`) {
		t.Fatalf("API write to the keystore's drive: %d %s", rec.Code, rec.Body.String())
	}
	if after := treeSnapshot(t, keyDir); len(after) != len(before) {
		t.Fatalf("something was written to the keystore's drive: %v", after)
	}

	// The user chose the override: written, with the notice.
	res, err := app.WriteChunk(c.ID, keyDir, 0, 0, 0, 0, noProg, PackageWriteOptions{AllowKeyDrive: true})
	if err != nil {
		t.Fatalf("overridden write: %v", err)
	}
	if n, _ := res["key_drive_notice"].(string); !strings.HasPrefix(n, "ACTION NEEDED") {
		t.Fatalf("overridden write gave no notice: %v", res)
	}
}

// A build whose staging folder is on a keystore's drive refuses before the job
// starts (409) and in the build itself.
func TestKeyPlacement_BuildStagingOnKeystoreDrive(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	cfg, _ := app.LoadConfig()
	stubDevices(t, map[string]string{
		staging:                            "disk-1",
		filepath.Dir(cfg.KeystorePaths[0]): "disk-1",
		filepath.Dir(cfg.KeystorePaths[1]): "disk-2",
	})
	c := app.Store.AddChunk(Chunk{Name: "ENC-STAGE", Status: "PLANNED", Encrypted: true, HashAlg: "SHA256", Par2: 5})
	keysBefore := len(app.Store.KeyMetas())
	err := app.BuildChunk(c.ID, noProg)
	var kd *KeyDriveError
	if !errors.As(err, &kd) {
		t.Fatalf("build staged on the keystore's drive: %v", err)
	}
	if len(app.Store.KeyMetas()) != keysBefore || app.Store.Chunk(c.ID).KeyRef != "" {
		t.Fatal("a refused build made or took a key")
	}
	mux := http.NewServeMux()
	api(mux, app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", fmt.Sprintf("/api/chunks/%d/build", c.ID), strings.NewReader("{}")))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"key_drive":true`) {
		t.Fatalf("API build staged on the keystore's drive: %d %s", rec.Code, rec.Body.String())
	}
}

// The sentinel check: no passphrase appears in any log, job record or error
// message from the key paths — generate, sync (agreeing, disagreeing, refused,
// failed partway), crash recovery, an encrypted build job, a refused key-drive
// write and its override.
func TestKeyPaths_NoPassphraseInLogsJobsOrErrors(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	cfg, _ := app.LoadConfig()
	var secrets, msgs []string
	note := func(err error) {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	_, pass, _, err := app.GenerateKey("sentinel")
	note(err)
	secrets = append(secrets, pass)
	rep, err := app.syncKeystores()
	note(err)
	rb, _ := json.Marshal(rep)
	msgs = append(msgs, string(rb))

	// Disagreeing keystores, then a conflict, then a failed publication.
	ks0, _ := readStore(cfg.KeystorePaths[0])
	ks0.Keys = append(ks0.Keys, kvKey("K-SENTINEL", "sentinel-secret-one"))
	_ = writeStore(cfg.KeystorePaths[0], ks0)
	secrets = append(secrets, "sentinel-secret-one")
	rep, err = app.syncKeystores()
	note(err)
	rb, _ = json.Marshal(rep)
	msgs = append(msgs, string(rb))
	ks1, _ := readStore(cfg.KeystorePaths[1])
	setSentinel := func(secret string) {
		for _, k := range ks1.Keys {
			if k["key_ref"] == "K-SENTINEL" {
				k["passphrase"] = secret
			}
		}
		_ = writeStore(cfg.KeystorePaths[1], ks1)
	}
	setSentinel("sentinel-secret-two")
	secrets = append(secrets, "sentinel-secret-two")
	_, err = app.SyncKeystores()
	if err == nil {
		t.Fatal("conflicting secrets were synced")
	}
	note(err)
	setSentinel("sentinel-secret-one")
	prevRename := renameFile
	failPublicationTo(t, cfg.KeystorePaths[1])
	_, _, _, err = app.GenerateKey("sentinel")
	note(err)
	renameFile = prevRename

	// A crash between the keystore writes, then recovery.
	crashKeystoreAt(t, "renamed", cfg.KeystorePaths[0])
	runUntilCrash(t, func() { _, _, _, err := app.GenerateKey("sentinel"); note(err) })
	keystoreWriteStep = nil
	rep, err = app.syncKeystores()
	note(err)
	rb, _ = json.Marshal(rep)
	msgs = append(msgs, string(rb))

	// An encrypted build as a job, and a refused then overridden key-drive write.
	src, refs := makeSource(t)
	c := app.Store.AddChunk(Chunk{
		Name: "ENC-SENTINEL", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: true,
		Files: append([]ChunkFileRef{}, refs...),
	})
	mux := http.NewServeMux()
	api(mux, app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", fmt.Sprintf("/api/chunks/%d/build", c.ID), strings.NewReader("{}")))
	var started struct {
		JobID int `json:"job_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || started.JobID == 0 {
		t.Fatalf("build did not start: %s", rec.Body.String())
	}
	if j := waitTerminalJob(t, app.Store, started.JobID); j.Status != "COMPLETED" {
		t.Fatalf("build job: %+v", j)
	}
	if p, err := app.Passphrase(app.Store.Chunk(c.ID).KeyRef); err == nil {
		secrets = append(secrets, p)
	} else {
		t.Fatal(err)
	}
	keyDir := filepath.Dir(cfg.KeystorePaths[0])
	body, _ := json.Marshal(map[string]any{"dest_dir": keyDir})
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", fmt.Sprintf("/api/chunks/%d/write", c.ID), bytes.NewReader(body)))
	msgs = append(msgs, rec.Body.String())
	_, err = app.WriteChunk(c.ID, keyDir, 0, 0, 0, 0, noProg)
	note(err)
	res, err := app.WriteChunk(c.ID, keyDir, 0, 0, 0, 0, noProg, PackageWriteOptions{AllowKeyDrive: true})
	note(err)
	rb, _ = json.Marshal(res)
	msgs = append(msgs, string(rb))

	// Everything the app recorded: its data folder (catalog, audit log, job
	// board, on either engine), the job board as served, and every message.
	jobs, _ := json.Marshal(app.Store.Jobs())
	msgs = append(msgs, string(jobs))
	_ = filepath.WalkDir(app.DataDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if b, e := os.ReadFile(p); e == nil {
				msgs = append(msgs, string(b))
			}
		}
		return nil
	})
	if len(secrets) < 4 {
		t.Fatalf("only %d secrets collected", len(secrets))
	}
	for i, s := range secrets {
		if s == "" {
			t.Fatal("an empty secret was collected")
		}
		for j, m := range msgs {
			if strings.Contains(m, s) {
				// Never print the text: it holds the secret.
				t.Fatalf("passphrase %d appeared in recorded text %d of %d", i, j, len(msgs))
			}
		}
	}
}
