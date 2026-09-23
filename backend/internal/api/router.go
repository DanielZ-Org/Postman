// Package api owns the backend HTTP boundary: versioned REST/JSON under /api/v1.
//
// It uses only the standard library (net/http, http.ServeMux). Business routes
// are registered on the inner v1 mux as M1 Acts implement them. GET endpoints
// must remain read-only and must not create or seed authoritative state.
package api

import (
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// NewRouter builds the backend handler using only the standard library.
//
// The versioned boundary is /api/v1; business routes are registered on the inner
// v1 mux as M1 Acts implement them. No third-party middleware or router is used.
// The authoritative clock is passed in so the API layer never owns mutable game
// state itself — it only reads a snapshot of it.
func NewRouter(clock game.Clock) http.Handler {
	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	// M1B: read-only clock endpoint. Further business routes are registered in later Acts.
	v1.HandleFunc("/clock", newClockHandler(clock).handle)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}
