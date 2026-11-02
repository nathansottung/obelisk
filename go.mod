module github.com/nathansottung/obelisk

go 1.26.0

// The Go toolchain for CI, release binaries, the container image and the Windows
// package. The CI and release workflows read this line, CI checks that the
// Dockerfile's GO_VERSION default matches it, and the Windows package builder
// refuses any other `go version`.
toolchain go1.27.0

require (
	// Code128 barcode rendering for printable volume labels. Small, pure-Go, no
	// CGO or transitive deps — the same "one static binary, hand-restorable"
	// bargain as go-qrcode. Justified in docs/ARCHITECTURE.md ("Dependencies").
	github.com/boombuler/barcode v1.1.0
	// QR rendering for key recovery cards and volume-ID labels.
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	// BLAKE3 for the hot-loop content hash (scans, drift, dock first-passes),
	// computed alongside SHA-256 in one read pass. Pure-Go with optional SIMD
	// assembly, no CGO — keeps the "one static binary" bargain. SHA-256 remains
	// the ONLY hash on media; BLAKE3 is catalog-internal (see docs/ARCHITECTURE.md).
	github.com/zeebo/blake3 v0.2.4
	// Native handle calls for the destination guard: Windows handle-relative
	// create, rename and dispose plus ACL and volume queries, and Linux renameat,
	// fstatfs and flock. Pure Go, no CGO. See docs/ARCHITECTURE.md.
	golang.org/x/sys v0.48.0
	// Unicode normalization (NFC) for comparing file names across drives and
	// platforms (classify.go): the same accented name can be stored in two forms.
	// From the Go project, pure Go, no CGO. See docs/ARCHITECTURE.md.
	golang.org/x/text v0.41.0
	// The SQLite catalog engine (catalogdb*.go): an embedded, single-file database
	// for catalogs of millions of files. A transpile of SQLite's C source to Go, so
	// no CGO. Linked into the full build only, never the launcher-only (guionly)
	// build. See docs/ARCHITECTURE.md ("Dependencies").
	modernc.org/sqlite v1.60.1
)

// klauspost/cpuid is BLAKE3's only transitive dep — a tiny pure-Go CPU-feature
// probe used to pick the SIMD path. No CGO.
require github.com/klauspost/cpuid/v2 v2.0.12 // indirect

// modernc.org/sqlite's transitive deps: its C runtime (libc, memory, mathutil) and
// small helpers. All pure Go, no CGO.
require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
