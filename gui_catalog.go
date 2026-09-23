package main

// Dedicated, bounded synthetic-catalog reader. Never call OpenStore here.
import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const guiCatalogMaxBytes = 4 << 20

type guiCatalogSnapshot struct {
	store  *Store
	digest string
	loaded time.Time
}

// Only this function touches the selected input. No write-capable open or path following.
func readGUICatalogFile(name string) ([]byte, error) {
	entry, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !entry.Mode().IsRegular() {
		return nil, errors.New("input must be a regular non-link file")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("input must be a regular file")
	}
	return io.ReadAll(io.LimitReader(f, guiCatalogMaxBytes+1))
}

func loadGUICatalog(name string, read func(string) ([]byte, error)) (*guiCatalogSnapshot, error) {
	raw, err := read(name)
	if err != nil {
		return nil, errors.New("catalog read refused")
	} // never parse partial bytes
	if len(raw) == 0 || len(raw) > guiCatalogMaxBytes {
		return nil, errors.New("catalog must contain 1..4194304 bytes")
	}
	var c catalog
	if err := validateGUIJSON(raw); err != nil {
		return nil, fmt.Errorf("catalog encoding refused: %w", err)
	}
	if err := validateGUIInventoryScopeEncoding(raw); err != nil {
		return nil, err
	}
	if err := decodeCatalogJSON(raw, &c); err != nil {
		return nil, errors.New("native catalog JSON is malformed")
	}
	// Native persisted File.SizeBytes is mandatory (not omitempty). Do not turn
	// missing/null input into a fabricated zero-byte observation.
	var presence struct {
		Files []map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(raw, &presence); err != nil {
		return nil, errors.New("invalid file records")
	}
	for _, f := range presence.Files {
		size, ok := f["size_bytes"]
		if !ok || string(size) == "null" {
			return nil, errors.New("file size_bytes must be recorded explicitly")
		}
	}
	if c.SchemaVersion != currentSchemaVersion {
		return nil, fmt.Errorf("only native schema %d is supported; no migration performed", currentSchemaVersion)
	}
	if err := validateGUICatalog(&c); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	// Defense in depth: Store.Search is pure; any future accidental save must fail.
	s := &Store{c: c, readOnly: true, readOnlyReason: "disposable GUI snapshot"}
	s.failSave = func() error { return errors.New("GUI snapshot persistence forbidden") }
	return &guiCatalogSnapshot{store: s, digest: hex.EncodeToString(digest[:]), loaded: time.Now().UTC()}, nil
}

func validateGUICatalog(c *catalog) error {
	if _, err := inventoryScope(c); err != nil {
		return err
	}
	if len(c.Collections) > 100 || len(c.Folders) > 100 || len(c.Chunks) > 100 || len(c.Volumes) > 100 || len(c.Locations) > 100 {
		return errors.New("catalog exceeds 100 rows per ancillary table")
	}
	occurrences := 0
	for _, ch := range c.Chunks {
		if ch != nil {
			occurrences += len(ch.Files) * len(ch.Copies)
		}
	}
	if occurrences > 1000 {
		return errors.New("catalog exceeds 1000 potential copy occurrences")
	}
	// Refuse advanced populated sections rather than partially presenting their meaning.
	allowed := map[string]bool{"SchemaVersion": true, "NextID": true, "Collections": true, "Folders": true, "Files": true, "Chunks": true, "Volumes": true, "Locations": true, "Audit": true}
	v := reflect.ValueOf(*c)
	for i := 0; i < v.NumField(); i++ {
		if !allowed[v.Type().Field(i).Name] && v.Field(i).Len() > 0 {
			return fmt.Errorf("unsupported populated section: %s", v.Type().Field(i).Name)
		}
	}
	var bounded func(reflect.Value) bool
	bounded = func(v reflect.Value) bool {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				return bounded(v.Elem())
			}
		case reflect.String:
			return len(v.String()) <= 4096
		case reflect.Slice:
			if v.Len() > 1000 {
				return false
			}
			for i := 0; i < v.Len(); i++ {
				if !bounded(v.Index(i)) {
					return false
				}
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() && !bounded(v.Field(i)) {
					return false
				}
			}
		}
		return true
	}
	if !bounded(v) {
		return errors.New("catalog exceeds row/string limits")
	}
	ids := func(rows any) (map[int]bool, error) {
		out := map[int]bool{}
		v := reflect.ValueOf(rows)
		for i := 0; i < v.Len(); i++ {
			r := v.Index(i)
			if r.IsNil() {
				return nil, errors.New("null catalog row")
			}
			id := int(r.Elem().FieldByName("ID").Int())
			if id <= 0 || out[id] {
				return nil, errors.New("invalid or duplicate catalog ID")
			}
			out[id] = true
		}
		return out, nil
	}
	collections, err := ids(c.Collections)
	if err != nil {
		return err
	}
	folders, err := ids(c.Folders)
	if err != nil {
		return err
	}
	files, err := ids(c.Files)
	if err != nil {
		return err
	}
	volumes, err := ids(c.Volumes)
	if err != nil {
		return err
	}
	locations, err := ids(c.Locations)
	if err != nil {
		return err
	}
	if _, err = ids(c.Chunks); err != nil {
		return err
	}
	folderCollection := map[int]int{}
	for _, f := range c.Folders {
		if !collections[f.CollectionID] {
			return errors.New("folder collection missing")
		}
		folderCollection[f.ID] = f.CollectionID
	}
	for _, f := range c.Files {
		if !collections[f.CollectionID] || !folders[f.FolderID] || folderCollection[f.FolderID] != f.CollectionID || f.SizeBytes < 0 {
			return errors.New("invalid file relationship/size")
		}
		if len(f.Versions) > 0 || f.EventID != 0 {
			return errors.New("retained versions/events unsupported in this slice")
		}
		if f.Hash != "" {
			b, e := hex.DecodeString(f.Hash)
			if e != nil || len(b) != 32 || f.HashAlg != "sha256" {
				return errors.New("only recorded SHA-256 supported")
			}
		}
	}
	for _, v := range c.Volumes {
		if v.LocationID != 0 && !locations[v.LocationID] {
			return errors.New("volume location missing")
		}
	}
	for _, ch := range c.Chunks {
		if !collections[ch.CollectionID] || ch.Spanned || len(ch.Segments) > 0 {
			return errors.New("chunk relationship/spanning unsupported")
		}
		for _, f := range ch.Files {
			if !files[f.FileID] {
				return errors.New("chunk file missing")
			}
		}
		for _, cp := range ch.Copies {
			if !volumes[cp.VolumeID] {
				return errors.New("copy volume missing")
			}
		}
	}
	return nil
}

func (s *guiCatalogSnapshot) projection() map[string]any {
	c := s.store.c
	collections := []map[string]any{}
	volumes := []map[string]any{}
	files := []map[string]any{}
	folders := map[int]string{}
	names := map[int]string{}
	labels := map[int]*Volume{}
	locations := map[int]string{}
	for _, l := range c.Locations {
		locations[l.ID] = l.Name
	}
	for _, f := range c.Folders {
		folders[f.ID] = f.Path
	}
	for _, co := range c.Collections {
		names[co.ID] = co.Name
		collections = append(collections, map[string]any{"id": strconv.Itoa(co.ID), "name": co.Name, "retired": co.Retired})
	}
	for _, v := range c.Volumes {
		labels[v.ID] = v
		loc := v.Location
		if v.LocationID != 0 {
			loc = locations[v.LocationID]
		}
		volumes = append(volumes, map[string]any{"id": strconv.Itoa(v.ID), "label": v.Label, "kind": v.Kind, "location": loc})
	}
	for _, f := range c.Files {
		copies := []map[string]any{}
		for _, ch := range c.Chunks {
			member := false
			for _, m := range ch.Files {
				if m.FileID == f.ID && f.Hash != "" && m.Hash == f.Hash && ch.CollectionID == f.CollectionID {
					member = true
				}
			}
			if !member {
				continue
			}
			for i, cp := range ch.Copies {
				v := labels[cp.VolumeID]
				loc := v.Location
				if v.LocationID != 0 {
					loc = locations[v.LocationID]
				}
				copies = append(copies, map[string]any{"id": fmt.Sprintf("chunk-%d-copy-%d", ch.ID, i), "chunk": ch.Name, "path": cp.Path, "volume": v.Label, "location": loc, "superseded": cp.Superseded, "recordedVerifyOK": cp.VerifyOK, "recordedVerifiedAt": cp.LastVerifiedAt})
			}
		}
		files = append(files, map[string]any{"id": strconv.Itoa(f.ID), "collection": names[f.CollectionID], "path": f.RelPath, "sourceFolder": folders[f.FolderID], "bytes": strconv.FormatInt(f.SizeBytes, 10), "hash": f.Hash, "algorithm": f.HashAlg, "firstSeen": f.FirstSeen, "copies": copies})
	}
	// Preserve native signed-int IDs and int64 sizes before any JavaScript parsing.
	scope, _ := inventoryScope(&c) // The complete snapshot was validated at adoption.
	var recordedAt any
	if scope != nil {
		recordedAt = c.Audit[0].At // Existing validated inventory event, not session load time.
	}
	// Preview-only frame: no host path interpretation or source-file access.
	// Multiple recorded roots remain browsable, but cannot be aligned implicitly.
	var frame any
	if len(c.Folders) == 1 && len(c.Collections) == 1 && c.Folders[0].Path != "" {
		frame = map[string]any{"convention": "slash-relative-v1", "root": c.Folders[0].Path, "recordCount": len(c.Files)}
	}
	return map[string]any{"schema": c.SchemaVersion, "idMax": strconv.Itoa(int(^uint(0) >> 1)), "digest": s.digest, "loadedAt": s.loaded, "recordedAt": recordedAt, "collections": collections, "volumes": volumes, "files": files, "inventoryScope": scope, "comparisonFrame": frame}
}

func runGUICatalog(args []string, input io.Reader, output io.Writer) error {
	enc := json.NewEncoder(output)
	if len(args) != 1 {
		return errors.New("usage: --gui-catalog-readonly <explicit-catalog-file>")
	}
	s, err := loadGUICatalog(args[0], readGUICatalogFile)
	if err != nil {
		_ = enc.Encode(map[string]any{"ok": false, "error": err.Error()})
		return err
	}
	if err = enc.Encode(map[string]any{"ok": true, "version": appVersion, "catalog": s.projection()}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 32768) // Bounded escaped exact names; legacy requests still capped below.
	for scanner.Scan() {
		var q struct {
			Text      string  `json:"text"`
			Hash      string  `json:"hash"`
			Exact     *string `json:"exact,omitempty"`
			Enumerate *bool   `json:"enumerate,omitempty"`
		}
		if validateGUIJSON(scanner.Bytes()) != nil || json.Unmarshal(scanner.Bytes(), &q) != nil || len(q.Text) > 256 || len(q.Hash) > 64 ||
			(q.Exact == nil && len(scanner.Bytes()) > 4096) || (q.Exact != nil && (len(*q.Exact) > 4096 || q.Text != "" || q.Hash != "")) {
			if err = enc.Encode(map[string]any{"ok": false, "error": "invalid bounded search"}); err != nil {
				return err
			}
			continue
		}
		if q.Enumerate != nil {
			if !*q.Enumerate || q.Text != "" || q.Hash != "" || q.Exact != nil {
				if err = enc.Encode(map[string]any{"ok": false, "error": "invalid enumeration"}); err != nil {
					return err
				}
				continue
			}
			// Enumerate every validated stored file, including retired collections.
			// This is deliberately independent of Search's filters and result limit.
			all := []string{}
			for _, f := range s.store.c.Files {
				all = append(all, strconv.Itoa(f.ID))
			}
			if err = enc.Encode(map[string]any{"ok": true, "ids": all, "complete": true, "count": len(all), "digest": s.digest}); err != nil {
				return err
			}
			continue
		}
		if strings.ContainsAny(q.Hash, "\r\n") {
			return errors.New("invalid hash")
		}
		ids := []string{}
		if q.Exact != nil {
			// Exact name is an explicit preview operation, not production Search's
			// trimmed/case-insensitive substring key. Identity remains the native ID.
			retired := map[int]bool{}
			for _, c := range s.store.c.Collections {
				retired[c.ID] = c.Retired
			}
			for _, f := range s.store.c.Files {
				if !retired[f.CollectionID] && f.RelPath == *q.Exact {
					ids = append(ids, strconv.Itoa(f.ID))
				}
			}
			if err = enc.Encode(map[string]any{"ok": true, "ids": ids}); err != nil {
				return err
			}
			continue
		}
		for _, r := range s.store.Search(SearchQuery{Text: q.Text, Hash: q.Hash, Limit: 1000}) {
			ids = append(ids, strconv.Itoa(r["file_id"].(int)))
		}
		if err = enc.Encode(map[string]any{"ok": true, "ids": ids}); err != nil {
			return err
		}
	}
	return scanner.Err()
}
