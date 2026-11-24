package main

// export_names.go — exports never replace an earlier export.
//
// Each Recovery Kit, escrow bundle, BagIt export and app backup is written under
// a new name that sorts by when it was made (<prefix>-YYYYMMDD-HHMMSS, with -2,
// -3, … if that second is taken), so it never meets an earlier one. Each states
// inside it when it was made, what it covers, and which earlier exports in the
// same folder it supersedes (EXPORT-INFO.txt; for an app backup, its manifest).
// An earlier export is never changed: it is marked superseded only by the newer
// one naming it, and by the export screen, which lists every export of the kind
// in the folder, newest first, with its size, and removes one only through the
// delete gate ("export-remove", typed "Delete").

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// exportInfoName is the file each folder export carries about itself.
const exportInfoName = "EXPORT-INFO.txt"

// exportKind is one kind of export and how its names look.
type exportKind struct {
	key    string // kit | escrow | bag | appbackup
	label  string
	prefix string // name prefix; the time follows
	ext    string // ".tar" for an app backup; "" for a folder
}

var exportKinds = map[string]exportKind{
	"kit":       {key: "kit", label: "Recovery Kit", prefix: "obelisk-recovery-kit"},
	"escrow":    {key: "escrow", label: "escrow bundle", prefix: escrowBundleDir},
	"bag":       {key: "bag", label: "BagIt export", prefix: ""}, // <name>-bag
	"appbackup": {key: "appbackup", label: "app backup", prefix: appBackupName + "-appbackup", ext: ".tar"},
}

// exportStamp is the time part of an export's name.
func exportStamp(t time.Time) string { return t.UTC().Format("20060102-150405") }

// freshExportName is base-<time><ext>, or base-<time>-N<ext>, whichever is not
// taken in out.
func freshExportName(out *outFolder, base, ext string, now time.Time) string {
	name := base + "-" + exportStamp(now)
	for i := 2; ; i++ {
		if _, err := out.Stat(name + ext); err != nil {
			return name + ext
		}
		name = fmt.Sprintf("%s-%s-%d", base, exportStamp(now), i)
	}
}

// matchesExport reports whether name is an export of kind (a bag's prefix is
// the bag's own base name, so any name with "-bag" matches it).
func matchesExport(k exportKind, name string) bool {
	if k.key == "bag" {
		return strings.Contains(name, "-bag") && (k.prefix == "" || name == k.prefix || strings.HasPrefix(name, k.prefix+"-"))
	}
	if k.ext != "" {
		return strings.HasPrefix(name, k.prefix+"-") && strings.HasSuffix(name, k.ext)
	}
	return name == k.prefix || strings.HasPrefix(name, k.prefix+"-")
}

// exportInfo is the EXPORT-INFO.txt text: when, by what, what it covers, and the
// earlier exports in the same folder it supersedes.
func exportInfo(k exportKind, name string, now time.Time, covers []string, supersedes []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "Obelisk %s\n\n", k.label)
	fmt.Fprintf(&b, "Name:  %s\n", name)
	fmt.Fprintf(&b, "Made:  %s (UTC)\n", now.UTC().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "By:    Obelisk %s\n\n", appVersion)
	b.WriteString("Covers:\n")
	for _, c := range covers {
		fmt.Fprintf(&b, "  - %s\n", c)
	}
	if len(supersedes) > 0 {
		b.WriteString("\nSupersedes these earlier exports in the same folder (left exactly as they were):\n")
		for _, s := range supersedes {
			fmt.Fprintf(&b, "  - %s\n", s)
		}
	}
	b.WriteString("\nNames sort by the time they were made: the last one is the newest.\n")
	return []byte(b.String())
}

// earlierExports lists the exports of kind already in out, oldest first.
func earlierExports(out *outFolder, k exportKind, except string) []string {
	entries, err := out.readDir()
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.Name() != except && matchesExport(k, e.Name()) && !strings.HasSuffix(e.Name(), ".sha256") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// ExportEntry is one export in a folder, for the export screen.
type ExportEntry struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Made       time.Time `json:"made"`
	Bytes      int64     `json:"bytes"`
	Newest     bool      `json:"newest"`
	Superseded bool      `json:"superseded"`
}

// ExportsIn lists the exports of kind in dir, newest first, with their sizes:
// the newest is marked, every older one is labelled superseded.
func ExportsIn(dir, kind string) ([]ExportEntry, int64, error) {
	k, ok := exportKinds[kind]
	if !ok {
		return nil, 0, fmt.Errorf("unknown export kind %q", kind)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, 0, err
	}
	defer root.Close()
	d, err := root.Open(".")
	if err != nil {
		return nil, 0, err
	}
	entries, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return nil, 0, err
	}
	var out []ExportEntry
	var total int64
	for _, e := range entries {
		n := e.Name()
		if !matchesExport(k, n) || strings.HasSuffix(n, ".sha256") {
			continue
		}
		fi, err := root.Lstat(n)
		if err != nil || isLinkMode(fi.Mode()) {
			continue
		}
		size := fi.Size()
		if fi.IsDir() {
			size = treeSizeIn(root, n)
		} else if sfi, err := root.Lstat(n + ".sha256"); err == nil && sfi.Mode().IsRegular() {
			size += sfi.Size()
		}
		out = append(out, ExportEntry{Name: n, Path: filepath.Join(dir, n), Made: fi.ModTime(), Bytes: size})
		total += size
	}
	// Names sort by the time they were made; an older-style name without a time
	// sorts before them.
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	// The newest of each export: for bags, of each bag's own name.
	seen := map[string]bool{}
	for i := range out {
		g := ""
		if k.key == "bag" {
			g, _, _ = strings.Cut(out[i].Name, "-bag")
		}
		out[i].Newest, out[i].Superseded = !seen[g], seen[g]
		seen[g] = true
	}
	return out, total, nil
}

// readDir lists the folder's top level.
func (o *outFolder) readDir() ([]os.DirEntry, error) {
	r := o.root
	if o.dest != nil {
		r = o.dest.root
	}
	d, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.ReadDir(-1)
}
