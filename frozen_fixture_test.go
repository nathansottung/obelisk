//go:build !guionly

package main

// Real catalogs of older formats, written by the code of that time (see
// testdata/frozen/README.md). They are never edited or rebuilt, and no test fakes an
// old format by stripping fields from a new one.

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	frozenRoot      = "testdata"
	frozenChecksums = "frozen/CHECKSUMS.sha256"
	frozenWroteAt   = `C:\obelisk-fixture\`
)

// frozenProblems compares the files under root/frozen, and any other file the list
// names, with the checksum list.
func frozenProblems(root string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(frozenChecksums)))
	if err != nil {
		return nil, err
	}
	want := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		sum, name, ok := strings.Cut(line, "  ")
		if !ok || len(sum) != 64 {
			return nil, fmt.Errorf("bad checksum line %q", line)
		}
		want[name] = sum
	}
	var problems []string
	for name, sum := range want {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			problems = append(problems, name+": missing")
			continue
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != sum {
			problems = append(problems, name+": bytes changed")
		}
	}
	err = filepath.WalkDir(filepath.Join(root, "frozen"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := want[rel]; !ok && rel != frozenChecksums && rel != "frozen/README.md" {
			problems = append(problems, rel+": not in the checksum list")
		}
		return nil
	})
	sort.Strings(problems)
	return problems, err
}

func TestFrozenFixtures_BytesUnchanged(t *testing.T) {
	problems, err := frozenProblems(frozenRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("frozen fixtures are never edited or rebuilt; a schema change adds a new one made by the old code:\n%s",
			strings.Join(problems, "\n"))
	}
}

// The check itself catches a changed byte, a lost file and an added one.
func TestFrozenFixtures_ChecksumCatchesEachChange(t *testing.T) {
	name := "frozen/catalog-v8-original.json"
	for what, change := range map[string]func(string) error{
		"bytes changed": func(root string) error {
			p := filepath.Join(root, filepath.FromSlash(name))
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b[len(b)/2] ^= 1
			return os.WriteFile(p, b, 0o644)
		},
		"missing": func(root string) error { return os.Remove(filepath.Join(root, filepath.FromSlash(name))) },
		"not in the checksum list": func(root string) error {
			return os.WriteFile(filepath.Join(root, "frozen", "added.json"), []byte("{}"), 0o644)
		},
	} {
		t.Run(what, func(t *testing.T) {
			root := t.TempDir()
			copyTree(t, filepath.Join(frozenRoot, "frozen"), filepath.Join(root, "frozen"))
			if err := copyListedOutside(root); err != nil {
				t.Fatal(err)
			}
			if err := change(root); err != nil {
				t.Fatal(err)
			}
			problems, err := frozenProblems(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 1 || !strings.Contains(problems[0], what) {
				t.Fatalf("problems = %v", problems)
			}
		})
	}
}

// copyListedOutside copies the listed fixtures that live beside frozen/.
func copyListedOutside(root string) error {
	raw, err := os.ReadFile(filepath.Join(frozenRoot, filepath.FromSlash(frozenChecksums)))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		_, name, _ := strings.Cut(line, "  ")
		if strings.HasPrefix(name, "frozen/") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(frozenRoot, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// frozenValues is what a package recorded in the old catalog, read before it opens.
type frozenValues struct {
	KeyRef   string            `json:"key_ref"`
	EncHash  string            `json:"enc_hash"`
	TarHash  string            `json:"tar_hash"`
	Sidecars map[string]string `json:"sidecars"`
	Name     string            `json:"name"`
}

func frozenJSONValues(t *testing.T, path string) map[string]frozenValues {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("payload_hash")) {
		t.Fatal("this fixture is not in the old format: it holds per-copy values")
	}
	var doc struct {
		SchemaVersion int            `json:"schema_version"`
		Chunks        []frozenValues `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != 8 {
		t.Fatalf("fixture schema %d, want 8", doc.SchemaVersion)
	}
	out := map[string]frozenValues{}
	for _, c := range doc.Chunks {
		out[c.Name] = c
	}
	return out
}

func frozenDBValues(t *testing.T, path string) map[string]frozenValues {
	t.Helper()
	db, err := sql.Open(catalogDBDriver, catalogDBReadOnlyDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version string
	var columns int
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('copy') WHERE name='key_ref'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if version != "1" || columns != 0 {
		t.Fatalf("this fixture is not in the old format: schema %s, %d per-copy columns", version, columns)
	}
	rows, err := db.Query(`SELECT name, key_ref, enc_hash, tar_hash, sidecars FROM chunk`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]frozenValues{}
	for rows.Next() {
		var v frozenValues
		var sidecars sql.NullString
		if err := rows.Scan(&v.Name, &v.KeyRef, &v.EncHash, &v.TarHash, &sidecars); err != nil {
			t.Fatal(err)
		}
		if err := jget(sidecars, &v.Sidecars); err != nil {
			t.Fatal(err)
		}
		out[v.Name] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// frozenHome is a private working copy of one frozen package fixture.
type frozenHome struct {
	root, data string
	want       map[string]frozenValues
	restored   string
}

func newFrozenHome(t *testing.T, fixture string) *frozenHome {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join(frozenRoot, "frozen", fixture), root)
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	h := &frozenHome{root: root, data: data}
	for _, name := range []string{"catalog.json", catalogDBName} {
		from := filepath.Join(root, name)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		to := filepath.Join(data, name)
		if err := os.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		if name == catalogDBName {
			h.want = frozenDBValues(t, to)
		} else {
			h.want = frozenJSONValues(t, to)
		}
	}
	if len(h.want) != 2 {
		t.Fatalf("fixture holds %d packages, want 2", len(h.want))
	}
	b, err := os.ReadFile(filepath.Join(root, "restored.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	h.restored = strings.TrimSpace(string(b))
	return h
}

// here moves a path recorded when the fixture was written into this working copy.
func (h *frozenHome) here(p string) string {
	if !strings.HasPrefix(p, frozenWroteAt) {
		return p
	}
	parts := strings.Split(strings.TrimPrefix(p, frozenWroteAt), `\`)
	return filepath.Join(append([]string{h.root}, parts[1:]...)...)
}

// open opens the catalog and points its recorded paths at this working copy. The
// fixture's staging folder is not kept, so packages have no staged build.
func (h *frozenHome) open(t *testing.T, tools map[string]string) *App {
	t.Helper()
	store, err := openTestStore(t, h.data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	app := initializedTestApp(t, &App{DataDir: h.data, Store: store})
	ks1, ks2 := filepath.Join(t.TempDir(), "keystore1.json"), filepath.Join(t.TempDir(), "keystore2.json")
	key, err := os.ReadFile(filepath.Join(h.root, "keystore.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{ks1, ks2} {
		if err := os.WriteFile(p, key, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.SaveConfig(map[string]any{"staging_dir": t.TempDir(), "keystore_paths": []string{ks1, ks2},
		"par2_redundancy": 5, "tools": tools, "private_media": true}); err != nil {
		t.Fatal(err)
	}
	h.relocate(app)
	return app
}

func (h *frozenHome) relocate(app *App) {
	for _, c := range app.Store.Chunks(0) {
		c.SrcRoot, c.WrittenDest, c.StagedDir = h.here(c.SrcRoot), h.here(c.WrittenDest), ""
		for i := range c.Copies {
			c.Copies[i].Path = h.here(c.Copies[i].Path)
			for j := range c.Copies[i].Segments {
				c.Copies[i].Segments[j].Dest = h.here(c.Copies[i].Segments[j].Dest)
			}
		}
		for j := range c.Segments {
			c.Segments[j].Dest = h.here(c.Segments[j].Dest)
		}
		app.Store.UpdateChunk(c)
	}
}

func frozenChunk(t *testing.T, app *App, name string) *Chunk {
	t.Helper()
	for _, c := range app.Store.Chunks(0) {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("%s is missing after the upgrade", name)
	return nil
}

// checkInherited requires every copy to hold what its package recorded before.
func (h *frozenHome) checkInherited(t *testing.T, app *App) {
	t.Helper()
	for name, want := range h.want {
		c := frozenChunk(t, app, name)
		if len(c.Copies) != 1 {
			t.Fatalf("%s has %d copies, want 1", name, len(c.Copies))
		}
		cp := c.Copies[0]
		if want.KeyRef == "" || cp.KeyRef != want.KeyRef || cp.PayloadHash != want.EncHash || cp.TarHash != want.TarHash ||
			len(want.Sidecars) == 0 || !reflect.DeepEqual(cp.Sidecars, want.Sidecars) {
			t.Fatalf("%s's copy did not inherit its package's values:\n want %+v\n got  %+v", name, want, cp)
		}
		if c.Spanned != (len(cp.Segments) > 0) {
			t.Fatalf("%s: spanned=%v with %d segments on its copy", name, c.Spanned, len(cp.Segments))
		}
	}
}

// checkOldCopies verifies the old single-medium copy and restores both old copies.
func (h *frozenHome) checkOldCopies(t *testing.T, app *App) {
	t.Helper()
	single, span := frozenChunk(t, app, "FROZEN-SINGLE"), frozenChunk(t, app, "FROZEN-SPANNED")
	medium := filepath.Join(h.root, "medium")
	r, err := app.VerifyChunk(single.ID, medium)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if ok, _ := r["verify_ok"].(bool); !ok || r["credited"] != 1 {
		t.Fatalf("the old copy did not verify: %v", r)
	}
	restore := func(id int, from string) {
		t.Helper()
		out := t.TempDir()
		if _, err := app.RestoreChunk(id, from, out, nil, noProg); err != nil {
			t.Fatalf("restore from %s: %v", from, err)
		}
		b, err := os.ReadFile(filepath.Join(out, "fixture-file.bin"))
		if err != nil {
			t.Fatal(err)
		}
		if sum := sha256.Sum256(b); hex.EncodeToString(sum[:]) != h.restored {
			t.Fatal("the restored file differs from the original")
		}
	}
	restore(single.ID, filepath.Join(medium, single.Name))
	pool := t.TempDir()
	tapes, _ := filepath.Glob(filepath.Join(h.root, "tapes", "tape*", span.Name))
	if len(tapes) < 2 {
		t.Fatalf("%d tapes in the fixture", len(tapes))
	}
	for _, tape := range tapes {
		copyFolderFiles(t, tape, pool, all)
	}
	restore(span.ID, pool)
}

// Each frozen catalog opens, migrates and keeps working on the engine it would meet,
// including catalog.json moved into the database on open.
func TestFrozenCatalogs_MigrateAndOldCopiesKeepWorking(t *testing.T) {
	tools := nativeTools(t)
	for _, tc := range []struct{ name, fixture, engine string }{
		{"v8 JSON on the JSON engine", "catalog-v8-packages", "json"},
		{"v8 JSON moved to the database on open", "catalog-v8-packages", "sqlite"},
		{"v1 database on the database engine", "catalog-v1-db", "sqlite"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, tc.engine)
			h := newFrozenHome(t, tc.fixture)
			app := h.open(t, tools)
			h.checkInherited(t, app)
			h.checkOldCopies(t, app)

			// A rebuild gives each package new values and leaves the old copy working.
			// An old catalog's keys were made one per package; a rebuild needs a
			// key that is ready, as the user makes on the Keys page.
			readyTestKey(t, app)
			for name, want := range h.want {
				c := frozenChunk(t, app, name)
				c.Status = "FAILED"
				app.Store.UpdateChunk(c)
				if err := app.BuildChunk(c.ID, noProg); err != nil {
					t.Fatalf("rebuild %s: %v", name, err)
				}
				c = frozenChunk(t, app, name)
				if c.KeyRef == want.KeyRef || c.Copies[0].KeyRef != want.KeyRef || c.Copies[0].PayloadHash != want.EncHash {
					t.Fatalf("%s: the rebuild did not leave the old copy on its own values", name)
				}
			}
			h.checkOldCopies(t, app)

			if err := app.Store.Close(); err != nil {
				t.Fatal(err)
			}
			store, err := openTestStore(t, h.data)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			app.Store = store
			for name, want := range h.want {
				if cp := frozenChunk(t, app, name).Copies[0]; cp.KeyRef != want.KeyRef || cp.PayloadHash != want.EncHash {
					t.Fatalf("%s: the old copy's values were lost in a reopen", name)
				}
			}
			h.checkOldCopies(t, app)
		})
	}
}

// The original schema 8 fixture migrates on the JSON engine, and every copy inherits.
func TestFrozenCatalogs_OriginalV8Migrates(t *testing.T) {
	t.Setenv(catalogEngineEnv, "json")
	dir := t.TempDir()
	path := filepath.Join(frozenRoot, "frozen", "catalog-v8-original.json")
	want := frozenJSONValues(t, path)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := openTestStore(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	copies := 0
	for _, c := range s.Chunks(0) {
		w := want[c.Name]
		for _, cp := range c.Copies {
			copies++
			if cp.KeyRef != w.KeyRef || cp.PayloadHash != w.EncHash || cp.TarHash != w.TarHash {
				t.Fatalf("%s's copy did not inherit: %+v", c.Name, cp)
			}
		}
	}
	if copies == 0 {
		t.Fatal("the fixture holds no copies")
	}
}

// The database upgrade keeps a checked backup of the old file.
func TestFrozenCatalogs_DatabaseUpgradeKeepsABackup(t *testing.T) {
	t.Setenv(catalogEngineEnv, "sqlite")
	h := newFrozenHome(t, "catalog-v1-db")
	before, err := os.ReadFile(filepath.Join(h.data, catalogDBName))
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 {
		t.Fatal("empty fixture")
	}
	s, err := openTestStore(t, h.data)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	kept, _ := filepath.Glob(filepath.Join(h.data, catalogDBName+".pre-schema-v1-*"))
	if len(kept) != 1 {
		t.Fatalf("backups kept: %v", kept)
	}
	if got := frozenDBValues(t, kept[0]); !reflect.DeepEqual(got, h.want) {
		t.Fatal("the backup does not hold the old records")
	}
}

// An interrupted upgrade changes nothing, and the next open finishes it.
func TestFrozenCatalogs_InterruptedDatabaseUpgradeRestartsCleanly(t *testing.T) {
	tools := nativeTools(t)
	t.Setenv(catalogEngineEnv, "sqlite")
	h := newFrozenHome(t, "catalog-v1-db")
	path := filepath.Join(h.data, catalogDBName)
	prev := catalogDBMigrateHook
	catalogDBMigrateHook = func() error { return errors.New("interrupted (injected)") }
	t.Cleanup(func() { catalogDBMigrateHook = prev })
	if s, err := openTestStore(t, h.data); err == nil {
		s.Close()
		t.Fatal("the interrupted upgrade reported success")
	}
	catalogDBMigrateHook = prev
	if got := frozenDBValues(t, path); !reflect.DeepEqual(got, h.want) { // also proves schema 1 and no new columns
		t.Fatal("the interrupted upgrade changed the records")
	}
	app := h.open(t, tools)
	h.checkInherited(t, app)
	h.checkOldCopies(t, app)
}

// The JSON engine does not silently start over beside a database it cannot use.
func TestFrozenCatalogs_DatabaseOnTheJSONEngine(t *testing.T) {
	t.Setenv(catalogEngineEnv, "json")
	h := newFrozenHome(t, "catalog-v1-db")
	path := filepath.Join(h.data, catalogDBName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(h.data)
	if s != nil {
		chunks := len(s.Chunks(0))
		s.Close()
		t.Logf("the JSON engine opened beside the database with %d packages", chunks)
	} else {
		t.Logf("the JSON engine refused: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the JSON engine changed the database")
	}
}
