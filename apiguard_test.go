package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apiGuard is the cross-origin / CSRF gate: every /api/ request must carry the
// X-Requested-By handshake header (which a cross-origin page can't set without a
// preflight we never grant), plus an Origin/Referer host check on state-changing
// methods. These tests drive the middleware directly with a trivial 200 handler.

func guardTS(t *testing.T) *httptest.Server {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	ts := httptest.NewServer(apiGuard(next))
	t.Cleanup(ts.Close)
	return ts
}

// do issues a request and returns the response; mutate lets a test set headers.
func do(t *testing.T, ts *httptest.Server, method, path string, mutate func(*http.Request)) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func assertNoCORS(t *testing.T, resp *http.Response) {
	t.Helper()
	for h := range resp.Header {
		if strings.HasPrefix(strings.ToLower(h), "access-control-") {
			t.Errorf("response must never carry a CORS header, found %q", h)
		}
	}
}

// A plain one-line 403 body (the spec: "403 with a plain one-line body").
func assertPlainForbidden(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Count(strings.TrimRight(string(b), "\n"), "\n") != 0 {
		t.Errorf("body is not one line: %q", string(b))
	}
	assertNoCORS(t, resp)
}

// (1) Same-origin fetch WITH the handshake header passes.
func TestAPIGuard_SameOriginWithHeaderPasses(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "GET", "/api/home", func(r *http.Request) {
		r.Header.Set(requestedByHeader, requestedByValue)
		r.Header.Set("Origin", ts.URL) // same host as the bound listener
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	assertNoCORS(t, resp)
	resp.Body.Close()
}

// A state-changing request with the header and a matching Origin passes.
func TestAPIGuard_SameOriginPostPasses(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "POST", "/api/plan", func(r *http.Request) {
		r.Header.Set(requestedByHeader, requestedByValue)
		r.Header.Set("Origin", ts.URL)
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// (2a) A request WITHOUT the header is refused 403 (plain one-line body).
func TestAPIGuard_MissingHeaderForbidden(t *testing.T) {
	ts := guardTS(t)
	for _, m := range []string{"GET", "POST", "DELETE"} {
		resp := do(t, ts, m, "/api/home", nil) // no header, no navigation signal
		assertPlainForbidden(t, resp)
	}
}

// (2b) A request with a FOREIGN Origin is refused, even carrying the header.
func TestAPIGuard_ForeignOriginForbidden(t *testing.T) {
	ts := guardTS(t)
	// State-changing + foreign Origin → refused by the Origin/Referer host check.
	resp := do(t, ts, "POST", "/api/plan", func(r *http.Request) {
		r.Header.Set(requestedByHeader, requestedByValue)
		r.Header.Set("Origin", "http://evil.example")
	})
	assertPlainForbidden(t, resp)

	// Cross-origin GET fetch without our header → refused by the header rule.
	resp = do(t, ts, "GET", "/api/home", func(r *http.Request) {
		r.Header.Set("Origin", "http://evil.example")
		r.Header.Set("Sec-Fetch-Mode", "cors")
	})
	assertPlainForbidden(t, resp)
}

// A foreign Referer (Origin absent) on a state-changing method is refused.
func TestAPIGuard_ForeignRefererForbidden(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "POST", "/api/plan", func(r *http.Request) {
		r.Header.Set(requestedByHeader, requestedByValue)
		r.Header.Set("Referer", "http://evil.example/page")
	})
	assertPlainForbidden(t, resp)
}

// (3) A genuine same-origin top-level navigation (new-tab download) that cannot
// carry a custom header is allowed — but only as a real navigation, not a fetch.
func TestAPIGuard_SameOriginNavigationDownloadAllowed(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "GET", "/api/collections/1/structure-export?format=csv", func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Mode", "navigate") // browser marks a top-level nav
		// no Origin on a same-origin GET navigation
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin navigation download status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

// A cross-origin navigation (foreign Referer) is NOT exempt.
func TestAPIGuard_CrossOriginNavigationForbidden(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "GET", "/api/home", func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Mode", "navigate")
		r.Header.Set("Referer", "http://evil.example/page")
	})
	assertPlainForbidden(t, resp)
}

// Non-/api paths (the static UI) are never gated.
func TestAPIGuard_NonAPIPassesThrough(t *testing.T) {
	ts := guardTS(t)
	resp := do(t, ts, "GET", "/index.html", nil) // no header, not /api
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("non-/api status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}
