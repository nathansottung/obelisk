//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// errWriteProtected reports a failure caused by a read-only filesystem or media.
func errWriteProtected(err error) bool {
	return errors.Is(err, syscall.EROFS)
}

// readWriteAccessFacts asks the kernel whether this account may add entries to dir.
// It writes nothing. A read-only mount answers EROFS.
func readWriteAccessFacts(dir string, _ bool) (writeFacts, error) {
	const wOK, xOK = 0x2, 0x1
	switch err := syscall.Access(dir, wOK|xOK); {
	case err == nil:
		return writeFacts{}, nil
	case errors.Is(err, syscall.EROFS):
		return writeFacts{ReadOnlyVolume: true}, nil
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return writeFacts{CannotChange: true}, nil
	default:
		return writeFacts{}, err
	}
}
