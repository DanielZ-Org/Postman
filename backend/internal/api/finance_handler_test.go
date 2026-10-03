package api

import (
	"net/http"
	"testing"
)

// TestRepayHappyPath verifies POST /api/v1/finance/repay (SPEC 11.1, decided 16.9/16.17):
// a valid amount reduces cash and principal by the repaid amount and echoes the updated
// player projection.
func TestRepayHappyPath(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodPost, "/api/v1/finance/repay", `{"amount":20000}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	if body["repaid_amount"] != float64(20000) {
		t.Errorf("repaid_amount = %v, want 20000", body["repaid_amount"])
	}
	player, ok := body["player"].(map[string]any)
	if !ok {
		t.Fatalf("response has no player object: %s", rec.Body.String())
	}
	if player["cash"] != float64(80000) || player["loan_principal"] != float64(80000) {
		t.Errorf("cash/principal = %v/%v, want 80000/80000", player["cash"], player["loan_principal"])
	}
}

// TestRepayRejectsInvalidAmounts verifies the strict body contract: GET is 405, and a
// missing, non-integer, extra or non-positive amount returns 400 INVALID_REQUEST with
// state unchanged.
func TestRepayRejectsInvalidAmounts(t *testing.T) {
	cases := []struct {
		name   string
		method string
		body   string
		code   string
		status int
	}{
		{"GET not allowed", http.MethodGet, "", "METHOD_NOT_ALLOWED", http.StatusMethodNotAllowed},
		{"missing amount", http.MethodPost, `{}`, "INVALID_REQUEST", http.StatusBadRequest},
		{"string amount", http.MethodPost, `{"amount":"20000"}`, "INVALID_REQUEST", http.StatusBadRequest},
		{"fractional amount", http.MethodPost, `{"amount":1.5}`, "INVALID_REQUEST", http.StatusBadRequest},
		{"extra field", http.MethodPost, `{"amount":100,"note":"x"}`, "INVALID_REQUEST", http.StatusBadRequest},
		{"zero amount", http.MethodPost, `{"amount":0}`, "INVALID_REQUEST", http.StatusBadRequest},
		{"negative amount", http.MethodPost, `{"amount":-1}`, "INVALID_REQUEST", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, state := newTestRouter()
			rec := doRequest(t, h, tc.method, "/api/v1/finance/repay", tc.body)
			if rec.Code != tc.status || errorCode(t, rec.Body.Bytes()) != tc.code {
				t.Fatalf("status/code = %d/%s, want %d/%s; body=%s",
					rec.Code, errorCodeSafe(rec), tc.status, tc.code, rec.Body.String())
			}
			if state.PlayerView().LoanPrincipal != 100000 || state.PlayerView().Cash != 100000 {
				t.Errorf("state mutated by rejected request: %+v", state.PlayerView())
			}
		})
	}
}

// TestRepayInsufficientFunds verifies an unaffordable amount returns 409
// INSUFFICIENT_FUNDS with required/available details and leaves state unchanged. The
// available amount is min(cash, principal).
func TestRepayInsufficientFunds(t *testing.T) {
	h, state := newTestRouter()

	rec := doRequest(t, h, http.MethodPost, "/api/v1/finance/repay", `{"amount":200000}`)
	if rec.Code != http.StatusConflict || errorCode(t, rec.Body.Bytes()) != "INSUFFICIENT_FUNDS" {
		t.Fatalf("status/code = %d/%s, want 409/INSUFFICIENT_FUNDS; body=%s",
			rec.Code, errorCodeSafe(rec), rec.Body.String())
	}
	details, _ := decodeJSON(t, rec.Body.Bytes())["error"].(map[string]any)["details"].(map[string]any)
	if details["required"] != float64(200000) || details["available"] != float64(100000) {
		t.Errorf("details = %v, want required 200000 available 100000", details)
	}
	if state.PlayerView().LoanPrincipal != 100000 || state.PlayerView().Cash != 100000 {
		t.Errorf("state mutated by rejected repayment: %+v", state.PlayerView())
	}
}
