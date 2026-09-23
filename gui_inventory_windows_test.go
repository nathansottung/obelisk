//go:build windows

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGUIInventoryNativeNameEncoding(t *testing.T) {
	for _, name := range [][]uint16{{'x', 0xd800, 0}, {0xdc00, 0}, {0xd800, 'x', 0}} {
		if inventoryValidUTF16(name) {
			t.Fatal("malformed native name accepted", name)
		}
	}
	for _, name := range [][]uint16{{0xfffd, 0}, {0xd83d, 0xde80, 0}, {'\\', 'u', 'D', '8', '0', '0', 0}} {
		if !inventoryValidUTF16(name) {
			t.Fatal("valid native name rejected", name)
		}
	}
}

func TestGUIInventoryWindowsPathPolicy(t *testing.T) {
	// Lexical refusals only: no device is opened and no real input is selected.
	for _, name := range []string{`\\server\share\file`, `\\?\C:\file`, `C:\generated\file:stream`, `C:\generated\file.`, `C:\generated\NUL`, `C:\generated\CON.txt`, `C:\generated\LPT1`, `C:\generated\COM9.log`} {
		if e := inventoryPlatformPath(name); e == nil {
			t.Fatalf("unsafe path accepted: %s", name)
		}
	}
}

func TestGUIInventoryJunctions(t *testing.T) {
	for _, kind := range []string{"entry", "source-root", "output-parent"} {
		t.Run(kind, func(t *testing.T) {
			src, dst := inventoryFixture(t)
			root := filepath.Dir(src)
			target := filepath.Join(root, "sentinel")
			if e := os.Mkdir(target, 0700); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(filepath.Join(target, "sentinel.txt"), []byte("expendable sentinel"), 0600); e != nil {
				t.Fatal(e)
			}
			link := filepath.Join(src, "junction")
			if kind == "source-root" {
				link = filepath.Join(root, "source junction")
				src = link
			}
			if kind == "output-parent" {
				link = filepath.Join(root, "output junction")
				dst = filepath.Join(link, "catalog.json")
			}
			// Only new generated paths; no global privilege or permission changes.
			if data, e := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); e != nil {
				t.Skipf("junction creation unavailable: %v %s", e, data)
			}
			before := inventoryTree(t, target)
			r, e := produceGUIInventory(context.Background(), src, dst, disposableInventoryLimits, inventoryIO{}, io.Discard)
			if e == nil || r.Published || !strings.Contains(e.Error(), "link/reparse") {
				t.Fatalf("junction accepted %+v %v", r, e)
			}
			if !reflect.DeepEqual(before, inventoryTree(t, target)) {
				t.Fatal("junction target changed")
			}
		})
	}
}
