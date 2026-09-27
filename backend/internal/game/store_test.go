package game

import (
	"testing"
)

// TestSnapshotRestoreRoundTrip captures a rich mid-game state, restores it into a
// fresh instance and verifies every observable survives (SPEC 13: save/load before
// every endpoint call, idempotent).
func TestSnapshotRestoreRoundTrip(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))
	// Start a run so in-flight state exists too.
	if _, _, err := s.AssignDelivery("emp-0001", 1); err != nil {
		t.Fatalf("AssignDelivery: %v", err)
	}
	// Advance into the delivery phase and settle a payroll so counters move.
	s.processLocked(mustParseTime(t, "1980-02-01T10:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-01T13:00:00"))
	s.generatePackagesLocked(mustParseTime(t, "1980-02-01T14:00:00"))
	s.processLocked(mustParseTime(t, "1980-02-05T09:00:00"))
	s.Clock.SetPaused(true)
	s.Clock.SetSpeed(4)

	snap := s.Snapshot()
	// Snapshot must be JSON-round-trippable (it is persisted as JSON).
	if snap.GameTime == "" || len(snap.Packages) == 0 || len(snap.Transactions) == 0 {
		t.Fatalf("snapshot incomplete: %+v", snap)
	}

	restored := NewInitialState()
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Clock: instant, speed, pause.
	gotClock := restored.Clock.Snapshot()
	wantClock := s.Clock.Snapshot()
	if gotClock.GameDatetime != wantClock.GameDatetime || gotClock.Speed != wantClock.Speed || gotClock.Paused != wantClock.Paused {
		t.Errorf("clock = %+v, want %+v", gotClock, wantClock)
	}

	// Money and player.
	if restored.cash != s.cash || restored.player.LoanPrincipal != s.player.LoanPrincipal || restored.player.Trait != s.player.Trait {
		t.Errorf("cash/player = %d/%+v, want %d/%+v", restored.cash, restored.player, s.cash, s.player)
	}
	if restored.totalHires != s.totalHires {
		t.Errorf("totalHires = %d, want %d", restored.totalHires, s.totalHires)
	}

	// Office contract.
	wantOffice := s.SelectedOffice()
	gotOffice := restored.SelectedOffice()
	if gotOffice == nil || wantOffice == nil {
		t.Fatalf("office missing after restore: got=%v want=%v", gotOffice, wantOffice)
	}
	if gotOffice.ID != wantOffice.ID || gotOffice.ContractStatus != wantOffice.ContractStatus ||
		gotOffice.NextRentDue != wantOffice.NextRentDue || gotOffice.MissedRentPayments != wantOffice.MissedRentPayments {
		t.Errorf("office = %+v, want %+v", gotOffice, wantOffice)
	}

	// Packages, employees, runs, transactions, counters.
	if len(restored.packages) != len(s.packages) || len(restored.employees) != len(s.employees) ||
		len(restored.runs) != len(s.runs) || len(restored.transactions) != len(s.transactions) {
		t.Fatalf("counts = pkg %d/%d, emp %d/%d, runs %d/%d, txns %d/%d",
			len(restored.packages), len(s.packages), len(restored.employees), len(s.employees),
			len(restored.runs), len(s.runs), len(restored.transactions), len(s.transactions))
	}
	for i := range s.packages {
		if !samePackage(restored.packages[i], s.packages[i]) {
			t.Errorf("package[%d] = %+v, want %+v", i, *restored.packages[i], *s.packages[i])
		}
	}
	for i := range s.employees {
		got, want := restored.employees[i], s.employees[i]
		if got.ID != want.ID || got.Status != want.Status || got.RunsToday != want.RunsToday ||
			got.AccruedWages != want.AccruedWages || got.PackagesDeliveredThisWeek != want.PackagesDeliveredThisWeek ||
			got.RunsTodayKey != want.RunsTodayKey || got.Skills == nil || len(got.Skills) != len(want.Skills) {
			t.Errorf("employee[%d] = %+v, want %+v", i, *got, *want)
		}
	}
	for i := range s.runs {
		got, want := restored.runs[i], s.runs[i]
		if got.EmployeeID != want.EmployeeID || got.Phase != want.Phase || !got.PhaseEnd.Equal(want.PhaseEnd) ||
			len(got.PackageIDs) != len(want.PackageIDs) {
			t.Errorf("run[%d] = %+v, want %+v", i, *got, *want)
		}
	}
	for i := range s.transactions {
		if restored.transactions[i] != s.transactions[i] {
			t.Errorf("transaction[%d] = %+v, want %+v", i, restored.transactions[i], s.transactions[i])
		}
	}
	if restored.deliveredToday != s.deliveredToday || restored.deliveredTodayKey != s.deliveredTodayKey ||
		restored.dailyRevenue != s.dailyRevenue || restored.dailyRevenueKey != s.dailyRevenueKey {
		t.Errorf("daily counters = %d/%s/%d/%s, want %d/%s/%d/%s",
			restored.deliveredToday, restored.deliveredTodayKey, restored.dailyRevenue, restored.dailyRevenueKey,
			s.deliveredToday, s.deliveredTodayKey, s.dailyRevenue, s.dailyRevenueKey)
	}
	if restored.nextPkgSeq != s.nextPkgSeq || restored.nextEmpSeq != s.nextEmpSeq || restored.nextTxnID != s.nextTxnID {
		t.Errorf("sequences = %d/%d/%d, want %d/%d/%d",
			restored.nextPkgSeq, restored.nextEmpSeq, restored.nextTxnID,
			s.nextPkgSeq, s.nextEmpSeq, s.nextTxnID)
	}
	if !restored.payrollDue.Equal(s.payrollDue) || !restored.interestDue.Equal(s.interestDue) ||
		!restored.lastGeneration.Equal(s.lastGeneration) {
		t.Errorf("schedules = payroll %s interest %s gen %s, want %s / %s / %s",
			restored.payrollDue, restored.interestDue, restored.lastGeneration,
			s.payrollDue, s.interestDue, s.lastGeneration)
	}
	if restored.status != s.status {
		t.Errorf("status = %s, want %s", restored.status, s.status)
	}
}

// TestRestoreIdempotent applies the same snapshot twice: the second restore must
// not drift (SPEC 13: idempotent).
func TestRestoreIdempotent(t *testing.T) {
	s := newAssignedOfficeState(t)
	addStoredPackage(s, "pkg-000001", "small", ServiceNormal, mustParseTime(t, "1980-02-01T08:00:00"), mustParseTime(t, "1980-02-06T08:00:00"))
	snap := s.Snapshot()

	restored := NewInitialState()
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("first Restore: %v", err)
	}
	first := restored.Snapshot()
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	second := restored.Snapshot()

	if first.GameTime != second.GameTime || first.Cash != second.Cash || first.Status != second.Status {
		t.Errorf("idempotency drift: %+v vs %+v", first, second)
	}
	if len(first.Packages) != len(second.Packages) || len(first.Transactions) != len(second.Transactions) ||
		len(first.Employees) != len(second.Employees) {
		t.Errorf("idempotency count drift: %+v vs %+v", first, second)
	}
}

// TestRestoreNilIsNoOp: restoring nothing must leave a fresh game untouched.
func TestRestoreNilIsNoOp(t *testing.T) {
	s := NewInitialState()
	before := s.Snapshot()
	if err := s.Restore(nil); err != nil {
		t.Fatalf("Restore(nil): %v", err)
	}
	after := s.Snapshot()
	if before.GameTime != after.GameTime || before.Cash != after.Cash || len(before.Packages) != len(after.Packages) {
		t.Errorf("Restore(nil) mutated state: %+v -> %+v", before, after)
	}
}

// TestRestoreMigratesV1PoundsToVPence verifies the load-time unit migration (SPEC 4.3 board
// decision): a v1 snapshot whose monetary fields are integer pounds loads successfully, every
// monetary value is multiplied by 100 into pence, and the state saves forward as schema v2 so
// existing dev saves keep their values across the unit change.
func TestRestoreMigratesV1PoundsToVPence(t *testing.T) {
	v1 := &Snapshot{
		Version:        1,
		GameTime:       "1980-02-03T09:00:00",
		Speed:          1,
		Paused:         false,
		Status:         GameStatusRunning,
		Player:         Player{ID: "player-1", Name: "Daniel", Trait: "financial", LoanPrincipal: 1000},
		Cash:           650,
		Office:         &RuntimeOffice{ID: "office-small-01", Type: "small", IsHeadOffice: true, DownPayment: 350, WeeklyRent: 50, ContractStatus: ContractActive},
		Packages:       []*Package{{ID: "pkg-000001", Size: "small", ServiceType: ServiceNormal, BaseFee: 12, FinalRevenue: intPtr(9), Status: PackageDelivered}},
		Employees:      []*Employee{{ID: "emp-0001", Name: "Bob Snail", AccruedWages: 4, CurrentDeliveryMode: ModeFoot, Status: EmployeeReady}},
		Transactions:   []Transaction{{ID: "txn-000002", GameDatetime: "1980-02-01T09:00:00", Category: CategoryOfficeDownPayment, Amount: -350}},
		NextTxnID:      3,
		LastGeneration: "1980-02-03T09:00:00",
		InterestDue:    "1980-02-29T09:00:00",
		PayrollDue:     "1980-02-05T09:00:00",
	}

	restored := NewInitialState()
	if err := restored.Restore(v1); err != nil {
		t.Fatalf("Restore(v1): %v", err)
	}

	got := restored.Snapshot()
	if got.Version != snapshotVersion {
		t.Errorf("version = %d, want %d (saved forward as v2)", got.Version, snapshotVersion)
	}
	if got.Cash != 65000 || got.Player.LoanPrincipal != 100000 {
		t.Errorf("cash/principal = %d/%d, want 65000p/100000p", got.Cash, got.Player.LoanPrincipal)
	}
	if got.Office == nil || got.Office.DownPayment != 35000 || got.Office.WeeklyRent != 5000 {
		t.Errorf("office = %+v, want down payment 35000p weekly rent 5000p", got.Office)
	}
	if len(got.Packages) != 1 || got.Packages[0].BaseFee != 1200 || got.Packages[0].FinalRevenue == nil || *got.Packages[0].FinalRevenue != 900 {
		t.Errorf("package = %+v, want base fee 1200p final revenue 900p", got.Packages)
	}
	if len(got.Employees) != 1 || got.Employees[0].AccruedWages != 400 {
		t.Errorf("employee = %+v, want accrued wages 400p", got.Employees)
	}
	if len(got.Transactions) != 1 || got.Transactions[0].Amount != -35000 {
		t.Errorf("transaction = %+v, want amount -35000p", got.Transactions)
	}

	// The migrated state must be usable: a fresh snapshot round-trips as v2 without further change.
	onceMore := NewInitialState()
	if err := onceMore.Restore(got); err != nil {
		t.Fatalf("Restore(v2): %v", err)
	}
	again := onceMore.Snapshot()
	if again.Cash != got.Cash || again.Version != snapshotVersion {
		t.Errorf("second restore drifted: %+v vs %+v", again, got)
	}
}

// TestRestoreRejectsUnknownVersion: a snapshot from a future schema must fail
// without touching state.
func TestRestoreRejectsUnknownVersion(t *testing.T) {
	s := newAssignedOfficeState(t)
	want := s.Snapshot()

	bad := s.Snapshot()
	bad.Version = 99
	if err := s.Restore(bad); err == nil {
		t.Fatal("Restore(version 99) succeeded, want error")
	}
	got := s.Snapshot()
	if got.Cash != want.Cash || got.GameTime != want.GameTime || len(got.Packages) != len(want.Packages) {
		t.Errorf("failed Restore mutated state: %+v, want %+v", got, want)
	}
}

// samePackage compares two packages by value, dereferencing pointer fields (they
// are deep-copied by Snapshot so pointer equality never holds).
func samePackage(a, b *Package) bool {
	if a.ID != b.ID || a.Size != b.Size || a.ServiceType != b.ServiceType ||
		a.DestinationType != b.DestinationType || a.StorageUnits != b.StorageUnits ||
		a.DeliveryCapacityUnits != b.DeliveryCapacityUnits || a.BaseFee != b.BaseFee ||
		a.ReceivedAt != b.ReceivedAt || a.DueAt != b.DueAt || a.Status != b.Status {
		return false
	}
	return ptrStrEqual(a.AssignedEmployeeID, b.AssignedEmployeeID) &&
		ptrStrEqual(a.DeliveredAt, b.DeliveredAt) &&
		ptrIntEqual(a.FinalRevenue, b.FinalRevenue)
}

func ptrStrEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func ptrIntEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
