package main

// write_over.go — a package write never replaces what is already at its target.
//
// Before a plain write puts a package in <dest>/<name>, it looks at what is
// there. Three answers, each said plainly, and the write stops:
//
//   - intact:  the same build, whole: its payload and sidecars hash to what the
//     catalog records. Verify it instead; writing it again changes nothing.
//   - partial: a write of this build that stopped part-way (a payload shorter
//     than the build's, a sidecar missing, an unfinished stage), and nothing
//     there contradicts this build.
//   - other:   an earlier build of this package, or files that are not this
//     package's.
//
// Only then is the delete gate's "write-over" offered, which lists exactly the
// files it would replace and runs only after the typed word "Overwrite".

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteOverError is a write that found something at its target.
type WriteOverError struct {
	State   string   `json:"state"` // intact | partial | other
	Dir     string   `json:"dir"`
	Files   []string `json:"files"`
	Message string   `json:"message"`
}

func (e *WriteOverError) Error() string { return e.Message }

// inspectWriteTarget reports what is already in dir, the package's folder on
// the destination, or nil when it holds nothing.
func inspectWriteTarget(c *Chunk, dir string) *WriteOverError {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var files []string
	stage := false
	for _, e := range entries {
		files = append(files, e.Name())
		n := e.Name()
		if strings.HasPrefix(n, ".obelisk-stage") || strings.HasSuffix(n, mirrorTmpSuffix) || kindStage.match(n) {
			stage = true
		}
	}
	sort.Strings(files)
	wo := &WriteOverError{Dir: dir, Files: files}
	payload := payloadPathIn(dir, c)
	var payloadSize int64 = -1
	payloadOK := false
	if payload != "" {
		if fi, err := os.Stat(payload); err == nil && fi.Mode().IsRegular() {
			payloadSize = fi.Size()
			if payloadSize == c.EncBytes && c.EncHash != "" {
				h, err := hashFileHex(payload)
				payloadOK = err == nil && strings.EqualFold(h, c.EncHash)
			}
		}
	}
	sidecarsOK, contradicts, missing := len(c.Sidecars) > 0, false, false
	for name, want := range c.Sidecars {
		h, err := hashFileHex(filepath.Join(dir, name))
		switch {
		case os.IsNotExist(err):
			sidecarsOK, missing = false, true
		case err != nil || !strings.EqualFold(h, want):
			sidecarsOK, contradicts = false, true
		}
	}
	switch {
	case payloadOK && sidecarsOK && !stage:
		wo.State = "intact"
		wo.Message = fmt.Sprintf("%s already holds this build of %s, intact (its payload and sidecars match the catalog). Nothing was written. Verify it instead; to write a new copy over it anyway, use Overwrite, which lists the %d file(s) it would replace", dir, c.Name, len(files))
	case !contradicts && !payloadOK && (payload == "" || payloadSize < c.EncBytes) && (missing || stage || payload == "" || payloadSize < c.EncBytes):
		wo.State = "partial"
		wo.Message = fmt.Sprintf("%s holds a partial write of this build of %s from an earlier run (%d file(s); the payload is missing or short, or a sidecar is missing). Nothing was written. To write it again over what is there, use Overwrite, which lists those files", dir, c.Name, len(files))
	default:
		wo.State = "other"
		wo.Message = fmt.Sprintf("%s already holds %d file(s) that are not this build of %s (an earlier build, or other files). Nothing was written. Write to another folder, or use Overwrite, which lists those files", dir, len(files), c.Name)
	}
	return wo
}

// packageWriteNames are the names a write of c puts in its folder: what its
// staged build holds for the medium, and the copy ID.
func packageWriteNames(c *Chunk) map[string]bool {
	out := map[string]bool{copyIDFile: true}
	entries, _ := os.ReadDir(c.StagedDir)
	for _, e := range entries {
		if !e.IsDir() && !stagedOnlyFile(c, e.Name()) {
			out[e.Name()] = true
		}
	}
	if c.Spanned {
		for _, sg := range c.Segments {
			out[fmt.Sprintf("%s.seg%03d", c.Name, sg.Index)] = true
		}
	}
	return out
}

// spanTargetConflict reports a tape folder that holds another set of this
// package (or files at its names that are not this set's), or nil.
func (a *App) spanTargetConflict(c *Chunk, dir string) *WriteOverError {
	p := priorBuildAt(c, dir)
	if !p.Present || (p.CopyID != "" && p.CopyID == a.Store.spanSetID(c)) {
		return nil
	}
	wo := inspectWriteTarget(c, dir)
	if wo == nil {
		return nil
	}
	wo.State = "other"
	wo.Message = fmt.Sprintf("%s holds another set of %s (or other files at its names). Nothing was written. Use another tape, or use Overwrite, which lists the %d file(s) it would replace", dir, c.Name, len(wo.Files))
	return wo
}

// writeOverResponse answers a write that found something at its target: 409,
// with what is there, so the screen can offer Verify or the gated Overwrite.
func writeOverResponse(w http.ResponseWriter, wo *WriteOverError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(409)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": wo.Message, "write_over": wo})
}
