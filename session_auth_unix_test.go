//go:build unix && !guionly

package main

import (
	"os/exec"
	"syscall"
)

// runAsNobody runs a command as an unprivileged second account.
func runAsNobody(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	return cmd.Run()
}
