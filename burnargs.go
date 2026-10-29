package main

// burnargs.go — the options each disc burner may be given in burn_command.
//
// Burners can do much more than burn a disc: xorriso can start other programs,
// mount, read commands from a file and write image or log files anywhere;
// cdrecord-style burners and ImgBurn can write files too. So each burner takes
// only the options known to burn a disc, and anything else is refused. An
// option's value is taken as the next word, as the burner reads it, so a value
// is never read as an option. Devices must name an optical drive.

import (
	"fmt"
	"regexp"
	"strings"
)

// Burner argument styles.
const (
	burnStyleXorriso   = "xorriso"   // xorriso commands (-outdev, -map, -commit …)
	burnStyleCdrecord  = "cdrecord"  // cdrecord style (dev=…, -v, -dao …)
	burnStyleXorrecord = "xorrecord" // cdrecord style, then xorriso commands after "--"
	burnStyleGrowisofs = "growisofs" // growisofs and its mkisofs options
	burnStyleImgBurn   = "imgburn"   // ImgBurn's /SWITCH value form
)

// burnPrograms are the burners burn_command may run, by program name, with the
// style of their arguments.
var burnPrograms = map[string]string{
	"xorriso": burnStyleXorriso, "xorrecord": burnStyleXorrecord,
	"cdrecord": burnStyleCdrecord, "wodim": burnStyleCdrecord, "cdrskin": burnStyleCdrecord,
	"growisofs": burnStyleGrowisofs, "imgburn": burnStyleImgBurn,
}

// burnDevicePattern is an optical drive: a Linux or macOS device node, a
// Windows drive letter, or a SCSI address (bus,target,lun).
var burnDevicePattern = regexp.MustCompile(`^(mmc:)?(/dev/(sr|scd|cdrom|cdrw|dvd|dvdrw|dvdrom|cdr|cd|sg|hd[a-z])[0-9]*|/dev/r?disk[0-9]+|[A-Za-z]:\\?|([A-Za-z]+:)?[0-9]+,[0-9]+,[0-9]+)$`)

func isBurnDevice(v string) bool { return burnDevicePattern.MatchString(v) }

// Value checks.
type burnValue func(string) bool

func anyValue(string) bool { return true }
func oneOf(vals ...string) burnValue {
	return func(v string) bool {
		for _, x := range vals {
			if strings.EqualFold(v, x) {
				return true
			}
		}
		return false
	}
}

var simpleValue = regexp.MustCompile(`^[A-Za-z0-9_.:,+-]*$`).MatchString

// xorrisoCommands are the xorriso commands allowed, each with checks for its
// values (as many as it takes). xorriso accepts one or two leading dashes.
// -return_with and -abort_on are left out: they can make a failed burn exit 0.
var xorrisoCommands = map[string][]burnValue{
	"dev": {isBurnDevice}, "outdev": {isBurnDevice}, "indev": {isBurnDevice},
	"volid": {anyValue}, "volset_id": {anyValue}, "publisher": {anyValue},
	"application_id": {anyValue}, "system_id": {anyValue}, "preparer_id": {anyValue},
	"volume_date":      {simpleValue, simpleValue},
	"map":              {anyValue, anyValue},
	"blank":            {oneOf("as_needed", "fast", "all", "full", "deformat", "deformat_quickest")},
	"format":           {simpleValue},
	"close":            {oneOf("on", "off", "as_needed")},
	"joliet":           {oneOf("on", "off")},
	"rockridge":        {oneOf("on", "off")},
	"hfsplus":          {oneOf("on", "off")},
	"md5":              {oneOf("on", "off", "all")},
	"compliance":       {simpleValue},
	"speed":            {simpleValue},
	"stream_recording": {simpleValue},
	"padding":          {simpleValue},
	"report_about":     {simpleValue},
	"charset":          {simpleValue},
	"local_charset":    {simpleValue},
	"out_charset":      {simpleValue},
	"follow":           {simpleValue},
	"commit_eject":     {oneOf("in", "out", "all", "none")},
	"commit":           {}, "end": {}, "for_backup": {},
}

// cdrecordFlags and cdrecordKeys are the cdrecord-style options allowed.
var cdrecordFlags = map[string]bool{
	"-v": true, "-vv": true, "-vvv": true, "-dao": true, "-sao": true, "-tao": true,
	"-eject": true, "-multi": true, "-pad": true, "-nopad": true,
	"-data": true, "-isosize": true, "-immed": true, "-waiti": true, "-force": true,
	"-overburn": true, "-nofix": true, "-fix": true, "-useinfo": true,
}
var cdrecordKeys = map[string]burnValue{
	"dev": isBurnDevice, "speed": simpleValue, "blank": simpleValue, "fs": simpleValue,
	"gracetime": simpleValue, "padsize": simpleValue, "tsize": simpleValue,
	"minbuf": simpleValue, "driveropts": simpleValue, "stream_recording": simpleValue,
}

// growisofsFlags and growisofsValued are the growisofs options allowed.
var growisofsFlags = map[string]bool{
	"-R": true, "-r": true, "-J": true, "-joliet-long": true, "-udf": true, "-l": true,
	"-D": true, "-f": true, "-dvd-compat": true, "-overburn": true,
	"-quiet": true, "-v": true, "-allow-limited-size": true, "-dvd-video": true,
	"-use-the-force-luke": true,
}
var growisofsValued = map[string]burnValue{
	"-Z": growisofsTarget, "-M": growisofsTarget, "-V": anyValue, "-A": anyValue,
	"-P": anyValue, "-p": anyValue, "-publisher": anyValue, "-volset": anyValue,
	"-sysid": anyValue, "-iso-level": simpleValue, "-input-charset": simpleValue,
}

// growisofsEquals are the -option=value forms allowed. -use-the-force-luke
// takes only sub-options that still burn: "dummy" (a test write that burns
// nothing yet succeeds) and the others are refused.
var growisofsEquals = map[string]burnValue{"-speed": simpleValue, "-use-the-force-luke": oneOf("tty", "dao", "notray")}

// growisofsTarget is the drive after -Z or -M. The "-Z device=image" form,
// which burns an image instead of the package folder, is refused.
func growisofsTarget(v string) bool { return isBurnDevice(v) }

// imgburnSwitches are the ImgBurn switches allowed, with a check for the value
// a switch takes (nil for none). The command must leave nothing to ImgBurn's
// saved settings that could send the output to a file or burn nothing: it
// builds a folder (/MODE BUILD) to a disc (/BUILDOUTPUTMODE DEVICE, never
// IMAGEFILE) in the standard layout, with /SRC (not a saved .ibb project with
// its own settings) and /DEST (a drive letter) given, each switch once, no
// value starting with "/", and no test mode (which burns nothing yet reports
// success).
var imgburnSwitches = map[string]burnValue{
	"MODE": oneOf("BUILD"), "BUILDOUTPUTMODE": oneOf("DEVICE"),
	"BUILDINPUTMODE": oneOf("STANDARD"),
	"SRC": func(v string) bool {
		return !strings.HasSuffix(strings.TrimRight(strings.ToLower(v), ". "), ".ibb")
	},
	"DEST":        regexp.MustCompile(`^[A-Za-z]:\\?$`).MatchString,
	"VOLUMELABEL": anyValue, "FILESYSTEM": anyValue, "UDFREVISION": simpleValue,
	"SPEED": simpleValue, "VERIFY": oneOf("YES", "NO"), "EJECT": oneOf("YES", "NO"),
	"ROOTFOLDER": oneOf("YES", "NO"), "TESTMODE": oneOf("NO"),
	"RECURSESUBDIRECTORIES": oneOf("YES", "NO"), "INCLUDEHIDDENFILES": oneOf("YES", "NO"),
	"INCLUDESYSTEMFILES": oneOf("YES", "NO"),
	"START":              nil, "CLOSE": nil, "CLOSESUCCESS": nil, "NOIMAGEDETAILS": nil,
	"WAITFORMEDIA": nil, "NOSAVESETTINGS": nil,
}

// burnPlan records, while a command is checked, whether it burns the package:
// {SRC} where the burner reads what it writes (and, for xorriso, a commit
// after it).
type burnPlan struct {
	mappedSrc bool // xorriso: a -map of {SRC} is waiting for a commit
	burned    bool
}

// burnsSrc reports whether a value names the package folder.
func burnsSrc(v string) bool { return strings.Contains(v, "{SRC}") }

// checkBurnArgs checks a burn command's arguments (args[0] is the program)
// against its burner's allowed options. It runs on the command as written
// (template) and again with {SRC} and {LABEL} filled in, so a filled-in value
// cannot become an option. The command as written must also burn {SRC}: a
// command that only blanks, fixates, burns something else or stops early
// exits as a success with the package not on the disc.
func checkBurnArgs(args []string, template bool) error {
	prog := strings.TrimSuffix(strings.ToLower(burnProgramName(args[0])), ".exe")
	var err error
	p := &burnPlan{}
	switch burnPrograms[prog] {
	case burnStyleXorriso:
		err = checkXorrisoArgs(args[1:], p)
	case burnStyleXorrecord:
		rest, e := checkCdrecordArgs(args[1:], true, p)
		if err = e; err == nil {
			err = checkXorrisoArgs(rest, p)
		}
	case burnStyleCdrecord:
		_, err = checkCdrecordArgs(args[1:], false, p)
	case burnStyleGrowisofs:
		err = checkGrowisofsArgs(args[1:], p)
	case burnStyleImgBurn:
		err = checkImgBurnArgs(args[1:], p)
	default:
		err = fmt.Errorf("%q is not a known burner", args[0])
	}
	if err != nil {
		return fmt.Errorf("burn_command: %w (only the burner's options for burning a disc are allowed)", err)
	}
	if template && !p.burned {
		return fmt.Errorf("burn_command must burn {SRC}: give it where the burner reads what it writes (xorriso -map {SRC} / then -commit, the cdrecord or growisofs source, ImgBurn /SRC)")
	}
	return nil
}

func burnProgramName(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		p = p[i+1:]
	}
	return p
}

func checkXorrisoArgs(args []string, p *burnPlan) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return fmt.Errorf("%q is not an xorriso command", a)
		}
		name := strings.ReplaceAll(strings.ToLower(strings.TrimLeft(a, "-")), "-", "_")
		if name == "eject" { // an optional in, out or all
			if i+1 < len(args) && oneOf("in", "out", "all")(args[i+1]) {
				i++
			}
			continue
		}
		if name == "end" {
			// -end stops xorriso: nothing may follow it.
			if i != len(args)-1 {
				return fmt.Errorf("xorriso -end must be last: it stops before the commands after it")
			}
			continue
		}
		// Once the package is burned only ejecting and ending may follow; a
		// later -blank, say, would erase it.
		if p.burned {
			return fmt.Errorf("xorriso %s is not allowed after the package is burned", a)
		}
		if name == "as" {
			if i+1 >= len(args) || !oneOf("cdrecord", "wodim", "cdrskin", "xorrecord")(args[i+1]) {
				return fmt.Errorf("xorriso -as may only emulate cdrecord")
			}
			rest, err := checkCdrecordArgs(args[i+2:], true, p)
			if err != nil {
				return err
			}
			args, i = rest, -1
			continue
		}
		switch name {
		case "map":
			if i+1 < len(args) && burnsSrc(args[i+1]) {
				p.mappedSrc = true
			}
		case "commit", "commit_eject":
			if p.mappedSrc {
				p.burned = true
			}
		}
		vals, ok := xorrisoCommands[name]
		if !ok {
			return fmt.Errorf("xorriso %s is not allowed", a)
		}
		for _, ok := range vals {
			i++
			if i >= len(args) {
				return fmt.Errorf("xorriso %s is missing a value", a)
			}
			if !ok(args[i]) {
				return fmt.Errorf("xorriso %s does not take %q", a, args[i])
			}
		}
	}
	return nil
}

var cdrecordKeyName = regexp.MustCompile(`^[A-Za-z_]+$`).MatchString

// checkCdrecordArgs checks cdrecord-style arguments. With dashDash, "--" ends
// them (xorriso emulation) and the rest are returned for xorriso.
func checkCdrecordArgs(args []string, dashDash bool, p *burnPlan) ([]string, error) {
	for i, a := range args {
		if dashDash && a == "--" {
			return args[i+1:], nil
		}
		if strings.HasPrefix(a, "-") {
			if !cdrecordFlags[a] {
				return nil, fmt.Errorf("option %s is not allowed", a)
			}
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok && cdrecordKeyName(k) {
			check, known := cdrecordKeys[strings.ToLower(k)]
			if !known {
				return nil, fmt.Errorf("option %s= is not allowed", k)
			}
			if !check(v) {
				return nil, fmt.Errorf("option %s= does not take %q", k, v)
			}
			continue
		}
		// A track to burn: read, never written.
		if burnsSrc(a) {
			p.burned = true
		}
	}
	return nil, nil
}

func checkGrowisofsArgs(args []string, p *burnPlan) error {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			// A folder to burn: read, never written.
			if burnsSrc(a) {
				p.burned = true
			}
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok {
			check, known := growisofsEquals[k]
			if !known || !check(v) {
				return fmt.Errorf("growisofs option %s is not allowed", a)
			}
			continue
		}
		if growisofsFlags[a] {
			continue
		}
		check, ok := growisofsValued[a]
		if !ok {
			return fmt.Errorf("growisofs option %s is not allowed", a)
		}
		i++
		if i >= len(args) || !check(args[i]) {
			return fmt.Errorf("growisofs %s needs a drive or value it accepts", a)
		}
	}
	return nil
}

func checkImgBurnArgs(args []string, p *burnPlan) error {
	seen := map[string]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "/") {
			return fmt.Errorf("ImgBurn takes /SWITCH words, not %q", a)
		}
		name := strings.ToUpper(a[1:])
		check, ok := imgburnSwitches[name]
		if !ok {
			return fmt.Errorf("ImgBurn %s is not allowed", a)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("ImgBurn %s is given twice", a)
		}
		seen[name] = ""
		if check == nil {
			continue
		}
		i++
		if i >= len(args) || strings.HasPrefix(args[i], "/") || !check(args[i]) {
			return fmt.Errorf("ImgBurn %s needs a value it accepts", a)
		}
		seen[name] = strings.ToUpper(args[i])
		if name == "SRC" && burnsSrc(args[i]) {
			p.burned = true
		}
	}
	for _, need := range []string{"MODE", "BUILDOUTPUTMODE", "BUILDINPUTMODE", "SRC", "DEST"} {
		if _, ok := seen[need]; !ok {
			return fmt.Errorf("ImgBurn needs /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD with /SRC and /DEST (missing /%s), so nothing is left to its saved settings", need)
		}
	}
	return nil
}
