//go:build !guionly

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// burnSetupForTest is a package staged for burning, with one pending disc.
func burnSetupForTest(t *testing.T, label string) (*App, int, string) {
	t.Helper()
	app := dockApp(t)
	staged := t.TempDir()
	mustWrite(t, filepath.Join(staged, "payload.tar"), "PAYLOAD")
	c := app.Store.AddChunk(Chunk{Name: label, Status: "BUILT", MediaKind: "BD-R25", StagedDir: staged})
	q := app.Store.AddBurnQueue(BurnQueue{Name: "Q", MediaKind: "BD-R25", Discs: []*BurnDisc{{ChunkID: c.ID, Status: "PENDING"}}})
	return app, q.ID, staged
}

// The burn command runs one program with an argument list, never through a
// shell: a label (or folder) holding shell syntax is passed as one argument
// and runs nothing.
func TestBurnCommand_RunsNoShell(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	label := `x; echo ran > ` + marker + `;`
	if runtime.GOOS == "windows" {
		label = `x & echo ran > ` + marker + ` & rem`
	}
	app, qid, _ := burnSetupForTest(t, label)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -volid {LABEL} -map {SRC} /`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	_, _ = app.BurnNext(qid, func(float64, string) {}) // xorriso may be absent; that is fine
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("shell syntax in the label ran a command: the burn command went through a shell")
	}
}

// {SRC} and {LABEL} are each one argument, whatever they hold.
func TestBurnCommand_PassesPlaceholdersWhole(t *testing.T) {
	label := `My "Disc" & 1; $(x)`
	app, qid, staged := burnSetupForTest(t, label)
	cfg := mustConfig(t, app)
	cfg.BurnCommand = `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`
	if err := writeConfigForTest(app, cfg); err != nil {
		t.Fatal(err)
	}
	var got []string
	prev := runBurnProgram
	runBurnProgram = func(argv []string) error { got = argv; return errStopBurnForTest }
	t.Cleanup(func() { runBurnProgram = prev })
	_, _ = app.BurnNext(qid, func(float64, string) {})
	want := []string{"xorriso", "-outdev", "/dev/sr0", "-volid", label, "-map", staged, "/", "-commit"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("argv %q, want %q", got, want)
	}
}

// Changing the burn command needs the typed word "Confirm", and only a known
// burner program with plain arguments is accepted.
func TestBurnCommand_ChangeNeedsConfirm(t *testing.T) {
	app := dockApp(t)
	if _, err := app.SaveConfig(map[string]any{"burn_command": "xorriso -map {SRC} /"}); err == nil || !strings.Contains(err.Error(), "Confirm") {
		t.Fatalf("changing burn_command without Confirm: got %v, want a refusal asking for it", err)
	}
	for _, bad := range []string{
		"xorriso -map {SRC} / | rm -rf /",
		"xorriso -map {SRC} / && del x",
		"xorriso -map {SRC} / > out",
		"rm -rf {SRC}",
		"cmd /c del {SRC}",
		`xorriso "unclosed`,
	} {
		if _, err := app.SaveConfig(map[string]any{"burn_command": bad, "burn_command_confirm": "Confirm"}); err == nil {
			t.Errorf("accepted burn_command %q", bad)
		}
	}
	cmd := `xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit -eject`
	if _, err := app.SaveConfig(map[string]any{"burn_command": cmd, "burn_command_confirm": "Confirm"}); err != nil {
		t.Fatalf("a confirmed, valid burn command was refused: %v", err)
	}
	if mustConfig(t, app).BurnCommand != cmd {
		t.Fatal("the confirmed burn command was not saved")
	}
	if b, _ := os.ReadFile(app.configPath()); strings.Contains(string(b), "burn_command_confirm") {
		t.Error("the typed confirmation was saved into the settings")
	}
	// Saving other settings with the same burn command needs no confirmation.
	if _, err := app.SaveConfig(map[string]any{"burn_command": cmd, "buffer_gb": 4}); err != nil {
		t.Fatalf("an unchanged burn command needed confirmation: %v", err)
	}
}

var errStopBurnForTest = errors.New("stopped by the test")

// xorriso can start other programs (-launch_frontend, -external_filter,
// -concat pipe) and read further commands from a file or the terminal. Those
// are refused in the burn command under any spelling xorriso accepts, and in
// a filled-in placeholder.
func TestBurnCommand_RefusesProgramLaunchingOptions(t *testing.T) {
	for _, bad := range []string{
		"xorriso -launch_frontend /bin/sh --",
		"xorriso --launch-frontend cmd.exe /c del x --",
		"xorriso -as cdrecord -v dev=/dev/sr0 {SRC} -- -options_from_file /tmp/cmds",
		"xorriso -external_filter f '' /bin/sh -- -map {SRC} /",
		"xorriso -concat pipe 'sh -c x' -- /a",
		"xorriso -dialog on",
		"xorrecord -v dev=/dev/sr0 {SRC} -- -Launch_Frontend sh --",
	} {
		if _, err := parseBurnCommand(bad); err == nil {
			t.Errorf("parseBurnCommand(%q) accepted an option that starts a program or reads commands", bad)
		}
	}
	for _, ok := range []string{
		"xorriso -as cdrecord -v dev=/dev/sr0 -eject {SRC}",
		"xorriso -outdev /dev/sr0 -volid {LABEL} -map {SRC} / -commit",
	} {
		if _, err := parseBurnCommand(ok); err != nil {
			t.Errorf("parseBurnCommand(%q): %v", ok, err)
		}
	}
	if _, err := burnArgv("xorriso -outdev /dev/sr0 -map {SRC} / {LABEL}", "/stage", "-launch_frontend"); err == nil {
		t.Error("a label filled in as -launch_frontend was accepted")
	}
}

// Each burner takes only the options known to burn a disc: anything else is
// refused, so an option that runs a program, mounts, or writes a file on this
// computer cannot be added. The documented burn commands are accepted.
func TestBurnCommand_AllowsOnlyKnownOptions(t *testing.T) {
	for _, bad := range []string{
		"xorriso -mount /dev/sr0 sbsector 0 /mnt",
		"xorriso -outdev stdio:/home/u/.bashrc -map {SRC} / -commit",
		"xorriso -dev stdio:C:/x.iso -map {SRC} / -commit",
		"xorriso -as mkisofs -o /tmp/x.iso {SRC}",
		"xorriso -session_log /tmp/x -outdev /dev/sr0 -map {SRC} / -commit",
		"xorriso -scdbackup_tag /tmp/x rec -outdev /dev/sr0 -map {SRC} / -commit",
		"xorriso -outdev /dev/sr0 -check_media data_to=/tmp/x --",
		"xorriso launch_frontend sh",
		"cdrskin dev=/dev/sr0 fallback_program=/bin/sh {SRC}",
		"cdrecord dev=/dev/sr0 msifile=/tmp/x {SRC}",
		"wodim dev=/dev/sr0 -v --help {SRC}",
		"growisofs -Z /dev/sr0 -o /tmp/x {SRC}",
		"growisofs -Z stdio:/tmp/x {SRC}",
		"imgburn /MODE BUILD /BUILDMODE DISC /SRC {SRC} /DEST C:/Windows/x.iso /START",
		"imgburn /MODE BUILD /BUILDMODE IMAGEFILE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE IMAGEFILE /SRC {SRC} /DEST C: /START",
		"imgburn /MODE BUILD /BUILDMODE DISC /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /SRC {SRC} /DEST E: /START",
		"imgburn /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /SRC {SRC} /DEST E: /TESTMODE YES /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /SRC {SRC} /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE WRITE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /SRC C:/x/p.ibb. /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /SRC C:/x/p.ibb|{SRC} /DEST E: /START",
		"wodim dev=/dev/sr0 -dummy {SRC}",
		"growisofs -Z /dev/sr0 -dry-run {SRC}",
		"imgburn /MODE BUILD /MODE WRITE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDOUTPUTMODE DEVICE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDMODE DEVICE /SRC {SRC} /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /SRC C:/x/proj.IBB /DEST E: /START",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /SRC {SRC} /VOLUMELABEL /FILESYSTEM /DEST E: /START",
		"imgburn /MODE BUILD /SRC {SRC} /DEST E: /LOG C:/x.log /START",
		"imgburn /MODE BUILD /SRC {SRC} /DEST E: extra",
	} {
		if _, err := parseBurnCommand(bad); err == nil {
			t.Errorf("parseBurnCommand(%q) accepted an option outside the burner's known ones", bad)
		}
	}
	for _, ok := range []string{
		defaultBurnCommand,
		`xorriso -outdev /dev/sr0 -volid "{LABEL}" -map "{SRC}" / -commit`,
		"xorriso -as cdrecord -v dev=/dev/sr0 -eject {SRC}",
		"xorrecord -v dev=/dev/sr0 -dao blank=as_needed {SRC}",
		"wodim dev=/dev/sr0 speed=4 -v -dao {SRC}",
		`growisofs -Z /dev/sr0 -R -J -udf -V "{LABEL}" "{SRC}"`,
		`"C:\Program Files (x86)\ImgBurn\ImgBurn.exe" /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /DEST E: /SRC "{SRC}" /VOLUMELABEL "{LABEL}" /START /CLOSESUCCESS /NOIMAGEDETAILS`,
	} {
		if _, err := parseBurnCommand(ok); err != nil {
			t.Errorf("parseBurnCommand(%q): %v", ok, err)
		}
	}
	// A filled-in placeholder cannot become an option.
	if _, err := burnArgv("wodim dev=/dev/sr0 -v {SRC}", "-msifile=/tmp/x", "L"); err == nil {
		t.Error("a folder filled in as an option was accepted")
	}
}

// growisofs's -use-the-force-luke=dummy is a test write: it burns nothing yet
// exits as a success, so the queue would record a blank disc.
func TestBurnCommand_RefusesGrowisofsTestWrite(t *testing.T) {
	for _, bad := range []string{
		"growisofs -Z /dev/sr0 -use-the-force-luke=dummy -R -J {SRC}",
		"growisofs -Z /dev/sr0 -use-the-force-luke=tty,dummy -R -J {SRC}",
	} {
		if _, err := parseBurnCommand(bad); err == nil {
			t.Errorf("parseBurnCommand(%q) accepted a test write", bad)
		}
	}
	if _, err := parseBurnCommand("growisofs -Z /dev/sr0 -use-the-force-luke=tty -R -J {SRC}"); err != nil {
		t.Errorf("-use-the-force-luke=tty: %v", err)
	}
}

// A burn command must burn the package: {SRC} where the burner reads what it
// writes, and for xorriso a -commit after it. A command that only blanks,
// fixates, burns something else or stops early exits as a success with the
// package not on the disc.
func TestBurnCommand_MustBurnThePackage(t *testing.T) {
	for _, bad := range []string{
		"xorriso -end",
		"xorriso -dev /dev/sr0 -blank fast",
		"xorriso -outdev /dev/sr0 -blank as_needed -commit",
		"xorriso -outdev /dev/sr0 -end -map {SRC} / -commit",
		"xorriso -outdev /dev/sr0 -map {SRC} /",
		"xorriso -outdev /dev/sr0 -map {SRC} / -commit -blank all",
		"xorriso -outdev /dev/sr0 -volid {SRC} -commit",
		"wodim dev=/dev/sr0 -fix",
		"cdrecord dev=/dev/sr0 blank=fast",
		"xorriso -as cdrecord dev=/dev/sr0 -v",
		"growisofs -Z /dev/sr0=/dev/zero",
		"growisofs -Z /dev/sr0=/dev/zero {SRC}",
		"growisofs -Z /dev/sr0 -V {SRC}",
		"imgburn /MODE BUILD /BUILDOUTPUTMODE DEVICE /BUILDINPUTMODE STANDARD /SRC C:/other /DEST E: /START",
	} {
		if _, err := parseBurnCommand(bad); err == nil {
			t.Errorf("parseBurnCommand(%q) accepted a command that does not burn the package", bad)
		}
	}
	for _, ok := range []string{
		defaultBurnCommand,
		"xorriso -outdev /dev/sr0 -map {SRC} / -commit_eject all",
		"xorriso -outdev /dev/sr0 -map {SRC} / -commit -end",
		"xorriso -as cdrecord -v dev=/dev/sr0 -eject {SRC}",
		"wodim dev=/dev/sr0 -v -dao {SRC}",
	} {
		if _, err := parseBurnCommand(ok); err != nil {
			t.Errorf("parseBurnCommand(%q): %v", ok, err)
		}
	}
}

// xorriso's -return_with sets the exit value for a given problem severity and
// -abort_on keeps it going past errors, so together they can make a failed
// burn exit 0. Neither is allowed.
func TestBurnCommand_RefusesExitValueMasking(t *testing.T) {
	for _, bad := range []string{
		"xorriso -outdev /dev/sr0 -return_with FAILURE 0 -map {SRC} / -commit",
		"xorriso -outdev /dev/sr0 -abort_on NEVER -map {SRC} / -commit",
		"xorriso -as cdrecord dev=/dev/sr0 {SRC} -- -return_with SORRY 0",
	} {
		if _, err := parseBurnCommand(bad); err == nil {
			t.Errorf("parseBurnCommand(%q) accepted an option that can hide a failed burn", bad)
		}
	}
}
