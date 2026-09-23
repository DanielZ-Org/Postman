// Command postman is the M1 backend entrypoint for Delivery Office.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DanielZ-Org/Postman/backend/internal/api"
)

const defaultListenAddr = "127.0.0.1:8080"

func main() {
	addr := resolveListenAddr(os.Getenv)

	if isAllInterfaceBind(addr) {
		log.Fatalf("[postman] refusing to bind %q: initial deployment must listen on localhost only", addr)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("[postman] listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[postman] server error: %v", err)
	}
}

// resolveListenAddr returns the listen address from BACKEND_ADDR, or the
// localhost-only default when the variable is unset or blank.
func resolveListenAddr(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("BACKEND_ADDR")); v != "" {
		return v
	}
	return defaultListenAddr
}

// isAllInterfaceBind reports whether addr targets every network interface, which
// the initial deployment must never do.
func isAllInterfaceBind(addr string) bool {
	host := addr
	if i := strings.LastIndex(host, ":"); i != -1 {
		host = host[:i]
	}

	switch strings.ToLower(host) {
	case "", "0.0.0.0", "::", "[::]", "*":
		return true
	default:
		return false
	}
}
