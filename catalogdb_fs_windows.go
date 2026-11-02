//go:build windows && !guionly

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// checkCatalogDBLocal refuses a data folder on a network share or a mapped network
// drive. It asks the folder's own handle where it really is, so a link or junction
// that leads to a share is caught too.
func checkCatalogDBLocal(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("the folder for the catalog database cannot be opened: %w", err)
	}
	defer f.Close()
	canon, err := handleCanonicalPath(f)
	if err != nil {
		return fmt.Errorf("the folder for the catalog database cannot be located: %w", err)
	}
	var driveType uint32
	if !strings.HasPrefix(canon, `\`) {
		root, err := windows.UTF16PtrFromString(filepath.VolumeName(canon) + `\`)
		if err != nil {
			return err
		}
		driveType = windows.GetDriveType(root)
	}
	return catalogDBNetworkRefusal(filepath.Join(dir, "catalog.db"), catalogDBWindowsKind(canon, driveType))
}

// catalogDBWindowsKind names the network filesystem behind a folder's canonical
// path and its drive type, or returns "" for a local one.
func catalogDBWindowsKind(canon string, driveType uint32) string {
	switch {
	case strings.HasPrefix(canon, `\`):
		return "network share"
	case driveType == windows.DRIVE_REMOTE:
		return "network drive"
	}
	return ""
}
