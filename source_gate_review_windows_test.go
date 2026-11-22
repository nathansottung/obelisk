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
			// Its volume is not recorded, so only its path can say what it blocks.
			setFolderVolumeForTest(app.Store, f.ID, "")
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
