package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain sets one test-only rule: each folder counts as its own device for
// the keystore rule, because every test keystore is on the one temp drive.
// Tests of the device rule itself switch the real lookup back on.
func TestMain(m *testing.M) {
	keystoreDeviceOf = func(path string) (string, error) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			return normPath(abs), nil
		}
		return normPath(filepath.Dir(abs)), nil
	}
	os.Exit(m.Run())
}
