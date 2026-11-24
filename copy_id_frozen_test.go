//go:build !guionly

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// restoreFolderByHand restores the package in folder with only the steps of its
// RESTORE.txt and the stock tools, in a working copy that holds exactly what the
// medium holds (its copy ID file included), and returns the restored file's hash.
func restoreFolderByHand(t *testing.T, tools map[string]string, folder string) string {
	t.Helper()
	work := t.TempDir()
	copyTree(t, folder, work)
	txt, err := os.ReadFile(filepath.Join(work, "RESTORE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range handRestoreSteps(t, string(txt)) {
		if n := runHandStep(t, work, step, "", toolDirs(tools)); n != 0 {
			t.Fatalf("step %q skipped %d files", step, n)
		}
	}
	b, err := os.ReadFile(filepath.Join(work, "fixture-file.bin"))
	if err != nil {
		t.Fatalf("nothing restored: %v", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// A medium written before copy IDs, by the code of that time: on either engine its
// copy keeps its last-verified date but no credit until a full check of its folder,
// which writes nothing to the medium. It still restores by hand, and a new copy
// written from it carries an ID and restores by hand too.
func TestFrozenMedium_BeforeCopyIDs(t *testing.T) {
	tools := nativeTools(t)
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			prevSerial, prevPart := driveSerialAt, partitionIDAt
			driveSerialAt = func(string) string { return "FROZEN-SERIAL" }
			partitionIDAt = func(string) string { return "FROZEN-PARTITION" }
			t.Cleanup(func() { driveSerialAt, partitionIDAt = prevSerial, prevPart })

			root := t.TempDir()
			copyTree(t, filepath.Join(frozenRoot, "frozen", "medium-v8-before-copy-id"), root)
			data := filepath.Join(root, "data")
			if err := os.MkdirAll(data, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(root, "catalog.json"), filepath.Join(data, "catalog.json")); err != nil {
				t.Fatal(err)
			}
			var frozen struct {
				Chunks []struct {
					Copies []struct {
						LastVerifiedAt time.Time `json:"last_verified_at"`
					} `json:"copies"`
				} `json:"chunks"`
			}
			raw, err := os.ReadFile(filepath.Join(data, "catalog.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &frozen); err != nil || len(frozen.Chunks) != 1 || len(frozen.Chunks[0].Copies) != 1 {
				t.Fatalf("fixture: %v", err)
			}
			wantDate := frozen.Chunks[0].Copies[0].LastVerifiedAt
			sumRaw, err := os.ReadFile(filepath.Join(root, "restored.sha256"))
			if err != nil {
				t.Fatal(err)
			}
			wantSum := strings.TrimSpace(string(sumRaw))

			store, err := openTestStore(t, data)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			app := initializedTestApp(t, &App{DataDir: data, Store: store})
			if _, err := app.SaveConfig(map[string]any{"staging_dir": t.TempDir(), "par2_redundancy": 10, "tools": tools}); err != nil {
				t.Fatal(err)
			}
			(&frozenHome{root: root}).relocate(app)
			medium := filepath.Join(root, "drive", "archive")
			c := frozenChunk(t, app, "FROZEN-PLAIN")
			folder := filepath.Join(medium, c.Name)

			cp := c.Copies[0]
			if cp.CopyID != "" || cp.Unidentified != idNone || cp.VerifyOK != nil {
				t.Fatalf("an old copy kept its credit: %+v", cp)
			}
			if cp.LastVerifiedAt == nil || !cp.LastVerifiedAt.Equal(wantDate) {
				t.Fatalf("last verified %v, want %v", cp.LastVerifiedAt, wantDate)
			}
			if c.VerifiedCopyCount() != 0 || c.Status == "VERIFIED" {
				t.Fatalf("status %s with %d verified copies", c.Status, c.VerifiedCopyCount())
			}

			wantVerifies(t, app, c.ID, medium)
			c = frozenChunk(t, app, "FROZEN-PLAIN")
			if cp := c.Copies[0]; cp.Unidentified != "" || !verified(cp) || c.Status != "VERIFIED" {
				t.Fatalf("after a full check: status %s, %+v", c.Status, cp)
			}
			if _, err := os.Stat(filepath.Join(folder, copyIDFile)); err == nil {
				t.Fatal("an ID was written onto an existing medium")
			}
			if got := restoreFolderByHand(t, tools, folder); got != wantSum {
				t.Fatal("the old copy restored by hand differs from the original")
			}

			// A new copy, written from staging refilled from the old one.
			if _, err := app.RefillStaging(c.ID, c.Copies[0].VolumeID, "", noProg); err != nil {
				t.Fatalf("refill: %v", err)
			}
			newMedium := filepath.Join(root, "drive", "second")
			if err := os.MkdirAll(newMedium, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := app.WriteChunk(c.ID, newMedium, 0, 0, 0, c.Copies[0].VolumeID, noProg); err != nil {
				t.Fatal(err)
			}
			newFolder := filepath.Join(newMedium, c.Name)
			id := copyIDIn(t, newFolder)
			if got := restoreFolderByHand(t, tools, newFolder); got != wantSum {
				t.Fatal("the new copy restored by hand differs from the original")
			}

			// Both survive a reopen as they are.
			if err := app.Store.Close(); err != nil {
				t.Fatal(err)
			}
			if app.Store, err = openTestStore(t, data); err != nil {
				t.Fatalf("reopen: %v", err)
			}
			c = frozenChunk(t, app, "FROZEN-PLAIN")
			if cp := currentCopyAt(t, c, folder); cp.CopyID != "" || cp.Unidentified != "" || !verified(cp) {
				t.Fatalf("old copy after reopen: %+v", cp)
			}
			if cp := currentCopyAt(t, c, newFolder); cp.CopyID != id || !verified(cp) {
				t.Fatalf("new copy after reopen: %+v", cp)
			}
		})
	}
}
