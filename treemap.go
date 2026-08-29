package main

// treemap.go — "WizTree colored by risk." A per-archive treemap whose rectangles
// are sized by bytes and filled by the six protection-status colors (or, when a
// reconcile report exists, by drift state). Everything here is computed from the
// catalog's folder/file sizes — it NEVER walks the disk.
//
// Responsiveness at a million files comes from laziness: the endpoint returns ONE
// level at a time (the immediate children of the zoomed-in directory), with a
// worst-status + byte rollup per child. Children too small to draw as anything
// but a sliver are folded into a single synthetic "other" block rather than
// emitting thousands of rectangles. Zooming in/out is just another one-level
// request, so no whole-tree structure is ever built or held.

import (
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Treemap tuning: cap the rectangles per level and fold anything below a fraction
// of the level's total, so a directory with 50k immediate entries still draws a
// readable handful plus one "other" block.
const (
	treemapMaxChildren = 120
	treemapMinFraction = 0.0015 // 0.15% of the level total
)

// TreemapNode is one rectangle: a child directory or file of the current level.
type TreemapNode struct {
	Name        string           `json:"name"`
	Path        string           `json:"path"` // canonical stored path; echo back to zoom in
	IsDir       bool             `json:"is_dir"`
	Size        int64            `json:"size"`
	Files       int              `json:"files"`
	Status      string           `json:"status"` // worst protection status / drift state in this node
	HasChildren bool             `json:"has_children"`
	Other       bool             `json:"other,omitempty"`        // the folded "everything else" block
	StatusBytes map[string]int64 `json:"status_bytes,omitempty"` // per-status bytes within this node
}

// TreemapCrumb is one hop in the zoom breadcrumb.
type TreemapCrumb struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// TreemapResult is one zoom level: the children to draw plus legend totals.
type TreemapResult struct {
	CollectionID   int              `json:"collection_id"`
	Name           string           `json:"name"`     // archive name
	Path           string           `json:"path"`     // the directory being shown ("" = archive root)
	ColorBy        string           `json:"color_by"` // "protection" | "drift" (the mode actually used)
	DriftAvailable bool             `json:"drift_available"`
	Crumbs         []TreemapCrumb   `json:"crumbs"`
	Children       []TreemapNode    `json:"children"`
	Size           int64            `json:"size"`         // total bytes at this level
	Files          int              `json:"files"`        // total files at this level
	StatusBytes    map[string]int64 `json:"status_bytes"` // legend: exact per-status bytes at this level (unaffected by folding)
	Folded         int              `json:"folded,omitempty"`
}

// driftSeverity ranks drift states for the folder worst-of-children rollup, higher
// = worse. UNCHANGED is best; MISSING is worst.
func driftSeverity(s string) int {
	switch s {
	case "MISSING":
		return 5
	case "MODIFIED":
		return 4
	case "NEW":
		return 3
	case "MOVED":
		return 2
	case "UNCHANGED":
		return 0
	}
	return 1
}

// validationSeverity ranks catalog-derived verify states for the folder rollup,
// higher = more attention-worthy. FAILED wins so a folder holding any failed-verify
// file shows red; a fully-verified folder shows green.
func validationSeverity(s string) int {
	switch s {
	case "FAILED":
		return 5
	case "UNHASHED":
		return 3
	case "HASHED":
		return 2
	case "VERIFIED":
		return 1
	}
	return 0
}

// treemapAgg accumulates one child (directory or file) at the current level.
type treemapAgg struct {
	name        string
	path        string
	isDir       bool
	hasSub      bool // holds at least one child DIRECTORY (vs. only files) — used by the folder tree
	size        int64
	files       int
	worst       string
	worstSev    int
	statusBytes map[string]int64
}

func (a *treemapAgg) node() TreemapNode {
	return TreemapNode{Name: a.name, Path: a.path, IsDir: a.isDir, Size: a.size, Files: a.files,
		Status: a.worst, HasChildren: a.isDir, StatusBytes: a.statusBytes}
}

// levelAggregate computes one zoom level: the immediate children of dirPath with a
// worst-status + byte rollup per child, plus the level totals and breadcrumb. It is
// the shared core behind both the risk treemap (which folds small children into an
// "other" block) and the Archives folder tree (which keeps every child folder,
// name-sorted and paginated). dirPath is "" for the archive root (whose children are
// the scanned folders). colorBy is "protection" (default), "drift", or "validation";
// drift falls back to protection when no reconcile report exists. The returned
// TreemapResult has every field set EXCEPT Children/Folded, which each caller fills;
// the returned severity ranker matches the active coloring mode.
func (s *Store) levelAggregate(collectionID int, dirPath, colorBy string) ([]*treemapAgg, TreemapResult, func(string) int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := TreemapResult{CollectionID: collectionID, Path: dirPath, ColorBy: "protection",
		StatusBytes: map[string]int64{}}
	for _, c := range s.c.Collections {
		if c.ID == collectionID {
			res.Name = c.Name
			break
		}
	}

	// Per-file verified physical copies, deduped across every non-FAILED chunk —
	// the same basis Protection uses.
	vols := map[int]*Volume{}
	for _, v := range s.c.Volumes {
		vols[v.ID] = v
	}
	locs := s.locationsMapLocked()
	fileCopies := map[int]map[string]physCopy{}
	// failedFiles: files backed by a chunk copy whose last verify FAILED (VerifyOK
	// explicitly false). Used only by the "validation" coloring — a verified copy
	// (fileCopies) still wins, so red marks files whose backups failed a check.
	failedFiles := map[int]bool{}
	for _, ch := range s.c.Chunks {
		if ch.CollectionID != collectionID || ch.Status == "FAILED" {
			continue
		}
		for _, cp := range ch.Copies {
			if cp.VerifyOK != nil && !*cp.VerifyOK {
				for _, cf := range ch.Files {
					failedFiles[cf.FileID] = true
				}
				break
			}
		}
		pcs := chunkPhysCopies(ch, vols, locs)
		if len(pcs) == 0 {
			continue
		}
		for _, cf := range ch.Files {
			m := fileCopies[cf.FileID]
			if m == nil {
				m = map[string]physCopy{}
				fileCopies[cf.FileID] = m
			}
			for sig, pc := range pcs {
				m[sig] = pc
			}
		}
	}

	folderPath := map[int]string{}
	for _, fo := range s.c.Folders {
		if fo.CollectionID == collectionID {
			// Canonical (clean, forward-slashed, no trailing slash) so per-file full paths
			// can be built by cheap concatenation in the hot loop below.
			folderPath[fo.ID] = strings.TrimRight(filepath.ToSlash(fo.Path), "/")
		}
	}

	// Drift overlay (by rel path) if a reconcile report exists and it was asked for.
	driftByRel := map[string]string{}
	for _, d := range s.c.Drift {
		if d.CollectionID == collectionID {
			res.DriftAvailable = true
			for _, it := range d.Items {
				driftByRel[normPath(it.Path)] = it.State
			}
			break
		}
	}
	useDrift := strings.EqualFold(colorBy, "drift") && res.DriftAvailable
	useValidation := strings.EqualFold(colorBy, "validation")
	if useDrift {
		res.ColorBy = "drift"
	} else if useValidation {
		res.ColorBy = "validation"
	}
	severity := statusSeverity
	if useDrift {
		severity = driftSeverity
	} else if useValidation {
		severity = validationSeverity
	}

	profCache := map[string]*Profile{}
	resolve := func(p string) *Profile {
		if v, ok := profCache[p]; ok {
			return v
		}
		v := s.resolveProfileLocked(collectionID, p)
		profCache[p] = v
		return v
	}

	P := strings.TrimRight(filepath.ToSlash(dirPath), "/")
	nP := normPath(P)
	archiveRoot := P == ""
	caseFold := runtime.GOOS == "windows" // case-insensitive path matching, as normPath does

	children := map[string]*treemapAgg{}
	get := func(key, name, cpath string, isDir bool) *treemapAgg {
		a := children[key]
		if a == nil {
			a = &treemapAgg{name: name, path: cpath, isDir: isDir, worstSev: -1, statusBytes: map[string]int64{}}
			children[key] = a
		}
		return a
	}

	for _, f := range s.c.Files {
		if f.CollectionID != collectionID {
			continue
		}
		root := folderPath[f.FolderID]
		// root is canonical (clean, forward-slashed, no trailing slash) and RelPath is
		// stored forward-slashed, so a plain concat equals ToSlash(Join(...)) without the
		// per-file filepath.Clean — the difference that keeps a 200k-file expansion under
		// the navigation budget (see TestTreeExpansionBudget).
		full := root + "/" + f.RelPath

		// Which immediate child of the current level does this file belong to?
		var childKey, childName, childPath string
		var childIsDir, childHasSub bool
		if archiveRoot {
			// The archive root's children are the scanned folders themselves.
			childPath = root
			childName = lastSeg(root)
			childKey = normPath(root)
			childIsDir = true
			childHasSub = strings.IndexByte(f.RelPath, '/') >= 0 // a deeper path → this root has subfolders
		} else {
			// full is already clean+forward-slashed, so folding is just the platform case
			// rule — skip the redundant Clean that normPath would repeat for every file.
			nFull := full
			if caseFold {
				nFull = strings.ToLower(full)
			}
			if nFull != nP && !strings.HasPrefix(nFull, nP+"/") {
				continue // not under the zoomed directory
			}
			if len(full) <= len(P)+1 {
				continue // guard: the directory itself
			}
			rest := full[len(P)+1:] // exact-case remainder (P is a canonical stored prefix)
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				childName = rest[:i]
				childPath = P + "/" + childName
				childIsDir = true
				childHasSub = strings.IndexByte(rest[i+1:], '/') >= 0 // another separator → a sub-subfolder
			} else {
				childName = rest
				childPath = full
				childIsDir = false
			}
			childKey = normPath(childPath)
		}

		// Status of this file in the active coloring mode.
		var st string
		if useValidation {
			// Catalog-derived verify state: a verified copy wins (green); otherwise a
			// file whose backup failed a check is red; a hashed-but-unverified file is
			// neutral; an unhashed one is grey.
			switch {
			case len(fileCopies[f.ID]) > 0:
				st = "VERIFIED"
			case failedFiles[f.ID]:
				st = "FAILED"
			case f.Hash != "":
				st = "HASHED"
			default:
				st = "UNHASHED"
			}
		} else if useDrift {
			if v, ok := driftByRel[normPath(f.RelPath)]; ok {
				st = v
			} else {
				st = "UNCHANGED"
			}
		} else {
			fileDir := path.Dir(full)
			prof := resolve(fileDir)
			dims := dimsFromCopies(fileCopies[f.ID], prof)
			st = statusFor(prof, dims)
		}

		res.Size += f.SizeBytes
		res.Files++
		res.StatusBytes[st] += f.SizeBytes

		a := get(childKey, childName, childPath, childIsDir)
		a.size += f.SizeBytes
		a.statusBytes[st] += f.SizeBytes
		if childIsDir {
			a.files++
			if childHasSub {
				a.hasSub = true
			}
		} else {
			a.files = 1
		}
		if sv := severity(st); sv > a.worstSev {
			a.worstSev, a.worst = sv, st
		}
	}

	// Materialize the children (unsorted); each caller applies its own ordering.
	all := make([]*treemapAgg, 0, len(children))
	for _, a := range children {
		all = append(all, a)
	}
	res.Crumbs = treemapCrumbs(res.Name, P, folderPath)
	return all, res, severity
}

// Treemap computes one zoom level of the risk treemap for an archive: the immediate
// children sorted by size, with the small tail folded into a single synthetic
// "other" block so a directory with tens of thousands of entries still draws a
// readable handful. See levelAggregate for dirPath/colorBy semantics.
func (s *Store) Treemap(collectionID int, dirPath, colorBy string) TreemapResult {
	all, res, severity := s.levelAggregate(collectionID, dirPath, colorBy)
	sort.Slice(all, func(i, j int) bool {
		if all[i].size != all[j].size {
			return all[i].size > all[j].size
		}
		return all[i].name < all[j].name
	})
	threshold := int64(float64(res.Size) * treemapMinFraction)
	var other *treemapAgg
	for i, a := range all {
		if i < treemapMaxChildren && a.size >= threshold {
			res.Children = append(res.Children, a.node())
			continue
		}
		if other == nil {
			other = &treemapAgg{name: "other", isDir: true, worstSev: -1, statusBytes: map[string]int64{}}
		}
		other.size += a.size
		other.files += a.files
		for k, v := range a.statusBytes {
			other.statusBytes[k] += v
			if sv := severity(k); sv > other.worstSev {
				other.worstSev, other.worst = sv, k
			}
		}
		res.Folded++
	}
	if other != nil {
		n := other.node()
		n.Other, n.HasChildren = true, false // the "other" bucket is a summary, not zoomable
		res.Children = append(res.Children, n)
	}
	return res
}

// FolderTreeNode is one row of the Archives folder tree: an immediate child
// directory of the level being shown, with server-side rollups. The tree's unit is
// the folder — individual files are never listed — so file counts and bytes are the
// rollup of everything beneath the folder, and Status is its worst-of-children
// protection status (color + shape + text in the UI).
type FolderTreeNode struct {
	Name        string `json:"name"`
	Path        string `json:"path"` // canonical stored path; echo back to expand this folder
	Files       int    `json:"files"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`       // worst protection status within this folder
	HasChildren bool   `json:"has_children"` // holds at least one SUBFOLDER (so the row is expandable)
}

// FolderTreeResult is one expanded level of the Archives folder tree: the child
// folders to draw, the breadcrumb from the archive root, and a virtualization window
// (Offset/Total/Truncated) so a pathological wide folder never ships the whole level.
type FolderTreeResult struct {
	CollectionID int              `json:"collection_id"`
	Name         string           `json:"name"` // archive name
	Path         string           `json:"path"` // directory being shown ("" = archive root)
	Crumbs       []TreemapCrumb   `json:"crumbs"`
	Children     []FolderTreeNode `json:"children"`
	Offset       int              `json:"offset"`
	Total        int              `json:"total"`     // total child folders at this level
	Truncated    bool             `json:"truncated"` // more child folders exist beyond this window
}

// folderTreeMaxLimit caps rows per expansion. Immediate-subfolder counts are small
// even at 200k files, but the cap guarantees the navigation payload budget (< 50KB;
// see TestTreeExpansionBudget) in the pathological wide-folder case.
const folderTreeMaxLimit = 200

// FolderTree returns one level of the Archives folder tree: the immediate child
// DIRECTORIES of dirPath, name-sorted (file-manager order) and paginated. It reuses
// levelAggregate (immediate children + server-side rollups, never the whole tree) and
// colors by protection status. Files are not listed individually — the tree's unit is
// the folder, with counts as rollups.
func (s *Store) FolderTree(collectionID int, dirPath string, offset, limit int) FolderTreeResult {
	all, agg, _ := s.levelAggregate(collectionID, dirPath, "protection")
	// Directories only; a file leaf is not an expandable folder row. Filter in place.
	dirs := all[:0]
	for _, a := range all {
		if a.isDir {
			dirs = append(dirs, a)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		li, lj := strings.ToLower(dirs[i].name), strings.ToLower(dirs[j].name)
		if li != lj {
			return li < lj
		}
		return dirs[i].name < dirs[j].name
	})
	if limit <= 0 || limit > folderTreeMaxLimit {
		limit = folderTreeMaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(dirs) {
		offset = len(dirs)
	}
	end := offset + limit
	if end > len(dirs) {
		end = len(dirs)
	}
	res := FolderTreeResult{CollectionID: collectionID, Name: agg.Name, Path: agg.Path,
		Crumbs: agg.Crumbs, Total: len(dirs), Offset: offset, Truncated: end < len(dirs)}
	for _, a := range dirs[offset:end] {
		res.Children = append(res.Children, FolderTreeNode{
			Name: a.name, Path: a.path, Files: a.files, Size: a.size,
			Status: a.worst, HasChildren: a.hasSub,
		})
	}
	return res
}

// treemapCrumbs builds the zoom breadcrumb from the archive root down to P. The
// first hop is the archive itself; then the scanned folder that owns P; then each
// path segment beneath it.
func treemapCrumbs(archiveName, P string, folderPath map[int]string) []TreemapCrumb {
	crumbs := []TreemapCrumb{{Name: archiveName, Path: ""}}
	if P == "" {
		return crumbs
	}
	// Find the scanned folder root that owns P (longest matching prefix).
	root := ""
	nP := normPath(P)
	for _, fp := range folderPath {
		nfp := normPath(fp)
		if (nP == nfp || strings.HasPrefix(nP, nfp+"/")) && len(fp) > len(root) {
			root = fp
		}
	}
	if root == "" {
		// P isn't under a known folder (defensive) — show it as a single hop.
		crumbs = append(crumbs, TreemapCrumb{Name: lastSeg(P), Path: P})
		return crumbs
	}
	crumbs = append(crumbs, TreemapCrumb{Name: lastSeg(root), Path: root})
	rest := strings.TrimPrefix(P[len(root):], "/")
	if rest == "" {
		return crumbs
	}
	cur := root
	for _, seg := range strings.Split(rest, "/") {
		cur = cur + "/" + seg
		crumbs = append(crumbs, TreemapCrumb{Name: seg, Path: cur})
	}
	return crumbs
}

// lastSeg returns the final path segment for display, falling back to the whole
// string (e.g. a bare drive root like "Y:/").
func lastSeg(p string) string {
	p = strings.TrimRight(filepath.ToSlash(p), "/")
	if p == "" {
		return "/"
	}
	if i := strings.LastIndexByte(p, '/'); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

// ExploreStatBucket is one (name, files, bytes) row in the Explorer info panel —
// used for the largest-folders, per-role, and per-extension breakdowns.
type ExploreStatBucket struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

// ExploreStats is the info panel beside the treemap: totals plus the largest
// folders, role breakdown, and per-extension counts, all scoped to dirPath.
type ExploreStats struct {
	CollectionID   int                 `json:"collection_id"`
	Path           string              `json:"path"`
	TotalFiles     int                 `json:"total_files"`
	TotalBytes     int64               `json:"total_bytes"`
	LargestFolders []ExploreStatBucket `json:"largest_folders"`
	Roles          []ExploreStatBucket `json:"roles"`
	Extensions     []ExploreStatBucket `json:"extensions"`
}

// exploreStatsMaxRows caps each breakdown list so a million-file archive still
// returns a readable panel (the treemap's own folding handles the map).
const exploreStatsMaxRows = 24

// ExploreStats computes the Explorer info-panel breakdown for one archive, scoped
// to dirPath ("" = whole archive). Pure catalog read — never walks the disk, the
// same contract as Treemap. Largest folders are the immediate children of dirPath
// (directories), matching what the treemap draws at that level.
func (s *Store) ExploreStats(collectionID int, dirPath string) ExploreStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := ExploreStats{CollectionID: collectionID, Path: dirPath}
	folderPath := map[int]string{}
	for _, fo := range s.c.Folders {
		if fo.CollectionID == collectionID {
			folderPath[fo.ID] = filepath.ToSlash(fo.Path)
		}
	}
	P := strings.TrimRight(filepath.ToSlash(dirPath), "/")
	nP := normPath(P)
	archiveRoot := P == ""

	folders := map[string]*statAcc{}
	roles := map[string]*statAcc{}
	exts := map[string]*statAcc{}
	bump := func(m map[string]*statAcc, key string, bytes int64) {
		a := m[key]
		if a == nil {
			a = &statAcc{}
			m[key] = a
		}
		a.files++
		a.bytes += bytes
	}

	for _, f := range s.c.Files {
		if f.CollectionID != collectionID {
			continue
		}
		root := folderPath[f.FolderID]
		full := filepath.ToSlash(filepath.Join(root, f.RelPath))

		// Immediate child folder of the current level this file rolls up into.
		var childName string
		if archiveRoot {
			childName = lastSeg(root)
		} else {
			nFull := normPath(full)
			if nFull != nP && !strings.HasPrefix(nFull, nP+"/") {
				continue // not under the zoomed directory
			}
			if len(full) <= len(P)+1 {
				continue
			}
			rest := full[len(P)+1:]
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				childName = rest[:i] // a sub-directory
			} else {
				childName = "" // a file directly at this level (no folder bucket)
			}
		}

		res.TotalFiles++
		res.TotalBytes += f.SizeBytes
		if childName != "" {
			bump(folders, childName, f.SizeBytes)
		}
		role := f.Role
		if role == "" {
			role = RoleOther
		}
		bump(roles, role, f.SizeBytes)
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.RelPath), "."))
		if ext == "" {
			ext = "(none)"
		}
		bump(exts, ext, f.SizeBytes)
	}

	res.LargestFolders = topBuckets(folders, exploreStatsMaxRows)
	res.Roles = topBuckets(roles, 0)
	res.Extensions = topBuckets(exts, exploreStatsMaxRows)
	return res
}

// statAcc accumulates files+bytes for one Explorer breakdown bucket.
type statAcc struct {
	files int
	bytes int64
}

// topBuckets flattens an accumulator map into rows sorted by bytes desc (name as
// tiebreak). limit<=0 returns all rows.
func topBuckets(m map[string]*statAcc, limit int) []ExploreStatBucket {
	out := make([]ExploreStatBucket, 0, len(m))
	for name, a := range m {
		out = append(out, ExploreStatBucket{Name: name, Files: a.files, Bytes: a.bytes})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Name < out[j].Name
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
