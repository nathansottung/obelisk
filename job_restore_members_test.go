//go:build !guionly

package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Unlike the map-based fixture helper, this writes every supplied tar record in
// order, including duplicates. For duplicate names the manifest describes the
// last payload, reproducing the former reader's last-wins integrity behavior.
func orderedJobRestoreBundle(t *testing.T, rows []rawMember, duplicateManifest bool) string {
	t.Helper()
	man := appBackupManifest{Format: appBackupFormat, Version: appBackupVersion, SchemaVersion: currentSchemaVersion}
	indices := map[string]int{}
	for _, row := range rows {
		m := appBackupMember{Name: row.name, Size: int64(len(row.data)), SHA256: sha256Hex(row.data)}
		if i, ok := indices[row.name]; ok {
			man.Members[i] = m
		} else {
			indices[row.name] = len(man.Members)
			man.Members = append(man.Members, m)
		}
	}
	if duplicateManifest {
		man.Members = append(man.Members, man.Members[0])
	}
	mb, err := json.Marshal(man)
	if err != nil {
		t.Fatal(err)
	}
	all := append([]rawMember{{"MANIFEST.json", mb}}, rows...)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, row := range all {
		if err := tw.WriteHeader(&tar.Header{Name: row.name, Mode: 0600, Size: int64(len(row.data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(row.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	// Inspect raw archive records, without the production map reader deduplicating.
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	for _, row := range all {
		h, err := tr.Next()
		if err != nil || h.Name != row.name {
			t.Fatalf("fixture record missing: %v", err)
		}
		b, err := io.ReadAll(tr)
		if err != nil || !bytes.Equal(b, row.data) {
			t.Fatal("fixture payload changed")
		}
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatal("unexpected fixture record")
	}
	p := filepath.Join(t.TempDir(), "ordered.tar")
	if err := os.WriteFile(p, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestJobRestoreMembers_RefusalBeforePublication(t *testing.T) {
	good := []byte(`{"next":70,"rows":[{"id":70,"status":"FAILED","label":"incoming"}]}`)
	bad := []byte(`{"rows":[null]}`)
	type fixture struct {
		name              string
		rows              []rawMember
		duplicateManifest bool
	}
	cases := []fixture{{"canonical-invalid", []rawMember{{"jobs.json", bad}}, false}}
	// These spellings are rejected by the format, not normalized or classified
	// solely by basename. Windows stream/trailing-dot spellings never reach Win32.
	for _, name := range []string{"./jobs.json", "Jobs.json", `sub/../jobs.json`, `.\jobs.json`, `sub\..\jobs.json`, "jobs.json.", "jobs.json ", "jobs.json::$DATA", "other/jobs.json", "keystores/../jobs.json", `keystores/..\jobs.json`} {
		cases = append(cases, fixture{name + "-invalid", []rawMember{{name, bad}}, false}, fixture{name + "-valid", []rawMember{{name, good}}, false})
	}
	for _, alias := range []string{"jobs.json", "./jobs.json", "Jobs.json"} {
		for _, reverse := range []bool{false, true} {
			rows := []rawMember{{"jobs.json", good}, {alias, bad}}
			label := alias + "-valid-first"
			if reverse {
				rows[0], rows[1] = rows[1], rows[0]
				label = alias + "-invalid-first"
			}
			cases = append(cases, fixture{label, rows, false})
		}
	}
	cases = append(cases, fixture{"duplicate-manifest", []rawMember{{"jobs.json", good}}, true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := abTestApp(t)
			prior := a.Store
			j, err := prior.NewJob("fixture", "recognizable original")
			if err != nil {
				t.Fatal(err)
			}
			before := configSnapshot(t, a.DataDir)
			memory, err := json.Marshal(prior.Jobs())
			if err != nil {
				t.Fatal(err)
			}
			bundle := orderedJobRestoreBundle(t, tc.rows, tc.duplicateManifest)
			_, preflightErr := a.InspectAppBackup(bundle)
			if preflightErr == nil {
				t.Fatal("read-only preflight accepted rejected member")
			}
			_, err = a.RestoreAppBackup(bundle)
			if err == nil || err.Error() != preflightErr.Error() {
				t.Fatalf("restore did not return preflight refusal: %v / %v", err, preflightErr)
			}
			if !strings.Contains(err.Error(), "member") && !strings.Contains(err.Error(), "job board") {
				t.Fatalf("unclear error: %v", err)
			}
			after, err := json.Marshal(prior.Jobs())
			if err != nil {
				t.Fatal(err)
			}
			if a.Store != prior || !bytes.Equal(memory, after) || prior.jobs.next != j.ID {
				t.Fatal("refusal adopted incoming state")
			}
			// No pre-restore directory was created. Production creates it before all
			// member writes and never rolls it back: combined with the same read-only
			// preflight error this establishes refusal before publication, not rollback.
			if !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
				t.Fatal("refusal changed authoritative bytes or started restore")
			}
			next, err := prior.NewJob("fixture", "legitimate later update")
			if err != nil || next.ID != j.ID+1 {
				t.Fatal("incoming refusal disabled valid original store")
			}
			reopened, err := OpenStore(a.DataDir)
			if err != nil || reopened.Job(70) != nil || reopened.Job(j.ID) == nil || reopened.Job(next.ID) == nil {
				t.Fatal("later save persisted rejected state")
			}
		})
	}
}

func TestJobRestoreMembers_PreservesExistingRefusal(t *testing.T) {
	a := abTestApp(t)
	if _, err := a.Store.NewJob("fixture", "original"); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"rows":[null]}`)
	if err := os.WriteFile(a.Store.jobs.path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.loadJobs(); err == nil {
		t.Fatal("expected existing refusal")
	}
	latch := a.Store.jobs.loadErr
	bundle := orderedJobRestoreBundle(t, []rawMember{{"./jobs.json", []byte(`{"rows":[]}`)}}, false)
	if _, err := a.RestoreAppBackup(bundle); err == nil {
		t.Fatal("alias accepted")
	}
	if a.Store.jobs.loadErr != latch {
		t.Fatal("incoming rejection changed original refusal")
	}
	if _, err := a.Store.NewJob("fixture", "blocked"); err == nil {
		t.Fatal("original refusal cleared")
	}
	b, err := os.ReadFile(a.Store.jobs.path)
	if err != nil || !bytes.Equal(b, bad) {
		t.Fatal("rejected current bytes changed")
	}
}

func TestJobRestoreMembers_NestedKeystoreIsNotJobBoard(t *testing.T) {
	a := abTestApp(t)
	if _, err := a.Store.NewJob("fixture", "original"); err != nil {
		t.Fatal(err)
	}
	// A supported keystore basename may itself be jobs.json. It lives in the
	// keystore directory and must not be decoded as a job board.
	key := []byte(`{"obelisk_keystore":1,"keys":[]}`)
	bundle := orderedJobRestoreBundle(t, []rawMember{{"keystores/jobs.json", key}}, false)
	if _, err := a.RestoreAppBackup(bundle); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(a.DataDir, "keystores", "jobs.json"))
	if err != nil || !bytes.Equal(b, key) || a.Store.Job(1) == nil {
		t.Fatal("keystore/job dispatch confused")
	}
}
