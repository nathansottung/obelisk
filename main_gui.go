//go:build guionly

package main

// main_gui.go — the launcher-only entry point, built with -tags guionly.
//
// This build contains only the two modes the Windows developer package runs:
// --gui-disposable-inventory and --gui-catalog-readonly. main.go (the HTTP
// server, embedded UI and API routes) is excluded by its build constraint, so
// none of that code is compiled into this binary. Every other argument is
// refused.
//
// The small helpers below are exact copies of main.go's versions, needed because
// other files in the package reference them. TestGUIOnlyHelpers_MatchMainGo fails
// if a copy drifts from main.go.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var appVersion = "0.9.0-dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--gui-disposable-inventory":
			if err := runGUIInventory(os.Args[2:], os.Stdout, os.Stderr); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		case "--gui-catalog-readonly":
			if err := runGUICatalog(os.Args[2:], os.Stdin, os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Obelisk %s: this build contains only --gui-disposable-inventory and --gui-catalog-readonly; use Launch.cmd\n", appVersion)
	os.Exit(2)
}

// ---- exact copies of main.go helpers (see the file comment) ----

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func s(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}

func f(m map[string]any, k string) float64 {
	v, _ := m[k].(float64)
	return v
}

func bl(m map[string]any, k string) bool {
	v, _ := m[k].(bool)
	return v
}

func pathFree(p string) (int64, error) {
	for {
		if _, err := os.Stat(p); err == nil {
			return diskFree(p)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return diskFree(p)
		}
		p = parent
	}
}

func progStats(bytesDone, bytesTotal, filesDone, filesTotal int64, human string) string {
	return fmt.Sprintf("\x1f%d\x1f%d\x1f%d\x1f%d\x1f%s", bytesDone, bytesTotal, filesDone, filesTotal, human)
}

func progBytes(done, total int64, human string) string {
	return progStats(done, total, 0, 0, human)
}

func offsiteWord(off bool) string {
	if off {
		return "offsite"
	}
	return "onsite"
}

func (a *App) noteUnrecordedJob(id int, status string, cause error) {
	msg := fmt.Sprintf("job %d reached %s but its record could not be written: %v", id, status, cause)
	log.Print("jobs: " + msg)
	a.Store.Log("job-unrecorded", msg)
}
