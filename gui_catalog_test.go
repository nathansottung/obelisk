package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func guiFixture(label string) catalog {
	c := catalog{SchemaVersion: currentSchemaVersion, NextID: map[string]int{"collection": 1, "folder": 2, "file": 2, "chunk": 1, "volume": 2}}
	c.Collections = []*Collection{{ID: 1, Name: label + " <img src=x onerror=alert(1)>"}}
	c.Folders = []*Folder{{ID: 1, CollectionID: 1, Path: "/synthetic-never-open/" + label + "/one"}, {ID: 2, CollectionID: 1, Path: "/synthetic-never-open/" + label + "/two"}}
	for i := 1; i <= 2; i++ {
		c.Files = append(c.Files, &File{ID: i, CollectionID: 1, FolderID: i, RelPath: label + "/duplicate.txt", SizeBytes: int64(i * 123), HashAlg: "sha256", Hash: strings.Repeat(string(rune('a'+i-1)), 64), FirstSeen: time.Date(2025, 1, i, 0, 0, 0, 0, time.UTC)})
	}
	c.Volumes = []*Volume{{ID: 1, Label: label + " medium one", Kind: "HDD", Location: "Synthetic shelf A"}, {ID: 2, Label: label + " medium two", Kind: "OPTICAL", Location: "Synthetic shelf B"}}
	c.Chunks = []*Chunk{{ID: 1, CollectionID: 1, Name: label + " package", Status: "WRITTEN", Files: []ChunkFileRef{{FileID: 1, Hash: c.Files[0].Hash, RelPath: c.Files[0].RelPath}}, Copies: []Copy{{VolumeID: 1, Path: "/synthetic-never-open/" + label + "/package.tar"}, {VolumeID: 2, Path: "/synthetic-never-open/" + label + "/second.tar"}}}}
	return c
}
func guiBytes(t *testing.T, c catalog) []byte {
	t.Helper()
	b, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// Explicit setup only. Never invoked by the application or reader.
func TestGUICatalogGenerateFixtures(t *testing.T) {
	dir := os.Getenv("OBELISK_GUI_FIXTURE_OUTPUT")
	if dir == "" {
		t.Skip("explicit fixture setup output not selected")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute setup directory required")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"alpha.json": guiBytes(t, guiFixture("ALPHA")), "beta.json": guiBytes(t, guiFixture("BETA")), "empty.json": guiBytes(t, catalog{SchemaVersion: currentSchemaVersion}), "malformed.json": []byte("{broken"), "zero.json": {}, "future.json": []byte(`{"schema_version":999}`)} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "directory.json"), 0700); err != nil {
		t.Fatal(err)
	}
}

func TestGUICatalogNativeDecodeAndSearch(t *testing.T) {
	for _, label := range []string{"ALPHA", "BETA"} {
		t.Run(label, func(t *testing.T) {
			raw := guiBytes(t, guiFixture(label))
			reads := 0
			s, err := loadGUICatalog("explicit-input", func(name string) ([]byte, error) {
				reads++
				if name != "explicit-input" {
					t.Fatal(name)
				}
				return raw, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writes := 0
			s.store.persistObserver = func(string, string) { writes++ }
			s.store.failSave = func() error { writes++; return errors.New("forbidden") }
			before := guiBytes(t, s.store.c)
			if len(s.store.Search(SearchQuery{Text: label + "/duplicate"})) != 2 || len(s.store.Search(SearchQuery{Hash: strings.Repeat("a", 8)})) != 1 || len(s.store.Search(SearchQuery{Text: "absent"})) != 0 {
				t.Fatal("native query mismatch")
			}
			p := s.projection()
			f := p["files"].([]map[string]any)
			if len(f) != 2 || len(f[0]["copies"].([]map[string]any)) != 2 || len(f[1]["copies"].([]map[string]any)) != 0 {
				t.Fatal("occurrence identity mismatch")
			}
			if reads != 1 || writes != 0 || !bytes.Equal(before, guiBytes(t, s.store.c)) {
				t.Fatal("read/query mutated snapshot")
			}
		})
	}
}

func TestGUICatalogRefusedLoads(t *testing.T) {
	valid := guiFixture("TEST")
	rawValid := guiBytes(t, valid)
	cases := map[string][]byte{"zero": {}, "malformed": []byte("{"), "null": []byte("null"), "old": []byte(`{"schema_version":7}`), "future": []byte(`{"schema_version":9}`), "large": bytes.Repeat([]byte(" "), guiCatalogMaxBytes+1)}
	bad := valid
	cases["missing-size"] = bytes.Replace(rawValid, []byte(`"size_bytes":123,`), nil, 1)
	cases["null-size"] = bytes.Replace(rawValid, []byte(`"size_bytes":123`), []byte(`"size_bytes":null`), 1)
	bad.Files = []*File{nil}
	cases["nil-row"] = guiBytes(t, bad)
	bad = guiFixture("TEST")
	bad.Files[1].ID = 1
	cases["duplicate-id"] = guiBytes(t, bad)
	bad = guiFixture("TEST")
	bad.Files[0].FolderID = 999
	cases["missing-folder"] = guiBytes(t, bad)
	bad = guiFixture("TEST")
	bad.Files[0].Hash = "fake"
	cases["bad-hash"] = guiBytes(t, bad)
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadGUICatalog("only", func(string) ([]byte, error) { return raw, nil }); err == nil {
				t.Fatal("accepted refused input")
			}
		})
	}
	for _, err := range []error{os.ErrNotExist, os.ErrPermission, errors.New("injected IO")} {
		if _, got := loadGUICatalog("only", func(string) ([]byte, error) { return guiBytes(t, valid), err }); got == nil {
			t.Fatal("parsed partial bytes accompanying error")
		}
	}
	if _, err := loadGUICatalog("empty", func(string) ([]byte, error) { return guiBytes(t, catalog{SchemaVersion: 8}), nil }); err != nil {
		t.Fatal(err)
	}
}

func TestGUICatalogReadLifecycle(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "catalog.json")
	raw := guiBytes(t, guiFixture("LIFE"))
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	before, _ := os.ReadDir(dir)
	for i := 0; i < 2; i++ {
		var out bytes.Buffer
		if err := runGUICatalog([]string{p}, strings.NewReader("{\"text\":\"duplicate\"}\n{\"hash\":\"bbbb\"}\n"), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"ids":["1","2"]`) || !strings.Contains(out.String(), `"ids":["2"]`) {
			t.Fatal(out.String())
		}
	}
	for _, name := range []string{filepath.Join(dir, "missing.json"), dir} {
		var out bytes.Buffer
		if runGUICatalog([]string{name}, strings.NewReader(""), &out) == nil {
			t.Fatal("expected refusal")
		}
	}
	after, _ := os.ReadDir(dir)
	names := func(es []os.DirEntry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.Name())
		}
		return out
	}
	got, _ := os.ReadFile(p)
	if !reflect.DeepEqual(names(before), names(after)) || !bytes.Equal(raw, got) {
		t.Fatal("reader changed input directory")
	}
}

func guiExactFixture(t *testing.T) catalog {
	t.Helper()
	values := []string{"1", "2147483646", "2147483647"}
	if strconv.IntSize == 64 {
		values = []string{"1", "9007199254740991", "9007199254740992", "9007199254740993", "9223372036854775807"}
	}
	c := catalog{SchemaVersion: currentSchemaVersion}
	for i, value := range values {
		id, err := strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
		hash := strings.Repeat(string(rune('a'+i)), 64)
		c.Collections = append(c.Collections, &Collection{ID: id, Name: "EXACT collection " + value})
		c.Folders = append(c.Folders, &Folder{ID: id, CollectionID: id, Path: "/synthetic-never-open/EXACT/" + value})
		c.Files = append(c.Files, &File{ID: id, CollectionID: id, FolderID: id, RelPath: "EXACT/duplicate.txt", SizeBytes: int64(id), Hash: hash, HashAlg: "sha256"})
		c.Locations = append(c.Locations, &Location{ID: id, Name: "EXACT shelf " + value})
		c.Volumes = append(c.Volumes, &Volume{ID: id, LocationID: id, Label: "EXACT volume " + value, Kind: "HDD"})
		c.Chunks = append(c.Chunks, &Chunk{ID: id, CollectionID: id, Name: "EXACT chunk " + value, Files: []ChunkFileRef{{FileID: id, Hash: hash}}, Copies: []Copy{{VolumeID: id, Path: "/synthetic-never-open/copy/" + value}}})
	}
	return c
}

func TestGUICatalogExactIntegers(t *testing.T) {
	c := guiExactFixture(t)
	raw := guiBytes(t, c)
	s, err := loadGUICatalog("only", func(string) ([]byte, error) { return raw, nil })
	if err != nil {
		t.Fatal(err)
	}
	p := s.projection()
	if p["idMax"] != strconv.Itoa(int(^uint(0)>>1)) {
		t.Fatal("native ID range changed")
	}
	files := p["files"].([]map[string]any)
	for i, f := range c.Files {
		t.Run(strconv.Itoa(f.ID), func(t *testing.T) {
			id := strconv.Itoa(f.ID)
			if files[i]["id"] != id || files[i]["bytes"] != strconv.FormatInt(f.SizeBytes, 10) {
				t.Fatal("lossy projection")
			}
			if p["collections"].([]map[string]any)[i]["id"] != id || p["volumes"].([]map[string]any)[i]["id"] != id {
				t.Fatal("lossy related identity")
			}
			copy := files[i]["copies"].([]map[string]any)[0]
			if copy["id"] != "chunk-"+id+"-copy-0" || copy["location"] != "EXACT shelf "+id {
				t.Fatal("wrong native join")
			}
			var output bytes.Buffer
			dir := t.TempDir()
			file := filepath.Join(dir, "catalog.json")
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			query := `{"hash":"` + f.Hash + `"}` + "\n"
			if err := runGUICatalog([]string{file}, strings.NewReader(query), &output); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), `"ids":["`+id+`"]`) {
				t.Fatal(output.String())
			}
		})
	}
	for _, invalid := range []int{0, -1} {
		bad := guiExactFixture(t)
		bad.Files[0].ID = invalid
		if _, err := loadGUICatalog("only", func(string) ([]byte, error) { return guiBytes(t, bad), nil }); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
	bad := bytes.Replace(raw, []byte(`"id":1,`), []byte(`"id":9223372036854775808,`), 1)
	if _, err := loadGUICatalog("only", func(string) ([]byte, error) { return bad, nil }); err == nil {
		t.Fatal("out of native range accepted")
	}
}

// Separate, explicit setup. Does not run from the reader/application.
func TestGUICatalogCorrectionFixtures(t *testing.T) {
	dir := os.Getenv("OBELISK_GUI_CORRECTION_FIXTURES")
	if dir == "" {
		t.Skip("explicit correction fixture output not selected")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute setup directory required")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	c := guiExactFixture(t)
	write := func(name string, value []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), value, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("exact.json", guiBytes(t, c))
	for i, j := 0, len(c.Files)-1; i < j; i, j = i+1, j-1 {
		c.Files[i], c.Files[j] = c.Files[j], c.Files[i]
	}
	write("exact-reversed.json", guiBytes(t, c))
	alpha := guiBytes(t, guiFixture("ALPHA"))
	s, err := loadGUICatalog("setup-memory", func(string) ([]byte, error) { return alpha, nil })
	if err != nil {
		t.Fatal(err)
	}
	projection, err := json.Marshal(map[string]any{"ok": true, "catalog": s.projection()})
	if err != nil {
		t.Fatal(err)
	}
	write("projection.json", projection)
	for _, name := range []string{"failed-exit", "shape", "partial", "timeout", "query-failed", "invalid-id", "unknown-id", "duplicate-id", "late", "numeric-startup", "out-of-range", "negative-id", "zero-id", "leading-zero", "exponent-id"} {
		write(name+".json", alpha)
	}
}
