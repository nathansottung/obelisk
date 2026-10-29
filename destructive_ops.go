//go:build !guionly

package main

// destructive_ops.go — the actions behind the delete gate (see destructive.go).

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
)

func init() {
	destructiveOps["rewrite-copy"] = &destructiveOp{
		word: wordOverwrite,
		title: func(a *App, p map[string]any) string {
			name := fmt.Sprintf("package %d", paramInt(p, "id"))
			if c := a.Store.Chunk(paramInt(p, "id")); c != nil {
				name = c.Name
			}
			return fmt.Sprintf("Re-write the copy of %s on volume %d from staging", name, paramInt(p, "volume_id"))
		},
		targets: rewriteCopyTargets,
		run: func(a *App, p map[string]any) (any, error) {
			id, vol := paramInt(p, "id"), paramInt(p, "volume_id")
			if err := sealGuard(a, vol); err != nil {
				return nil, refuseDestructive(409, "%s", err.Error())
			}
			c := a.Store.Chunk(id)
			return runJob(a, "write", "Re-write "+c.Name+" copy", func(pr func(float64, string)) (map[string]any, error) {
				return a.RewriteCopy(id, vol, paramFloat(p, "buffer_gb"), paramInt(p, "block_mb"), paramFloat(p, "throttle_mbps"), pr)
			})
		},
	}
	destructiveOps["record-delete"] = &destructiveOp{
		word: wordDelete,
		title: func(a *App, p map[string]any) string {
			return fmt.Sprintf("Delete the %s record %s from the catalog", paramString(p, "kind"), recordIDParam(p))
		},
		targets: recordDeleteTargets,
		run:     runRecordDelete,
	}
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
	if dir := paramString(p, "export_dir"); dir != "" {
		if err := a.outFolderReady(dir); err != nil {
			return nil, refuseDestructive(409, "%s", err.Error())
		}
	}
	files, folders, events := a.Store.FilesOf(id), a.Store.FoldersOf(id), a.Store.Events(id)
	return []destructiveTarget{
		recordTarget(fmt.Sprintf("archive %d (%s)", id, c.Name), c, "removed from the catalog"),
		recordTarget(fmt.Sprintf("%d file record(s) of archive %d", len(files), id), files, "forgotten (the files themselves are not touched)"),
		recordTarget(fmt.Sprintf("%d folder record(s) of archive %d", len(folders), id), folders, "forgotten"),
		recordTarget(fmt.Sprintf("%d event record(s) of archive %d", len(events), id), events, "forgotten"),
	}, nil
}

// recordIDParam is a record-delete's ID: a number, or a profile's string ID.
func recordIDParam(p map[string]any) string {
	if v, ok := p["id"].(string); ok {
		return v
	}
	return fmt.Sprint(paramInt(p, "id"))
}

// recordDeleteTargets is the record a delete removes, and what else it changes:
// for an event, the files whose membership it clears.
func recordDeleteTargets(a *App, p map[string]any) ([]destructiveTarget, error) {
	kind, id := paramString(p, "kind"), paramInt(p, "id")
	switch kind {
	case "event":
		ev := a.Store.Event(id)
		if ev == nil {
			return nil, refuseDestructive(404, "event %d not found", id)
		}
		// Deleting an event clears membership on every file assigned to it, in
		// any archive (an event need not belong to one), so list exactly those.
		var members []int
		for _, f := range a.Store.FilesOfEvent(id) {
			members = append(members, f.ID)
		}
		sort.Ints(members)
		return []destructiveTarget{
			recordTarget(fmt.Sprintf("event %d (%s)", id, ev.Name), ev, "deleted"),
			recordTarget(fmt.Sprintf("%d file(s) in event %d", len(members), id), members, "left in the catalog, no longer in the event"),
		}, nil
	case "template":
		tm := a.Store.Template(id)
		if tm == nil {
			return nil, refuseDestructive(404, "template %d not found", id)
		}
		return []destructiveTarget{recordTarget(fmt.Sprintf("template %d (%s)", id, tm.Name), tm, "deleted")}, nil
	case "plan":
		pl := a.Store.Plan(id)
		if pl == nil {
			return nil, refuseDestructive(404, "plan %d not found", id)
		}
		return []destructiveTarget{recordTarget(fmt.Sprintf("plan %d (%s)", id, pl.Name), pl, "deleted; nothing on disk is affected")}, nil
	case "profile":
		pid := recordIDParam(p)
		pr := a.Store.Profile(pid)
		if pr == nil {
			return nil, refuseDestructive(404, "profile %q not found", pid)
		}
		return []destructiveTarget{recordTarget(fmt.Sprintf("profile %s (%s)", pid, pr.Name), pr, "deleted")}, nil
	}
	return nil, refuseDestructive(400, "unknown record kind %q (event, template, plan or profile)", kind)
}

func runRecordDelete(a *App, p map[string]any) (any, error) {
	kind, id := paramString(p, "kind"), paramInt(p, "id")
	switch kind {
	case "event":
		a.Store.DeleteEvent(id)
	case "template":
		a.Store.DeleteTemplate(id)
	case "plan":
		a.Store.DeletePlan(id)
	case "profile":
		if err := a.Store.DeleteProfile(recordIDParam(p)); err != nil {
			return nil, err
		}
	}
	a.Store.Log("delete", fmt.Sprintf("deleted %s %s", kind, recordIDParam(p)))
	return map[string]any{"ok": true, "deleted": true}, nil
}

// rewriteCopyTargets is the copy's record and every file in the copy's folder,
// which the re-write replaces. The package, the copy and the volume's seal are
// checked here too, so a confirmation is only issued for a re-write that can run.
func rewriteCopyTargets(a *App, p map[string]any) ([]destructiveTarget, error) {
	id, vol := paramInt(p, "id"), paramInt(p, "volume_id")
	c := a.Store.Chunk(id)
	if c == nil {
		return nil, refuseDestructive(404, "package %d not found", id)
	}
	if vol <= 0 {
		return nil, refuseDestructive(400, "volume_id required (the volume whose copy to re-write)")
	}
	if c.Spanned {
		return nil, refuseDestructive(400, "package %s is spanned; re-write its segments via span-write", c.Name)
	}
	if err := sealGuard(a, vol); err != nil {
		return nil, refuseDestructive(409, "%s", err.Error())
	}
	var cp *Copy
	for i := range c.Copies {
		if c.Copies[i].VolumeID == vol && !c.Copies[i].Superseded {
			cp = &c.Copies[i]
			break
		}
	}
	if cp == nil {
		return nil, refuseDestructive(404, "no current copy of %s on volume %d to re-write", c.Name, vol)
	}
	if err := a.outFolderReady(filepath.Dir(cp.Path)); err != nil {
		return nil, refuseDestructive(409, "%s", err.Error())
	}
	out := []destructiveTarget{recordTarget(fmt.Sprintf("copy of %s on volume %d", c.Name, vol), cp, "marked superseded, and a new copy is written")}
	files, err := treeTargets(cp.Path, "replaced by the re-written copy")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if len(files) == 0 {
		files = []destructiveTarget{fileTarget(cp.Path, "the copy's folder (not there now); written by the re-write")}
	}
	return append(out, files...), nil
}

func paramFloat(p map[string]any, k string) float64 {
	v, _ := p[k].(float64)
	return v
}
