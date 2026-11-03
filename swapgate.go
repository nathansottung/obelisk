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
	"errors"
	"fmt"
	"sync"
	"time"
)

// swapGateState is the app's side of a swap (see the file comment).
type swapGateState struct {
	mu       sync.Mutex
	cond     *sync.Cond
	what     string // the restore or move swapping the catalog; "" when none
	inflight int    // requests and actions that entered and have not left
	// upgrade is the move of catalog.json into the catalog database at the first
	// start on that engine, while it runs or after it was refused
	// (catalog_upgrade.go). The app has no store meanwhile.
	upgrade *catalogUpgrade
}

func (g *swapGateState) condLocked() *sync.Cond {
	if g.cond == nil {
		g.cond = sync.NewCond(&g.mu)
	}
	return g.cond
}

// swapDrainWait is how long a swap waits for requests in progress.
var swapDrainWait = 30 * time.Second

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
	// A request that never finishes (a stalled network folder) must not hold every
	// other request off for good: after swapDrainWait the swap is refused instead.
	deadline := time.Now().Add(swapDrainWait)
	timer := time.AfterFunc(swapDrainWait, func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.condLocked().Broadcast()
	})
	defer timer.Stop()
	for g.inflight > 0 {
		if !time.Now().Before(deadline) {
			g.what = ""
			g.condLocked().Broadcast()
			return nil, fmt.Errorf("%s waits until no request is in progress, and %d still were after %v, so nothing "+
				"was changed. Try again", what, g.inflight, swapDrainWait)
		}
		g.condLocked().Wait()
	}
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.what = ""
	}, nil
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

// catalogUpgradeJobKind is the job kind of the move.
const catalogUpgradeJobKind = "catalog-upgrade"

const catalogUpgradeLabel = "Upgrade the catalog to the catalog database"

// catalogUpgrade is a move in progress, or one that was refused.
type catalogUpgrade struct {
	mu  sync.Mutex
	job Job
	err error // the refusal, once the move was refused
	// onStep is a test seam: nil in production. It is told each step as it
	// starts, after the job shows it.
	onStep func(step string)
}

// snapshot is the job as it stands.
func (u *catalogUpgrade) snapshot() Job {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.job
}

// refusal is what a request is answered while the move runs or after it failed.
func (u *catalogUpgrade) refusal() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.err != nil {
		return fmt.Errorf("the catalog could not be upgraded, so Obelisk is not using it: %v", u.err)
	}
	return errors.New("the catalog is being upgraded to the catalog database; nothing was changed. " +
		"Try again when the upgrade job has finished")
}

// catalogUpgradeSteps place each step of the move on the job's progress bar.
var catalogUpgradeSteps = map[string]struct {
	from, to float64
	text     string
}{
	"reading":    {0, 0.05, "reading and checking catalog.json"},
	"importing":  {0.05, 0.70, "copying records"},
	"indexing":   {0.70, 0.75, "building indexes"},
	"checking":   {0.75, 0.97, "checking every record against catalog.json"},
	"publishing": {0.97, 0.99, "keeping the old catalog.json and switching over"},
}

func (u *catalogUpgrade) progress(step string, done, total int) {
	st, ok := catalogUpgradeSteps[step]
	if !ok {
		return
	}
	u.mu.Lock()
	p, text := st.from, st.text
	if total > 0 {
		p += (st.to - st.from) * float64(done) / float64(total)
		text = fmt.Sprintf("%s (%d of %d)", text, done, total)
	}
	u.job.Progress = p
	u.job.Label = catalogUpgradeLabel + " — " + text
	u.mu.Unlock()
	if u.onStep != nil && done == 0 {
		u.onStep(step)
	}
}
