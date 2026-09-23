package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const guiInventoryAuditAction = "GUI_DISPOSABLE_INVENTORY_V1"

var errGUIInventoryScope = errors.New("unsupported or malformed inventory audit scope")

// Decode member names before checking them, so escaped canonical spellings work
// and encoded duplicates cannot evade the same exact-key policy. Values remain
// raw until every required member has appeared exactly once without a null.
func guiScopeMembers(raw []byte, required ...string) (map[string]json.RawMessage, error) {
	if validateGUIJSON(raw) != nil {
		return nil, errGUIInventoryScope
	}
	allowed := make(map[string]bool, len(required))
	for _, key := range required {
		allowed[key] = true
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil, errGUIInventoryScope
	}
	members := make(map[string]json.RawMessage, len(required))
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || members[key] != nil {
			return nil, errGUIInventoryScope
		}
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errGUIInventoryScope
		}
		members[key] = value
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || len(members) != len(required) || d.Decode(new(any)) != io.EOF {
		return nil, errGUIInventoryScope
	}
	return members, nil
}

// Inspect only the native audit path before the shared struct decoder can fold
// keys or discard duplicates. Other catalog member policies remain unchanged.
// Missing audit, audit:null and audit:[] are the existing no-history forms;
// an alias, duplicate wrapper or malformed event must never become legacy scope.
func validateGUIInventoryScopeEncoding(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return errGUIInventoryScope
	}
	seen := false
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return errGUIInventoryScope
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return errGUIInventoryScope
		}
		if !strings.EqualFold(key, "audit") {
			continue
		}
		if key != "audit" || seen {
			return errGUIInventoryScope
		}
		seen = true
		var events []json.RawMessage
		if json.Unmarshal(value, &events) != nil || len(events) > 1 {
			return errGUIInventoryScope
		}
		for _, event := range events {
			members, err := guiScopeMembers(event, "at", "action", "detail")
			if err != nil {
				return err
			}
			var detail string
			if json.Unmarshal(members["detail"], &detail) != nil {
				return errGUIInventoryScope
			}
			if _, err = guiInventoryDetailMembers([]byte(detail)); err != nil {
				return err
			}
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return errGUIInventoryScope
	}
	return nil
}

func guiInventoryDetailMembers(raw []byte) (map[string]json.RawMessage, error) {
	return guiScopeMembers(raw, "version", "policy", "entries", "regularFiles", "includedFiles", "excludedFiles", "readBytes", "complete")
}

// A versioned action detail in the existing native Audit history. It is not a
// new catalog schema or an unrelated field repurposed as an exclusion sidecar.
type guiInventoryScope struct {
	Version       int    `json:"version"`
	Policy        string `json:"policy"`
	Entries       int    `json:"entries"`
	RegularFiles  int    `json:"regularFiles"`
	IncludedFiles int    `json:"includedFiles"`
	ExcludedFiles int    `json:"excludedFiles"`
	ReadBytes     int64  `json:"readBytes"`
	Complete      bool   `json:"complete"`
}

func inventoryScope(c *catalog) (*guiInventoryScope, error) {
	if len(c.Audit) == 0 {
		return nil, nil // Unknown historical policy/counts, never fabricated OFF.
	}
	bad := errGUIInventoryScope
	if len(c.Audit) != 1 || c.Audit[0].Action != guiInventoryAuditAction || c.Audit[0].At.IsZero() || len(c.Audit[0].Detail) > 1024 {
		return nil, bad
	}
	raw := []byte(c.Audit[0].Detail)
	if _, err := guiInventoryDetailMembers(raw); err != nil {
		return nil, bad
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var scope guiInventoryScope
	if d.Decode(&scope) != nil || d.Decode(new(any)) != io.EOF {
		return nil, bad
	}
	if scope.Version != 1 || !scope.Complete || (scope.Policy != "include-all" && scope.Policy != "ignore-exact-ds-store") ||
		scope.Entries < 0 || scope.Entries > 128 || scope.RegularFiles < 0 || scope.RegularFiles > 64 || scope.RegularFiles > scope.Entries ||
		scope.IncludedFiles < 0 || scope.ExcludedFiles < 0 || scope.IncludedFiles+scope.ExcludedFiles != scope.RegularFiles || scope.IncludedFiles != len(c.Files) ||
		(scope.Policy == "include-all" && scope.ExcludedFiles != 0) || scope.ReadBytes < 0 || scope.ReadBytes > 32<<20 {
		return nil, bad
	}
	if len(c.Collections) != 1 || len(c.Folders) != 1 || c.Collections[0] == nil || c.Folders[0] == nil ||
		len(c.Chunks)+len(c.Volumes)+len(c.Locations) != 0 || !c.Audit[0].At.Equal(c.Collections[0].CreatedAt) {
		return nil, bad
	}
	var total int64
	for _, f := range c.Files {
		if f == nil || f.SizeBytes < 0 || f.SizeBytes > 8<<20 {
			return nil, bad
		}
		total += f.SizeBytes
	}
	if total != scope.ReadBytes {
		return nil, bad
	}
	return &scope, nil
}
