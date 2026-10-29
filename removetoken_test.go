//go:build !guionly

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Settings can remove the stored access token: the token leaves the settings
// file, other settings are kept, and the answer says when it takes effect.
func TestSettings_RemoveToken(t *testing.T) {
	app := dockApp(t)
	cfg := mustConfig(t, app)
	cfg.AuthToken, cfg.BufferGB = "remove-me-0123456789", 3
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	api(mux, app)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/config/remove-token", strings.NewReader("{}")))
	if rec.Code != 200 {
		t.Fatalf("remove token answered %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["removed"] != true || !strings.Contains(strings.ToLower(out["note"].(string)), "next") {
		t.Errorf("the answer does not say the token was removed and when it takes effect: %v", out)
	}
	if b, _ := os.ReadFile(app.configPath()); strings.Contains(string(b), "remove-me-0123456789") {
		t.Fatal("the token is still in the settings file")
	}
	if got := mustConfig(t, app); got.AuthToken != "" || got.BufferGB != 3 {
		t.Fatalf("after removal: token %q, buffer_gb %v (other settings must be kept)", got.AuthToken, got.BufferGB)
	}
}
