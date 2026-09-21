package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"unicode/utf8"
)

// Validate original bytes/escapes before encoding/json can repair them. This is
// preview input policy, not a change to the shared production catalog decoder.
func validateGUIJSON(raw []byte) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return errors.New("invalid UTF-8 or JSON encoding")
	}
	quoted := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			quoted = !quoted
			continue
		}
		if !quoted || raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		u, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if u >= 0xdc00 && u <= 0xdfff {
			return errors.New("isolated low surrogate in JSON string")
		}
		if u >= 0xd800 && u <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return errors.New("isolated high surrogate in JSON string")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return errors.New("invalid JSON surrogate pair")
			}
			i += 6
		}
	}
	return nil
}
