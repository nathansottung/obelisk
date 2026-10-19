//go:build windows

package main

// destprivate_windows.go — "Make this folder private".
//
// The change is to ONE folder's own permissions: entries for other principals are
// removed, the entries for this user, SYSTEM, Administrators and TrustedInstaller
// are kept (as entries set on the folder rather than inherited), and the folder
// stops inheriting from its parent. It is applied by handle with
// SetKernelObjectSecurity, which — unlike SetNamedSecurityInfo or SetSecurityInfo —
// does not walk into the folder and rewrite the permissions of what is inside.
// Files and subfolders keep exactly the permissions they had.

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const destMakePrivateSupported = true

// openFolderForSecurity opens the folder itself (not a link target) for reading,
// and with write access to its permissions when write is set.
func openFolderForSecurity(path string, write bool) (windows.Handle, fileIdent, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fileIdent{}, err
	}
	access := uint32(windows.READ_CONTROL)
	if write {
		access |= windows.WRITE_DAC
	}
	h, err := windows.CreateFile(p, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, fileIdent{}, fmt.Errorf("cannot open %s to read its permissions: %w", path, err)
	}
	var bi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &bi); err != nil {
		windows.CloseHandle(h)
		return 0, fileIdent{}, err
	}
	if bi.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || bi.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(h)
		return 0, fileIdent{}, fmt.Errorf("%s is not a plain folder", path)
	}
	return h, fileIdent{vol: uint64(bi.VolumeSerialNumber), idx: uint64(bi.FileIndexHigh)<<32 | uint64(bi.FileIndexLow)}, nil
}

func folderSDDL(h windows.Handle) (string, error) {
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return "", fmt.Errorf("cannot read the folder's permissions: %w", err)
	}
	return sd.String(), nil
}

// planPrivateFolder works out the private permissions for the folder at path.
func planPrivateFolder(path string) (*destPrivateChange, error) {
	h, id, err := openFolderForSecurity(path, false)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(h)
	before, err := folderSDDL(h)
	if err != nil {
		return nil, err
	}
	user, err := destUserSID()
	if err != nil {
		return nil, err
	}
	after, beforeLines, afterLines, removed, err := privateDACL(before, user)
	if err != nil {
		return nil, err
	}
	return &destPrivateChange{id: id, before: before, after: after, beforeLines: beforeLines, afterLines: afterLines, removedLines: removed}, nil
}

// applyPrivateFolder applies a planned change, only if the folder is still the same
// folder with exactly the permissions the plan was made from.
func applyPrivateFolder(path string, ch *destPrivateChange) error {
	h, id, err := openFolderForSecurity(path, true)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	if id != ch.id {
		return fmt.Errorf("%s is no longer the folder the change was shown for; preview it again", path)
	}
	now, err := folderSDDL(h)
	if err != nil {
		return err
	}
	if now != ch.before {
		return fmt.Errorf("the permissions of %s changed since the change was shown; preview it again", path)
	}
	sd, err := windows.SecurityDescriptorFromString(destDACLPart(ch.after))
	if err != nil {
		return err
	}
	if err := windows.SetKernelObjectSecurity(h, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, sd); err != nil {
		return fmt.Errorf("cannot change the permissions of %s: %w", path, err)
	}
	return nil
}

// destDACLPart returns the "D:..." part of an SDDL string.
func destDACLPart(sddl string) string {
	i := strings.Index(sddl, "D:")
	if i < 0 {
		return ""
	}
	d := sddl[i:]
	if j := strings.Index(d, "S:"); j > 0 {
		d = d[:j]
	}
	return d
}

// privateDACL keeps the entries for trusted principals and every deny entry,
// removes the rest, marks the DACL protected and makes sure this user keeps full
// control. It works from the binary ACL, entry by entry, and refuses any entry it
// does not fully understand — including inherit-only and conditional entries — so
// nothing is ever dropped without being listed. The result puts deny entries
// before allow entries, the order Windows evaluates correctly. It returns the new
// SDDL (owner unchanged) and plain-words lines for before, after and removed.
func privateDACL(before, user string) (after string, beforeLines, afterLines, removed []string, err error) {
	sd, err := windows.SecurityDescriptorFromString(before)
	if err != nil {
		return "", nil, nil, nil, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return "", nil, nil, nil, err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return "", nil, nil, nil, err
	}
	if acl == nil {
		return "", nil, nil, nil, fmt.Errorf("the folder has no permission list to change")
	}
	var denies, allows []destACE
	userFull := false
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return "", nil, nil, nil, err
		}
		t := ace.Header.AceType
		if t != windows.ACCESS_ALLOWED_ACE_TYPE && t != windows.ACCESS_DENIED_ACE_TYPE {
			return "", nil, nil, nil, fmt.Errorf("Obelisk can't safely rewrite the permissions of this folder: it has a kind of permission entry (type %d) that Obelisk does not change. Choose Use it anyway or pick another folder", t)
		}
		e := destACE{deny: t == windows.ACCESS_DENIED_ACE_TYPE, flags: ace.Header.AceFlags, mask: uint32(ace.Mask),
			sid: (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()}
		beforeLines = append(beforeLines, e.line())
		if !e.deny && !trustedDestSID(e.sid, user) {
			removed = append(removed, e.line())
			continue
		}
		e.flags &^= destAceInherited
		if e.deny {
			denies = append(denies, e)
		} else {
			allows = append(allows, e)
			if e.sid == user && e.full() && e.flags&destAceInheritOnly == 0 && e.flags&(destAceObjectInherit|destAceContainerInherit) == destAceObjectInherit|destAceContainerInherit {
				userFull = true
			}
		}
	}
	if !userFull {
		allows = append([]destACE{{flags: destAceObjectInherit | destAceContainerInherit, mask: 0x1f01ff, sid: user}}, allows...)
	}
	var b strings.Builder
	b.WriteString("O:" + owner.String() + "D:P")
	for _, e := range append(denies, allows...) {
		b.WriteString(e.sddl())
		afterLines = append(afterLines, e.line())
	}
	return b.String(), beforeLines, afterLines, removed, nil
}

const destAceObjectInherit = 0x1

// destACE is one allow or deny entry of a folder's permissions.
type destACE struct {
	deny  bool
	flags byte
	mask  uint32
	sid   string
}

func (e destACE) full() bool { return e.mask&0x1f01ff == 0x1f01ff || e.mask&windows.GENERIC_ALL != 0 }

func (e destACE) sddl() string {
	t := "A"
	if e.deny {
		t = "D"
	}
	var f strings.Builder
	for _, x := range []struct {
		bit  byte
		name string
	}{{0x1, "OI"}, {0x2, "CI"}, {0x4, "NP"}, {0x8, "IO"}, {0x10, "ID"}} {
		if e.flags&x.bit != 0 {
			f.WriteString(x.name)
		}
	}
	return fmt.Sprintf("(%s;%s;0x%x;;;%s)", t, f.String(), e.mask, e.sid)
}

// line describes the entry in plain words.
func (e destACE) line() string {
	verb := "can"
	if e.deny {
		verb = "is denied"
	}
	scope := "this folder only"
	switch {
	case e.flags&destAceInheritOnly != 0:
		scope = "files and folders inside only"
	case e.flags&(destAceObjectInherit|destAceContainerInherit) != 0:
		scope = "this folder and what's inside"
	}
	from := "set on this folder"
	if e.flags&destAceInherited != 0 {
		from = "inherited"
	}
	return fmt.Sprintf("%s %s %s (%s, %s)", destGroupName(e.sid), verb, destRightsName(e.mask), scope, from)
}

// destRightsName names an access mask in plain words.
func destRightsName(m uint32) string {
	switch {
	case m&0x1f01ff == 0x1f01ff || m&windows.GENERIC_ALL != 0:
		return "do anything, including change permissions"
	case m&0x1301bf == 0x1301bf || m&(windows.GENERIC_WRITE|windows.GENERIC_READ|windows.GENERIC_EXECUTE|windows.DELETE) == windows.GENERIC_WRITE|windows.GENERIC_READ|windows.GENERIC_EXECUTE|windows.DELETE:
		return "add, change and delete files"
	case m == 0x1200a9 || m&^(windows.GENERIC_READ|windows.GENERIC_EXECUTE) == 0:
		return "read and open files"
	case m == windows.FILE_APPEND_DATA:
		return "create folders"
	case m == windows.FILE_WRITE_DATA:
		return "create files"
	}
	var parts []string
	for _, x := range []struct {
		bit  uint32
		name string
	}{
		{windows.FILE_WRITE_DATA, "create files"}, {windows.FILE_APPEND_DATA, "create folders"},
		{destFileDeleteChild, "delete what's inside"}, {windows.DELETE, "delete"},
		{windows.WRITE_DAC, "change permissions"}, {windows.WRITE_OWNER, "take ownership"},
		{windows.FILE_WRITE_ATTRIBUTES, "change attributes"}, {windows.FILE_WRITE_EA, "change extended attributes"},
		{windows.GENERIC_WRITE, "write"}, {windows.FILE_READ_DATA | windows.GENERIC_READ, "read"},
	} {
		if m&x.bit != 0 {
			parts = append(parts, x.name)
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("use special access (0x%x)", m)
	}
	return strings.Join(parts, ", ")
}
