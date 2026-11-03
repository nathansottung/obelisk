//go:build !guionly

package main

// catalogdb_jobs.go — the job board and the audit log in the catalog database.
//
// The job board is the job table, one row per job kept whole as JSON, with its
// next ID in meta. A job's rows are written by commit, in the transaction that
// also commits the catalog's pending changes, so a job's end and the changes it
// made are saved together or not at all. The audit log is append-only: an entry
// is queued in memory until the next commit writes it, and is read back from the
// table.

import (
	"database/sql"
	"errors"
	"fmt"
)

// catalogDBJobTableDDL makes the job table in a catalog database created before it
// existed.
const catalogDBJobTableDDL = `CREATE TABLE IF NOT EXISTS job(id INTEGER PRIMARY KEY, doc TEXT NOT NULL) STRICT`

// ensureJobTable makes the job table if the database does not have it yet. It
// writes only then, so an open never waits on another writer for nothing.
func ensureJobTable(d *catalogDB) error {
	var n int
	if err := d.Read(func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'job'`).Scan(&n)
	}); err != nil || n > 0 {
		return err
	}
	return d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(catalogDBJobTableDDL)
		return err
	})
}

// writeAudit writes the queued audit entries; the table numbers them in order.
func (b *dbBackend) writeAudit(tx *sql.Tx) error {
	st, err := tx.Prepare(`INSERT INTO audit(at, action, detail) VALUES(?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	defer st.Close()
	var e dbEnc
	for _, a := range b.audit {
		at := e.t(a.At)
		if e.err != nil {
			err := e.err
			e.err = nil
			return fmt.Errorf("audit entry %q: %w", a.Action, err)
		}
		if _, err := st.Exec(at, a.Action, a.Detail); err != nil {
			return fmt.Errorf("audit entry %q: %w", a.Action, err)
		}
	}
	return nil
}

// writeJobRows writes job rows and meta values in tx.
func writeJobRows(tx *sql.Tx, rows []storedJob, meta map[string]string) error {
	for _, r := range rows {
		if _, err := tx.Exec(`INSERT INTO job(id, doc) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET doc = excluded.doc`,
			r.ID, r.Doc); err != nil {
			return fmt.Errorf("job %d: %w", r.ID, err)
		}
	}
	for k, v := range meta {
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
			k, v); err != nil {
			return fmt.Errorf("meta %s: %w", k, err)
		}
	}
	return nil
}

// jobsLoad reads the committed job board: its rows in ID order and the board's
// meta values.
func (b *dbBackend) jobsLoad() ([]storedJob, map[string]string, error) {
	var rows []storedJob
	meta := map[string]string{}
	err := b.d.Read(func(tx *sql.Tx) error {
		r, err := tx.Query(`SELECT id, doc FROM job ORDER BY id`)
		if err != nil {
			return fmt.Errorf("job: %w", err)
		}
		defer r.Close()
		for r.Next() {
			var j storedJob
			if err := r.Scan(&j.ID, &j.Doc); err != nil {
				return fmt.Errorf("job: %w", err)
			}
			rows = append(rows, j)
		}
		if err := r.Err(); err != nil {
			return fmt.Errorf("job: %w", err)
		}
		for _, k := range []string{metaJobNext, metaJobMoved} {
			var v string
			switch err := tx.QueryRow(`SELECT value FROM meta WHERE key = ?`, k).Scan(&v); {
			case err == nil:
				meta[k] = v
			case !errors.Is(err, sql.ErrNoRows):
				return fmt.Errorf("meta %s: %w", k, err)
			}
		}
		return nil
	})
	return rows, meta, err
}

// auditAdd queues an audit entry for the next commit.
func (b *dbBackend) auditAdd(a Audit) { b.audit = append(b.audit, a) }

// lastAudit is the newest audit entry, queued or committed (nil when there is none).
func (b *dbBackend) lastAudit() (*Audit, error) {
	if n := len(b.audit); n > 0 {
		a := b.audit[n-1]
		return &a, nil
	}
	st, err := b.stmt(`SELECT at, action, detail FROM audit ORDER BY id DESC LIMIT 1`)
	if err != nil {
		return nil, err
	}
	var a Audit
	var at dbInt
	switch err := st.QueryRow().Scan(&at, &a.Action, &a.Detail); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("audit: %w", err)
	}
	a.At = tget(at)
	return &a, nil
}
