package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func comparisonFixture(label string) catalog {
	c := guiFixture(label)
	c.Folders = c.Folders[:1]
	c.Files = nil
	c.Chunks = nil
	return c
}

func TestGUIComparisonEnumeration(t *testing.T) {
	c := comparisonFixture("enumeration")
	c.Collections[0].Retired = true
	for i := 1; i <= 3; i++ {
		c.Files = append(c.Files, &File{ID: i, CollectionID: 1, FolderID: 1, RelPath: strconv.Itoa(i), FirstSeen: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
	}
	c.NextID["file"] = 3
	file := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(file, guiBytes(t, c), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runGUICatalog([]string{file}, strings.NewReader("{\"enumerate\":true}\n{\"enumerate\":true,\"text\":\"x\"}\n"), &out); err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatal(out.String())
	}
	var got struct {
		OK       bool     `json:"ok"`
		Complete bool     `json:"complete"`
		IDs      []string `json:"ids"`
		Count    int      `json:"count"`
		Digest   string   `json:"digest"`
	}
	if err := json.Unmarshal(lines[1], &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || !got.Complete || got.Count != 3 || strings.Join(got.IDs, ",") != "1,2,3" || len(got.Digest) != 64 {
		t.Fatal(string(lines[1]))
	}
	if !bytes.Contains(lines[2], []byte(`"ok":false`)) {
		t.Fatal(string(lines[2]))
	}
	s, err := loadGUICatalog(file, readGUICatalogFile)
	if err != nil {
		t.Fatal(err)
	}
	if s.projection()["comparisonFrame"] == nil {
		t.Fatal("single root frame missing")
	}
	s.store.c.Folders = append(s.store.c.Folders, &Folder{ID: 2, CollectionID: 1, Path: "other"})
	if s.projection()["comparisonFrame"] != nil {
		t.Fatal("ambiguous frame inferred")
	}
}

// Synthetic edge-case catalogs; control names and large IDs are not filesystem
// observations. Native producer fixtures are generated separately by TestGUIMultiSnapshotFixtures.
func TestGUIComparisonFixtures(t *testing.T) {
	dir := os.Getenv("OBELISK_GUI_COMPARISON_FIXTURES")
	if dir == "" {
		t.Skip("explicit comparison fixture output required")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute fixture directory required")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if strconv.IntSize != 64 {
		t.Skip("large-ID fixtures require retained amd64 toolchain")
	}
	write := func(name string, c catalog) {
		t.Helper()
		raw := guiBytes(t, c)
		if _, err := loadGUICatalog("synthetic", func(string) ([]byte, error) { return raw, nil }); err != nil {
			t.Fatal(name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	a, b := comparisonFixture("reference"), comparisonFixture("counterpart")
	add := func(c *catalog, id int, name string, size int64, hash string) {
		c.Files = append(c.Files, &File{ID: id, CollectionID: 1, FolderID: 1, RelPath: name, SizeBytes: size, HashAlg: "sha256", Hash: hash, FirstSeen: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)})
		c.NextID["file"] = id
	}
	h := func(s string) string { return strings.Repeat(s, 64) }
	paths := []string{"agreement", "different", "size", "unknown", "contradiction", "line\nname", "line\rname", "line\r\nname", `line\nname`, "<img src=x onerror=alert(1)>", "long/" + strings.Repeat("n", 130)}
	for i, p := range paths {
		ah, bh, as, bs := h("a"), h("a"), int64(4), int64(4)
		switch p {
		case "different":
			bh = h("b")
		case "size":
			bh = h("b")
			bs = 8
		case "unknown":
			ah = ""
			bh = ""
		case "contradiction":
			bs = 5
		}
		add(&a, i+1, p, as, ah)
		add(&b, i+1, p, bs, bh)
	}
	for i, p := range []string{"only-reference", "dir-one/same.txt", "Case", "é"} {
		add(&a, 30+i, p, 4, h("a"))
	}
	for i, p := range []string{"only-counterpart", "dir-two/same.txt", "case", "e\u0301"} {
		add(&b, 30+i, p, 4, h("a"))
	}
	large, _ := strconv.ParseInt("9007199254740993", 10, 64)
	add(&a, int(large), "large", large-1, "")
	add(&b, int(large+1), "large", large, "")
	write("a", a)
	write("b", b)
	duplicate := a
	duplicate.Files = append([]*File{}, a.Files...)
	copyFile := *a.Files[0]
	copyFile.ID = 100
	duplicate.Files = append(duplicate.Files, &copyFile)
	write("duplicate", duplicate)
	bad := comparisonFixture("bad")
	add(&bad, 1, "../outside", 1, h("a"))
	write("outside", bad)
	write("multi-root", guiFixture("multi-root"))
	write("empty", comparisonFixture("empty"))
	retired := a
	retired.Collections = []*Collection{{ID: 1, Name: "retired", Retired: true}}
	write("retired", retired)
}
