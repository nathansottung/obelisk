//go:build !guionly

package main

// catalogdb_migrate.go — the one-way move of a v8 catalog.json into catalog.db.
//
// Nothing calls it yet; the cutover slice runs it on open. The move changes nothing
// until it has proved the database holds the same catalog:
//
//  1. catalog.json is read and checked: every key must be one this build knows
//     (no unknown, duplicate or differently-cased key, which the JSON decoder would
//     silently drop or merge), and every hash the database stores as bytes must be
//     64 lowercase hex characters.
//  2. It is imported into a staging database beside it, in one transaction.
//  3. The staging database is checked (PRAGMA integrity_check), its row count per
//     table must equal the catalog's, and the catalog read back from it must hash
//     the same as catalog.json's, section by section. Any difference refuses the
//     move: the staging files are removed and catalog.json stays the catalog.
//  4. Only then: the exact catalog.json bytes are kept as
//     catalog.json.pre-sqlite-v<N>-<time>; the staging database becomes catalog.db;
//     and catalog.json is replaced by a small stub that says where the catalog went.
//
// The stub's schema_version is far above any JSON schema, so an older Obelisk that
// opens this folder goes read-only with its "newer version" message instead of
// starting an empty catalog. Each step is safe to stop after: the next run finishes
// the job, or refuses when catalog.json no longer matches what was moved.
//
// The section comparison treats a time as its instant (times are kept in UTC) and an
// empty list or map as no list or map; nothing else is normalized.

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// catalogStubSchemaVersion marks the stub left in catalog.json after the move. It
// is above any catalog.json schema, so every build since schema versioning opens
// the folder read-only rather than as a new catalog.
const catalogStubSchemaVersion = 1000

// catalogStub is the content of catalog.json once the catalog has moved.
type catalogStub struct {
	SchemaVersion int    `json:"schema_version"`
	MovedTo       string `json:"moved_to"`
	Note          string `json:"note"`
}

const (
	catalogDBName      = "catalog.db"
	catalogStagePrefix = "catalog.db.migrating-"
	catalogPreSQLite   = "catalog.json.pre-sqlite-v"
	catalogMoveLock    = "catalog.db.move-lock"
)

// catalogMigrateOptions are test seams; production passes the zero value.
type catalogMigrateOptions struct {
	// afterImport runs on the staging database after the import, before it is
	// checked, so a test can damage it and prove the check refuses.
	afterImport func(db *sql.DB) error
	// beforeStub runs after catalog.db is published, just before catalog.json is
	// replaced by the stub.
	beforeStub func() error
	// stopAt names a step ("imported", "backup", "publish", "stub") at which the
	// move stops as if the process died there: nothing is cleaned up.
	stopAt string
}

// errCatalogMoveStopped is returned when a test stops the move at a step.
var errCatalogMoveStopped = errors.New("stopped for a test")

// catalogMigrateResult says what a move did.
type catalogMigrateResult struct {
	Moved    bool           // imported and published by this run
	Finished bool           // an earlier, interrupted move was completed by this run
	Already  bool           // the catalog had already moved
	Backup   string         // the kept catalog.json bytes
	Counts   map[string]int // rows per table
}

// migrateCatalogToDB moves dataDir's catalog.json into catalog.db (see the file
// comment). It is safe to call again after any interruption.
func migrateCatalogToDB(dataDir string, opt catalogMigrateOptions) (catalogMigrateResult, error) {
	jsonPath := filepath.Join(dataDir, "catalog.json")
	dbPath := filepath.Join(dataDir, catalogDBName)
	release, err := lockCatalogMove(dataDir)
	if err != nil {
		return catalogMigrateResult{}, err
	}
	defer release()
	sweepCatalogStages(dataDir)

	raw, jsonErr := readFileRetrying(jsonPath)
	jsonMissing := errors.Is(jsonErr, fs.ErrNotExist)
	if jsonErr != nil && !jsonMissing {
		return catalogMigrateResult{}, fmt.Errorf("cannot read %s: %w — the catalog was not moved", jsonPath, jsonErr)
	}
	stub := !jsonMissing && isCatalogStub(raw)

	if _, err := os.Lstat(dbPath); errors.Is(err, fs.ErrNotExist) {
		switch {
		case jsonMissing:
			return catalogMigrateResult{}, fmt.Errorf("there is no catalog in %s to move", dataDir)
		case stub:
			return catalogMigrateResult{}, fmt.Errorf("%s says the catalog moved to %s, but %s is missing. Restore it "+
				"from a backup, or put back the catalog.json.pre-sqlite-* file beside it as catalog.json", jsonPath,
				catalogDBName, dbPath)
		}
		return moveCatalog(dataDir, raw, opt)
	} else if err != nil {
		return catalogMigrateResult{}, catalogDBUnreadable(dbPath, err)
	}
	return finishCatalogMove(dataDir, raw, jsonMissing, stub, opt)
}

// moveCatalog imports, checks and publishes a catalog.json that has not moved yet.
func moveCatalog(dataDir string, raw []byte, opt catalogMigrateOptions) (catalogMigrateResult, error) {
	jsonPath := filepath.Join(dataDir, "catalog.json")
	refuse := func(err error) (catalogMigrateResult, error) {
		return catalogMigrateResult{}, fmt.Errorf("the catalog was not moved to %s: %w. catalog.json is unchanged "+
			"and is still the catalog", catalogDBName, err)
	}
	c, from, err := readCatalogForMove(raw)
	if err != nil {
		return refuse(err)
	}
	want := catalogDBRowCounts(c)

	stage := filepath.Join(dataDir, catalogStagePrefix+randomHex(6))
	removeStage := func() {
		for _, side := range []string{"", "-wal", "-shm", "-journal"} {
			os.Remove(stage + side)
		}
	}
	d, err := openCatalogDB(stage, true)
	if err != nil {
		return refuse(err)
	}
	err = importCatalogDB(d, c, raw, from)
	if err == nil && opt.afterImport != nil {
		err = opt.afterImport(d.w)
	}
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		removeStage()
		return refuse(err)
	}
	if opt.stopAt == "imported" {
		return catalogMigrateResult{}, errCatalogMoveStopped
	}

	// The check reads the staging database through a fresh open, as the app will.
	canonCatalog(c)
	counts, err := checkCatalogDB(stage, c, want)
	if err != nil {
		removeStage()
		return refuse(err)
	}

	if opt.stopAt == "backup" {
		return catalogMigrateResult{}, errCatalogMoveStopped
	}
	// The import took a while; another Obelisk may have saved since. The database
	// is only published while catalog.json still holds what was imported.
	if err := catalogJSONUnchanged(jsonPath, raw); err != nil {
		removeStage()
		return catalogMigrateResult{}, fmt.Errorf("catalog.json changed while it was being moved to %s (%v), so the "+
			"move was abandoned: catalog.json keeps the change and is still the catalog. Close any other copy of "+
			"Obelisk using this folder and try again", catalogDBName, err)
	}
	backup, err := keepCatalogJSON(dataDir, raw, from)
	if err != nil {
		removeStage()
		return refuse(fmt.Errorf("keeping a copy of catalog.json: %w", err))
	}
	if opt.stopAt == "publish" {
		return catalogMigrateResult{}, errCatalogMoveStopped
	}
	if err := publishNoReplace(stage, filepath.Join(dataDir, catalogDBName)); err != nil {
		removeStage()
		return refuse(err)
	}
	if opt.stopAt == "stub" {
		return catalogMigrateResult{}, errCatalogMoveStopped
	}
	if err := replaceWithStub(dataDir, raw, backup, opt); err != nil {
		return catalogMigrateResult{}, err
	}
	return catalogMigrateResult{Moved: true, Backup: backup, Counts: counts}, nil
}

// finishCatalogMove handles a data folder that already has catalog.db: a finished
// move, or one interrupted after the database was published.
func finishCatalogMove(dataDir string, raw []byte, jsonMissing, stub bool, opt catalogMigrateOptions) (catalogMigrateResult, error) {
	dbPath := filepath.Join(dataDir, catalogDBName)
	if stub {
		return catalogMigrateResult{Already: true}, nil
	}
	d, err := openCatalogDB(dbPath, false)
	if err != nil {
		return catalogMigrateResult{}, err
	}
	var source, fromVersion string
	err = d.Read(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT value FROM meta WHERE key='source_sha256'`).Scan(&source); err != nil {
			return err
		}
		return tx.QueryRow(`SELECT value FROM meta WHERE key='source_schema_version'`).Scan(&fromVersion)
	})
	d.Close()
	if err != nil {
		return catalogMigrateResult{}, fmt.Errorf("%s does not record which catalog.json it was made from: %w", dbPath, err)
	}
	backup := ""
	if !jsonMissing {
		if sha256Hex(raw) != source {
			return catalogMigrateResult{}, errTwoCatalogs(dataDir)
		}
		from, _ := strconv.Atoi(fromVersion)
		if backup, err = keepCatalogJSON(dataDir, raw, from); err != nil {
			return catalogMigrateResult{}, fmt.Errorf("keeping a copy of catalog.json: %w", err)
		}
	} else {
		raw = nil
	}
	if opt.stopAt == "stub" {
		return catalogMigrateResult{}, errCatalogMoveStopped
	}
	if err := replaceWithStub(dataDir, raw, backup, opt); err != nil {
		return catalogMigrateResult{}, err
	}
	return catalogMigrateResult{Finished: true, Backup: backup}, nil
}

// errTwoCatalogs is the refusal when catalog.db and a catalog.json it was not made
// from are both in the folder.
func errTwoCatalogs(dataDir string) error {
	return fmt.Errorf("both %s and a catalog.json that is not the one it was made from are in %s. Refusing to "+
		"choose between them: catalog.json changed after the move, perhaps written by another copy of Obelisk. "+
		"Keep the one you trust and move the other out of the folder", catalogDBName, dataDir)
}

// replaceWithStub replaces catalog.json by the stub, but only while it still holds
// raw, the catalog that was moved (or, when raw is nil, while it is still absent),
// so a save by another copy of Obelisk is never overwritten. What is left is the
// moment between this check and the rename itself.
func replaceWithStub(dataDir string, raw []byte, backup string, opt catalogMigrateOptions) error {
	jsonPath := filepath.Join(dataDir, "catalog.json")
	if opt.beforeStub != nil {
		if err := opt.beforeStub(); err != nil {
			return err
		}
	}
	if raw != nil {
		if err := catalogJSONUnchanged(jsonPath, raw); err != nil {
			return errTwoCatalogs(dataDir)
		}
	} else if _, err := os.Lstat(jsonPath); !errors.Is(err, fs.ErrNotExist) {
		return errTwoCatalogs(dataDir)
	}
	if err := writeCatalogStub(jsonPath, backup); err != nil {
		return fmt.Errorf("the catalog moved to %s, but catalog.json could not be replaced by its pointer (%w). "+
			"Run the move again to finish it", catalogDBName, err)
	}
	return nil
}

// catalogJSONUnchanged returns an error unless catalog.json still holds raw.
func catalogJSONUnchanged(jsonPath string, raw []byte) error {
	now, err := readFileRetrying(jsonPath)
	if err != nil {
		return err
	}
	if sha256Hex(now) != sha256Hex(raw) {
		return errors.New("its contents differ from what was read")
	}
	return nil
}

// lockCatalogMove takes the folder's move lock, so two moves never run at once
// (one would sweep the other's staging files). The operating system drops the lock
// if the process dies; the lock file is removed on release.
func lockCatalogMove(dataDir string) (func(), error) {
	path := filepath.Join(dataDir, catalogMoveLock)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot lock %s for the move: %w", dataDir, err)
	}
	if err := lockFileExclusive(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("another copy of Obelisk is moving the catalog in %s right now; try again when it "+
			"has finished", dataDir)
	}
	return func() {
		unlockFile(f)
		f.Close()
		os.Remove(path)
	}, nil
}

// readCatalogForMove decodes catalog.json for the move: it must be a catalog this
// build knows every part of, at schema v8 or older (an older one is migrated to v8
// in memory, as openStore does), with every stored hash in canonical form.
func readCatalogForMove(raw []byte) (*catalog, int, error) {
	if len(raw) == 0 {
		return nil, 0, errors.New("catalog.json is empty (0 bytes); that is a damaged catalog")
	}
	if isCatalogStub(raw) {
		return nil, 0, errors.New("catalog.json is the pointer to a catalog that has already moved")
	}
	if err := checkCatalogJSONKeys(raw); err != nil {
		return nil, 0, err
	}
	c := &catalog{}
	if err := decodeCatalogJSON(raw, c); err != nil {
		return nil, 0, fmt.Errorf("catalog.json is damaged: %w", err)
	}
	from := c.SchemaVersion
	if from > currentSchemaVersion {
		return nil, 0, fmt.Errorf("catalog.json was written by a newer version of Obelisk (catalog schema v%d; "+
			"this build understands v%d)", from, currentSchemaVersion)
	}
	if c.NextID == nil {
		c.NextID = map[string]int{}
	}
	if from < currentSchemaVersion {
		s := &Store{c: *c}
		s.runMigrations(from)
		*c = s.c
	}
	if err := checkCatalogHashes(c); err != nil {
		return nil, 0, err
	}
	return c, from, nil
}

// importCatalogDB writes c into the new database d in one transaction, with the
// secondary indexes built afterwards (faster than updating them per row), and
// records where it came from.
func importCatalogDB(d *catalogDB, c *catalog, raw []byte, from int) error {
	if _, err := d.w.Exec(`PRAGMA cache_size=-262144`); err != nil {
		return err
	}
	return d.Write(func(tx *sql.Tx) error {
		type index struct{ name, sql string }
		var indexes []index
		rows, err := tx.Query(`SELECT name, sql FROM sqlite_schema WHERE type='index' AND sql IS NOT NULL ORDER BY name`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var ix index
			if err := rows.Scan(&ix.name, &ix.sql); err != nil {
				rows.Close()
				return err
			}
			indexes = append(indexes, ix)
		}
		rows.Close()
		for _, ix := range indexes {
			if _, err := tx.Exec(`DROP INDEX ` + ix.name); err != nil {
				return err
			}
		}
		if err := importCatalogRows(tx, c); err != nil {
			return err
		}
		for _, ix := range indexes {
			if _, err := tx.Exec(ix.sql); err != nil {
				return fmt.Errorf("building index %s: %w", ix.name, err)
			}
		}
		for _, kv := range [][2]string{
			{"source_sha256", sha256Hex(raw)},
			{"source_schema_version", strconv.Itoa(from)},
			{"source_bytes", strconv.Itoa(len(raw))},
			{"imported_at", time.Now().UTC().Format(time.RFC3339Nano)},
		} {
			if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES(?, ?)`, kv[0], kv[1]); err != nil {
				return err
			}
		}
		return nil
	})
}

// checkCatalogDB opens the database at path and proves it holds want: SQLite's own
// integrity check, the row count of every table, and the catalog read back from it
// hashing the same as want, section by section. want must already be canonical.
func checkCatalogDB(path string, want *catalog, wantCounts map[string]int) (map[string]int, error) {
	d, err := openCatalogDB(path, false)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	var got *catalog
	counts := map[string]int{}
	err = d.Read(func(tx *sql.Tx) error {
		var problems []string
		rows, err := tx.Query(`PRAGMA integrity_check`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return err
			}
			if s != "ok" {
				problems = append(problems, s)
			}
		}
		rows.Close()
		if len(problems) > 0 {
			return fmt.Errorf("the database failed its integrity check: %s", strings.Join(problems, "; "))
		}
		for table := range wantCounts {
			var n int
			if err := tx.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
				return err
			}
			counts[table] = n
		}
		got, err = exportCatalogRows(tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	var diff []string
	for _, table := range catalogKeys(wantCounts) {
		if counts[table] != wantCounts[table] {
			diff = append(diff, fmt.Sprintf("%s has %d rows, catalog.json has %d", table, counts[table], wantCounts[table]))
		}
	}
	if len(diff) > 0 {
		return nil, fmt.Errorf("the database does not match catalog.json: %s", strings.Join(diff, "; "))
	}
	canonCatalog(got)
	wantSums, gotSums := catalogSectionDigests(want), catalogSectionDigests(got)
	for _, section := range catalogKeys(wantSums) {
		if wantSums[section] != gotSums[section] {
			diff = append(diff, section)
		}
	}
	if len(diff) > 0 {
		return nil, fmt.Errorf("the database does not match catalog.json: these sections read back differently: %s",
			strings.Join(diff, ", "))
	}
	return counts, nil
}

// exportCatalogDB reads the whole catalog out of d, as one consistent state.
func exportCatalogDB(d *catalogDB) (*catalog, error) {
	var c *catalog
	err := d.Read(func(tx *sql.Tx) error {
		var err error
		c, err = exportCatalogRows(tx)
		return err
	})
	return c, err
}

// exportCatalogJSON writes d's catalog as a portable catalog.json document (the
// v8 JSON shape, compact): readable without Obelisk or SQLite.
func exportCatalogJSON(d *catalogDB, w io.Writer) error {
	c, err := exportCatalogDB(d)
	if err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(c)
}

// catalogDBRowCounts is the number of rows each table must hold for c.
func catalogDBRowCounts(c *catalog) map[string]int {
	n := map[string]int{
		"collection": len(c.Collections), "folder": len(c.Folders), "file": len(c.Files), "chunk": len(c.Chunks),
		"volume": len(c.Volumes), "location": len(c.Locations), "volume_snapshot": len(c.Snapshots),
		"scan": len(c.Scans), "event": len(c.Events), "audit": len(c.Audit), "plan": len(c.Plans),
		"drift_report": len(c.Drift),
	}
	for _, f := range c.Files {
		n["file_version"] += len(f.Versions)
	}
	for _, ch := range c.Chunks {
		n["chunk_file"] += len(ch.Files) + len(ch.UnknownFiles)
		n["copy"] += len(ch.Copies)
		n["segment"] += len(ch.Segments)
		n["verify_event"] += len(ch.VerifyEvents)
	}
	for _, v := range c.Volumes {
		n["smart_snapshot"] += len(v.SmartHistory)
		n["finalization"] += len(v.Finalizations)
	}
	for _, s := range c.Snapshots {
		n["snap_file"] += len(s.Files)
	}
	for _, p := range c.Plans {
		n["plan_entry"] += len(planHashes(p))
	}
	for _, r := range c.Drift {
		n["drift_item"] += len(r.Items)
	}
	for table, field := range catalogDocFields(c) {
		n[table] = field.Len()
	}
	return n
}

// checkCatalogHashes refuses a catalog with a hash the database cannot store as
// bytes: anything but empty or 64 lowercase hex characters.
func checkCatalogHashes(c *catalog) error {
	var bad []string
	check := func(where, h string) {
		if h == "" {
			return
		}
		if _, ok := hashBytes(h); !ok && len(bad) < 5 {
			bad = append(bad, fmt.Sprintf("%s: %q", where, h))
		}
	}
	for _, f := range c.Files {
		where := fmt.Sprintf("file %d (%s)", f.ID, f.RelPath)
		check(where+" hash", f.Hash)
		check(where+" blake3", f.Blake3)
		for i, v := range f.Versions {
			check(fmt.Sprintf("%s version %d hash", where, i+1), v.Hash)
		}
	}
	for _, ch := range c.Chunks {
		for _, refs := range [][]ChunkFileRef{ch.Files, ch.UnknownFiles} {
			for _, r := range refs {
				where := fmt.Sprintf("package %s file %s", ch.Name, r.RelPath)
				check(where+" hash", r.Hash)
				check(where+" sample hash", r.SampleHash)
			}
		}
	}
	for _, s := range c.Snapshots {
		for _, f := range s.Files {
			where := fmt.Sprintf("snapshot of volume %d file %s", s.VolumeID, f.RelPath)
			check(where+" hash", f.Hash)
			check(where+" blake3", f.Blake3)
		}
	}
	for _, p := range c.Plans {
		for _, h := range planHashes(p) {
			if h == "" {
				bad = append(bad, fmt.Sprintf("plan %s: an empty hash", p.Name))
			}
			check(fmt.Sprintf("plan %s", p.Name), h)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("catalog.json holds hashes that are not 64 lowercase hexadecimal characters, for example %s",
			strings.Join(bad, "; "))
	}
	return nil
}

// checkCatalogJSONKeys walks raw against the catalog's Go types and refuses any key
// the decoder would drop (unknown), merge (duplicate) or match only by ignoring
// case, and any data after the catalog. Values are not checked here; the decoder
// does that.
func checkCatalogJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := walkJSONKeys(dec, reflect.TypeOf(catalog{}), "catalog"); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("catalog.json has more data after the catalog")
	}
	return nil
}

var (
	timeType       = reflect.TypeOf(time.Time{})
	rawMessageType = reflect.TypeOf(json.RawMessage{})
)

func walkJSONKeys(dec *json.Decoder, t reflect.Type, path string) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("catalog.json is damaged at %s: %w", path, err)
	}
	delim, isDelim := tok.(json.Delim)
	switch {
	case tok == nil, !isDelim:
		return nil
	case t == timeType || t == rawMessageType || t.Kind() == reflect.Interface:
		return skipJSON(dec, delim)
	case t.Kind() == reflect.Struct && delim == '{':
		fields := jsonFields(t)
		seen := map[string]bool{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return fmt.Errorf("catalog.json is damaged at %s: %w", path, err)
			}
			key := k.(string)
			ft, ok := fields[key]
			if !ok {
				return fmt.Errorf("catalog.json has a key this version does not know: %s.%s", path, key)
			}
			if seen[key] {
				return fmt.Errorf("catalog.json has the key %s.%s twice", path, key)
			}
			seen[key] = true
			if err := walkJSONKeys(dec, ft, path+"."+key); err != nil {
				return err
			}
		}
	case t.Kind() == reflect.Map && delim == '{':
		seen := map[string]bool{}
		for dec.More() {
			k, err := dec.Token()
			if err != nil {
				return fmt.Errorf("catalog.json is damaged at %s: %w", path, err)
			}
			key := k.(string)
			if seen[key] {
				return fmt.Errorf("catalog.json has the key %s[%q] twice", path, key)
			}
			seen[key] = true
			if err := walkJSONKeys(dec, t.Elem(), fmt.Sprintf("%s[%q]", path, key)); err != nil {
				return err
			}
		}
	case (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && delim == '[':
		for i := 0; dec.More(); i++ {
			if err := walkJSONKeys(dec, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	default:
		// The wrong kind of value for the field: the decoder reports it.
		return skipJSON(dec, delim)
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("catalog.json is damaged at %s: %w", path, err)
	}
	return nil
}

// skipJSON consumes the rest of the object or array opened by delim.
func skipJSON(dec *json.Decoder, delim json.Delim) error {
	if delim != '{' && delim != '[' {
		return nil
	}
	for depth := 1; depth > 0; {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("catalog.json is damaged: %w", err)
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
	}
	return nil
}

// jsonFields maps each JSON key a struct type decodes to the field's type, by the
// exact spelling encoding/json writes.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			for k, v := range jsonFields(f.Type) {
				out[k] = v
			}
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}

// canonCatalog puts c in the form the section comparison uses: every time in UTC,
// and every empty list or map as none. It changes nothing a reader of the catalog
// can tell apart.
func canonCatalog(c *catalog) { canonValue(reflect.ValueOf(c).Elem()) }

func canonValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			canonValue(v.Elem())
		}
	case reflect.Struct:
		if v.Type() == timeType {
			if v.CanSet() {
				v.Set(reflect.ValueOf(v.Interface().(time.Time).UTC()))
			}
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				canonValue(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Len() == 0 {
			if !v.IsNil() && v.CanSet() {
				v.Set(reflect.Zero(v.Type()))
			}
			return
		}
		for i := 0; i < v.Len(); i++ {
			canonValue(v.Index(i))
		}
	case reflect.Map:
		if v.Len() == 0 {
			if !v.IsNil() && v.CanSet() {
				v.Set(reflect.Zero(v.Type()))
			}
			return
		}
		iter := v.MapRange()
		for iter.Next() {
			e := reflect.New(v.Type().Elem()).Elem()
			e.Set(iter.Value())
			canonValue(e)
			v.SetMapIndex(iter.Key(), e)
		}
	}
}

// catalogSectionDigests hashes each top-level section of c as JSON, by its key.
func catalogSectionDigests(c *catalog) map[string]string {
	out := map[string]string{}
	rv := reflect.ValueOf(c).Elem()
	for i := 0; i < rv.NumField(); i++ {
		name, _, _ := strings.Cut(rv.Type().Field(i).Tag.Get("json"), ",")
		h := sha256.New()
		if err := json.NewEncoder(h).Encode(rv.Field(i).Interface()); err != nil {
			out[name] = "unencodable: " + err.Error()
			continue
		}
		out[name] = hex.EncodeToString(h.Sum(nil))
	}
	return out
}

func catalogKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// isCatalogStub reports whether raw is the pointer left in catalog.json by a move.
func isCatalogStub(raw []byte) bool {
	var s catalogStub
	return json.Unmarshal(raw, &s) == nil && s.SchemaVersion == catalogStubSchemaVersion && s.MovedTo == catalogDBName
}

// keepCatalogJSON keeps raw as catalog.json.pre-sqlite-v<from>-<time> beside the
// catalog, unless a kept copy with the same bytes is already there, and returns its
// name. It never replaces an existing file.
func keepCatalogJSON(dataDir string, raw []byte, from int) (string, error) {
	sum := sha256Hex(raw)
	matches, _ := filepath.Glob(filepath.Join(dataDir, catalogPreSQLite+"*"))
	for _, m := range matches {
		if b, err := readFileRetrying(m); err == nil && sha256Hex(b) == sum {
			return m, nil
		}
	}
	final := filepath.Join(dataDir, fmt.Sprintf("%s%d-%s", catalogPreSQLite, from, time.Now().UTC().Format("20060102-150405.000000000")))
	tmp, err := writeStage(final, raw, 0o644)
	if err != nil {
		return "", err
	}
	if err := publishNoReplace(tmp, final); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return final, nil
}

// publishNoReplace moves the finished file from to final, refusing when final
// already exists. A hard link is the atomic no-replace publish where the
// filesystem has links; elsewhere (FAT, exFAT) the absence is checked just before
// the rename. The folder entry is then flushed.
func publishNoReplace(from, final string) error {
	if err := os.Link(from, final); err == nil {
		// final is in place. A staging name that cannot be removed now is only a
		// second name for the same file; the next move sweeps it.
		_ = os.Remove(from)
	} else {
		if _, serr := os.Lstat(final); !errors.Is(serr, fs.ErrNotExist) {
			return fmt.Errorf("%s already exists; refusing to replace it", final)
		}
		if err := renameReplacing(from, final); err != nil {
			return err
		}
	}
	return syncDir(filepath.Dir(final))
}

// writeCatalogStub replaces catalog.json with the pointer to catalog.db.
func writeCatalogStub(jsonPath, backup string) error {
	note := "This catalog has moved into catalog.db, a SQLite database in this folder."
	if backup != "" {
		note += " The catalog.json it replaced is kept beside it as " + filepath.Base(backup) + "."
	}
	note += " A version of Obelisk that cannot read catalog.db opens this folder read-only."
	b, err := json.MarshalIndent(catalogStub{SchemaVersion: catalogStubSchemaVersion, MovedTo: catalogDBName, Note: note}, "", " ")
	if err != nil {
		return err
	}
	tmp, err := writeStage(jsonPath, append(b, '\n'), 0o644)
	if err != nil {
		return err
	}
	if err := renameReplacing(tmp, jsonPath); err != nil {
		os.Remove(tmp)
		return err
	}
	return syncDir(filepath.Dir(jsonPath))
}

// sweepCatalogStages removes staging databases a stopped move left behind. Only
// this file makes names with that prefix.
func sweepCatalogStages(dataDir string) {
	matches, _ := filepath.Glob(filepath.Join(dataDir, catalogStagePrefix+"*"))
	for _, m := range matches {
		os.Remove(m)
	}
}
