package main

// tar_names_test.go — hostile-but-legal filenames must survive a full build → write →
// restore round trip, byte for byte.
//
// Two shell-shaped assumptions used to break this. The build fed tar a
// NEWLINE-delimited -T list, so a filename containing a newline (legal on POSIX) split
// into two paths that do not exist, and a filename starting with "-" was read by tar as
// an OPTION — both of which take out the whole package build, not just that file. And
// the restore appended caller-chosen member names straight onto tar's argv, so a member
// beginning with "-" was parsed as an option instead of as the file to restore.
//
// The fixes are --null -T (names taken verbatim) and a literal "--" before the members.
// This test is the proof, so it uses real filenames of each hostile shape rather than
// mocking tar.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hostileNames returns tree-relative names legal for the test platform and awkward for a
// shell. Windows filename rules exclude TAB and newline, so those fixtures stay in the
// non-Windows group, where the existing long-path fixture also remains. Unicode, leading
// dashes, spaces and quotes are valid Windows names and stay unconditional — gated here
// rather than left to fail on a box the developer is not looking at. Gating is not a
// softer assertion: a name the platform will not create makes the fixture die in
// os.WriteFile, so the product code under test never runs at all.
func hostileNames() []string {
	names := []string{
		"ordinary.txt",
		"--dashy.txt",             // the argv-injection case: tar reads this as an option
		"sub/--also-dashy.txt",    // …and in a subfolder, where the basename still leads
		"-x",                      // short-option shape
		"ünïcødé★ 日本語.txt",        // non-ASCII, with a space
		"spaces and 'quotes'.txt", // shell-quoting bait
	}
	if runtime.GOOS != "windows" {
		names = append(names,
			"line\nbreak.txt",          // the -T list splitter
			"sub/two\nlines\nhere.txt", // more than one split, in a subfolder
			"tab\there.txt",            // a control character that is not a newline
			// A path over 260 characters: fine on POSIX (the limit is per component),
			// but past Windows' default MAX_PATH, so it must not run there.
			strings.Repeat("d", 80)+"/"+strings.Repeat("e", 80)+"/"+
				strings.Repeat("f", 80)+"/"+strings.Repeat("g", 80)+"/deep.txt",
		)
	}
	return names
}

// makeHostileSource writes one file per hostile name and returns the source root plus
// the ChunkFileRefs BuildChunk needs, hashed — so stage verification compares the tar's
// members against real source hashes rather than waving them through.
func makeHostileSource(t *testing.T, names []string) (string, []ChunkFileRef) {
	t.Helper()
	src := t.TempDir()
	var refs []ChunkFileRef
	for i, rel := range names {
		body := "content of file " + string(rune('A'+i)) + ": " + rel + "\n"
		p := filepath.Join(src, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("cannot stage %q: %v", rel, err)
		}
		sum := sha256.Sum256([]byte(body))
		refs = append(refs, ChunkFileRef{RelPath: rel, SizeBytes: int64(len(body)), Hash: hex.EncodeToString(sum[:])})
	}
	return src, refs
}

func TestBuildRestore_HostileFilenamesRoundTrip(t *testing.T) {
	tools := nativeTools(t)
	app, _ := newTestApp(t, tools)
	names := hostileNames()
	src, refs := makeHostileSource(t, names)

	c := app.Store.AddChunk(Chunk{
		Name: "HOSTILE-NAMES", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: false,
		Files: append([]ChunkFileRef{}, refs...),
	})

	// Build. With a newline-delimited list this fails outright: tar cannot stat the
	// halves of a split name, and reads "--dashy.txt" as an unrecognized option.
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk with hostile filenames: %v", err)
	}
	c = app.Store.Chunk(c.ID)
	if c.Status != "STAGED" {
		t.Fatalf("status = %s (%s), want STAGED", c.Status, c.Error)
	}

	medium := t.TempDir()
	if _, err := app.WriteChunk(c.ID, medium, 0, 0, 0, 0, noProg); err != nil {
		t.Fatalf("WriteChunk: %v", err)
	}
	pkgDir := filepath.Join(medium, c.Name)

	// Whole-package restore: every hostile name comes back, byte for byte, at the
	// same tree-relative path it went in at.
	out := t.TempDir()
	if _, err := app.RestoreChunk(c.ID, pkgDir, out, nil, noProg); err != nil {
		t.Fatalf("RestoreChunk (whole package): %v", err)
	}
	assertRestored(t, src, out, refs)

	// Selective restore of the "--"-prefixed member: this is the argv-injection case.
	// Without a literal "--" ahead of the members, tar answers "unrecognized option
	// '--dashy.txt'" and restores nothing.
	only := t.TempDir()
	if _, err := app.RestoreChunk(c.ID, pkgDir, only, []string{"--dashy.txt"}, noProg); err != nil {
		t.Fatalf("RestoreChunk of a member whose name starts with a dash: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(only, "--dashy.txt"))
	if err != nil {
		t.Fatalf("the dash-named member was not restored: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(src, "--dashy.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("restored --dashy.txt differs from source")
	}
	// A selective restore must restore only what was asked for — proof the member
	// was consumed as a member and not swallowed as an option.
	if _, err := os.Stat(filepath.Join(only, "ordinary.txt")); err == nil {
		t.Error("selective restore extracted files that were not requested")
	}
}

// The build's -T list is NUL-delimited, and every hostile name must appear in it whole.
// A newline-delimited list would show the same bytes but split at the newline, which is
// exactly the failure this guards — so assert the delimiter, not just the content.
func TestBuildFilelist_IsNulDelimited(t *testing.T) {
	tools := nativeTools(t)
	app, staging := newTestApp(t, tools)
	names := hostileNames()
	src, refs := makeHostileSource(t, names)

	c := app.Store.AddChunk(Chunk{
		Name: "NUL-LIST", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: 4096, FileCount: len(refs),
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: false,
		Files: append([]ChunkFileRef{}, refs...),
	})
	if err := app.BuildChunk(c.ID, noProg); err != nil {
		t.Fatalf("BuildChunk: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(staging, c.Name, "filelist.txt"))
	if err != nil {
		t.Fatalf("reading the staged filelist: %v", err)
	}
	entries := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if len(entries) != len(names) {
		t.Fatalf("filelist holds %d NUL-delimited entries, want %d — a name was split", len(entries), len(names))
	}
	byName := map[string]bool{}
	for _, e := range entries {
		byName[e] = true
	}
	for _, n := range names {
		if !byName[n] {
			t.Errorf("filelist is missing %q whole", n)
		}
	}
}
