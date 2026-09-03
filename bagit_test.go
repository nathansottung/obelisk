package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteBagItTags(t *testing.T) {
	dir := t.TempDir()
	c := &Chunk{Name: "PKG-1", MediaKind: "HDD", EncHash: "deadbeef", HashAlg: "SHA256",
		Files: []ChunkFileRef{
			{FileID: 1, RelPath: "trip/a.nef", SizeBytes: 100, Hash: "aa"},
			{FileID: 2, RelPath: "trip/b.nef", SizeBytes: 50, Hash: "bb"},
		}}
	writeBagItTags(dir, c)

	decl, err := os.ReadFile(filepath.Join(dir, "bagit.txt"))
	if err != nil || !strings.Contains(string(decl), "BagIt-Version: 1.0") {
		t.Fatalf("bagit.txt wrong: %v / %s", err, decl)
	}
	man, err := os.ReadFile(filepath.Join(dir, "manifest-sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Manifest lists source files by their ORIGINAL tree paths (no data/ prefix — the
	// sidecar/in-tar manifest describes the tree the payload tar yields), with SHA-256.
	if !strings.Contains(string(man), "aa  trip/a.nef") || !strings.Contains(string(man), "bb  trip/b.nef") {
		t.Fatalf("manifest should list tree-relative <relpath> with sha256:\n%s", man)
	}
	if strings.Contains(string(man), "data/") {
		t.Errorf("the per-package manifest must NOT use a data/ prefix (that is export-only):\n%s", man)
	}
	info, _ := os.ReadFile(filepath.Join(dir, "bag-info.txt"))
	if !strings.Contains(string(info), "Payload-Oxum: 150.2") {
		t.Fatalf("Payload-Oxum should be totalbytes.count (150.2):\n%s", info)
	}
}

func TestExportBagConformant(t *testing.T) {
	a := versApp(t)
	coll := a.Store.AddCollection("Arch")
	// A staged package: a folder with a payload + par2 + manifest + RESTORE.txt.
	staged := t.TempDir()
	writeF := func(name, content string) {
		if err := os.WriteFile(filepath.Join(staged, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeF("PKG-1.tar", "PAYLOAD-TAR-BYTES")
	writeF("PKG-1.tar.par2", "PAR2")
	writeF("PKG-1.manifest.json", `{"name":"PKG-1"}`)
	writeF("RESTORE.txt", "how to restore")
	writeF("bagit.txt", "should be skipped") // per-package tag file must NOT nest in data/
	a.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-1", Status: "STAGED", StagedDir: staged, MediaKind: "HDD",
		Files: []ChunkFileRef{{FileID: 1, RelPath: "a.nef", SizeBytes: 5, Hash: "aa"}}})

	out := t.TempDir()
	res, err := a.ExportBag(coll.ID, out, func(float64, string) {})
	if err != nil {
		t.Fatalf("ExportBag: %v", err)
	}
	bag := res["bag"].(string)

	// Required bag structure.
	for _, f := range []string{"bagit.txt", "bag-info.txt", "manifest-sha256.txt", "tagmanifest-sha256.txt", "COMPARISON.md"} {
		if _, err := os.Stat(filepath.Join(bag, f)); err != nil {
			t.Errorf("missing bag tag file %s: %v", f, err)
		}
	}
	// data/ holds the package artifacts (but NOT the per-package bagit.txt).
	if _, err := os.Stat(filepath.Join(bag, "data", "PKG-1", "PKG-1.tar")); err != nil {
		t.Errorf("data/PKG-1/PKG-1.tar should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bag, "data", "PKG-1", "bagit.txt")); !os.IsNotExist(err) {
		t.Error("per-package bagit.txt must not be nested inside data/")
	}
	// COMPARISON.md answers the r/DataHoarder question.
	comp, _ := os.ReadFile(filepath.Join(bag, "COMPARISON.md"))
	for _, name := range []string{"restic", "borg", "Bacula", "dar", "Canister"} {
		if !strings.Contains(string(comp), name) {
			t.Errorf("COMPARISON.md should address %q", name)
		}
	}
	// Every payload manifest line must match the actual file on disk (valid bag).
	verifyBagManifest(t, bag)
	if conf, _ := res["conformant"].(bool); !conf {
		t.Error("a fully-staged archive should export a conformant bag")
	}
}

// TestExportPackageBag proves the per-package "Export as BagIt" action produces a
// conformant single-package bag whose payload manifest matches the data/ files.
func TestExportPackageBag(t *testing.T) {
	a := versApp(t)
	coll := a.Store.AddCollection("Arch")
	staged := t.TempDir()
	for name, content := range map[string]string{
		"PKG-9.tar": "PAYLOAD", "PKG-9.tar.par2": "PAR2", "PKG-9.manifest.json": `{"name":"PKG-9"}`,
		"RESTORE.txt": "how", "manifest-sha256.txt": "aa  a.nef\n", // per-package tag must not nest in data/
	} {
		if err := os.WriteFile(filepath.Join(staged, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := a.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-9", Status: "STAGED", StagedDir: staged, MediaKind: "HDD",
		Files: []ChunkFileRef{{FileID: 1, RelPath: "a.nef", SizeBytes: 5, Hash: "aa"}}})

	out := t.TempDir()
	res, err := a.ExportPackageBag(c.ID, out, func(float64, string) {})
	if err != nil {
		t.Fatalf("ExportPackageBag: %v", err)
	}
	bag := res["bag"].(string)
	if _, err := os.Stat(filepath.Join(bag, "data", "PKG-9", "PKG-9.tar")); err != nil {
		t.Errorf("data/PKG-9/PKG-9.tar should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bag, "data", "PKG-9", "manifest-sha256.txt")); !os.IsNotExist(err) {
		t.Error("the per-package manifest tag file must not nest inside data/")
	}
	if pkgs, _ := res["packages"].(int); pkgs != 1 {
		t.Errorf("a single-package bag should report 1 package, got %d", pkgs)
	}
	verifyBagManifest(t, bag)

	// A package that isn't staged locally can't be bagged — clear error, no panic.
	unstaged := a.Store.AddChunk(Chunk{CollectionID: coll.ID, Name: "PKG-10", Status: "PLANNED", MediaKind: "HDD"})
	if _, err := a.ExportPackageBag(unstaged.ID, out, nil); err == nil {
		t.Error("exporting an unstaged package should error")
	}
}

// verifyBagManifest checks manifest-sha256.txt against the data/ payload.
func verifyBagManifest(t *testing.T, bag string) {
	t.Helper()
	f, err := os.Open(filepath.Join(bag, "manifest-sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("bad manifest line: %q", line)
		}
		data, err := os.ReadFile(filepath.Join(bag, filepath.FromSlash(parts[1])))
		if err != nil {
			t.Errorf("manifest lists %s but it is missing: %v", parts[1], err)
			continue
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != parts[0] {
			t.Errorf("checksum mismatch for %s", parts[1])
		}
		n++
	}
	if n == 0 {
		t.Error("bag manifest is empty")
	}
}

// ---- RFC 8493 §2.1.3 path encoding -----------------------------------------
//
// The manifest ships on the medium — inside the tar and beside the package — and is
// meant to be read by stock BagIt tooling with no Obelisk present. A filename holding a
// newline would otherwise end the manifest line early and forge a second, bogus entry,
// which a validator reads as fact. The spec's rule is exact: LF, CR, CRLF and "%" (and
// ONLY those) are percent-encoded in the filepath column.

func TestBagEncodePath_EncodesOnlyWhatRFC8493Requires(t *testing.T) {
	cases := []struct{ in, want, why string }{
		{"trip/a.nef", "trip/a.nef", "an ordinary path is untouched"},
		{"line\nbreak.nef", "line%0Abreak.nef", "LF would end the manifest line"},
		{"carriage\rreturn.nef", "carriage%0Dreturn.nef", "CR would end the line for CRLF readers"},
		{"crlf\r\nname.nef", "crlf%0D%0Aname.nef", "CRLF encodes as its two parts"},
		{"100%pure.nef", "100%25pure.nef", "a literal percent must not open an escape"},
		{"literal%0Aname.nef", "literal%250Aname.nef", "a name that LOOKS like an escape is encoded, not passed through"},
		{"%.nef", "%25.nef", "a bare trailing percent"},
		// "…and only those": encoding anything else would change the path a
		// validator compares against the file on disk.
		{"space and 'quotes' ünïcødé★.nef", "space and 'quotes' ünïcødé★.nef", "spaces, quotes and non-ASCII stay literal"},
		{"tab\there.nef", "tab\there.nef", "a tab is not one of the three"},
	}
	for _, c := range cases {
		got := bagEncodePath(c.in)
		if got != c.want {
			t.Errorf("bagEncodePath(%q) = %q, want %q — %s", c.in, got, c.want, c.why)
		}
		if back := bagDecodePath(got); back != c.in {
			t.Errorf("round trip broken: %q → %q → %q", c.in, got, back)
		}
	}
	// A decoder must accept lowercase hex too — another producer's manifest may use it.
	if got := bagDecodePath("low%0acase%0d%25.nef"); got != "low\ncase\r%.nef" {
		t.Errorf("lowercase escapes not decoded: %q", got)
	}
}

func TestBagPayloadManifest_HostileNamesStayOneLinePerFile(t *testing.T) {
	files := []ChunkFileRef{
		{FileID: 1, RelPath: "trip/ordinary.nef", SizeBytes: 10, Hash: "aa"},
		{FileID: 2, RelPath: "trip/line\nbreak.nef", SizeBytes: 10, Hash: "bb"},
		{FileID: 3, RelPath: "trip/carriage\rreturn.nef", SizeBytes: 10, Hash: "cc"},
		{FileID: 4, RelPath: "trip/100%pure.nef", SizeBytes: 10, Hash: "dd"},
		{FileID: 5, RelPath: "trip/crlf\r\nboth.nef", SizeBytes: 10, Hash: "ee"},
	}
	man := bagPayloadManifest(files)

	lines := strings.Split(strings.TrimSuffix(man, "\n"), "\n")
	if len(lines) != len(files) {
		t.Fatalf("manifest has %d lines for %d files — a name forged an extra entry:\n%q", len(lines), len(files), man)
	}
	// Every line parses as "<checksum><whitespace><filepath>" and decodes back to the
	// exact name that went in — the property third-party tooling depends on.
	byHash := map[string]string{}
	for _, ln := range lines {
		i := strings.IndexAny(ln, " \t")
		if i < 0 {
			t.Fatalf("manifest line has no separator: %q", ln)
		}
		hash := ln[:i]
		pathCol := strings.TrimLeft(ln[i:], " \t")
		if strings.ContainsAny(pathCol, "\n\r") {
			t.Errorf("raw newline/CR left in the path column: %q", pathCol)
		}
		byHash[hash] = bagDecodePath(pathCol)
	}
	for _, f := range files {
		if got := byHash[f.Hash]; got != f.RelPath {
			t.Errorf("path for %s decoded to %q, want %q", f.Hash, got, f.RelPath)
		}
	}
	// Spot-check the wire form, so a future refactor cannot quietly switch encodings.
	if !strings.Contains(man, "bb  trip/line%0Abreak.nef") {
		t.Errorf("LF should be written as %%0A:\n%s", man)
	}
	if !strings.Contains(man, "dd  trip/100%25pure.nef") {
		t.Errorf("%% should be written as %%25:\n%s", man)
	}
}

// The same rule must hold for the manifest actually written to the medium, not just
// for the string builder — this is the file a curator's validator opens.
func TestWriteBagItTags_ManifestOnDiskIsEncoded(t *testing.T) {
	dir := t.TempDir()
	c := &Chunk{Name: "PKG-HOSTILE", MediaKind: "HDD", EncHash: "deadbeef", HashAlg: "SHA256",
		Files: []ChunkFileRef{
			{FileID: 1, RelPath: "a.nef", SizeBytes: 1, Hash: "aa"},
			{FileID: 2, RelPath: "two\nlines.nef", SizeBytes: 1, Hash: "bb"},
		}}
	if err := writeBagItTags(dir, c); err != nil {
		t.Fatal(err)
	}
	man, err := os.ReadFile(filepath.Join(dir, bagPayloadManifestName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(man), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("on-medium manifest has %d lines for 2 files:\n%s", len(lines), man)
	}
	if !strings.Contains(string(man), "bb  two%0Alines.nef") {
		t.Errorf("on-medium manifest is not encoded:\n%s", man)
	}
}
