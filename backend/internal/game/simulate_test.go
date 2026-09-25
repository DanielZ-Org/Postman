package game

import (
	"testing"
	"time"
)

// mustParseTime parses a canonical game instant in tests.
func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(GameTimeFormat, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// newStateAt builds a fresh state whose clock reads the given canonical instant.
func newStateAt(t *testing.T, at string) *GameState {
	t.Helper()
	s := NewInitialState()
	s.Clock.restore(mustParseTime(t, at), 1, false)
	return s
}

// selectSmall selects the small office in tests that need an active contract.
func selectSmall(t *testing.T, s *GameState) *RuntimeOffice {
	t.Helper()
	office, _, err := s.SelectOffice("office-small-01")
	if err != nil {
		t.Fatalf("SelectOffice(small): %v", err)
	}
	return office
}

// addStoredPackage injects a stored package directly (bypassing generation) for
// deterministic delivery tests.
func addStoredPackage(s *GameState, id, size, service string, received, due time.Time) *Package {
	p := newPackage(id, size, service, received)
	p.DueAt = due.Format(GameTimeFormat)
	s.packages = append(s.packages, p)
	s.nextPkgSeq++
	return p
}

// withDeterministicRand replaces the generation RNG for the duration of a test.
func withDeterministicRand(t *testing.T, fn func(int) int) {
	t.Helper()
	old := randIntn
	randIntn = fn
	t.Cleanup(func() { randIntn = old })
}

func TestGenerationRateDuringOpenHours(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	withDeterministicRand(t, func(int) int { return 0 }) // always small, never express

	// One 30-minute tick during open hours produces exactly one package (SPEC 6: 2 per open hour).
	if !s.generatePackagesLocked(mustParseTime(t, "1980-02-01T09:30:00")) {
		t.Fatal("generation at 09:30 reported no change, want one package")
	}
	if len(s.packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(s.packages))
	}
	p := s.packages[0]
	if p.Size != "small" || p.ServiceType != ServiceNormal || p.Status != PackageStored {
		t.Errorf("package = %+v, want small/normal/stored", p)
	}
	if p.ReceivedAt != "1980-02-01T09:30:00" || p.DueAt != "1980-02-06T09:30:00" {
		t.Errorf("received/due = %q/%q, want 09:30 receipt + 5 days (SPEC 5.2)", p.ReceivedAt, p.DueAt)
	}
	if p.BaseFee != 5 || p.StorageUnits != 1 || p.DeliveryCapacityUnits != 1 {
		t.Errorf("fee/units = %d/%d/%d, want 5/1/1", p.BaseFee, p.StorageUnits, p.DeliveryCapacityUnits)
	}

	// Catching up from 09:30 to 12:00 spawns at 10:00, 10:30, 11:00, 11:30, 12:00.
	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T12:00:00"))
	if len(s.packages) != 6 {
		t.Fatalf("packages after catch-up = %d, want 6", len(s.packages))
	}
}

func TestGenerationSkipsClosedHours(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	withDeterministicRand(t, func(int) int { return 0 })

	// Run the cursor across Friday evening (closed), Saturday's short opening and the
	// closed Sunday: every spawned package must land inside opening hours (SPEC 6).
	s.generatePackagesLocked(mustParseTime(t, "1980-02-04T08:59:00"))
	if len(s.packages) == 0 {
		t.Fatal("expected packages during open ticks")
	}
	for _, p := range s.packages {
		received := mustParseTime(t, p.ReceivedAt)
		if !officeOpen(received) {
			t.Errorf("package received at %s outside opening hours", p.ReceivedAt)
		}
		if received.Weekday() == time.Sunday {
			t.Errorf("package generated on Sunday %s (closed all day)", p.ReceivedAt)
		}
	}
	// The specific closed instants in the gap must be absent.
	for _, p := range s.packages {
		switch p.ReceivedAt {
		case "1980-02-01T17:00:00", "1980-02-01T17:30:00", "1980-02-02T09:30:00", "1980-02-02T13:00:00":
			t.Errorf("package spawned at closed instant %s", p.ReceivedAt)
		}
	}
}

func TestGenerationRequiresActiveOffice(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	withDeterministicRand(t, func(int) int { return 0 })

	// No office selected: no generation.
	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T12:00:00"))
	if len(s.packages) != 0 {
		t.Fatalf("packages without office = %d, want 0", len(s.packages))
	}

	// Terminated contract: generation stops.
	office := selectSmall(t, s)
	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T12:30:00"))
	if len(s.packages) == 0 {
		t.Fatal("expected packages with active office")
	}
	office.ContractStatus = ContractTerminated
	before := len(s.packages)
	s.lastGeneration = mustParseTime(t, "1980-02-01T12:30:00")
	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T16:30:00"))
	if len(s.packages) != before {
		t.Fatalf("packages after termination = %d, want unchanged %d", len(s.packages), before)
	}
}

func TestExpressOnlyAfterFourWeeks(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	// Force express on every eligible roll.
	withDeterministicRand(t, func(int) int { return 0 })

	// During the first month the service must stay normal even though RNG says express.
	s.lastGeneration = mustParseTime(t, "1980-02-19T09:00:00")
	s.generatePackagesLocked(mustParseTime(t, "1980-02-20T09:30:00"))
	if len(s.packages) == 0 {
		t.Fatal("expected packages during the first month")
	}
	for _, p := range s.packages {
		if p.ServiceType != ServiceNormal {
			t.Fatalf("month-1 package service = %s, want normal (SPEC 6: 0%% express in first 4 weeks)", p.ServiceType)
		}
	}

	// Exactly at start + 4 weeks (1980-02-29T09:00:00) the 3% roll becomes eligible.
	s.lastGeneration = mustParseTime(t, "1980-02-29T09:00:00")
	s.generatePackagesLocked(mustParseTime(t, "1980-02-29T10:00:00"))
	if len(s.packages) < 2 {
		t.Fatalf("packages = %d, want the pre-window one plus at least one in-window", len(s.packages))
	}
	last := s.packages[len(s.packages)-1]
	if last.ReceivedAt < "1980-02-29T09:00:00" {
		t.Fatalf("last package received %s, want inside express window", last.ReceivedAt)
	}
	if last.ServiceType != ServiceExpress {
		t.Errorf("in-window package service = %s, want express (RNG forced)", last.ServiceType)
	}
	if last.BaseFee != 12 {
		t.Errorf("express small base fee = %d, want 12 (SPEC 5.3)", last.BaseFee)
	}
}

func TestGenerationSkipsWhenStorageFull(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	office := selectSmall(t, s)
	withDeterministicRand(t, func(int) int { return 0 })

	// Fill storage completely (100 units for the small office).
	full := newPackage("pkg-099999", "small", ServiceNormal, mustParseTime(t, "1980-02-01T09:05:00"))
	full.StorageUnits = 100
	full.DueAt = "1980-02-06T09:05:00"
	s.packages = append(s.packages, full)
	if s.storageUsedLocked() != office.Storage.Current {
		t.Fatalf("storage used = %d, want %d", s.storageUsedLocked(), office.Storage.Current)
	}

	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T16:30:00"))
	// Only the pre-filled package exists; nothing overfilled storage (SPEC 16.3: do not overfill).
	if len(s.packages) != 1 {
		t.Fatalf("packages = %d, want 1 (generation skipped at capacity)", len(s.packages))
	}
}

func TestRunLifecycleDeliversAndAccrues(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T09:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))
	addStoredPackage(s, "pkg-000002", "medium", ServiceNormal, mustParseTime(t, "1980-02-01T09:00:00"), mustParseTime(t, "1980-02-06T09:00:00"))

	emp, ids, err := s.AssignDelivery("emp-0001", 2)
	if err != nil {
		t.Fatalf("AssignDelivery: %v", err)
	}
	if emp.Status != EmployeePacking || len(ids) != 2 {
		t.Fatalf("employee/status ids = %s/%v, want packing with 2 packages", emp.Status, ids)
	}
	if s.packages[0].Status != PackageAssigned || s.packages[0].AssignedEmployeeID == nil {
		t.Fatalf("package status = %s, want assigned with employee id", s.packages[0].Status)
	}

	// Packing ends at 10:00: packages go out for delivery, employee state follows.
	s.processLocked(mustParseTime(t, "1980-02-01T10:00:00"))
	if s.packages[0].Status != PackageOutForDelivery || s.employees[0].Status != EmployeeOutForDelivery {
		t.Fatalf("after packing: package=%s employee=%s, want out_for_delivery", s.packages[0].Status, s.employees[0].Status)
	}
	if len(s.runs) != 1 {
		t.Fatalf("active runs = %d, want 1", len(s.runs))
	}

	// Delivery ends at 13:00: packages deliver, revenue posts, wages accrue.
	cashBefore := s.cash
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00"))
	if len(s.runs) != 0 {
		t.Fatalf("active runs after completion = %d, want 0", len(s.runs))
	}
	if s.employees[0].Status != EmployeeReady {
		t.Errorf("employee status = %s, want ready", s.employees[0].Status)
	}
	wantRevenue := 5 + 7 // small + medium normal (SPEC 5.3)
	if s.cash != cashBefore+wantRevenue {
		t.Errorf("cash = %d, want %d (revenue %d)", s.cash, cashBefore+wantRevenue, wantRevenue)
	}
	var revenueTxn *Transaction
	for i := range s.transactions {
		if s.transactions[i].Category == CategoryPackageRevenue {
			revenueTxn = &s.transactions[i]
		}
	}
	if revenueTxn == nil || revenueTxn.Amount != wantRevenue {
		t.Fatalf("revenue transaction = %+v, want +%d", revenueTxn, wantRevenue)
	}
	if s.employees[0].PackagesDeliveredThisWeek != 2 || s.employees[0].AccruedWages != 2*footWagePerPackage {
		t.Errorf("weekly/wages = %d/%d, want 2/%d (SPEC 10: £2 per package)", s.employees[0].PackagesDeliveredThisWeek, s.employees[0].AccruedWages, 2*footWagePerPackage)
	}
	if s.deliveredToday != 2 {
		t.Errorf("delivered_today = %d, want 2", s.deliveredToday)
	}
	for _, p := range s.packages {
		if p.ID == "pkg-000001" || p.ID == "pkg-000002" {
			if p.Status != PackageDelivered || p.DeliveredAt == nil || p.FinalRevenue == nil {
				t.Errorf("package = %+v, want delivered with timestamp and revenue", p)
			}
		}
	}
	// No trait-bonus transaction yet: the financial bonus settles at day rollover (SPEC 3.1).
	for _, tr := range s.transactions {
		if tr.Category == CategoryTraitBonus {
			t.Errorf("unexpected early trait bonus: %+v", tr)
		}
	}
}

func TestLateDeliveryPaysReducedRevenue(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	// Already late when assigned (due yesterday).
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-01-31T09:00:00"), mustParseTime(t, "1980-01-31T09:00:00"))
	if _, _, err := s.AssignDelivery("emp-0001", 1); err != nil {
		t.Fatalf("AssignDelivery: %v", err)
	}
	cashBefore := s.cash
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00"))
	// Normal late: 25% reduction -> £5 * 0.75 = £3.75 -> rounds to £4 (integer pounds).
	if s.cash != cashBefore+4 {
		t.Errorf("late revenue cash = %d, want +%d (25%% reduction, rounded)", s.cash-cashBefore, 4)
	}
	if s.employees[0].AccruedWages != footWagePerPackage {
		t.Errorf("wages = %d, want %d (express/late still counts one package)", s.employees[0].AccruedWages, footWagePerPackage)
	}
}

func TestFinancialTraitBonusSettlesAtDayRollover(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	s.dailyRevenue = 100
	cashBefore := s.cash

	s.processLocked(mustParseTime(t, "1980-02-02T00:00:01"))
	if s.dailyRevenue != 0 || s.dailyRevenueKey != "1980-02-02" {
		t.Fatalf("daily revenue = %d/%s, want reset for new day", s.dailyRevenue, s.dailyRevenueKey)
	}
	found := false
	for _, tr := range s.transactions {
		if tr.Category == CategoryTraitBonus {
			found = true
			if tr.Amount != 10 {
				t.Errorf("bonus amount = %d, want 10 (10%% of 100, SPEC 3.1)", tr.Amount)
			}
		}
	}
	if !found {
		t.Fatal("no financial_trait_bonus transaction at day rollover")
	}
	if s.cash != cashBefore+10 {
		t.Errorf("cash = %d, want +%d", s.cash-cashBefore, 10)
	}
	if s.deliveredToday != 0 {
		t.Errorf("delivered_today = %d, want reset at rollover", s.deliveredToday)
	}
}

func TestPayrollSettlesTuesdayWages(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	if _, _, err := s.HireEmployee(); err != nil {
		t.Fatalf("HireEmployee: %v", err)
	}
	s.employees[0].AccruedWages = 20
	s.employees[0].PackagesDeliveredThisWeek = 10
	cashBefore := s.cash

	// First Tuesday after the canonical start is 5 February 09:00.
	if s.payrollDue.Format(GameTimeFormat) != "1980-02-05T09:00:00" {
		t.Fatalf("first payroll due = %s, want 1980-02-05T09:00:00", s.payrollDue.Format(GameTimeFormat))
	}
	s.processLocked(mustParseTime(t, "1980-02-05T09:00:00"))

	if s.cash != cashBefore-20 {
		t.Errorf("cash = %d, want -20 wages", s.cash-cashBefore)
	}
	if s.employees[0].AccruedWages != 0 || s.employees[0].PackagesDeliveredThisWeek != 0 {
		t.Errorf("accrued/weekly = %d/%d, want reset after payroll", s.employees[0].AccruedWages, s.employees[0].PackagesDeliveredThisWeek)
	}
	var wages *Transaction
	for i := range s.transactions {
		if s.transactions[i].Category == CategoryEmployeeWages {
			wages = &s.transactions[i]
		}
	}
	if wages == nil || wages.Amount != -20 {
		t.Fatalf("wage transaction = %+v, want -20", wages)
	}
	if s.payrollDue.Format(GameTimeFormat) != "1980-02-12T09:00:00" {
		t.Errorf("next payroll = %s, want +7 days", s.payrollDue.Format(GameTimeFormat))
	}
}

func TestRentPaymentMissesAndTermination(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	office := selectSmall(t, s) // cash 650
	if office.NextRentDue != "1980-02-29T00:00:00" {
		t.Fatalf("next_rent_due = %s, want prepaid 4 weeks (SPEC 4.3)", office.NextRentDue)
	}

	// First due Friday: rent paid from cash.
	cash := s.cash
	s.processRentLocked(mustParseTime(t, "1980-02-29T00:00:00"))
	if s.cash != cash-office.WeeklyRent {
		t.Errorf("cash = %d, want -%d rent", s.cash-cash, office.WeeklyRent)
	}
	if office.NextRentDue != "1980-03-07T00:00:00" || office.MissedRentPayments != 0 {
		t.Errorf("due/missed = %s/%d, want +7 days/0", office.NextRentDue, office.MissedRentPayments)
	}

	// Second due: force insufficient cash -> first miss: 20% late fee, counter = 1.
	s.cash = 30
	s.processRentLocked(mustParseTime(t, "1980-03-07T00:00:00"))
	if office.MissedRentPayments != 1 {
		t.Fatalf("missed = %d, want 1", office.MissedRentPayments)
	}
	var fee *Transaction
	for i := range s.transactions {
		if s.transactions[i].Category == CategoryRentLateFee {
			fee = &s.transactions[i]
		}
	}
	if fee == nil || fee.Amount != -10 {
		t.Fatalf("late fee transaction = %+v, want -10 (20%% of £50)", fee)
	}
	if office.ContractStatus != ContractActive {
		t.Errorf("contract = %s, want active after first miss (SPEC 4.2)", office.ContractStatus)
	}
	if office.NextRentDue != "1980-03-14T00:00:00" {
		t.Errorf("next due = %s, want +7 days", office.NextRentDue)
	}

	// Third due with no cash: second miss -> termination.
	s.cash = 0
	s.processRentLocked(mustParseTime(t, "1980-03-14T00:00:00"))
	if office.ContractStatus != ContractTerminated {
		t.Fatalf("contract = %s, want terminated after second miss (SPEC 4.2)", office.ContractStatus)
	}
}

func TestLoanInterestEveryFourWeeks(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	cash := s.cash

	s.processInterestLocked(mustParseTime(t, "1980-02-29T08:59:00"))
	if s.cash != cash {
		t.Fatalf("cash charged before due instant: %d, want unchanged", s.cash)
	}
	s.processInterestLocked(mustParseTime(t, "1980-02-29T09:00:00"))
	if s.cash != cash-50 {
		t.Errorf("cash = %d, want -50 (5%% of £1000, SPEC 11.1)", s.cash-cash)
	}
	if s.interestDue.Format(GameTimeFormat) != "1980-03-28T09:00:00" {
		t.Errorf("next interest = %s, want +4 weeks", s.interestDue.Format(GameTimeFormat))
	}
	// Principal must not amortise (SPEC 16.9: repayment OPEN).
	if s.player.LoanPrincipal != 1000 {
		t.Errorf("loan principal = %d, want unchanged 1000", s.player.LoanPrincipal)
	}
}

func TestGameOverWhenTerminatedAndCannotAffordNewOffice(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	office := selectSmall(t, s)

	office.ContractStatus = ContractTerminated
	s.cash = 300 // below the £350 cheapest new contract
	s.processLocked(mustParseTime(t, "1980-03-01T09:00:00"))

	if s.status != GameStatusGameOver {
		t.Fatalf("status = %s, want game_over (SPEC 4.2)", s.status)
	}
	if !s.Clock.Snapshot().Paused {
		t.Error("clock not paused at game over")
	}
	// Frozen: further scheduled events must not fire after game over.
	before := len(s.transactions)
	s.processLocked(mustParseTime(t, "1980-04-01T09:00:00"))
	if len(s.transactions) != before {
		t.Errorf("transactions advanced after game over: %d -> %d", before, len(s.transactions))
	}
}

func TestTerminatedOfficeStillAffordableDoesNotEndGame(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	office := selectSmall(t, s) // cash 650
	office.ContractStatus = ContractTerminated

	s.processLocked(mustParseTime(t, "1980-03-01T09:00:00"))
	if s.status != GameStatusRunning {
		t.Fatalf("status = %s, want running (player can still afford a new office)", s.status)
	}
}

func TestReSelectionAfterTermination(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s) // cash 650

	// While active: re-selection is rejected.
	if _, _, err := s.SelectOffice("office-large-01"); err == nil {
		t.Fatal("expected ErrAlreadySelected while contract active")
	}

	// After termination the slot frees up (SPEC 4.2 re-entry rule).
	s.SelectedOffice().ContractStatus = ContractTerminated
	office, cash, err := s.SelectOffice("office-large-01")
	if err != nil {
		t.Fatalf("SelectOffice after termination: %v", err)
	}
	if office.ID != "office-large-01" || cash != 650-450 {
		t.Errorf("office/cash = %s/%d, want office-large-01/200", office.ID, cash)
	}
}

func TestTickAdvancesClockAndProcessesEvents(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	selectSmall(t, s)
	withDeterministicRand(t, func(int) int { return 0 })

	// 15 real seconds at speed 1 = 30 game minutes (SPEC 2.2), landing on a generation tick.
	changed := s.Tick(15 * time.Second)
	if !changed {
		t.Fatal("Tick reported no change over a generation interval")
	}
	if got := s.Clock.Snapshot().GameDatetime; got != "1980-02-01T09:30:00" {
		t.Errorf("game_datetime = %s, want 1980-02-01T09:30:00", got)
	}
	if len(s.packages) != 1 {
		t.Errorf("packages = %d, want 1 from the 09:30 tick", len(s.packages))
	}

	// Paused clock: elapsed time must not advance game time (SPEC 2.3).
	s.Clock.SetPaused(true)
	s.Tick(10 * time.Second)
	if got := s.Clock.Snapshot().GameDatetime; got != "1980-02-01T09:30:00" {
		t.Errorf("paused game_datetime = %s, want unchanged", got)
	}
}

func TestSkipToNextOpeningProcessesSkippedInterval(t *testing.T) {
	s := newStateAt(t, "1980-02-01T17:30:00") // Friday after close
	selectSmall(t, s)
	withDeterministicRand(t, func(int) int { return 0 })

	s.SkipToNextOpening()
	snap := s.Clock.Snapshot()
	if snap.GameDatetime != "1980-02-02T10:00:00" {
		t.Fatalf("after skip game_datetime = %s, want Saturday 10:00 opening", snap.GameDatetime)
	}
	// The skipped interval must not generate packages (Saturday 10:00 is the landing
	// instant itself; ticks between Friday close and Saturday opening were closed).
	for _, p := range s.packages {
		if mustParseTime(t, p.ReceivedAt).Before(mustParseTime(t, snap.GameDatetime)) {
			t.Errorf("package generated at %s during skipped closed interval", p.ReceivedAt)
		}
	}
}
