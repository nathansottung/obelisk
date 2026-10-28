package main

import (
	"os"
	"strings"
	"testing"
)

// The README must not claim that every deleting or overwriting action is
// gated: some are not (they have their own confirmations or none), and it
// says which ones.
func TestREADME_DeleteGateClaimIsAccurate(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "### The delete gate")
	if i < 0 {
		t.Fatal("README has no delete gate section")
	}
	section := s[i:]
	if j := strings.Index(section[4:], "\n### "); j >= 0 {
		section = section[:j+4]
	}
	lower := strings.ToLower(section)
	for _, claim := range []string{"every action", "all actions", "every destructive", "all destructive"} {
		if strings.Contains(lower, claim) {
			t.Errorf("the delete gate section claims %q", claim)
		}
	}
	if !strings.Contains(lower, "not behind the gate") {
		t.Error("the delete gate section does not say which actions are not behind it")
	}
}
