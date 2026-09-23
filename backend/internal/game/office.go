package game

import (
	"errors"
	"fmt"
	"time"
)

// OfficeDefinition is immutable catalogue data for a selectable M1 office. It carries only the
// static options shown by GET /api/v1/offices — no runtime fields such as next-rent-due, current
// storage usage, contract status or missed-rent count (those are created when an office is
// selected). Definitions are backend-owned and cannot be mutated by callers.
type OfficeDefinition struct {
	ID                   string
	Type                 string
	DownPayment          int // integer pounds
	WeeklyRent           int // integer pounds
	RentPrepaidWeeks     int
	StorageBase          int
	StorageMax           int
	EmployeeCapacity     int
	BicycleCapacity      int
	VehicleCapacity      int
	AcceptedPackageSizes []string
}

// officeCatalogue is the canonical, immutable set of M1 selectable offices in deterministic order
// (small then large). It is backend-owned data; callers access it only through OfficeDefinitions,
// which returns copies so the canonical definitions cannot be mutated.
var officeCatalogue = []OfficeDefinition{
	{
		ID:                   "office-small-01",
		Type:                 "small",
		DownPayment:          350,
		WeeklyRent:           50,
		RentPrepaidWeeks:     4,
		StorageBase:          100,
		StorageMax:           150,
		EmployeeCapacity:     5,
		BicycleCapacity:      5,
		VehicleCapacity:      1,
		AcceptedPackageSizes: []string{"small", "medium"},
	},
	{
		ID:                   "office-large-01",
		Type:                 "large",
		DownPayment:          450,
		WeeklyRent:           75,
		RentPrepaidWeeks:     4,
		StorageBase:          150,
		StorageMax:           250,
		EmployeeCapacity:     7,
		BicycleCapacity:      7,
		VehicleCapacity:      2,
		AcceptedPackageSizes: []string{"small", "medium"},
	},
}

// OfficeDefinitions returns a copy of the canonical M1 office catalogue in deterministic order.
// Callers cannot mutate the canonical definitions through the returned values (each element and its
// accepted-sizes slice are copied).
func OfficeDefinitions() []OfficeDefinition {
	out := make([]OfficeDefinition, len(officeCatalogue))
	for i, def := range officeCatalogue {
		def.AcceptedPackageSizes = append([]string(nil), def.AcceptedPackageSizes...)
		out[i] = def
	}
	return out
}

// findOfficeDefinition resolves a canonical selectable office by ID.
func findOfficeDefinition(id string) (OfficeDefinition, bool) {
	for _, def := range officeCatalogue {
		if def.ID == id {
			return def, true
		}
	}
	return OfficeDefinition{}, false
}

// StorageState is the runtime storage view of a selected office.
type StorageState struct {
	Base    int // initial capacity from the definition
	Current int // current capacity (equals Base until future upgrades)
	Max     int // maximum upgradable capacity from the definition
	Used    int // units currently in use (0 at selection; M1D has no package logic)
}

// RuntimeOffice is a selected office instance created when an office is chosen. It carries the
// runtime fields that do not exist on the immutable catalogue definition: head-office flag, next
// rent due date, current storage usage, contract status and missed-rent count (SPEC 4.1).
type RuntimeOffice struct {
	ID                   string
	Type                 string
	IsHeadOffice         bool
	DownPayment          int // integer pounds
	WeeklyRent           int // integer pounds
	RentPrepaidWeeks     int
	NextRentDue          string // canonical game-time ISO format (midnight of the first rent-due date)
	Storage              StorageState
	EmployeeCapacity     int
	BicycleCapacity      int
	VehicleCapacity      int
	AcceptedPackageSizes []string
	ContractStatus       string // "active" at selection
	MissedRentPayments   int    // 0 at selection
}

// ErrOfficeNotFound is returned by SelectOffice when the requested office_id does not identify a
// canonical selectable office.
var ErrOfficeNotFound = errors.New("office not found")

// ErrAlreadySelected is returned by SelectOffice when a head office has already been selected (M1
// permits exactly one selection).
var ErrAlreadySelected = errors.New("office already selected")

// InsufficientFundsError is returned by SelectOffice when the requested office exists but the
// current cash is less than its down payment. It carries the required and available amounts for
// API error details.
type InsufficientFundsError struct {
	Required  int // integer pounds required (the down payment)
	Available int // integer pounds currently available
}

func (e *InsufficientFundsError) Error() string {
	return fmt.Sprintf("insufficient funds: need %d, have %d", e.Required, e.Available)
}

// SelectOffice atomically selects a canonical office as the head office. It performs one atomic
// game operation under s.mu in the deterministic validation order required by SPEC 4.3:
// (1) already-selected state; (2) office existence; (3) sufficient funds. On success it constructs
// the runtime Office, deducts the exact down payment from cash, and appends exactly one
// office_down_payment finance transaction — all committed together. If any validation fails, no
// state is changed (no partial mutation).
//
// Lock ordering: SelectOffice holds s.mu while reading the authoritative clock via Clock.Now()
// (which briefly takes the clock's read lock). No code path ever acquires a clock lock and then
// s.mu, so this nesting cannot deadlock. All locks are released before returning, so no game lock
// is held during downstream HTTP JSON encoding.
func (s *GameState) SelectOffice(officeID string) (*RuntimeOffice, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// (1) already-selected state — checked before existence and funds per the canonical order.
	if s.selectedOffice != nil {
		return nil, 0, ErrAlreadySelected
	}

	// (2) office existence.
	def, ok := findOfficeDefinition(officeID)
	if !ok {
		return nil, 0, ErrOfficeNotFound
	}

	// (3) sufficient funds.
	if s.cash < def.DownPayment {
		return nil, 0, &InsufficientFundsError{Required: def.DownPayment, Available: s.cash}
	}

	// Commit atomically: construct the runtime office, deduct the down payment, append one
	// transaction — all under the same lock so a concurrent selection cannot interleave.
	now := s.Clock.Now() // authoritative fictional clock time; no wall clock
	office := &RuntimeOffice{
		ID:                   def.ID,
		Type:                 def.Type,
		IsHeadOffice:         true,
		DownPayment:          def.DownPayment,
		WeeklyRent:           def.WeeklyRent,
		RentPrepaidWeeks:     def.RentPrepaidWeeks,
		NextRentDue:          computeNextRentDue(now, def.RentPrepaidWeeks),
		Storage:              StorageState{Base: def.StorageBase, Current: def.StorageBase, Max: def.StorageMax, Used: 0},
		EmployeeCapacity:     def.EmployeeCapacity,
		BicycleCapacity:      def.BicycleCapacity,
		VehicleCapacity:      def.VehicleCapacity,
		AcceptedPackageSizes: append([]string(nil), def.AcceptedPackageSizes...),
		ContractStatus:       "active",
		MissedRentPayments:   0,
	}

	s.cash -= def.DownPayment
	s.selectedOffice = office
	s.transactions = append(s.transactions, Transaction{
		Type:         "office_down_payment",
		Amount:       -def.DownPayment,
		GameDatetime: now.Format("2006-01-02T15:04:05"),
		BalanceAfter: s.cash,
		OfficeID:     def.ID,
	})

	return office, s.cash, nil
}

// computeNextRentDue returns the canonical next-rent-due instant for an office selected at
// selectionTime (SPEC 4.1/4.3): the down payment covers rentPrepaidWeeks weeks, so weekly rent is
// first due that many weeks after the selection date, at midnight. Example: selected Feb 1 -> Feb 29.
func computeNextRentDue(selection time.Time, prepaidWeeks int) string {
	due := time.Date(selection.Year(), selection.Month(), selection.Day()+prepaidWeeks*7, 0, 0, 0, 0, selection.Location())
	return due.Format("2006-01-02T15:04:05")
}
