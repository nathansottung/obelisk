package main

// removal.go — permanent archive removal (the guarded, catalog-only "forget"). Retire
// is reversible and lives in the store (SetCollectionRetired); Remove is the one-way
// door, so its guards live here in the App layer where they can be tested directly and
// are enforced server-side, not just in the dialog. NOTHING here touches files on
// disk — it only changes what the app remembers.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RemoveCounts reports what a removal forgot vs. what it deliberately kept.
type RemoveCounts struct {
	Files    int `json:"files"`    // file records forgotten
	Folders  int `json:"folders"`  // source-folder records forgotten
	Packages int `json:"packages"` // packages KEPT (now "from a removed archive")
	Volumes  int `json:"volumes"`  // distinct volumes those kept packages sit on
}

// writeStructureExportTo writes a fresh Structure Export (json + csv + markdown, all
// content-free — hashes and paths only) for an archive into dir, creating dir if
// needed. Returns the written paths. Reuses the same builder/renderers the Recovery
// Kit uses (StructureExport + StructureCSV/StructureMarkdown/exportJSON).
func (a *App) writeStructureExportTo(collectionID int, dir string) ([]string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("choose a folder to save the Structure Export into")
	}
	exp, err := a.StructureExport(collectionID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	base := "STRUCTURE-" + safeName(exp.Archive) + "-" + time.Now().UTC().Format("20060102-150405")
	writes := []struct {
		name string
		data []byte
	}{
		{base + ".json", exportJSON(exp)},
		{base + ".csv", StructureCSV(exp)},
		{base + ".md", []byte(StructureMarkdown(exp))},
	}
	var paths []string
	for _, wr := range writes {
		p := filepath.Join(dir, wr.name)
		if err := os.WriteFile(p, wr.data, 0o644); err != nil {
			return paths, fmt.Errorf("writing %s: %w", wr.name, err)
		}
		paths = append(paths, p)
	}
	a.Store.Log("structure-export", fmt.Sprintf("%s: saved %d-file structure export to %s", exp.Archive, len(exp.Files), dir))
	return paths, nil
}

// archiveGuarded reports whether an archive has any packages (hence copies) — the
// branch that requires a fresh export before Remove. Zero packages = a test/mistake,
// where the typed name alone suffices.
func (a *App) archiveGuarded(collectionID int) bool {
	return len(a.Store.Chunks(collectionID)) > 0
}

// RemoveArchive permanently forgets an archive after enforcing the guards, in order:
//  1. the archive must exist;
//  2. the typed confirmName must match the archive's exact name (BOTH branches);
//  3. if it has packages/copies, a fresh Structure Export must be saved to exportDir
//     first (written here, so the export is a hard precondition — Remove cannot
//     succeed without a real export on disk).
//
// It then deletes the archive's records via Store.RemoveCollection (volumes,
// inventories, and package/copy rows are kept) and audit-logs the counts. Files on
// disk are never touched.
func (a *App) RemoveArchive(id int, confirmName, exportDir string) (RemoveCounts, error) {
	var zero RemoveCounts
	c := a.Store.Collection(id)
	if c == nil {
		return zero, fmt.Errorf("archive %d not found", id)
	}
	name := c.Name
	if strings.TrimSpace(confirmName) != name {
		return zero, fmt.Errorf("type the archive's exact name (%q) to confirm removal", name)
	}
	if a.archiveGuarded(id) {
		if strings.TrimSpace(exportDir) == "" {
			return zero, fmt.Errorf("save a fresh Structure Export first — this archive has packages, so its record is worth keeping a copy of")
		}
		if _, err := a.writeStructureExportTo(id, exportDir); err != nil {
			return zero, err
		}
	}
	counts, err := a.Store.RemoveCollection(id)
	if err != nil {
		return zero, err
	}
	a.Store.Log("remove", fmt.Sprintf("%s: forgot %d file(s), %d folder(s); kept %d package(s) on %d volume(s) — files on disk untouched",
		name, counts.Files, counts.Folders, counts.Packages, counts.Volumes))
	return counts, nil
}
