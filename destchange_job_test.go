//go:build (windows || linux) && !guionly

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A destination that changes in the middle of a job stops the job at once, with
// the reason, and nothing else is finished: no further file is attempted, the
// files copied so far are not recorded, and the inventory file is not written.
func TestDestChange_StopsJobAtOnce(t *testing.T) {
	jobs := map[string]func(app *App, collID int, dest string, volID int) error{
		"mirror": func(app *App, collID int, dest string, volID int) error {
			res, err := app.MirrorToVolume(collID, nil, dest, volID, 0, "", noProgMirror)
			if res != nil {
				t.Errorf("a stopped mirror returned a result: %+v", res)
			}
			return err
		},
		"back up changes": func(app *App, collID int, dest string, volID int) error {
			res, err := app.BackupChanges(collID, nil, volID, BaseVolume, ModeMirror, dest, 0, "", noProgMirror)
			if res != nil {
				t.Errorf("a stopped backup returned a result: %+v", res)
			}
			return err
		},
	}
	for name, run := range jobs {
		t.Run(name, func(t *testing.T) {
			app, collID, _, vol := mirrorSetup(t, "Stop", map[string]string{
				"a.txt": "alpha", "b.txt": "bravo", "sub/c.txt": "charlie", "sub/d.txt": "delta",
			})
			dest := privateDestForTest(t)
			// The destination changes when the second file is about to be published.
			old := beforeDestPublish
			t.Cleanup(func() { beforeDestPublish = old })
			publishes := 0
			beforeDestPublish = func(r *os.Root, tmp, final string) error {
				publishes++
				if publishes == 2 {
					loosenDestForTest(t, dest)
				}
				return nil
			}
			err := run(app, collID, dest, vol.ID)
			var changed *DestinationChangedError
			if !errors.As(err, &changed) {
				t.Fatalf("got %v, want the job stopped with a DestinationChangedError", err)
			}
			if !strings.HasPrefix(err.Error(), "stopped: the destination ") || !strings.Contains(err.Error(), "not recorded") {
				t.Fatalf("the reason is not clear: %v", err)
			}
			if publishes != 2 {
				t.Fatalf("%d publications attempted, want 2: nothing after the change", publishes)
			}
			var files []string
			_ = filepath.WalkDir(dest, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					rel, _ := filepath.Rel(dest, p)
					files = append(files, filepath.ToSlash(rel))
				}
				return nil
			})
			if len(files) != 1 {
				t.Fatalf("destination holds %q, want only the one file published before the change (no inventory file)", files)
			}
			for _, c := range app.Store.Chunks(collID) {
				if c.Mirror {
					t.Fatalf("the stopped job recorded mirror copies: %+v", c)
				}
			}
		})
	}
}

// A change found only while the inventory files are written, after every copy
// was made, leaves the run's records consistent: the copies and, for a
// back-up-changes run, its history entry are recorded together, and the job
// still stops with the reason. The change is made before the first inventory
// file (publication 5) and between the two (publication 6).
func TestDestChange_StopDuringInventoryKeepsRecordsTogether(t *testing.T) {
	for _, at := range []int{5, 6} {
		for _, job := range []string{"mirror", "back up changes"} {
			t.Run(fmt.Sprintf("%s/publication %d", job, at), func(t *testing.T) {
				app, collID, _, vol := mirrorSetup(t, "Inv", map[string]string{
					"a.txt": "alpha", "b.txt": "bravo", "sub/c.txt": "charlie", "sub/d.txt": "delta",
				})
				dest := privateDestForTest(t)
				old := beforeDestPublish
				t.Cleanup(func() { beforeDestPublish = old })
				publishes := 0
				beforeDestPublish = func(r *os.Root, tmp, final string) error {
					publishes++
					if publishes == at {
						loosenDestForTest(t, dest)
					}
					return nil
				}
				var err error
				if job == "mirror" {
					_, err = app.MirrorToVolume(collID, nil, dest, vol.ID, 0, "", noProgMirror)
				} else {
					_, err = app.BackupChanges(collID, nil, vol.ID, BaseVolume, ModeMirror, dest, 0, "", noProgMirror)
				}
				var changed *DestinationChangedError
				if !errors.As(err, &changed) {
					t.Fatalf("got %v, want the job stopped with a DestinationChangedError", err)
				}
				if !strings.Contains(err.Error(), "may not be up to date") {
					t.Fatalf("the reason does not say the inventory may be stale: %v", err)
				}
				if publishes != at {
					t.Fatalf("%d publications attempted, want %d: nothing after the change", publishes, at)
				}
				recorded := 0
				for _, c := range app.Store.Chunks(collID) {
					if c.Mirror {
						recorded += len(c.Files)
					}
				}
				if recorded != 4 {
					t.Fatalf("%d copies recorded, want all 4 made before the change", recorded)
				}
				sessions := app.Store.BackupSessions(collID)
				switch {
				case job == "mirror" && len(sessions) != 0:
					t.Fatalf("a mirror recorded backup history: %+v", sessions)
				case job != "mirror" && (len(sessions) != 1 || sessions[0].Files != 4):
					t.Fatalf("history %+v, want one backup of the 4 recorded copies", sessions)
				}
			})
		}
	}
}
