//go:build !guionly

package main

// session_auth_test.go: every route needs a signed-in session. The first test runs
// the real server binary and calls it as another local program would, including
// with the public X-Requested-By header.

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type noTokenServer struct {
	base    string
	port    int
	out     *syncBuffer
	env     []string
	profile string
}

// startNoTokenServer runs the real binary with no token and no browser. Its per-user
// folders point into the test's own temp folder.
func startNoTokenServer(t *testing.T, bin, dataDir, engine string) *noTokenServer {
	t.Helper()
	port := freePort(t)
	cmd := exec.Command(bin, "-data", dataDir, "-init-config", "-listen", fmt.Sprintf("127.0.0.1:%d", port))
	profile := t.TempDir()
	var env []string
	for _, kv := range os.Environ() {
		switch strings.ToUpper(strings.SplitN(kv, "=", 2)[0]) {
		case strings.ToUpper(catalogEngineEnv), "OBELISK_AUTH_TOKEN", "MNEMO_AUTH_TOKEN", "LOCALAPPDATA", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR":
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "OBELISK_NO_BROWSER=1", "LOCALAPPDATA="+profile, "XDG_CACHE_HOME="+profile)
	if engine != "" {
		env = append(env, catalogEngineEnv+"="+engine)
	}
	cmd.Env = env
	out := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-done })

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			t.Fatalf("the server exited:\n%s", out)
		default:
		}
		if res, err := http.Get(base + "/"); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return &noTokenServer{base: base, port: port, out: out, env: env, profile: profile}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the server did not answer:\n%s", out)
	return nil
}

func TestNoToken_LocalProgramWithoutSessionIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	bin := buildBinary(t, t.TempDir(), "obelisk-session-e2e", "")
	for _, engine := range ownerEngines() {
		name := engine
		if name == "" {
			name = "database"
		}
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			srv := startNoTokenServer(t, bin, dataDir, engine)
			configPath := filepath.Join(dataDir, "config.json")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			routes := []struct{ name, method, path, body string }{
				{"read", http.MethodGet, "/api/config", ""},
				{"write", http.MethodPut, "/api/config", `{"buffer_gb":3}`},
			}
			callers := []struct {
				name    string
				headers map[string]string
				want    int
			}{
				{"no headers", nil, http.StatusForbidden},
				{"public header", map[string]string{requestedByHeader: requestedByValue, "Content-Type": "application/json"}, http.StatusUnauthorized},
			}
			for _, route := range routes {
				for _, caller := range callers {
					req, err := http.NewRequest(route.method, srv.base+route.path, strings.NewReader(route.body))
					if err != nil {
						t.Fatal(err)
					}
					for k, v := range caller.headers {
						req.Header.Set(k, v)
					}
					res, err := http.DefaultClient.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					_, _ = io.Copy(io.Discard, res.Body)
					res.Body.Close()
					if res.StatusCode != caller.want {
						t.Errorf("%s with %s: got HTTP %d, want %d\n%s", route.name, caller.name, res.StatusCode, caller.want, srv.out)
					}
				}
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Errorf("a caller with no session changed the saved settings")
			}
		})
	}
}

// The way back in, through the real binary: -signin gets a link for the same account,
// the link gives a working session once, and the session reaches the API.
func TestSignIn_CommandGivesTheSameAccountAWayBackIn(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	bin := buildBinary(t, t.TempDir(), "obelisk-signin-e2e", "")
	for _, engine := range ownerEngines() {
		name := engine
		if name == "" {
			name = "database"
		}
		t.Run(name, func(t *testing.T) {
			srv := startNoTokenServer(t, bin, t.TempDir(), engine)
			cmd := exec.Command(bin, "-signin", "-print-link", "-listen", fmt.Sprintf("127.0.0.1:%d", srv.port))
			cmd.Env = srv.env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("-signin: %v\n%s\nserver:\n%s", err, out, srv.out)
			}
			link := regexp.MustCompile(`http://\S+`).FindString(string(out))
			if link == "" {
				t.Fatalf("-signin printed no link: %s", out)
			}
			noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			res, err := noFollow.Get(link)
			if err != nil {
				t.Fatal(err)
			}
			session := readSignInPage(t, res)
			req := uiCall(http.MethodGet, srv.base+"/api/config", session)
			got, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			got.Body.Close()
			if got.StatusCode != http.StatusOK {
				t.Fatalf("a signed-in read answered %d", got.StatusCode)
			}
			again, err := noFollow.Get(link)
			if err != nil {
				t.Fatal(err)
			}
			again.Body.Close()
			if again.StatusCode != http.StatusUnauthorized {
				t.Fatalf("a used link answered %d", again.StatusCode)
			}
			if strings.Contains(srv.out.String(), strings.SplitN(link, "code=", 2)[1]) {
				t.Fatal("the server log holds the sign-in code")
			}
		})
	}
}

// ---- routes, built from the mux itself ------------------------------------

func sessionTestApp(t *testing.T) *App {
	t.Helper()
	dataDir := t.TempDir()
	store, err := openTestStore(t, dataDir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	return initializedTestApp(t, &App{DataDir: dataDir, Store: store})
}

var routeWildcard = regexp.MustCompile(`\{[^}]*\}`)

// routeSample turns a registered pattern into a request that matches it.
func routeSample(pattern string) (method, path string) {
	method, path = http.MethodGet, pattern
	if f := strings.Fields(pattern); len(f) == 2 {
		method, path = f[0], f[1]
	}
	path = strings.ReplaceAll(path, "{$}", "")
	return method, routeWildcard.ReplaceAllString(path, "1")
}

// browserSession is what a signed-in browser holds.
type browserSession struct {
	cookie *http.Cookie
	key    string
}

func uiCall(method, target string, s *browserSession) *http.Request {
	req, _ := http.NewRequest(method, target, strings.NewReader("{}"))
	req.Header.Set(requestedByHeader, requestedByValue)
	req.Header.Set("Content-Type", "application/json")
	if s != nil {
		req.AddCookie(s.cookie)
		req.Header.Set(sessionKeyHeader, s.key)
	}
	return req
}

var signInPageKey = regexp.MustCompile(`obelisk_page_key','([0-9a-f]{64})'`)

// readSignInPage checks a successful sign-in answer and returns its session.
func readSignInPage(t *testing.T, res *http.Response) *browserSession {
	t.Helper()
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	m := signInPageKey.FindSubmatch(b)
	if res.StatusCode != http.StatusOK || len(res.Cookies()) != 1 || m == nil || !bytes.Contains(b, []byte("location.replace('/')")) {
		t.Fatalf("sign-in: HTTP %d with %d cookies: %s", res.StatusCode, len(res.Cookies()), b)
	}
	return &browserSession{cookie: res.Cookies()[0], key: string(m[1])}
}

func TestRoutes_EveryRegisteredRouteNeedsASession(t *testing.T) {
	app := sessionTestApp(t)
	_, patterns := newAppMux(app)
	if len(patterns) < 100 {
		t.Fatalf("only %d routes recorded; the recording mux is not seeing them", len(patterns))
	}
	auth, err := newSessionAuth("", 7821)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(serverHandler(app, auth))
	defer ts.Close()
	for _, p := range patterns {
		if p == "/" {
			continue
		}
		method, path := routeSample(p)
		res, err := http.DefaultClient.Do(uiCall(method, ts.URL+path, nil))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s with no session answered %d, want 401", p, res.StatusCode)
		}
	}
	// Anything not registered and not a UI file is refused too, so a new route is
	// covered before anyone adds it to a list.
	for _, path := range []string{"/downloads/x", "/reports/1", "/labels/1", "/exports/a.csv", "//api/config"} {
		res, err := http.DefaultClient.Do(uiCall(http.MethodGet, ts.URL+path, nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("unlisted %s answered %d, want 401", path, res.StatusCode)
		}
	}
}

// A valid session reaches every route. The handlers are stand-ins on the same
// patterns, so this test starts no real job.
func TestRoutes_ValidSessionIsAcceptedOnEveryRoute(t *testing.T) {
	_, patterns := newAppMux(sessionTestApp(t))
	stub := http.NewServeMux()
	for _, p := range patterns {
		if p != "/" {
			stub.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
		}
	}
	auth, err := newSessionAuth("", 7821)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(apiGuard(auth.middleware(stub, func(string) bool { return false })))
	defer ts.Close()
	c := signInCookie(t, ts.URL, auth)
	for _, p := range patterns {
		if p == "/" {
			continue
		}
		method, path := routeSample(p)
		res, err := http.DefaultClient.Do(uiCall(method, ts.URL+path, c))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusTeapot {
			t.Errorf("%s with a session answered %d", p, res.StatusCode)
		}
	}
}

func TestUIShell_LoadsWithoutSessionAndHoldsNoUserData(t *testing.T) {
	app := sessionTestApp(t)
	const sentinel = "SENTINEL-PRIVATE-ARCHIVE-NAME"
	if app.Store.AddCollection(sentinel) == nil {
		t.Fatal("cannot seed an archive")
	}
	auth, _ := newSessionAuth("", 7821)
	ts := httptest.NewServer(serverHandler(app, auth))
	defer ts.Close()
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || !bytes.Contains(b, []byte("<script>")) {
		t.Fatalf("the UI shell did not load: %d", res.StatusCode)
	}
	if bytes.Contains(b, []byte(sentinel)) {
		t.Fatal("the public UI shell contains catalog data")
	}
	if bytes.Contains(b, []byte("'token='")) {
		t.Fatal("the UI still builds a URL with the token in it")
	}
}

// ---- links and cookies -----------------------------------------------------

func signInClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func signInCookie(t *testing.T, base string, auth *sessionAuth) *browserSession {
	t.Helper()
	code, err := auth.mintCode()
	if err != nil {
		t.Fatal(err)
	}
	res, err := signInClient().Get(base + signInPath + "?code=" + url.QueryEscape(code))
	if err != nil {
		t.Fatal(err)
	}
	return readSignInPage(t, res)
}

func sessionGateServer(t *testing.T, auth *sessionAuth) *httptest.Server {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	ts := httptest.NewServer(apiGuard(auth.middleware(inner, func(p string) bool { return p == "/" })))
	t.Cleanup(ts.Close)
	return ts
}

func TestSignIn_LinkWorksOnceThenIsRefused(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	code, _ := auth.mintCode()
	link := ts.URL + signInPath + "?code=" + url.QueryEscape(code)
	first, err := signInClient().Get(link)
	if err != nil {
		t.Fatal(err)
	}
	readSignInPage(t, first)
	second, err := signInClient().Get(link)
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusUnauthorized || len(second.Cookies()) != 0 {
		t.Fatalf("second use: %d with %d cookies", second.StatusCode, len(second.Cookies()))
	}
}

func TestSignIn_ExpiredLinkIsRefused(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	now := time.Now()
	auth.now = func() time.Time { return now }
	ts := sessionGateServer(t, auth)
	code, _ := auth.mintCode()
	now = now.Add(signInLinkLifetime)
	res, err := signInClient().Get(ts.URL + signInPath + "?code=" + url.QueryEscape(code))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized || len(res.Cookies()) != 0 {
		t.Fatalf("expired link: %d with %d cookies", res.StatusCode, len(res.Cookies()))
	}
}

func TestSession_CookieFlags(t *testing.T) {
	auth, _ := newSessionAuth("", 43210)
	ts := sessionGateServer(t, auth)
	c := signInCookie(t, ts.URL, auth).cookie
	if c.Name != auth.cookieName() || !strings.HasPrefix(c.Name, "obelisk_session_") {
		t.Errorf("unexpected cookie name %q", c.Name)
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != "" {
		t.Errorf("flags: HttpOnly=%v SameSite=%v Path=%q Domain=%q", c.HttpOnly, c.SameSite, c.Path, c.Domain)
	}
}

func TestSession_ExpiresAndCannotBeAltered(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	now := time.Now()
	auth.now = func() time.Time { return now }
	ts := sessionGateServer(t, auth)
	session := signInCookie(t, ts.URL, auth)
	c := session.cookie
	status := func(v string) int {
		res, err := http.DefaultClient.Do(uiCall(http.MethodGet, ts.URL+"/api/x",
			&browserSession{cookie: &http.Cookie{Name: c.Name, Value: v}, key: session.key}))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if got := status(c.Value); got != http.StatusOK {
		t.Fatalf("valid cookie: %d", got)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(c.Value)
	raw[0] ^= 1
	if got := status(base64.RawURLEncoding.EncodeToString(raw)); got != http.StatusUnauthorized {
		t.Errorf("altered cookie: %d", got)
	}
	now = now.Add(sessionLifetime)
	if got := status(c.Value); got != http.StatusUnauthorized {
		t.Errorf("expired cookie: %d", got)
	}
}

func TestSession_SecondInstanceCookieDoesNotWork(t *testing.T) {
	first, _ := newSessionAuth("", 7821)
	second, _ := newSessionAuth("", 7821) // same port, as after a restart
	firstTS, secondTS := sessionGateServer(t, first), sessionGateServer(t, second)
	c := signInCookie(t, secondTS.URL, second)
	res, err := http.DefaultClient.Do(uiCall(http.MethodGet, firstTS.URL+"/api/x", c))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("another instance's cookie answered %d", res.StatusCode)
	}
}

// ---- configured token ------------------------------------------------------

func TestToken_ExchangeGivesAOneTimeLinkAndURLTokenIsRefused(t *testing.T) {
	auth, _ := newSessionAuth("fixture-configured-token", 7821)
	ts := sessionGateServer(t, auth)
	req := uiCall(http.MethodPost, ts.URL+sessionExchangePath, nil)
	if res, err := http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	} else if res.Body.Close(); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("exchange with no token: %d", res.StatusCode)
	}
	req = uiCall(http.MethodPost, ts.URL+sessionExchangePath, nil)
	req.Header.Set("Authorization", "Bearer fixture-configured-token")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || !strings.HasPrefix(out.Link, signInPath+"?code=") {
		t.Fatalf("exchange: %v %+v", err, out)
	}
	if strings.Contains(out.Link, "fixture-configured-token") {
		t.Fatal("the link holds the token")
	}
	signed, err := signInClient().Get(ts.URL + out.Link)
	if err != nil {
		t.Fatal(err)
	}
	readSignInPage(t, signed)
	byURL, err := http.DefaultClient.Do(uiCall(http.MethodGet, ts.URL+"/api/x?token=fixture-configured-token", nil))
	if err != nil {
		t.Fatal(err)
	}
	byURL.Body.Close()
	if byURL.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a token in the URL answered %d", byURL.StatusCode)
	}
}

// ---- the way back in --------------------------------------------------------

func isolateSigninFolder(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("XDG_RUNTIME_DIR", "")
}

func TestReenter_WrongProofAndReplayAreRefused(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	nonce := strings.Repeat("ab", 32)
	post := func(proof string) int {
		b, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": proof})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+sessionReenterPath, bytes.NewReader(b))
		req.Header.Set(requestedByHeader, requestedByValue)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if got := post(strings.Repeat("0", 64)); got != http.StatusUnauthorized {
		t.Errorf("wrong proof: %d", got)
	}
	good := reentryClientProof(auth.reentryKey(), auth.addr, nonce)
	if got := post(good); got != http.StatusOK {
		t.Errorf("right proof: %d", got)
	}
	if got := post(good); got != http.StatusUnauthorized {
		t.Errorf("replayed proof: %d", got)
	}
}

// A program that took over the port cannot pass as Obelisk, so no browser opens.
func TestSignInCommand_RefusesAnImpostorOnThePort(t *testing.T) {
	isolateSigninFolder(t)
	impostor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		jsonOut(w, map[string]string{"sealed": "00ff", "proof": strings.Repeat("0", 64)})
	}))
	defer impostor.Close()
	u, _ := url.Parse(impostor.URL)
	port, _ := strconv.Atoi(u.Port())
	auth, _ := newSessionAuth("", port)
	auth.addr = u.Host
	if _, err := auth.publishSigninFile(); err != nil {
		t.Fatal(err)
	}
	opened := false
	old := openBrowserAt
	openBrowserAt = func(string) error { opened = true; return nil }
	defer func() { openBrowserAt = old }()
	var out, errOut bytes.Buffer
	if code := runSignIn(u.Host, false, &out, &errOut); code == 0 {
		t.Fatalf("-signin accepted an impostor: %s", out.String())
	}
	if opened {
		t.Fatal("a browser was opened for an impostor")
	}
}

// Where other accounts can read a command line, the browser gets a private local
// page instead of the link. On Windows the link goes to the default browser directly.
func TestSignIn_BrowserCommandLineHoldsNoLink(t *testing.T) {
	isolateSigninFolder(t)
	var target string
	old := openBrowserAt
	openBrowserAt = func(s string) error { target = s; return nil }
	defer func() { openBrowserAt = old }()
	const link = "http://127.0.0.1:7821/auth/signin?code=SENTINELCODE"
	page, err := openSignInLink(link)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		if target != link || page != "" {
			t.Fatalf("Windows target %q, page %q", target, page)
		}
		return
	}
	if strings.Contains(target, "SENTINELCODE") || !strings.HasPrefix(target, "file:") {
		t.Fatalf("browser target: %q", target)
	}
	b, err := os.ReadFile(page)
	if err != nil || !strings.Contains(string(b), "SENTINELCODE") {
		t.Fatalf("forwarding page: %v %q", err, b)
	}
	if err := verifyPrivateFile(page); err != nil {
		t.Fatalf("the forwarding page is not private: %v", err)
	}
}

func TestSigninFile_IsPrivateAndRefusedIfOpened(t *testing.T) {
	isolateSigninFolder(t)
	auth, _ := newSessionAuth("", 7821)
	path, err := auth.publishSigninFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateFile(path); err != nil {
		t.Fatalf("the sign-in file is not private: %v", err)
	}
	if err := ensurePrivateDir(filepath.Dir(path)); err != nil {
		t.Fatalf("the sign-in folder is not private: %v", err)
	}
	if _, err := readSigninFile(auth.addr); err != nil {
		t.Fatalf("the owner cannot read it: %v", err)
	}
	if runtime.GOOS == "windows" {
		if b, err := exec.Command("icacls", path, "/grant", "*S-1-1-0:(R)").CombinedOutput(); err != nil {
			t.Skipf("cannot change the file's ACL here: %v %s", err, b)
		}
	} else if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readSigninFile(auth.addr); err == nil {
		t.Fatal("a sign-in file that others can read was trusted")
	}
}

// A real second account, where the host can run one without new privileges.
func TestSigninFile_SecondAccountCannotRead(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() != 0 {
		t.Skip("needs root on Linux to run as a second account; Windows is covered by the ACL checks")
	}
	isolateSigninFolder(t)
	auth, _ := newSessionAuth("", 7821)
	path, err := auth.publishSigninFile()
	if err != nil {
		t.Fatal(err)
	}
	if err := runAsNobody("cat", path); err == nil {
		t.Fatal("a second account read the sign-in file")
	}
}

// ---- the secret stays out of everything -------------------------------------

func TestSecret_AppearsInNoLogJobRecordOrBackup(t *testing.T) {
	isolateSigninFolder(t)
	app := sessionTestApp(t)
	auth, _ := newSessionAuth("", 7821)
	copy(auth.secret[:], "SENTINEL-INSTANCE-SECRET-32BYTES")

	var logged syncBuffer
	oldOut := log.Writer()
	log.SetOutput(&logged)
	defer log.SetOutput(oldOut)

	ts := httptest.NewServer(serverHandler(app, auth))
	defer ts.Close()
	signinFile, err := auth.publishSigninFile()
	if err != nil {
		t.Fatal(err)
	}
	code, _ := auth.mintCode()
	res, err := signInClient().Get(ts.URL + signInPath + "?code=" + url.QueryEscape(code))
	if err != nil {
		t.Fatal(err)
	}
	session := readSignInPage(t, res)
	cookie := session.cookie

	call := func(method, path string, body any) []byte {
		b, _ := json.Marshal(body)
		req := uiCall(method, ts.URL+path, session)
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.ContentLength = int64(len(b))
		req.Header.Set(sessionHeader, "sentinel-test-browser-0123456789")
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		out, _ := io.ReadAll(r.Body)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: %d %s", method, path, r.StatusCode, out)
		}
		return out
	}
	var created struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(call(http.MethodPost, "/api/collections", map[string]any{"name": "Sentinel"}), &created); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	call(http.MethodPost, fmt.Sprintf("/api/collections/%d/scan", created.ID), map[string]any{"path": src})
	deadline := time.Now().Add(30 * time.Second)
	for bytes.Contains(call(http.MethodGet, "/api/jobs", nil), []byte(`"RUNNING"`)) {
		if time.Now().After(deadline) {
			t.Fatal("the scan job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	backupDir := t.TempDir()
	call(http.MethodPost, "/api/appbackup/export", map[string]any{"dest_dir": backupDir})

	secretRaw := string(auth.secret[:])
	needles := map[string]string{
		"secret":         secretRaw,
		"secret (hex)":   hex.EncodeToString(auth.secret[:]),
		"secret (b64)":   base64.StdEncoding.EncodeToString(auth.secret[:]),
		"re-entry key":   auth.reentryKey(),
		"session cookie": cookie.Value,
		"page key":       session.key,
		"sign-in code":   code,
	}
	for label, n := range needles {
		if strings.Contains(logged.String(), n) {
			t.Errorf("the log holds the %s", label)
		}
		for _, dir := range []string{app.DataDir, backupDir} {
			_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if b, e := os.ReadFile(p); e == nil && bytes.Contains(b, []byte(n)) {
					t.Errorf("%s holds the %s", p, label)
				}
				return nil
			})
		}
	}
	file, err := os.ReadFile(signinFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"secret", "secret (hex)", "secret (b64)"} {
		if bytes.Contains(file, []byte(needles[label])) {
			t.Errorf("the sign-in file holds the %s", label)
		}
	}
	if strings.HasPrefix(filepath.Clean(signinFile), filepath.Clean(app.DataDir)) {
		t.Error("the sign-in file is inside the data folder")
	}
}
