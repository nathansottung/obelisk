//go:build !guionly

package main

// swapgate_http.go — the API side of the swap gate (swapgate.go).

import (
	"net/http"
	"strings"
)

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
		if strings.HasPrefix(r.URL.Path, "/api/") && a.answerDuringUpgrade(w, r) {
			return
		}
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
