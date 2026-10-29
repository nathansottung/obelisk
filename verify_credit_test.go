//go:build !guionly

package main

import (
	"testing"
)

func copyOn(c *Chunk, volID int) *Copy {
	for i := range c.Copies {
		if c.Copies[i].VolumeID == volID && !c.Copies[i].Superseded {
			return &c.Copies[i]
		}
	}
	return nil
}

// A verify credits only the copy recorded at the folder it read. Reading the
// staged payload says nothing about the copy on the medium, even when it is the
// package's only copy.
func TestVerify_CreditsOnlyTheCopyItRead(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "CREDIT-1")
	medium := t.TempDir()
	vol := app.Store.AddVolume(Volume{Label: "TAPE-1", Kind: "TAPE"})
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, vol.ID, noProg); err != nil {
		t.Fatal(err)
	}
	corrupt(t, findPayload(medium, app.Store.Chunk(c.ID)))
	if _, err := app.VerifyChunk(c.ID, medium); err != nil {
		t.Fatal(err)
	}
	if cp := copyOn(app.Store.Chunk(c.ID), vol.ID); cp == nil || cp.VerifyOK == nil || *cp.VerifyOK {
		t.Fatalf("setup: the damaged medium copy should be failed: %+v", cp)
	}
	res, err := app.VerifyChunk(c.ID, staging)
	if err != nil {
		t.Fatal(err)
	}
	if cp := copyOn(app.Store.Chunk(c.ID), vol.ID); cp.VerifyOK == nil || *cp.VerifyOK {
		t.Error("verifying the staged payload credited the copy on the medium")
	}
	if cr, _ := res["credited"].(int); cr != 0 {
		t.Errorf("credited %d copies, want 0", cr)
	}
}

// Two drives whose copies were recorded at the same path (both written while
// mounted at the same letter) are not both credited by reading one of them;
// when the drive being read cannot be told apart, neither is.
func TestVerify_SamePathOnTwoDrivesIsNotDoubleCredited(t *testing.T) {
	app, _ := newTestApp(t, nativeTools(t))
	c := builtTestChunk(t, app, "CREDIT-2")
	medium := t.TempDir()
	v1 := app.Store.AddVolume(Volume{Label: "USB-1", Kind: "HDD"})
	v2 := app.Store.AddVolume(Volume{Label: "USB-2", Kind: "HDD"})
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, v1.ID, noProg); err != nil {
		t.Fatal(err)
	}
	c = app.Store.Chunk(c.ID)
	app.Store.RecordCopy(c, v2.ID, copyOn(c, v1.ID).Path, false) // USB-2 at the same path, failed
	if _, err := app.VerifyChunk(c.ID, medium); err != nil {
		t.Fatal(err)
	}
	if cp := copyOn(app.Store.Chunk(c.ID), v2.ID); cp.VerifyOK == nil || *cp.VerifyOK {
		t.Error("reading one drive credited the other drive's copy at the same path")
	}
}

// A check is credited to the copy whose drive's recorded serial matches the drive
// read, and to none when it cannot tell which of two drives it read.
func TestCreditCopyCheck_Serial(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := st.AddVolume(Volume{Label: "A", Serial: "SER-A"})
	b := st.AddVolume(Volume{Label: "B", Serial: "SER-B"})
	c := st.AddChunk(Chunk{Name: "P"})
	st.RecordCopy(c, a.ID, "E:/P", false)
	st.RecordCopy(c, b.ID, "E:/P", false)
	if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/P", Serial: "ser-b"}, true, "B"); n != 1 {
		t.Fatalf("credited %d, want only B", n)
	}
	if cp := copyOn(c, b.ID); cp.VerifyOK == nil || !*cp.VerifyOK {
		t.Error("B was not credited")
	}
	if cp := copyOn(c, a.ID); cp.VerifyOK != nil && *cp.VerifyOK {
		t.Error("A was credited for B's read")
	}
	if n := st.CreditCopyCheck(c, CopyAt{Path: "E:/P"}, true, "B"); n != 0 {
		t.Errorf("with the drive unknown, credited %d of two drives' copies, want 0", n)
	}
}
