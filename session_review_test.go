//go:build !guionly

package main

// session_review_test.go: a cookie collected on another port is not a session, the
// way back in cannot be relayed, and the sign-in screens do not trap the user.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func gateStatus(t *testing.T, req *http.Request) int {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// Browsers send a host's cookies to all of its ports, so a program on another port
// can collect the cookie. It must not be enough.
func TestSession_CookieAloneIsNotASession(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	s := signInCookie(t, ts.URL, auth)
	stolen := uiCall(http.MethodGet, ts.URL+"/api/x", &browserSession{cookie: s.cookie})
	stolen.Header.Del(sessionKeyHeader)
	if got := gateStatus(t, stolen); got != http.StatusUnauthorized {
		t.Fatalf("a cookie with no page key answered %d", got)
	}
	nav := uiCall(http.MethodGet, ts.URL+"/api/x", &browserSession{cookie: s.cookie})
	nav.Header.Del(sessionKeyHeader)
	nav.Header.Del(requestedByHeader)
	nav.Header.Set("Sec-Fetch-Mode", "navigate")
	if got := gateStatus(t, nav); got != http.StatusUnauthorized {
		t.Fatalf("a navigation with only the cookie answered %d", got)
	}
	wrong := uiCall(http.MethodGet, ts.URL+"/api/x", &browserSession{cookie: s.cookie, key: strings.Repeat("0", 64)})
	if got := gateStatus(t, wrong); got != http.StatusUnauthorized {
		t.Fatalf("a wrong page key answered %d", got)
	}
	if got := gateStatus(t, uiCall(http.MethodGet, ts.URL+"/api/x", s)); got != http.StatusOK {
		t.Fatalf("the full session answered %d", got)
	}
}

func issueTicket(t *testing.T, base string, s *browserSession, target string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"url": target})
	req, _ := http.NewRequest(http.MethodPost, base+sessionTicketPath, bytes.NewReader(b))
	req.Header.Set(requestedByHeader, requestedByValue)
	if s != nil {
		req.AddCookie(s.cookie)
		req.Header.Set(sessionKeyHeader, s.key)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("ticket: HTTP %d", res.StatusCode)
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out.URL
}

func TestDownloadTicket_WorksOnceForOneURLAndOneSession(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	s := signInCookie(t, ts.URL, auth)
	other := signInCookie(t, ts.URL, auth)
	nav := func(target string, c *http.Cookie) int {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+target, nil)
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		if c != nil {
			req.AddCookie(c)
		}
		return gateStatus(t, req)
	}
	link := issueTicket(t, ts.URL, s, "/api/x/report?format=csv")
	if strings.Contains(link, s.key) || strings.Contains(link, s.cookie.Value) {
		t.Fatal("the download address holds a session credential")
	}
	if got := nav(link, s.cookie); got != http.StatusOK {
		t.Fatalf("first use: %d", got)
	}
	if got := nav(link, s.cookie); got != http.StatusUnauthorized {
		t.Fatalf("second use: %d", got)
	}
	if got := nav(issueTicket(t, ts.URL, s, "/api/x/report"), nil); got != http.StatusUnauthorized {
		t.Fatalf("a ticket with no cookie: %d", got)
	}
	if got := nav(issueTicket(t, ts.URL, s, "/api/x/report"), other.cookie); got != http.StatusUnauthorized {
		t.Fatalf("a ticket from another session: %d", got)
	}
	moved := strings.Replace(issueTicket(t, ts.URL, s, "/api/x/report"), "/report", "/secret", 1)
	if got := nav(moved, s.cookie); got != http.StatusUnauthorized {
		t.Fatalf("a ticket used on another URL: %d", got)
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+sessionTicketPath, strings.NewReader(`{"url":"/api/x"}`))
	req.Header.Set(requestedByHeader, requestedByValue)
	req.AddCookie(s.cookie)
	if got := gateStatus(t, req); got != http.StatusUnauthorized {
		t.Fatalf("a ticket requested with only the cookie: %d", got)
	}
}

func TestSession_PlantedCookieCannotHideTheRealOne(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	s := signInCookie(t, ts.URL, auth)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/x", nil)
	req.Header.Set(requestedByHeader, requestedByValue)
	req.Header.Set(sessionKeyHeader, s.key)
	req.Header.Set("Cookie", s.cookie.Name+"=junk; "+s.cookie.Name+"="+s.cookie.Value)
	if got := gateStatus(t, req); got != http.StatusOK {
		t.Fatalf("a planted cookie locked the owner out: %d", got)
	}
}

func TestSession_InstancesOnOnePortUseDifferentCookieNames(t *testing.T) {
	a, _ := newSessionAuth("token-a", 7821)
	b, _ := newSessionAuth("token-b", 7821)
	if a.cookieName() == b.cookieName() {
		t.Fatalf("two instances share the cookie name %q", a.cookieName())
	}
}

// A relay on another loopback address must get neither a working proof nor a code.
func TestReenter_RelayToAnotherAddressFails(t *testing.T) {
	real, _ := newSessionAuth("", 7821)
	real.addr = "[::1]:7821"
	ts := sessionGateServer(t, real)
	key := real.reentryKey()
	nonce := strings.Repeat("cd", 32)
	// The owner's client aimed at 127.0.0.1:7821, where the relay is listening.
	proof := reentryClientProof(key, "127.0.0.1:7821", nonce)
	b, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": proof})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+sessionReenterPath, bytes.NewReader(b))
	req.Header.Set(requestedByHeader, requestedByValue)
	if got := gateStatus(t, req); got != http.StatusUnauthorized {
		t.Fatalf("a proof made for another address answered %d", got)
	}
}

func TestReenter_AnswerHoldsNoUsableCode(t *testing.T) {
	auth, _ := newSessionAuth("", 7821)
	ts := sessionGateServer(t, auth)
	nonce := strings.Repeat("ef", 32)
	b, _ := json.Marshal(map[string]string{"nonce": nonce, "proof": reentryClientProof(auth.reentryKey(), auth.addr, nonce)})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+sessionReenterPath, bytes.NewReader(b))
	req.Header.Set(requestedByHeader, requestedByValue)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]string
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["code"] != "" || out["sealed"] == "" {
		t.Fatalf("the answer: %v", out)
	}
	for _, candidate := range []string{out["sealed"], out["proof"]} {
		got, err := signInClient().Get(ts.URL + signInPath + "?code=" + url.QueryEscape(candidate))
		if err != nil {
			t.Fatal(err)
		}
		got.Body.Close()
		if got.StatusCode != http.StatusUnauthorized {
			t.Fatalf("a value seen on the wire signed in: %d", got.StatusCode)
		}
	}
}

// -signin looks only at the exact address, so it never talks to a relay on a
// neighbouring address when the owner's Obelisk is elsewhere.
func TestSignInCommand_DoesNotCallAnotherAddress(t *testing.T) {
	isolateSigninFolder(t)
	var calls atomic.Int32
	relay := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer relay.Close()
	u, _ := url.Parse(relay.URL)
	port, _ := strconv.Atoi(u.Port())
	owner, _ := newSessionAuth("", port)
	owner.addr = "[::1]:" + u.Port()
	if _, err := owner.publishSigninFile(); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runSignIn(u.Host, true, &out, &errOut); code == 0 {
		t.Fatalf("-signin succeeded against another address: %s", out.String())
	}
	if calls.Load() != 0 {
		t.Fatal("-signin contacted a program on another address")
	}
}

func TestSignIn_DesktopOpenerKeepsTheSessionVariables(t *testing.T) {
	env := desktopSessionEnv(func(k string) string {
		if k == "OBELISK_AUTH_TOKEN" {
			return "must-not-pass"
		}
		return "value"
	})
	joined := strings.Join(env, "\n")
	for _, k := range []string{"DISPLAY=", "WAYLAND_DISPLAY=", "XAUTHORITY=", "XDG_DATA_DIRS=", "XDG_CURRENT_DESKTOP=", "DESKTOP_SESSION=", "DBUS_SESSION_BUS_ADDRESS="} {
		if !strings.Contains(joined, k) {
			t.Errorf("the opener loses %s", k)
		}
	}
	if strings.Contains(joined, "must-not-pass") {
		t.Error("the opener received the token")
	}
}

// Whatever happens to the browser launch, the way in is printed.
func TestSignIn_RecoveryCommandIsAlwaysPrinted(t *testing.T) {
	isolateSigninFolder(t)
	t.Setenv("OBELISK_NO_BROWSER", "")
	old := openBrowserAt
	openBrowserAt = func(string) error { return nil }
	defer func() { openBrowserAt = old }()
	auth, _ := newSessionAuth("", 7821)
	var lines []string
	auth.startOwnerSignIn("http://127.0.0.1:7821", func(f string, a ...any) { lines = append(lines, f); lines = append(lines, anyStrings(a)...) })
	if joined := strings.Join(lines, " "); !strings.Contains(joined, "obelisk -signin -print-link -listen 127.0.0.1:7821") {
		t.Fatalf("no recovery command printed: %s", joined)
	}
}

// A page on another loopback port is same-site, so the browser would send the cookie
// to a framed Obelisk. No response may be framed.
func TestServer_NoResponseCanBeFramed(t *testing.T) {
	app := sessionTestApp(t)
	auth, _ := newSessionAuth("", 7821)
	ts := httptest.NewServer(serverHandler(app, auth))
	defer ts.Close()
	code, _ := auth.mintCode()
	for _, path := range []string{"/", signInPath + "?code=" + url.QueryEscape(code), signInPath + "?code=used", "/api/config", "/no-such-page"} {
		res, err := signInClient().Do(uiCall(http.MethodGet, ts.URL+path, nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if got := res.Header.Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
			t.Errorf("%s: Content-Security-Policy %q", strings.SplitN(path, "?", 2)[0], got)
		}
		if got := res.Header.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options %q", strings.SplitN(path, "?", 2)[0], got)
		}
	}
}

func anyStrings(a []any) []string {
	var out []string
	for _, v := range a {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestUI_TokenSignInDoesNotRepromptAndSendsThePageKey(t *testing.T) {
	b, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	ui := string(b)
	if strings.Contains(ui, "window.prompt('This Obelisk server requires") {
		t.Error("the token request is still a blocking prompt that repeats on each refused call")
	}
	if !strings.Contains(ui, "'X-Obelisk-Page-Key':pageKey()") {
		t.Error("API calls do not send the page key")
	}
	if strings.Contains(ui, "window.open(mediaURL(") {
		t.Error("a new-tab download skips the ticket")
	}
}
