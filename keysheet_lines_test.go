//go:build !guionly

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// dropSheetLine removes payload line no (1-based) from a key sheet.
func dropSheetLine(t *testing.T, sheet string, no int) string {
	t.Helper()
	var out []string
	dropped := false
	for _, ln := range strings.Split(sheet, "\n") {
		m := keySheetLineRe.FindStringSubmatch(ln)
		if m != nil && !dropped && sheetLineNo(ln) == no {
			dropped = true
			continue
		}
		out = append(out, ln)
	}
	if !dropped {
		t.Fatalf("sheet has no payload line %d:\n%s", no, sheet)
	}
	return strings.Join(out, "\n")
}

func sheetLineNo(ln string) int {
	s := strings.TrimSpace(ln)
	n := 0
	for i := 1; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// A sheet with a middle line missing must fail and name that line, even when
// every line left passes its own check and no key is named to compare with.
func TestKeySheet_MissingMiddleLineFailsAndNamesIt(t *testing.T) {
	pass := strings.Repeat("ABCDwxyz12", 6) // 60 characters: three lines
	sheet := keySheet("K-mid", "", passFingerprint(pass), pass)
	short := dropSheetLine(t, sheet, 2)
	_, res := parseKeySheet(short)
	if res.OK {
		t.Fatalf("a sheet missing line 2 was accepted: %+v", res)
	}
	if len(res.MissingLines) != 1 || res.MissingLines[0] != 2 {
		t.Fatalf("want line 2 named as missing, got %+v", res)
	}
	if !strings.Contains(strings.Join(res.Notes, " "), "L02") {
		t.Fatalf("the notes must name line L02: %v", res.Notes)
	}
}

// The last line missing is found too: the sheet says how many lines it has.
func TestKeySheet_MissingLastLineFailsAndNamesIt(t *testing.T) {
	pass := strings.Repeat("ABCDwxyz12", 6)
	sheet := keySheet("K-last", "", passFingerprint(pass), pass)
	_, res := parseKeySheet(dropSheetLine(t, sheet, 3))
	if res.OK || len(res.MissingLines) != 1 || res.MissingLines[0] != 3 {
		t.Fatalf("want line 3 named as missing, got %+v", res)
	}
}

// The existing verify route, with no key named, refuses the short sheet.
func TestKeySheetVerifyRoute_MissingMiddleLine(t *testing.T) {
	app, _ := newTestApp(t, nil)
	pass := strings.Repeat("ABCDwxyz12", 6)
	short := dropSheetLine(t, keySheet("K-mid", "", passFingerprint(pass), pass), 2)
	rec := postAPI(t, app, "/api/keysheet/verify", map[string]any{"sheet": short})
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["ok"] != false {
		t.Fatalf("the verify route accepted a sheet missing line 2: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "L02") {
		t.Fatalf("the verify route must name line L02: %s", rec.Body.String())
	}
}
