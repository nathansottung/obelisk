//go:build !guionly

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// v8Folder is a data folder the catalog.json engine made with real operations,
// opened and closed once so it is as a user's would be, and its bytes as found.
type v8Folder struct {
	dir                 string
	catalogRaw, jobsRaw []byte
}

// makeV8DataFolder builds a real v8 data folder on the catalog.json engine: an
// archive scanned twice (so a file has versions), a volume in a location, a
// package with a verified copy and verify history, an event with files, a
// profile assignment, audit entries and a job board with a finished job.
func makeV8DataFolder(t *testing.T) v8Folder {
	t.Helper()
	t.Setenv(catalogEngineEnv, "json")
	app := abTestApp(t)
	abPopulate(t, app)
	s := app.Store
	coll := s.Collections()[0]
	loc := s.AddLocation("Shoe box", true, "under the stairs")
	vol := s.Volumes()[0]
	if err := s.SetVolumeLocation(vol.ID, loc.ID); err != nil {
		t.Fatal(err)
	}
	ev := s.AddEvent(&Event{Name: "Wedding", EventType: "wedding", Year: 2024, CollectionID: coll.ID})
	var ids []int
	for _, f := range testFiles(t, s.FilesOf(coll.ID)) {
		ids = append(ids, f.ID)
	}
	if _, err := s.AssignFilesToEvent(ids, ev.ID); err != nil {
		t.Fatal(err)
	}
	prof := s.AddProfile(Profile{Name: "Keep two", RequiredCopies: 2})
	if err := s.SetAssignment(coll.ID, "sub", prof.ID); err != nil {
		t.Fatal(err)
	}
	// A rescan after a change gives the file a second version.
	src := s.FoldersOf(coll.ID)[0].Path
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("alpha, changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.ScanFolder(coll.ID, src, func(float64, string) {}); err != nil {
		t.Fatal(err)
	}
	s.Log("test", "a record of the v8 era")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Opened and closed once more, as a user's folder would have been.
	again, err := OpenStore(app.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(app.DataDir, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c catalog
	if err := decodeCatalogJSON(raw, &c); err != nil || c.SchemaVersion != 8 {
		t.Fatalf("the folder's catalog.json is schema v%d (%v), want v8", c.SchemaVersion, err)
	}
	jobsRaw, err := os.ReadFile(filepath.Join(app.DataDir, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	return v8Folder{dir: app.DataDir, catalogRaw: raw, jobsRaw: jobsRaw}
}

// assertEveryRecordKept proves the catalog database in dir holds every record of
// the v8 catalog.json raw, unchanged, and that the bytes of raw are kept beside
// it. Only the audit log may have grown (entries the app added since), and only
// at its end.
func assertEveryRecordKept(t testing.TB, dir string, raw []byte) {
	t.Helper()
	d, err := openCatalogDB(filepath.Join(dir, catalogDBName), false)
	if err != nil {
		t.Fatalf("open the catalog database: %v", err)
	}
	got, err := exportCatalogDB(d)
	d.Close()
	if err != nil {
		t.Fatal(err)
	}
	var want catalog
	if err := decodeCatalogJSON(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(got.Audit) < len(want.Audit) || !reflect.DeepEqual(canonAudit(got.Audit[:len(want.Audit)]), canonAudit(want.Audit)) {
		t.Fatalf("the audit log was not kept: %d entries before, %d after", len(want.Audit), len(got.Audit))
	}
	got.Audit = want.Audit
	canonCatalog(&want)
	canonCatalog(got)
	ws, gs := catalogSectionDigests(&want), catalogSectionDigests(got)
	for k := range ws {
		if ws[k] != gs[k] {
			wb, _ := json.Marshal(reflect.ValueOf(want).FieldByNameFunc(func(n string) bool { return jsonName(n) == k }).Interface())
			t.Errorf("section %s changed in the upgrade:\nbefore %s", k, wb)
		}
	}
	kept, _ := filepath.Glob(filepath.Join(dir, catalogPreSQLite+"*"))
	if len(kept) != 1 {
		t.Fatalf("kept copies of catalog.json: %v", kept)
	}
	if b, err := os.ReadFile(kept[0]); err != nil || !bytes.Equal(b, raw) {
		t.Fatalf("the kept catalog.json is not the original bytes (%v)", err)
	}
	stub, err := os.ReadFile(filepath.Join(dir, "catalog.json"))
	if err != nil || !isCatalogStub(stub) {
		t.Fatalf("catalog.json is not the stub after the upgrade (%v)", err)
	}
}

func canonAudit(a []Audit) []Audit {
	c := catalog{Audit: append([]Audit(nil), a...)}
	canonCatalog(&c)
	return c.Audit
}

// jsonName is the catalog field's JSON name, for messages.
func jsonName(field string) string {
	f, ok := reflect.TypeOf(catalog{}).FieldByName(field)
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

// assertJobsKept proves every job of the v8 jobs.json is on the board unchanged.
func assertJobsKept(t *testing.T, jobsRaw []byte, board []Job) {
	t.Helper()
	var file struct {
		Jobs []Job `json:"rows"`
	}
	if err := json.Unmarshal(jobsRaw, &file); err != nil {
		t.Fatalf("read jobs.json: %v", err)
	}
	if len(file.Jobs) == 0 {
		t.Fatal("the v8 folder has no jobs to keep")
	}
	byID := map[int]Job{}
	for _, j := range board {
		byID[j.ID] = j
	}
	for _, want := range file.Jobs {
		got, ok := byID[want.ID]
		wb, _ := json.Marshal(want)
		gb, _ := json.Marshal(got)
		if !ok || !bytes.Equal(wb, gb) {
			t.Errorf("job %d was not kept:\nbefore %s\nafter  %s", want.ID, wb, gb)
		}
	}
}

func apiGet(h http.Handler, path string) (int, string) {
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// The first start on the default engine moves a v8 catalog.json into the catalog
// database as a job the API shows while it runs, refuses every other request
// meanwhile, and then opens on the database with every record and job kept and
// the upgrade recorded on the board.
func TestCatalogUpgrade_RunsAsAVisibleJobAndKeepsEveryRecord(t *testing.T) {
	f := makeV8DataFolder(t)
	t.Setenv(catalogEngineEnv, "") // the default engine
	if need, err := catalogUpgradeNeeded(f.dir); err != nil || !need {
		t.Fatalf("upgrade needed: %v %v", need, err)
	}
	app := &App{DataDir: f.dir}
	paused, resume := make(chan struct{}), make(chan struct{})
	app.upgradeStep = func(step string) {
		if step == "checking" {
			close(paused)
			<-resume
		}
	}
	done := app.beginCatalogUpgrade()
	h := swapTestHandler(app)
	<-paused

	code, body := apiGet(h, "/api/jobs")
	var jobs []Job
	if err := json.Unmarshal([]byte(body), &jobs); code != 200 || err != nil || len(jobs) != 1 {
		t.Fatalf("GET /api/jobs during the upgrade: %d %s", code, body)
	}
	if j := jobs[0]; j.Kind != catalogUpgradeJobKind || j.Status != "RUNNING" || j.Progress < 0.75 || !strings.Contains(j.Label, "checking") {
		t.Fatalf("the upgrade job during its check: %+v", j)
	}
	code, body = apiGet(h, "/api/health")
	if code != 200 || !strings.Contains(body, `"upgrading":true`) {
		t.Fatalf("GET /api/health during the upgrade: %d %s", code, body)
	}
	for _, req := range []struct{ method, path string }{
		{"POST", "/api/locations"}, {"GET", "/api/home"}, {"POST", "/api/destructive/execute"}, {"POST", "/api/data-migration"},
	} {
		var code int
		if req.method == "GET" {
			code, body = apiGet(h, req.path)
		} else {
			code, body = apiPost(h, req.path, `{"name":"x"}`)
		}
		if code != http.StatusServiceUnavailable || !strings.Contains(body, "being upgraded") {
			t.Errorf("%s %s during the upgrade: %d %s", req.method, req.path, code, body)
		}
	}
	if err := app.maybeAutoExport(time.Now()); err != nil {
		t.Errorf("the automatic backup during the upgrade: %v", err)
	}
	if _, err := app.beginSwap("a restore"); err == nil {
		t.Error("a restore could start during the upgrade")
	}
	close(resume)
	<-done

	if app.Store == nil || !app.Store.UsesDatabase() {
		t.Fatal("the app is not on the catalog database after the upgrade")
	}
	t.Cleanup(func() { app.Store.Close() })
	if err := app.Store.FlushPending(); err != nil {
		t.Fatal(err)
	}
	assertEveryRecordKept(t, f.dir, f.catalogRaw)
	board := app.Store.Jobs()
	var flat []Job
	var upgrade *Job
	for _, j := range board {
		flat = append(flat, *j)
		if j.Kind == catalogUpgradeJobKind {
			upgrade = j
		}
	}
	assertJobsKept(t, f.jobsRaw, flat)
	if upgrade == nil || upgrade.Status != "COMPLETED" || upgrade.Result["kept_catalog_json"] == nil || upgrade.CreatedAt.IsZero() {
		t.Fatalf("the upgrade on the board: %+v", upgrade)
	}
	if code, body := apiPost(h, "/api/locations", `{"name":"After the upgrade"}`); code != 200 {
		t.Fatalf("an edit after the upgrade: %d %s", code, body)
	}
	if code, body := apiGet(h, "/api/health"); code != 200 || strings.Contains(body, "upgrad") {
		t.Fatalf("GET /api/health after the upgrade: %d %s", code, body)
	}
}

// A refused upgrade changes nothing: catalog.json stays the catalog, byte for
// byte, no database or stage is left, and the job and every request say why.
func TestCatalogUpgrade_ARefusedUpgradeChangesNothing(t *testing.T) {
	f := makeV8DataFolder(t)
	bad := append([]byte(`{"a_key_no_build_knows": 1, `), bytes.TrimPrefix(bytes.TrimSpace(f.catalogRaw), []byte("{"))...)
	if err := os.WriteFile(filepath.Join(f.dir, "catalog.json"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	before := dirNames(t, f.dir)
	t.Setenv(catalogEngineEnv, "")
	app := &App{DataDir: f.dir}
	<-app.beginCatalogUpgrade()
	h := swapTestHandler(app)
	if app.Store != nil {
		t.Fatal("the app opened a store after a refused upgrade")
	}
	code, body := apiGet(h, "/api/jobs")
	if code != 200 || !strings.Contains(body, `"status":"FAILED"`) || !strings.Contains(body, "a_key_no_build_knows") {
		t.Fatalf("GET /api/jobs after a refused upgrade: %d %s", code, body)
	}
	code, body = apiPost(h, "/api/locations", `{"name":"x"}`)
	if code != http.StatusServiceUnavailable || !strings.Contains(body, "could not be upgraded") {
		t.Fatalf("an edit after a refused upgrade: %d %s", code, body)
	}
	if b, err := os.ReadFile(filepath.Join(f.dir, "catalog.json")); err != nil || !bytes.Equal(b, bad) {
		t.Fatalf("catalog.json changed (%v)", err)
	}
	if after := dirNames(t, f.dir); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("the data folder changed:\nbefore %v\nafter  %v", before, after)
	}
	if err := app.Shutdown(); err != nil {
		t.Fatalf("shutting down after a refused upgrade: %v", err)
	}
}

// The default engine is the catalog database; catalog.json is chosen by name.
func TestCatalogEngine_TheDatabaseIsTheDefault(t *testing.T) {
	t.Setenv(catalogEngineEnv, "")
	if e, err := catalogEngine(); err != nil || e != "sqlite" {
		t.Fatalf("default engine %q (%v)", e, err)
	}
	t.Setenv(catalogEngineEnv, "json")
	if e, err := catalogEngine(); err != nil || e != "json" {
		t.Fatalf("json engine %q (%v)", e, err)
	}
	dir := t.TempDir()
	t.Setenv(catalogEngineEnv, "")
	s, err := openTestStore(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !s.UsesDatabase() {
		t.Fatal("a new data folder on the default engine is not a catalog database")
	}
	if _, err := os.Stat(filepath.Join(dir, "catalog.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a new catalog database folder has a catalog.json (%v)", err)
	}
}

// The portable catalog holds every record, pending ones too, on either engine,
// and opens as a catalog on either engine.
func TestPortableCatalog_HoldsEveryRecordOnBothEngines(t *testing.T) {
	for _, engine := range []string{"json", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			t.Setenv(catalogEngineEnv, engine)
			app := abTestApp(t)
			abPopulate(t, app)
			holdBatch(app.Store)
			app.Store.AddCollection("Pending")
			res, err := app.ExportPortableCatalog(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(res.Path)
			if err != nil || sha256Hex(raw) != res.SHA256 || int64(len(raw)) != res.Bytes {
				t.Fatalf("the export at %s does not match its result (%v)", res.Path, err)
			}
			if err := checkCatalogJSONKeys(raw); err != nil {
				t.Fatalf("the export has keys a catalog does not: %v", err)
			}
			for _, open := range []string{"json", "sqlite"} {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "catalog.json"), raw, 0o644); err != nil {
					t.Fatal(err)
				}
				s := openEngineStore(t, open, dir)
				if got := strings.Join(collectionNames(s), ","); got != "Backup Me,Pending" {
					t.Errorf("opened on %s, the export holds archives %s", open, got)
				}
				if n := len(testFiles(t, s.AllFiles())); n != 2 {
					t.Errorf("opened on %s, the export holds %d files", open, n)
				}
			}
		})
	}
}

// failRecorder is a testing.TB that records a failure instead of failing.
type failRecorder struct {
	testing.TB
	failed bool
	msg    string
}

func (r *failRecorder) Helper() {}
func (r *failRecorder) Errorf(f string, a ...any) {
	r.failed, r.msg = true, fmt.Sprintf(f, a...)
}
func (r *failRecorder) Fatalf(f string, a ...any) {
	r.Errorf(f, a...)
	runtime.Goexit()
}

// The record check catches a changed record, a lost one and an audit entry
// rewritten, so a passing upgrade test means something.
func TestAssertEveryRecordKept_CatchesAChange(t *testing.T) {
	f := makeV8DataFolder(t)
	if _, err := migrateCatalogToDB(f.dir, catalogMigrateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertEveryRecordKept(t, f.dir, f.catalogRaw)
	for name, edit := range map[string]func(string) string{
		"a changed record": func(s string) string { return strings.Replace(s, "SERIAL-XYZ-123", "SERIAL-XYZ-124", 1) },
		"a changed flag":   func(s string) string { return strings.Replace(s, `"offsite": true`, `"offsite": false`, 1) },
		"a lost record": func(s string) string {
			return strings.Replace(s, `"locations": [`, `"locations": [{"id": 99, "name": "Lost"},`, 1)
		},
		"an audit entry": func(s string) string {
			return strings.Replace(s, "a record of the v8 era", "a record of another era", 1)
		},
	} {
		changed := edit(string(f.catalogRaw))
		if changed == string(f.catalogRaw) {
			t.Fatalf("%s: the edit found nothing to change", name)
		}
		r := &failRecorder{TB: t}
		done := make(chan struct{})
		go func() {
			defer close(done)
			assertEveryRecordKept(r, f.dir, []byte(changed))
		}()
		<-done
		if !r.failed {
			t.Errorf("%s was not caught", name)
		}
	}
}
