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
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

const destMakePrivateSupported = true

var sddlACE = regexp.MustCompile(`\(([^()]*)\)`)

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

// privateDACL keeps the entries for trusted principals, removes the rest, marks
// the DACL protected and makes sure this user keeps full control. It returns the
// new SDDL (owner unchanged) and plain-words lines for before, after and removed.
func privateDACL(before, user string) (after string, beforeLines, afterLines, removed []string, err error) {
	dacl := destDACLPart(before)
	if dacl == "" {
		return "", nil, nil, nil, fmt.Errorf("the folder has no permission list to change")
	}
	owner := before[:strings.Index(before, "D:")]
	var kept []string
	userFull := false
	for _, m := range sddlACE.FindAllStringSubmatch(dacl, -1) {
		ace := m[1]
		fields := strings.Split(ace, ";")
		if len(fields) < 6 {
			return "", nil, nil, nil, fmt.Errorf("unexpected permission entry %q", ace)
		}
		sid := fields[5]
		if s, err := windows.StringToSid(sid); err == nil {
			sid = s.String()
		}
		line := destACELine(fields, sid)
		beforeLines = append(beforeLines, line)
		if fields[0] == "D" || trustedDestSID(sid, user) {
			fields[1] = dropSDDLFlag(fields[1], "ID")
			kept = append(kept, "("+strings.Join(fields, ";")+")")
			afterLines = append(afterLines, destACELine(fields, sid))
			if sid == user && fields[0] == "A" && (fields[2] == "FA" || strings.EqualFold(fields[2], "0x1f01ff")) && !strings.Contains(fields[1], "IO") {
				userFull = true
			}
			continue
		}
		removed = append(removed, line)
	}
	if !userFull {
		f := []string{"A", "OICI", "FA", "", "", user}
		kept = append([]string{"(" + strings.Join(f, ";") + ")"}, kept...)
		afterLines = append([]string{destACELine(f, user)}, afterLines...)
	}
	return owner + "D:P" + strings.Join(kept, ""), beforeLines, afterLines, removed, nil
}

// dropSDDLFlag removes one two-letter flag from an SDDL ACE flag string.
func dropSDDLFlag(flags, flag string) string {
	var out strings.Builder
	for i := 0; i+1 < len(flags); i += 2 {
		if f := flags[i : i+2]; f != flag {
			out.WriteString(f)
		}
	}
	return out.String()
}

// destACELine describes one permission entry in plain words.
func destACELine(fields []string, sid string) string {
	verb := "can"
	if fields[0] == "D" {
		verb = "is denied"
	}
	rights := destRightsName(fields[2])
	flags := fields[1]
	scope := "this folder only"
	switch {
	case strings.Contains(flags, "IO"):
		scope = "files and folders inside only"
	case strings.Contains(flags, "OI") || strings.Contains(flags, "CI"):
		scope = "this folder and what's inside"
	}
	from := "set on this folder"
	if strings.Contains(flags, "ID") {
		from = "inherited"
	}
	return fmt.Sprintf("%s %s %s (%s, %s)", destGroupName(sid), verb, rights, scope, from)
}

func destRightsName(r string) string {
	switch strings.ToLower(r) {
	case "fa", "ga", "0x1f01ff", "0x10000000":
		return "do anything"
	case "0x1301bf", "sdgxgwgr", "0x13016f":
		return "add, change and delete files"
	case "0x1200a9", "fr", "fx", "grgx", "0xa0000000":
		return "read and open files"
	case "lc", "0x4":
		return "create folders"
	case "0x2", "cc":
		return "create files"
	}
	return "use access " + r
}
