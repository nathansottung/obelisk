//go:build !guionly

package main

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func newCatalogDBForTest(t *testing.T) (*catalogDB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.db")
	d, err := openCatalogDB(path, true)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d, path
}

func pragmaString(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	if err := db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}

func readMeta(t *testing.T, d *catalogDB, key string) string {
	t.Helper()
	var v string
	err := d.Read(func(tx *sql.Tx) error { return tx.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v) })
	if err != nil {
		t.Fatalf("meta %s: %v", key, err)
	}
	return v
}

func setMetaRaw(t *testing.T, path, key, value string) {
	t.Helper()
	db, err := sql.Open(catalogDBDriver, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE meta SET value=? WHERE key=?`, value, key); err != nil {
		t.Fatal(err)
	}
}

func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A new catalog database runs in WAL mode with full fsync on commit, enforces
// foreign keys, and records its format and schema version.
func TestCatalogDB_CreateIsDurable(t *testing.T) {
	d, _ := newCatalogDBForTest(t)
	if got := pragmaString(t, d.w, "journal_mode"); got != "wal" {
		t.Errorf("journal_mode = %q, want wal", got)
	}
	if got := pragmaString(t, d.w, "synchronous"); got != "2" { // 2 = FULL
		t.Errorf("synchronous = %q, want 2 (FULL)", got)
	}
	if got := pragmaString(t, d.w, "foreign_keys"); got != "1" {
		t.Errorf("foreign_keys = %q, want 1", got)
	}
	if got := readMeta(t, d, "format"); got != catalogDBFormat {
		t.Errorf("format = %q", got)
	}
	if got := readMeta(t, d, "schema_version"); got != "1" {
		t.Errorf("schema_version = %q, want 1", got)
	}
	if ro, why := d.ReadOnly(); ro {
		t.Errorf("a new catalog database is read-only: %s", why)
	}
}

// Every write goes through one connection; reads use a separate pool that cannot
// write, and see the last committed state while a write is open.
func TestCatalogDB_OneWriterAndReadPool(t *testing.T) {
	d, _ := newCatalogDBForTest(t)
	if n := d.w.Stats().MaxOpenConnections; n != 1 {
		t.Fatalf("writer allows %d connections, want 1", n)
	}
	if _, err := d.r.Exec(`INSERT INTO meta(key, value) VALUES('x', 'y')`); err == nil {
		t.Fatal("the read pool accepted a write")
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('counter', '0')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// Read-modify-write from many goroutines: one writer means none is lost.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := d.Write(func(tx *sql.Tx) error {
				var n int
				if err := tx.QueryRow(`SELECT CAST(value AS INTEGER) FROM meta WHERE key='counter'`).Scan(&n); err != nil {
					return err
				}
				_, err := tx.Exec(`UPDATE meta SET value=? WHERE key='counter'`, n+1)
				return err
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := readMeta(t, d, "counter"); got != "20" {
		t.Fatalf("counter = %s after 20 serialized increments", got)
	}

	// A reader is not blocked by an open write and sees the committed value.
	inWrite, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- d.Write(func(tx *sql.Tx) error {
			if _, err := tx.Exec(`UPDATE meta SET value='21' WHERE key='counter'`); err != nil {
				return err
			}
			close(inWrite)
			<-release
			return nil
		})
	}()
	<-inWrite
	got := make(chan string, 1)
	go func() { got <- readMeta(t, d, "counter") }()
	select {
	case v := <-got:
		if v != "20" {
			t.Errorf("reader during a write saw %s, want the committed 20", v)
		}
	case <-time.After(5 * time.Second):
		t.Error("a reader blocked behind an open write")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if v := readMeta(t, d, "counter"); v != "21" {
		t.Errorf("after commit counter = %s", v)
	}
}

// A failed write function rolls back everything it did.
func TestCatalogDB_WriteRollsBackOnError(t *testing.T) {
	d, _ := newCatalogDBForTest(t)
	boom := errors.New("boom")
	err := d.Write(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('half', 'done')`); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write returned %v, want the function's error", err)
	}
	var n int
	if err := d.Read(func(tx *sql.Tx) error { return tx.QueryRow(`SELECT count(*) FROM meta WHERE key='half'`).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("a failed write left its row behind")
	}
}

// Reopening finds what was written; a clean close leaves no -wal or -shm file.
func TestCatalogDB_ReopenAndCleanClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	d, err := openCatalogDB(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('kept', 'yes')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(path + side); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("after a clean close %s exists (err=%v)", side, err)
		}
	}
	d, err = openCatalogDB(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if got := readMeta(t, d, "kept"); got != "yes" {
		t.Errorf("kept = %q", got)
	}
}

// Opening without create never makes a file.
func TestCatalogDB_MissingIsNotCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	if _, err := openCatalogDB(path, false); err == nil {
		t.Fatal("opened a catalog database that does not exist")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused open created %s (err=%v)", path, err)
	}
}

// A catalog database that cannot be read is refused and left exactly as it was:
// never opened as new, never initialized, never overwritten.
func TestCatalogDB_UnreadableIsRefused(t *testing.T) {
	sqliteNoMeta := func(t *testing.T, path string) {
		db, err := sql.Open(catalogDBDriver, path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE other(x)`); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	cases := []struct {
		name string
		make func(t *testing.T, path string)
		want string
	}{
		{"empty file", func(t *testing.T, path string) { os.WriteFile(path, nil, 0o644) }, "empty"},
		{"not a database", func(t *testing.T, path string) {
			os.WriteFile(path, []byte(`{"schema_version":8,"files":[]}`), 0o644)
		}, "not a catalog database"},
		{"another program's database", sqliteNoMeta, "not an Obelisk catalog"},
		{"a folder in the way", func(t *testing.T, path string) { os.Mkdir(path, 0o755) }, "not a file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "catalog.db")
			tc.make(t, path)
			before := dirSnapshot(t, filepath.Dir(path))
			for _, create := range []bool{false, true} {
				d, err := openCatalogDB(path, create)
				if err == nil {
					d.Close()
					t.Fatalf("create=%v: opened it", create)
				}
				if !strings.Contains(err.Error(), tc.want) {
					t.Errorf("create=%v: error %q does not say %q", create, err, tc.want)
				}
			}
			if after := dirSnapshot(t, filepath.Dir(path)); after != before {
				t.Errorf("a refused open changed the folder:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}

// A catalog database the user cannot read is refused with the reason, not
// opened as new. (Unix permission bits; Windows is covered by the hold tests.)
func TestCatalogDB_PermissionDeniedIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not deny the owner on Windows; see the hold tests")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	d, path := newCatalogDBForTest(t)
	d.Close()
	before := fileBytes(t, path)
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)
	for _, create := range []bool{false, true} {
		if d, err := openCatalogDB(path, create); err == nil {
			d.Close()
			t.Fatalf("create=%v: opened an unreadable catalog database", create)
		} else if !strings.Contains(err.Error(), "cannot be read") {
			t.Errorf("create=%v: %v", create, err)
		}
	}
	os.Chmod(path, 0o644)
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Error("a refused open changed the database")
	}
}

// A database written by a newer schema opens read-only: reads work, every write is
// refused with the reason, and the file is left as it was.
func TestCatalogDB_NewerVersionIsReadOnly(t *testing.T) {
	d, path := newCatalogDBForTest(t)
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('from', 'newer')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	setMetaRaw(t, path, "schema_version", "2")
	before := fileBytes(t, path)

	d, err := openCatalogDB(path, false)
	if err != nil {
		t.Fatalf("a newer database should open read-only, got %v", err)
	}
	ro, why := d.ReadOnly()
	if !ro || !strings.Contains(why, "newer version") || !strings.Contains(why, "v2") {
		t.Fatalf("ReadOnly = %v, %q", ro, why)
	}
	if got := readMeta(t, d, "from"); got != "newer" {
		t.Errorf("read %q", got)
	}
	called := false
	err = d.Write(func(tx *sql.Tx) error { called = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "newer version") {
		t.Errorf("Write on a newer database: %v", err)
	}
	if called {
		t.Error("Write ran its function on a newer database")
	}
	if _, err := d.w.Exec(`UPDATE meta SET value='x' WHERE key='from'`); err == nil {
		t.Error("the writer connection accepted a write on a newer database")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Error("opening a newer database read-only changed the file")
	}
}

// A database whose recorded schema or format this build cannot place is refused.
func TestCatalogDB_BadMetaIsRefused(t *testing.T) {
	for _, tc := range []struct{ key, value, want string }{
		{"schema_version", "0", "schema version"},
		{"schema_version", "seven", "schema version"},
		{"format", "something-else", "not an Obelisk catalog"},
	} {
		d, path := newCatalogDBForTest(t)
		d.Close()
		setMetaRaw(t, path, tc.key, tc.value)
		if d, err := openCatalogDB(path, false); err == nil {
			d.Close()
			t.Errorf("%s=%s: opened", tc.key, tc.value)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s=%s: %v", tc.key, tc.value, err)
		}
	}
}

// A path the driver would split is refused rather than opened as another file.
func TestCatalogDB_QuestionMarkPathIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names cannot contain ?")
	}
	dir := filepath.Join(t.TempDir(), "a?b")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if d, err := openCatalogDB(filepath.Join(dir, "catalog.db"), true); err == nil {
		d.Close()
		t.Fatal("opened a path containing ?")
	}
	if names, _ := os.ReadDir(dir); len(names) != 0 {
		t.Errorf("a refused open wrote %v", names)
	}
	if names, _ := os.ReadDir(filepath.Dir(dir)); len(names) != 1 {
		t.Errorf("a refused open wrote beside the folder: %v", names)
	}
}

// The network-drive decision: known network filesystems are refused by name.
func TestCatalogDB_NetworkFilesystemDecision(t *testing.T) {
	for _, kind := range []string{"NFS", "SMB", "CIFS", "SMB2", "AFS", "9P", "Ceph", "network drive"} {
		err := catalogDBNetworkRefusal("/data/catalog.db", kind)
		if err == nil || !strings.Contains(err.Error(), "network") || !strings.Contains(err.Error(), "local drive") {
			t.Errorf("%s: %v", kind, err)
		}
	}
	if err := catalogDBNetworkRefusal("/data/catalog.db", ""); err != nil {
		t.Errorf("a local filesystem was refused: %v", err)
	}
}

// The temp folder the tests use is local, so the real check accepts it.
func TestCatalogDB_LocalFolderPassesNetworkCheck(t *testing.T) {
	if err := checkCatalogDBLocal(t.TempDir()); err != nil {
		t.Fatalf("a local temp folder was refused: %v", err)
	}
}

// dirSnapshot describes a folder's entries with their sizes and contents' hash.
func dirSnapshot(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Name())
		if e.Type().IsRegular() {
			b.WriteString(":" + sha256Hex(fileBytes(t, filepath.Join(dir, e.Name()))))
		} else {
			b.WriteString(":" + e.Type().String())
		}
		b.WriteString(" ")
	}
	return b.String()
}
