package main

// appbackup.go — one-file export/restore of the app's COMPLETE state, so the
// verification history and everything the tool knows can travel with (or ahead of)
// the data to a new machine or OS. The bundle is a plain, uncompressed tar
// (inspectable with stock `tar`, per doctrine) plus a `.sha256` sidecar over the
// whole file.
//
// What travels: the catalog (files, hashes, volumes, locations, copies, packages,
// segments, events, plans, conflicts, version histories, drift/scrub reports, dock
// sessions, profiles, templates), config, the persisted job history with artifacts,
// and any user format-registry overrides. Keystores are SECRETS the user may keep on
// separate devices; they are EXCLUDED by default and only ride along on explicit
// opt-in.
//
// Restore is defensive: it verifies the sidecar and every member hash BEFORE it
// touches the data dir, refuses a newer-schema backup (update the app first) up
// front, backs up the current records, then extracts and reopens the store — which
// runs the ordinary schema-migration registry to bring an older backup forward.
// Volumes reconnect by serial automatically the next time they're seen (see
// Store.VolumeBySerial); restore does nothing special for that.

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	appBackupFormat = "obelisk-appbackup"
	// appBackupFormatLegacy is the pre-rename marker. Bundles created as Mnemosyne
	// carry it and must import/restore forever — see verifyAppBackup and the "Name
	// compatibility" section in ARCHITECTURE.md.
	appBackupFormatLegacy = "mnemosyne-appbackup"
	appBackupVersion      = 1
	appBackupName         = "obelisk" // filename stem: <appname>-appbackup-<date>.tar
)

// appBackupMember is one file inside the bundle, with its SHA-256 for the manifest.
type appBackupMember struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// appBackupManifest is MANIFEST.json — the self-describing header. It lists a
// SHA-256 of every OTHER member (never itself); the whole-tar .sha256 sidecar covers
// the manifest too.
type appBackupManifest struct {
	Format        string            `json:"format"`
	Version       int               `json:"version"`
	SchemaVersion int               `json:"schema_version"`
	AppVersion    string            `json:"app_version"`
	Created       string            `json:"created"`
	IncludesKeys  bool              `json:"includes_keys"`
	Members       []appBackupMember `json:"members"`
}

// ExportResult is the summary returned after writing a bundle.
type ExportResult struct {
	TarPath       string `json:"tar_path"`
	SidecarPath   string `json:"sidecar_path"`
	Bytes         int64  `json:"bytes"`
	MemberCount   int    `json:"member_count"`
	IncludesKeys  bool   `json:"includes_keys"`
	SchemaVersion int    `json:"schema_version"`
	// ScanFilesLeftOut are scan file lists not put in the backup: missing, or not
	// matching their record.
	ScanFilesLeftOut []string `json:"scan_files_left_out,omitempty"`
}

// RestoreResult is the post-restore summary.
type RestoreResult struct {
	// ScanFilesSkipped are the backup's scan file lists not restored because they
	// do not match a scan record in the backup's catalog.
	ScanFilesSkipped []string `json:"scan_files_skipped,omitempty"`
	SchemaVersion    int      `json:"schema_version"`
	AppVersion       string   `json:"app_version"`
	Created          string   `json:"created"`
	IncludedKeys     bool     `json:"included_keys"`
	Archives         int      `json:"archives"`
	Files            int      `json:"files"`
	Volumes          int      `json:"volumes"`
	Packages         int      `json:"packages"`
	Jobs             int      `json:"jobs"`
	VerifyEvents     int      `json:"verify_events"`
	PreRestoreDir    string   `json:"pre_restore_dir"`
	ReadOnly         bool     `json:"read_only,omitempty"`
	ReadOnlyWhy      string   `json:"read_only_why,omitempty"`
	// HeldSettings are the backup's program settings, not applied (see
	// heldsettings.go); nil when they match the current ones.
	HeldSettings *HeldSettings `json:"held_settings,omitempty"`
	// KeystoreNote says where restored keystores are when they were not made
	// the keystore paths.
	KeystoreNote string `json:"keystore_note,omitempty"`
}

// rawMember is a gathered file: its in-tar name and exact bytes.
type rawMember struct {
	name string
	data []byte
}

// gatherMembers reads the data-dir files that make up the app state. config.json is
// re-marshaled with the auth token scrubbed (a deployment secret, not knowledge);
// keystores are included only when opted in. Missing optional files are skipped.
func (a *App) gatherMembers(includeKeys bool) ([]rawMember, error) {
	var members []rawMember

	// catalog.json — the exact on-disk bytes (the source of truth).
	catalog, err := os.ReadFile(filepath.Join(a.DataDir, "catalog.json"))
	if err != nil {
		return nil, fmt.Errorf("read catalog.json: %w", err)
	}
	members = append(members, rawMember{"catalog.json", catalog})

	// config.json — scrub the auth token so a deployment secret never travels.
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return nil, cfgErr
	}
	cfg.AuthToken = ""
	cfgBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	members = append(members, rawMember{"config.json", cfgBytes})

	// jobs.json — persisted job board (history + artifacts). Optional (a brand-new
	// install may have none).
	if b, err := os.ReadFile(filepath.Join(a.DataDir, "jobs.json")); err == nil {
		members = append(members, rawMember{"jobs.json", b})
	}
	// formats.json — user format-registry overrides. Optional.
	if b, err := os.ReadFile(filepath.Join(a.DataDir, "formats.json")); err == nil {
		members = append(members, rawMember{"formats.json", b})
	}

	if includeKeys {
		seen := map[string]bool{}
		for _, ksPath := range cfg.KeystorePaths {
			b, err := os.ReadFile(ksPath)
			if err != nil {
				return nil, fmt.Errorf("read keystore %q: %w", ksPath, err)
			}
			name := "keystores/" + filepath.Base(ksPath)
			if seen[name] { // two keystores with the same basename — disambiguate
				name = fmt.Sprintf("keystores/%d-%s", len(members), filepath.Base(ksPath))
			}
			if !validAppBackupMemberName(name) {
				return nil, fmt.Errorf("keystore filename %q is not portable in an app backup; rename it before exporting", filepath.Base(ksPath))
			}
			seen[name] = true
			members = append(members, rawMember{name, b})
		}
	}
	return members, nil
}

// exportAppBackupTo writes the bundle to exactly tarPath (+ .sha256 sidecar),
// through an output folder opened on tarPath's folder (see exportAppBackupInto).
func (a *App) exportAppBackupTo(tarPath string, includeKeys bool) (ExportResult, error) {
	out, err := a.openOutFolder(filepath.Dir(tarPath))
	if err != nil {
		return ExportResult{}, err
	}
	defer out.Close()
	return a.exportAppBackupInto(out, filepath.Base(tarPath), includeKeys)
}

// exportAppBackupInto writes the bundle as name (+ name.sha256) in out. Core used
// by both the manual export (dated filename) and the auto-export ticker
// (per-period filename). The bundle is staged in a new file and named only once
// it reads back as written.
func (a *App) exportAppBackupInto(out *outFolder, name string, includeKeys bool) (ExportResult, error) {
	members, err := a.gatherMembers(includeKeys)
	if err != nil {
		return ExportResult{}, err
	}
	// scans/<name> — each recorded drive scan's file list (scanhistory.go), so the
	// scan history moves with the catalog. A list that is missing, or does not
	// match its record's SHA-256, is left out and reported; its record then has
	// no file list on the restored side. It never fails the backup.
	var leftOut []string
	lists := a.Store.scanFileLists()
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(a.scansDir(), name))
		if err != nil || sha256Hex(b) != lists[name] {
			leftOut = append(leftOut, name)
			continue
		}
		members = append(members, rawMember{"scans/" + name, b})
	}

	// Schema version comes from the catalog member itself, so the manifest states what
	// the bundle actually holds.
	schemaVer := currentSchemaVersion
	for _, m := range members {
		if m.name == "catalog.json" {
			var head struct {
				SchemaVersion int `json:"schema_version"`
			}
			if json.Unmarshal(m.data, &head) == nil && head.SchemaVersion > 0 {
				schemaVer = head.SchemaVersion
			}
		}
	}

	man := appBackupManifest{
		Format: appBackupFormat, Version: appBackupVersion, SchemaVersion: schemaVer,
		AppVersion: appVersion, Created: time.Now().UTC().Format(time.RFC3339),
		IncludesKeys: includeKeys,
	}
	for _, m := range members {
		man.Members = append(man.Members, appBackupMember{Name: m.name, SHA256: sha256Hex(m.data), Size: int64(len(m.data))})
	}
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return ExportResult{}, err
	}

	perm := os.FileMode(0o644)
	if includeKeys {
		perm = 0o600 // carries keystores: owner-only
	}
	tmp, f, err := out.Create(name)
	if err != nil {
		return ExportResult{}, err
	}
	h := sha256.New()
	tw := tar.NewWriter(io.MultiWriter(f, h))
	now := time.Now().UTC()
	// MANIFEST.json first, then every gathered member.
	write := func(name string, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: now, Format: tar.FormatPAX}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	writeErr := write("MANIFEST.json", manBytes)
	for _, m := range members {
		if writeErr != nil {
			break
		}
		writeErr = write(m.name, m.data)
	}
	if writeErr == nil {
		writeErr = tw.Close()
	} else {
		_ = tw.Close()
	}
	if writeErr != nil {
		out.Discard(tmp)
		return ExportResult{}, writeErr
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if err := out.Commit(tmp, name, perm, sum); err != nil {
		return ExportResult{}, err
	}

	// Whole-tar sidecar: <tar>.sha256, "  <sha>  <basename>\n" (sha256sum format).
	sidecar := name + ".sha256"
	if err := out.WriteFile(sidecar, []byte(fmt.Sprintf("%s  %s\n", sum, name)), 0o644); err != nil {
		return ExportResult{}, err
	}

	var size int64
	if fi, err := out.Stat(name); err == nil {
		size = fi.Size()
	}
	return ExportResult{
		TarPath: out.Join(name), SidecarPath: out.Join(sidecar), Bytes: size, ScanFilesLeftOut: leftOut,
		MemberCount: len(members) + 1, IncludesKeys: includeKeys, SchemaVersion: schemaVer,
	}, nil
}

// appBackupBaseName builds the dated bundle filename for a manual export.
func appBackupBaseName(t time.Time) string {
	return fmt.Sprintf("%s-appbackup-%s.tar", appBackupName, t.UTC().Format("20060102-150405"))
}

// ExportAppBackup writes a dated app-backup bundle into destDir and returns a summary.
func (a *App) ExportAppBackup(destDir string, includeKeys bool) (ExportResult, error) {
	if strings.TrimSpace(destDir) == "" {
		return ExportResult{}, fmt.Errorf("a destination folder is required")
	}
	// Never into source data: the folder is decided by real location and the
	// bundle written beneath it.
	out, err := a.openOutFolder(destDir)
	if err != nil {
		return ExportResult{}, err
	}
	defer out.Close()
	return a.exportAppBackupInto(out, appBackupBaseName(time.Now()), includeKeys)
}

// ---- restore -------------------------------------------------------------

// App backups use literal state-file names and flat keystores/<filename> entries
// (both the current and legacy exporters). Do not clean or case-fold names: that
// would let validation and extraction select different payloads for one file.
// Portable leaf spelling also excludes Windows separators, streams and trailing
// dot/space aliases. This is a format check, not filesystem/symlink containment.
func validAppBackupMemberName(name string) bool {
	switch name {
	case "MANIFEST.json", "catalog.json", "config.json", "jobs.json", "formats.json":
		return true
	}
	if leaf, ok := strings.CutPrefix(name, "scans/"); ok {
		return validScanFileName(leaf)
	}
	if !strings.HasPrefix(name, "keystores/") {
		return false
	}
	leaf := strings.TrimPrefix(name, "keystores/")
	return leaf != "" && leaf != "." && leaf != ".." &&
		!strings.ContainsAny(leaf, "/\\:") && strings.TrimRight(leaf, " .") == leaf
}

// readTarMembers reads every member of a tar into memory, keyed by name.
func readTarMembers(tarPath string) (map[string][]byte, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("not a readable tar (%s): %w", filepath.Base(tarPath), err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if !validAppBackupMemberName(hdr.Name) {
			return nil, fmt.Errorf("unsupported app-backup member name %q; use canonical state-file names or flat keystores/<filename> entries", hdr.Name)
		}
		if _, exists := out[hdr.Name]; exists {
			return nil, fmt.Errorf("duplicate app-backup member %q; restore requires one payload per name", hdr.Name)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[hdr.Name] = b
	}
	return out, nil
}

// verifyAppBackup validates a bundle WITHOUT touching the data dir: the whole-tar
// sidecar (if present), that it's a well-formed app-backup, the schema gate, and
// every member hash against the manifest. Returns the parsed manifest and members.
func verifyAppBackup(tarPath string) (appBackupManifest, map[string][]byte, error) {
	var man appBackupManifest

	// Whole-tar sidecar check (if the sidecar is present).
	if sc, err := os.ReadFile(tarPath + ".sha256"); err == nil {
		want := strings.TrimSpace(string(sc))
		if i := strings.IndexAny(want, " \t"); i >= 0 {
			want = want[:i]
		}
		got, err := hashFileHex(tarPath)
		if err != nil {
			return man, nil, err
		}
		if !strings.EqualFold(strings.TrimSpace(want), got) {
			return man, nil, fmt.Errorf("the .sha256 sidecar does not match this file — the backup is corrupted or was altered")
		}
	}

	members, err := readTarMembers(tarPath)
	if err != nil {
		return man, nil, err
	}
	manBytes, ok := members["MANIFEST.json"]
	if !ok {
		return man, nil, fmt.Errorf("this is not an Obelisk app backup (no MANIFEST.json)")
	}
	if err := json.Unmarshal(manBytes, &man); err != nil {
		return man, nil, fmt.Errorf("MANIFEST.json is unreadable: %w", err)
	}
	if man.Format != appBackupFormat && man.Format != appBackupFormatLegacy {
		return man, nil, fmt.Errorf("this is not an Obelisk app backup (format %q)", man.Format)
	}
	// Schema gate — same rule as OpenStore: refuse a bundle a newer app created rather
	// than silently dropping fields we don't understand.
	if man.SchemaVersion > currentSchemaVersion {
		return man, nil, fmt.Errorf("this backup was created by a newer version of Obelisk "+
			"(catalog schema v%d; this build understands v%d). Update the app first, then restore.",
			man.SchemaVersion, currentSchemaVersion)
	}
	// Every member hash must match the manifest (tamper / corruption detection).
	seen := make(map[string]bool, len(man.Members))
	for _, m := range man.Members {
		if !validAppBackupMemberName(m.Name) || m.Name == "MANIFEST.json" {
			return man, nil, fmt.Errorf("unsupported app-backup manifest member %q", m.Name)
		}
		if seen[m.Name] {
			return man, nil, fmt.Errorf("duplicate app-backup manifest member %q", m.Name)
		}
		seen[m.Name] = true
		b, ok := members[m.Name]
		if !ok {
			return man, nil, fmt.Errorf("backup is incomplete — member %q is missing", m.Name)
		}
		if got := sha256Hex(b); !strings.EqualFold(got, m.SHA256) {
			return man, nil, fmt.Errorf("integrity check failed on %q — the backup is corrupted or was altered", m.Name)
		}
		if int64(len(b)) != m.Size {
			return man, nil, fmt.Errorf("integrity check failed on %q — size mismatch", m.Name)
		}
	}
	// A correctly hashed member can still be an invalid job board. Refuse it
	// before restore publishes it and leaves the running Store holding stale rows.
	if b, ok := members["jobs.json"]; ok {
		if _, err := decodeJobBoard(b); err != nil {
			return man, nil, fmt.Errorf("backup job board: %w", err)
		}
	}
	return man, members, nil
}

// InspectAppBackup validates a bundle read-only and returns its manifest, powering a
// pre-restore preview.
func (a *App) InspectAppBackup(tarPath string) (appBackupManifest, error) {
	man, _, err := verifyAppBackup(tarPath)
	return man, err
}

// RestoreAppBackup verifies a bundle, backs up the current records, then replaces the
// data-dir files and reopens the store (migrating an older backup forward). All
// verification happens before ANYTHING is written, so a bad backup leaves the current
// data dir untouched.
// preRestoreWriteFile writes a pre-restore copy; a test seam.
var preRestoreWriteFile = func(name string, b []byte, _ os.FileMode) error { return writeSecretFile(name, b) }

func (a *App) RestoreAppBackup(tarPath string) (RestoreResult, error) {
	man, members, err := verifyAppBackup(tarPath)
	if err != nil {
		return RestoreResult{}, err
	}

	// Preserve the current machine's auth token when the backup's is blank (we scrub on
	// export) — never lock a running deployment out of its own API.
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return RestoreResult{}, cfgErr
	}
	curToken := cfg.AuthToken

	// Validate and prepare the config member before any restore mutation. Restore
	// remains a multi-file operation; a later publication failure can leave a
	// partial restore and must reach its caller without a success result.
	var restoredConfig []byte
	var held *HeldSettings
	keystoreNote := ""
	if cfgBytes, ok := members["config.json"]; ok {
		restored, fields, e := decodeConfig(cfgBytes)
		if e != nil {
			return RestoreResult{}, fmt.Errorf("restored configuration is invalid: %w", e)
		}
		if restored.AuthToken == "" {
			restored.AuthToken = curToken
		}
		// The settings that choose what runs are not applied from a backup: the
		// current ones stay, and the backup's are held until the user confirms
		// them (see heldsettings.go).
		held = holdProgramSettings(&restored, cfg, tarPath)
		var paths []string
		for _, m := range man.Members {
			if strings.HasPrefix(m.Name, "keystores/") {
				paths = append(paths, filepath.Join(a.DataDir, "keystores", filepath.Base(m.Name)))
			}
		}
		// The restored keystores land in this data folder. They become the
		// keystore paths only if that is still a valid set (one keystore, or
		// several on separate devices): restoring never puts two keystores on
		// one device. Otherwise the current paths are kept.
		if len(paths) > 0 {
			if err := checkKeystorePaths(paths); err == nil {
				restored.KeystorePaths = paths
			} else {
				restored.KeystorePaths = cfg.KeystorePaths
				keystoreNote = fmt.Sprintf("The backup's %d keystore(s) were restored to %s, all on one device, so your keystore paths were left as they were. Copy each to a different device and set the keystore paths to them.", len(paths), filepath.Join(a.DataDir, "keystores"))
			}
		}
		// The settings a backup brings get the same checks as settings being
		// saved, against the sources of the catalog in use and of the catalog
		// being restored (the one in force afterwards).
		if e := checkConfigPaths(restored, a.Store); e != nil {
			return RestoreResult{}, fmt.Errorf("the backup's settings are refused: %w", e)
		}
		if catBytes, ok := members["catalog.json"]; ok {
			incoming := &Store{}
			if e := decodeCatalogJSON(catBytes, &incoming.c); e != nil {
				return RestoreResult{}, fmt.Errorf("the backup's catalog is damaged: %w", e)
			}
			if e := checkConfigPaths(restored, incoming); e != nil {
				return RestoreResult{}, fmt.Errorf("the backup's settings are refused: %w", e)
			}
		}
		restoredConfig, e = encodeConfig(restored, fields)
		if e != nil {
			return RestoreResult{}, e
		}
	}

	// Each scan file list must be the one a scan record in the backup's catalog
	// names, with its recorded SHA-256. One that is not is skipped (and reported):
	// it never stops the rest of the restore.
	skipScans, err := checkBackupScanFiles(man, members)
	if err != nil {
		return RestoreResult{}, err
	}

	// (4) Back up the current records first, so the restore is itself reversible.
	stamp := time.Now().UTC().Format("20060102-150405")
	preDir := filepath.Join(a.DataDir, "pre-restore-"+stamp)
	if err := os.MkdirAll(filepath.Join(preDir, "keystores"), 0o700); err != nil {
		return RestoreResult{}, fmt.Errorf("prepare pre-restore backup: %w", err)
	}
	// Every current record and keystore is copied first, the keystores included
	// (a restore replaces them, and a key held only in the current one would
	// otherwise be lost). If any copy fails, nothing is restored.
	if err := a.copyPreRestore(cfg, preDir); err != nil {
		return RestoreResult{}, fmt.Errorf("the restore stopped before changing anything: the pre-restore copy in %s failed: %w", preDir, err)
	}

	// (5) Extract members (except the manifest + config, handled below). Keystores land
	// in <data>/keystores/ and the restored config is repointed at them.
	writeFile := func(dest string, b []byte, perm os.FileMode) error {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		tmp, err := writeStage(dest, b, perm)
		if err != nil {
			return err
		}
		if err := atomicRename(tmp, dest); err != nil {
			// atomicRename leaves the existing dest untouched, so the live file is still
			// good. Best-effort cleanup of this operation's staging file: the removal can
			// fail too (its error is deliberately discarded here), and unlinking a path is
			// not secure erasure of the bytes behind it. Either way the original
			// publication error is what gets returned.
			_ = os.Remove(tmp)
			return err
		}
		return nil
	}
	for _, m := range man.Members {
		b := members[m.Name]
		switch {
		case m.Name == "config.json":
			continue // handled last, after keystore paths are known
		case strings.HasPrefix(m.Name, "keystores/"):
			dest := filepath.Join(a.DataDir, "keystores", filepath.Base(m.Name))
			if err := writeFile(dest, b, 0o600); err != nil { // key material: owner only
				return RestoreResult{}, fmt.Errorf("restore %s: %w", m.Name, err)
			}
		case strings.HasPrefix(m.Name, "scans/"):
			if skipScans[m.Name] {
				continue
			}
			if err := writeFile(filepath.Join(a.scansDir(), strings.TrimPrefix(m.Name, "scans/")), b, 0o600); err != nil {
				return RestoreResult{}, fmt.Errorf("restore %s: %w", m.Name, err)
			}
		default: // catalog.json, jobs.json, formats.json
			if err := writeFile(filepath.Join(a.DataDir, m.Name), b, 0o644); err != nil {
				return RestoreResult{}, fmt.Errorf("restore %s: %w", m.Name, err)
			}
		}
	}

	if restoredConfig != nil {
		a.configMu.Lock()
		_, e := a.publishConfig(restoredConfig, false)
		a.configMu.Unlock()
		if e != nil {
			return RestoreResult{}, fmt.Errorf("restore config.json (other members may already be restored): %w", e)
		}
	}
	if held != nil {
		if e := a.saveHeldSettings(held); e != nil {
			return RestoreResult{}, fmt.Errorf("the backup's program settings could not be held for confirmation (other members are already restored): %w", e)
		}
	}

	// (6) Reopen the store from the restored files — this runs the migration registry,
	// bringing an older backup forward — and swap it in.
	ns, err := OpenStore(a.DataDir)
	if err != nil {
		// Other members may already have changed. If the on-disk board is now
		// rejected, prevent this retained Store from publishing its stale history.
		a.Store.checkJobsAfterFailedReopen()
		return RestoreResult{}, fmt.Errorf("reopen catalog after restore (members may already be restored): %w", err)
	}
	a.Store = ns

	// (7) Summary.
	res := RestoreResult{
		ScanFilesSkipped: sortedKeys(skipScans),
		SchemaVersion:    man.SchemaVersion, AppVersion: man.AppVersion, Created: man.Created,
		IncludedKeys: man.IncludesKeys, PreRestoreDir: preDir, HeldSettings: held, KeystoreNote: keystoreNote,
		Archives: len(ns.Collections()), Files: len(ns.AllFiles()),
		Volumes: len(ns.Volumes()), Jobs: len(ns.Jobs()),
	}
	for _, c := range ns.Collections() {
		chunks := ns.Chunks(c.ID)
		res.Packages += len(chunks)
		for _, ch := range chunks {
			res.VerifyEvents += len(ch.VerifyEvents)
		}
	}
	if ro, why := ns.ReadOnly(); ro {
		res.ReadOnly, res.ReadOnlyWhy = true, why
	}
	return res, nil
}

// ---- auto-export ---------------------------------------------------------

// autoExportPeriodName returns the per-period bundle filename for a cadence, so the
// ticker can tell "have I already written this period's backup?" without extra state
// (mirroring the daily-backup once-per-day check).
func autoExportPeriodName(cadence string, t time.Time) (string, bool) {
	t = t.UTC()
	switch strings.ToLower(strings.TrimSpace(cadence)) {
	case "daily":
		return fmt.Sprintf("%s-appbackup-%s.tar", appBackupName, t.Format("20060102")), true
	case "weekly":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%s-appbackup-%04d-W%02d.tar", appBackupName, y, w), true
	default:
		return "", false // "off" or unknown → no auto-export
	}
}

// autoExportCadenceLabel renders a cadence for display ("daily"/"weekly"/"off").
func autoExportCadenceLabel(cadence string) string {
	switch strings.ToLower(strings.TrimSpace(cadence)) {
	case "daily":
		return "every day"
	case "weekly":
		return "every week"
	default:
		return "off"
	}
}

// maybeAutoExport writes one app backup for the current period if a cadence is set
// and this period's file is not already present. Keys are never included in an
// automated export. Best-effort: returns nil when there is nothing to do.
func (a *App) maybeAutoExport(now time.Time) error {
	cfg, cfgErr := a.LoadConfig()
	if cfgErr != nil {
		return cfgErr
	}
	dir := strings.TrimSpace(cfg.AutoExportDir)
	if dir == "" {
		return nil
	}
	name, on := autoExportPeriodName(cfg.AutoExportCadence, now)
	if !on {
		return nil
	}
	// Already written this period: nothing to do, and the folder is not opened,
	// so the run neither waits on another write to that drive nor asks anything.
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return nil
	}
	err := a.autoExportInto(dir, name)
	if err != nil {
		// Into the event log once per period and reason, so it shows in the app.
		if key := name + " " + err.Error(); key != a.autoExportFailed {
			a.autoExportFailed = key
			a.Store.Log("appbackup", fmt.Sprintf("automatic app backup into %s failed: %v", dir, err))
		}
	}
	return err
}

// autoExportInto writes this period's automatic app backup as name in dir.
func (a *App) autoExportInto(dir, name string) error {
	out, err := a.openOutFolder(dir)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.Stat(name); err == nil {
		return nil // written a moment ago
	}
	_, err = a.exportAppBackupInto(out, name, false)
	return err
}

// copyPreRestore copies the current records and every current keystore into
// preDir before a restore changes anything: catalog, config, jobs and formats,
// each file in the data folder's keystores folder, and each configured
// keystore (under keystores/configured-N-<name>). A file that does not exist is
// skipped; any other read or write failure is returned.
func (a *App) copyPreRestore(cfg Config, preDir string) error {
	copyOne := func(src, dst string, perm os.FileMode) error {
		b, err := os.ReadFile(src)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := preRestoreWriteFile(dst, b, perm); err != nil {
			return fmt.Errorf("copy %s: %w", src, err)
		}
		return nil
	}
	for _, name := range []string{"catalog.json", "config.json", "jobs.json", "formats.json"} {
		if err := copyOne(filepath.Join(a.DataDir, name), filepath.Join(preDir, name), 0o600); err != nil {
			return err
		}
	}
	ksDir := filepath.Join(a.DataDir, "keystores")
	entries, err := os.ReadDir(ksDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", ksDir, err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if err := copyOne(filepath.Join(ksDir, e.Name()), filepath.Join(preDir, "keystores", e.Name()), 0o600); err != nil {
			return err
		}
	}
	for i, p := range cfg.KeystorePaths {
		dst := filepath.Join(preDir, "keystores", fmt.Sprintf("configured-%d-%s", i+1, filepath.Base(p)))
		if err := copyOne(p, dst, 0o600); err != nil {
			return err
		}
	}
	// The drive scan file lists, which a restore can add to (scanhistory.go).
	scans, err := os.ReadDir(a.scansDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read %s: %w", a.scansDir(), err)
	}
	for _, e := range scans {
		if !e.Type().IsRegular() || !validScanFileName(e.Name()) {
			continue
		}
		if err := os.MkdirAll(filepath.Join(preDir, "scans"), 0o700); err != nil {
			return fmt.Errorf("prepare %s: %w", filepath.Join(preDir, "scans"), err)
		}
		if err := copyOne(filepath.Join(a.scansDir(), e.Name()), filepath.Join(preDir, "scans", e.Name()), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// checkBackupScanFiles returns the backup's scans/ members to skip: those that
// are not a file list its own catalog's scan records name, with the recorded
// SHA-256. It fails only when the catalog itself cannot be read.
func checkBackupScanFiles(man appBackupManifest, members map[string][]byte) (map[string]bool, error) {
	want := map[string]string{} // file list -> recorded SHA-256
	if catBytes, ok := members["catalog.json"]; ok {
		var c catalog
		if err := decodeCatalogJSON(catBytes, &c); err != nil {
			return nil, fmt.Errorf("the backup's catalog is damaged: %w", err)
		}
		for _, r := range c.Scans {
			want[r.FileList] = r.FileListSHA256
		}
	}
	skip := map[string]bool{}
	for _, m := range man.Members {
		name, ok := strings.CutPrefix(m.Name, "scans/")
		if !ok {
			continue
		}
		if sum, recorded := want[name]; !recorded || sha256Hex(members[m.Name]) != sum {
			skip[m.Name] = true
		}
	}
	return skip, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
