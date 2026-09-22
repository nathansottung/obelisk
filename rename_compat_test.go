//go:build !guionly

package main

// rename_compat_test.go — pins the permanent Mnemosyne -> Obelisk read-compatibility
// promised in ARCHITECTURE.md "Name compatibility". One test per compat item (a-g).
// None of these need the native tar/gpg/par2 toolchain, so they always run.

import (
	"archive/tar"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compatApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	return initializedTestApp(t, &App{DataDir: dir, Store: store}), dir
}

// (a) Data dir: prefer ~/.obelisk, fall back to ~/.mnemo silently.
func TestRenameCompat_DataDirFallback(t *testing.T) {
	home := t.TempDir()
	obelisk := filepath.Join(home, ".obelisk")
	legacy := filepath.Join(home, ".mnemo")

	if got := resolveDataDir(home); got != obelisk {
		t.Errorf("fresh home should resolve to ~/.obelisk, got %s", got)
	}
	mustMkdir(t, legacy)
	mustWrite(t, filepath.Join(legacy, "catalog.json"), `{"schema_version":1}`)
	if got := resolveDataDir(home); got != legacy {
		t.Errorf("legacy-only home should fall back to ~/.mnemo, got %s", got)
	}
	mustMkdir(t, obelisk)
	mustWrite(t, filepath.Join(obelisk, "catalog.json"), `{"schema_version":1}`)
	if got := resolveDataDir(home); got != obelisk {
		t.Errorf("with both present, should prefer ~/.obelisk, got %s", got)
	}
}

// (a) Migrate: COPY (never move) + verify by hash before switching.
func TestRenameCompat_MigrateCopiesVerifiesAndSwitches(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".mnemo")
	target := filepath.Join(root, ".obelisk")
	mustMkdir(t, legacy)
	store, err := OpenStore(legacy) // creates catalog.json
	if err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(legacy, "keystores"))
	mustWrite(t, filepath.Join(legacy, "keystores", "k.json"), `{"obelisk_keystore":1,"keys":[]}`)
	app := initializedTestApp(t, &App{DataDir: legacy, Store: store})

	if !app.LegacyDataDirMigrationAvailable() {
		// LegacyDataDirMigrationAvailable checks the REAL ~/.mnemo home, so it may be
		// false here; the migrate core is what we verify. (Documented, not fatal.)
	}
	res, err := app.migrateDataDir(legacy, target)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The migration verifies every file by hash AS it copies (a mismatch would have
	// aborted with an error above). Here we confirm the copy landed: catalog.json is
	// present (the reopen at the new home legitimately re-stamps it), and a file the
	// store never rewrites — the keystore — is byte-identical to its original.
	if !fileExists(filepath.Join(target, "catalog.json")) {
		t.Error("catalog.json was not copied into the new home")
	}
	ksRel := filepath.FromSlash("keystores/k.json")
	sh, e1 := hashFileHex(filepath.Join(legacy, ksRel))
	dh, e2 := hashFileHex(filepath.Join(target, ksRel))
	if e1 != nil || e2 != nil {
		t.Fatalf("hashing keystore: %v / %v", e1, e2)
	}
	if sh != dh {
		t.Error("the migrated keystore is not byte-identical to the original")
	}
	// The originals are NEVER moved or deleted.
	if !fileExists(filepath.Join(legacy, "catalog.json")) {
		t.Error("migration must leave the legacy originals in place")
	}
	// The running app switched to the new home.
	if app.DataDir != target {
		t.Errorf("app.DataDir = %s, want %s", app.DataDir, target)
	}
	if res["to"] != target {
		t.Errorf("result 'to' = %v, want %s", res["to"], target)
	}

	// Refuses to clobber a target that already holds a catalog.
	app2 := initializedTestApp(t, &App{DataDir: legacy, Store: store})
	if _, err := app2.migrateDataDir(legacy, target); err == nil {
		t.Error("migrate must refuse when the target already holds a catalog")
	}
}

// (b) Keystores: accept "mnemosyne_keystore": 1 forever; write the Obelisk marker.
func TestRenameCompat_KeystoreMarkers(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.json")
	mustWrite(t, legacy, `{"mnemosyne_keystore":1,"keys":[]}`)
	if _, err := readStore(legacy); err != nil {
		t.Errorf("legacy mnemosyne_keystore marker must be accepted forever: %v", err)
	}
	newer := filepath.Join(dir, "new.json")
	mustWrite(t, newer, `{"obelisk_keystore":1,"keys":[]}`)
	if _, err := readStore(newer); err != nil {
		t.Errorf("obelisk_keystore marker must be accepted: %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	mustWrite(t, bad, `{"keys":[]}`)
	if _, err := readStore(bad); err == nil {
		t.Error("a file with neither marker must be rejected")
	}
	// New writes carry the Obelisk marker only.
	out := filepath.Join(dir, "written.json")
	if err := writeStore(out, &keystoreFile{Marker: 1, Keys: []map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	b := mustRead(t, out)
	if !strings.Contains(b, `"obelisk_keystore"`) {
		t.Error("new keystores must be written under obelisk_keystore")
	}
	if strings.Contains(b, `"mnemosyne_keystore"`) {
		t.Error("new keystores must not emit the legacy marker")
	}
}

// (c) Manifests: a package manifest with either marker reads cleanly.
func TestRenameCompat_ManifestMarkers(t *testing.T) {
	app, _ := compatApp(t)
	oldDir := t.TempDir()
	mustWrite(t, filepath.Join(oldDir, "PKG.manifest.json"), `{"mnemosyne_chunk":1,"name":"OLD","payload_file":"PKG.tar"}`)
	m, _, found, err := app.readAdoptManifest(oldDir, "PKG")
	if err != nil || !found || m["name"] != "OLD" {
		t.Fatalf("legacy mnemosyne_chunk manifest must read: found=%v err=%v m=%v", found, err, m)
	}
	newDir := t.TempDir()
	mustWrite(t, filepath.Join(newDir, "PKG.manifest.json"), `{"obelisk_package":1,"name":"NEW"}`)
	m2, _, found2, err2 := app.readAdoptManifest(newDir, "PKG")
	if err2 != nil || !found2 || m2["name"] != "NEW" {
		t.Fatalf("obelisk_package manifest must read: found=%v err=%v m=%v", found2, err2, m2)
	}
}

// (d) Media sidecars: BOTH the Obelisk and legacy Mnemosyne folders are recognized.
func TestRenameCompat_SidecarDirsRecognized(t *testing.T) {
	if dockSidecarDir != "OBELISK_DOCK" || dockSidecarDirLegacy != "MNEMOSYNE_DOCK" {
		t.Fatalf("dock sidecar constants drifted: %q / %q", dockSidecarDir, dockSidecarDirLegacy)
	}
	if sealSidecarDir != "OBELISK_SEAL" || sealSidecarDirLegacy != "MNEMOSYNE_SEAL" {
		t.Fatalf("seal sidecar constants drifted: %q / %q", sealSidecarDir, sealSidecarDirLegacy)
	}
	for _, n := range []string{dockSidecarDir, sealSidecarDir, dockSidecarDirLegacy, sealSidecarDirLegacy} {
		if !cardSkipDir(n) {
			t.Errorf("cardSkipDir(%q) = false, want true (must skip our own sidecars, old and new)", n)
		}
		if !skipDockDir(n) {
			t.Errorf("skipDockDir(%q) = false, want true", n)
		}
	}
}

// (e) Structure/plan exports made under the old name import cleanly.
func TestRenameCompat_StructurePlanImportMarkers(t *testing.T) {
	app, _ := compatApp(t)

	// Legacy structure format imports; a bogus format is refused.
	legacyExp := StructureExport{Format: structureFormatLegacy, Version: structureExportVersion,
		Archive: "Legacy Arc", Files: []ExportFile{{Hash: "aa", Size: 1, RelPath: "a.txt"}}}
	if _, err := app.ImportStructure(legacyExp); err != nil {
		t.Errorf("legacy mnemosyne-structure must import: %v", err)
	}
	if _, err := app.ImportStructure(StructureExport{Format: "not-ours"}); err == nil {
		t.Error("an unknown structure format must be refused")
	}

	// Plan format gate: legacy accepted, bogus refused (isolate the format check).
	if _, err := app.ImportPlan(PlanExport{Format: "not-ours"}); err == nil || !strings.Contains(err.Error(), "plan export") {
		t.Errorf("bogus plan format must be refused at the format gate, got: %v", err)
	}
	if _, err := app.ImportPlan(PlanExport{Format: planFormatLegacy}); err != nil && strings.Contains(err.Error(), "plan export") {
		t.Errorf("legacy mnemosyne-plan must pass the format gate, got format error: %v", err)
	}
}

// (e) App backups made under the old name verify + restore.
func TestRenameCompat_AppBackupLegacyFormat(t *testing.T) {
	app, _ := compatApp(t)
	dir := t.TempDir()
	legacyTar := buildLegacyAppBackup(t, app, dir)
	if _, _, err := verifyAppBackup(legacyTar); err != nil {
		t.Errorf("an app backup carrying the legacy mnemosyne-appbackup format must verify: %v", err)
	}
}

// (f) Legacy plaintext *.tar.gpg payloads are still recognized for restore/adopt.
func TestRenameCompat_LegacyEncryptedPayloadRecognized(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "OLDPKG.tar.gpg"), "ciphertext")
	cands := scanAdoptCandidates(dir)
	found := false
	for _, c := range cands {
		if c.name == "OLDPKG" && c.encrypted {
			found = true
		}
	}
	if !found {
		t.Error("a legacy *.tar.gpg payload must still be recognized as an adopt candidate")
	}
}

// (g) A written RESTORE.txt is a historical document, never REQUIRED to restore:
// findPayload locates the payload with no RESTORE.txt present at all.
func TestRenameCompat_RestoreTxtNotRequired(t *testing.T) {
	dir := t.TempDir()
	c := &Chunk{Name: "PKG", Encrypted: false}
	mustWrite(t, filepath.Join(dir, payloadName(c)), "payload-bytes")
	if got := findPayload(dir, c); got == "" {
		t.Error("findPayload must locate the payload without any RESTORE.txt present")
	}
}

// ---- helpers ----

func buildLegacyAppBackup(t *testing.T, app *App, dir string) string {
	t.Helper()
	res, err := app.exportAppBackupTo(filepath.Join(dir, "obelisk-appbackup-src.tar"), false)
	if err != nil {
		t.Fatalf("export app backup: %v", err)
	}
	members, err := readTarMembers(res.TarPath)
	if err != nil {
		t.Fatal(err)
	}
	var man map[string]any
	if err := json.Unmarshal(members["MANIFEST.json"], &man); err != nil {
		t.Fatal(err)
	}
	man["format"] = appBackupFormatLegacy // pretend this bundle was made by Mnemosyne
	newMan, _ := json.MarshalIndent(man, "", "  ")

	legacyTar := filepath.Join(dir, "mnemosyne-appbackup-legacy.tar")
	f, err := os.Create(legacyTar)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	write := func(name string, data []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Format: tar.FormatPAX}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write("MANIFEST.json", newMan) // member hashes are over the OTHER members, so still valid
	for name, data := range members {
		if name == "MANIFEST.json" {
			continue
		}
		write(name, data)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	// Fresh whole-tar sidecar over the rewritten bundle.
	sum, err := hashFileHex(legacyTar)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, legacyTar+".sha256", sum+"  "+filepath.Base(legacyTar)+"\n")
	return legacyTar
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
