package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These tests also compile on the exact published parent. They exercise the real
// OpenStore and publication paths, not a replacement loader.
func TestJobLoad_RejectExisting(t *testing.T) {
	cases := map[string]string{
		"empty": "", "truncated": `{"rows":[`, "null-board": `null`,
		"array-board": `[]`, "wrong-rows": `{"rows":{}}`,
		"null-first":   `{"next":99,"rows":[null,{"id":1,"status":"RUNNING"}]}`,
		"null-last":    `{"next":99,"rows":[{"id":1,"status":"RUNNING"},null]}`,
		"wrong-record": `{"rows":[42]}`, "missing-id": `{"rows":[{}]}`,
		"zero-id": `{"rows":[{"id":0}]}`, "negative-id": `{"rows":[{"id":-1}]}`,
		"duplicate-id":    `{"rows":[{"id":1,"status":"RUNNING"},{"id":1,"status":"FAILED"}]}`,
		"duplicate-field": `{"rows":[{"id":1,"id":2}]}`,
		"id-alias":        `{"rows":[{"id":1,"ID":2}]}`,
		"rows-alias":      `{"rows":[{"id":1}],"Rows":[]}`,
		"negative-next":   `{"next":-1,"rows":[]}`, "null-next": `{"next":null,"rows":[]}`,
		"invalid-utf8": "{\"rows\":[{\"id\":1,\"label\":\"\xff\"}]}",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() != nil {
					t.Error("refused job state caused a panic")
				}
			}()
			dir := t.TempDir()
			if _, err := OpenStore(dir); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "jobs.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			before := configSnapshot(t, dir)
			s, err := OpenStore(dir)
			if err == nil || s != nil {
				t.Error("rejected job state yielded a usable store")
				if s != nil {
					_, _ = s.NewJob("probe", "must not overwrite history")
				}
			}
			if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
				t.Error("refusal or subsequent job creation changed persisted state")
			}
		})
	}
}

func TestJobLoad_ValidAndReconcile(t *testing.T) {
	for _, raw := range []string{`{}`, `{"next":0,"rows":null}`, `{"rows":[]}`, `{"Next":7,"Rows":[]}`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "jobs.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			s, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(b, []byte(raw)) {
				t.Fatal("valid empty board rewritten")
			}
			j, err := s.NewJob("scan", "fixture")
			if err != nil || j.ID <= 0 {
				t.Fatal("valid board cannot create job")
			}
			if _, err := OpenStore(dir); err != nil {
				t.Fatal(err)
			}
		})
	}
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs.json")); !os.IsNotExist(err) {
		t.Fatal("first use unexpectedly wrote jobs")
	}
	j, err := s.NewJob("scan", "fixture")
	if err != nil || j.ID != 1 {
		t.Fatal("first job failed")
	}
	raw := `{"next":0,"rows":[{"id":8,"status":"RUNNING","rate_mbps":3,"eta_seconds":4,"artifacts":[{"kind":"catalog","count":2}]},{"id":3,"status":"FAILED","unrecorded":true,"persist_error":"synthetic historical failure"}]}`
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, b := s.Job(8), s.Job(3)
	if a.Status != "INTERRUPTED" || a.RateMBps != 0 || a.ETASeconds != 0 || len(a.Artifacts) != 1 {
		t.Fatal("interrupted reconciliation changed")
	}
	if b.Status != "FAILED" || b.Unrecorded || b.PersistError == "" {
		t.Fatal("recording history or execution status changed")
	}
	j, err = s.NewJob("scan", "next")
	if err != nil || j.ID != 9 {
		t.Fatal("counter was not recovered")
	}
}

func TestJobLoad_RefusedReloadCannotSaveStaleBoard(t *testing.T) {
	for _, raw := range []string{`{"rows":[`, `{"next":80,"rows":[{"id":11,"status":"RUNNING"},null]}`} {
		t.Run(raw, func(t *testing.T) {
			defer func() {
				if recover() != nil {
					t.Error("reload panicked")
				}
			}()
			dir := t.TempDir()
			s, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.NewJob("fixture", "prior"); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(s.Jobs())
			next := s.jobs.next
			path := filepath.Join(dir, "jobs.json")
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			s.loadJobs() // both parent and candidate: test downstream refusal even if caller ignores return
			after, _ := json.Marshal(s.Jobs())
			if !bytes.Equal(before, after) || next != s.jobs.next {
				t.Error("failed reload adopted partial state")
			}
			if j, err := s.NewJob("probe", "must not run"); err == nil || j != nil {
				t.Error("failed reload allowed new job")
			}
			b, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(b, []byte(raw)) {
				t.Error("stale board clobbered rejected file")
			}
		})
	}
}
