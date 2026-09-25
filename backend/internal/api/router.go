// Package api owns the backend HTTP boundary: versioned REST/JSON under /api/v1.
//
// It uses only the standard library (net/http, http.ServeMux). Business routes are
// registered on the inner v1 mux as the game slice implements them. GET endpoints
// remain read-only; mutation endpoints validate input and return machine-readable
// JSON errors.
package api

import (
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// NewRouter builds the backend handler using only the standard library.
//
// The versioned boundary is /api/v1; business routes are registered on the inner v1
// mux. No third-party middleware or router is used. The authoritative game state is
// passed in by reference so every route reads and mutates the same single instance —
// the API layer never owns a copy of mutable game state.
func NewRouter(state *game.GameState) http.Handler {
	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	// Clock: read plus the three mutation endpoints (SPEC 14.4).
	ch := newClockHandler(state)
	v1.HandleFunc("/clock", ch.handle)
	v1.HandleFunc("/clock/speed", ch.handleSpeed)
	v1.HandleFunc("/clock/pause", ch.handlePause)
	v1.HandleFunc("/clock/skip-to-next-opening", ch.handleSkip)

	// Game projections (SPEC 14.2/3/4).
	gh := newGameHandler(state)
	v1.HandleFunc("/game", gh.handleGame)
	v1.HandleFunc("/player", gh.handlePlayer)
	v1.HandleFunc("/office", gh.handleOffice)

	// Office catalogue plus head-office selection (SPEC 4.3).
	oh := newOfficeHandler(state)
	v1.HandleFunc("/offices", oh.handleList)
	v1.HandleFunc("/offices/select", oh.handleSelect)

	// Packages, employees/hiring, delivery assignment (SPEC 5-9).
	ph := newPackageHandler(state)
	v1.HandleFunc("/packages", ph.handleList)
	eh := newEmployeeHandler(state)
	v1.HandleFunc("/employees", eh.handleList)
	v1.HandleFunc("/employees/hire", eh.handleHire)
	dh := newDeliveryHandler(state)
	v1.HandleFunc("/deliveries/assign", dh.handleAssign)

	// Finance statement and transaction history (SPEC 11).
	fh := newFinanceHandler(state)
	v1.HandleFunc("/finance", fh.handleStatement)
	v1.HandleFunc("/finance/transactions", fh.handleTransactions)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}
