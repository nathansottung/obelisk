//go:build !guionly

package main

// catalogdb_store.go — the store's catalog database backend (the SQLite engine,
// OBELISK_CATALOG_ENGINE=sqlite).
//
// Write-through: the store keeps its records in memory as before, and every save
// writes only the records that changed since the last commit, in one
// transaction. Each record is a unit (see catalogUnits): a key naming its rows, a
// hash of its content and place, and its write. A save compares hashes, deletes
// the rows of records that are gone or changed, writes the changed ones again and
// commits. During a batched job saves are coalesced exactly as for catalog.json
// (at most one every batchInterval, about 3 s), so a batch commits about that
// often, and EndBatch commits the rest.

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func init() { openCatalogBackend = openDBBackend }

// dbBackend is an open catalog database with the hashes of what it last committed.
type dbBackend struct {
	d      *catalogDB
	tx     *sql.Tx              // the open write transaction, if any
	txw    *rowWriter           // the open transaction's row writer (prepared inserts)
	txStmt map[string]*sql.Stmt // statements prepared on the open transaction
	dbStmt map[string]*sql.Stmt // statements prepared on the writer connection
	hashes map[string]string    // unit key → hash, as last committed
	closed bool
	// lastWritten and lastRemoved count the records the last flush wrote and
	// removed, for tests.
	lastWritten, lastRemoved int
	// failCommit is a test seam: nil in production. When set, its error stands in
	// for a failed commit.
	failCommit func() error
	// failFileWrite and failAfterFileUpdate are test seams: nil in production. They
	// stand in for a file write the database refuses, before it starts and after the
	// file's row is updated.
	failFileWrite, failAfterFileUpdate func() error
}

// openDBBackend opens dataDir's catalog.db. When there is none, an existing
// catalog.json is moved into a new one first (migrateCatalogToDB, which refuses
// rather than guess); with neither, a new catalog database is made.
func openDBBackend(dataDir string) (catalogBackend, *catalog, string, error) {
	dbPath := filepath.Join(dataDir, catalogDBName)
	if _, err := os.Lstat(dbPath); errors.Is(err, fs.ErrNotExist) {
		jsonPath := filepath.Join(dataDir, "catalog.json")
		switch _, jerr := os.Lstat(jsonPath); {
		case jerr == nil:
			if _, err := migrateCatalogToDB(dataDir, catalogMigrateOptions{}); err != nil {
				return nil, nil, "", err
			}
		case !errors.Is(jerr, fs.ErrNotExist):
			return nil, nil, "", fmt.Errorf("cannot check for %s: %w — refusing to start so an unreadable catalog is "+
				"never replaced by a new empty one", jsonPath, jerr)
		}
	} else if err != nil {
		return nil, nil, "", catalogDBUnreadable(dbPath, err)
	}
	d, err := openCatalogDB(dbPath, true)
	if err != nil {
		return nil, nil, "", err
	}
	c, err := loadCatalogWithoutFiles(d)
	if err != nil {
		d.Close()
		return nil, nil, "", fmt.Errorf("reading the catalog database %s: %w", dbPath, err)
	}
	_, why := d.ReadOnly()
	return &dbBackend{d: d, hashes: catalogUnitHashes(c)}, c, why, nil
}

// loadCatalogWithoutFiles reads every record but the files, which the store
// reaches in their tables.
func loadCatalogWithoutFiles(d *catalogDB) (*catalog, error) {
	var c *catalog
	err := d.Read(func(tx *sql.Tx) error {
		var err error
		c, err = exportCatalogTables(tx, false)
		return err
	})
	return c, err
}

// catalogUnitHashes is the hash of every unit of c but the files, by key.
func catalogUnitHashes(c *catalog) map[string]string {
	units := catalogUnits(c, false)
	out := make(map[string]string, len(units))
	for _, u := range units {
		out[u.key] = u.hash
	}
	return out
}

var errCatalogClosed = errors.New("the catalog database is closed")

// begin returns the open write transaction, starting one if needed.
func (b *dbBackend) begin() (*sql.Tx, error) {
	if b.tx == nil {
		tx, err := b.d.w.Begin()
		if err != nil {
			return nil, err
		}
		b.tx = tx
	}
	return b.tx, nil
}

// endTx forgets the transaction's prepared statements; the transaction closes them.
func (b *dbBackend) endTx() {
	b.tx, b.txw, b.txStmt = nil, nil, nil
}

// writer is the open transaction's row writer, made once per transaction so each
// table's insert is prepared once.
func (b *dbBackend) writer() (*rowWriter, error) {
	tx, err := b.begin()
	if err != nil {
		return nil, err
	}
	if b.txw == nil {
		b.txw = newRowWriter(tx, nil)
	}
	return b.txw, nil
}

// stmt is query prepared once: on the open transaction when there is one (so it
// sees the store's own writes), otherwise on the writer connection.
func (b *dbBackend) stmt(query string) (*sql.Stmt, error) {
	if b.closed {
		return nil, errCatalogClosed
	}
	if b.tx != nil {
		if st := b.txStmt[query]; st != nil {
			return st, nil
		}
		st, err := b.tx.Prepare(query)
		if err != nil {
			return nil, err
		}
		if b.txStmt == nil {
			b.txStmt = map[string]*sql.Stmt{}
		}
		b.txStmt[query] = st
		return st, nil
	}
	if st := b.dbStmt[query]; st != nil {
		return st, nil
	}
	st, err := b.d.w.Prepare(query)
	if err != nil {
		return nil, err
	}
	if b.dbStmt == nil {
		b.dbStmt = map[string]*sql.Stmt{}
	}
	b.dbStmt[query] = st
	return st, nil
}

func (b *dbBackend) flush(c *catalog) error {
	if b.closed {
		return errCatalogClosed
	}
	units := catalogUnits(c, false)
	next := make(map[string]string, len(units))
	var changed []catalogUnit
	for _, u := range units {
		if _, dup := next[u.key]; dup {
			return fmt.Errorf("the catalog holds two records named %s; neither was saved", u.key)
		}
		next[u.key] = u.hash
		if b.hashes[u.key] != u.hash {
			changed = append(changed, u)
		}
	}
	var removed []string
	for k := range b.hashes {
		if _, ok := next[k]; !ok {
			removed = append(removed, k)
		}
	}
	if len(changed) == 0 && len(removed) == 0 && b.tx == nil {
		return nil
	}
	w, err := b.writer()
	if err != nil {
		return err
	}
	tx := b.tx
	// Each record is its own savepoint: one the database refuses is reported and
	// keeps its old hash, so the next save tries it again, while the others commit.
	var unitErrs []error
	for _, k := range removed {
		if err := savepoint(tx, "unit", func() error { return deleteCatalogUnit(tx, k) }); err != nil {
			unitErrs = append(unitErrs, err)
			next[k] = b.hashes[k]
		}
	}
	for _, u := range changed {
		if err := savepoint(tx, "unit", func() error {
			if err := deleteCatalogUnit(tx, u.key); err != nil {
				return err
			}
			return u.write(w)
		}); err != nil {
			unitErrs = append(unitErrs, err)
			if old, ok := b.hashes[u.key]; ok {
				next[u.key] = old
			} else {
				delete(next, u.key)
			}
		}
	}
	b.lastWritten, b.lastRemoved = len(changed)-len(unitErrs), len(removed)
	b.endTx()
	commit := tx.Commit
	if b.failCommit != nil {
		if ferr := b.failCommit(); ferr != nil {
			commit = func() error { tx.Rollback(); return ferr }
		}
	}
	if err := commit(); err != nil {
		tx.Rollback()
		// Memory keeps every record; the hashes go back to what the database holds,
		// so the next save writes them again. File writes in the transaction cannot
		// be replayed from memory: they are lost, and the store says so from now on.
		if committed, rerr := loadCatalogWithoutFiles(b.d); rerr == nil {
			b.hashes = catalogUnitHashes(committed)
		} else {
			b.hashes = map[string]string{}
		}
		return &catalogChangesLost{err: err}
	}
	b.hashes = next
	return errors.Join(unitErrs...)
}

func (b *dbBackend) close() error {
	if b.closed {
		return nil
	}
	b.closed = true
	if b.tx != nil {
		b.tx.Rollback()
		b.endTx()
	}
	for _, st := range b.dbStmt {
		st.Close()
	}
	b.dbStmt = nil
	return b.d.Close()
}

func (b *dbBackend) abandon() {
	if b.closed {
		return
	}
	b.closed = true
	if b.tx != nil {
		b.tx.Rollback()
		b.endTx()
	}
	b.d.abandon()
}
