//go:build !guionly

package main

// catalogdb_copy_values.go — catalog database schema v2: each copy keeps the key,
// hashes, sidecar hashes and tapes it was written with.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// catalogDBSchemaV2 adds them. Every column has a default, so old rows stay valid.
const catalogDBSchemaV2 = `
ALTER TABLE copy ADD COLUMN key_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE copy ADD COLUMN payload_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE copy ADD COLUMN tar_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE copy ADD COLUMN sidecars TEXT;
ALTER TABLE copy ADD COLUMN segments TEXT;
ALTER TABLE copy ADD COLUMN repairable INTEGER NOT NULL DEFAULT 0;
`

// catalogDBMigrateHook is a test seam: an error inside the migration's transaction,
// after its changes and before its commit.
var catalogDBMigrateHook func() error

// migrateIfOlder brings a writable catalog database up to this build's schema.
func (d *catalogDB) migrateIfOlder() error {
	if d.readOnly {
		return nil
	}
	var value string
	if err := d.w.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&value); err != nil {
		return err
	}
	from, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("schema version %q: %w", value, err)
	}
	if from >= 2 {
		return nil
	}
	if err := d.backupBeforeMigrate(from); err != nil {
		return fmt.Errorf("backing up the catalog database before its upgrade (nothing was changed): %w", err)
	}
	// One transaction: an interrupted upgrade leaves the old schema and old rows,
	// and the next open runs it again from the start.
	return d.Write(applyCatalogDBV2)
}

// applyCatalogDBV2 is the v1 to v2 upgrade, inside the caller's transaction.
func applyCatalogDBV2(tx *sql.Tx) error {
	{
		if _, err := tx.Exec(catalogDBSchemaV2); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE copy SET
			key_ref = (SELECT key_ref FROM chunk WHERE chunk.id = copy.chunk_id),
			payload_hash = (SELECT enc_hash FROM chunk WHERE chunk.id = copy.chunk_id),
			tar_hash = (SELECT tar_hash FROM chunk WHERE chunk.id = copy.chunk_id),
			sidecars = (SELECT sidecars FROM chunk WHERE chunk.id = copy.chunk_id)`); err != nil {
			return err
		}
		if err := migrateSpanCopies(tx); err != nil {
			return err
		}
		if catalogDBMigrateHook != nil {
			if err := catalogDBMigrateHook(); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`UPDATE meta SET value = ? WHERE key = 'schema_version'`, strconv.Itoa(catalogDBSchemaVersion))
		return err
	}
}

// dbFileSchema is the schema version recorded in a database file, or 0.
func dbFileSchema(path string) int {
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path))
	if err != nil {
		return 0
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&value); err != nil {
		return 0
	}
	v, _ := strconv.Atoi(value)
	return v
}

// checkOlderDBCopy checks a database copy of an older schema (an app backup made
// before an upgrade, say) by upgrading a private copy of it. The file at path is not
// changed: the normal open upgrades it, with its backup, once it is the catalog.
func checkOlderDBCopy(path, pragma string) (*catalog, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".check-*")
	if err != nil {
		return nil, err
	}
	tmp := f.Name()
	f.Close()
	defer removeDBFile(tmp)
	if err := copyFile(path, tmp); err != nil {
		return nil, err
	}
	if err := upgradeDBFile(tmp); err != nil {
		return nil, fmt.Errorf("upgrading a copy of it: %w", err)
	}
	return checkDBCopy(tmp, pragma)
}

// upgradeDBFile runs the upgrade on a database file no one else has open.
func upgradeDBFile(path string) error {
	db, err := sql.Open(catalogDBDriver, catalogDBDSN(path, catalogDBWriterPragmas...)+"&_txlock=immediate")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if err := applyCatalogDBV2(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// migrateSpanCopies gives each current copy of a spanned package its segments.
func migrateSpanCopies(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT s.chunk_id, s.idx, s.bytes, s.hash, s.status, s.dest, s.volume_id, s.par2
		FROM segment s JOIN chunk c ON c.id = s.chunk_id WHERE c.spanned = 1 ORDER BY s.chunk_id, s.ord`)
	if err != nil {
		return err
	}
	sets := map[int][]Segment{}
	for rows.Next() {
		var id int
		var sg Segment
		if err := rows.Scan(&id, &sg.Index, &sg.Bytes, &sg.Hash, &sg.Status, &sg.Dest, &sg.VolumeID, &sg.Par2); err != nil {
			rows.Close()
			return err
		}
		sets[id] = append(sets[id], sg)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for id, set := range sets {
		b, err := json.Marshal(set)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE copy SET segments = ? WHERE chunk_id = ? AND superseded = 0`, string(b), id); err != nil {
			return err
		}
	}
	return nil
}

// backupBeforeMigrate keeps a checked copy of the database as it was. A copy left by
// an upgrade that was interrupted is the same data, so it is kept and not repeated.
func (d *catalogDB) backupBeforeMigrate(from int) error {
	prefix := fmt.Sprintf("%s.pre-schema-v%d-", d.path, from)
	m, _ := filepath.Glob(prefix + "*")
	for _, p := range m {
		// A stage is a backup a crash may have cut short: it never counts.
		if strings.HasSuffix(p, ".stage") {
			removeDBFile(p)
			continue
		}
		if quickCheckFile(p) == nil && dbFileSchema(p) == from {
			return nil
		}
	}
	final := prefix + time.Now().UTC().Format("20060102-150405.000000000")
	stage := final + ".stage"
	if _, err := d.w.Exec(`VACUUM INTO ?`, stage); err != nil {
		_ = os.Remove(stage)
		return err
	}
	if err := quickCheckFile(stage); err != nil {
		removeDBFile(stage)
		return err
	}
	err := publishNoReplace(stage, final)
	removeDBFile(stage)
	return err
}

// quickCheckFile runs SQLite's quick_check on a database of any schema version.
func quickCheckFile(path string) error {
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path))
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA quick_check(1)`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("the backup failed its check: %s", result)
	}
	return nil
}
