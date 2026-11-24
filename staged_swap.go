package main

// staged_swap.go — putting a finished build in place of the earlier one.
//
// A build is made in its own folder beside the package's staged folder
// (".<name>.build-*"). Only when it has succeeded and verified is it put in
// place: the earlier build is renamed aside (".<name>.previous-*"), the new one
// takes the package's name, the catalog records it, and only then is the
// earlier build removed. Every step is a rename within the staging folder, so a
// crash at any point leaves both builds whole; the next build puts the one the
// catalog records back under the package's name (recoverStagedSwap) before it
// starts. Nothing here removes anything.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// errBuildCrashed is what a build returns when buildSwapCrash stops it.
var errBuildCrashed = errors.New("build stopped part-way (test crash)")

// renameNew renames from to to only when nothing is at to. The check and the
// rename are two steps; both names are in a folder only Obelisk writes, and a
// folder cannot be renamed over another on Windows.
func renameNew(from, to string) error {
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%s is already there", to)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(from, to)
}

func previousName(work string) string {
	return filepath.Join(filepath.Dir(work), "."+filepath.Base(work)+".previous-"+randomHex(4))
}

// swapInBuild puts the finished build in place at work. The earlier build, if
// any, is renamed aside and returned, recorded by identity; it is not removed.
func swapInBuild(build *madeEntry, work string) (*madeEntry, error) {
	var prev *madeEntry
	fi, err := os.Lstat(work)
	switch {
	case err == nil:
		if isLinkMode(fi.Mode()) || !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a folder Obelisk made; move it out of staging and build again", work)
		}
		if prev, err = madeDir(work); err != nil {
			return nil, err
		}
		aside := previousName(work)
		if err := renameNew(work, aside); err != nil {
			return nil, fmt.Errorf("moving the earlier build aside: %w", err)
		}
		prev.Moved(aside)
		if buildSwapCrash != nil && buildSwapCrash("aside") {
			return prev, errBuildCrashed
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	if err := renameNew(build.Path(), work); err != nil {
		if prev != nil {
			if rerr := renameNew(prev.Path(), work); rerr == nil {
				prev.Moved(work)
			}
		}
		return nil, err
	}
	build.Moved(work)
	if buildSwapCrash != nil && buildSwapCrash("in-place") {
		return prev, errBuildCrashed
	}
	return prev, nil
}

// unswapBuild undoes swapInBuild: the new build is moved back aside and the
// earlier one, if any, back under work.
func unswapBuild(build, prev *madeEntry, work string) error {
	aside := filepath.Join(filepath.Dir(work), "."+filepath.Base(work)+".build-"+randomHex(4))
	if err := renameNew(work, aside); err != nil {
		return err
	}
	build.Moved(aside)
	if prev != nil {
		if err := renameNew(prev.Path(), work); err != nil {
			return err
		}
		prev.Moved(work)
	}
	return nil
}

// sidecarsMatch reports whether dir holds every recorded sidecar unchanged, and
// the payload the catalog records (when it records one).
func sidecarsMatch(dir string, recorded map[string]string, payload, payloadHash string) bool {
	if len(recorded) == 0 {
		return false
	}
	if payload != "" && payloadHash != "" {
		if got, err := hashFileHex(filepath.Join(dir, payload)); err != nil || !strings.EqualFold(got, payloadHash) {
			return false
		}
	}
	for name, want := range recorded {
		got, err := hashFileHex(filepath.Join(dir, name))
		if err != nil || got != want {
			return false
		}
	}
	return true
}

// recoverStagedSwap puts the build the catalog records (by its sidecar hashes)
// back under work after a crash part-way through swapInBuild. It only renames;
// whatever is not the recorded build stays beside it and is listed to the user
// as a leftover.
func recoverStagedSwap(work string, recorded map[string]string, payload, payloadHash string) error {
	match := func(dir string) bool { return sidecarsMatch(dir, recorded, payload, payloadHash) }
	dir, name := filepath.Dir(work), filepath.Base(work)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	var aside []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "."+name+".previous-") && e.IsDir() {
			aside = append(aside, filepath.Join(dir, e.Name()))
		}
	}
	if len(aside) == 0 {
		return nil
	}
	// The aside folder holding the recorded build; with nothing recorded, the
	// only one there is the earlier build.
	pick := ""
	for _, p := range aside {
		if match(p) {
			pick = p
		}
	}
	if pick == "" && len(recorded) == 0 && len(aside) == 1 {
		pick = aside[0]
	}
	_, werr := os.Lstat(work)
	switch {
	case errors.Is(werr, fs.ErrNotExist):
		if pick != "" {
			return renameNew(pick, work)
		}
	case werr != nil:
		return werr
	case pick != "" && !match(work):
		// The build under the name is one the catalog never recorded.
		if err := renameNew(work, filepath.Join(dir, "."+name+".build-"+randomHex(4))); err != nil {
			return err
		}
		return renameNew(pick, work)
	}
	return nil
}
