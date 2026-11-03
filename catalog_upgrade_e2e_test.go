//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a process's output, safe to read while it runs.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// e2eServer is the real Obelisk binary serving a data folder.
type e2eServer struct {
	cmd  *exec.Cmd
	base string
	out  *syncBuffer
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startServer runs bin on dataDir on the default catalog engine.
func startServer(t *testing.T, bin, dataDir string) *e2eServer {
	t.Helper()
	port := freePort(t)
	cmd := exec.Command(bin, "-data", dataDir, "-listen", fmt.Sprintf("127.0.0.1:%d", port))
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, catalogEngineEnv+"=") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	out := &syncBuffer{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &e2eServer{cmd: cmd, base: fmt.Sprintf("http://127.0.0.1:%d", port), out: out}
	t.Cleanup(func() { s.stop(t) })
	return s
}

// get answers GET path as the UI asks it, or an error while the server is not up.
func (s *e2eServer) get(path string, into any) (int, error) {
	req, _ := http.NewRequest("GET", s.base+path, nil)
	req.Header.Set(requestedByHeader, requestedByValue)
	req.Header.Set("Authorization", "Bearer secret-token-should-not-travel") // abTestApp's token
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if into != nil {
		if err := json.Unmarshal(b, into); err != nil {
			return res.StatusCode, fmt.Errorf("%s: %w", b, err)
		}
	}
	return res.StatusCode, nil
}

// waitOpen waits until the server answers health with no upgrade running, and
// reports whether it ever saw one.
func (s *e2eServer) waitOpen(t *testing.T) (sawUpgrade bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		var h map[string]any
		code, err := s.get("/api/health", &h)
		if err == nil && code != 200 {
			t.Fatalf("GET /api/health answered %d: %v\n%s", code, h, s.out)
		}
		if err == nil {
			if h["upgrade"] == nil {
				return sawUpgrade
			}
			sawUpgrade = true
			if h["upgrading"] != true {
				t.Fatalf("the upgrade did not finish: %v\n%s", h, s.out)
			}
		}
		if s.cmd.ProcessState != nil {
			t.Fatalf("the server exited:\n%s", s.out)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the server did not open in time:\n%s", s.out)
	return false
}

func (s *e2eServer) stop(t *testing.T) {
	if s.cmd.ProcessState != nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = s.cmd.Process.Kill()
	} else {
		_ = s.cmd.Process.Signal(os.Interrupt)
	}
	_ = s.cmd.Wait()
}

// The real binary, started on a real v8 data folder with no engine named, moves
// the catalog into the catalog database on its first start, keeps every record
// and job unchanged and the old catalog.json bytes beside it, records the upgrade
// once, and opens straight on the database from then on.
func TestCatalogUpgrade_EndToEndOnARealV8DataFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	f := makeV8DataFolder(t)
	bin := buildBinary(t, t.TempDir(), "obelisk-upgrade-e2e", "")

	first := startServer(t, bin, f.dir)
	first.waitOpen(t)
	var jobs []Job
	if code, err := first.get("/api/jobs", &jobs); err != nil || code != 200 {
		t.Fatalf("GET /api/jobs: %d %v", code, err)
	}
	upgrades := 0
	for _, j := range jobs {
		if j.Kind == catalogUpgradeJobKind {
			upgrades++
			if j.Status != "COMPLETED" {
				t.Fatalf("the upgrade job: %+v", j)
			}
		}
	}
	if upgrades != 1 {
		t.Fatalf("%d upgrade jobs on the board after the first start:\n%s", upgrades, first.out)
	}
	assertJobsKept(t, f.jobsRaw, jobs)
	first.stop(t)
	assertEveryRecordKept(t, f.dir, f.catalogRaw)

	// The second start opens the database directly: no upgrade, nothing moved again.
	second := startServer(t, bin, f.dir)
	if second.waitOpen(t) {
		t.Fatal("the second start upgraded again")
	}
	jobs = nil
	if _, err := second.get("/api/jobs", &jobs); err != nil {
		t.Fatal(err)
	}
	upgrades = 0
	for _, j := range jobs {
		if j.Kind == catalogUpgradeJobKind {
			upgrades++
		}
	}
	if upgrades != 1 {
		t.Fatalf("%d upgrade jobs on the board after the second start", upgrades)
	}
	assertJobsKept(t, f.jobsRaw, jobs)
	second.stop(t)
	assertEveryRecordKept(t, f.dir, f.catalogRaw)
	if kept, _ := filepath.Glob(filepath.Join(f.dir, catalogPreSQLite+"*")); len(kept) != 1 {
		t.Fatalf("kept copies of catalog.json after two starts: %v", kept)
	}
}
