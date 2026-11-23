package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// blockFixture builds a /sys/class/block tree. Each device names the devices it is
// built on, and "<disk>p1"-style partitions name their parent disk.
func blockFixture(t *testing.T, devs map[string]string, slaves map[string][]string, partitionOf map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, majMin := range devs {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(dir, "slaves"), 0o755); err != nil {
			t.Fatal(err)
		}
		put := func(p, body string) {
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		put(filepath.Join(dir, "dev"), majMin+"\n")
		for _, s := range slaves[name] {
			put(filepath.Join(dir, "slaves", s), "")
		}
		if p, ok := partitionOf[name]; ok {
			put(filepath.Join(dir, "partition"), "1\n")
			put(filepath.Join(dir, "whole-disk"), p+"\n")
		}
	}
	return root
}

// Two logical volumes on one disk share that disk; a mirror names both of its disks.
func TestLeafDisks_FollowsStackedDevices(t *testing.T) {
	root := blockFixture(t,
		map[string]string{"sda": "8:0", "sda1": "8:1", "sdb": "8:16", "sdb1": "8:17", "dm-0": "253:0", "dm-1": "253:1", "md0": "9:0", "dm-2": "253:2"},
		map[string][]string{"dm-0": {"sda1"}, "dm-1": {"sda1"}, "md0": {"sda1", "sdb1"}, "dm-2": {"md0"}},
		map[string]string{"sda1": "sda", "sdb1": "sdb"})
	parent := func(name string) string {
		return map[string]string{"sda1": "sda", "sdb1": "sdb"}[name]
	}
	for start, want := range map[string][]string{
		"sda":  {"disk:8:0"},
		"sda1": {"disk:8:0"},
		"dm-0": {"disk:8:0"},
		"dm-1": {"disk:8:0"},
		"md0":  {"disk:8:0", "disk:8:16"},
		"dm-2": {"disk:8:0", "disk:8:16"},
	} {
		got, err := leafDisks(root, start, parent)
		sort.Strings(got)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, %v; want %v", start, got, err, want)
		}
	}
}

func TestLeafDisks_StopsOnALoopAndAMissingDevice(t *testing.T) {
	root := blockFixture(t,
		map[string]string{"dm-0": "253:0", "dm-1": "253:1", "dm-2": "253:2"},
		map[string][]string{"dm-0": {"dm-1"}, "dm-1": {"dm-0"}, "dm-2": {"gone"}}, nil)
	none := func(string) string { return "" }
	if got, err := leafDisks(root, "dm-0", none); err == nil {
		t.Errorf("a loop gave %v", got)
	}
	if got, err := leafDisks(root, "dm-2", none); err == nil {
		t.Errorf("a missing device gave %v", got)
	}
}
