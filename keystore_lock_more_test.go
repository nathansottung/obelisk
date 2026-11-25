//go:build !guionly

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// kvRefs reads the key references in the keystore at p.
func kvRefs(t *testing.T, p string) map[string]string {
	t.Helper()
	ks, err := readStore(p)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, k := range ks.Keys {
		out[k["key_ref"].(string)] = k["passphrase"].(string)
	}
	return out
}

// kvAllHold checks that every keystore holds exactly the same keys, and that
// each key in want is among them with its secret.
func kvAllHold(t *testing.T, paths []string, want map[string]string) map[string]string {
	t.Helper()
	first := kvRefs(t, paths[0])
	for _, p := range paths[1:] {
		got := kvRefs(t, p)
		if len(got) != len(first) {
			t.Fatalf("keystores hold %d and %d keys", len(first), len(got))
		}
		for ref, pass := range first {
			if got[ref] != pass {
				t.Fatalf("keystores disagree on key %s", ref)
			}
		}
	}
	for ref, pass := range want {
		if first[ref] != pass {
			t.Fatalf("key %s is missing or changed", ref)
		}
	}
	return first
}

// kvNoPrev checks that nothing is left to recover: each previous generation
// kept beside a keystore is whole and holds no key the keystore lacks.
func kvNoPrev(t *testing.T, paths []string) {
	t.Helper()
	for _, p := range paths {
		if _, err := os.Lstat(p + keystorePrevSuffix); err != nil {
			continue
		}
		prev, err := readStore(p + keystorePrevSuffix)
		if err != nil {
			t.Fatalf("the previous copy beside %s is not whole: %v", p, err)
		}
		cur, err := readStore(p)
		if err != nil || !keystoreCovers(cur, prev) {
			t.Errorf("the previous copy beside %s holds keys it lacks", p)
		}
	}
}

// Many generates at once each get their own key, and every one of them is in
// every keystore afterwards.
func TestKeystoreLock_GenerateAgainstGenerate(t *testing.T) {
	f := newKV(t, nil, nil)
	const workers, each = 6, 4
	var mu sync.Mutex
	want := map[string]string{}
	var wg sync.WaitGroup
	errs := make(chan error, workers*each)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				ref, pass, _, err := f.app.GenerateKey("synthetic")
				if err != nil {
					errs <- err
					continue
				}
				mu.Lock()
				want[ref] = pass
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("generate: %v", err)
	}
	if len(want) != workers*each {
		t.Fatalf("got %d distinct keys, want %d", len(want), workers*each)
	}
	if got := kvAllHold(t, f.paths, want); len(got) != len(want) {
		t.Fatalf("keystores hold %d keys, want %d", len(got), len(want))
	}
	kvNoPrev(t, f.paths)
}

// Generates and syncs interleaved freely lose nothing.
func TestKeystoreLock_GenerateAgainstSyncLoop(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	var mu sync.Mutex
	want := map[string]string{"A": "synthetic-secret-A"}
	var wg sync.WaitGroup
	fail := make(chan error, 64)
	for w := 0; w < 3; w++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				ref, pass, _, err := f.app.GenerateKey("synthetic")
				if err != nil {
					fail <- err
					return
				}
				mu.Lock()
				want[ref] = pass
				mu.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				if _, err := f.app.SyncKeystores(); err != nil {
					fail <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(fail)
	for err := range fail {
		t.Fatal(err)
	}
	kvAllHold(t, f.paths, want)
}

// The helper process for the two-process tests. It is a second Obelisk process
// with its own catalog, configured with the same keystore files.
func TestKeystoreLockHelperProcess(t *testing.T) {
	mode := os.Getenv("OBELISK_KS_HELPER")
	if mode == "" {
		t.Skip("helper process for the keystore lock tests")
	}
	paths := filepath.SplitList(os.Getenv("OBELISK_KS_PATHS"))
	switch mode {
	case "hold":
		release, err := lockKeystores(paths, false)
		if err != nil {
			fmt.Println("ERR", err)
			return
		}
		fmt.Println("LOCKED")
		_, _ = io.ReadAll(os.Stdin)
		release()
	case "generate":
		data := t.TempDir()
		st, err := openTestStore(t, data)
		if err != nil {
			fmt.Println("ERR", err)
			return
		}
		app := initializedTestApp(t, &App{DataDir: data, Store: st})
		if _, err := app.SaveConfig(map[string]any{"keystore_paths": paths}); err != nil {
			fmt.Println("ERR", err)
			return
		}
		for i := 0; i < 8; i++ {
			ref, _, _, err := app.GenerateKey("helper")
			if err != nil {
				fmt.Println("ERR", err)
				continue
			}
			fmt.Println("REF", ref)
		}
	}
}

func keystoreHelper(t *testing.T, mode string, paths []string) (*exec.Cmd, io.WriteCloser, *bufio.Scanner) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestKeystoreLockHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), "OBELISK_KS_HELPER="+mode, "OBELISK_KS_PATHS="+strings.Join(paths, string(filepath.ListSeparator)))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Wait() })
	return cmd, stdin, bufio.NewScanner(stdout)
}

// Another process holding the keystore lock stops a key change here: it waits,
// and past the wait it refuses and changes nothing. Once the other process lets
// go, the change runs.
func TestKeystoreLock_SecondProcessHoldsLock(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("B", "synthetic-secret-B")})
	prevWait := keystoreLockWait
	keystoreLockWait = 300 * time.Millisecond
	t.Cleanup(func() { keystoreLockWait = prevWait })

	_, stdin, out := keystoreHelper(t, "hold", f.paths)
	if !out.Scan() || out.Text() != "LOCKED" {
		t.Fatalf("helper did not take the lock: %q", out.Text())
	}
	before := [][]byte{kvRead(t, f.paths[0]), kvRead(t, f.paths[1])}
	if _, err := f.app.SyncKeystores(); !errors.Is(err, errKeystoreBusy) {
		t.Fatalf("sync while another process holds the lock: %v", err)
	}
	if _, _, _, err := f.app.GenerateKey("synthetic"); err == nil {
		t.Fatal("generate ran while another process held the lock")
	}
	for i, p := range f.paths {
		if string(kvRead(t, p)) != string(before[i]) {
			t.Fatal("a keystore changed while another process held the lock")
		}
	}
	_ = stdin.Close()
	keystoreLockWait = 10 * time.Second
	if _, err := f.app.SyncKeystores(); err != nil {
		t.Fatalf("sync after the other process let go: %v", err)
	}
	kvAllHold(t, f.paths, map[string]string{"A": "synthetic-secret-A", "B": "synthetic-secret-B"})
}

// Two processes, each with its own catalog and the same keystore files,
// generating and syncing at once, lose no key.
func TestKeystoreLock_TwoProcessesTwoCatalogs(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	cmd, _, out := keystoreHelper(t, "generate", f.paths)
	helperRefs := make(chan []string, 1)
	go func() {
		var refs []string
		for out.Scan() {
			line := out.Text()
			if r, ok := strings.CutPrefix(line, "REF "); ok {
				refs = append(refs, r)
			} else if strings.HasPrefix(line, "ERR") {
				refs = append(refs, "!"+line)
			}
		}
		helperRefs <- refs
	}()
	want := map[string]string{"A": "synthetic-secret-A"}
	for i := 0; i < 8; i++ {
		ref, pass, _, err := f.app.GenerateKey("synthetic")
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		want[ref] = pass
		if _, err := f.app.SyncKeystores(); err != nil {
			t.Fatalf("sync: %v", err)
		}
	}
	refs := <-helperRefs
	_ = cmd.Wait()
	if len(refs) != 8 {
		t.Fatalf("helper made %d keys: %v", len(refs), refs)
	}
	got := kvAllHold(t, f.paths, want)
	for _, r := range refs {
		if strings.HasPrefix(r, "!") {
			t.Fatalf("helper: %s", r)
		}
		if _, ok := got[r]; !ok {
			t.Fatalf("the other process's key %s was lost", r)
		}
	}
}

type ksCrash struct{}

// crashKeystoreAt makes the keystore write stop dead at step for path, as a
// crash would: no cleanup in the write runs.
func crashKeystoreAt(t *testing.T, step, path string) {
	t.Helper()
	prev := keystoreWriteStep
	keystoreWriteStep = func(s, p string) {
		if s == step && p == path {
			panic(ksCrash{})
		}
	}
	t.Cleanup(func() { keystoreWriteStep = prev })
}

func runUntilCrash(t *testing.T, fn func()) (crashed bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(ksCrash); !ok {
				panic(r)
			}
			crashed = true
		}
	}()
	fn()
	return false
}

// A crash at any point of a key change, including between the two keystore
// writes, leaves every key that was in a keystore readable from at least one
// file, and the next key change finishes the job with no key lost.
func TestKeystoreLock_CrashAtEveryKillPoint(t *testing.T) {
	steps := []string{"prev-kept", "staged", "renamed", "read-back", "done"}
	for _, op := range []string{"generate", "sync"} {
		for idx := 0; idx < 2; idx++ {
			for _, step := range steps {
				t.Run(fmt.Sprintf("%s/keystore%d/%s", op, idx, step), func(t *testing.T) {
					var f kvFixture
					held := map[string]string{"A": "synthetic-secret-A"}
					if op == "generate" {
						f = newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
					} else {
						// Each keystore lacks a key the other has, so the sync writes both.
						f = newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A"), kvKey("B", "synthetic-secret-B")},
							[]map[string]any{kvKey("A", "synthetic-secret-A"), kvKey("C", "synthetic-secret-C")})
						held["B"], held["C"] = "synthetic-secret-B", "synthetic-secret-C"
					}
					crashKeystoreAt(t, step, f.paths[idx])
					crashed := runUntilCrash(t, func() {
						if op == "generate" {
							_, _, _, _ = f.app.GenerateKey("synthetic")
						} else {
							_, _ = f.app.SyncKeystores()
						}
					})
					if !crashed {
						t.Fatal("the kill point was never reached")
					}
					keystoreWriteStep = nil
					// After the crash: each key still readable from some file.
					for ref, pass := range held {
						got, err := f.app.Passphrase(ref)
						if err != nil || got != pass {
							t.Fatalf("key %s is not readable after a crash at %s: %v", ref, step, err)
						}
					}
					// Every file that is there is whole.
					for _, p := range f.paths {
						for _, file := range []string{p, p + keystorePrevSuffix} {
							if _, err := os.Lstat(file); err != nil {
								continue
							}
							if _, err := f.app.readExistingKeystore(file, false); err != nil {
								t.Fatalf("%s is not a whole keystore after the crash: %v", filepath.Base(file), err)
							}
						}
					}
					// The next key change finishes it.
					if _, err := f.app.SyncKeystores(); err != nil {
						t.Fatalf("sync after the crash: %v", err)
					}
					kvAllHold(t, f.paths, held)
					kvNoPrev(t, f.paths)
					if !f.app.KeystoreStatus()["ok"].(bool) {
						t.Fatal("keystores not consistent after recovery")
					}
				})
			}
		}
	}
}

// A keystore left as its earlier copy alone (the new file lost) is rebuilt
// from that copy by the next key change, and its keys are readable meanwhile.
func TestKeystoreLock_RecoverFromEarlierCopyAlone(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	b := kvRead(t, f.paths[1])
	kvPut(t, f.paths[1]+keystorePrevSuffix, b)
	if err := os.Remove(f.paths[1]); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.paths[0]); err != nil {
		t.Fatal(err)
	}
	kvPut(t, f.paths[0], []byte("{not json"))
	if got, err := f.app.Passphrase("A"); err != nil || got != "synthetic-secret-A" {
		t.Fatalf("the earlier copy is not read: %v", err)
	}
	if w, _ := f.app.KeystoreStatus()["warning"].(string); !strings.Contains(w, "interrupted") {
		t.Errorf("status does not say a write was interrupted: %q", w)
	}
	// Keystore 0 is damaged with no earlier copy: that is for the user.
	if _, err := f.app.SyncKeystores(); err == nil {
		t.Fatal("sync accepted a damaged keystore")
	}
	kvPut(t, f.paths[0], b)
	rep, err := f.app.syncKeystores()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(rep.Recovered) != 1 || rep.Recovered[0] != f.paths[1] {
		t.Fatalf("recovered %v", rep.Recovered)
	}
	kvAllHold(t, f.paths, map[string]string{"A": "synthetic-secret-A"})
	kvNoPrev(t, f.paths)
}

// failReadOf makes the keystore at p unreadable, as an unplugged or failing
// drive is, for the rest of the test.
func failReadOf(f kvFixture, p string) {
	f.app.keystoreReadFile = func(q string) ([]byte, error) {
		if q == p || q == p+keystorePrevSuffix {
			return nil, &os.PathError{Op: "open", Path: q, Err: errors.New("the device is not ready")}
		}
		return os.ReadFile(q)
	}
}

// With one keystore offline, no key is generated, no sync runs and no
// encrypted build starts; the other keystore is untouched and its keys can
// still be read.
func TestKeystoreLock_OneKeystoreOffline(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	before := kvRead(t, f.paths[0])
	failReadOf(f, f.paths[1])
	if _, _, _, err := f.app.GenerateKey("synthetic"); err == nil {
		t.Fatal("a key was generated with a keystore offline")
	}
	if _, err := f.app.SyncKeystores(); err == nil {
		t.Fatal("sync ran with a keystore offline")
	}
	c := f.app.Store.AddChunk(Chunk{Name: "synthetic", Status: "PLANNED", Encrypted: true})
	if err := f.app.BuildChunk(c.ID, noProg); err == nil || !strings.Contains(err.Error(), "refusing to encrypt") {
		t.Fatalf("an encrypted build started with a keystore offline: %v", err)
	}
	if string(kvRead(t, f.paths[0])) != string(before) {
		t.Fatal("the reachable keystore changed")
	}
	if got, err := f.app.Passphrase("A"); err != nil || got != "synthetic-secret-A" {
		t.Fatalf("offline recovery from the other keystore failed: %v", err)
	}
	if len(f.app.Store.KeyMetas()) != 0 {
		t.Fatal("a key was recorded as issued")
	}
}

// A key that reached only one keystore is not issued.
func TestKeystoreLock_KeyInOneKeystoreIsNotIssued(t *testing.T) {
	f := newKV(t, nil, nil)
	failPublicationTo(t, f.paths[1])
	ref, pass, _, err := f.app.GenerateKey("synthetic")
	if err == nil || ref != "" || pass != "" {
		t.Fatal("a key reported as issued though one keystore was not written")
	}
	kvError(t, err)
	if !strings.Contains(err.Error(), "not issued") {
		t.Errorf("refusal does not say the key was not issued: %v", err)
	}
	if len(f.app.Store.KeyMetas()) != 0 {
		t.Fatal("a key was recorded as issued")
	}
}

// The normal path: a key goes to both keystores, nothing is left behind, and a
// sync of keystores that disagree keeps the union and says what each lacked.
func TestKeystoreLock_NormalPathAndSyncReport(t *testing.T) {
	f := newKV(t, nil, nil)
	ref, pass, fpr, err := f.app.GenerateKey("synthetic")
	if err != nil || ref == "" || pass == "" || fpr == "" {
		t.Fatalf("generate: %v", err)
	}
	kvAllHold(t, f.paths, map[string]string{ref: pass})
	kvNoPrev(t, f.paths)
	if !f.app.KeystoreStatus()["ok"].(bool) {
		t.Fatal("status not ok after a generate")
	}
	rep, err := f.app.syncKeystores()
	if err != nil || len(rep.Added) != 0 || rep.KeyCount != 1 {
		t.Fatalf("sync of agreeing keystores: %+v %v", rep, err)
	}

	// Keystore 1 gains a key keystore 0 lacks, and keystore 0 one it lacks.
	ks0, _ := readStore(f.paths[0])
	ks0.Keys = append(ks0.Keys, kvKey("K-ONLY0", "synthetic-secret-B"))
	if err := writeStore(f.paths[0], ks0); err != nil {
		t.Fatal(err)
	}
	ks1, _ := readStore(f.paths[1])
	ks1.Keys = append(ks1.Keys, kvKey("K-ONLY1", "synthetic-secret-C"))
	if err := writeStore(f.paths[1], ks1); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api(mux, f.app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/keys/sync", nil))
	if rec.Code != 200 {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	kvNoSecrets(t, rec.Body.String())
	if strings.Contains(rec.Body.String(), pass) {
		t.Fatal("the sync report holds a secret")
	}
	var got KeySyncReport
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	sort.Slice(got.Added, func(i, j int) bool { return got.Added[i].Keystore < got.Added[j].Keystore })
	if got.KeyCount != 3 || len(got.Added) != 2 ||
		got.Added[0].Keystore != f.paths[0] || strings.Join(got.Added[0].Keys, ",") != "K-ONLY1" ||
		got.Added[1].Keystore != f.paths[1] || strings.Join(got.Added[1].Keys, ",") != "K-ONLY0" {
		t.Fatalf("sync report: %+v", got)
	}
	kvAllHold(t, f.paths, map[string]string{ref: pass, "K-ONLY0": "synthetic-secret-B", "K-ONLY1": "synthetic-secret-C"})
	if a := f.app.Store.LastAudit(); a == nil || !strings.Contains(a.Detail, "kept the union") {
		t.Fatalf("the disagreement was not logged: %+v", a)
	}
}
