package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func inventoryFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "generated source")
	out := filepath.Join(root, "output")
	for _, dir := range []string{src, out, filepath.Join(src, "nested")} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for p, data := range map[string]string{"same.txt": "first", "nested/same.txt": "second", "empty.txt": "", "equal one.txt": "equal", "nested/雪.txt": "equal"} {
		if err := os.WriteFile(filepath.Join(src, filepath.FromSlash(p)), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return src, filepath.Join(out, "catalog.json")
}

func inventoryTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		if d.Type()&os.ModeSymlink != 0 {
			out[rel] = "link"
			return nil
		}
		if d.IsDir() {
			out[rel] = "directory"
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		h := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(h[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGUIInventoryObservedRecords(t *testing.T) {
	src, dst := inventoryFixture(t)
	before := inventoryTree(t, src)
	r, err := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard)
	if err != nil || !r.Published || r.Files != 5 {
		t.Fatalf("%+v %v", r, err)
	}
	if !reflect.DeepEqual(before, inventoryTree(t, src)) {
		t.Fatal("source modified")
	}
	s, err := loadGUICatalog(dst, readGUICatalogFile)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"same.txt": "first", "nested/same.txt": "second", "empty.txt": "", "equal one.txt": "equal", "nested/雪.txt": "equal"}
	seen := map[int]bool{}
	hashIDs := map[string][]int{}
	for _, f := range s.store.c.Files {
		data, ok := want[f.RelPath]
		if !ok {
			t.Fatal(f.RelPath)
		}
		h := sha256.Sum256([]byte(data))
		info, e := os.Stat(filepath.Join(src, filepath.FromSlash(f.RelPath)))
		if e != nil {
			t.Fatal(e)
		}
		if seen[f.ID] || f.ID <= 0 || f.Hash != hex.EncodeToString(h[:]) || f.SizeBytes != int64(len(data)) || !f.ModTime.Equal(info.ModTime()) || f.FirstSeen.IsZero() || len(f.Blake3) != 64 {
			t.Fatalf("bad observed record: %+v", f)
		}
		seen[f.ID] = true
		hashIDs[f.Hash] = append(hashIDs[f.Hash], f.ID)
	}
	equal := sha256.Sum256([]byte("equal"))
	if len(hashIDs[hex.EncodeToString(equal[:])]) != 2 {
		t.Fatal("equal content collapsed")
	}
	if len(s.store.c.Chunks) != 0 || len(s.store.c.Volumes) != 0 || len(s.store.c.Locations) != 0 || s.store.c.Folders[0].Path != src {
		t.Fatal("invented physical backup/storage evidence")
	}
	// The reader must work when the source no longer exists at its recorded path.
	// Test-owned rename, after producer completion, not a producer source mutation.
	raw, _ := os.ReadFile(dst)
	if err = os.Rename(src, src+" unavailable"); err != nil {
		t.Fatal(err)
	}
	var viewed bytes.Buffer
	if err = runGUICatalog([]string{dst}, strings.NewReader("{\"text\":\"nested/same.txt\"}\n"), &viewed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(viewed.String(), `"ok":true`) {
		t.Fatal(viewed.String())
	}
	after, _ := os.ReadFile(dst)
	if !bytes.Equal(raw, after) {
		t.Fatal("reader wrote catalog")
	}
}

func TestGUIInventoryRefusalAndBounds(t *testing.T) {
	for _, name := range []string{"missing", "wrong-kind", "overlap", "ancestor-output", "preexisting", "file-count", "entry-count", "depth", "path", "file-bytes", "total-bytes", "enumerate", "stat", "read", "hashed-change", "cancel", "cancel-late", "output-parent-missing", "write", "link"} {
		t.Run(name, func(t *testing.T) {
			src, dst := inventoryFixture(t)
			limits := disposableInventoryLimits
			ops := inventoryIO{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := src
			switch name {
			case "missing":
				src = filepath.Join(filepath.Dir(src), "missing")
			case "wrong-kind":
				src = filepath.Join(src, "same.txt")
			case "overlap":
				dst = filepath.Join(src, "catalog.json")
			case "ancestor-output":
				dst = filepath.Join(filepath.Dir(src), "catalog.json")
			case "preexisting":
				if e := os.WriteFile(dst, []byte("keep existing"), 0600); e != nil {
					t.Fatal(e)
				}
			case "file-count":
				limits.files = 4
			case "entry-count":
				limits.entries = 2
			case "depth":
				limits.depth = 0
			case "path":
				limits.path = 3
			case "file-bytes":
				limits.fileBytes = 2
			case "total-bytes":
				limits.totalBytes = 6
			case "cancel":
				cancel()
			case "cancel-late":
				ops.before = func(stage, p string) error {
					if stage == "publish" {
						cancel()
					}
					return nil
				}
			case "enumerate", "stat", "read":
				ops.before = func(stage, p string) error {
					if stage == name {
						return os.ErrPermission
					}
					return nil
				}
			case "hashed-change":
				ops.before = func(stage, p string) error {
					if stage == "hashed" {
						return os.WriteFile(p, []byte("intentional test change"), 0600)
					}
					return nil
				}
			case "output-parent-missing":
				dst = filepath.Join(filepath.Dir(dst), "absent", "catalog.json")
			case "write":
				ops.write = func(f *os.File, b []byte) (int, error) { return f.Write(b[:3]) }
			case "link":
				ops.link = func(string, string) error { return errors.New("injected unsupported output filesystem") }
			}
			before := inventoryTree(t, original)
			r, e := produceGUIInventory(ctx, src, dst, limits, ops, io.Discard)
			if e == nil || r.Published {
				t.Fatalf("false success %+v %v", r, e)
			}
			if name == "preexisting" {
				raw, _ := os.ReadFile(dst)
				if string(raw) != "keep existing" {
					t.Fatal("replaced existing output")
				}
			} else if _, e = os.Lstat(dst); !errors.Is(e, os.ErrNotExist) {
				t.Fatalf("published failed output: %v", e)
			}
			if name != "hashed-change" && !reflect.DeepEqual(before, inventoryTree(t, original)) {
				t.Fatal("source changed")
			}
			matches, _ := filepath.Glob(filepath.Join(filepath.Dir(dst), ".inventory-*.tmp"))
			if len(matches) != 0 {
				t.Fatal("unpublished staging leaked", matches)
			}
		})
	}
}

func TestGUIInventoryPublicationPoint(t *testing.T) {
	for _, name := range []string{"late-destination", "cleanup-after-success", "cleanup-after-failure", "progress-failure", "changed-directory"} {
		t.Run(name, func(t *testing.T) {
			src, dst := inventoryFixture(t)
			ops := inventoryIO{}
			progress := io.Writer(io.Discard)
			switch name {
			case "late-destination":
				ops.before = func(stage, p string) error {
					if stage == "publish" {
						return os.WriteFile(dst, []byte("late owner"), 0600)
					}
					return nil
				}
			case "cleanup-after-success":
				ops.remove = func(string) error { return os.ErrPermission }
			case "cleanup-after-failure":
				ops.remove = func(string) error { return os.ErrPermission }
				ops.link = func(string, string) error { return os.ErrPermission }
			case "progress-failure":
				progress = inventoryErrorWriter{}
			case "changed-directory":
				ops.before = func(stage, p string) error {
					if stage == "validate" {
						return os.WriteFile(filepath.Join(src, "late.txt"), []byte("setup"), 0600)
					}
					return nil
				}
			}
			r, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, ops, progress)
			if e == nil {
				t.Fatal("expected error")
			}
			if r.Published != (name == "cleanup-after-success") {
				t.Fatalf("publication truth lost %+v %v", r, e)
			}
			if name == "late-destination" {
				raw, _ := os.ReadFile(dst)
				if string(raw) != "late owner" {
					t.Fatal("late output overwritten")
				}
			}
			if r.Published {
				if _, e := loadGUICatalog(dst, readGUICatalogFile); e != nil {
					t.Fatal(e)
				}
			}
			if strings.HasPrefix(name, "cleanup-") {
				if r.Staging == "" {
					t.Fatal("unreported staging")
				}
				t.Logf("retained test-owned staging %s; published=%t", r.Staging, r.Published)
			}
		})
	}
}

type inventoryErrorWriter struct{}

func (inventoryErrorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestGUIInventoryEmptyAndCLI(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "empty")
	out := filepath.Join(root, "output")
	for _, p := range []string{src, out} {
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
	}
	dst := filepath.Join(out, "empty.json")
	var status bytes.Buffer
	if e := runGUIInventory([]string{src, dst}, &status, io.Discard); e != nil {
		t.Fatal(e)
	}
	var r inventoryResult
	if e := json.Unmarshal(status.Bytes(), &r); e != nil || !r.Published || r.Files != 0 {
		t.Fatal(status.String(), e)
	}
	s, e := loadGUICatalog(dst, readGUICatalogFile)
	if e != nil || len(s.store.c.Files) != 0 {
		t.Fatal(e)
	}
	if e = runGUIInventory(nil, io.Discard, io.Discard); e == nil {
		t.Fatal("implicit roots allowed")
	}
	if e = runGUIInventory([]string{src, filepath.Join(out, "status-error.json")}, inventoryErrorWriter{}, io.Discard); e == nil || !strings.Contains(e.Error(), "published=true") {
		t.Fatal("post-publication status error concealed", e)
	}
}

func TestGUIInventoryAtLimits(t *testing.T) {
	src, dst := inventoryFixture(t)
	limits := inventoryLimits{files: 5, entries: 6, depth: 1, path: 15, fileBytes: 6, totalBytes: 21}
	r, e := produceGUIInventory(context.Background(), src, dst, limits, inventoryIO{}, io.Discard)
	if e != nil || !r.Published || r.Files != 5 || r.Bytes != 21 {
		t.Fatal(r, e)
	}
}

func TestGUIInventoryObjectSeparation(t *testing.T) {
	src, dst := inventoryFixture(t)
	info, e := os.Lstat(src)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{src, filepath.Join(src, "nested")} {
		inside, e := inventoryObjectWithin(info, p)
		if e != nil || !inside {
			t.Fatal(p, inside, e)
		}
	}
	inside, e := inventoryObjectWithin(info, filepath.Dir(dst))
	if e != nil || inside {
		t.Fatal(inside, e)
	}
	if inventoryWithin(src, src+"-different") {
		t.Fatal("unchecked path prefix containment")
	}
}

func TestGUIInventorySymlink(t *testing.T) {
	src, dst := inventoryFixture(t)
	sentinel := filepath.Join(filepath.Dir(src), "sentinel")
	if e := os.Mkdir(sentinel, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(sentinel, "private-test.txt"), []byte("test sentinel"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(sentinel, filepath.Join(src, "link")); e != nil {
		t.Skipf("native symlink creation unavailable without privilege changes: %v", e)
	}
	before := inventoryTree(t, sentinel)
	r, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard)
	if e == nil || r.Published || !strings.Contains(e.Error(), "link/reparse") {
		t.Fatal(r, e)
	}
	if !reflect.DeepEqual(before, inventoryTree(t, sentinel)) {
		t.Fatal("sentinel changed")
	}
}
