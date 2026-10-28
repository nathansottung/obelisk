//go:build !guionly

package main

import (
	"sync"
	"testing"
)

// Looking up a covering choice while another request records one must not race
// (run under -race in CI).
func TestDestChoice_LookupWhileRecordingNoRace(t *testing.T) {
	app := dockApp(t)
	dir := t.TempDir()
	in := &destInspection{anchorID: fileIdent{vol: 1, idx: 99}, anchorCanon: dir, fingerprint: "x"}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			app.Store.coveringChoice(in)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			rec := &DestinationChoice{Volume: 1, Folder: uint64(100 + i), Path: dir, Keys: []string{"sid:x"}}
			app.Store.mu.Lock()
			app.Store.c.DestinationChoices = append(app.Store.c.DestinationChoices, rec)
			app.Store.mu.Unlock()
		}
	}()
	wg.Wait()
}
