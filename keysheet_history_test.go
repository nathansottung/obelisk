//go:build !guionly

package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// The sheet's hand checks ask for the passphrase instead of taking it on the
// command line, so it never lands in shell history, and each one prints the
// fingerprint. Each is run here with the passphrase typed on stdin.
func TestKeySheet_HandChecksKeepThePassphraseOffTheCommandLine(t *testing.T) {
	pass := "Sheet-Check_0123456789abcdefghijklmnopqrstuv"
	fp := passFingerprint(pass)
	sheet := keySheet("K-hist", "", fp, pass)
	if strings.Contains(sheet, "THEWHOLEPASSPHRASE") {
		t.Fatal("the sheet still tells the user to type the passphrase on a command line")
	}
	cmds := map[string]string{}
	for _, ln := range strings.Split(sheet, "\n") {
		s := strings.TrimSpace(ln)
		for _, shell := range []string{"bash or zsh:", "PowerShell:"} {
			if strings.HasPrefix(s, shell) {
				cmds[shell] = strings.TrimSpace(strings.TrimPrefix(s, shell))
			}
		}
	}
	if len(cmds) != 2 {
		t.Fatalf("want a bash and a PowerShell hand check, got %v", cmds)
	}
	ran := 0
	for shell, cmd := range cmds {
		if strings.Contains(cmd, pass) {
			t.Fatalf("%s check carries the passphrase", shell)
		}
		var c *exec.Cmd
		switch shell {
		case "bash or zsh:":
			bin, err := exec.LookPath("bash")
			if git := `C:\Program Files\Git\bin\bash.exe`; runtime.GOOS == "windows" {
				if _, serr := os.Stat(git); serr == nil {
					bin, err = git, nil
				} else if err == nil && (strings.Contains(strings.ToLower(bin), "windowsapps") || strings.Contains(strings.ToLower(bin), "system32")) {
					continue // the WSL launcher, not a shell on this system
				}
			}
			if err != nil {
				continue
			}
			c = exec.Command(bin, "-c", cmd)
		case "PowerShell:":
			bin, err := exec.LookPath("pwsh")
			if err != nil {
				if bin, err = exec.LookPath("powershell"); err != nil {
					continue
				}
			}
			c = exec.Command(bin, "-NoProfile", "-Command", cmd)
		}
		c.Stdin = strings.NewReader(pass + "\n")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%s check failed: %v\n%s", shell, err, out)
		}
		if !strings.Contains(string(out), fp) {
			t.Fatalf("%s check printed %q, want the fingerprint %s", shell, out, fp)
		}
		ran++
	}
	if ran == 0 {
		t.Skip("neither bash nor PowerShell is installed")
	}
}
