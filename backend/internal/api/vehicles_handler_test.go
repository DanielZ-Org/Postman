package api

import (
	"net/http"
	"testing"
	"time"
)

// TestGetVehiclesProjection verifies the GET /api/v1/vehicles shape and values after
// a purchase, plus 405 for unsupported methods.
func TestGetVehiclesProjection(t *testing.T) {
	h, _ := newTestRouter()
	selectSmallOffice(t, h)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/vehicles/purchase", `{"vehicle":"bicycle"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("purchase = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, h, http.MethodGet, "/api/v1/vehicles", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /vehicles = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	if body["bicycles_owned"] != float64(1) || body["cars_owned"] != float64(0) ||
		body["bicycle_capacity"] != float64(5) || body["vehicle_capacity"] != float64(1) {
		t.Errorf("vehicles = %v, want owned 1/0 capacities 5/1 (small office)", body)
	}

	rec = doRequest(t, h, http.MethodPost, "/api/v1/vehicles", "{}")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /vehicles = %d, want 405", rec.Code)
	}
}

// TestVehiclePurchaseUnknownTypeRejected verifies an unknown vehicle type is rejected
// with INVALID_VEHICLE and leaves state unchanged (approved plan section 3.3).
func TestVehiclePurchaseUnknownTypeRejected(t *testing.T) {
	h, state := newTestRouter()
	selectSmallOffice(t, h)
	cashBefore := state.Cash()

	rec := doRequest(t, h, http.MethodPost, "/api/v1/vehicles/purchase", `{"vehicle":"drone"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown type = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	code, details := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "INVALID_VEHICLE" {
		t.Errorf("error code = %v, want INVALID_VEHICLE", code)
	}
	if allowed, ok := details["allowed"].([]any); !ok || len(allowed) != 2 || allowed[0] != "bicycle" || allowed[1] != "car" {
		t.Errorf("details.allowed = %v, want [bicycle car]", details["allowed"])
	}

	if got := state.Cash(); got != cashBefore {
		t.Errorf("cash = %d after rejected purchase, want unchanged %d", got, cashBefore)
	}
	rec = doRequest(t, h, http.MethodGet, "/api/v1/vehicles", "")
	body := decodeJSON(t, rec.Body.Bytes())
	if body["bicycles_owned"] != float64(0) || body["cars_owned"] != float64(0) {
		t.Errorf("owned = %v/%v after rejected purchase, want 0/0", body["bicycles_owned"], body["cars_owned"])
	}
}

// TestVehiclePurchaseCapacityAndFundsErrors verifies the capacity-limit and funds
// error envelopes end to end through the public API.
func TestVehiclePurchaseCapacityAndFundsErrors(t *testing.T) {
	h, _ := newTestRouter()
	selectSmallOffice(t, h) // cash 65000p

	// Buy all five bicycles (5 x 5000p = 25000p; affordable): cash drops to 40000p.
	for i := 0; i < 5; i++ {
		rec := doRequest(t, h, http.MethodPost, "/api/v1/vehicles/purchase", `{"vehicle":"bicycle"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("bicycle %d = %d, want 200; body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	rec := doRequest(t, h, http.MethodPost, "/api/v1/vehicles/purchase", `{"vehicle":"bicycle"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("6th bicycle = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, details := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "VEHICLE_CAPACITY_REACHED" || details["limit"] != float64(5) || details["owned"] != float64(5) {
		t.Errorf("capacity error = %v %v, want VEHICLE_CAPACITY_REACHED limit 5 owned 5", code, details)
	}

	// Cash is now 40000p, below the car price of 50000p.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/vehicles/purchase", `{"vehicle":"car"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unaffordable car = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, details = apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "INSUFFICIENT_FUNDS" || details["required"] != float64(50000) || details["available"] != float64(40000) {
		t.Errorf("funds error = %v %v, want INSUFFICIENT_FUNDS required 50000 available 40000", code, details)
	}
}

// TestEmployeeModeEndpoint verifies POST /api/v1/employees/{id}/mode end to end:
// skill gating with MISSING_SKILL details, successful switches, unknown employee,
// invalid mode value and busy rejection (approved plan section 3.3).
func TestEmployeeModeEndpoint(t *testing.T) {
	h, state := newTestRouter()
	selectSmallOffice(t, h)
	for i := 0; i < 3; i++ { // hire #1 no skills, #2 bicycle, #3 both
		hireOne(t, h)
	}

	// emp-0001 (no skills) -> car: MISSING_SKILL with details.
	rec := doRequest(t, h, http.MethodPost, "/api/v1/employees/emp-0001/mode", `{"mode":"car"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("car without licence = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, details := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "MISSING_SKILL" || details["employee_id"] != "emp-0001" || details["required_skill"] != "driving_licence" {
		t.Errorf("missing-skill error = %v %v, want MISSING_SKILL emp-0001 driving_licence", code, details)
	}

	// emp-0002 (bicycle skill) -> bicycle: success with the updated employee.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/emp-0002/mode", `{"mode":"bicycle"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bicycle switch = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	if body["current_delivery_mode"] != "bicycle" || body["id"] != "emp-0002" {
		t.Errorf("switched employee = %v, want emp-0002 in bicycle mode", body)
	}

	// Unknown employee: 404 EMPLOYEE_NOT_FOUND.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/emp-9999/mode", `{"mode":"foot"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown employee = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	code, _ = apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "EMPLOYEE_NOT_FOUND" {
		t.Errorf("error code = %v, want EMPLOYEE_NOT_FOUND", code)
	}

	// Invalid mode value: 400 INVALID_MODE with the allowed set.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/emp-0003/mode", `{"mode":"drone"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	code, details = apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "INVALID_MODE" {
		t.Errorf("error code = %v, want INVALID_MODE (allowed %v)", code, details["allowed"])
	}

	// Mid-run employee: 409 EMPLOYEE_BUSY. Generate a package and assign it to the
	// bicycle-mode emp-0002 so packing starts, then attempt a switch.
	advanceGame(state, 15*time.Second)
	rec = doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0002","package_count":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign for busy test = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/emp-0002/mode", `{"mode":"foot"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("busy switch = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, _ = apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "EMPLOYEE_BUSY" {
		t.Errorf("error code = %v, want EMPLOYEE_BUSY", code)
	}

	// Unsupported method: 405.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/employees/emp-0001/mode", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET mode path = %d, want 405", rec.Code)
	}

	// Malformed path: 400.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/a/b/mode", `{"mode":"foot"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed path = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
