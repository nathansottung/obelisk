//go:build darwin && !guionly

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// catalogDBNetworkTypes names the macOS network filesystems a catalog database
// must not live on, by their mount type.
var catalogDBNetworkTypes = map[string]string{
	"nfs": "NFS", "smbfs": "SMB", "afpfs": "AFP", "webdav": "WebDAV", "cifs": "CIFS",
}

// checkCatalogDBLocal refuses a data folder on a network filesystem.
func checkCatalogDBLocal(dir string) error {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return fmt.Errorf("the folder for the catalog database cannot be checked: %w", err)
	}
	return catalogDBNetworkRefusal(dir+"/catalog.db", catalogDBNetworkTypes[unix.ByteSliceToString(st.Fstypename[:])])
}
