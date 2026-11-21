//go:build !guionly

package main

// session_auth.go: who may use a running Obelisk.
//
// Every route that returns data or changes state needs a signed-in session, on
// loopback too. Each start makes a fresh random secret that lives only in memory.
// Session credentials are signed with it, so one from another run or another
// instance is worthless. The secret is never written anywhere.
//
// A session has two halves, because browsers send a host's cookies to ALL of its
// ports and a program on another port could collect the cookie:
//   - an HttpOnly, SameSite=Strict cookie;
//   - a page key, kept in localStorage, which browsers do scope to host AND port.
//
// An API call must present both. A new-tab download cannot set a header, so the page
// first asks for a single-use ticket bound to that exact URL and to the cookie.
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
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionLifetime     = 24 * time.Hour
	signInLinkLifetime  = 60 * time.Second
	downloadTicketLife  = 30 * time.Second
	maxPendingSignIns   = 32
	maxPendingTickets   = 64
	signInPath          = "/auth/signin"
	sessionReenterPath  = "/api/session/reenter"
	sessionExchangePath = "/api/session/exchange"
	sessionTicketPath   = "/api/session/ticket"
	sessionKeyHeader    = "X-Obelisk-Page-Key"
	cookieBodyLen       = 24
)

var errTooManySignIns = errors.New("too many sign-in links are waiting; try again in a minute")

type downloadTicket struct {
	expires time.Time
	target  string   // canonical path and query, without the ticket
	cookie  [32]byte // hash of the session cookie's body
}

// sessionAuth holds one run's sign-in state. Nothing in it is persisted.
type sessionAuth struct {
	token  string // the configured token; empty means owner sign-in
	secret [32]byte
	port   int
	addr   string // the exact address this server listens on
	now    func() time.Time

	mu       sync.Mutex
	codes    map[[32]byte]time.Time // hash of each unused link code, with its expiry
	tickets  map[[32]byte]downloadTicket
	reentrys map[string]struct{} // re-entry nonces already used in this run
}

func newSessionAuth(token string, port int) (*sessionAuth, error) {
	s := &sessionAuth{token: token, port: port, addr: "127.0.0.1:" + strconv.Itoa(port), now: time.Now,
		codes: map[[32]byte]time.Time{}, tickets: map[[32]byte]downloadTicket{}, reentrys: map[string]struct{}{}}
	if _, err := rand.Read(s.secret[:]); err != nil {
		return nil, fmt.Errorf("cannot make a sign-in secret: %w", err)
	}
	return s, nil
}

func (s *sessionAuth) ownerMode() bool { return s.token == "" }

// cookieName differs on every run, so instances that share a host name (two
// containers on one NAS, say) cannot overwrite each other's cookie.
func (s *sessionAuth) cookieName() string {
	return "obelisk_session_" + hex.EncodeToString(s.mac("cookie-name"))[:16]
}

func (s *sessionAuth) mac(parts ...string) []byte {
	m := hmac.New(sha256.New, s.secret[:])
	_, _ = io.WriteString(m, strings.Join(parts, "|"))
	return m.Sum(nil)
}

// reentryKey is what the owner-only sign-in file holds. It is derived, so the file
// never contains the secret, and it stops working when this process ends.
func (s *sessionAuth) reentryKey() string {
	return hex.EncodeToString(s.mac("reentry-key", s.addr))
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
func (s *sessionAuth) newCookieValue() (value string, body []byte, err error) {
	raw := make([]byte, cookieBodyLen)
	binary.BigEndian.PutUint64(raw[:8], uint64(s.now().Add(sessionLifetime).Unix()))
	if _, err := rand.Read(raw[8:]); err != nil {
		return "", nil, err
	}
	sig := s.mac("session", string(raw))
	return base64.RawURLEncoding.EncodeToString(append(append([]byte{}, raw...), sig...)), raw, nil
}

// validCookieBodies returns the body of every valid session cookie on the request.
// All are checked, so a junk cookie planted under the same name cannot hide a real one.
func (s *sessionAuth) validCookieBodies(r *http.Request) [][]byte {
	var bodies [][]byte
	for _, c := range r.CookiesNamed(s.cookieName()) {
		raw, err := base64.RawURLEncoding.DecodeString(c.Value)
		if err != nil || len(raw) != cookieBodyLen+sha256.Size {
			continue
		}
		if !hmac.Equal(raw[cookieBodyLen:], s.mac("session", string(raw[:cookieBodyLen]))) {
			continue
		}
		if s.now().Unix() < int64(binary.BigEndian.Uint64(raw[:8])) {
			bodies = append(bodies, raw[:cookieBodyLen])
		}
	}
	return bodies
}

// pageKey is the half of a session that only this exact origin can read.
func (s *sessionAuth) pageKey(body []byte) string {
	return hex.EncodeToString(s.mac("page-key", string(body)))
}

// sessionBody returns the cookie body whose page key the request presents.
func (s *sessionAuth) sessionBody(r *http.Request) ([]byte, bool) {
	key := r.Header.Get(sessionKeyHeader)
	if key == "" {
		return nil, false
	}
	for _, body := range s.validCookieBodies(r) {
		if hmac.Equal([]byte(key), []byte(s.pageKey(body))) {
			return body, true
		}
	}
	return nil, false
}

func (s *sessionAuth) validBearer(r *http.Request) bool {
	if s.token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.token)) == 1
}

// canonicalTarget is a URL's path and sorted query, without any ticket.
func canonicalTarget(u *url.URL) string {
	q := u.Query()
	q.Del("ticket")
	if len(q) == 0 {
		return u.Path
	}
	return u.Path + "?" + q.Encode()
}

// redeemTicket admits one GET of the exact URL the ticket was issued for, from the
// browser holding the session it was issued to.
func (s *sessionAuth) redeemTicket(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	raw := r.URL.Query().Get("ticket")
	if raw == "" {
		return false
	}
	h := sha256.Sum256([]byte(raw))
	s.mu.Lock()
	t, ok := s.tickets[h]
	delete(s.tickets, h)
	s.mu.Unlock()
	if !ok || !s.now().Before(t.expires) || t.target != canonicalTarget(r.URL) {
		return false
	}
	for _, body := range s.validCookieBodies(r) {
		if sha256.Sum256(body) == t.cookie {
			return true
		}
	}
	return false
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
		case sessionTicketPath:
			s.handleTicket(w, r)
			return
		}
		if isSafeMethod(r.Method) && public(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.sessionBody(r); ok || s.validBearer(r) || s.redeemTicket(r) {
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

// handleSignIn swaps a one-time link for a session. It sets the cookie, stores the
// page key in this origin's storage, and replaces the URL with a clean one, so the
// link leaves the tab's history. It does not check Sec-Fetch-Site: the owner's link
// arrives from a local file or the OS, and the code itself is the proof.
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if !s.redeemCode(r.URL.Query().Get("code")) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Sign-in link not valid</title>`+
			`<p>This sign-in link was already used or has expired.</p>`+
			`<p>Run <code>obelisk -signin</code> from the account that started Obelisk to get a new one.</p>`)
		return
	}
	value, body, err := s.newCookieValue()
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
	_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>Signing in</title>`+
		`<script>try{localStorage.setItem('obelisk_page_key','`+s.pageKey(body)+`')}catch(e){}location.replace('/')</script>`+
		`<noscript>Obelisk needs JavaScript.</noscript>`)
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

// handleTicket issues a single-use ticket for one new-tab download.
func (s *sessionAuth) handleTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, ok := s.sessionBody(r)
	if !ok {
		s.refuse(w)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, errors.New("a download address is required"))
		return
	}
	u, err := url.ParseRequestURI(req.URL)
	if err != nil || u.Host != "" || u.Scheme != "" || !strings.HasPrefix(u.Path, "/api/") {
		jsonErr(w, http.StatusBadRequest, errors.New("that is not a download address"))
		return
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		jsonErr(w, http.StatusInternalServerError, errors.New("cannot issue a download ticket"))
		return
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw[:])
	now := s.now()
	s.mu.Lock()
	for h, t := range s.tickets {
		if !now.Before(t.expires) {
			delete(s.tickets, h)
		}
	}
	if len(s.tickets) >= maxPendingTickets {
		s.mu.Unlock()
		jsonErr(w, http.StatusTooManyRequests, errors.New("too many downloads are waiting; try again shortly"))
		return
	}
	s.tickets[sha256.Sum256([]byte(ticket))] = downloadTicket{
		expires: now.Add(downloadTicketLife), target: canonicalTarget(u), cookie: sha256.Sum256(body)}
	s.mu.Unlock()
	q := u.Query()
	q.Set("ticket", ticket)
	w.Header().Set("Cache-Control", "no-store")
	jsonOut(w, map[string]string{"url": u.Path + "?" + q.Encode()})
}

func reentryClientProof(key, addr, nonce string) string {
	return hex.EncodeToString(reentryMAC(key, "client", addr, nonce, ""))
}

func reentryServerProof(key, addr, nonce, sealed string) string {
	return hex.EncodeToString(reentryMAC(key, "server", addr, nonce, sealed))
}

func reentryMAC(key, side, addr, nonce, extra string) []byte {
	m := hmac.New(sha256.New, []byte(key))
	_, _ = io.WriteString(m, strings.Join([]string{"obelisk-reentry", side, addr, nonce, extra}, "|"))
	return m.Sum(nil)
}

// sealReentryCode masks a sign-in code with a pad only the key holder can compute, so
// a program relaying the exchange never sees a usable code. It also unseals.
func sealReentryCode(key, addr, nonce string, code []byte) []byte {
	pad := reentryMAC(key, "seal", addr, nonce, "")
	out := make([]byte, len(code))
	for i := range code {
		out[i] = code[i] ^ pad[i%len(pad)]
	}
	return out
}

// handleReenter gives obelisk -signin a new link. Each side proves it holds the key
// from the owner-only file without sending it. The proofs name this exact listener
// address, so an exchange aimed at another address cannot be relayed here.
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
	key := s.reentryKey()
	if !hmac.Equal([]byte(req.Proof), []byte(reentryClientProof(key, s.addr, req.Nonce))) {
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
	sealed := hex.EncodeToString(sealReentryCode(key, s.addr, req.Nonce, []byte(code)))
	w.Header().Set("Cache-Control", "no-store")
	jsonOut(w, map[string]string{"sealed": sealed, "proof": reentryServerProof(key, s.addr, req.Nonce, sealed)})
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
	return denyFraming(apiGuard(auth.middleware(app.swapGate(mux), func(p string) bool { return public[p] })))
}

// denyFraming stops any page from framing Obelisk. A page on another loopback port
// counts as same-site, so a framed Obelisk would be signed in and could be clicked.
func denyFraming(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
