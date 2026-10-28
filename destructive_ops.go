//go:build !guionly

package main

// destructive_ops.go — the actions behind the delete gate (see destructive.go).

import (
	"fmt"
	"path/filepath"
)

func init() {
	destructiveOps["archive-remove"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			name := fmt.Sprintf("archive %d", paramInt(p, "id"))
			if c := a.Store.Collection(paramInt(p, "id")); c != nil {
				name = c.Name
			}
			return fmt.Sprintf("Remove the archive %q from the catalog (files on disk and packages on media are kept)", name)
		},
		targets: archiveRemoveTargets,
		run: func(a *App, p map[string]any) (any, error) {
			counts, err := a.RemoveArchive(paramInt(p, "id"), paramString(p, "confirm_name"), paramString(p, "export_dir"))
			if err != nil {
				return nil, err
			}
			return map[string]any{"removed": true, "counts": counts}, nil
		},
	}
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

// archiveRemoveTargets is the archive's record and every record removing it
// forgets: its file, folder and event records. The typed archive name and, for
// an archive with packages, the export folder are checked here as well, so a
// confirmation is only issued for a removal that can run.
func archiveRemoveTargets(a *App, p map[string]any) ([]destructiveTarget, error) {
	id := paramInt(p, "id")
	c := a.Store.Collection(id)
	if c == nil {
		return nil, refuseDestructive(404, "archive %d not found", id)
	}
	if paramString(p, "confirm_name") != c.Name {
		return nil, refuseDestructive(400, "type the archive's exact name (%q) to confirm removal", c.Name)
	}
	if a.archiveGuarded(id) && paramString(p, "export_dir") == "" {
		return nil, refuseDestructive(400, "save a fresh Structure Export first — this archive has packages, so its record is worth keeping a copy of")
	}
	files, folders, events := a.Store.FilesOf(id), a.Store.FoldersOf(id), a.Store.Events(id)
	return []destructiveTarget{
		recordTarget(fmt.Sprintf("archive %d (%s)", id, c.Name), c, "removed from the catalog"),
		recordTarget(fmt.Sprintf("%d file record(s) of archive %d", len(files), id), files, "forgotten (the files themselves are not touched)"),
		recordTarget(fmt.Sprintf("%d folder record(s) of archive %d", len(folders), id), folders, "forgotten"),
		recordTarget(fmt.Sprintf("%d event record(s) of archive %d", len(events), id), events, "forgotten"),
	}, nil
}
