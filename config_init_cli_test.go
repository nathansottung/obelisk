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
	"strconv"
	"strings"
	"testing"
	"time"
)

// Build the real program (not a helper-only startup surrogate), then exercise the
// same initialize-and-serve/stop/normal-start sequence described for containers.
func TestConfigInitCLI_Lifecycle(t *testing.T) {
	root := t.TempDir()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binary := filepath.Join(root, "obelisk"+ext)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go"+ext), "build", "-mod=readonly", "-o", binary, ".")
	if out, e := build.CombinedOutput(); e != nil {
		_ = out
		t.Fatal("CLI fixture binary build failed")
	}
	evidence := root
	if retained := os.Getenv("OBX_CONFIG_CLI_EVIDENCE"); retained != "" {
		if e := os.MkdirAll(retained, 0700); e != nil {
			t.Fatal("CLI evidence directory unavailable")
		}
		var e error
		evidence, e = os.MkdirTemp(retained, "lifecycle-")
		if e != nil {
			t.Fatal("CLI evidence allocation failed")
		}
	}
	// Exclude inherited real bearer tokens; fixtures use only this synthetic token.
	env := []string{}
	for _, v := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(v, "=", 2)[0])
		if key != "OBELISK_AUTH_TOKEN" && key != "MNEMO_AUTH_TOKEN" && key != "GNUPGHOME" {
			env = append(env, v)
		}
	}
	keyring := filepath.Join(root, "gnupg")
	if os.Mkdir(keyring, 0700) != nil {
		t.Fatal("keyring fixture failed")
	}
	env = append(env, "OBELISK_AUTH_TOKEN=cli-fixture-secret", "GNUPGHOME="+keyring)
	run := func(name, dir string, initialize, serve bool) {
		t.Helper()
		before := configSnapshot(t, dir)
		listener, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal("loopback port allocation failed")
		}
		addr := listener.Addr().String()
		if listener.Close() != nil {
			t.Fatal("port reservation close failed")
		}
		args := []string{"-data", dir, "-listen", addr}
		if initialize {
			args = append(args, "-init-config")
		}
		out, e := os.Create(filepath.Join(evidence, name+".log"))
		if e != nil {
			t.Fatal("CLI log allocation failed")
		}
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		cmd.Stdout = out
		cmd.Stderr = out
		start := time.Now()
		if cmd.Start() != nil {
			out.Close()
			t.Fatal("CLI did not start")
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		reaped, forced, ready := false, false, false
		code := -999
		// All paths kill and wait for this exact process before fixture reuse/cleanup.
		defer func() {
			if !reaped {
				_ = cmd.Process.Kill()
				<-done
			}
			_ = out.Close()
		}()
		client := &http.Client{Timeout: 200 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}}
		deadline := time.Now().Add(8 * time.Second)
	loop:
		for time.Now().Before(deadline) {
			select {
			case <-done:
				reaped = true
				code = cmd.ProcessState.ExitCode()
				break loop
			default:
			}
			if serve {
				resp, e := client.Get("http://" + addr + "/")
				if e == nil {
					ready = resp.StatusCode == http.StatusOK
					resp.Body.Close()
					if ready {
						break
					}
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		if ready {
			// Readiness must be sustained: initialization continues serving, not exit-only.
			time.Sleep(50 * time.Millisecond)
			select {
			case <-done:
				reaped = true
				code = cmd.ProcessState.ExitCode()
				t.Error("initialized process exited instead of continuing to serve")
			default:
				resp, e := client.Get("http://" + addr + "/")
				if e != nil {
					t.Error("server did not remain ready")
				} else {
					if resp.StatusCode != 200 {
						t.Error("server readiness changed")
					}
					resp.Body.Close()
				}
			}
		}
		if !reaped {
			forced = true
			if cmd.Process.Kill() != nil {
				t.Error("CLI forced stop failed")
			}
			<-done
			reaped = true
			code = cmd.ProcessState.ExitCode()
		}
		if e := out.Close(); e != nil {
			t.Error("CLI log close failed")
		}
		record := map[string]any{"command": append([]string{binary}, args...), "start": start.UTC(), "finish": time.Now().UTC(), "pid": cmd.Process.Pid, "native_exit": code, "forced_stop": forced, "http_ready": ready, "before": before, "after": configSnapshot(t, dir)}
		b, e := json.MarshalIndent(record, "", "  ")
		if e != nil || os.WriteFile(filepath.Join(evidence, name+".json"), b, 0600) != nil {
			t.Fatal("CLI evidence write failed")
		}
		raw, e := os.ReadFile(filepath.Join(evidence, name+".log"))
		if e != nil {
			t.Fatal("CLI log read failed")
		}
		if strings.Contains(string(raw), "cli-fixture-secret") {
			t.Fatal("CLI exposed synthetic token")
		}
		t.Log(name + ": readiness=" + strconv.FormatBool(ready) + ", forced_stop=" + strconv.FormatBool(forced) + ", native_exit=" + strconv.Itoa(code))
		if serve {
			if !ready || !forced {
				t.Fatal("expected continued serving followed by bounded forced stop")
			}
		} else if ready || forced || code != 1 {
			t.Fatal("expected natural refusal exit 1")
		}
	}
	fresh := filepath.Join(root, "fresh")
	if os.Mkdir(fresh, 0700) != nil {
		t.Fatal("fixture failed")
	}
	run("fresh-refused", fresh, false, false)
	if len(configSnapshot(t, fresh)) != 1 {
		t.Fatal("ordinary first use created state")
	}
	run("first-use", fresh, true, true)
	cfgPath := filepath.Join(fresh, "config.json")
	if _, e := os.Stat(cfgPath); e != nil {
		t.Fatal("first use did not initialize")
	}
	if _, e := os.Stat(filepath.Join(fresh, "catalog.json")); e != nil {
		t.Fatal("first use did not reach catalog startup")
	}
	before := configSnapshot(t, fresh)
	run("ordinary-restart", fresh, false, true)
	if !reflect.DeepEqual(before, configSnapshot(t, fresh)) {
		t.Fatal("ordinary restart changed fixture bytes")
	}
	// A non-default valid config must survive -init-config as a no-op.
	valid := []byte(`{"auth_token":"cli-fixture-secret","hash_accel":false,"tools":{"gpg":"synthetic-helper"},"keystore_paths":["synthetic-key-path"]}`)
	if os.WriteFile(cfgPath, valid, 0600) != nil {
		t.Fatal("fixture failed")
	}
	before = configSnapshot(t, fresh)
	run("existing-valid", fresh, true, true)
	if !reflect.DeepEqual(before, configSnapshot(t, fresh)) {
		t.Fatal("repeat initialization changed valid settings")
	}
	for _, kind := range []string{"malformed", "empty", "unreadable"} {
		dir := filepath.Join(root, kind)
		if os.Mkdir(dir, 0700) != nil {
			t.Fatal("fixture failed")
		}
		p := filepath.Join(dir, "config.json")
		if kind == "unreadable" {
			if os.Mkdir(p, 0700) != nil {
				t.Fatal("fixture failed")
			}
		} else {
			b := []byte("{")
			if kind == "empty" {
				b = nil
			}
			if os.WriteFile(p, b, 0600) != nil {
				t.Fatal("fixture failed")
			}
		}
		before := configSnapshot(t, dir)
		run(kind, dir, true, false)
		if !reflect.DeepEqual(before, configSnapshot(t, dir)) {
			t.Fatal("refusal changed damaged/unreadable fixture")
		}
	}
	existing := filepath.Join(root, "existing-state")
	if os.Mkdir(existing, 0700) != nil {
		t.Fatal("fixture failed")
	}
	if _, e := OpenStore(existing); e != nil {
		t.Fatal("catalog fixture failed")
	}
	if os.WriteFile(filepath.Join(existing, "synthetic-keystore.json"), []byte(`{"obelisk_keystore":1,"keys":[]}`), 0600) != nil {
		t.Fatal("key fixture failed")
	}
	before = configSnapshot(t, existing)
	run("existing-state-refused", existing, false, false)
	if !reflect.DeepEqual(before, configSnapshot(t, existing)) {
		t.Fatal("ordinary startup changed existing state")
	}
	run("existing-state-defaults", existing, true, true)
	after := configSnapshot(t, existing)
	delete(after, "config.json")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("deliberate defaults changed existing catalog/key state")
	}
	initialized, e := (&App{DataDir: existing}).LoadConfig()
	if e != nil || len(initialized.KeystorePaths) != 0 || initialized.AuthToken != "" {
		t.Fatal("defaults were incorrectly described as recovered settings")
	}
}
