//go:build windows && !guionly

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBatchForTest writes name in dir as a batch file that leaves a "ran"
// file next to itself, and returns that file's path.
func writeBatchForTest(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("@echo ran> \"%~dp0ran.txt\"\r\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "ran.txt")
}

// On Windows a program name with no extension also finds .bat and .cmd files,
// which run through cmd.exe and re-read their arguments as commands. The
// burner must be a program file, so a batch file is refused before it runs.
func TestBurnProgram_RefusesBatchFiles(t *testing.T) {
	onPath := t.TempDir()
	ranOnPath := writeBatchForTest(t, onPath, "xorriso.bat")
	t.Setenv("PATH", onPath+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := runBurnProgram([]string{"xorriso", "-version"}); err == nil {
		t.Error("xorriso.bat found on PATH ran as the burner")
	}
	if _, err := os.Stat(ranOnPath); err == nil {
		t.Error("xorriso.bat found on PATH was run")
	}

	byPath := t.TempDir()
	ranByPath := writeBatchForTest(t, byPath, "xorriso.cmd")
	if err := runBurnProgram([]string{filepath.Join(byPath, "xorriso"), "-version"}); err == nil {
		t.Error("a burner path with no extension ran xorriso.cmd")
	}
	if _, err := os.Stat(ranByPath); err == nil {
		t.Error("a burner path with no extension ran xorriso.cmd")
	}
}

// The same holds for the tools Obelisk runs: a configured path with no
// extension, or the PATH fallback, never resolves to a batch file.
func TestResolveConfigTool_RefusesBatchFiles(t *testing.T) {
	dir := t.TempDir()
	writeBatchForTest(t, dir, "par2.bat")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, cfg := range []Config{
		{Tools: map[string]string{}},
		{Tools: map[string]string{"par2": filepath.Join(dir, "par2")}},
	} {
		p, err := resolveConfigTool("par2", cfg)
		if err == nil && !strings.EqualFold(filepath.Ext(p), ".exe") {
			t.Errorf("tools %v: par2 resolved to %s, a batch file", cfg.Tools, p)
		}
	}
}
