//go:build !guionly

package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestJobLoad_ReadFailures(t *testing.T) {
	for _, cause := range []error{os.ErrPermission, errors.New("synthetic I/O error")} {
		t.Run(cause.Error(), func(t *testing.T) {
			dir := t.TempDir()
			s, err := OpenStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.NewJob("fixture", "existing"); err != nil {
				t.Fatal(err)
			}
			before := configSnapshot(t, dir)
			read := func(p string) ([]byte, error) {
				if p == s.jobs.path {
					return []byte(`{"rows":[]}`), cause
				}
				return os.ReadFile(p)
			}
			if got, err := openStore(dir, read, nil); got != nil || !errors.Is(err, cause) {
				t.Fatal("startup did not propagate job read failure")
			}
			if err := s.loadJobsFrom(read); !errors.Is(err, cause) {
				t.Fatal("reload lost OS cause")
			}
			if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
				t.Fatal("read failure mutated disk")
			}
		})
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "jobs.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if s, err := OpenStore(dir); s != nil || err == nil {
		t.Fatal("directory obstruction was accepted")
	}
	fi, err := os.Stat(filepath.Join(dir, "jobs.json"))
	if err != nil || !fi.IsDir() {
		t.Fatal("obstruction lost")
	}
	// OpenStore still seeds the catalog before reading jobs. This is not a
	// cross-store rollback claim; the obstruction itself must remain untouched.
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); err != nil {
		t.Fatal("expected catalog-first startup order changed")
	}
}

func TestJobLoad_LatchBlocksAllPublicationAndWorkers(t *testing.T) {
	a := newSetupApp(t)
	s := a.Store
	j, err := s.NewJob("fixture", "existing")
	if err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"rows":[{"id":2},null]}`)
	if err := os.WriteFile(s.jobs.path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.loadJobs(); err == nil {
		t.Fatal("bad reload accepted")
	}
	for name, attempt := range map[string]func() error{
		"new":             func() error { _, err := s.NewJob("fixture", "blocked"); return err },
		"finish":          func() error { return s.FinishJob(j.ID, 1, "", "FAILED", nil, nil) },
		"set":             func() error { return s.SetJob(j.ID, 1, "", "COMPLETED") },
		"append-artifact": func() error { return s.AppendJobArtifact(j.ID, Artifact{Kind: "catalog"}) },
		"set-artifacts":   func() error { return s.SetJobArtifacts(j.ID, []Artifact{{Kind: "catalog"}}) },
		"result":          func() error { return s.SetJobResult(j.ID, map[string]any{"fixture": true}) },
		"save":            func() error { s.jobs.mu.Lock(); defer s.jobs.mu.Unlock(); return s.saveJobs() },
	} {
		t.Run(name, func(t *testing.T) {
			if err := attempt(); err == nil {
				t.Fatal("publication was not refused")
			}
			b, err := os.ReadFile(s.jobs.path)
			if err != nil || !bytes.Equal(b, bad) {
				t.Fatal("rejected bytes overwritten")
			}
		})
	}
	var ran atomic.Bool
	if result, err := runJob(a, "probe", "blocked", func(func(float64, string)) (map[string]any, error) { ran.Store(true); return nil, nil }); err == nil || result != nil {
		t.Fatal("worker creation accepted")
	}
	if ran.Load() {
		t.Fatal("work ran after refused load")
	}
	if err := os.Remove(s.jobs.path); err != nil {
		t.Fatal(err)
	}
	if err := s.loadJobs(); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing reload cleared refusal")
	}
	if _, err := s.NewJob("probe", "blocked"); err == nil {
		t.Fatal("missing state allowed stale save")
	}
	good := []byte(`{"next":12,"rows":[{"id":12,"status":"FAILED","persist_error":"history"}]}`)
	if err := os.WriteFile(s.jobs.path, good, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.loadJobs(); err != nil {
		t.Fatal(err)
	}
	if j, err := s.NewJob("probe", "allowed"); err != nil || j.ID != 13 {
		t.Fatal("validated reload did not restore service")
	}
}

func TestJobLoad_ReconcileWriteFailureAndCounterLimit(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"next":2,"rows":[{"id":2,"status":"RUNNING","unrecorded":true,"persist_error":"history"}]}`)
	if err := os.WriteFile(s.jobs.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.failSaveJobs = func() error { calls++; return errors.New("synthetic reconciliation write failure") }
	if err := s.loadJobs(); err != nil || calls != 1 {
		t.Fatal("best-effort reconciliation contract changed")
	}
	if j := s.Job(2); j.Status != "INTERRUPTED" || j.Unrecorded || j.PersistError != "history" {
		t.Fatal("reconciliation recording semantics changed")
	}
	b, err := os.ReadFile(s.jobs.path)
	if err != nil || !bytes.Equal(b, raw) {
		t.Fatal("failed write changed prior file")
	}
	s.failSaveJobs = nil
	if _, err := s.NewJob("fixture", "next save"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{fmt.Sprintf(`{"next":%d,"rows":[]}`, int(^uint(0)>>1)), fmt.Sprintf(`{"rows":[{"id":%d}]}`, int(^uint(0)>>1))} {
		if err := os.WriteFile(s.jobs.path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.loadJobs(); err != nil {
			t.Fatal("valid exhausted board must remain readable")
		}
		if j, err := s.NewJob("fixture", "overflow"); err == nil || j != nil {
			t.Fatal("identity counter wrapped")
		}
		b, err := os.ReadFile(s.jobs.path)
		if err != nil || !bytes.Equal(b, []byte(raw)) {
			t.Fatal("exhausted board changed")
		}
	}
}

func jobLoadBundle(t *testing.T, members map[string][]byte) string {
	t.Helper()
	man := appBackupManifest{Format: appBackupFormat, SchemaVersion: currentSchemaVersion}
	for name, b := range members {
		man.Members = append(man.Members, appBackupMember{Name: name, Size: int64(len(b)), SHA256: sha256Hex(b)})
	}
	var err error
	members["MANIFEST.json"], err = json.Marshal(man)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, b := range members {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(b))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.tar")
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJobLoad_RestoreCaller(t *testing.T) {
	a := abTestApp(t)
	if _, err := a.Store.NewJob("fixture", "prior"); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"rows":[null]}`)
	bundle := jobLoadBundle(t, map[string][]byte{"jobs.json": bad})
	before := configSnapshot(t, a.DataDir)
	prior := a.Store
	if _, err := a.RestoreAppBackup(bundle); err == nil {
		t.Fatal("restore accepted structurally invalid jobs with valid hashes")
	}
	if a.Store != prior || !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
		t.Fatal("rejected archive changed live state")
	}
	// If a jobs-omitting restore encounters rejected current history on reopen,
	// it returns an error, keeps the old Store, and latches its jobs writes closed.
	if err := os.WriteFile(a.Store.jobs.path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	bundle = jobLoadBundle(t, map[string][]byte{"formats.json": []byte(`[]`)})
	if _, err := a.RestoreAppBackup(bundle); err == nil {
		t.Fatal("reopen error hidden")
	}
	if a.Store != prior {
		t.Fatal("failed reopen swapped store")
	}
	if _, err := a.Store.NewJob("fixture", "blocked"); err == nil {
		t.Fatal("retained store clobbered rejected history")
	}
	b, err := os.ReadFile(a.Store.jobs.path)
	if err != nil || !bytes.Equal(b, bad) {
		t.Fatal("restore failure lost rejected bytes")
	}
}

func TestJobLoad_MigrationCaller(t *testing.T) {
	a := abTestApp(t)
	prior := a.Store
	bad := []byte(`{"rows":[null]}`)
	if err := os.WriteFile(prior.jobs.path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if _, err := a.migrateDataDir(a.DataDir, target); err == nil {
		t.Fatal("migration hid rejected target jobs")
	}
	if a.Store != prior || a.DataDir == target {
		t.Fatal("migration adopted rejected store")
	}
	if _, err := prior.NewJob("fixture", "blocked"); err == nil {
		t.Fatal("migration retained a writable stale board")
	}
	for _, p := range []string{prior.jobs.path, filepath.Join(target, "jobs.json")} {
		b, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(b, bad) {
			t.Fatal("migration altered job bytes")
		}
	}
}

func TestJobLoad_IdentityAndOptionalCompatibility(t *testing.T) {
	for name, raw := range map[string]string{
		"null-id":            `{"rows":[{"id":null}]}`,
		"typed-id":           `{"rows":[{"id":"1"}]}`,
		"next-alias":         `{"next":2,"Next":3,"rows":[]}`,
		"trailing-value":     `{"rows":[]} {}`,
		"invalid-last-field": `{"rows":[{"id":1,"status":"RUNNING"},{"id":2,"created_at":"bad"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			s, err := OpenStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			writes := 0
			s.failSaveJobs = func() error { writes++; return nil }
			if err := os.WriteFile(s.jobs.path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := s.loadJobs(); err == nil {
				t.Fatal("ambiguous/invalid candidate accepted")
			}
			if writes != 0 || s.jobs.next != 0 || len(s.jobs.rows) != 0 {
				t.Fatal("reconciliation or adoption preceded complete validation")
			}
		})
	}
	// Optional fields and unknown extensions retain the former reader's policy.
	// A single historical field-name alias remains unambiguous and supported.
	raw := []byte(`{"Rows":[{"ID":4,"result":null,"artifacts":null,"future_extension":true}],"future_board":{}}`)
	s, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.jobs.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.loadJobs(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.jobs.path)
	if err != nil || !bytes.Equal(b, raw) {
		t.Fatal("valid omitted fields caused rewrite")
	}
	if j, err := s.NewJob("fixture", "next"); err != nil || j.ID != 5 {
		t.Fatal("legacy ID was not honored")
	}
}
