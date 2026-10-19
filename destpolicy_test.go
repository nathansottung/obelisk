//go:build !guionly

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDestAuth_F5ExpectedDigestMismatch(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"good": "PRIOR GOOD"})
	prior := identOf(t, filepath.Join(dir, "good"))
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	rel, f, err := d.stagingFor("good")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("EXPECTED"); err != nil {
		t.Fatal(err)
	}
	if err := d.VerifyStage(rel, sha256Hex([]byte("EXPECTED"))); err != nil {
		t.Fatal(err)
	}
	// Controlled same-object corruption, not an isolation claim. An observed
	// mismatch must never be adopted as the expected source digest.
	if _, err := f.WriteAt([]byte("MUTATED!"), 0); err != nil {
		t.Fatal(err)
	}
	if err := d.Publish(rel, "good"); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("publish: %v", err)
	}
	if err := d.VerifyStage(rel, sha256Hex([]byte("MUTATED!"))); err == nil {
		t.Fatal("adopted replacement digest")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "good")); string(got) != "PRIOR GOOD" || identOf(t, filepath.Join(dir, "good")) != prior {
		t.Fatal("prior destination changed")
	}
	if err := d.Discard(rel); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteFileAtomic("good", []byte("SAFE RETRY"), 0o600); err != nil {
		t.Fatal(err)
	}
	rel, f, err = d.stagingFor("first-check-mismatch")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("SUBSTITUTE"); err != nil {
		t.Fatal(err)
	}
	if err := d.VerifyStage(rel, sha256Hex([]byte("SOURCE"))); err == nil {
		t.Fatal("initial mismatch admitted")
	}
	if err := d.VerifyStage(rel, sha256Hex([]byte("SUBSTITUTE"))); err == nil {
		t.Fatal("failed verification forgot the source digest")
	}
	if err := d.Discard(rel); err != nil {
		t.Fatal(err)
	}
}

func TestDestAuth_F5IndependentStages(t *testing.T) {
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("owned-%d", i)
			errs <- d.WriteFileAtomic(name, []byte(name), 0o600)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("owned-%d", i)
		if got, err := d.HashRel(name); err != nil || got != sha256Hex([]byte(name)) {
			t.Fatalf("%s: %s %v", name, got, err)
		}
	}
}

func TestDestAuth_F5CooperatingJobs(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	first, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	contended := make(chan struct{}, 1)
	old := destLeaseContended
	destLeaseContended = func() { contended <- struct{}{} }
	defer func() { destLeaseContended = old }()
	done := make(chan error, 1)
	go func() {
		second, err := app.Store.AuthorizeDestination(filepath.Join(dir, "."))
		if err == nil {
			err = second.WriteFileAtomic("same", []byte("SECOND JOB"), 0o600)
			if cerr := second.Close(); err == nil {
				err = cerr
			}
		}
		done <- err
	}()
	select {
	case <-contended:
	case err := <-done:
		t.Fatalf("second job did not coordinate: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("no deterministic contention observation")
	}
	if err := first.WriteFileAtomic("same", []byte("FIRST JOB"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := first.HashRel("same"); err != nil || got != sha256Hex([]byte("FIRST JOB")) {
		t.Fatalf("first job verification: %s %v", got, err)
	}
	// The lease is still held here, where a real job records catalog credit.
	select {
	case err := <-done:
		t.Fatalf("lease released before job completion: %v", err)
	default:
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "same")); string(got) != "SECOND JOB" {
		t.Fatalf("second job did not finish: %q", got)
	}
}

func TestDestAuth_F5CooperatingProcesses(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	d, err := app.Store.AuthorizeDestination(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDestAuth_F5LeaseProcessHelper$")
	cmd.Env = append(os.Environ(), "OBELISK_LEASE_TEST_DIR="+dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewScanner(stdout)
	if !r.Scan() || r.Text() != "CONTENDED" {
		t.Fatalf("child did not observe held lease: %q %v", r.Text(), r.Err())
	}
	if err := d.WriteFileAtomic("owned", []byte("VERIFIED"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !r.Scan() || r.Text() != "ACQUIRED" {
		t.Fatalf("child did not acquire released lease: %q %v", r.Text(), r.Err())
	}
	for r.Scan() {
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestDestAuth_F5LeaseProcessHelper(t *testing.T) {
	dir := os.Getenv("OBELISK_LEASE_TEST_DIR")
	if dir == "" {
		return
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	destLeaseContended = func() { fmt.Println("CONTENDED") }
	release, err := acquireDestLease(r)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ACQUIRED")
	if err := release(); err != nil {
		t.Fatal(err)
	}
}
