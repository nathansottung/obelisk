//go:build !guionly

package main

// compare_routes.go — the HTTP routes for comparing drive scans (compare.go)
// and the cross-drive lists (xdrive_lists.go). All are reports: none changes a
// package, copy or count.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func registerCompareRoutes(mux *http.ServeMux, app *App) {
	registerCompareExports(mux, app)
	// The marked reference (a drive or an archive), or null.
	mux.HandleFunc("GET /api/reference", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]any{"reference": app.Store.Reference()})
	})
	// Mark a drive ({"kind":"volume","id":N}) or an archive ({"kind":"archive",...})
	// as the reference; an empty kind clears it.
	mux.HandleFunc("POST /api/reference", func(w http.ResponseWriter, r *http.Request) {
		b := body(r)
		kind, id := s(b, "kind"), int(f(b, "id"))
		var m *ReferenceMark
		switch kind {
		case "":
		case "volume":
			if app.Store.Volume(id) == nil {
				jsonErr(w, 404, fmt.Errorf("volume %d not found", id))
				return
			}
			m = &ReferenceMark{Kind: kind, ID: id}
		case "archive":
			if app.Store.Collection(id) == nil {
				jsonErr(w, 404, fmt.Errorf("archive %d not found", id))
				return
			}
			m = &ReferenceMark{Kind: kind, ID: id}
		default:
			jsonErr(w, 400, fmt.Errorf("kind must be volume, archive or empty"))
			return
		}
		if err := app.Store.SetReference(m); err != nil {
			jsonErr(w, 500, err)
			return
		}
		jsonOut(w, map[string]any{"reference": m})
	})
	// Every scan of every drive, newest first, for choosing what to compare.
	mux.HandleFunc("GET /api/scans", func(w http.ResponseWriter, r *http.Request) {
		recs := app.Store.AllScanRecords()
		out := make([]map[string]any, 0, len(recs))
		for _, rec := range recs {
			out = append(out, map[string]any{"scan": rec, "volume_label": app.volumeLabel(rec.VolumeID)})
		}
		jsonOut(w, map[string]any{"scans": out})
	})
	// Compare other_scan_id against ref ({"scan_id"}, {"volume_id"}, {"archive_id"},
	// or nothing for the marked reference). The run is saved.
	mux.HandleFunc("POST /api/compare", func(w http.ResponseWriter, r *http.Request) {
		b := body(r)
		var ref CompareRef
		if m, ok := b["ref"].(map[string]any); ok {
			ref = CompareRef{ScanID: int(f(m, "scan_id")), VolumeID: int(f(m, "volume_id")), ArchiveID: int(f(m, "archive_id"))}
		}
		run, err := app.CompareScans(ref, int(f(b, "other_scan_id")))
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"run": run})
	})
	// Files only on drive ?volume_id= and files on two or more drives, from each
	// drive's latest complete scan.
	mux.HandleFunc("GET /api/xdrive/lists", func(w http.ResponseWriter, r *http.Request) {
		vid, _ := strconv.Atoi(r.URL.Query().Get("volume_id"))
		l, err := app.CrossDriveLists(vid)
		if err != nil {
			jsonErr(w, 500, err)
			return
		}
		jsonOut(w, l)
	})
	mux.HandleFunc("GET /api/compare/runs", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]any{"runs": app.Store.CompareRuns()})
	})
	// One run's results: ?state=moved,new (any of), ?q= (in either path, any
	// case), ?offset= and ?limit= (at most 2000; 200 by default).
	mux.HandleFunc("GET /api/compare/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		run, results, ok := compareRunResults(w, r, app)
		if !ok {
			return
		}
		q := r.URL.Query()
		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 200
		}
		limit = min(limit, 2000)
		offset = max(offset, 0)
		page := results[min(offset, len(results)):min(offset+limit, len(results))]
		jsonOut(w, map[string]any{"run": run, "matching": len(results), "offset": offset, "results": page})
	})
}

// registerCompareExports adds the CSV and JSON downloads of a run's results and
// of the cross-drive lists.
func registerCompareExports(mux *http.ServeMux, app *App) {
	// A run's results with the view's filters (?state=, ?q=), as ?format=csv or json.
	mux.HandleFunc("GET /api/compare/runs/{id}/export", func(w http.ResponseWriter, r *http.Request) {
		run, results, ok := compareRunResults(w, r, app)
		if !ok {
			return
		}
		name := fmt.Sprintf("obelisk-comparison-%d", run.ID)
		if r.URL.Query().Get("format") == "json" {
			attachmentHeaders(w, name+".json", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"run": run, "results": results})
			return
		}
		attachmentHeaders(w, name+".csv", "text/csv; charset=utf-8")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"state", "ref_path", "path", "ref_hash", "hash", "size"})
		for _, c := range results {
			_ = cw.Write(csvSafe(c.State, c.RefPath, c.Path, c.RefHash, c.Hash, strconv.FormatInt(c.Size, 10)))
		}
		cw.Flush()
	})
	// ?list=only_on (with ?volume_id=) or on_two_or_more, as ?format=csv or json.
	mux.HandleFunc("GET /api/xdrive/lists/export", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		vid, _ := strconv.Atoi(q.Get("volume_id"))
		l, err := app.CrossDriveLists(vid)
		if err != nil {
			jsonErr(w, 500, err)
			return
		}
		list, entries := "on-two-or-more", l.OnTwoOrMore
		if q.Get("list") == "only_on" {
			list, entries = fmt.Sprintf("only-on-volume-%d", vid), l.OnlyOn
		}
		name := "obelisk-files-" + list
		if q.Get("format") == "json" {
			attachmentHeaders(w, name+".json", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"scans": l.Scans, "left_out": l.LeftOut, "files": entries})
			return
		}
		attachmentHeaders(w, name+".csv", "text/csv; charset=utf-8")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"hash", "size", "drives", "label", "path"})
		for _, e := range entries {
			for _, p := range e.Places {
				_ = cw.Write(csvSafe(e.Hash, strconv.FormatInt(e.Size, 10), strconv.Itoa(e.Drives), p.Label, p.Path))
			}
		}
		cw.Flush()
	})
}

func attachmentHeaders(w http.ResponseWriter, name, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
}

// csvSafe writes a cell a spreadsheet would run as a formula (one starting with
// =, +, -, @, a tab or a carriage return) as text, with a leading quote mark.
func csvSafe(cells ...string) []string {
	for i, c := range cells {
		if c != "" && strings.ContainsRune("=+-@\t\r", rune(c[0])) {
			cells[i] = "'" + c
		}
	}
	return cells
}

// compareRunResults loads run {id}'s results filtered by ?state= and ?q=.
func compareRunResults(w http.ResponseWriter, r *http.Request, app *App) (*CompareRun, []ClassResult, bool) {
	run := app.Store.CompareRun(pathID(r))
	if run == nil {
		jsonErr(w, 404, fmt.Errorf("comparison not found"))
		return nil, nil, false
	}
	all, err := app.LoadCompareResults(run)
	if err != nil {
		jsonErr(w, 500, err)
		return nil, nil, false
	}
	return run, filterClassResults(all, r.URL.Query().Get("state"), r.URL.Query().Get("q")), true
}

// filterClassResults keeps results in any of the comma-separated states (all
// when empty) whose path or reference path contains q, ignoring case.
func filterClassResults(all []ClassResult, states, q string) []ClassResult {
	want := map[string]bool{}
	for _, st := range strings.Split(states, ",") {
		if st = strings.TrimSpace(strings.ToLower(st)); st != "" {
			want[st] = true
		}
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]ClassResult, 0, len(all))
	for _, c := range all {
		if len(want) > 0 && !want[c.State] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Path), q) && !strings.Contains(strings.ToLower(c.RefPath), q) {
			continue
		}
		out = append(out, c)
	}
	return out
}
