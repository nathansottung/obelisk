package main

import "testing"

// readyTestKey creates a shared key and proves it from its password-manager
// note, as a user does on the Keys page, so encrypted builds can run.
func readyTestKey(t *testing.T, app *App) string {
	t.Helper()
	st, err := app.CreateKey("", keyPurposeShared, 0)
	if err != nil {
		t.Fatalf("create the test key: %v", err)
	}
	out, err := app.RevealKey(st.Ref, "password-manager")
	if err != nil {
		t.Fatalf("reveal the test key: %v", err)
	}
	res, err := app.ProveKey(st.Ref, out["block"].(string), "password-manager")
	if err != nil || !res.OK {
		t.Fatalf("prove the test key: %v %+v", err, res)
	}
	return st.Ref
}
