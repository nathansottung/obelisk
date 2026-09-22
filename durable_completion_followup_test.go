//go:build !guionly

package main

// durable_completion_followup_test.go — OB-002 / PR-03 follow-up: the recording-state
// contract, at the boundaries the first round of tests did not reach.
//
// The three blockers these pin, each named by the test that would fail without the fix:
//
//  1. FinishJob published COMPLETED and released s.jobs.mu, and the qualification was
//     set by a SEPARATE later acquisition of the same mutex. Between the two, GET
//     /api/jobs handed out a plain, unqualified COMPLETED for a job whose record on
//     disk still said RUNNING — the exact falsehood OB-002 exists to remove.
//     → _H_NoUnqualifiedCompletedIsObservable, _H_ConcurrentReaderNeverSeesUnqualified
//  2. No in-tree consumer read the field, so the UI rendered a green VERIFIED stamp
//     and waitJob resolved as clean success.
//     → durable_completion_ui_test.go
//  3. "Recorded" and "a recording failure once happened" were the same string, so a
//     job that a later ordinary save legitimately recorded went on saying NOT RECORDED
//     forever.
//     → _I_LaterSuccessfulWriteRecords, _I_LaterFailedWriteDoesNotRecord
//
// Same discipline as durable_completion_test.go: disposable temp stores, per-Store
// fault seams, bytes on disk or a genuine OpenStore reopen as the evidence, and
// channel barriers rather than sleeps wherever ordering is the thing being asserted.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- helpers ---------------------------------------------------------------

// dcJobsAPI stands up the real /api/jobs routes over a disposable app, so the reader
// under test is the actual handler chain and the actual JSON encoder — not Store.Jobs
// with a test-shaped wrapper around it.
func dcJobsAPI(t *testing.T, app *App) func(path string) []map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api(mux, app)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return func(path string) []map[string]any {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var rows []map[string]any
		if err := json.Unmarshal(b, &rows); err == nil {
			return rows
		}
		var one map[string]any
		if err := json.Unmarshal(b, &one); err != nil {
			t.Fatalf("GET %s: cannot parse %q: %v", path, string(b), err)
		}
		return []map[string]any{one}
	}
}

// apiRow picks one job out of an /api/jobs payload.
func apiRow(rows []map[string]any, id int) map[string]any {
	for _, r := range rows {
		if n, ok := r["id"].(float64); ok && int(n) == id {
			return r
		}
	}
	return nil
}

// unqualifiedCompleted is the single forbidden observation: a terminal COMPLETED with
// no recording qualification on it, for a job whose record never landed.
func unqualifiedCompleted(row map[string]any) bool {
	if row == nil {
		return false
	}
	if row["status"] != "COMPLETED" {
		return false
	}
	unrec, _ := row["unrecorded"].(bool)
	return !unrec
}

// dcJob creates a job row on a disposable store the ordinary way (through NewJob, so
// the initial RUNNING record is genuinely on disk) and returns its ID.
func dcJob(t *testing.T, s *Store, label string) int {
	t.Helper()
	j, err := s.NewJob("scan", label)
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	return j.ID
}

// ---- H. the visibility gap (Blocker 1) -------------------------------------

// TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable is the tightest possible
// statement of Blocker 1: with NO intervening call of any kind, the read that happens
// on the very next line after FinishJob returns its error must already be qualified.
//
// Against the pre-follow-up implementation this fails, because PersistError was set by
// a later, separate NoteJobUnrecorded call that had not happened yet at this point.
func TestDurableCompletion_H_NoUnqualifiedCompletedIsObservable(t *testing.T) {
	app, _ := dcApp(t)
	id := dcJob(t, app.Store, "probe")
	get := dcJobsAPI(t, app)

	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	err := app.Store.FinishJob(id, 1, "", "COMPLETED", nil, map[string]any{"files": 2})
	if err == nil {
		t.Fatal("FinishJob must report the failed terminal write")
	}

	// No call between the failed write and these reads. Store first, then the real
	// HTTP path, because a difference between the two would be its own bug.
	j := app.Store.Job(id)
	if j == nil {
		t.Fatal("job vanished")
	}
	if j.Status != "COMPLETED" {
		t.Fatalf("status = %q: the work finished and must stay truthful about that", j.Status)
	}
	if !j.Unrecorded {
		t.Error("Store.Job served an unqualified COMPLETED after the terminal write had already failed")
	}
	if j.PersistError == "" {
		t.Error("the cause of the recording failure must be published with the status")
	}
	if row := apiRow(get("/api/jobs"), id); unqualifiedCompleted(row) {
		t.Errorf("GET /api/jobs served an unqualified COMPLETED: %v", row)
	}
	if row := apiRow(get(fmt.Sprintf("/api/jobs/%d", id)), id); unqualifiedCompleted(row) {
		t.Errorf("GET /api/jobs/{id} served an unqualified COMPLETED: %v", row)
	} else if row["persist_error"] == nil {
		t.Error("the individual-job endpoint must carry the same qualification as the list")
	}

	// And the bytes are the control: nothing was written, so disk still says RUNNING.
	if onDisk := jobOnDisk(t, app.DataDir, id); onDisk == nil || onDisk.Status != "RUNNING" {
		t.Errorf("jobs.json must still hold the RUNNING record; got %+v", onDisk)
	}
}

// TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified holds the real save
// boundary open and reads through the real API path across it.
//
// The barrier makes the interleaving deterministic rather than hoped-for: the readers
// are running before the write begins and keep running until after FinishJob has
// returned, and the writer is provably parked inside the save boundary in between.
// Every observation is collected and every one is checked. A reader is allowed to
// block, and is allowed to see the prior safe RUNNING snapshot; what it may never see
// is COMPLETED without the qualification.
func TestDurableCompletion_H_ConcurrentReaderNeverSeesUnqualified(t *testing.T) {
	app, _ := dcApp(t)
	id := dcJob(t, app.Store, "probe")
	get := dcJobsAPI(t, app)

	entered := make(chan struct{}) // the save boundary has been reached
	release := make(chan struct{}) // …and may now fail
	app.Store.failSaveJobs = func() error {
		close(entered)
		<-release
		return fmt.Errorf("jobs.json unwritable (injected)")
	}

	stop := make(chan struct{})
	var mu sync.Mutex
	var bad []map[string]any
	var seen int
	var readers sync.WaitGroup
	for i := 0; i < 3; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				row := apiRow(get("/api/jobs"), id)
				mu.Lock()
				seen++
				if unqualifiedCompleted(row) {
					bad = append(bad, row)
				}
				mu.Unlock()
			}
		}()
	}

	done := make(chan error, 1)
	go func() { done <- app.Store.FinishJob(id, 1, "", "COMPLETED", nil, map[string]any{"files": 2}) }()

	<-entered      // the writer is inside the save; readers are blocked on s.jobs.mu
	close(release) // let it fail
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FinishJob must report the failed terminal write")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("FinishJob never returned — the qualification must not be waiting on a lock")
	}

	// Keep reading past the failure, then stop.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if row := apiRow(get("/api/jobs"), id); unqualifiedCompleted(row) {
			t.Errorf("after the failed write, GET /api/jobs served an unqualified COMPLETED: %v", row)
		}
	}
	close(stop)
	readers.Wait()

	mu.Lock()
	defer mu.Unlock()
	if seen == 0 {
		t.Fatal("the readers never observed the job at all — the test proved nothing")
	}
	if len(bad) > 0 {
		t.Errorf("%d of %d concurrent reads saw an unqualified COMPLETED, e.g. %v", len(bad), seen, bad[0])
	}
}

// ---- I. current recording state vs error history (Blocker 3) ---------------

// finishUnrecorded drives a job to a COMPLETED that could not be written, and returns
// its ID. The seam is disarmed on the way out, so the caller decides what happens next.
func finishUnrecorded(t *testing.T, app *App, label string) int {
	t.Helper()
	id := dcJob(t, app.Store, label)
	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	if err := app.Store.FinishJob(id, 1, "", "COMPLETED", []Artifact{{Kind: "catalog", Label: "2 records"}}, map[string]any{"files": 2}); err == nil {
		t.Fatal("FinishJob must report the failed terminal write")
	}
	app.Store.failSaveJobs = nil
	j := app.Store.Job(id)
	if j == nil || !j.Unrecorded || j.PersistError == "" {
		t.Fatalf("setup: the job must start out qualified as unrecorded; got %+v", j)
	}
	return id
}

// TestDurableCompletion_I_LaterSuccessfulWriteRecords is the review's step A-F.
//
// A's terminal write fails; an UNRELATED ordinary jobs update later saves a collection
// that contains A's current terminal snapshot. A is then genuinely recorded, and must
// say so — while keeping the earlier failure as history rather than as a standing
// warning about a record that is present.
func TestDurableCompletion_I_LaterSuccessfulWriteRecords(t *testing.T) {
	app, _ := dcApp(t)
	get := dcJobsAPI(t, app)
	id := finishUnrecorded(t, app, "Scan A")
	cause := app.Store.Job(id).PersistError

	// Before the later save: on disk, A is still the RUNNING row NewJob wrote.
	if onDisk := jobOnDisk(t, app.DataDir, id); onDisk == nil || onDisk.Status != "RUNNING" {
		t.Fatalf("precondition: jobs.json must still hold RUNNING; got %+v", onDisk)
	}

	// An unrelated ordinary jobs update. NewJob serialises the whole board, so the
	// bytes it writes contain A's current terminal snapshot.
	other := dcJob(t, app.Store, "Scan B")
	if other == id {
		t.Fatal("the second job must be a different job")
	}

	// (E) The bytes are the evidence, read straight from the file.
	onDisk := jobOnDisk(t, app.DataDir, id)
	if onDisk == nil {
		t.Fatal("A must be in jobs.json after the later save")
	}
	if onDisk.Status != "COMPLETED" {
		t.Errorf("jobs.json must hold A's terminal status; got %q", onDisk.Status)
	}
	if onDisk.Unrecorded {
		t.Error("a row that IS in the file must not be written claiming it is not recorded")
	}
	if onDisk.PersistError != cause {
		t.Errorf("the earlier failure must be kept as history; got %q want %q", onDisk.PersistError, cause)
	}
	if len(onDisk.Artifacts) != 1 || onDisk.Result == nil {
		t.Errorf("the recorded snapshot must carry the artifacts and result it was published with; got %+v", onDisk)
	}

	// The published in-memory state must agree with those bytes — not be optimistic
	// about them, and not lag behind them.
	j := app.Store.Job(id)
	if j == nil || j.Unrecorded {
		t.Errorf("in memory, A must now read as recorded; got %+v", j)
	}
	if j.PersistError != cause {
		t.Errorf("in memory, the earlier failure must be retained as history; got %q", j.PersistError)
	}
	row := apiRow(get("/api/jobs"), id)
	if row["unrecorded"] != nil {
		t.Errorf("the API must stop reporting a recorded job as unrecorded; got %v", row["unrecorded"])
	}
	if row["persist_error"] != cause {
		t.Errorf("the API must keep the failure history; got %v", row["persist_error"])
	}

	// (F) Restart reconstructs exactly what is in that file.
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	back := store2.Job(id)
	if back == nil {
		t.Fatal("A must survive the restart")
	}
	if back.Status != "COMPLETED" {
		t.Errorf("after restart A must read COMPLETED — its terminal snapshot was recorded; got %q", back.Status)
	}
	if back.Unrecorded {
		t.Error("after restart a recorded job must not claim to be unrecorded")
	}
	if back.PersistError != cause {
		t.Errorf("after restart the failure history must survive; got %q", back.PersistError)
	}
	if len(back.Artifacts) != 1 {
		t.Errorf("after restart the recorded artifacts must survive; got %+v", back.Artifacts)
	}
}

// TestDurableCompletion_I_LaterFailedWriteDoesNotRecord: a save that does not land
// cannot acknowledge anything. A's qualification must survive it unchanged.
func TestDurableCompletion_I_LaterFailedWriteDoesNotRecord(t *testing.T) {
	app, _ := dcApp(t)
	id := finishUnrecorded(t, app, "Scan A")
	cause := app.Store.Job(id).PersistError

	// A later jobs write that fails. NewJob rolls its own row back and returns the
	// error; what matters here is that A is not swept up in the optimistic clear.
	app.Store.failSaveJobs = func() error { return fmt.Errorf("still unwritable (injected)") }
	if _, err := app.Store.NewJob("scan", "Scan B"); err == nil {
		t.Fatal("NewJob must report the failed write")
	}
	app.Store.failSaveJobs = nil

	j := app.Store.Job(id)
	if j == nil || !j.Unrecorded {
		t.Errorf("a failed save must NOT mark A recorded; got %+v", j)
	}
	if j.PersistError != cause {
		t.Errorf("a failed save must not rewrite A's failure history; got %q want %q", j.PersistError, cause)
	}
	if onDisk := jobOnDisk(t, app.DataDir, id); onDisk == nil || onDisk.Status != "RUNNING" {
		t.Errorf("jobs.json must be untouched by the failed write; got %+v", onDisk)
	}

	// And once a write does land, the same row recovers — proving the failed attempt
	// left the recovery path intact rather than latching the flag permanently.
	if _, err := app.Store.NewJob("scan", "Scan C"); err != nil {
		t.Fatalf("the recovered write must succeed: %v", err)
	}
	if j := app.Store.Job(id); j == nil || j.Unrecorded {
		t.Errorf("an ordinary later save must record A; got %+v", j)
	}
}

// TestDurableCompletion_I_RestartWithoutRecoveryPublication is the other branch, and
// the one the patch's comments used to state unconditionally: when NO later write
// contains A's terminal result, a restart can only recover the older record that
// actually survived — the RUNNING row, reported as INTERRUPTED. Nothing may invent
// the terminal artifacts, the result, or a completion time that never existed.
func TestDurableCompletion_I_RestartWithoutRecoveryPublication(t *testing.T) {
	app, _ := dcApp(t)
	id := finishUnrecorded(t, app, "Scan A")

	// Deliberately no further jobs write of any kind before the reopen.
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	back := store2.Job(id)
	if back == nil {
		t.Fatal("the surviving RUNNING record must still be there")
	}
	if back.Status != "INTERRUPTED" {
		t.Errorf("with no recovery publication, restart must report the conservative state; got %q", back.Status)
	}
	if len(back.Artifacts) != 0 || back.Result != nil {
		t.Errorf("restart must not fabricate a result that was never recorded; got artifacts=%+v result=%+v", back.Artifacts, back.Result)
	}
	if back.FinishedAt != nil {
		t.Errorf("restart must not invent a completion time that was never established; got %v", back.FinishedAt)
	}
	if back.PersistError != "" {
		t.Errorf("the failure was never written into the failing sidecar, so it cannot come back out of it; got %q", back.PersistError)
	}
	if back.Unrecorded {
		t.Error("the recovered row IS the record on disk; it must not claim to be unrecorded")
	}
}

// ---- J. combined failure ---------------------------------------------------

// TestDurableCompletion_J_CombinedFailureStaysObservable fails the jobs sidecar AND
// the catalog behind the audit fallback — the realistic whole-volume failure.
//
// What must hold: the original persistence failure stays observable on the job, the
// call returns (no retry loop, no deadlock between s.jobs.mu and s.mu), and NOTHING
// claims a durable audit entry, because there is not one.
func TestDurableCompletion_J_CombinedFailureStaysObservable(t *testing.T) {
	app, _ := dcApp(t)
	id := dcJob(t, app.Store, "Scan A")

	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	app.Store.failSave = func() error { return fmt.Errorf("catalog.json unwritable (injected)") }

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := app.Store.FinishJob(id, 1, "", "COMPLETED", nil, map[string]any{"files": 2}); err != nil {
			// The production reporting path, with both files failing under it.
			app.noteUnrecordedJob(id, "COMPLETED", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the failure-reporting path hung — a retry loop or a lock-order deadlock")
	}

	j := app.Store.Job(id)
	if j == nil || !j.Unrecorded {
		t.Fatalf("the original problem must remain observable on the job; got %+v", j)
	}
	if j.PersistError == "" || j.PersistError == "catalog.json unwritable (injected)" {
		t.Errorf("the ORIGINAL persistence failure must be preserved, not replaced by the fallback's; got %q", j.PersistError)
	}

	// The audit fallback is best effort and this is what "best effort" cost here: with
	// the catalog unwritable there is no durable audit entry, and no code may say
	// otherwise. Read it back from a genuine reopen rather than from memory.
	app.Store.failSaveJobs = nil
	app.Store.failSave = nil
	store2, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if a := store2.LastAudit(); a != nil && a.Action == "job-unrecorded" {
		t.Error("no durable audit entry was written, so none may be read back — the fallback attempt is not evidence")
	}
	// The conservative durable answer is still the honest one.
	if back := store2.Job(id); back == nil || back.Status != "INTERRUPTED" {
		t.Errorf("with nothing recorded, restart must report INTERRUPTED; got %+v", back)
	}
}

// TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch is the coalescing case
// the audit comment now names: another job holds a batch open, so Store.Log's save()
// only marks the catalog dirty. The entry is not in the file, and the job's own
// qualification does not depend on it being there.
func TestDurableCompletion_J_UnrecordedSurvivesAConcurrentBatch(t *testing.T) {
	app, _ := dcApp(t)
	id := dcJob(t, app.Store, "Scan A")
	app.Store.BeginBatch() // another job's batch, deliberately left open

	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	err := app.Store.FinishJob(id, 1, "", "COMPLETED", nil, map[string]any{"files": 2})
	if err == nil {
		t.Fatal("FinishJob must report the failed terminal write")
	}
	app.noteUnrecordedJob(id, "COMPLETED", err)
	app.Store.failSaveJobs = nil

	if j := app.Store.Job(id); j == nil || !j.Unrecorded || j.PersistError == "" {
		t.Errorf("the qualification must not depend on the audit fallback landing; got %+v", j)
	}
	// The audit entry was appended in memory and coalesced into the open batch, so it
	// is not in the file yet. Assert the file, not the in-memory slice.
	b, rerr := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	if rerr != nil {
		t.Fatalf("read catalog.json: %v", rerr)
	}
	var cat struct {
		Audit []Audit `json:"audit"`
	}
	if err := json.Unmarshal(b, &cat); err != nil {
		t.Fatalf("parse catalog.json: %v", err)
	}
	for _, a := range cat.Audit {
		if a.Action == "job-unrecorded" {
			t.Error("with a batch open the audit save coalesces; an entry in the file here would mean the comment is wrong about which is which")
		}
	}
}

// TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded is the backend premise the UI
// status-precedence fix rests on, and the case no test at either level covered before.
//
// markJobUnrecordedLocked flags ANY terminal status whose write failed, and main.go
// finishes a failing job with FinishJob(…, "FAILED", nil, nil) — so one disk condition
// (an unreadable source, a full or unwritable volume) can genuinely produce BOTH halves
// at once: the operation failed, AND its failure record could not be saved.
//
// What this asserts is that the two facts stay SEPARATE and neither is upgraded into
// the other: the status stays FAILED, the operation's own error stays in the label, and
// the recording problem lives in unrecorded/persist_error. The corresponding UI rule —
// that the FAILED stamp survives and the recording failure is shown beside it rather
// than in place of it — is TestJobsUI_ExecutionOutcomeTakesPrecedence.
func TestDurableCompletion_K_FailedJobCanAlsoBeUnrecorded(t *testing.T) {
	app, _ := dcApp(t)
	id := dcJob(t, app.Store, "Scan the vault")

	app.Store.failSaveJobs = func() error { return fmt.Errorf("jobs.json unwritable (injected)") }
	// Exactly the shape main.go uses for a failing job: FAILED, no artifacts, no result.
	err := app.Store.FinishJob(id, 0, "Scan the vault — ERROR: source unreadable", "FAILED", nil, nil)
	if err == nil {
		t.Fatal("FinishJob must report the failed terminal write")
	}
	app.Store.failSaveJobs = nil

	j := app.Store.Job(id)
	if j == nil {
		t.Fatal("the job must still be readable")
	}
	// The execution outcome is the primary fact and is not displaced by the recording
	// failure. Nothing may promote this to COMPLETED.
	if j.Status != "FAILED" {
		t.Errorf("status = %q: a failed operation whose record also failed is still FAILED", j.Status)
	}
	// The operation's own error stays available, distinct from the recording cause.
	if !strings.Contains(j.Label, "source unreadable") {
		t.Errorf("the operation's own error must remain available; label = %q", j.Label)
	}
	// The recording problem is present, and as its own separate pair of fields.
	if !j.Unrecorded {
		t.Error("a FAILED job whose record could not be written must be marked unrecorded — this combination is reachable, not hypothetical")
	}
	if !strings.Contains(j.PersistError, "jobs.json unwritable") {
		t.Errorf("the recording cause must be kept separately from the operation error; persist_error = %q", j.PersistError)
	}
	if strings.Contains(j.PersistError, "source unreadable") {
		t.Error("the operation's error must not be copied into persist_error; the two causes are different facts")
	}
	// A failed job produced nothing, and nothing may invent otherwise.
	if len(j.Artifacts) != 0 || j.Result != nil {
		t.Errorf("a failed job must carry no artifacts and no result; got %d artifact(s), result %v", len(j.Artifacts), j.Result)
	}

	// The same two facts survive the real handler chain and JSON encoder, which is what
	// the UI actually reads — the UI cannot honour a distinction the API drops.
	var got map[string]any
	for _, row := range dcJobsAPI(t, app)("/api/jobs") {
		if int(row["id"].(float64)) == id {
			got = row
		}
	}
	if got == nil {
		t.Fatal("the job must be served by /api/jobs")
	}
	if got["status"] != "FAILED" || got["unrecorded"] != true {
		t.Errorf("the API must serve BOTH facts to the UI: status = %v, unrecorded = %v", got["status"], got["unrecorded"])
	}
	if lbl, _ := got["label"].(string); !strings.Contains(lbl, "source unreadable") {
		t.Errorf("the operation's own error must reach the UI too; label = %q", lbl)
	}
}
