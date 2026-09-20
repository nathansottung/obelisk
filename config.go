package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

// ConfigPublicationError distinguishes publication from later staging cleanup or directory sync.
// Published does not assert crash durability or multi-process exclusion.
type ConfigPublicationError struct {
	Published bool
	Err       error
}

func (e *ConfigPublicationError) Error() string {
	if e.Published {
		return "configuration was published, but a post-publication step failed: " + e.Err.Error()
	}
	return "configuration was not published: " + e.Err.Error()
}
func (e *ConfigPublicationError) Unwrap() error { return e.Err }

type configTempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}
type configHooks struct {
	readFile   func(string) ([]byte, error)
	observe    func(string)
	createTemp func(string, string) (configTempFile, error)
	rename     func(string, string) error
	link       func(string, string) error // initialization only; nil uses os.Link
	removeTemp func(string) error         // checked post-link cleanup; nil uses os.Remove
	syncDir    func(string) error
}

func decodeConfig(b []byte) (Config, map[string]json.RawMessage, error) {
	if !utf8.Valid(b) {
		return Config{}, nil, fmt.Errorf("configuration is not valid UTF-8")
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return Config{}, nil, fmt.Errorf("configuration is empty; refusing defaults")
	}
	if err := uniqueJSON(b); err != nil {
		return Config{}, nil, fmt.Errorf("configuration contains malformed or duplicate-field JSON")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil || fields == nil {
		return Config{}, nil, fmt.Errorf("configuration must be a JSON object")
	}
	// Reject competing aliases of known fields. A single historical case alias
	// remains accepted as encoding/json accepted it before this repair.
	seen := map[string]bool{}
	t := reflect.TypeOf(Config{})
	known := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		known[strings.ToLower(name)] = name
	}
	for name := range fields {
		if canonical, ok := known[strings.ToLower(name)]; ok {
			if seen[canonical] {
				return Config{}, nil, fmt.Errorf("configuration contains competing field aliases")
			}
			seen[canonical] = true
			for i := 0; i < t.NumField(); i++ {
				if strings.Split(t.Field(i).Tag.Get("json"), ",")[0] != canonical {
					continue
				}
				kind := t.Field(i).Type.Kind()
				value := bytes.TrimSpace(fields[name])
				if bytes.Equal(value, []byte("null")) && kind != reflect.Map && kind != reflect.Slice {
					return Config{}, nil, fmt.Errorf("configuration contains a null scalar field")
				}
				// Null maps/slices remain compatible; their string entries must be strings.
				if kind == reflect.Map {
					var entries map[string]json.RawMessage
					if json.Unmarshal(value, &entries) == nil {
						for _, v := range entries {
							if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
								return Config{}, nil, fmt.Errorf("configuration contains a null map value")
							}
						}
					}
				}
				if kind == reflect.Slice {
					var entries []json.RawMessage
					if json.Unmarshal(value, &entries) == nil {
						for _, v := range entries {
							if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
								return Config{}, nil, fmt.Errorf("configuration contains a null list entry")
							}
						}
					}
				}
			}
		}
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("configuration has an invalid field type")
	}
	if cfg.Tools == nil {
		cfg.Tools = map[string]string{}
	}
	return cfg, fields, nil
}

func (a *App) readConfig() (Config, map[string]json.RawMessage, error) {
	read := a.configIO.readFile
	if read == nil {
		read = os.ReadFile
	}
	b, err := read(a.configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil, fmt.Errorf("expected configuration is missing; reconnect storage or explicitly initialize with -init-config: %w", err)
		}
		return Config{}, nil, fmt.Errorf("cannot read configuration: %w", err)
	}
	return decodeConfig(b)
}

func (a *App) LoadConfig() (Config, error) {
	cfg, _, err := a.readConfig()
	return cfg, err
}

// InitializeConfig is explicit enrollment, never called by an ordinary update.
// Existing valid settings are returned unchanged; damaged settings are refused.
// A destination arriving after the absence read is refused at publication.
func (a *App) InitializeConfig() (Config, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg, _, err := a.readConfig()
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg = defaultConfig()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Config{}, fmt.Errorf("cannot serialize initial configuration")
	}
	published, err := a.publishConfig(b, true)
	if published {
		setHashAccel(cfg.HashAccel)
	}
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (a *App) SaveConfig(in map[string]any) (Config, error) {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	cfg, fields, err := a.readConfig()
	if err != nil {
		return Config{}, err
	}
	b, err := json.Marshal(in)
	if err != nil {
		return Config{}, fmt.Errorf("cannot serialize configuration update")
	}
	if _, _, err := decodeConfig(b); err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("configuration update has an invalid field type")
	}
	if cfg.Tools == nil {
		cfg.Tools = map[string]string{}
	}
	if a.Store == nil {
		return Config{}, fmt.Errorf("configuration update requires an open catalog")
	}
	if err := a.Store.AssertOutsideSources(cfg.StagingDir); err != nil {
		return Config{}, err
	}
	for _, p := range cfg.KeystorePaths {
		if err := a.Store.AssertOutsideSources(p); err != nil {
			return Config{}, err
		}
	}
	if cfg.AutoExportDir != "" {
		if err := a.Store.AssertOutsideSources(cfg.AutoExportDir); err != nil {
			return Config{}, err
		}
	}
	// Preserve unknown extension fields, while canonicalizing known field aliases.
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(b, &patch); err != nil {
		return Config{}, fmt.Errorf("invalid configuration update")
	}
	for k, v := range patch {
		fields[k] = v
	}
	out, err := encodeConfig(cfg, fields)
	if err != nil {
		return Config{}, err
	}
	published, err := a.publishConfig(out, false)
	if published {
		setHashAccel(cfg.HashAccel)
	}
	if err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// encodeConfig retains unknown extension fields, including when an optional known
// field is cleared and would otherwise be omitted by encoding/json.
func encodeConfig(cfg Config, fields map[string]json.RawMessage) ([]byte, error) {
	knownBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot serialize configuration")
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(knownBytes, &known); err != nil {
		return nil, fmt.Errorf("cannot encode configuration")
	}
	t := reflect.TypeOf(Config{})
	for k := range fields {
		for i := 0; i < t.NumField(); i++ {
			name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
			if strings.EqualFold(k, name) {
				delete(fields, k)
				break
			}
		}
	}
	// Include zero optional fields too: an explicit blank must survive defaults on reopen.
	value := reflect.ValueOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if _, ok := known[name]; !ok {
			b, e := json.Marshal(value.Field(i).Interface())
			if e != nil {
				return nil, fmt.Errorf("cannot encode configuration")
			}
			known[name] = b
		}
	}
	for k, v := range known {
		fields[k] = v
	}
	out, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("cannot serialize configuration")
	}
	return out, nil
}

func (a *App) publishConfig(b []byte, initialize bool) (published bool, err error) {
	// This is the actual mutation boundary, including initialization's MkdirAll.
	if a.configIO.observe != nil {
		a.configIO.observe(a.configPath())
	}
	fail := func(e error) (bool, error) { return false, &ConfigPublicationError{Err: e} }
	dir := filepath.Dir(a.configPath())
	if initialize {
		if e := os.MkdirAll(dir, 0700); e != nil {
			return fail(e)
		}
	}
	create := a.configIO.createTemp
	if create == nil {
		create = func(d, p string) (configTempFile, error) { return os.CreateTemp(d, p) }
	}
	f, e := create(dir, ".config-*.tmp")
	if e != nil {
		return fail(e)
	}
	defer os.Remove(f.Name()) // only our staging file; never remove the destination
	if n, e := f.Write(b); e != nil {
		_ = f.Close()
		return fail(e)
	} else if n != len(b) {
		_ = f.Close()
		return fail(io.ErrShortWrite)
	}
	if e := f.Sync(); e != nil {
		_ = f.Close()
		return fail(e)
	}
	if e := f.Close(); e != nil {
		return fail(e)
	}
	if initialize {
		// Link gives the fully written, synced, closed file its final name ONLY if
		// that directory entry is absent. Unlike Rename it never replaces an entry.
		// This is the publication point. Unsupported hard links fail closed; there
		// is no replacement fallback or placeholder written at the destination.
		link := a.configIO.link
		if link == nil {
			link = os.Link
		}
		if e := link(f.Name(), a.configPath()); e != nil {
			return fail(fmt.Errorf("initialization refused: destination must remain absent and the filesystem must support hard links; inspect or restore the existing configuration: %w", e))
		}
		remove := a.configIO.removeTemp
		if remove == nil {
			remove = os.Remove
		}
		if e := remove(f.Name()); e != nil {
			return true, &ConfigPublicationError{Published: true, Err: fmt.Errorf("remove initialization staging: %w", e)}
		}
	} else {
		rename := a.configIO.rename
		if rename == nil {
			rename = os.Rename
		}
		if e := rename(f.Name(), a.configPath()); e != nil {
			return fail(e)
		}
	}
	sync := a.configIO.syncDir
	if sync == nil {
		sync = syncDir
	}
	if e := sync(dir); e != nil {
		return true, &ConfigPublicationError{Published: true, Err: fmt.Errorf("sync configuration directory: %w", e)}
	}
	return true, nil
}

// Called before catalog opening, authentication/binding and background work.
// Initialization requires the caller's explicit first-use intent.
func loadStartupConfig(dataDir string, initialize bool) (Config, error) {
	a := &App{DataDir: dataDir}
	if initialize {
		return a.InitializeConfig()
	}
	return a.LoadConfig()
}
