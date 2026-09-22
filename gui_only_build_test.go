package main

// gui_only_build_test.go — the launcher-only build (-tags guionly) contains the two
// GUI adapter modes and nothing that serves HTTP, dials, spawns processes or
// carries the escrow payload: no route table, no embedded UI, no server loop. It
// refuses every other argument without binding a port or touching a data
// directory, and it reports the version stamped with -X main.appVersion. Its
// copies of main.go helpers must not drift.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const guiTestVersion = "0.0.0-gui-only-test.0123456789ab"

func goTool() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go"+ext)
}

func buildBinary(t *testing.T, dir, name, version string, tags ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	args := []string{"build", "-mod=readonly", "-trimpath"}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
	}
	if version != "" {
		args = append(args, "-ldflags", "-X main.appVersion="+version)
	}
	args = append(args, "-o", out, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, goTool(), args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, b)
	}
	return out
}

func symbolsOf(t *testing.T, bin string) string {
	t.Helper()
	b, err := exec.Command(goTool(), "tool", "nm", bin).Output()
	if err != nil {
		t.Fatalf("go tool nm: %v", err)
	}
	return string(b)
}

func hasSymbol(nm, sym string) bool {
	for _, line := range strings.Split(nm, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[len(fields)-1] == sym {
			return true
		}
	}
	return false
}

func TestGUIOnlyBinary(t *testing.T) {
	dir := t.TempDir()
	gui := buildBinary(t, dir, "obelisk-gui", guiTestVersion, "guionly")
	full := buildBinary(t, dir, "obelisk-full", guiTestVersion)

	t.Run("no server, routes, UI, network, processes or escrow linked", func(t *testing.T) {
		guiNM, fullNM := symbolsOf(t, gui), symbolsOf(t, full)
		absent := []string{
			// HTTP server, route table, embedded UI and job runner (main.go).
			"main.api", "main.uiFS", "main.runJob",
			"net/http.(*Server).Serve", "net/http.(*conn).serve",
			// Listening, outbound HTTP and helper processes.
			"net.Listen", "net.(*TCPListener).Accept",
			"net/http.(*Transport).RoundTrip",
			"os/exec.Command", "os/exec.(*Cmd).Start",
			// Escrow payload embedded by escrow.go.
			"main.embeddedObeliskSource", "main.escrowManifestJSON",
		}
		for _, sym := range absent {
			if !hasSymbol(fullNM, sym) {
				t.Fatalf("control: full build lacks %s, so this check proves nothing", sym)
			}
			if hasSymbol(guiNM, sym) {
				t.Errorf("launcher-only build links %s", sym)
			}
		}
		for _, sym := range []string{"main.runGUIInventory", "main.runGUICatalog", "main.appVersion"} {
			if !hasSymbol(guiNM, sym) {
				t.Errorf("launcher-only build lacks %s", sym)
			}
		}
		guiBytes, _ := os.ReadFile(gui)
		fullBytes, _ := os.ReadFile(full)
		markers := [][]byte{
			// Strings that exist only in the embedded UI or the route table.
			[]byte("instrument of negentropy"), []byte("Paranoid mode"), []byte("X-Requested-By"), []byte("/api/keys"),
		}
		// The embedded escrow files, byte for byte.
		for _, name := range []string{"escrow/obelisk-src.tar.gz", "escrow_manifest.json"} {
			b, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			markers = append(markers, b)
		}
		for _, marker := range markers {
			label := string(marker)
			if len(label) > 40 {
				label = label[:40] + "…"
			}
			if !bytes.Contains(fullBytes, marker) {
				t.Fatalf("control: full build lacks %q, so this check proves nothing", label)
			}
			if bytes.Contains(guiBytes, marker) {
				t.Errorf("launcher-only build contains %q", label)
			}
		}
		if !bytes.Contains(guiBytes, []byte(guiTestVersion)) {
			t.Errorf("launcher-only build does not carry the -X version %q", guiTestVersion)
		}
	})

	t.Run("refuses every other mode without binding or writing", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		l.Close()
		data := filepath.Join(t.TempDir(), "data")
		for _, args := range [][]string{
			nil,
			{""},
			{"-listen", addr},
			{"-init-config"},
			{"-data", data, "-init-config", "-listen", addr},
			{"--help"},
			{"-h"},
			{"version"},
			{"--version"},
			{"serve"},
			{"gui-catalog-readonly"},
			{"--gui-disposable-inventoryx"},
			{"--GUI-CATALOG-READONLY"},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, gui, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			timedOut := ctx.Err() == context.DeadlineExceeded
			cancel()
			if timedOut {
				t.Fatalf("%q did not exit promptly; a server may be running", args)
			}
			var exit *exec.ExitError
			if !errorsAs(err, &exit) || exit.ExitCode() != 2 {
				t.Errorf("%q: err=%v, want exit status 2", args, err)
			}
			want := "Obelisk " + guiTestVersion + ": this build contains only --gui-disposable-inventory and --gui-catalog-readonly"
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("%q: refusal message missing: %q", args, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("%q: refusal wrote stdout: %q", args, stdout.String())
			}
			// Nothing may hold the port after the refused run.
			l, err := net.Listen("tcp", addr)
			if err != nil {
				t.Errorf("%q: port %s is not free after the refused run: %v", args, addr, err)
			} else {
				l.Close()
			}
		}
		if _, err := os.Stat(data); !os.IsNotExist(err) {
			t.Errorf("a refused mode created the data directory (stat err=%v)", err)
		}
	})

	t.Run("inventory and catalog modes work and report the version", func(t *testing.T) {
		root := t.TempDir()
		src, out := filepath.Join(root, "source"), filepath.Join(root, "out")
		for _, d := range []string{src, out} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(src, "one.txt"), []byte("one\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog := filepath.Join(out, "snap.json")
		b, err := exec.Command(gui, "--gui-disposable-inventory", src, catalog).Output()
		if err != nil {
			t.Fatalf("inventory mode: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		var res struct {
			Version   string `json:"version"`
			Published bool   `json:"published"`
		}
		if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil || !res.Published {
			t.Fatalf("inventory mode did not publish: %s", b)
		}
		if res.Version != guiTestVersion {
			t.Errorf("inventory reports version %q, want %q", res.Version, guiTestVersion)
		}

		cmd := exec.Command(gui, "--gui-catalog-readonly", catalog)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		first, err := bufio.NewReader(stdout).ReadBytes('\n')
		stdin.Close()
		if werr := cmd.Wait(); werr != nil {
			t.Errorf("catalog reader exit: %v", werr)
		}
		if err != nil {
			t.Fatalf("catalog reader gave no first response: %v", err)
		}
		var hello struct {
			OK      bool            `json:"ok"`
			Version string          `json:"version"`
			Catalog json.RawMessage `json:"catalog"`
		}
		if err := json.Unmarshal(first, &hello); err != nil || !hello.OK || len(hello.Catalog) == 0 {
			t.Fatalf("catalog reader did not load: %s", first)
		}
		if hello.Version != guiTestVersion {
			t.Errorf("catalog reader reports version %q, want %q", hello.Version, guiTestVersion)
		}
	})
}

func errorsAs(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// The launcher-only build keeps exact copies of a few main.go helpers. Compare
// the printed declarations so a change to one side cannot go unnoticed.
func TestGUIOnlyHelpers_MatchMainGo(t *testing.T) {
	decls := func(file string) (map[string]string, string) {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, nil, 0) // no comments: doc text may differ
		if err != nil {
			t.Fatal(err)
		}
		funcs := map[string]string{}
		version := ""
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				var b bytes.Buffer
				if err := printer.Fprint(&b, fset, d); err != nil {
					t.Fatal(err)
				}
				funcs[d.Name.Name] = strings.Join(strings.Fields(b.String()), " ")
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Names) == 1 && vs.Names[0].Name == "appVersion" && len(vs.Values) == 1 {
						if lit, ok := vs.Values[0].(*ast.BasicLit); ok {
							version = lit.Value
						}
					}
				}
			}
		}
		return funcs, version
	}
	mainFuncs, mainVersion := decls("main.go")
	guiFuncs, guiVersion := decls("main_gui.go")
	if mainVersion == "" || mainVersion != guiVersion {
		t.Errorf("appVersion default differs: main.go %s, main_gui.go %s", mainVersion, guiVersion)
	}
	for name, body := range guiFuncs {
		if name == "main" {
			continue
		}
		want, ok := mainFuncs[name]
		if !ok {
			t.Errorf("main_gui.go defines %s, which main.go does not", name)
			continue
		}
		if body != want {
			t.Errorf("main_gui.go %s differs from main.go:\n--- main.go\n%s\n--- main_gui.go\n%s", name, want, body)
		}
	}
}
