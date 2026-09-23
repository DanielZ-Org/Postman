package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// TestGetOfficesReturnsCatalogue verifies GET /api/v1/offices: 200, JSON content type, a single
// "offices" wrapper containing exactly two choices in deterministic order with the canonical public
// catalogue fields and no runtime fields.
func TestGetOfficesReturnsCatalogue(t *testing.T) {
	h, _ := newTestRouter()
	rec := doRequest(t, h, http.MethodGet, "/api/v1/offices", "")

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
	if len(top) != 1 || top["offices"] == nil {
		t.Fatalf("wrapper keys = %d (has offices? %v), want exactly one key 'offices'", len(top), top["offices"] != nil)
	}

	var body struct {
		Offices []map[string]any `json:"offices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode offices: %v", err)
	}
	if len(body.Offices) != 2 {
		t.Fatalf("len(offices) = %d, want exactly 2", len(body.Offices))
	}
	if body.Offices[0]["id"] != "office-small-01" || body.Offices[1]["id"] != "office-large-01" {
		t.Errorf("order/ids = [%v, %v], want [office-small-01, office-large-01]", body.Offices[0]["id"], body.Offices[1]["id"])
	}

	small := body.Offices[0]
	if small["type"] != "small" || small["down_payment"] != float64(350) || small["weekly_rent"] != float64(50) || small["rent_prepaid_weeks"] != float64(4) {
		t.Errorf("small core = type %v dp %v rent %v prepaid %v, want small/350/50/4", small["type"], small["down_payment"], small["weekly_rent"], small["rent_prepaid_weeks"])
	}
	if emp := small["employee_capacity"]; emp != float64(5) {
		t.Errorf("small employee_capacity = %v, want 5", emp)
	}
	if bike := small["bicycle_capacity"]; bike != float64(5) {
		t.Errorf("small bicycle_capacity = %v, want 5", bike)
	}
	if veh := small["vehicle_capacity"]; veh != float64(1) {
		t.Errorf("small vehicle_capacity = %v, want 1", veh)
	}
	sizes, _ := small["accepted_package_sizes"].([]any)
	if len(sizes) != 2 || sizes[0] != "small" || sizes[1] != "medium" {
		t.Errorf("small accepted_package_sizes = %v, want [small medium]", small["accepted_package_sizes"])
	}
	stor, _ := small["storage"].(map[string]any)
	if stor == nil || stor["base"] != float64(100) || stor["max"] != float64(150) {
		t.Errorf("small storage = %v, want base 100 max 150", small["storage"])
	}

	// Catalogue objects must NOT contain runtime fields.
	for _, forbidden := range []string{"is_head_office", "next_rent_due", "contract_status", "missed_rent_payments"} {
		if _, present := small[forbidden]; present {
			t.Errorf("catalogue object contains runtime field %q, want absent", forbidden)
		}
	}
	for _, forbidden := range []string{"current", "used"} {
		if _, present := stor[forbidden]; present {
			t.Errorf("catalogue storage contains runtime field %q, want only base/max", forbidden)
		}
	}
}

// TestSelectSmallSucceeds verifies a successful Small selection via the API: 200, JSON content type,
// an exactly-{office, cash_balance} wrapper with the canonical runtime office shape and £650 cash.
func TestSelectSmallSucceeds(t *testing.T) {
	h, state := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`)

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
	if len(top) != 2 || top["office"] == nil || top["cash_balance"] == nil {
		t.Fatalf("wrapper keys = %d (office? %v cash_balance? %v), want exactly {office, cash_balance}", len(top), top["office"] != nil, top["cash_balance"] != nil)
	}

	var body struct {
		Office      map[string]any `json:"office"`
		CashBalance float64        `json:"cash_balance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode select response: %v", err)
	}
	if body.CashBalance != 650 {
		t.Errorf("cash_balance = %v, want 650", body.CashBalance)
	}
	o := body.Office
	if o["id"] != "office-small-01" || o["is_head_office"] != true || o["type"] != "small" {
		t.Errorf("selected office id/head/type = %v/%v/%v, want office-small-01/true/small", o["id"], o["is_head_office"], o["type"])
	}
	if o["contract_status"] != "active" || o["missed_rent_payments"] != float64(0) {
		t.Errorf("contract status/missed rent = %v/%v, want active/0", o["contract_status"], o["missed_rent_payments"])
	}
	if o["next_rent_due"] != "1980-02-29T00:00:00" {
		t.Errorf("next_rent_due = %v, want 1980-02-29T00:00:00", o["next_rent_due"])
	}
	stor, _ := o["storage"].(map[string]any)
	if stor == nil || stor["base"] != float64(100) || stor["current"] != float64(100) || stor["max"] != float64(150) || stor["used"] != float64(0) {
		t.Errorf("storage = %v, want base/current/max/used 100/100/150/0", o["storage"])
	}

	if got := state.Cash(); got != 650 {
		t.Errorf("authoritative cash = %d, want 650", got)
	}
	if off := state.SelectedOffice(); off == nil || off.ID != "office-small-01" {
		t.Errorf("authoritative selected office = %v, want office-small-01", off)
	}
}

// TestSelectLargeSucceeds verifies a successful Large selection via the API: 200 and £550 cash.
func TestSelectLargeSucceeds(t *testing.T) {
	h, state := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-large-01"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		CashBalance float64 `json:"cash_balance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode select response: %v; body=%s", err, rec.Body.String())
	}
	if body.CashBalance != 550 {
		t.Errorf("cash_balance = %v, want 550", body.CashBalance)
	}
	if got := state.Cash(); got != 550 {
		t.Errorf("authoritative cash = %d, want 550", got)
	}
}

// TestSelectInvalidRequests verifies each transport-level rejection returns 400 with the correct code:
// malformed JSON, missing office_id, wrong type, empty string, unknown field, and a trailing value.
func TestSelectInvalidRequests(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"malformed JSON", `{not json`, "INVALID_JSON"},
		{"missing office_id", `{}`, "INVALID_REQUEST"},
		{"wrong type (number)", `{"office_id":123}`, "INVALID_REQUEST"},
		{"empty string", `{"office_id":""}`, "INVALID_REQUEST"},
		{"unknown field", `{"office_id":"office-small-01","extra":1}`, "INVALID_REQUEST"},
		{"trailing JSON value", `{"office_id":"office-small-01"} extra`, "INVALID_JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, state := newTestRouter()
			rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if got := errorCode(t, rec.Body.Bytes()); got != tc.wantCode {
				t.Errorf("code = %q, want %q; body=%s", got, tc.wantCode, rec.Body.String())
			}
			// Rejected requests must not mutate authoritative state.
			if off := state.SelectedOffice(); off != nil {
				t.Errorf("selected office = %v after rejected request, want nil", off.ID)
			}
			if got := state.Cash(); got != 1000 {
				t.Errorf("cash = %d after rejected request, want unchanged 1000", got)
			}
		})
	}
}

// TestSelectUnknownOffice404 verifies an unknown (but syntactically valid) office_id returns 404
// OFFICE_NOT_FOUND and leaves state unchanged.
func TestSelectUnknownOffice404(t *testing.T) {
	h, state := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-bogus-99"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec.Body.Bytes()); got != "OFFICE_NOT_FOUND" {
		t.Errorf("code = %q, want OFFICE_NOT_FOUND; body=%s", got, rec.Body.String())
	}
	if off := state.SelectedOffice(); off != nil {
		t.Errorf("selected office = %v after unknown-office request, want nil", off.ID)
	}
	if got := state.Cash(); got != 1000 {
		t.Errorf("cash = %d after unknown-office request, want unchanged 1000", got)
	}
}

// TestSelectAlreadySelected409 verifies that once an office is selected, any further selection
// returns 409 OFFICE_ALREADY_SELECTED and leaves state unchanged.
func TestSelectAlreadySelected409(t *testing.T) {
	h, state := newTestRouter()
	if rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`); rec.Code != http.StatusOK {
		t.Fatalf("setup select small: status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-large-01"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if got := errorCode(t, rec.Body.Bytes()); got != "OFFICE_ALREADY_SELECTED" {
		t.Errorf("code = %q, want OFFICE_ALREADY_SELECTED; body=%s", got, rec.Body.String())
	}
	if off := state.SelectedOffice(); off == nil || off.ID != "office-small-01" {
		t.Errorf("selected office = %v after second selection, want unchanged office-small-01", off)
	}
	if got := state.Cash(); got != 650 {
		t.Errorf("cash = %d after rejected second selection, want unchanged 650", got)
	}
}

// TestSelectInsufficientFundsMapsTo409 verifies the HTTP-layer mapping of an insufficient-funds
// error to 409 INSUFFICIENT_FUNDS with required/available details. M1D has no public API that
// reduces cash below a down payment while unselected (starting £1000 covers both offices), so this
// code path is exercised directly through the handler's error mapping rather than via doRequest; the
// full domain behavior is covered by the game-layer tests.
func TestSelectInsufficientFundsMapsTo409(t *testing.T) {
	state := game.NewInitialState()
	oh := newOfficeHandler(state)

	w := httptest.NewRecorder()
	oh.writeSelectionError(w, &game.InsufficientFundsError{Required: 350, Available: 300})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v; body=%s", err, w.Body.String())
	}
	if body.Error.Code != "INSUFFICIENT_FUNDS" {
		t.Errorf("code = %q, want INSUFFICIENT_FUNDS", body.Error.Code)
	}
	if body.Error.Details["required"] != float64(350) || body.Error.Details["available"] != float64(300) {
		t.Errorf("details = %v, want required 350 available 300", body.Error.Details)
	}
}

// TestOfficesUnsupportedMethods verifies unsupported methods on the Office routes return 405 with a
// JSON METHOD_NOT_ALLOWED error and do not mutate state.
func TestOfficesUnsupportedMethods(t *testing.T) {
	t.Run("POST /offices rejected", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/offices", `{"office_id":"office-small-01"}`)
		if rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
			t.Fatalf("status=%d code=%s, want 405 METHOD_NOT_ALLOWED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if off := state.SelectedOffice(); off != nil {
			t.Errorf("selected office = %v after rejected method, want nil", off.ID)
		}
	})

	t.Run("GET /offices/select rejected", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodGet, "/api/v1/offices/select", "")
		if rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
			t.Fatalf("status=%d code=%s, want 405 METHOD_NOT_ALLOWED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if off := state.SelectedOffice(); off != nil {
			t.Errorf("selected office = %v after rejected method, want nil", off.ID)
		}
	})

	t.Run("DELETE /offices/select rejected without mutation", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodDelete, "/api/v1/offices/select", "")
		if rec.Code != http.StatusMethodNotAllowed || errorCode(t, rec.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
			t.Fatalf("status=%d code=%s, want 405 METHOD_NOT_ALLOWED; body=%s", rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if got := state.Cash(); got != 1000 {
			t.Errorf("cash = %d after rejected method, want unchanged 1000", got)
		}
	})
}
