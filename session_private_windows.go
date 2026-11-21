//go:build windows && !guionly

package main

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ensurePrivateDir creates path with an ACL for this user alone, or confirms that an
// existing folder is a real folder owned by and open only to this user.
func ensurePrivateDir(path string) error {
	sd, err := privateDestSD()
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sa := windows.SecurityAttributes{SecurityDescriptor: sd}
	sa.Length = uint32(unsafe.Sizeof(sa))
	if err := windows.CreateDirectory(p, &sa); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return &os.PathError{Op: "mkdir", Path: path, Err: err}
	}
	h, _, err := openFolderForSecurity(path, false)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return checkOwnerOnlyHandle(h, path)
}

// ensureSigninParentPrivate has nothing to check on Windows: the parent is inside
// %LocalAppData%, which is per-user, and the signin folder below it has its own
// protected access list.
func ensureSigninParentPrivate(string) error { return nil }

// verifyPrivateFile confirms path is a plain file owned by and open only to this user.
func verifyPrivateFile(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
		}
		return &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer windows.CloseHandle(h)
	var bi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &bi); err != nil {
		return err
	}
	if bi.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		return fmt.Errorf("%s is not a plain file", path)
	}
	return checkOwnerOnlyHandle(h, path)
}

// checkOwnerOnlyHandle requires this user as owner and as the only entry in the ACL.
func checkOwnerOnlyHandle(h windows.Handle, path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("cannot read the permissions of %s: %w", path, err)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if !owner.Equals(user.User.Sid) {
		return fmt.Errorf("%s belongs to another account", path)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("%s has no access list", path)
	}
	if dacl.AceCount == 0 {
		return fmt.Errorf("%s has an empty access list", path)
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !sid.Equals(user.User.Sid) {
			return fmt.Errorf("%s can be reached by another account", path)
		}
	}
	return nil
}
