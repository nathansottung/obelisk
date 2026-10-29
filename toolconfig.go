package main

// toolconfig.go — checks on the settings that choose which program runs and
// with what extra arguments: tool paths and par2's extra arguments.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// toolNames are the tools a path may be configured for, each with the program
// names accepted for it.
var toolNames = map[string][]string{
	"tar":        {"tar", "bsdtar", "gtar"},
	"gpg":        {"gpg", "gpg2"},
	"par2":       {"par2"},
	"smartctl":   {"smartctl"},
	"ffprobe":    {"ffprobe"},
	"dvdisaster": {"dvdisaster"},
	"czkawka":    {"czkawka", "czkawka_cli"},
	"rclone":     {"rclone"},
	"xorriso":    {"xorriso"},
	"stenc":      {"stenc"},
}

// checkToolPath checks a configured path for tool name: a known tool, and an
// absolute path whose program name is that tool. exists also requires a file
// there (when saving); a missing file is otherwise left to the PATH fallback.
func checkToolPath(name, p string, exists bool) error {
	accepted, ok := toolNames[name]
	if !ok {
		return fmt.Errorf("tools: %q is not a tool Obelisk runs", name)
	}
	if strings.TrimSpace(p) == "" {
		return nil
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("tools: the path for %s must be absolute, not %q", name, p)
	}
	prog := strings.TrimSuffix(strings.ToLower(filepath.Base(p)), ".exe")
	match := false
	for _, a := range accepted {
		if prog == a {
			match = true
		}
	}
	if !match {
		return fmt.Errorf("tools: %q is not %s (the program must be named %s)", p, name, strings.Join(accepted, " or "))
	}
	if exists {
		fi, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("tools: %s: %w", p, err)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("tools: %s is not a program file", p)
		}
	}
	return nil
}

// par2ArgPattern is one allowed par2 tuning flag: threads (-t, -T), memory
// (-m) and output level (-q, -qq, -v, -vv).
var par2ArgPattern = regexp.MustCompile(`^(-t[0-9]{1,3}|-T[0-9]{1,3}|-m[0-9]{1,6}|-q|-qq|-v|-vv)$`)

// checkPar2ExtraArgs allows only par2's tuning flags. Anything else is
// refused, in particular flags that delete files (-p, purge) or change what is
// read (-R, -N, -B), and anything that is not a flag.
func checkPar2ExtraArgs(s string) error {
	for _, a := range strings.Fields(s) {
		if !par2ArgPattern.MatchString(a) {
			why := "only -t, -T, -m, -q, -qq, -v and -vv are allowed"
			if a == "-p" || strings.Contains(strings.ToLower(a), "purge") {
				why = "it deletes files after repair; " + why
			}
			return fmt.Errorf("par2_extra_args: %q is refused: %s", a, why)
		}
	}
	return nil
}

// tapeToolProgram is the tape tool a program name is, by exact name (".exe"
// aside), or "" when it is none of them.
func tapeToolProgram(p string) string {
	prog := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(p, `\`, "/"))), ".exe")
	for _, d := range tapeTools {
		for _, b := range d.Bins {
			if prog == strings.TrimSuffix(strings.ToLower(b), ".exe") {
				return d.Name
			}
		}
	}
	return ""
}

// checkTapeToolPath checks a configured tape_tool: blank, or an absolute path
// to a known tape tool. exists also requires a file there (when saving).
func checkTapeToolPath(p string, exists bool) error {
	if strings.TrimSpace(p) == "" {
		return nil
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("tape_tool must be an absolute path, not %q", p)
	}
	if tapeToolProgram(p) == "" {
		return fmt.Errorf("tape_tool: %q is not a known tape tool (itdt, tapeinfo, sg_logs, hp_ltt, ltt or hpe_ltt)", p)
	}
	if exists {
		fi, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("tape_tool: %s: %w", p, err)
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("tape_tool: %s is not a program file", p)
		}
	}
	return nil
}
