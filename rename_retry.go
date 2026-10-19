package main

import "time"

// renameRetryLimit bounds how long renameReplacing keeps retrying a rename that
// failed with a transient lock. The limit is checked before each wait, so the total
// time spent is at most this plus one rename attempt.
const renameRetryLimit = 2 * time.Second

// renameRetryMaxWait caps the wait between attempts.
const renameRetryMaxWait = 200 * time.Millisecond

// renameReplacing gives the staged file tmp the name final, replacing any existing
// final, through renameFile. It is the one rename used by every replace-on-publish
// path (catalog, jobs, configuration, keystores, app backup and restore, mirror and
// plan copies, downloaded tools).
//
// On Windows another process can briefly hold a just-written file open without
// FILE_SHARE_DELETE — antivirus, the search indexer, a backup agent — and the rename
// then fails with access denied, a sharing violation or a lock violation even though
// nothing is wrong. Those three errors are retried with a short, growing wait until
// renameRetryLimit has passed. Any other error is returned at once. When the limit is
// reached the first error is returned, unchanged, so callers can still inspect it.
// Elsewhere transientRenameError is always false and this is a single rename.
//
// A failed rename never touches final, so on any failure the existing destination is
// left as it was. tmp is left in place; removing it is the caller's decision.
func renameReplacing(tmp, final string) error {
	err := renameFile(tmp, final)
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
		err = renameFile(tmp, final)
		if err == nil {
			return nil
		}
		if !transientRenameError(err) {
			return err
		}
	}
}
