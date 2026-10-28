//go:build windows && !guionly

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ownerOnlyACLForTest requires p's DACL to be protected (nothing inherited) and
// every allow entry in it to name the current user. SIDs are compared as SIDs:
// the text form abbreviates some (the built-in Administrator account is "LA").
func ownerOnlyACLForTest(p string) error {
	tok, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	user := tok.User.Sid
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	ctl, _, err := sd.Control()
	if err != nil {
		return err
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("the DACL inherits from the folder: %s", sd.String())
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil || dacl.AceCount == 0 {
		return fmt.Errorf("the DACL grants nobody access: %s", sd.String())
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if !sid.Equals(user) {
			return fmt.Errorf("the DACL grants access to %s as well as the owner: %s", sid.String(), sd.String())
		}
	}
	return nil
}
