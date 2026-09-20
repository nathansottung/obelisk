package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func configSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, p)
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
		out[rel] = sha256Hex(b)
		return nil
	})
	if err != nil {
		t.Fatal("fixture snapshot failed")
	}
	return out
}

func TestConfigBoundary_RefusedReadsObserveNoMutation(t *testing.T) {
	cases := []struct {
		name    string
		content string
		readErr error
		missing bool
	}{
		{"permission", "", os.ErrPermission, false}, {"io-error", "", io.ErrUnexpectedEOF, false},
		{"missing", "", nil, true}, {"empty", "", nil, false}, {"malformed", `{"auth_token":"fixture-private-value",`, nil, false},
		{"invalid-utf8", "{\"auth_token\":\"\xff\"}", nil, false},
		{"null", "null", nil, false}, {"array", "[]", nil, false}, {"scalar", "true", nil, false},
		{"null-scalar", `{"auth_token":null}`, nil, false}, {"null-map-entry", `{"tools":{"gpg":null}}`, nil, false}, {"null-list-entry", `{"keystore_paths":[null]}`, nil, false},
		{"wrong-type", `{"keystore_paths":42}`, nil, false}, {"duplicate", `{"ui_mode":"guided","ui_mode":"complete"}`, nil, false},
		{"alias-conflict", `{"ui_mode":"guided","UI_MODE":"complete"}`, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newSetupApp(t)
			path := a.configPath()
			mutations := 0
			a.configIO.observe = func(string) { mutations++ }
			// Positive control uses the very same observer and real publication path.
			if _, e := a.SaveConfig(map[string]any{"ui_mode": "standard"}); e != nil || mutations != 1 {
				t.Fatal("mutation observer positive control failed")
			}
			if tc.missing {
				if os.Remove(path) != nil {
					t.Fatal("fixture removal failed")
				}
			} else if tc.readErr == nil {
				if os.WriteFile(path, []byte(tc.content), 0600) != nil {
					t.Fatal("fixture write failed")
				}
			}
			reads := 0
			if tc.readErr != nil {
				a.configIO.readFile = func(p string) ([]byte, error) {
					reads++
					if p != path {
						t.Error("read seam missed actual config path")
					}
					return nil, &os.PathError{Op: "read", Path: p, Err: tc.readErr}
				}
			}
			before := configSnapshot(t, a.DataDir)
			mutations = 0
			if cfg, e := a.LoadConfig(); e == nil || !reflect.DeepEqual(cfg, Config{}) {
				t.Error("failed read returned successful/default state")
			}
			if cfg, e := a.SaveConfig(map[string]any{"ui_mode": "complete"}); e == nil || !reflect.DeepEqual(cfg, Config{}) {
				t.Error("failed update returned successful/default state")
			}
			mux := http.NewServeMux()
			api(mux, a)
			for _, req := range []struct{ method, path, body string }{{"GET", "/api/config", ""}, {"PUT", "/api/config", `{"ui_mode":"complete"}`}, {"GET", "/api/setup", ""}, {"POST", "/api/setup", `{"skipped":true}`}, {"GET", "/api/home", ""}} {
				w := httptest.NewRecorder()
				mux.ServeHTTP(w, httptest.NewRequest(req.method, req.path, strings.NewReader(req.body)))
				if w.Code < 400 || w.Code == 404 {
					t.Errorf("%s %s did not propagate failure", req.method, req.path)
				}
				if strings.Contains(w.Body.String(), "fixture-private-value") {
					t.Error("response exposed secret")
				}
			}
			if tc.readErr != nil && reads < 2 {
				t.Error("read injection did not enter production decision path")
			}
			if mutations != 0 {
				t.Error("mutation attempted after validation failure")
			}
			if !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
				t.Error("refused operation changed fixture entries/bytes")
			}
		})
	}
}

func TestConfigBoundary_ExplicitInitialization(t *testing.T) {
	startupDir := t.TempDir()
	if _, e := loadStartupConfig(startupDir, true); e != nil {
		t.Fatal("startup explicit initialization failed")
	}
	if _, e := loadStartupConfig(startupDir, false); e != nil {
		t.Fatal("startup reopen failed")
	}
	dir := filepath.Join(t.TempDir(), "first-use")
	a := &App{DataDir: dir}
	n := 0
	a.configIO.observe = func(string) { n++ }
	if _, e := a.LoadConfig(); e == nil {
		t.Fatal("missing read silently initialized")
	}
	if _, e := a.SaveConfig(map[string]any{}); e == nil || n != 0 {
		t.Fatal("ordinary update silently initialized")
	}
	cfg, e := a.InitializeConfig()
	if e != nil || n != 1 || !cfg.HashAccel {
		t.Fatal("explicit first use failed")
	}
	before := configSnapshot(t, dir)
	if _, e := a.InitializeConfig(); e != nil || n != 1 {
		t.Fatal("repeat initialization mutated valid state")
	}
	if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
		t.Fatal("repeat initialization changed bytes")
	}
	if os.WriteFile(a.configPath(), []byte("null"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	n = 0
	if _, e := a.InitializeConfig(); e == nil || n != 0 {
		t.Fatal("initialization replaced invalid existing state")
	}
}

type configFaultFile struct {
	*os.File
	failure string
}

func (f configFaultFile) Write(b []byte) (int, error) {
	if f.failure == "write" {
		return 0, errors.New("injected write failure")
	}
	if f.failure == "short-write" {
		return f.File.Write(b[:len(b)/2])
	}
	return f.File.Write(b)
}
func (f configFaultFile) Sync() error {
	if f.failure == "sync" {
		return errors.New("injected sync failure")
	}
	return f.File.Sync()
}
func (f configFaultFile) Close() error {
	e := f.File.Close()
	if f.failure == "close" {
		return errors.New("injected close failure")
	}
	return e
}

func TestConfigBoundary_CheckedPublication(t *testing.T) {
	oldAccel := hashAccelOn.Load()
	defer setHashAccel(oldAccel)
	for _, kind := range []string{"serialize", "create", "write", "short-write", "sync", "close", "rename", "dir-sync", "success"} {
		t.Run(kind, func(t *testing.T) {
			a := newSetupApp(t)
			before := configSnapshot(t, a.DataDir)
			mutations := 0
			a.configIO.observe = func(string) { mutations++ }
			a.configIO.createTemp = func(d, p string) (configTempFile, error) {
				if kind == "create" {
					return nil, errors.New("injected creation failure")
				}
				f, e := os.CreateTemp(d, p)
				if e != nil {
					return nil, e
				}
				return configFaultFile{f, kind}, nil
			}
			if kind == "rename" {
				a.configIO.rename = func(string, string) error { return errors.New("injected rename failure") }
			}
			if kind == "dir-sync" {
				a.configIO.syncDir = func(string) error { return errors.New("injected directory sync failure") }
			}
			in := map[string]any{"ui_mode": "complete", "hash_accel": false}
			if kind == "serialize" {
				in["unknown"] = make(chan int)
			}
			cfg, e := a.SaveConfig(in)
			if kind == "success" {
				if e != nil || cfg.UIMode != "complete" || mutations != 1 {
					t.Fatal("positive publication control failed")
				}
			} else {
				if e == nil || !reflect.DeepEqual(cfg, Config{}) {
					t.Fatal("failed publication exposed successful state")
				}
				if kind == "serialize" {
					if mutations != 0 {
						t.Fatal("serialization failure crossed mutation boundary")
					}
				} else {
					var pe *ConfigPublicationError
					if !errors.As(e, &pe) || pe.Published != (kind == "dir-sync") || mutations != 1 {
						t.Fatal("publication phase was misreported")
					}
				}
			}
			if kind == "success" || kind == "dir-sync" {
				if hashAccelOn.Load() {
					t.Fatal("runtime preference did not follow published bytes")
				}
				reopened, e := a.LoadConfig()
				if e != nil || reopened.UIMode != "complete" || reopened.HashAccel {
					t.Fatal("published bytes not current after publication")
				}
			} else {
				if !hashAccelOn.Load() {
					t.Fatal("runtime preference changed before publication")
				}
				if !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
					t.Fatal("pre-publication failure changed fixture entries/bytes")
				}
			}
		})
	}
}

func TestConfigBoundary_RealReadError(t *testing.T) {
	a := newSetupApp(t)
	n := 0
	a.configIO.observe = func(string) { n++ }
	if _, e := a.SaveConfig(map[string]any{"ui_mode": "standard"}); e != nil || n != 1 {
		t.Fatal("positive control failed")
	}
	if os.Rename(a.configPath(), a.configPath()+".held") != nil || os.Mkdir(a.configPath(), 0700) != nil {
		t.Fatal("fixture obstruction failed")
	}
	before := configSnapshot(t, a.DataDir)
	n = 0
	if _, e := a.LoadConfig(); e == nil {
		t.Error("directory read succeeded")
	}
	if _, e := a.SaveConfig(map[string]any{"ui_mode": "complete"}); e == nil {
		t.Error("directory update succeeded")
	}
	if n != 0 || !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
		t.Fatal("read refusal mutated fixture")
	}
}

func TestConfigBoundary_BackgroundAndKeystoreRefusal(t *testing.T) {
	a := newSetupApp(t)
	before := configSnapshot(t, a.DataDir)
	cfgMut, ksMut := 0, 0
	a.configIO.readFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	a.configIO.observe = func(string) { cfgMut++ }
	a.keystoreMutationObserver = func(string) { ksMut++ }
	if a.maybeAutoExport(time.Now()) == nil {
		t.Error("automatic export ignored configuration failure")
	}
	if _, e := a.SyncKeystores(); e == nil {
		t.Error("sync ignored configuration failure")
	}
	if _, _, _, e := a.GenerateKey("fixture"); e == nil {
		t.Error("generation ignored configuration failure")
	}
	if _, e := a.Passphrase("fixture"); e == nil {
		t.Error("lookup ignored configuration failure")
	}
	if ok, _ := a.KeystoreStatus()["ok"].(bool); ok {
		t.Error("status reported ready after config failure")
	}
	a.pfVal = map[string]any{"ok": true}
	a.pfAt = time.Now()
	if ok, _ := a.Preflight()["ok"].(bool); ok {
		t.Error("cached preflight concealed current config failure")
	}
	output := t.TempDir()
	if _, e := a.BuildRecoveryKit(output, func(float64, string) {}); e == nil {
		t.Error("recovery kit ignored configuration failure")
	}
	entries, e := os.ReadDir(output)
	if e != nil || len(entries) != 0 {
		t.Error("recovery kit wrote before validation")
	}
	if cfgMut != 0 || ksMut != 0 || !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
		t.Fatal("failed background/keystore reads mutated state")
	}
}

func TestConfigBoundary_JobRecordsReadFailure(t *testing.T) {
	a := newSetupApp(t)
	a.configIO.readFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	// The real recovery-kit route dispatches this same operation through runJob.
	output := t.TempDir()
	result, e := runJob(a, "recovery-kit", "config failure probe", func(p func(float64, string)) (map[string]any, error) { return a.BuildRecoveryKit(output, p) })
	if e != nil {
		t.Fatal("job dispatch failed")
	}
	id := result["job_id"].(int)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j := a.Store.Job(id)
		if j != nil && j.Status != "RUNNING" {
			if j.Status != "FAILED" || !strings.Contains(j.Label, "configuration") {
				t.Fatal("job concealed configuration read failure")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not record terminal failure")
}

func TestConfigBoundary_OptionalToolsUseValidatedSnapshot(t *testing.T) {
	a := newSetupApp(t)
	b, e := os.ReadFile(a.configPath())
	if e != nil {
		t.Fatal("fixture read failed")
	}
	reads := 0
	a.configIO.readFile = func(string) ([]byte, error) {
		reads++
		if reads > 1 {
			return nil, os.ErrPermission
		}
		return b, nil
	}
	if _, e := a.ToolsView(); e != nil || reads != 1 {
		t.Fatal("optional tools concealed a repeated config read")
	}
	reads = 0
	if _, e := a.LoadConfig(); e != nil {
		t.Fatal("snapshot read failed")
	}
	if _, e := a.ToolsView(); e == nil {
		t.Fatal("tools view ignored its actual failed prerequisite read")
	}
}

func TestConfigBoundary_RestoreUsesCheckedPublisher(t *testing.T) {
	src := abTestApp(t)
	bundle, e := src.ExportAppBackup(t.TempDir(), false)
	if e != nil {
		t.Fatal("backup fixture failed")
	}
	dst := newSetupApp(t)
	before, e := os.ReadFile(dst.configPath())
	if e != nil {
		t.Fatal("fixture read failed")
	}
	n := 0
	dst.configIO.observe = func(string) { n++ }
	dst.configIO.rename = func(string, string) error { return errors.New("injected rename failure") }
	result, e := dst.RestoreAppBackup(bundle.TarPath)
	var pe *ConfigPublicationError
	if !errors.As(e, &pe) || pe.Published || n != 1 || !reflect.DeepEqual(result, RestoreResult{}) {
		t.Fatal("restore did not propagate checked config publication failure")
	}
	after, e := os.ReadFile(dst.configPath())
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("failed restore publication changed prior config")
	}
	// Other members may already be restored; no cross-file rollback is asserted.
}

func TestConfigBoundary_HTTPPublicationFailure(t *testing.T) {
	a := newSetupApp(t)
	a.configIO.rename = func(string, string) error { return errors.New("injected rename failure") }
	before := configSnapshot(t, a.DataDir)
	mux := http.NewServeMux()
	api(mux, a)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(`{"ui_mode":"complete"}`)))
	if w.Code < 400 || strings.Contains(w.Body.String(), `"config":`) {
		t.Fatal("API exposed failed settings as successful")
	}
	if !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
		t.Fatal("failed API publication mutated old state")
	}
}

func TestConfigBoundary_InvalidRequestNoMutation(t *testing.T) {
	a := newSetupApp(t)
	n := 0
	a.configIO.observe = func(string) { n++ }
	before := configSnapshot(t, a.DataDir)
	mux := http.NewServeMux()
	api(mux, a)
	for _, body := range []string{"null", "[]", `{"ui_mode":`, `{"ui_mode":42}`} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/config", strings.NewReader(body)))
		if w.Code < 400 {
			t.Error("invalid request accepted")
		}
	}
	if n != 0 || !reflect.DeepEqual(before, configSnapshot(t, a.DataDir)) {
		t.Fatal("invalid request attempted mutation")
	}
}

func TestConfigBoundary_StartupRefusesBeforeCatalogOrBind(t *testing.T) {
	if os.Getenv("OBX_CONFIG_STARTUP_CHILD") == "1" {
		flag.CommandLine = flag.NewFlagSet("config-startup-probe", flag.ExitOnError)
		os.Args = []string{"obelisk", "-data", os.Getenv("OBX_CONFIG_STARTUP_DIR"), "-listen", "0.0.0.0:0"}
		main()
		os.Exit(97)
	}
	for _, kind := range []string{"missing", "malformed"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			if kind == "malformed" {
				if os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"auth_token":"fixture-private-value",`), 0600) != nil {
					t.Fatal("fixture failed")
				}
			}
			before := configSnapshot(t, dir)
			cmd := exec.Command(os.Args[0], "-test.run=^TestConfigBoundary_StartupRefusesBeforeCatalogOrBind$")
			cmd.Env = append(os.Environ(), "OBX_CONFIG_STARTUP_CHILD=1", "OBX_CONFIG_STARTUP_DIR="+dir, "MNEMO_AUTH_TOKEN=fixture-env-token")
			out, e := cmd.CombinedOutput()
			var ee *exec.ExitError
			if !errors.As(e, &ee) || ee.ExitCode() != 1 || !bytes.Contains(out, []byte("configuration")) {
				t.Fatal("actual startup did not fail at configuration gate")
			}
			if bytes.Contains(out, []byte("fixture-private-value")) || bytes.Contains(out, []byte("fixture-env-token")) {
				t.Fatal("startup exposed secret")
			}
			if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
				t.Fatal("startup opened/mutated state after invalid config")
			}
		})
	}
}

func TestConfigBoundary_OptionalDefaultsAndReopen(t *testing.T) {
	a := newSetupApp(t)
	if os.WriteFile(a.configPath(), []byte(`{"UI_MODE":"standard","tools":null,"keystore_paths":null}`), 0600) != nil {
		t.Fatal("fixture failed")
	}
	cfg, e := a.LoadConfig()
	if e != nil || cfg.UIMode != "standard" || cfg.BuildVerify != "full" || cfg.Tools == nil {
		t.Fatal("valid optional fields compatibility failed")
	}
	if _, e := a.SaveConfig(map[string]any{"label_size": ""}); e != nil {
		t.Fatal("clear failed")
	}
	cfg, e = a.LoadConfig()
	if e != nil || cfg.LabelSize != "" || cfg.UIMode != "standard" {
		t.Fatal("optional clear/alias reopen failed")
	}
	b, e := os.ReadFile(a.configPath())
	if e != nil {
		t.Fatal("reopen failed")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(b, &fields) != nil {
		t.Fatal("invalid JSON")
	}
	if _, ok := fields["UI_MODE"]; ok {
		t.Fatal("alias not canonicalized")
	}
}
