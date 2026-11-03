//go:build !guionly

package main

// catalogdb_drives.go — the lists of drive snapshots, plans and drift reports on
// the database engine: snap_file, plan_entry and drift_item, read and written
// inside the backend's open transaction. Their header rows are kept in memory
// and rewritten in place (catalogDBHeaderColumns), so saving a header never
// touches its list.

import (
	"database/sql"
	"fmt"
)

// remember records that a header was written ahead of the next flush, so a flush
// that finds the record gone from memory deletes it again.
func (b *dbBackend) remember(key string) {
	if b.txKeys == nil {
		b.txKeys = map[string]bool{}
	}
	b.txKeys[key] = true
}

// snapshotPut writes a drive snapshot's header and replaces its files, all or
// nothing.
func (b *dbBackend) snapshotPut(snap *VolumeSnapshot, ord int, files []SnapFile) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	return b.atomic(func(tx *sql.Tx) error {
		if err := w.snapshotHeader(ord, snap); err != nil {
			return err
		}
		b.remember(fmt.Sprintf("volume_snapshot:%d", snap.VolumeID))
		if _, err := tx.Exec(`DELETE FROM snap_file WHERE volume_id = ?`, snap.VolumeID); err != nil {
			return err
		}
		return w.snapFiles(snap.VolumeID, files)
	})
}

// snapFilePage reads up to n files of a drive snapshot after position after (-1
// for the first page), in order, and the position of the last one read.
func (b *dbBackend) snapFilePage(volumeID, after, n int) ([]SnapFile, int, error) {
	st, err := b.stmt(`SELECT ord, rel_path, size_bytes, mtime, sha256, blake3, role, critical, shot_at, camera_serial
		FROM snap_file WHERE volume_id = ? AND ord > ? ORDER BY ord LIMIT ?`)
	if err != nil {
		return nil, after, err
	}
	rows, err := st.Query(volumeID, after, n)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	out := make([]SnapFile, 0, n)
	for rows.Next() {
		var f SnapFile
		var mt, shot dbInt
		var sha, b3 []byte
		if err := rows.Scan(&after, &f.RelPath, &f.SizeBytes, &mt, &sha, &b3, &f.Role, &f.Critical, &shot,
			&f.CameraSerial); err != nil {
			return nil, after, err
		}
		f.ModTime, f.Hash, f.Blake3, f.ShotAt = tget(mt), hget(sha), hget(b3), tget(shot)
		out = append(out, f)
	}
	return out, after, rows.Err()
}

// planEntries reads a plan's four maps.
func (b *dbBackend) planEntries(planID int) (*PlanMaps, error) {
	st, err := b.stmt(`SELECT sha256, override, parked, mapped, satisfied FROM plan_entry WHERE plan_id = ?`)
	if err != nil {
		return nil, err
	}
	rows, err := st.Query(planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := newPlanMaps()
	for rows.Next() {
		var sha []byte
		var override, mapped sql.NullString
		var parked, satisfied sql.NullInt64
		if err := rows.Scan(&sha, &override, &parked, &mapped, &satisfied); err != nil {
			return nil, err
		}
		h := hget(sha)
		if override.Valid {
			m.Overrides[h] = override.String
		}
		if mapped.Valid {
			m.Mapping[h] = mapped.String
		}
		if parked.Valid {
			m.Parked[h] = parked.Int64 != 0
		}
		if satisfied.Valid {
			m.Satisfied[h] = satisfied.Int64 != 0
		}
	}
	return m, rows.Err()
}

// planHasSatisfied reports whether the plan has any satisfied entry.
func (b *dbBackend) planHasSatisfied(planID int) (bool, error) {
	st, err := b.stmt(`SELECT EXISTS(SELECT 1 FROM plan_entry WHERE plan_id = ? AND satisfied IS NOT NULL)`)
	if err != nil {
		return false, err
	}
	var has bool
	err = st.QueryRow(planID).Scan(&has)
	return has, err
}

// planEntryUpsert sets one column of a plan's entry for a hash, adding the entry
// when there is none.
func planEntryUpsert(col string) string {
	return `INSERT INTO plan_entry(plan_id, sha256, ` + col + `) VALUES(?, ?, ?)
		ON CONFLICT(plan_id, sha256) DO UPDATE SET ` + col + ` = excluded.` + col
}

// planPut writes a plan's header and the changes ch makes to its entries, all or
// nothing. Entries left with nothing in any column are removed.
func (b *dbBackend) planPut(p *Plan, ord int, ch PlanChange) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	return b.atomic(func(tx *sql.Tx) error {
		if err := w.planHeader(ord, p); err != nil {
			return err
		}
		b.remember(fmt.Sprintf("plan:%d", p.ID))
		if ch.ClearMapping || ch.Mapping != nil {
			if _, err := tx.Exec(`UPDATE plan_entry SET mapped = NULL WHERE plan_id = ?`, p.ID); err != nil {
				return err
			}
		}
		set := func(col, hash string, v any) error {
			st, err := b.stmt(planEntryUpsert(col))
			if err != nil {
				return err
			}
			hb, ok := hashBytes(hash)
			if !ok {
				return fmt.Errorf("plan %s: %q is not a content hash", p.Name, hash)
			}
			if _, err := st.Exec(p.ID, hb, v); err != nil {
				return fmt.Errorf("plan %s entry %s: %w", p.Name, hash, err)
			}
			return nil
		}
		for h, dest := range ch.Mapping {
			if err := set("mapped", h, dest); err != nil {
				return err
			}
		}
		for h, dest := range ch.Overrides {
			var v any
			if dest != "" {
				v = dest
			}
			if err := set("override", h, v); err != nil {
				return err
			}
		}
		for h, on := range ch.Parked {
			var v any
			if on {
				v = 1
			}
			if err := set("parked", h, v); err != nil {
				return err
			}
		}
		for h, on := range ch.Satisfied {
			var v any
			if on {
				v = 1
			}
			if err := set("satisfied", h, v); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`DELETE FROM plan_entry WHERE plan_id = ? AND override IS NULL AND parked IS NULL
			AND mapped IS NULL AND satisfied IS NULL`, p.ID)
		return err
	})
}

// driftPut writes a drift report's header and replaces its items, all or nothing.
func (b *dbBackend) driftPut(r *DriftReport, ord int, items []DriftItem) error {
	w, err := b.writer()
	if err != nil {
		return err
	}
	return b.atomic(func(tx *sql.Tx) error {
		if err := w.driftHeader(ord, r); err != nil {
			return err
		}
		b.remember(fmt.Sprintf("drift_report:%d", r.CollectionID))
		if _, err := tx.Exec(`DELETE FROM drift_item WHERE collection_id = ?`, r.CollectionID); err != nil {
			return err
		}
		return w.driftItems(r.CollectionID, items)
	})
}

// driftItems reads a drift report's items, in order.
func (b *dbBackend) driftItems(collectionID int) ([]DriftItem, error) {
	st, err := b.stmt(`SELECT state, path, ext, hash, moved_from, chunk, volumes, needs_backup, informational,
		file_id, prior_hash, prior_version FROM drift_item WHERE collection_id = ? ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	rows, err := st.Query(collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DriftItem
	for rows.Next() {
		var it DriftItem
		var vols sql.NullString
		if err := rows.Scan(&it.State, &it.Path, &it.Ext, &it.Hash, &it.MovedFrom, &it.Chunk, &vols, &it.NeedsBackup,
			&it.Informational, &it.FileID, &it.PriorHash, &it.PriorVersion); err != nil {
			return nil, err
		}
		if err := jget(vols, &it.Volumes); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
