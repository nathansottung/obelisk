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

// checkDestDACL refuses a directory whose owner is untrusted or whose DACL lets an
// untrusted principal mutate it. One grant is tolerated with a warning: Windows
// gives Authenticated Users "create folders" (and nothing else) on a volume root
// by default, so a destination that IS a drive root would otherwise always fail.
// Any other grant to other users, on a root or anywhere else, still refuses.
func checkDestDACL(sd *windows.SECURITY_DESCRIPTOR, user string, publication, volumeRoot bool) (warning string, err error) {
	owner, _, err := sd.Owner()
	if err != nil {
		return "", err
	}
	if owner == nil || !trustedDestSID(owner.String(), user) {
		return "", fmt.Errorf("unsafe destination namespace: untrusted directory owner")
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return "", err
	}
	if acl == nil {
		return "", fmt.Errorf("unsafe destination namespace: unrestricted NULL DACL")
	}
	mask := uint32(destDACLMutation)
	if publication {
		mask |= destDACLPublication
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return "", err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
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
				return "", fmt.Errorf("unsafe destination namespace: DACL entry with an invalid principal")
			}
			s := sid.String()
			if trustedDestSID(s, user) {
				continue
			}
			if volumeRoot && publication && s == sidAuthenticatedUsers && granted == windows.FILE_APPEND_DATA {
				warning = "the drive root lets any signed-in user create folders (the Windows default); a folder another user creates here is refused when Obelisk reaches it"
				continue
			}
			return "", fmt.Errorf("unsafe destination namespace: DACL grants mutation mask %#x to untrusted principal %s", granted, s)
		default:
			return "", fmt.Errorf("unsupported destination namespace: ACE type %d requires native policy evaluation", ace.Header.AceType)
		}
	}
	return warning, nil
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
		fs := destFS{name: fsName, notChecked: []string{
			"folder permissions (" + fsName + " has no access control)",
			"hard links (" + fsName + " has none)",
			"stable file IDs (" + fsName + " file IDs can change when a file is renamed)",
		}}
		if fsName != "exFAT" {
			fs.maxFile = fatMaxFile
		}
		return fs, false, nil
	}
	return destFS{}, false, fmt.Errorf("unsupported destination filesystem %q: use an NTFS, exFAT or FAT32 drive", fsName)
}

// checkDestNamespace checks the directory held by r and every ancestor up to the
// volume root: each must be on an accepted filesystem and, where the filesystem
// keeps permissions, owned and writable only by this user and the system. It
// returns what it learned about the destination's own filesystem.
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
	publication := true
	for {
		fs, acls, err := destFilesystem(f)
		if err != nil {
			return destFS{}, err
		}
		if publication {
			dest = fs
		}
		canon, err := handleCanonicalPath(f)
		if err != nil {
			return destFS{}, err
		}
		parent := filepath.Dir(canon)
		volumeRoot := parent == canon
		if acls {
			sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
			runtime.KeepAlive(f)
			if err != nil {
				return destFS{}, fmt.Errorf("cannot inspect destination namespace DACL: %w", err)
			}
			warning, err := checkDestDACL(sd, user, publication, volumeRoot)
			if err != nil {
				return destFS{}, fmt.Errorf("%s: %w", canon, err)
			}
			if warning != "" {
				dest.warnings = append(dest.warnings, warning)
			}
		}
		if volumeRoot {
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
