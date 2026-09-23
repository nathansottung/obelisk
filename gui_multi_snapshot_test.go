package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestGUIMultiSnapshotRecordedTime(t *testing.T) {
	legacy := guiBytes(t, guiFixture("legacy"))
	s, err := loadGUICatalog("memory", func(string) ([]byte, error) { return legacy, nil })
	if err != nil {
		t.Fatal(err)
	}
	if s.projection()["recordedAt"] != nil {
		t.Fatal("legacy time must remain unknown")
	}
	for _, tc := range scopeKeyCases(t) {
		if !tc.ok {
			continue
		}
		s, err := loadGUICatalog("memory", func(string) ([]byte, error) { return tc.raw, nil })
		if err != nil {
			t.Fatal(err)
		}
		if len(s.store.c.Audit) != 0 && s.projection()["recordedAt"] != s.store.c.Audit[0].At {
			t.Fatal("recorded time was replaced by load time")
		}
	}
}

// Explicit setup creates ONLY new synthetic inputs beneath the task directory.
// Derived age/name/large-ID catalogs are fixtures, not claims of native filenames.
func TestGUIMultiSnapshotFixtures(t *testing.T) {
	dir := os.Getenv("OBELISK_GUI_MULTI_FIXTURES")
	if dir == "" {
		t.Skip("explicit multi fixture output required")
	}
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute fixture output required")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	sources := dir + "-sources"
	if err := os.Mkdir(sources, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, label := range []string{"a", "b", "zero", "empty", "excluded"} {
		src := filepath.Join(sources, "source-"+label)
		if err := os.Mkdir(src, 0700); err != nil {
			t.Fatal(err)
		}
		if label == "a" || label == "b" || label == "zero" {
			for name, body := range map[string]string{"shared.txt": label + " different evidence", "café & +%# note.txt": "same bytes"} {
				if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		if label == "a" || label == "b" || label == "excluded" {
			if err := os.WriteFile(filepath.Join(src, ".DS_Store"), []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		out := filepath.Join(dir, label+".json")
		if _, err := produceGUIInventory(context.Background(), src, out, disposableInventoryLimits, inventoryIO{ignoreDSStore: label != "a"}, io.Discard); err != nil {
			t.Fatal(err)
		}
		if label == "a" {
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			write("a-copy", raw)
			var c catalog
			if err = json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			c.Audit = nil
			write("unknown", guiBytes(t, c))
		} else if label == "b" {
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var c catalog
			if err = json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			at, err := time.Parse(time.RFC3339, "2024-02-03T04:05:06-05:00")
			if err != nil {
				t.Fatal(err)
			}
			c.Audit[0].At = at
			c.Collections[0].CreatedAt = at
			write("b-aged", guiBytes(t, c))
		}
	}
	c := guiExactFixture(t)
	write("large-a", guiBytes(t, c))
	c.Collections[0].Name = "Other <script>inert</script>"
	c.Files[0].SizeBytes = 42
	write("large-b", guiBytes(t, c))
	c = guiFixture("controls")
	c.Chunks = nil
	c.Files = nil
	for i, name := range []string{"line\nname", "line\rname", "line\r\nname", `line\nname`, " leading ", "<img src=x onerror=alert(1)>"} {
		f := *guiFixture("controls").Files[0]
		f.ID = i + 1
		f.RelPath = name
		c.Files = append(c.Files, &f)
	}
	c.NextID["file"] = len(c.Files)
	write("controls", guiBytes(t, c))
	for _, name := range []string{"limit-a", "limit-b"} {
		c := guiFixture(name)
		c.Chunks = nil
		c.Files = nil
		for i := 1; i <= 501; i++ {
			f := *guiFixture(name).Files[0]
			f.ID = i
			c.Files = append(c.Files, &f)
		}
		c.NextID["file"] = 501
		write(name, guiBytes(t, c))
	}
	var cases []map[string]any
	for i, tc := range scopeKeyCases(t) {
		name := "scope-" + strconv.Itoa(i)
		write(name, tc.raw)
		cases = append(cases, map[string]any{"name": name, "ok": tc.ok})
	}
	b, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "scope-cases.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
