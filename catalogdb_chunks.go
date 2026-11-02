//go:build !guionly

package main

// catalogdb_chunks.go — package listings on the database engine: the chunk_file
// table, read and written inside the backend's open transaction. A listing names
// its file by ID; its path, size and hash are stored only where there is no such
// file or they differ from the file's (a package can hold an older version), and
// are read through a join otherwise. The triggers on file keep a listing's values
// when its file later changes or is removed.

import (
	"database/sql"
	"fmt"
)

// chunkListingSelect reads a listing with its file's values filled in.
const chunkListingSelect = `SELECT cf.chunk_id, cf.file_id, cf.rel_path, cf.size_bytes, cf.sha256, cf.sha256 IS NULL,
	cf.sample_sha256, f.id IS NOT NULL, f.rel_path, f.size_bytes, f.sha256
	FROM chunk_file cf LEFT JOIN file f ON f.id = cf.file_id`

func scanListing(r *sql.Rows) (int, ChunkFileRef, error) {
	var chunkID int
	var ref ChunkFileRef
	var rel, fileRel sql.NullString
	var size, fileSize sql.NullInt64
	var sha, sample, fileSha []byte
	var shaNull, hasFile bool
	if err := r.Scan(&chunkID, &ref.FileID, &rel, &size, &sha, &shaNull, &sample, &hasFile, &fileRel, &fileSize,
		&fileSha); err != nil {
		return 0, ref, err
	}
	if !hasFile && (!rel.Valid || !size.Valid || shaNull) {
		return 0, ref, fmt.Errorf("package %d lists file %d, which is not in the catalog, without its own path, size and hash",
			chunkID, ref.FileID)
	}
	ref.RelPath, ref.SizeBytes = rel.String, size.Int64
	if !rel.Valid {
		ref.RelPath = fileRel.String
	}
	if !size.Valid {
		ref.SizeBytes = fileSize.Int64
	}
	switch {
	case shaNull:
		ref.Hash = hget(fileSha)
	case len(sha) > 0:
		ref.Hash = hget(sha)
	}
	ref.SampleHash = hget(sample)
	return chunkID, ref, nil
}

// chunkFiles reads one listing of a package, in its order.
func (b *dbBackend) chunkFiles(chunkID, list int) ([]ChunkFileRef, error) {
	st, err := b.stmt(chunkListingSelect + ` WHERE cf.chunk_id = ? AND cf.list = ? ORDER BY cf.ord`)
	if err != nil {
		return nil, err
	}
	rows, err := st.Query(chunkID, list)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChunkFileRef
	for rows.Next() {
		_, ref, err := scanListing(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// chunkListingInsert stores a listing, leaving out (NULL) each of its path, size
// and hash that equals its file's. An empty hash that differs from the file's is a
// zero-length BLOB, so it is not read as "the file's".
const chunkListingInsert = `INSERT INTO chunk_file(chunk_id, list, ord, file_id, rel_path, size_bytes, sha256, sample_sha256)
	SELECT ?1, ?2, ?3, ?4,
		CASE WHEN f.id IS NOT NULL AND f.rel_path = ?5 THEN NULL ELSE ?5 END,
		CASE WHEN f.id IS NOT NULL AND f.size_bytes = ?6 THEN NULL ELSE ?6 END,
		CASE WHEN f.id IS NOT NULL AND f.sha256 IS ?7 THEN NULL WHEN ?7 IS NULL THEN zeroblob(0) ELSE ?7 END,
		?8
	FROM (SELECT 1) LEFT JOIN file f ON f.id = ?4`

// chunkFilesPut replaces one listing of a package. The package's header row is
// written first, so the listing's foreign key holds, and remembered, so a flush
// that finds the package gone from memory deletes it again.
func (b *dbBackend) chunkFilesPut(ch *Chunk, ord, list int, refs []ChunkFileRef) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	return b.atomic(func(tx *sql.Tx) error {
		if err := w.chunkHeader(ord, ch); err != nil {
			return err
		}
		if b.txKeys == nil {
			b.txKeys = map[string]bool{}
		}
		b.txKeys[fmt.Sprintf("chunk:%d", ch.ID)] = true
		if _, err := tx.Exec(`DELETE FROM chunk_file WHERE chunk_id = ? AND list = ?`, ch.ID, list); err != nil {
			return err
		}
		ins, err := b.stmt(chunkListingInsert)
		if err != nil {
			return err
		}
		e := &w.e
		for k, r := range refs {
			args := []any{ch.ID, list, k, r.FileID, r.RelPath, r.SizeBytes, e.h(r.Hash), e.h(r.SampleHash)}
			if e.err != nil {
				err := e.err
				e.err = nil
				return fmt.Errorf("package %s file %s: %w", ch.Name, r.RelPath, err)
			}
			if _, err := ins.Exec(args...); err != nil {
				return fmt.Errorf("package %s file %s: %w", ch.Name, r.RelPath, err)
			}
		}
		return nil
	})
}

// listingScan calls fn for every Files listing of the packages chunkIDs, or of
// any package naming one of fileIDs. With idsOnly, only the file ID is read.
func (b *dbBackend) listingScan(chunkIDs, fileIDs []int, idsOnly bool, fn func(chunkID int, ref ChunkFileRef) error) error {
	c, err := b.conn()
	if err != nil {
		return err
	}
	var where string
	var arg string
	switch {
	case chunkIDs != nil:
		where, arg = `cf.list = 0 AND cf.chunk_id IN (SELECT value FROM json_each(?))`, idsJSON(chunkIDs)
	default:
		where, arg = `cf.list = 0 AND cf.file_id IN (SELECT value FROM json_each(?))`, idsJSON(fileIDs)
	}
	if idsOnly {
		rows, err := c.Query(`SELECT cf.chunk_id, cf.file_id FROM chunk_file cf WHERE `+where+` ORDER BY cf.chunk_id, cf.ord`, arg)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var chunkID int
			var ref ChunkFileRef
			if err := rows.Scan(&chunkID, &ref.FileID); err != nil {
				return err
			}
			if err := fn(chunkID, ref); err != nil {
				return err
			}
		}
		return rows.Err()
	}
	rows, err := c.Query(chunkListingSelect+` WHERE `+where+` ORDER BY cf.chunk_id, cf.ord`, arg)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		chunkID, ref, err := scanListing(rows)
		if err != nil {
			return err
		}
		if err := fn(chunkID, ref); err != nil {
			return err
		}
	}
	return rows.Err()
}
