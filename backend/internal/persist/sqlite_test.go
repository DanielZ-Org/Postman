package persist

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

func TestSQLiteLoadOnEmptyStoreReturnsNil(t *testing.T) {
	store, err := NewSQLite(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer store.Close()

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if snap != nil {
		t.Fatalf("Load = %+v, want nil for a fresh database", snap)
	}
}

func TestSQLiteSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.db")
	store, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}

	// A snapshot with every representative field populated.
	snap := &game.Snapshot{
		Version:           1,
		GameTime:          "1980-02-01T09:30:00",
		Speed:             4,
		Paused:            true,
		Status:            "running",
		Player:            game.Player{ID: "player-001", Trait: "financial", LoanPrincipal: 1000, HeadOfficeID: "office-small-01"},
		Cash:              650,
		NextTxnID:         2,
		NextPkgSeq:        3,
		NextEmpSeq:        1,
		TotalHires:        1,
		LastGeneration:    "1980-02-01T09:30:00",
		InterestDue:       "1980-02-29T09:00:00",
		PayrollDue:        "1980-02-05T09:00:00",
		DeliveredToday:    4,
		DeliveredTodayKey: "1980-02-01",
		DailyRevenue:      25,
		DailyRevenueKey:   "1980-02-01",
		Packages: []*game.Package{
			{
				ID: "pkg-000001", Size: "small", ServiceType: "express",
				DestinationType: "local", StorageUnits: 1, DeliveryCapacityUnits: 2,
				BaseFee: 12, ReceivedAt: "1980-02-01T09:30:00", DueAt: "1980-02-03T09:30:00",
				Status: "out_for_delivery", AssignedEmployeeID: strPtr("emp-0001"),
			},
		},
		Employees: []*game.Employee{
			{ID: "emp-0001", Status: "out_for_delivery", RunsToday: 1, AccruedWages: 2, PackagesDeliveredThisWeek: 3, RunsTodayKey: "1980-02-01"},
		},
		Runs: []game.SnapshotRun{
			{EmployeeID: "emp-0001", PackageIDs: []string{"pkg-000001"}, Phase: "out_for_delivery", PhaseEnd: "1980-02-01T13:00:00"},
		},
		Transactions: []game.Transaction{
			{ID: "txn-000001", GameDatetime: "1980-02-01T09:00:00", Category: "loan_disbursement", Amount: 1000, Description: "Loan disbursement", ReferenceID: ""},
		},
		Office: &game.RuntimeOffice{
			ID: "office-small-01", ContractStatus: "active", WeeklyRent: 50,
			NextRentDue: "1980-02-29T00:00:00", MissedRentPayments: 1,
			AcceptedPackageSizes: []string{"small", "medium", "large"},
		},
	}
	if err := store.Save(snap); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Same process: read back through the connection.
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertSnapshotEqual(t, got, snap)

	// Re-open the file fresh: data must be durable on disk, not cached in memory.
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite(reopen): %v", err)
	}
	defer reopened.Close()
	got, err = reopened.Load()
	if err != nil {
		t.Fatalf("Load(reopened): %v", err)
	}
	assertSnapshotEqual(t, got, snap)
}

func TestSQLiteSaveOverwritesPreviousSnapshot(t *testing.T) {
	store, err := NewSQLite(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer store.Close()

	first := &game.Snapshot{Version: 1, GameTime: "1980-02-01T09:00:00", Status: "running"}
	second := &game.Snapshot{Version: 1, GameTime: "1980-03-01T10:00:00", Status: "game_over", Cash: 42}

	if err := store.Save(first); err != nil {
		t.Fatalf("Save(first): %v", err)
	}
	if err := store.Save(second); err != nil {
		t.Fatalf("Save(second): %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.GameTime != second.GameTime || got.Status != second.Status || got.Cash != second.Cash {
		t.Fatalf("Load = %+v, want latest snapshot %+v", got, second)
	}
}

func TestSQLiteSaveRejectsNilSnapshot(t *testing.T) {
	store, err := NewSQLite(filepath.Join(t.TempDir(), "game.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer store.Close()
	if err := store.Save(nil); err == nil {
		t.Fatal("Save(nil) succeeded, want error")
	}
}

func TestSQLiteLoadCorruptDataReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.db")
	store, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO save (id, data, updated_at) VALUES (1, ?, datetime('now'))`, "{not json"); err != nil {
		t.Fatalf("seed corrupt row: %v", err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("Load(corrupt) succeeded, want decode error")
	}
	store.Close()
}

func TestSQLiteCreatesMissingParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "game.db")
	store, err := NewSQLite(path)
	if err != nil {
		t.Fatalf("NewSQLite(nested): %v", err)
	}
	defer store.Close()
	if err := store.Save(&game.Snapshot{Version: 1, GameTime: "1980-02-01T09:00:00"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestStoreSatisfiesGameInterface(t *testing.T) {
	// Compile-time guarantee that SQLiteStore implements game.Store.
	var _ game.Store = (*SQLiteStore)(nil)
}

func assertSnapshotEqual(t *testing.T, got, want *game.Snapshot) {
	t.Helper()
	if got == nil {
		t.Fatal("snapshot is nil")
	}
	// Compare through JSON so both fresh and reloaded snapshots use identical logic
	// (matches how the adapter actually stores them).
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("snapshot mismatch:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
	if got.Version != want.Version {
		t.Errorf("version = %d, want %d", got.Version, want.Version)
	}
}

func strPtr(s string) *string { return &s }
