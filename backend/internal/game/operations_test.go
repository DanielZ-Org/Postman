package game

import (
	"errors"
	"testing"
)

func TestHireFeeLadderAndCapacity(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s) // cash 1000 - 350 = 650
	s.cash = 10000    // isolate the fee ladder from affordability

	// Five hires: fees 50, 100, 150, 200, 250 (SPEC 10.1 lifetime counter).
	wantFees := []int{50, 100, 150, 200, 250}
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
	s.cash = 40 // fee is 50
	_, _, err := s.HireEmployee()
	var ife *InsufficientFundsError
	if !errors.As(err, &ife) || ife.Required != 50 || ife.Available != 40 {
		t.Fatalf("hire without cash err = %v, want InsufficientFundsError{50, 40}", err)
	}
	if s.cash != 40 {
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
	if tr.Category != CategoryHiringBonus || tr.Amount != -50 || tr.Description == "" {
		t.Errorf("transaction = %+v, want hiring_bonus -50 with description", tr)
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
	if hiring.NextHiringFee != 100 {
		t.Errorf("next_hiring_fee = %d, want 100 (fee ladder)", hiring.NextHiringFee)
	}
}

func TestEmployeesViewShape(t *testing.T) {
	s := newAssignedOfficeState(t)
	emp, hiring := s.EmployeesView()
	if len(emp) != 1 || emp[0].ID != "emp-0001" {
		t.Fatalf("employees = %+v, want one emp-0001", emp)
	}
	if hiring.CurrentEmployeeCount != 1 || hiring.NextHiringFee != 100 {
		t.Errorf("hiring = %+v, want 1 employee / fee 100", hiring)
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
