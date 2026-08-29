package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// ftByName finds an emitted folder-tree row by display name.
func ftByName(res FolderTreeResult, name string) *FolderTreeNode {
	for i := range res.Children {
		if res.Children[i].Name == name {
			return &res.Children[i]
		}
	}
	return nil
}

// TestFolderTreeLevels checks the Archives folder tree contract: immediate child
// FOLDERS only, name-sorted (file-manager order), with size/file rollups and a
// worst-status roll-up — plus the pagination window that bounds a wide level.
func TestFolderTreeLevels(t *testing.T) {
	a := tmApp(t)
	coll := a.Store.AddCollection("Photos")
	fo := a.Store.AddFolder(coll.ID, "/src")
	add := func(rel string, size int64) {
		a.Store.UpsertFile(File{CollectionID: coll.ID, FolderID: fo.ID, RelPath: rel, SizeBytes: size, HashAlg: "SHA256", Hash: rel})
	}
	add("trip/a.nef", 100)
	add("trip/b.nef", 50)
	add("trip/raw/c.nef", 25)
	add("docs/readme.txt", 5)

	// Archive root → the scanned folder is the sole child folder.
	root := a.Store.FolderTree(coll.ID, "", 0, 0)
	if len(root.Children) != 1 || root.Children[0].Name != "src" {
		t.Fatalf("root should show the scanned folder 'src', got %+v", root.Children)
	}
	src := root.Children[0]
	if src.Files != 4 || src.Size != 180 || !src.HasChildren {
		t.Fatalf("root child 'src' rollups wrong: %+v", src)
	}

	// Zoom into /src → child FOLDERS docs, trip (name-sorted); no files listed.
	lvl := a.Store.FolderTree(coll.ID, src.Path, 0, 0)
	if len(lvl.Children) != 2 {
		t.Fatalf("expected 2 child folders under /src, got %d: %+v", len(lvl.Children), lvl.Children)
	}
	if lvl.Children[0].Name != "docs" || lvl.Children[1].Name != "trip" {
		t.Fatalf("child folders should be name-sorted [docs, trip], got %s,%s", lvl.Children[0].Name, lvl.Children[1].Name)
	}
	trip := ftByName(lvl, "trip")
	if trip == nil || trip.Files != 3 || trip.Size != 175 || !trip.HasChildren {
		t.Fatalf("trip rollups wrong: %+v", trip)
	}

	// Zoom into /src/trip → only the 'raw' subfolder is a row (a.nef/b.nef are files).
	deep := a.Store.FolderTree(coll.ID, trip.Path, 0, 0)
	if len(deep.Children) != 1 || deep.Children[0].Name != "raw" {
		t.Fatalf("under trip, only 'raw' should be a folder row, got %+v", deep.Children)
	}
	if deep.Children[0].HasChildren {
		t.Fatalf("'raw' holds only files; it should not advertise child folders unless present: %+v", deep.Children[0])
	}
}

// TestFolderTreePaginationWindow checks that a wide level is capped and paged.
func TestFolderTreePaginationWindow(t *testing.T) {
	a := tmApp(t)
	coll := a.Store.AddCollection("Wide")
	fo := a.Store.AddFolder(coll.ID, "/w")
	const n = 450
	for i := 0; i < n; i++ {
		a.Store.UpsertFile(File{CollectionID: coll.ID, FolderID: fo.ID,
			RelPath: fmt.Sprintf("wide/sub-%05d/f.nef", i), SizeBytes: 10, HashAlg: "SHA256", Hash: fmt.Sprintf("h%05d", i)})
	}
	first := a.Store.FolderTree(coll.ID, "/w/wide", 0, 200)
	if first.Total != n || len(first.Children) != 200 || !first.Truncated {
		t.Fatalf("first window wrong: total=%d shown=%d truncated=%v", first.Total, len(first.Children), first.Truncated)
	}
	// The default/max cap must hold even when a bigger limit is requested.
	capped := a.Store.FolderTree(coll.ID, "/w/wide", 0, 100000)
	if len(capped.Children) != folderTreeMaxLimit {
		t.Fatalf("limit should be capped at %d, got %d", folderTreeMaxLimit, len(capped.Children))
	}
	// Next page continues from the offset and is name-sorted contiguous.
	next := a.Store.FolderTree(coll.ID, "/w/wide", 200, 200)
	if next.Offset != 200 || len(next.Children) != 200 || !next.Truncated {
		t.Fatalf("second window wrong: offset=%d shown=%d truncated=%v", next.Offset, len(next.Children), next.Truncated)
	}
	last := a.Store.FolderTree(coll.ID, "/w/wide", 400, 200)
	if len(last.Children) != n-400 || last.Truncated {
		t.Fatalf("final window wrong: shown=%d truncated=%v", len(last.Children), last.Truncated)
	}
	if first.Children[0].Name != "sub-00000" || next.Children[0].Name != "sub-00200" {
		t.Fatalf("windows not name-sorted contiguous: %q then %q", first.Children[0].Name, next.Children[0].Name)
	}
}

// TestTreeExpansionBudget is the navigation performance budget for expanding a level
// of the Archives folder tree. It builds a ~200k-file catalog whose WIDEST folder
// holds thousands of immediate subfolders — the worst case for payload discipline —
// and asserts that expanding that level stays within budget:
//
//   - server compute  < 100ms   (one-level aggregate over the whole catalog)
//   - JSON payload     < 50KB    (the virtualization cap makes this hold at any width)
//
// Render time (client JS) can't be measured from Go; the 50KB payload cap is its
// enforcement proxy — under 50KB of virtualized rows, render time is arithmetic
// (see docs/CONTRIBUTING.md, the folder-tree note). Env-gated for a quick local
// `go test`, but wired into CI (see .github/workflows/ci.yml) so it runs on every push.
func TestTreeExpansionBudget(t *testing.T) {
	if os.Getenv("OBELISK_PERF") == "" {
		t.Skip("set OBELISK_PERF=1 to run the tree-expansion budget test")
	}
	const (
		serverBudget  = 100 * time.Millisecond
		payloadBudget = 50 * 1024
	)
	nFiles := envInt("OBELISK_FILES", 200_000)
	wide := envInt("OBELISK_WIDE", 5_000) // immediate subfolders under the widest folder

	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	coll := &Collection{ID: 1, Name: "Scale", CreatedAt: time.Now().UTC()}
	folder := &Folder{ID: 1, CollectionID: 1, Path: "/srv/scale"}
	st.c.Collections = []*Collection{coll}
	st.c.Folders = []*Folder{folder}
	st.c.Files = make([]*File, 0, nFiles)
	// The WIDE folder: `wide` immediate subfolders, one file each — this is the level
	// whose expansion is measured.
	for i := 0; i < wide && i < nFiles; i++ {
		st.c.Files = append(st.c.Files, &File{
			ID: len(st.c.Files) + 1, CollectionID: 1, FolderID: 1,
			RelPath:   fmt.Sprintf("wide/sub-%06d/photo-%06d.nef", i, i),
			SizeBytes: 20_000_000 + int64(i), HashAlg: "SHA256", Hash: fakeHash(i),
		})
	}
	// Filler to reach nFiles, spread across a deep-but-normal bulk tree.
	for j := len(st.c.Files); j < nFiles; j++ {
		st.c.Files = append(st.c.Files, &File{
			ID: j + 1, CollectionID: 1, FolderID: 1,
			RelPath:   fmt.Sprintf("bulk/dir%04d/file%08d.nef", j%1000, j),
			SizeBytes: 20_000_000 + int64(j%1000), HashAlg: "SHA256", Hash: fakeHash(j),
		})
	}
	st.c.NextID = map[string]int{"file": len(st.c.Files), "collection": 1, "folder": 1}

	// Warm once (fault in pages), then take the best of a few runs to shed GC noise —
	// the budget is about the algorithm, not scheduler jitter.
	widest := "/srv/scale/wide"
	_ = st.FolderTree(1, widest, 0, 0)
	best := time.Hour
	var res FolderTreeResult
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		res = st.FolderTree(1, widest, 0, 0)
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	if res.Total != wide {
		t.Fatalf("expected %d subfolders at the widest level, got %d", wide, res.Total)
	}
	if !res.Truncated || len(res.Children) != folderTreeMaxLimit {
		t.Fatalf("wide level should be capped/truncated: shown=%d truncated=%v", len(res.Children), res.Truncated)
	}
	payload, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("tree expansion @ %d files, widest folder = %d subfolders: server %v (best of 3), payload %d bytes",
		nFiles, wide, best, len(payload))
	if best > serverBudget {
		t.Fatalf("server budget exceeded: %v > %v", best, serverBudget)
	}
	if len(payload) > payloadBudget {
		t.Fatalf("payload budget exceeded: %d bytes > %d bytes", len(payload), payloadBudget)
	}
}
