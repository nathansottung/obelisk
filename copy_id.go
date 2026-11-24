package main

// copy_id.go — the copy ID file. Each write of a package to a medium puts a small
// file beside it holding a format version and a fresh random ID, and the catalog
// records that ID on the copy. A check credits a copy only when the medium holds its
// ID, so two copies of one build, a drive mounted somewhere else and a retired copy
// are told apart by what is on the medium, not by where it is mounted.
//
// The file holds nothing else: no package name, path, key or hash. A restore by hand
// with par2, gpg and tar never needs it.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// copyIDFile is the file beside a package on a medium that identifies that copy.
const copyIDFile = "COPY-ID.txt"

// copyIDVersion is the format of the file: its first line.
const copyIDVersion = "obelisk-copy-id 1"

var copyIDContentRe = regexp.MustCompile(`^obelisk-copy-id 1\r?\n([0-9a-f]{32})\r?\n?$`)

// Why a copy earns no credit until it is identified (Copy.Unidentified).
const (
	idNone       = "no_id"      // written before copy IDs; a full check at its folder settles it
	idMissing    = "missing"    // its folder holds no copy ID file
	idUnreadable = "unreadable" // the file is there but cannot be read as an ID
	idMismatch   = "mismatch"   // its folder holds another copy's ID
	idConflict   = "conflict"   // another medium holds the same ID
	idUnchecked  = "unchecked"  // a conflict was settled; a full check confirms it
)

// copyIDNote is the line in RESTORE.txt about the file.
const copyIDNote = copyIDFile + " beside these files only tells Obelisk which copy this is; it is not needed for a restore and holds no private data.\n"

// CopyLocation is a place a copy's ID was seen.
type CopyLocation struct {
	VolumeID int    `json:"volume_id,omitempty"`
	Path     string `json:"path"`
}

// newCopyID is a fresh random ID for one write.
func newCopyID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func copyIDContent(id string) []byte { return []byte(copyIDVersion + "\n" + id + "\n") }

// readCopyID reads the copy ID in a package folder. state is "ok", idMissing or
// idUnreadable.
func readCopyID(dir string) (id, state string) {
	b, err := os.ReadFile(filepath.Join(dir, copyIDFile))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", idMissing
	case err != nil:
		return "", idUnreadable
	}
	m := copyIDContentRe.FindSubmatch(b)
	if m == nil {
		return "", idUnreadable
	}
	return string(m[1]), "ok"
}

// copyIDAt is the ID state of what a check reads in dir. Only a package's own
// folder carries an ID; a flat payload at a medium's root has none.
func copyIDAt(c *Chunk, dir string) (id, state string) {
	if filepath.Base(dir) != c.Name {
		return "", idMissing
	}
	return readCopyID(dir)
}

// writeCopyIDFile writes id as the ID file of the package folder rel beneath out.
// It is published only once it reads back.
func writeCopyIDFile(out *outFolder, rel, id string) error {
	if err := out.WriteFile(rel+"/"+copyIDFile, copyIDContent(id), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", copyIDFile, err)
	}
	return nil
}

// SettleCopyConflict settles a copy whose ID two media hold. keep is "recorded" (the
// medium at the copy's recorded place is the copy) or "found" (the one found
// elsewhere is). The other medium is set aside: it earns nothing and raises no new
// conflict. Either way a full check must confirm the copy before it earns credit.
func (a *App) SettleCopyConflict(chunkID int, copyID, keep string) error {
	c := a.Store.Chunk(chunkID)
	if c == nil {
		return fmt.Errorf("package %d not found", chunkID)
	}
	if err := a.Store.SettleCopyConflict(c, copyID, keep); err != nil {
		return err
	}
	a.refreshChunkStatus(c)
	return a.Store.UpdateChunkErr(c)
}
