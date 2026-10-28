package main

import "testing"

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
