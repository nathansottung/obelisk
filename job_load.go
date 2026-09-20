package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type jobBoard struct {
	Next int    `json:"next"`
	Rows []*Job `json:"rows"`
}

// jobField preserves the reader's historical single case alias, but refuses
// competing aliases for fields that determine board membership or job identity.
func jobField(fields map[string]json.RawMessage, name string) (json.RawMessage, error) {
	var found json.RawMessage
	for key, value := range fields {
		if strings.EqualFold(key, name) {
			if found != nil {
				return nil, fmt.Errorf("competing %s fields", name)
			}
			found = value
		}
	}
	return found, nil
}

func decodeJobBoard(b []byte) (jobBoard, error) {
	fail := func(reason string) (jobBoard, error) { return jobBoard{}, fmt.Errorf("invalid jobs.json: %s", reason) }
	if !utf8.Valid(b) {
		return fail("invalid UTF-8")
	}
	if err := uniqueJSON(b); err != nil {
		return fail("malformed JSON or duplicate fields")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil || fields == nil {
		return fail("expected an object")
	}
	for _, name := range []string{"next", "rows"} {
		value, err := jobField(fields, name)
		if err != nil {
			return fail(err.Error())
		}
		if name == "next" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fail("next must be an integer")
		}
	}
	var in jobBoard
	if err := json.Unmarshal(b, &in); err != nil {
		return fail("invalid field type or timestamp")
	}
	if in.Next < 0 {
		return fail("negative next counter")
	}
	rows, _ := jobField(fields, "rows")
	var rawRows []map[string]json.RawMessage
	if len(rows) > 0 {
		if err := json.Unmarshal(rows, &rawRows); err != nil {
			return fail("rows must contain objects")
		}
	}
	seen := make(map[int]bool, len(in.Rows))
	for i, j := range in.Rows {
		if j == nil {
			return fail(fmt.Sprintf("null row at index %d", i))
		}
		if _, err := jobField(rawRows[i], "id"); err != nil {
			return fail(fmt.Sprintf("ambiguous identity at row %d", i))
		}
		// NewJob has always allocated positive integer identities. Zero/missing,
		// negative and duplicate IDs cannot be addressed unambiguously by Job/SetJob.
		if j.ID <= 0 || seen[j.ID] {
			return fail(fmt.Sprintf("invalid or duplicate identity at row %d", i))
		}
		seen[j.ID] = true
		if j.ID > in.Next {
			in.Next = j.ID
		}
	}
	return in, nil
}

// readJobBoard treats an absent sidecar as optional first-use history. This is
// not proof of storage identity. Other read failures discard even partial bytes.
func readJobBoard(path string, read func(string) ([]byte, error)) (jobBoard, bool, error) {
	b, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return jobBoard{}, false, nil
	}
	if err != nil {
		return jobBoard{}, false, fmt.Errorf("cannot read jobs.json: %w", err)
	}
	in, err := decodeJobBoard(b)
	return in, true, err
}

func (s *Store) loadJobs() error { return s.loadJobsFrom(os.ReadFile) }

// Restore and migration keep the original Store when reopen fails. Validate
// without adopting or reconciling rows: workers may still own the old board. This does
// not make restore a cross-file transaction or coordinate external writers.
func (s *Store) checkJobsAfterFailedReopen() {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	_, exists, err := readJobBoard(s.jobs.path, os.ReadFile)
	if err == nil && !exists && s.jobs.loaded {
		err = fmt.Errorf("expected jobs.json is missing after failed reopen: %w", os.ErrNotExist)
	}
	if err != nil {
		s.jobs.loadErr = err
	}
}

// The lock spans read/validate/adopt so a failed reload retains the complete old
// board and counter. Its failure is latched before releasing the lock: even a
// caller ignoring the error cannot later save stale rows over rejected history.
func (s *Store) loadJobsFrom(read func(string) ([]byte, error)) error {
	s.jobs.mu.Lock()
	defer s.jobs.mu.Unlock()
	if s.jobs.path == "" {
		return nil
	}
	in, exists, err := readJobBoard(s.jobs.path, read)
	if err == nil && !exists && (s.jobs.loaded || s.jobs.loadErr != nil) {
		err = fmt.Errorf("expected jobs.json is missing on reload: %w", os.ErrNotExist)
	}
	if err != nil {
		s.jobs.loadErr = err
		return err
	}
	// Nothing below runs until the entire candidate is valid. In particular a
	// bad last row must not trigger reconciliation writes for earlier rows.
	s.jobs.next, s.jobs.rows = in.Next, in.Rows
	s.jobs.loaded, s.jobs.loadErr = exists, nil
	changed := false
	for _, j := range s.jobs.rows {
		// PersistError is history; a row read from this file is currently recorded.
		j.Unrecorded = false
		if j.Status == "RUNNING" {
			j.Status = "INTERRUPTED"
			j.RateMBps, j.ETASeconds = 0, 0
			changed = true
		}
	}
	if changed {
		// Accepted OB-002 contract: reconciliation is re-derived from the stored
		// RUNNING row on each restart; failure to persist it does not imply lost
		// new work or require Unrecorded. Work is never resumed automatically.
		_ = s.saveJobs()
	}
	return nil
}
