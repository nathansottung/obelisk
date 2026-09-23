package main

// Development-only producer for explicitly authorized expendable directories.
// Never calls App, OpenStore, ScanFolder, registration or persistence methods.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type inventoryLimits struct {
	files, entries, depth, path int
	fileBytes, totalBytes       int64
}

var disposableInventoryLimits = inventoryLimits{64, 128, 8, 512, 8 << 20, 32 << 20}

type inventoryResult struct {
	Version   string `json:"version"` // producing binary's appVersion
	Published bool   `json:"published"`
	Output    string `json:"output"`
	Files     int    `json:"processedFiles"`
	Bytes     int64  `json:"readBytes"`
	Staging   string `json:"remainingStaging,omitempty"`
}

// Per-invocation seams exercise deterministic failures, never exposed by CLI/UI.
type inventoryIO struct {
	before        func(stage, path string) error
	link          func(string, string) error
	remove        func(string) error
	write         func(*os.File, []byte) (int, error)
	ignoreDSStore bool
}

func inventoryCheck(ctx context.Context, ops inventoryIO, stage, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ops.before != nil {
		if err := ops.before(stage, path); err != nil {
			return fmt.Errorf("%s %q: %w", stage, path, err)
		}
	}
	return ctx.Err()
}

// Reject links in every existing component, including source/output ancestors.
// This is a stable-tree policy, not a race-resistant handle confinement scheme.
func inventoryPath(name string) (string, os.FileInfo, error) {
	if !utf8.ValidString(name) || !filepath.IsAbs(name) || len(name) > 4096 {
		return "", nil, errors.New("explicit bounded absolute path required")
	}
	name = filepath.Clean(name)
	if err := inventoryPlatformPath(name); err != nil {
		return "", nil, err
	}
	var chain []string
	for p := name; ; p = filepath.Dir(p) {
		chain = append(chain, p)
		if filepath.Dir(p) == p {
			break
		}
	}
	var result os.FileInfo
	for i := len(chain) - 1; i >= 0; i-- {
		info, err := os.Lstat(chain[i])
		if err != nil {
			return "", nil, err
		}
		if inventoryIsLink(info) || (!info.IsDir() && !info.Mode().IsRegular()) {
			return "", nil, fmt.Errorf("unsupported link/reparse/special path %q", chain[i])
		}
		if i > 0 && !info.IsDir() {
			return "", nil, fmt.Errorf("non-directory ancestor %q", chain[i])
		}
		result = info
	}
	return name, result, nil
}

func inventoryWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// Also compare actual ancestor identities: lexical Rel alone cannot account for
// aliases such as Windows short names. All these ancestors were checked for links.
func inventoryObjectWithin(parent os.FileInfo, child string) (bool, error) {
	for p := child; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return false, err
		}
		if inventoryIsLink(info) {
			return false, fmt.Errorf("link/reparse ancestor changed: %q", p)
		}
		if os.SameFile(parent, info) {
			return true, nil
		}
		if filepath.Dir(p) == p {
			return false, nil
		}
	}
}

func inventorySame(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && !inventoryIsLink(b)
}

type inventoryObservation struct {
	path  string
	info  os.FileInfo
	names []string
}

func inventoryReadDir(name string, max int) (entries []os.DirEntry, err error) {
	if err = inventoryDirectoryEncoding(name, max); err != nil {
		return nil, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	entries, err = f.ReadDir(max + 1)
	if err == io.EOF {
		err = nil
	}
	if len(entries) > max {
		return nil, errors.New("entry limit exceeded")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, err
}

type inventoryReader struct {
	ctx context.Context
	r   io.Reader
	n   int64
}

func (r *inventoryReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

func produceGUIInventory(ctx context.Context, source, output string, limits inventoryLimits, ops inventoryIO, progress io.Writer) (result inventoryResult, err error) {
	result.Output = output
	if err = inventoryCheck(ctx, ops, "start", source); err != nil {
		return
	}
	if !utf8.ValidString(output) || !filepath.IsAbs(output) || len(output) > 4096 {
		err = errors.New("explicit bounded absolute output required")
		return
	}
	output = filepath.Clean(output)
	result.Output = output
	if err = inventoryPlatformPath(output); err != nil {
		return
	}
	source, rootInfo, e := inventoryPath(source)
	if e != nil {
		err = e
		return
	}
	parent, parentInfo, e := inventoryPath(filepath.Dir(output))
	if e != nil {
		err = e
		return
	}
	if !rootInfo.IsDir() || !parentInfo.IsDir() || filepath.Dir(source) == source {
		err = errors.New("source and output parent must be ordinary non-root directories")
		return
	}
	outputInSource, e := inventoryObjectWithin(rootInfo, parent)
	if e != nil {
		err = e
		return
	}
	sourceInOutput, e := inventoryObjectWithin(parentInfo, source)
	if e != nil {
		err = e
		return
	}
	if inventoryWithin(source, parent) || inventoryWithin(parent, source) || outputInSource || sourceInOutput {
		err = errors.New("source/output-parent overlap refused")
		return
	}
	if _, e = os.Lstat(output); !errors.Is(e, os.ErrNotExist) {
		err = fmt.Errorf("output must be absent: %q (%v)", output, e)
		return
	}
	observedAt := time.Now().UTC()
	c := catalog{SchemaVersion: currentSchemaVersion, NextID: map[string]int{"collection": 1, "folder": 1}}
	c.Collections = []*Collection{{ID: 1, Name: "Disposable inventory: " + filepath.Base(source), CreatedAt: observedAt, Kind: ArchiveSourced}}
	c.Folders = []*Folder{{ID: 1, CollectionID: 1, Path: source}}
	var observations []inventoryObservation
	entriesSeen := 0
	excluded := 0
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if depth > limits.depth {
			return fmt.Errorf("depth limit: %q", dir)
		}
		if e := inventoryCheck(ctx, ops, "enumerate", dir); e != nil {
			return e
		}
		_, info, e := inventoryPath(dir)
		if e != nil {
			return e
		}
		if !info.IsDir() {
			return fmt.Errorf("directory changed: %q", dir)
		}
		children, e := inventoryReadDir(dir, limits.entries-entriesSeen)
		if e != nil {
			return fmt.Errorf("enumerate %q: %w", dir, e)
		}
		entriesSeen += len(children)
		obs := inventoryObservation{path: dir, info: info}
		for _, child := range children {
			if !utf8.ValidString(child.Name()) {
				return errors.New("unsupported non-Unicode filename; no snapshot published")
			}
			obs.names = append(obs.names, child.Name())
		}
		observations = append(observations, obs)
		for _, child := range children {
			p := filepath.Join(dir, child.Name())
			rel, e := filepath.Rel(source, p)
			if e != nil {
				return e
			}
			if len(rel) > limits.path {
				return fmt.Errorf("relative path limit: %q", rel)
			}
			if e = inventoryCheck(ctx, ops, "stat", p); e != nil {
				return e
			}
			_, info, e := inventoryPath(p)
			if e != nil {
				return e
			}
			if info.IsDir() {
				if e = walk(p, depth+1); e != nil {
					return e
				}
				continue
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unsupported entry: %q", p)
			}
			if result.Files+excluded >= limits.files {
				return fmt.Errorf("regular-file count limit: %q", p)
			}
			if ops.ignoreDSStore && child.Name() == ".DS_Store" {
				excluded++
				observations = append(observations, inventoryObservation{path: p, info: info})
				continue // Classified regular, bounded and rechecked; contents unopened.
			}
			if info.Size() < 0 || info.Size() > limits.fileBytes || info.Size() > limits.totalBytes-result.Bytes {
				return fmt.Errorf("file/count/byte limit: %q", p)
			}
			if e = inventoryCheck(ctx, ops, "read", p); e != nil {
				return e
			}
			f, e := os.Open(p)
			if e != nil {
				return e
			}
			opened, e := f.Stat()
			if e != nil {
				_ = f.Close()
				return e
			}
			if !inventorySame(info, opened) {
				_ = f.Close()
				return fmt.Errorf("file changed before read: %q", p)
			}
			bounded := &inventoryReader{ctx: ctx, r: io.LimitReader(f, info.Size()+1)}
			sha, b3, readErr := hashReaderBoth(bounded)
			after, statErr := f.Stat()
			closeErr := f.Close()
			result.Bytes += bounded.n
			if e = errors.Join(readErr, statErr, closeErr); e != nil {
				return e
			}
			if bounded.n != info.Size() || !inventorySame(info, after) {
				return fmt.Errorf("file changed during read: %q", p)
			}
			if e = inventoryCheck(ctx, ops, "hashed", p); e != nil {
				return e
			}
			result.Files++
			c.Files = append(c.Files, &File{ID: result.Files, CollectionID: 1, FolderID: 1, RelPath: filepath.ToSlash(rel), SizeBytes: info.Size(), HashAlg: "sha256", Hash: sha, Blake3: b3, ModTime: info.ModTime().UTC(), FirstSeen: observedAt})
			observations = append(observations, inventoryObservation{path: p, info: info})
			if _, e = fmt.Fprintf(progress, "processed files=%d read bytes=%d (source content read; no complete snapshot yet)\n", result.Files, result.Bytes); e != nil {
				return e
			}
		}
		return nil
	}
	if err = walk(source, 0); err != nil {
		return
	}
	c.NextID["file"] = result.Files
	policy := "include-all"
	if ops.ignoreDSStore {
		policy = "ignore-exact-ds-store"
	}
	scope := guiInventoryScope{1, policy, entriesSeen, result.Files + excluded, result.Files, excluded, result.Bytes, true}
	detail, e := json.Marshal(scope)
	if e != nil {
		err = e
		return
	}
	c.Audit = []Audit{{At: observedAt, Action: guiInventoryAuditAction, Detail: string(detail)}}
	if err = inventoryCheck(ctx, ops, "validate", source); err != nil {
		return
	}
	raw, e := json.Marshal(&c)
	if e != nil {
		err = e
		return
	}
	// Use the accepted native reader's entire current shape/presence/subset policy.
	if _, err = loadGUICatalog("in-memory inventory", func(string) ([]byte, error) { return raw, nil }); err != nil {
		return
	}
	for _, obs := range observations {
		if err = inventoryCheck(ctx, ops, "recheck", obs.path); err != nil {
			return
		}
		_, info, e := inventoryPath(obs.path)
		if e != nil {
			err = e
			return
		}
		if !inventorySame(obs.info, info) {
			err = fmt.Errorf("observed source changed: %q", obs.path)
			return
		}
		if info.IsDir() {
			children, e := inventoryReadDir(obs.path, limits.entries)
			if e != nil {
				err = e
				return
			}
			names := []string{}
			for _, child := range children {
				names = append(names, child.Name())
			}
			if strings.Join(names, "\x00") != strings.Join(obs.names, "\x00") {
				err = fmt.Errorf("directory entries changed: %q", obs.path)
				return
			}
		}
	}
	_, info, e := inventoryPath(parent)
	if e != nil {
		err = e
		return
	}
	if !os.SameFile(parentInfo, info) {
		err = errors.New("output parent changed")
		return
	}
	if err = inventoryCheck(ctx, ops, "stage", output); err != nil {
		return
	}
	stage, e := os.CreateTemp(parent, ".inventory-*.tmp")
	if e != nil {
		err = e
		return
	}
	stageName := stage.Name()
	closed := false
	remove := ops.remove
	if remove == nil {
		remove = os.Remove
	}
	defer func() {
		if !closed {
			err = errors.Join(err, stage.Close())
		}
		if e := remove(stageName); e != nil {
			result.Staging = stageName
			err = errors.Join(err, fmt.Errorf("staging cleanup %q (published=%t): %w", stageName, result.Published, e))
		}
	}()
	write := ops.write
	if write == nil {
		write = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
	}
	n, e := write(stage, raw)
	if e != nil {
		err = e
		return
	}
	if n != len(raw) {
		err = io.ErrShortWrite
		return
	}
	if err = stage.Sync(); err != nil {
		return
	}
	err = stage.Close()
	closed = true
	if err != nil {
		return
	}
	if err = inventoryCheck(ctx, ops, "publish", output); err != nil {
		return
	}
	link := ops.link
	if link == nil {
		link = os.Link
	}
	if err = link(stageName, output); err != nil {
		return
	}
	// No-replace link is the publication point. Later failure never removes output.
	result.Published = true
	return
}

func runGUIInventory(args []string, output, diagnostics io.Writer) error {
	ignore := false
	if len(args) > 0 && (args[0] == "--ignore-ds-store" || args[0] == "--ignore-ds-store=true" || args[0] == "--ignore-ds-store=false") {
		ignore = args[0] != "--ignore-ds-store=false"
		args = args[1:]
	}
	if len(args) != 2 {
		return errors.New("usage: --gui-disposable-inventory [--ignore-ds-store[=true|false]] <absolute-generated-source> <absolute-new-catalog>; flag precedes paths, default false; exclude only regular .DS_Store files, leave sources unchanged")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := produceGUIInventory(ctx, args[0], args[1], disposableInventoryLimits, inventoryIO{ignoreDSStore: ignore}, diagnostics)
	result.Version = appVersion
	encodeErr := json.NewEncoder(output).Encode(result)
	if encodeErr != nil {
		return errors.Join(err, fmt.Errorf("status output failed (published=%t, catalog=%q): %w", result.Published, result.Output, encodeErr))
	}
	return err
}
