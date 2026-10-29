//go:build windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createOwnerOnlyTemp creates a new, uniquely named file in dir (the * in
// pattern is replaced) whose security descriptor gives full access to the
// current user only and inherits nothing from dir. The descriptor is set when
// the file is created, so the file is never readable by anyone else. Creation
// fails rather than open an existing file.
func createOwnerOnlyTemp(dir, pattern string) (*os.File, error) {
	sd, err := privateDestSD()
	if err != nil {
		return nil, err
	}
	sa := windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(sa))
	for i := 0; i < 100; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, strings.Replace(pattern, "*", hex.EncodeToString(b), 1))
		p, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
			&sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) {
			continue
		}
		if err != nil {
			return nil, &os.PathError{Op: "create", Path: name, Err: err}
		}
		return os.NewFile(uintptr(h), name), nil
	}
	return nil, &os.PathError{Op: "create", Path: filepath.Join(dir, pattern), Err: os.ErrExist}
}
