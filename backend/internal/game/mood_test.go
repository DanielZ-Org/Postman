package game

import (
	"encoding/json"
	"testing"
)

// TestHireStartsAtFullMood verifies new employees start at mood 100 (SPEC 10,
// decided 16.15).
func TestHireStartsAtFullMood(t *testing.T) {
	s := newAssignedOfficeState(t)
	if s.employees[0].Mood != moodInitial {
		t.Errorf("hire #1 mood = %d, want %d", s.employees[0].Mood, moodInitial)
	}
	emp, _, err := s.HireEmployee()
	if err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	if emp.Mood != moodInitial {
		t.Errorf("hire #2 mood = %d, want %d", emp.Mood, moodInitial)
	}
}

// TestPayrollMoodPositiveDeltaAndCap verifies the rewarding payroll outcome: every
// employee gains moodPayrollUp when payroll leaves cash non-negative, capped at 100.
func TestPayrollMoodPositiveDeltaAndCap(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.employees[0].Mood = 80
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	s.employees[1].Mood = 98

	s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

	if s.employees[0].Mood != 85 {
		t.Errorf("mood 80 after payroll = %d, want 85 (+5, SPEC 10)", s.employees[0].Mood)
	}
	if s.employees[1].Mood != moodMax {
		t.Errorf("mood 98 after payroll = %d, want %d (capped)", s.employees[1].Mood, moodMax)
	}
	if s.payrollDue.Day() != 12 {
		t.Errorf("next payroll day = %d, want 12 (advanced a week)", s.payrollDue.Day())
	}
}

// TestPayrollMoodNegativeWhenCashGoesNegative verifies the demoralising payroll
// outcome: cash below zero after the wage posting costs every employee 25 mood,
// floored at 0.
func TestPayrollMoodNegativeWhenCashGoesNegative(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.employees[0].Mood = 80
	s.employees[0].AccruedWages = 100
	s.cash = 50 // payroll of 100 leaves cash at -50

	s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

	if s.employees[0].Mood != 55 {
		t.Errorf("mood after negative payroll = %d, want 55 (-25, SPEC 10)", s.employees[0].Mood)
	}
	if s.cash != -50 {
		t.Errorf("cash = %d, want -50 (payroll settles unconditionally, SPEC 16.8)", s.cash)
	}
	if s.employees[0].AccruedWages != 0 {
		t.Errorf("accrued wages = %d, want 0 after settlement", s.employees[0].AccruedWages)
	}
}

// TestPayrollMoodNegativeFloorsAtZero verifies the mood floor.
func TestPayrollMoodNegativeFloorsAtZero(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.employees[0].Mood = 10
	s.employees[0].AccruedWages = 100
	s.cash = 50

	s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

	if s.employees[0].Mood != 0 {
		t.Errorf("mood = %d, want 0 (floored)", s.employees[0].Mood)
	}
}

// TestPayrollQuitRollBoundaries verifies the retention roll (SPEC 10, decided 16.15):
// a ready employee at the threshold quits on a roll below 20 and stays at exactly 20;
// a ready employee above the threshold never rolls at all.
func TestPayrollQuitRollBoundaries(t *testing.T) {
	t.Run("roll 19 quits", func(t *testing.T) {
		s := newAssignedOfficeState(t)
		s.employees[0].Mood = moodQuitThreshold - moodPayrollUp // +5 lands exactly at 30
		withDeterministicRand(t, func(int) int { return 19 })

		s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

		if len(s.employees) != 0 {
			t.Errorf("employees = %d, want 0 (mood 30, roll 19 < 20 -> quit)", len(s.employees))
		}
	})

	t.Run("roll 20 stays", func(t *testing.T) {
		s := newAssignedOfficeState(t)
		s.employees[0].Mood = moodQuitThreshold - moodPayrollUp
		withDeterministicRand(t, func(int) int { return 20 })

		s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

		if len(s.employees) != 1 {
			t.Fatalf("employees = %d, want 1 (roll 20 is not below 20)", len(s.employees))
		}
		if s.employees[0].Mood != moodQuitThreshold {
			t.Errorf("mood = %d, want %d (positive payroll first, then no quit)", s.employees[0].Mood, moodQuitThreshold)
		}
	})

	t.Run("above threshold never rolls", func(t *testing.T) {
		s := newAssignedOfficeState(t)
		s.employees[0].Mood = moodQuitThreshold + 1
		withDeterministicRand(t, func(n int) int {
			t.Fatalf("unexpected rand call with bound %d: employees above the threshold must not roll", n)
			return 0
		})

		s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))

		if len(s.employees) != 1 || s.employees[0].Mood != moodQuitThreshold+1+moodPayrollUp {
			t.Errorf("employees/mood = %d/%d, want 1/%d (31+5=36, no roll)", len(s.employees), s.employees[0].Mood, moodQuitThreshold+1+moodPayrollUp)
		}
	})
}

// TestPayrollMidRunEmployeeQuitsOnNextPayroll verifies a mid-run low-mood employee
// survives the payroll that would otherwise remove them and is re-checked once ready.
func TestPayrollMidRunEmployeeQuitsOnNextPayroll(t *testing.T) {
	s := newAssignedOfficeState(t)
	s.employees[0].Mood = moodQuitThreshold - moodPayrollUp // +5 reaches 30: would roll if ready
	s.employees[0].Status = EmployeeOutForDelivery
	withDeterministicRand(t, func(int) int { return 0 }) // would quit whenever rolled

	s.processPayrollLocked(mustParseTime(t, "1980-02-05T09:00:00"))
	if len(s.employees) != 1 {
		t.Fatalf("mid-run employees = %d, want 1 (mid-run checks next payroll, SPEC 10)", len(s.employees))
	}

	// The employee finishes the run and their mood has dropped further by the next
	// Tuesday: now ready, the +5 still leaves them at or below the threshold -> rolls.
	s.employees[0].Status = EmployeeReady
	s.employees[0].Mood = 20
	s.processPayrollLocked(mustParseTime(t, "1980-02-12T09:00:00"))
	if len(s.employees) != 0 {
		t.Errorf("employees after ready payroll = %d, want 0 (rolls once ready)", len(s.employees))
	}
}

// TestEmployeeMoodUnmarshalAcceptsLegacyAndNumeric verifies saves written before mood
// became a number still load: the legacy "neutral" label maps to the initial mood,
// numeric values parse as-is, a missing field defaults to the initial mood, and a
// malformed value fails loudly.
func TestEmployeeMoodUnmarshalAcceptsLegacyAndNumeric(t *testing.T) {
	const legacy = `{"id":"emp-0001","name":"Bob Snail","speed_trait":"snail","skills":[],` +
		`"mood":"neutral","current_delivery_mode":"foot","status":"ready"}`

	var e Employee
	if err := json.Unmarshal([]byte(legacy), &e); err != nil {
		t.Fatalf("legacy unmarshal: %v", err)
	}
	if e.Mood != moodInitial {
		t.Errorf("legacy mood = %d, want %d", e.Mood, moodInitial)
	}
	if e.Name != "Bob Snail" || e.Status != EmployeeReady {
		t.Errorf("other fields lost: %+v", e)
	}

	var numeric Employee
	if err := json.Unmarshal([]byte(`{"id":"emp-0002","mood":42}`), &numeric); err != nil {
		t.Fatalf("numeric unmarshal: %v", err)
	}
	if numeric.Mood != 42 {
		t.Errorf("numeric mood = %d, want 42", numeric.Mood)
	}

	var missing Employee
	if err := json.Unmarshal([]byte(`{"id":"emp-0003"}`), &missing); err != nil {
		t.Fatalf("missing-mood unmarshal: %v", err)
	}
	if missing.Mood != moodInitial {
		t.Errorf("missing mood = %d, want %d (default)", missing.Mood, moodInitial)
	}

	var malformed Employee
	if err := json.Unmarshal([]byte(`{"id":"emp-0004","mood":{}}`), &malformed); err == nil {
		t.Error("malformed mood accepted, want error")
	}
}
