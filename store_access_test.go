//go:build !guionly

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// storeFiles may reach the catalog's records directly: the store and the two
// engines behind it. The GUI preview reader and inventory writer build their own
// catalog documents (never the store's) and are listed separately.
var storeFiles = map[string]bool{
	"store.go": true, "store_files.go": true, "store_chunks.go": true, "store_drives.go": true, "store_plans.go": true,
	"catalogdb_rows.go": true, "catalogdb_migrate.go": true, "catalogdb_store.go": true,
	"catalogdb_files.go": true, "catalogdb_chunks.go": true, "catalogdb_drives.go": true,
	"store_jobs.go": true, "catalogdb_jobs.go": true,
}

// snapshotBuilders build a new drive snapshot from a scan, so they set and read
// its files before handing it to the store. No other code reads a snapshot's
// files as a field.
var snapshotBuilders = map[string]bool{"buildSnapshot": true, "recordScan": true}

var catalogDocumentFiles = map[string]bool{"gui_catalog.go": true, "gui_inventory.go": true, "gui_inventory_scope.go": true}

// typeCheckPackage parses this package's non-test Go files for the current
// platform and type-checks them against the compiled export data of their imports.
func typeCheckPackage(t *testing.T) (*token.FileSet, []*ast.File, *types.Info) {
	t.Helper()
	out, err := exec.Command(goTool(), "list", "-f", "{{join .GoFiles \"\\n\"}}", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	exports := map[string]string{}
	exp, err := exec.Command(goTool(), "list", "-export", "-deps", "-f", "{{.ImportPath}}={{.Export}}", ".").Output()
	if err != nil {
		t.Fatalf("go list -export: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(exp)), "\n") {
		if path, file, ok := strings.Cut(strings.TrimSpace(line), "="); ok && file != "" {
			exports[path] = file
		}
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range strings.Fields(string(out)) {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}, Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		file, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	})}
	if _, err := conf.Check("main", fset, files, info); err != nil {
		t.Fatalf("type-check: %v", err)
	}
	return fset, files, info
}

// fieldOf names the struct type and field a selector reads, when it reads a field
// of a named struct type of this package.
func fieldOf(info *types.Info, sel *ast.SelectorExpr) (typ, field string) {
	s := info.Selections[sel]
	if s == nil || s.Kind() != types.FieldVal {
		return "", ""
	}
	recv := s.Recv()
	for {
		p, ok := recv.(*types.Pointer)
		if !ok {
			break
		}
		recv = p.Elem()
	}
	named, ok := recv.(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Name() != "main" {
		return "", ""
	}
	return named.Obj().Name(), sel.Sel.Name
}

// rootSelectors lists the selectors along an lvalue: x.A[i].B → x.A, x.A[i].B.
func rootSelectors(e ast.Expr) []*ast.SelectorExpr {
	var out []*ast.SelectorExpr
	for {
		switch v := e.(type) {
		case *ast.SelectorExpr:
			out = append(out, v)
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		default:
			return out
		}
	}
}

// Every reach into the catalog's own records goes through the store: no code
// outside it reads the store's lists of files, packages, drive snapshots, scans,
// comparisons, plans, drift reports or audit entries, reads a package's listings, a snapshot's
// files, a plan's maps or a drift report's items, or changes a package's copies,
// segments or verify events, so on the database engine (where those lists are
// not in memory) nothing is missed.
func TestPackages_OnlyTheStoreTouchesCatalogRecords(t *testing.T) {
	fset, files, info := typeCheckPackage(t)
	never := map[string]bool{ // read or written only by the store
		"catalog.Files": true, "catalog.Chunks": true,
		"Chunk.Files": true, "Chunk.UnknownFiles": true,
		"catalog.Snapshots": true, "catalog.Scans": true, "catalog.CompareRuns": true, "catalog.Plans": true,
		"catalog.Drift": true, "VolumeSnapshot.Files": true, "DriftReport.Items": true, "catalog.Audit": true,
		"Plan.Overrides": true, "Plan.Parked": true, "Plan.Mapping": true, "Plan.Satisfied": true,
	}
	changedOnlyByStore := map[string]bool{"Chunk.Copies": true, "Chunk.Segments": true, "Chunk.VerifyEvents": true}
	var bad []string
	report := func(n ast.Node, what string) {
		p := fset.Position(n.Pos())
		bad = append(bad, fmt.Sprintf("%s:%d: %s", p.Filename, p.Line, what))
	}
	for _, f := range files {
		name := fset.Position(f.Pos()).Filename
		if storeFiles[name] {
			continue
		}
		writes := map[*ast.SelectorExpr]bool{}
		markWrite := func(e ast.Expr) {
			for _, sel := range rootSelectors(e) {
				writes[sel] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.AssignStmt:
				for _, l := range v.Lhs {
					markWrite(l)
				}
			case *ast.IncDecStmt:
				markWrite(v.X)
			case *ast.UnaryExpr:
				if v.Op == token.AND {
					markWrite(v.X)
				}
			}
			return true
		})
		fn := ""
		ast.Inspect(f, func(n ast.Node) bool {
			if d, ok := n.(*ast.FuncDecl); ok {
				fn = d.Name.Name
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			typ, field := fieldOf(info, sel)
			key := typ + "." + field
			switch {
			case key == "VolumeSnapshot.Files" && snapshotBuilders[fn]:
			case never[key] && !(catalogDocumentFiles[name] && typ == "catalog") && !(catalogDocumentFiles[name] && typ == "Chunk"):
				report(sel, "reaches "+key+" directly; use a Store method")
			case changedOnlyByStore[key] && writes[sel]:
				report(sel, "changes "+key+" directly; use a Store method")
			}
			return true
		})
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Error(b)
	}
}
