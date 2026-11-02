//go:build !guionly

package main

// catalogdb_rows.go — writing a catalog into the v1 tables and reading it back.
// importCatalogRows and exportCatalogRows are exact inverses: the migration
// (catalogdb_migrate.go) proves it for every catalog it moves, section by section,
// before it changes anything.

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"
)

// catalogDBZeroTime stores a pointer time that points at the zero time, which is
// not the same as no time at all (nil, stored as NULL).
const catalogDBZeroTime = math.MinInt64

// dbEnc converts catalog values to column values and keeps the first conversion
// error, so a row's columns can be listed without an error check per field.
type dbEnc struct{ err error }

func (e *dbEnc) fail(format string, a ...any) {
	if e.err == nil {
		e.err = fmt.Errorf(format, a...)
	}
}

// t stores a time as UTC Unix nanoseconds; the zero time is NULL. A time outside
// what nanoseconds can hold (about 1678 to 2261) is refused rather than changed.
func (e *dbEnc) t(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	ns := t.UnixNano()
	if ns == catalogDBZeroTime || !time.Unix(0, ns).Equal(t) {
		e.fail("the time %s cannot be stored exactly", t.Format(time.RFC3339Nano))
		return nil
	}
	return ns
}

// tp stores a pointer time: NULL for nil, catalogDBZeroTime for a zero time.
func (e *dbEnc) tp(t *time.Time) any {
	if t == nil {
		return nil
	}
	if t.IsZero() {
		return int64(catalogDBZeroTime)
	}
	return e.t(*t)
}

func b(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func bp(v *bool) any {
	if v == nil {
		return nil
	}
	return b(*v)
}

// j stores a value as JSON text, or NULL when it is a nil pointer, map or slice.
func (e *dbEnc) j(v any) any {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return nil
		}
	}
	out, err := json.Marshal(v)
	if err != nil {
		e.fail("encoding %T: %v", v, err)
		return nil
	}
	return string(out)
}

// h stores a SHA-256 or BLAKE3 hex hash as 32 bytes; the empty hash is NULL.
func (e *dbEnc) h(s string) any {
	if s == "" {
		return nil
	}
	raw, ok := hashBytes(s)
	if !ok {
		e.fail("the hash %q is not 64 lowercase hexadecimal characters", s)
		return nil
	}
	return raw
}

// hashBytes decodes a canonical hash: exactly 64 lowercase hex characters.
func hashBytes(s string) ([]byte, bool) {
	if len(s) != 64 {
		return nil, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return nil, false
		}
	}
	raw, err := hex.DecodeString(s)
	return raw, err == nil
}

// Readers: the inverse of each encoder.
func tget(n sql.NullInt64) time.Time {
	if !n.Valid {
		return time.Time{}
	}
	return time.Unix(0, n.Int64).UTC()
}

func tpget(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	if n.Int64 == catalogDBZeroTime {
		return &time.Time{}
	}
	t := time.Unix(0, n.Int64).UTC()
	return &t
}

func bpget(n sql.NullInt64) *bool {
	if !n.Valid {
		return nil
	}
	v := n.Int64 != 0
	return &v
}

func hget(raw []byte) string {
	if raw == nil {
		return ""
	}
	return hex.EncodeToString(raw)
}

func jget(s sql.NullString, dst any) error {
	if !s.Valid {
		return nil
	}
	return json.Unmarshal([]byte(s.String), dst)
}

// catalogDocFields returns the catalog slices kept in doc tables, by table.
func catalogDocFields(c *catalog) map[string]reflect.Value {
	rv := reflect.ValueOf(c).Elem()
	out := map[string]reflect.Value{}
	for _, d := range catalogDBDocTables {
		out[d.table] = rv.FieldByName(d.field)
	}
	return out
}

// importCatalogRows writes every record of c into the v1 tables, inside tx.
func importCatalogRows(tx *sql.Tx, c *catalog) error {
	e := &dbEnc{}
	type stmt struct {
		*sql.Stmt
		table string
	}
	prep := func(table, q string) (*stmt, error) {
		s, err := tx.Prepare(q)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", table, err)
		}
		return &stmt{s, table}, nil
	}
	exec := func(s *stmt, what string, args ...any) error {
		if e.err != nil {
			err := e.err
			e.err = nil
			return fmt.Errorf("%s %s: %w", s.table, what, err)
		}
		if _, err := s.Exec(args...); err != nil {
			return fmt.Errorf("%s %s: %w", s.table, what, err)
		}
		return nil
	}

	ins := map[string]string{
		"collection":      `INSERT INTO collection VALUES(?,?,?,?,?,?,?,?,?)`,
		"folder":          `INSERT INTO folder VALUES(?,?,?,?,?)`,
		"file":            `INSERT INTO file VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"file_version":    `INSERT INTO file_version VALUES(?,?,?,?,?,?,?,?)`,
		"chunk":           `INSERT INTO chunk VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"chunk_file":      `INSERT INTO chunk_file VALUES(?,?,?,?,?,?,?,?)`,
		"copy":            `INSERT INTO copy VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		"segment":         `INSERT INTO segment VALUES(?,?,?,?,?,?,?,?,?)`,
		"verify_event":    `INSERT INTO verify_event VALUES(?,?,?,?,?,?,?,?)`,
		"volume":          `INSERT INTO volume VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"smart_snapshot":  `INSERT INTO smart_snapshot VALUES(?,?,?)`,
		"finalization":    `INSERT INTO finalization VALUES(?,?,?)`,
		"location":        `INSERT INTO location VALUES(?,?,?,?,?,?)`,
		"volume_snapshot": `INSERT INTO volume_snapshot VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"snap_file":       `INSERT INTO snap_file VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		"scan":            `INSERT INTO scan VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"event":           `INSERT INTO event VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		"audit":           `INSERT INTO audit VALUES(?,?,?,?)`,
		"plan":            `INSERT INTO plan VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		"plan_entry":      `INSERT INTO plan_entry VALUES(?,?,?,?,?,?)`,
		"drift_report":    `INSERT INTO drift_report VALUES(?,?,?,?,?,?,?)`,
		"drift_item":      `INSERT INTO drift_item VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
	}
	st := map[string]*stmt{}
	for table, q := range ins {
		s, err := prep(table, q)
		if err != nil {
			return err
		}
		defer s.Close()
		st[table] = s
	}

	for i, x := range c.Collections {
		if err := exec(st["collection"], fmt.Sprintf("id %d", x.ID), x.ID, i, x.Name, e.t(x.CreatedAt), x.Kind,
			e.j(x.Integrity), b(x.Retired), b(x.RetireHidden), e.tp(x.RetiredAt)); err != nil {
			return err
		}
	}
	for i, x := range c.Folders {
		if err := exec(st["folder"], fmt.Sprintf("id %d", x.ID), x.ID, i, x.CollectionID, x.Path, x.VolumeID); err != nil {
			return err
		}
	}
	files := make(map[int]*File, len(c.Files))
	for i, f := range c.Files {
		files[f.ID] = f
		what := fmt.Sprintf("id %d (%s)", f.ID, f.RelPath)
		if err := exec(st["file"], what, f.ID, i, f.CollectionID, f.FolderID, f.RelPath, f.SizeBytes, f.HashAlg,
			e.h(f.Hash), e.h(f.Blake3), e.t(f.ModTime), e.t(f.FirstSeen), e.t(f.ShotAt), f.Role, f.CameraSerial,
			f.EventID); err != nil {
			return err
		}
		for k, v := range f.Versions {
			if err := exec(st["file_version"], what, f.ID, k, e.h(v.Hash), v.HashAlg, v.SizeBytes, e.t(v.ModTime),
				e.t(v.FirstSeen), e.t(v.SupersededAt)); err != nil {
				return err
			}
		}
	}
	for i, ch := range c.Chunks {
		what := fmt.Sprintf("id %d (%s)", ch.ID, ch.Name)
		if err := exec(st["chunk"], what, ch.ID, i, ch.CollectionID, ch.Name, ch.Status, ch.MediaKind, ch.TargetBytes,
			ch.DataBytes, ch.EncBytes, ch.FileCount, ch.SrcRoot, ch.HashAlg, ch.TarHash, ch.EncHash, b(ch.Encrypted),
			ch.KeyRef, b(ch.PrivateManifest), ch.Par2, ch.StagedDir, ch.WrittenDest, bp(ch.VerifyOK), ch.Error,
			e.j(ch.Sidecars), ch.MirrorRoot, e.j(ch.BuildTimings), e.j(ch.BuildVerified), e.j(ch.RingStats),
			e.j(ch.BuildWarnings), b(ch.Spanned), b(ch.Adopted), b(ch.Mirror), b(ch.ListingUnknown),
			e.t(ch.CreatedAt), e.tp(ch.WrittenAt), e.tp(ch.VerifiedAt)); err != nil {
			return err
		}
		for list, refs := range [][]ChunkFileRef{ch.Files, ch.UnknownFiles} {
			for k, r := range refs {
				relPath, size, sha := any(r.RelPath), any(r.SizeBytes), any(nil)
				f := files[r.FileID]
				if f != nil && r.RelPath == f.RelPath {
					relPath = nil
				}
				if f != nil && r.SizeBytes == f.SizeBytes {
					size = nil
				}
				switch {
				case f != nil && r.Hash == f.Hash:
				case r.Hash == "":
					sha = []byte{}
				default:
					sha = e.h(r.Hash)
				}
				if err := exec(st["chunk_file"], what, ch.ID, list, k, r.FileID, relPath, size, sha,
					e.h(r.SampleHash)); err != nil {
					return err
				}
			}
		}
		for k, cp := range ch.Copies {
			if err := exec(st["copy"], what, ch.ID, k, cp.VolumeID, cp.Path, e.tp(cp.WrittenAt), e.tp(cp.LastVerifiedAt),
				bp(cp.VerifyOK), e.tp(cp.LastCheckAt), cp.LastCheckLevel, bp(cp.LastCheckOK), b(cp.Superseded)); err != nil {
				return err
			}
		}
		for k, sg := range ch.Segments {
			if err := exec(st["segment"], what, ch.ID, k, sg.Index, sg.Bytes, sg.Hash, sg.Status, sg.Dest, sg.VolumeID,
				b(sg.Par2)); err != nil {
				return err
			}
		}
		for k, ev := range ch.VerifyEvents {
			if err := exec(st["verify_event"], what, ch.ID, k, e.t(ev.At), b(ev.OK), ev.Path, ev.Note, ev.Level,
				b(ev.Advisory)); err != nil {
				return err
			}
		}
	}
	for i, v := range c.Volumes {
		what := fmt.Sprintf("id %d (%s)", v.ID, v.Label)
		if err := exec(st["volume"], what, v.ID, i, v.Label, v.Barcode, v.Kind, v.Location, v.LocationID, b(v.Offsite),
			v.Notes, e.t(v.CreatedAt), v.Serial, v.Model, v.DeviceSize, v.DeviceNote, e.tp(v.DeviceAt), v.PartitionID,
			b(v.Sealed), e.tp(v.SealedAt), b(v.DriveEncrypted), v.DriveEncNote); err != nil {
			return err
		}
		for k := range v.SmartHistory {
			if err := exec(st["smart_snapshot"], what, v.ID, k, e.j(&v.SmartHistory[k])); err != nil {
				return err
			}
		}
		for k := range v.Finalizations {
			if err := exec(st["finalization"], what, v.ID, k, e.j(&v.Finalizations[k])); err != nil {
				return err
			}
		}
	}
	for i, l := range c.Locations {
		if err := exec(st["location"], fmt.Sprintf("id %d", l.ID), l.ID, i, l.Name, b(l.Offsite), l.Notes,
			e.t(l.CreatedAt)); err != nil {
			return err
		}
	}
	for i, s := range c.Snapshots {
		what := fmt.Sprintf("of volume %d", s.VolumeID)
		if err := exec(st["volume_snapshot"], what, s.VolumeID, i, e.t(s.CapturedAt), s.SessionID, s.Label, s.Serial,
			s.Model, s.DeviceSize, s.LocationID, e.j(s.Smart), s.TotalFiles, s.TotalBytes, s.Unreadable, s.Root, s.Scan,
			e.j(s.UnreadableFolders), e.j(s.UnreadableFiles), e.j(s.SkippedFolders), e.j(s.RoleFiles),
			e.j(s.RoleBytes)); err != nil {
			return err
		}
		for k, f := range s.Files {
			if err := exec(st["snap_file"], what, s.VolumeID, k, f.RelPath, f.SizeBytes, e.t(f.ModTime), e.h(f.Hash),
				e.h(f.Blake3), f.Role, b(f.Critical), e.t(f.ShotAt), f.CameraSerial); err != nil {
				return err
			}
		}
	}
	for i, s := range c.Scans {
		if err := exec(st["scan"], fmt.Sprintf("id %d", s.ID), s.ID, i, s.VolumeID, e.t(s.At), s.SessionID, s.Root,
			s.Label, s.Serial, s.Scan, s.TotalFiles, s.TotalBytes, s.UnreadableFolders, s.UnreadableFiles,
			s.SkippedFolders, s.FileList, s.FileListSHA256); err != nil {
			return err
		}
	}
	for i, ev := range c.Events {
		if err := exec(st["event"], fmt.Sprintf("id %d", ev.ID), ev.ID, i, ev.Name, ev.EventType, ev.Year,
			e.t(ev.CaptureStart), e.t(ev.CaptureEnd), ev.Notes, ev.CollectionID, ev.FolderHint, ev.Source,
			e.t(ev.CreatedAt)); err != nil {
			return err
		}
	}
	for i, a := range c.Audit {
		if err := exec(st["audit"], fmt.Sprintf("entry %d", i+1), i+1, e.t(a.At), a.Action, a.Detail); err != nil {
			return err
		}
	}
	for i, p := range c.Plans {
		what := fmt.Sprintf("id %d (%s)", p.ID, p.Name)
		if err := exec(st["plan"], what, p.ID, i, p.Name, e.j(p.ArchiveIDs), p.ScopePrefix, p.TemplateID,
			p.DestinationRoot, p.Status, p.DestVolumeID, e.t(p.CreatedAt), e.tp(p.CompiledAt), e.tp(p.ClosedAt)); err != nil {
			return err
		}
		for _, hash := range planHashes(p) {
			var override, mapped, parked, satisfied any
			if v, ok := p.Overrides[hash]; ok {
				override = v
			}
			if v, ok := p.Mapping[hash]; ok {
				mapped = v
			}
			if v, ok := p.Parked[hash]; ok {
				parked = b(v)
			}
			if v, ok := p.Satisfied[hash]; ok {
				satisfied = b(v)
			}
			if err := exec(st["plan_entry"], what, p.ID, e.h(hash), override, parked, mapped, satisfied); err != nil {
				return err
			}
		}
	}
	for i, r := range c.Drift {
		what := fmt.Sprintf("of archive %d", r.CollectionID)
		if err := exec(st["drift_report"], what, r.CollectionID, i, e.t(r.At), e.j(r.Counts), e.j(r.InfoCounts),
			e.j(r.ByExt), e.j(r.Unread)); err != nil {
			return err
		}
		for k, it := range r.Items {
			if err := exec(st["drift_item"], what, r.CollectionID, k, it.State, it.Path, it.Ext, it.Hash, it.MovedFrom,
				it.Chunk, e.j(it.Volumes), b(it.NeedsBackup), b(it.Informational), it.FileID, it.PriorHash,
				it.PriorVersion); err != nil {
				return err
			}
		}
	}
	for table, field := range catalogDocFields(c) {
		s, err := prep(table, `INSERT INTO `+table+` VALUES(?,?)`)
		if err != nil {
			return err
		}
		for i := 0; i < field.Len(); i++ {
			if err := exec(s, fmt.Sprintf("record %d", i+1), i, e.j(field.Index(i).Interface())); err != nil {
				s.Close()
				return err
			}
		}
		s.Close()
	}

	// The singletons, and each ID sequence continued from the catalog's counter.
	for _, kv := range []struct {
		key string
		v   any
	}{{"next_id", c.NextID}, {"reference", c.Reference}} {
		v := e.j(kv.v)
		if e.err != nil {
			return e.err
		}
		if v == nil {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)`, kv.key, v); err != nil {
			return err
		}
	}
	// Inserting a row with an explicit ID already records it in sqlite_sequence, so
	// the counter is raised in place, or added for a table that got no rows.
	for table, kind := range catalogDBSequences {
		res, err := tx.Exec(`UPDATE sqlite_sequence SET seq = max(seq, ?) WHERE name = ?`, c.NextID[kind], table)
		if err != nil {
			return fmt.Errorf("sequence %s: %w", table, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := tx.Exec(`INSERT INTO sqlite_sequence(name, seq) VALUES(?, ?)`, table, c.NextID[kind]); err != nil {
				return fmt.Errorf("sequence %s: %w", table, err)
			}
		}
	}
	return nil
}

// planHashes is every hash a plan's four maps mention, each once.
func planHashes(p *Plan) []string {
	seen := map[string]bool{}
	var out []string
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range p.Overrides {
		add(k)
	}
	for k := range p.Mapping {
		add(k)
	}
	for k := range p.Parked {
		add(k)
	}
	for k := range p.Satisfied {
		add(k)
	}
	return out
}

// exportCatalogRows reads every table back into a catalog, inside one read
// transaction, so it is one consistent state.
func exportCatalogRows(tx *sql.Tx) (*catalog, error) {
	c := &catalog{SchemaVersion: currentSchemaVersion, NextID: map[string]int{}}
	q := func(table, query string, scan func(*sql.Rows) error) error {
		rows, err := tx.Query(query)
		if err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows); err != nil {
				return fmt.Errorf("%s: %w", table, err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		return nil
	}
	type nint = sql.NullInt64
	type nstr = sql.NullString

	meta := map[string]string{}
	if err := q("meta", `SELECT key, value FROM meta`, func(r *sql.Rows) error {
		var k, v string
		if err := r.Scan(&k, &v); err != nil {
			return err
		}
		meta[k] = v
		return nil
	}); err != nil {
		return nil, err
	}
	if v, ok := meta["next_id"]; ok {
		if err := json.Unmarshal([]byte(v), &c.NextID); err != nil {
			return nil, fmt.Errorf("meta next_id: %w", err)
		}
	}
	if v, ok := meta["reference"]; ok {
		if err := json.Unmarshal([]byte(v), &c.Reference); err != nil {
			return nil, fmt.Errorf("meta reference: %w", err)
		}
	}

	if err := q("collection", `SELECT id, name, created_at, kind, integrity, retired, retire_hidden, retired_at
		FROM collection ORDER BY ord`, func(r *sql.Rows) error {
		x := &Collection{}
		var created, retiredAt nint
		var integ nstr
		if err := r.Scan(&x.ID, &x.Name, &created, &x.Kind, &integ, &x.Retired, &x.RetireHidden, &retiredAt); err != nil {
			return err
		}
		x.CreatedAt, x.RetiredAt = tget(created), tpget(retiredAt)
		c.Collections = append(c.Collections, x)
		return jget(integ, &x.Integrity)
	}); err != nil {
		return nil, err
	}
	if err := q("folder", `SELECT id, collection_id, path, volume_id FROM folder ORDER BY ord`, func(r *sql.Rows) error {
		x := &Folder{}
		c.Folders = append(c.Folders, x)
		return r.Scan(&x.ID, &x.CollectionID, &x.Path, &x.VolumeID)
	}); err != nil {
		return nil, err
	}
	files := map[int]*File{}
	if err := q("file", `SELECT id, collection_id, folder_id, rel_path, size_bytes, hash_alg, sha256, blake3, mtime,
		first_seen, shot_at, role, camera_serial, event_id FROM file ORDER BY ord`, func(r *sql.Rows) error {
		f := &File{}
		var sha, b3 []byte
		var mt, fs, shot nint
		if err := r.Scan(&f.ID, &f.CollectionID, &f.FolderID, &f.RelPath, &f.SizeBytes, &f.HashAlg, &sha, &b3, &mt, &fs,
			&shot, &f.Role, &f.CameraSerial, &f.EventID); err != nil {
			return err
		}
		f.Hash, f.Blake3, f.ModTime, f.FirstSeen, f.ShotAt = hget(sha), hget(b3), tget(mt), tget(fs), tget(shot)
		files[f.ID] = f
		c.Files = append(c.Files, f)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("file_version", `SELECT file_id, sha256, hash_alg, size_bytes, mtime, first_seen, superseded_at
		FROM file_version ORDER BY file_id, seq`, func(r *sql.Rows) error {
		var id int
		var v FileVersion
		var sha []byte
		var mt, fs, sup nint
		if err := r.Scan(&id, &sha, &v.HashAlg, &v.SizeBytes, &mt, &fs, &sup); err != nil {
			return err
		}
		v.Hash, v.ModTime, v.FirstSeen, v.SupersededAt = hget(sha), tget(mt), tget(fs), tget(sup)
		f := files[id]
		if f == nil {
			return fmt.Errorf("a version of file %d, which is not in the catalog", id)
		}
		f.Versions = append(f.Versions, v)
		return nil
	}); err != nil {
		return nil, err
	}

	chunks := map[int]*Chunk{}
	if err := q("chunk", `SELECT id, collection_id, name, status, media_kind, target_bytes, data_bytes, enc_bytes,
		file_count, src_root, hash_alg, tar_hash, enc_hash, encrypted, key_ref, private_manifest, par2, staged_dir,
		written_dest, verify_ok, error, sidecars, mirror_root, build_timings, build_verified, ring_stats,
		build_warnings, spanned, adopted, mirror, listing_unknown, created_at, written_at, verified_at
		FROM chunk ORDER BY ord`, func(r *sql.Rows) error {
		ch := &Chunk{}
		var verifyOK, created, written, verified nint
		var sidecars, timings, bv, ring, warnings nstr
		if err := r.Scan(&ch.ID, &ch.CollectionID, &ch.Name, &ch.Status, &ch.MediaKind, &ch.TargetBytes, &ch.DataBytes,
			&ch.EncBytes, &ch.FileCount, &ch.SrcRoot, &ch.HashAlg, &ch.TarHash, &ch.EncHash, &ch.Encrypted, &ch.KeyRef,
			&ch.PrivateManifest, &ch.Par2, &ch.StagedDir, &ch.WrittenDest, &verifyOK, &ch.Error, &sidecars,
			&ch.MirrorRoot, &timings, &bv, &ring, &warnings, &ch.Spanned, &ch.Adopted, &ch.Mirror, &ch.ListingUnknown,
			&created, &written, &verified); err != nil {
			return err
		}
		ch.VerifyOK, ch.CreatedAt, ch.WrittenAt, ch.VerifiedAt = bpget(verifyOK), tget(created), tpget(written), tpget(verified)
		for _, x := range []struct {
			s   nstr
			dst any
		}{{sidecars, &ch.Sidecars}, {timings, &ch.BuildTimings}, {bv, &ch.BuildVerified}, {ring, &ch.RingStats},
			{warnings, &ch.BuildWarnings}} {
			if err := jget(x.s, x.dst); err != nil {
				return err
			}
		}
		chunks[ch.ID] = ch
		c.Chunks = append(c.Chunks, ch)
		return nil
	}); err != nil {
		return nil, err
	}
	chunkOf := func(id int, table string) (*Chunk, error) {
		if ch := chunks[id]; ch != nil {
			return ch, nil
		}
		return nil, fmt.Errorf("a %s row of package %d, which is not in the catalog", table, id)
	}
	if err := q("chunk_file", `SELECT chunk_id, list, file_id, rel_path, size_bytes, sha256, sha256 IS NULL,
		sample_sha256 FROM chunk_file ORDER BY chunk_id, list, ord`, func(r *sql.Rows) error {
		var id, list int
		var ref ChunkFileRef
		var rel nstr
		var size nint
		var sha, sample []byte
		var shaNull bool
		if err := r.Scan(&id, &list, &ref.FileID, &rel, &size, &sha, &shaNull, &sample); err != nil {
			return err
		}
		ch, err := chunkOf(id, "package file")
		if err != nil {
			return err
		}
		f := files[ref.FileID]
		if f == nil && (!rel.Valid || !size.Valid || shaNull) {
			return fmt.Errorf("package %d lists file %d, which is not in the catalog, without its own path, size and hash", id, ref.FileID)
		}
		ref.RelPath, ref.SizeBytes = rel.String, size.Int64
		if !rel.Valid {
			ref.RelPath = f.RelPath
		}
		if !size.Valid {
			ref.SizeBytes = f.SizeBytes
		}
		switch {
		case shaNull:
			ref.Hash = f.Hash
		case len(sha) > 0:
			ref.Hash = hget(sha)
		}
		ref.SampleHash = hget(sample)
		if list == 0 {
			ch.Files = append(ch.Files, ref)
		} else {
			ch.UnknownFiles = append(ch.UnknownFiles, ref)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("copy", `SELECT chunk_id, volume_id, path, written_at, last_verified_at, verify_ok, last_check_at,
		last_check_level, last_check_ok, superseded FROM copy ORDER BY chunk_id, ord`, func(r *sql.Rows) error {
		var id int
		var cp Copy
		var written, verified, verifyOK, checkAt, checkOK nint
		if err := r.Scan(&id, &cp.VolumeID, &cp.Path, &written, &verified, &verifyOK, &checkAt, &cp.LastCheckLevel,
			&checkOK, &cp.Superseded); err != nil {
			return err
		}
		cp.WrittenAt, cp.LastVerifiedAt, cp.VerifyOK = tpget(written), tpget(verified), bpget(verifyOK)
		cp.LastCheckAt, cp.LastCheckOK = tpget(checkAt), bpget(checkOK)
		ch, err := chunkOf(id, "copy")
		if err != nil {
			return err
		}
		ch.Copies = append(ch.Copies, cp)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("segment", `SELECT chunk_id, idx, bytes, hash, status, dest, volume_id, par2 FROM segment
		ORDER BY chunk_id, ord`, func(r *sql.Rows) error {
		var id int
		var sg Segment
		if err := r.Scan(&id, &sg.Index, &sg.Bytes, &sg.Hash, &sg.Status, &sg.Dest, &sg.VolumeID, &sg.Par2); err != nil {
			return err
		}
		ch, err := chunkOf(id, "segment")
		if err != nil {
			return err
		}
		ch.Segments = append(ch.Segments, sg)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("verify_event", `SELECT chunk_id, at, ok, path, note, level, advisory FROM verify_event
		ORDER BY chunk_id, seq`, func(r *sql.Rows) error {
		var id int
		var ev VerifyEvent
		var at nint
		if err := r.Scan(&id, &at, &ev.OK, &ev.Path, &ev.Note, &ev.Level, &ev.Advisory); err != nil {
			return err
		}
		ev.At = tget(at)
		ch, err := chunkOf(id, "verify event")
		if err != nil {
			return err
		}
		ch.VerifyEvents = append(ch.VerifyEvents, ev)
		return nil
	}); err != nil {
		return nil, err
	}

	volumes := map[int]*Volume{}
	if err := q("volume", `SELECT id, label, barcode, kind, location, location_id, offsite, notes, created_at, serial,
		model, device_size, device_note, device_at, partition_id, sealed, sealed_at, drive_encrypted, drive_enc_note
		FROM volume ORDER BY ord`, func(r *sql.Rows) error {
		v := &Volume{}
		var created, deviceAt, sealedAt nint
		if err := r.Scan(&v.ID, &v.Label, &v.Barcode, &v.Kind, &v.Location, &v.LocationID, &v.Offsite, &v.Notes, &created,
			&v.Serial, &v.Model, &v.DeviceSize, &v.DeviceNote, &deviceAt, &v.PartitionID, &v.Sealed, &sealedAt,
			&v.DriveEncrypted, &v.DriveEncNote); err != nil {
			return err
		}
		v.CreatedAt, v.DeviceAt, v.SealedAt = tget(created), tpget(deviceAt), tpget(sealedAt)
		volumes[v.ID] = v
		c.Volumes = append(c.Volumes, v)
		return nil
	}); err != nil {
		return nil, err
	}
	volumeOf := func(id int, table string) (*Volume, error) {
		if v := volumes[id]; v != nil {
			return v, nil
		}
		return nil, fmt.Errorf("a %s row of volume %d, which is not in the catalog", table, id)
	}
	if err := q("smart_snapshot", `SELECT volume_id, doc FROM smart_snapshot ORDER BY volume_id, seq`, func(r *sql.Rows) error {
		var id int
		var doc nstr
		if err := r.Scan(&id, &doc); err != nil {
			return err
		}
		v, err := volumeOf(id, "SMART")
		if err != nil {
			return err
		}
		var s SmartSnapshot
		if err := jget(doc, &s); err != nil {
			return err
		}
		v.SmartHistory = append(v.SmartHistory, s)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("finalization", `SELECT volume_id, doc FROM finalization ORDER BY volume_id, seq`, func(r *sql.Rows) error {
		var id int
		var doc nstr
		if err := r.Scan(&id, &doc); err != nil {
			return err
		}
		v, err := volumeOf(id, "seal history")
		if err != nil {
			return err
		}
		var f Finalization
		if err := jget(doc, &f); err != nil {
			return err
		}
		v.Finalizations = append(v.Finalizations, f)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("location", `SELECT id, name, offsite, notes, created_at FROM location ORDER BY ord`, func(r *sql.Rows) error {
		l := &Location{}
		var created nint
		if err := r.Scan(&l.ID, &l.Name, &l.Offsite, &l.Notes, &created); err != nil {
			return err
		}
		l.CreatedAt = tget(created)
		c.Locations = append(c.Locations, l)
		return nil
	}); err != nil {
		return nil, err
	}

	snaps := map[int]*VolumeSnapshot{}
	if err := q("volume_snapshot", `SELECT volume_id, captured_at, session_id, label, serial, model, device_size,
		location_id, smart, total_files, total_bytes, unreadable, root, scan, unreadable_folders, unreadable_files,
		skipped_folders, role_files, role_bytes FROM volume_snapshot ORDER BY ord`, func(r *sql.Rows) error {
		s := &VolumeSnapshot{}
		var captured nint
		var smart, uf, ufi, sk, rf, rb nstr
		if err := r.Scan(&s.VolumeID, &captured, &s.SessionID, &s.Label, &s.Serial, &s.Model, &s.DeviceSize, &s.LocationID,
			&smart, &s.TotalFiles, &s.TotalBytes, &s.Unreadable, &s.Root, &s.Scan, &uf, &ufi, &sk, &rf, &rb); err != nil {
			return err
		}
		s.CapturedAt = tget(captured)
		for _, x := range []struct {
			s   nstr
			dst any
		}{{smart, &s.Smart}, {uf, &s.UnreadableFolders}, {ufi, &s.UnreadableFiles}, {sk, &s.SkippedFolders},
			{rf, &s.RoleFiles}, {rb, &s.RoleBytes}} {
			if err := jget(x.s, x.dst); err != nil {
				return err
			}
		}
		snaps[s.VolumeID] = s
		c.Snapshots = append(c.Snapshots, s)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("snap_file", `SELECT volume_id, rel_path, size_bytes, mtime, sha256, blake3, role, critical, shot_at,
		camera_serial FROM snap_file ORDER BY volume_id, ord`, func(r *sql.Rows) error {
		var id int
		var f SnapFile
		var mt, shot nint
		var sha, b3 []byte
		if err := r.Scan(&id, &f.RelPath, &f.SizeBytes, &mt, &sha, &b3, &f.Role, &f.Critical, &shot, &f.CameraSerial); err != nil {
			return err
		}
		f.ModTime, f.Hash, f.Blake3, f.ShotAt = tget(mt), hget(sha), hget(b3), tget(shot)
		s := snaps[id]
		if s == nil {
			return fmt.Errorf("a file of the snapshot of volume %d, which is not in the catalog", id)
		}
		s.Files = append(s.Files, f)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("scan", `SELECT id, volume_id, at, session_id, root, label, serial, scan, total_files, total_bytes,
		unreadable_folders, unreadable_files, skipped_folders, file_list, file_list_sha256 FROM scan ORDER BY ord`,
		func(r *sql.Rows) error {
			s := &ScanRecord{}
			var at nint
			if err := r.Scan(&s.ID, &s.VolumeID, &at, &s.SessionID, &s.Root, &s.Label, &s.Serial, &s.Scan, &s.TotalFiles,
				&s.TotalBytes, &s.UnreadableFolders, &s.UnreadableFiles, &s.SkippedFolders, &s.FileList,
				&s.FileListSHA256); err != nil {
				return err
			}
			s.At = tget(at)
			c.Scans = append(c.Scans, s)
			return nil
		}); err != nil {
		return nil, err
	}
	if err := q("event", `SELECT id, name, event_type, year, capture_start, capture_end, notes, collection_id,
		folder_hint, source, created_at FROM event ORDER BY ord`, func(r *sql.Rows) error {
		ev := &Event{}
		var start, end, created nint
		if err := r.Scan(&ev.ID, &ev.Name, &ev.EventType, &ev.Year, &start, &end, &ev.Notes, &ev.CollectionID,
			&ev.FolderHint, &ev.Source, &created); err != nil {
			return err
		}
		ev.CaptureStart, ev.CaptureEnd, ev.CreatedAt = tget(start), tget(end), tget(created)
		c.Events = append(c.Events, ev)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("audit", `SELECT at, action, detail FROM audit ORDER BY id`, func(r *sql.Rows) error {
		var a Audit
		var at nint
		if err := r.Scan(&at, &a.Action, &a.Detail); err != nil {
			return err
		}
		a.At = tget(at)
		c.Audit = append(c.Audit, a)
		return nil
	}); err != nil {
		return nil, err
	}
	plans := map[int]*Plan{}
	if err := q("plan", `SELECT id, name, archive_ids, scope_prefix, template_id, destination_root, status,
		dest_volume_id, created_at, compiled_at, closed_at FROM plan ORDER BY ord`, func(r *sql.Rows) error {
		p := &Plan{}
		var ids nstr
		var created, compiled, closed nint
		if err := r.Scan(&p.ID, &p.Name, &ids, &p.ScopePrefix, &p.TemplateID, &p.DestinationRoot, &p.Status,
			&p.DestVolumeID, &created, &compiled, &closed); err != nil {
			return err
		}
		p.CreatedAt, p.CompiledAt, p.ClosedAt = tget(created), tpget(compiled), tpget(closed)
		plans[p.ID] = p
		c.Plans = append(c.Plans, p)
		return jget(ids, &p.ArchiveIDs)
	}); err != nil {
		return nil, err
	}
	if err := q("plan_entry", `SELECT plan_id, sha256, override, parked, mapped, satisfied FROM plan_entry`,
		func(r *sql.Rows) error {
			var id int
			var sha []byte
			var override, mapped nstr
			var parked, satisfied nint
			if err := r.Scan(&id, &sha, &override, &parked, &mapped, &satisfied); err != nil {
				return err
			}
			p := plans[id]
			if p == nil {
				return fmt.Errorf("an entry of plan %d, which is not in the catalog", id)
			}
			h := hget(sha)
			if override.Valid {
				if p.Overrides == nil {
					p.Overrides = map[string]string{}
				}
				p.Overrides[h] = override.String
			}
			if mapped.Valid {
				if p.Mapping == nil {
					p.Mapping = map[string]string{}
				}
				p.Mapping[h] = mapped.String
			}
			if parked.Valid {
				if p.Parked == nil {
					p.Parked = map[string]bool{}
				}
				p.Parked[h] = parked.Int64 != 0
			}
			if satisfied.Valid {
				if p.Satisfied == nil {
					p.Satisfied = map[string]bool{}
				}
				p.Satisfied[h] = satisfied.Int64 != 0
			}
			return nil
		}); err != nil {
		return nil, err
	}
	reports := map[int]*DriftReport{}
	if err := q("drift_report", `SELECT collection_id, at, counts, info_counts, by_ext, unread FROM drift_report
		ORDER BY ord`, func(r *sql.Rows) error {
		d := &DriftReport{}
		var at nint
		var counts, info, byExt, unread nstr
		if err := r.Scan(&d.CollectionID, &at, &counts, &info, &byExt, &unread); err != nil {
			return err
		}
		d.At = tget(at)
		for _, x := range []struct {
			s   nstr
			dst any
		}{{counts, &d.Counts}, {info, &d.InfoCounts}, {byExt, &d.ByExt}, {unread, &d.Unread}} {
			if err := jget(x.s, x.dst); err != nil {
				return err
			}
		}
		reports[d.CollectionID] = d
		c.Drift = append(c.Drift, d)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := q("drift_item", `SELECT collection_id, state, path, ext, hash, moved_from, chunk, volumes, needs_backup,
		informational, file_id, prior_hash, prior_version FROM drift_item ORDER BY collection_id, seq`,
		func(r *sql.Rows) error {
			var id int
			var it DriftItem
			var vols nstr
			if err := r.Scan(&id, &it.State, &it.Path, &it.Ext, &it.Hash, &it.MovedFrom, &it.Chunk, &vols, &it.NeedsBackup,
				&it.Informational, &it.FileID, &it.PriorHash, &it.PriorVersion); err != nil {
				return err
			}
			d := reports[id]
			if d == nil {
				return fmt.Errorf("an item of the drift report of archive %d, which is not in the catalog", id)
			}
			if err := jget(vols, &it.Volumes); err != nil {
				return err
			}
			d.Items = append(d.Items, it)
			return nil
		}); err != nil {
		return nil, err
	}
	for table, field := range catalogDocFields(c) {
		if err := q(table, `SELECT doc FROM `+table+` ORDER BY ord`, func(r *sql.Rows) error {
			var doc nstr
			if err := r.Scan(&doc); err != nil {
				return err
			}
			elem := reflect.New(field.Type().Elem())
			if err := jget(doc, elem.Interface()); err != nil {
				return err
			}
			field.Set(reflect.Append(field, elem.Elem()))
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return c, nil
}
