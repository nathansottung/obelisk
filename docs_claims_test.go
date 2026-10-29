package main

import (
	"os"
	"strings"
	"testing"
)

// The status page records Slice C as merged, with its pull request.
func TestStatus_SliceCIsMerged(t *testing.T) {
	b, err := os.ReadFile("docs/STATUS.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "Slice C") {
			if !strings.Contains(line, "| Merged |") || !strings.Contains(line, "/pull/12") {
				t.Errorf("Slice C row is not recorded as merged in #12: %s", line)
			}
			return
		}
	}
	t.Error("docs/STATUS.md has no Slice C row")
}

// The dock is read-only toward the drive: its file header must not say it writes
// onto the drive.
func TestDockHeader_SaysNothingIsWrittenToTheDrive(t *testing.T) {
	b, err := os.ReadFile("dock.go")
	if err != nil {
		t.Fatal(err)
	}
	header := string(b)
	if i := strings.Index(header, "\nimport"); i >= 0 {
		header = header[:i]
	}
	if strings.Contains(strings.ToUpper(header), "ONTO THE DRIVE") || strings.Contains(header, "sidecar") {
		t.Error("dock.go's header says the dock writes onto the drive")
	}
}
