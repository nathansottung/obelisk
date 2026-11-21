//go:build !guionly

package main

// session_auth.go: who may use a running Obelisk.
//
// Every route that returns data or changes state needs a signed-in session, on
// loopback too. Each start makes a fresh random secret that lives only in memory.
// A session cookie is signed with it, so a cookie from another run or another
// instance is worthless. The secret is never written anywhere.
//
// A browser gets a session by redeeming a one-time link. The link comes from one of:
//   - the start itself, when no token is configured (the owner's browser opens);
//   - obelisk -signin, run by the same OS account (session_signin.go);
//   - a configured token, exchanged by the UI (containers and remote binds).
//
// The gate denies by default: only the embedded UI files are public, so a route
// added later needs a session without anyone remembering to list it.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionLifetime     = 24 * time.Hour
	signInLinkLifetime  = 60 * time.Second
	maxPendingSignIns   = 32
	signInPath          = "/auth/signin"
	sessionReenterPath  = "/api/session/reenter"
	sessionExchangePath = "/api/session/exchange"
)

var errTooManySignIns = errors.New("too many sign-in links are waiting; try again in a minute")

// sessionAuth holds one run's sign-in state. Nothing in it is persisted.
type sessionAuth struct {
	token  string // the configured token; empty means owner sign-in
	secret [32]byte
	port   int
	now    func() time.Time

	mu       sync.Mutex
	codes    map[[32]byte]time.Time // hash of each unused link code, with its expiry
	reentrys map[string]struct{}    // re-entry nonces already used in this run
}

func newSessionAuth(token string, port int) (*sessionAuth, error) {
	s := &sessionAuth{token: token, port: port, now: time.Now,
		codes: map[[32]byte]time.Time{}, reentrys: map[string]struct{}{}}
	if _, err := rand.Read(s.secret[:]); err != nil {
		return nil, fmt.Errorf("cannot make a sign-in secret: %w", err)
	}
	return s, nil
}

func (s *sessionAuth) ownerMode() bool { return s.token == "" }

// cookieName carries the port because browsers send a host's cookies to all of its
// ports. Two instances on one host therefore keep separate cookies.
func (s *sessionAuth) cookieName() string { return "obelisk_session_" + strconv.Itoa(s.port) }

func (s *sessionAuth) mac(parts ...string) []byte {
	m := hmac.New(sha256.New, s.secret[:])
	_, _ = io.WriteString(m, strings.Join(parts, "|"))
	return m.Sum(nil)
}

// reentryKey is what the owner-only sign-in file holds. It is derived, so the file
// never contains the secret, and it stops working when this process ends.
func (s *sessionAuth) reentryKey() string {
	return hex.EncodeToString(s.mac("reentry-key", strconv.Itoa(s.port)))
}

// mintCode makes a one-time sign-in code. Only its hash is kept.
func (s *sessionAuth) mintCode() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(raw[:])
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, exp := range s.codes {
		if !now.Before(exp) {
			delete(s.codes, h)
		}
	}
	if len(s.codes) >= maxPendingSignIns {
		return "", errTooManySignIns
	}
	s.codes[sha256.Sum256([]byte(code))] = now.Add(signInLinkLifetime)
	return code, nil
}

// redeemCode reports whether code is unused and unexpired, and uses it up.
func (s *sessionAuth) redeemCode(code string) bool {
	h := sha256.Sum256([]byte(code))
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.codes[h]
	if !ok {
		return false
	}
	delete(s.codes, h)
	return s.now().Before(exp)
}

// newCookieValue is expiry (8 bytes), random (16) and a signature over both.
func (s *sessionAuth) newCookieValue() (string, error) {
	var body [24]byte
	binary.BigEndian.PutUint64(body[:8], uint64(s.now().Add(sessionLifetime).Unix()))
	if _, err := rand.Read(body[8:]); err != nil {
		return "", err
	}
	sig := s.mac("session", strconv.Itoa(s.port), string(body[:]))
	return base64.RawURLEncoding.EncodeToString(append(body[:], sig...)), nil
}

func (s *sessionAuth) validCookie(r *http.Request) bool {
	c, err := r.Cookie(s.cookieName())
	if err != nil {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(raw) != 24+sha256.Size {
		return false
	}
	if !hmac.Equal(raw[24:], s.mac("session", strconv.Itoa(s.port), string(raw[:24]))) {
		return false
	}
	return s.now().Unix() < int64(binary.BigEndian.Uint64(raw[:8]))
}

func (s *sessionAuth) validBearer(r *http.Request) bool {
	if s.token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) == 1
}

// middleware is the session gate. public names the UI files anyone may load.
func (s *sessionAuth) middleware(next http.Handler, public func(path string) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case signInPath:
			s.handleSignIn(w, r)
			return
		case sessionReenterPath:
			s.handleReenter(w, r)
			return
		case sessionExchangePath:
			s.handleExchange(w, r)
			return
		}
		if isSafeMethod(r.Method) && public(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if s.validCookie(r) || s.validBearer(r) {
			next.ServeHTTP(w, r)
			return
		}
		s.refuse(w)
	})
}

// refuse answers 401 and tells the UI which way in applies.
func (s *sessionAuth) refuse(w http.ResponseWriter) {
	mode, msg := "owner", "sign-in required: run obelisk -signin from the account that started Obelisk"
	if !s.ownerMode() {
		mode, msg = "token", "authorization required: send Authorization: Bearer <token>"
		w.Header().Set("WWW-Authenticate", `Bearer realm="obelisk"`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "sign_in": mode})
}

// handleSignIn swaps a one-time link for a session cookie and redirects to a clean
// URL. It does not check Sec-Fetch-Site: the owner's link arrives from a local file
// or the OS, and the code itself is the proof.
func (s *sessionAuth) handleSignIn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !loopbackHostAllowed(r) {
		forbidCSRF(w, "host not allowed")
		return
	}
	if !s.redeemCode(r.URL.Query().Get("code")) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Sign-in link not valid</title>`+
			`<p>This sign-in link was already used or has expired.</p>`+
			`<p>Run <code>obelisk -signin</code> from the account that started Obelisk to get a new one.</p>`)
		return
	}
	value, err := s.newCookieValue()
	if err != nil {
		http.Error(w, "cannot start a session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName(),
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleExchange turns a configured token into a one-time link, so a browser never
// has to carry the token in a URL.
func (s *sessionAuth) handleExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.validBearer(r) {
		s.refuse(w)
		return
	}
	code, err := s.mintCode()
	if err != nil {
		jsonErr(w, http.StatusTooManyRequests, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonOut(w, map[string]string{"link": signInPath + "?code=" + code})
}

func reentryClientProof(key, port, nonce string) string { return reentryProof(key, "client", port, nonce, "") }

func reentryServerProof(key, port, nonce, code string) string {
	return reentryProof(key, "server", port, nonce, code)
}

func reentryProof(key, side, port, nonce, code string) string {
	m := hmac.New(sha256.New, []byte(key))
	_, _ = io.WriteString(m, strings.Join([]string{"obelisk-reentry", side, port, nonce, code}, "|"))
	return hex.EncodeToString(m.Sum(nil))
}

// handleReenter gives obelisk -signin a new link. Each side proves it holds the key
// from the owner-only file without sending it, so a program that took over the port
// after Obelisk stopped learns nothing and cannot pass as Obelisk.
func (s *sessionAuth) handleReenter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.ownerMode() {
		s.refuse(w)
		return
	}
	var req struct {
		Nonce string `json:"nonce"`
		Proof string `json:"proof"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil || len(req.Nonce) != 64 {
		s.refuse(w)
		return
	}
	key, port := s.reentryKey(), strconv.Itoa(s.port)
	if !hmac.Equal([]byte(req.Proof), []byte(reentryClientProof(key, port, req.Nonce))) {
		s.refuse(w)
		return
	}
	s.mu.Lock()
	_, used := s.reentrys[req.Nonce]
	if !used {
		s.reentrys[req.Nonce] = struct{}{}
	}
	s.mu.Unlock()
	if used {
		s.refuse(w)
		return
	}
	code, err := s.mintCode()
	if err != nil {
		jsonErr(w, http.StatusTooManyRequests, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonOut(w, map[string]string{"code": code, "proof": reentryServerProof(key, port, req.Nonce, code)})
}

// ---- the routes ----------------------------------------------------------

// routeRegistrar is what route setup needs from a mux. It lets the server record
// every pattern it registers, so the route test is built from the mux itself.
type routeRegistrar interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

type recordingMux struct {
	*http.ServeMux
	patterns []string
}

func (m *recordingMux) Handle(pattern string, handler http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.ServeMux.Handle(pattern, handler)
}

func (m *recordingMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.patterns = append(m.patterns, pattern)
	m.ServeMux.HandleFunc(pattern, handler)
}

// newAppMux builds the server's routes and returns every registered pattern.
func newAppMux(app *App) (*http.ServeMux, []string) {
	rec := &recordingMux{ServeMux: http.NewServeMux()}
	api(rec, app)
	sub, _ := fs.Sub(uiFS, "ui")
	rec.Handle("/", http.FileServer(http.FS(sub)))
	return rec.ServeMux, rec.patterns
}

// uiPublicPaths lists the embedded UI files, the only things served with no session.
func uiPublicPaths() map[string]bool {
	public := map[string]bool{"/": true}
	sub, _ := fs.Sub(uiFS, "ui")
	_ = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			public["/"+p] = true
		}
		return nil
	})
	return public
}

// serverHandler is the whole request chain the server runs.
func serverHandler(app *App, auth *sessionAuth) http.Handler {
	mux, _ := newAppMux(app)
	public := uiPublicPaths()
	return apiGuard(auth.middleware(app.swapGate(mux), func(p string) bool { return public[p] }))
}
