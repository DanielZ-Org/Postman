package game

import (
	"errors"
	"sync"
	"testing"
)

// TestOfficeCatalogue verifies the canonical M1 office catalogue: exactly two definitions, their
// deterministic order and IDs, the exact Small/Large values, and that callers cannot mutate the
// canonical data through the returned copies.
func TestOfficeCatalogue(t *testing.T) {
	defs := OfficeDefinitions()

	t.Run("exactly two definitions", func(t *testing.T) {
		if len(defs) != 2 {
			t.Fatalf("len(OfficeDefinitions()) = %d, want exactly 2", len(defs))
		}
	})

	t.Run("canonical IDs in deterministic order", func(t *testing.T) {
		if defs[0].ID != "office-small-01" || defs[1].ID != "office-large-01" {
			t.Fatalf("IDs = [%s, %s], want [office-small-01, office-large-01]", defs[0].ID, defs[1].ID)
		}
	})

	t.Run("small office values", func(t *testing.T) {
		s := defs[0]
		if s.Type != "small" || s.DownPayment != 350 || s.WeeklyRent != 50 || s.RentPrepaidWeeks != 4 {
			t.Errorf("small core = type %q dp %d rent %d prepaid %d, want small/350/50/4", s.Type, s.DownPayment, s.WeeklyRent, s.RentPrepaidWeeks)
		}
		if s.StorageBase != 100 || s.StorageMax != 150 {
			t.Errorf("small storage base/max = %d/%d, want 100/150", s.StorageBase, s.StorageMax)
		}
		if s.EmployeeCapacity != 5 || s.BicycleCapacity != 5 || s.VehicleCapacity != 1 {
			t.Errorf("small capacities emp/bike/veh = %d/%d/%d, want 5/5/1", s.EmployeeCapacity, s.BicycleCapacity, s.VehicleCapacity)
		}
		if len(s.AcceptedPackageSizes) != 2 || s.AcceptedPackageSizes[0] != "small" || s.AcceptedPackageSizes[1] != "medium" {
			t.Errorf("small accepted sizes = %v, want [small medium]", s.AcceptedPackageSizes)
		}
	})

	t.Run("large office values", func(t *testing.T) {
		l := defs[1]
		if l.Type != "large" || l.DownPayment != 450 || l.WeeklyRent != 75 || l.RentPrepaidWeeks != 4 {
			t.Errorf("large core = type %q dp %d rent %d prepaid %d, want large/450/75/4", l.Type, l.DownPayment, l.WeeklyRent, l.RentPrepaidWeeks)
		}
		if l.StorageBase != 150 || l.StorageMax != 250 {
			t.Errorf("large storage base/max = %d/%d, want 150/250", l.StorageBase, l.StorageMax)
		}
		if l.EmployeeCapacity != 7 || l.BicycleCapacity != 7 || l.VehicleCapacity != 2 {
			t.Errorf("large capacities emp/bike/veh = %d/%d/%d, want 7/7/2", l.EmployeeCapacity, l.BicycleCapacity, l.VehicleCapacity)
		}
		if len(l.AcceptedPackageSizes) != 2 || l.AcceptedPackageSizes[0] != "small" || l.AcceptedPackageSizes[1] != "medium" {
			t.Errorf("large accepted sizes = %v, want [small medium]", l.AcceptedPackageSizes)
		}
	})

	t.Run("callers cannot mutate canonical definitions", func(t *testing.T) {
		canonical := OfficeDefinitions()
		wantID, wantDP := canonical[0].ID, canonical[0].DownPayment

		mutated := OfficeDefinitions() // a fresh copy; mutating it must not affect the catalogue
		mutated[0].ID = "mutated-id"
		mutated[0].DownPayment = 999
		mutated[0].AcceptedPackageSizes[0] = "MUTATED"

		fresh := OfficeDefinitions()
		if fresh[0].ID != wantID || fresh[0].DownPayment != wantDP {
			t.Errorf("canonical small mutated: id %q dp %d, want %q/%d", fresh[0].ID, fresh[0].DownPayment, wantID, wantDP)
		}
		if fresh[0].AcceptedPackageSizes[0] == "MUTATED" {
			t.Error("canonical accepted sizes mutated through a returned copy")
		}
	})
}

// TestNewInitialStateDefaults verifies the initial state: no selected office, exactly £1000 cash,
// and exactly the initial loan-disbursement transaction.
func TestNewInitialStateDefaults(t *testing.T) {
	s := NewInitialState()
	if s.SelectedOffice() != nil {
		t.Errorf("initial selected office = %v, want nil", s.SelectedOffice())
	}
	if got := s.Cash(); got != 1000 {
		t.Errorf("initial cash = %d, want exactly 1000 (StartingCash)", got)
	}
	txn := s.Transactions()
	if len(txn) != 1 {
		t.Fatalf("initial transactions = %d, want exactly 1 (loan disbursement)", len(txn))
	}
	if txn[0].Category != CategoryLoanDisbursement || txn[0].Amount != 1000 || txn[0].ID != "txn-000001" {
		t.Errorf("initial transaction = %+v, want txn-000001 loan_disbursement +1000", txn[0])
	}
}

// TestSelectSmallOffice verifies a successful Small selection: the runtime office shape, cash
// deduction to £650, exactly one correct transaction, and that the clock itself is unchanged.
func TestSelectSmallOffice(t *testing.T) {
	s := NewInitialState()
	before := s.Clock.Snapshot()

	office, cash, err := s.SelectOffice("office-small-01")
	if err != nil {
		t.Fatalf("SelectOffice(small) error = %v, want nil", err)
	}
	if office.ID != "office-small-01" || !office.IsHeadOffice || office.Type != "small" {
		t.Errorf("selected office id/head/type = %q/%v/%q, want office-small-01/true/small", office.ID, office.IsHeadOffice, office.Type)
	}
	if office.Storage.Base != 100 || office.Storage.Current != 100 || office.Storage.Max != 150 || office.Storage.Used != 0 {
		t.Errorf("storage = %+v, want base/current/max/used = 100/100/150/0", office.Storage)
	}
	if office.ContractStatus != "active" || office.MissedRentPayments != 0 {
		t.Errorf("contract status/missed rent = %q/%d, want active/0", office.ContractStatus, office.MissedRentPayments)
	}
	if cash != 650 || s.Cash() != 650 {
		t.Errorf("cash after small selection = %d (state %d), want 650", cash, s.Cash())
	}

	txn := s.Transactions()
	if len(txn) != 2 {
		t.Fatalf("transactions = %d, want exactly 2 (loan + down payment)", len(txn))
	}
	tr := txn[1]
	if tr.Category != CategoryOfficeDownPayment || tr.Amount != -350 || tr.ReferenceID != "office-small-01" || tr.ID != "txn-000002" {
		t.Errorf("transaction = %+v, want txn-000002 category office_down_payment amount -350 reference_id office-small-01", tr)
	}
	if tr.GameDatetime != before.GameDatetime {
		t.Errorf("transaction game_datetime = %q, want clock snapshot %q", tr.GameDatetime, before.GameDatetime)
	}

	after := s.Clock.Snapshot()
	if after.GameDatetime != before.GameDatetime || after.Speed != before.Speed || after.Paused != before.Paused {
		t.Errorf("clock changed during selection: before %+v after %+v (selection must not advance time)", before, after)
	}
}

// TestSelectLargeOffice verifies a successful Large selection: cash deduction to £550 and a -450
// transaction.
func TestSelectLargeOffice(t *testing.T) {
	s := NewInitialState()
	office, cash, err := s.SelectOffice("office-large-01")
	if err != nil {
		t.Fatalf("SelectOffice(large) error = %v, want nil", err)
	}
	if office.ID != "office-large-01" || !office.IsHeadOffice || office.Type != "large" {
		t.Errorf("selected office id/head/type = %q/%v/%q, want office-large-01/true/large", office.ID, office.IsHeadOffice, office.Type)
	}
	if cash != 550 || s.Cash() != 550 {
		t.Errorf("cash after large selection = %d (state %d), want 550", cash, s.Cash())
	}
	txn := s.Transactions()
	if len(txn) != 2 || txn[1].Category != CategoryOfficeDownPayment || txn[1].Amount != -450 || txn[1].ReferenceID != "office-large-01" {
		t.Errorf("transaction = %+v, want loan + one office_down_payment amount -450 reference_id office-large-01", txn)
	}
}

// TestSelectRejectionsLeaveStateUnchanged verifies each rejection path (unknown office, insufficient
// funds, already-selected other office, same-office reselection) leaves the selected office, cash and
// transaction list unchanged.
func TestSelectRejectionsLeaveStateUnchanged(t *testing.T) {
	t.Run("unknown office", func(t *testing.T) {
		s := NewInitialState()
		if _, _, err := s.SelectOffice("office-bogus-99"); !errors.Is(err, ErrOfficeNotFound) {
			t.Fatalf("error = %v, want ErrOfficeNotFound", err)
		}
		assertUnchanged(t, s, "", 1000, 1)
	})

	t.Run("insufficient funds", func(t *testing.T) {
		s := NewInitialState()
		// M1D has no API to reduce cash; set the authoritative balance directly (same-package test)
		// to reach the insufficient-funds branch: £300 < Small's £350 down payment.
		s.cash = 300
		_, _, err := s.SelectOffice("office-small-01")
		var ife *InsufficientFundsError
		if !errors.As(err, &ife) || ife.Required != 350 || ife.Available != 300 {
			t.Fatalf("error = %v, want *InsufficientFundsError{Required:350, Available:300}", err)
		}
		assertUnchanged(t, s, "", 300, 1)
	})

	t.Run("already selected other office", func(t *testing.T) {
		s := NewInitialState()
		if _, _, err := s.SelectOffice("office-small-01"); err != nil {
			t.Fatalf("setup SelectOffice(small): %v", err)
		}
		if _, _, err := s.SelectOffice("office-large-01"); !errors.Is(err, ErrAlreadySelected) {
			t.Fatalf("error = %v, want ErrAlreadySelected", err)
		}
		assertUnchanged(t, s, "office-small-01", 650, 2)
	})

	t.Run("same office reselection rejected", func(t *testing.T) {
		s := NewInitialState()
		if _, _, err := s.SelectOffice("office-large-01"); err != nil {
			t.Fatalf("setup SelectOffice(large): %v", err)
		}
		if _, _, err := s.SelectOffice("office-large-01"); !errors.Is(err, ErrAlreadySelected) {
			t.Fatalf("error = %v, want ErrAlreadySelected (same office)", err)
		}
		assertUnchanged(t, s, "office-large-01", 550, 2)
	})
}

// assertUnchanged verifies the authoritative state matches the expected selected-office ID (or nil),
// cash balance and transaction count — i.e., a rejected operation left everything unchanged.
func assertUnchanged(t *testing.T, s *GameState, wantSelectedID string, wantCash int, wantTxns int) {
	t.Helper()
	off := s.SelectedOffice()
	if wantSelectedID == "" {
		if off != nil {
			t.Errorf("selected office = %v, want nil", off.ID)
		}
	} else if off == nil || off.ID != wantSelectedID {
		t.Errorf("selected office = %v, want %s", off, wantSelectedID)
	}
	if got := s.Cash(); got != wantCash {
		t.Errorf("cash = %d, want unchanged %d", got, wantCash)
	}
	if n := len(s.Transactions()); n != wantTxns {
		t.Errorf("transactions = %d, want unchanged %d", n, wantTxns)
	}
}

// TestConcurrentSelectionOnlyOneSucceeds proves that competing concurrent selections cannot both
// succeed: exactly one office is selected, exactly one down payment is deducted, and exactly one
// transaction is created — regardless of which office wins (no reliance on request ordering).
func TestConcurrentSelectionOnlyOneSucceeds(t *testing.T) {
	s := NewInitialState()
	const n = 8
	ids := []string{"office-small-01", "office-large-01"}

	var wg sync.WaitGroup
	results := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // barrier: release all goroutines at once to maximize contention
			_, _, err := s.SelectOffice(ids[i%2])
			results[i] = err
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful selections = %d, want exactly 1", successes)
	}

	off := s.SelectedOffice()
	if off == nil {
		t.Fatal("no office selected after concurrent selection")
	}
	cash := s.Cash()
	if cash != 1000-off.DownPayment {
		t.Errorf("cash = %d, want 1000 - %d (one down payment) = %d", cash, off.DownPayment, 1000-off.DownPayment)
	}
	if n := len(s.Transactions()); n != 2 {
		t.Fatalf("transactions = %d, want exactly 2 (loan + one down payment)", n)
	}
	if tr := s.Transactions()[1]; tr.Category != CategoryOfficeDownPayment || tr.Amount != -off.DownPayment || tr.ReferenceID != off.ID {
		t.Errorf("transaction = %+v, want category office_down_payment amount -%d reference_id %s", tr, off.DownPayment, off.ID)
	}
}
