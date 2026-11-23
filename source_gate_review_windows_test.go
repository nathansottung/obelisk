//go:build windows && !guionly

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// A source whose drive is not ready is offline: it blocks only what could be it.
func TestSourceNotReady_BlocksOnlyItsOwnPath(t *testing.T) {
	for name, code := range map[string]windows.Errno{
		"an empty card reader": windows.ERROR_NOT_READY,
		"a network path":       windows.ERROR_BAD_NETPATH,
		"a network share":      windows.ERROR_BAD_NET_NAME,
		"no network":           windows.Errno(1222),
	} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			card := filepath.Join(t.TempDir(), "card")
			if err := os.Mkdir(card, 0o755); err != nil {
				t.Fatal(err)
			}
			f := app.Store.AddFolder(app.Store.AddCollection("Live").ID, card)
			setFolderVolumeForTest(app.Store, f.ID, "the-away-volume")
			prev := openSourceRoot
			openSourceRoot = func(p string) (*os.File, error) {
				if strings.EqualFold(p, card) {
					return nil, &fs.PathError{Op: "open", Path: p, Err: code}
				}
				return prev(p)
			}
			t.Cleanup(func() { openSourceRoot = prev })
			release, err := app.Store.reserveOutputPath(t.TempDir())
			if err != nil {
				t.Fatalf("an unrelated output was stopped: %v", err)
			}
			release()
			if r, err := app.Store.reserveOutputPath(filepath.Join(card, "inside")); err == nil {
				r()
				t.Fatal("an output at the unavailable source's own path was allowed")
			}
		})
	}
}

// A folder holding an unavailable source's mount point still holds it, even though
// that folder is on a different volume, and whatever name the source was added by.
func TestSourceNotReady_StillProtectsWhatHoldsIt(t *testing.T) {
	for _, viaLink := range []bool{false, true} {
		name := "by its own path"
		if viaLink {
			name = "by another name"
		}
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			box := filepath.Join(t.TempDir(), "box")
			src := filepath.Join(box, "Card")
			if err := os.MkdirAll(src, 0o755); err != nil {
				t.Fatal(err)
			}
			if viaLink {
				alias := filepath.Join(t.TempDir(), "alias")
				mustDirLink(t, alias, box)
				src = filepath.Join(alias, "Card")
			}
			f := app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
			setFolderVolumeForTest(app.Store, f.ID, "the-card-volume")
			prev := openSourceRoot
			openSourceRoot = func(p string) (*os.File, error) {
				if strings.EqualFold(p, src) {
					return nil, &fs.PathError{Op: "open", Path: p, Err: windows.ERROR_NOT_READY}
				}
				return prev(p)
			}
			t.Cleanup(func() { openSourceRoot = prev })
			if r, err := app.Store.reserveOutputPath(box); err == nil {
				r()
				t.Fatal("a folder holding an unavailable source was allowed")
			}
			r, err := app.Store.reserveOutputPath(t.TempDir())
			if err != nil {
				t.Fatalf("an unrelated output was stopped: %v", err)
			}
			r()
		})
	}
}

// With no recorded volume, an unreachable share could be the same place under
// another name, so it is not treated as safely away.
func TestSourceOnUnreachableShare_NoVolumeStillRefuses(t *testing.T) {
	// ERROR_BAD_NETPATH is not listed: Go reports it as "does not exist", which the
	// snapshot has always handled as a source that is away.
	for name, code := range map[string]windows.Errno{
		"a network share": windows.ERROR_BAD_NET_NAME,
		"no network":      windows.Errno(1222),
	} {
		t.Run(name, func(t *testing.T) {
			app := dockApp(t)
			src := filepath.Join(t.TempDir(), "share")
			if err := os.Mkdir(src, 0o755); err != nil {
				t.Fatal(err)
			}
			f := app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
			setFolderVolumeForTest(app.Store, f.ID, "")
			prev := openSourceRoot
			openSourceRoot = func(p string) (*os.File, error) {
				if strings.EqualFold(p, src) {
					return nil, &fs.PathError{Op: "open", Path: p, Err: code}
				}
				return prev(p)
			}
			t.Cleanup(func() { openSourceRoot = prev })
			if r, err := app.Store.reserveOutputPath(t.TempDir()); err == nil {
				r()
				t.Fatal("an output was allowed beside a share that could be anywhere")
			}
		})
	}
}

// More extents than the first buffer holds are still read in full.
func TestDisksFromExtents_ManyExtents(t *testing.T) {
	disks := make([]uint32, 40)
	for i := range disks {
		disks[i] = uint32(i % 3)
	}
	got, err := disksFromExtents(volumeExtents(disks...))
	if err != nil || len(got) != 3 {
		t.Fatalf("got %v, %v", got, err)
	}
	if n := extentBufferFor(volumeExtents(disks...)[:8]); n < 8+24*40 {
		t.Fatalf("a buffer of %d bytes cannot hold 40 extents", n)
	}
}

// Access denied is not "away": deciding without that source would be a guess.
func TestSourceDenied_StillRefuses(t *testing.T) {
	app := dockApp(t)
	src := t.TempDir()
	app.Store.AddFolder(app.Store.AddCollection("Live").ID, src)
	prev := openSourceRoot
	openSourceRoot = func(p string) (*os.File, error) {
		if strings.EqualFold(p, src) {
			return nil, &fs.PathError{Op: "open", Path: p, Err: windows.ERROR_ACCESS_DENIED}
		}
		return prev(p)
	}
	t.Cleanup(func() { openSourceRoot = prev })
	r, err := app.Store.reserveOutputPath(t.TempDir())
	if err == nil {
		r()
		t.Fatal("an output was allowed while a source could not be read")
	}
	if !strings.Contains(err.Error(), "cannot open registered source") {
		t.Fatalf("refused for another reason: %v", err)
	}
}

// Losing one disk must not lose two keystores, even through a volume across disks.
func TestKeystores_RefusedWhenASpanningVolumeSharesADisk(t *testing.T) {
	a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
	prev := volumeDiskSet
	volumeDiskSet = func(p string) ([]uint32, error) {
		switch {
		case strings.HasPrefix(p, a):
			return []uint32{1, 2}, nil
		case strings.HasPrefix(p, b):
			return []uint32{1}, nil
		}
		return []uint32{3}, nil
	}
	t.Cleanup(func() { volumeDiskSet = prev })
	prevDev := keystoreDeviceOf
	keystoreDeviceOf = realKeystoreDevice
	t.Cleanup(func() { keystoreDeviceOf = prevDev })
	pa, pb, pc := filepath.Join(a, "k.json"), filepath.Join(b, "k.json"), filepath.Join(c, "k.json")
	if err := checkKeystorePaths([]string{pa, pb}); err == nil || !strings.Contains(err.Error(), "same device") {
		t.Errorf("a spanning volume sharing a disk: %v", err)
	}
	if err := checkKeystorePaths([]string{pb, pa}); err == nil || !strings.Contains(err.Error(), "same device") {
		t.Errorf("the same pair in the other order: %v", err)
	}
	if err := checkKeystorePaths([]string{pa, pc}); err != nil {
		t.Errorf("a spanning volume and a separate disk: %v", err)
	}
}
