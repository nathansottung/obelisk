//go:build !guionly

package main

// owner_e2e_test.go — one Obelisk process owns a data folder. The real binary is
// started twice on one fresh data folder; the second start must refuse, so both
// archives created through the instances that did run survive a third start.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ownerServer is the real binary on a data folder and catalog engine.
type ownerServer struct {
	cmd  *exec.Cmd
	base string
	out  *syncBuffer
	done chan struct{}
}

func startOwnerServer(t *testing.T, bin, dataDir, engine string) *ownerServer {
	t.Helper()
	port := freePort(t)
	cmd := exec.Command(bin, "-data", dataDir, "-init-config", "-listen", fmt.Sprintf("127.0.0.1:%d", port))
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, catalogEngineEnv+"=") && !strings.HasPrefix(kv, "OBELISK_AUTH_TOKEN=") && !strings.HasPrefix(kv, "MNEMO_AUTH_TOKEN=") {
			env = append(env, kv)
		}
	}
	if engine != "" {
		env = append(env, catalogEngineEnv+"="+engine)
	}
	cmd.Env = env
	out := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &ownerServer{cmd: cmd, base: fmt.Sprintf("http://127.0.0.1:%d", port), out: out, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(s.done) }()
	t.Cleanup(func() { s.kill() })
	return s
}

func (s *ownerServer) exited() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *ownerServer) call(method, path string, body any, into any) (int, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.base+path, rd)
	req.Header.Set(requestedByHeader, requestedByValue)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if into != nil && len(b) > 0 {
		if err := json.Unmarshal(b, into); err != nil {
			return res.StatusCode, fmt.Errorf("%s: %w", b, err)
		}
	}
	return res.StatusCode, nil
}

// waitUp waits until the server answers, or reports that it exited first.
func (s *ownerServer) waitUp(t *testing.T) (up bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if s.exited() {
			return false
		}
		var h map[string]any
		if code, err := s.call("GET", "/api/health", nil, &h); err == nil && code == 200 && h["upgrade"] == nil {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the server neither answered nor exited in time:\n%s", s.out)
	return false
}

// stop asks the server to stop and waits; on Windows, where a console interrupt
// cannot be sent to a child, it is killed — every acknowledged write is already
// committed by then.
func (s *ownerServer) stop(t *testing.T) {
	t.Helper()
	if s.exited() {
		return
	}
	_ = s.cmd.Process.Signal(os.Interrupt)
	select {
	case <-s.done:
	case <-time.After(3 * time.Second):
		s.kill()
	}
}

func (s *ownerServer) kill() {
	if s.exited() {
		return
	}
	_ = s.cmd.Process.Kill()
	<-s.done
}

func (s *ownerServer) createArchive(t *testing.T, name string) {
	t.Helper()
	var c map[string]any
	if code, err := s.call("POST", "/api/collections", map[string]any{"name": name}, &c); err != nil || code != 200 {
		t.Fatalf("create archive %q: %d %v %v\n%s", name, code, err, c, s.out)
	}
}

func (s *ownerServer) archiveNames(t *testing.T) []string {
	t.Helper()
	var list []map[string]any
	if code, err := s.call("GET", "/api/collections", nil, &list); err != nil || code != 200 {
		t.Fatalf("list archives: %d %v\n%s", code, err, s.out)
	}
	var names []string
	for _, c := range list {
		if n, ok := c["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

func ownerEngines() []string { return []string{"", "json"} }

// Two instances on one data folder: whatever happens, no archive created through an
// instance that answered may be lost. The second instance must refuse to start.
func TestOwner_TwoInstancesOneDataFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	bin := buildBinary(t, t.TempDir(), "obelisk-owner-e2e", "")
	for _, engine := range ownerEngines() {
		name := engine
		if name == "" {
			name = "database"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			first := startOwnerServer(t, bin, dir, engine)
			if !first.waitUp(t) {
				t.Fatalf("the first instance did not start:\n%s", first.out)
			}
			second := startOwnerServer(t, bin, dir, engine)
			secondUp := second.waitUp(t)

			first.createArchive(t, "From the first")
			if secondUp {
				second.createArchive(t, "From the second")
			}
			first.stop(t)
			second.stop(t)

			third := startOwnerServer(t, bin, dir, engine)
			if !third.waitUp(t) {
				t.Fatalf("the third instance did not start:\n%s", third.out)
			}
			names := strings.Join(third.archiveNames(t), " | ")
			if !strings.Contains(names, "From the first") || (secondUp && !strings.Contains(names, "From the second")) {
				t.Fatalf("an acknowledged archive was lost (second instance up: %v): %s\nfirst:\n%s\nsecond:\n%s",
					secondUp, names, first.out, second.out)
			}
			if secondUp {
				t.Fatalf("the second instance started on a data folder another instance owns:\n%s", second.out)
			}
			third.stop(t)
		})
	}
}

// The second instance exits at once and says which folder, that another Obelisk
// owns it (by process ID), and what to do.
func TestOwner_SecondInstanceSaysWhy(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	bin := buildBinary(t, t.TempDir(), "obelisk-owner-msg", "")
	dir := t.TempDir()
	first := startOwnerServer(t, bin, dir, "")
	if !first.waitUp(t) {
		t.Fatalf("the first instance did not start:\n%s", first.out)
	}
	second := startOwnerServer(t, bin, dir, "")
	select {
	case <-second.done:
	case <-time.After(30 * time.Second):
		t.Fatalf("the second instance did not exit; it must refuse at once, not wait:\n%s", second.out)
	}
	if second.cmd.ProcessState.Success() {
		t.Fatalf("the second instance exited with success:\n%s", second.out)
	}
	out := second.out.String()
	for _, want := range []string{dir, fmt.Sprintf("Another Obelisk (process %d)", first.cmd.Process.Pid), "Only one Obelisk", "-data"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "http://127.0.0.1") {
		t.Errorf("the refused instance started serving:\n%s", out)
	}
	first.stop(t)
}

// A killed owner leaves no lock behind: the next start succeeds and every
// acknowledged record is there.
func TestOwner_KilledOwnerRestarts(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	bin := buildBinary(t, t.TempDir(), "obelisk-owner-kill", "")
	for _, engine := range ownerEngines() {
		name := engine
		if name == "" {
			name = "database"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			first := startOwnerServer(t, bin, dir, engine)
			if !first.waitUp(t) {
				t.Fatalf("the first instance did not start:\n%s", first.out)
			}
			first.createArchive(t, "Acknowledged before the kill")
			first.kill()
			next := startOwnerServer(t, bin, dir, engine)
			if !next.waitUp(t) {
				t.Fatalf("the start after a kill was refused:\n%s", next.out)
			}
			if names := strings.Join(next.archiveNames(t), " | "); !strings.Contains(names, "Acknowledged before the kill") {
				t.Fatalf("an acknowledged archive was lost in the kill: %s", names)
			}
			next.stop(t)
		})
	}
}
