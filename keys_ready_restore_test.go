//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The media format does not change with the shared key. A package from before,
// made with its own key, and two new packages made with one shared key, all
// restore by hand from their RESTORE.txt with par2, gpg and tar alone.
func TestHandRestore_OldPerPackageKeyAndNewSharedKey(t *testing.T) {
	tools := nativeTools(t)

	t.Run("old package with its own key (frozen)", func(t *testing.T) {
		root := filepath.Join("testdata", "frozen", "catalog-v8-packages")
		raw, err := os.ReadFile(filepath.Join(root, "keystore.json"))
		if err != nil {
			t.Fatal(err)
		}
		var ks struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.Unmarshal(raw, &ks); err != nil {
			t.Fatal(err)
		}
		pass := ""
		for _, k := range ks.Keys {
			if k["key_ref"] == "K-977128B4" {
				pass, _ = k["passphrase"].(string)
			}
		}
		if pass == "" {
			t.Fatal("the frozen keystore has no key K-977128B4")
		}
		out := t.TempDir()
		gatherMedia(t, out, filepath.Join(root, "medium", "FROZEN-SINGLE")) // copies; the fixture is only read
		txt, err := os.ReadFile(filepath.Join(out, "RESTORE.txt"))
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range handRestoreSteps(t, string(txt)) {
			runHandStep(t, out, step, pass, toolDirs(tools))
		}
		want, err := os.ReadFile(filepath.Join(root, "source", "fixture-file.bin"))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		_ = filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() == "fixture-file.bin" {
				got, _ := os.ReadFile(p)
				found = bytes.Equal(got, want)
			}
			return nil
		})
		if !found {
			t.Fatal("the old package did not restore by hand to its source bytes")
		}
	})

	t.Run("two new packages with one shared key", func(t *testing.T) {
		app, _ := newTestApp(t, tools)
		var shared string
		for _, name := range []string{"SHAREDA", "SHAREDB"} {
			src, refs := handSource(t, 0)
			c := app.Store.AddChunk(Chunk{Name: name, Status: "PLANNED", MediaKind: "CUSTOM",
				TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs), SrcRoot: src,
				HashAlg: "SHA256", Par2: 5, Encrypted: true, Files: append([]ChunkFileRef{}, refs...)})
			if err := app.BuildChunk(c.ID, noProg); err != nil {
				t.Fatal(err)
			}
			medium := t.TempDir()
			if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
				t.Fatal(err)
			}
			c = app.Store.Chunk(c.ID)
			if shared == "" {
				shared = c.KeyRef
			} else if c.KeyRef != shared {
				t.Fatalf("the second package has key %s, not the shared %s", c.KeyRef, shared)
			}
			pass, err := app.Passphrase(c.KeyRef)
			if err != nil {
				t.Fatal(err)
			}
			out := t.TempDir()
			gatherMedia(t, out, filepath.Join(medium, c.Name))
			txt, err := os.ReadFile(filepath.Join(out, "RESTORE.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(txt, []byte(shared)) || !bytes.Contains(txt, []byte("KEY: this package opens with key")) {
				t.Fatalf("RESTORE.txt must name the shared key:\n%s", txt)
			}
			skips := 0
			for _, step := range handRestoreSteps(t, string(txt)) {
				skips += runHandStep(t, out, step, pass, toolDirs(tools))
			}
			assertHandRestored(t, src, out, refs, skips)
		}
	})
}
