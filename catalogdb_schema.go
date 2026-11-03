//go:build !guionly

package main

// catalogdb_schema.go — the tables of catalog database schema v1.
//
// Conventions:
//   - Every table is STRICT. Hashes the catalog looks files up by (file, version,
//     package listing, drive snapshot, plan) are 32-byte BLOBs; the import refuses
//     any that is not 64 lowercase hex characters.
//   - Times are ANY columns: INTEGER Unix nanoseconds in UTC when the time fits
//     (about 1678 to 2261), and otherwise its exact RFC 3339 text in UTC (the zero
//     FILETIME of 1601 a Windows file can carry, a camera clock set to 2300). A zero
//     time is NULL; a pointer time is NULL when nil and catalogDBZeroTime when it
//     points at a zero time.
//   - Records with an ID keep it as the primary key (AUTOINCREMENT, so an ID is
//     never reused) and an ord column for their order in the catalog. Nested lists
//     are child tables keyed by their parent and their position.
//   - Small records nobody queries by field yet are kept whole as JSON in a doc
//     column, one row each; later slices give them columns as they move off JSON.
//   - Cross-record references (a file's folder, a copy's volume) are indexed but
//     not foreign keys: a catalog can hold a reference to a record it no longer
//     has, and the import must keep it exactly. A parent's own nested rows are
//     foreign keys that go with it.

// catalogDBDocTables are the tables that hold one catalog record per row as JSON,
// with the catalog field each comes from.
var catalogDBDocTables = []struct{ table, field string }{
	{"key_meta", "Keys"},
	{"burn_queue", "BurnQueues"},
	{"dock_session", "DockSessions"},
	{"template", "Templates"},
	{"tape_check", "TapeChecks"},
	{"profile", "Profiles"},
	{"assignment", "Assignments"},
	{"protection_summary", "Protection"},
	{"conflict", "Conflicts"},
	{"quarantine_entry", "Quarantine"},
	{"backup_session", "BackupSessions"},
	{"destination_choice", "DestinationChoices"},
}

// catalogDBSequences maps each AUTOINCREMENT table to the catalog's next_id
// counter it continues.
var catalogDBSequences = map[string]string{
	"collection": "collection", "folder": "folder", "file": "file", "chunk": "chunk",
	"volume": "volume", "location": "location", "event": "event", "plan": "plan", "scan": "scan",
	"compare_run": "compare",
}

const catalogDBSchemaV1 = `
CREATE TABLE collection(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, name TEXT NOT NULL, created_at ANY,
	kind TEXT NOT NULL, integrity TEXT, retired INTEGER NOT NULL, retire_hidden INTEGER NOT NULL, retired_at ANY
) STRICT;

CREATE TABLE folder(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, collection_id INTEGER NOT NULL,
	path TEXT NOT NULL, volume_id TEXT NOT NULL
) STRICT;
CREATE INDEX folder_collection ON folder(collection_id);

CREATE TABLE file(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, collection_id INTEGER NOT NULL,
	folder_id INTEGER NOT NULL, rel_path TEXT NOT NULL, size_bytes INTEGER NOT NULL, hash_alg TEXT NOT NULL,
	sha256 BLOB CHECK(sha256 IS NULL OR length(sha256) = 32),
	blake3 BLOB CHECK(blake3 IS NULL OR length(blake3) = 32),
	mtime ANY, first_seen ANY, shot_at ANY, role TEXT NOT NULL, camera_serial TEXT NOT NULL,
	event_id INTEGER NOT NULL
) STRICT;
-- Not unique: a sourceless archive's union of drives (folder 0) keeps two files
-- at one path when their bytes differ.
CREATE INDEX file_path ON file(collection_id, folder_id, rel_path);
-- An archive's files by path, and every file in catalog order: the pages the
-- store's file iterators read.
CREATE INDEX file_collection_path ON file(collection_id, rel_path, ord);
CREATE INDEX file_ord ON file(ord);
CREATE INDEX file_collection_ord ON file(collection_id, ord);
CREATE INDEX file_sha256 ON file(sha256);
CREATE INDEX file_blake3 ON file(blake3);
CREATE INDEX file_event ON file(event_id) WHERE event_id <> 0;
CREATE INDEX file_collection_size ON file(collection_id, size_bytes);

CREATE TABLE file_version(
	file_id INTEGER NOT NULL REFERENCES file(id) ON DELETE CASCADE, seq INTEGER NOT NULL,
	sha256 BLOB CHECK(sha256 IS NULL OR length(sha256) = 32), hash_alg TEXT NOT NULL,
	size_bytes INTEGER NOT NULL, mtime ANY, first_seen ANY, superseded_at ANY,
	PRIMARY KEY(file_id, seq)
) STRICT, WITHOUT ROWID;
CREATE INDEX file_version_sha256 ON file_version(sha256);

CREATE TABLE chunk(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, collection_id INTEGER NOT NULL,
	name TEXT NOT NULL, status TEXT NOT NULL, media_kind TEXT NOT NULL, target_bytes INTEGER NOT NULL,
	data_bytes INTEGER NOT NULL, enc_bytes INTEGER NOT NULL, file_count INTEGER NOT NULL, src_root TEXT NOT NULL,
	hash_alg TEXT NOT NULL, tar_hash TEXT NOT NULL, enc_hash TEXT NOT NULL, encrypted INTEGER NOT NULL,
	key_ref TEXT NOT NULL, private_manifest INTEGER NOT NULL, par2 INTEGER NOT NULL, staged_dir TEXT NOT NULL,
	written_dest TEXT NOT NULL, verify_ok INTEGER, error TEXT NOT NULL, sidecars TEXT, mirror_root TEXT NOT NULL,
	build_timings TEXT, build_verified TEXT, ring_stats TEXT, build_warnings TEXT, spanned INTEGER NOT NULL,
	adopted INTEGER NOT NULL, mirror INTEGER NOT NULL, listing_unknown INTEGER NOT NULL, created_at ANY,
	written_at ANY, verified_at ANY
) STRICT;
CREATE INDEX chunk_collection ON chunk(collection_id);
CREATE INDEX chunk_status ON chunk(status);

-- A package's file list (list 0) and the files an earlier scan found but a later
-- one could not read (list 1). rel_path, size_bytes and sha256 are NULL when they
-- equal the referenced file's own values; sha256 is a zero-length BLOB for an
-- empty hash that differs from the file's.
CREATE TABLE chunk_file(
	chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE, list INTEGER NOT NULL,
	ord INTEGER NOT NULL, file_id INTEGER NOT NULL, rel_path TEXT, size_bytes INTEGER,
	sha256 BLOB CHECK(sha256 IS NULL OR length(sha256) IN (0, 32)),
	sample_sha256 BLOB CHECK(sample_sha256 IS NULL OR length(sample_sha256) = 32),
	PRIMARY KEY(chunk_id, list, ord)
) STRICT, WITHOUT ROWID;
CREATE INDEX chunk_file_file ON chunk_file(file_id);
CREATE INDEX chunk_file_sha256 ON chunk_file(sha256) WHERE sha256 IS NOT NULL;

-- A listing that shares its file's path, size or hash keeps the values it had
-- when the file changes or is removed: a package holds the bytes it was built
-- from, not the file's new ones. The shared values are written into the listing
-- first (an empty hash as a zero-length BLOB, which is not NULL).
CREATE TRIGGER file_update_keeps_listings BEFORE UPDATE OF rel_path, size_bytes, sha256 ON file
WHEN OLD.rel_path IS NOT NEW.rel_path OR OLD.size_bytes IS NOT NEW.size_bytes OR OLD.sha256 IS NOT NEW.sha256
BEGIN
	UPDATE chunk_file SET rel_path = coalesce(rel_path, OLD.rel_path),
		size_bytes = coalesce(size_bytes, OLD.size_bytes),
		sha256 = coalesce(sha256, OLD.sha256, zeroblob(0))
	WHERE file_id = OLD.id AND (rel_path IS NULL OR size_bytes IS NULL OR sha256 IS NULL);
END;
CREATE TRIGGER file_delete_keeps_listings BEFORE DELETE ON file
BEGIN
	UPDATE chunk_file SET rel_path = coalesce(rel_path, OLD.rel_path),
		size_bytes = coalesce(size_bytes, OLD.size_bytes),
		sha256 = coalesce(sha256, OLD.sha256, zeroblob(0))
	WHERE file_id = OLD.id AND (rel_path IS NULL OR size_bytes IS NULL OR sha256 IS NULL);
END;

CREATE TABLE copy(
	chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE, ord INTEGER NOT NULL,
	volume_id INTEGER NOT NULL, path TEXT NOT NULL, written_at ANY, last_verified_at ANY,
	verify_ok INTEGER, last_check_at ANY, last_check_level TEXT NOT NULL, last_check_ok INTEGER,
	superseded INTEGER NOT NULL,
	PRIMARY KEY(chunk_id, ord)
) STRICT, WITHOUT ROWID;
CREATE INDEX copy_volume ON copy(volume_id);

CREATE TABLE segment(
	chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE, ord INTEGER NOT NULL,
	idx INTEGER NOT NULL, bytes INTEGER NOT NULL, hash TEXT NOT NULL, status TEXT NOT NULL, dest TEXT NOT NULL,
	volume_id INTEGER NOT NULL, par2 INTEGER NOT NULL,
	PRIMARY KEY(chunk_id, ord)
) STRICT, WITHOUT ROWID;
CREATE INDEX segment_volume ON segment(volume_id);

CREATE TABLE verify_event(
	chunk_id INTEGER NOT NULL REFERENCES chunk(id) ON DELETE CASCADE, seq INTEGER NOT NULL,
	at ANY, ok INTEGER NOT NULL, path TEXT NOT NULL, note TEXT NOT NULL, level TEXT NOT NULL,
	advisory INTEGER NOT NULL,
	PRIMARY KEY(chunk_id, seq)
) STRICT, WITHOUT ROWID;
CREATE INDEX verify_event_at ON verify_event(at);

CREATE TABLE volume(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, label TEXT NOT NULL, barcode TEXT NOT NULL,
	kind TEXT NOT NULL, location TEXT NOT NULL, location_id INTEGER NOT NULL, offsite INTEGER NOT NULL,
	notes TEXT NOT NULL, created_at ANY, serial TEXT NOT NULL, model TEXT NOT NULL,
	device_size INTEGER NOT NULL, device_note TEXT NOT NULL, device_at ANY, partition_id TEXT NOT NULL,
	sealed INTEGER NOT NULL, sealed_at ANY, drive_encrypted INTEGER NOT NULL, drive_enc_note TEXT NOT NULL
) STRICT;
CREATE INDEX volume_identity ON volume(serial, partition_id);
CREATE INDEX volume_barcode ON volume(barcode);

CREATE TABLE smart_snapshot(
	volume_id INTEGER NOT NULL REFERENCES volume(id) ON DELETE CASCADE, seq INTEGER NOT NULL,
	doc TEXT NOT NULL, PRIMARY KEY(volume_id, seq)
) STRICT, WITHOUT ROWID;

CREATE TABLE finalization(
	volume_id INTEGER NOT NULL REFERENCES volume(id) ON DELETE CASCADE, seq INTEGER NOT NULL,
	doc TEXT NOT NULL, PRIMARY KEY(volume_id, seq)
) STRICT, WITHOUT ROWID;

CREATE TABLE location(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, name TEXT NOT NULL, offsite INTEGER NOT NULL,
	notes TEXT NOT NULL, created_at ANY
) STRICT;

CREATE TABLE volume_snapshot(
	volume_id INTEGER PRIMARY KEY, ord INTEGER NOT NULL, captured_at ANY, session_id INTEGER NOT NULL,
	label TEXT NOT NULL, serial TEXT NOT NULL, model TEXT NOT NULL, device_size INTEGER NOT NULL,
	location_id INTEGER NOT NULL, smart TEXT, total_files INTEGER NOT NULL, total_bytes INTEGER NOT NULL,
	unreadable INTEGER NOT NULL, root TEXT NOT NULL, scan TEXT NOT NULL, unreadable_folders TEXT,
	unreadable_files TEXT, skipped_folders TEXT, role_files TEXT, role_bytes TEXT
) STRICT;

CREATE TABLE snap_file(
	volume_id INTEGER NOT NULL REFERENCES volume_snapshot(volume_id) ON DELETE CASCADE, ord INTEGER NOT NULL,
	rel_path TEXT NOT NULL, size_bytes INTEGER NOT NULL, mtime ANY,
	sha256 BLOB CHECK(sha256 IS NULL OR length(sha256) = 32),
	blake3 BLOB CHECK(blake3 IS NULL OR length(blake3) = 32),
	role TEXT NOT NULL, critical INTEGER NOT NULL, shot_at ANY, camera_serial TEXT NOT NULL,
	PRIMARY KEY(volume_id, ord)
) STRICT, WITHOUT ROWID;
CREATE INDEX snap_file_path ON snap_file(volume_id, rel_path);
CREATE INDEX snap_file_sha256 ON snap_file(sha256);

CREATE TABLE scan(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, volume_id INTEGER NOT NULL, at ANY,
	session_id INTEGER NOT NULL, root TEXT NOT NULL, label TEXT NOT NULL, serial TEXT NOT NULL,
	scan TEXT NOT NULL, total_files INTEGER NOT NULL, total_bytes INTEGER NOT NULL,
	unreadable_folders INTEGER NOT NULL, unreadable_files INTEGER NOT NULL, skipped_folders INTEGER NOT NULL,
	file_list TEXT NOT NULL, file_list_sha256 TEXT NOT NULL
) STRICT;
CREATE INDEX scan_volume ON scan(volume_id, at);

-- A saved comparison of two scans, or of a scan and an archive. Its per-file
-- results stay in the compares folder, named here with their SHA-256.
CREATE TABLE compare_run(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, at ANY, ref_scan_id INTEGER NOT NULL,
	ref_volume_id INTEGER NOT NULL, ref_archive_id INTEGER NOT NULL, ref_label TEXT NOT NULL,
	ref_complete INTEGER NOT NULL, ref_root TEXT NOT NULL, other_scan_id INTEGER NOT NULL,
	other_volume_id INTEGER NOT NULL, other_label TEXT NOT NULL, other_complete INTEGER NOT NULL,
	other_root TEXT NOT NULL, counts TEXT, total INTEGER NOT NULL, result_file TEXT NOT NULL,
	result_sha256 TEXT NOT NULL
) STRICT;
CREATE INDEX compare_run_at ON compare_run(at);

CREATE TABLE event(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, name TEXT NOT NULL, event_type TEXT NOT NULL,
	year INTEGER NOT NULL, capture_start ANY, capture_end ANY, notes TEXT NOT NULL,
	collection_id INTEGER NOT NULL, folder_hint TEXT NOT NULL, source TEXT NOT NULL, created_at ANY
) STRICT;
CREATE INDEX event_capture ON event(capture_start);

CREATE TABLE audit(
	id INTEGER PRIMARY KEY, at ANY, action TEXT NOT NULL, detail TEXT NOT NULL
) STRICT;
CREATE INDEX audit_at ON audit(at);

CREATE TABLE plan(
	id INTEGER PRIMARY KEY AUTOINCREMENT, ord INTEGER NOT NULL, name TEXT NOT NULL, archive_ids TEXT,
	scope_prefix TEXT NOT NULL, template_id INTEGER NOT NULL, destination_root TEXT NOT NULL,
	status TEXT NOT NULL, dest_volume_id INTEGER NOT NULL, created_at ANY, compiled_at ANY,
	closed_at ANY
) STRICT;

-- One row per content hash a plan mentions. Each column is NULL when the hash is
-- not in that one of the plan's maps.
CREATE TABLE plan_entry(
	plan_id INTEGER NOT NULL REFERENCES plan(id) ON DELETE CASCADE,
	sha256 BLOB NOT NULL CHECK(length(sha256) = 32),
	override TEXT, parked INTEGER, mapped TEXT, satisfied INTEGER,
	PRIMARY KEY(plan_id, sha256)
) STRICT, WITHOUT ROWID;

CREATE TABLE drift_report(
	collection_id INTEGER PRIMARY KEY, ord INTEGER NOT NULL, at ANY, counts TEXT, info_counts TEXT,
	by_ext TEXT, unread TEXT
) STRICT;

CREATE TABLE drift_item(
	collection_id INTEGER NOT NULL REFERENCES drift_report(collection_id) ON DELETE CASCADE,
	seq INTEGER NOT NULL, state TEXT NOT NULL, path TEXT NOT NULL, ext TEXT NOT NULL, hash TEXT NOT NULL,
	moved_from TEXT NOT NULL, chunk TEXT NOT NULL, volumes TEXT, needs_backup INTEGER NOT NULL,
	informational INTEGER NOT NULL, file_id INTEGER NOT NULL, prior_hash TEXT NOT NULL,
	prior_version TEXT NOT NULL,
	PRIMARY KEY(collection_id, seq)
) STRICT, WITHOUT ROWID;
`

// catalogDBDocTableDDL is the shared shape of the doc tables.
const catalogDBDocTableDDL = `CREATE TABLE %s(ord INTEGER PRIMARY KEY, doc TEXT NOT NULL) STRICT`
