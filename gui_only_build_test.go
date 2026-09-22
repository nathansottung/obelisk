package main

// gui_only_build_test.go — the launcher-only build (-tags guionly) contains the two
// GUI adapter modes and nothing that serves HTTP: no route table, no embedded UI,
// no server loop. It refuses every other argument without binding a port or
// touching a data directory. Its copies of main.go helpers must not drift.

import (
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

func goTool() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join(runtime.GOROOT(), "bin", "go"+ext)
}

func buildBinary(t *testing.T, dir, name string, tags ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	args := []string{"build", "-mod=readonly", "-trimpath"}
	if len(tags) > 0 {
		args = append(args, "-tags", strings.Join(tags, ","))
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
	gui := buildBinary(t, dir, "obelisk-gui", "guionly")
	full := buildBinary(t, dir, "obelisk-full")

	t.Run("no server, routes or UI linked", func(t *testing.T) {
		guiNM, fullNM := symbolsOf(t, gui), symbolsOf(t, full)
		absent := []string{"main.api", "main.uiFS", "main.runJob", "net/http.(*Server).Serve", "net/http.(*conn).serve"}
		for _, sym := range absent {
			if !hasSymbol(fullNM, sym) {
				t.Fatalf("control: full build lacks %s, so this check proves nothing", sym)
			}
			if hasSymbol(guiNM, sym) {
				t.Errorf("launcher-only build links %s", sym)
			}
		}
		for _, sym := range []string{"main.runGUIInventory", "main.runGUICatalog"} {
			if !hasSymbol(guiNM, sym) {
				t.Errorf("launcher-only build lacks %s", sym)
			}
		}
		guiBytes, _ := os.ReadFile(gui)
		fullBytes, _ := os.ReadFile(full)
		// Strings that exist only in the embedded UI or the route table.
		for _, marker := range []string{"instrument of negentropy", "Paranoid mode", "X-Requested-By", "/api/keys"} {
			if !bytes.Contains(fullBytes, []byte(marker)) {
				t.Fatalf("control: full build lacks %q, so this check proves nothing", marker)
			}
			if bytes.Contains(guiBytes, []byte(marker)) {
				t.Errorf("launcher-only build contains %q", marker)
			}
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
			{"-listen", addr},
			{"-init-config"},
			{"-data", data, "-init-config", "-listen", addr},
			{"--help"},
			{"-h"},
			{"gui-catalog-readonly"},
			{"--gui-disposable-inventoryx"},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, gui, args...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
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
			if !strings.Contains(stderr.String(), "only --gui-disposable-inventory and --gui-catalog-readonly") {
				t.Errorf("%q: refusal message missing: %q", args, stderr.String())
			}
		}
		if _, err := os.Stat(data); !os.IsNotExist(err) {
			t.Errorf("a refused mode created the data directory (stat err=%v)", err)
		}
		l, err = net.Listen("tcp", addr)
		if err != nil {
			t.Errorf("port %s is not free after the refused runs: %v", addr, err)
		} else {
			l.Close()
		}
	})

	t.Run("inventory mode still works", func(t *testing.T) {
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
			Published bool `json:"published"`
		}
		if err := json.Unmarshal([]byte(lines[len(lines)-1]), &res); err != nil || !res.Published {
			t.Fatalf("inventory mode did not publish: %s", b)
		}
		if _, err := os.Stat(catalog); err != nil {
			t.Fatal(err)
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
