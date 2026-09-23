package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// newTestRouter builds a fresh router with an isolated authoritative clock so each test
// starts from the canonical initial state.
func newTestRouter() (http.Handler, *game.Clock) {
	clock := game.NewClock()
	return NewRouter(clock), clock
}

// doRequest runs method+path against the handler with the given raw body and returns the recorder.
func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody=%s", err, body)
	}
	return m
}

func clockField(t *testing.T, body []byte) map[string]any {
	t.Helper()
	m := decodeJSON(t, body)
	clock, ok := m["clock"].(map[string]any)
	if !ok {
		t.Fatalf("response has no 'clock' wrapper: %s", body)
	}
	return clock
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	m := decodeJSON(t, body)
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no 'error' object: %s", body)
	}
	code, _ := errObj["code"].(string)
	return code
}

// errorCodeSafe returns the error code without failing, for use inside Fatalf messages.
func errorCodeSafe(rec *httptest.ResponseRecorder) string {
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		return "<not-json>"
	}
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		return "<no-error-object>"
	}
	code, _ := errObj["code"].(string)
	return code
}

func jsonInt(t *testing.T, m map[string]any, key string) int {
	t.Helper()
	v, ok := m[key].(float64)
	if !ok {
		t.Fatalf("field %q is not a JSON number: %v", key, m[key])
	}
	return int(v)
}

func jsonBool(t *testing.T, m map[string]any, key string) bool {
	t.Helper()
	v, ok := m[key].(bool)
	if !ok {
		t.Fatalf("field %q is not a JSON boolean: %v", key, m[key])
	}
	return v
}

// --- GET /api/v1/clock (regression) -------------------------------------------

func TestGetClockReturnsCanonicalShape(t *testing.T) {
	h, _ := newTestRouter()
	rec := doRequest(t, h, http.MethodGet, "/api/v1/clock", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /clock status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	clock := clockField(t, rec.Body.Bytes())
	if got := clock["game_datetime"]; got != "1980-02-01T09:00:00" {
		t.Errorf("game_datetime = %v, want the canonical start", got)
	}
	if got := clock["day_of_week"]; got != "friday" {
		t.Errorf("day_of_week = %v, want friday", got)
	}
	if got := jsonInt(t, clock, "speed"); got != 1 {
		t.Errorf("speed = %d, want 1", got)
	}
	if got := jsonBool(t, clock, "paused"); got {
		t.Error("paused = true, want false for a fresh clock")
	}
	if got := jsonBool(t, clock, "office_open"); !got {
		t.Error("office_open = false at the canonical start, want true")
	}
}

// --- POST /api/v1/clock/pause -------------------------------------------------

func TestPauseEndpoint(t *testing.T) {
	t.Run("valid true", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		clock := clockField(t, rec.Body.Bytes())
		if !jsonBool(t, clock, "paused") {
			t.Error("clock.paused = false after pause true, want true (updated wrapper)")
		}
	})

	t.Run("valid false", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":false}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		clock := clockField(t, rec.Body.Bytes())
		if jsonBool(t, clock, "paused") {
			t.Error("clock.paused = true after pause false, want false")
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{not json`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("missing field", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("wrong type string", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":"yes"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("wrong type number", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":1}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":true,"x":1}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("trailing JSON value", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":true}{"paused":false}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("success returns updated clock wrapper", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/pause", `{"paused":true}`)
		clock := clockField(t, rec.Body.Bytes()) // panics if no wrapper
		if got := clock["game_datetime"]; got != "1980-02-01T09:00:00" {
			t.Errorf("updated wrapper game_datetime = %v, want unchanged start", got)
		}
	})
}

// --- POST /api/v1/clock/speed -------------------------------------------------

func TestSpeedEndpoint(t *testing.T) {
	for _, speed := range []int{1, 2, 3} {
		t.Run("valid "+strconv.Itoa(speed), func(t *testing.T) {
			h, _ := newTestRouter()
			rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":`+strconv.Itoa(speed)+`}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			clock := clockField(t, rec.Body.Bytes())
			if got := jsonInt(t, clock, "speed"); got != speed {
				t.Errorf("clock.speed = %d after set %d, want %d", got, speed, speed)
			}
		})
	}

	for _, invalid := range []int{0, 4, -1} {
		t.Run("invalid integer "+strconv.Itoa(invalid), func(t *testing.T) {
			h, _ := newTestRouter()
			rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":`+strconv.Itoa(invalid)+`}`)
			if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_CLOCK_SPEED" {
				t.Fatalf("status=%d code=%s, want 400 INVALID_CLOCK_SPEED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
			}
		})
	}

	t.Run("malformed JSON", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{bad`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("missing field", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("wrong type string", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":"2"}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("wrong type float", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":2.5}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":1,"x":1}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("trailing JSON value", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":1}{"speed":2}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("rejected mutation leaves state unchanged", func(t *testing.T) {
		h, clock := newTestRouter()
		if err := clock.SetSpeed(2); err != nil {
			t.Fatalf("setup SetSpeed(2): %v", err)
		}
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{"speed":9}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_CLOCK_SPEED" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_CLOCK_SPEED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if got := clock.Snapshot().Speed; got != 2 {
			t.Errorf("speed = %d after rejected mutation, want previous value 2 preserved", got)
		}
	})
}

// --- POST /api/v1/clock/skip-to-next-opening ----------------------------------

func TestSkipEndpoint(t *testing.T) {
	t.Run("empty object no-op from open start", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/skip-to-next-opening", `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		clock := clockField(t, rec.Body.Bytes())
		// Canonical start (Friday 09:00) is open → skip is a no-op.
		if got := clock["game_datetime"]; got != "1980-02-01T09:00:00" {
			t.Errorf("skip while open moved time to %v, want unchanged start", got)
		}
	})

	t.Run("zero-length body accepted", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/skip-to-next-opening", ``)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 for zero-length body; body=%s", rec.Code, rec.Body.String())
		}
		clockField(t, rec.Body.Bytes()) // must still return the clock wrapper
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/skip-to-next-opening", `{"x":1}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_REQUEST; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("malformed body rejected", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/skip-to-next-opening", `{bad`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})

	t.Run("trailing JSON value rejected", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/skip-to-next-opening", `{}{}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_JSON" {
			t.Fatalf("status=%d code=%s, want 400 INVALID_JSON; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
	})
}

// --- Method handling ----------------------------------------------------------

func TestUnsupportedMethodsReturnJSONMethodNotAllowed(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/clock"},
		{http.MethodPut, "/api/v1/clock"},
		{http.MethodDelete, "/api/v1/clock"},
		{http.MethodGet, "/api/v1/clock/speed"},
		{http.MethodGet, "/api/v1/clock/pause"},
		{http.MethodGet, "/api/v1/clock/skip-to-next-opening"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			h, _ := newTestRouter()
			rec := doRequest(t, h, tc.method, tc.path, "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405; body=%s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json (no plain-text errors)", ct)
			}
			if code := errorCode(t, rec.Body.Bytes()); code != "METHOD_NOT_ALLOWED" {
				t.Errorf("error.code = %s, want METHOD_NOT_ALLOWED; body=%s", code, rec.Body.String())
			}
		})
	}
}

func TestMethodNotAllowedDoesNotMutateState(t *testing.T) {
	h, clock := newTestRouter()
	rec := doRequest(t, h, http.MethodGet, "/api/v1/clock/speed", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405; body=%s", rec.Code, rec.Body.String())
	}
	if got := clock.Snapshot().Speed; got != 1 {
		t.Errorf("speed = %d after a rejected method, want unchanged 1", got)
	}
}

// --- Error envelope shape -----------------------------------------------------

func TestErrorEnvelopeIsCanonicalJSON(t *testing.T) {
	h, _ := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/clock/speed", `{bad`)
	m := decodeJSON(t, rec.Body.Bytes())
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("no 'error' object in envelope: %s", rec.Body.String())
	}
	if _, hasCode := errObj["code"]; !hasCode {
		t.Error("error envelope missing 'code'")
	}
	if _, hasMsg := errObj["message"]; !hasMsg {
		t.Error("error envelope missing 'message'")
	}
}
