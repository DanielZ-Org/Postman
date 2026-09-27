package api

import (
	"errors"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// traitHandler serves the one-time player-trait selection POST /api/v1/player/trait (SPEC 3). It
// holds a reference to the authoritative game state; it performs no business validation itself —
// that lives in the game layer. Transport JSON concerns (request decoding, response shapes) are
// owned here.
type traitHandler struct {
	state *game.GameState
}

func newTraitHandler(state *game.GameState) *traitHandler {
	return &traitHandler{state: state}
}

// handleSelectTrait serves POST /api/v1/player/trait. It decodes a strict {"trait": <string>} body,
// delegates the business rules to the game layer (atomic selection), and returns either the updated
// player projection or a canonical JSON error. Unsupported methods are rejected without mutating
// state.
func (h *traitHandler) handleSelectTrait(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, code := readBody(r, false) // JSON required; zero-length not allowed
	if code != "" {
		writeAPIError(w, http.StatusBadRequest, code, "request body must be a single valid JSON value", nil)
		return
	}
	obj, ok := decodeObject(body)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object with only the 'trait' field", nil)
		return
	}
	for k := range obj {
		if k != "trait" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request", map[string]any{"field": k})
			return
		}
	}
	rawTrait, present := obj["trait"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required field 'trait'", map[string]any{"field": "trait"})
		return
	}
	traitStr, err := parseStringField(rawTrait)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "field 'trait' must be a string", map[string]any{"field": "trait"})
		return
	}

	view, err := h.state.SelectTrait(traitStr)
	if err != nil {
		h.writeTraitError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playerResponse{Player: view})
}

// writeTraitError maps a game-layer trait-selection error to the canonical JSON error envelope with
// the SPEC-defined code and HTTP status. All four SelectTrait failure kinds are handled; the final
// defensive branch is unreachable (SelectTrait returns exactly one of them) but still emits
// well-formed JSON so the trait API never falls back to plain-text errors.
func (h *traitHandler) writeTraitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrGameOver):
		writeAPIError(w, http.StatusConflict, "GAME_OVER", "the game has ended", nil)
	case errors.Is(err, game.ErrInvalidTrait):
		writeAPIError(w, http.StatusBadRequest, "INVALID_TRAIT", "invalid trait value", map[string]any{"allowed": game.ValidTraits()})
	case errors.Is(err, game.ErrTraitAlreadySelected):
		writeAPIError(w, http.StatusConflict, "TRAIT_ALREADY_SELECTED", "a trait has already been selected", nil)
	case errors.Is(err, game.ErrAlreadySelected):
		writeAPIError(w, http.StatusConflict, "OFFICE_ALREADY_SELECTED", "select a trait before choosing an office", nil)
	default:
		// Unreachable: SelectTrait returns exactly one of the error kinds above. Emit a JSON 500
		// (never plain text) to keep the trait API contract intact in an impossible state.
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected trait selection failure", nil)
	}
}
