//go:build !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A spanned package is marked verified only when its whole set checks: each
// segment as written must be its range of a staged payload that still hashes
// to the package's hash. A payload damaged between segments fails the set.
func TestSpanWrite_WholeSetIsChecked(t *testing.T) {
	app, staging := newTestApp(t, nativeTools(t))
	src := t.TempDir()
	big := strings.Repeat("spanning-payload-", 2500)
	mustWrite(t, filepath.Join(src, "big.dat"), big)
	c := app.Store.AddChunk(Chunk{
		Name: "SPAN-SET", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 12000, DataBytes: int64(len(big)), FileCount: 1,
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Spanned: true,
		Segments: planSegments(int64(len(big))+4096, 12000),
		Files:    []ChunkFileRef{{RelPath: "big.dat", SizeBytes: int64(len(big))}},
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk: %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if dataSegmentCount(c.Segments) < 2 {
		t.Fatalf("setup: %d data segments", dataSegmentCount(c.Segments))
	}
	medium := t.TempDir()
	if _, err := app.SpanWriteNext(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatal(err)
	}
	// The staged payload changes in the second segment's range.
	payload := filepath.Join(staging, c.Name, payloadName(c))
	b, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	b[c.Segments[0].Bytes+10] ^= 0xff
	if err := os.WriteFile(payload, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var last error
	for i := 0; i < 50; i++ {
		res, err := app.SpanWriteNext(c.ID, medium, 0, 0, 0, 0, noProg)
		if err != nil {
			last = err
			break
		}
		if done, _ := res["complete"].(bool); done {
			break
		}
	}
	c = app.Store.Chunk(c.ID)
	if last == nil || c.Status == "VERIFIED" || c.VerifiedCopyCount() != 0 {
		t.Errorf("a set written from a damaged payload: err %v, status %s, %d verified copies; want refused", last, c.Status, c.VerifiedCopyCount())
	}
}
