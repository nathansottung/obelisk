//go:build !guionly

package main

// catalogdb.go — the SQLite catalog engine (an embedded, single-file database).
// It is being built in slices; this file is the open layer. Nothing in the app
// uses it yet: catalog.json stays the live catalog until the cutover slice.
//
// The driver is modernc.org/sqlite: pure Go, no CGO. It is linked into the full
// build only, never the launcher-only (-tags guionly) build.
//
// Durability, in the same spirit as the JSON store (store.go):
//
//   - WAL journal with synchronous=FULL: a commit is on disk before Write returns,
//     so a job never reports work the catalog does not hold (OB-002).
//   - One writer connection. Every write is one immediate transaction on it, so
//     writes are serialized and a failed one rolls back whole. Reads use a separate
//     pool that cannot write and sees the last commit while a write is open.
//   - The open is classified the way openStore classifies catalog.json (OB-001):
//     only a file that does not exist may become a new catalog. An empty file, a
//     file that is not a catalog database, or one that cannot be read is refused
//     and left exactly as it was.
//   - A database written by a newer schema opens read-only, as a newer
//     catalog.json does.
//   - A data folder on a network filesystem is refused: SQLite's WAL needs shared
//     memory and file locks that network shares do not give reliably.
//   - On Windows a virus scanner, the search indexer or a backup agent can hold a
//     file for a moment. SQLite's own Windows layer retries an open, read or write
//     that fails with a sharing or access error; this layer retries its own reads
//     of the file the same way as readFileRetrying.

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// catalogDBDriver is the database/sql driver name modernc.org/sqlite registers.
const catalogDBDriver = "sqlite"

// catalogDBFormat is the meta "format" value that marks an Obelisk catalog.
const catalogDBFormat = "obelisk-catalog"

// catalogDBSchemaVersion is the database schema this build understands. The
// contract is the one catalog.json keeps (see currentSchemaVersion): equal opens
// normally, newer opens read-only, and an older one will be migrated in place once
// a later schema exists.
const catalogDBSchemaVersion = 1

// sqliteHeader is the first 16 bytes of every SQLite database file.
const sqliteHeader = "SQLite format 3\x00"

// catalogDBWriterPragmas are set on every writer connection: a 64 MB page cache
// (a file write updates several indexes), full fsync on commit, foreign keys, and
// recursive triggers, without which SQLite skips the delete triggers of a row that
// INSERT OR REPLACE removes (see file_delete_keeps_listings).
var catalogDBWriterPragmas = []string{"cache_size(-65536)", "foreign_keys(1)", "recursive_triggers(1)", "synchronous(FULL)"}

// catalogDBReaders is the size of the read pool.
const catalogDBReaders = 4

// catalogDBBeforeCreate is a test seam: nil in production. It runs after the open
// has found that path does not exist and before a new catalog is made there.
var catalogDBBeforeCreate func(path string)

// catalogDB is an open catalog database.
type catalogDB struct {
	path string
	// w is the one writer connection (MaxOpenConns 1). Every Write runs on it.
	w *sql.DB
	// r is the read pool. Its connections are opened in SQLite's read-only mode,
	// so it cannot write, and closing one never folds or removes the -wal.
	r *sql.DB
	// readOnly latches when the database was written by a newer schema. Every
	// Write is refused with readOnlyReason, and the writer connection is opened in
	// SQLite's read-only mode.
	readOnly       bool
	readOnlyReason string
}

// catalogDBDSN builds a driver DSN for path. The driver splits the DSN at its first
// "?", which is why openCatalogDB refuses a path containing one. Pragmas in the DSN
// are applied to every connection the pool opens, including a replacement for one
// it dropped.
func catalogDBDSN(path string, pragmas ...string) string {
	q := []string{"_pragma=busy_timeout(10000)"}
	for _, p := range pragmas {
		q = append(q, "_pragma="+p)
	}
	return path + "?" + strings.Join(q, "&")
}

// catalogDBReadOnlyDSN builds a DSN that opens path with SQLite's own read-only
// mode (a file: URI with mode=ro). Unlike query_only, such a connection never
// folds the -wal into the database or removes it when it closes, so a database
// opened only to look at it is left exactly as it was found.
func catalogDBReadOnlyDSN(path string, pragmas ...string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // a Windows drive path: file:///C:/...
	}
	u := url.URL{Scheme: "file", Path: p}
	q := []string{"mode=ro", "_pragma=busy_timeout(10000)", "_pragma=query_only(1)"}
	for _, pr := range pragmas {
		q = append(q, "_pragma="+pr)
	}
	return u.String() + "?" + strings.Join(q, "&")
}

// openCatalogDB opens the catalog database at path. Only when the file does not
// exist and create is true is a new catalog made. Everything else that is not a
// readable Obelisk catalog database is refused without being changed.
func openCatalogDB(path string, create bool) (*catalogDB, error) {
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("the catalog database path %s contains a ?, which the database driver cannot open; choose a folder without one", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	path = abs
	if err := checkCatalogDBLocal(filepath.Dir(path)); err != nil {
		return nil, err
	}

	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		if !create {
			return nil, fmt.Errorf("the catalog database %s does not exist", path)
		}
		if catalogDBBeforeCreate != nil {
			catalogDBBeforeCreate(path)
		}
		// Made whole under another name, then published only if nothing is at
		// path by then; from here on it is opened like any existing catalog.
		if err := createCatalogDBFile(path); err != nil {
			return nil, err
		}
		fi, err = os.Lstat(path)
	}
	if err != nil {
		return nil, catalogDBUnreadable(path, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a file, so it is not a catalog database. Refusing to open it", path)
	}
	if fi.Size() == 0 {
		return nil, fmt.Errorf("%s is empty (0 bytes); that is a damaged catalog database, not a new one. "+
			"Refusing to open it", path)
	}
	if err := checkSQLiteHeader(path); err != nil {
		return nil, err
	}

	d := &catalogDB{path: path}
	if err := d.classifyExisting(); err != nil {
		return nil, err
	}
	var w *sql.DB
	if d.readOnly {
		w, err = sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path, "foreign_keys(1)"))
	} else {
		w, err = sql.Open(catalogDBDriver, catalogDBDSN(path, catalogDBWriterPragmas...)+"&_txlock=immediate")
	}
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)
	w.SetConnMaxIdleTime(0)
	d.w = w
	if !d.readOnly {
		if err := d.enableWAL(); err != nil {
			w.Close()
			return nil, catalogDBUnreadable(path, err)
		}
	}
	r, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path, "foreign_keys(1)"))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(catalogDBReaders)
	d.r = r
	if err := r.Ping(); err != nil {
		d.Close()
		return nil, catalogDBUnreadable(path, err)
	}
	return d, nil
}

// createCatalogDBFile makes a new, empty catalog database at a temporary name
// beside path (WAL mode, every table), folds it into one file and publishes it as
// path without replacing anything: if another process put a database at path in
// the meantime, this one is discarded and theirs is left alone. Only files this
// call created are ever removed.
func createCatalogDBFile(path string) error {
	tmp := path + ".creating-" + randomHex(6)
	discard := func() {
		for _, side := range []string{"", "-wal", "-shm", "-journal"} {
			os.Remove(tmp + side)
		}
	}
	w, err := sql.Open(catalogDBDriver, catalogDBDSN(tmp, catalogDBWriterPragmas...)+"&_txlock=immediate")
	if err != nil {
		return err
	}
	w.SetMaxOpenConns(1)
	d := &catalogDB{path: tmp, w: w}
	err = d.enableWAL()
	if err == nil {
		err = d.createSchema()
	}
	if err == nil {
		_, err = w.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		if fi, serr := os.Stat(tmp + "-wal"); serr == nil && fi.Size() > 0 {
			err = fmt.Errorf("the new database's write-ahead log was not folded in")
		}
	}
	if err == nil {
		err = publishNoReplace(tmp, path)
	}
	if err != nil {
		discard()
		return fmt.Errorf("creating the catalog database %s: %w", path, err)
	}
	discard() // a staging name left by a hard-link publish
	return nil
}

// catalogDBUnreadable is the refusal for a catalog database that exists but cannot
// be read: it is never replaced by a new one.
func catalogDBUnreadable(path string, err error) error {
	return fmt.Errorf("the catalog database %s cannot be read: %w — refusing to open it, so it is never "+
		"replaced by a new one. Check the file's permissions, and that the drive holding the data folder is connected", path, err)
}

// checkSQLiteHeader reads the first bytes of path, waiting out a brief hold by
// another program the same way readFileRetrying does, and refuses a file that is
// not a SQLite database.
func checkSQLiteHeader(path string) error {
	head := make([]byte, len(sqliteHeader))
	var n int
	err := retryTransientRename(func() error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err = io.ReadFull(f, head)
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			err = nil
		}
		return err
	})
	if err != nil {
		return catalogDBUnreadable(path, err)
	}
	if string(head[:n]) != sqliteHeader {
		return fmt.Errorf("%s is not a catalog database (it does not start with the SQLite header). Refusing to open it", path)
	}
	return nil
}

// classifyExisting reads the meta table through a connection that cannot write,
// and decides whether this build may open the database, and whether read-only.
func (d *catalogDB) classifyExisting() error {
	probe, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(d.path))
	if err != nil {
		return err
	}
	defer probe.Close()
	probe.SetMaxOpenConns(1)
	var tables int
	if err := probe.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='meta'`).Scan(&tables); err != nil {
		return catalogDBUnreadable(d.path, err)
	}
	if tables == 0 {
		return fmt.Errorf("%s is a SQLite database but not an Obelisk catalog (it has no meta table). Refusing to open it", d.path)
	}
	meta := map[string]string{}
	rows, err := probe.Query(`SELECT key, value FROM meta WHERE key IN ('format', 'schema_version')`)
	if err != nil {
		return catalogDBUnreadable(d.path, err)
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return catalogDBUnreadable(d.path, err)
		}
		meta[k] = v
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return catalogDBUnreadable(d.path, err)
	}
	if meta["format"] != catalogDBFormat {
		return fmt.Errorf("%s is a SQLite database but not an Obelisk catalog (format %q). Refusing to open it", d.path, meta["format"])
	}
	v, err := strconv.Atoi(meta["schema_version"])
	if err != nil || v < 1 {
		return fmt.Errorf("%s records schema version %q, which no version of Obelisk writes. Refusing to open it", d.path, meta["schema_version"])
	}
	if v > catalogDBSchemaVersion {
		d.readOnly = true
		d.readOnlyReason = fmt.Sprintf("the catalog database was created by a newer version of Obelisk (catalog database "+
			"schema v%d; this build understands v%d). Upgrade the app to change it — it is open read-only so newer "+
			"records are never lost", v, catalogDBSchemaVersion)
	}
	return nil
}

// enableWAL puts the database in WAL mode (a no-op when it already is) and checks
// that it took: a filesystem without shared memory can leave it in another mode.
func (d *catalogDB) enableWAL() error {
	var mode string
	if err := d.w.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("the database could not use its write-ahead log (journal mode %q)", mode)
	}
	return nil
}

// createSchema makes the tables of a new catalog database in one transaction.
func (d *catalogDB) createSchema() error {
	return d.Write(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE meta(key TEXT PRIMARY KEY, value TEXT NOT NULL) STRICT`); err != nil {
			return err
		}
		if _, err := tx.Exec(catalogDBSchemaV1); err != nil {
			return err
		}
		for _, d := range catalogDBDocTables {
			if _, err := tx.Exec(fmt.Sprintf(catalogDBDocTableDDL, d.table)); err != nil {
				return err
			}
		}
		for _, kv := range [][2]string{
			{"format", catalogDBFormat},
			{"schema_version", strconv.Itoa(catalogDBSchemaVersion)},
			{"created_at", time.Now().UTC().Format(time.RFC3339Nano)},
			{"created_by", appVersion},
		} {
			if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)`, kv[0], kv[1]); err != nil {
				return err
			}
		}
		return nil
	})
}

// ReadOnly reports whether every write is refused, and why.
func (d *catalogDB) ReadOnly() (bool, string) { return d.readOnly, d.readOnlyReason }

// Write runs fn in one immediate transaction on the writer connection and commits
// it, durably, before returning. An error from fn, or a failed commit, rolls the
// whole transaction back and is returned.
func (d *catalogDB) Write(fn func(*sql.Tx) error) error {
	if d.readOnly {
		return errors.New(d.readOnlyReason)
	}
	tx, err := d.w.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Read runs fn in one read transaction from the read pool: a consistent view of
// the last commit, unaffected by a write that is open at the same time.
func (d *catalogDB) Read(fn func(*sql.Tx) error) error {
	tx, err := d.r.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return fn(tx)
}

// Close closes the read pool, folds the -wal into the database and closes the
// writer. The writer is the last connection, so SQLite removes the -wal and -shm
// files. A read-only database is not checkpointed: it is left as it was found.
func (d *catalogDB) Close() error {
	var errs []error
	if d.r != nil {
		errs = append(errs, d.r.Close())
	}
	if d.w != nil {
		if !d.readOnly {
			if _, err := d.w.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
				errs = append(errs, fmt.Errorf("folding the write-ahead log into the catalog database: %w", err))
			}
		}
		errs = append(errs, d.w.Close())
	}
	d.r, d.w = nil, nil
	return errors.Join(errs...)
}

// abandon closes the database without folding its -wal in: nothing is written.
// (When this was the last connection, SQLite may still fold a -wal of committed
// commits into the file as it closes; the data is the same.)
func (d *catalogDB) abandon() {
	if d.r != nil {
		d.r.Close()
	}
	if d.w != nil {
		d.w.Close()
	}
	d.r, d.w = nil, nil
}

// catalogDBNetworkRefusal is the decision behind checkCatalogDBLocal: kind names
// the network filesystem the data folder is on, or is empty for a local one.
func catalogDBNetworkRefusal(path, kind string) error {
	if kind == "" {
		return nil
	}
	return fmt.Errorf("the catalog database %s would be on a network filesystem (%s). It must be on a local drive: "+
		"a network share cannot give it the shared memory and file locks it needs. Choose a data folder on a local drive", path, kind)
}
