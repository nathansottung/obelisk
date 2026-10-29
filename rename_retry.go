package main

import (
	"os"
	"path/filepath"
	"time"
)

// renameRetryLimit bounds how long a rename (or a catalog or jobs read) that failed
// with a transient lock is retried. The limit is checked before each wait, so the total time spent is at
// most this plus one rename attempt.
const renameRetryLimit = 2 * time.Second

// renameRetryMaxWait caps the wait between attempts.
const renameRetryMaxWait = 200 * time.Millisecond

// renameReplacing gives the staged file tmp the name final, replacing any existing
// final, through renameFile. It is the one pathname rename used by every
// replace-on-publish path (catalog, jobs, configuration, keystores, app backup and
// restore, plan copies, downloaded tools). The destination guard's handle rename
// (publishDestStage) uses the same retry.
//
// A failed rename never touches final, so on any failure the existing destination is
// left as it was. tmp is left in place; removing it is the caller's decision.
func renameReplacing(tmp, final string) error {
	return retryTransientRename(func() error { return renameFile(tmp, final) })
}

// createStage creates the staging file for final: a new, uniquely named file in
// final's folder, created exclusively (os.CreateTemp uses O_EXCL), so nothing
// already in that folder is ever opened, truncated or later renamed into place.
// The file is given perm. The caller writes it, closes it and renames it to final
// (renameReplacing), and removes it on failure.
func createStage(final string, perm os.FileMode) (*os.File, error) {
	create := os.CreateTemp
	if perm&0o077 == 0 {
		// Secret material (keystores, settings, key-bearing backups): created
		// owner-only, on Windows by an ACL set at creation.
		create = createOwnerOnlyTemp
	}
	f, err := create(filepath.Dir(final), "."+filepath.Base(final)+".*.tmp")
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

// writeStage writes data into a new staging file for final (see createStage),
// flushed to disk, and returns its name.
func writeStage(final string, data []byte, perm os.FileMode) (string, error) {
	f, err := createStage(final, perm)
	if err != nil {
		return "", err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// writeSecretFile writes data to name owner-only (on Windows, an ACL granting
// only the current user, set when the file is created), staged and renamed into
// place so a partly written file never appears under the name.
func writeSecretFile(name string, data []byte) error {
	tmp, err := writeStage(name, data, 0o600)
	if err != nil {
		return err
	}
	if err := renameReplacing(tmp, name); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// readFileRetrying is os.ReadFile with the same bounded retry as renameReplacing:
// on Windows, a read that fails only because another process briefly holds the
// file open without sharing it (access denied, a sharing violation or a lock
// violation) is tried again for about two seconds. Opening the catalog and the
// jobs file use it, so a scan by antivirus or the search indexer just after a
// save does not stop Obelisk from starting. Any other error, or reaching the
// limit, returns the read's error; the bytes returned with an error are those of
// the last attempt.
func readFileRetrying(path string) ([]byte, error) {
	var b []byte
	err := retryTransientRename(func() error {
		var rerr error
		b, rerr = os.ReadFile(path)
		return rerr
	})
	return b, err
}

// retryTransientRename runs rename and, on Windows, retries it while it fails only
// because another process briefly holds the file open without share-delete —
// antivirus, the search indexer, a backup agent. Those failures (access denied, a
// sharing violation or a lock violation) are retried with a short, growing wait
// until renameRetryLimit has passed. Any other error is returned at once. When the
// limit is reached the first error is returned, unchanged, so callers can still
// inspect it. Elsewhere transientRenameError is always false and this is a single
// attempt.
func retryTransientRename(rename func() error) error {
	err := rename()
	if err == nil || !transientRenameError(err) {
		return err
	}
	first := err
	deadline := time.Now().Add(renameRetryLimit)
	wait := 5 * time.Millisecond
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return first
		}
		time.Sleep(min(wait, left))
		wait = min(wait*2, renameRetryMaxWait)
		err = rename()
		if err == nil {
			return nil
		}
		if !transientRenameError(err) {
			return err
		}
	}
}
