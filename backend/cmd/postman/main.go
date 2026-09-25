// Command postman is the backend entrypoint for Delivery Office.
package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DanielZ-Org/Postman/backend/internal/api"
	"github.com/DanielZ-Org/Postman/backend/internal/game"
	"github.com/DanielZ-Org/Postman/backend/internal/persist"
)

const (
	defaultListenAddr = "127.0.0.1:8080"
	defaultDBPath     = "data/postman.db"
	defaultLogPath    = "data/postman.log"
	simulationTick    = 500 * time.Millisecond
	shutdownTimeout   = 5 * time.Second
)

func main() {
	addr := resolveListenAddr(os.Getenv)

	if isAllInterfaceBind(addr) {
		log.Fatalf("[postman] refusing to bind %q: initial deployment must listen on localhost only", addr)
	}

	closeLog := setupLogToFile(os.Getenv("POSTMAN_LOG_FILE"))
	defer closeLog()

	// Persistence (SPEC 13): the game layer owns the Store interface; this process
	// wires the SQLite adapter and loads any saved game before serving.
	store, err := persist.NewSQLite(resolveDBPath(os.Getenv))
	if err != nil {
		log.Fatalf("[postman] persistence unavailable: %v", err)
	}
	defer store.Close()

	state := game.NewInitialState()
	snapshot, err := store.Load()
	if err != nil {
		// A corrupt save must not brick the server; log it loudly and start fresh.
		log.Printf("[postman] ERROR: could not load saved game, starting fresh: %v", err)
	} else if snapshot != nil {
		if err := state.Restore(snapshot); err != nil {
			log.Printf("[postman] ERROR: could not restore saved game, starting fresh: %v", err)
		} else {
			log.Printf("[postman] restored saved game at %s", snapshot.GameTime)
		}
	}

	// Background simulation: advances the authoritative clock and processes due game
	// events in real time; each state-changing tick persists a snapshot.
	save := func() {
		if err := store.Save(state.Snapshot()); err != nil {
			log.Printf("[postman] ERROR: saving game state failed: %v", err)
		}
	}
	sim := state.StartSimulation(simulationTick, save)

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewRouter(state),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Printf("[postman] listening on %s", addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[postman] received %s, shutting down", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("[postman] ERROR: server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[postman] ERROR: graceful shutdown failed: %v", err)
	}
	sim.Stop()
	save() // final snapshot so no completed events are lost
	log.Printf("[postman] stopped")
}

// setupLogToFile routes application logging to both stderr and a permanent log file
// (required: console output alone scrolls away). It returns a closer for the file;
// when the file cannot be opened, logging stays on stderr with a warning.
func setupLogToFile(path string) func() {
	if strings.TrimSpace(path) == "" {
		path = defaultLogPath
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("[postman] WARNING: cannot create log directory %s: %v (logging to stderr only)", dir, err)
			return func() {}
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("[postman] WARNING: cannot open log file %s: %v (logging to stderr only)", path, err)
		return func() {}
	}
	log.SetOutput(io.MultiWriter(os.Stderr, file))
	return func() { _ = file.Close() }
}

// resolveListenAddr returns the listen address from BACKEND_ADDR, or the
// localhost-only default when the variable is unset or blank.
func resolveListenAddr(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("BACKEND_ADDR")); v != "" {
		return v
	}
	return defaultListenAddr
}

// resolveDBPath returns the SQLite path from POSTMAN_DB_PATH, or the default when
// the variable is unset or blank.
func resolveDBPath(getenv func(string) string) string {
	if v := strings.TrimSpace(getenv("POSTMAN_DB_PATH")); v != "" {
		return v
	}
	return defaultDBPath
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
