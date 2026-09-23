// Package api owns the backend HTTP boundary: versioned REST/JSON under /api/v1.
//
// It uses only the standard library (net/http, http.ServeMux). Business routes
// are registered on the inner v1 mux as M1 Acts implement them. GET endpoints
// must remain read-only and must not create or seed authoritative state.
package api

import "net/http"

// NewRouter builds the backend handler using only the standard library.
//
// The versioned boundary is /api/v1; business routes are registered on the
// inner v1 mux in later Acts. No third-party middleware or router is used.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	v1 := http.NewServeMux()

	// Business routes (game, clock, offices, ...) are registered on v1 in later Acts.
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1))

	return mux
}
