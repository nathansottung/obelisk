//go:build !guionly

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// An escrow bundle is never written into, beside-through-a-link-into or around a
// registered source: the folder is decided by where it really is.
func TestEscrowBundle_RefusesSourcesByRealLocation(t *testing.T) {
	a := escrowApp(t)
	seedEscrowCache(t, a, struct{ binaries, toolchain, readers bool }{binaries: true})
	parent := t.TempDir()
	src := filepath.Join(parent, "photos")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	a.Store.AddFolder(a.Store.AddCollection("Live").ID, src)
	link := filepath.Join(t.TempDir(), "alias")
	mustDirLink(t, link, src)
	plan := a.planEscrow(EscrowBinariesOnly, false, Census{}, mustConfig(t, a))

	for name, dir := range map[string]string{"a link to the source": link, "the source's parent": parent, "a folder inside the source": filepath.Join(src, "kit")} {
		if _, err := a.WriteEscrowBundle(dir, plan, nil); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("bundle into %s: got %v, want a refusal naming the source", name, err)
		}
	}
	if got := treeOf(t, parent); !equalStrings(got, []string{"photos", "photos/orig.nef"}) {
		t.Errorf("a refused bundle wrote files: %v", got)
	}
}

// The escrow cache is written by a fetch, so a cache folder that is, is inside or
// holds a source is refused before anything is fetched or created, and saving it
// in settings is refused the same way.
func TestEscrowCache_RefusesSources(t *testing.T) {
	a := escrowApp(t)
	parent := t.TempDir()
	src := filepath.Join(parent, "photos")
	writeTree(t, src, map[string]string{"orig.nef": "ORIG"})
	a.Store.AddFolder(a.Store.AddCollection("Live").ID, src)

	for _, cache := range []string{parent, src, filepath.Join(src, "cache")} {
		if _, err := a.SaveConfig(map[string]any{"escrow_cache_dir": cache}); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("saving escrow_cache_dir %s: got %v, want a refusal naming the source", cache, err)
		}
		// Put the setting in place directly to test the fetch's own check.
		cfg := mustConfig(t, a)
		cfg.EscrowCacheDir = cache
		if err := writeConfigForTest(a, cfg); err != nil {
			t.Fatal(err)
		}
		if _, err := a.FetchEscrowCache(false, Census{}, nil); err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("fetch into cache %s: got %v, want a refusal naming the source", cache, err)
		}
	}
	if got := treeOf(t, parent); !equalStrings(got, []string{"photos", "photos/orig.nef"}) {
		t.Errorf("a refused fetch created files: %v", got)
	}
}

// writeConfigForTest publishes cfg as the configuration without SaveConfig's
// checks, standing in for a configuration that arrived some other way.
func writeConfigForTest(a *App, cfg Config) error {
	b, err := encodeConfig(cfg, map[string]json.RawMessage{})
	if err != nil {
		return err
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	_, err = a.publishConfig(b, false)
	return err
}
