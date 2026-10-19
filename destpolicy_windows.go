//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sidAuthenticatedUsers = "S-1-5-11"
	sidLocalSystem        = "S-1-5-18"
	sidAdministrators     = "S-1-5-32-544"
	sidTrustedInstaller   = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
)

// platformDestName refuses names Windows would silently alter or read as a stream
// or device: trailing spaces or dots, a colon, a NUL.
func platformDestName(rel string) error {
	if !filepath.IsLocal(filepath.FromSlash(rel)) {
		return fmt.Errorf("unsupported Windows name")
	}
	for _, el := range strings.Split(rel, "/") {
		if strings.HasSuffix(el, " ") || strings.HasSuffix(el, ".") || strings.ContainsAny(el, ":\x00") {
			return fmt.Errorf("unsupported or ambiguous Windows component %q", el)
		}
	}
	return nil
}

func destUserSID() (string, error) {
	var token windows.Token
	if err := windows.OpenThreadToken(windows.CurrentThread(), windows.TOKEN_QUERY, true, &token); err == nil {
		token.Close()
		return "", fmt.Errorf("unsupported destination authority: impersonating thread")
	} else if err != windows.ERROR_NO_TOKEN {
		return "", err
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

func privateDestSD() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := destUserSID()
	if err != nil {
		return nil, err
	}
	// Protected DACL: do not inherit grants from the publication parent.
	return windows.SecurityDescriptorFromString("O:" + user + "D:P(A;;FA;;;" + user + ")")
}

func trustedDestSID(sid, user string) bool {
	return sid == user || sid == sidLocalSystem || sid == sidAdministrators || sid == sidTrustedInstaller
}

// destDACLAccess are the rights that let another principal replace or redirect
// entries in a directory. A publication directory (the one files are written
// into) also must not let others add entries.
const (
	destFileDeleteChild = 0x40
	destDACLMutation    = windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | destFileDeleteChild |
		windows.FILE_WRITE_ATTRIBUTES | windows.FILE_WRITE_EA | windows.GENERIC_ALL | windows.GENERIC_WRITE
	destDACLPublication = windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA // ADD_FILE, ADD_SUBDIRECTORY
)

// ACE header flags.
const (
	destAceContainerInherit = 0x2
	destAceInheritOnly      = 0x8
	destAceInherited        = 0x10
)

const (
	sidUsers     = "S-1-5-32-545"
	sidGuests    = "S-1-5-32-546"
	sidEveryone  = "S-1-1-0"
	sidAnonymous = "S-1-5-7"
)

// destGroupName is the plain name of a principal for notices and refusals.
func destGroupName(sid string) string {
	switch sid {
	case sidAuthenticatedUsers:
		return "Authenticated Users"
	case sidUsers:
		return "Users"
	case sidEveryone:
		return "Everyone"
	case sidGuests:
		return "Guests"
	case sidAnonymous:
		return "Anonymous Logon"
	case sidLocalSystem:
		return "SYSTEM"
	case sidAdministrators:
		return "Administrators"
	case sidTrustedInstaller:
		return "TrustedInstaller"
	}
	if s, err := windows.StringToSid(sid); err == nil {
		if name, domain, _, err := s.LookupAccount(""); err == nil && name != "" {
			if domain != "" {
				return domain + `\` + name
			}
			return name
		}
	}
	return sid
}

// classifyDestGrant decides what one grant of change rights to another principal
// means for a destination folder:
//
//   - the Windows default on a drive root, where Authenticated Users may create
//     folders and nothing else, is a finding the user decides about;
//   - an INHERITED grant to Authenticated Users or Users, which is how Windows lays
//     out folders made directly on a drive, is a finding the user decides about;
//   - anything else — Everyone, Guests, Anonymous Logon, any other account or
//     group, or a grant set explicitly on this folder — refuses.
func classifyDestGrant(sid, folder string, granted uint32, inherited, publication, volumeRoot bool) (*destFinding, error) {
	if volumeRoot && publication && sid == sidAuthenticatedUsers && granted == windows.FILE_APPEND_DATA {
		return &destFinding{Kind: destFindingDriveRoot, Folder: folder, Group: destGroupName(sid), SID: sid}, nil
	}
	if (sid == sidAuthenticatedUsers || sid == sidUsers) && inherited && !volumeRoot {
		return &destFinding{Kind: destFindingSharedGroup, Folder: folder, Group: destGroupName(sid), SID: sid, Inherited: true}, nil
	}
	how := "set on this folder"
	if inherited {
		how = "inherited"
	}
	return nil, fmt.Errorf("unsafe destination namespace: %s lets %s change it (%s; mutation mask %#x to untrusted principal %s)", folder, destGroupName(sid), how, granted, sid)
}

// checkDestDACL refuses a directory whose owner is untrusted or whose DACL lets
// another principal change it in a way classifyDestGrant refuses, and returns the
// grants the user must decide about.
func checkDestDACL(sd *windows.SECURITY_DESCRIPTOR, user, folder string, publication, volumeRoot bool) ([]destFinding, error) {
	owner, _, err := sd.Owner()
	if err != nil {
		return nil, err
	}
	if owner == nil || !trustedDestSID(owner.String(), user) {
		return nil, fmt.Errorf("unsafe destination namespace: %s is owned by another account (untrusted directory owner)", folder)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return nil, err
	}
	if acl == nil {
		return nil, fmt.Errorf("unsafe destination namespace: %s has no permissions set, so anyone can change it (unrestricted NULL DACL)", folder)
	}
	mask := uint32(destDACLMutation)
	if publication {
		mask |= destDACLPublication
	}
	var findings []destFinding
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return nil, err
		}
		if ace.Header.AceFlags&destAceInheritOnly != 0 {
			continue
		}
		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue // Conservative: never subtract a deny from a later grant.
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			granted := uint32(ace.Mask) & mask
			if granted == 0 {
				continue
			}
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
			if !sid.IsValid() {
				return nil, fmt.Errorf("unsafe destination namespace: %s has a permission entry with an invalid principal", folder)
			}
			s := sid.String()
			if trustedDestSID(s, user) {
				continue
			}
			f, err := classifyDestGrant(s, folder, granted, ace.Header.AceFlags&destAceInherited != 0, publication, volumeRoot)
			if err != nil {
				return nil, err
			}
			findings = appendFinding(findings, *f)
		default:
			return nil, fmt.Errorf("unsupported destination namespace: ACE type %d requires native policy evaluation", ace.Header.AceType)
		}
	}
	return findings, nil
}

// predictDestChild returns what a folder Obelisk creates inside the directory r
// holds would inherit, so the choice can be made before anything is created. Every
// container-inheritable grant on r becomes an inherited grant on the new folder.
func predictDestChild(r *os.Root, child string) ([]destFinding, error) {
	f, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, acls, err := destFilesystem(f)
	if err != nil || !acls {
		return nil, err
	}
	user, err := destUserSID()
	if err != nil {
		return nil, err
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	runtime.KeepAlive(f)
	if err != nil {
		return nil, fmt.Errorf("cannot inspect destination namespace DACL: %w", err)
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return nil, err
	}
	mask := uint32(destDACLMutation | destDACLPublication)
	var findings []destFinding
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return nil, err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&destAceContainerInherit == 0 {
			continue
		}
		granted := uint32(ace.Mask) & mask
		if granted == 0 {
			continue
		}
		s := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if trustedDestSID(s, user) {
			continue
		}
		fd, err := classifyDestGrant(s, child, granted, true, true, false)
		if err != nil {
			return nil, err
		}
		findings = appendFinding(findings, *fd)
	}
	return findings, nil
}

// destVolumeInformation reports the filesystem name and flags of the volume behind
// f. A variable so tests can present a FAT or exFAT volume, which cannot be created
// here without administrator rights.
var destVolumeInformation = func(f *os.File) (string, uint32, error) {
	var flags uint32
	var name [32]uint16
	err := windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, nil, nil, &flags, &name[0], uint32(len(name)))
	runtime.KeepAlive(f)
	return windows.UTF16ToString(name[:]), flags, err
}

// fatMaxFile is the largest file FAT32 (and FAT16) can hold: 4 GiB minus one byte.
const fatMaxFile = 1<<32 - 1

// destFilesystem describes the filesystem behind f and whether Obelisk accepts it
// as a destination. NTFS with persistent ACLs gets every check. exFAT and FAT are
// accepted too, recording what cannot be checked there. Network shares, ReFS and
// anything else are refused.
func destFilesystem(f *os.File) (destFS, bool, error) {
	canon, err := handleCanonicalPath(f)
	if err != nil {
		return destFS{}, false, err
	}
	if strings.HasPrefix(canon, `\\`) {
		return destFS{}, false, fmt.Errorf("unsupported destination: network shares are not supported; choose a folder on a local or USB drive")
	}
	drive, err := windows.UTF16PtrFromString(filepath.VolumeName(canon) + `\`)
	if err != nil {
		return destFS{}, false, err
	}
	typeID := windows.GetDriveType(drive)
	if typeID != windows.DRIVE_FIXED && typeID != windows.DRIVE_REMOVABLE {
		return destFS{}, false, fmt.Errorf("unsupported destination drive type %d: choose a folder on a local or USB drive", typeID)
	}
	fsName, flags, err := destVolumeInformation(f)
	if err != nil {
		return destFS{}, false, fmt.Errorf("cannot establish destination filesystem authority: %w", err)
	}
	switch fsName {
	case "NTFS":
		if flags&windows.FILE_PERSISTENT_ACLS == 0 {
			return destFS{}, false, fmt.Errorf("unsupported destination filesystem: this NTFS volume does not keep permissions")
		}
		return destFS{name: fsName}, true, nil
	case "exFAT", "FAT32", "FAT":
		fs := destFS{name: fsName, notChecked: []string{destCheckPermissions, destCheckHardLinks, destCheckFileIDs}}
		if fsName != "exFAT" {
			fs.maxFile = fatMaxFile
		}
		return fs, false, nil
	}
	return destFS{}, false, fmt.Errorf("unsupported destination filesystem %q: use an NTFS, exFAT or FAT32 drive", fsName)
}

// checkDestNamespace checks the directory held by r and every ancestor up to the
// volume root: each must be on an accepted filesystem and, where the filesystem
// keeps permissions, owned by this user or the system and changeable by others
// only in the ways classifyDestGrant accepts. It returns what it learned about the
// destination's filesystem, the grants the user must decide about, and each
// level's permissions for the choice fingerprint.
func checkDestNamespace(r *os.Root) (destFS, error) {
	user, err := destUserSID()
	if err != nil {
		return destFS{}, err
	}
	f, err := r.Open(".")
	if err != nil {
		return destFS{}, err
	}
	defer func() { f.Close() }()
	var dest destFS
	var findings []destFinding
	var chain []destLevel
	publication := true
	for {
		fs, acls, err := destFilesystem(f)
		if err != nil {
			return destFS{}, err
		}
		canon, err := handleCanonicalPath(f)
		if err != nil {
			return destFS{}, err
		}
		if publication {
			dest = fs
			if len(fs.notChecked) > 0 {
				findings = append(findings, destFinding{Kind: destFindingUnchecked, Folder: canon, FS: fs.name, MaxFile: fs.maxFile})
			}
		}
		parent := filepath.Dir(canon)
		volumeRoot := parent == canon
		level := destLevel{path: canon, fs: fs.name}
		if acls {
			sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
			runtime.KeepAlive(f)
			if err != nil {
				return destFS{}, fmt.Errorf("cannot inspect destination namespace DACL: %w", err)
			}
			found, err := checkDestDACL(sd, user, canon, publication, volumeRoot)
			if err != nil {
				return destFS{}, err
			}
			for _, fd := range found {
				findings = appendFinding(findings, fd)
			}
			level.perms = sd.String()
		} else if id, err := handleIdentity(f); err == nil {
			level.perms = fmt.Sprintf("vsn:%08X", id.vol)
		}
		chain = append(chain, level)
		if volumeRoot {
			dest.findings, dest.chain = findings, chain
			return dest, nil
		}
		p, err := os.Open(parent) // read-only authority inspection, never a mutation
		if err != nil {
			return destFS{}, err
		}
		f.Close()
		f, publication = p, false
	}
}

// One cooperative lease per local volume across processes and login sessions.
// Conservatively serializes even disjoint output roots on that volume. A dedicated
// locked thread acquires AND releases the Windows thread-owned mutex. This is
// application coordination, not protection against malicious same-principal code.
func acquireDestLease(r *os.Root) (func() error, error) {
	f, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	id, err := handleIdentity(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	sd, err := privateDestSD()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`Global\Obelisk-destination-v1-%x`, id.vol))
	if err != nil {
		return nil, err
	}
	ready, release, done := make(chan error, 1), make(chan struct{}), make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
		h, err := windows.CreateMutex(&sa, false, name)
		if err != nil && err != windows.ERROR_ALREADY_EXISTS {
			ready <- err
			return
		}
		defer windows.CloseHandle(h)
		status, err := windows.WaitForSingleObject(h, 0)
		if err == nil && status == uint32(windows.WAIT_TIMEOUT) {
			destLeaseContended()
			status, err = windows.WaitForSingleObject(h, 30000)
		}
		if err != nil || status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED {
			ready <- fmt.Errorf("destination volume is busy or cannot coordinate Obelisk publication (wait %d): %v", status, err)
			return
		}
		ready <- nil
		<-release
		done <- windows.ReleaseMutex(h)
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	var once sync.Once
	var closeErr error
	return func() error { once.Do(func() { close(release); closeErr = <-done }); return closeErr }, nil
}
