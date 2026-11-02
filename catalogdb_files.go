//go:build !guionly

package main

// catalogdb_files.go — file records on the database engine: the file and
// file_version tables, read and written inside the backend's open transaction (or
// on its writer connection when none is open), so a read always sees the store's
// own writes. The rules match fileMatches in store_files.go.

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math"
	"strings"
	"time"

	"modernc.org/sqlite"
)

func init() {
	// obelisk_lower is Go's strings.ToLower, so a path search matches exactly as it
	// does on the JSON engine (SQLite's own lower() folds ASCII only).
	sqlite.MustRegisterDeterministicScalarFunction("obelisk_lower", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			switch v := args[0].(type) {
			case string:
				return strings.ToLower(v), nil
			case nil:
				return nil, nil
			default:
				return nil, fmt.Errorf("obelisk_lower takes text, got %T", v)
			}
		})
}

// queryer is the transaction or connection file reads and writes run on.
type queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// conn is the open transaction, or the writer connection when none is open.
func (b *dbBackend) conn() (queryer, error) {
	if b.closed {
		return nil, errCatalogClosed
	}
	if b.tx != nil {
		return b.tx, nil
	}
	return b.d.w, nil
}

// fileColumns ends with whether the file has retained versions, so version rows
// are only read for files that have them.
const fileColumns = `id, collection_id, folder_id, rel_path, size_bytes, hash_alg, sha256, blake3, mtime,
	first_seen, shot_at, role, camera_serial, event_id, ord,
	EXISTS (SELECT 1 FROM file_version v WHERE v.file_id = file.id)`

func scanFileRow(r *sql.Rows) (f *File, ord int64, hasVersions bool, err error) {
	f = &File{}
	var sha, b3 []byte
	var mt, fs, shot sql.NullInt64
	if err := r.Scan(&f.ID, &f.CollectionID, &f.FolderID, &f.RelPath, &f.SizeBytes, &f.HashAlg, &sha, &b3, &mt, &fs,
		&shot, &f.Role, &f.CameraSerial, &f.EventID, &ord, &hasVersions); err != nil {
		return nil, 0, false, err
	}
	f.Hash, f.Blake3, f.ModTime, f.FirstSeen, f.ShotAt = hget(sha), hget(b3), tget(mt), tget(fs), tget(shot)
	return f, ord, hasVersions, nil
}

// nsBound is a time as nanoseconds for a range bound, clamped to what fits.
func nsBound(t time.Time) int64 {
	switch {
	case t.Year() < 1678:
		return math.MinInt64 + 1
	case t.Year() > 2261:
		return math.MaxInt64
	}
	return t.UnixNano()
}

// fileWhere is q as an SQL condition with its arguments. It reports false when q
// can match nothing (a hash prefix that is not hexadecimal).
func fileWhere(q fileQuery) (string, []any, bool) {
	var conds []string
	var args []any
	add := func(c string, a ...any) { conds = append(conds, c); args = append(args, a...) }
	if q.CollectionID != 0 {
		add(`collection_id = ?`, q.CollectionID)
	}
	if q.IDs != nil {
		if len(q.IDs) == 0 {
			return "", nil, false
		}
		ph := strings.Repeat("?,", len(q.IDs))
		ids := make([]any, len(q.IDs))
		for i, id := range q.IDs {
			ids[i] = id
		}
		add(`id IN (`+ph[:len(ph)-1]+`)`, ids...)
	}
	if q.EventID > 0 {
		add(`event_id = ?`, q.EventID)
	}
	if q.InEvent {
		add(`event_id <> 0`)
	}
	if q.NoEvent {
		add(`event_id = 0`)
	}
	if q.WithHash {
		add(`sha256 IS NOT NULL`)
	}
	if q.WithVersions {
		add(`EXISTS (SELECT 1 FROM file_version v WHERE v.file_id = file.id)`)
	}
	if q.Text != "" {
		add(`instr(obelisk_lower(rel_path), ?) > 0`, q.Text)
	}
	if q.HashPrefix != "" {
		for _, c := range q.HashPrefix {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return "", nil, false // stored hashes are lowercase hex: nothing can match
			}
		}
		if len(q.HashPrefix) > 64 {
			return "", nil, false
		}
		add(`sha256 IS NOT NULL AND substr(lower(hex(sha256)), 1, ?) = ?`, len(q.HashPrefix), q.HashPrefix)
	}
	if q.Role != "" {
		add(`role = ? COLLATE NOCASE`, q.Role)
	}
	if !q.ShotFrom.IsZero() {
		add(`shot_at IS NOT NULL AND shot_at >= ?`, nsBound(q.ShotFrom))
	}
	if !q.ShotTo.IsZero() {
		add(`shot_at IS NOT NULL AND shot_at <= ?`, nsBound(q.ShotTo))
	}
	if len(conds) == 0 {
		return "1", nil, true
	}
	return strings.Join(conds, " AND "), args, true
}

// filePage reads up to n files q selects, after cur, in q's order.
func (b *dbBackend) filePage(q fileQuery, cur fileCursor, n int) ([]*File, fileCursor, error) {
	c, err := b.conn()
	if err != nil {
		return nil, cur, err
	}
	where, args, ok := fileWhere(q)
	if !ok {
		return nil, cur, nil
	}
	order := `ord`
	if q.ByPath {
		order = `rel_path, ord`
		if cur.set {
			where += ` AND (rel_path, ord) > (?, ?)`
			args = append(args, cur.relPath, cur.ord)
		}
	} else if cur.set {
		where += ` AND ord > ?`
		args = append(args, cur.ord)
	}
	rows, err := c.Query(`SELECT `+fileColumns+` FROM file WHERE `+where+` ORDER BY `+order+` LIMIT ?`, append(args, n)...)
	if err != nil {
		return nil, cur, err
	}
	var out, versioned []*File
	byID := map[int]*File{}
	for rows.Next() {
		f, ord, hasVersions, err := scanFileRow(rows)
		if err != nil {
			rows.Close()
			return nil, cur, err
		}
		out = append(out, f)
		if hasVersions {
			versioned = append(versioned, f)
			byID[f.ID] = f
		}
		cur = fileCursor{set: true, relPath: f.RelPath, ord: ord}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, cur, err
	}
	if q.Versions && len(versioned) > 0 {
		if err := b.loadVersions(c, versioned, byID); err != nil {
			return nil, cur, err
		}
	}
	return out, cur, nil
}

// loadVersions reads the retained versions of files, oldest first.
func (b *dbBackend) loadVersions(c queryer, files []*File, byID map[int]*File) error {
	const batch = 500
	for i := 0; i < len(files); i += batch {
		end := min(i+batch, len(files))
		ids := make([]any, 0, end-i)
		for _, f := range files[i:end] {
			ids = append(ids, f.ID)
		}
		ph := strings.Repeat("?,", len(ids))
		rows, err := c.Query(`SELECT file_id, sha256, hash_alg, size_bytes, mtime, first_seen, superseded_at
			FROM file_version WHERE file_id IN (`+ph[:len(ph)-1]+`) ORDER BY file_id, seq`, ids...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int
			var v FileVersion
			var sha []byte
			var mt, fs, sup sql.NullInt64
			if err := rows.Scan(&id, &sha, &v.HashAlg, &v.SizeBytes, &mt, &fs, &sup); err != nil {
				rows.Close()
				return err
			}
			v.Hash, v.ModTime, v.FirstSeen, v.SupersededAt = hget(sha), tget(mt), tget(fs), tget(sup)
			if f := byID[id]; f != nil {
				f.Versions = append(f.Versions, v)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// fileScan calls fn for every file q selects, a page at a time.
func (b *dbBackend) fileScan(q fileQuery, fn func(*File) error) error {
	var cur fileCursor
	for {
		page, next, err := b.filePage(q, cur, filePageSize)
		if err != nil {
			return err
		}
		for _, f := range page {
			if err := fn(f); err != nil {
				return err
			}
		}
		if len(page) < filePageSize {
			return nil
		}
		cur = next
	}
}

func (b *dbBackend) fileGet(id int) (*File, error) {
	page, _, err := b.filePage(fileQuery{IDs: []int{id}, Versions: true}, fileCursor{}, 1)
	if err != nil || len(page) == 0 {
		return nil, err
	}
	return page[0], nil
}

// fileByKey is the first file (in catalog order) at a path of a folder of an
// archive, with its versions.
func (b *dbBackend) fileByKey(collectionID, folderID int, relPath string) (*File, error) {
	st, err := b.stmt(`SELECT ` + fileColumns + ` FROM file WHERE collection_id = ? AND folder_id = ? AND rel_path = ?
		ORDER BY ord LIMIT 1`)
	if err != nil {
		return nil, err
	}
	rows, err := st.Query(collectionID, folderID, relPath)
	if err != nil {
		return nil, err
	}
	var f *File
	hasVersions := false
	if rows.Next() {
		f, _, hasVersions, err = scanFileRow(rows)
	}
	rows.Close()
	if err != nil || f == nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !hasVersions {
		return f, nil
	}
	c, err := b.conn()
	if err != nil {
		return nil, err
	}
	return f, b.loadVersions(c, []*File{f}, map[int]*File{f.ID: f})
}

// filePut writes f whole: a new row (its catalog place is its ID, so it sorts
// after every earlier file) or the row with its ID, with its versions replaced.
func (b *dbBackend) filePut(f *File, isNew bool) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	if isNew {
		return w.file(f.ID, f)
	}
	update, err := b.stmt(`UPDATE file SET collection_id = ?, folder_id = ?, rel_path = ?, size_bytes = ?,
		hash_alg = ?, sha256 = ?, blake3 = ?, mtime = ?, first_seen = ?, shot_at = ?, role = ?, camera_serial = ?,
		event_id = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	e := &w.e
	res, err := update.Exec(f.CollectionID, f.FolderID, f.RelPath, f.SizeBytes, f.HashAlg, e.h(f.Hash), e.h(f.Blake3),
		e.t(f.ModTime), e.t(f.FirstSeen), e.t(f.ShotAt), f.Role, f.CameraSerial, f.EventID, f.ID)
	if e.err == nil && err == nil {
		if n, _ := res.RowsAffected(); n == 0 {
			err = fmt.Errorf("file %d is not in the catalog", f.ID)
		}
	}
	if e.err != nil {
		return fmt.Errorf("file id %d (%s): %w", f.ID, f.RelPath, e.err)
	}
	if err != nil {
		return fmt.Errorf("file id %d (%s): %w", f.ID, f.RelPath, err)
	}
	drop, err := b.stmt(`DELETE FROM file_version WHERE file_id = ?`)
	if err != nil {
		return err
	}
	if _, err := drop.Exec(f.ID); err != nil {
		return err
	}
	what := fmt.Sprintf("id %d (%s)", f.ID, f.RelPath)
	for k, v := range f.Versions {
		if err := w.exec("file_version", what, f.ID, k, e.h(v.Hash), v.HashAlg, v.SizeBytes, e.t(v.ModTime),
			e.t(v.FirstSeen), e.t(v.SupersededAt)); err != nil {
			return err
		}
	}
	return nil
}

func (b *dbBackend) fileDelete(q fileQuery) (int, error) {
	tx, err := b.begin()
	if err != nil {
		return 0, err
	}
	where, args, ok := fileWhere(q)
	if !ok {
		return 0, nil
	}
	res, err := tx.Exec(`DELETE FROM file WHERE `+where, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// fileSetEvent points files at an event (0 = none) and returns how many changed.
func (b *dbBackend) fileSetEvent(ids []int, eventID int) (int, error) {
	tx, err := b.begin()
	if err != nil {
		return 0, err
	}
	total := 0
	for i := 0; i < len(ids); i += 500 {
		end := min(i+500, len(ids))
		args := []any{eventID}
		for _, id := range ids[i:end] {
			args = append(args, id)
		}
		ph := strings.Repeat("?,", end-i)
		res, err := tx.Exec(`UPDATE file SET event_id = ? WHERE id IN (`+ph[:len(ph)-1]+`) AND event_id <> ?`,
			append(args, eventID)...)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, nil
}

// fileClearEvent takes every file out of an event.
func (b *dbBackend) fileClearEvent(eventID int) error {
	tx, err := b.begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE file SET event_id = 0 WHERE event_id = ?`, eventID)
	return err
}

func (b *dbBackend) fileStats(collectionID int) (int, int64, error) {
	c, err := b.conn()
	if err != nil {
		return 0, 0, err
	}
	var n int
	var bytes int64
	err = c.QueryRow(`SELECT count(*), coalesce(sum(size_bytes), 0) FROM file WHERE ? = 0 OR collection_id = ?`,
		collectionID, collectionID).Scan(&n, &bytes)
	return n, bytes, err
}
