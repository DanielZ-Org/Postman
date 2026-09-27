package game

import (
	"errors"
	"testing"
)

func TestHireFeeLadderAndCapacity(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s) // cash 100000p - 35000p = 65000p
	s.cash = 1000000  // isolate the fee ladder from affordability (£10000 in pence)

	// Five hires: fees 5000, 10000, 15000, 20000, 25000p (SPEC 8 lifetime counter).
	wantFees := []int{5000, 10000, 15000, 20000, 25000}
	cash := s.cash
	for i, fee := range wantFees {
		emp, _, err := s.HireEmployee()
		if err != nil {
			t.Fatalf("hire %d: %v", i+1, err)
		}
		wantID := employeeID(i + 1)
		if emp.ID != wantID {
			t.Errorf("hire %d id = %s, want %s", i+1, emp.ID, wantID)
		}
		if emp.Status != EmployeeReady {
			t.Errorf("hire %d status = %s, want ready", i+1, emp.Status)
		}
		cash -= fee
		if s.cash != cash {
			t.Fatalf("hire %d cash = %d, want %d (fee %d)", i+1, s.cash, cash, fee)
		}
	}
	if s.totalHires != 5 {
		t.Errorf("totalHires = %d, want 5", s.totalHires)
	}

	// Office capacity (small = 5): sixth hire fails regardless of cash.
	if _, _, err := s.HireEmployee(); !errors.Is(err, ErrOfficeFull) {
		t.Fatalf("sixth hire err = %v, want ErrOfficeFull", err)
	}
}

func TestHireRequiresOfficeAndCash(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	if _, _, err := s.HireEmployee(); !errors.Is(err, ErrNoOffice) {
		t.Fatalf("hire without office err = %v, want ErrNoOffice", err)
	}

	selectSmall(t, s)
	s.cash = 4000 // fee is 5000p
	_, _, err := s.HireEmployee()
	var ife *InsufficientFundsError
	if !errors.As(err, &ife) || ife.Required != 5000 || ife.Available != 4000 {
		t.Fatalf("hire without cash err = %v, want InsufficientFundsError{5000p, 4000p}", err)
	}
	if s.cash != 4000 {
		t.Errorf("cash changed to %d, want unchanged", s.cash)
	}
	if s.totalHires != 0 || len(s.employees) != 0 {
		t.Errorf("failed hire mutated state: hires=%d employees=%d", s.totalHires, len(s.employees))
	}
}

func TestHireFailsWhenNotRunning(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	s.status = GameStatusGameOver
	if _, _, err := s.HireEmployee(); !errors.Is(err, ErrGameOver) {
		t.Fatalf("hire after game over err = %v, want ErrGameOver", err)
	}
}

func TestHirePostsHiringBonusTransaction(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	before := len(s.transactions)
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	if len(s.transactions) != before+1 {
		t.Fatalf("transactions = %d, want %d (one hiring_bonus)", len(s.transactions), before+1)
	}
	tr := s.transactions[len(s.transactions)-1]
	if tr.Category != CategoryHiringBonus || tr.Amount != -5000 || tr.Description == "" {
		t.Errorf("transaction = %+v, want hiring_bonus -5000p with description", tr)
	}
}

// newAssignedOfficeState builds a state with an active small office, one ready
// employee and the clock at the canonical opening instant.
func newAssignedOfficeState(t *testing.T) *GameState {
	t.Helper()
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	return s
}

func TestAssignValidations(t *testing.T) {
	s := newAssignedOfficeState(t)

	// Unknown employee.
	if _, _, err := s.AssignDelivery("emp-9999", 1); !errors.Is(err, ErrEmployeeNotFound) {
		t.Errorf("unknown employee err = %v, want ErrEmployeeNotFound", err)
	}

	// Known employee but empty storage.
	if _, _, err := s.AssignDelivery("emp-0001", 1); !errors.Is(err, ErrNoStoredPackages) {
		t.Errorf("no packages err = %v, want ErrNoStoredPackages", err)
	}

	// Requested count larger than stored -> capacity error (SPEC 14.5).
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	_, _, err := s.AssignDelivery("emp-0001", 3)
	var capErr *CapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("over-count err = %T (%v), want *CapacityError", err, err)
	}
	if capErr.Available != 10 || capErr.Requested != 1 || capErr.StoredAvailable != 1 || capErr.EmployeeID != "emp-0001" {
		t.Errorf("capacity error = %+v, want 10/1/1/emp-0001", capErr)
	}

	// Batch units beyond foot capacity: express packages cost 2 capacity units each
	// (SPEC 8), so 6 express = 12 requested of 10 available. They carry the earliest
	// due dates so canonical priority always includes them in the batch.
	for i := 2; i <= 7; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceExpress, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-03T08:00:00"))
	}
	_, _, err = s.AssignDelivery("emp-0001", 7)
	capErr = nil
	if !errors.As(err, &capErr) {
		t.Fatalf("over-units err = %T (%v), want *CapacityError", err, err)
	}
	if capErr.Requested != 13 || capErr.Available != 10 {
		t.Errorf("capacity = requested %d / available %d, want 13/10 (6 express + 1 small)", capErr.Requested, capErr.Available)
	}

	// First real assignment succeeds; the second hits the busy employee.
	if _, _, err := s.AssignDelivery("emp-0001", 1); err != nil {
		t.Fatalf("first assign: %v", err)
	}
	if _, _, err := s.AssignDelivery("emp-0001", 1); !errors.Is(err, ErrEmployeeBusy) {
		t.Errorf("busy err = %v, want ErrEmployeeBusy", err)
	}
}

func TestAssignSelectionPriorityExpressEarliest(t *testing.T) {
	s := newAssignedOfficeState(t)
	// Mixed storage: normal with later due, express with late due, normal earliest.
	addStoredPackage(s, "pkg-000001", "medium", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))
	pExpr := addStoredPackage(s, "pkg-000002", "small", ServiceExpress, mustParseTime(t, "1980-02-01T08:30:00"), mustParseTime(t, "1980-02-03T08:30:00"))
	pEarly := addStoredPackage(s, "pkg-000003", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:15:00"), mustParseTime(t, "1980-02-02T08:15:00"))

	_, ids, err := s.AssignDelivery("emp-0001", 3)
	if err != nil {
		t.Fatalf("AssignDelivery: %v", err)
	}
	want := []string{pExpr.ID, pEarly.ID, "pkg-000001"} // express first, then earliest due
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("selected order = %v, want %v (SPEC 16.4: express, then earliest due)", ids, want)
	}
}

func TestAssignRunsPerDayLimit(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))

	// Run 1: 09:00 -> pack 10:00 -> deliver 13:00.
	if _, _, err := s.AssignDelivery("emp-0001", 1); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T10:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00"))
	if s.employees[0].Status != EmployeeReady {
		t.Fatalf("employee status = %s, want ready", s.employees[0].Status)
	}

	// Run 2: 13:00 -> pack 14:00 -> deliver 17:00 (exactly at closing, allowed).
	if _, _, err := s.AssignDelivery("emp-0001", 1); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T14:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-01T17:00:00"))
	if s.employees[0].Status != EmployeeReady {
		t.Fatalf("employee status = %s, want ready", s.employees[0].Status)
	}
	if s.employees[0].RunsToday != 2 {
		t.Fatalf("runs_today = %d, want 2", s.employees[0].RunsToday)
	}

	// Run 3 same day: rejected (SPEC 9.3: two completed cycles per day).
	if _, _, err := s.AssignDelivery("emp-0001", 1); !errors.Is(err, ErrRunsLimitReached) {
		t.Fatalf("run 3 err = %v, want ErrRunsLimitReached", err)
	}
}

func TestAssignCycleWouldNotFinish(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))

	// 14:00 + 4h = 18:00 > 17:00 closing: cycle would not finish (SPEC 14.5).
	s.Clock.restore(mustParseTime(t, "1980-02-01T14:00:00"), 1, false)
	_, _, err := s.AssignDelivery("emp-0001", 1)
	var cycErr *CycleError
	if !errors.As(err, &cycErr) {
		t.Fatalf("late assign err = %T (%v), want *CycleError", err, err)
	}
	if cycErr.RequestedStart != "1980-02-01T14:00:00" || cycErr.ClosingTime != "1980-02-01T17:00:00" {
		t.Errorf("cycle error = %+v, want start 14:00 / closing 17:00", cycErr)
	}

	// Sunday: no opening today, cycle cannot finish (SPEC 6: closed all day).
	s.Clock.restore(mustParseTime(t, "1980-02-03T10:00:00"), 1, false)
	_, _, err = s.AssignDelivery("emp-0001", 1)
	cycErr = nil
	if !errors.As(err, &cycErr) {
		t.Fatalf("sunday assign err = %T (%v), want *CycleError", err, err)
	}
	if cycErr.ClosingTime != "" {
		t.Errorf("sunday closing = %q, want empty (office closed)", cycErr.ClosingTime)
	}
	if s.employees[0].Status != EmployeeReady {
		t.Errorf("employee status = %s, want ready (no run started)", s.employees[0].Status)
	}
}

func TestAssignFailsWhenNotRunning(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))
	s.status = GameStatusGameOver
	if _, _, err := s.AssignDelivery("emp-0001", 1); !errors.Is(err, ErrGameOver) {
		t.Fatalf("assign after game over err = %v, want ErrGameOver", err)
	}
}

func TestHiringStateShape(t *testing.T) {
	s := newAssignedOfficeState(t)
	hiring := s.hiringStateLocked()
	if hiring.CurrentEmployeeCount != 1 {
		t.Errorf("current_employee_count = %d, want 1", hiring.CurrentEmployeeCount)
	}
	if hiring.TotalHiresLifetime != 1 {
		t.Errorf("total_hires_lifetime = %d, want 1", hiring.TotalHiresLifetime)
	}
	if hiring.NextHiringFee != 10000 {
		t.Errorf("next_hiring_fee = %d, want 10000p (fee ladder)", hiring.NextHiringFee)
	}
}

func TestEmployeesViewShape(t *testing.T) {
	s := newAssignedOfficeState(t)
	emp, hiring := s.EmployeesView()
	if len(emp) != 1 || emp[0].ID != "emp-0001" {
		t.Fatalf("employees = %+v, want one emp-0001", emp)
	}
	if hiring.CurrentEmployeeCount != 1 || hiring.NextHiringFee != 10000 {
		t.Errorf("hiring = %+v, want 1 employee / fee 10000p", hiring)
	}
	// View mutation must not leak into state (plain values returned).
	emp[0].Status = "mutated"
	if s.employees[0].Status == "mutated" {
		t.Error("view mutation leaked into employee state")
	}
}

func TestCapacityErrorDetailsShape(t *testing.T) {
	s := newAssignedOfficeState(t)
	// Six express packages = 12 capacity units > 10 usable on foot (SPEC 8).
	for i := 1; i <= 6; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceExpress, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-03T08:00:00"))
	}
	_, _, err := s.AssignDelivery("emp-0001", 6)
	var capErr *CapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("err = %T (%v), want *CapacityError", err, err)
	}
	if capErr.EmployeeID != "emp-0001" || capErr.Available != 10 || capErr.Requested != 12 || capErr.StoredAvailable != 6 {
		t.Errorf("capacity error = %+v, want emp-0001/10/12/6", capErr)
	}
	// Failed validation must leave every package stored.
	for _, p := range s.packages {
		if p.Status != PackageStored {
			t.Errorf("package %s status = %s after failed assign, want stored", p.ID, p.Status)
		}
	}
}

func TestGameViewMatchesState(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))

	view := s.GameView()
	if view.Status != GameStatusRunning || view.GameDatetime != "1980-02-01T09:00:00" {
		t.Errorf("status/datetime = %s/%s", view.Status, view.GameDatetime)
	}
	if !view.HasOffice || view.OfficeID != "office-small-01" {
		t.Errorf("office = %v/%s, want active small office", view.HasOffice, view.OfficeID)
	}
	if view.Cash != s.cash || view.StoredPackages != 1 || view.EmployeeCount != 1 {
		t.Errorf("cash/stored/employees = %d/%d/%d, want %d/1/1", view.Cash, view.StoredPackages, view.EmployeeCount, s.cash)
	}
	if view.StorageCapacity != 100 || view.EmployeeCapacity != 5 {
		t.Errorf("storage/employee capacity = %d/%d, want 100/5", view.StorageCapacity, view.EmployeeCapacity)
	}
	// Mutating the view must not affect state (plain-value aggregate).
	view.Cash = -1
	view.StoredPackages = 99
	if s.cash < 0 {
		t.Error("view mutation leaked into state cash")
	}
}

// newCarModeState builds a state with an active small office and a ready employee
// switched to car mode. Hire #3 carries the driving licence (SPEC 7.2 hire cycle),
// which is the skill gate for car mode.
func newCarModeState(t *testing.T) (*GameState, string) {
	t.Helper()
	s := newAssignedOfficeState(t)                 // emp-0001: no skills
	if _, _, err := s.HireEmployee(); err != nil { // emp-0002: bicycle skill
		t.Fatalf("hire #2: %v", err)
	}
	if _, _, err := s.HireEmployee(); err != nil { // emp-0003: bicycle + driving licence
		t.Fatalf("hire #3: %v", err)
	}
	emp, err := s.SetEmployeeMode("emp-0003", ModeCar)
	if err != nil || emp.CurrentDeliveryMode != ModeCar {
		t.Fatalf("SetEmployeeMode(car) = %v/%s, want success in car mode", err, emp.CurrentDeliveryMode)
	}
	return s, "emp-0003"
}

// TestCarModeCapacityFiftyUnits verifies the car-mode capacity of 50 units per run
// (SPEC 9.2): a batch of 50 normal packages fits exactly, express packages consume
// two units each so 26 express overflow with the mode-specific error details, and
// 25 express fit exactly.
func TestCarModeCapacityFiftyUnits(t *testing.T) {
	s, carEmp := newCarModeState(t)

	// Run 1: 50 normal packages = exactly 50 capacity units (SPEC 9.2).
	for i := 1; i <= 50; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	}
	if _, ids, err := s.AssignDelivery(carEmp, 50); err != nil || len(ids) != 50 {
		t.Fatalf("car assign of 50 normal = %v/%d ids, want success with 50", err, len(ids))
	}
	// Advance run phases only (no package generation), so the stored inventory stays
	// exact for the capacity assertions below.
	s.processRunsLocked(mustParseTime(t, "1980-02-01T10:00:00")) // packing ends
	s.processRunsLocked(mustParseTime(t, "1980-02-01T13:00:00")) // delivery ends

	// Express double-consumption (SPEC 9.2): 26 express = 52 units > 50 available.
	for i := 51; i <= 76; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceExpress, mustParseTime(t, "1980-02-01T08:30:00"), mustParseTime(t, "1980-02-03T08:30:00"))
	}
	_, _, err := s.AssignDelivery(carEmp, 26)
	var capErr *CapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("car over-units err = %T (%v), want *CapacityError", err, err)
	}
	if capErr.Available != carDeliveryCapacity || capErr.Requested != 52 || capErr.StoredAvailable != 26 {
		t.Errorf("capacity error = %+v, want available %d / requested 52 / stored 26", capErr, carDeliveryCapacity)
	}

	// 25 express = exactly 50 units: fits (second run of the day).
	if _, ids, err := s.AssignDelivery(carEmp, 25); err != nil || len(ids) != 25 {
		t.Fatalf("car assign of 25 express = %v/%d ids, want success with 25", err, len(ids))
	}
}

// TestCarModeRunsPerDayLimitAndRollover verifies the car-mode budget of two local
// runs per working day (SPEC 9.3): a third same-day run is rejected and the budget
// resets on the next game date so a fresh run succeeds. Far runs stay deferred
// (SPEC 16.11), so car runs consume the same local daily slots as foot.
func TestCarModeRunsPerDayLimitAndRollover(t *testing.T) {
	s, carEmp := newCarModeState(t)
	for i := 1; i <= 3; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	}

	// Run 1: 09:00 -> pack 10:00 -> deliver 13:00.
	if _, _, err := s.AssignDelivery(carEmp, 1); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T10:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00"))

	// Run 2: 13:00 -> pack 14:00 -> deliver 17:00 (exactly at closing, allowed).
	if _, _, err := s.AssignDelivery(carEmp, 1); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T14:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-01T17:00:00"))
	if s.employees[2].RunsToday != 2 {
		t.Fatalf("runs_today = %d, want 2", s.employees[2].RunsToday)
	}

	// Run 3 same day: rejected (SPEC 9.3).
	if _, _, err := s.AssignDelivery(carEmp, 1); !errors.Is(err, ErrRunsLimitReached) {
		t.Fatalf("run 3 err = %v, want ErrRunsLimitReached", err)
	}

	// Day rollover (SPEC 9.3): jump the clock to Monday opening and let the
	// simulation process the day change, which resets the run budget.
	s.Clock.restore(mustParseTime(t, "1980-02-04T09:00:00"), 1, false)
	s.processLocked(s.Clock.Now())
	if s.employees[2].RunsToday != 0 {
		t.Fatalf("runs_today after rollover = %d, want 0", s.employees[2].RunsToday)
	}

	// A fresh run succeeds on the new day (Monday is open; cycle finishes by closing).
	if _, _, err := s.AssignDelivery(carEmp, 1); err != nil {
		t.Fatalf("run 4 next day: %v", err)
	}
}

// TestCarModeParameters pins the car-mode parameters (SPEC 9.2/9.3/10) and guards
// the foot values against regression: capacity 50 units per run (+10% logistics with
// floor rounding), two local runs per working day, exactly 250p wage per delivered
// package. Far runs stay deferred (SPEC 9.3/16.11): no far-destination state or
// budget exists in this milestone.
func TestCarModeParameters(t *testing.T) {
	if got := deliveryCapacityUnits(ModeCar, false); got != 50 {
		t.Errorf("car capacity = %d, want 50", got)
	}
	if got := deliveryCapacityUnits(ModeCar, true); got != 55 {
		t.Errorf("car capacity with logistics = %d, want 55 (floor of +10%%)", got)
	}
	if got := localRunsPerDay(ModeCar); got != 2 {
		t.Errorf("car runs per day = %d, want 2", got)
	}
	if got := wagePerPackageFor(ModeCar); got != 250 {
		t.Errorf("car wage = %d, want exactly 250p (board decision)", got)
	}

	// Foot values unchanged.
	if got := deliveryCapacityUnits(ModeFoot, false); got != footDeliveryCapacity {
		t.Errorf("foot capacity = %d, want %d", got, footDeliveryCapacity)
	}
	if got := localRunsPerDay(ModeFoot); got != footRunsPerDay {
		t.Errorf("foot runs per day = %d, want %d", got, footRunsPerDay)
	}
	if got := wagePerPackageFor(ModeFoot); got != footWagePerPackage {
		t.Errorf("foot wage = %d, want %d", got, footWagePerPackage)
	}
}

// newBicycleModeState builds a state with an active small office and a ready employee
// switched to bicycle mode. Hire #2 carries the bicycle skill (SPEC 7.2 hire cycle),
// which is the skill gate for bicycle mode.
func newBicycleModeState(t *testing.T) (*GameState, string) {
	t.Helper()
	s := newAssignedOfficeState(t)                 // emp-0001: no skills
	if _, _, err := s.HireEmployee(); err != nil { // emp-0002: bicycle skill
		t.Fatalf("hire #2: %v", err)
	}
	emp, err := s.SetEmployeeMode("emp-0002", ModeBicycle)
	if err != nil || emp.CurrentDeliveryMode != ModeBicycle {
		t.Fatalf("SetEmployeeMode(bicycle) = %v/%s, want success in bicycle mode", err, emp.CurrentDeliveryMode)
	}
	return s, "emp-0002"
}

// TestBicycleModeCapacityTwentyUnits verifies the bicycle-mode capacity of 20 units per
// run (SPEC 9.2): a batch of 20 normal packages fits exactly, express packages consume
// two units each so 11 express overflow with the mode-specific error details, and
// 10 express fit exactly.
func TestBicycleModeCapacityTwentyUnits(t *testing.T) {
	s, bikeEmp := newBicycleModeState(t)

	// Run 1: 20 normal packages = exactly 20 capacity units (SPEC 9.2).
	for i := 1; i <= 20; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	}
	if _, ids, err := s.AssignDelivery(bikeEmp, 20); err != nil || len(ids) != 20 {
		t.Fatalf("bicycle assign of 20 normal = %v/%d ids, want success with 20", err, len(ids))
	}
	// Advance run phases only (no package generation), so the stored inventory stays
	// exact for the capacity assertions below.
	s.processRunsLocked(mustParseTime(t, "1980-02-01T10:00:00")) // packing ends
	s.processRunsLocked(mustParseTime(t, "1980-02-01T13:00:00")) // delivery ends

	// Express double-consumption (SPEC 9.2): 11 express = 22 units > 20 available.
	for i := 21; i <= 31; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceExpress, mustParseTime(t, "1980-02-01T08:30:00"), mustParseTime(t, "1980-02-03T08:30:00"))
	}
	_, _, err := s.AssignDelivery(bikeEmp, 11)
	var capErr *CapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("bicycle over-units err = %T (%v), want *CapacityError", err, err)
	}
	if capErr.Available != bicycleDeliveryCapacity || capErr.Requested != 22 || capErr.StoredAvailable != 11 {
		t.Errorf("capacity error = %+v, want available %d / requested 22 / stored 11", capErr, bicycleDeliveryCapacity)
	}

	// 10 express = exactly 20 units: fits (second run of the day).
	if _, ids, err := s.AssignDelivery(bikeEmp, 10); err != nil || len(ids) != 10 {
		t.Fatalf("bicycle assign of 10 express = %v/%d ids, want success with 10", err, len(ids))
	}
}

// TestBicycleModeRunsPerDayLimitAndRollover verifies the bicycle-mode budget of three
// local runs per working day (SPEC 9.3): after two completed runs a third same-day
// assignment succeeds where a foot-mode employee would hit ErrRunsLimitReached, a fourth
// is rejected by the budget check, and the budget resets on the next game date so a fresh
// run succeeds.
func TestBicycleModeRunsPerDayLimitAndRollover(t *testing.T) {
	s, bikeEmp := newBicycleModeState(t)
	for i := 1; i <= 4; i++ {
		addStoredPackage(s, packageID(i), "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	}

	// Run 1: assign -> pack until 10:00 -> deliver until 13:00.
	if _, _, err := s.AssignDelivery(bikeEmp, 1); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T10:00:00")) // packing ends
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00")) // delivery ends

	// Run 2: assign -> packing drains at 14:00 -> deliver until 17:00 (exactly at
	// closing, allowed).
	if _, _, err := s.AssignDelivery(bikeEmp, 1); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T14:00:00")) // packing ends
	s.processLocked(mustParseTime(t, "1980-02-01T17:00:00")) // delivery ends
	if s.employees[1].RunsToday != 2 {
		t.Fatalf("runs_today = %d, want 2", s.employees[1].RunsToday)
	}

	// Run 3 same day: with two runs consumed the bicycle budget of three allows a third
	// assignment (a foot-mode employee would hit ErrRunsLimitReached here).
	if _, _, err := s.AssignDelivery(bikeEmp, 1); err != nil {
		t.Fatalf("run 3 = %v, want success (bicycle budget is three runs per day)", err)
	}
	s.processLocked(mustParseTime(t, "1980-02-01T14:00:00")) // packing ends
	s.processLocked(mustParseTime(t, "1980-02-01T17:00:00")) // delivery ends
	if s.employees[1].RunsToday != 3 {
		t.Fatalf("runs_today = %d, want 3", s.employees[1].RunsToday)
	}

	// Run 4 same day: rejected by the budget check (SPEC 9.3).
	if _, _, err := s.AssignDelivery(bikeEmp, 1); !errors.Is(err, ErrRunsLimitReached) {
		t.Fatalf("run 4 err = %v, want ErrRunsLimitReached", err)
	}

	// Day rollover (SPEC 9.3): jump the clock to Monday opening and let the
	// simulation process the day change, which resets the run budget.
	s.Clock.restore(mustParseTime(t, "1980-02-04T09:00:00"), 1, false)
	s.processLocked(s.Clock.Now())
	if s.employees[1].RunsToday != 0 {
		t.Fatalf("runs_today after rollover = %d, want 0", s.employees[1].RunsToday)
	}

	// A fresh run succeeds on the new day (Monday is open; cycle finishes by closing).
	if _, _, err := s.AssignDelivery(bikeEmp, 1); err != nil {
		t.Fatalf("run 5 next day: %v", err)
	}
}

// TestBicycleModeParameters pins the bicycle-mode parameters (SPEC 9.2/9.3/10) and guards
// the foot and car values against regression: capacity 20 units per run (+10% logistics
// with floor rounding), three local runs per working day, exactly 300p wage per delivered
// package. Far runs stay deferred (SPEC 9.3/16.11): no far-destination state or budget
// exists in this milestone.
func TestBicycleModeParameters(t *testing.T) {
	if got := deliveryCapacityUnits(ModeBicycle, false); got != 20 {
		t.Errorf("bicycle capacity = %d, want 20", got)
	}
	if got := deliveryCapacityUnits(ModeBicycle, true); got != 22 {
		t.Errorf("bicycle capacity with logistics = %d, want 22 (floor of +10%%)", got)
	}
	if got := localRunsPerDay(ModeBicycle); got != 3 {
		t.Errorf("bicycle runs per day = %d, want 3", got)
	}
	if got := wagePerPackageFor(ModeBicycle); got != 300 {
		t.Errorf("bicycle wage = %d, want exactly 300p (SPEC 10)", got)
	}

	// Foot and car values unchanged by M2-3.
	if got := deliveryCapacityUnits(ModeFoot, false); got != footDeliveryCapacity {
		t.Errorf("foot capacity = %d, want %d", got, footDeliveryCapacity)
	}
	if got := localRunsPerDay(ModeFoot); got != footRunsPerDay {
		t.Errorf("foot runs per day = %d, want %d", got, footRunsPerDay)
	}
	if got := wagePerPackageFor(ModeFoot); got != footWagePerPackage {
		t.Errorf("foot wage = %d, want %d", got, footWagePerPackage)
	}
	if got := deliveryCapacityUnits(ModeCar, false); got != carDeliveryCapacity {
		t.Errorf("car capacity = %d, want %d", got, carDeliveryCapacity)
	}
	if got := localRunsPerDay(ModeCar); got != carRunsPerDay {
		t.Errorf("car runs per day = %d, want %d", got, carRunsPerDay)
	}
	if got := wagePerPackageFor(ModeCar); got != carWagePerPackage {
		t.Errorf("car wage = %d, want %d", got, carWagePerPackage)
	}
}
