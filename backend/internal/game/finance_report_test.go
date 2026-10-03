package game

import "testing"

// TestStatementPreviousPeriodBeforeStart pins the week-0 previous window to the same
// weekly grid before the canonical start (issue #27): it is a zero-activity window
// with real boundaries, not a missing section.
func TestStatementPreviousPeriodBeforeStart(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	st := s.financeStatementLocked(mustParseTime(t, "1980-02-01T09:00:00"))

	if st.Period.From != "1980-02-01T00:00:00" || st.Period.To != "1980-02-07T23:59:59" {
		t.Fatalf("period = %v..%v, want the canonical week 0", st.Period.From, st.Period.To)
	}
	prev := st.PreviousPeriod
	if prev.From != "1980-01-25T00:00:00" || prev.To != "1980-01-31T23:59:59" {
		t.Errorf("previous_period = %v..%v, want the week before the canonical start",
			prev.From, prev.To)
	}
	if prev.Income.Total != 0 || prev.Expenses.Total != 0 || prev.NetChange != 0 {
		t.Errorf("previous_period totals = income %d expenses %d net %d, want zeros before start",
			prev.Income.Total, prev.Expenses.Total, prev.NetChange)
	}
}

// TestStatementPreviousPeriodCarriesPriorWeek pins the week-over-week data path
// (issue #27): a down payment posted in week 0 surfaces as the previous period's
// expense once the statement rolls into week 1, while the current week stays quiet.
func TestStatementPreviousPeriodCarriesPriorWeek(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	s.Clock.restore(mustParseTime(t, "1980-02-08T09:00:00"), 1, false)

	st := s.financeStatementLocked(mustParseTime(t, "1980-02-08T09:00:00"))

	if st.Period.From != "1980-02-08T00:00:00" {
		t.Fatalf("period.from = %v, want week 1", st.Period.From)
	}
	if st.Income.Total != 0 || st.Expenses.Total != 0 {
		t.Errorf("week 1 totals = income %d expenses %d, want a quiet week",
			st.Income.Total, st.Expenses.Total)
	}
	prev := st.PreviousPeriod
	if prev.From != "1980-02-01T00:00:00" || prev.To != "1980-02-07T23:59:59" {
		t.Errorf("previous_period = %v..%v, want week 0", prev.From, prev.To)
	}
	if prev.Expenses.Other != 35000 || prev.Expenses.Total != 35000 {
		t.Errorf("previous expenses = %+v, want the 35000p down payment under other", prev.Expenses)
	}
	if prev.NetChange != -35000 {
		t.Errorf("previous net_change = %d, want -35000", prev.NetChange)
	}
}

// TestDaysUntilNextInterest pins the countdown the finance UI renders (issue #27):
// 28 days from the canonical start to the first four-week charge, 0 on the due day.
func TestDaysUntilNextInterest(t *testing.T) {
	cases := []struct {
		at   string
		want int
	}{
		{"1980-02-01T09:00:00", 28},
		{"1980-02-15T09:00:00", 14},
		{"1980-02-29T09:00:00", 0},
	}
	for _, tc := range cases {
		s := newStateAt(t, tc.at)
		if got := s.DaysUntilNextInterest(); got != tc.want {
			t.Errorf("DaysUntilNextInterest at %s = %d, want %d", tc.at, got, tc.want)
		}
	}
}
