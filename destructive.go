//go:build !guionly

package main

// destructive.go — the delete gate.
//
// Every action that deletes, moves away or overwrites something the user has
// (files or catalog records) goes through two steps over HTTP:
//
//  1. POST /api/destructive/prepare {op, params} works out the EXACT targets the
//     action would touch — each file by identity (volume and file ID), size and
//     modification time, each record by ID and a digest of its contents — and
//     returns them with a one-use token. The token expires after five minutes,
//     belongs to the browser session that asked (the X-Obelisk-Session header),
//     and is bound to a digest of the operation, its parameters and its targets.
//  2. POST /api/destructive/execute {token, confirm} runs it, only when confirm
//     is exactly the operation's word ("Delete" or "Overwrite"), the session is
//     the same, and a fresh count of the targets gives the same digest. Anything
//     else refuses, and the token is used up either way.
//
// There is no bypass: the old direct routes for these actions answer 428 and say
// to prepare first, and no parameter skips any step.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// sessionHeader names the browser session a destructive token belongs to.
	// The UI makes a random value per page load and sends it on every request.
	sessionHeader = "X-Obelisk-Session"
	// destructiveTTL is how long a prepared action can be executed.
	destructiveTTL = 5 * time.Minute
	// Typed words, case-sensitive.
	wordDelete    = "Delete"
	wordOverwrite = "Overwrite"
)

// destructiveNow is the clock for token expiry; a test seam.
var destructiveNow = time.Now

// destructiveTarget is one thing an action would touch, recorded exactly.
type destructiveTarget struct {
	Kind   string `json:"kind"` // file | folder | missing | record
	Path   string `json:"path,omitempty"`
	Record string `json:"record,omitempty"` // e.g. "event 3"
	Volume uint64 `json:"volume,omitempty"`
	FileID uint64 `json:"file_id,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Mtime  int64  `json:"mtime_ns,omitempty"`
	Digest string `json:"digest,omitempty"` // records: SHA-256 of the record's contents
	Note   string `json:"note,omitempty"`   // what happens to it
}

// destructiveOp is one gated action.
type destructiveOp struct {
	word  string
	title func(a *App, p map[string]any) string
	// targets works out exactly what the action would touch, or refuses.
	targets func(a *App, p map[string]any) ([]destructiveTarget, error)
	// run performs the action and returns the response body.
	run func(a *App, p map[string]any) (any, error)
}

// destructiveOps is the registry of gated actions, filled in per action.
var destructiveOps = map[string]*destructiveOp{}

// destructiveToken is a prepared action waiting for its typed word.
type destructiveToken struct {
	app     *App
	op      string
	params  map[string]any
	digest  string
	session string
	expires time.Time
}

// destructiveTokens holds prepared actions by the SHA-256 of their token (the
// token itself is never kept).
type destructiveTokens struct {
	mu sync.Mutex
	m  map[string]*destructiveToken
}

var destructiveGate = &destructiveTokens{m: map[string]*destructiveToken{}}

// PreparedAction is what prepare returns.
type PreparedAction struct {
	Token     string              `json:"token"`
	Op        string              `json:"op"`
	Word      string              `json:"word"`
	Title     string              `json:"title"`
	Targets   []destructiveTarget `json:"targets"`
	Digest    string              `json:"digest"`
	ExpiresAt time.Time           `json:"expires_at"`
}

// destructiveError is a refusal with the HTTP status it answers with.
type destructiveError struct {
	status int
	msg    string
}

func (e *destructiveError) Error() string { return e.msg }

func refuseDestructive(status int, format string, args ...any) error {
	return &destructiveError{status: status, msg: fmt.Sprintf(format, args...)}
}

// targetDigest binds an operation, its parameters and its exact targets.
func targetDigest(op string, params map[string]any, targets []destructiveTarget) (string, error) {
	b, err := json.Marshal(struct {
		Op      string              `json:"op"`
		Params  map[string]any      `json:"params"`
		Targets []destructiveTarget `json:"targets"`
	}{op, params, targets})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func validSession(s string) bool {
	return len(s) >= 16 && len(s) <= 128 && !strings.ContainsAny(s, " \t\r\n")
}

// PrepareDestructive works out an action's exact targets and issues its token.
func (a *App) PrepareDestructive(session, op string, params map[string]any) (*PreparedAction, error) {
	if !validSession(session) {
		return nil, refuseDestructive(400, "this action needs the browser session that asks for it (%s header); reload the page and try again", sessionHeader)
	}
	d := destructiveOps[op]
	if d == nil {
		return nil, refuseDestructive(400, "unknown action %q", op)
	}
	if params == nil {
		params = map[string]any{}
	}
	targets, err := d.targets(a, params)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, refuseDestructive(400, "nothing to do: the action has no targets")
	}
	digest, err := targetDigest(op, params, targets)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)
	key := sha256.Sum256([]byte(token))
	now := destructiveNow()
	destructiveGate.mu.Lock()
	for k, t := range destructiveGate.m { // drop expired ones
		if now.After(t.expires) {
			delete(destructiveGate.m, k)
		}
	}
	destructiveGate.m[hex.EncodeToString(key[:])] = &destructiveToken{app: a, op: op, params: params, digest: digest, session: session, expires: now.Add(destructiveTTL)}
	destructiveGate.mu.Unlock()
	return &PreparedAction{Token: token, Op: op, Word: d.word, Title: d.title(a, params), Targets: targets, Digest: digest, ExpiresAt: now.Add(destructiveTTL)}, nil
}

// ExecuteDestructive runs a prepared action. The token is used up by this call
// whatever the outcome.
func (a *App) ExecuteDestructive(session, token, confirm string) (any, error) {
	key := sha256.Sum256([]byte(token))
	destructiveGate.mu.Lock()
	t := destructiveGate.m[hex.EncodeToString(key[:])]
	delete(destructiveGate.m, hex.EncodeToString(key[:]))
	destructiveGate.mu.Unlock()
	if t == nil {
		return nil, refuseDestructive(410, "this confirmation is unknown or was already used; prepare the action again")
	}
	if destructiveNow().After(t.expires) {
		return nil, refuseDestructive(410, "this confirmation expired (it lasts %v); prepare the action again", destructiveTTL)
	}
	if t.app != a {
		return nil, refuseDestructive(410, "this confirmation is unknown or was already used; prepare the action again")
	}
	if session != t.session {
		return nil, refuseDestructive(403, "this confirmation belongs to another browser session; prepare the action again here")
	}
	d := destructiveOps[t.op]
	if confirm != d.word {
		return nil, refuseDestructive(400, "type %q exactly to confirm; nothing was changed. Prepare the action again", d.word)
	}
	targets, err := d.targets(a, t.params)
	if err != nil {
		return nil, err
	}
	digest, err := targetDigest(t.op, t.params, targets)
	if err != nil {
		return nil, err
	}
	if digest != t.digest {
		return nil, refuseDestructive(409, "what this action would touch changed after it was prepared (%d target(s) now); nothing was changed. Prepare it again and check the list", len(targets))
	}
	return d.run(a, t.params)
}

// needsConfirmation answers an old direct route for a gated action.
func needsConfirmation(w http.ResponseWriter, op string) {
	jsonErr(w, 428, fmt.Errorf("this action needs confirmation: prepare it with POST /api/destructive/prepare (op %q), then execute it with the typed word", op))
}

// destructiveHTTPError answers a refusal with its status (400 otherwise).
func destructiveHTTPError(w http.ResponseWriter, err error) {
	var de *destructiveError
	if errors.As(err, &de) {
		jsonErr(w, de.status, err)
		return
	}
	jsonErr(w, 400, err)
}

// destructiveRoutes registers prepare and execute.
func destructiveRoutes(mux *http.ServeMux, app *App) {
	mux.HandleFunc("POST /api/destructive/prepare", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Op     string         `json:"op"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonErr(w, 400, fmt.Errorf("invalid request"))
			return
		}
		res, err := app.PrepareDestructive(r.Header.Get(sessionHeader), in.Op, in.Params)
		if err != nil {
			destructiveHTTPError(w, err)
			return
		}
		jsonOut(w, res)
	})
	mux.HandleFunc("POST /api/destructive/execute", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Token   string `json:"token"`
			Confirm string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			jsonErr(w, 400, fmt.Errorf("invalid request"))
			return
		}
		res, err := app.ExecuteDestructive(r.Header.Get(sessionHeader), in.Token, in.Confirm)
		if err != nil {
			destructiveHTTPError(w, err)
			return
		}
		jsonOut(w, res)
	})
}

// ---- target helpers -----------------------------------------------------

// fileTarget records the object at p exactly, without following a final link:
// its identity from a handle, size and modification time. A path with nothing
// at it is recorded as missing, so its appearing later changes the digest too.
func fileTarget(p, note string) destructiveTarget {
	t := destructiveTarget{Kind: "missing", Path: p, Note: note}
	fi, err := os.Lstat(p)
	if err != nil {
		return t
	}
	t.Kind, t.Size, t.Mtime = "file", fi.Size(), fi.ModTime().UnixNano()
	switch {
	case fi.IsDir():
		t.Kind, t.Size = "folder", 0
	case fi.Mode()&fs.ModeSymlink != 0:
		t.Kind = "link"
		return t
	}
	if f, err := os.Open(p); err == nil {
		if id, err := handleIdentity(f); err == nil {
			t.Volume, t.FileID = id.vol, id.idx
		}
		f.Close()
	}
	return t
}

// treeTargets records root and everything below it (without following links).
func treeTargets(root, note string) ([]destructiveTarget, error) {
	var out []destructiveTarget
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out = append(out, fileTarget(p, note))
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// recordTarget records a catalog record by name and a digest of its contents.
func recordTarget(name string, v any, note string) destructiveTarget {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return destructiveTarget{Kind: "record", Record: name, Digest: hex.EncodeToString(h[:]), Note: note}
}

// paramString and paramInt read action parameters.
func paramString(p map[string]any, k string) string {
	v, _ := p[k].(string)
	return strings.TrimSpace(v)
}

func paramInt(p map[string]any, k string) int {
	switch v := p[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
