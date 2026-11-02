//go:build !guionly

package main

// catalogdb.go — the SQLite catalog engine (an embedded, single-file database).
// It is being built in slices: this file is the open layer. Nothing in the app
// uses it yet; catalog.json stays the live catalog until the cutover slice.
//
// The driver is modernc.org/sqlite: pure Go, no CGO. It is linked into the full
// build only, never the launcher-only (-tags guionly) build.

import (
	_ "modernc.org/sqlite"
)

// catalogDBDriver is the database/sql driver name modernc.org/sqlite registers.
const catalogDBDriver = "sqlite"
