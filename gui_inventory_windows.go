//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// Check native UTF-16 names before os.ReadDir converts them to Go strings. A
// quiescent tree is still required between this bounded pass and observation.
func inventoryDirectoryEncoding(dir string, max int) error {
	var data syscall.Win32finddata
	pattern, err := syscall.UTF16PtrFromString(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}
	handle, err := syscall.FindFirstFile(pattern, &data)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return nil
	}
	if err != nil {
		return err
	}
	defer syscall.FindClose(handle)
	for count := 0; ; count++ {
		if count > max+2 {
			return errors.New("entry limit exceeded in native name validation")
		}
		if !inventoryValidUTF16(data.FileName[:]) {
			return errors.New("unsupported malformed UTF-16 directory entry; no snapshot published")
		}
		err = syscall.FindNextFile(handle, &data)
		if err == syscall.ERROR_NO_MORE_FILES {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func inventoryValidUTF16(name []uint16) bool {
	for i := 0; i < len(name) && name[i] != 0; i++ {
		if name[i] >= 0xdc00 && name[i] <= 0xdfff {
			return false
		}
		if name[i] >= 0xd800 && name[i] <= 0xdbff {
			if i+1 >= len(name) || name[i+1] < 0xdc00 || name[i+1] > 0xdfff {
				return false
			}
			i++
		}
	}
	return true
}

func inventoryIsLink(info os.FileInfo) bool {
	attr, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.Mode()&os.ModeSymlink != 0 || !ok || attr.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func inventoryPlatformPath(name string) error {
	vol := filepath.VolumeName(name)
	if len(vol) != 2 || vol[1] != ':' {
		return fmt.Errorf("only ordinary local drive paths supported: %q", name)
	}
	for _, part := range strings.FieldsFunc(name[len(vol):], func(r rune) bool { return r == '/' || r == '\\' }) {
		if strings.Contains(part, ":") || strings.TrimRight(part, " .") != part {
			return fmt.Errorf("ambiguous/device/stream path refused: %q", name)
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" ||
			(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return fmt.Errorf("reserved device path refused: %q", name)
		}
	}
	root, err := syscall.UTF16PtrFromString(vol + "\\")
	if err != nil {
		return err
	}
	kind, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(root)))
	if kind != 3 {
		return fmt.Errorf("only fixed local drive paths supported: %q", name)
	}
	return nil
}
