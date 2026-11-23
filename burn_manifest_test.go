//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// privateBurn is a queued private-manifest package with its plaintext manifest staged.
func privateBurn(t *testing.T) (*App, int, *Chunk, string) {
	t.Helper()
	app, qid, _ := burnSetupForTest(t, "Disc")
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	c := app.Store.Chunk(app.Store.BurnQueue(qid).Discs[0].ChunkID)
	c.PrivateManifest, c.Encrypted = true, true
	app.Store.UpdateChunk(c)
	plain := filepath.Join(c.StagedDir, c.Name+".manifest.json")
	if err := os.WriteFile(plain, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	return app, qid, c, plain
}

func onBurn(t *testing.T, fn func()) {
	t.Helper()
	prev := runBurnProgram
	runBurnProgram = func([]string) error {
		fn()
		return nil
	}
	t.Cleanup(func() { runBurnProgram = prev })
}

// A manifest left aside by a crashed burn goes back before the next burn.
func TestBurn_ManifestLeftByACrashIsPutBack(t *testing.T) {
	app, qid, c, plain := privateBurn(t)
	hidden := filepath.Join(filepath.Dir(c.StagedDir), "."+c.Name+".manifest.hidden")
	if err := os.Rename(plain, hidden); err != nil {
		t.Fatal(err)
	}
	onBurn(t, func() {
		if _, err := os.Stat(plain); err == nil {
			t.Error("the manifest was in the folder during the burn")
		}
	})
	if _, err := app.BurnNext(qid, func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(plain); err != nil {
		t.Fatalf("the manifest was not put back: %v", err)
	}
	if _, err := os.Stat(hidden); !os.IsNotExist(err) {
		t.Fatalf("the hidden copy is still there: %v", err)
	}
}

// A manifest that could not be put back is reported, and the good burn stands.
func TestBurn_FailedPutBackIsReported(t *testing.T) {
	app, qid, _, _ := privateBurn(t)
	onBurn(t, func() {})
	prev := moveManifestBack
	moveManifestBack = func(string, string) error { return os.ErrPermission }
	t.Cleanup(func() { moveManifestBack = prev })
	if _, err := app.BurnNext(qid, func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	d := app.Store.BurnQueue(qid).Discs[0]
	if d.Status != "DONE" || !strings.Contains(d.Detail, "could not be put back") {
		t.Fatalf("status %q, detail %q", d.Status, d.Detail)
	}
}

// Another package's plaintext inside the folder must never reach the disc.
func TestBurn_RefusedWhenTheFolderHoldsAnotherPackagesManifest(t *testing.T) {
	for name, rel := range map[string]string{
		"a nested package":   filepath.Join("B", "B.manifest.json"),
		"a crashed burn's":   ".B.manifest.hidden",
		"a different letter": filepath.Join("B", "B.MANIFEST.JSON"),
	} {
		t.Run(name, func(t *testing.T) {
			app, qid, c, _ := privateBurn(t)
			p := filepath.Join(c.StagedDir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			onBurn(t, func() { t.Error("the burn ran") })
			if _, err := app.BurnNext(qid, func(float64, string) {}); err == nil {
				t.Fatal("the burn was not refused")
			}
		})
	}
}

// However the folder is spelled, the manifest goes outside it.
func TestBurn_TrailingSeparatorStillMovesTheManifestOut(t *testing.T) {
	app, qid, c, _ := privateBurn(t)
	folder := c.StagedDir
	c.StagedDir = folder + string(filepath.Separator)
	app.Store.UpdateChunk(c)
	ran := false
	onBurn(t, func() {
		ran = true
		entries, err := os.ReadDir(folder)
		if err != nil {
			t.Error(err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name(), ".manifest.") && !strings.HasSuffix(e.Name(), ".gpg") {
				t.Errorf("%s was in the folder being burned", e.Name())
			}
		}
	})
	_, _ = app.BurnNext(qid, func(float64, string) {})
	if !ran {
		t.Fatal("the burn did not run")
	}
}

// What a private or encrypted disc must not carry: the plaintext tar, the file list,
// and the BagIt tags that name the files. Each is back in staging afterward.
func TestBurn_KeepsEveryPlaintextFileOffTheDisc(t *testing.T) {
	app, qid, c, plain := privateBurn(t)
	off := []string{c.Name + ".tar", "filelist.txt", "manifest-sha256.txt", "bagit.txt", "bag-info.txt", "tagmanifest-sha256.txt"}
	for _, n := range off {
		mustWrite(t, filepath.Join(c.StagedDir, n), "content of "+n)
	}
	kept := filepath.Join(c.StagedDir, c.Name+".manifest.json.gpg")
	mustWrite(t, kept, "ciphertext")
	ran := false
	onBurn(t, func() {
		ran = true
		for _, n := range append(off, filepath.Base(plain)) {
			if _, err := os.Stat(filepath.Join(c.StagedDir, n)); err == nil {
				t.Errorf("%s was in the folder being burned", n)
			}
		}
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("the encrypted manifest was not in the folder: %v", err)
		}
	})
	if _, err := app.BurnNext(qid, func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("the burn did not run")
	}
	for _, n := range off {
		b, err := os.ReadFile(filepath.Join(c.StagedDir, n))
		if err != nil || string(b) != "content of "+n {
			t.Errorf("%s was not put back: %q, %v", n, b, err)
		}
	}
	if _, err := os.Stat(burnHoldDir(c, filepath.Dir(c.StagedDir))); !os.IsNotExist(err) {
		t.Errorf("the holding folder was left behind: %v", err)
	}
}

// An encrypted package that is not private keeps its manifest, but not its plaintext tar.
func TestBurn_EncryptedPackageLeavesItsTarBehind(t *testing.T) {
	app, qid, c, plain := privateBurn(t)
	c.PrivateManifest = false
	app.Store.UpdateChunk(c)
	tar := filepath.Join(c.StagedDir, c.Name+".tar")
	mustWrite(t, tar, "plaintext")
	onBurn(t, func() {
		if _, err := os.Stat(tar); err == nil {
			t.Error("the plaintext tar was in the folder being burned")
		}
		if _, err := os.Stat(plain); err != nil {
			t.Errorf("a non-private manifest should be on the disc: %v", err)
		}
	})
	if _, err := app.BurnNext(qid, func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tar); err != nil {
		t.Fatalf("the tar was not put back: %v", err)
	}
}

// The write path keeps the same files off private media.
func TestWrite_PrivateMediaCarriesNoFileList(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "PRIVATE-WRITE")
	gpg := filepath.Join(c.StagedDir, c.Name+".manifest.json.gpg")
	mustWrite(t, gpg, "ciphertext")
	h, err := hashFileHex(gpg)
	if err != nil {
		t.Fatal(err)
	}
	c.PrivateManifest = true
	if c.Sidecars == nil {
		c.Sidecars = map[string]string{}
	}
	c.Sidecars[c.Name+".manifest.json.gpg"] = h
	app.Store.UpdateChunk(c)
	dest := t.TempDir()
	if _, err := app.WriteChunk(c.ID, dest, 0, 0, 0, 0, noProg2); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"manifest-sha256.txt", "tagmanifest-sha256.txt", "bag-info.txt", "bagit.txt", c.Name + ".manifest.json"} {
		if _, err := os.Stat(filepath.Join(dest, c.Name, n)); err == nil {
			t.Errorf("%s is on the private medium", n)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, c.Name, c.Name+".manifest.json.gpg")); err != nil {
		t.Errorf("the encrypted manifest is missing: %v", err)
	}
}
