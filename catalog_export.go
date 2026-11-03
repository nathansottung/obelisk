//go:build !guionly

package main

// catalog_export.go — the portable catalog: the whole catalog written as a v8
// catalog.json document, on either engine. It is plain JSON any text editor can
// read, and the JSON engine (or the importer, on a fresh data folder) opens it as
// a catalog. On the catalog database it is read from a checked copy of the last
// commit, made after every pending change is committed (exportCatalogJSON).

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// PortableCatalogResult says where the portable catalog was written.
type PortableCatalogResult struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// ExportPortableCatalog writes the catalog into destDir as
// obelisk-catalog-<time>.json.
func (a *App) ExportPortableCatalog(destDir string) (PortableCatalogResult, error) {
	if strings.TrimSpace(destDir) == "" {
		return PortableCatalogResult{}, fmt.Errorf("a destination folder is required")
	}
	out, err := a.openOutFolder(destDir)
	if err != nil {
		return PortableCatalogResult{}, err
	}
	defer out.Close()
	name := "obelisk-catalog-" + time.Now().UTC().Format("20060102-150405") + ".json"
	tmp, f, err := out.Create(name)
	if err != nil {
		return PortableCatalogResult{}, err
	}
	h := sha256.New()
	cw := &countingWriter{w: io.MultiWriter(f, h)}
	if err := a.Store.writePortableCatalog(cw); err != nil {
		out.Discard(tmp)
		return PortableCatalogResult{}, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if err := out.Commit(tmp, name, 0o644, sum); err != nil {
		return PortableCatalogResult{}, err
	}
	return PortableCatalogResult{Path: out.Join(name), SHA256: sum, Bytes: cw.n}, nil
}

// countingWriter counts what passes through it.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// writePortableCatalog writes the catalog as a catalog.json document to w: on
// catalog.json, the file as saved with every pending change; on the catalog
// database, the export of a checked copy of the last commit.
func (s *Store) writePortableCatalog(w io.Writer) error {
	if !s.UsesDatabase() {
		if err := s.savePending(); err != nil {
			return fmt.Errorf("the catalog has changes that could not be saved: %w", err)
		}
		f, err := os.Open(s.path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	}
	tmp := s.path + ".export-" + randomHex(6)
	defer func() {
		for _, side := range []string{"", "-wal", "-shm", "-journal"} {
			os.Remove(tmp + side)
		}
	}()
	if _, err := s.snapshotDB(tmp); err != nil {
		return err
	}
	d, err := openCatalogDB(tmp, false)
	if err != nil {
		return err
	}
	defer d.Close()
	return exportCatalogJSON(d, w)
}
