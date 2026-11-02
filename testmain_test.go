package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain sets two test-only rules, and checks one: each folder counts as its own device for
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
	// serial rule is tested directly (TestCreditCopyCheck_Serial), and so is the
	// partition rule (partition_identity_test.go).
	driveSerialAt = func(string) string { return "" }
	partitionIDAt = func(string) string { return "" }
	// Every file copy a Store method hands out is recorded; one that a test run
	// changed without passing it back through a method fails the run (see
	// fileCopyGuard). The limit bounds the record on the large-catalog benchmarks.
	fileCopyGuard = newFileGuard(500_000)
	code := m.Run()
	if v := fileCopyGuard.violations(); len(v) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d file record(s) were changed without a Store method:\n", len(v))
		for i, line := range v {
			if i == 20 {
				fmt.Fprintf(os.Stderr, "  … and %d more\n", len(v)-20)
				break
			}
			fmt.Fprintln(os.Stderr, "  "+line)
		}
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
