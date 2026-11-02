package main

// Shutdown saves what the catalog has pending (a batch's coalesced changes, file
// writes in the database's open transaction) and closes it. The server calls it
// on an interrupt or termination signal.
func (a *App) Shutdown() error { return a.Store.Close() }
