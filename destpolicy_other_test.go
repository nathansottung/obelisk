//go:build !windows && !linux && !guionly

package main

import (
	"strings"
	"testing"
)

func TestDestPolicy_UnsupportedPlatformRefusesWithMessage(t *testing.T) {
	app := dockApp(t)
	d, err := app.Store.AuthorizeDestination(t.TempDir())
	if err == nil {
		d.Close()
		t.Fatal("a guarded write was authorized on an unsupported platform")
	}
	if !strings.Contains(err.Error(), "not yet supported") {
		t.Fatalf("refusal should say the platform is not yet supported: %v", err)
	}
}
