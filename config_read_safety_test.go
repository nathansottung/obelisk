package main

// This file also compiles unchanged at the pinned parent. It exercises real
// settings endpoints and file state without depending on the new read signature.
import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigReadSafety_SettingsRejectDamagedPrerequisite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content []byte
		missing bool
	}{
		{"empty", []byte{}, false}, {"malformed", []byte(`{"auth_token":"fixture-private-value",`), false},
		{"null", []byte(`null`), false}, {"array", []byte(`[]`), false},
		{"wrong-field-type", []byte(`{"tools":42}`), false}, {"missing", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			st, e := OpenStore(dir)
			if e != nil {
				t.Fatal("fixture catalog failed")
			}
			a := &App{DataDir: dir, Store: st}
			path := filepath.Join(dir, "config.json")
			if !tc.missing {
				if e := os.WriteFile(path, tc.content, 0600); e != nil {
					t.Fatal("fixture write failed")
				}
			}
			mux := http.NewServeMux()
			api(mux, a)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", bytes.NewBufferString(`{"ui_mode":"complete"}`)))
			if w.Code < 400 {
				t.Error("settings update succeeded after a failed prerequisite read")
			}
			after, e := os.ReadFile(path)
			if tc.missing {
				if !os.IsNotExist(e) {
					t.Error("ordinary update created absent expected config")
				}
			} else if e != nil || !bytes.Equal(after, tc.content) {
				t.Error("refused update did not preserve original configuration bytes")
			}
			if bytes.Contains(w.Body.Bytes(), []byte("fixture-private-value")) {
				t.Error("response exposed a fixture secret")
			}
		})
	}
}

func TestConfigReadSafety_ValidUpdatePreservesFields(t *testing.T) {
	dir := t.TempDir()
	st, e := OpenStore(dir)
	if e != nil {
		t.Fatal("fixture catalog failed")
	}
	a := &App{DataDir: dir, Store: st}
	path := filepath.Join(dir, "config.json")
	input := map[string]any{"auth_token": "fixture-private-value", "staging_dir": filepath.Join(dir, "stage"), "keystore_paths": []string{filepath.Join(dir, "k1"), filepath.Join(dir, "k2")}, "tools": map[string]string{"gpg": "fixture-helper"}, "private_media": true, "par2_redundancy": 17, "label_size": "custom", "future_extension": map[string]any{"enabled": true}}
	b, e := json.Marshal(input)
	if e != nil {
		t.Fatal("fixture encoding failed")
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal("fixture write failed")
	}
	if _, e := a.SaveConfig(map[string]any{"ui_mode": "complete", "label_size": ""}); e != nil {
		t.Fatal("valid update failed")
	}
	after, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("reopen failed")
	}
	var got map[string]json.RawMessage
	var want map[string]json.RawMessage
	if json.Unmarshal(after, &got) != nil || json.Unmarshal(b, &want) != nil {
		t.Fatal("reopen invalid JSON")
	}
	for k, v := range want {
		if k == "label_size" {
			continue
		}
		if !bytes.Equal(compactConfigJSON(t, got[k]), compactConfigJSON(t, v)) {
			t.Errorf("unaffected field %s changed", k)
		}
	}
	var label string
	if v, ok := got["label_size"]; !ok || json.Unmarshal(v, &label) != nil || label != "" {
		t.Error("explicit optional field clear was not retained")
	}
}

func compactConfigJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	if json.Compact(&out, b) != nil {
		t.Fatal("missing or invalid reopened field")
	}
	return out.Bytes()
}
