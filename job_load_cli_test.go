package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestJobLoadCLI_StartupGate(t *testing.T) {
	oldAccel := hashAccelOn.Load()
	defer setHashAccel(oldAccel)
	root := t.TempDir()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binary := filepath.Join(root, "obelisk"+ext)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"+ext), "build", "-mod=readonly", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("CLI build: %v: %s", err, out)
	}
	evidence := root
	if p := os.Getenv("OBX_JOBS_CLI_EVIDENCE"); p != "" {
		var err error
		evidence, err = os.MkdirTemp(p, "jobs-lifecycle-")
		if err != nil {
			t.Fatal(err)
		}
	}
	keyring := filepath.Join(root, "gnupg")
	if err := os.Mkdir(keyring, 0700); err != nil {
		t.Fatal(err)
	}
	env := []string{}
	for _, v := range os.Environ() {
		k := strings.ToUpper(strings.SplitN(v, "=", 2)[0])
		if k != "OBELISK_AUTH_TOKEN" && k != "MNEMO_AUTH_TOKEN" && k != "GNUPGHOME" {
			env = append(env, v)
		}
	}
	env = append(env, "OBELISK_AUTH_TOKEN=jobs-cli-synthetic-token", "GNUPGHOME="+keyring)
	run := func(name, dir string, serve bool) {
		t.Helper()
		before := configSnapshot(t, dir)
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		args := []string{"-data", dir, "-listen", addr}
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		log, err := os.Create(filepath.Join(evidence, name+".log"))
		if err != nil {
			t.Fatal(err)
		}
		defer log.Close()
		cmd.Stdout = log
		cmd.Stderr = log
		start := time.Now()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		reaped := false
		defer func() {
			if !reaped {
				_ = cmd.Process.Kill()
				<-done
			}
		}()
		ready, forced := false, false
		client := &http.Client{Timeout: 200 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
		deadline := time.Now().Add(8 * time.Second)
	loop:
		for time.Now().Before(deadline) {
			select {
			case <-done:
				reaped = true
				break loop
			default:
			}
			resp, err := client.Get("http://" + addr + "/")
			if err == nil {
				ready = resp.StatusCode == 200
				resp.Body.Close()
				if ready {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !reaped {
			forced = true
			if err := cmd.Process.Kill(); err != nil {
				t.Error(err)
			}
			<-done
			reaped = true
		}
		if err := log.Close(); err != nil {
			t.Fatal(err)
		}
		code := cmd.ProcessState.ExitCode()
		after := configSnapshot(t, dir)
		record := map[string]any{"command": append([]string{binary}, args...), "pid": cmd.Process.Pid, "start": start.UTC(), "finish": time.Now().UTC(), "native_exit": code, "forced_stop": forced, "http_ready": ready, "before": before, "after": after}
		b, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(evidence, name+".json"), b, 0600); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(evidence, name+".log"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "panic:") || strings.Contains(string(raw), "jobs-cli-synthetic-token") {
			t.Fatal("startup panicked or disclosed token")
		}
		if serve {
			if !ready || !forced {
				t.Fatal("valid job board did not serve")
			}
		} else {
			if ready || forced || code != 1 || !strings.Contains(string(raw), "job board") {
				t.Fatalf("expected natural job-load refusal; ready=%v forced=%v exit=%d", ready, forced, code)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("rejected startup mutated fixture")
			}
		}
		t.Logf("%s: ready=%v forced_stop=%v native_exit=%d", name, ready, forced, code)
	}
	for name, raw := range map[string]string{"malformed": `{"rows":[`, "null-row": `{"rows":[{"id":1,"status":"RUNNING"},null]}`, "duplicate": `{"rows":[{"id":1},{"id":1}]}`, "obstruction": ""} {
		dir := filepath.Join(root, name)
		if _, err := (&App{DataDir: dir}).InitializeConfig(); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenStore(dir); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "jobs.json")
		if name == "obstruction" {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
		}
		run(name, dir, false)
	}
	dir := filepath.Join(root, "valid")
	if _, err := (&App{DataDir: dir}).InitializeConfig(); err != nil {
		t.Fatal(err)
	}
	run("optional-first-use", dir, true)
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	j, err := s.NewJob("fixture", "interrupted")
	if err != nil {
		t.Fatal(err)
	}
	run("valid-reconcile", dir, true)
	s, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Job(j.ID); got == nil || got.Status != "INTERRUPTED" {
		t.Fatal("CLI reconciliation absent")
	}
	before := configSnapshot(t, dir)
	run("ordinary-restart", dir, true)
	if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
		t.Fatal("ordinary restart rewrote valid board")
	}
}
