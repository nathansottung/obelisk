//go:build windows

package main

// Windows owns staging from FILE_CREATE through rename/disposition by retaining
// a read/write/delete handle that shares neither writes nor deletes. The kernel
// refuses name substitution and competing writes while that handle is held.
// Publication targets a single leaf under an already-confined parent handle.
// No reopened staging name, delete-destination fallback, or absolute mutation.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func createDestStage(parent *os.Root, name string) (*destStage, error) {
	p, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer p.Close()
	u, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	sd, err := privateDestSD()
	if err != nil {
		return nil, err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(p.Fd()), ObjectName: u, Attributes: windows.OBJ_CASE_INSENSITIVE, SecurityDescriptor: sd}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE|windows.SYNCHRONIZE,
		&oa, &iosb, nil, windows.FILE_ATTRIBUTE_NORMAL, 0, windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
	runtime.KeepAlive(p)
	runtime.KeepAlive(sd)
	if err != nil {
		return nil, fmt.Errorf("create owned staging: %w", err)
	}
	return &destStage{file: os.NewFile(uintptr(h), filepath.Join(parent.Name(), name)), leaf: name}, nil
}

// publishDestStage renames the staged file, by its handle, to final in its parent
// folder. With replace it replaces any existing entry; without, the rename fails
// if final exists (reported as fs.ErrExist). A rename blocked by a transient lock (for
// example antivirus scanning the replaced file) is retried for a bounded time, as
// pathname renames are; see retryTransientRename.
func publishDestStage(s *destStage, final string, replace bool) error {
	p, err := s.parent.Open(".")
	if err != nil {
		return err
	}
	defer p.Close()
	name, err := windows.UTF16FromString(final)
	if err != nil {
		return err
	}
	// Native FILE_RENAME_INFORMATION has pointer-size alignment on both 32/64 bit.
	type renameInfo struct {
		Replace uint32
		Root    windows.Handle
		Length  uint32
		Name    [1]uint16
	}
	var layout renameInfo
	offset := unsafe.Offsetof(layout.Name)
	buf := make([]byte, int(offset)+2*len(name))
	info := (*renameInfo)(unsafe.Pointer(&buf[0]))
	info.Root, info.Length = windows.Handle(p.Fd()), uint32(2*(len(name)-1))
	if replace {
		info.Replace = 1
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[offset])), len(name)), name)
	err = retryTransientRename(func() error {
		var iosb windows.IO_STATUS_BLOCK
		return handleRename(windows.Handle(s.file.Fd()), &iosb, &buf[0], uint32(len(buf)))
	})
	runtime.KeepAlive(s.file)
	runtime.KeepAlive(p)
	var status windows.NTStatus
	if !replace && errors.As(err, &status) && status == windows.STATUS_OBJECT_NAME_COLLISION {
		return fmt.Errorf("%w: %w", fs.ErrExist, err)
	}
	return err
}

// handleRename issues the handle rename. A variable so tests can make it fail the
// way a briefly locked destination does.
var handleRename = func(h windows.Handle, iosb *windows.IO_STATUS_BLOCK, info *byte, n uint32) error {
	return windows.NtSetInformationFile(h, iosb, info, n, windows.FileRenameInformation)
}

func discardDestStage(s *destStage) error {
	var err error
	if !s.published {
		// FileDispositionInformation acts on our handle, even on an error path.
		// Clear read-only on OUR object if a sidecar requested read-only mode.
		err = s.file.Chmod(0o600)
		remove := byte(1)
		var iosb windows.IO_STATUS_BLOCK
		err = errors.Join(err, windows.NtSetInformationFile(windows.Handle(s.file.Fd()), &iosb, &remove, 1, windows.FileDispositionInformation))
		runtime.KeepAlive(s.file)
	}
	return errors.Join(err, s.file.Close())
}
