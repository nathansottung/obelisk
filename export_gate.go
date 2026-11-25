//go:build !guionly

package main

// export_gate.go — listing earlier exports in a folder, and removing one only
// through the delete gate (export_names.go).

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

func init() {
	destructiveOps["export-remove"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Remove the export %s from %s", paramString(p, "name"), paramString(p, "dir"))
		},
		targets: func(a *App, p map[string]any) ([]destructiveTarget, error) {
			e, err := listedExport(a, p)
			if err != nil {
				return nil, err
			}
			ts, err := treeTargets(e.Path, "removed: an export, superseded or not")
			if err != nil {
				return nil, err
			}
			if strings.HasSuffix(e.Name, ".tar") {
				ts = append(ts, fileTarget(e.Path+".sha256", "removed with its bundle"))
			}
			return ts, nil
		},
		run: func(a *App, p map[string]any) (any, error) {
			e, err := listedExport(a, p)
			if err != nil {
				return nil, err
			}
			dir := paramString(p, "dir")
			if err := removeGatedEntry(dir, e.Name); err != nil {
				return nil, err
			}
			if strings.HasSuffix(e.Name, ".tar") {
				_ = removeGatedEntry(dir, e.Name+".sha256")
			}
			a.Store.Log("export", fmt.Sprintf("removed the export %s (%d bytes)", e.Path, e.Bytes))
			return map[string]any{"removed": e.Path}, nil
		},
	}
}

// listedExport is the export named in p, as ExportsIn lists it now.
func listedExport(a *App, p map[string]any) (*ExportEntry, error) {
	dir, kind, name := paramString(p, "dir"), paramString(p, "kind"), paramString(p, "name")
	if !validEntryName(name) {
		return nil, refuseDestructive(400, "not an export name: %q", name)
	}
	if err := a.Store.CheckOutsideSources(dir); err != nil {
		return nil, refuseDestructive(400, "refusing: %v", err)
	}
	list, _, err := ExportsIn(dir, kind)
	if err != nil {
		return nil, refuseDestructive(400, "%v", err)
	}
	for _, e := range list {
		if e.Name == name {
			e := e
			return &e, nil
		}
	}
	return nil, refuseDestructive(400, "%s is not a %s export in %s", name, kind, filepath.Clean(dir))
}

// exportRoutes registers the listing.
func exportRoutes(mux routeRegistrar, app *App) {
	mux.HandleFunc("GET /api/exports", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		list, total, err := ExportsIn(q.Get("dir"), q.Get("kind"))
		if list == nil {
			list = []ExportEntry{}
		}
		jsonResult(w, map[string]any{"exports": list, "count": len(list), "bytes": total}, err)
	})
}
