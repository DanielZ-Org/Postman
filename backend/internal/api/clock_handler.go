package api

import (
	"encoding/json"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// clockResponse is the wire shape for GET /api/v1/clock, matching SPEC 14.3: a
// single top-level "clock" wrapper whose fields use the exact JSON names from
// the canonical contract.
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

// clockHandler serves the read-only GET /api/v1/clock endpoint. It holds an
// immutable copy of the authoritative clock and performs no game mutation or rule
// evaluation beyond reading a snapshot.
type clockHandler struct {
	clock game.Clock
}

func newClockHandler(clock game.Clock) *clockHandler {
	return &clockHandler{clock: clock}
}

// handle serves GET /api/v1/clock with the canonical "clock" wrapper. Unsupported
// methods are rejected (405) and never execute the read-only GET behaviour.
func (h *clockHandler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := h.clock.Snapshot()
	resp := clockResponse{}
	resp.Clock.GameDatetime = snap.GameDatetime
	resp.Clock.DayOfWeek = snap.DayOfWeek
	resp.Clock.Speed = snap.Speed
	resp.Clock.Paused = snap.Paused
	resp.Clock.OfficeOpen = snap.OfficeOpen
	resp.Clock.DaysUntilNextPayroll = snap.DaysUntilNextPayroll
	resp.Clock.DaysUntilNextRent = snap.DaysUntilNextRent

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Headers are already sent; nothing further can be done for this request.
	}
}
