//go:build linux

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Filesystem magic numbers x/sys does not name on every architecture.
const (
	destMagicMSDOS = 0x4d44     // vfat, msdos
	destMagicExFAT = 0x2011bab0 // exfat
)

// destFilesystem describes the filesystem behind f and whether Obelisk accepts it
// as a destination. ext2/3/4, XFS, Btrfs and tmpfs get every check. FAT and exFAT
// are accepted, recording that their owner and mode bits come from mount options
// rather than the disk. A container overlay is refused with a pointer to the fix,
// and network, FUSE and other filesystems are refused.
func destFilesystem(f *os.File) (destFS, bool, error) {
	fsType, err := destStatfsType(f)
	if err != nil {
		return destFS{}, false, err
	}
	switch fsType {
	case unix.EXT4_SUPER_MAGIC:
		return destFS{name: "ext4"}, true, nil
	case unix.XFS_SUPER_MAGIC:
		return destFS{name: "XFS"}, true, nil
	case unix.BTRFS_SUPER_MAGIC:
		return destFS{name: "Btrfs"}, true, nil
	case unix.TMPFS_MAGIC:
		return destFS{name: "tmpfs"}, true, nil
	case destMagicMSDOS:
		return destFS{name: "FAT", maxFile: 1<<32 - 1, notChecked: fatNotChecked()}, false, nil
	case destMagicExFAT:
		return destFS{name: "exFAT", notChecked: fatNotChecked()}, false, nil
	case unix.OVERLAYFS_SUPER_MAGIC:
		return destFS{}, false, fmt.Errorf("unsupported destination: this folder is on a container overlay filesystem (for example inside Docker). Obelisk will not write there; mount a host folder into the container (for example -v /mnt/backup:/backup) and choose that as the destination")
	}
	return destFS{}, false, fmt.Errorf("unsupported destination filesystem %#x: use a local ext4, XFS, Btrfs, exFAT or FAT drive", fsType)
}

// destStatfsType reports the filesystem magic number behind f. A variable so tests
// can present filesystems that cannot be mounted without root.
var destStatfsType = func(f *os.File) (int64, error) {
	var st unix.Statfs_t
	err := unix.Fstatfs(int(f.Fd()), &st)
	runtime.KeepAlive(f)
	return int64(st.Type), err
}

func platformDestName(rel string) error {
	if strings.HasPrefix(rel, `\\`) || strings.ContainsRune(rel, 0) {
		return fmt.Errorf("unsupported rooted or NUL name")
	}
	return nil
}

func fatNotChecked() []string {
	return []string{destCheckPermissions, destCheckHardLinks, destCheckFileIDs}
}

func trustedDestOwner(uid uint32) bool { return uid == 0 || uid == uint32(os.Geteuid()) }

// checkDestNamespace walks actual directory descriptors, including ancestors
// outside the root. POSIX access ACL write grants are bounded by the group-class
// mode mask on the admitted filesystems. A sticky ancestor such as /tmp is safe
// only because the child and ancestor owners are trusted. Publication parents must
// not be shared. Owner and mode are not checked where the filesystem emulates them;
// that is returned as a finding the user decides about.
func checkDestNamespace(r *os.Root) (destFS, error) {
	f, err := r.Open(".")
	if err != nil {
		return destFS{}, err
	}
	defer func() { f.Close() }()
	var dest destFS
	var chain []destLevel
	leaf := true
	for {
		fs, perms, err := destFilesystem(f)
		if err != nil {
			return destFS{}, err
		}
		var st unix.Stat_t
		if err := unix.Fstat(int(f.Fd()), &st); err != nil {
			return destFS{}, err
		}
		canon, _ := handleCanonicalPath(f)
		if leaf {
			dest = fs
			if len(fs.notChecked) > 0 {
				dest.findings = append(dest.findings, destFinding{Kind: destFindingUnchecked, Folder: canon, FS: fs.name, MaxFile: fs.maxFile})
			}
		}
		if perms && (!trustedDestOwner(st.Uid) || st.Mode&0o022 != 0 && (leaf || st.Mode&unix.S_ISVTX == 0)) {
			return destFS{}, fmt.Errorf("unsafe destination namespace: directory owner %d or mode %#o permits untrusted entry replacement", st.Uid, st.Mode&0o7777)
		}
		chain = append(chain, destLevel{path: canon, fs: fs.name, perms: fmt.Sprintf("dev=%d ino=%d uid=%d mode=%o", st.Dev, st.Ino, st.Uid, st.Mode&0o7777)})
		fd, err := unix.Openat(int(f.Fd()), "..", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		runtime.KeepAlive(f)
		if err != nil {
			return destFS{}, err
		}
		parent := os.NewFile(uintptr(fd), "destination ancestor")
		var ps unix.Stat_t
		if err := unix.Fstat(fd, &ps); err != nil {
			parent.Close()
			return destFS{}, err
		}
		if ps.Dev == st.Dev && ps.Ino == st.Ino {
			parent.Close()
			dest.chain = chain
			return dest, nil
		}
		f.Close()
		f, leaf = parent, false
	}
}

// predictDestChild has nothing to predict on Linux: a folder Obelisk creates is
// owned by this user with mode 0755 less the umask, and inherits no grants.
func predictDestChild(*os.Root, string, int) ([]destFinding, error) { return nil, nil }

// destStagePermsEnforced reports whether the filesystem behind f keeps real owner
// and mode bits, so the private staging directory's 0700 can be relied on.
func destStagePermsEnforced(f *os.File) bool {
	_, perms, err := destFilesystem(f)
	return err == nil && perms
}

// A cooperative, crash-released lease on this mount namespace's root inode.
// All converted jobs serialize, even across volumes and pre-existing bind aliases.
// flock is coordination between Obelisk jobs, never adversarial isolation.
func acquireDestLease(r *os.Root) (func() error, error) {
	f, err := os.Open("/")
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	observed := false
	for {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		runtime.KeepAlive(f)
		if err == nil {
			return f.Close, nil
		}
		if err != unix.EWOULDBLOCK || time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("destination filesystem is busy or cannot coordinate Obelisk publication: %w", err)
		}
		if !observed {
			destLeaseContended()
			observed = true
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// destGuardPlatform: the destination guard can authorize folders here.
const destGuardPlatform = true
