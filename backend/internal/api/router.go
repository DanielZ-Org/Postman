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
// The versioned boundary is /api/v1; business routes are registered on the inner v1 mux
// as M1 Acts implement them. No third-party middleware or router is used. The
// authoritative clock is passed in by reference so every route reads and mutates the same
// single instance — the API layer never owns a copy of mutable game state.
func NewRouter(clock *game.Clock) http.Handler {
	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	// M1C: read-only clock plus its three mutation endpoints (SPEC 14.4). Further
	// business routes are registered in later Acts.
	h := newClockHandler(clock)
	v1.HandleFunc("/clock", h.handle)
	v1.HandleFunc("/clock/speed", h.handleSpeed)
	v1.HandleFunc("/clock/pause", h.handlePause)
	v1.HandleFunc("/clock/skip-to-next-opening", h.handleSkip)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}
