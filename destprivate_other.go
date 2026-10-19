//go:build !windows

package main

import "fmt"

// "Make this folder private" changes Windows folder permissions; elsewhere the
// notice does not offer it.
const destMakePrivateSupported = false

func planPrivateFolder(string, []string) (*destPrivateChange, error) {
	return nil, fmt.Errorf("making a folder private is only available on Windows")
}

func applyPrivateFolder(string, *destPrivateChange) error {
	return fmt.Errorf("making a folder private is only available on Windows")
}
