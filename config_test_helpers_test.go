package main

import "testing"

func initializedTestApp(t *testing.T, a *App) *App {
	t.Helper()
	if _, err := a.InitializeConfig(); err != nil {
		t.Fatal("fixture configuration initialization failed")
	}
	// The store the app ends with is closed before the test's folders are removed:
	// a restore or a data folder move opens one of its own.
	t.Cleanup(func() {
		if a.Store != nil {
			a.Store.Close()
		}
	})
	return a
}
func mustConfig(t *testing.T, a *App) Config {
	t.Helper()
	cfg, err := a.LoadConfig()
	if err != nil {
		t.Fatal("fixture configuration read failed")
	}
	return cfg
}

// testValue adapts existing single-value assertions without dropping read errors.
func testValue[T any](v T, err error) T {
	if err != nil {
		panic("fixture read failed")
	}
	return v
}
