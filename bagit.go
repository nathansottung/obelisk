package main

// bagit.go — BagIt (RFC 8493) without the trap.
//
// Two things, kept distinct:
//
//  1. Every package gets BagIt-format tag files written BESIDE its payload
//     (bagit.txt, bag-info.txt, manifest-sha256.txt). A curator or an
//     institutional ingest pipeline can read a standard, parseable manifest of
//     exactly what the package preserves — "institutional legibility for free" —
//     without Obelisk changing anything about how the data is stored.
//
//  2. A conformant-bag EXPORT action materializes a fully valid BagIt bag (data/
//     payload + manifest + tagmanifest) for handoff to a repository that ingests
//     bags. The COMPARISON.md ("why not restic/borg/Bacula/dar/Canister") rides
//     along inside it.
//
// The trap BagIt usually sets is that adopting it reshapes your storage into a
// data/ tree you can only navigate through bag tooling. Obelisk refuses that:
// the storage format stays a plain tar that yields your ORIGINAL tree on
// extraction. BagIt here is a *description* layer, never the storage layer.
//
// Every checksum here is SHA-256 — the only hash allowed on media (see hashing.go).

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed docs/COMPARISON.md
var comparisonMD []byte

const bagItDeclaration = "BagIt-Version: 1.0\nTag-File-Character-Encoding: UTF-8\n"

// bagPayloadManifestName is the BagIt manifest filename. It is written as the FIRST
// member INSIDE each package's tar and, identically, as a sidecar beside the payload
// on media — a description layer, never a restructuring of the payload.
const bagPayloadManifestName = "manifest-sha256.txt"

// bagEncodePath percent-encodes the FILEPATH column of a manifest line as RFC 8493
// §2.1.3 requires: "If a filepath includes a Line Feed (LF), a Carriage Return (CR), a
// Carriage-Return Line Feed (CRLF), or a percent sign (%), those characters (and only
// those) MUST be percent-encoded following [RFC3986]." So exactly three replacements —
// % → %25, LF → %0A, CR → %0D (CRLF falls out as %0D%0A) — and nothing else: a space,
// a quote or a non-ASCII character is left alone, since encoding them would change the
// path a validator compares against.
//
// This matters because these are the only bytes that could end a manifest LINE or
// disguise an escape, and a manifest with an ambiguous line is a manifest stock BagIt
// tooling reads wrongly — silently, and on the medium, where Obelisk is not there to
// explain. A single Replacer pass is deliberate: it never rescans what it just wrote,
// so a literal "%0A" in a name encodes to "%250A" and not to a bare newline escape.
func bagEncodePath(p string) string {
	if !strings.ContainsAny(p, "%\n\r") {
		return p
	}
	return strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D").Replace(p)
}

// bagDecodePath is the exact inverse of bagEncodePath, decoding only the three escapes
// the spec defines for this column (hex case-insensitively, since a producer other than
// us may write %0a). Nothing in Obelisk reads these manifests back today — they are
// written for third-party BagIt tooling — but the inverse belongs beside the encoder so
// the round trip is testable and any future reader decodes symmetrically instead of
// inventing its own rules.
func bagDecodePath(p string) string {
	if !strings.Contains(p, "%") {
		return p
	}
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); {
		if p[i] == '%' && i+3 <= len(p) {
			switch strings.ToUpper(p[i+1 : i+3]) {
			case "25":
				b.WriteByte('%')
				i += 3
				continue
			case "0A":
				b.WriteByte('\n')
				i += 3
				continue
			case "0D":
				b.WriteByte('\r')
				i += 3
				continue
			}
		}
		b.WriteByte(p[i])
		i++
	}
	return b.String()
}

// bagOxum returns the BagIt Payload-Oxum "octetstream sum": total bytes "." file
// count over the source files a package preserves.
func bagOxum(files []ChunkFileRef) (bytes int64, count int) {
	for _, f := range files {
		bytes += f.SizeBytes
		count++
	}
	return
}

// bagPayloadManifest renders BagIt manifest lines ("<sha256>  <relpath>") over a
// package's source files, sorted by path for a stable, diffable manifest. Paths are
// percent-encoded per RFC 8493 (see bagEncodePath) so a name holding a newline cannot
// forge a second manifest line. The paths are otherwise the ORIGINAL tree-relative paths — exactly what the payload tar yields on
// extraction (no data/ prefix), because this manifest lives inside that tar and
// beside it on media, describing the tree as it comes out. Files with no recorded
// SHA-256 (legacy/adopted-without-hash) are skipped so no unverifiable entry is
// listed; the conformant EXPORT rehashes every byte and is always complete.
func bagPayloadManifest(files []ChunkFileRef) string {
	rows := make([]string, 0, len(files))
	for _, f := range files {
		if f.Hash == "" {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s  %s", f.Hash, bagEncodePath(filepath.ToSlash(f.RelPath))))
	}
	sort.Strings(rows)
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n") + "\n"
}

// bagInfo builds a bag-info.txt describing what the package preserves. The
// External-Description states plainly that the payload lives in the package's tar
// and that a fully conformant bag comes from the export action — so a reader is
// never misled into treating the beside-the-package tags as a validatable bag.
func bagInfo(c *Chunk, oxumBytes int64, oxumCount int, conformant bool) string {
	desc := "Obelisk package. The payload lives in " + payloadName(c) +
		" (a plain POSIX tar); manifest-sha256.txt lists the original files it preserves, " +
		"by their tree-relative paths — exactly what the tar yields on extraction, and it " +
		"is also the FIRST member inside the tar. For a fully conformant BagIt bag with a " +
		"data/ payload tree, use Obelisk's bag export."
	if conformant {
		desc = "Conformant BagIt bag exported by Obelisk. data/ holds this package's " +
			"artifacts (the tar payload, par2 parity, package manifest, and RESTORE.txt). " +
			"Extract data/" + payloadName(c) + " with tar to recover the original tree."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Source-Organization: Obelisk\n")
	fmt.Fprintf(&b, "Bagging-Date: %s\n", nowDate())
	fmt.Fprintf(&b, "External-Identifier: %s\n", c.Name)
	fmt.Fprintf(&b, "External-Description: %s\n", desc)
	fmt.Fprintf(&b, "Payload-Oxum: %d.%d\n", oxumBytes, oxumCount)
	fmt.Fprintf(&b, "Bag-Software-Agent: Obelisk %s\n", appVersion)
	if c.Encrypted {
		fmt.Fprintf(&b, "Obelisk-Encryption: OpenPGP symmetric AES-256; key_ref %s (secret not in this bag)\n", c.KeyRef)
	}
	fmt.Fprintf(&b, "Obelisk-Payload-SHA256: %s\n", c.EncHash)
	return b.String()
}

func nowDate() string { return time.Now().UTC().Format("2006-01-02") }

func sha256Hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// writeBagItTags writes the BagIt-format tag files beside a package's payload.
// Best-effort and never fatal to a build — these are a legibility layer, not part
// of the custody chain. manifest-sha256.txt lists the SOURCE files the package
// preserves (as data/<relpath>), matching what the conformant export materializes.
func writeBagItTags(dir string, c *Chunk) error {
	oxB, oxN := bagOxum(c.Files)
	manifest := bagPayloadManifest(c.Files)
	info := bagInfo(c, oxB, oxN, false)
	tags := map[string]string{
		"bagit.txt":            bagItDeclaration,
		"bag-info.txt":         info,
		bagPayloadManifestName: manifest,
	}
	// tagmanifest over the tag files, for bag tooling that expects it.
	names := make([]string, 0, len(tags))
	for n := range tags {
		names = append(names, n)
	}
	sort.Strings(names)
	var tm strings.Builder
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(tags[n]), 0o644); err != nil {
			return fmt.Errorf("writing BagIt tag %s: %w", n, err)
		}
		fmt.Fprintf(&tm, "%s  %s\n", sha256Hex([]byte(tags[n])), bagEncodePath(n))
	}
	if err := os.WriteFile(filepath.Join(dir, "tagmanifest-sha256.txt"), []byte(tm.String()), 0o644); err != nil {
		return fmt.Errorf("writing BagIt tagmanifest: %w", err)
	}
	return nil
}

// ---- conformant bag export ------------------------------------------------

// ExportBag writes a fully conformant BagIt bag for an archive: outputDir/<name>-bag
// with a data/ payload holding each package's artifacts (tar payload, par2 set,
// manifest, RESTORE.txt), a manifest-sha256.txt over data/, bagit.txt,
// bag-info.txt, tagmanifest-sha256.txt, and the COMPARISON.md. Package artifacts
// are copied from each chunk's staged folder; packages not staged locally are
// recorded as skipped (their bytes live only on media). Never writes into a source.
func (a *App) ExportBag(collectionID int, outputDir string, progress func(float64, string)) (map[string]any, error) {
	coll := a.Store.Collection(collectionID)
	if coll == nil {
		return nil, fmt.Errorf("archive %d not found", collectionID)
	}
	return a.exportBagForChunks(coll.Name, a.Store.Chunks(collectionID), outputDir, progress)
}

// ExportPackageBag is the per-package "Export as BagIt" action: a fully conformant
// bag holding just this one package's artifacts, for handoff to institutional
// tooling. Same format as the archive-level export; the storage the package was
// built into is untouched — this is an export, never the storage layout.
func (a *App) ExportPackageBag(chunkID int, outputDir string, progress func(float64, string)) (map[string]any, error) {
	c := a.Store.Chunk(chunkID)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", chunkID)
	}
	if c.StagedDir == "" {
		return nil, fmt.Errorf("package %s is not staged locally — its artifacts live only on media, so there is nothing to bag here", c.Name)
	}
	return a.exportBagForChunks(c.Name, []*Chunk{c}, outputDir, progress)
}

// exportBagForChunks materializes a conformant BagIt bag for a set of packages
// (one archive, or a single package) into outputDir/<name>-bag.
func (a *App) exportBagForChunks(bagBaseName string, chunks []*Chunk, outputDir string, progress func(float64, string)) (map[string]any, error) {
	if strings.TrimSpace(outputDir) == "" {
		return nil, fmt.Errorf("output_dir required")
	}
	if err := a.Store.AssertOutsideSources(outputDir); err != nil {
		return nil, err
	}
	if progress == nil {
		progress = func(float64, string) {}
	}
	bagRoot := filepath.Join(outputDir, fsSafe(bagBaseName)+"-bag")
	dataDir := filepath.Join(bagRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	var manifestLines []string
	var skipped []string
	var payloadBytes int64
	payloadCount := 0

	addFile := func(rel string, data []byte) error {
		dst := filepath.Join(dataDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		manifestLines = append(manifestLines, fmt.Sprintf("%s  data/%s", sha256Hex(data), bagEncodePath(rel)))
		payloadBytes += int64(len(data))
		payloadCount++
		return nil
	}
	copyFile := func(rel, srcPath string) error {
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		return addFile(rel, data)
	}

	for i, c := range chunks {
		progress(0.1+0.7*float64(i)/float64(len(chunks)+1), "bagging "+c.Name)
		if c.StagedDir == "" {
			skipped = append(skipped, c.Name+" (not staged locally — artifacts are only on media)")
			continue
		}
		entries, err := os.ReadDir(c.StagedDir)
		if err != nil {
			skipped = append(skipped, c.Name+" (staged folder unreadable)")
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			// BagIt manages its own tag files at the bag root — don't nest the
			// package's own BagIt tags inside data/.
			if isBagItTagFile(e.Name()) {
				continue
			}
			rel := c.Name + "/" + e.Name()
			if err := copyFile(rel, filepath.Join(c.StagedDir, e.Name())); err != nil {
				return nil, fmt.Errorf("copying %s: %w", rel, err)
			}
		}
	}

	// Payload manifest (over data/), sorted.
	sort.Strings(manifestLines)
	manifest := strings.Join(manifestLines, "\n")
	if manifest != "" {
		manifest += "\n"
	}

	progress(0.85, "bag tags")
	info := exportBagInfo(bagBaseName, payloadBytes, payloadCount, len(chunks), len(skipped))
	tagFiles := map[string]string{
		"bagit.txt":            bagItDeclaration,
		"bag-info.txt":         info,
		bagPayloadManifestName: manifest,
		"COMPARISON.md":        string(comparisonMD),
	}
	names := make([]string, 0, len(tagFiles))
	for n := range tagFiles {
		names = append(names, n)
	}
	sort.Strings(names)
	var tm strings.Builder
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(bagRoot, n), []byte(tagFiles[n]), 0o644); err != nil {
			return nil, err
		}
		fmt.Fprintf(&tm, "%s  %s\n", sha256Hex([]byte(tagFiles[n])), bagEncodePath(n))
	}
	if err := os.WriteFile(filepath.Join(bagRoot, "tagmanifest-sha256.txt"), []byte(tm.String()), 0o644); err != nil {
		return nil, err
	}

	a.Store.Log("bagit", fmt.Sprintf("%s: exported bag (%d file(s), %d package(s), %d skipped)", bagBaseName, payloadCount, len(chunks), len(skipped)))
	progress(1.0, "done")
	return map[string]any{
		"bag": bagRoot, "payload_files": payloadCount, "payload_bytes": payloadBytes,
		"packages": len(chunks), "skipped": skipped, "conformant": len(skipped) == 0,
	}, nil
}

func exportBagInfo(name string, payloadBytes int64, payloadCount, pkgs, skipped int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source-Organization: Obelisk\n")
	fmt.Fprintf(&b, "Bagging-Date: %s\n", nowDate())
	fmt.Fprintf(&b, "External-Identifier: %s\n", name)
	fmt.Fprintf(&b, "External-Description: Conformant BagIt export of Obelisk archive %q. "+
		"data/ holds each package's artifacts (plain-tar payload, par2 parity, manifest, RESTORE.txt). "+
		"Extract any data/<package>/<name>.tar to recover the original tree — no Obelisk required. "+
		"See COMPARISON.md for why this format over restic/borg/Bacula/dar/Canister.\n", name)
	fmt.Fprintf(&b, "Payload-Oxum: %d.%d\n", payloadBytes, payloadCount)
	fmt.Fprintf(&b, "Bag-Count: %d package(s)\n", pkgs)
	if skipped > 0 {
		fmt.Fprintf(&b, "Obelisk-Skipped-Packages: %d (artifacts only on media, not staged locally)\n", skipped)
	}
	fmt.Fprintf(&b, "Bag-Software-Agent: Obelisk %s\n", appVersion)
	return b.String()
}

// isBagItTagFile reports whether a filename is a BagIt tag file (so the export
// doesn't copy a package's own BagIt tags down into the bag's data/ payload).
func isBagItTagFile(name string) bool {
	switch name {
	case "bagit.txt", "bag-info.txt", "manifest-sha256.txt", "tagmanifest-sha256.txt":
		return true
	}
	return false
}
