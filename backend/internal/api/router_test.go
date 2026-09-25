package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

func TestNewRouterBuildsWithoutExternalServices(t *testing.T) {
	handler := NewRouter(game.NewInitialState())
	if handler == nil {
		t.Fatal("NewRouter returned nil handler")
	}
}

func TestGameRouteIsRegisteredUnderV1Boundary(t *testing.T) {
	router := NewRouter(game.NewInitialState())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/game", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/game = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestUnknownRouteStaysUnderV1Boundary(t *testing.T) {
	router := NewRouter(game.NewInitialState())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/does-not-exist = %d, want 404", w.Code)
	}
}
