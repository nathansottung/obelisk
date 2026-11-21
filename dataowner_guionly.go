//go:build guionly

package main

import (
	"errors"
	"os"
)

// The launcher-only build never opens a data folder (main_gui.go). Should its
// shared code ever try, the owner lock refuses rather than open one unowned.
func lockFileExclusive(*os.File) error {
	return errors.New("this build of Obelisk does not open data folders")
}

func unlockFile(*os.File) error { return nil }
