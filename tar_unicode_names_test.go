package main

// tar_unicode_names_test.go — OBX-006 regressions: the archive-build path must put the
// EXACT selected Unicode filenames, and their exact bytes, into the package.
//
// These sit alongside tar_names_test.go rather than inside it. That file proves the
// shell-shaped hazards (newline, leading dash, quoting); this one proves the ENCODING
// hazard, which is a different failure with a different signature: the helper can exit 0
// having archived a DIFFERENT, similarly-named file. So every assertion here reads the
// produced archive with Go's own archive/tar reader and compares member names and content
// hashes — a zero exit status is never accepted as evidence.
//
// Names are constructed as Go string literals so the fixture bytes are unambiguous and do
// not depend on the encoding any shell or editor would apply.

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Unicode fixture names, chosen to span the ways a Windows ANSI code page can lose a name.
// cp1252Safe is true when every rune has a Windows-1252 representation — the ones where a
// code-page round trip is merely lossy rather than impossible.
//
// label is ASCII on purpose. Go builds t.TempDir() paths out of the (sub)test name, so a
// Unicode subtest name would put Unicode into the staging and source ROOT paths as well as
// into the filename — three variables at once, and the failure would be reported against
// whichever the helper tripped over first. Keeping the label ASCII leaves the fixture name
// as the single variable under test; the root-path cases have their own tests below.
var unicodeNameCases = []struct {
	label      string
	rel        string
	why        string
	cp1252Safe bool
}{
	{"ascii", "ordinary.txt", "ASCII control", true},
	{"accented", "café.txt", "accented, representable in CP1252", true},
	{"japanese", "日本語.txt", "Japanese, no CP1252 representation", false},
	{"supplementary", "\U0001d54fsupplementary.txt", "supplementary plane (U+1D54F, a surrogate pair in UTF-16)", false},
	{"bmp-symbol", "star★.txt", "BMP symbol, no CP1252 representation", false},
	{"unicode-dir-component", "日本dir/ascii.txt", "ASCII leaf beneath a Unicode DIRECTORY component", false},
}

func unicodeNames() []string {
	out := make([]string, 0, len(unicodeNameCases))
	for _, c := range unicodeNameCases {
		out = append(out, c.rel)
	}
	return out
}

// unicodeBody gives every fixture file distinct, known bytes, so a test can tell a
// correctly named member from a look-alike neighbour by content alone.
func unicodeBody(rel string) string { return "OBX-006 payload for <" + rel + ">\n" }

// makeUnicodeSource writes one file per name under root and CHECKS the fixture actually
// exists under the exact name requested — reading the directory back and comparing bytes,
// not just trusting os.WriteFile. A filesystem that silently normalises the name (some
// non-Windows filesystems apply NFD) would otherwise make this suite test something other
// than what it claims; that is reported as a fixture limitation, never as a pass. It is
// NOT a way to excuse a product failure: on Windows/NTFS the names below are stored
// verbatim, so this branch cannot hide the OBX-006 defect.
func makeUnicodeSource(t *testing.T, root string, names []string) []ChunkFileRef {
	t.Helper()
	var refs []ChunkFileRef
	for _, rel := range names {
		body := unicodeBody(rel)
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("cannot create the directory for %q: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("cannot create the fixture %q (% x): %v", rel, []byte(rel), err)
		}
		refs = append(refs, ChunkFileRef{RelPath: rel, SizeBytes: int64(len(body)),
			Hash: sha256Hex([]byte(body))})
	}
	// The fixture must be byte-identical to what was asked for, or the assertions below
	// would be measuring the filesystem rather than the product.
	onDisk := map[string]bool{}
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		r, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		onDisk[filepath.ToSlash(r)] = true
		return nil
	}); err != nil {
		t.Fatalf("cannot read the fixture tree back: %v", err)
	}
	for _, rel := range names {
		if !onDisk[rel] {
			t.Skipf("fixture limitation, not a product result: this filesystem did not store %q "+
				"(% x) under the exact name written — it stored %v. The Unicode build path cannot "+
				"be tested here.", rel, []byte(rel), sortedNames(onDisk))
		}
	}
	return refs
}

func sortedNames(m map[string]bool) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

// newUnicodeTestApp is newTestApp with a caller-chosen staging directory, so a test can
// put the staging/output path itself outside ASCII. It deliberately does not modify
// newTestApp, whose existing callers rely on its temp-dir behaviour.
func newUnicodeTestApp(t *testing.T, tools map[string]string, staging string) *App {
	t.Helper()
	dataDir := t.TempDir()
	store, err := OpenStore(dataDir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	app := initializedTestApp(t, &App{DataDir: dataDir, Store: store})
	ks1 := filepath.Join(t.TempDir(), "keystore1.json")
	ks2 := filepath.Join(t.TempDir(), "keystore2.json")
	if err := writeStore(ks1, &keystoreFile{Marker: 1}); err != nil {
		t.Fatal(err)
	}
	if err := writeStore(ks2, &keystoreFile{Marker: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SaveConfig(map[string]any{
		"staging_dir":     staging,
		"keystore_paths":  []string{ks1, ks2},
		"par2_redundancy": 5,
		"tools":           tools,
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	return app
}

// tarPayload reads a staged tar with Go's standard-library reader — an INDEPENDENT
// structured reader, not the same tar executable's human-readable listing — and returns
// every regular member as name -> sha256 of its bytes.
func tarPayload(t *testing.T, tarPath string) map[string]string {
	t.Helper()
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatalf("cannot open the staged tar %s: %v", tarPath, err)
	}
	defer f.Close()
	got := map[string]string{}
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the staged tar: %v", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		h := sha256.New()
		if _, err := io.Copy(h, tr); err != nil {
			t.Fatalf("reading member %q: %v", name, err)
		}
		if _, dup := got[name]; dup {
			t.Errorf("the package contains %q more than once", name)
		}
		got[name] = hex.EncodeToString(h.Sum(nil))
	}
	return got
}

// assertExactPayload demands the EXACT intended member set: the BagIt payload manifest
// (the documented first member) plus one member per catalogued file, each with matching
// content, and nothing else. No duplicates, no unrequested source files, no renamed
// approximations.
func assertExactPayload(t *testing.T, tarPath string, refs []ChunkFileRef) {
	t.Helper()
	got := tarPayload(t, tarPath)
	if _, ok := got[bagPayloadManifestName]; !ok {
		t.Errorf("the package is missing its BagIt payload manifest %q", bagPayloadManifestName)
	}
	delete(got, bagPayloadManifestName)
	for _, r := range refs {
		h, ok := got[r.RelPath]
		if !ok {
			t.Errorf("the package does not contain %q (% x) — the intended member is absent",
				r.RelPath, []byte(r.RelPath))
			continue
		}
		if h != r.Hash {
			t.Errorf("member %q has content hash %s, want %s — the package holds different bytes "+
				"than the selected source file", r.RelPath, h, r.Hash)
		}
		delete(got, r.RelPath)
	}
	for name := range got {
		t.Errorf("the package contains %q (% x), which was not selected — a differently named "+
			"neighbour or an unexpected source file was archived", name, []byte(name))
	}
}

// stagedTar is where BuildChunk leaves the plaintext tar for an unencrypted package.
func stagedTar(staging string, c *Chunk) string {
	return filepath.Join(staging, c.Name, c.Name+".tar")
}

func planUnicodeChunk(app *App, name, src string, refs []ChunkFileRef) *Chunk {
	return app.Store.AddChunk(Chunk{
		Name: name, Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: false,
		Files: append([]ChunkFileRef{}, refs...),
	})
}

// TestBuildChunk_UnicodeFilenames_ExactMembers is the core OBX-006 regression: each
// Unicode shape must reach the archive under its own name, with its own bytes. Each case
// is a subtest so a partial capability boundary is reported per name rather than collapsed
// into a single failure.
func TestBuildChunk_UnicodeFilenames_ExactMembers(t *testing.T) {
	tools := nativeTools(t)
	for _, tc := range unicodeNameCases {
		t.Run(tc.label, func(t *testing.T) {
			app, staging := newTestApp(t, tools)
			src := t.TempDir()
			refs := makeUnicodeSource(t, src, []string{tc.rel})
			c := planUnicodeChunk(app, "UNI-ONE", src, refs)
			if err := app.BuildChunk(c.ID, noProg); err != nil {
				t.Fatalf("BuildChunk with a %s filename %q (% x): %v",
					tc.why, tc.rel, []byte(tc.rel), err)
			}
			if got := app.Store.Chunk(c.ID); got.Status != "STAGED" {
				t.Fatalf("status = %s (%s), want STAGED", got.Status, got.Error)
			}
			assertExactPayload(t, stagedTar(staging, c), refs)
		})
	}
}

// TestBuildChunk_WrongFileSelection_LookalikeNeighbour is the wrong-file regression.
// "café.txt" encoded UTF-8 and then decoded as Windows-1252 spells "cafÃ©.txt" exactly —
// so if the build hands the helper UTF-8 bytes that the helper reads in the ANSI code
// page, and that other file exists, the build can succeed while archiving the WRONG file.
// The two files carry distinct known bytes, so name and content each independently catch
// it.
func TestBuildChunk_WrongFileSelection_LookalikeNeighbour(t *testing.T) {
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	src := t.TempDir()

	const wanted = "café.txt" // UTF-8 bytes: 63 61 66 c3 a9 2e 74 78 74
	const decoy = "cafÃ©.txt" // what those UTF-8 bytes spell when read as Windows-1252
	all := makeUnicodeSource(t, src, []string{wanted, decoy})
	if all[0].Hash == all[1].Hash {
		t.Fatalf("fixture error: the two files must have distinct bytes")
	}

	// Select ONLY café.txt. The decoy stays on disk, unselected.
	refs := []ChunkFileRef{all[0]}
	c := planUnicodeChunk(app, "UNI-DECOY", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk selecting only %q while %q sits beside it: %v", wanted, decoy, err)
	}
	// The package must hold café.txt with café.txt's bytes — not the neighbour, not both,
	// not a renamed approximation.
	assertExactPayload(t, stagedTar(staging, c), refs)
}

// TestBuildChunk_UnicodeSourceRoot puts the Unicode outside the tree, in the source root
// path the build passes as the helper's working directory. The archived member names are
// tree-relative ASCII, so nothing about the package layout changes — only the path the
// helper must resolve.
func TestBuildChunk_UnicodeSourceRoot(t *testing.T) {
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	src := filepath.Join(t.TempDir(), "source 日本★")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("cannot create a Unicode source root: %v", err)
	}
	refs := makeUnicodeSource(t, src, []string{"ordinary.txt", "sub/plain.txt"})
	c := planUnicodeChunk(app, "UNI-SRCROOT", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk with a Unicode source root %q: %v", src, err)
	}
	assertExactPayload(t, stagedTar(staging, c), refs)
}

// TestBuildChunk_UnicodeStagingDir puts the Unicode in the staging/output path — the
// directory the build writes the tar and the BagIt manifest into. Like the source root,
// this never appears in a member name.
func TestBuildChunk_UnicodeStagingDir(t *testing.T) {
	tools := nativeTools(t)
	staging := filepath.Join(t.TempDir(), "staging 日本★")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatalf("cannot create a Unicode staging dir: %v", err)
	}
	app := newUnicodeTestApp(t, tools, staging)
	src := t.TempDir()
	refs := makeUnicodeSource(t, src, []string{"ordinary.txt", "café.txt"})
	c := planUnicodeChunk(app, "UNI-STAGING", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk with a Unicode staging dir %q: %v", staging, err)
	}
	assertExactPayload(t, stagedTar(staging, c), refs)
}

// TestBuildRestore_UnicodeRoundTrip separates a successful BUILD from a successful
// COMPLETE ROUND TRIP: it writes the package to a medium, restores the whole tree, and
// then restores ONE selected Unicode member at a time, comparing restored paths and bytes
// both times. A restore that exits 0 without producing the requested file fails here.
func TestBuildRestore_UnicodeRoundTrip(t *testing.T) {
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	src := t.TempDir()
	refs := makeUnicodeSource(t, src, unicodeNames())

	c := planUnicodeChunk(app, "UNI-ROUNDTRIP", src, refs)
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk with Unicode filenames: %v", err)
	}
	assertExactPayload(t, stagedTar(staging, c), refs)

	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	pkgDir := filepath.Join(medium, c.Name)

	out := t.TempDir()
	if _, err := app.RestoreChunk(c.ID, pkgDir, out, nil, noProg); err != nil {
		t.Fatalf("RestoreChunk (whole package): %v", err)
	}
	for _, r := range refs {
		got, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(r.RelPath)))
		if err != nil {
			t.Errorf("whole-package restore did not produce %q (% x): %v",
				r.RelPath, []byte(r.RelPath), err)
			continue
		}
		if h := sha256Hex(got); h != r.Hash {
			t.Errorf("restored %q has hash %s, want %s", r.RelPath, h, r.Hash)
		}
	}

	// Selected-member restore, one Unicode member at a time: the requested path and bytes
	// must appear, and nothing else may.
	for i, r := range refs {
		only := filepath.Join(t.TempDir(), fmt.Sprintf("sel%d", i))
		if err := os.MkdirAll(only, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := app.RestoreChunk(c.ID, pkgDir, only, []string{r.RelPath}, noProg); err != nil {
			t.Errorf("selected-member restore of %q (% x): %v", r.RelPath, []byte(r.RelPath), err)
			continue
		}
		got, err := os.ReadFile(filepath.Join(only, filepath.FromSlash(r.RelPath)))
		if err != nil {
			t.Errorf("selected-member restore of %q exited without producing it: %v", r.RelPath, err)
			continue
		}
		if h := sha256Hex(got); h != r.Hash {
			t.Errorf("selected-member restore of %q gave hash %s, want %s", r.RelPath, h, r.Hash)
		}
		var extra []string
		_ = filepath.WalkDir(only, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(only, p)
			if filepath.ToSlash(rel) != r.RelPath {
				extra = append(extra, filepath.ToSlash(rel))
			}
			return nil
		})
		if len(extra) > 0 {
			t.Errorf("selected-member restore of %q also extracted %v", r.RelPath, extra)
		}
	}
}
