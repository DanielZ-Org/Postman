package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

func TestNewRouterBuildsWithoutExternalServices(t *testing.T) {
	handler := NewRouter(game.NewClock())
	if handler == nil {
		t.Fatal("NewRouter returned nil handler")
	}
}

func TestUnimplementedRouteStaysUnderV1Boundary(t *testing.T) {
	router := NewRouter(game.NewClock())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/game", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/game = %d, want 404 (not implemented yet)", w.Code)
	}
}
