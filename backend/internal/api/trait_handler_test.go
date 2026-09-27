package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// TestSelectTraitSucceeds verifies a valid selection via the API: 200, JSON content type, an
// exactly-one-key "player" wrapper with the updated trait (same shape as GET /api/v1/player), and
// GET /api/v1/player reflecting the committed selection. No cash change.
func TestSelectTraitSucceeds(t *testing.T) {
	h, state := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", `{"trait":"logistics"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var top map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &top); err != nil {
		t.Fatalf("decode body: %v; body=%s", err, rec.Body.String())
	}
	if len(top) != 1 || top["player"] == nil {
		t.Fatalf("wrapper keys = %d (has player? %v), want exactly one key 'player'", len(top), top["player"] != nil)
	}

	var body struct {
		Player map[string]any `json:"player"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode player: %v", err)
	}
	if body.Player["trait"] != "logistics" || body.Player["id"] != "player-1" {
		t.Errorf("player = %v, want trait logistics id player-1", body.Player)
	}

	// GET /api/v1/player reflects the committed selection.
	get := doRequest(t, h, http.MethodGet, "/api/v1/player", "")
	if get.Code != http.StatusOK {
		t.Fatalf("GET /player status = %d, want 200; body=%s", get.Code, get.Body.String())
	}
	var playerBody struct {
		Player map[string]any `json:"player"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &playerBody); err != nil {
		t.Fatalf("decode GET /player: %v", err)
	}
	if playerBody.Player["trait"] != "logistics" {
		t.Errorf("GET /player trait = %v, want logistics", playerBody.Player["trait"])
	}

	// Authoritative state committed atomically; no cash change (SPEC defines no cost).
	if got := state.Cash(); got != 100000 {
		t.Errorf("cash = %d after selection, want unchanged 100000p", got)
	}
	if view := state.PlayerView(); view.Trait != "logistics" {
		t.Errorf("authoritative trait = %q, want logistics", view.Trait)
	}
}

// TestSelectTraitInvalidRequests verifies each transport-level rejection returns 400 with the correct
// code and leaves authoritative state unchanged.
func TestSelectTraitInvalidRequests(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"malformed JSON", `{not json`, "INVALID_JSON"},
		{"trailing JSON value", `{"trait":"storage"} extra`, "INVALID_JSON"},
		{"missing trait", `{}`, "INVALID_REQUEST"},
		{"wrong type (number)", `{"trait":123}`, "INVALID_REQUEST"},
		{"unknown field", `{"trait":"storage","extra":1}`, "INVALID_REQUEST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, state := newTestRouter()
			rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if got := errorCode(t, rec.Body.Bytes()); got != tc.wantCode {
				t.Errorf("code = %q, want %q; body=%s", got, tc.wantCode, rec.Body.String())
			}
			// Rejected requests must not mutate authoritative state.
			if view := state.PlayerView(); view.Trait != "financial" {
				t.Errorf("trait = %v after rejected request, want unchanged financial", view.Trait)
			}
			if got := state.Cash(); got != 100000 {
				t.Errorf("cash = %d after rejected request, want unchanged 100000p", got)
			}
		})
	}
}

// TestSelectTraitOutOfSet400 verifies an out-of-set (but syntactically valid string) trait value
// returns 400 INVALID_TRAIT with the allowed-values details and leaves state unchanged.
func TestSelectTraitOutOfSet400(t *testing.T) {
	h, state := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", `{"trait":"bogus"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec.Body.Bytes()); got != "INVALID_TRAIT" {
		t.Errorf("code = %q, want INVALID_TRAIT; body=%s", got, rec.Body.String())
	}

	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body=%s", err, rec.Body.String())
	}
	allowed, _ := body.Error.Details["allowed"].([]any)
	if len(allowed) != 3 || allowed[0] != "financial" || allowed[1] != "storage" || allowed[2] != "logistics" {
		t.Errorf("details.allowed = %v, want [financial storage logistics]", body.Error.Details["allowed"])
	}

	if view := state.PlayerView(); view.Trait != "financial" {
		t.Errorf("trait = %v after rejected request, want unchanged financial", view.Trait)
	}
}

// TestSelectTraitAlreadySelected409 verifies the one-time rule via the API: a second selection
// returns 409 TRAIT_ALREADY_SELECTED and leaves state unchanged.
func TestSelectTraitAlreadySelected409(t *testing.T) {
	h, state := newTestRouter()
	if rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", `{"trait":"storage"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup select storage: status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", `{"trait":"logistics"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec.Body.Bytes()); got != "TRAIT_ALREADY_SELECTED" {
		t.Errorf("code = %q, want TRAIT_ALREADY_SELECTED; body=%s", got, rec.Body.String())
	}
	if view := state.PlayerView(); view.Trait != "storage" {
		t.Errorf("trait = %v after rejected second selection, want unchanged storage", view.Trait)
	}
}

// TestSelectTraitAfterOffice409 verifies the window closes at the first office selection: 409
// OFFICE_ALREADY_SELECTED with the trait-first message and state unchanged.
func TestSelectTraitAfterOffice409(t *testing.T) {
	h, state := newTestRouter()
	if rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup select office: status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v1/player/trait", `{"trait":"logistics"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec.Body.Bytes()); got != "OFFICE_ALREADY_SELECTED" {
		t.Errorf("code = %q, want OFFICE_ALREADY_SELECTED; body=%s", got, rec.Body.String())
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Message != "select a trait before choosing an office" {
		t.Errorf("message = %q, want 'select a trait before choosing an office'", body.Error.Message)
	}

	if view := state.PlayerView(); view.Trait != "financial" {
		t.Errorf("trait = %v after rejected request, want unchanged financial", view.Trait)
	}
}

// TestSelectTraitGameOverMapsTo409 verifies the HTTP-layer mapping of a game-over error to 409
// GAME_OVER. The api package cannot set unexported status fields directly, so this code path is
// exercised through the handler's error mapping rather than via doRequest; the full domain behavior
// (game over takes precedence in the validation order) is covered by the game-layer tests.
func TestSelectTraitGameOverMapsTo409(t *testing.T) {
	state := game.NewInitialState()
	th := newTraitHandler(state)

	w := httptest.NewRecorder()
	th.writeTraitError(w, game.ErrGameOver)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body=%s", err, w.Body.String())
	}
	if body.Error.Code != "GAME_OVER" {
		t.Errorf("code = %q, want GAME_OVER", body.Error.Code)
	}
}

// TestPlayerTraitUnsupportedMethods verifies unsupported methods on the trait route return 405 with a
// JSON METHOD_NOT_ALLOWED error and do not mutate state.
func TestPlayerTraitUnsupportedMethods(t *testing.T) {
	t.Run("GET /player/trait rejected", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodGet, "/api/v1/player/trait", "")
		if rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
			t.Fatalf("status=%d code=%s, want 405 METHOD_NOT_ALLOWED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if view := state.PlayerView(); view.Trait != "financial" {
			t.Errorf("trait = %v after rejected method, want unchanged financial", view.Trait)
		}
	})

	t.Run("DELETE /player/trait rejected without mutation", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodDelete, "/api/v1/player/trait", "")
		if rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
			t.Fatalf("status=%d code=%s, want 405 METHOD_NOT_ALLOWED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if got := state.Cash(); got != 100000 {
			t.Errorf("cash = %d after rejected method, want unchanged 100000p", got)
		}
	})
}
