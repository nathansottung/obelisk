package main

// durable_completion_test.go — OB-002 / PR-03: a completion acknowledgment must be
// true.
//
// Three concrete falsehoods existed on this path, and each test below names the one
// it pins:
//
//  1. EndBatch discarded its final catalog write error (`_ = s.writeCatalog()`), so a
//     batched job returned success for changes that never reached the disk.
//  2. EndBatch only wrote when batchDepth reached zero. batchDepth is shared by every
//     concurrent job, so a job finishing while ANOTHER job held a batch wrote nothing
//     and depended on that unrelated job flushing later.
//  3. saveJobs returned nothing and swallowed marshal, write and rename failures, so
//     "COMPLETED" could be an in-memory claim while jobs.json still said RUNNING.
//
// Everything here drives the real production paths — ScanFolder, runJob, Store —
// against disposable temp stores, and checks the BYTES on disk or a genuine reopen
// rather than an observer count or an in-memory field. The failure seams are the
// per-Store failSave / failSaveJobs fields, owned by the store under test, so the
// concurrency cases cannot leak a fault into another test.
//
// These tests share package-level state only through the stores they create; none
// call t.Parallel(), matching the convention in build_verify_test.go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- helpers ---------------------------------------------------------------

// dcSource writes a small disposable source tree and returns its root.
func dcSource(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	if len(names) == 0 {
		names = []string{"a.txt", "b.txt"}
	}
	for _, n := range names {
		p := filepath.Join(root, n)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("OB-002 "+n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// dcApp builds an App on a disposable catalog with one archive, and returns the
// archive id. Setup writes are allowed to succeed: every fault below is armed only
// after the fixture is legitimately in place, so a failure can never be setup noise.
func dcApp(t *testing.T) (*App, int) {
	t.Helper()
	app, _ := newTestApp(t, map[string]string{})
	coll := app.Store.AddCollection("OB-002")
	return app, coll.ID
}

// readJobsFile reads the jobs.json sidecar as it exists ON DISK. The point of most of
// these assertions is the difference between this and Store.Jobs().
func readJobsFile(t *testing.T, dataDir string) []*Job {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dataDir, "jobs.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read jobs.json: %v", err)
	}
	var in struct {
		Next int    `json:"next"`
		Rows []*Job `json:"rows"`
	}
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatalf("parse jobs.json: %v", err)
	}
	return in.Rows
}

func jobOnDisk(t *testing.T, dataDir string, id int) *Job {
	t.Helper()
	for _, j := range readJobsFile(t, dataDir) {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// waitTerminalJob polls the real job board for a terminal state, bounded. runJob
// finishes in a goroutine, so there is no barrier to wait on from outside it; the
// bound keeps a hang from becoming a stuck suite.
func waitTerminalJob(t *testing.T, s *Store, id int) *Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if j := s.Job(id); j != nil {
			switch j.Status {
			case "COMPLETED", "FAILED", "INTERRUPTED":
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %d never reached a terminal state", id)
	return nil
}

// ---- A. final catalog-flush failure is reported ----------------------------

// TestDurableCompletion_A_FinalFlushFailureIsReported: batched work that completes but
// whose final catalog write fails must be reported as a failure, and nothing may
// present it as committed. Before OB-002, EndBatch dropped this error and ScanFolder
// returned a file count with err == nil.
func TestDurableCompletion_A_FinalFlushFailureIsReported(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t)

	// Fixture is fully in place and saved; only now does persistence start failing.
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }

	n, _, err := app.ScanFolder(cid, src, noProg)
	if err == nil {
		t.Fatalf("ScanFolder reported success (%d files) although the catalog could never be written", n)
	}
	if !strings.Contains(err.Error(), "catalog could not be saved") {
		t.Errorf("the error must say the catalog was not saved; got: %v", err)
	}
	if !strings.Contains(err.Error(), "NOT recorded") {
		t.Errorf("the error must say the work is not recorded; got: %v", err)
	}

	// And a reopened catalog must not show the scan: the write never landed.
	app.Store.failSave = nil
	store2, oerr := OpenStore(app.DataDir)
	if oerr != nil {
		t.Fatalf("reopen: %v", oerr)
	}
	if got := len(store2.AllFiles()); got != 0 {
		t.Errorf("reopened catalog holds %d file(s); the failed flush must have recorded none", got)
	}
}

// The same failure seen through the job board: the API must show FAILED, never an
// unqualified COMPLETED, for work whose catalog write did not land.
func TestDurableCompletion_A_FailedFlushJobIsNotCompleted(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t)
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }

	resp, jerr := runJob(app, "scan", "Scan "+src, func(p func(float64, string)) (map[string]any, error) {
		n, _, err := app.ScanFolder(cid, src, p)
		return map[string]any{"files": n}, err
	})
	if jerr != nil {
		t.Fatalf("the job itself was recorded fine; only the catalog write fails here: %v", jerr)
	}
	id := resp["job_id"].(int)
	j := waitTerminalJob(t, app.Store, id)
	if j.Status != "FAILED" {
		t.Fatalf("status = %q, want FAILED — a job whose catalog write failed must not read as completed", j.Status)
	}
	if !strings.Contains(j.Label, "catalog could not be saved") {
		t.Errorf("the job label must carry the persistence failure; got %q", j.Label)
	}
}

// ---- B. an operation error and a finalization error must not mask each other ----

// TestDurableCompletion_B_BothCausesSurvive drives endBatchInto — the shared
// finalization helper every one of the nine batch owners defers to — with a REAL
// dirty store whose writes fail.
//
// It is exercised directly rather than through one of those owners because none of
// them has a deterministically reachable operation error that occurs after the
// catalog has been dirtied (their post-BeginBatch failures all return before any
// mutation). Test A and test D cover the integrated path; this pins the one decision
// those cannot reach: what happens when BOTH causes exist at once.
func TestDurableCompletion_B_BothCausesSurvive(t *testing.T) {
	app, cid := dcApp(t)
	opErr := errors.New("the copy itself failed")

	// A genuinely dirty catalog inside a real batch, then a real write failure.
	app.Store.BeginBatch()
	app.Store.AddFolder(cid, t.TempDir())
	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }

	err := opErr
	endBatchInto(app.Store, &err)

	if err == nil {
		t.Fatal("both an operation error and a flush failure were present; the result must not be nil")
	}
	if !errors.Is(err, opErr) {
		t.Errorf("the original operation error was masked by the flush failure: %v", err)
	}
	if !strings.Contains(err.Error(), "disk full (injected)") {
		t.Errorf("the flush failure was discarded: %v", err)
	}

	// The mirror image: with no operation error, the flush failure must become one.
	app.Store.BeginBatch()
	app.Store.AddFolder(cid, t.TempDir())
	var only error
	endBatchInto(app.Store, &only)
	if only == nil || !strings.Contains(only.Error(), "disk full (injected)") {
		t.Errorf("a lone flush failure must be reported; got %v", only)
	}

	// Positive control: a clean flush must not invent an error, and must not disturb
	// an operation error that is already there.
	app.Store.failSave = nil
	app.Store.BeginBatch()
	app.Store.AddFolder(cid, t.TempDir())
	var clean error
	endBatchInto(app.Store, &clean)
	if clean != nil {
		t.Errorf("a successful flush must report nothing; got %v", clean)
	}
	app.Store.BeginBatch()
	app.Store.AddFolder(cid, t.TempDir())
	kept := opErr
	endBatchInto(app.Store, &kept)
	if kept != opErr {
		t.Errorf("a successful flush must leave the operation error exactly as it was; got %v", kept)
	}
}

// ---- C. catalog committed, terminal job record not ------------------------

// TestDurableCompletion_C_UnrecordedTerminalStateIsQualified: the catalog write
// succeeds, the jobs-sidecar terminal write fails. The work is real and must not be
// rolled back; what must not happen is the process presenting that completion as
// recorded history.
func TestDurableCompletion_C_UnrecordedTerminalStateIsQualified(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t)

	resp, jerr := runJob(app, "scan", "Scan "+src, func(p func(float64, string)) (map[string]any, error) {
		n, _, err := app.ScanFolder(cid, src, p)
		// Arm the sidecar failure from INSIDE the job: the initial NewJob write has
		// already succeeded, so this hits exactly the terminal write and nothing else.
		app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
		return map[string]any{"files": n}, err
	})
	if jerr != nil {
		t.Fatalf("the initial job record must still have been written: %v", jerr)
	}
	id := resp["job_id"].(int)
	j := waitTerminalJob(t, app.Store, id)

	// The work really did finish, so the status stays truthful about the work…
	if j.Status != "COMPLETED" {
		t.Fatalf("status = %q: the scan itself succeeded and must not be reported as failed", j.Status)
	}
	// …but it must be explicitly qualified as not recorded, in BOTH halves of the
	// contract: unrecorded says the current snapshot is not on disk, persist_error
	// says why. The label is deliberately NOT mutated any more — a mutated label
	// cannot be un-mutated when a later ordinary write records the row (see the
	// LaterSuccessfulWrite test), and appending to it twice would double the prefix.
	if !j.Unrecorded {
		t.Error("an in-memory COMPLETED whose record could not be written must be marked unrecorded")
	}
	if j.PersistError == "" {
		t.Error("an in-memory COMPLETED whose record could not be written must carry persist_error")
	}
	if strings.Contains(j.Label, "NOT RECORDED") {
		t.Errorf("the qualification belongs in the unrecorded field, not in a permanently mutated label; got %q", j.Label)
	}

	// The bytes on disk are the actual evidence: the sidecar still says RUNNING.
	onDisk := jobOnDisk(t, app.DataDir, id)
	if onDisk == nil {
		t.Fatal("the initial RUNNING record should still be on disk")
	}
	if onDisk.Status == "COMPLETED" {
		t.Error("jobs.json claims COMPLETED although every write of it failed")
	}
	if onDisk.PersistError != "" {
		t.Error("the failure must not have been written into the very sidecar that is failing")
	}

	// The catalog, a separate file, committed and must be left alone.
	app.Store.failSaveJobs = nil
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(store2.AllFiles()); got != 2 {
		t.Errorf("the catalog committed and must survive: got %d file(s), want 2", got)
	}
	// Restart reconciliation gives the conservative answer, not a fabricated success.
	if got := store2.Job(id); got == nil || got.Status != "INTERRUPTED" {
		t.Errorf("after a restart the unrecorded job must read INTERRUPTED; got %+v", got)
	}
}

// ---- D. overlapping batches ------------------------------------------------

// TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch is the shared-depth
// case. Job B holds a batch open across job A's entire lifetime. A's completion must
// be durable when A finishes — it may not depend on B flushing later.
//
// Deterministic barriers, no sleeps: B signals that its batch is open, then blocks
// until A has finished before ending its own.
func TestDurableCompletion_D_FinishingJobDoesNotWaitForAnotherBatch(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t, "only.txt")

	bOpen := make(chan struct{})
	aDone := make(chan struct{})
	bDone := make(chan error, 1)

	go func() { // job B: opens a batch, dirties the catalog, holds it open
		app.Store.BeginBatch()
		app.Store.AddFolder(cid, t.TempDir())
		close(bOpen)
		<-aDone
		bDone <- app.Store.EndBatch()
	}()

	select {
	case <-bOpen:
	case <-time.After(20 * time.Second):
		t.Fatal("job B never opened its batch")
	}

	// Job A runs entirely while B's batch is open.
	n, _, err := app.ScanFolder(cid, src, noProg)
	close(aDone)
	if err != nil {
		t.Fatalf("job A: %v", err)
	}
	if n != 1 {
		t.Fatalf("job A scanned %d files, want 1", n)
	}

	// A has returned success. Its work must ALREADY be on disk, with B's batch still
	// only just closing — read the catalog from disk, not from this Store.
	files := reopenFiles(t, app.DataDir)
	if len(files) != 1 {
		t.Fatalf("job A reported success but the catalog on disk holds %d file(s): "+
			"A's durability depended on job B's later flush", len(files))
	}

	select {
	case berr := <-bDone:
		if berr != nil {
			t.Fatalf("job B's own EndBatch: %v", berr)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("job B never finished")
	}
}

// The failure half of D: while another batch is open, a finishing job whose write
// fails must still learn about it. Under the old depth condition it wrote nothing and
// therefore could never fail — silence that looked like success.
func TestDurableCompletion_D_FlushFailureSurfacesWithAnotherBatchOpen(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t, "only.txt")

	bOpen := make(chan struct{})
	aDone := make(chan struct{})
	bDone := make(chan struct{})

	go func() {
		app.Store.BeginBatch()
		close(bOpen)
		<-aDone
		_ = app.Store.EndBatch()
		close(bDone)
	}()

	select {
	case <-bOpen:
	case <-time.After(20 * time.Second):
		t.Fatal("job B never opened its batch")
	}

	app.Store.failSave = func() error { return fmt.Errorf("disk full (injected)") }
	_, _, err := app.ScanFolder(cid, src, noProg)
	close(aDone)
	<-bDone

	if err == nil {
		t.Fatal("job A must report the failed flush even though another batch was open")
	}
	if !strings.Contains(err.Error(), "catalog could not be saved") {
		t.Errorf("got: %v", err)
	}
}

// reopenFiles opens the catalog fresh from disk and returns its files — proof about
// bytes, not about the in-memory Store that just claimed to have written them.
func reopenFiles(t *testing.T, dataDir string) []*File {
	t.Helper()
	s, err := OpenStore(dataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return s.AllFiles()
}

// ---- E. the all-succeed path still works end to end ------------------------

// TestDurableCompletion_E_SuccessfulJobIsRecorded: with every write succeeding, the
// catalog and the terminal job record must both be on disk, verified by reopening
// the real files. An observer counter would not prove this.
func TestDurableCompletion_E_SuccessfulJobIsRecorded(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t)

	resp, jerr := runJob(app, "scan", "Scan "+src, func(p func(float64, string)) (map[string]any, error) {
		n, _, err := app.ScanFolder(cid, src, p)
		if err != nil {
			return nil, err
		}
		return map[string]any{"files": n,
			"artifacts": []Artifact{{Kind: "catalog", Label: "2 files cataloged", Count: n}}}, nil
	})
	if jerr != nil {
		t.Fatalf("runJob: %v", jerr)
	}
	id := resp["job_id"].(int)
	if j := waitTerminalJob(t, app.Store, id); j.Status != "COMPLETED" || j.PersistError != "" || j.Unrecorded {
		t.Fatalf("in-memory job = %q persist_error=%q unrecorded=%v, want a clean COMPLETED", j.Status, j.PersistError, j.Unrecorded)
	}

	// Catalog: reopened from disk.
	if got := len(reopenFiles(t, app.DataDir)); got != 2 {
		t.Errorf("reopened catalog holds %d file(s), want 2", got)
	}
	// Jobs sidecar: read as bytes, and the terminal record must be complete —
	// status, result and artifacts together, not a status whose outputs never landed.
	onDisk := jobOnDisk(t, app.DataDir, id)
	if onDisk == nil {
		t.Fatal("the completed job is absent from jobs.json")
	}
	if onDisk.Status != "COMPLETED" {
		t.Errorf("jobs.json status = %q, want COMPLETED", onDisk.Status)
	}
	if onDisk.Unrecorded {
		t.Error("a cleanly recorded job must never be written to disk marked unrecorded")
	}
	if onDisk.PersistError != "" {
		t.Errorf("a cleanly recorded job must carry no persist_error; got %q", onDisk.PersistError)
	}
	if onDisk.Result == nil || onDisk.Result["files"] == nil {
		t.Errorf("the terminal record must carry the job's result; got %+v", onDisk.Result)
	}
	if len(onDisk.Artifacts) != 1 {
		t.Errorf("the terminal record must carry the job's artifacts; got %+v", onDisk.Artifacts)
	}
	// And a genuine reopen agrees.
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := store2.Job(id); got == nil || got.Status != "COMPLETED" {
		t.Errorf("after restart the completed job must still read COMPLETED; got %+v", got)
	}
}

// ---- F. a job that cannot be recorded must not start -----------------------

// TestDurableCompletion_F_UnrecordableJobStartsNoWork: when the initial job record
// cannot be written, no work runs and no ID is handed out. Previously NewJob ignored
// the failure and returned an ID the system described as recorded.
func TestDurableCompletion_F_UnrecordableJobStartsNoWork(t *testing.T) {
	app, cid := dcApp(t)
	src := dcSource(t)
	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }

	ran := make(chan struct{})
	resp, jerr := runJob(app, "scan", "Scan "+src, func(p func(float64, string)) (map[string]any, error) {
		close(ran) // must never happen
		n, _, err := app.ScanFolder(cid, src, p)
		return map[string]any{"files": n}, err
	})
	if jerr == nil {
		t.Fatal("runJob must refuse when the job board cannot record the job")
	}
	if resp != nil {
		t.Errorf("no job response may be handed back; got %+v", resp)
	}
	if !strings.Contains(jerr.Error(), "no work was started") {
		t.Errorf("the error must say no work was started; got: %v", jerr)
	}
	select {
	case <-ran:
		t.Fatal("work was launched under a job ID that was never recorded")
	case <-time.After(150 * time.Millisecond):
	}
	if got := len(app.Store.Jobs()); got != 0 {
		t.Errorf("the rolled-back job must leave no row; got %d", got)
	}

	// The failed attempt must not have burned an ID: the next job is still #1.
	app.Store.failSaveJobs = nil
	j, err := app.Store.NewJob("scan", "second attempt")
	if err != nil {
		t.Fatalf("NewJob after the fault is cleared: %v", err)
	}
	if j.ID != 1 {
		t.Errorf("next job id = %d, want 1 — the refused attempt consumed an ID", j.ID)
	}
	if jobOnDisk(t, app.DataDir, j.ID) == nil {
		t.Error("the successfully created job must be on disk")
	}
}

// ---- G. batch bookkeeping stays sound --------------------------------------

// TestDurableCompletion_G_BatchDepthBookkeeping: nesting, early returns and an
// unbalanced EndBatch must leave no leaked depth and lose no final write.
func TestDurableCompletion_G_BatchDepthBookkeeping(t *testing.T) {
	app, cid := dcApp(t)

	// Nested begin/end: depth returns to zero and the work is flushed.
	app.Store.BeginBatch()
	app.Store.BeginBatch()
	app.Store.AddFolder(cid, t.TempDir())
	if err := app.Store.EndBatch(); err != nil { // inner
		t.Fatalf("inner EndBatch: %v", err)
	}
	if err := app.Store.EndBatch(); err != nil { // outer
		t.Fatalf("outer EndBatch: %v", err)
	}
	app.Store.mu.Lock()
	depth, dirty := app.Store.batchDepth, app.Store.dirty
	app.Store.mu.Unlock()
	if depth != 0 {
		t.Errorf("batchDepth leaked: %d", depth)
	}
	if dirty {
		t.Error("the catalog is still dirty after the batch closed")
	}

	// An extra EndBatch must not drive the depth negative.
	if err := app.Store.EndBatch(); err != nil {
		t.Fatalf("unbalanced EndBatch: %v", err)
	}
	app.Store.mu.Lock()
	depth = app.Store.batchDepth
	app.Store.mu.Unlock()
	if depth != 0 {
		t.Errorf("batchDepth went to %d on an unbalanced EndBatch", depth)
	}

	// An early return inside a batch owner still flushes: ScanFolder's deferred
	// endBatchInto runs on every exit path.
	src := dcSource(t, "x.txt")
	if _, _, err := app.ScanFolder(cid, src, noProg); err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	app.Store.mu.Lock()
	depth = app.Store.batchDepth
	app.Store.mu.Unlock()
	if depth != 0 {
		t.Errorf("ScanFolder leaked batch depth: %d", depth)
	}
	if got := len(reopenFiles(t, app.DataDir)); got != 1 {
		t.Errorf("reopened catalog holds %d file(s), want 1", got)
	}
}

// TestDurableCompletion_SaveJobsReportsFailure is the direct unit-level control for
// the seam the tests above depend on: saveJobs must actually return its error, and
// must leave the previous good sidecar in place when publication fails.
func TestDurableCompletion_SaveJobsReportsFailure(t *testing.T) {
	app, _ := dcApp(t)
	good, err := app.Store.NewJob("scan", "first")
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	if err := app.Store.SetJob(good.ID, 1, "", "COMPLETED"); err != nil {
		t.Fatalf("terminal SetJob: %v", err)
	}
	before := jobOnDisk(t, app.DataDir, good.ID)
	if before == nil || before.Status != "COMPLETED" {
		t.Fatalf("fixture: the first job should be recorded COMPLETED; got %+v", before)
	}

	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	if err := app.Store.SetJob(good.ID, 1, "relabelled", "FAILED"); err == nil {
		t.Error("SetJob must return the sidecar write failure on a terminal transition")
	}
	// The previous good record survives an unsuccessful publication.
	after := jobOnDisk(t, app.DataDir, good.ID)
	if after == nil || after.Status != "COMPLETED" {
		t.Errorf("a failed write must leave the previous good record intact; got %+v", after)
	}
}
