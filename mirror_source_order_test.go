//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A source that cannot be opened must not truncate the staged destination, even
// when that name is a hard link to a file the operator cares about.
func TestMirrorCopyFile_SourceOpenFailurePreservesHardLink(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "protected-original")
	tmp := filepath.Join(dir, "planned.txt"+mirrorTmpSuffix)
	mustWrite(t, original, "ORIGINAL")
	if err := os.Link(original, tmp); err != nil {
		t.Skipf("hard links unavailable here: %v", err)
	}

	_, n, err := mirrorCopyFile(filepath.Join(dir, "missing-source"), tmp, nil, nil)
	if err == nil || n != 0 {
		t.Fatalf("missing source: bytes=%d err=%v; want a failure and nothing written", n, err)
	}
	if !errors.Is(err, errCopySource) {
		t.Fatalf("missing source: %v is not marked as a source failure", err)
	}
	mustBytes(t, original, "ORIGINAL", "source-open failure")
	mustBytes(t, tmp, "ORIGINAL", "source-open failure")
	a, aerr := os.Stat(original)
	b, berr := os.Stat(tmp)
	if aerr != nil || berr != nil || !os.SameFile(a, b) {
		t.Fatalf("source-open failure replaced the staged name: %v %v", aerr, berr)
	}
}
