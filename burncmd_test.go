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
