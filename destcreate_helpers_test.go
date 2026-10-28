package main

import (
	"errors"
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
