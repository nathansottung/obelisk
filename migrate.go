package main

// migrate.go — the one-time, opt-in move of a pre-rename ~/.mnemo data directory
// into the new ~/.obelisk home. It COPIES (never moves) and VERIFIES every file by
// hash before switching the running app over, so a Mnemosyne user's records travel
// forward with zero risk: the originals under ~/.mnemo stay a complete, valid data
// directory forever. See defaultDataDir (main.go) for the silent read fallback, and
// the "Name compatibility" section in ARCHITECTURE.md.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// legacyAndTargetDataDirs returns the pre-rename (~/.mnemo) and current (~/.obelisk)
// default data directories. Both are empty if the home directory can't be resolved.
func legacyAndTargetDataDirs() (legacy, target string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	return filepath.Join(home, ".mnemo"), filepath.Join(home, ".obelisk")
}

// sameDir reports whether two paths name the same directory after cleaning.
func sameDir(a, b string) bool {
	ca, _ := filepath.Abs(filepath.Clean(a))
	cb, _ := filepath.Abs(filepath.Clean(b))
	return ca == cb
}

// LegacyDataDirMigrationAvailable reports whether a one-time copy from a pre-rename
// ~/.mnemo into ~/.obelisk can be OFFERED: the app is currently running out of the
// legacy ~/.mnemo directory (the silent fallback in defaultDataDir) and ~/.obelisk
// does not already hold a catalog. It never triggers anything on its own.
func (a *App) LegacyDataDirMigrationAvailable() bool {
	legacy, target := legacyAndTargetDataDirs()
	if legacy == "" || target == "" {
		return false
	}
	return sameDir(a.DataDir, legacy) && !targetHoldsCatalog(target)
}

// targetHoldsCatalog reports whether dir already holds a catalog, on either engine.
func targetHoldsCatalog(dir string) bool {
	return fileExists(filepath.Join(dir, "catalog.json")) || fileExists(filepath.Join(dir, "catalog.db"))
}

// liveDatabaseFile reports whether rel (relative to the data folder) is the catalog
// database, one of its side files, or a copy of it still being written. A data
// folder move copies the database as one consistent snapshot instead.
func liveDatabaseFile(rel string) bool {
	if filepath.Dir(rel) != "." {
		return false
	}
	switch rel {
	case "catalog.db", "catalog.db-wal", "catalog.db-shm", "catalog.db-journal", "catalog.db.move-lock":
		return true
	}
	for _, stage := range []string{".stage-", ".restore-", ".export-", ".creating-", ".migrating-"} {
		if strings.HasPrefix(rel, "catalog.db") && strings.Contains(rel, stage) {
			return true
		}
	}
	return false
}

// MigrateLegacyDataDir COPIES the running (legacy ~/.mnemo) data directory into
// ~/.obelisk, VERIFYING every copied file by SHA-256, then switches the running app
// to the new directory. It never moves or deletes the originals. It refuses if the
// target already holds a catalog, so it can never clobber an existing new-home
// catalog. Returns a summary of what was copied and verified.
func (a *App) MigrateLegacyDataDir() (map[string]any, error) {
	legacy, target := legacyAndTargetDataDirs()
	if legacy == "" || target == "" {
		return nil, fmt.Errorf("cannot resolve your home directory, so records can't be migrated. Pass -data explicitly instead")
	}
	return a.migrateDataDir(legacy, target)
}

// migrateDataDir is the testable core of MigrateLegacyDataDir with explicit source
// and destination directories.
func (a *App) migrateDataDir(legacy, target string) (map[string]any, error) {
	if running := a.Store.RunningJobs(); len(running) > 0 {
		return nil, errJobsRunning("moving the data folder", running)
	}
	if !sameDir(a.DataDir, legacy) {
		return nil, fmt.Errorf("nothing to migrate: the app is not running from the legacy %s directory. Nothing was changed", legacy)
	}
	if targetHoldsCatalog(target) {
		return nil, fmt.Errorf("%s already holds a catalog. Refusing to overwrite it; move it aside first if you meant to replace it", target)
	}
	// No edit reaches the catalog while it is copied: one made after its file was
	// copied would stay behind in the original folder (swapgate.go). Every pending
	// change is saved first.
	keep, done, err := a.holdForSwap("moving the data folder")
	if err != nil {
		return nil, err
	}
	defer done()

	// Copy every regular file under legacy into target, verifying each by hash. On
	// the database engine the catalog database is not copied file by file (its -wal
	// would make a copy of a moment that never was): it is snapshot whole below.
	useDB := a.Store.UsesDatabase()
	var copied, bytes int64
	err = filepath.WalkDir(legacy, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(legacy, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil // skip symlinks / sockets / devices
		}
		if useDB && liveDatabaseFile(rel) {
			return nil // copied below, as one consistent snapshot
		}
		if err := copyFile(path, dst); err != nil {
			return fmt.Errorf("copying %s: %w", rel, err)
		}
		// Verify the copy byte-for-byte by hash BEFORE counting it. A mismatch aborts
		// the whole migration; the originals under ~/.mnemo are never touched.
		sh, err := hashFileHex(path)
		if err != nil {
			return fmt.Errorf("hashing source %s: %w", rel, err)
		}
		dh, err := hashFileHex(dst)
		if err != nil {
			return fmt.Errorf("hashing copy %s: %w", rel, err)
		}
		if sh != dh {
			return fmt.Errorf("verifying %s: the copy does not match the original, so the migration was stopped. Your original records under %s are untouched", rel, legacy)
		}
		copied++
		if fi, e := os.Stat(dst); e == nil {
			bytes += fi.Size()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if useDB {
		// Every change the store holds is committed first, the copy is checked, and
		// the store is closed with nothing let in between: no change can reach the
		// original folder after the copy. From here the app switches to the copy.
		if err := a.Store.closeWithCopy("moving the data folder", filepath.Join(target, "catalog.db")); err != nil {
			err = fmt.Errorf("copying the catalog database: %w. Your original records under %s are untouched", err, legacy)
			return nil, a.reopenAfterRefusal(err)
		}
		copied++
		if fi, e := os.Stat(filepath.Join(target, "catalog.db")); e == nil {
			bytes += fi.Size()
		}
	}

	// Everything copied and verified — switch the running app to the new home by
	// reopening the store there. The catalog and job board now come from ~/.obelisk.
	ns, err := OpenStore(target)
	if err != nil {
		if useDB {
			return nil, a.reopenAfterRefusal(fmt.Errorf("your records copied and verified, but opening the catalog at %s failed: %w. The app keeps using %s", target, err, legacy))
		}
		a.Store.checkJobsAfterFailedReopen()
		return nil, fmt.Errorf("your records copied and verified, but reopening the catalog at %s failed: %w. Resolve the rejected state before retrying", target, err)
	}
	a.Store = ns
	a.DataDir = target
	keep("") // replaced: the original folder's store never takes an edit again
	a.Store.Log("migrate", fmt.Sprintf("copied %d verified file(s) from %s to %s", copied, legacy, target))
	return map[string]any{
		"from": legacy, "to": target, "files": copied, "bytes": bytes, "verified": copied,
	}, nil
}
