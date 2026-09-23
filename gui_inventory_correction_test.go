package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGUIEncodingOriginalBytes(t *testing.T) {
	bad := [][]byte{[]byte{'"', 0xff, '"'}, []byte{'"', 0xc3, '"'}, []byte{'"', 0xed, 0xa0, 0x80, '"'}, []byte(`"\ud800"`), []byte(`"\udfff"`), []byte(`"\ud800x"`), []byte(`"\ud800\u0061"`), []byte(`"\ud800\ud800"`), []byte(`"\u123`)}
	for i, raw := range bad {
		if validateGUIJSON(raw) == nil {
			t.Fatalf("accepted malformed case %d: %x", i, raw)
		}
	}
	for _, raw := range []string{`"\ufffd"`, `"\ud83d\ude80"`, `"caf\u00e9\u96ea"`, `"\\uD800"`, `"quote\"\\"`, `"line\r\n\tname"`} {
		if e := validateGUIJSON([]byte(raw)); e != nil {
			t.Fatal(raw, e)
		}
	}
}

func TestGUIEncodingCompleteAdoption(t *testing.T) {
	src, dst := inventoryFixture(t)
	if _, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(dst)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"empty.txt", "same.txt"} {
		for _, value := range [][]byte{[]byte(`"x\ud800.txt"`), {'"', 'x', 255, '"'}} {
			original := bytes.Replace(raw, []byte(`"rel_path":"`+name+`"`), append([]byte(`"rel_path":`), value...), 1)
			if bytes.Equal(raw, original) {
				t.Fatal("missing probe record")
			}
			if s, e := loadGUICatalog("original bytes", func(string) ([]byte, error) { return original, nil }); e == nil || s != nil {
				t.Fatal("partial or repaired adoption", name, e)
			}
		}
	}
	// A legitimate replacement character is not corruption.
	valid := bytes.Replace(raw, []byte(`"rel_path":"empty.txt"`), []byte(`"rel_path":"literal\ufffd.txt"`), 1)
	if _, e := loadGUICatalog("valid bytes", func(string) ([]byte, error) { return valid, nil }); e != nil {
		t.Fatal(e)
	}
	if _, _, e := inventoryPath(string([]byte{'C', ':', '\\', 255})); e == nil {
		t.Fatal("invalid path bytes accepted")
	}
	after, _ := os.ReadFile(dst)
	if !bytes.Equal(raw, after) {
		t.Fatal("original output changed")
	}
}

func TestGUIExactNameProtocol(t *testing.T) {
	src, dst := inventoryFixture(t)
	if _, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(dst)
	var c catalog
	if e := json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	c.Audit = nil // Data-only older catalog: scope genuinely unknown.
	names := []string{"line\nname.txt", "line\rname.txt", "line\r\nname.txt", `line\nname.txt`, "linename.txt", "\nleading and trailing\r", "\ttab\t", " space ", ""}
	base := *c.Files[0]
	c.Files = nil
	for i, name := range names {
		f := base
		f.ID = i + 1
		f.RelPath = name
		c.Files = append(c.Files, &f)
	}
	encoded, _ := json.Marshal(c)
	foreign := filepath.Join(filepath.Dir(dst), "foreign.json")
	if e := os.WriteFile(foreign, encoded, 0600); e != nil {
		t.Fatal(e)
	}
	for i, name := range names {
		request, _ := json.Marshal(map[string]string{"exact": name})
		var output bytes.Buffer
		if e := runGUICatalog([]string{foreign}, bytes.NewReader(append(request, '\n')), &output); e != nil {
			t.Fatal(e)
		}
		lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
		var response struct {
			OK  bool     `json:"ok"`
			IDs []string `json:"ids"`
		}
		if e := json.Unmarshal(lines[1], &response); e != nil {
			t.Fatal(e)
		}
		if !response.OK || len(response.IDs) != 1 || response.IDs[0] != jsonNumber(i+1) {
			t.Fatalf("%q: %s", name, lines[1])
		}
	}
	for _, request := range []string{`{"exact":"\ud800"}`, `{"exact":"x","text":"different"}`, `{"exact":"x","hash":"a"}`} {
		var out bytes.Buffer
		if e := runGUICatalog([]string{foreign}, strings.NewReader(request+"\n"), &out); e != nil {
			t.Fatal(e)
		}
		if !bytes.Contains(bytes.Split(bytes.TrimSpace(out.Bytes()), []byte{'\n'})[1], []byte(`"ok":false`)) {
			t.Fatal("bad exact query succeeded")
		}
	}
}

func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestGUIInventoryExclusion(t *testing.T) {
	src, dst := inventoryFixture(t)
	for _, name := range []string{".DS_Store", "nested/.DS_Store", ".DS_Store.bak", "photo.DS_Store", "._.DS_Store", "photo.xmp", "photo.aae", "__MACOSX/value", "directory/.DS_Store/valuable"} {
		p := filepath.Join(src, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte("keep "+name), 0600); e != nil {
			t.Fatal(e)
		}
	}
	before := inventoryTree(t, src)
	for _, ignore := range []bool{false, true} {
		output := dst
		if ignore {
			output = filepath.Join(filepath.Dir(dst), "on.json")
		}
		reads := []string{}
		ops := inventoryIO{ignoreDSStore: ignore, before: func(stage, p string) error {
			if stage == "read" {
				reads = append(reads, p)
				if ignore && filepath.Base(p) == ".DS_Store" {
					t.Fatal("excluded contents opened")
				}
			}
			return nil
		}}
		r, e := produceGUIInventory(context.Background(), src, output, disposableInventoryLimits, ops, io.Discard)
		if e != nil || !r.Published {
			t.Fatal(r, e)
		}
		s, e := loadGUICatalog(output, readGUICatalogFile)
		if e != nil {
			t.Fatal(e)
		}
		scope, e := inventoryScope(&s.store.c)
		if e != nil || scope == nil {
			t.Fatal(e)
		}
		excluded := 0
		if ignore {
			excluded = 2
		}
		if scope.ExcludedFiles != excluded || scope.RegularFiles != 14 || scope.IncludedFiles != 14-excluded || len(reads) != 14-excluded {
			t.Fatal(scope, reads)
		}
		if !reflect.DeepEqual(before, inventoryTree(t, src)) {
			t.Fatal("source modified")
		}
	}
}

func TestGUIInventoryExcludedBoundaries(t *testing.T) {
	for _, kind := range []string{"all-excluded", "empty", "near-case", "type-link", "stat-error", "cancel", "entry-limit", "file-limit", "existing", "late", "changed-excluded"} {
		t.Run(kind, func(t *testing.T) {
			src, dst := inventoryFixture(t)
			empty := filepath.Join(filepath.Dir(src), "special")
			if e := os.Mkdir(empty, 0700); e != nil {
				t.Fatal(e)
			}
			src = empty
			if kind != "empty" {
				if e := os.WriteFile(filepath.Join(src, ".DS_Store"), []byte("keep"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			ops := inventoryIO{ignoreDSStore: true}
			limits := disposableInventoryLimits
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "near-case":
				for i, name := range []string{".ds_store", ".DS_STORE", ".dstore"} { // distinct numbered directories permit case variants on Windows
					p := filepath.Join(src, jsonNumber(i)+"-container")
					if e := os.Mkdir(p, 0700); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(filepath.Join(p, name), []byte("include"), 0600); e != nil {
						t.Fatal(e)
					}
				}
			case "type-link":
				if e := os.Remove(filepath.Join(src, ".DS_Store")); e != nil {
					t.Fatal(e)
				}
				target := filepath.Join(filepath.Dir(src), "sentinel.txt")
				if e := os.WriteFile(target, []byte("sentinel"), 0600); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(target, filepath.Join(src, ".DS_Store")); e != nil {
					t.Skip(e)
				}
			case "stat-error":
				ops.before = func(stage, p string) error {
					if stage == "stat" {
						return os.ErrPermission
					}
					return nil
				}
			case "cancel":
				ops.before = func(stage, p string) error {
					if stage == "publish" {
						cancel()
					}
					return nil
				}
			case "entry-limit":
				limits.entries = 0
			case "file-limit":
				limits.files = 0
			case "existing":
				if e := os.WriteFile(dst, []byte("existing"), 0600); e != nil {
					t.Fatal(e)
				}
			case "late":
				ops.before = func(stage, p string) error {
					if stage == "publish" {
						return os.WriteFile(dst, []byte("late"), 0600)
					}
					return nil
				}
			case "changed-excluded":
				ops.before = func(stage, p string) error {
					if stage == "validate" {
						return os.WriteFile(filepath.Join(src, ".DS_Store"), []byte("intentional setup change"), 0600)
					}
					return nil
				}
			}
			r, e := produceGUIInventory(ctx, src, dst, limits, ops, io.Discard)
			success := kind == "all-excluded" || kind == "empty" || kind == "near-case"
			if (e == nil) != success || r.Published != success {
				t.Fatal(r, e)
			}
			if success {
				s, e := loadGUICatalog(dst, readGUICatalogFile)
				if e != nil {
					t.Fatal(e)
				}
				scope, e := inventoryScope(&s.store.c)
				if e != nil {
					t.Fatal(e)
				}
				if kind == "all-excluded" && (scope.Entries != 1 || scope.ExcludedFiles != 1 || scope.IncludedFiles != 0) {
					t.Fatal(scope)
				}
				if kind == "empty" && (scope.Entries != 0 || scope.ExcludedFiles != 0) {
					t.Fatal(scope)
				}
				if kind == "near-case" && scope.IncludedFiles != 3 {
					t.Fatal(scope)
				}
			} else if kind == "existing" || kind == "late" {
				b, _ := os.ReadFile(dst)
				if string(b) != kind {
					t.Fatal("collision changed", string(b))
				}
			} else if _, e := os.Stat(dst); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("failed output exists")
			}
		})
	}
}

func TestGUIInventoryScopeValidation(t *testing.T) {
	src, dst := inventoryFixture(t)
	if _, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(dst)
	for _, kind := range []string{"unknown-action", "unknown-version", "missing-count", "unknown-field", "duplicate", "null", "wrong-count", "not-complete", "extra-audit", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			var c catalog
			if e := json.Unmarshal(raw, &c); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "unknown-action":
				c.Audit[0].Action = "other"
			case "unknown-version":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"version":1`, `"version":2`, 1)
			case "missing-count":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"excludedFiles":0,`, "", 1)
			case "unknown-field":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"complete":true`, `"unknown":true`, 1)
			case "duplicate":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"version":1`, `"version":1,"version":1`, 1)
			case "null":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"excludedFiles":0`, `"excludedFiles":null`, 1)
			case "wrong-count":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"includedFiles":5`, `"includedFiles":4`, 1)
			case "not-complete":
				c.Audit[0].Detail = strings.Replace(c.Audit[0].Detail, `"complete":true`, `"complete":false`, 1)
			case "extra-audit":
				c.Audit = append(c.Audit, c.Audit[0])
			case "legacy":
				c.Audit = nil
			}
			changed, _ := json.Marshal(c)
			s, e := loadGUICatalog("test", func(string) ([]byte, error) { return changed, nil })
			if kind == "legacy" {
				if e != nil {
					t.Fatal(e)
				}
				if s.projection()["inventoryScope"].(*guiInventoryScope) != nil {
					t.Fatal("legacy scope fabricated")
				}
			} else if e == nil || s != nil {
				t.Fatal("malformed scope adopted", kind)
			}
		})
	}
}
