//go:build !windows

package main

// Unix publication uses renameat between retained directory descriptors. A
// mode-0700 private staging directory is used only after the platform policy has
// admitted the filesystem and the publication parent's ancestor namespace.
//
// SECURITY ASSUMPTION: processes with this UID (or elevated filesystem authority)
// must not modify the private staging directory or its objects. Permissions do
// not isolate mutually hostile processes with the same UID. Identity checks here
// detect ordered violations; they are NOT a race-proof substitute for isolation.
//
// On FAT and exFAT the owner and mode bits come from mount options, so the 0700
// and owner checks below are skipped there; the result already lists folder
// permissions as not checked on such a drive.

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func createDestStage(parent *os.Root, name string) (*destStage, error) {
	container := strings.TrimSuffix(name, mirrorTmpSuffix) + ".obelisk-stage"
	if err := parent.Mkdir(container, 0o700); err != nil {
		return nil, err
	}
	private, err := parent.OpenRoot(container)
	if err != nil {
		return nil, err
	}
	p, err := private.Open(".")
	if err != nil {
		private.Close()
		return nil, err
	}
	var st unix.Stat_t
	err = unix.Fstat(int(p.Fd()), &st)
	runtime.KeepAlive(p)
	perms := destStagePermsEnforced(p)
	p.Close()
	if err != nil || perms && (st.Uid != uint32(os.Geteuid()) || st.Mode&0o777 != 0o700) {
		private.Close()
		return nil, fmt.Errorf("cannot establish private staging permissions: %v", err)
	}
	f, err := private.OpenFile("payload"+mirrorTmpSuffix, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		private.Close()
		return nil, err
	}
	return &destStage{file: f, private: private, leaf: path.Join(container, "payload"+mirrorTmpSuffix), perms: perms}, nil
}

// The identity is taken from an opened file. On disagreement leave the unknown
// entry alone. Atomicity against a same-UID adversary is NOT claimed here.
func stageEntryOwned(s *destStage) error {
	f, err := s.private.Open("payload" + mirrorTmpSuffix)
	if err != nil {
		return err
	}
	defer f.Close()
	id, err := handleIdentity(f)
	if err != nil {
		return err
	}
	if id != s.identity {
		return fmt.Errorf("private staging entry was substituted; refusing publication/cleanup")
	}
	return nil
}

// Characterization seam for the expressly excluded late same-principal attack.
var beforeUnixDestRename = func(s *destStage) error { return nil }

func publishDestStage(s *destStage, final string) error {
	var st unix.Stat_t
	err := unix.Fstat(int(s.file.Fd()), &st)
	runtime.KeepAlive(s.file)
	if err != nil {
		return err
	}
	if st.Nlink != 1 || s.perms && (st.Uid != uint32(os.Geteuid()) || st.Mode&0o022 != 0) {
		return fmt.Errorf("staging ownership, link count or writable publication permissions changed")
	}
	if err := stageEntryOwned(s); err != nil {
		return err
	}
	from, err := s.private.Open(".")
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := s.parent.Open(".")
	if err != nil {
		return err
	}
	defer to.Close()
	if err := beforeUnixDestRename(s); err != nil {
		return err
	}
	err = unix.Renameat(int(from.Fd()), "payload"+mirrorTmpSuffix, int(to.Fd()), final)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	return err
}

func discardDestStage(s *destStage) error {
	var err error
	if !s.published {
		err = stageEntryOwned(s)
		if err == nil {
			err = s.private.Remove("payload" + mirrorTmpSuffix)
		}
	}
	// Never remove the public container name: another writer can replace it.
	// Empty .obelisk-stage directories intentionally remain; crash remnants and
	// substituted private entries are retained for inspection, not recursive cleanup.
	return errors.Join(err, s.file.Close(), s.private.Close())
}
