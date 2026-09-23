// Package api owns the backend HTTP boundary: versioned REST/JSON under /api/v1.
//
// It uses only the standard library (net/http, http.ServeMux). Business routes are
// registered on the inner v1 mux as M1 Acts implement them. GET endpoints remain
// read-only; mutation endpoints validate input and return machine-readable JSON errors.
package api

import (
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// NewRouter builds the backend handler using only the standard library.
//
// The versioned boundary is /api/v1; business routes are registered on the inner v1 mux as M1 Acts
// implement them. No third-party middleware or router is used. The authoritative game state is
// passed in by reference so every route reads and mutates the same single instance — the API layer
// never owns a copy of mutable game state (clock, selected office, cash, transactions).
func NewRouter(state *game.GameState) http.Handler {
	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	// M1C: read-only clock plus its three mutation endpoints (SPEC 14.4).
	ch := newClockHandler(state.Clock)
	v1.HandleFunc("/clock", ch.handle)
	v1.HandleFunc("/clock/speed", ch.handleSpeed)
	v1.HandleFunc("/clock/pause", ch.handlePause)
	v1.HandleFunc("/clock/skip-to-next-opening", ch.handleSkip)

	// M1D: read-only office catalogue plus the one-time head-office selection (SPEC 4.3).
	oh := newOfficeHandler(state)
	v1.HandleFunc("/offices", oh.handleList)
	v1.HandleFunc("/offices/select", oh.handleSelect)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}
