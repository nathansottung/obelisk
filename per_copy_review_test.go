//go:build !guionly

package main

import (
	"archive/tar"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frozenBackupHome is a working copy of a frozen app backup and its media, with what
// the backup's catalog recorded, read before anything opens it.
func frozenBackupHome(t *testing.T, fixture string) (*frozenHome, string) {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join(frozenRoot, "frozen", fixture), root)
	tars, _ := filepath.Glob(filepath.Join(root, "obelisk-appbackup-*.tar"))
	if len(tars) != 1 {
		t.Fatalf("backups in the fixture: %v", tars)
	}
	h := &frozenHome{root: root}
	if strings.Contains(fixture, "v1") {
		member := filepath.Join(t.TempDir(), catalogDBName)
		extractTarMember(t, tars[0], "catalog.db", member)
		h.want = frozenDBValues(t, member)
	} else {
		member := filepath.Join(t.TempDir(), "catalog.json")
		extractTarMember(t, tars[0], "catalog.json", member)
		h.want = frozenJSONValues(t, member)
	}
	if len(h.want) != 2 {
		t.Fatalf("the backup holds %d packages, want 2", len(h.want))
	}
	b, err := os.ReadFile(filepath.Join(root, "restored.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	h.restored = strings.TrimSpace(string(b))
	return h, tars[0]
}

// Real app backups made before this schema restore on every engine that can hold
// them. Every copy inherits its values, and old copies verify and restore, also
// after a rebuild and a reopen.
func TestFrozenBackups_RestoreOnBothEngines(t *testing.T) {
	tools := nativeTools(t)
	for _, tc := range []struct{ name, fixture, engine string }{
		{"catalog.json backup on the JSON engine", "catalog-v8-appbackup", "json"},
		{"catalog.json backup on the database engine", "catalog-v8-appbackup", "sqlite"},
		{"catalog.db backup on the database engine", "catalog-v1-appbackup", "sqlite"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, tc.engine)
			h, backup := frozenBackupHome(t, tc.fixture)
			app, _ := newTestApp(t, tools)
			h.data = app.DataDir
			if _, err := app.RestoreAppBackup(backup); err != nil {
				t.Fatalf("restoring a backup from before the upgrade: %v", err)
			}
			if got := app.Store.UsesDatabase(); got != (tc.engine == "sqlite") {
				t.Fatalf("restored onto the wrong engine: database=%v", got)
			}
			ks1, ks2 := filepath.Join(t.TempDir(), "keystore1.json"), filepath.Join(t.TempDir(), "keystore2.json")
			key, err := os.ReadFile(filepath.Join(h.root, "keystore.json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{ks1, ks2} {
				if err := os.WriteFile(p, key, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := app.SaveConfig(map[string]any{"staging_dir": t.TempDir(), "keystore_paths": []string{ks1, ks2},
				"tools": tools, "private_media": true}); err != nil {
				t.Fatal(err)
			}
			h.relocate(app)
			h.checkInherited(t, app)
			h.checkOldCopies(t, app)

			for name, want := range h.want {
				c := frozenChunk(t, app, name)
				c.Status = "FAILED"
				app.Store.UpdateChunk(c)
				if err := app.BuildChunk(c.ID, noProg); err != nil {
					t.Fatalf("rebuild %s: %v", name, err)
				}
				c = frozenChunk(t, app, name)
				if c.KeyRef == want.KeyRef || c.Copies[0].KeyRef != want.KeyRef || c.Copies[0].PayloadHash != want.EncHash {
					t.Fatalf("%s: the rebuild did not leave the old copy on its own values", name)
				}
			}
			h.checkOldCopies(t, app)

			if err := app.Store.Close(); err != nil {
				t.Fatal(err)
			}
			store, err := openTestStore(t, h.data)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			app.Store = store
			h.checkOldCopies(t, app)
		})
	}
}

// The JSON engine cannot hold a catalog.db backup: it says so and changes nothing.
func TestFrozenBackups_DatabaseBackupIsRefusedOnTheJSONEngine(t *testing.T) {
	t.Setenv(catalogEngineEnv, "json")
	_, backup := frozenBackupHome(t, "catalog-v1-appbackup")
	app, _ := newTestApp(t, nativeTools(t))
	path := filepath.Join(app.DataDir, "catalog.json")
	app.Store.Log("test", "a record that must survive a refused restore")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.RestoreAppBackup(backup)
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") || !strings.Contains(err.Error(), "OBELISK_CATALOG_ENGINE=sqlite") {
		t.Fatalf("got %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("a refused restore changed the catalog")
	}
}

func extractTarMember(t *testing.T, tarPath, name, to string) {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("%s is not in %s", name, tarPath)
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name != name {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
}

// remountable is a volume whose drive reports a fixed serial and partition.
func remountable(t *testing.T, app *App) *Volume {
	t.Helper()
	prevSerial, prevPart := driveSerialAt, partitionIDAt
	driveSerialAt = func(string) string { return "SERIAL-REMOUNT" }
	partitionIDAt = func(string) string { return "PARTITION-REMOUNT" }
	t.Cleanup(func() { driveSerialAt, partitionIDAt = prevSerial, prevPart })
	return app.Store.AddVolume(Volume{Label: "REMOUNT", Kind: "HDD", Serial: "SERIAL-REMOUNT", PartitionID: "PARTITION-REMOUNT"})
}

// Two builds in two folders of one remounted drive: each check reads and credits
// only the copy whose bytes it read.
func TestPerCopy_RemountedDriveChecksTheCopyItRead(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, "TWO-BUILDS", false)
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "drive", "a"), filepath.Join(root, "drive", "b")
	for _, d := range []string{oldDir, newDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.WriteChunk(c.ID, oldDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
	if _, err := app.WriteChunk(c.ID, newDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.Chunk(c.ID); got.CurrentCopyCount() != 2 {
		t.Fatalf("copies: %+v", got.Copies)
	}
	moved := filepath.Join(root, "mounted-elsewhere")
	if err := os.Rename(filepath.Join(root, "drive"), moved); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"b", "a"} {
		r, err := app.VerifyChunk(c.ID, filepath.Join(moved, sub))
		if err != nil {
			t.Fatal(err)
		}
		if ok, _ := r["verify_ok"].(bool); !ok || r["credited"] != 1 {
			t.Fatalf("folder %s: %v", sub, r)
		}
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 2 {
		t.Fatalf("verified = %d: %+v", got.VerifiedCopyCount(), got.Copies)
	}
}

// Damage to one of two builds on a remounted drive fails neither by guesswork.
func TestPerCopy_RemountedDriveDoesNotBlameTheOtherCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, "TWO-BUILDS-BAD", false)
	root := t.TempDir()
	oldDir, newDir := filepath.Join(root, "drive", "a"), filepath.Join(root, "drive", "b")
	for _, d := range []string{oldDir, newDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.WriteChunk(c.ID, oldDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
	if _, err := app.WriteChunk(c.ID, newDir, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "mounted-elsewhere")
	if err := os.Rename(filepath.Join(root, "drive"), moved); err != nil {
		t.Fatal(err)
	}
	flipByte(t, filepath.Join(moved, "b", c.Name, payloadName(c)))
	r, err := app.VerifyChunk(c.ID, filepath.Join(moved, "b"))
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := r["verify_ok"].(bool); ok {
		t.Fatal("the damage was not found")
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 || got.Copies[0].VerifyOK == nil || !*got.Copies[0].VerifyOK {
		t.Fatalf("damage to one copy must cost that copy its credit and no other: %+v", got.Copies)
	}
}

// frozenSpanRebuilt is a frozen spanned set whose package has a new build.
func frozenSpanRebuilt(t *testing.T) (*frozenHome, *App, *Chunk) {
	t.Helper()
	tools := nativeTools(t)
	t.Setenv(catalogEngineEnv, "sqlite")
	h := newFrozenHome(t, "catalog-v1-db")
	app := h.open(t, tools)
	c := frozenChunk(t, app, "FROZEN-SPANNED")
	c.Status = "FAILED"
	app.Store.UpdateChunk(c)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	return h, app, frozenChunk(t, app, "FROZEN-SPANNED")
}

// Writing a new set's par2 tape over the old set's retires the old set.
func TestPerCopy_NewPar2TapeRetiresTheSetItOverwrote(t *testing.T) {
	h, app, c := frozenSpanRebuilt(t)
	last := len(c.Segments) - 1
	if !c.Segments[last].Par2 {
		t.Fatal("the fixture's set has no par2 tape")
	}
	for i := 0; i < last; i++ {
		if err := app.Store.UpdateSegment(c, i, func(s *Segment) { s.Status = "VERIFIED" }); err != nil {
			t.Fatal(err)
		}
	}
	tape := filepath.Join(h.root, "tapes", fmt.Sprintf("tape%02d", last))
	// The other segments were only marked done, so the whole-set check refuses it.
	if _, err := spanWriteOver(t, app, c.ID, tape, c.Copies[0].VolumeID); err == nil || !strings.Contains(err.Error(), "does not check") {
		t.Fatalf("got %v, want the whole-set check's refusal after the par2 files were written", err)
	}
	if got := frozenChunk(t, app, c.Name); !got.Copies[0].Superseded {
		t.Fatal("the old set still counts after its par2 tape was overwritten")
	}
}

// Any tape of an old set overwritten by a new set takes the old set out of service,
// even when the tape is now mounted somewhere else.
func TestPerCopy_NewTapeOverAMovedOldTapeRetiresTheSet(t *testing.T) {
	h, app, c := frozenSpanRebuilt(t)
	from := filepath.Join(h.root, "tapes", "tape00")
	to := filepath.Join(h.root, "tapes", "tape00-remounted")
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	if _, err := spanWriteOver(t, app, c.ID, to, c.Copies[0].VolumeID); err != nil {
		t.Fatal(err)
	}
	if got := frozenChunk(t, app, c.Name); !got.Copies[0].Superseded {
		t.Fatal("the old set still counts after one of its tapes was overwritten")
	}
}

// After a rebuild every tape of the older set is still findable.
func TestPerCopy_OldSetStaysInVolumeViewsAndTheKit(t *testing.T) {
	h, app, c := frozenSpanRebuilt(t)
	vols := map[int]*Volume{}
	var ids []int
	for i := range c.Copies[0].Segments {
		v := app.Store.AddVolume(Volume{Label: fmt.Sprintf("OLD-TAPE-%d", i+1), Kind: "TAPE"})
		vols[v.ID] = v
		ids = append(ids, v.ID)
		c.Copies[0].Segments[i].VolumeID = v.ID
	}
	c.Copies[0].VolumeID = ids[0]
	app.Store.UpdateChunk(c)
	var b strings.Builder
	writeSegmentsTable(&b, c, vols)
	for _, sg := range c.Copies[0].Segments {
		if !strings.Contains(b.String(), normalizeKitPath(sg.Dest)) {
			t.Errorf("the kit omits the tape at %s", filepath.Base(sg.Dest))
		}
	}
	mux := http.NewServeMux()
	api(mux, app)
	for _, id := range ids {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/volumes/%d", id), nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), c.Name) {
			t.Errorf("volume %d does not list the package: %d", id, w.Code)
		}
	}
	_ = h
}

func normalizeKitPath(p string) string { return mdCell(p) }

// Overwriting a copy through a new drive letter leaves no phantom verified copy.
func TestPerCopy_OverwriteAfterARemountLeavesNoPhantomCopy(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	vol := remountable(t, app)
	c, _ := privateChunk(t, app, "PHANTOM", false)
	root := t.TempDir()
	first := filepath.Join(root, "first-letter")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := app.WriteChunk(c.ID, first, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = rebuildPackage(t, app, app.Store.Chunk(c.ID))
	second := filepath.Join(root, "second-letter")
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	if _, err := writeOver(t, app, c.ID, second, vol.ID); err != nil {
		t.Fatal(err)
	}
	if got := app.Store.Chunk(c.ID); got.VerifiedCopyCount() != 1 || got.CurrentCopyCount() != 1 {
		t.Fatalf("one physical copy, but verified=%d current=%d: %+v", got.VerifiedCopyCount(), got.CurrentCopyCount(), got.Copies)
	}
}

// A partial backup left by a crash never stands in for the real one.
func TestFrozenCatalogs_LeftoverStageIsNotABackup(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	h := newFrozenHome(t, "catalog-v1-db")
	junk := filepath.Join(h.data, catalogDBName+".pre-schema-v1-20260101-000000.stage")
	if err := os.WriteFile(junk, []byte("half a backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := openTestStore(t, h.data)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	m, _ := filepath.Glob(filepath.Join(h.data, catalogDBName+".pre-schema-v1-*"))
	good := 0
	for _, p := range m {
		if strings.HasSuffix(p, ".stage") {
			continue
		}
		db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(p))
		if err != nil {
			t.Fatal(err)
		}
		var v string
		err = db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v)
		db.Close()
		if err == nil && v == "1" {
			good++
		}
	}
	if good != 1 {
		t.Fatalf("usable backups: %d of %v", good, m)
	}
}

// A refill from a medium's root takes only this package's files.
func TestPerCopy_RefillFromAFlatCopyTakesOnlyItsOwnFiles(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "REFILL-FLAT")
	vol := c.Copies[0].VolumeID
	flat := t.TempDir()
	copyFolderFiles(t, filepath.Join(medium, c.Name), flat, all)
	if err := os.WriteFile(filepath.Join(flat, "NEIGHBOUR.manifest.json"), []byte("another package's file list"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(c.StagedDir); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RefillStaging(c.ID, vol, flat, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	if _, err := os.Stat(filepath.Join(c.StagedDir, "NEIGHBOUR.manifest.json")); err == nil {
		t.Fatal("another package's file was pulled into staging")
	}
	if err := chunkStagedForWrite(c); err != nil {
		t.Fatal(err)
	}
	if m, _ := filepath.Glob(filepath.Join(c.StagedDir, payloadName(c)+"*.par2")); len(m) == 0 {
		t.Fatal("the package's par2 set was left behind")
	}
}

// A medium holding an older build is already on record: adopting it adds nothing.
func TestPerCopy_AdoptingAnOlderBuildAddsNoPackage(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c, medium, _ := copiedPackage(t, app, "ADOPT-OLD")
	collection := app.Store.AddCollection("adopt")
	c = rebuildPackage(t, app, c)
	before := len(app.Store.Chunks(0))
	if _, err := app.AdoptMedia(medium, collection.ID, c.Copies[0].VolumeID, false, noProg); err != nil {
		t.Fatal(err)
	}
	if after := len(app.Store.Chunks(0)); after != before {
		b, _ := json.Marshal(app.Store.Chunks(0)[after-1].Name)
		t.Fatalf("packages %d -> %d (added %s)", before, after, b)
	}
}
