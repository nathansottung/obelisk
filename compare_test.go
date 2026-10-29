//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanDrive docks drive under serial in a scan-only session and returns its
// newest scan.
func scanDrive(t *testing.T, app *App, drive, serial string) *ScanRecord {
	t.Helper()
	ds, err := app.StartDockSession(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.IngestDrive(ds.ID, drive, serial, serial, "", "", false, noProg); err != nil {
		t.Fatal(err)
	}
	return app.Store.ScanRecords(app.Store.VolumeBySerial(serial).ID)[0]
}

// A drive or an archive can be the reference. A comparison of two scans (or of a
// scan against the reference) is saved with its counts and a result file, and
// changes no copy counts.
func TestCompare_ReferenceAndSavedRuns(t *testing.T) {
	app := dockApp(t)
	refDrive := t.TempDir()
	writeTree(t, refDrive, map[string]string{"a.txt": "alpha\n", "b.txt": "bravo\n", "c.txt": "charlie\n", "locked/d.txt": "delta\n"})
	refScan := scanDrive(t, app, refDrive, "REF")
	refVol := app.Store.VolumeBySerial("REF")

	other := t.TempDir()
	writeTree(t, other, map[string]string{"a.txt": "alpha\n", "b.txt": "BRAVO\n", "moved/c.txt": "charlie\n", "e.txt": "echo\n", "locked/d.txt": "delta\n"})
	makeUnlistable(t, filepath.Join(other, "locked"))
	otherScan := scanDrive(t, app, other, "OTH")

	chunksBefore, _ := json.Marshal(app.Store.Chunks(0))

	// Two scans by ID.
	run, err := app.CompareScans(CompareRef{ScanID: refScan.ID}, otherScan.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{ClassSame: 1, ClassChanged: 1, ClassMoved: 1, ClassNew: 1, ClassUnknown: 1}
	for st, n := range want {
		if run.Counts[st] != n {
			t.Errorf("%s: %d, want %d (all counts %v)", st, run.Counts[st], n, run.Counts)
		}
	}
	if run.RefScanID != refScan.ID || run.OtherScanID != otherScan.ID || run.OtherComplete {
		t.Errorf("run %+v", run)
	}
	if _, err := os.Stat(filepath.Join(app.DataDir, "compares", run.ResultFile)); err != nil {
		t.Fatalf("result file: %v", err)
	}
	res, err := app.LoadCompareResults(run)
	if err != nil || len(res) != 5 {
		t.Fatalf("results %v %v", res, err)
	}

	// The marked reference drive: its latest complete scan.
	if err := app.Store.SetReference(&ReferenceMark{Kind: "volume", ID: refVol.ID}); err != nil {
		t.Fatal(err)
	}
	run2, err := app.CompareScans(CompareRef{}, otherScan.ID)
	if err != nil || run2.RefScanID != refScan.ID {
		t.Fatalf("against the reference drive: %+v %v", run2, err)
	}

	// An archive as the reference: its current files.
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a.txt": "alpha\n", "z.txt": "zulu\n"})
	coll := app.Store.AddCollection("Photos")
	if _, _, err := app.ScanFolder(coll.ID, src, noProg); err != nil {
		t.Fatal(err)
	}
	run3, err := app.CompareScans(CompareRef{ArchiveID: coll.ID}, otherScan.ID)
	if err != nil || run3.RefArchiveID != coll.ID || run3.Counts[ClassSame] != 1 || run3.Counts[ClassMissing] != 1 {
		t.Fatalf("against an archive: %+v %v", run3, err)
	}

	runs := app.Store.CompareRuns()
	if len(runs) != 3 || runs[0].ID != run3.ID {
		t.Errorf("history %d runs, newest first", len(runs))
	}
	if after, _ := json.Marshal(app.Store.Chunks(0)); string(after) != string(chunksBefore) {
		t.Error("a comparison changed the packages or copies")
	}
}

// The routes mark the reference, run and list comparisons, and filter a run's
// results by state and text.
func TestCompare_Routes(t *testing.T) {
	app := dockApp(t)
	a := t.TempDir()
	writeTree(t, a, map[string]string{"a.txt": "alpha\n", "b.txt": "bravo\n"})
	scanDrive(t, app, a, "R1")
	b := t.TempDir()
	writeTree(t, b, map[string]string{"a.txt": "alpha\n", "x/b.txt": "bravo\n", "n.txt": "new\n"})
	other := scanDrive(t, app, b, "O1")
	mux := http.NewServeMux()
	api(mux, app)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	call := func(method, path string, body any) map[string]any {
		t.Helper()
		bb, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(bb))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if resp.StatusCode != 200 {
			t.Fatalf("%s %s: %d %v", method, path, resp.StatusCode, out)
		}
		return out
	}
	call("POST", "/api/reference", map[string]any{"kind": "volume", "id": app.Store.VolumeBySerial("R1").ID})
	run := call("POST", "/api/compare", map[string]any{"other_scan_id": other.ID})["run"].(map[string]any)
	id := int(run["id"].(float64))
	page := call("GET", fmt.Sprintf("/api/compare/runs/%d?state=moved,new", id), nil)
	if n := page["matching"].(float64); n != 2 {
		t.Errorf("moved and new: %v results, want 2", n)
	}
	page = call("GET", fmt.Sprintf("/api/compare/runs/%d?q=X/B", id), nil)
	if n := page["matching"].(float64); n != 1 {
		t.Errorf("text filter: %v results, want 1", n)
	}
	if runs := call("GET", "/api/compare/runs", nil)["runs"].([]any); len(runs) != 1 {
		t.Errorf("%d runs listed", len(runs))
	}
}

// A run's results, and the cross-drive lists, export as CSV and JSON
// downloads, with the view's filters applied. A cell that a spreadsheet would
// run as a formula is written as text.
func TestCompare_Export(t *testing.T) {
	app := dockApp(t)
	a := t.TempDir()
	writeTree(t, a, map[string]string{"a.txt": "alpha\n", "=SUM(1).txt": "formula\n"})
	ref := scanDrive(t, app, a, "R2")
	b := t.TempDir()
	writeTree(t, b, map[string]string{"a.txt": "alpha\n", "n.txt": "new\n"})
	other := scanDrive(t, app, b, "O2")
	run, err := app.CompareScans(CompareRef{ScanID: ref.ID}, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api(mux, app)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	get := func(path string) (string, http.Header) {
		t.Helper()
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, b)
		}
		return string(b), resp.Header
	}
	body, hdr := get(fmt.Sprintf("/api/compare/runs/%d/export?format=csv", run.ID))
	if !strings.Contains(hdr.Get("Content-Disposition"), "attachment") || !strings.HasPrefix(body, "state,ref_path,path,") {
		t.Errorf("CSV export: %q %q", hdr.Get("Content-Disposition"), body)
	}
	if !strings.Contains(body, "'=SUM(1).txt") {
		t.Errorf("a formula-like path was not written as text: %s", body)
	}
	body, _ = get(fmt.Sprintf("/api/compare/runs/%d/export?format=json&state=new", run.ID))
	var got struct {
		Run     CompareRun    `json:"run"`
		Results []ClassResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil || len(got.Results) != 1 || got.Results[0].Path != "n.txt" {
		t.Errorf("JSON export filtered to new: %v %+v", err, got.Results)
	}
	body, _ = get("/api/xdrive/lists/export?format=csv&list=on_two_or_more")
	if !strings.HasPrefix(body, "hash,size,drives,label,path") || !strings.Contains(body, "a.txt") {
		t.Errorf("lists CSV: %s", body)
	}
}

// The compare screen is a view in the app with the controls it needs.
func TestCompareScreenWiredInUI(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("ui", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)
	for _, need := range []string{
		"async function vCompare(", "compare:vCompare", "['compare',", "/api/compare/runs", "/api/xdrive/lists",
		"/export?format=csv", "/export?format=json", "/api/reference",
	} {
		if !strings.Contains(html, need) {
			t.Errorf("UI missing %q", need)
		}
	}
}
