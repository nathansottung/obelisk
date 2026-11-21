//go:build !guionly

package main

// auth_test.go — the container-deployment guardrails: localhost detection and the
// bearer-token middleware that must gate /api (but not the static UI) the moment a
// token is set.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLocalhostAddr(t *testing.T) {
	local := []string{"127.0.0.1:7821", "localhost:7821", "[::1]:7821", "127.0.0.1:0"}
	remote := []string{"0.0.0.0:7821", ":7821", "192.168.1.10:7821", "[::]:7821", "10.0.0.5:7821"}
	for _, a := range local {
		if !isLocalhostAddr(a) {
			t.Errorf("%q should be localhost", a)
		}
	}
	for _, a := range remote {
		if isLocalhostAddr(a) {
			t.Errorf("%q should NOT be localhost", a)
		}
	}
}

func TestAuthMiddleware(t *testing.T) {
	inner := http.NewServeMux()
	inner.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	inner.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	public := func(p string) bool { return p == "/" }

	// No token: /api still needs a session; there is no pass-through.
	owner, err := newSessionAuth("", 7821)
	if err != nil {
		t.Fatal(err)
	}
	h := owner.middleware(inner, public)
	if code := hitAuth(h, "GET", "/api/health", ""); code != 401 {
		t.Errorf("no-token: /api/health should need a session, got %d", code)
	}

	// Token set → /api gated, static UI still public.
	withToken, err := newSessionAuth("s3cret", 7821)
	if err != nil {
		t.Fatal(err)
	}
	h = withToken.middleware(inner, public)
	cases := []struct {
		path, auth string
		query      string
		want       int
	}{
		{"/api/health", "", "", 401},
		{"/api/health", "Bearer s3cret", "", 200},
		{"/api/health", "Bearer wrong", "", 401},
		{"/api/health", "s3cret", "", 401},       // missing "Bearer " prefix
		{"/api/health", "", "token=s3cret", 401}, // a token in the URL is no longer accepted
		{"/api/health", "", "token=nope", 401},   // wrong query token
		{"/", "", "", 200},                       // static UI is public even with a token set
	}
	for _, c := range cases {
		u := c.path
		if c.query != "" {
			u += "?" + c.query
		}
		if code := hitAuth(h, "GET", u, c.auth); code != c.want {
			t.Errorf("path=%s auth=%q query=%q: got %d want %d", c.path, c.auth, c.query, code, c.want)
		}
	}
}

func hitAuth(h http.Handler, method, url, auth string) int {
	req := httptest.NewRequest(method, url, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}
