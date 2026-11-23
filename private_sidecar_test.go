//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sealing or docking a medium must not list what its private packages hide, and must
// keep listing the packages that are not private.
func TestSealAndDock_KeepPrivateFileNamesOffTheMedium(t *testing.T) {
	const hidden, kind = "secret-location-plan", "zzprivate"
	app, vol, _, mount := finalizeApp(t)
	p := app.Store.AddChunk(Chunk{Name: "PRIV-C0002", Status: "WRITTEN", Encrypted: true, PrivateManifest: true,
		EncBytes: 1000, DataBytes: 900, FileCount: 1, Files: []ChunkFileRef{{FileID: 2, RelPath: hidden + "." + kind, SizeBytes: 900}}})
	app.Store.RecordCopy(p, vol.ID, "M:/PRIV-C0002", true)
	if _, err := app.FinalizeVolume(vol, mount, "NS", false, ""); err != nil {
		t.Fatal(err)
	}
	dockDir := t.TempDir()
	dest, err := app.Store.AuthorizeDestination(dockDir)
	if err != nil {
		t.Fatal(err)
	}
	defer dest.Close()
	if _, err := app.writeVolumeInventoryTo(dest, vol); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(mount, sealSidecarDir), filepath.Join(dockDir, dockSidecarDir)} {
		for _, f := range []string{"catalog_snapshot.json", "INVENTORY.md"} {
			b, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil {
				t.Fatal(err)
			}
			s := string(b)
			if strings.Contains(s, hidden) || strings.Contains(s, kind) {
				t.Errorf("%s in %s shows a private file", f, filepath.Base(dir))
			}
			if !strings.Contains(s, "a.nef") {
				t.Errorf("%s in %s lost the package that is not private", f, filepath.Base(dir))
			}
			if !strings.Contains(s, "PRIV-C0002.manifest.json.gpg") {
				t.Errorf("%s in %s does not say where the private list is", f, filepath.Base(dir))
			}
		}
	}
}

// A rebuild after private mode is turned off must not expose a copy already written
// as private: the next seal of that medium would list its names.
func TestRebuild_StaysPrivateWhileAPrivateCopyExists(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "PRIV-REBUILD", false)
	if _, err := app.WriteChunk(c.ID, t.TempDir(), 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if _, err := app.SaveConfig(map[string]any{"private_media": false}); err != nil {
		t.Fatal(err)
	}
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	if !c.PrivateManifest {
		t.Fatal("the rebuild cleared the private flag of a package with a private copy")
	}
	mustExist(t, filepath.Join(c.StagedDir, c.Name+".manifest.json.gpg"))
}

// With no copy written, a rebuild follows the current setting.
func TestRebuild_FollowsTheSettingWhenNoCopyExists(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, _ := privateChunk(t, app, "PRIV-UNWRITTEN", false)
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if _, err := app.SaveConfig(map[string]any{"private_media": false}); err != nil {
		t.Fatal(err)
	}
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if app.Store.Chunk(c.ID).PrivateManifest {
		t.Fatal("a package with no private copy stayed private after the setting was turned off")
	}
}
