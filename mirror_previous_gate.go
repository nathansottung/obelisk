//go:build !guionly

package main

// mirror_previous_gate.go — listing and removing the earlier versions a mirror
// kept (mirror_previous.go). Removal goes only through the delete gate.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
)

// MirrorKeptRun is one run's kept versions on a destination.
type MirrorKeptRun struct {
	Run    string `json:"run"`
	Path   string `json:"path"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
	Status string `json:"status"`
}

// MirrorKeptRuns lists the kept-version runs on the destination at destDir.
func (a *App) MirrorKeptRuns(destDir string) ([]MirrorKeptRun, int64, error) {
	dest, err := a.Store.AuthorizeDestination(destDir)
	if err != nil {
		return nil, 0, err
	}
	defer dest.Close()
	d, err := dest.root.Open(mirrorPreviousDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return nil, 0, err
	}
	var out []MirrorKeptRun
	var total int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := filepath.Join(mirrorPreviousDir, e.Name())
		r := MirrorKeptRun{Run: e.Name(), Path: filepath.Join(dest.Canonical(), rel), Status: "unknown"}
		if b, err := dest.root.ReadFile(filepath.Join(rel, "RUN.json")); err == nil {
			var m runMarker
			if json.Unmarshal(b, &m) == nil {
				r.Status, r.Files = m.Status, len(m.Kept)
			}
		}
		r.Bytes = treeSizeIn(dest.root, rel)
		total += r.Bytes
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Run < out[j].Run })
	return out, total, nil
}

func init() {
	destructiveOps["mirror-previous-remove"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Remove the earlier versions a mirror kept in %s/%s on %s", mirrorPreviousDir, paramString(p, "run"), paramString(p, "dest"))
		},
		targets: func(a *App, p map[string]any) ([]destructiveTarget, error) {
			run := paramString(p, "run")
			if !validEntryName(run) {
				return nil, refuseDestructive(400, "not a kept-version run: %q", run)
			}
			dest, err := a.Store.AuthorizeDestination(paramString(p, "dest"))
			if err != nil {
				return nil, err
			}
			defer dest.Close()
			top, terr := dest.root.Lstat(mirrorPreviousDir)
			fi, err := dest.root.Lstat(filepath.Join(mirrorPreviousDir, run))
			if terr != nil || isLinkMode(top.Mode()) || err != nil || !fi.IsDir() || isLinkMode(fi.Mode()) {
				return nil, refuseDestructive(400, "%s/%s is not a kept-version folder on %s", mirrorPreviousDir, run, paramString(p, "dest"))
			}
			return treeTargets(filepath.Join(dest.Canonical(), mirrorPreviousDir, run), "earlier version a mirror kept")
		},
		run: func(a *App, p map[string]any) (any, error) {
			dest, err := a.Store.AuthorizeDestination(paramString(p, "dest"))
			if err != nil {
				return nil, err
			}
			defer dest.Close()
			if err := removeGatedEntryIn(dest.root, mirrorPreviousDir, paramString(p, "run")); err != nil {
				return nil, err
			}
			a.Store.Log("mirror", fmt.Sprintf("removed the earlier versions kept in %s/%s on %s", mirrorPreviousDir, paramString(p, "run"), paramString(p, "dest")))
			return map[string]any{"removed": true}, nil
		},
	}
}

// mirrorPreviousRoutes registers the listing of kept versions.
func mirrorPreviousRoutes(mux routeRegistrar, app *App) {
	mux.HandleFunc("GET /api/mirror-previous", func(w http.ResponseWriter, r *http.Request) {
		runs, total, err := app.MirrorKeptRuns(r.URL.Query().Get("dest"))
		if runs == nil {
			runs = []MirrorKeptRun{}
		}
		jsonResult(w, map[string]any{"runs": runs, "bytes": total}, err)
	})
}
