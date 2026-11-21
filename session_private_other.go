//go:build !windows && !unix && !guionly

package main

import "errors"

var errNoPrivateFiles = errors.New("this platform has no owner-only file check, so -signin is not available")

func ensurePrivateDir(string) error  { return errNoPrivateFiles }
func verifyPrivateFile(string) error { return errNoPrivateFiles }

func ensureSigninParentPrivate(string) error { return errNoPrivateFiles }
