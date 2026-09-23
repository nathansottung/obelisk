//go:build guionly

package main

// main_gui.go — the launcher-only entry point, built with -tags guionly.
//
// This build contains only the two modes the Windows developer package runs:
// --gui-disposable-inventory and --gui-catalog-readonly. main.go (the HTTP
// server, embedded UI and API routes) is excluded by its build constraint, so
// none of that code is compiled into this binary. Every other argument is
// refused. Helpers that other files need live in app_helpers.go, shared with
// the full build.

import (
	"fmt"
	"os"
)

// appVersion is set only by -X main.appVersion at build time. It has no literal
// default, so no unused version string is left in the binary; an unstamped
// build reports "unstamped".
var appVersion string

func main() {
	if appVersion == "" {
		appVersion = "unstamped"
	}
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
