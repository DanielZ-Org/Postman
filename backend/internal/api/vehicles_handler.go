package api

import (
	"errors"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// vehiclesHandler serves GET /api/v1/vehicles and POST /api/v1/vehicles/purchase
// (approved plan section 3.3). It holds a reference to the authoritative game state;
// it performs no business validation itself — that lives in the game layer. Transport
// JSON concerns (request decoding, response shapes) are owned here.
type vehiclesHandler struct {
	state *game.GameState
}

func newVehiclesHandler(state *game.GameState) *vehiclesHandler {
	return &vehiclesHandler{state: state}
}

// handleList serves GET /api/v1/vehicles with the vehicle projection (owned counts
// plus the selected office's per-type capacity limits). Unsupported methods are
// rejected without mutating state.
func (h *vehiclesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, h.state.VehiclesView())
}

// handlePurchase serves POST /api/v1/vehicles/purchase with a strict
// {"vehicle": "bicycle"|"car"} body. Business rules live in the game layer (atomic
// purchase); this maps its errors to canonical JSON codes.
func (h *vehiclesHandler) handlePurchase(w http.ResponseWriter, r *http.Request) {
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
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object with only the 'vehicle' field", nil)
		return
	}
	for k := range obj {
		if k != "vehicle" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request", map[string]any{"field": k})
			return
		}
	}
	rawVehicle, present := obj["vehicle"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required field 'vehicle'", map[string]any{"field": "vehicle"})
		return
	}
	vehicle, err := parseStringField(rawVehicle)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_VEHICLE", "field 'vehicle' must be a string", nil)
		return
	}

	view, err := h.state.PurchaseVehicle(vehicle)
	if err != nil {
		h.writePurchaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// writePurchaseError maps purchase failures to canonical codes (SPEC 14.1 envelope).
func (h *vehiclesHandler) writePurchaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrGameOver):
		writeAPIError(w, http.StatusConflict, "GAME_OVER", "the game has ended", nil)
	case errors.Is(err, game.ErrNoOffice):
		writeAPIError(w, http.StatusConflict, "NO_OFFICE", "select a head office before purchasing vehicles", nil)
	case errors.Is(err, game.ErrUnknownVehicle):
		writeAPIError(w, http.StatusBadRequest, "INVALID_VEHICLE", "unknown vehicle type", map[string]any{"allowed": game.ValidVehicles()})
	default:
		var capErr *game.VehicleCapacityError
		if errors.As(err, &capErr) {
			writeAPIError(w, http.StatusConflict, "VEHICLE_CAPACITY_REACHED",
				"the office vehicle capacity for this type is already reached",
				map[string]any{"limit": capErr.Limit, "owned": capErr.Owned})
			return
		}
		var ife *game.InsufficientFundsError
		if errors.As(err, &ife) {
			writeAPIError(w, http.StatusConflict, "INSUFFICIENT_FUNDS", "not enough cash for the vehicle purchase",
				map[string]any{"required": ife.Required, "available": ife.Available})
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected vehicle purchase failure", nil)
	}
}
