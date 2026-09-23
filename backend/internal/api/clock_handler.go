package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// clockResponse is the wire shape for GET /api/v1/clock and, per SPEC 14.4, also for
// the successful responses of the three mutation endpoints: a single top-level "clock"
// wrapper whose fields use the exact JSON names from the canonical contract. No separate
// mutation-response DTO shapes are introduced.
type clockResponse struct {
	Clock struct {
		GameDatetime         string `json:"game_datetime"`
		DayOfWeek            string `json:"day_of_week"`
		Speed                int    `json:"speed"`
		Paused               bool   `json:"paused"`
		OfficeOpen           bool   `json:"office_open"`
		DaysUntilNextPayroll int    `json:"days_until_next_payroll"`
		DaysUntilNextRent    int    `json:"days_until_next_rent"`
	} `json:"clock"`
}

// apiError is the canonical machine-readable error envelope (SPEC 14.1). details is
// optional and omitted when nil.
type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// clockHandler serves the read-only GET /api/v1/clock endpoint and its three mutation
// endpoints (SPEC 14.4). It holds a reference to the single authoritative clock; it never
// owns a copy of mutable game state and performs no working-hour or weekday calculations
// itself — those live in the game layer.
type clockHandler struct {
	clock *game.Clock
}

func newClockHandler(clock *game.Clock) *clockHandler {
	return &clockHandler{clock: clock}
}

// handle serves GET /api/v1/clock with the canonical "clock" wrapper. Unsupported methods
// are rejected (405 METHOD_NOT_ALLOWED, JSON envelope) and never execute the read behaviour.
func (h *clockHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	h.writeClock(w, h.clock.Snapshot())
}

// handleSpeed serves POST /api/v1/clock/speed. It decodes a strict {"speed": <int>} body,
// validates the value in the game layer, and returns the updated clock snapshot on success.
func (h *clockHandler) handleSpeed(w http.ResponseWriter, r *http.Request) {
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
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object with only the 'speed' field", nil)
		return
	}
	for k := range obj {
		if k != "speed" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request", map[string]any{"field": k})
			return
		}
	}
	rawSpeed, present := obj["speed"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required field 'speed'", map[string]any{"field": "speed"})
		return
	}
	speed, err := parseClockSpeed(rawSpeed)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "field 'speed' must be an integer", map[string]any{"field": "speed"})
		return
	}
	if err := h.clock.SetSpeed(speed); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_CLOCK_SPEED", "speed must be one of the allowed values", map[string]any{"allowed": game.ValidSpeeds()})
		return
	}
	h.writeClock(w, h.clock.Snapshot())
}

// handlePause serves POST /api/v1/clock/pause. It decodes a strict {"paused": <bool>} body
// and returns the updated clock snapshot on success.
func (h *clockHandler) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, code := readBody(r, false)
	if code != "" {
		writeAPIError(w, http.StatusBadRequest, code, "request body must be a single valid JSON value", nil)
		return
	}
	obj, ok := decodeObject(body)
	if !ok {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object with only the 'paused' field", nil)
		return
	}
	for k := range obj {
		if k != "paused" {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "unknown or unexpected field in request", map[string]any{"field": k})
			return
		}
	}
	rawPaused, present := obj["paused"]
	if !present {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "missing required field 'paused'", map[string]any{"field": "paused"})
		return
	}
	paused, err := parseBoolField(rawPaused)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "field 'paused' must be a boolean", map[string]any{"field": "paused"})
		return
	}
	h.clock.SetPaused(paused)
	h.writeClock(w, h.clock.Snapshot())
}

// handleSkip serves POST /api/v1/clock/skip-to-next-opening. It accepts an empty body or {}
// (no fields), advances the clock in the game layer, and returns the updated snapshot.
func (h *clockHandler) handleSkip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, code := readBody(r, true) // zero-length body accepted
	if code != "" {
		writeAPIError(w, http.StatusBadRequest, code, "request body must be a single valid JSON value", nil)
		return
	}
	if len(body) > 0 {
		obj, ok := decodeObject(body)
		if !ok {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "skip request body must be an empty JSON object {}", nil)
			return
		}
		for k := range obj {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "skip request must not contain fields", map[string]any{"field": k})
			return
		}
	}

	h.clock.SkipToNextOpening()
	h.writeClock(w, h.clock.Snapshot())
}

// writeClock writes the updated clock snapshot under the canonical "clock" wrapper with HTTP 200.
func (h *clockHandler) writeClock(w http.ResponseWriter, snap game.ClockSnapshot) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(clockResponseFrom(snap))
}

// clockResponseFrom maps a read-only snapshot to the wire shape.
func clockResponseFrom(snap game.ClockSnapshot) clockResponse {
	var resp clockResponse
	resp.Clock.GameDatetime = snap.GameDatetime
	resp.Clock.DayOfWeek = snap.DayOfWeek
	resp.Clock.Speed = snap.Speed
	resp.Clock.Paused = snap.Paused
	resp.Clock.OfficeOpen = snap.OfficeOpen
	resp.Clock.DaysUntilNextPayroll = snap.DaysUntilNextPayroll
	resp.Clock.DaysUntilNextRent = snap.DaysUntilNextRent
	return resp
}

// writeAPIError writes the canonical machine-readable error envelope (SPEC 14.1). details is
// included only when non-nil.
func writeAPIError(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]*apiError{"error": {Code: code, Message: message, Details: details}})
}

// writeMethodNotAllowed writes the canonical 405 METHOD_NOT_ALLOWED JSON error.
func writeMethodNotAllowed(w http.ResponseWriter) {
	writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed on this clock route", nil)
}

// readBody reads r.Body and returns the trimmed bytes of exactly one top-level JSON value, or a
// non-empty error code. allowEmpty permits a zero-length body (the skip endpoint). It rejects
// malformed JSON, trailing non-JSON content, and more than one JSON value as INVALID_JSON.
func readBody(r *http.Request, allowEmpty bool) ([]byte, string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, "INVALID_JSON"
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		if allowEmpty {
			return nil, "" // zero-length body accepted (skip)
		}
		return nil, "INVALID_JSON" // empty where JSON is required
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var first any
	if err := dec.Decode(&first); err != nil {
		return nil, "INVALID_JSON" // malformed
	}
	var second any
	err = dec.Decode(&second)
	if err == nil {
		return nil, "INVALID_JSON" // a second JSON value is present
	}
	if !errors.Is(err, io.EOF) {
		return nil, "INVALID_JSON" // trailing non-JSON content
	}
	return trimmed, ""
}

// decodeObject reports whether body is a JSON object and returns its fields as raw messages. A
// non-object (array/scalar) or invalid value yields ok=false.
func decodeObject(body []byte) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, false
	}
	return obj, true
}

// parseClockSpeed parses a JSON integer literal into an int. It rejects any value that is not a
// clean integer literal (strings, booleans, fractional/exponent numbers, objects, etc.).
func parseClockSpeed(raw json.RawMessage) (int, error) {
	s := strings.TrimSpace(string(raw))
	if !isIntegerLiteral(s) {
		return 0, errors.New("not an integer")
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// isIntegerLiteral reports whether s is a JSON integer literal: an optional leading '-' followed
// by one or more digits (no decimal point, exponent, quotes or other characters).
func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	if i >= len(s) {
		return false // just a "-"
	}
	for _, ch := range s[i:] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// parseBoolField parses a JSON boolean literal (true/false) into a bool.
func parseBoolField(raw json.RawMessage) (bool, error) {
	switch strings.TrimSpace(string(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New("not a boolean")
	}
}
