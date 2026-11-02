package main

// catalogChangesLost is a commit of the catalog database that failed: what its
// transaction held is gone (see Store.lostErr).
type catalogChangesLost struct{ err error }

func (e *catalogChangesLost) Error() string {
	return "the catalog database lost changes when a save failed (" + e.err.Error() + "). Records still in " +
		"memory are written again by the next save, but file changes made since the last save are gone: restart " +
		"Obelisk and run the affected scans again"
}

func (e *catalogChangesLost) Unwrap() error { return e.err }
