package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// deliveryHandler serves POST /api/v1/deliveries/assign (SPEC 9): the player
// requests a batch count for a specific employee; the backend chooses and validates
// the actual packages.
type deliveryHandler struct {
	state *game.GameState
}

func newDeliveryHandler(state *game.GameState) *deliveryHandler {
	return &deliveryHandler{state: state}
}

// assignResponse is the successful assignment payload: the updated employee and the
// ids of the packages that joined the run.
type assignResponse struct {
	Employee employeeJSON `json:"employee"`
	Assigned []string     `json:"assigned"`
}

// handleAssign serves POST /api/v1/deliveries/assign with a strict
// {"employee_id": <string>, "package_count": <int>} body.
func (h *deliveryHandler) handleAssign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, code := readBody(r, false) // JSON required
	if code != "" {
		writeAPIError(w, http.StatusBadRequest, code, "request body must be a single valid JSON value", nil)
		return
	}
	obj, ok := decodeObject(body)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST",
			"request body must be a JSON object with the 'employee_id' and 'package_count' fields", nil)
		return
	}
	for k := range obj {
		if k != "employee_id" && k != "package_count" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request",
				map[string]any{"field": k})
			return
		}
	}

	rawID, present := obj["employee_id"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_EMPLOYEE_ID", "employee_id must be a string", nil)
		return
	}
	employeeID, err := parseStringField(rawID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_EMPLOYEE_ID", "employee_id must be a string", nil)
		return
	}

	rawCount, present := obj["package_count"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE_COUNT", "package_count must be a positive integer", nil)
		return
	}
	count, err := parseJSONInt(rawCount)
	if err != nil || count < 1 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PACKAGE_COUNT", "package_count must be a positive integer", nil)
		return
	}

	emp, assigned, err := h.state.AssignDelivery(employeeID, count)
	if err != nil {
		h.writeAssignError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assignResponse{Employee: employeeFrom(*emp), Assigned: assigned})
}

// writeAssignError maps assignment failures to canonical codes (SPEC 14.1).
func (h *deliveryHandler) writeAssignError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, game.ErrGameOver):
		writeAPIError(w, http.StatusConflict, "GAME_OVER", "the game has ended", nil)
	case errors.Is(err, game.ErrEmployeeNotFound):
		writeAPIError(w, http.StatusNotFound, "EMPLOYEE_NOT_FOUND", "unknown employee id", nil)
	case errors.Is(err, game.ErrEmployeeBusy):
		writeAPIError(w, http.StatusConflict, "EMPLOYEE_BUSY", "employee is not available for a new assignment", nil)
	case errors.Is(err, game.ErrNoStoredPackages):
		writeAPIError(w, http.StatusConflict, "NO_STORED_PACKAGES", "no stored packages available for assignment", nil)
	case errors.Is(err, game.ErrRunsLimitReached):
		writeAPIError(w, http.StatusConflict, "RUNS_LIMIT_REACHED", "the employee has no delivery runs left today", nil)
	default:
		var capErr *game.CapacityError
		if errors.As(err, &capErr) {
			writeAPIError(w, http.StatusBadRequest, "INSUFFICIENT_DELIVERY_CAPACITY",
				"employee does not have enough capacity for this assignment",
				map[string]any{
					"employee_id":        capErr.EmployeeID,
					"available_capacity": capErr.Available,
					"requested_capacity": capErr.Requested,
					"stored_available":   capErr.StoredAvailable,
				})
			return
		}
		var cycErr *game.CycleError
		if errors.As(err, &cycErr) {
			details := map[string]any{"requested_start": cycErr.RequestedStart}
			if cycErr.ClosingTime != "" {
				details["closing_time"] = cycErr.ClosingTime
			}
			writeAPIError(w, http.StatusConflict, "CYCLE_WOULD_NOT_FINISH",
				"a full walking cycle (1h packing + 3h delivery) must finish before closing time", details)
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected assignment failure", nil)
	}
}

// financeHandler serves GET /api/v1/finance (SPEC 11.3 statement, bare at the top
// level as the frontend expects) and GET /api/v1/finance/transactions.
type financeHandler struct {
	state *game.GameState
}

func newFinanceHandler(state *game.GameState) *financeHandler {
	return &financeHandler{state: state}
}

// transactionsResponse wraps the transaction list, newest first.
type transactionsResponse struct {
	Transactions []game.Transaction `json:"transactions"`
}

// handleStatement serves GET /api/v1/finance with the current-week statement.
func (h *financeHandler) handleStatement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, h.state.FinanceView())
}

// handleTransactions serves GET /api/v1/finance/transactions, newest first.
func (h *financeHandler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	txns := h.state.TransactionsView()
	if txns == nil {
		txns = []game.Transaction{}
	}
	writeJSON(w, http.StatusOK, transactionsResponse{Transactions: txns})
}

// parseJSONInt parses a JSON integer literal into an int, rejecting fractional,
// exponent or non-numeric values.
func parseJSONInt(raw json.RawMessage) (int, error) {
	return parseClockSpeed(raw)
}
