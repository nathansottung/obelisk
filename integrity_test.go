//go:build !guionly

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrityPresetLabels(t *testing.T) {
	for _, name := range integrityPresetOrder {
		if got := presetLabelFor(integrityPresets()[name]); got != name {
			t.Errorf("preset %s should label as itself, got %q", name, got)
		}
	}
	// Nudge one knob off ARCHIVAL → Custom.
	iv := integrityPresets()["ARCHIVAL"]
	iv.Par2Redundancy = 7
	if got := presetLabelFor(iv); got != "Custom" {
		t.Errorf("edited preset should read Custom, got %q", got)
	}
}

func TestEffectiveIntegrityArchiveOverride(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	app := initializedTestApp(t, &App{DataDir: filepath.Dir(st.path), Store: st})
	coll := st.AddCollection("A")

	// Default (no override) is the global ARCHIVAL preset.
	if iv := app.effectiveIntegrity(coll.ID, mustConfig(t, app)); iv.Preset != "ARCHIVAL" || iv.BuildVerify != BuildVerifyFull {
		t.Fatalf("default should be ARCHIVAL/full, got %+v", iv)
	}
	// Override to FAST.
	if _, err := app.applyArchiveIntegrity(coll.ID, map[string]any{"preset": "FAST"}); err != nil {
		t.Fatalf("apply FAST: %v", err)
	}
	iv := app.effectiveIntegrity(coll.ID, mustConfig(t, app))
	if iv.Preset != "FAST" || iv.BuildVerify != BuildVerifyNone || iv.Par2Redundancy != 5 || iv.VerifyDueMonths != 24 {
		t.Fatalf("archive should be FAST, got %+v", iv)
	}
	if !iv.ReadbackAfterWrite {
		t.Fatal("read-back must always be on")
	}
	// Editing a single knob makes it Custom, not a named preset.
	if _, err := app.applyArchiveIntegrity(coll.ID, map[string]any{"build_verify": "contents"}); err != nil {
		t.Fatalf("edit knob: %v", err)
	}
	if iv := app.effectiveIntegrity(coll.ID, mustConfig(t, app)); iv.Preset != "Custom" || iv.Par2Redundancy != 5 {
		t.Fatalf("edited FAST should be Custom keeping par2 5, got %+v", iv)
	}
	// Clear → back to global ARCHIVAL.
	if _, err := app.applyArchiveIntegrity(coll.ID, map[string]any{"clear": true}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if iv := app.effectiveIntegrity(coll.ID, mustConfig(t, app)); iv.Preset != "ARCHIVAL" {
		t.Fatalf("cleared override should inherit global ARCHIVAL, got %+v", iv)
	}
}

// TestFastArchiveAttestsReducedIntegrity is the end-to-end guarantee: switch an
// archive to FAST, build a package, and confirm the built package (and its
// on-disk manifest) attest the reduced settings that the UI renders amber.
func TestFastArchiveAttestsReducedIntegrity(t *testing.T) {
	s := newIT(t) // skips if tar/gpg/par2 are unavailable
	s.setConfig(nil)
	src := s.makeSource(map[string][]byte{"a.txt": []byte("re-creatable data — fine to build FAST")})

	coll := s.obj("POST", "/api/collections", map[string]any{"name": "Recreatable"})
	cid := int(coll["id"].(float64))
	iv := s.obj("PUT", fmt.Sprintf("/api/collections/%d/integrity", cid), map[string]any{"preset": "FAST"})
	eff := iv["effective"].(map[string]any)
	if eff["preset"] != "FAST" || eff["build_verify"] != "none" {
		t.Fatalf("archive should be FAST/none, got %v", eff)
	}

	s.job(s.obj("POST", fmt.Sprintf("/api/collections/%d/scan", cid), map[string]any{"path": src}))
	plan := s.obj("POST", "/api/plan", map[string]any{"collection_id": cid, "media_kind": "CUSTOM", "target_gb": 1.0, "encrypted": false})
	pkg := plan["chunks_created"].([]any)[0].(map[string]any)
	pid := int(pkg["id"].(float64))

	// INTENTIONAL BEHAVIOUR CHANGE (OBX-006 containment, 2026-09-07): on the Windows
	// external-tar path a FAST archive can no longer be BUILT, so there is no built
	// package to attest. Everything above — the override, its effective FAST/none
	// configuration and the 5% par2 it plans — is platform-independent and still
	// asserted. What changes here is only the outcome of the build itself, which must
	// now be an explicit refusal rather than an unverified success.
	if buildUsesWindowsExternalTar() {
		label := s.jobFailure(s.obj("POST", fmt.Sprintf("/api/chunks/%d/build", pid), nil))
		if !strings.Contains(label, "refusing to build without content verification on Windows") {
			t.Fatalf("a FAST build on Windows must be refused by the OBX-006 containment, got: %s", label)
		}
		if chunk := s.obj("GET", fmt.Sprintf("/api/chunks/%d", pid), nil); chunk["status"] == "STAGED" {
			t.Error("a refused FAST build must not leave the package STAGED")
		}
		return
	}

	s.job(s.obj("POST", fmt.Sprintf("/api/chunks/%d/build", pid), nil))
	chunk := s.obj("GET", fmt.Sprintf("/api/chunks/%d", pid), nil)

	// par2 comes from the FAST preset (5%), not the global 10%.
	if p := int(chunk["par2_redundancy"].(float64)); p != 5 {
		t.Errorf("FAST archive should plan 5%% par2, got %d", p)
	}
	bv := chunk["build_verified"].(map[string]any)
	if bv["mode"] != "none" || bv["preset"] != "FAST" {
		t.Fatalf("build_verified should attest FAST/none, got %v", bv)
	}
	if bv["contents"] == true || bv["decrypt_roundtrip"] == true {
		t.Errorf("FAST build must not claim contents/round-trip proven: %v", bv)
	}
	if w, _ := bv["warning"].(string); w == "" {
		t.Error("FAST build must carry the amber warning the UI renders")
	}
	if bv["readback_after_write"] != true {
		t.Error("read-back must be attested always-on even at FAST")
	}
	if int(bv["par2_percent"].(float64)) != 5 {
		t.Errorf("attested par2_percent should be 5, got %v", bv["par2_percent"])
	}

	// The on-medium manifest must carry the same attestation (media self-document).
	manifest := filepath.Join(chunk["staged_dir"].(string), chunk["name"].(string)+".manifest.json")
	mb, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	mbv, ok := m["build_verified"].(map[string]any)
	if !ok || mbv["mode"] != "none" || mbv["preset"] != "FAST" {
		t.Fatalf("manifest must attest FAST/none integrity, got %v", m["build_verified"])
	}
}
