//go:build !windows && !linux

package main

// destauth_other.go — identity, and a weaker canonical location, on macOS and the
// BSDs. Writes through the destination guard are refused on these platforms (see
// destpolicy_other.go); this file exists so that source registration and the
// overlap decision still compile and behave sensibly there.
//
// The location is resolved by NAME with filepath.EvalSymlinks and then checked
// against the descriptor actually held: if the two identities disagree, the name
// moved between resolving and opening, and the answer is a refusal. normPath folds
// case only on Windows, so two spellings of one path on a case-insensitive macOS
// volume can still compare unequal.

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// handleCanonicalPath returns where the object behind f lives, resolved by name
// and then verified against the identity of the descriptor actually held.
func handleCanonicalPath(f *os.File) (string, error) {
	held, err := handleIdentity(f)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(f.Name())
	if err != nil {
		return "", fmt.Errorf("cannot resolve the real location of %s: %w", f.Name(), err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot resolve the real location of %s: %w", f.Name(), err)
	}
	probe, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot confirm the real location of %s: %w", f.Name(), err)
	}
	defer probe.Close()
	named, err := handleIdentity(probe)
	if err != nil {
		return "", err
	}
	if named != held {
		return "", fmt.Errorf("refusing: %s no longer names the folder that was opened (it now resolves to %s)", f.Name(), resolved)
	}
	return filepath.Clean(resolved), nil
}

// fileIdent is a filesystem object's identity, taken from an OPEN DESCRIPTOR.
type fileIdent struct {
	vol uint64
	idx uint64
}

// handleIdentity reports the identity of the object behind f, via fstat on the
// descriptor — device plus inode, forced at the moment of the call.
func handleIdentity(f *os.File) (fileIdent, error) {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		return fileIdent{}, fmt.Errorf("fstat failed for %s: %w", f.Name(), err)
	}
	return fileIdent{vol: uint64(st.Dev), idx: uint64(st.Ino)}, nil
}

// volumeIDOf records no volume identity here; the overlap decision falls back to
// comparing paths for a source whose drive is not attached.
func volumeIDOf(*os.File) (string, error) { return "", nil }
