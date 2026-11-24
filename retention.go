package main

// retention.go — how long Obelisk keeps what it rotates.
//
// Obelisk rotates three kinds of record on its own: daily catalog backups,
// drive scan lists and saved comparisons. Each keeps a count (see where each is
// pruned), and the rule over every count is the same:
//
//   - the newest good copy is never removed (for backups, the newest; for scans
//     of a drive, the newest complete scan);
//   - nothing made in the last day is removed, however many there are.
//
// The rule is stated in Settings (ui/index.html, "Kept automatically, and left
// behind") and in docs/handbook/10-where-your-data-lives.md. Removal goes through the scratch helper (scratch.go).

import (
	"os"
	"path/filepath"
	"time"
)

// retentionMinAge is how young a kept record is always kept.
const retentionMinAge = 24 * time.Hour

// retentionNow is the clock for the rule; a test seam.
var retentionNow = time.Now

// madeInTheLastDay reports whether t is less than a day ago (or in the future,
// which a clock set back can make).
func madeInTheLastDay(t time.Time) bool {
	return retentionNow().Sub(t) < retentionMinAge
}

// removeRetained removes the named files of kind in dir that retention drops,
// except the newest file of that kind in dir and any made in the last day. s,
// when set, refuses anything under a source root. dir must be Obelisk's data
// folder or a folder in it.
func removeRetained(s *Store, dir string, drop []string, kind scratchKind) {
	if len(drop) == 0 || !(isCatalogFolder(dir) || isCatalogFolder(filepath.Dir(dir))) {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// The newest is kept by both measures: the last written (by modification
	// time) and the last by name (names carry the date they were made).
	newest, newestAt, lastName := "", time.Time{}, ""
	mod := map[string]time.Time{}
	for _, e := range entries {
		if !kind.match(e.Name()) || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		mod[e.Name()] = fi.ModTime()
		if newest == "" || fi.ModTime().After(newestAt) || fi.ModTime().Equal(newestAt) && e.Name() > newest {
			newest, newestAt = e.Name(), fi.ModTime()
		}
		if e.Name() > lastName {
			lastName = e.Name()
		}
	}
	for _, name := range drop {
		at, ok := mod[name]
		if !ok || name == newest || name == lastName || madeInTheLastDay(at) {
			continue
		}
		_ = removeOwned(s, []string{dir}, dir, name, kind)
	}
}

// keepScanRecord reports whether retention keeps a scan record that its count
// would drop: one made in the last day, or the newest complete scan of its
// drive.
func keepScanRecord(r *ScanRecord, newestComplete *ScanRecord) bool {
	return madeInTheLastDay(r.At) || r == newestComplete
}
