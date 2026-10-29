package main

// progpath.go — finding the program file a tool or burner name runs.

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveProgram finds the program p runs (a path, or a name looked up on
// PATH). On Windows a name with no extension also finds .bat and .cmd files,
// which run through cmd.exe and re-read their arguments as commands, so only a
// program file (.exe) is accepted there.
func resolveProgram(p string) (string, error) {
	lp, err := exec.LookPath(p)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(lp), ".exe") {
		return "", fmt.Errorf("%s is refused: only a program file (.exe) can be run, because batch and script files run through the command shell", lp)
	}
	return lp, nil
}
