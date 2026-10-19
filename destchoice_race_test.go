//go:build !guionly

package main

import (
	"sync"
	"testing"
)

// Reading a choice's created folders while a job records new ones must not race
// (run under -race in CI).
func TestDestChoice_CreatedListNoRace(t *testing.T) {
	app := dockApp(t)
	rec := &DestinationChoice{Volume: 1, Folder: 2, Path: t.TempDir(), Created: []uint64{3}}
	app.Store.mu.Lock()
	app.Store.c.DestinationChoices = append(app.Store.c.DestinationChoices, rec)
	app.Store.mu.Unlock()
	in := &destInspection{anchorID: fileIdent{vol: 1, idx: 99}, fingerprint: "x"}
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
			app.Store.recordCreatedUnderChoice(&DestinationChoice{Volume: 1, Folder: 2}, []uint64{uint64(100 + i)})
		}
	}()
	wg.Wait()
}
