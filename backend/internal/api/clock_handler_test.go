package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// clockBody mirrors the SPEC 14.3 wire shape for assertions in tests. The Clock
// field is a pointer so a missing/null wrapper can be distinguished from present.
type clockBody struct {
	Clock *struct {
		GameDatetime         string `json:"game_datetime"`
		DayOfWeek            string `json:"day_of_week"`
		Speed                int    `json:"speed"`
		Paused               bool   `json:"paused"`
		OfficeOpen           bool   `json:"office_open"`
		DaysUntilNextPayroll int    `json:"days_until_next_payroll"`
		DaysUntilNextRent    int    `json:"days_until_next_rent"`
	} `json:"clock"`
}

// doClockRequest builds a fresh router and issues one request to /api/v1/clock.
func doClockRequest(t *testing.T, method string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(game.NewClock())
	req := httptest.NewRequest(method, "/api/v1/clock", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGetClockReturns200(t *testing.T) {
	if w := doClockRequest(t, http.MethodGet); w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/clock = %d, want 200", w.Code)
	}
}

func TestGetClockContentTypeIsJSON(t *testing.T) {
	w := doClockRequest(t, http.MethodGet)
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}

func TestGetClockResponseWrapperIsExactlyClock(t *testing.T) {
	w := doClockRequest(t, http.MethodGet)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("top-level keys = %d, want exactly 1 (the 'clock' wrapper)", len(raw))
	}
	if _, ok := raw["clock"]; !ok {
		t.Fatalf("response missing the 'clock' wrapper; body=%s", w.Body.String())
	}
}

func TestGetClockFieldsMatchSpec(t *testing.T) {
	w := doClockRequest(t, http.MethodGet)

	var body clockBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Clock == nil {
		t.Fatal("response 'clock' wrapper is null")
	}
	c := *body.Clock

	if c.GameDatetime != "1980-02-01T09:00:00" {
		t.Errorf("game_datetime = %q, want %q", c.GameDatetime, "1980-02-01T09:00:00")
	}
	if c.DayOfWeek != "friday" {
		t.Errorf("day_of_week = %q, want %q", c.DayOfWeek, "friday")
	}
	if c.Speed != 1 {
		t.Errorf("speed = %d, want 1", c.Speed)
	}
	if c.Paused {
		t.Error("paused = true, want false")
	}
	if !c.OfficeOpen {
		t.Error("office_open = false, want true")
	}
	if c.DaysUntilNextPayroll != 4 {
		t.Errorf("days_until_next_payroll = %d, want 4", c.DaysUntilNextPayroll)
	}
	if c.DaysUntilNextRent != 0 {
		t.Errorf("days_until_next_rent = %d, want 0", c.DaysUntilNextRent)
	}
}

func TestRepeatedGetsDoNotChangeState(t *testing.T) {
	router := NewRouter(game.NewClock())

	first := serveGet(router)
	for i := 0; i < 3; i++ {
		if got := serveGet(router); got != first {
			t.Fatalf("response changed on repeated GET: %q vs %q", got, first)
		}
	}
}

func TestUnsupportedMethodDoesNotExecuteGETBehaviour(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		w := doClockRequest(t, method)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /api/v1/clock = %d, want 405", method, w.Code)
		}
		if strings.Contains(w.Body.String(), `"clock"`) {
			t.Fatalf("%s executed GET behaviour (returned clock data): %s", method, w.Body.String())
		}
	}
}

// serveGet issues a single GET /api/v1/clock against the given router and returns
// the response body. Used to verify repeated reads on one authoritative instance.
func serveGet(router http.Handler) string {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clock", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Body.String()
}
