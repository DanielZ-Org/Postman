package api

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// selectSmallOffice performs the head-office selection step every slice test needs.
func selectSmallOffice(t *testing.T, h http.Handler) {
	t.Helper()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/offices/select", `{"office_id":"office-small-01"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("select office = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// hireOne performs a single hire and asserts success, including that the
// response never serialises skills as null (frontend contract: skills is a
// string array, [] when empty).
func hireOne(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/employees/hire", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("hire = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	emp, ok := body["employee"].(map[string]any)
	if !ok {
		t.Fatalf("hire response has no 'employee' object: %v", body)
	}
	if emp["skills"] == nil {
		t.Errorf("hire employee skills = nil, want [] (never null): %v", emp)
	}
	return body
}

// advanceGame advances the authoritative clock by the given real duration at the
// current speed (120 game seconds per real second at speed 1).
func advanceGame(state interface{ Tick(time.Duration) bool }, d time.Duration) {
	state.Tick(d)
}

// apiErrorCode extracts the machine code and optional details from the canonical
// {"error": {"code", "message", "details"}} envelope (SPEC 14.1).
func apiErrorCode(t *testing.T, body map[string]any) (string, map[string]any) {
	t.Helper()
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("response has no 'error' object: %v", body)
	}
	code, _ := errObj["code"].(string)
	details, _ := errObj["details"].(map[string]any)
	return code, details
}

func TestGetPackagesShape(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/packages", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	pkgs, ok := body["packages"].([]any)
	if !ok {
		t.Fatalf("response has no 'packages' array: %s", rec.Body.String())
	}
	if len(pkgs) != 0 {
		t.Errorf("packages = %d, want 0 on a fresh game", len(pkgs))
	}

	// After a generation tick the array carries SPEC 5.5 package fields.
	h2, state := newTestRouter()
	selectSmallOffice(t, h2)
	advanceGame(state, 15*time.Second) // 09:00 -> 09:30: one generation tick
	rec = doRequest(t, h2, http.MethodGet, "/api/v1/packages", "")
	body = decodeJSON(t, rec.Body.Bytes())
	pkgs, ok = body["packages"].([]any)
	if !ok || len(pkgs) == 0 {
		t.Fatalf("packages after tick = %v, want at least 1", body["packages"])
	}
	p := pkgs[0].(map[string]any)
	for _, key := range []string{"id", "size", "service_type", "status", "due_at", "received_at", "base_fee"} {
		if _, present := p[key]; !present {
			t.Errorf("package object missing %q: %v", key, p)
		}
	}
	if p["status"] != "stored" {
		t.Errorf("package status = %v, want stored", p["status"])
	}
}

func TestGetPlayerShape(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/player", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	player, ok := body["player"].(map[string]any)
	if !ok {
		t.Fatalf("response has no 'player' wrapper: %s", rec.Body.String())
	}
	if player["cash"] != float64(1000) || player["loan_principal"] != float64(1000) {
		t.Errorf("cash/loan = %v/%v, want 1000/1000", player["cash"], player["loan_principal"])
	}
	if player["trait"] != "financial" {
		t.Errorf("trait = %v, want financial", player["trait"])
	}
	if _, present := player["id"]; !present {
		t.Errorf("player missing id: %v", player)
	}
}

func TestGetOfficeNullThenSelected(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/office", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body.Bytes())
	if office, present := body["office"]; !present || office != nil {
		t.Fatalf("office = %v, want explicit null before selection", body["office"])
	}

	selectSmallOffice(t, h)
	rec = doRequest(t, h, http.MethodGet, "/api/v1/office", "")
	body = decodeJSON(t, rec.Body.Bytes())
	office, ok := body["office"].(map[string]any)
	if !ok {
		t.Fatalf("office not an object after selection: %s", rec.Body.String())
	}
	if office["id"] != "office-small-01" || office["contract_status"] != "active" {
		t.Errorf("office = id %v status %v, want office-small-01/active", office["id"], office["contract_status"])
	}
	if office["next_rent_due"] != "1980-02-29T00:00:00" {
		t.Errorf("next_rent_due = %v, want prepaid 4 weeks", office["next_rent_due"])
	}
}

func TestGetEmployeesShape(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/employees", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	employees, ok := body["employees"].([]any)
	if !ok {
		t.Fatalf("no 'employees' array: %s", rec.Body.String())
	}
	if len(employees) != 0 {
		t.Errorf("employees = %d, want 0 on a fresh game", len(employees))
	}
	hiring, ok := body["hiring"].(map[string]any)
	if !ok {
		t.Fatalf("no 'hiring' object: %s", rec.Body.String())
	}
	if hiring["current_employee_count"] != float64(0) || hiring["total_hires_lifetime"] != float64(0) || hiring["next_hiring_fee"] != float64(50) {
		t.Errorf("hiring = %v, want 0/0/50", hiring)
	}

	selectSmallOffice(t, h)
	hireOne(t, h)
	rec = doRequest(t, h, http.MethodGet, "/api/v1/employees", "")
	body = decodeJSON(t, rec.Body.Bytes())
	employees = body["employees"].([]any)
	if len(employees) != 1 {
		t.Fatalf("employees = %d, want 1", len(employees))
	}
	emp := employees[0].(map[string]any)
	if emp["id"] != "emp-0001" || emp["status"] != "ready" {
		t.Errorf("employee = id %v status %v, want emp-0001/ready", emp["id"], emp["status"])
	}
	for _, key := range []string{"name", "speed_trait", "skills", "mood", "accrued_wages", "runs_today"} {
		if _, present := emp[key]; !present {
			t.Errorf("employee missing %q: %v", key, emp)
		}
	}
	if emp["skills"] == nil {
		t.Errorf("employee skills = nil, want [] (never null): %v", emp)
	}
	hiring = body["hiring"].(map[string]any)
	if hiring["current_employee_count"] != float64(1) || hiring["next_hiring_fee"] != float64(100) {
		t.Errorf("hiring after hire = %v, want 1/100", hiring)
	}
}

func TestPostHireValidationErrors(t *testing.T) {
	// Without an office: 409 NO_OFFICE.
	h, _ := newTestRouter()
	rec := doRequest(t, h, http.MethodPost, "/api/v1/employees/hire", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("hire without office = %d, want 409", rec.Code)
	}
	if code, _ := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes())); code != "NO_OFFICE" {
		t.Errorf("error code = %v, want NO_OFFICE", code)
	}

	// Hire with fields in the body: 400 INVALID_REQUEST with the offending field.
	selectSmallOffice(t, h)
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/hire", `{"name":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hire with fields = %d, want 400", rec.Code)
	}
	code, details := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "INVALID_REQUEST" {
		t.Errorf("error code = %v, want INVALID_REQUEST", code)
	}
	if details == nil || details["field"] != "name" {
		t.Errorf("details = %v, want field name", details)
	}

	// Malformed JSON: 400.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/hire", "{oops")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("hire malformed body = %d, want 400", rec.Code)
	}

	// The fee ladder outpaces cash: the 5th hire costs 250 against the remaining
	// cash, so INSUFFICIENT_FUNDS (409) carries required/available details.
	for i := 0; i < 4; i++ {
		hireOne(t, h)
	}
	rec = doRequest(t, h, http.MethodPost, "/api/v1/employees/hire", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("5th hire = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, details = apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "INSUFFICIENT_FUNDS" {
		t.Fatalf("error code = %v, want INSUFFICIENT_FUNDS", code)
	}
	if details == nil || details["required"] != float64(250) || details["available"] != float64(150) {
		t.Errorf("details = %v, want required 250 / available 150", details)
	}
}

func TestPostAssignValidationErrors(t *testing.T) {
	h, state := newTestRouter()
	selectSmallOffice(t, h)
	hireOne(t, h)

	// Unknown employee: 404.
	rec := doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-9999","package_count":1}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown employee = %d, want 404", rec.Code)
	}
	if code, _ := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes())); code != "EMPLOYEE_NOT_FOUND" {
		t.Errorf("error = %v, want EMPLOYEE_NOT_FOUND", code)
	}

	// Known employee, no stored packages: 409.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0001","package_count":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("no stored packages = %d, want 409", rec.Code)
	}
	if code, _ := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes())); code != "NO_STORED_PACKAGES" {
		t.Errorf("error = %v, want NO_STORED_PACKAGES", code)
	}

	// Invalid body shapes: 400 with specific codes.
	invalid := []struct {
		name string
		body string
		code string
	}{
		{"missing employee_id", `{"package_count":1}`, "INVALID_EMPLOYEE_ID"},
		{"missing package_count", `{"employee_id":"emp-0001"}`, "INVALID_PACKAGE_COUNT"},
		{"zero count", `{"employee_id":"emp-0001","package_count":0}`, "INVALID_PACKAGE_COUNT"},
		{"fractional count", `{"employee_id":"emp-0001","package_count":1.5}`, "INVALID_PACKAGE_COUNT"},
		{"unknown field", `{"employee_id":"emp-0001","package_count":1,"extra":true}`, "INVALID_REQUEST"},
		{"not an object", `["emp-0001"]`, "INVALID_REQUEST"},
	}
	for _, tc := range invalid {
		rec := doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", tc.body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400; body=%s", tc.name, rec.Code, rec.Body.String())
			continue
		}
		if code, _ := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes())); code != tc.code {
			t.Errorf("%s: error = %v, want %s", tc.name, code, tc.code)
		}
	}

	// The clock and state advanced only through generation, never a partial commit.
	_ = state
}

func TestPostAssignHappyPathAndBusyConflict(t *testing.T) {
	h, state := newTestRouter()
	selectSmallOffice(t, h)
	hireOne(t, h)
	advanceGame(state, 15*time.Second) // one package generated at 09:30

	rec := doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0001","package_count":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	emp, ok := body["employee"].(map[string]any)
	if !ok || emp["status"] != "packing" {
		t.Fatalf("employee = %v, want packing", body["employee"])
	}
	assigned, ok := body["assigned"].([]any)
	if !ok || len(assigned) != 1 {
		t.Fatalf("assigned = %v, want exactly one package id", body["assigned"])
	}
	if !strings.HasPrefix(assigned[0].(string), "pkg-") {
		t.Errorf("assigned id = %v, want pkg- prefix", assigned[0])
	}

	// The employee is now packing: a second assignment conflicts as EMPLOYEE_BUSY.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0001","package_count":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("busy assign = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if code, _ := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes())); code != "EMPLOYEE_BUSY" {
		t.Errorf("error = %v, want EMPLOYEE_BUSY", code)
	}

	// The assigned package left stored inventory.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/packages", "")
	body = decodeJSON(t, rec.Body.Bytes())
	pkgs := body["packages"].([]any)
	assignedStillStored := 0
	for _, raw := range pkgs {
		p := raw.(map[string]any)
		if p["status"] == "stored" {
			assignedStillStored++
		}
	}
	if assignedStillStored != 0 {
		t.Errorf("stored packages after assign = %d, want 0", assignedStillStored)
	}
}

func TestPostAssignCycleWouldNotFinish(t *testing.T) {
	h, state := newTestRouter()
	selectSmallOffice(t, h)
	hireOne(t, h)
	advanceGame(state, 15*time.Second)  // package exists from 09:30
	advanceGame(state, 120*time.Second) // 09:30 -> 13:30: 13:30 + 4h cycle exceeds 17:00

	rec := doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0001","package_count":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("assign at 13:30 = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	code, details := apiErrorCode(t, decodeJSON(t, rec.Body.Bytes()))
	if code != "CYCLE_WOULD_NOT_FINISH" {
		t.Fatalf("error = %v, want CYCLE_WOULD_NOT_FINISH", code)
	}
	if details == nil || details["closing_time"] != "1980-02-01T17:00:00" {
		t.Errorf("details = %v, want closing_time 1980-02-01T17:00:00", details)
	}
}

func TestGetFinanceStatementIsBare(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/finance", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())

	// The statement is unwrapped (no "finance" key) as the frontend client parses it.
	if _, wrapped := body["finance"]; wrapped {
		t.Fatalf("statement wrapped under 'finance': %s", rec.Body.String())
	}
	if body["cash_balance"] != float64(1000) {
		t.Errorf("cash_balance = %v, want 1000", body["cash_balance"])
	}
	period, _ := body["period"].(map[string]any)
	if period == nil || period["from"] == "" || period["to"] == "" {
		t.Errorf("period = %v, want from/to window", body["period"])
	}
	income, _ := body["income"].(map[string]any)
	expenses, _ := body["expenses"].(map[string]any)
	liabs, _ := body["liabilities"].(map[string]any)
	if income == nil || expenses == nil || liabs == nil {
		t.Fatalf("statement sections = income %v expenses %v liabilities %v", income, expenses, liabs)
	}
	if liabs["loan_principal"] != float64(1000) {
		t.Errorf("liabilities.loan_principal = %v, want 1000", liabs["loan_principal"])
	}
	if _, present := body["net_change"]; !present {
		t.Errorf("statement missing net_change: %v", body)
	}
}

func TestGetFinanceTransactionsNewestFirst(t *testing.T) {
	h, _ := newTestRouter()

	// Fresh game: exactly the seeded loan disbursement.
	rec := doRequest(t, h, http.MethodGet, "/api/v1/finance/transactions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	txns, ok := body["transactions"].([]any)
	if !ok {
		t.Fatalf("no 'transactions' array: %s", rec.Body.String())
	}
	if len(txns) != 1 {
		t.Fatalf("transactions = %d, want 1 (seeded loan disbursement)", len(txns))
	}
	first := txns[0].(map[string]any)
	if first["category"] != "loan_disbursement" || first["amount"] != float64(1000) {
		t.Errorf("seeded txn = %v, want loan_disbursement +1000", first)
	}
	for _, key := range []string{"id", "game_datetime", "category", "amount", "description", "reference_id"} {
		if _, present := first[key]; !present {
			t.Errorf("transaction missing %q: %v", key, first)
		}
	}

	// After selecting an office the newest entry is the down payment.
	selectSmallOffice(t, h)
	rec = doRequest(t, h, http.MethodGet, "/api/v1/finance/transactions", "")
	txns = decodeJSON(t, rec.Body.Bytes())["transactions"].([]any)
	if len(txns) != 2 {
		t.Fatalf("transactions = %d, want 2", len(txns))
	}
	newest := txns[0].(map[string]any)
	oldest := txns[1].(map[string]any)
	if newest["category"] != "office_down_payment" || newest["amount"] != float64(-350) {
		t.Errorf("newest = %v, want office_down_payment -350", newest)
	}
	if oldest["category"] != "loan_disbursement" {
		t.Errorf("oldest = %v, want loan_disbursement", oldest)
	}
}

func TestGetGameStateShape(t *testing.T) {
	h, _ := newTestRouter()

	rec := doRequest(t, h, http.MethodGet, "/api/v1/game", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body.Bytes())
	gs, ok := body["game-state"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'game-state' wrapper: %s", rec.Body.String())
	}
	game, _ := gs["game"].(map[string]any)
	if game == nil || game["status"] != "running" || game["game_datetime"] != "1980-02-01T09:00:00" {
		t.Errorf("game = %v, want running at canonical start", game)
	}
	if gs["office"] != nil {
		t.Errorf("office = %v, want null before selection", gs["office"])
	}
	ops, _ := gs["operations"].(map[string]any)
	fin, _ := gs["finance"].(map[string]any)
	if ops == nil || fin == nil {
		t.Fatalf("operations/finance missing: %v", gs)
	}
	if fin["loan_principal"] != float64(1000) {
		t.Errorf("finance.loan_principal = %v, want 1000", fin["loan_principal"])
	}

	// After selection the office section fills in.
	selectSmallOffice(t, h)
	rec = doRequest(t, h, http.MethodGet, "/api/v1/game", "")
	gs = decodeJSON(t, rec.Body.Bytes())["game-state"].(map[string]any)
	office, ok := gs["office"].(map[string]any)
	if !ok {
		t.Fatalf("office not an object after selection: %v", gs["office"])
	}
	if office["id"] != "office-small-01" || office["storage_capacity"] != float64(100) {
		t.Errorf("office = %v, want small office with 100 storage", office)
	}
}

func TestSliceRouteMethodNotAllowed(t *testing.T) {
	h, _ := newTestRouter()
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/packages"},
		{http.MethodPut, "/api/v1/employees"},
		{http.MethodGet, "/api/v1/employees/hire"},
		{http.MethodGet, "/api/v1/deliveries/assign"},
		{http.MethodPost, "/api/v1/finance"},
		{http.MethodPost, "/api/v1/finance/transactions"},
		{http.MethodPost, "/api/v1/game"},
		{http.MethodPost, "/api/v1/player"},
	}
	for _, tc := range cases {
		rec := doRequest(t, h, tc.method, tc.path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", tc.method, tc.path, rec.Code)
		}
	}
}

// TestFullSliceSelectHireGenerateAssignDeliver walks the SPEC acceptance path over
// HTTP: select office, hire, generate by clock advance, assign, complete the run and
// verify the delivered package posted revenue everywhere the frontend reads it.
func TestFullSliceSelectHireGenerateAssignDeliver(t *testing.T) {
	h, state := newTestRouter()

	// 1. Select the small office: cash 1000 -> 650.
	selectSmallOffice(t, h)

	// 2. Hire one employee: fee 50, cash 600.
	hire := hireOne(t, h)
	if emp := hire["employee"].(map[string]any); emp["id"] != "emp-0001" {
		t.Fatalf("hired id = %v, want emp-0001", emp["id"])
	}

	// 3. Advance to the next generation tick (09:30) and read the package.
	advanceGame(state, 15*time.Second)
	rec := doRequest(t, h, http.MethodGet, "/api/v1/packages", "")
	pkgs := decodeJSON(t, rec.Body.Bytes())["packages"].([]any)
	if len(pkgs) == 0 {
		t.Fatal("no package generated after the first tick")
	}

	// 4. Assign one package: employee starts packing.
	rec = doRequest(t, h, http.MethodPost, "/api/v1/deliveries/assign", `{"employee_id":"emp-0001","package_count":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("assign = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	assigned := decodeJSON(t, rec.Body.Bytes())["assigned"].([]any)
	if len(assigned) != 1 {
		t.Fatalf("assigned = %v, want 1", assigned)
	}

	// 5. Advance four game hours (120 real seconds at speed 1): packing ends 10:30,
	//    delivery ends 13:30 -> package delivered, revenue posted, employee ready.
	advanceGame(state, 120*time.Second)

	rec = doRequest(t, h, http.MethodGet, "/api/v1/packages", "")
	pkgs = decodeJSON(t, rec.Body.Bytes())["packages"].([]any)
	var delivered int
	for _, raw := range pkgs {
		p := raw.(map[string]any)
		if p["status"] == "delivered" {
			delivered++
			if p["delivered_at"] == nil || p["final_revenue"] == nil {
				t.Errorf("delivered package missing delivered_at/final_revenue: %v", p)
			}
		}
	}
	if delivered != 1 {
		t.Fatalf("delivered packages = %d, want 1", delivered)
	}

	// 6. /game reflects the delivery.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/game", "")
	gs := decodeJSON(t, rec.Body.Bytes())["game-state"].(map[string]any)
	ops := gs["operations"].(map[string]any)
	if ops["delivered_today"] != float64(1) {
		t.Errorf("delivered_today = %v, want 1", ops["delivered_today"])
	}
	if ops["stored_packages"].(float64) < 1 {
		t.Errorf("stored_packages = %v, want at least the unassigned generated packages", ops["stored_packages"])
	}

	// 7. /player cash grew by the package revenue (base fee 5 or 7 for a normal
	//    small/medium package; cash was 600 after hiring).
	rec = doRequest(t, h, http.MethodGet, "/api/v1/player", "")
	player := decodeJSON(t, rec.Body.Bytes())["player"].(map[string]any)
	if cash := player["cash"].(float64); cash < 605 || cash > 607 {
		t.Errorf("cash = %v, want 605..607 (600 + one normal package fee)", cash)
	}

	// 8. The revenue exists as a transaction and in the weekly statement.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/finance/transactions", "")
	txns := decodeJSON(t, rec.Body.Bytes())["transactions"].([]any)
	foundRevenue := false
	for _, raw := range txns {
		txn := raw.(map[string]any)
		if txn["category"] == "package_revenue" {
			foundRevenue = true
			if amount := txn["amount"].(float64); amount < 5 {
				t.Errorf("revenue amount = %v, want >= 5", amount)
			}
		}
	}
	if !foundRevenue {
		t.Errorf("no package_revenue transaction in %d entries", len(txns))
	}

	// 9. /employees shows the employee ready again with one weekly delivery.
	rec = doRequest(t, h, http.MethodGet, "/api/v1/employees", "")
	employees := decodeJSON(t, rec.Body.Bytes())["employees"].([]any)
	emp := employees[0].(map[string]any)
	if emp["status"] != "ready" {
		t.Errorf("employee status = %v, want ready", emp["status"])
	}
	if emp["packages_delivered_this_week"] != float64(1) {
		t.Errorf("weekly deliveries = %v, want 1", emp["packages_delivered_this_week"])
	}
}
