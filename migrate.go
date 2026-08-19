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
	return sameDir(a.DataDir, legacy) && !fileExists(filepath.Join(target, "catalog.json"))
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
	if !sameDir(a.DataDir, legacy) {
		return nil, fmt.Errorf("nothing to migrate: the app is not running from the legacy %s directory. Nothing was changed", legacy)
	}
	if fileExists(filepath.Join(target, "catalog.json")) {
		return nil, fmt.Errorf("%s already holds a catalog. Refusing to overwrite it; move it aside first if you meant to replace it", target)
	}

	// Copy every regular file under legacy into target, verifying each by hash.
	var copied, bytes int64
	err := filepath.WalkDir(legacy, func(path string, d fs.DirEntry, walkErr error) error {
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

	// Everything copied and verified — switch the running app to the new home by
	// reopening the store there. The catalog and job board now come from ~/.obelisk.
	ns, err := OpenStore(target)
	if err != nil {
		return nil, fmt.Errorf("your records copied and verified, but reopening the catalog at %s failed: %w. Restart the app to use the new location", target, err)
	}
	a.Store = ns
	a.DataDir = target
	a.Store.Log("migrate", fmt.Sprintf("copied %d verified file(s) from %s to %s", copied, legacy, target))
	return map[string]any{
		"from": legacy, "to": target, "files": copied, "bytes": bytes, "verified": copied,
	}, nil
}
