package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// store_jobs.go — how a job's end is published, and the job board on the catalog
// database engine.
//
// On catalog.json the board is the jobs.json sidecar, a second file written after
// the catalog. On the database engine the board is the job table in catalog.db,
// and every write of it is a commit of the catalog too: the job rows that changed
// go in the same transaction as every catalog change not yet committed. So a job's
// end and its changes are saved together or not at all, and a job is "not
// recorded" exactly when that commit failed. A database a newer version wrote is
// read-only, and its board stays in jobs.json.

// storedJob is a job row as the catalog database keeps it: the job whole as JSON.
type storedJob struct {
	ID  int
	Doc string
}

// Meta keys of the job board in the catalog database.
const (
	metaJobNext  = "job_next"   // the last job ID handed out
	metaJobMoved = "jobs_moved" // when jobs.json's board was moved in; set once
)

// jobsInDB reports whether the job board lives in the catalog database.
func (s *Store) jobsInDB() bool { return s.jobs.inDB }

// encodeStoredJob is j's row. A stored row never says it is not recorded.
func encodeStoredJob(j *Job) (string, error) {
	cp := *j
	cp.Unrecorded = false
	b, err := json.Marshal(&cp)
	if err != nil {
		return "", fmt.Errorf("job %d: %w", j.ID, err)
	}
	return string(b), nil
}

// decodeStoredJob reads a job row the catalog database stores.
func decodeStoredJob(id int, doc string) (*Job, error) {
	var j Job
	if err := json.Unmarshal([]byte(doc), &j); err != nil {
		return nil, fmt.Errorf("job %d: %w", id, err)
	}
	if j.ID != id {
		return nil, fmt.Errorf("job %d is stored under ID %d", j.ID, id)
	}
	return &j, nil
}

// CompleteJob publishes the successful end of job id. When the job changed the
// catalog (a save was asked for after gen and a change is not saved yet), its
// changes must be saved before the job may say COMPLETED: if they cannot be, or a
// change was lost while it ran (maybe its own), the job is published FAILED with
// the reason, and saveErr is that reason. recordErr
// is the error of writing the job's own record; the job then also says it is not
// recorded. label is the job's label without progress text.
//
// On the database engine the completion and the changes are one commit. When it
// fails, nothing of it is saved: a job that changed the catalog is FAILED, one
// that did not stays COMPLETED, and both say they are not recorded. The changes
// stay in memory and a later commit writes them, as for any failed commit.
func (s *Store) CompleteJob(id int, gen uint64, label string, arts []Artifact, res map[string]any) (saveErr, recordErr error) {
	if s.jobsInDB() {
		return s.completeJobDB(id, gen, label, arts, res)
	}
	s.mu.Lock()
	lost := s.lostSinceLocked(gen)
	s.mu.Unlock()
	if lost != nil {
		// A change was lost while the job ran, maybe its own.
		saveErr = workNotRecorded(lost)
		return saveErr, s.FinishJob(id, -1, label+" — ERROR: "+storedErrorText(saveErr.Error()), "FAILED", nil, nil)
	}
	if s.ChangedSince(gen) {
		if ro, why := s.ReadOnly(); ro {
			if res == nil {
				res = map[string]any{}
			}
			res["catalog_not_recorded"] = "the catalog is read-only (" + why + "), so this job's changes to it were not recorded"
		} else if ferr := s.FlushPending(); ferr != nil {
			saveErr = workNotRecorded(ferr)
			return saveErr, s.FinishJob(id, -1, label+" — ERROR: "+storedErrorText(saveErr.Error()), "FAILED", nil, nil)
		}
	}
	return nil, s.FinishJob(id, 1, "", "COMPLETED", arts, res)
}

func (s *Store) completeJobDB(id int, gen uint64, label string, arts []Artifact, res map[string]any) (saveErr, recordErr error) {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if s.jobs.loadErr != nil {
		return nil, fmt.Errorf("job board unavailable after refused load: %w", s.jobs.loadErr)
	}
	var row *Job
	for _, j := range s.jobs.rows {
		if j.ID == id {
			row = j
		}
	}
	if row == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.saveGen > gen && s.dirty
	now := time.Now().UTC()
	row.RateMBps, row.ETASeconds, row.FinishedAt = 0, 0, &now
	fail := func(cause error) {
		saveErr = workNotRecorded(cause)
		row.Status, row.Label = "FAILED", label+" — ERROR: "+storedErrorText(saveErr.Error())
	}
	decided := false
	decide := func(unitErr error) {
		decided = true
		// A change lost while the job ran may be its own, so it fails the job even
		// when nothing of the job is left to commit.
		cause := s.lostSinceLocked(gen)
		if changed {
			cause = errors.Join(unitErr, s.lostErr)
		}
		if cause != nil {
			fail(cause)
			return
		}
		row.Progress, row.Status = 1, "COMPLETED"
		if len(arts) > 0 {
			row.Artifacts = arts
		}
		if res != nil {
			row.Result = res
		}
	}
	_, recordErr = s.commitBoardLocked(decide)
	if recordErr != nil {
		if !decided {
			decide(nil)
		}
		if changed && saveErr == nil {
			fail(recordErr)
		}
		markJobUnrecordedLocked(row, recordErr)
	}
	return saveErr, recordErr
}

// workNotRecorded is the error of a job whose work finished but whose catalog
// changes could not be saved.
func workNotRecorded(cause error) error {
	return fmt.Errorf("the work finished but the catalog could not be saved, so it is NOT recorded: %w", cause)
}

// commitJobsLocked commits the board's changed rows with the catalog's pending
// changes (commitBoardLocked). Caller holds s.jobs.mu.
func (s *Store) commitJobsLocked(decide func(unitErr error)) (unitErr, commitErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitBoardLocked(decide)
}

// commitBoardLocked writes every job row that differs from its committed row, and
// the board's next ID, in one commit with every catalog change not yet committed.
// decide, when not nil, runs in that transaction once the catalog's records are
// written, with the error of any the database refused, and may change rows before
// they are written. A commit that succeeds records every row, so none is left
// marked not recorded. commitErr means nothing was saved. Caller holds s.jobs.mu
// and s.mu.
func (s *Store) commitBoardLocked(decide func(unitErr error)) (unitErr, commitErr error) {
	if s.failSaveJobs != nil {
		if err := s.failSaveJobs(); err != nil {
			return nil, err
		}
	}
	if err := s.writeGateLocked(); err != nil {
		return nil, err
	}
	var written map[int]string
	unitErr, commitErr = s.commitDBLocked(func(uerr error) ([]storedJob, map[string]string, error) {
		if decide != nil {
			decide(uerr)
		}
		written = map[int]string{}
		var rows []storedJob
		for _, j := range s.jobs.rows {
			doc, err := encodeStoredJob(j)
			if err != nil {
				return nil, nil, err
			}
			if s.jobs.stored[j.ID] != doc {
				rows = append(rows, storedJob{j.ID, doc})
				written[j.ID] = doc
			}
		}
		meta := map[string]string{}
		if s.jobs.next != s.jobs.storedNext {
			meta[metaJobNext] = strconv.Itoa(s.jobs.next)
		}
		if s.jobs.moving {
			meta[metaJobMoved] = time.Now().UTC().Format(time.RFC3339Nano)
		}
		return rows, meta, nil
	})
	if commitErr != nil {
		return unitErr, commitErr
	}
	if s.jobs.stored == nil {
		s.jobs.stored = map[int]string{}
	}
	for id, doc := range written {
		s.jobs.stored[id] = doc
	}
	s.jobs.storedNext, s.jobs.moving, s.jobs.loaded = s.jobs.next, false, true
	for _, j := range s.jobs.rows {
		j.Unrecorded = false
	}
	return unitErr, nil
}

// loadJobsDB reads the job board from the catalog database. On the first open of
// a data folder on this engine it moves jobs.json's board in (read with read and
// left as it is), refusing a damaged one as the catalog.json engine does; later
// opens never read jobs.json again. A job left RUNNING is INTERRUPTED, as on
// catalog.json, and the result is committed when it can be.
func (s *Store) loadJobsDB(read func(string) ([]byte, error)) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	refuse := func(err error) error {
		s.jobs.loadErr = err
		return err
	}
	stored, meta, err := s.be.jobsLoad()
	if err != nil {
		return refuse(fmt.Errorf("reading the job board: %w", err))
	}
	s.jobs.stored = make(map[int]string, len(stored))
	rows := make([]*Job, 0, len(stored))
	next := 0
	for _, r := range stored {
		j, err := decodeStoredJob(r.ID, r.Doc)
		if err != nil || j.ID <= 0 {
			return refuse(fmt.Errorf("the catalog database's job board: job %d: %v", r.ID, err))
		}
		s.jobs.stored[j.ID] = r.Doc
		rows = append(rows, j)
		next = max(next, j.ID)
	}
	if v, ok := meta[metaJobNext]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return refuse(fmt.Errorf("the catalog database's job board has next ID %q", v))
		}
		s.jobs.storedNext = n
		next = max(next, n)
	}
	s.jobs.moving = meta[metaJobMoved] == ""
	if s.jobs.moving {
		in, exists, err := readJobBoard(s.jobs.path, read)
		if err != nil {
			return refuse(err)
		}
		if exists {
			for _, j := range in.Rows {
				if _, dup := s.jobs.stored[j.ID]; !dup {
					rows = append(rows, j)
				}
			}
			next = max(next, in.Next)
			sort.Slice(rows, func(a, b int) bool { return rows[a].ID < rows[b].ID })
		}
	}
	s.jobs.rows, s.jobs.next = rows, next
	s.jobs.loaded, s.jobs.loadErr = true, nil
	changed := s.jobs.moving
	for _, j := range rows {
		j.Unrecorded = false
		if j.Status == "RUNNING" {
			j.Status = "INTERRUPTED"
			j.RateMBps, j.ETASeconds = 0, 0
			changed = true
		}
	}
	if changed {
		// As on catalog.json, the reconciliation (and the move) is worked out again on
		// the next open when it cannot be committed now.
		_, _ = s.commitJobsLocked(nil)
	}
	return nil
}

// recordFinishedJob adds a job that ran before the store was open (the first
// start's catalog upgrade) to the board, COMPLETED, with its result.
func (s *Store) recordFinishedJob(kind, label string, started time.Time, result map[string]any) error {
	j, err := s.NewJob(kind, label)
	if err != nil {
		return err
	}
	s.jobs.mu.Lock()
	j.CreatedAt = started
	s.jobs.mu.Unlock()
	return s.FinishJob(j.ID, 1, "", "COMPLETED", nil, result)
}
