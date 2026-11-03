//go:build !guionly

package main

// catalogdb_backup.go — copies of the catalog database: the daily backups beside
// it, the snapshot an app backup, a pre-restore copy and a data-folder move take,
// and the staging of a database a restore puts in its place.
//
// Every copy is made with VACUUM INTO on a read-only connection of its own: one
// consistent picture of the last commit, written as a single file (no -wal), which
// opens on its own. The copy is synced and checked before it is used, and is made
// under a stage name and renamed into place, so a partial copy never has a real
// name.

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// catalogDBBackupPrefix names the daily backups beside catalog.db:
// catalog.db.bak-YYYYMMDD.
const catalogDBBackupPrefix = catalogDBName + ".bak-"

// catalogDBBackupStage marks a daily backup still being written.
const catalogDBBackupStage = ".stage-"

// The daily backups kept: the newest seven days, then the newest of each of the
// four ISO weeks before them, all within catalogDBBackupCap.
const (
	catalogDBBackupDaily  = 7
	catalogDBBackupWeekly = 4
)

// catalogDBBackupCap is the most the kept daily backups may take together. Past it
// the oldest go first; the newest is always kept.
var catalogDBBackupCap int64 = 16 << 30

// catalogDBStageAge is how old a backup stage must be before it is taken for one a
// crash left: a newer one may be another copy of Obelisk's backup in progress.
const catalogDBStageAge = time.Hour

func init() {
	stageCatalogDBRestore = stageRestoreDB
	importCatalogJSONRestore = stageRestoreImport
}

// copyDSN opens path read-only for copying it: SQLite's read-only mode, so the
// connection never folds the -wal or removes it, but without query_only, which
// would refuse VACUUM INTO.
func copyDSN(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	u := url.URL{Scheme: "file", Path: p}
	return u.String() + "?mode=ro&_pragma=busy_timeout(10000)"
}

// vacuumCopy writes a consistent copy of the database at src to dest, which must
// not exist, and syncs it. A failed copy is removed.
func vacuumCopy(src, dest string) error {
	if strings.ContainsRune(dest, '?') {
		return fmt.Errorf("the copy's path %s contains a ?, which the database driver cannot write", dest)
	}
	db, err := sql.Open(catalogDBDriver, copyDSN(src))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`VACUUM INTO ?`, dest)
	if cerr := db.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = syncFile(dest)
	}
	if err != nil {
		removeDBFile(dest)
		return err
	}
	return nil
}

// syncFile flushes path's bytes to stable storage.
func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// removeDBFile removes a copy this file made, with any side file SQLite left.
func removeDBFile(path string) {
	for _, side := range []string{"", "-wal", "-shm", "-journal"} {
		os.Remove(path + side)
	}
}

// checkDB runs pragma (quick_check or integrity_check) through db and returns
// what it found when that is not "ok".
func checkDB(db interface {
	Query(string, ...any) (*sql.Rows, error)
}, pragma string) error {
	rows, err := db.Query(`PRAGMA ` + pragma)
	if err != nil {
		return err
	}
	defer rows.Close()
	var found []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return err
		}
		if s != "ok" {
			found = append(found, s)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(found) > 0 {
		if len(found) > 5 {
			found = append(found[:5], fmt.Sprintf("and %d more", len(found)-5))
		}
		return fmt.Errorf("%s found: %s", pragma, strings.Join(found, "; "))
	}
	return nil
}

// checkDBCopy opens the database copy at path read-only and checks it: an Obelisk
// catalog this build can write (a newer schema is refused), that passes pragma,
// and whose records all read. It returns the catalog without its files.
func checkDBCopy(path, pragma string) (*catalog, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Size() == 0 {
		return nil, fmt.Errorf("%s is empty or not a file", filepath.Base(path))
	}
	if err := checkSQLiteHeader(path); err != nil {
		return nil, err
	}
	d := &catalogDB{path: path}
	if err := d.classifyExisting(); err != nil {
		return nil, err
	}
	if d.readOnly {
		return nil, errors.New(strings.Replace(d.readOnlyReason, "Upgrade the app to change it — it is open read-only so newer records are never lost",
			"Update the app first, then restore", 1))
	}
	r, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path, "foreign_keys(1)"))
	if err != nil {
		return nil, err
	}
	r.SetMaxOpenConns(1)
	d.r = r
	defer r.Close()
	if err := checkDB(r, pragma); err != nil {
		return nil, err
	}
	c, err := loadCatalogWithoutFiles(d)
	if err != nil {
		return nil, fmt.Errorf("reading its records: %w", err)
	}
	return c, nil
}

// snapshot writes a checked copy of the last commit to dest, which must not
// exist, and returns its schema version. The caller has committed what it wants
// in the copy.
func (b *dbBackend) snapshot(dest string) (int, error) {
	if b.closed {
		return 0, errCatalogClosed
	}
	if b.failVacuum != nil {
		if err := b.failVacuum(); err != nil {
			return 0, err
		}
	}
	if err := vacuumCopy(b.d.path, dest); err != nil {
		return 0, err
	}
	err := b.check("copy")
	if err == nil {
		_, err = checkDBCopy(dest, "quick_check")
	}
	if err != nil {
		removeDBFile(dest)
		return 0, fmt.Errorf("the copy failed its check: %w", err)
	}
	return catalogDBSchemaVersion, nil
}

// check runs the test seam for what ("database" or "copy").
func (b *dbBackend) check(what string) error {
	if b.failBackupCheck != nil {
		return b.failBackupCheck(what)
	}
	return nil
}

// quickCheck runs quick_check on the catalog database through the read pool.
func (b *dbBackend) quickCheck() error {
	if err := b.check("database"); err != nil {
		return err
	}
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(b.d.path))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	return checkDB(db, "quick_check")
}

// dailyBackup writes the day's backup, catalog.db.bak-YYYYMMDD, unless it exists,
// and prunes the backups to those kept. The database must pass quick_check first:
// a damaged one is not copied and no backup is pruned for it. The copy is made
// under a stage name, checked, and renamed to the day's name without replacing
// anything.
func (b *dbBackend) dailyBackup(now time.Time) error {
	if b.closed {
		return errCatalogClosed
	}
	dir := filepath.Dir(b.d.path)
	sweepBackupStages(dir)
	final := filepath.Join(dir, catalogDBBackupPrefix+now.Format("20060102"))
	if _, err := os.Lstat(final); err == nil {
		return nil // never replaced
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := b.quickCheck(); err != nil {
		return fmt.Errorf("the catalog database failed its quick check, so no backup was made and none was pruned: %w", err)
	}
	stage := final + catalogDBBackupStage + randomHex(6)
	if _, err := b.snapshot(stage); err != nil {
		return err
	}
	err := publishNoReplace(stage, final)
	removeDBFile(stage) // a staging name a hard-link publish leaves
	if err != nil {
		return err
	}
	_ = syncDir(dir)
	pruneDBBackups(dir, catalogDBBackupCap)
	return nil
}

// dbBackupFile is one daily backup.
type dbBackupFile struct {
	name string
	day  time.Time
	size int64
}

// listDBBackups returns the daily backups in dir, newest first. Only names this
// file writes are listed: catalog.db.bak- and a real date.
func listDBBackups(dir string) []dbBackupFile {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []dbBackupFile
	for _, e := range ents {
		rest, ok := strings.CutPrefix(e.Name(), catalogDBBackupPrefix)
		if !ok || len(rest) != 8 || !e.Type().IsRegular() {
			continue
		}
		day, err := time.Parse("20060102", rest)
		if err != nil || day.Format("20060102") != rest {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, dbBackupFile{name: e.Name(), day: day, size: fi.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].day.After(out[j].day) })
	return out
}

// keptDBBackups chooses from files (newest first) the backups to keep: the newest
// seven, then the newest of each of the four ISO weeks before them (a week a daily
// copy is in is not counted again), then the oldest of those go while they take
// more than limit together. The newest is always kept.
func keptDBBackups(files []dbBackupFile, limit int64) map[string]bool {
	var chosen []dbBackupFile
	weeks := map[[2]int]bool{}
	weekly := 0
	for i, f := range files {
		y, w := f.day.ISOWeek()
		wk := [2]int{y, w}
		switch {
		case i < catalogDBBackupDaily:
		case weeks[wk] || weekly >= catalogDBBackupWeekly:
			continue
		default:
			weekly++
		}
		weeks[wk] = true
		chosen = append(chosen, f)
	}
	keep := map[string]bool{}
	var total int64
	for i, f := range chosen {
		total += f.size
		if i > 0 && total > limit {
			break
		}
		keep[f.name] = true
	}
	return keep
}

// pruneDBBackups removes the daily backups in dir that are not kept.
func pruneDBBackups(dir string, limit int64) {
	files := listDBBackups(dir)
	keep := keptDBBackups(files, limit)
	for _, f := range files {
		if !keep[f.name] {
			os.Remove(filepath.Join(dir, f.name))
		}
	}
}

// sweepBackupStages removes daily-backup stages old enough that a crash left them.
func sweepBackupStages(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		name := e.Name()
		if !strings.HasPrefix(name, catalogDBBackupPrefix) || !strings.Contains(name, catalogDBBackupStage) || !e.Type().IsRegular() {
			continue
		}
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > catalogDBStageAge {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// restoreStageName is a new stage name beside the catalog database in dataDir.
func restoreStageName(dataDir string) string {
	return filepath.Join(dataDir, catalogDBName+".restore-"+randomHex(6))
}

// stageRestoreDB writes a backup's catalog.db beside the catalog in dataDir under a
// stage name, synced, and checks it whole (integrity_check): a damaged database,
// or one a newer version wrote, is refused and its stage removed. A database whose
// board was never moved in is marked moved, so the restore never takes in
// dataDir's jobs.json. It returns the stage and the backup's catalog without files.
func stageRestoreDB(dataDir string, member []byte) (string, *catalog, error) {
	stage := restoreStageName(dataDir)
	f, err := os.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", nil, err
	}
	_, err = f.Write(member)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = markBoardMoved(stage, nil)
	}
	var c *catalog
	if err == nil {
		c, err = checkDBCopy(stage, "integrity_check")
	}
	if err != nil {
		removeDBFile(stage)
		return "", nil, fmt.Errorf("the backup's catalog database is refused: %w", err)
	}
	return stage, c, nil
}

// stageRestoreImport moves a backup's catalog.json (and its job board, when the
// backup has one) into a new catalog database through the importer, in a folder of
// its own inside dataDir, and copies the result beside the catalog under a stage
// name. Nothing outside that folder and the stage is touched.
func stageRestoreImport(dataDir string, raw, jobs []byte) (string, *catalog, error) {
	refuse := func(err error) (string, *catalog, error) {
		return "", nil, fmt.Errorf("the backup's catalog.json could not be moved into a catalog database: %w", err)
	}
	work := filepath.Join(dataDir, "restore-stage-"+randomHex(6))
	if err := os.Mkdir(work, 0o700); err != nil {
		return refuse(err)
	}
	defer os.RemoveAll(work) // only the folder made above, which holds copies of the backup's members
	if err := os.WriteFile(filepath.Join(work, "catalog.json"), raw, 0o600); err != nil {
		return refuse(err)
	}
	if _, err := migrateCatalogToDB(work, catalogMigrateOptions{}); err != nil {
		return refuse(err)
	}
	var board *jobBoard
	if jobs != nil {
		in, err := decodeJobBoard(jobs)
		if err != nil {
			return refuse(fmt.Errorf("its job board: %w", err))
		}
		board = &in
	}
	imported := filepath.Join(work, catalogDBName)
	if err := markBoardMoved(imported, board); err != nil {
		return refuse(fmt.Errorf("writing its job board: %w", err))
	}
	stage := restoreStageName(dataDir)
	if err := vacuumCopy(imported, stage); err != nil {
		return refuse(err)
	}
	c, err := checkDBCopy(stage, "integrity_check")
	if err != nil {
		removeDBFile(stage)
		return refuse(err)
	}
	return stage, c, nil
}

// markBoardMoved writes board (when not nil) into the job table of the database at
// path and marks the board moved, so opening it never reads a jobs.json. A database
// already marked is left as it is. The database keeps its journal mode.
func markBoardMoved(path string, board *jobBoard) error {
	db, err := sql.Open(catalogDBDriver, catalogDBDSN(path, "synchronous(FULL)"))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var moved int
	if err := tx.QueryRow(`SELECT count(*) FROM meta WHERE key = ?`, metaJobMoved).Scan(&moved); err != nil {
		return err
	}
	if moved > 0 {
		return nil
	}
	if _, err := tx.Exec(catalogDBJobTableDDL); err != nil {
		return err
	}
	meta := map[string]string{metaJobMoved: time.Now().UTC().Format(time.RFC3339Nano)}
	var rows []storedJob
	if board != nil {
		for _, j := range board.Rows {
			doc, err := encodeStoredJob(j)
			if err != nil {
				return fmt.Errorf("job %d: %w", j.ID, err)
			}
			rows = append(rows, storedJob{ID: j.ID, Doc: doc})
		}
		meta[metaJobNext] = strconv.Itoa(board.Next)
	}
	if err := writeJobRows(tx, rows, meta); err != nil {
		return err
	}
	return tx.Commit()
}

// releaseCatalogDB makes sure nothing else has the catalog database at path open
// and that no old log is left beside it, before a restore replaces it. The store
// has closed it. Taking it out of WAL mode needs every other connection gone, and
// folds the -wal in and removes it and the -shm; any side file still there after
// that is removed, and one that cannot be removed (held open, on Windows) refuses
// the swap. The database's contents are in the pre-restore copy already.
func releaseCatalogDB(path string) error {
	held := func(detail string) error {
		return fmt.Errorf("another program still has the catalog database %s open (%s). Close every other copy of "+
			"Obelisk or tool using this data folder and try again", path, detail)
	}
	db, err := sql.Open(catalogDBDriver, path+"?_pragma=busy_timeout(2000)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	var mode string
	err = db.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode)
	if cerr := db.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return held(err.Error())
	}
	if !strings.EqualFold(mode, "delete") {
		return held("it could not be taken out of its write-ahead log mode")
	}
	for _, side := range []string{"-wal", "-shm", "-journal"} {
		p := path + side
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			if runtime.GOOS == "windows" {
				return held(fmt.Sprintf("%s is in use: %v", filepath.Base(p), err))
			}
			return fmt.Errorf("removing the old %s: %w", filepath.Base(p), err)
		}
	}
	return nil
}

// swapCatalogDB puts the staged database in place of the catalog database in
// dataDir, after releaseCatalogDB. The swap is one rename.
func swapCatalogDB(dataDir, stage string) error {
	live := filepath.Join(dataDir, catalogDBName)
	if err := releaseCatalogDB(live); err != nil {
		return err
	}
	if err := renameReplacing(stage, live); err != nil {
		return err
	}
	_ = syncDir(dataDir)
	return nil
}

func init() { swapRestoredCatalogDB = swapCatalogDB }
