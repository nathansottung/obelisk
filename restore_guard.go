package main

// restore_guard.go — restore output through the destination guard.
//
// A restore writes only into an output folder authorized by the destination
// guard. Packages are extracted into a private work folder inside it, and each
// file is then published under its real name with the guard's no-replace step:
// a name already taken in the output folder is never replaced, and if any is
// taken the whole restore is refused before anything is published.

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// restoreWork is a restore's private work folder inside the authorized output.
type restoreWork struct {
	rel string // relative to the output folder
	abs string // its real location, for tools that take a path
}

// newRestoreWork creates a uniquely named, new work folder in dest.
func newRestoreWork(dest *DestRoot) (restoreWork, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return restoreWork{}, err
	}
	rel := ".obelisk-restore-" + hex.EncodeToString(b)
	if _, err := dest.Stat(rel); err == nil {
		return restoreWork{}, fmt.Errorf("restore work folder %s already exists", rel)
	}
	if err := dest.MkdirAll(rel + "/x"); err != nil {
		return restoreWork{}, err
	}
	return restoreWork{rel: rel, abs: filepath.Join(dest.Canonical(), rel)}, nil
}

// extracted is where the package's files are unpacked.
func (w restoreWork) extracted() string { return filepath.Join(w.abs, "x") }

// removeWork removes a restore work folder, through the output folder's handle.
func (d *DestRoot) removeWork(rel string) error {
	v, err := destRel(rel)
	if err != nil {
		return err
	}
	return d.root.RemoveAll(v)
}

// publishRestored moves what was extracted into w to its real names in dest.
// Every name is checked first: if any file would replace something already in
// the output folder, or a folder would replace a file, nothing is published and
// the restore is refused, naming what is in the way. Files are then published
// with copy-then-verify and the no-replace step, so a name taken in the
// meantime is refused too. Links and other special entries are not restored
// and are reported. Returns the number of files restored.
func publishRestored(dest *DestRoot, w restoreWork) (int, []string, error) {
	root := w.extracted()
	var files, dirs, warnings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case d.IsDir():
			dirs = append(dirs, rel)
		case d.Type().IsRegular():
			files = append(files, rel)
		default:
			warnings = append(warnings, fmt.Sprintf("%s was not restored: it is a link or special file, not a regular file", rel))
		}
		return nil
	})
	if err != nil {
		return 0, nil, fmt.Errorf("reading the extracted files: %w", err)
	}
	var taken []string
	for _, rel := range dirs {
		if fi, err := dest.Stat(rel); err == nil && !fi.IsDir() {
			taken = append(taken, rel)
		}
	}
	for _, rel := range files {
		if _, err := dest.Stat(rel); err == nil {
			taken = append(taken, rel)
		}
	}
	if len(taken) > 0 {
		shown := taken
		if len(shown) > 5 {
			shown = shown[:5]
		}
		more := ""
		if len(taken) > len(shown) {
			more = fmt.Sprintf(" and %d more", len(taken)-len(shown))
		}
		return 0, nil, fmt.Errorf("refusing to restore into %s: %d name(s) it would write are already taken there (%s%s). Obelisk never replaces a file when restoring; restore into an empty folder, or move those aside",
			dest.Name(), len(taken), strings.Join(shown, ", "), more)
	}
	for _, rel := range dirs {
		if err := dest.MkdirAll(rel); err != nil {
			return 0, warnings, err
		}
	}
	n := 0
	for _, rel := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if _, _, err := copyVerifyNewToRoot(dest, p, rel, "", nil, nil); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return n, warnings, fmt.Errorf("restore stopped: %s appeared in the output folder while restoring and was not replaced (%d file(s) were restored before it)", rel, n)
			}
			return n, warnings, fmt.Errorf("restore %s: %w (%d file(s) were restored before it)", rel, err, n)
		}
		_ = os.Remove(p) // free the space as we go; the work folder is removed at the end
		n++
	}
	return n, warnings, nil
}
