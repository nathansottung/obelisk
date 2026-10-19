//go:build windows && !guionly

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func postJSON(t *testing.T, mux *http.ServeMux, path string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(string(b))))
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// The server refuses to start a mirror into a folder that needs a choice, returns
// the notice, and starts it once the choice has been made with the typed word.
func TestDestChoiceAPI_MirrorWaitsForChoice(t *testing.T) {
	app, collID, _, vol := mirrorSetup(t, "API", map[string]string{"a.txt": "alpha"})
	folder, _ := sharedFolder(t)
	mux := http.NewServeMux()
	api(mux, app)
	mirror := map[string]any{"collection_id": collID, "targets": []any{map[string]any{"dest_dir": folder, "volume_id": vol.ID}}}

	code, out := postJSON(t, mux, "/api/mirror", mirror)
	chk, _ := out["destination_check"].(map[string]any)
	if code != 409 || chk == nil || chk["status"] != destStatusChoose || chk["notice"] == nil {
		t.Fatalf("mirror before a choice: %d %v", code, out)
	}
	if _, ok := out["jobs"]; ok {
		t.Fatal("a job was started before the choice")
	}
	if got := entriesOf(t, folder); len(got) != 0 {
		t.Fatalf("written before the choice: %q", got)
	}
	fp := fmt.Sprint(chk["fingerprint"])

	if code, _ := postJSON(t, mux, "/api/destinations/use-anyway", map[string]any{"path": folder, "fingerprint": fp, "typed": "ok"}); code != 400 {
		t.Fatalf("use anyway without the word: %d", code)
	}
	if code, out := postJSON(t, mux, "/api/destinations/use-anyway", map[string]any{"path": folder, "fingerprint": fp, "typed": "Continue"}); code != 200 || out["status"] != destStatusChosen {
		t.Fatalf("use anyway: %d %v", code, out)
	}
	if code, out := postJSON(t, mux, "/api/destinations/check", map[string]any{"path": folder}); code != 200 || out["status"] != destStatusChosen {
		t.Fatalf("check after the choice: %d %v", code, out)
	}
	code, out = postJSON(t, mux, "/api/mirror", mirror)
	jobs, _ := out["jobs"].([]any)
	if code != 200 || len(jobs) != 1 {
		t.Fatalf("mirror after the choice: %d %v", code, out)
	}
	id := int(jobs[0].(map[string]any)["job_id"].(float64))
	deadline := time.Now().Add(30 * time.Second)
	for {
		j := app.Store.Job(id)
		if j != nil && j.Status != "RUNNING" {
			if j.Status != "COMPLETED" {
				t.Fatalf("mirror job ended %s: %v", j.Status, j.Result)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mirror job did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	mustBytes(t, filepath.Join(folder, "a.txt"), "alpha", "mirror after the choice")
}

// Make private through the API: the preview changes nothing, apply needs the
// confirm flag and the token, and afterwards the folder passes.
func TestDestChoiceAPI_MakePrivate(t *testing.T) {
	app := dockApp(t)
	folder, _ := sharedFolder(t)
	mux := http.NewServeMux()
	api(mux, app)
	before := folderSDDLOf(t, folder)
	code, plan := postJSON(t, mux, "/api/destinations/make-private/preview", map[string]any{"path": folder})
	if code != 200 || plan["token"] == "" {
		t.Fatalf("preview: %d %v", code, plan)
	}
	if folderSDDLOf(t, folder) != before {
		t.Fatal("the preview changed permissions")
	}
	if code, _ := postJSON(t, mux, "/api/destinations/make-private/apply", map[string]any{"path": folder, "token": plan["token"]}); code != 400 {
		t.Fatalf("apply without confirm: %d", code)
	}
	if folderSDDLOf(t, folder) != before {
		t.Fatal("an unconfirmed apply changed permissions")
	}
	_, plan = postJSON(t, mux, "/api/destinations/make-private/preview", map[string]any{"path": folder})
	code, out := postJSON(t, mux, "/api/destinations/make-private/apply", map[string]any{"path": folder, "token": plan["token"], "confirm": true})
	if code != 200 || out["status"] != destStatusPass {
		t.Fatalf("apply: %d %v", code, out)
	}
}
