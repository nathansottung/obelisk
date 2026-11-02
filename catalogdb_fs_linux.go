//go:build linux && !guionly

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// catalogDBStatfs reports the filesystem magic number of dir. A variable so tests
// can stand in a network filesystem.
var catalogDBStatfs = func(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Type), nil
}

// catalogDBNetworkMagic names the network filesystems a catalog database must not
// live on. FUSE is not listed: it also carries local filesystems and the bind
// mounts of Docker Desktop.
var catalogDBNetworkMagic = map[int64]string{
	0x6969:     "NFS",
	0x517B:     "SMB",
	0xFF534D42: "CIFS",
	0xFE534D42: "SMB2",
	0x5346414F: "AFS",
	0x73757245: "Coda",
	0x01021997: "9P",
	0x00C36400: "Ceph",
	0x564C:     "NCP",
}

// checkCatalogDBLocal refuses a data folder on a network filesystem.
func checkCatalogDBLocal(dir string) error {
	magic, err := catalogDBStatfs(dir)
	if err != nil {
		return fmt.Errorf("the folder for the catalog database cannot be checked: %w", err)
	}
	return catalogDBNetworkRefusal(dir+"/catalog.db", catalogDBNetworkMagic[magic])
}
