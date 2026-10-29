package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain sets two test-only rules: each folder counts as its own device for
// the keystore rule, because every test keystore is on the one temp drive
// (tests of the device rule itself switch the real lookup back on); and no
// verify looks up a drive's serial.
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
	// No test probes a real disk for its serial when crediting a verify; the
	// serial rule is tested directly (TestCreditCopyCheck_Serial).
	driveSerialAt = func(string) string { return "" }
	os.Exit(m.Run())
}
