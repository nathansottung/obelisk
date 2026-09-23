package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type scopeKeyCase struct {
	name string
	raw  []byte
	ok   bool
}

// Adversarial members are composed as JSON text, never round-tripped through a
// map: duplicate and escaped member names must reach the actual loader intact.
func scopeKeyCases(t *testing.T) []scopeKeyCase {
	t.Helper()
	src, dst := inventoryFixture(t)
	empty := filepath.Join(filepath.Dir(src), "empty-scope")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := produceGUIInventory(context.Background(), empty, dst, disposableInventoryLimits, inventoryIO{}, io.Discard); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var c catalog
	if err = json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	quote := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	at, _ := json.Marshal(c.Audit[0].At)
	detail := c.Audit[0].Detail
	event := func(d string) string {
		return `{"at":` + string(at) + `,"action":"GUI_DISPOSABLE_INVENTORY_V1","detail":` + quote(d) + `}`
	}
	// Removing canonical audit from this independently generated envelope is only
	// fixture construction. Its replacement members below are kept as raw text.
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	delete(envelope, "audit")
	base, _ := json.Marshal(envelope)
	wrap := func(members string) []byte {
		if members == "" {
			return base
		}
		return []byte(string(base[:len(base)-1]) + `,` + members + `}`)
	}
	cases := []scopeKeyCase{{"canonical-zero", raw, true}, {"legacy-absent", wrap(""), true}, {"legacy-null", wrap(`"audit":null`), true}, {"legacy-empty", wrap(`"audit":[]`), true}}
	addDetail := func(name, value string, ok bool) {
		cases = append(cases, scopeKeyCase{name, wrap(`"audit":[` + event(value) + `]`), ok})
	}
	addDetail("reported-missing-entries", `{"Version":2,"version":1,"policy":"ignore-exact-ds-store","regularFiles":0,"includedFiles":0,"excludedFiles":0,"readBytes":0,"complete":true}`, false)
	addDetail("reported-missing-excluded", `{"version":1,"policy":"ignore-exact-ds-store","entries":0,"regularFiles":0,"includedFiles":0,"readBytes":0,"Complete":false,"complete":true}`, false)
	keys := []string{"version", "policy", "entries", "regularFiles", "includedFiles", "excludedFiles", "readBytes", "complete"}
	values := []string{"1", `"include-all"`, "0", "0", "0", "0", "0", "true"}
	for i, key := range keys {
		member := quote(key) + `:` + values[i]
		alias := strings.ToUpper(key[:1]) + key[1:]
		escaped := fmt.Sprintf(`"\u%04x%s"`, key[0], key[1:])
		for name, replacement := range map[string]string{
			"case":              quote(alias) + `:` + values[i],
			"null":              quote(key) + `:null`,
			"wrong-type":        quote(key) + `:[]`,
			"duplicate-same":    member + `,` + member,
			"duplicate-before":  quote(key) + `:null,` + member,
			"duplicate-after":   member + `,` + quote(key) + `:null`,
			"alias-before":      quote(alias) + `:null,` + member,
			"alias-after":       member + `,` + quote(alias) + `:null`,
			"alias-same-before": quote(alias) + `:` + values[i] + `,` + member,
			"alias-same-after":  member + `,` + quote(alias) + `:` + values[i],
			"encoded-duplicate": member + `,` + escaped + `:` + values[i],
		} {
			addDetail(key+"/"+name, strings.Replace(detail, member, replacement, 1), false)
		}
		missing := strings.Replace(detail, member+`,`, "", 1)
		if missing == detail {
			missing = strings.Replace(detail, `,`+member, "", 1)
		}
		addDetail(key+"/missing", missing, false)
		addDetail(key+"/escaped-canonical", strings.Replace(detail, quote(key), escaped, 1), true)
		addDetail(key+"/encoded-alias", strings.Replace(detail, quote(key), fmt.Sprintf(`"\u%04x%s"`, alias[0], alias[1:]), 1), false)
	}
	canonical := event(detail)
	for _, key := range []string{"at", "action", "detail"} {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal([]byte(canonical), &fields)
		member := quote(key) + `:` + string(fields[key])
		alias := quote(strings.ToUpper(key[:1])+key[1:]) + `:` + string(fields[key])
		for name, replacement := range map[string]string{"case": alias, "null": quote(key) + `:null`, "wrong-type": quote(key) + `:[]`, "duplicate-same": member + `,` + member, "duplicate-before": quote(key) + `:null,` + member, "duplicate-after": member + `,` + quote(key) + `:null`, "alias-before": alias + `,` + member, "alias-after": member + `,` + alias} {
			cases = append(cases, scopeKeyCase{"event/" + key + "/" + name, wrap(`"audit":[` + strings.Replace(canonical, member, replacement, 1) + `]`), false})
		}
		missing := strings.Replace(canonical, member+`,`, "", 1)
		if missing == canonical {
			missing = strings.Replace(canonical, `,`+member, "", 1)
		}
		cases = append(cases, scopeKeyCase{"event/" + key + "/missing", wrap(`"audit":[` + missing + `]`), false})
	}
	for name, members := range map[string]string{
		"case":              `"Audit":[` + canonical + `]`,
		"case-empty":        `"AUDIT":[]`,
		"encoded-case":      `"\u0041udit":[` + canonical + `]`,
		"duplicate-same":    `"audit":[` + canonical + `],"audit":[` + canonical + `]`,
		"duplicate-before":  `"audit":[null],"audit":[` + canonical + `]`,
		"duplicate-after":   `"audit":[` + canonical + `],"audit":[null]`,
		"alias-before":      `"Audit":[null],"audit":[` + canonical + `]`,
		"alias-after":       `"audit":[` + canonical + `],"Audit":[]`,
		"encoded-duplicate": `"audit":[` + canonical + `],"\u0061udit":null`,
		"object":            `"audit":{}`,
		"null-event":        `"audit":[null]`,
		"extra-event":       `"audit":[` + canonical + `,` + canonical + `]`,
	} {
		cases = append(cases, scopeKeyCase{"wrapper/" + name, wrap(members), false})
	}
	escapedEvent := strings.ReplaceAll(canonical, `"at":`, `"\u0061t":`)
	escapedEvent = strings.ReplaceAll(escapedEvent, `"action":`, `"\u0061ction":`)
	escapedEvent = strings.ReplaceAll(escapedEvent, `"detail":`, `"\u0064etail":`)
	cases = append(cases, scopeKeyCase{"wrapper/escaped-canonical", wrap(`"\u0061udit":[` + escapedEvent + `]`), true})
	// These strings are ordinary record values, not alternate scope wrappers.
	cases = append(cases, scopeKeyCase{"unrelated-strings", bytes.ReplaceAll(raw, []byte(`"name":"Disposable inventory: empty-scope"`), []byte(`"name":"audit detail entries"`)), true})
	return cases
}

func TestGUIInventoryScopeKeys(t *testing.T) {
	for _, tc := range scopeKeyCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			if !json.Valid(tc.raw) {
				t.Fatal("fixture is not valid JSON")
			}
			before := bytes.Clone(tc.raw)
			s, err := loadGUICatalog("raw scope fixture", func(string) ([]byte, error) { return tc.raw, nil })
			if tc.ok != (err == nil && s != nil) || (!tc.ok && s != nil) {
				t.Fatalf("ok=%v: adopted=%v err=%v\n%s", tc.ok, s != nil, err, tc.raw)
			}
			if !bytes.Equal(before, tc.raw) {
				t.Fatal("input changed")
			}
		})
	}
}

func TestGUIInventoryScopeRefusalProtocol(t *testing.T) {
	for _, tc := range scopeKeyCases(t) {
		if !strings.HasPrefix(tc.name, "reported-") && !strings.HasPrefix(tc.name, "wrapper/") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "scope.json")
			if err := os.WriteFile(file, tc.raw, 0600); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			err := runGUICatalog([]string{file}, strings.NewReader("{\"text\":\"\"}\n"), &out)
			if tc.ok != (err == nil) {
				t.Fatal(err, out.String())
			}
			if !tc.ok && (bytes.Contains(out.Bytes(), []byte(`"ok":true`)) || bytes.Contains(out.Bytes(), []byte(`"catalog"`)) || bytes.Contains(out.Bytes(), []byte(`"ids"`))) {
				t.Fatal("failed load emitted successful or partial data", out.String())
			}
			after, _ := os.ReadFile(file)
			if !bytes.Equal(after, tc.raw) {
				t.Fatal("file changed")
			}
		})
	}
}
