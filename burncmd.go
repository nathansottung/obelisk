package main

// burncmd.go — the burn command, run as one program with an argument list.
//
// burn_command is split into a program and its arguments here, never handed to
// a shell. Quotes ("..." or '...') group words into one argument; nothing else
// is interpreted. The program must be a known disc burner. {SRC} (the staged
// package folder) and {LABEL} (the package name) are filled in after the split,
// so each stays exactly one argument whatever it holds: a folder or name with
// spaces, quotes, &, ;, | or $( ) cannot become extra arguments or commands.

import (
	"fmt"
	"path/filepath"
	"strings"
)

// burnPrograms are the burners burn_command may run, by program name.
var burnPrograms = map[string]bool{
	"xorriso": true, "xorrecord": true, "growisofs": true, "cdrecord": true,
	"wodim": true, "cdrskin": true, "imgburn": true, "cdburn": true,
}

// burnRefusedOptions are xorriso commands a burn command may not use: they
// start other programs (-launch_frontend, -external_filter, -concat pipe), read
// further commands from a file or the terminal, or write files on this
// computer outside the disc. xorriso takes one or two leading dashes and "-"
// for "_", so options are compared in that normal form (see burnOptionWord).
var burnRefusedOptions = map[string]bool{
	"launch_frontend": true, "external_filter": true, "concat": true,
	"options_from_file": true, "dialog": true, "read_mkisofsrc": true,
	"extract": true, "extract_l": true, "extract_cut": true, "extract_single": true,
	"extract_boot_images": true, "cpx": true, "cpax": true, "cp_rx": true, "cp_rax": true,
	"paste_in": true, "osirrox": true, "logfile": true, "errfile_log": true,
}

// burnOptionWord is an argument in the form burnRefusedOptions lists it.
func burnOptionWord(a string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimLeft(a, "-")), "-", "_")
}

// checkBurnArgs refuses an argument list that uses one of burnRefusedOptions.
func checkBurnArgs(args []string) error {
	for _, a := range args[1:] {
		if strings.HasPrefix(a, "-") && burnRefusedOptions[burnOptionWord(a)] {
			return fmt.Errorf("burn_command may not use %s: it starts other programs, reads more commands, or writes files outside the disc", a)
		}
	}
	return nil
}

// parseBurnCommand splits a burn command into a program and arguments, and
// checks it: quotes must be closed, shell syntax (| & ; < > ` $( ) and line
// breaks) is refused outside quotes, and the program must be a known burner.
func parseBurnCommand(cmd string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inWord := false
	var quote rune
	for _, r := range cmd {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				args = append(args, cur.String())
				cur.Reset()
				inWord = false
			}
		case strings.ContainsRune("|&;<>`$()\r\n", r):
			return nil, fmt.Errorf("burn_command runs one program without a shell, so %q is not allowed outside quotes (no pipes, redirects, && or ;)", string(r))
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("burn_command has an unclosed %c quote", quote)
	}
	if inWord {
		args = append(args, cur.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("burn_command is empty")
	}
	prog := strings.ToLower(filepath.Base(strings.ReplaceAll(args[0], `\`, "/")))
	prog = strings.TrimSuffix(prog, ".exe")
	if !burnPrograms[prog] {
		return nil, fmt.Errorf("burn_command must run a disc burner (one of xorriso, xorrecord, growisofs, cdrecord, wodim, cdrskin, ImgBurn, cdburn), not %q", args[0])
	}
	if err := checkBurnArgs(args); err != nil {
		return nil, err
	}
	return args, nil
}

// burnArgv is the burn command for one package: parsed, with {SRC} and {LABEL}
// filled in inside each argument.
func burnArgv(cmd, src, label string) ([]string, error) {
	args, err := parseBurnCommand(cmd)
	if err != nil {
		return nil, err
	}
	for i, a := range args {
		if i == 0 {
			continue // the program itself takes no placeholders
		}
		a = strings.ReplaceAll(a, "{SRC}", src)
		args[i] = strings.ReplaceAll(a, "{LABEL}", label)
	}
	// A filled-in folder or label is checked too.
	if err := checkBurnArgs(args); err != nil {
		return nil, err
	}
	return args, nil
}

// runBurnProgram runs the burner with its argument list, without a shell. Exit
// code 0 is success; anything else fails the disc with the tail of its output.
// A variable so tests can see the argument list.
var runBurnProgram = func(argv []string) error {
	prog, err := resolveProgram(argv[0])
	if err != nil {
		return fmt.Errorf("burn_command: %w", err)
	}
	out, err := burnProgramCommand(append([]string{prog}, argv[1:]...)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, toolOutputTail(out))
	}
	return nil
}

// burnCommandConfirmWord is the word that confirms a change of burn_command.
const burnCommandConfirmWord = "Confirm"
