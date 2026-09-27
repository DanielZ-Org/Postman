package game

import (
	"errors"
	"testing"
)

// TestVehiclePurchaseCapacityLimits verifies purchases are allowed up to exactly the
// office's per-type limit and rejected at the limit with details (approved plan
// section 3.3). The small office caps bicycles at 5 and cars at 1.
func TestVehiclePurchaseCapacityLimits(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.cash = 100000 // isolate the capacity ladder from affordability

	for i := 0; i < 5; i++ {
		if _, err := s.PurchaseVehicle("bicycle"); err != nil {
			t.Fatalf("bicycle %d: %v", i+1, err)
		}
	}
	if s.bicyclesOwned != 5 || s.carsOwned != 0 {
		t.Errorf("owned = %d/%d, want 5/0", s.bicyclesOwned, s.carsOwned)
	}

	_, err := s.PurchaseVehicle("bicycle")
	var capErr *VehicleCapacityError
	if !errors.As(err, &capErr) || capErr.Limit != 5 || capErr.Owned != 5 {
		t.Fatalf("6th bicycle err = %v, want VehicleCapacityError{Limit:5, Owned:5}", err)
	}

	if _, err := s.PurchaseVehicle("car"); err != nil {
		t.Fatalf("first car: %v", err)
	}
	_, err = s.PurchaseVehicle("car")
	if !errors.As(err, &capErr) || capErr.Limit != 1 || capErr.Owned != 1 {
		t.Fatalf("second car err = %v, want VehicleCapacityError{Limit:1, Owned:1}", err)
	}
}

// TestVehiclePurchaseInsufficientFunds verifies the funds check (validation step 4)
// and that a failed purchase leaves state unchanged.
func TestVehiclePurchaseInsufficientFunds(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.cash = bicyclePurchasePrice - 1 // one penny short
	txnBefore := len(s.transactions)

	_, err := s.PurchaseVehicle("bicycle")
	var ife *InsufficientFundsError
	if !errors.As(err, &ife) || ife.Required != bicyclePurchasePrice || ife.Available != bicyclePurchasePrice-1 {
		t.Fatalf("err = %v, want InsufficientFundsError{Required:%d, Available:%d}", err, bicyclePurchasePrice, bicyclePurchasePrice-1)
	}
	if s.bicyclesOwned != 0 || s.cash != bicyclePurchasePrice-1 || len(s.transactions) != txnBefore {
		t.Errorf("failed purchase mutated state: owned=%d cash=%d txns=%d", s.bicyclesOwned, s.cash, len(s.transactions))
	}
}

// TestVehiclePurchaseRequiresOfficeAndRunning verifies validation steps 1 and 2.
func TestVehiclePurchaseRequiresOfficeAndRunning(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	if _, err := s.PurchaseVehicle("bicycle"); !errors.Is(err, ErrNoOffice) {
		t.Errorf("purchase without office err = %v, want ErrNoOffice", err)
	}

	selectSmall(t, s)
	s.status = GameStatusGameOver
	if _, err := s.PurchaseVehicle("bicycle"); !errors.Is(err, ErrGameOver) {
		t.Errorf("purchase after game over err = %v, want ErrGameOver", err)
	}
}

// TestHireSkillCycle verifies the deterministic hire-time skill distribution (SPEC
// 7.2 labelled assumption): none -> bicycle -> bicycle + driving licence, repeating.
func TestHireSkillCycle(t *testing.T) {
	s := newAssignedOfficeState(t) // hire #1: no skills
	if len(s.employees[0].Skills) != 0 {
		t.Errorf("hire #1 skills = %v, want none", s.employees[0].Skills)
	}
	for i := 2; i <= 4; i++ {
		if _, _, err := s.HireEmployee(); err != nil {
			t.Fatalf("hire #%d: %v", i, err)
		}
	}
	want := [][]string{
		{},
		{SkillBicycle},
		{SkillBicycle, SkillDrivingLicence},
		{}, // hire #4 restarts the cycle
	}
	for i, emp := range s.employees {
		if len(emp.Skills) != len(want[i]) {
			t.Errorf("hire #%d skills = %v, want %v", i+1, emp.Skills, want[i])
			continue
		}
		for j := range want[i] {
			if emp.Skills[j] != want[i][j] {
				t.Errorf("hire #%d skill[%d] = %q, want %q", i+1, j, emp.Skills[j], want[i][j])
			}
		}
	}
}

// TestSetEmployeeModeValidation verifies the mode-switch validation order (approved
// plan section 3.3): employee exists -> mode valid -> skill present for non-foot
// modes -> employee ready. Foot is always allowed regardless of skills.
func TestSetEmployeeModeValidation(t *testing.T) {
	s := newAssignedOfficeState(t) // hire #1: no skills

	if _, err := s.SetEmployeeMode("emp-9999", ModeBicycle); !errors.Is(err, ErrEmployeeNotFound) {
		t.Errorf("unknown employee err = %v, want ErrEmployeeNotFound", err)
	}
	if _, err := s.SetEmployeeMode("emp-0001", "drone"); !errors.Is(err, ErrInvalidMode) {
		t.Errorf("invalid mode err = %v, want ErrInvalidMode", err)
	}

	// No skills: both vehicle modes are blocked with the missing-skill details.
	_, err := s.SetEmployeeMode("emp-0001", ModeBicycle)
	var skillErr *MissingSkillError
	if !errors.As(err, &skillErr) || skillErr.EmployeeID != "emp-0001" || skillErr.RequiredSkill != SkillBicycle {
		t.Errorf("bicycle without skill err = %v, want MissingSkillError{emp-0001, %s}", err, SkillBicycle)
	}
	_, err = s.SetEmployeeMode("emp-0001", ModeCar)
	if !errors.As(err, &skillErr) || skillErr.RequiredSkill != SkillDrivingLicence {
		t.Errorf("car without skill err = %v, want MissingSkillError{RequiredSkill:%s}", err, SkillDrivingLicence)
	}

	// Foot is always allowed and needs no skill.
	emp, err := s.SetEmployeeMode("emp-0001", ModeFoot)
	if err != nil || emp.CurrentDeliveryMode != ModeFoot {
		t.Errorf("foot switch = %v/%s, want success in foot mode", err, emp.CurrentDeliveryMode)
	}

	// Hire #2 carries the bicycle skill: bicycle works, car still needs the licence.
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("hire #2: %v", err)
	}
	if emp, err = s.SetEmployeeMode("emp-0002", ModeBicycle); err != nil || emp.CurrentDeliveryMode != ModeBicycle {
		t.Errorf("bicycle with skill = %v/%s, want success in bicycle mode", err, emp.CurrentDeliveryMode)
	}
	if _, err := s.SetEmployeeMode("emp-0002", ModeCar); !errors.As(err, &skillErr) || skillErr.RequiredSkill != SkillDrivingLicence {
		t.Errorf("car without licence err = %v, want MissingSkillError{RequiredSkill:%s}", err, SkillDrivingLicence)
	}

	// Hire #3 carries both skills: car works.
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("hire #3: %v", err)
	}
	if emp, err = s.SetEmployeeMode("emp-0003", ModeCar); err != nil || emp.CurrentDeliveryMode != ModeCar {
		t.Errorf("car with licence = %v/%s, want success in car mode", err, emp.CurrentDeliveryMode)
	}
}

// TestSetEmployeeModeBusyRejected verifies a mid-run employee cannot switch modes.
func TestSetEmployeeModeBusyRejected(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.employees[0].Status = EmployeePacking // mid-run (same-package test shortcut)

	if _, err := s.SetEmployeeMode("emp-0001", ModeFoot); !errors.Is(err, ErrEmployeeBusy) {
		t.Errorf("busy mode switch err = %v, want ErrEmployeeBusy", err)
	}
}

// TestVehiclePurchasePostsExactlyOneTransaction verifies the atomic commit: one
// vehicle_purchase transaction at the exact price, cash deducted once, inventory
// incremented once.
func TestVehiclePurchasePostsExactlyOneTransaction(t *testing.T) {
	s := newAssignedOfficeState(t)
	before := len(s.transactions)
	cashBefore := s.cash

	view, err := s.PurchaseVehicle("bicycle")
	if err != nil {
		t.Fatalf("PurchaseVehicle: %v", err)
	}
	if view.BicyclesOwned != 1 || view.CarsOwned != 0 {
		t.Errorf("view = %+v, want bicycles_owned 1 cars_owned 0", view)
	}
	if len(s.transactions) != before+1 {
		t.Fatalf("transactions = %d, want exactly one new entry (%d)", len(s.transactions), before)
	}
	tr := s.transactions[len(s.transactions)-1]
	if tr.Category != CategoryVehiclePurchase || tr.Amount != -bicyclePurchasePrice || tr.ReferenceID != "bicycle" {
		t.Errorf("transaction = %+v, want vehicle_purchase -%d reference bicycle", tr, bicyclePurchasePrice)
	}
	if s.cash != cashBefore-bicyclePurchasePrice {
		t.Errorf("cash = %d, want %d (one price deducted)", s.cash, cashBefore-bicyclePurchasePrice)
	}
}

// TestVehicleInventoryAndModesPersistRoundTrip verifies the full persistence round-
// trip of vehicle inventory and employee delivery modes across save/restore.
func TestVehicleInventoryAndModesPersistRoundTrip(t *testing.T) {
	s := newAssignedOfficeState(t)                 // hire #1: no skills
	if _, _, err := s.HireEmployee(); err != nil { // hire #2: bicycle skill
		t.Fatalf("hire #2: %v", err)
	}
	if _, _, err := s.HireEmployee(); err != nil { // hire #3: both skills
		t.Fatalf("hire #3: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := s.PurchaseVehicle("bicycle"); err != nil {
			t.Fatalf("bicycle %d: %v", i+1, err)
		}
	}
	s.cash = 100000 // isolate the inventory round-trip from affordability
	if _, err := s.PurchaseVehicle("car"); err != nil {
		t.Fatalf("car: %v", err)
	}
	if _, err := s.SetEmployeeMode("emp-0002", ModeBicycle); err != nil {
		t.Fatalf("mode emp-0002: %v", err)
	}
	if _, err := s.SetEmployeeMode("emp-0003", ModeCar); err != nil {
		t.Fatalf("mode emp-0003: %v", err)
	}

	snap := s.Snapshot()
	if snap.BicyclesOwned != 2 || snap.CarsOwned != 1 {
		t.Errorf("snapshot inventory = %d/%d, want 2/1", snap.BicyclesOwned, snap.CarsOwned)
	}

	restored := NewInitialState()
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.bicyclesOwned != 2 || restored.carsOwned != 1 {
		t.Errorf("restored inventory = %d/%d, want 2/1", restored.bicyclesOwned, restored.carsOwned)
	}
	modes := map[string]string{}
	for _, e := range restored.employees {
		modes[e.ID] = e.CurrentDeliveryMode
	}
	if modes["emp-0001"] != ModeFoot || modes["emp-0002"] != ModeBicycle || modes["emp-0003"] != ModeCar {
		t.Errorf("restored modes = %v, want foot/bicycle/car", modes)
	}

	// The restored state must be usable: the next snapshot carries the same values.
	again := restored.Snapshot()
	if again.BicyclesOwned != 2 || again.CarsOwned != 1 {
		t.Errorf("second snapshot inventory = %d/%d, want 2/1", again.BicyclesOwned, again.CarsOwned)
	}
}
