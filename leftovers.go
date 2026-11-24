//go:build !guionly

package main

// leftovers.go — what an earlier or crashed run left behind.
//
// Obelisk never removes these on its own and never ignores them: they are
// listed in Settings with their size and age (GET /api/leftovers), and each is
// removed only through the delete gate (op "leftover-remove", typed "Delete").

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// leftoverFolders are the folders checked for leftovers: the data folder and
// the folders Obelisk writes its own work into.
func (a *App) leftoverFolders() []string {
	var dirs []string
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" {
			return
		}
		for _, x := range dirs {
			if strings.EqualFold(filepath.Clean(x), filepath.Clean(d)) {
				return
			}
		}
		dirs = append(dirs, d)
	}
	add(a.DataDir)
	add(a.scansDir())
	add(a.comparesDir())
	if cfg, err := a.LoadConfig(); err == nil {
		add(cfg.StagingDir)
		add(a.escrowCacheDir(cfg))
	}
	return dirs
}

// Leftovers lists what earlier runs left in Obelisk's folders.
func (a *App) Leftovers() []Leftover {
	var out []Leftover
	for _, d := range a.leftoverFolders() {
		l, err := listLeftoversIn(d)
		if err == nil {
			out = append(out, l...)
		}
	}
	out = append(out, a.unreferencedScanFiles()...)
	// A private temporary folder (a gpg home) whose removal stopped part-way.
	if l, err := listLeftoversIn(os.TempDir()); err == nil {
		for _, x := range l {
			if strings.HasPrefix(x.Name, removingPrefix) {
				out = append(out, x)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// leftoverAt is the listed leftover at path, or nil.
func (a *App) leftoverAt(path string) *Leftover {
	for _, l := range a.Leftovers() {
		if l.Path == filepath.Clean(path) {
			l := l
			return &l
		}
	}
	return nil
}

func init() {
	destructiveOps["leftover-remove"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Remove %s, left by an earlier run", paramString(p, "path"))
		},
		targets: func(a *App, p map[string]any) ([]destructiveTarget, error) {
			l := a.leftoverAt(paramString(p, "path"))
			if l == nil {
				return nil, refuseDestructive(400, "%s is not a leftover Obelisk lists; nothing was changed", paramString(p, "path"))
			}
			if err := a.Store.CheckOutsideSources(l.Path); err != nil {
				return nil, refuseDestructive(400, "refusing: %v", err)
			}
			return treeTargets(l.Path, "removed: "+l.Kind)
		},
		run: func(a *App, p map[string]any) (any, error) {
			l := a.leftoverAt(paramString(p, "path"))
			if l == nil {
				return nil, fmt.Errorf("%s is no longer listed; nothing was changed", paramString(p, "path"))
			}
			if err := removeGatedEntry(l.Folder, l.Name); err != nil {
				return nil, err
			}
			a.Store.Log("leftovers", fmt.Sprintf("removed %s (%s, %d bytes), left by an earlier run", l.Path, l.Kind, l.Size))
			return map[string]any{"removed": l.Path}, nil
		},
	}
}

// leftoverRoutes registers the listing.
func leftoverRoutes(mux routeRegistrar, app *App) {
	mux.HandleFunc("GET /api/leftovers", func(w http.ResponseWriter, r *http.Request) {
		l := app.Leftovers()
		if l == nil {
			l = []Leftover{}
		}
		jsonOut(w, map[string]any{"leftovers": l})
	})
}
