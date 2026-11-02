//go:build !guionly

package main

// catalogdb_files.go — file records on the database engine: the file and
// file_version tables, read and written inside the backend's open transaction (or
// on its writer connection when none is open), so a read always sees the store's
// own writes. The rules match fileMatches in store_files.go.

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"strconv"
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
	var mt, fs, shot dbInt
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
		// One JSON array, so any number of IDs fits in a statement.
		add(`id IN (SELECT value FROM json_each(?))`, idsJSON(q.IDs))
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
	// A capture time outside what nanoseconds hold is kept as text (see dbEnc.t);
	// those rows pass the bounds here and fileMatches decides them.
	if !q.ShotFrom.IsZero() {
		add(`shot_at IS NOT NULL AND (typeof(shot_at) = 'text' OR shot_at >= ?)`, nsBound(q.ShotFrom))
	}
	if !q.ShotTo.IsZero() {
		add(`shot_at IS NOT NULL AND (typeof(shot_at) = 'text' OR shot_at <= ?)`, nsBound(q.ShotTo))
	}
	if len(conds) == 0 {
		return "1", nil, true
	}
	return strings.Join(conds, " AND "), args, true
}

// filePage reads the next n rows q's SQL selects, after cur, in q's order, and
// returns the files among them that q matches (fileMatches decides; the SQL only
// narrows), where the page ended, and whether more rows may follow.
func (b *dbBackend) filePage(q fileQuery, cur fileCursor, n int) ([]*File, fileCursor, bool, error) {
	c, err := b.conn()
	if err != nil {
		return nil, cur, false, err
	}
	where, args, ok := fileWhere(q)
	if !ok {
		return nil, cur, false, nil
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
		return nil, cur, false, err
	}
	ids := idSet(q.IDs)
	// Versions load after the page is read, and the SQL decides WithVersions exactly.
	check := q
	check.WithVersions = false
	read := 0
	var out, versioned []*File
	byID := map[int]*File{}
	for rows.Next() {
		f, ord, hasVersions, err := scanFileRow(rows)
		if err != nil {
			rows.Close()
			return nil, cur, false, err
		}
		read++
		cur = fileCursor{set: true, relPath: f.RelPath, ord: ord}
		if !fileMatches(f, &check, ids) {
			continue
		}
		out = append(out, f)
		if hasVersions {
			versioned = append(versioned, f)
			byID[f.ID] = f
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, cur, false, err
	}
	if q.Versions && len(versioned) > 0 {
		if err := b.loadVersions(c, versioned, byID); err != nil {
			return nil, cur, false, err
		}
	}
	return out, cur, read == n, nil
}

// idsJSON is a list of IDs as a JSON array, for json_each.
func idsJSON(ids []int) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(id))
	}
	b.WriteByte(']')
	return b.String()
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
			var mt, fs, sup dbInt
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
		page, next, more, err := b.filePage(q, cur, filePageSize)
		if err != nil {
			return err
		}
		for _, f := range page {
			if err := fn(f); err != nil {
				return err
			}
		}
		if !more {
			return nil
		}
		cur = next
	}
}

func (b *dbBackend) fileGet(id int) (*File, error) {
	page, _, _, err := b.filePage(fileQuery{IDs: []int{id}, Versions: true}, fileCursor{}, 1)
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
// It is all or nothing: a failure leaves the file as it was.
func (b *dbBackend) filePut(f *File, isNew bool) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	return b.atomic(func(tx *sql.Tx) error {
		if isNew {
			return w.file(f.ID, f)
		}
		e := &w.e
		args := []any{f.CollectionID, f.FolderID, f.RelPath, f.SizeBytes, f.HashAlg, e.h(f.Hash), e.h(f.Blake3),
			e.t(f.ModTime), e.t(f.FirstSeen), e.t(f.ShotAt), f.Role, f.CameraSerial, f.EventID, f.ID}
		what := fmt.Sprintf("file id %d (%s)", f.ID, f.RelPath)
		if e.err != nil {
			err := e.err
			e.err = nil
			return fmt.Errorf("%s: %w", what, err)
		}
		update, err := b.stmt(`UPDATE file SET collection_id = ?, folder_id = ?, rel_path = ?, size_bytes = ?,
			hash_alg = ?, sha256 = ?, blake3 = ?, mtime = ?, first_seen = ?, shot_at = ?, role = ?, camera_serial = ?,
			event_id = ? WHERE id = ?`)
		if err != nil {
			return err
		}
		res, err := update.Exec(args...)
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%s is not in the catalog", what)
		}
		if b.failAfterFileUpdate != nil {
			if err := b.failAfterFileUpdate(); err != nil {
				return err
			}
		}
		drop, err := b.stmt(`DELETE FROM file_version WHERE file_id = ?`)
		if err != nil {
			return err
		}
		if _, err := drop.Exec(f.ID); err != nil {
			return err
		}
		for k, v := range f.Versions {
			if err := w.exec("file_version", what, f.ID, k, e.h(v.Hash), v.HashAlg, v.SizeBytes, e.t(v.ModTime),
				e.t(v.FirstSeen), e.t(v.SupersededAt)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *dbBackend) fileDelete(q fileQuery) (int, error) {
	where, args, ok := fileWhere(q)
	if !ok {
		return 0, nil
	}
	n := 0
	err := b.atomic(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM file WHERE `+where, args...)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		n = int(affected)
		return err
	})
	return n, err
}

// fileSetEvent points files at an event (0 = none) and returns how many changed.
func (b *dbBackend) fileSetEvent(ids []int, eventID int) (int, error) {
	n := 0
	err := b.atomic(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE file SET event_id = ? WHERE id IN (SELECT value FROM json_each(?)) AND event_id <> ?`,
			eventID, idsJSON(ids), eventID)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		n = int(affected)
		return err
	})
	return n, err
}

// fileClearEvent takes every file out of an event.
func (b *dbBackend) fileClearEvent(eventID int) error {
	return b.atomic(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE file SET event_id = 0 WHERE event_id = ?`, eventID)
		return err
	})
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

// atomic runs fn inside the open transaction as one savepoint: when fn fails,
// everything it did is undone and the rest of the transaction is kept.
func (b *dbBackend) atomic(fn func(tx *sql.Tx) error) error {
	tx, err := b.begin()
	if err != nil {
		return err
	}
	if b.failFileWrite != nil {
		if err := b.failFileWrite(); err != nil {
			return err
		}
	}
	b.txFiles = true
	return savepoint(tx, "file_write", func() error { return fn(tx) })
}

// savepoint runs fn as a savepoint named name of tx: on failure it is rolled
// back, leaving the rest of the transaction as it was.
func savepoint(tx *sql.Tx, name string, fn func() error) error {
	if _, err := tx.Exec(`SAVEPOINT ` + name); err != nil {
		return err
	}
	if err := fn(); err != nil {
		_, rerr := tx.Exec(`ROLLBACK TO ` + name)
		_, xerr := tx.Exec(`RELEASE ` + name)
		return errors.Join(err, rerr, xerr)
	}
	_, err := tx.Exec(`RELEASE ` + name)
	return err
}
