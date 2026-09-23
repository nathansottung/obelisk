//go:build !guionly

package main

// hardening_test.go — the local API Host and navigation-site rules, config
// redaction, the minimal helper environment, and cleaning of stored tool output.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

// ---- stand-in helper -----------------------------------------------------------

// The test binary doubles as a stand-in helper. Copied under one of these names
// and run as tar/gpg/par2, it prints to stderr and exits non-zero, so the build
// fails and the printed text flows into the stored error.
const (
	helperDumpEnv = "obelisk-helperdump-env" // reports whether the sentinels reached it
	helperDumpBig = "obelisk-helperdump-big" // prints a large block of binary junk

	sentinelObelisk = "OBELISK_M1_SENTINEL"
	sentinelOther   = "M1_UNLISTED_SENTINEL"
	sentinelValue   = "m1-sentinel-7f3a9c"
)

func init() {
	base := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	switch base {
	case helperDumpEnv:
		report := func(k string) string {
			if _, ok := os.LookupEnv(k); ok {
				return "present"
			}
			return "absent"
		}
		fmt.Fprintf(os.Stderr, "HELPERDUMP env-count=%d\n", len(os.Environ()))
		fmt.Fprintf(os.Stderr, "SENTINEL-OBELISK=%s\nSENTINEL-OTHER=%s\nHELPERDUMP-END\n",
			report(sentinelObelisk), report(sentinelOther))
		os.Exit(3)
	case helperDumpBig:
		junk := []byte("\x00\x1b[31m\xff\xfe junk\r\n\x07")
		for i := 0; i < 20000; i++ {
			os.Stderr.Write(junk)
		}
		os.Stderr.WriteString("caf\xc3\xa9 BIGDUMP-END")
		os.Exit(1)
	}
}

// copySelfAs copies the running test binary to a temp folder under name.
func copySelfAs(t *testing.T, name string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), name)
	if runtime.GOOS == "windows" {
		dst += ".exe"
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

// failBuildWith builds a one-file plaintext package with every archive helper
// pointed at helper, waits for the job, and returns the app, chunk and job.
func failBuildWith(t *testing.T, helper string) (*App, *Chunk, *Job) {
	t.Helper()
	app, _ := newTestApp(t, map[string]string{"tar": helper, "gpg": helper, "par2": helper})
	src := t.TempDir()
	body := []byte("hardening fixture\n")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	c := app.Store.AddChunk(Chunk{
		Name: "M1-HELPER", Status: "PLANNED", MediaKind: "CUSTOM",
		TargetBytes: 1 << 30, DataBytes: int64(len(body)), FileCount: 1,
		SrcRoot: src, HashAlg: "SHA256", Par2: 5, Encrypted: false,
		Files: []ChunkFileRef{{RelPath: "a.txt", SizeBytes: int64(len(body)), Hash: hex.EncodeToString(sum[:])}},
	})
	res, err := runJob(app, "build", "Build M1-HELPER", func(p func(float64, string)) (map[string]any, error) {
		return nil, app.BuildChunk(c.ID, p)
	})
	if err != nil {
		t.Fatal(err)
	}
	id := res["job_id"].(int)
	deadline := time.Now().Add(30 * time.Second)
	for {
		j := app.Store.Job(id)
		if j != nil && j.Status != "RUNNING" {
			return app, app.Store.Chunk(c.ID), j
		}
		if time.Now().After(deadline) {
			t.Fatal("build job did not finish")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func assertCleanStored(t *testing.T, what, s string, max int) {
	t.Helper()
	if len(s) > max {
		t.Errorf("%s: %d bytes, want at most %d", what, len(s), max)
	}
	if !utf8.ValidString(s) {
		t.Errorf("%s: not valid UTF-8", what)
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && !unicode.IsPrint(r) {
			t.Errorf("%s: contains non-printable %U", what, r)
			return
		}
	}
}

// ---- Host rule -----------------------------------------------------------------

func TestAPIGuard_NonLoopbackHostRefused(t *testing.T) {
	ts := guardTS(t)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	for _, host := range []string{"evil.example:" + port, "evil.example", "127.0.0.2:" + port, "127.0.0.1:1", "localhost.evil.example:" + port} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			resp := do(t, ts, method, "/api/config", func(r *http.Request) {
				r.Host = host
				r.Header.Set(requestedByHeader, requestedByValue)
				r.Header.Set("Origin", "http://"+host)
			})
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s with Host %q: status %d, want 403", method, host, resp.StatusCode)
			}
		}
	}
}

func TestAPIGuard_LoopbackHostVariantsPass(t *testing.T) {
	ts := guardTS(t)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	for _, host := range []string{"127.0.0.1:" + port, "localhost:" + port, "LOCALHOST:" + port, "[::1]:" + port} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			resp := do(t, ts, method, "/api/config", func(r *http.Request) {
				r.Host = host
				r.Header.Set(requestedByHeader, requestedByValue)
				r.Header.Set("Origin", "http://"+host)
			})
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s with Host %q: status %d, want 200", method, host, resp.StatusCode)
			}
		}
	}
}

func TestAPIGuard_HostRuleOnlyOnLoopbackConnections(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Host = "backup.example:7821"
	ctx := context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 7821})
	if !loopbackHostAllowed(req.WithContext(ctx)) {
		t.Error("a connection on a non-loopback address must not be restricted by the loopback Host rule")
	}
	ctx = context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 7821})
	if loopbackHostAllowed(req.WithContext(ctx)) {
		t.Error("a loopback connection with a non-loopback Host must be refused")
	}
	if loopbackHostAllowed(req) {
		t.Error("a request with no local address must be refused")
	}
}

// ---- navigation-site rule --------------------------------------------------------

func TestAPIGuard_CrossSiteNavigationRefused(t *testing.T) {
	ts := guardTS(t)
	cases := map[string]int{
		"cross-site":  http.StatusForbidden,
		"same-site":   http.StatusForbidden,
		"same-origin": http.StatusOK,
		"none":        http.StatusOK,
		"":            http.StatusOK, // no Fetch Metadata: the Origin/Referer rule still applies
	}
	for site, want := range cases {
		resp := do(t, ts, http.MethodGet, "/api/chunks", func(r *http.Request) {
			r.Header.Set("Sec-Fetch-Mode", "navigate")
			if site != "" {
				r.Header.Set("Sec-Fetch-Site", site)
			}
		})
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("navigation with Sec-Fetch-Site %q: status %d, want %d", site, resp.StatusCode, want)
		}
	}
}

// ---- config redaction ------------------------------------------------------------

func TestConfigAPI_NeverReturnsToken(t *testing.T) {
	a := newSetupApp(t)
	const token = "m1-token-value-4d8e2b"
	if _, err := a.SaveConfig(map[string]any{"auth_token": token}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api(mux, a)
	get := func(path string) string {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d", path, w.Code)
		}
		return w.Body.String()
	}
	body := get("/api/config")
	if strings.Contains(body, token) {
		t.Fatal("GET /api/config returned the token value")
	}
	var resp struct {
		AuthTokenSet bool `json:"auth_token_set"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.AuthTokenSet {
		t.Fatalf("GET /api/config must report auth_token_set=true (err=%v)", err)
	}
	if strings.Contains(get("/api/setup"), token) {
		t.Fatal("GET /api/setup returned the token value")
	}
	// A settings save that does not mention the token keeps it and does not echo it.
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"ui_mode":"complete"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/config: status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), token) {
		t.Fatal("PUT /api/config returned the token value")
	}
	if got := mustConfig(t, a).AuthToken; got != token {
		t.Fatalf("stored token changed by an unrelated settings save: %q", got)
	}
}

// ---- helper environment ------------------------------------------------------------

func TestHelperEnv_WithholdsUnlistedVariables(t *testing.T) {
	t.Setenv(sentinelObelisk, sentinelValue)
	t.Setenv(sentinelOther, sentinelValue)
	t.Setenv("MNEMO_AUTH_TOKEN", sentinelValue)
	for _, keep := range []bool{false, true} {
		env := helperEnv(filepath.Join(t.TempDir(), "tool"), keep)
		joined := strings.Join(env, "\n")
		if strings.Contains(joined, sentinelValue) {
			t.Fatalf("keepPath=%v: helper environment carries a withheld variable", keep)
		}
		hasPath := false
		for _, kv := range env {
			if strings.HasPrefix(kv, "PATH=") {
				hasPath = true
			}
		}
		if !hasPath {
			t.Fatalf("keepPath=%v: helper environment has no PATH", keep)
		}
	}
}

func TestHelperEnv_SentinelNeverReachesHelperOrRecords(t *testing.T) {
	t.Setenv(sentinelObelisk, sentinelValue)
	t.Setenv(sentinelOther, sentinelValue)
	dump := copySelfAs(t, helperDumpEnv)

	// Control: with the full environment, the stand-in helper does see both.
	out, _ := exec.Command(dump).CombinedOutput()
	if !strings.Contains(string(out), "SENTINEL-OBELISK=present") || !strings.Contains(string(out), "SENTINEL-OTHER=present") {
		t.Fatalf("control run did not see the sentinels; the probe is not working: %s", out)
	}

	app, c, j := failBuildWith(t, dump)
	if c.Status != "FAILED" || j.Status != "FAILED" {
		t.Fatalf("chunk %s / job %s, want both FAILED", c.Status, j.Status)
	}
	// The helper ran with the helper environment and its report reached the record.
	if !strings.Contains(c.Error, "SENTINEL-OBELISK=absent") || !strings.Contains(c.Error, "SENTINEL-OTHER=absent") {
		t.Fatalf("stored error does not show the helper's report: %q", c.Error)
	}

	mux := http.NewServeMux()
	api(mux, app)
	httpBody := func(path string) string {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w.Body.String()
	}
	catalog, _ := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	jobs, _ := os.ReadFile(filepath.Join(app.DataDir, "jobs.json"))
	for what, s := range map[string]string{
		"chunk error":      c.Error,
		"job label":        j.Label,
		"catalog.json":     string(catalog),
		"jobs.json":        string(jobs),
		"GET /api/chunks":  httpBody(fmt.Sprintf("/api/chunks/%d", c.ID)),
		"GET /api/jobs":    httpBody("/api/jobs"),
		"GET /api/job one": httpBody(fmt.Sprintf("/api/jobs/%d", j.ID)),
	} {
		if strings.Contains(s, sentinelValue) {
			t.Errorf("%s contains the sentinel value", what)
		}
	}
}

// ---- stored tool text ----------------------------------------------------------------

func TestCleanToolText_CapsAndCleans(t *testing.T) {
	junk := strings.Repeat("\x00\x1b[31m\xff\xfe junk\r\n\x07", 5000) + "café ★ end"
	for _, max := range []int{toolOutputMax, storedErrorMax} {
		got := cleanToolText(junk, max)
		assertCleanStored(t, fmt.Sprintf("cleanToolText max=%d", max), got, max)
		if !strings.HasSuffix(got, "[truncated]") {
			t.Errorf("max=%d: oversized text is not marked as truncated", max)
		}
	}
	if got := cleanToolText("café ★\tok\nline", 0); got != "café ★\tok\nline" {
		t.Errorf("printable text, tab and newline must survive unchanged, got %q", got)
	}
	if got := toolOutputTail([]byte(junk)); !strings.HasSuffix(got, "café ★ end") {
		t.Errorf("toolOutputTail must keep the end of the output, got %q", got)
	}
}

func TestStoredErrors_OversizedBinaryHelperOutputIsCappedAndClean(t *testing.T) {
	big := copySelfAs(t, helperDumpBig)
	_, c, j := failBuildWith(t, big)
	if c.Status != "FAILED" {
		t.Fatalf("chunk status %s, want FAILED", c.Status)
	}
	if !strings.Contains(c.Error, "BIGDUMP-END") {
		t.Fatalf("stored error lost the end of the helper output: %q", c.Error)
	}
	assertCleanStored(t, "chunk error", c.Error, storedErrorMax)
	prefix := "Build M1-HELPER — ERROR: "
	if !strings.HasPrefix(j.Label, prefix) {
		t.Fatalf("unexpected job label %q", j.Label)
	}
	assertCleanStored(t, "job label error", strings.TrimPrefix(j.Label, prefix), storedErrorMax)
}

func TestJSONErr_CapsAndCleans(t *testing.T) {
	w := httptest.NewRecorder()
	jsonErr(w, http.StatusBadRequest, errors.New(strings.Repeat("\x00\xff\x1b bad ", 3000)))
	raw, _ := io.ReadAll(w.Body)
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	assertCleanStored(t, "HTTP error", body["error"], storedErrorMax)
}
