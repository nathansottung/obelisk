package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKeystoreValidation_ReadBoundaryNoMutation(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		err  error
	}{
		{"permission", nil, fs.ErrPermission},
		{"partial_read", []byte(`{"obelisk_keystore":1,"keys":[]}`), fs.ErrPermission},
		{"missing", nil, fs.ErrNotExist},
		{"malformed", []byte("{"), nil},
		{"empty", nil, nil},
		{"future_schema", []byte(`{"obelisk_keystore":1,"schema_version":999,"keys":[]}`), nil},
		{"null", []byte("null"), nil},
		{"bad_marker", []byte(`{"obelisk_keystore":2,"keys":[]}`), nil},
		{"unknown_field", []byte(`{"obelisk_keystore":1,"future_field":true,"keys":[]}`), nil},
		{"missing_secret", []byte(`{"obelisk_keystore":1,"keys":[{"key_ref":"A"}]}`), nil},
		{"wrong_secret", []byte(`{"obelisk_keystore":1,"keys":[{"key_ref":"A","passphrase":4}]}`), nil},
		{"duplicate_field", []byte(`{"obelisk_keystore":1,"keys":[],"keys":[]}`), nil},
		{"case_alias", []byte(`{"obelisk_keystore":1,"keys":[],"Keys":[]}`), nil},
		{"unsupported_algorithm", []byte(`{"obelisk_keystore":1,"keys":[{"key_ref":"A","passphrase":"synthetic-secret-A","algorithm":"unknown"}]}`), nil},
		{"invalid_record", []byte(`{"obelisk_keystore":1,"keys":[null]}`), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")})
			before := kvSnapshot(t, f.root)
			attempts, reads := 0, 0
			// Armed AFTER real setup. Read-result injection runs through the same parser
			// and classification as os.ReadFile, including bytes returned with an error.
			f.app.keystoreMutationObserver = func(string) { attempts++ }
			f.app.keystoreReadFile = func(p string) ([]byte, error) {
				reads++
				if p == f.paths[1] {
					return tc.data, tc.err
				}
				return os.ReadFile(p)
			}
			n, e := f.app.SyncKeystores()
			kvError(t, e)
			if n != 0 || attempts != 0 || reads != 2 {
				t.Fatalf("boundary counts: n=%d attempts=%d reads=%d", n, attempts, reads)
			}
			if tc.err != nil && !errors.Is(e, tc.err) {
				t.Error("underlying read cause lost")
			}
			kvUnchanged(t, f, before)
			// Same observer, real read and actual successful publication: proves that a
			// refusal did not merely use an observer disconnected from writeStore.
			f.app.keystoreReadFile = nil
			if n, e := f.app.SyncKeystores(); e != nil || n != 2 {
				t.Fatal("positive control sync failed")
			}
			if attempts != 2 {
				t.Fatal("positive control did not observe every mutation")
			}
			for _, p := range f.paths {
				kvMappings(t, p, map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
			}
		})
	}
}
func TestKeystoreValidation_ConflictMutationBoundary(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")}, []map[string]any{kvKey("A", "synthetic-secret-C")})
			if reverse {
				f.configure(t, []string{f.paths[2], f.paths[1], f.paths[0]})
			}
			before := kvSnapshot(t, f.root)
			attempts := 0
			f.app.keystoreMutationObserver = func(string) { attempts++ }
			_, e := f.app.SyncKeystores()
			kvError(t, e)
			if attempts != 0 {
				t.Fatal("conflict reached mutation boundary")
			}
			kvUnchanged(t, f, before)
		})
	}
}
func TestKeystoreValidation_MissingMutationBoundary(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, nil)
	if e := os.Remove(f.paths[1]); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Dir(f.paths[1])); e != nil {
		t.Fatal(e)
	}
	attempts := 0
	f.app.keystoreMutationObserver = func(string) { attempts++ }
	before := kvSnapshot(t, f.root)
	_, e := f.app.SyncKeystores()
	kvError(t, e)
	if attempts != 0 {
		t.Fatal("missing participant caused mutation")
	}
	kvUnchanged(t, f, before)
}
func TestKeystoreValidation_PublicationBoundary(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")})
	before := kvRead(t, f.paths[1])
	if e := os.Mkdir(f.paths[1]+".tmp", 0700); e != nil {
		t.Fatal(e)
	}
	attempts := 0
	f.app.keystoreMutationObserver = func(string) { attempts++ }
	n, e := f.app.SyncKeystores()
	kvError(t, e)
	if n != 0 || attempts != 2 {
		t.Fatal("publication failure not accurately observed")
	}
	var pe *os.PathError
	if !errors.As(e, &pe) {
		t.Fatal("publication error lost OS cause")
	}
	kvMappings(t, f.paths[0], map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
	if !bytes.Equal(before, kvRead(t, f.paths[1])) {
		t.Fatal("failed destination changed")
	}
}
func TestKeystoreValidation_ParticipantSnapshot(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")}, []map[string]any{kvKey("C", "synthetic-secret-C")})
	f.configure(t, f.paths[:2])
	untouched := kvRead(t, f.paths[2])
	f.app.keystoreReadFile = func(p string) ([]byte, error) {
		b, e := os.ReadFile(p)
		if p == f.paths[1] {
			f.configure(t, []string{f.paths[2]})
		} // changing config is not a keystore mutation
		return b, e
	}
	if n, e := f.app.SyncKeystores(); e != nil || n != 2 {
		t.Fatal("snapshot sync failed")
	}
	for _, p := range f.paths[:2] {
		kvMappings(t, p, map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
	}
	if !bytes.Equal(untouched, kvRead(t, f.paths[2])) {
		t.Fatal("publication reread changing configuration")
	}
}
func TestKeystoreValidation_LookupIgnoresUnrelatedConflict(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A"), kvKey("B", "synthetic-secret-B")}, []map[string]any{kvKey("A", "synthetic-secret-A"), kvKey("B", "synthetic-secret-C")})
	p, e := f.app.Passphrase("A")
	if e != nil || p != "synthetic-secret-A" {
		t.Fatal("unrelated conflict blocked unambiguous available key")
	}
	_, e = f.app.Passphrase("B")
	kvError(t, e)
}

func TestKeystoreValidation_ExactSecretsAndLocalDuplicates(t *testing.T) {
	for _, other := range []string{"synthetic-secret-a", " synthetic-secret-A", "synthetic-secret-A "} {
		f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A"), kvKey("A", other)})
		attempts := 0
		f.app.keystoreMutationObserver = func(string) { attempts++ }
		before := kvSnapshot(t, f.root)
		_, err := f.app.SyncKeystores()
		kvError(t, err)
		if attempts != 0 {
			t.Fatal("secret conflict reached publication")
		}
		pass, err := f.app.Passphrase("A")
		kvError(t, err)
		if pass != "" {
			t.Fatal("ambiguous lookup returned material")
		}
		kvUnchanged(t, f, before)
	}
}

// The Windows containment gate deliberately stops before helper lookup or any
// key generation. Reaching it proves the encrypted-build first-use precheck
// remains available without pretending to test encryption with missing helpers.
func TestKeystoreValidation_FirstBuildInitializationPrecheck(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows containment precheck control")
	}
	f := newKV(t, nil, nil)
	for _, p := range f.paths {
		if e := os.Remove(p); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := f.app.SaveConfig(map[string]any{"build_verify": "none"}); e != nil {
		t.Fatal(e)
	}
	c := f.app.Store.AddChunk(Chunk{Name: "synthetic", Status: "PLANNED", Encrypted: true})
	before := kvSnapshot(t, f.root)
	err := f.app.BuildChunk(c.ID, func(float64, string) {})
	want := assertWindowsTarBuildVerifiable(Integrity{BuildVerify: "none"})
	if err == nil || want == nil || err.Error() != want.Error() {
		t.Fatal("first-use build did not reach the unchanged containment refusal")
	}
	kvUnchanged(t, f, before)
}
