//go:build !guionly

package main

import (
	"os"
	"sync"
	"testing"
	"time"
)

type kvGen struct {
	ref, pass string
	err       error
}

// A sync that read the keystores before a key was generated must not write its
// older snapshot over the new key. Each time the sync reads the last keystore it
// is held there while a generate runs (for up to half a second, so a generate
// the lock holds back does not stall the test), and every key generated meanwhile
// must be in both keystores afterwards. Before the keystore lock, the sync wrote
// back {A} to both files after the generate had added its key to both, and both
// calls reported success.
func TestKeystoreLock_SyncPausedAfterReadKeepsGeneratedKey(t *testing.T) {
	f := newKV(t, []map[string]any{kvKey("A", "synthetic-secret-A")}, []map[string]any{kvKey("A", "synthetic-secret-A")})
	var mu sync.Mutex
	syncing, genInFlight := true, 0
	var gens []chan kvGen
	f.app.keystoreReadFile = func(p string) ([]byte, error) {
		b, err := os.ReadFile(p)
		mu.Lock()
		pause := syncing && genInFlight == 0 && p == f.paths[len(f.paths)-1]
		if pause {
			genInFlight++
		}
		mu.Unlock()
		if !pause {
			return b, err
		}
		done := make(chan kvGen, 1)
		mu.Lock()
		gens = append(gens, done)
		mu.Unlock()
		finished := make(chan struct{})
		go func() {
			ref, pass, _, gerr := f.app.GenerateKey("synthetic")
			mu.Lock()
			genInFlight--
			mu.Unlock()
			done <- kvGen{ref, pass, gerr}
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(500 * time.Millisecond):
		}
		return b, err
	}

	_, syncErr := f.app.SyncKeystores()
	mu.Lock()
	syncing = false
	mu.Unlock()
	if syncErr != nil {
		t.Fatalf("sync: %v", syncErr)
	}
	want := map[string]string{"A": "synthetic-secret-A"}
	if len(gens) == 0 {
		t.Fatal("the sync never read the last keystore")
	}
	for _, ch := range gens {
		g := <-ch
		if g.err != nil {
			t.Fatalf("generate: %v", g.err)
		}
		want[g.ref] = g.pass
	}
	f.app.keystoreReadFile = nil
	for _, p := range f.paths {
		kvMappings(t, p, want)
	}
	for ref, pass := range want {
		got, err := f.app.Passphrase(ref)
		if err != nil || got != pass {
			t.Fatalf("key %s cannot be fetched after the sync: %v", ref, err)
		}
	}
}
