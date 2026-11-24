package main

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// Every function a button names in data-call is one the page lets be called, or the
// button does nothing when clicked.
func TestUI_EveryDataCallIsAllowed(t *testing.T) {
	b, err := os.ReadFile("ui/index.html")
	if err != nil {
		t.Fatal(err)
	}
	set := regexp.MustCompile(`const UI_CALLS=new Set\(\[([^\]]*)\]\)`).FindSubmatch(b)
	if set == nil {
		t.Fatal("UI_CALLS not found")
	}
	allowed := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([A-Za-z0-9_]+)'`).FindAllSubmatch(set[1], -1) {
		allowed[string(m[1])] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`data-(?:input-)?call="([A-Za-z0-9_]+)"`).FindAllSubmatch(b, -1) {
		if n := string(m[1]); !allowed[n] && !seen[n] {
			seen[n] = true
			missing = append(missing, n)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("buttons call functions missing from UI_CALLS: %v", missing)
	}
}
