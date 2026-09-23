package main

// exec_guard_test.go — every helper subprocess must start through the hardening
// helpers (helperCommand, helperCommandContext, burnShellCommand), which give it
// the allowlisted environment. This test parses every non-test Go file in the
// package, whatever its build tags, and fails if a process is started any other
// way: exec.Command or exec.CommandContext outside those helpers, a hand-built
// exec.Cmd, os.StartProcess or the syscall process calls.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var execGuardAllowed = map[string]bool{
	"hardening.go:helperCommand":        true,
	"hardening.go:helperCommandContext": true,
	"hardening.go:burnShellCommand":     true,
}

func TestProcessStartsOnlyThroughHardeningHelpers(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var violations, allowed []string
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		// Local names for the packages that can start a process.
		names := map[string]string{} // local name -> import path
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path != "os/exec" && path != "os" && path != "syscall" {
				continue
			}
			name := filepath.Base(path)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			if name == "." {
				violations = append(violations, fset.Position(imp.Pos()).String()+": dot import of "+path)
				continue
			}
			names[name] = path
		}
		if len(names) == 0 {
			continue
		}
		for _, decl := range f.Decls {
			fn := "(package level)"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				fn = fd.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				var what string
				switch n := n.(type) {
				case *ast.SelectorExpr:
					id, ok := n.X.(*ast.Ident)
					if !ok {
						return true
					}
					switch names[id.Name] + "." + n.Sel.Name {
					case "os/exec.Command", "os/exec.CommandContext":
						what = "exec." + n.Sel.Name
					case "os.StartProcess", "syscall.StartProcess", "syscall.ForkExec", "syscall.CreateProcess":
						what = id.Name + "." + n.Sel.Name
					}
				case *ast.CompositeLit:
					if sel, ok := n.Type.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] == "os/exec" && sel.Sel.Name == "Cmd" {
							what = "exec.Cmd literal"
						}
					}
				}
				if what == "" {
					return true
				}
				where := fset.Position(n.Pos()).String() + " in " + fn + ": " + what
				if execGuardAllowed[file+":"+fn] && strings.HasPrefix(what, "exec.Command") {
					allowed = append(allowed, where)
				} else {
					violations = append(violations, where)
				}
				return true
			})
		}
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("process started outside the hardening helpers: %s", v)
	}
	// Control: the scan must see the helpers' own calls, or it proves nothing.
	if len(allowed) < 4 {
		t.Errorf("scan found only %d exec.Command calls inside the hardening helpers, want at least 4: %v", len(allowed), allowed)
	}
}
