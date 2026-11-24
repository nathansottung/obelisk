//go:build !guionly

package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"
	"testing"
)

// removalFiles are the only files that may call a function that removes or
// truncates a file: the scratch helper (Obelisk's own files) and the delete
// gate (everything else, after the typed word).
var removalFiles = map[string]bool{
	"scratch.go": true, "scratch_windows.go": true, "scratch_unix.go": true, "scratch_other.go": true,
	"destructive.go": true, "destructive_ops.go": true,
}

// removalFuncs remove, unlink or truncate.
var removalFuncs = map[string]bool{
	// This package's own general removals: only the helper and the gate's own
	// file may call them (gateOnly covers removeGatedEntry).
	"main.removeEntryIn": true, "main.removeEntryInAs": true, "main.removeEntryInStep": true, "main.removeTreeIn": true,
	"syscall.Ftruncate": true, "syscall.Truncate": true,
	"golang.org/x/sys/unix.Ftruncate": true, "golang.org/x/sys/unix.Truncate": true,
	"golang.org/x/sys/windows.SetEndOfFile":        true,
	"golang.org/x/sys/windows.FileDispositionInfo": true, "golang.org/x/sys/windows.FileDispositionInfoEx": true,
	"os.Remove": true, "os.RemoveAll": true, "os.Truncate": true,
	"(*os.File).Truncate": true, "(*os.Root).Remove": true, "(*os.Root).RemoveAll": true,
	"syscall.Unlink": true, "syscall.Rmdir": true, "syscall.DeleteFile": true, "syscall.RemoveDirectory": true,
	"golang.org/x/sys/unix.Unlink": true, "golang.org/x/sys/unix.Unlinkat": true, "golang.org/x/sys/unix.Rmdir": true,
	"golang.org/x/sys/windows.DeleteFile": true, "golang.org/x/sys/windows.RemoveDirectory": true,
	"golang.org/x/sys/windows.FileDispositionInformation": true, "golang.org/x/sys/windows.FileDispositionInformationEx": true,
	"golang.org/x/sys/windows.FILE_FLAG_DELETE_ON_CLOSE": true, "golang.org/x/sys/windows.FILE_DELETE_ON_CLOSE": true,
}

// replaceFuncs give an existing name to another file, or truncate one on open.
var replaceFuncs = map[string]bool{
	"os.Rename": true, "(*os.Root).Rename": true, "os.WriteFile": true, "os.Create": true,
	"(*os.Root).Create": true, "(*os.Root).WriteFile": true,
	"os.O_TRUNC": true, "syscall.O_TRUNC": true, "golang.org/x/sys/unix.O_TRUNC": true,
	"syscall.Rename": true, "syscall.MoveFile": true,
	"golang.org/x/sys/unix.Rename": true, "golang.org/x/sys/unix.Renameat": true, "golang.org/x/sys/unix.Renameat2": true,
	"golang.org/x/sys/windows.MoveFile": true, "golang.org/x/sys/windows.MoveFileEx": true,
	"golang.org/x/sys/windows.MOVEFILE_REPLACE_EXISTING": true,
	"golang.org/x/sys/windows.CREATE_ALWAYS":             true, "golang.org/x/sys/windows.TRUNCATE_EXISTING": true,
	"golang.org/x/sys/windows.FileRenameInformation": true, "golang.org/x/sys/windows.FileRenameInformationEx": true,
	"golang.org/x/sys/windows.FileRenameInfo": true, "golang.org/x/sys/windows.FileRenameInfoEx": true,
	// This package's own replacing renames.
	"main.renameReplacing": true, "main.renameFile": true, "main.atomicRename": true,
}

// replaceFiles may replace names: the helper, the gate, the staged-build swap
// (renames that never replace) and the one replacing rename itself.
var replaceFiles = map[string]bool{
	"staged_swap.go": true, "rename_retry.go": true, "rename_retry_windows.go": true, "rename_retry_other.go": true,
}

// replaceAllowed are the functions that save Obelisk's own state by atomic
// replace (a new copy is written beside it, then given its name), or write a
// file into a folder this same operation just made. Each is a save, not a
// removal: the earlier contents are Obelisk's own record of the same thing.
var replaceAllowed = map[string]string{
	// Saves of Obelisk's own records.
	"store.go:Store.writeCatalogJSON":                 "the catalog (catalog.json)",
	"store.go:Store.writeJobsRows":                    "the job board",
	"config.go:App.publishConfig":                     "the settings",
	"heldsettings.go:App.saveHeldSettings":            "settings held from a backup restore",
	"pipeline.go:writeStoreObserved":                  "a keystore, rewritten with the keys it already holds and any new one",
	"session_signin.go:sessionAuth.publishSigninFile": "this run's sign-in file, in Obelisk's private sign-in folder",
	"catalogdb_migrate.go:writeCatalogStub":           "catalog.json becomes the pointer to catalog.db once its bytes are kept beside it",
	"escrow.go:writeEscrowDownloads":                  "the cache's record of what it downloaded",
	"escrow.go:publishEscrowDownload":                 "replaces only a file the cache downloaded itself, checked by identity first",
	"dvdisaster.go:App.generateDiscEcc":               "the .ecc of a staged package Obelisk built, replaced only by a finished one",
	"burner.go:App.BurnNext":                          "COPY-ID.txt, a name Obelisk keeps for itself in a staged package it built",
	"writer.go:copyFileAs":                            "replaces only through copyFileOver (see copyFileOverCallers)",
	"mirror.go:atomicRename":                          "used only by the app-backup restore (gateOnly)",
	"mirror.go:renameFile":                            "the rename primitive behind renameReplacing",
	// The two restores the gate runs after the typed word (gateOnly).
	"appbackup.go:App.RestoreAppBackup": "the gate's appbackup-restore: restored members replace live ones",
	"catalogdb_backup.go:swapCatalogDB": "the gate's appbackup-restore: the restored catalog database is swapped in",
	"outfolder.go:outFolder.Commit":     "replaces only in a folder opened by openOutFolderOver (gateOnly), or Obelisk's own seal file checked by content",
	// Moves that never replace.
	"publish_absent_windows.go:publishAbsent":      "MoveFileEx without MOVEFILE_REPLACE_EXISTING: never replaces",
	"deststage_noreplace_linux.go:renameNoReplace": "renameat2 with RENAME_NOREPLACE: never replaces",
	"quarantine.go:moveWithinRoot":                 "moves a file within its own root only when nothing is at the target (the gate's quarantine, and its undo)",
	// The destination publish; it replaces only for DestRoot.Publish (publishReplaceCallers).
	"deststage_windows.go:handleRename":  "the handle rename behind publishDestStage",
	"deststage_unix.go:publishDestStage": "replace only when DestRoot.Publish asks",
}

// publishReplaceCallers are the only functions that may publish over an
// existing name on a destination (DestRoot.Publish): Obelisk's own sidecars on
// a volume it manages.
var publishReplaceCallers = map[string]bool{
	"destroot_stage.go:DestRoot.WriteFileAtomic":  true,
	"destroot_stage.go:DestRoot.Publish":          true,
	"mirror_previous.go:keepPlan.keepThenPublish": true, // after keeping what is there
	"outfolder.go:outFolder.Commit":               true, // only for openOutFolderOver (gateOnly), or Obelisk's own seal (keepOwn)
}

// copyFileOverCallers may replace a file with copyFileOver.
var copyFileOverCallers = map[string]bool{"burner.go:App.carryEccToNextDisc": true, "writer.go:copyFileOver": true}

// gateOnly are reached only from the delete gate's operations (destructive_ops.go),
// or from the listed functions.
var gateOnly = map[string]map[string]bool{
	"(*main.App).RestoreAppBackup":  {},
	"(*main.App).Quarantine":        {},
	"main.swapRestoredCatalogDB":    {"appbackup.go:App.RestoreAppBackup": true, "catalogdb_backup.go:init": true, "appbackup.go:swapRestoredCatalogDB": true},
	"main.atomicRename":             {"appbackup.go:App.RestoreAppBackup": true},
	"(*main.App).RewriteCopy":       {},
	"(*main.App).WriteChunkOver":    {},
	"(*main.App).SpanWriteNextOver": {},
	// The gate's own removal: only from its operations' run functions.
	"main.removeGatedEntry":         {"leftovers.go:init": true, "export_gate.go:init": true},
	"main.removeGatedEntryIn":       {"mirror_previous_gate.go:init": true},
	"(*main.App).openOutFolderOver": {"writer.go:App.RewriteCopy": true, "writer.go:App.WriteChunkOver": true, "span.go:App.spanWriteNext": true},
}

// objKey names a package-level object the way removalFuncs does.
func objKey(o types.Object) string {
	if f, ok := o.(*types.Func); ok {
		return f.FullName()
	}
	if o.Pkg() == nil || o.Parent() != o.Pkg().Scope() {
		return ""
	}
	return o.Pkg().Path() + "." + o.Name()
}

// Nothing the user owns is removed or overwritten except through the delete
// gate; Obelisk's own scratch is removed only through the scratch helper. This
// reads the source: no other file names a function that removes, truncates or
// renames over a file, except the saves listed in replaceAllowed.
func TestRemovals_OnlyTheHelperAndTheGateRemoveFiles(t *testing.T) {
	fset, files, info := typeCheckPackage(t)
	var bad []string
	used := map[string]bool{}
	for _, f := range files {
		name := fset.Position(f.Pos()).Filename
		for _, decl := range f.Decls {
			fn := declName(decl)
			ast.Inspect(decl, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				o := info.Uses[id]
				if o == nil {
					return true
				}
				key := objKey(o)
				where := name + ":" + fn
				at := fmt.Sprintf("%s:%d (%s)", name, fset.Position(id.Pos()).Line, fn)
				switch {
				case removalFuncs[key] && !removalFiles[name]:
					bad = append(bad, at+": removes with "+key+"; use the scratch helper (scratch.go) or the delete gate")
				case replaceFuncs[key] && !removalFiles[name] && !replaceFiles[name]:
					if _, ok := replaceAllowed[where]; ok {
						used[where] = true
						return true
					}
					bad = append(bad, at+": replaces or truncates with "+key+"; publish without replacing, or list the save in replaceAllowed")
				case key == "(*main.DestRoot).Publish" && !publishReplaceCallers[where]:
					bad = append(bad, at+": publishes over an existing name on a destination; use PublishNew, or the delete gate")
				case key == "main.copyFileOver" && !copyFileOverCallers[where]:
					bad = append(bad, at+": replaces a file with copyFileOver; use copyFile")
				}
				if allowed, ok := gateOnly[key]; ok && name != "destructive_ops.go" && !allowed[where] {
					bad = append(bad, at+": reaches "+key+", which only the delete gate may run")
				}
				return true
			})
		}
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Error(b)
	}
	parsed := map[string]bool{}
	for _, f := range files {
		parsed[fset.Position(f.Pos()).Filename] = true
	}
	for k := range replaceAllowed {
		file, _, _ := strings.Cut(k, ":")
		if parsed[file] && !used[k] { // another platform's file is checked on that platform
			t.Errorf("replaceAllowed lists %s, which no longer replaces anything; remove it from the list", k)
		}
	}
}

// declName names a top-level declaration: Recv.Func for a method, the first
// name for a var, const or type declaration, "init" for init.
func declName(d ast.Decl) string {
	switch v := d.(type) {
	case *ast.FuncDecl:
		if v.Recv != nil && len(v.Recv.List) == 1 {
			return recvName(v.Recv.List[0].Type) + "." + v.Name.Name
		}
		return v.Name.Name
	case *ast.GenDecl:
		for _, sp := range v.Specs {
			switch s := sp.(type) {
			case *ast.ValueSpec:
				return s.Names[0].Name
			case *ast.TypeSpec:
				return s.Name.Name
			}
		}
	}
	return ""
}

func recvName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return recvName(v.X)
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr:
		return recvName(v.X)
	}
	return ""
}
