//go:build !guionly

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// dbJobs reads the job board the catalog database has committed, through its read
// pool, which sees only committed transactions.
func dbJobs(t *testing.T, s *Store) map[int]*Job {
	t.Helper()
	out := map[int]*Job{}
	err := s.be.(*dbBackend).d.Read(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT id, doc FROM job ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			var doc string
			if err := rows.Scan(&id, &doc); err != nil {
				return err
			}
			j, err := decodeStoredJob(id, doc)
			if err != nil {
				return err
			}
			out[id] = j
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("read the committed jobs: %v", err)
	}
	return out
}

// readCommittedJobs reads the job table of the catalog database at path through a
// connection of its own, which sees only committed transactions.
func readCommittedJobs(t *testing.T, path string) []*Job {
	t.Helper()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, doc FROM job ORDER BY id`)
	if err != nil {
		t.Fatalf("read the committed jobs: %v", err)
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		var id int
		var doc string
		if err := rows.Scan(&id, &doc); err != nil {
			t.Fatal(err)
		}
		j, err := decodeStoredJob(id, doc)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// skipWithDatabaseJobs skips a test of how the jobs.json sidecar is read again on
// each open: on the database engine the board is in catalog.db, and jobs.json is
// read only by the first open, which moves it (the TestJobs_ tests below).
func skipWithDatabaseJobs(t *testing.T) {
	t.Helper()
	if engine, _ := catalogEngine(); engine == "sqlite" {
		t.Skip("the job board is in the catalog database; jobs.json is read only when it is moved")
	}
}

// dbCount is the committed number of rows in table.
func dbCount(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.be.(*dbBackend).d.Read(func(tx *sql.Tx) error {
		return tx.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

// holdBatch leaves a batch open, as another running job does, with saves put off
// for an hour, so the changes made next are pending until something commits them.
func holdBatch(s *Store) {
	s.BeginBatch()
	s.mu.Lock()
	s.batchInterval, s.lastSave = time.Hour, time.Now()
	s.mu.Unlock()
}

// On the database engine the job board is in catalog.db, not in jobs.json, and a
// job left running is interrupted on the next open.
func TestJobs_DatabaseEngineKeepsTheBoardInTheDatabase(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	j, err := s.NewJob("scan", "Scan A")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the database engine wrote jobs.json (err=%v)", err)
	}
	if got := dbJobs(t, s)[j.ID]; got == nil || got.Status != "RUNNING" || got.Label != "Scan A" {
		t.Fatalf("committed job = %+v, want the running job", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	defer s.Close()
	if got := s.Job(j.ID); got == nil || got.Status != "INTERRUPTED" {
		t.Fatalf("after reopen = %+v, want INTERRUPTED", got)
	}
	if got := dbJobs(t, s)[j.ID]; got == nil || got.Status != "INTERRUPTED" {
		t.Errorf("committed after reopen = %+v, want INTERRUPTED", got)
	}
	k, err := s.NewJob("scan", "Scan B")
	if err != nil {
		t.Fatal(err)
	}
	if k.ID != j.ID+1 {
		t.Errorf("next job ID = %d, want %d", k.ID, j.ID+1)
	}
}

// A job's completion commits in the same transaction as the catalog changes not
// yet committed, even with another job's batch holding saves back.
func TestJobs_CompletionCommitsWithThePendingChanges(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	j, err := s.NewJob("scan", "Scan A")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.SaveGen()
	holdBatch(s)
	before := dbCount(t, s, "collection")
	s.AddCollection("A")
	if dbCount(t, s, "collection") != before {
		t.Fatal("setup: the change was committed before the job finished")
	}
	saveErr, recErr := s.CompleteJob(j.ID, gen, "Scan A", nil, map[string]any{"n": 1})
	if saveErr != nil || recErr != nil {
		t.Fatalf("CompleteJob = %v, %v", saveErr, recErr)
	}
	if got := dbJobs(t, s)[j.ID]; got == nil || got.Status != "COMPLETED" {
		t.Errorf("committed job = %+v, want COMPLETED", got)
	}
	if dbCount(t, s, "collection") != before+1 {
		t.Error("the job's change was not committed with its completion")
	}
}

// A final commit that fails records neither the completion nor the changes: the job
// shows FAILED and not recorded, and the database still says RUNNING. The next
// commit records both the failure and the changes.
func TestJobs_FailedFinalCommitRecordsNeitherTheJobNorItsChanges(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	be := s.be.(*dbBackend)
	j, err := s.NewJob("scan", "Scan A")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.SaveGen()
	holdBatch(s)
	before := dbCount(t, s, "collection")
	s.AddCollection("A")
	be.failCommit = func() error { return errors.New("disk full") }
	saveErr, recErr := s.CompleteJob(j.ID, gen, "Scan A", nil, nil)
	be.failCommit = nil
	if saveErr == nil || !strings.Contains(saveErr.Error(), "NOT recorded") {
		t.Errorf("save error = %v, want the work reported as not recorded", saveErr)
	}
	if recErr == nil {
		t.Error("the job's record was reported written although the commit failed")
	}
	got := s.Job(j.ID)
	if got.Status == "COMPLETED" || !got.Unrecorded || got.PersistError == "" {
		t.Errorf("in memory = %s unrecorded=%v persist_error=%q, want FAILED and not recorded",
			got.Status, got.Unrecorded, got.PersistError)
	}
	if !strings.Contains(got.Label, "NOT recorded") {
		t.Errorf("label = %q, want it to say the work is not recorded", got.Label)
	}
	if c := dbJobs(t, s)[j.ID]; c == nil || c.Status != "RUNNING" {
		t.Errorf("committed job = %+v, want still RUNNING", c)
	}
	if dbCount(t, s, "collection") != before {
		t.Error("the job's change was committed although its final commit failed")
	}
	if _, err := s.NewJob("scan", "Scan B"); err != nil {
		t.Fatal(err)
	}
	if c := dbJobs(t, s)[j.ID]; c == nil || c.Status != "FAILED" {
		t.Errorf("committed job after the next commit = %+v, want FAILED", c)
	}
	if got := s.Job(j.ID); got.Unrecorded || got.PersistError == "" {
		t.Errorf("after the next commit unrecorded=%v persist_error=%q, want recorded with the history kept",
			got.Unrecorded, got.PersistError)
	}
	if dbCount(t, s, "collection") != before+1 {
		t.Error("the change was not committed by the next commit")
	}
}

// A record the database refuses fails a job that changed the catalog: it is recorded
// FAILED, never COMPLETED.
func TestJobs_RefusedRecordFailsTheCompletion(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	j, err := s.NewJob("scan", "Scan A")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.SaveGen()
	holdBatch(s)
	s.mu.Lock()
	s.c.Snapshots = append(s.c.Snapshots, &VolumeSnapshot{VolumeID: 7,
		CapturedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)})
	_ = s.save()
	s.mu.Unlock()
	saveErr, recErr := s.CompleteJob(j.ID, gen, "Scan A", nil, nil)
	if saveErr == nil || !strings.Contains(saveErr.Error(), "NOT recorded") {
		t.Errorf("save error = %v, want the work reported as not recorded", saveErr)
	}
	if recErr != nil {
		t.Errorf("record error = %v, want the failure recorded", recErr)
	}
	if c := dbJobs(t, s)[j.ID]; c == nil || c.Status != "FAILED" {
		t.Errorf("committed job = %+v, want FAILED", c)
	}
	if got := s.Job(j.ID); got.Status != "FAILED" || got.Unrecorded {
		t.Errorf("in memory = %s unrecorded=%v, want FAILED and recorded", got.Status, got.Unrecorded)
	}
}

// A job that changed nothing keeps its completion when only its record fails, and
// says so; the next commit records it.
func TestJobs_UnchangedJobKeepsItsCompletionWhenOnlyTheRecordFails(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	be := s.be.(*dbBackend)
	j, err := s.NewJob("check", "Check")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.SaveGen()
	be.failCommit = func() error { return errors.New("disk full") }
	saveErr, recErr := s.CompleteJob(j.ID, gen, "Check", nil, nil)
	be.failCommit = nil
	if saveErr != nil || recErr == nil {
		t.Fatalf("CompleteJob = %v, %v; want only the record to fail", saveErr, recErr)
	}
	if got := s.Job(j.ID); got.Status != "COMPLETED" || !got.Unrecorded {
		t.Errorf("in memory = %s unrecorded=%v, want COMPLETED and not recorded", got.Status, got.Unrecorded)
	}
	if _, err := s.NewJob("check", "Check 2"); err != nil {
		t.Fatal(err)
	}
	if c := dbJobs(t, s)[j.ID]; c == nil || c.Status != "COMPLETED" {
		t.Errorf("committed = %+v, want COMPLETED", c)
	}
	if got := s.Job(j.ID); got.Unrecorded {
		t.Error("still marked not recorded after the next commit")
	}
}

// The first open on the database engine moves jobs.json's board into the database
// once and leaves the file as it was; later opens read only the database.
func TestJobs_TheBoardMovesFromJobsJSONOnce(t *testing.T) {
	dir := t.TempDir()
	js := openEngineStore(t, "json", dir)
	done, err := js.NewJob("scan", "Done")
	if err != nil {
		t.Fatal(err)
	}
	if err := js.FinishJob(done.ID, 1, "", "COMPLETED", nil, map[string]any{"n": 2}); err != nil {
		t.Fatal(err)
	}
	running, err := js.NewJob("scan", "Running")
	if err != nil {
		t.Fatal(err)
	}
	if err := js.Close(); err != nil {
		t.Fatal(err)
	}
	board := filepath.Join(dir, "jobs.json")
	raw, err := os.ReadFile(board)
	if err != nil {
		t.Fatal(err)
	}

	s := openEngineStore(t, "sqlite", dir)
	if got := s.Job(done.ID); got == nil || got.Status != "COMPLETED" || got.Result["n"] != float64(2) {
		t.Errorf("moved job = %+v, want COMPLETED with its result", got)
	}
	if got := s.Job(running.ID); got == nil || got.Status != "INTERRUPTED" {
		t.Errorf("moved running job = %+v, want INTERRUPTED", got)
	}
	committed := dbJobs(t, s)
	if len(committed) != 2 || committed[done.ID] == nil || committed[running.ID].Status != "INTERRUPTED" {
		t.Errorf("committed board = %v, want both jobs", committed)
	}
	if after, _ := os.ReadFile(board); string(after) != string(raw) {
		t.Error("the move changed jobs.json")
	}
	next, err := s.NewJob("scan", "Next")
	if err != nil {
		t.Fatal(err)
	}
	if next.ID != running.ID+1 {
		t.Errorf("next job ID = %d, want %d", next.ID, running.ID+1)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(board, []byte("not a board"), 0o644); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	defer s.Close()
	if got := s.Job(next.ID); got == nil || got.Status != "INTERRUPTED" {
		t.Errorf("after reopen = %+v, want the job read from the database", got)
	}
	if n := len(s.Jobs()); n != 3 {
		t.Errorf("board has %d jobs after reopen, want 3", n)
	}
}

// A damaged jobs.json stops the first open on the database engine, as it stops a
// catalog.json open, and is left as it is.
func TestJobs_ADamagedJobsJSONIsNotMoved(t *testing.T) {
	dir := t.TempDir()
	board := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(board, []byte(`{"next":1,"rows":[{"id":0}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(catalogEngineEnv, "sqlite")
	s, err := openTestStore(t, dir)
	if err == nil {
		s.Close()
		t.Fatal("the database engine opened with a damaged jobs.json")
	}
	if !strings.Contains(err.Error(), "jobs.json") {
		t.Errorf("error = %v, want it to name jobs.json", err)
	}
	if b, _ := os.ReadFile(board); string(b) != `{"next":1,"rows":[{"id":0}]}` {
		t.Error("the refused open changed jobs.json")
	}
}

// A jobs.json the first open cannot read stops it with the cause, as on catalog.json.
func TestJobs_AnUnreadableJobsJSONIsNotMoved(t *testing.T) {
	for _, cause := range []error{os.ErrPermission, errors.New("synthetic I/O error")} {
		t.Run(cause.Error(), func(t *testing.T) {
			dir := t.TempDir()
			board := filepath.Join(dir, "jobs.json")
			if err := os.WriteFile(board, []byte(`{"next":1,"rows":[]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv(catalogEngineEnv, "sqlite")
			read := func(p string) ([]byte, error) {
				if p == board {
					return nil, cause
				}
				return os.ReadFile(p)
			}
			s, err := openStore(dir, read, nil)
			if s != nil || !errors.Is(err, cause) {
				if s != nil {
					s.Close()
				}
				t.Fatalf("open = %v, want the read failure", err)
			}
		})
	}
}

// The move keeps the catalog.json engine's rules for a board read on open: a
// running job is interrupted with its live figures cleared, a row in the file is
// recorded (its failure history kept), and the next ID continues past every row.
func TestJobs_TheMoveReconcilesTheBoard(t *testing.T) {
	dir := t.TempDir()
	raw := `{"next":0,"rows":[{"id":8,"status":"RUNNING","rate_mbps":3,"eta_seconds":4,"artifacts":[{"kind":"catalog","count":2}]},` +
		`{"id":3,"status":"FAILED","unrecorded":true,"persist_error":"synthetic historical failure"}]}`
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	defer s.Close()
	run, failed := s.Job(8), s.Job(3)
	if run == nil || run.Status != "INTERRUPTED" || run.RateMBps != 0 || run.ETASeconds != 0 || len(run.Artifacts) != 1 {
		t.Errorf("running job after the move = %+v", run)
	}
	if failed == nil || failed.Unrecorded || failed.PersistError != "synthetic historical failure" {
		t.Errorf("failed job after the move = %+v", failed)
	}
	if c := dbJobs(t, s); c[8] == nil || c[8].Status != "INTERRUPTED" || c[3] == nil {
		t.Errorf("committed board = %v", c)
	}
	if ids := s.Jobs(); len(ids) != 2 || ids[0].ID != 8 || ids[1].ID != 3 {
		t.Errorf("board order = %v, want 8 then 3", ids)
	}
	j, err := s.NewJob("scan", "next")
	if err != nil || j.ID != 9 {
		t.Errorf("next job = %v, %v; want ID 9", j, err)
	}
}

// The board's next ID is kept when it is past every job, as jobs.json's can be.
func TestJobs_TheNextIDIsKept(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(`{"next":7,"rows":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openEngineStore(t, "sqlite", dir)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openEngineStore(t, "sqlite", dir)
	defer s.Close()
	j, err := s.NewJob("scan", "Scan")
	if err != nil || j.ID != 8 {
		t.Fatalf("NewJob = %v, %v; want ID 8", j, err)
	}
}

// A job row that cannot be written fails the whole commit: the catalog changes
// in it are not saved either.
func TestJobs_AnUnwritableJobRowFailsTheWholeCommit(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	j, err := s.NewJob("scan", "Scan A")
	if err != nil {
		t.Fatal(err)
	}
	gen := s.SaveGen()
	holdBatch(s)
	before := dbCount(t, s, "collection")
	s.AddCollection("A")
	_, recErr := s.CompleteJob(j.ID, gen, "Scan A", nil, map[string]any{"rate": math.NaN()})
	if recErr == nil {
		t.Fatal("a job row JSON cannot hold was reported written")
	}
	if dbCount(t, s, "collection") != before {
		t.Error("the catalog change was committed without the job's row")
	}
	if got := s.Job(j.ID); got.Status == "COMPLETED" || !got.Unrecorded {
		t.Errorf("job = %s unrecorded=%v, want FAILED and not recorded", got.Status, got.Unrecorded)
	}
}

// A catalog database made before the job table existed gets it on open.
func TestJobs_AnOlderDatabaseGetsTheJobTable(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DROP TABLE job`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	s = openEngineStore(t, "sqlite", dir)
	defer s.Close()
	j, err := s.NewJob("scan", "Scan")
	if err != nil {
		t.Fatal(err)
	}
	if c := dbJobs(t, s)[j.ID]; c == nil {
		t.Error("the job was not committed")
	}
}

// A catalog database a newer version wrote opens read-only, and its jobs stay in
// jobs.json, which this version can still write.
func TestJobs_ReadOnlyDatabaseKeepsJobsInJobsJSON(t *testing.T) {
	dir := t.TempDir()
	s := openEngineStore(t, "sqlite", dir)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE meta SET value='2' WHERE key='schema_version'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	d.Close()
	s = openEngineStore(t, "sqlite", dir)
	defer s.Close()
	if ro, _ := s.ReadOnly(); !ro {
		t.Fatal("setup: the newer database did not open read-only")
	}
	j, err := s.NewJob("check", "Check")
	if err != nil {
		t.Fatalf("NewJob on a read-only database: %v", err)
	}
	board, _, err := readJobBoard(filepath.Join(dir, "jobs.json"), os.ReadFile)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range board.Rows {
		found = found || r.ID == j.ID
	}
	if !found {
		t.Error("the job was not recorded in jobs.json")
	}
	// A job that changed the catalog completes with a note that its changes were
	// not recorded.
	gen := s.SaveGen()
	s.Log("test", "a change")
	if saveErr, recErr := s.CompleteJob(j.ID, gen, "Check", nil, nil); saveErr != nil || recErr != nil {
		t.Fatalf("CompleteJob = %v, %v", saveErr, recErr)
	}
	if got := s.Job(j.ID); got.Status != "COMPLETED" || !strings.Contains(fmt.Sprint(got.Result["catalog_not_recorded"]), "read-only") {
		t.Errorf("job = %s %v, want COMPLETED with a read-only note", got.Status, got.Result)
	}
}

// On the database engine the audit log is not held in memory: entries are added to
// the open transaction, the last one is read back, and one whose commit failed is
// written by the next.
func TestAudit_DatabaseEngineAppendsEntriesWithoutHoldingThem(t *testing.T) {
	s := openEngineStore(t, "sqlite", t.TempDir())
	defer s.Close()
	be := s.be.(*dbBackend)
	before := dbCount(t, s, "audit")
	s.Log("test", "one")
	s.Log("test", "two")
	s.mu.Lock()
	held := len(s.c.Audit)
	s.mu.Unlock()
	if held != 0 {
		t.Errorf("%d audit entries held in memory, want none", held)
	}
	if la := s.LastAudit(); la == nil || la.Detail != "two" {
		t.Errorf("LastAudit = %+v, want two", la)
	}
	be.failCommit = func() error { return errors.New("disk full") }
	s.Log("test", "three")
	be.failCommit = nil
	if la := s.LastAudit(); la == nil || la.Detail != "three" {
		t.Errorf("LastAudit after a failed commit = %+v, want three", la)
	}
	s.Log("test", "four")
	if got := dbCount(t, s, "audit") - before; got != 4 {
		t.Errorf("%d new audit rows committed, want 4", got)
	}
	var details []string
	if err := be.d.Read(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT detail FROM audit WHERE action = 'test' ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				return err
			}
			details = append(details, d)
		}
		return rows.Err()
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(details, ",") != "one,two,three,four" {
		t.Errorf("committed entries = %v, want one,two,three,four in order", details)
	}
}

// The audit log reads the same on both engines after a reopen, and the portable
// export of the database carries every entry.
func TestAudit_SameOnBothEnginesAfterReopen(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			dir := t.TempDir()
			s := openEngineStore(t, engine, dir)
			s.Log("test", "first")
			s.Log("test", "last")
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s = openEngineStore(t, engine, dir)
			defer s.Close()
			if la := s.LastAudit(); la == nil || la.Action != "test" || la.Detail != "last" {
				t.Errorf("LastAudit = %+v, want last", la)
			}
			if engine != "sqlite" {
				return
			}
			if n := len(s.c.Audit); n != 0 {
				t.Errorf("the reopened store holds %d audit entries in memory, want none", n)
			}
			c, err := exportCatalogDB(s.be.(*dbBackend).d)
			if err != nil {
				t.Fatal(err)
			}
			var details []string
			for _, a := range c.Audit {
				if a.Action == "test" {
					details = append(details, a.Detail)
				}
			}
			if strings.Join(details, ",") != "first,last" {
				t.Errorf("exported entries = %v, want first,last", details)
			}
		})
	}
}

// Whatever the engine, a job whose catalog changes could not be saved is never
// reported or recorded as COMPLETED.
func TestJobs_NoJobReportsSuccessForUnsavedWork(t *testing.T) {
	for _, engine := range bothEngines {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			app, _ := newTestApp(t, map[string]string{})
			s := app.Store
			resp, err := runJob(app, "test", "Change the catalog", func(func(float64, string)) (map[string]any, error) {
				if engine == "sqlite" {
					s.be.(*dbBackend).failCommit = func() error { return errors.New("disk full") }
				} else {
					s.failSave = func() error { return errors.New("disk full") }
				}
				s.AddCollection("A")
				return map[string]any{}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			id := resp["job_id"].(int)
			got := waitTerminalJob(t, s, id)
			if engine == "sqlite" {
				s.be.(*dbBackend).failCommit = nil
			} else {
				s.failSave = nil
			}
			if got.Status == "COMPLETED" {
				t.Errorf("the job reports COMPLETED for work that was not saved: %+v", got)
			}
			var stored *Job
			if engine == "sqlite" {
				stored = dbJobs(t, s)[id]
			} else {
				stored = jobOnDisk(t, app.DataDir, id)
			}
			if stored != nil && stored.Status == "COMPLETED" {
				t.Errorf("the job is recorded COMPLETED for work that was not saved")
			}
		})
	}
}
