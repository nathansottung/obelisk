//go:build !guionly

package main

// destructive_ops.go — the actions behind the delete gate (see destructive.go).

import (
	"fmt"
	"path/filepath"
)

func init() {
	destructiveOps["quarantine"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Quarantine %s (moved into %s; reversible)", paramString(p, "path"), QuarantineDir)
		},
		targets: func(a *App, p map[string]any) ([]destructiveTarget, error) {
			path := paramString(p, "path")
			t, err := a.Store.resolveQuarantine(path)
			if err != nil {
				return nil, err
			}
			return treeTargets(t.real, "moved into "+QuarantineDir)
		},
		run: func(a *App, p map[string]any) (any, error) {
			e, err := a.Quarantine(paramString(p, "path"), "user", paramString(p, "reason"))
			if err != nil {
				return nil, err
			}
			return map[string]any{"ok": true, "entry": e}, nil
		},
	}
	destructiveOps["appbackup-restore"] = &destructiveOp{
		word: wordOverwrite,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Restore the app's records from %s", paramString(p, "tar_path"))
		},
		targets: appBackupRestoreTargets,
		run: func(a *App, p map[string]any) (any, error) {
			res, err := a.RestoreAppBackup(paramString(p, "tar_path"))
			if err != nil {
				return nil, err
			}
			a.Store.Log("restore", fmt.Sprintf("app backup restored: %d archives, %d files, %d volumes, %d packages, %d jobs",
				res.Archives, res.Files, res.Volumes, res.Packages, res.Jobs))
			return res, nil
		},
	}
}

// appBackupRestoreTargets is the backup file itself and every file in the data
// folder the restore would replace or create: the records it carries and, for
// each keystore it carries, the keystore of that name in the data folder.
func appBackupRestoreTargets(a *App, p map[string]any) ([]destructiveTarget, error) {
	tarPath := paramString(p, "tar_path")
	if tarPath == "" {
		return nil, refuseDestructive(400, "tar_path required")
	}
	man, _, err := verifyAppBackup(tarPath)
	if err != nil {
		return nil, err
	}
	out := []destructiveTarget{fileTarget(tarPath, "the backup restored from (read only)")}
	for _, m := range man.Members {
		var dest string
		switch {
		case m.Name == "MANIFEST.json":
			continue
		case len(m.Name) > len("keystores/") && m.Name[:len("keystores/")] == "keystores/":
			dest = filepath.Join(a.DataDir, "keystores", filepath.Base(m.Name))
		default:
			dest = filepath.Join(a.DataDir, m.Name)
		}
		t := fileTarget(dest, "replaced by the backup's copy (the current one is copied to pre-restore first)")
		if t.Kind == "missing" {
			t.Note = "created from the backup"
		}
		out = append(out, t)
	}
	return out, nil
}
