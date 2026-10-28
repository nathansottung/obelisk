//go:build !windows && !linux

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// destUnsupportedPlatform is the refusal for every guarded write on macOS and the
// BSDs. Their ACLs are not bounded by the POSIX mode mask, so 0700 is not evidence
// that no one else can change a folder, and the native checks are not written yet.
func destUnsupportedPlatform() error {
	return fmt.Errorf("mirror, backup and plan jobs are not yet supported on %s: Obelisk cannot yet confirm that no one else can change the destination folder there, so it refuses rather than guess. Run them from Windows or Linux for now (a restore can go into a new folder)", runtime.GOOS)
}

// destGuardPlatform: the destination guard cannot authorize a folder here.
const destGuardPlatform = false

func platformDestName(rel string) error {
	if strings.HasPrefix(rel, `\\`) || strings.ContainsRune(rel, 0) {
		return fmt.Errorf("unsupported rooted or NUL name")
	}
	return nil
}

func checkDestNamespace(*os.Root) (destFS, error) { return destFS{}, destUnsupportedPlatform() }

func acquireDestLease(*os.Root) (func() error, error) { return nil, destUnsupportedPlatform() }

func destStagePermsEnforced(*os.File) bool { return true }

func predictDestChild(*os.Root, string, int) ([]destFinding, error) {
	return nil, destUnsupportedPlatform()
}
