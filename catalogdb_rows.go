//go:build !guionly

package main

// catalogdb_rows.go — writing a catalog into the v1 tables and reading it back.
// importCatalogRows and exportCatalogRows are exact inverses: the migration
// (catalogdb_migrate.go) proves it for every catalog it moves, section by section,
// before it changes anything.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
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

// t stores a time as UTC Unix nanoseconds when it fits, and otherwise as its exact
// RFC 3339 text in UTC; the zero time is NULL. A year RFC 3339 cannot write (past
// 9999 or before 0) is refused, as catalog.json refuses it.
func (e *dbEnc) t(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	if ns := t.UnixNano(); ns != catalogDBZeroTime && time.Unix(0, ns).Equal(t) {
		return ns
	}
	if y := t.UTC().Year(); y < 0 || y > 9999 {
		e.fail("the time %s is outside the years 0 to 9999 a catalog can hold", t.UTC().Format(time.RFC3339Nano))
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
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

// dbInt reads an INTEGER column, or a time column that may hold the text form of a
// time (see dbEnc.t).
type dbInt struct {
	Int64 int64
	Valid bool
	tm    *time.Time // a time kept as RFC 3339 text, parsed
}

func (n *dbInt) Scan(src any) error {
	*n = dbInt{}
	switch v := src.(type) {
	case nil:
	case int64:
		n.Int64, n.Valid = v, true
	case string, []byte:
		text, _ := v.(string)
		if b, ok := v.([]byte); ok {
			text = string(b)
		}
		t, err := time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return fmt.Errorf("the time %q in the catalog database cannot be read: %w", text, err)
		}
		t = t.UTC()
		n.tm, n.Valid = &t, true
	default:
		return fmt.Errorf("unexpected %T in an integer or time column", src)
	}
	return nil
}

// Readers: the inverse of each encoder.
func tget(n dbInt) time.Time {
	switch {
	case !n.Valid:
		return time.Time{}
	case n.tm != nil:
		return *n.tm
	}
	return time.Unix(0, n.Int64).UTC()
}

func tpget(n dbInt) *time.Time {
	if !n.Valid {
		return nil
	}
	if n.tm == nil && n.Int64 == catalogDBZeroTime {
		return &time.Time{}
	}
	t := tget(n)
	return &t
}

func bpget(n dbInt) *bool {
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

// catalogDBInserts is the INSERT for each table, in column order.
var catalogDBInserts = map[string]string{
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

// rowWriter writes catalog records into the v1 tables inside one transaction,
// with each table's statement prepared once. The import and the store's
// write-through both use it, so a record has one encoding.
type rowWriter struct {
	tx *sql.Tx
	e  dbEnc
	st map[string]*sql.Stmt
	// files, when set, lets a package listing leave out the path, size and hash it
	// shares with its file (the import). nil stores every listing whole.
	files map[int]*File
}

func newRowWriter(tx *sql.Tx, files map[int]*File) *rowWriter {
	return &rowWriter{tx: tx, st: map[string]*sql.Stmt{}, files: files}
}

func (w *rowWriter) close() {
	for _, s := range w.st {
		s.Close()
	}
}

// exec runs table's INSERT with args, reporting a conversion error from building
// args (kept in w.e) or the insert's own error, named by what.
func (w *rowWriter) exec(table, what string, args ...any) error {
	if w.e.err != nil {
		err := w.e.err
		w.e.err = nil
		return fmt.Errorf("%s %s: %w", table, what, err)
	}
	s := w.st[table]
	if s == nil {
		var err error
		if s, err = w.tx.Prepare(catalogDBInserts[table]); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		w.st[table] = s
	}
	if _, err := s.Exec(args...); err != nil {
		return fmt.Errorf("%s %s: %w", table, what, err)
	}
	return nil
}

func (w *rowWriter) collection(i int, x *Collection) error {
	e := &w.e
	return w.exec("collection", fmt.Sprintf("id %d", x.ID), x.ID, i, x.Name, e.t(x.CreatedAt), x.Kind,
		e.j(x.Integrity), b(x.Retired), b(x.RetireHidden), e.tp(x.RetiredAt))
}

func (w *rowWriter) folder(i int, x *Folder) error {
	return w.exec("folder", fmt.Sprintf("id %d", x.ID), x.ID, i, x.CollectionID, x.Path, x.VolumeID)
}

// file writes a file and its retained versions. ord is its place in the catalog.
func (w *rowWriter) file(ord int, f *File) error {
	e := &w.e
	what := fmt.Sprintf("id %d (%s)", f.ID, f.RelPath)
	if err := w.exec("file", what, f.ID, ord, f.CollectionID, f.FolderID, f.RelPath, f.SizeBytes, f.HashAlg,
		e.h(f.Hash), e.h(f.Blake3), e.t(f.ModTime), e.t(f.FirstSeen), e.t(f.ShotAt), f.Role, f.CameraSerial,
		f.EventID); err != nil {
		return err
	}
	for k, v := range f.Versions {
		if err := w.exec("file_version", what, f.ID, k, e.h(v.Hash), v.HashAlg, v.SizeBytes, e.t(v.ModTime),
			e.t(v.FirstSeen), e.t(v.SupersededAt)); err != nil {
			return err
		}
	}
	return nil
}

// chunkColumns are a package's header values in the chunk table's column order,
// after id and ord.
func (w *rowWriter) chunkColumns(ch *Chunk) []any {
	e := &w.e
	return []any{ch.CollectionID, ch.Name, ch.Status, ch.MediaKind, ch.TargetBytes,
		ch.DataBytes, ch.EncBytes, ch.FileCount, ch.SrcRoot, ch.HashAlg, ch.TarHash, ch.EncHash, b(ch.Encrypted),
		ch.KeyRef, b(ch.PrivateManifest), ch.Par2, ch.StagedDir, ch.WrittenDest, bp(ch.VerifyOK), ch.Error,
		e.j(ch.Sidecars), ch.MirrorRoot, e.j(ch.BuildTimings), e.j(ch.BuildVerified), e.j(ch.RingStats),
		e.j(ch.BuildWarnings), b(ch.Spanned), b(ch.Adopted), b(ch.Mirror), b(ch.ListingUnknown),
		e.t(ch.CreatedAt), e.tp(ch.WrittenAt), e.tp(ch.VerifiedAt)}
}

// chunkHeader rewrites a package's header row in place (no delete, so its
// listings stay), or inserts it, and replaces its copies, segments and verify
// events. The store writes packages this way; their listings are written by
// their own methods.
func (w *rowWriter) chunkHeader(i int, ch *Chunk) error {
	what := fmt.Sprintf("id %d (%s)", ch.ID, ch.Name)
	cols := w.chunkColumns(ch)
	if w.e.err != nil {
		err := w.e.err
		w.e.err = nil
		return fmt.Errorf("chunk %s: %w", what, err)
	}
	res, err := w.tx.Exec(`UPDATE chunk SET ord = ?, collection_id = ?, name = ?, status = ?, media_kind = ?,
		target_bytes = ?, data_bytes = ?, enc_bytes = ?, file_count = ?, src_root = ?, hash_alg = ?, tar_hash = ?,
		enc_hash = ?, encrypted = ?, key_ref = ?, private_manifest = ?, par2 = ?, staged_dir = ?, written_dest = ?,
		verify_ok = ?, error = ?, sidecars = ?, mirror_root = ?, build_timings = ?, build_verified = ?, ring_stats = ?,
		build_warnings = ?, spanned = ?, adopted = ?, mirror = ?, listing_unknown = ?, created_at = ?, written_at = ?,
		verified_at = ? WHERE id = ?`, append(append([]any{i}, cols...), ch.ID)...)
	if err != nil {
		return fmt.Errorf("chunk %s: %w", what, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if err := w.exec("chunk", what, append([]any{ch.ID, i}, cols...)...); err != nil {
			return err
		}
	}
	for _, table := range []string{"copy", "segment", "verify_event"} {
		if _, err := w.tx.Exec(`DELETE FROM `+table+` WHERE chunk_id = ?`, ch.ID); err != nil {
			return fmt.Errorf("%s %s: %w", table, what, err)
		}
	}
	return w.chunkChildren(ch, what)
}

func (w *rowWriter) chunk(i int, ch *Chunk) error {
	what := fmt.Sprintf("id %d (%s)", ch.ID, ch.Name)
	if err := w.exec("chunk", what, append([]any{ch.ID, i}, w.chunkColumns(ch)...)...); err != nil {
		return err
	}
	e := &w.e
	for list, refs := range [][]ChunkFileRef{ch.Files, ch.UnknownFiles} {
		for k, r := range refs {
			relPath, size, sha := any(r.RelPath), any(r.SizeBytes), any(nil)
			f := w.files[r.FileID]
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
			if err := w.exec("chunk_file", what, ch.ID, list, k, r.FileID, relPath, size, sha,
				e.h(r.SampleHash)); err != nil {
				return err
			}
		}
	}
	return w.chunkChildren(ch, what)
}

// chunkChildren writes a package's copies, segments and verify events.
func (w *rowWriter) chunkChildren(ch *Chunk, what string) error {
	e := &w.e
	for k, cp := range ch.Copies {
		if err := w.exec("copy", what, ch.ID, k, cp.VolumeID, cp.Path, e.tp(cp.WrittenAt), e.tp(cp.LastVerifiedAt),
			bp(cp.VerifyOK), e.tp(cp.LastCheckAt), cp.LastCheckLevel, bp(cp.LastCheckOK), b(cp.Superseded)); err != nil {
			return err
		}
	}
	for k, sg := range ch.Segments {
		if err := w.exec("segment", what, ch.ID, k, sg.Index, sg.Bytes, sg.Hash, sg.Status, sg.Dest, sg.VolumeID,
			b(sg.Par2)); err != nil {
			return err
		}
	}
	for k, ev := range ch.VerifyEvents {
		if err := w.exec("verify_event", what, ch.ID, k, e.t(ev.At), b(ev.OK), ev.Path, ev.Note, ev.Level,
			b(ev.Advisory)); err != nil {
			return err
		}
	}
	return nil
}

func (w *rowWriter) volume(i int, v *Volume) error {
	e := &w.e
	what := fmt.Sprintf("id %d (%s)", v.ID, v.Label)
	if err := w.exec("volume", what, v.ID, i, v.Label, v.Barcode, v.Kind, v.Location, v.LocationID, b(v.Offsite),
		v.Notes, e.t(v.CreatedAt), v.Serial, v.Model, v.DeviceSize, v.DeviceNote, e.tp(v.DeviceAt), v.PartitionID,
		b(v.Sealed), e.tp(v.SealedAt), b(v.DriveEncrypted), v.DriveEncNote); err != nil {
		return err
	}
	for k := range v.SmartHistory {
		if err := w.exec("smart_snapshot", what, v.ID, k, e.j(&v.SmartHistory[k])); err != nil {
			return err
		}
	}
	for k := range v.Finalizations {
		if err := w.exec("finalization", what, v.ID, k, e.j(&v.Finalizations[k])); err != nil {
			return err
		}
	}
	return nil
}

func (w *rowWriter) location(i int, l *Location) error {
	return w.exec("location", fmt.Sprintf("id %d", l.ID), l.ID, i, l.Name, b(l.Offsite), l.Notes, w.e.t(l.CreatedAt))
}

func (w *rowWriter) snapshot(i int, s *VolumeSnapshot) error {
	e := &w.e
	what := fmt.Sprintf("of volume %d", s.VolumeID)
	if err := w.exec("volume_snapshot", what, s.VolumeID, i, e.t(s.CapturedAt), s.SessionID, s.Label, s.Serial,
		s.Model, s.DeviceSize, s.LocationID, e.j(s.Smart), s.TotalFiles, s.TotalBytes, s.Unreadable, s.Root, s.Scan,
		e.j(s.UnreadableFolders), e.j(s.UnreadableFiles), e.j(s.SkippedFolders), e.j(s.RoleFiles),
		e.j(s.RoleBytes)); err != nil {
		return err
	}
	for k, f := range s.Files {
		if err := w.exec("snap_file", what, s.VolumeID, k, f.RelPath, f.SizeBytes, e.t(f.ModTime), e.h(f.Hash),
			e.h(f.Blake3), f.Role, b(f.Critical), e.t(f.ShotAt), f.CameraSerial); err != nil {
			return err
		}
	}
	return nil
}

func (w *rowWriter) scan(i int, s *ScanRecord) error {
	return w.exec("scan", fmt.Sprintf("id %d", s.ID), s.ID, i, s.VolumeID, w.e.t(s.At), s.SessionID, s.Root,
		s.Label, s.Serial, s.Scan, s.TotalFiles, s.TotalBytes, s.UnreadableFolders, s.UnreadableFiles,
		s.SkippedFolders, s.FileList, s.FileListSHA256)
}

func (w *rowWriter) event(i int, ev *Event) error {
	e := &w.e
	return w.exec("event", fmt.Sprintf("id %d", ev.ID), ev.ID, i, ev.Name, ev.EventType, ev.Year,
		e.t(ev.CaptureStart), e.t(ev.CaptureEnd), ev.Notes, ev.CollectionID, ev.FolderHint, ev.Source,
		e.t(ev.CreatedAt))
}

// audit writes the i-th audit entry (0-based) as id i+1.
func (w *rowWriter) audit(i int, a Audit) error {
	return w.exec("audit", fmt.Sprintf("entry %d", i+1), i+1, w.e.t(a.At), a.Action, a.Detail)
}

func (w *rowWriter) plan(i int, p *Plan) error {
	e := &w.e
	what := fmt.Sprintf("id %d (%s)", p.ID, p.Name)
	if err := w.exec("plan", what, p.ID, i, p.Name, e.j(p.ArchiveIDs), p.ScopePrefix, p.TemplateID,
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
		if err := w.exec("plan_entry", what, p.ID, e.h(hash), override, parked, mapped, satisfied); err != nil {
			return err
		}
	}
	return nil
}

func (w *rowWriter) drift(i int, r *DriftReport) error {
	e := &w.e
	what := fmt.Sprintf("of archive %d", r.CollectionID)
	if err := w.exec("drift_report", what, r.CollectionID, i, e.t(r.At), e.j(r.Counts), e.j(r.InfoCounts),
		e.j(r.ByExt), e.j(r.Unread)); err != nil {
		return err
	}
	for k, it := range r.Items {
		if err := w.exec("drift_item", what, r.CollectionID, k, it.State, it.Path, it.Ext, it.Hash, it.MovedFrom,
			it.Chunk, e.j(it.Volumes), b(it.NeedsBackup), b(it.Informational), it.FileID, it.PriorHash,
			it.PriorVersion); err != nil {
			return err
		}
	}
	return nil
}

// doc writes one record of a doc table as JSON.
func (w *rowWriter) doc(table string, i int, v any) error {
	s := w.st[table]
	if s == nil {
		var err error
		if s, err = w.tx.Prepare(`INSERT INTO ` + table + ` VALUES(?,?)`); err != nil {
			return fmt.Errorf("%s: %w", table, err)
		}
		w.st[table] = s
	}
	doc := w.e.j(v)
	if w.e.err != nil {
		err := w.e.err
		w.e.err = nil
		return fmt.Errorf("%s record %d: %w", table, i+1, err)
	}
	if _, err := s.Exec(i, doc); err != nil {
		return fmt.Errorf("%s record %d: %w", table, i+1, err)
	}
	return nil
}

// meta stores a singleton (next_id, reference) as JSON, or removes it when nil.
func (w *rowWriter) meta(key string, v any) error {
	doc := w.e.j(v)
	if w.e.err != nil {
		err := w.e.err
		w.e.err = nil
		return fmt.Errorf("meta %s: %w", key, err)
	}
	if doc == nil {
		_, err := w.tx.Exec(`DELETE FROM meta WHERE key = ?`, key)
		return err
	}
	_, err := w.tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, doc)
	return err
}

// importCatalogRows writes every record of c into the v1 tables, inside tx, and
// continues each ID sequence from the catalog's next_id.
func importCatalogRows(tx *sql.Tx, c *catalog) error {
	files := make(map[int]*File, len(c.Files))
	for _, f := range c.Files {
		files[f.ID] = f
	}
	w := newRowWriter(tx, files)
	defer w.close()
	for _, u := range catalogUnits(c, true) {
		if err := u.write(w); err != nil {
			return err
		}
	}
	return seedCatalogSequences(tx, c.NextID)
}

// seedCatalogSequences raises each AUTOINCREMENT counter to the catalog's next_id
// counter, so an ID is never reused. Inserting a row with an explicit ID already
// records it in sqlite_sequence, so a counter is raised in place, or added for a
// table that got no rows.
func seedCatalogSequences(tx *sql.Tx, next map[string]int) error {
	for table, kind := range catalogDBSequences {
		res, err := tx.Exec(`UPDATE sqlite_sequence SET seq = max(seq, ?) WHERE name = ?`, next[kind], table)
		if err != nil {
			return fmt.Errorf("sequence %s: %w", table, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := tx.Exec(`INSERT INTO sqlite_sequence(name, seq) VALUES(?, ?)`, table, next[kind]); err != nil {
				return fmt.Errorf("sequence %s: %w", table, err)
			}
		}
	}
	return nil
}

// catalogUnit is one record of the catalog as the database stores it: a key that
// names its rows (so they can be deleted and rewritten together), a hash of its
// content and place, and the write that puts it there.
type catalogUnit struct {
	key   string
	hash  string
	write func(w *rowWriter) error
	// inPlace units replace their own rows when written, so a flush does not
	// delete them first (a package's header: deleting it would take its listings).
	inPlace bool
}

// unitHash is the fingerprint the write-through compares: the record's place in
// its list and its JSON.
func unitHash(ord int, v any) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", ord)
	if err := json.NewEncoder(h).Encode(v); err != nil {
		return "unencodable: " + err.Error()
	}
	return hex.EncodeToString(h.Sum(nil))
}

// catalogUnits lists every record of c as a unit, in the order the import writes
// them. withFiles includes the file records (the import, and a store that keeps
// files in memory).
func catalogUnits(c *catalog, withFiles bool) []catalogUnit {
	var out []catalogUnit
	add := func(key string, ord int, v any, write func(w *rowWriter) error) {
		out = append(out, catalogUnit{key: key, hash: unitHash(ord, v), write: write})
	}
	for i, x := range c.Collections {
		i, x := i, x
		add(fmt.Sprintf("collection:%d", x.ID), i, x, func(w *rowWriter) error { return w.collection(i, x) })
	}
	for i, x := range c.Folders {
		i, x := i, x
		add(fmt.Sprintf("folder:%d", x.ID), i, x, func(w *rowWriter) error { return w.folder(i, x) })
	}
	if withFiles {
		for i, x := range c.Files {
			i, x := i, x
			add(fmt.Sprintf("file:%d", x.ID), i, x, func(w *rowWriter) error { return w.file(i, x) })
		}
	}
	for i, x := range c.Chunks {
		i, x := i, x
		if withFiles {
			add(fmt.Sprintf("chunk:%d", x.ID), i, x, func(w *rowWriter) error { return w.chunk(i, x) })
			continue
		}
		// The store's packages: the header, copies, segments and verify events; the
		// listings are written by their own methods and are not in memory.
		add(fmt.Sprintf("chunk:%d", x.ID), i, x, func(w *rowWriter) error { return w.chunkHeader(i, x) })
		out[len(out)-1].inPlace = true
	}
	for i, x := range c.Volumes {
		i, x := i, x
		add(fmt.Sprintf("volume:%d", x.ID), i, x, func(w *rowWriter) error { return w.volume(i, x) })
	}
	for i, x := range c.Locations {
		i, x := i, x
		add(fmt.Sprintf("location:%d", x.ID), i, x, func(w *rowWriter) error { return w.location(i, x) })
	}
	for i, x := range c.Snapshots {
		i, x := i, x
		add(fmt.Sprintf("volume_snapshot:%d", x.VolumeID), i, x, func(w *rowWriter) error { return w.snapshot(i, x) })
	}
	for i, x := range c.Scans {
		i, x := i, x
		add(fmt.Sprintf("scan:%d", x.ID), i, x, func(w *rowWriter) error { return w.scan(i, x) })
	}
	for i, x := range c.Events {
		i, x := i, x
		add(fmt.Sprintf("event:%d", x.ID), i, x, func(w *rowWriter) error { return w.event(i, x) })
	}
	for i, x := range c.Audit {
		i, x := i, x
		add(fmt.Sprintf("audit:%d", i+1), i, x, func(w *rowWriter) error { return w.audit(i, x) })
	}
	for i, x := range c.Plans {
		i, x := i, x
		add(fmt.Sprintf("plan:%d", x.ID), i, x, func(w *rowWriter) error { return w.plan(i, x) })
	}
	for i, x := range c.Drift {
		i, x := i, x
		add(fmt.Sprintf("drift_report:%d", x.CollectionID), i, x, func(w *rowWriter) error { return w.drift(i, x) })
	}
	for _, d := range catalogDBDocTables {
		field := reflect.ValueOf(c).Elem().FieldByName(d.field)
		for i := 0; i < field.Len(); i++ {
			i, table, v := i, d.table, field.Index(i).Interface()
			add(fmt.Sprintf("%s:%d", table, i), i, v, func(w *rowWriter) error { return w.doc(table, i, v) })
		}
	}
	for _, m := range []struct {
		key string
		v   any
	}{{"next_id", c.NextID}, {"reference", c.Reference}} {
		m := m
		add("meta:"+m.key, 0, m.v, func(w *rowWriter) error { return w.meta(m.key, m.v) })
	}
	return out
}

// catalogUnitDeletes is how each kind of unit's rows are removed, by the number
// after the colon in its key. A record's own nested rows go with it (foreign keys
// with ON DELETE CASCADE).
var catalogUnitDeletes = map[string]string{
	"collection": `DELETE FROM collection WHERE id = ?`, "folder": `DELETE FROM folder WHERE id = ?`,
	"file": `DELETE FROM file WHERE id = ?`, "chunk": `DELETE FROM chunk WHERE id = ?`,
	"volume": `DELETE FROM volume WHERE id = ?`, "location": `DELETE FROM location WHERE id = ?`,
	"volume_snapshot": `DELETE FROM volume_snapshot WHERE volume_id = ?`, "scan": `DELETE FROM scan WHERE id = ?`,
	"event": `DELETE FROM event WHERE id = ?`, "audit": `DELETE FROM audit WHERE id = ?`,
	"plan": `DELETE FROM plan WHERE id = ?`, "drift_report": `DELETE FROM drift_report WHERE collection_id = ?`,
}

// deleteCatalogUnit removes the rows of the unit named key.
func deleteCatalogUnit(tx *sql.Tx, key string) error {
	kind, rest, _ := strings.Cut(key, ":")
	if kind == "meta" {
		_, err := tx.Exec(`DELETE FROM meta WHERE key = ?`, rest)
		return err
	}
	q, ok := catalogUnitDeletes[kind]
	if !ok {
		q = `DELETE FROM ` + kind + ` WHERE ord = ?` // a doc table
		for _, d := range catalogDBDocTables {
			ok = ok || d.table == kind
		}
		if !ok {
			return fmt.Errorf("no table for catalog record %q", key)
		}
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return fmt.Errorf("catalog record %q: %w", key, err)
	}
	_, err = tx.Exec(q, n)
	return err
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
func exportCatalogRows(tx *sql.Tx) (*catalog, error) { return exportCatalogTables(tx, true) }

// exportCatalogTables is exportCatalogRows, leaving out the file records when
// withFiles is false (a store on the database engine keeps them only in tables).
func exportCatalogTables(tx *sql.Tx, withFiles bool) (*catalog, error) {
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
	type nint = dbInt
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
	if withFiles {
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
	// A listing's NULL path, size or hash is its file's (see chunk_file), read here
	// with a join so the files need not be in memory.
	if withFiles {
		if err := q("chunk_file", `SELECT cf.chunk_id, cf.list, cf.file_id, cf.rel_path, cf.size_bytes, cf.sha256,
		cf.sha256 IS NULL, cf.sample_sha256, f.id IS NOT NULL, f.rel_path, f.size_bytes, f.sha256
		FROM chunk_file cf LEFT JOIN file f ON f.id = cf.file_id ORDER BY cf.chunk_id, cf.list, cf.ord`,
			func(r *sql.Rows) error {
				var id, list int
				var ref ChunkFileRef
				var rel, fileRel nstr
				var size, fileSize nint
				var sha, sample, fileSha []byte
				var shaNull, hasFile bool
				if err := r.Scan(&id, &list, &ref.FileID, &rel, &size, &sha, &shaNull, &sample, &hasFile, &fileRel, &fileSize,
					&fileSha); err != nil {
					return err
				}
				ch, err := chunkOf(id, "package file")
				if err != nil {
					return err
				}
				if !hasFile && (!rel.Valid || !size.Valid || shaNull) {
					return fmt.Errorf("package %d lists file %d, which is not in the catalog, without its own path, size and hash", id, ref.FileID)
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
				if list == 0 {
					ch.Files = append(ch.Files, ref)
				} else {
					ch.UnknownFiles = append(ch.UnknownFiles, ref)
				}
				return nil
			}); err != nil {
			return nil, err
		}
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
