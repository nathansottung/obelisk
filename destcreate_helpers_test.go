package main

import (
	"errors"
	"strings"
	"testing"
)

// checkCreatingForTest checks path and, when it does not exist yet, confirms
// "Create <folder>" as the user would, returning the check that follows. For a
// destination that exists (or is refused) it is just the check.
func checkCreatingForTest(t *testing.T, app *App, path string) *DestinationCheck {
	t.Helper()
	c := app.Store.CheckDestination(path)
	if c.Status != destStatusCreate {
		return c
	}
	got, err := app.Store.CreateDestination(path, c.Folder, true)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	return got
}

// createdForTest is checkCreatingForTest for a call site that wants the path.
func createdForTest(t *testing.T, app *App, path string) string {
	t.Helper()
	checkCreatingForTest(t, app, path)
	return path
}

// refusedForTest asserts that path is refused the way the user would meet it:
// checked, created when the check offers that (so the creation itself is tried),
// and checked again. It returns the refusal; a destination that is not refused is
// reported as a test error and returns nil.
func refusedForTest(t *testing.T, app *App, path string) error {
	t.Helper()
	c := app.Store.CheckDestination(path)
	if c.Status == destStatusCreate {
		if _, err := app.Store.CreateDestination(path, c.Folder, true); err != nil {
			return err
		}
		c = app.Store.CheckDestination(path)
	}
	if c.Status != destStatusRefuse {
		t.Errorf("%s: %q, want a refusal", path, c.Status)
		return nil
	}
	return errors.New(c.Reason)
}

// assertSourceRefusal requires err to be the refusal of a destination that is,
// is inside, or contains a registered source. Any other error (a missing folder
// waiting for Create, say) would let a refusal test pass without testing it.
func assertSourceRefusal(t *testing.T, what string, err error) {
	t.Helper()
	var choice *DestinationChoiceError
	switch {
	case err == nil:
		t.Errorf("%s: not refused", what)
	case errors.As(err, &choice):
		t.Errorf("%s: refused only pending a choice (%q), want refused as a source", what, choice.Check.Status)
	case !strings.Contains(err.Error(), "source root"):
		t.Errorf("%s: refused for another reason, want refused as a source: %v", what, err)
	}
}
