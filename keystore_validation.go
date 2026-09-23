package main

// OB-006: synchronization is not enrollment. These readers do not mutate the
// filesystem. readStore keeps its legacy first-use contract for other callers.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
)

// readExistingKeystore injects the read RESULT into the real classification
// path, not an early refusal hook. Only the key-generation workflow (including
// its build precheck) permits missing stores; status/sync require existing ones.
func (a *App) readExistingKeystore(path string, allowMissing bool) (*keystoreFile, error) {
	read := a.keystoreReadFile
	if read == nil {
		read = os.ReadFile
	}
	b, err := read(path)
	if os.IsNotExist(err) {
		if allowMissing {
			return &keystoreFile{Marker: 1}, nil
		}
		return nil, fmt.Errorf("keystore %s is missing; reconnect it or explicitly provision it through the separate first-use key-generation or recovery workflow before syncing: %w", path, err)
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read keystore %s: %w", path, err)
	}
	// Reject duplicate JSON fields before decoding: accepting the last value
	// could hide a second secret or silently lose metadata. Never echo tokens.
	if err := uniqueJSON(b); err != nil {
		return nil, fmt.Errorf("keystore %s contains malformed or duplicate-field JSON", path)
	}
	// encoding/json matches struct fields case-insensitively. Check exact names
	// first so aliases such as keys/Keys cannot overwrite one another on decode.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("keystore %s has an invalid object structure", path)
	}
	for name := range fields {
		switch name {
		case "obelisk_keystore", "mnemosyne_keystore", "schema_version", "keys":
		default:
			return nil, fmt.Errorf("keystore %s contains unsupported fields", path)
		}
	}
	var ks keystoreFile
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()             // unknown key metadata must not lose integer precision
	d.DisallowUnknownFields() // refuse future top-level fields rather than erase them
	if err := d.Decode(&ks); err != nil || (ks.Marker != 1 && ks.LegacyMarker != 1) {
		return nil, fmt.Errorf("keystore %s has an invalid or unsupported format", path)
	}
	if ks.SchemaVersion < 0 || ks.SchemaVersion > currentSchemaVersion {
		return nil, fmt.Errorf("keystore %s has an unsupported schema version", path)
	}
	for _, key := range ks.Keys {
		ref, refOK := key["key_ref"].(string)
		secret, secretOK := key["passphrase"].(string)
		if !refOK || ref == "" || !secretOK || secret == "" {
			return nil, fmt.Errorf("keystore %s contains an invalid key record", path)
		}
		if algorithm, present := key["algorithm"]; present && algorithm != "GPG-AES256" {
			return nil, fmt.Errorf("keystore %s contains an unsupported key algorithm", path)
		}
	}
	return &ks, nil
}

// uniqueJSON validates exactly one JSON value and unique field names at every
// object level, including metadata. UseNumber avoids numeric precision loss.
func uniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func() error
	value = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, compound := t.(json.Delim)
		if !compound {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate or invalid field")
				}
				seen[name] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}

// Matching secrets merge metadata by field union. Differing values for the same
// metadata field are an explicit reconciliation error, never last-wins. Unknown
// record fields are retained; unknown top-level fields are refused by the reader.
// The result is sorted by key reference, independent of participant order.
func mergeKeystores(stores []*keystoreFile) (*keystoreFile, error) {
	merged := map[string]map[string]any{}
	for _, ks := range stores {
		for _, key := range ks.Keys {
			ref := key["key_ref"].(string)
			prior := merged[ref]
			if prior == nil {
				prior = map[string]any{}
				merged[ref] = prior
			} else if prior["passphrase"] != key["passphrase"] {
				return nil, fmt.Errorf("conflicting secret material for a key reference; reconcile the keystores before syncing")
			}
			for field, v := range key {
				if old, exists := prior[field]; exists && !reflect.DeepEqual(old, v) {
					return nil, fmt.Errorf("conflicting metadata for matching key material; reconcile the keystores before syncing")
				}
				prior[field] = v
			}
		}
	}
	refs := make([]string, 0, len(merged))
	for ref := range merged {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	out := &keystoreFile{Marker: 1}
	for _, ref := range refs {
		out.Keys = append(out.Keys, merged[ref])
	}
	return out, nil
}

// The public status is strict. allowMissing is restricted to the existing
// key-generation initialization path, not evidence of replica availability.
func (a *App) keystoreStatus(paths []string, allowMissing bool) map[string]any {
	rows := []map[string]any{}
	stores := []*keystoreFile{}
	sets := []map[string]bool{}
	valid := true
	for _, path := range paths {
		row := map[string]any{"path": path, "reachable": false, "key_count": 0}
		ks, err := a.readExistingKeystore(path, allowMissing)
		if err != nil {
			row["error"] = err.Error()
			valid = false
		} else {
			row["reachable"], row["key_count"] = true, len(ks.Keys)
			stores = append(stores, ks)
			set := map[string]bool{}
			for _, key := range ks.Keys {
				set[key["key_ref"].(string)] = true
			}
			sets = append(sets, set)
		}
		rows = append(rows, row)
	}
	_, conflict := mergeKeystores(stores)
	consistent := valid && conflict == nil
	for i := 1; i < len(sets); i++ {
		if !reflect.DeepEqual(sets[0], sets[i]) {
			consistent = false
		}
	}
	reason := ""
	switch {
	case conflict != nil:
		reason = conflict.Error()
	case !valid:
		reason = "One or more keystores are missing, unreadable or invalid; reconnect or reconcile them before syncing."
	case len(paths) < MinKeystores:
		reason = fmt.Sprintf("Only %d keystore path(s) registered; %d required.", len(paths), MinKeystores)
	case !consistent:
		reason = "Keystores hold different key sets; run key sync."
	}
	return map[string]any{"ok": len(paths) >= MinKeystores && consistent, "reason": reason, "min_required": MinKeystores, "stores": rows}
}
