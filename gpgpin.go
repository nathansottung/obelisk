package main

// gpgpin.go — every gpg run Obelisk makes, pinned. The output of a package
// build does not depend on the local gpg's defaults, configuration or agent:
//
//   - AES-256, with an iterated and salted SHA-512 S2K at a fixed count (the
//     maximum, 65011712), and no compression;
//   - a fresh, empty home folder for each run (--homedir, --no-options), so no
//     gpg.conf and no keyring of the operator's takes part, and the folder is
//     removed afterwards;
//   - no passphrase cache (--no-symkey-cache), so the agent never keeps a
//     package passphrase.
//
// RESTORE.txt's plain `gpg -d` decrypts the result with any gpg 2.x.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// gpgS2KCount is the fixed S2K iteration count: the largest OpenPGP can encode.
const gpgS2KCount = "65011712"

// gpgSession is one isolated gpg run: its own home folder, and the arguments
// that use it.
type gpgSession struct {
	bin  string
	home string
}

func newGpgSession(gpgBin string) (*gpgSession, error) {
	home, err := os.MkdirTemp("", "obelisk-gpg-")
	if err != nil {
		return nil, fmt.Errorf("making a home folder for gpg: %w", err)
	}
	_ = os.Chmod(home, 0o700)
	return &gpgSession{bin: gpgBin, home: home}, nil
}

// args are the options every run takes: the isolated home, no options file,
// batch mode, quiet (so stderr holds only warnings and errors), no passphrase
// cache, and the passphrase read from stdin.
func (s *gpgSession) args() []string {
	return []string{"--homedir", s.home, "--no-options", "--batch", "--quiet", "--yes",
		"--no-symkey-cache", "--pinentry-mode", "loopback", "--passphrase-fd", "0"}
}

// encryptArgs encrypt in to out with the pinned symmetric settings.
func (s *gpgSession) encryptArgs(out, in string) []string {
	return append(s.args(), "--symmetric",
		"--cipher-algo", "AES256", "--s2k-cipher-algo", "AES256",
		"--s2k-mode", "3", "--s2k-digest-algo", "SHA512", "--s2k-count", gpgS2KCount,
		"--compress-algo", "none", "-o", out, in)
}

// decryptArgs decrypt in to stdout.
func (s *gpgSession) decryptArgs(in string) []string {
	return append(s.args(), "-d", in)
}

// close stops any agent gpg started for the home folder and removes it.
func (s *gpgSession) close() {
	conf := filepath.Join(filepath.Dir(s.bin), "gpgconf")
	if runtime.GOOS == "windows" {
		conf += ".exe"
	}
	if _, err := os.Stat(conf); err == nil {
		_ = helperCommand(conf, "--homedir", s.home, "--kill", "all").Run()
	}
	// An agent on Windows can take a moment to let go of its files.
	for i := 0; i < 20; i++ {
		if err := os.RemoveAll(s.home); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// gpgEncrypt encrypts in to out with pass, pinned and isolated.
func gpgEncrypt(gpgBin, pass, out, in string) error {
	s, err := newGpgSession(gpgBin)
	if err != nil {
		return err
	}
	defer s.close()
	return run(gpgBin, pass, s.encryptArgs(out, in)...)
}
