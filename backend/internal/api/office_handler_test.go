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
	if small["type"] != "small" || small["down_payment"] != float64(35000) || small["weekly_rent"] != float64(5000) || small["rent_prepaid_weeks"] != float64(4) {
		t.Errorf("small core = type %v dp %v rent %v prepaid %v, want small/35000p/5000p/4", small["type"], small["down_payment"], small["weekly_rent"], small["rent_prepaid_weeks"])
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
// an exactly-{office, cash_balance} wrapper with the canonical runtime office shape and 65000p (£650) cash.
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
	if body.CashBalance != 65000 {
		t.Errorf("cash_balance = %v, want 65000p", body.CashBalance)
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

	if got := state.Cash(); got != 65000 {
		t.Errorf("authoritative cash = %d, want 65000p", got)
	}
	if off := state.SelectedOffice(); off == nil || off.ID != "office-small-01" {
		t.Errorf("authoritative selected office = %v, want office-small-01", off)
	}
}

// TestSelectLargeSucceeds verifies a successful Large selection via the API: 200 and 55000p (£550) cash.
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
	if body.CashBalance != 55000 {
		t.Errorf("cash_balance = %v, want 55000p", body.CashBalance)
	}
	if got := state.Cash(); got != 55000 {
		t.Errorf("authoritative cash = %d, want 55000p", got)
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
			if got := state.Cash(); got != 100000 {
				t.Errorf("cash = %d after rejected request, want unchanged 100000p", got)
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
	if got := state.Cash(); got != 100000 {
		t.Errorf("cash = %d after unknown-office request, want unchanged 100000p", got)
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
	if got := state.Cash(); got != 65000 {
		t.Errorf("cash = %d after rejected second selection, want unchanged 65000p", got)
	}
}

// TestSelectInsufficientFundsMapsTo409 verifies the HTTP-layer mapping of an insufficient-funds
// error to 409 INSUFFICIENT_FUNDS with required/available details. M1D has no public API that
// reduces cash below a down payment while unselected (starting 100000p covers both offices), so this
// code path is exercised directly through the handler's error mapping rather than via doRequest; the
// full domain behavior is covered by the game-layer tests.
func TestSelectInsufficientFundsMapsTo409(t *testing.T) {
	state := game.NewInitialState()
	oh := newOfficeHandler(state)

	w := httptest.NewRecorder()
	oh.writeSelectionError(w, &game.InsufficientFundsError{Required: 35000, Available: 30000})

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
	if body.Error.Details["required"] != float64(35000) || body.Error.Details["available"] != float64(30000) {
		t.Errorf("details = %v, want required 35000p available 30000p", body.Error.Details)
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
		if got := state.Cash(); got != 100000 {
			t.Errorf("cash = %d after rejected method, want unchanged 100000p", got)
		}
	})
}

// TestUpgradeEndpoint verifies POST /api/v1/offices/upgrade end to end (SPEC 4.4): a small
// office upgrades for 10000p (200 with the upgraded office + cash balance), a second upgrade
// returns 409 UPGRADE_NOT_AVAILABLE, and unsupported methods are rejected without mutation.
func TestUpgradeEndpoint(t *testing.T) {
	h, state := newTestRouter()
	if rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`); rec.Code != http.StatusOK {
		t.Fatalf("select small status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("upgrade status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Office struct {
			Type             string `json:"type"`
			EmployeeCapacity int    `json:"employee_capacity"`
			VehicleCapacity  int    `json:"vehicle_capacity"`
		} `json:"office"`
		CashBalance int `json:"cash_balance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v; body=%s", err, rec.Body.String())
	}
	if body.Office.Type != "large" || body.Office.EmployeeCapacity != 7 || body.Office.VehicleCapacity != 2 {
		t.Errorf("office = %+v, want large/7/2", body.Office)
	}
	if body.CashBalance != 55000 || state.Cash() != 55000 {
		t.Errorf("cash = %d/%d, want 55000p", body.CashBalance, state.Cash())
	}

	// Second upgrade: the active office is already large.
	again := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", "")
	if again.Code != http.StatusConflict || errorCode(t, again.Body.Bytes()) != "UPGRADE_NOT_AVAILABLE" {
		t.Fatalf("second upgrade = %d/%s, want 409 UPGRADE_NOT_AVAILABLE; body=%s",
			again.Code, errorCodeSafe(again), again.Body.String())
	}

	// Unsupported method without mutation.
	get := doRequest(t, h, http.MethodGet, "/api/v1/offices/upgrade", "")
	if get.Code != http.StatusMethodNotAllowed || errorCode(t, get.Body.Bytes()) != "METHOD_NOT_ALLOWED" {
		t.Errorf("GET upgrade = %d/%s, want 405 METHOD_NOT_ALLOWED", get.Code, errorCodeSafe(get))
	}
}

// TestUpgradeInvalidRequests verifies transport validation and the NO_OFFICE mapping: a body
// with fields is a 400 before any business rule, garbage is a 400, and calling without an
// active office returns 404 NO_OFFICE (SPEC 4.4 validation order).
func TestUpgradeInvalidRequests(t *testing.T) {
	t.Run("unexpected body field", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", `{"amount":100}`)
		if rec.Code != http.StatusBadRequest || errorCode(t, rec.Body.Bytes()) != "INVALID_REQUEST" {
			t.Fatalf("status/code = %d/%s, want 400 INVALID_REQUEST; body=%s",
				rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if state.Cash() != 100000 || state.SelectedOffice() != nil {
			t.Errorf("state mutated: cash %d office %v", state.Cash(), state.SelectedOffice())
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		h, _ := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", `not-json`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("no active office", func(t *testing.T) {
		h, state := newTestRouter()
		rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", "")
		if rec.Code != http.StatusNotFound || errorCode(t, rec.Body.Bytes()) != "NO_OFFICE" {
			t.Fatalf("status/code = %d/%s, want 404 NO_OFFICE; body=%s",
				rec.Code, errorCodeSafe(rec), rec.Body.String())
		}
		if state.Cash() != 100000 {
			t.Errorf("cash = %d, want unchanged", state.Cash())
		}
	})
}

// TestUpgradeErrorMapping verifies the canonical SPEC 4.4 error codes for all three failure
// kinds, including the required/available details on INSUFFICIENT_FUNDS.
func TestUpgradeErrorMapping(t *testing.T) {
	state := game.NewInitialState()
	oh := newOfficeHandler(state)

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"no active office", game.ErrNoActiveOffice, http.StatusNotFound, "NO_OFFICE"},
		{"already large", game.ErrUpgradeNotAvailable, http.StatusConflict, "UPGRADE_NOT_AVAILABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			oh.writeUpgradeError(w, tc.err)
			if w.Code != tc.status || errorCode(t, w.Body.Bytes()) != tc.code {
				t.Errorf("status/code = %d/%s, want %d/%s", w.Code, errorCodeSafe(w), tc.status, tc.code)
			}
		})
	}

	t.Run("insufficient funds details", func(t *testing.T) {
		w := httptest.NewRecorder()
		oh.writeUpgradeError(w, &game.InsufficientFundsError{Required: 10000, Available: 4000})
		if w.Code != http.StatusConflict || errorCode(t, w.Body.Bytes()) != "INSUFFICIENT_FUNDS" {
			t.Fatalf("status/code = %d/%s, want 409 INSUFFICIENT_FUNDS", w.Code, errorCodeSafe(w))
		}
		details, _ := decodeJSON(t, w.Body.Bytes())["error"].(map[string]any)["details"].(map[string]any)
		if details["required"] != float64(10000) || details["available"] != float64(4000) {
			t.Errorf("details = %v, want required 10000 available 4000", details)
		}
	})
}

// TestGetOfficesUpgradeCostPence verifies the UI field (SPEC 4.4): upgrade_cost_pence is 10000
// on every catalogue entry only while an active small head office exists, 0 otherwise.
func TestGetOfficesUpgradeCostPence(t *testing.T) {
	h, _ := newTestRouter()

	costs := func() []float64 {
		rec := doRequest(t, h, http.MethodGet, "/api/v1/offices", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /offices status = %d", rec.Code)
		}
		var body struct {
			Offices []map[string]any `json:"offices"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out := make([]float64, 0, len(body.Offices))
		for _, o := range body.Offices {
			out = append(out, o["upgrade_cost_pence"].(float64))
		}
		return out
	}

	if got := costs(); len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("initial costs = %v, want [0 0]", got)
	}

	if rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`); rec.Code != http.StatusOK {
		t.Fatalf("select = %d", rec.Code)
	}
	if got := costs(); got[0] != 10000 || got[1] != 10000 {
		t.Errorf("costs with active small = %v, want [10000 10000]", got)
	}

	if rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/upgrade", ""); rec.Code != http.StatusOK {
		t.Fatalf("upgrade = %d", rec.Code)
	}
	if got := costs(); got[0] != 0 || got[1] != 0 {
		t.Errorf("costs after upgrade = %v, want [0 0]", got)
	}
}
