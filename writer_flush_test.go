package main

// writer_flush_test.go — a write is not finished until the medium has taken the bytes.
//
// Both write paths (the ring-buffered payload stream and the sidecar copy) used to
// hand the destination file to a deferred Close(), which discards its error. A failed
// final flush — a disk that fills on the last block, a drive that only reports a write
// error at close — therefore returned success, and ringCopy returned a SHA-256 computed
// over the bytes in memory, describing a file that is short on the medium. That is a
// false success on the one path that writes archives to tape and disk.
//
// The faults are injected at finalizeWrite's real flush branch via the
// writeFlushFaultHook seam (nil in production), because a genuinely full disk cannot be
// staged portably. These tests set that package-level seam, so they do not run in
// parallel.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errFlushFault = errors.New("no space left on device")

func TestRingCopy_FailedFinalFlushIsNotSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "payload.tar")
	if err := os.WriteFile(src, []byte("payload bytes that reach the page cache but not the medium"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst.tar")

	writeFlushFaultHook = func(p string) error {
		if p == dst {
			return errFlushFault
		}
		return nil
	}
	defer func() { writeFlushFaultHook = nil }()

	hash, stats, err := ringCopy(src, dst, 0, 0, 1, 0.01, 0, func(done, total int64) {}, nil)
	if err == nil {
		t.Fatal("a destination that could not be flushed must fail the copy; got nil (the caller would then trust a hash over a truncated file)")
	}
	if !errors.Is(err, errFlushFault) {
		t.Errorf("error = %v, want it to wrap the underlying flush failure", err)
	}
	if !strings.Contains(err.Error(), dst) {
		t.Errorf("error = %q, want it to name the destination that failed", err.Error())
	}
	// The hash is the whole danger: handed back, it would be compared against the
	// staged hash, match (it is computed over the source stream), and certify a file
	// that is not fully on the medium.
	if hash != "" {
		t.Errorf("no hash may be returned when the flush failed, got %q", hash)
	}
	_ = stats
}

func TestCopyFile_FailedFinalFlushIsNotSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "package.par2")
	if err := os.WriteFile(src, []byte("recovery blocks"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "copy.par2")

	writeFlushFaultHook = func(p string) error {
		if p == dst {
			return errFlushFault
		}
		return nil
	}
	defer func() { writeFlushFaultHook = nil }()

	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile must report a failed final flush, not return nil over an unflushed sidecar")
	} else if !errors.Is(err, errFlushFault) {
		t.Errorf("error = %v, want it to wrap the underlying flush failure", err)
	}
}

// End to end: a payload whose final flush fails must leave NO recorded copy — the
// package falls back to STAGED (the staged artifact is fine; the medium is not), and
// WriteChunk returns no result map for the UI to read as a completed write.
func TestWriteChunk_FailedFinalFlushRecordsNoCopy(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	src, refs := makeSource(t)

	c := app.Store.AddChunk(Chunk{
		Name: "FLUSH-PKG", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: false,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk: %v", err)
	}

	vol := app.Store.AddVolume(Volume{Label: "FULL-DISK-01", Kind: "HDD", Location: "office"})
	medium := t.TempDir()

	// Fail the flush only on the destination medium — staging is healthy, which is
	// exactly the real case (the disk we are writing TO is the one that filled up).
	writeFlushFaultHook = func(p string) error {
		if strings.HasPrefix(p, medium) {
			return errFlushFault
		}
		return nil
	}
	defer func() { writeFlushFaultHook = nil }()

	res, err := app.WriteChunk(c.ID, medium, 0, 0, 0, vol.ID, noProg)
	if err == nil {
		t.Fatal("WriteChunk must fail when the payload could not be flushed to the medium")
	}
	if res != nil {
		t.Errorf("no result map may be returned on a failed write, got %+v", res)
	}
	if !errors.Is(err, errFlushFault) {
		t.Errorf("error = %v, want it to wrap the underlying flush failure", err)
	}

	c = app.Store.Chunk(c.ID)
	if len(c.Copies) != 0 {
		t.Errorf("a copy was recorded for a write that never reached the medium: %+v", c.Copies)
	}
	if c.VerifiedCopyCount() != 0 {
		t.Errorf("verified copies = %d, want 0", c.VerifiedCopyCount())
	}
	if c.WrittenAt != nil {
		t.Errorf("package marked written at %v despite the failed flush", c.WrittenAt)
	}
	// A bad medium is not a bad package: the staged artifact is intact, so the
	// package falls back to STAGED rather than FAILED.
	if c.Status != "STAGED" {
		t.Errorf("status = %s, want STAGED (the staging copy survives a bad medium)", c.Status)
	}
}
