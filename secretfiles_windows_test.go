//go:build windows && !guionly

package main

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

var aceSIDPattern = regexp.MustCompile(`\(([^;]*);[^;]*;[^;]*;[^;]*;[^;]*;([^)]*)\)`)

// ownerOnlyACLForTest requires p's DACL to be protected (nothing inherited) and
// to allow access to the current user only.
func ownerOnlyACLForTest(p string) error {
	user, err := destUserSID()
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	s := sd.String()
	i := strings.Index(s, "D:")
	if i < 0 {
		return fmt.Errorf("no DACL: %s", s)
	}
	dacl := s[i:]
	if !strings.HasPrefix(dacl, "D:P") {
		return fmt.Errorf("the DACL inherits from the folder: %s", dacl)
	}
	aces := aceSIDPattern.FindAllStringSubmatch(dacl, -1)
	if len(aces) == 0 {
		return fmt.Errorf("the DACL grants nobody access: %s", dacl)
	}
	for _, a := range aces {
		if a[1] == "A" && a[2] != user {
			return fmt.Errorf("the DACL grants access to %s as well as the owner: %s", a[2], dacl)
		}
	}
	return nil
}
