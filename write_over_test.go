//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOver writes package id over what is already in its folder at dest, as
// the user does: a plain write refuses first and says what is there, then the
// delete gate's write-over (prepare, then the typed "Overwrite") writes it.
func writeOver(t *testing.T, app *App, id int, dest string, vol int) (map[string]any, error) {
	t.Helper()
	var wo *WriteOverError
	if _, err := app.WriteChunk(id, dest, 0, 0, 0, vol, noProg); !errors.As(err, &wo) {
		t.Fatalf("a plain write over an existing copy did not refuse and say what is there: %v", err)
	}
	if len(wo.Files) == 0 || wo.Message == "" {
		t.Fatalf("the refusal does not list what is there: %+v", wo)
	}
	p := prepareForTest(t, app, "write-over", map[string]any{"id": float64(id), "dest_dir": dest, "volume_id": float64(vol)})
	if len(p.Targets) < len(wo.Files) || p.Word != wordOverwrite {
		t.Fatalf("the gate does not list every file it would replace: %+v", p)
	}
	return app.WriteChunkOver(id, dest, 0, 0, 0, vol, noProg)
}

// spanWriteOver is writeOver for the next tape of a spanned package.
func spanWriteOver(t *testing.T, app *App, id int, dest string, vol int) (map[string]any, error) {
	t.Helper()
	var wo *WriteOverError
	if _, err := app.SpanWriteNext(id, dest, 0, 0, 0, vol, noProg); !errors.As(err, &wo) {
		t.Fatalf("a plain span write onto another set did not refuse and say what is there: %v", err)
	}
	prepareForTest(t, app, "span-write-over", map[string]any{"id": float64(id), "dest_dir": dest, "volume_id": float64(vol)})
	return app.SpanWriteNextOver(id, dest, 0, 0, 0, vol, noProg)
}

// A write over an existing copy says what is there first: this build intact
// (with Verify offered), a partial earlier write, or other files.
func TestWriteOver_SaysWhatIsThereFirst(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "WHAT-IS-THERE")
	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	dir := filepath.Join(medium, c.Name)
	var wo *WriteOverError
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); !errors.As(err, &wo) || wo.State != "intact" || !strings.Contains(wo.Message, "Verify") {
		t.Fatalf("an intact copy: %v", err)
	}
	// A partial earlier write: the payload cut short.
	payload := payloadPathIn(dir, c)
	if err := os.Truncate(payload, 10); err != nil { // test setup
		t.Fatal(err)
	}
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); !errors.As(err, &wo) || wo.State != "partial" {
		t.Fatalf("a partial write: %v", err)
	}
	// Something else: a sidecar that is not this build's.
	mustWrite(t, filepath.Join(dir, "RESTORE.txt"), "another build's notes")
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); !errors.As(err, &wo) || wo.State != "other" {
		t.Fatalf("other files: %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "RESTORE.txt")); got != "another build's notes" {
		t.Errorf("a refused write changed what was there: %q", got)
	}
	// Only through the gate, with the exact list.
	p := prepareForTest(t, app, "write-over", map[string]any{"id": float64(c.ID), "dest_dir": medium})
	if _, err := app.ExecuteDestructive(testSession, p.Token, "overwrite"); err == nil {
		t.Fatal("written over without the exact word")
	}
	if _, err := app.WriteChunkOver(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	if h, _ := hashFileHex(filepath.Join(dir, "RESTORE.txt")); h != c.Sidecars["RESTORE.txt"] {
		t.Error("the gated write did not put this build's sidecar in place")
	}
}

// A seal already on the medium is replaced only when it is Obelisk's own, by
// its content; anything else at those names refuses the seal.
func TestFinalize_ReplacesOnlyItsOwnSeal(t *testing.T) {
	app, vol, _, mount := finalizeApp(t)
	if _, err := app.FinalizeVolume(vol, mount, "NS", false, ""); err != nil {
		t.Fatal(err)
	}
	if !isOwnSealFile(nil, sealSidecarDir+"/FINALIZATION.json", []byte(mustRead(t, filepath.Join(mount, sealSidecarDir, "FINALIZATION.json")))) {
		t.Fatal("the seal Obelisk wrote is not recognised as its own")
	}
	if isOwnSealFile(nil, sealSidecarDir+"/INVENTORY.md", []byte("my notes")) {
		t.Error("someone else's file is taken for Obelisk's seal")
	}
}
