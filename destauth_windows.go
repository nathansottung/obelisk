//go:build windows

package main

// destauth_windows.go — handle-derived location and identity on Windows.
//
// Both answers come from the OPEN HANDLE, never from the pathname that was used to
// obtain it. That is what makes a junction, an 8.3 short name and an extended-length
// spelling all reduce to the same verdict as the plain path: the kernel is asked
// where the object it just handed us actually lives. This file uses the standard
// syscall package only.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modkernel32destauth        = syscall.NewLazyDLL("kernel32.dll")
	procGetFinalPathNameByHndl = modkernel32destauth.NewProc("GetFinalPathNameByHandleW")
)

// handleCanonicalPath returns where the object behind f actually lives, via
// GetFinalPathNameByHandle. The result is returned in the DOS-drive form with the
// extended-length prefix stripped, so it compares directly with a filepath.Abs
// spelling of the same location under normPath. A directory junction reports its
// target, and an 8.3 short name reports the long name.
func handleCanonicalPath(f *os.File) (string, error) {
	h := syscall.Handle(f.Fd())
	buf := make([]uint16, 1024)
	for {
		r, _, e := procGetFinalPathNameByHndl.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		n := uint32(r)
		if n == 0 {
			if e == nil {
				return "", fmt.Errorf("GetFinalPathNameByHandle failed for %s", f.Name())
			}
			return "", fmt.Errorf("GetFinalPathNameByHandle failed for %s: %w", f.Name(), e)
		}
		if int(n) < len(buf) {
			return stripExtendedPrefix(syscall.UTF16ToString(buf[:n])), nil
		}
		buf = make([]uint16, n+1)
	}
}

// stripExtendedPrefix removes the extended-length prefix so canonical names from a
// handle and absolute names built by filepath.Abs share one spelling. filepath.Abs
// leaves `\\?\` in place, so without this `\\?\C:\...\Source` would compare unequal
// to the registered `C:\...\Source` it names.
func stripExtendedPrefix(p string) string {
	switch {
	case strings.HasPrefix(p, `\\?\UNC\`):
		p = `\\` + p[len(`\\?\UNC\`):]
	case strings.HasPrefix(p, `\\?\`):
		p = p[len(`\\?\`):]
	}
	return filepath.Clean(p)
}

// fileIdent is a filesystem object's identity, taken from an OPEN HANDLE.
type fileIdent struct {
	vol uint64
	idx uint64
}

// handleIdentity reports the identity of the object behind f, from the handle.
//
// os.Stat is NOT equivalent here. On Windows the identity fields of a FileInfo are
// populated lazily, by reopening the name, so a FileInfo captured before a mutation
// and compared with os.SameFile afterwards can report "same file" about a different
// object. Reading the identity off the handle forces it at the moment of the call.
func handleIdentity(f *os.File) (fileIdent, error) {
	var bi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &bi); err != nil {
		return fileIdent{}, fmt.Errorf("GetFileInformationByHandle failed for %s: %w", f.Name(), err)
	}
	return fileIdent{
		vol: uint64(bi.VolumeSerialNumber),
		idx: uint64(bi.FileIndexHigh)<<32 | uint64(bi.FileIndexLow),
	}, nil
}

// volumeIDOf names the volume holding f by its serial number, read from the handle.
// It is what a source folder records so that a later run can tell "that drive is
// not attached" from "that folder was deleted".
func volumeIDOf(f *os.File) (string, error) {
	id, err := handleIdentity(f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("win-vsn:%08X", id.vol), nil
}
