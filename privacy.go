package main

// privacy.go — reading a chunk's file listing back from a mounted medium. This
// is the catalog-loss fallback: a found tape whose manifest is encrypted
// (private media), decrypted with the passphrase from the keystores. If the
// keystores are gone too, RESTORE.txt on the medium documents the manual step.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadMediumManifest locates <chunk>.manifest.json[.gpg] under mountPath (flat
// or in a NAME/ subfolder), decrypting an encrypted one with the keystore
// passphrase, and returns the parsed manifest. It never touches the catalog for
// the listing itself — this works even if the catalog is gone.
func (a *App) ReadMediumManifest(chunkID int, mountPath string) (map[string]any, error) {
	c := a.Store.Chunk(chunkID)
	if c == nil {
		return nil, fmt.Errorf("package %d not found", chunkID)
	}
	if strings.TrimSpace(mountPath) == "" {
		return nil, fmt.Errorf("mount path required (point at the mounted tape/disc/drive)")
	}
	encPaths := []string{
		filepath.Join(mountPath, c.Name+".manifest.json.gpg"),
		filepath.Join(mountPath, c.Name, c.Name+".manifest.json.gpg"),
	}
	for _, p := range encPaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		gpgBin, err := a.tool("gpg")
		if err != nil {
			return nil, err
		}
		// This medium may hold an older build: each key a copy recorded is tried.
		var plain []byte
		var decErr error
		reached := false
		for _, ref := range c.keyRefs() {
			pass, perr := a.Passphrase(ref)
			if perr != nil {
				continue
			}
			reached = true
			if plain, decErr = gpgDecryptToMem(gpgBin, pass, p); decErr == nil {
				break
			}
		}
		if !reached {
			return nil, fmt.Errorf("found an ENCRYPTED manifest (%s) but no reachable keystore holds key %s. "+
				"If the keystores are also gone, decrypt it by hand as RESTORE.txt on the medium says: "+
				"gpg -d %s", filepath.Base(p), strings.Join(c.keyRefs(), " or "), filepath.Base(p))
		}
		if decErr != nil {
			return nil, fmt.Errorf("decrypt failed: %w", decErr)
		}
		var m map[string]any
		if err := json.Unmarshal(plain, &m); err != nil {
			return nil, fmt.Errorf("decrypted manifest is not valid JSON: %w", err)
		}
		m["_source"], m["_encrypted"] = p, true
		return m, nil
	}
	// plaintext fallback (non-private chunks)
	for _, p := range []string{
		filepath.Join(mountPath, c.Name+".manifest.json"),
		filepath.Join(mountPath, c.Name, c.Name+".manifest.json"),
	} {
		if b, err := os.ReadFile(p); err == nil {
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return nil, fmt.Errorf("manifest is not valid JSON: %w", err)
			}
			m["_source"], m["_encrypted"] = p, false
			return m, nil
		}
	}
	return nil, fmt.Errorf("no manifest (%[1]s.manifest.json or .gpg) found under %s", c.Name, mountPath)
}

func gpgDecryptToMem(gpgBin, pass, path string) ([]byte, error) {
	s, err := newGpgSession(gpgBin)
	if err != nil {
		return nil, err
	}
	defer s.close()
	cmd := helperCommand(gpgBin, s.decryptArgs(path)...)
	cmd.Stdin = strings.NewReader(pass)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, tail(errb.String(), 300))
	}
	return out.Bytes(), nil
}
