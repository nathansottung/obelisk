//go:build windows

package main

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ioctlDiskIsWritable = 0x00070024
	fileAddFile         = 0x2
	fileAddSubdirectory = 0x4
)

var procAccessCheck = windows.NewLazySystemDLL("advapi32.dll").NewProc("AccessCheck")

// fileGenericMapping maps generic rights to file rights for AccessCheck.
type fileGenericMapping struct{ read, write, execute, all uint32 }

// errWriteProtected reports a failure caused by write-protected media.
func errWriteProtected(err error) bool {
	return errors.Is(err, windows.ERROR_WRITE_PROTECT)
}

// readWriteAccessFacts reads the volume's and media's state and, on a local volume
// that keeps permissions, what the access list gives this account. It writes nothing.
func readWriteAccessFacts(dir string, newFolder bool) (writeFacts, error) {
	var facts writeFacts
	f, err := os.Open(dir)
	if err != nil {
		return facts, err
	}
	defer f.Close()
	network := pathOnNetwork(dir)
	var flags uint32
	if err := windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, nil, nil, &flags, nil, 0); err == nil &&
		flags&windows.FILE_READ_ONLY_VOLUME != 0 {
		if network {
			facts.ReadOnlyShare = true
		} else {
			facts.ReadOnlyVolume = true
		}
	}
	runtime.KeepAlive(f)
	if network {
		return facts, nil // a share's own limits show in the real attempt
	}
	if vol, err := volumeDevicePath(dir); err == nil {
		if h, err := openForQuery(vol); err == nil {
			var n uint32
			if errors.Is(windows.DeviceIoControl(h, ioctlDiskIsWritable, nil, 0, nil, 0, &n, nil), windows.ERROR_WRITE_PROTECT) {
				facts.WriteProtected = true
			}
			windows.CloseHandle(h)
		}
	}
	if flags&windows.FILE_PERSISTENT_ACLS == 0 {
		return facts, nil
	}
	want := uint32(fileAddFile | fileAddSubdirectory)
	if newFolder {
		want = fileAddSubdirectory // a drive root lets users add folders but not files
	}
	sd, err := windows.GetSecurityInfo(windows.Handle(f.Fd()), windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	runtime.KeepAlive(f)
	if err != nil {
		return facts, err
	}
	var proc windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &proc); err != nil {
		return facts, err
	}
	defer proc.Close()
	var token windows.Token
	if err := windows.DuplicateTokenEx(proc, windows.TOKEN_QUERY, nil, windows.SecurityIdentification, windows.TokenImpersonation, &token); err != nil {
		return facts, err
	}
	defer token.Close()
	// Windows decides by the order of the entries, so it is asked first. A deny entry
	// only names the cause of a real denial.
	if accessGranted(sd, token, want) {
		return facts, nil
	}
	facts.DeniedBy = deniedFor(sd, token, want)
	facts.CannotChange = len(facts.DeniedBy) == 0
	return facts, nil
}

// deniedFor names the accounts and groups of token with a deny entry for any of want
// that applies to this folder itself.
func deniedFor(sd *windows.SECURITY_DESCRIPTOR, token windows.Token, want uint32) []string {
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return nil
	}
	var names []string
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil || ace.Header.AceType != windows.ACCESS_DENIED_ACE_TYPE ||
			ace.Header.AceFlags&destAceInheritOnly != 0 || uint32(ace.Mask)&want == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if member, err := token.IsMember(sid); err == nil && member {
			if n := destGroupName(sid.String()); !destContains(names, n) {
				names = append(names, n)
			}
		}
	}
	return names
}

// accessGranted asks Windows whether token holds all of want on this descriptor. If
// Windows cannot answer, it says yes here: the real attempt then decides.
func accessGranted(sd *windows.SECURITY_DESCRIPTOR, token windows.Token, want uint32) bool {
	mapping := fileGenericMapping{read: 0x120089, write: 0x120116, execute: 0x1200A0, all: 0x1F01FF}
	var privileges [256]byte
	size := uint32(len(privileges))
	var granted, status uint32
	r, _, _ := syscall.SyscallN(procAccessCheck.Addr(),
		uintptr(unsafe.Pointer(sd)), uintptr(token), uintptr(want), uintptr(unsafe.Pointer(&mapping)),
		uintptr(unsafe.Pointer(&privileges[0])), uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Pointer(&granted)), uintptr(unsafe.Pointer(&status)))
	runtime.KeepAlive(sd)
	if r == 0 {
		return true
	}
	return status != 0
}
