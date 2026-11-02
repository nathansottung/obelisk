//go:build windows && !guionly

package main

import (
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// holdExclusive opens path the way a virus scanner or backup agent can: with no
// sharing at all, so every other open fails with a sharing violation until the
// returned release is called.
func holdExclusive(t *testing.T, path string) (release func()) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("hold %s: %v", path, err)
	}
	done := false
	return func() {
		if !done {
			done = true
			windows.CloseHandle(h)
		}
	}
}

// copyHeldFile copies a file another connection holds open; SQLite opens its files
// with read and write sharing, so this works on Windows.
func copyHeldFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// crashedCatalogDB leaves a catalog database whose last commit is only in its
// -wal file, as after a crash or power cut: the next open must replay it.
func crashedCatalogDB(t *testing.T) string {
	t.Helper()
	live := filepath.Join(t.TempDir(), "catalog.db")
	d, err := openCatalogDB(live, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.w.Exec(`PRAGMA wal_autocheckpoint=0`); err != nil {
		t.Fatal(err)
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('only-in-wal', 'yes')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	crashed := filepath.Join(t.TempDir(), "catalog.db")
	copyHeldFile(t, live, crashed)
	copyHeldFile(t, live+"-wal", crashed+"-wal")
	return crashed
}

// A scanner holding the -wal file for a moment does not stop the catalog
// database from opening, and the commit that lives only in the -wal is there.
func TestCatalogDB_AntivirusHoldOnWALIsWaitedOut(t *testing.T) {
	path := crashedCatalogDB(t)
	release := holdExclusive(t, path+"-wal")
	defer release()
	go func() { time.Sleep(400 * time.Millisecond); release() }()

	d, err := openCatalogDB(path, false)
	if err != nil {
		t.Fatalf("open while the -wal was briefly held: %v", err)
	}
	defer d.Close()
	if got := readMeta(t, d, "only-in-wal"); got != "yes" {
		t.Fatalf("the commit in the -wal is missing: %q", got)
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO meta(key, value) VALUES('after', 'hold')`)
		return err
	}); err != nil {
		t.Fatalf("write after the hold: %v", err)
	}
}

// A scanner holding the database file itself for a moment is waited out too.
func TestCatalogDB_AntivirusHoldOnDatabaseIsWaitedOut(t *testing.T) {
	d, path := newCatalogDBForTest(t)
	d.Close()
	release := holdExclusive(t, path)
	defer release()
	go func() { time.Sleep(400 * time.Millisecond); release() }()
	d, err := openCatalogDB(path, false)
	if err != nil {
		t.Fatalf("open while the database was briefly held: %v", err)
	}
	d.Close()
}

// A hold that outlasts the wait is refused with the reason, and the database and
// its -wal are left exactly as they were: nothing is replaced or initialized.
func TestCatalogDB_LongHoldIsRefusedAndNothingChanges(t *testing.T) {
	for _, held := range []string{"", "-wal"} {
		t.Run("hold"+held, func(t *testing.T) {
			path := crashedCatalogDB(t)
			before := dirSnapshot(t, filepath.Dir(path))
			release := holdExclusive(t, path+held)
			d, err := openCatalogDB(path, false)
			if err == nil {
				// A database held only on its -wal can open; it must not be usable
				// as a catalog without that -wal's commit.
				v := ""
				rerr := d.Read(func(tx *sql.Tx) error {
					return tx.QueryRow(`SELECT value FROM meta WHERE key='only-in-wal'`).Scan(&v)
				})
				d.Close()
				release()
				t.Fatalf("opened while %q was held for good (read %q, %v)", "catalog.db"+held, v, rerr)
			}
			release()
			if !strings.Contains(err.Error(), "cannot be read") {
				t.Errorf("error does not say it cannot be read: %v", err)
			}
			if after := dirSnapshot(t, filepath.Dir(path)); after != before {
				t.Errorf("a refused open changed the folder:\nbefore %s\nafter  %s", before, after)
			}
			// Once the hold ends, the database opens with its -wal's commit.
			d, err = openCatalogDB(path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if got := readMeta(t, d, "only-in-wal"); got != "yes" {
				t.Fatalf("the commit in the -wal is missing after the hold: %q", got)
			}
		})
	}
}

// A data folder on a network share is refused, by its UNC spelling.
func TestCatalogDB_UNCPathRefused(t *testing.T) {
	err := catalogDBNetworkRefusal(`\\server\share\obelisk\catalog.db`, catalogDBWindowsKind(`\\server\share\obelisk`, windows.DRIVE_FIXED))
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("UNC: %v", err)
	}
	if kind := catalogDBWindowsKind(`Z:\obelisk`, windows.DRIVE_REMOTE); kind == "" {
		t.Fatal("a mapped network drive letter was not recognized")
	}
	if kind := catalogDBWindowsKind(`C:\obelisk`, windows.DRIVE_FIXED); kind != "" {
		t.Fatalf("a fixed drive was called %q", kind)
	}
}
