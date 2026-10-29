//go:build !guionly

package main

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// brokenTar is a configured "tar" that cannot run, so a build that still calls
// the external tar fails.
func brokenTar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		p := filepath.Join(dir, "tar.exe")
		if err := os.WriteFile(p, []byte("not a program"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := filepath.Join(dir, "tar")
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho 'external tar called' >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A package is written by Obelisk itself, never by the external tar: a build
// with a tar that cannot run still succeeds, and holds the exact members.
func TestBuildChunk_DoesNotRunExternalTar(t *testing.T) {
	tools := nativeTools(t)
	tools["tar"] = brokenTar(t)
	app, staging := newTestApp(t, tools)
	src := t.TempDir()
	refs := makeUnicodeSource(t, src, []string{"ordinary.txt", "café.txt", "日本dir/ascii.txt"})
	c := planUnicodeChunk(app, "NATIVE-TAR", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk with an external tar that cannot run: %v", err)
	}
	assertExactPayload(t, stagedTar(staging, c), refs)
}

// The package is a POSIX pax archive: ustar headers, with each non-ASCII name
// in a pax "path" record as UTF-8, so any stock tar restores it under its own
// name. The manifest leads the archive.
func TestBuildChunk_WritesPaxWithUTF8Names(t *testing.T) {
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	src := t.TempDir()
	names := []string{"ordinary.txt", "日本語.txt", "\U0001d54fsupplementary.txt"}
	refs := makeUnicodeSource(t, src, names)
	c := planUnicodeChunk(app, "PAX-NAMES", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(stagedTar(staging, c))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 512 || string(raw[257:263]) != "ustar\x00" || string(raw[263:265]) != "00" {
		t.Fatalf("the first header is not POSIX ustar/pax (magic %q version %q)", raw[257:263], raw[263:265])
	}
	for _, n := range names[1:] {
		rec := []byte(" path=" + n + "\n")
		if !bytes.Contains(raw, rec) {
			t.Errorf("no pax path record holds %q as UTF-8", n)
		}
	}
	tr := tar.NewReader(bytes.NewReader(raw))
	var order []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Format != tar.FormatUnknown && h.Format&(tar.FormatPAX|tar.FormatUSTAR) == 0 {
			t.Errorf("member %q has format %v, want pax/ustar", h.Name, h.Format)
		}
		order = append(order, h.Name)
	}
	if len(order) == 0 || order[0] != bagPayloadManifestName {
		t.Errorf("members %v, want the manifest first", order)
	}
}

// A file that is not a regular file is refused, not archived.
func TestBuildChunk_RefusesANonRegularFile(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	c := planUnicodeChunk(app, "NOT-REGULAR", src, []ChunkFileRef{{RelPath: "adir", SizeBytes: 0}})
	err := app.BuildChunk(c.ID, noProg)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("building a package that names a folder: %v, want refused", err)
	}
}

// A member name that would leave the restore folder is refused by restore.
func TestExtractTar_RefusesEscapingNames(t *testing.T) {
	for _, name := range []string{"../escape.txt", "a/../../escape.txt", "/abs.txt"} {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		_ = tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 1, Mode: 0o644})
		_, _ = tw.Write([]byte("x"))
		_ = tw.Close()
		into := t.TempDir()
		if err := extractTar(&buf, into, nil); err == nil {
			t.Errorf("member %q was extracted, want refused", name)
		}
	}
}

// A selected member that is not in the package is an error, as with tar.
func TestExtractTar_MissingMemberIsAnError(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	if err := extractTar(bytes.NewReader(buf.Bytes()), t.TempDir(), []string{"b.txt"}); err == nil {
		t.Error("restoring a member that is not in the package succeeded")
	}
	into := t.TempDir()
	if err := extractTar(bytes.NewReader(buf.Bytes()), into, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(into, "a.txt")); err != nil || string(b) != "x" {
		t.Errorf("a.txt: %q %v", b, err)
	}
}
