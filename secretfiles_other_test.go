//go:build !windows && !guionly

package main

func ownerOnlyACLForTest(string) error { return nil }
