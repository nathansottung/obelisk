//go:build !guionly

package main

// swapgate.go — no catalog edit while a restore or a data folder move swaps the
// catalog.
//
// A restore or a move copies the catalog and then replaces the store the app
// uses. An edit that lands in between would be lost: on the catalog database it
// reaches a store being closed, and on catalog.json it could even be saved over
// the restored file. So a swap holds the catalog at two levels:
//
//   - The app's swap gate. Every API request (except the two that run a restore or
//     a move, which take the gate themselves), each destructive action, and the
//     automatic app backup enter it. A swap waits for every request already in
//     progress to finish, then refuses new ones with 409 until it is done. So no
//     handler runs while the store is replaced, and none reports an edit that was
//     not kept.
//   - The store's hold (Store.holdEdits). It is refused while a job runs, saves
//     every pending change, and then refuses every job start and every save until
//     it is released. A store a swap has replaced is never released.

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// swapGate is the app's side of a swap (see the file comment).
type swapGateState struct {
	mu       sync.Mutex
	cond     *sync.Cond
	what     string // the restore or move swapping the catalog; "" when none
	inflight int    // requests and actions that entered and have not left
}

func (g *swapGateState) condLocked() *sync.Cond {
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
	return g.cond
}

// errSwapping refuses a request while what swaps the catalog.
func errSwapping(what string) error {
	return fmt.Errorf("%s is replacing the catalog, so nothing was changed. Try again when it has finished", what)
}

// enterCatalog lets a request or an action use the catalog, or refuses it while a
// swap runs. leave must be called when it is done.
func (a *App) enterCatalog() (leave func(), err error) {
	g := &a.swap
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.what != "" {
		return nil, errSwapping(g.what)
	}
	g.inflight++
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.inflight--
		g.condLocked().Broadcast()
	}, nil
}

// beginSwap starts what (a restore or a move): from now on every request is
// refused, and it returns once each request already in progress has finished.
// end lets requests in again. One swap runs at a time. A caller must not have
// entered the catalog itself.
func (a *App) beginSwap(what string) (end func(), err error) {
	g := &a.swap
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.what != "" {
		return nil, fmt.Errorf("%s waits until %s has finished, so nothing was changed", what, g.what)
	}
	g.what = what
	for g.inflight > 0 {
		g.condLocked().Wait()
	}
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.what = ""
	}, nil
}

// swapRunners are the requests that run a restore or a move: they take the gate
// themselves (beginSwap), and the destructive actions behind execute enter it one
// by one (ExecuteDestructive).
func swapRunner(r *http.Request) bool {
	return r.Method == http.MethodPost && (r.URL.Path == "/api/destructive/execute" || r.URL.Path == "/api/data-migration")
}

// swapGate puts every API request through the app's swap gate: one that arrives
// while the catalog is being swapped is answered 409, and a swap waits for the
// ones in progress.
func (a *App) swapGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || swapRunner(r) {
			next.ServeHTTP(w, r)
			return
		}
		leave, err := a.enterCatalog()
		if err != nil {
			jsonErr(w, http.StatusConflict, err)
			return
		}
		defer leave()
		next.ServeHTTP(w, r)
	})
}

// holdForSwap takes both gates for what: the app's (waiting for requests in
// progress) and the store's (refused while a job runs; pending changes saved).
// done ends the swap; it releases the store too unless keep was called, which a
// swap calls once the store must never take an edit again (it was closed or
// replaced, or a restore stopped partway).
func (a *App) holdForSwap(what string) (keep func(why string), done func(), err error) {
	end, err := a.beginSwap(what)
	if err != nil {
		return nil, nil, err
	}
	old := a.Store
	release, err := old.holdEdits(what)
	if err != nil {
		end()
		return nil, nil, err
	}
	kept := false
	keep = func(why string) {
		kept = true
		if why != "" {
			old.setHeld(why)
		}
	}
	done = func() {
		if !kept {
			release()
		}
		end()
	}
	if a.duringSwap != nil {
		a.duringSwap(what)
	}
	return keep, done, nil
}
