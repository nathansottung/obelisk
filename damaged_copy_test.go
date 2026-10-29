//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A copy found damaged loses its credit at once, whatever level of check found
// it. A census (A) or sample (C) check cannot earn credit, but a failed one is
// still evidence the copy is bad.
func TestCreditCopyCheck_FailureClearsCreditAtAnyLevel(t *testing.T) {
	for _, level := range []string{VerifyA, VerifyB, VerifyC} {
		t.Run(level, func(t *testing.T) {
			st, err := OpenStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			v := st.AddVolume(Volume{Label: "D"})
			c := st.AddChunk(Chunk{Name: "P"})
			st.RecordCopy(c, v.ID, "E:/P", true)
			if c.VerifiedCopyCount() != 1 {
				t.Fatal("setup: the copy starts verified")
			}
			if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/P", VolumeID: v.ID}, false, level); n != 1 {
				t.Fatalf("the failed check was recorded on %d copies, want 1", n)
			}
			if c.VerifiedCopyCount() != 0 {
				t.Errorf("a copy that failed a level-%s check still counts as verified", level)
			}
		})
	}
}

// A passing advisory check never grants credit to a copy that does not have it.
func TestCreditCopyCheck_AdvisoryPassGrantsNothing(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v := st.AddVolume(Volume{Label: "D"})
	c := st.AddChunk(Chunk{Name: "P"})
	st.RecordCopy(c, v.ID, "E:/P", false)
	st.CreditCopyCheck(c, CopyAt{Path: "E:/P", VolumeID: v.ID}, true, VerifyC)
	if c.VerifiedCopyCount() != 0 {
		t.Error("a passing sample check credited a copy")
	}
}

// An advisory check of a mirror drive that finds a damaged file takes the
// credit away at once: the copy is no longer verified, without waiting for a
// full check.
func TestMirrorVerify_AdvisoryFailureDropsCreditAtOnce(t *testing.T) {
	app := dockApp(t)
	arch := app.Store.AddCollectionKind("Loose", ArchiveSourceless)
	drive := t.TempDir()
	writeTree(t, drive, map[string]string{"a.jpg": "AAAA", "b.jpg": "BBBB"})
	vol := app.Store.AddVolume(Volume{Label: "M1", Kind: "HDD"})
	if _, err := app.AdoptFolder(drive, arch.ID, vol.ID, nil); err != nil {
		t.Fatal(err)
	}
	var mirror *Chunk
	for _, c := range app.Store.Chunks(arch.ID) {
		if c.Mirror {
			mirror = c
		}
	}
	if mirror == nil || mirror.VerifiedCopyCount() != 1 {
		t.Fatalf("setup: want one verified mirror copy, got %+v", mirror)
	}
	// A file cut short: the census (A) check finds it.
	if err := os.WriteFile(filepath.Join(drive, "a.jpg"), []byte("AA"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := app.VerifyMirrorVolume(vol.ID, "", VerifyA, nil); err != nil {
		t.Fatal(err)
	}
	mirror = app.Store.Chunk(mirror.ID)
	if n := mirror.VerifiedCopyCount(); n != 0 {
		t.Errorf("after a failed census check the mirror has %d verified copies, want 0", n)
	}
}
