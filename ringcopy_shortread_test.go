package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRingCopy_ShortSourceSurfacesReadError proves that a bounded (byte-range) copy
// whose source ends before the requested length returns the real I/O (short-read)
// error — not a nil error with a truncated stream, which would later surface as a
// misleading "hash mismatch". This is the fail-closed guarantee the span-write and
// write callers rely on (they check the returned error before comparing hashes).
func TestRingCopy_ShortSourceSurfacesReadError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, []byte("thirteen bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst.bin")

	// Ask for far more bytes than the source holds → truncated read.
	hash, _, err := ringCopy(src, dst, 0, 1<<20, 1, 0.01, 0, func(done, total int64) {})
	if err == nil {
		t.Fatal("expected a short-read I/O error; got nil (a truncation would otherwise manifest downstream as a hash mismatch)")
	}
	if !strings.Contains(err.Error(), "short read") {
		t.Errorf("error = %q, want it to name the short read", err.Error())
	}
	if hash != "" {
		t.Errorf("hash must be empty on a read error, got %q", hash)
	}
}

// TestRingCopy_WholeFileCopyIsClean proves the whole-file path (length<=0) still
// treats EOF as the normal end of stream — the short-read guard must not fire when
// no explicit length was requested.
func TestRingCopy_WholeFileCopyIsClean(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	data := []byte("a whole file that copies cleanly to EOF")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst.bin")

	hash, stats, err := ringCopy(src, dst, 0, 0, 1, 0.01, 0, func(done, total int64) {})
	if err != nil {
		t.Fatalf("whole-file copy should succeed, got %v", err)
	}
	if hash == "" {
		t.Error("expected a non-empty stream hash on success")
	}
	if stats.Bytes != int64(len(data)) {
		t.Errorf("copied %d bytes, want %d", stats.Bytes, len(data))
	}
	got, _ := os.ReadFile(dst)
	if string(got) != string(data) {
		t.Errorf("destination content mismatch")
	}
}
