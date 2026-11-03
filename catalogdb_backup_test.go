//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// setBackupClock makes the daily backup see now as the time, until the test ends.
// A zero time makes no backup, so a test can open a store before its day starts.
func setBackupClock(t *testing.T, now time.Time) *time.Time {
	t.Helper()
	clock := now
	prev := catalogDBBackupNow
	catalogDBBackupNow = func() time.Time { return clock }
	t.Cleanup(func() { catalogDBBackupNow = prev })
	return &clock
}

// backupNames lists the daily backups in dir, oldest first.
func backupNames(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, catalogDBBackupPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range m {
		out = append(out, filepath.Base(p))
	}
	sort.Strings(out)
	return out
}

// openBackupCopy puts a backup in a new data folder as its catalog.db, as a person
// restoring it by hand would, and opens it.
func openBackupCopy(t *testing.T, bak string) *Store {
	t.Helper()
	b, err := os.ReadFile(bak)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, catalogDBName), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return openEngineStore(t, "sqlite", dir)
}

func collectionNames(s *Store) []string {
	var out []string
	for _, c := range s.Collections() {
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

// The first commit of a day writes that day's backup: a whole catalog database in
// one file, which opens on its own with everything committed. No stage is left.
func TestDBBackup_TheFirstCommitOfADayWritesACompleteCopy(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.AddCollection("Kept")
	s.waitBackups()
	if got := backupNames(t, dir); len(got) != 1 || got[0] != "catalog.db.bak-20261103" {
		t.Fatalf("backups = %v, want catalog.db.bak-20261103", got)
	}
	bak := filepath.Join(dir, "catalog.db.bak-20261103")
	for _, side := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(bak + side); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the backup has a %s beside it (err=%v); it must be one file", side, err)
		}
	}
	if got := collectionNames(openBackupCopy(t, bak)); strings.Join(got, ",") != "Kept" {
		t.Fatalf("the backup holds %v, want [Kept]", got)
	}
}

// Later commits that day leave the day's backup alone; the next day gets its own.
func TestDBBackup_OnePerDay(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.AddCollection("First")
	s.waitBackups()
	s.AddCollection("Second")
	bak := filepath.Join(dir, "catalog.db.bak-20261103")
	if got := collectionNames(openBackupCopy(t, bak)); strings.Join(got, ",") != "First" {
		t.Fatalf("the day's backup holds %v, want only the first commit's [First]", got)
	}
	*clock = clock.Add(24 * time.Hour)
	s.AddCollection("Third")
	s.waitBackups()
	next := filepath.Join(dir, "catalog.db.bak-20261104")
	if got := collectionNames(openBackupCopy(t, next)); strings.Join(got, ",") != "First,Second,Third" {
		t.Fatalf("the next day's backup holds %v", got)
	}
}

// quick_check runs on the database first: a database that fails it is not copied,
// and no older backup is pruned to make room for a copy of damage. The save that
// triggered it still succeeds.
func TestDBBackup_ADatabaseThatFailsQuickCheckIsNotCopied(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	old := fakeBackups(t, dir, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local), 40, 10)
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.mu.Lock()
	s.be.(*dbBackend).failBackupCheck = func(what string) error {
		if what == "database" {
			return errors.New("*** in database main *** page 7 is never used")
		}
		return nil
	}
	s.mu.Unlock()
	s.AddCollection("Kept")
	s.waitBackups()
	if err := s.FlushPending(); err != nil {
		t.Fatalf("a backup that could not be made failed the save: %v", err)
	}
	if got := backupNames(t, dir); strings.Join(got, ",") != strings.Join(old, ",") {
		t.Fatalf("backups = %v, want the older ones untouched and no new one", got)
	}
	if got := collectionNames(persistedStore(t, s, dir)); strings.Join(got, ",") != "Kept" {
		t.Fatalf("the catalog holds %v", got)
	}
}

// A copy that fails its own check is never published under the day's name, its
// stage is removed, and no older backup is pruned.
func TestDBBackup_ACopyThatFailsItsCheckIsNotPublished(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	old := fakeBackups(t, dir, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local), 40, 10)
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.mu.Lock()
	s.be.(*dbBackend).failBackupCheck = func(what string) error {
		if what == "copy" {
			return errors.New("the copy is damaged")
		}
		return nil
	}
	s.mu.Unlock()
	s.AddCollection("Kept")
	s.waitBackups()
	if got := backupNames(t, dir); strings.Join(got, ",") != strings.Join(old, ",") {
		t.Fatalf("backups = %v, want the older ones untouched and no new one", got)
	}
}

// A backup already under the day's name (another copy of Obelisk wrote it) is
// never replaced.
func TestDBBackup_AnExistingBackupIsNeverReplaced(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	bak := filepath.Join(dir, "catalog.db.bak-20261103")
	if err := os.WriteFile(bak, []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.AddCollection("Kept")
	s.waitBackups()
	if b, _ := os.ReadFile(bak); string(b) != "theirs" {
		t.Fatalf("the existing backup was replaced (%d bytes)", len(b))
	}
}

// fakeBackups writes n daily backups of size bytes ending the day before first+n,
// and returns their names, oldest first.
func fakeBackups(t *testing.T, dir string, first time.Time, n int, size int) []string {
	t.Helper()
	var names []string
	for i := 0; i < n; i++ {
		name := catalogDBBackupPrefix + first.AddDate(0, 0, i).Format("20060102")
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

// Seven daily copies are kept, then the newest of each of the four ISO weeks
// before them. Files that are not daily backups are never touched.
func TestDBBackup_KeepsSevenDailyAndFourWeekly(t *testing.T) {
	dir := t.TempDir()
	// Tuesday 3 November 2026, in ISO week 45.
	clock := setBackupClock(t, time.Time{})
	fakeBackups(t, dir, time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local), 63, 10) // 1 Sep to 2 Nov
	others := []string{"catalog.db.bak-notes.txt", "catalog.db.bak-2026110", "catalog.db.bak-20261340"}
	for _, name := range others {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	jsonBak := filepath.Join(dir, "catalog.json.bak-20200101")
	if err := os.WriteFile(jsonBak, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.AddCollection("Kept")
	s.waitBackups()
	want := []string{
		// the four weeks before the dailies: weeks 40 to 43, the newest of each
		"catalog.db.bak-20261004", "catalog.db.bak-20261011", "catalog.db.bak-20261018", "catalog.db.bak-20261025",
		// seven daily
		"catalog.db.bak-20261028", "catalog.db.bak-20261029", "catalog.db.bak-20261030", "catalog.db.bak-20261031",
		"catalog.db.bak-20261101", "catalog.db.bak-20261102", "catalog.db.bak-20261103",
	}
	want = append(want, others...)
	sort.Strings(want)
	if got := backupNames(t, dir); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("backups kept:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if _, err := os.Stat(jsonBak); err != nil {
		t.Fatalf("a catalog.json backup was touched: %v", err)
	}
}

// Past the size cap the oldest copies go first, and the newest is always kept.
func TestDBBackup_TheSizeCapDropsTheOldestFirst(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	prev := catalogDBBackupCap
	t.Cleanup(func() { catalogDBBackupCap = prev })
	fakeBackups(t, dir, time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local), 7, 1<<20) // 27 Oct to 2 Nov
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	catalogDBBackupCap = 3 << 20 // today's small copy and the two days before it
	s.AddCollection("Kept")
	s.waitBackups()
	want := "catalog.db.bak-20261101,catalog.db.bak-20261102,catalog.db.bak-20261103"
	if got := backupNames(t, dir); strings.Join(got, ",") != want {
		t.Fatalf("backups = %v, want %s", got, want)
	}

	// A cap smaller than the newest copy still keeps it.
	catalogDBBackupCap = 1
	*clock = time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local)
	s.AddCollection("More")
	s.waitBackups()
	if got := backupNames(t, dir); strings.Join(got, ",") != "catalog.db.bak-20261104" {
		t.Fatalf("backups = %v, want only the newest", got)
	}
}

// A stage a crash left behind is removed once it is old; a recent one may be
// another copy of Obelisk's backup in progress and is left alone.
func TestDBBackup_OldStagesAreSwept(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	oldStage := filepath.Join(dir, "catalog.db.bak-20261001.stage-0123456789ab")
	newStage := filepath.Join(dir, "catalog.db.bak-20261103.stage-ba9876543210")
	for _, p := range []string{oldStage, newStage} {
		if err := os.WriteFile(p, []byte("partial"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldStage, past, past); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	s.AddCollection("Kept")
	s.waitBackups()
	if _, err := os.Stat(oldStage); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an old stage was left (err=%v)", err)
	}
	if _, err := os.Stat(newStage); err != nil {
		t.Errorf("a recent stage was removed: %v", err)
	}
}

// persistedStore closes s and opens dir again, to read what was committed.
func persistedStore(t *testing.T, s *Store, dir string) *Store {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return openEngineStore(t, "sqlite", dir)
}

// The daily backup runs without holding the store: the catalog answers while a
// backup is being made.
func TestDBBackup_TheStoreAnswersWhileABackupIsMade(t *testing.T) {
	dir := t.TempDir()
	clock := setBackupClock(t, time.Time{})
	s := openEngineStore(t, "sqlite", dir)
	*clock = time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local)
	started, release := make(chan struct{}), make(chan struct{})
	s.mu.Lock()
	s.be.(*dbBackend).failBackupCheck = func(what string) error {
		if what == "database" {
			close(started)
			<-release
		}
		return nil
	}
	s.mu.Unlock()
	s.AddCollection("Kept")
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("no backup started")
	}
	answered := make(chan struct{})
	go func() {
		s.AddCollection("While it runs")
		s.Collections()
		close(answered)
	}()
	select {
	case <-answered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("the store was held while the backup was made")
	}
	close(release)
	s.waitBackups()
	if got := backupNames(t, dir); len(got) != 1 {
		t.Fatalf("backups = %v", got)
	}
}
