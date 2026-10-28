//go:build !windows

package main

import "os"

// createOwnerOnlyTemp creates a new, uniquely named file in dir that only its
// owner can read or write (mode 0600, as os.CreateTemp creates it).
func createOwnerOnlyTemp(dir, pattern string) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
