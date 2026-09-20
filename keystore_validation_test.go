package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type kvFixture struct {
	app   *App
	root  string
	paths []string
}

func newKV(t *testing.T, records ...[]map[string]any) kvFixture {
	t.Helper()
	data := t.TempDir()
	st, err := OpenStore(data)
	if err != nil {
		t.Fatal(err)
	}
	f := kvFixture{app: initializedTestApp(t, &App{DataDir: data, Store: st}), root: t.TempDir()}
	for i, keys := range records {
		p := filepath.Join(f.root, fmt.Sprint(i), "keys.json")
		if err := writeStore(p, &keystoreFile{Marker: 1, Keys: keys}); err != nil {
			t.Fatal(err)
		}
		f.paths = append(f.paths, p)
	}
	f.configure(t, f.paths)
	return f
}
func (f kvFixture) configure(t *testing.T, paths []string) {
	t.Helper()
	if _, err := f.app.SaveConfig(map[string]any{"keystore_paths": paths}); err != nil {
		t.Fatal(err)
	}
}
func kvKey(ref, secret string) map[string]any {
	return map[string]any{"key_ref": ref, "passphrase": secret}
}
func kvRead(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func kvPut(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}

// Keep snapshots secret-free even in failure diagnostics. Include directory and
// file entries, not just final store bytes or the absence of a temporary name.
func kvSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if d.IsDir() {
			out[rel] = "directory"
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out[rel] = fmt.Sprintf("%x", sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func kvUnchanged(t *testing.T, f kvFixture, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, kvSnapshot(t, f.root)) {
		t.Error("keystore files or directory entries changed")
	}
}
func kvNoSecrets(t *testing.T, v string) {
	t.Helper()
	for _, secret := range []string{"synthetic-secret-A", "synthetic-secret-B", "synthetic-secret-C"} {
		if strings.Contains(v, secret) {
			t.Fatal("secret appeared in diagnostics")
		}
	}
}
func kvError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected refusal")
		return
	}
	kvNoSecrets(t, err.Error())
}
func kvMappings(t *testing.T, p string, want map[string]string) {
	t.Helper()
	ks, e := readStore(p)
	if e != nil {
		t.Fatal(e)
	}
	got := map[string]string{}
	for _, k := range ks.Keys {
		ref, ok := k["key_ref"].(string)
		if !ok {
			t.Fatal("invalid reference")
		}
		v, ok := k["passphrase"].(string)
		if !ok {
			t.Fatal("invalid secret")
		}
		got[ref] = v
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("reopened secret mappings differ")
	}
}

func TestKeystoreValidation_UnionAndReopen(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")})
	n, e := f.app.SyncKeystores()
	if e != nil || n != 2 {
		t.Fatal("valid union failed")
	}
	for _, p := range f.paths {
		kvMappings(t, p, map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
	}
	if !f.app.KeystoreStatus()["ok"].(bool) {
		t.Fatal("synced status not OK")
	}
}
func TestKeystoreValidation_InvalidFinalParticipant(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"malformed", "{"}, {"empty", ""}, {"null", "null"}, {"bad_marker", `{"obelisk_keystore":2,"keys":[]}`},
		{"future_schema", `{"obelisk_keystore":1,"schema_version":999,"keys":[]}`},
		{"unknown_field", `{"obelisk_keystore":1,"future_safety_field":true,"keys":[]}`},
		{"null_key", `{"obelisk_keystore":1,"keys":[null]}`},
		{"missing_secret", `{"obelisk_keystore":1,"keys":[{"key_ref":"A"}]}`},
		{"wrong_secret_type", `{"obelisk_keystore":1,"keys":[{"key_ref":"A","passphrase":4}]}`},
		{"duplicate_field", `{"obelisk_keystore":1,"keys":[{"key_ref":"A","passphrase":"synthetic-secret-A","passphrase":"synthetic-secret-B"}]}`},
		{"unsupported_algorithm", `{"obelisk_keystore":1,"keys":[{"key_ref":"A","passphrase":"synthetic-secret-A","algorithm":"unknown"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, nil)
			kvPut(t, f.paths[1], []byte(tc.raw))
			before := kvSnapshot(t, f.root)
			n, e := f.app.SyncKeystores()
			kvError(t, e)
			if n != 0 {
				t.Error("refused sync returned completed count")
			}
			kvUnchanged(t, f, before)
			if f.app.KeystoreStatus()["ok"].(bool) {
				t.Error("invalid replica status claims consistency")
			}
		})
	}
}
func TestKeystoreValidation_MissingDoesNotProvision(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, nil)
	if e := os.Remove(f.paths[1]); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Dir(f.paths[1])); e != nil {
		t.Fatal(e)
	}
	before := kvSnapshot(t, f.root)
	_, e := f.app.SyncKeystores()
	kvError(t, e)
	kvUnchanged(t, f, before)
	if f.app.KeystoreStatus()["ok"].(bool) {
		t.Error("missing replica status claims consistency")
	}
}
func TestKeystoreValidation_ConflictBothOrders(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-B")})
			if reverse {
				f.configure(t, []string{f.paths[1], f.paths[0]})
			}
			before := kvSnapshot(t, f.root)
			status := f.app.KeystoreStatus()
			if status["ok"].(bool) {
				t.Error("conflict status claims consistency")
			}
			kvNoSecrets(t, fmt.Sprint(status))
			pass, e := f.app.Passphrase("A")
			kvError(t, e)
			if pass != "" {
				t.Error("conflict lookup returned material")
			}
			n, e := f.app.SyncKeystores()
			kvError(t, e)
			if n != 0 {
				t.Error("conflict sync reported completion")
			}
			kvUnchanged(t, f, before)
		})
	}
}
func TestKeystoreValidation_IdenticalAndMetadata(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	if n, e := f.app.SyncKeystores(); e != nil || n != 1 {
		t.Fatal("identical secret rejected")
	}
	a := kvKey("A", "synthetic-secret-A")
	a["note"] = "retain note"
	b := kvKey("A", "synthetic-secret-A")
	b["created_at"] = "retain date"
	b["future_metadata"] = json.Number("9007199254740993")
	f = newKV(t, []map[string]any{a}, []map[string]any{b})
	if _, e := f.app.SyncKeystores(); e != nil {
		t.Fatal("compatible metadata rejected")
	}
	first := kvRead(t, f.paths[0])
	if !bytes.Contains(first, []byte("9007199254740993")) {
		t.Fatal("metadata precision lost")
	}
	if !bytes.Contains(first, []byte("retain note")) || !bytes.Contains(first, []byte("retain date")) {
		t.Fatal("metadata lost")
	}
	f.configure(t, []string{f.paths[1], f.paths[0]})
	if _, e := f.app.SyncKeystores(); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(first, kvRead(t, f.paths[0])) {
		t.Fatal("output depends on configuration order")
	}
}
func TestKeystoreValidation_MetadataConflictRefused(t *testing.T) {
	a := kvKey("A", "synthetic-secret-A")
	a["note"] = "first"
	b := kvKey("A", "synthetic-secret-A")
	b["note"] = "second"
	f := newKV(t, []map[string]any{a}, []map[string]any{b})
	before := kvSnapshot(t, f.root)
	_, e := f.app.SyncKeystores()
	kvError(t, e)
	kvUnchanged(t, f, before)
	if e != nil && !strings.Contains(e.Error(), "metadata") {
		t.Error("metadata conflict mislabeled")
	}
	pass, e := f.app.Passphrase("A")
	if e != nil || pass != "synthetic-secret-A" {
		t.Fatal("matching secret must remain recoverable despite metadata conflict")
	}
}
func TestKeystoreValidation_OfflineRecovery(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, nil)
	if e := os.Remove(f.paths[1]); e != nil {
		t.Fatal(e)
	}
	before := kvSnapshot(t, f.root)
	pass, e := f.app.Passphrase("A")
	if e != nil || pass != "synthetic-secret-A" {
		t.Fatal("offline replica blocked available key")
	}
	if f.app.KeystoreStatus()["ok"].(bool) {
		t.Error("offline status claims complete consistency")
	}
	kvUnchanged(t, f, before)
}
func TestKeystoreValidation_PublicationFailureAPI(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")})
	before := kvRead(t, f.paths[1])
	if e := os.Mkdir(f.paths[1]+".tmp", 0700); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	api(mux, f.app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/keys/sync", nil))
	if rec.Code != 400 {
		t.Fatalf("expected error status, got %d", rec.Code)
	}
	kvNoSecrets(t, rec.Body.String())
	if strings.Contains(rec.Body.String(), "key_count") {
		t.Error("failed publication reported completed count")
	}
	if !strings.Contains(rec.Body.String(), "earlier participants") {
		t.Error("partial publication limit missing")
	}
	kvMappings(t, f.paths[0], map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
	if !bytes.Equal(before, kvRead(t, f.paths[1])) {
		t.Fatal("failed participant changed")
	}
}
func TestKeystoreValidation_FirstUseCompatibility(t *testing.T) {
	f := newKV(t, nil, nil)
	for _, p := range f.paths {
		if e := os.Remove(p); e != nil {
			t.Fatal(e)
		}
	}
	if f.app.KeystoreStatus()["ok"].(bool) {
		t.Error("public status claims missing stores exist")
	}
	if ks, e := readStore(f.paths[0]); e != nil || len(ks.Keys) != 0 {
		t.Fatal("legacy first-use reader broken")
	}
	ref, pass, _, e := f.app.GenerateKey("synthetic")
	if e != nil || ref == "" || pass == "" {
		t.Fatal("first-use generation failed")
	}
	for _, p := range f.paths {
		kvMappings(t, p, map[string]string{ref: pass})
	}
	if !f.app.KeystoreStatus()["ok"].(bool) {
		t.Fatal("initialized stores not consistent")
	}
}
