package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// officeHandler serves the read-only GET /api/v1/offices catalogue and the one-time head-office
// selection POST /api/v1/offices/select (SPEC 4.3). It holds a reference to the authoritative game
// state; it performs no business validation itself — that lives in the game layer. Transport JSON
// concerns (request decoding, response shapes) are owned here.
type officeHandler struct {
	state *game.GameState
}

func newOfficeHandler(state *game.GameState) *officeHandler {
	return &officeHandler{state: state}
}

// catalogueStorageJSON is the storage view of a catalogue definition (base/max only — no runtime
// current/used fields, per SPEC 4.3).
type catalogueStorageJSON struct {
	Base int `json:"base"`
	Max  int `json:"max"`
}

// catalogueOfficeJSON is the wire shape for one office option in GET /api/v1/offices. It contains
// only static catalogue fields — no runtime fields (is_head_office, next_rent_due, storage.current/
// used, contract_status, missed_rent_payments).
type catalogueOfficeJSON struct {
	ID                   string               `json:"id"`
	Type                 string               `json:"type"`
	DownPayment          int                  `json:"down_payment"`
	WeeklyRent           int                  `json:"weekly_rent"`
	RentPrepaidWeeks     int                  `json:"rent_prepaid_weeks"`
	Storage              catalogueStorageJSON `json:"storage"`
	EmployeeCapacity     int                  `json:"employee_capacity"`
	BicycleCapacity      int                  `json:"bicycle_capacity"`
	VehicleCapacity      int                  `json:"vehicle_capacity"`
	AcceptedPackageSizes []string             `json:"accepted_package_sizes"`
}

// officesResponse is the wire shape for GET /api/v1/offices: a single "offices" wrapper.
type officesResponse struct {
	Offices []catalogueOfficeJSON `json:"offices"`
}

func catalogueFrom(def game.OfficeDefinition) catalogueOfficeJSON {
	return catalogueOfficeJSON{
		ID:                   def.ID,
		Type:                 def.Type,
		DownPayment:          def.DownPayment,
		WeeklyRent:           def.WeeklyRent,
		RentPrepaidWeeks:     def.RentPrepaidWeeks,
		Storage:              catalogueStorageJSON{Base: def.StorageBase, Max: def.StorageMax},
		EmployeeCapacity:     def.EmployeeCapacity,
		BicycleCapacity:      def.BicycleCapacity,
		VehicleCapacity:      def.VehicleCapacity,
		AcceptedPackageSizes: def.AcceptedPackageSizes,
	}
}

// runtimeStorageJSON is the storage view of a selected (runtime) office.
type runtimeStorageJSON struct {
	Base    int `json:"base"`
	Current int `json:"current"`
	Max     int `json:"max"`
	Used    int `json:"used"`
}

// runtimeOfficeJSON is the wire shape for the selected runtime office returned by a successful
// selection (SPEC 4.1 representation).
type runtimeOfficeJSON struct {
	ID                   string             `json:"id"`
	Type                 string             `json:"type"`
	IsHeadOffice         bool               `json:"is_head_office"`
	DownPayment          int                `json:"down_payment"`
	WeeklyRent           int                `json:"weekly_rent"`
	RentPrepaidWeeks     int                `json:"rent_prepaid_weeks"`
	NextRentDue          string             `json:"next_rent_due"`
	Storage              runtimeStorageJSON `json:"storage"`
	EmployeeCapacity     int                `json:"employee_capacity"`
	BicycleCapacity      int                `json:"bicycle_capacity"`
	VehicleCapacity      int                `json:"vehicle_capacity"`
	AcceptedPackageSizes []string           `json:"accepted_package_sizes"`
	ContractStatus       string             `json:"contract_status"`
	MissedRentPayments   int                `json:"missed_rent_payments"`
}

// selectResponse is the wire shape for a successful POST /api/v1/offices/select: the selected
// runtime office plus the resulting cash balance (integer pounds). It does not return the whole
// game state.
type selectResponse struct {
	Office      runtimeOfficeJSON `json:"office"`
	CashBalance int               `json:"cash_balance"`
}

func runtimeFrom(o *game.RuntimeOffice) runtimeOfficeJSON {
	return runtimeOfficeJSON{
		ID:                   o.ID,
		Type:                 o.Type,
		IsHeadOffice:         o.IsHeadOffice,
		DownPayment:          o.DownPayment,
		WeeklyRent:           o.WeeklyRent,
		RentPrepaidWeeks:     o.RentPrepaidWeeks,
		NextRentDue:          o.NextRentDue,
		Storage:              runtimeStorageJSON{Base: o.Storage.Base, Current: o.Storage.Current, Max: o.Storage.Max, Used: o.Storage.Used},
		EmployeeCapacity:     o.EmployeeCapacity,
		BicycleCapacity:      o.BicycleCapacity,
		VehicleCapacity:      o.VehicleCapacity,
		AcceptedPackageSizes: o.AcceptedPackageSizes,
		ContractStatus:       o.ContractStatus,
		MissedRentPayments:   o.MissedRentPayments,
	}
}

// handleList serves GET /api/v1/offices with the canonical catalogue wrapper in deterministic order.
// It is read-only: it does not mutate selected-office state, cash, or transactions. Unsupported
// methods are rejected (405 METHOD_NOT_ALLOWED, JSON envelope).
func (h *officeHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	resp := officesResponse{Offices: make([]catalogueOfficeJSON, 0)}
	for _, def := range game.OfficeDefinitions() {
		resp.Offices = append(resp.Offices, catalogueFrom(def))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSelect serves POST /api/v1/offices/select. It decodes a strict {"office_id": <string>} body,
// delegates the business rules to the game layer (atomic selection), and returns either the updated
// office + cash balance or a canonical JSON error. Unsupported methods are rejected without mutating
// state.
func (h *officeHandler) handleSelect(w http.ResponseWriter, r *http.Request) {
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
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object with only the 'office_id' field", nil)
		return
	}
	for k := range obj {
		if k != "office_id" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request", map[string]any{"field": k})
			return
		}
	}
	rawID, present := obj["office_id"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required field 'office_id'", map[string]any{"field": "office_id"})
		return
	}
	idStr, err := parseStringField(rawID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "field 'office_id' must be a string", map[string]any{"field": "office_id"})
		return
	}
	if idStr == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "field 'office_id' must not be empty", map[string]any{"field": "office_id"})
		return
	}

	office, cash, err := h.state.SelectOffice(idStr)
	if err != nil {
		h.writeSelectionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, selectResponse{Office: runtimeFrom(office), CashBalance: cash})
}

// writeSelectionError maps a game-layer selection error to the canonical JSON error envelope with the
// SPEC-defined code and HTTP status. All three SelectOffice failure kinds are handled; the final
// defensive branch is unreachable (SelectOffice returns exactly one of them) but still emits
// well-formed JSON so the Office API never falls back to plain-text errors.
func (h *officeHandler) writeSelectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrOfficeNotFound):
		writeAPIError(w, http.StatusNotFound, "OFFICE_NOT_FOUND", "no selectable office with the given id", nil)
	case errors.Is(err, game.ErrAlreadySelected):
		writeAPIError(w, http.StatusConflict, "OFFICE_ALREADY_SELECTED", "a head office is already selected", nil)
	case errors.Is(err, game.ErrGameOver):
		writeAPIError(w, http.StatusConflict, "GAME_OVER", "the game has ended", nil)
	default:
		var ife *game.InsufficientFundsError
		if errors.As(err, &ife) {
			writeAPIError(w, http.StatusConflict, "INSUFFICIENT_FUNDS", "insufficient cash for the down payment", map[string]any{"required": ife.Required, "available": ife.Available})
			return
		}
		// Unreachable: SelectOffice returns exactly one of the error kinds above. Emit a JSON 500
		// (never plain text) to keep the Office API contract intact in an impossible state.
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected office selection failure", nil)
	}
}

// parseStringField parses a JSON string literal into a Go string. It rejects any value that is not a
// JSON string (numbers, booleans, objects, arrays).
func parseStringField(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", errors.New("not a string")
	}
	return s, nil
}

// writeJSON writes v as JSON with the given status and application/json content type.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
