package main

// Shutdown saves what the catalog has pending (a batch's coalesced changes, file
// writes in the database's open transaction) and closes it. The server calls it
// on an interrupt or termination signal. While the first start's catalog upgrade
// runs there is no store yet: the move is safe to stop at any step, and the next
// start finishes it.
func (a *App) Shutdown() error {
	a.swap.mu.Lock()
	s := a.Store
	a.swap.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.Close()
}
