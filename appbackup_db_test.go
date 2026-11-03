//go:build !guionly

package main

import (
	"archive/tar"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// dbTestApp is abTestApp on the catalog database engine, whatever the run chose.
func dbTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv(catalogEngineEnv, "sqlite")
	return abTestApp(t)
}

// dirNames lists every name under dir, for a test that nothing changed. The daily
// backups of the catalog database are left out: a commit makes them, in the
// background, not the operation under test.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if strings.HasPrefix(rel, catalogDBBackupPrefix) {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// noSideFiles lists names in dir no successful restore may leave: the restore's own
// stages and an old -journal.
func leftStages(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	for _, n := range dirNames(t, dir) {
		if strings.Contains(n, ".restore-") || strings.HasPrefix(n, "restore-stage-") || strings.HasSuffix(n, "-journal") ||
			strings.Contains(n, ".export-") {
			out = append(out, n)
		}
	}
	return out
}

// rewriteBackup rebuilds tarPath with edit applied to its members and the manifest's
// sizes and hashes updated to match, without a sidecar: a backup that passes every
// hash check, whatever its members hold.
func rewriteBackup(t *testing.T, tarPath string, edit func(members map[string][]byte)) string {
	t.Helper()
	members, err := readTarMembers(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	var man appBackupManifest
	if err := json.Unmarshal(members["MANIFEST.json"], &man); err != nil {
		t.Fatal(err)
	}
	delete(members, "MANIFEST.json")
	edit(members)
	man.Members = nil
	var names []string
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		man.Members = append(man.Members, appBackupMember{Name: name, SHA256: sha256Hex(members[name]), Size: int64(len(members[name]))})
	}
	mb, err := json.Marshal(man)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(name string, b []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	write("MANIFEST.json", mb)
	for _, name := range names {
		write(name, members[name])
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "rewritten.tar")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// On the database engine an app backup carries catalog.db: a whole catalog, with
// its job board, that opens on its own. It carries no catalog.json or jobs.json.
func TestAppBackupDB_ExportCarriesTheCatalogDatabase(t *testing.T) {
	app := dbTestApp(t)
	_, wantFiles, _, _, _, _ := abPopulate(t, app)
	res, err := app.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	members, err := readTarMembers(res.TarPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalog.json", "jobs.json"} {
		if _, ok := members[name]; ok {
			t.Errorf("the backup carries %s; on the database engine the catalog and its board are in catalog.db", name)
		}
	}
	db, ok := members["catalog.db"]
	if !ok {
		t.Fatalf("the backup has no catalog.db (members %v)", memberNames(readManifest(t, res.TarPath)))
	}
	if man := readManifest(t, res.TarPath); man.CatalogDBSchemaVersion != catalogDBSchemaVersion {
		t.Errorf("manifest catalog database schema = %d, want %d", man.CatalogDBSchemaVersion, catalogDBSchemaVersion)
	}
	if got := leftStages(t, app.DataDir); len(got) > 0 {
		t.Errorf("the export left %v in the data folder", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, catalogDBName), db, 0o644); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	if got := collectionNames(s); strings.Join(got, ",") != "Backup Me" {
		t.Errorf("the backup's catalog holds %v", got)
	}
	if n := len(testFiles(t, s.AllFiles())); n != wantFiles {
		t.Errorf("the backup's catalog holds %d files, want %d", n, wantFiles)
	}
	if jobs := s.Jobs(); len(jobs) != 1 || len(jobs[0].Artifacts) != 1 {
		t.Errorf("the backup's job board = %+v, want the one job with its artifact", jobs)
	}
}

// Changes not yet committed (a batch still open) are in the backup.
func TestAppBackupDB_ExportIncludesChangesNotYetCommitted(t *testing.T) {
	app := dbTestApp(t)
	holdBatch(app.Store)
	app.Store.AddCollection("Pending")
	res, err := app.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	members, _ := readTarMembers(res.TarPath)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, catalogDBName), members["catalog.db"], 0o644); err != nil {
		t.Fatal(err)
	}
	if got := collectionNames(openEngineStore(t, "sqlite", dir)); strings.Join(got, ",") != "Pending" {
		t.Fatalf("the backup holds %v, want the pending [Pending]", got)
	}
}

// A database backup restores on the database engine: the catalog and its job board
// come back, and the catalog it replaced is kept whole in pre-restore.
func TestAppBackupDB_RestoreRoundTrip(t *testing.T) {
	src := dbTestApp(t)
	wantArch, wantFiles, wantVols, wantPkgs, wantVE, wantJobs := abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := dbTestApp(t)
	dst.Store.AddCollection("Old one")
	res, err := dst.RestoreAppBackup(exp.TarPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if res.Archives != wantArch || res.Files != wantFiles || res.Volumes != wantVols || res.Packages != wantPkgs ||
		res.VerifyEvents != wantVE || res.Jobs != wantJobs {
		t.Fatalf("restored counts %+v, want %d/%d/%d/%d/%d/%d", res, wantArch, wantFiles, wantVols, wantPkgs, wantVE, wantJobs)
	}
	if got := collectionNames(dst.Store); strings.Join(got, ",") != "Backup Me" {
		t.Fatalf("after the restore the catalog holds %v", got)
	}
	if jobs := dst.Store.Jobs(); len(jobs) != 1 || len(jobs[0].Artifacts) != 1 {
		t.Fatalf("restored job board = %+v", jobs)
	}
	if got := leftStages(t, dst.DataDir); len(got) > 0 {
		t.Errorf("the restore left %v in the data folder", got)
	}
	pre := openBackupCopy(t, filepath.Join(res.PreRestoreDir, catalogDBName))
	if got := collectionNames(pre); strings.Join(got, ",") != "Old one" {
		t.Fatalf("the pre-restore copy holds %v, want the replaced catalog [Old one]", got)
	}
	// The restored catalog is what the data folder holds, not only what is in memory.
	if err := dst.Store.Close(); err != nil {
		t.Fatal(err)
	}
	again := openEngineStore(t, "sqlite", dst.DataDir)
	if got := collectionNames(again); strings.Join(got, ",") != "Backup Me" {
		t.Fatalf("reopened, the catalog holds %v", got)
	}
}

// A backup an older version made holds catalog.json and jobs.json. On the
// database engine they go through the importer: the catalog is moved into a new
// database with the full check, and the board moves in on the reopen. The data
// folder's catalog.json stays the stub, so no older version can mistake it.
func TestAppBackupDB_ACatalogJSONBackupGoesThroughTheImporter(t *testing.T) {
	t.Setenv(catalogEngineEnv, "json")
	src := abTestApp(t)
	wantArch, wantFiles, wantVols, wantPkgs, wantVE, wantJobs := abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !memberNames(readManifest(t, exp.TarPath))["catalog.json"] {
		t.Fatal("the catalog.json engine's backup has no catalog.json")
	}
	dst := dbTestApp(t)
	dst.Store.AddCollection("Old one")
	res, err := dst.RestoreAppBackup(exp.TarPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if res.Archives != wantArch || res.Files != wantFiles || res.Volumes != wantVols || res.Packages != wantPkgs ||
		res.VerifyEvents != wantVE || res.Jobs != wantJobs {
		t.Fatalf("restored counts %+v, want %d/%d/%d/%d/%d/%d", res, wantArch, wantFiles, wantVols, wantPkgs, wantVE, wantJobs)
	}
	if !dst.Store.UsesDatabase() {
		t.Fatal("the restored store is not on the database")
	}
	if got := collectionNames(dst.Store); strings.Join(got, ",") != "Backup Me" {
		t.Fatalf("after the restore the catalog holds %v", got)
	}
	if raw, err := os.ReadFile(filepath.Join(dst.DataDir, "catalog.json")); err == nil && !isCatalogStub(raw) {
		t.Errorf("the restore put the backup's catalog.json in the data folder beside catalog.db")
	}
	if got := leftStages(t, dst.DataDir); len(got) > 0 {
		t.Errorf("the restore left %v in the data folder", got)
	}
	pre := openBackupCopy(t, filepath.Join(res.PreRestoreDir, catalogDBName))
	if got := collectionNames(pre); strings.Join(got, ",") != "Old one" {
		t.Fatalf("the pre-restore copy holds %v", got)
	}
}

// The catalog.json engine cannot use a catalog database: such a backup is refused
// before anything changes.
func TestAppBackupDB_TheCatalogJSONEngineRefusesADatabaseBackup(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(catalogEngineEnv, "json")
	dst := abTestApp(t)
	dst.Store.AddCollection("Old one")
	before := dirNames(t, dst.DataDir)
	if _, err := dst.RestoreAppBackup(exp.TarPath); err == nil || !strings.Contains(err.Error(), "catalog.db") {
		t.Fatalf("restore = %v, want a refusal naming catalog.db", err)
	}
	if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("the refused restore changed the data folder:\n%v\nwas\n%v", got, before)
	}
}

// A backup holding both catalogs is ambiguous and refused.
func TestAppBackupDB_ABackupWithBothCatalogsIsRefused(t *testing.T) {
	src := dbTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	both := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) { m["catalog.json"] = []byte(`{"schema_version":8}`) })
	dst := dbTestApp(t)
	before := dirNames(t, dst.DataDir)
	if _, err := dst.RestoreAppBackup(both); err == nil {
		t.Fatal("a backup with both catalog.json and catalog.db was restored")
	}
	if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("the refused restore changed the data folder")
	}
}

// A damaged catalog.db (whose hashes match the manifest) is refused before
// anything changes, and the store in use keeps working.
func TestAppBackupDB_ADamagedDatabaseIsRefusedBeforeAnyChange(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func([]byte) []byte{
		"pages": func(b []byte) []byte {
			// Every page after the first gets a page type no b-tree page has.
			b = append([]byte(nil), b...)
			for p := 4096; p < len(b); p += 4096 {
				b[p] = 0x77
			}
			return b
		},
		// Every record still reads; only integrity_check sees the index is wrong.
		"an index without its entries": func(b []byte) []byte { return emptyAnIndex(t, b) },
		"not a database":               func([]byte) []byte { return []byte("not a database at all, just text") },
		"empty":                        func([]byte) []byte { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			bad := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) { m["catalog.db"] = edit(m["catalog.db"]) })
			dst := dbTestApp(t)
			dst.Store.AddCollection("Old one")
			if err := dst.Store.FlushPending(); err != nil {
				t.Fatal(err)
			}
			before := dirNames(t, dst.DataDir)
			if _, err := dst.RestoreAppBackup(bad); err == nil {
				t.Fatal("a damaged catalog database was restored")
			}
			if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
				t.Fatalf("the refused restore changed the data folder:\n%v\nwas\n%v", got, before)
			}
			dst.Store.AddCollection("Still works")
			if err := dst.Store.FlushPending(); err != nil {
				t.Fatalf("the store stopped working after a refused restore: %v", err)
			}
		})
	}
}

// A backup whose catalog database a newer version wrote is refused: update the
// app first, as for a newer catalog.json.
func TestAppBackupDB_ANewerDatabaseIsRefused(t *testing.T) {
	src := dbTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	members, _ := readTarMembers(exp.TarPath)
	newer := setDBMetaInBytes(t, members["catalog.db"], "schema_version", "99")
	bad := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) { m["catalog.db"] = newer })
	dst := dbTestApp(t)
	before := dirNames(t, dst.DataDir)
	if _, err := dst.RestoreAppBackup(bad); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("restore = %v, want a refusal for a newer database", err)
	}
	if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("the refused restore changed the data folder")
	}
}

// setDBMetaInBytes returns a copy of the database db with a meta value changed.
func setDBMetaInBytes(t *testing.T, db []byte, key, value string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "edit.db")
	if err := os.WriteFile(p, db, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open(catalogDBDriver, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE meta SET value=? WHERE key=?`, value, key); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// An old write-ahead log never meets the restored database: one left beside
// catalog.db when the store closed is folded into the old database or removed
// before the swap, so SQLite cannot replay the replaced catalog's pages into the
// restored one. A database whose header says WAL would read such a log as its own
// the moment it opened.
func TestAppBackupDB_AnOldWALNeverMeetsTheRestoredDatabase(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	walHeader := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) {
		db := append([]byte(nil), m["catalog.db"]...)
		db[18], db[19] = 2, 2 // the file format versions that mean WAL
		m["catalog.db"] = db
	})
	for name, bundle := range map[string]string{"as exported": exp.TarPath, "a database in WAL mode": walHeader} {
		t.Run(name, func(t *testing.T) {
			dst := dbTestApp(t)
			// A real log of the catalog being replaced, with pages of its own.
			holdBatch(dst.Store)
			for i := 0; i < 50; i++ {
				dst.Store.AddCollection("Old " + strings.Repeat("x", i))
			}
			if err := dst.Store.FlushPending(); err != nil {
				t.Fatal(err)
			}
			wal, err := os.ReadFile(filepath.Join(dst.DataDir, catalogDBName+"-wal"))
			if err != nil || len(wal) == 0 {
				t.Fatalf("no write-ahead log to copy (%d bytes, %v)", len(wal), err)
			}
			prev := restoreAfterClose
			restoreAfterClose = func(dataDir string) {
				if err := os.WriteFile(filepath.Join(dataDir, catalogDBName+"-wal"), wal, 0o644); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(func() { restoreAfterClose = prev })
			if _, err := dst.RestoreAppBackup(bundle); err != nil {
				t.Fatalf("restore: %v", err)
			}
			if got := collectionNames(dst.Store); strings.Join(got, ",") != "Backup Me" {
				t.Fatalf("after the restore the catalog holds %v: the old log was replayed into it", got)
			}
			if got := leftStages(t, dst.DataDir); len(got) > 0 {
				t.Errorf("the restore left %v in the data folder", got)
			}
			if err := dst.Store.Close(); err != nil {
				t.Fatal(err)
			}
			if got := collectionNames(openEngineStore(t, "sqlite", dst.DataDir)); strings.Join(got, ",") != "Backup Me" {
				t.Fatalf("reopened, the catalog holds %v", got)
			}
		})
	}
}

// A catalog another program still holds open is never swapped: its log is in use.
// The restore is refused, the catalog is unchanged, and the store is open again.
func TestAppBackupDB_ACatalogHeldOpenElsewhereIsNotSwapped(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := dbTestApp(t)
	dst.Store.AddCollection("Old one")
	other, err := openTestStore(t, dst.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	other.AddCollection("Other's")
	if _, err := dst.RestoreAppBackup(exp.TarPath); err == nil {
		t.Fatal("a catalog held open by another program was swapped")
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	if got := collectionNames(dst.Store); strings.Join(got, ",") != "Old one,Other's" {
		t.Fatalf("after the refused restore the store holds %v", got)
	}
	dst.Store.AddCollection("Still works")
	if err := dst.Store.FlushPending(); err != nil {
		t.Fatalf("the store does not work after the refused restore: %v", err)
	}
}

// Changes the database will not take are never lost to a restore: the restore is
// refused before anything changes, and the changes stay to be saved.
func TestAppBackupDB_UnsavedChangesRefuseTheRestore(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := dbTestApp(t)
	before := dirNames(t, dst.DataDir)
	holdBatch(dst.Store)
	dst.Store.AddCollection("Unsaved")
	dst.Store.mu.Lock()
	dst.Store.be.(*dbBackend).failCommit = func() error { return errors.New("disk full") }
	dst.Store.mu.Unlock()
	if _, err := dst.RestoreAppBackup(exp.TarPath); err == nil {
		t.Fatal("restored over changes that were not saved")
	}
	if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("the refused restore changed the data folder:\n%v\nwas\n%v", got, before)
	}
	dst.Store.mu.Lock()
	dst.Store.be.(*dbBackend).failCommit = nil
	dst.Store.mu.Unlock()
	if err := dst.Store.FlushPending(); err != nil {
		t.Fatal(err)
	}
	if got := collectionNames(persistedStore(t, dst.Store, dst.DataDir)); strings.Join(got, ",") != "Unsaved" {
		t.Fatalf("the changes were lost: the catalog holds %v", got)
	}
}

// When the pre-restore copy of the catalog cannot be made, nothing is restored and
// the store keeps working.
func TestAppBackupDB_AFailedPreRestoreCopyChangesNothing(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := dbTestApp(t)
	dst.Store.AddCollection("Old one")
	dst.Store.mu.Lock()
	dst.Store.be.(*dbBackend).failVacuum = func() error { return errors.New("no space for the copy") }
	dst.Store.mu.Unlock()
	if _, err := dst.RestoreAppBackup(exp.TarPath); err == nil || !strings.Contains(err.Error(), "pre-restore") {
		t.Fatalf("restore = %v, want a refusal naming the pre-restore copy", err)
	}
	if got := collectionNames(dst.Store); strings.Join(got, ",") != "Old one" {
		t.Fatalf("after the refused restore the store holds %v", got)
	}
	dst.Store.AddCollection("Still works")
	if err := dst.Store.FlushPending(); err != nil {
		t.Fatalf("the store does not work after the refused restore: %v", err)
	}
}

// saveTestCompare records a comparison with one result and returns its record.
func saveTestCompare(t *testing.T, app *App) (*CompareRun, []ClassResult) {
	t.Helper()
	results := []ClassResult{{State: "SAME", Path: "a.txt", Hash: testHash("a"), Size: 1}}
	run := &CompareRun{At: time.Now().UTC().Truncate(time.Second), RefLabel: "A", OtherLabel: "B",
		Counts: map[string]int{"SAME": 1}, Total: 1}
	if err := app.saveCompareRun(run, results); err != nil {
		t.Fatal(err)
	}
	return run, results
}

// App backups carry each saved comparison's results file, on either engine, and a
// restore puts it back where its record finds it. One that is missing or does not
// match its record is left out and reported.
func TestAppBackup_CarriesComparisonResults(t *testing.T) {
	src := abTestApp(t)
	run, results := saveTestCompare(t, src)
	damaged, _ := saveTestCompare(t, src)
	if err := os.WriteFile(filepath.Join(src.comparesDir(), damaged.ResultFile), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	names := memberNames(readManifest(t, exp.TarPath))
	if !names["compares/"+run.ResultFile] {
		t.Fatalf("the backup has no compares/%s (members %v)", run.ResultFile, names)
	}
	if names["compares/"+damaged.ResultFile] {
		t.Errorf("the backup carries a results file that does not match its record")
	}
	if !reflect.DeepEqual(exp.CompareFilesLeftOut, []string{damaged.ResultFile}) {
		t.Errorf("left out = %v, want [%s]", exp.CompareFilesLeftOut, damaged.ResultFile)
	}

	dst := abTestApp(t)
	if _, err := dst.RestoreAppBackup(exp.TarPath); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, err := dst.LoadCompareResults(dst.Store.CompareRun(run.ID))
	if err != nil {
		t.Fatalf("the restored comparison's results: %v", err)
	}
	if !reflect.DeepEqual(got, results) {
		t.Fatalf("restored results %+v, want %+v", got, results)
	}
}

// A comparison results file a backup carries but its catalog does not record is
// not restored, and the restore says so.
func TestAppBackup_AnUnrecordedComparisonFileIsSkipped(t *testing.T) {
	src := abTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	extra := compareFilePrefix + "20260101T000000Z-0123456789ab" + compareFileSuffix
	bad := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) { m["compares/"+extra] = []byte("stray") })
	dst := abTestApp(t)
	res, err := dst.RestoreAppBackup(bad)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !reflect.DeepEqual(res.CompareFilesSkipped, []string{"compares/" + extra}) {
		t.Errorf("skipped = %v", res.CompareFilesSkipped)
	}
	if _, err := os.Stat(filepath.Join(dst.comparesDir(), extra)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an unrecorded results file was restored (err=%v)", err)
	}
}

// The comparison results a restore can replace are copied to pre-restore first.
func TestAppBackup_PreRestoreKeepsTheComparisons(t *testing.T) {
	src := abTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := abTestApp(t)
	run, _ := saveTestCompare(t, dst)
	want, err := os.ReadFile(filepath.Join(dst.comparesDir(), run.ResultFile))
	if err != nil {
		t.Fatal(err)
	}
	res, err := dst.RestoreAppBackup(exp.TarPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(res.PreRestoreDir, "compares", run.ResultFile))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("pre-restore copy of the comparison: %v", err)
	}
}

// Moving a data folder that holds a catalog database carries the database whole,
// with changes not yet committed, and leaves the original folder as it was.
func TestMigrateDataDir_CarriesTheCatalogDatabase(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	legacy := t.TempDir()
	target := filepath.Join(t.TempDir(), "new-home")
	s, err := openTestStore(t, legacy)
	if err != nil {
		t.Fatal(err)
	}
	app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
	app.Store.AddCollection("Committed")
	holdBatch(app.Store)
	app.Store.AddCollection("Pending")
	if _, err := app.migrateDataDir(legacy, target); err != nil {
		t.Fatalf("move: %v", err)
	}
	t.Cleanup(func() { app.Store.Close() })
	if app.DataDir != target || !app.Store.UsesDatabase() {
		t.Fatalf("the app runs from %s (database %v)", app.DataDir, app.Store.UsesDatabase())
	}
	if got := collectionNames(app.Store); strings.Join(got, ",") != "Committed,Pending" {
		t.Fatalf("the moved catalog holds %v", got)
	}
	for _, side := range []string{"-journal"} {
		if _, err := os.Stat(filepath.Join(target, catalogDBName+side)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the move copied %s%s", catalogDBName, side)
		}
	}
	if got := collectionNames(openEngineStore(t, "sqlite", legacy)); strings.Join(got, ",") != "Committed,Pending" {
		t.Fatalf("the original folder holds %v", got)
	}
}

// A target that already holds a catalog database is never overwritten.
func TestMigrateDataDir_RefusesATargetWithACatalogDatabase(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	legacy, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(target, catalogDBName), []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := openTestStore(t, legacy)
	if err != nil {
		t.Fatal(err)
	}
	app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
	if _, err := app.migrateDataDir(legacy, target); err == nil {
		t.Fatal("moved over a catalog database")
	}
	if b, _ := os.ReadFile(filepath.Join(target, catalogDBName)); string(b) != "theirs" {
		t.Fatal("the target's catalog database was changed")
	}
}

var _ = io.EOF

// A restore never writes into a pre-restore folder an earlier restore made, even
// one made the same second.
func TestAppBackup_EachRestoreHasItsOwnPreRestoreFolder(t *testing.T) {
	src := abTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := abTestApp(t)
	now := time.Now().UTC()
	var earlier []string
	for i := 0; i < 3; i++ {
		dir := filepath.Join(dst.DataDir, "pre-restore-"+now.Add(time.Duration(i)*time.Second).Format("20060102-150405"))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("earlier"), 0o600); err != nil {
			t.Fatal(err)
		}
		earlier = append(earlier, dir)
	}
	res, err := dst.RestoreAppBackup(exp.TarPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	for _, dir := range earlier {
		if res.PreRestoreDir == dir {
			t.Fatalf("the restore used the earlier folder %s", dir)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "config.json")); string(b) != "earlier" {
			t.Fatalf("the restore wrote into the earlier folder %s", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(res.PreRestoreDir, "config.json")); err != nil {
		t.Fatalf("the restore's own pre-restore copy: %v", err)
	}
}

// appendUnlisted returns a copy of tarPath with a member the manifest does not list
// added at the end, without a sidecar.
func appendUnlisted(t *testing.T, tarPath, name string, data []byte) string {
	t.Helper()
	members, err := readTarMembers(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for n := range members {
		if n != "MANIFEST.json" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	write := func(n string, b []byte) {
		if err := tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(b))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	write("MANIFEST.json", members["MANIFEST.json"])
	for _, n := range names {
		write(n, members[n])
	}
	write(name, data)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "unlisted.tar")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Only members the manifest lists, with their hashes checked, are used: a
// catalog.db added to an older backup without being listed is not restored, and
// the backup's listed catalog.json is.
func TestAppBackupDB_AMemberTheManifestDoesNotListIsNotUsed(t *testing.T) {
	other := dbTestApp(t)
	other.Store.AddCollection("Unlisted")
	stray, _, err := other.Store.snapshotDBBytes()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(catalogEngineEnv, "json")
	src := abTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	bundle := appendUnlisted(t, exp.TarPath, "catalog.db", stray)
	dst := dbTestApp(t)
	if _, err := dst.RestoreAppBackup(bundle); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := collectionNames(dst.Store); strings.Join(got, ",") != "Backup Me" {
		t.Fatalf("after the restore the catalog holds %v, want the listed catalog's [Backup Me]", got)
	}
}

// A backup's database whose board was never marked moved in never takes in the
// data folder's jobs.json: its board is the one it carries.
func TestAppBackupDB_ARestoredDatabaseNeverTakesInTheLocalJobsJSON(t *testing.T) {
	src := dbTestApp(t)
	abPopulate(t, src)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	members, _ := readTarMembers(exp.TarPath)
	unmarked := deleteDBMetaInBytes(t, members["catalog.db"], metaJobMoved)
	bundle := rewriteBackup(t, exp.TarPath, func(m map[string][]byte) { m["catalog.db"] = unmarked })
	dst := dbTestApp(t)
	local := `{"next":77,"rows":[{"id":77,"kind":"scan","label":"local only","status":"COMPLETED"}]}`
	if err := os.WriteFile(filepath.Join(dst.DataDir, "jobs.json"), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.RestoreAppBackup(bundle); err != nil {
		t.Fatalf("restore: %v", err)
	}
	for _, j := range dst.Store.Jobs() {
		if j.ID == 77 {
			t.Fatalf("the restored board took in the data folder's jobs.json: %+v", j)
		}
	}
	if n := len(dst.Store.Jobs()); n != 1 {
		t.Fatalf("restored board has %d jobs, want the backup's 1", n)
	}
}

// deleteDBMetaInBytes returns a copy of the database db without a meta value.
func deleteDBMetaInBytes(t *testing.T, db []byte, key string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "edit.db")
	if err := os.WriteFile(p, db, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open(catalogDBDriver, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`DELETE FROM meta WHERE key=?`, key); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// emptyAnIndex returns a copy of the database db in which one index of the file
// table has no entries: the table reads whole, and only integrity_check finds it.
func emptyAnIndex(t *testing.T, db []byte) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "index.db")
	if err := os.WriteFile(p, db, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := sql.Open(catalogDBDriver, p)
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	var name string
	if err := d.QueryRow(`SELECT name FROM sqlite_schema WHERE type='index' AND tbl_name='file' AND sql IS NOT NULL ORDER BY name LIMIT 1`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	var root int
	// An empty index of a table of its own lends its pages: an index b-tree with no
	// entries.
	for _, q := range []string{`CREATE TABLE zz_empty(x)`, `CREATE INDEX zz_empty_x ON zz_empty(x)`} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.QueryRow(`SELECT rootpage FROM sqlite_schema WHERE name='zz_empty_x'`).Scan(&root); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`PRAGMA writable_schema=ON`, `UPDATE sqlite_schema SET rootpage=` + strconv.Itoa(root) + ` WHERE name='` + name + `'`,
		`DELETE FROM sqlite_schema WHERE name='zz_empty_x'`} {
		if _, err := d.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A restore never runs while a job is running: the job's work would go to a store
// the restore closes or replaces. It is refused before anything changes, and the
// job can still record its end.
func TestAppBackup_ARestoreIsRefusedWhileAJobRuns(t *testing.T) {
	src := abTestApp(t)
	exp, err := src.ExportAppBackup(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dst := abTestApp(t)
	j, err := dst.Store.NewJob("write", "Write a package")
	if err != nil {
		t.Fatal(err)
	}
	before := dirNames(t, dst.DataDir)
	if _, err := dst.RestoreAppBackup(exp.TarPath); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("restore = %v, want a refusal while a job runs", err)
	}
	if got := dirNames(t, dst.DataDir); !reflect.DeepEqual(got, before) {
		t.Fatalf("the refused restore changed the data folder:\n%v\nwas\n%v", got, before)
	}
	if err := dst.Store.SetJob(j.ID, 1, "", "COMPLETED"); err != nil {
		t.Fatalf("the running job could not record its end: %v", err)
	}
	if _, err := dst.RestoreAppBackup(exp.TarPath); err != nil {
		t.Fatalf("restore once no job runs: %v", err)
	}
}

// A data folder move never runs while a job is running.
func TestMigrateDataDir_IsRefusedWhileAJobRuns(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	legacy := t.TempDir()
	target := filepath.Join(t.TempDir(), "new-home")
	s, err := openTestStore(t, legacy)
	if err != nil {
		t.Fatal(err)
	}
	app := initializedTestApp(t, &App{DataDir: legacy, Store: s})
	if _, err := app.Store.NewJob("write", "Write a package"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.migrateDataDir(legacy, target); err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("move = %v, want a refusal while a job runs", err)
	}
	if app.Store != s || app.DataDir != legacy {
		t.Fatal("the refused move switched the app")
	}
}

// A copy never removes a file it did not make: one already at its destination is
// left as it is.
func TestVacuumCopy_NeverRemovesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	s.AddCollection("A")
	dest := filepath.Join(t.TempDir(), "catalog.db")
	if err := os.WriteFile(dest, []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := vacuumCopy(filepath.Join(dir, catalogDBName), dest); err == nil {
		t.Fatal("copied over an existing file")
	}
	if b, _ := os.ReadFile(dest); string(b) != "theirs" {
		t.Fatalf("the existing file was changed or removed (%q)", b)
	}
}
