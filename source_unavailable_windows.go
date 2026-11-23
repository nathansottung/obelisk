//go:build windows

package main

import (
	"errors"

	"golang.org/x/sys/windows"
)

// sourceUnavailable reports an open that failed because the drive or share is not
// there right now: an empty card reader or optical drive, or a network path that
// cannot be reached. Access denied is not one of these.
func sourceUnavailable(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_READY) || sourceNetworkError(err)
}

// sourceNetworkError reports an open that failed because a network path cannot be
// reached. Such a source may still be reachable under another name.
func sourceNetworkError(err error) bool {
	for _, code := range []windows.Errno{
		windows.ERROR_BAD_NETPATH,
		windows.ERROR_BAD_NET_NAME,
		windows.Errno(1222), // ERROR_NO_NETWORK
	} {
		if errors.Is(err, code) {
			return true
		}
	}
	return false
}
