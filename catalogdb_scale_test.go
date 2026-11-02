//go:build !guionly

package main

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCatalogDBEngineScale measures the SQLite engine itself at catalog scale: bulk
// insert of file rows and copy rows, index build, point lookups, a folder listing,
// a full-table search, totals, VACUUM INTO (the backup), quick_check and the cost
// of one durable commit. It is the engine half of the decision gate in
// docs/ARCHITECTURE.md; TestCatalogDBRoundTrip1M measures the import itself.
//
// SKIPPED by default; run it with:
//
//	OBELISK_SCALE=1 go test -run TestCatalogDBEngineScale -v -timeout 30m
//
// Override the size with OBELISK_FILES.
func TestCatalogDBEngineScale(t *testing.T) {
	if os.Getenv("OBELISK_SCALE") == "" {
		t.Skip("set OBELISK_SCALE=1 to run the SQLite engine benchmark")
	}
	n := envInt("OBELISK_FILES", 1_000_000)
	dir := t.TempDir()
	path := filepath.Join(dir, "bench.db")
	db, err := sql.Open(catalogDBDriver, path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"+
		"&_pragma=busy_timeout(5000)&_pragma=cache_size(-262144)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`CREATE TABLE file(id INTEGER PRIMARY KEY, collection_id INTEGER NOT NULL, folder_id INTEGER NOT NULL,
		rel_path TEXT NOT NULL, size_bytes INTEGER NOT NULL, sha256 BLOB, blake3 BLOB, mtime_ns INTEGER,
		UNIQUE(collection_id, folder_id, rel_path)) STRICT`)
	exec(`CREATE TABLE chunk_file(chunk_id INTEGER NOT NULL, ord INTEGER NOT NULL, file_id INTEGER,
		sample_sha256 BLOB, PRIMARY KEY(chunk_id, ord)) STRICT, WITHOUT ROWID`)
	rel := func(i int) string { return fmt.Sprintf("Clients/2019/Event %04d/RAW/DSC_%08d.NEF", i%5000, i) }
	blob := func(i int) []byte { b, _ := hex.DecodeString(fakeHash(i)); return b }

	insert := func(q string, row func(i int) []any) time.Duration {
		t0 := time.Now()
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		st, err := tx.Prepare(q)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i <= n; i++ {
			if _, err := st.Exec(row(i)...); err != nil {
				t.Fatal(err)
			}
		}
		st.Close()
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return time.Since(t0)
	}
	mtime := time.Date(2024, 3, 12, 14, 5, 9, 0, time.UTC).UnixNano()
	insFiles := insert(`INSERT INTO file VALUES(?,?,?,?,?,?,?,?)`, func(i int) []any {
		return []any{i, 1 + i%4, 1 + i%40, rel(i), 38_000_000 + i%1000, blob(i), blob(i + 1), mtime}
	})
	insRefs := insert(`INSERT INTO chunk_file VALUES(?,?,?,?)`, func(i int) []any {
		return []any{1 + i%10, i, i, blob(i + 7)}
	})
	t0 := time.Now()
	exec(`CREATE INDEX file_sha256 ON file(sha256)`)
	exec(`CREATE INDEX file_size ON file(collection_id, size_bytes)`)
	exec(`CREATE INDEX chunk_file_file ON chunk_file(file_id)`)
	indexes := time.Since(t0)
	exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	query := func(q string, args ...any) (time.Duration, int) {
		t.Helper()
		t0 := time.Now()
		rows, err := db.Query(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		c := 0
		for rows.Next() {
			c++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		return time.Since(t0), c
	}
	k := n / 2
	byKey, c1 := query(`SELECT id FROM file WHERE collection_id=? AND folder_id=? AND rel_path=?`, 1+k%4, 1+k%40, rel(k))
	byHash, c2 := query(`SELECT id FROM file WHERE sha256=?`, blob(k))
	copies, c3 := query(`SELECT c.chunk_id FROM chunk_file c JOIN file f ON f.id=c.file_id WHERE f.sha256=?`, blob(k))
	listing, _ := query(`SELECT id FROM file WHERE collection_id=2 AND folder_id=2 AND rel_path >= ? AND rel_path < ?`,
		"Clients/2019/Event 0041/", "Clients/2019/Event 00410")
	like, _ := query(`SELECT id FROM file WHERE rel_path LIKE ? LIMIT 50`, "%DSC_00000042%")
	totals, _ := query(`SELECT collection_id, count(*), sum(size_bytes) FROM file GROUP BY collection_id`)
	if c1 != 1 || c2 != 1 || c3 != 1 {
		t.Fatalf("lookups found %d, %d, %d rows; want 1 each", c1, c2, c3)
	}
	t0 = time.Now()
	exec(`VACUUM INTO ?`, filepath.Join(dir, "backup.db"))
	vacuum := time.Since(t0)
	t0 = time.Now()
	var ok string
	if err := db.QueryRow(`PRAGMA quick_check`).Scan(&ok); err != nil || ok != "ok" {
		t.Fatalf("quick_check: %q %v", ok, err)
	}
	check := time.Since(t0)
	t0 = time.Now()
	for i := 1; i <= 200; i++ {
		exec(`UPDATE file SET mtime_ns=mtime_ns+1 WHERE id=?`, i)
	}
	commit := time.Since(t0) / 200

	t.Logf("\n"+
		"=== SQLite engine benchmark (%s file rows, %s copy rows) ===\n"+
		"insert file rows        %10s  (%s rows/s)\n"+
		"insert copy rows        %10s  (%s rows/s)\n"+
		"build indexes           %10s\n"+
		"database size           %10.1f MB\n"+
		"lookup by key           %10s\n"+
		"lookup by SHA-256       %10s\n"+
		"copies of a hash        %10s\n"+
		"folder listing          %10s\n"+
		"substring path search   %10s\n"+
		"totals per archive      %10s\n"+
		"VACUUM INTO (backup)    %10s\n"+
		"quick_check             %10s\n"+
		"one durable commit      %10s",
		comma(n), comma(n), round(insFiles), comma(int(float64(n)/insFiles.Seconds())),
		round(insRefs), comma(int(float64(n)/insRefs.Seconds())), round(indexes),
		float64(fi.Size())/1e6, byKey, byHash, copies, listing, round(like), round(totals),
		round(vacuum), round(check), commit)
}
